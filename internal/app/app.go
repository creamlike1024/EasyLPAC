package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"slices"
	"time"

	"EasyLPAC/internal/lpa"
	"EasyLPAC/internal/settings"
	"EasyLPAC/internal/transport"
)

// ErrBusy is returned when a card task is already running.
var ErrBusy = errors.New("another operation is in progress")

// TaskKind names a card operation; the UI says it in its language.
type TaskKind string

const (
	TaskRefresh             TaskKind = "refresh"
	TaskEnable              TaskKind = "enable-profile"
	TaskDisable             TaskKind = "disable-profile"
	TaskDelete              TaskKind = "delete-profile"
	TaskRename              TaskKind = "rename-profile"
	TaskSetDefaultSMDP      TaskKind = "set-default-smdp"
	TaskSendNotifications   TaskKind = "send-notifications"
	TaskRemoveNotifications TaskKind = "remove-notifications"
	TaskDownload            TaskKind = "download-profile"
)

// NoticeKind is what a Notice reports.
type NoticeKind int

const (
	// NoticeDone: Task succeeded. After a profile change, Sent
	// notifications were sent and Unsent could not be.
	NoticeDone NoticeKind = iota
	// NoticeCancelled: Task was cancelled.
	NoticeCancelled
	// NoticeFailed: Task failed with Err.
	NoticeFailed
	// NoticeSent: Sent of Total notifications were sent; Err is the
	// first failure, if any.
	NoticeSent
	// NoticeRemoved: Total notifications were removed.
	NoticeRemoved
)

// Notice reports the outcome of a task to the user; the UI words it.
type Notice struct {
	Kind   NoticeKind
	Task   TaskKind
	Sent   int
	Unsent int
	Total  int
	Err    error
}

// IsError reports whether the notice is about a failure.
func (n Notice) IsError() bool {
	return n.Kind == NoticeFailed || n.Err != nil
}

// StageMark is a stage a task reached, and when.
type StageMark struct {
	Name string
	At   time.Time
}

// Task is the card operation in progress.
type Task struct {
	Kind  TaskKind
	Stage string
	// Stages are the stages reached so far, in order.
	Stages  []StageMark
	Started time.Time
	cancel  context.CancelFunc
}

// Cancel asks the task to stop at its next safe point.
func (t *Task) Cancel() {
	if t != nil && t.cancel != nil {
		t.cancel()
	}
}

// ConfirmRequest is a download preview waiting for the user.
type ConfirmRequest struct {
	Metadata lpa.ProfileMetadata
	answer   chan bool
}

// App is the application state and its operations. Every exported field
// and method is owned by the UI thread: work runs on goroutines and its
// results are applied through the dispatch function, which must run its
// argument on the UI thread (mygo's Window.Update).
type App struct {
	Settings *settings.Config
	Log      *LogBuffer
	Logger   *slog.Logger

	// Devices of the selected backend, as last discovered.
	Devices []transport.Device
	// Card data of the selected device; nil until loaded.
	Chip          *lpa.EuiccInfo
	Profiles      []*lpa.Profile
	Notifications []*lpa.Notification
	// Loaded reports that the card data above belongs to the selected
	// device.
	Loaded bool
	// Task is the operation in progress, nil when idle.
	Task *Task
	// Confirm is the download preview waiting for an answer.
	Confirm *ConfirmRequest
	// LastError is the error of the last failed task, cleared by the next.
	LastError error
	// Discovering is set while devices are being looked for.
	Discovering bool

	notices  []Notice
	dispatch func(func())
	level    *slog.LevelVar
	// Seams for tests.
	newService func(cfg *settings.Config, onStage func(string)) (lpa.Service, error)
	discover   func(ctx context.Context, backend string) ([]transport.Device, error)
}

// Options configure New.
type Options struct {
	Settings *settings.Config
	// Dispatch runs fn on the UI thread.
	Dispatch func(fn func())
	// LogLimit is the number of log entries kept in memory (0: 10000).
	LogLimit int
	// Version is the application version written to the log header.
	Version string

	// NewService and Discover replace the real card and device
	// discovery (tests, demo builds).
	NewService func(cfg *settings.Config, onStage func(string)) (lpa.Service, error)
	Discover   func(ctx context.Context, backend string) ([]transport.Device, error)
}

// New creates the application state.
func New(o Options) *App {
	limit := o.LogLimit
	if limit == 0 {
		limit = 10000
	}
	buf := NewLogBuffer(limit)
	level := new(slog.LevelVar)
	a := &App{
		Settings: o.Settings,
		Log:      buf,
		Logger:   slog.New(buf.Handler(level)),
		dispatch: o.Dispatch,
		level:    level,
		discover: DiscoverDevices,
	}
	if a.dispatch == nil {
		a.dispatch = func(fn func()) { fn() }
	}
	a.newService = func(cfg *settings.Config, onStage func(string)) (lpa.Service, error) {
		return newService(cfg, onStage, a.Logger.With("component", "card"))
	}
	if o.NewService != nil {
		a.newService = o.NewService
	}
	if o.Discover != nil {
		a.discover = o.Discover
	}
	a.syncLevel()
	buf.SetHeader(
		"EasyLPAC "+o.Version+", libeuicc from lpac "+LibeuiccVersion(),
		"OS "+runtime.GOOS+"/"+runtime.GOARCH,
		fmt.Sprintf("debug: apdu=%v http=%v", o.Settings.DebugAPDU, o.Settings.DebugHTTP),
	)
	a.Logger.Info("started", "version", o.Version, "libeuicc", LibeuiccVersion(),
		"os", runtime.GOOS+"/"+runtime.GOARCH, "backend", o.Settings.Backend)
	return a
}

// Busy reports whether a card task is running.
func (a *App) Busy() bool { return a.Task != nil }

// TakeNotices returns and clears the pending notices.
func (a *App) TakeNotices() []Notice {
	n := a.notices
	a.notices = nil
	return n
}

func (a *App) notice(n Notice) {
	a.notices = append(a.notices, n)
}

// CurrentDevice is the device id selected for the current backend.
func (a *App) CurrentDevice() string {
	b := a.Settings.Backends[a.Settings.Backend]
	switch a.Settings.Backend {
	case settings.BackendPCSC:
		return b.Reader
	case settings.BackendAT:
		return b.Port
	}
	return ""
}

// run starts fn as the single card task. fn runs on a goroutine; the
// function it returns (if any) is applied on the UI thread.
func (a *App) run(kind TaskKind, fn func(ctx context.Context, t *Task) (func(), error)) error {
	name := string(kind)
	if a.Task != nil {
		return ErrBusy
	}
	ctx, cancel := context.WithCancel(context.Background())
	t := &Task{Kind: kind, Started: time.Now(), cancel: cancel}
	a.Task = t
	a.LastError = nil
	a.Logger.Info("task start", "task", name)
	go func() {
		apply, err := fn(ctx, t)
		cancel()
		a.dispatch(func() {
			a.Task = nil
			a.Confirm = nil
			// The outcome is recorded before apply runs, so that the
			// task's own callbacks see LastError.
			elapsed := time.Since(t.Started).Round(time.Millisecond)
			switch {
			case err == nil:
				a.Logger.Info("task done", "task", name, "elapsed", elapsed)
			case errors.Is(err, context.Canceled) || lpa.IsUserCancelled(err):
				a.Logger.Info("task cancelled", "task", name, "elapsed", elapsed)
				a.notice(Notice{Kind: NoticeCancelled, Task: kind})
			default:
				a.Logger.Error("task failed", "task", name, "elapsed", elapsed, "err", err)
				a.LastError = err
				a.notice(Notice{Kind: NoticeFailed, Task: kind, Err: err})
			}
			if apply != nil {
				apply()
			}
		})
	}()
	return nil
}

func (a *App) stage(t *Task) func(string) {
	return func(s string) {
		a.Logger.Debug("stage", "task", string(t.Kind), "stage", s)
		now := time.Now()
		a.dispatch(func() {
			t.Stage = s
			t.Stages = append(t.Stages, StageMark{Name: s, At: now})
		})
	}
}

// service builds a service from a snapshot of the settings.
func (a *App) service(t *Task) (lpa.Service, error) {
	cfg := *a.Settings
	svc, err := a.newService(&cfg, a.stage(t))
	if err != nil {
		return nil, err
	}
	return knownIDs{svc, a.Log}, nil
}

// RefreshDevices looks for the devices of the selected backend, once:
// at launch and when the user asks (there is no polling). It does not use
// the card and may run alongside a task.
func (a *App) RefreshDevices() {
	backend := a.Settings.Backend
	a.Discovering = true
	go func() {
		devices, err := a.discover(context.Background(), backend)
		a.dispatch(func() {
			if backend != a.Settings.Backend {
				return
			}
			a.Discovering = false
			if err != nil {
				a.Logger.Warn("discover failed", "backend", backend, "err", err)
				a.Devices = nil
				return
			}
			if !slices.Equal(deviceIDs(devices), deviceIDs(a.Devices)) {
				a.Logger.Info("devices", "backend", backend, "count", len(devices))
			}
			a.Devices = devices
			a.syncDevice()
		})
	}()
}

// syncDevice drops the card data of a chosen reader that is gone. It
// never chooses a device: that is the user's to do.
func (a *App) syncDevice() {
	cur := a.CurrentDevice()
	if cur != "" && a.Loaded && a.Settings.Backend == settings.BackendPCSC &&
		!slices.Contains(deviceIDs(a.Devices), cur) {
		a.Logger.Info("reader removed", "reader", cur)
		a.clearCard()
	}
}

// DevicePresent reports whether the selected device was discovered.
func (a *App) DevicePresent() bool {
	return slices.Contains(deviceIDs(a.Devices), a.CurrentDevice())
}

func deviceIDs(ds []transport.Device) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.ID
	}
	return out
}

// SelectBackend switches backend; the card data is cleared.
func (a *App) SelectBackend(backend string) {
	if backend == a.Settings.Backend {
		return
	}
	a.Settings.Backend = backend
	if _, ok := a.Settings.Backends[backend]; !ok {
		a.Settings.Backends[backend] = settings.Backend{}
	}
	a.Devices = nil
	a.clearCard()
	a.RefreshDevices()
}

// SelectDevice selects a device of the current backend and loads it.
func (a *App) SelectDevice(id string) {
	if id == a.CurrentDevice() {
		return
	}
	b := a.Settings.Backends[a.Settings.Backend]
	switch a.Settings.Backend {
	case settings.BackendPCSC:
		b.Reader = id
	case settings.BackendAT:
		b.Port = id
		if b.Baud == 0 {
			b.Baud = settings.DefaultATBaud
		}
		if b.Timeout == 0 {
			b.Timeout = settings.DefaultATTimeout
		}
	}
	a.Settings.Backends[a.Settings.Backend] = b
	a.clearCard()
	if id != "" {
		_ = a.Refresh()
	}
}

// UpdateSettings applies a change to the settings (of this run only).
// Changes that affect how the card is reached clear the loaded data.
func (a *App) UpdateSettings(fn func(cfg *settings.Config)) {
	before := *a.Settings
	fn(a.Settings)
	if before.AID != a.Settings.AID {
		a.clearCard()
	}
	if before.DebugAPDU != a.Settings.DebugAPDU || before.DebugHTTP != a.Settings.DebugHTTP {
		a.syncLevel()
		a.Logger.Info("debug logging", "apdu", a.Settings.DebugAPDU, "http", a.Settings.DebugHTTP)
		if a.Settings.DebugAPDU || a.Settings.DebugHTTP {
			a.Log.SetRaw()
		}
	}
}

// syncLevel logs debug entries while a debug option is on.
func (a *App) syncLevel() {
	if a.Settings.DebugAPDU || a.Settings.DebugHTTP {
		a.level.Set(slog.LevelDebug)
		a.Log.SetRaw()
		return
	}
	a.level.Set(slog.LevelInfo)
}

func (a *App) clearCard() {
	a.Chip, a.Profiles, a.Notifications, a.Loaded = nil, nil, nil, false
}

// cardData is everything read from the card in one refresh.
type cardData struct {
	chip          *lpa.EuiccInfo
	profiles      []*lpa.Profile
	notifications []*lpa.Notification
}

func readCard(ctx context.Context, svc lpa.Service) (*cardData, error) {
	var d cardData
	var err error
	if d.chip, err = svc.ChipInfo(ctx); err != nil {
		return nil, err
	}
	if d.profiles, err = svc.ProfileList(ctx); err != nil {
		return nil, err
	}
	if d.notifications, err = svc.NotificationList(ctx); err != nil {
		return nil, err
	}
	return &d, nil
}

func (a *App) applyCard(d *cardData) func() {
	return func() {
		a.Chip, a.Profiles, a.Notifications, a.Loaded = d.chip, d.profiles, d.notifications, true
		a.Logger.Info("card loaded", "eid", d.chip.EidValue,
			"profiles", len(d.profiles), "notifications", len(d.notifications))
	}
}

// Refresh reads the chip information, profiles and notifications.
func (a *App) Refresh() error {
	return a.run(TaskRefresh, func(ctx context.Context, t *Task) (func(), error) {
		svc, err := a.service(t)
		if err != nil {
			return nil, err
		}
		d, err := readCard(ctx, svc)
		if err != nil {
			return nil, err
		}
		return a.applyCard(d), nil
	})
}

// afterWrite reads the notifications a write produced (sequence numbers
// above maxSeq: tasks run one at a time, so every new one is the write's,
// as the disable notification of the profile an enable turned off), sends
// and removes them when automatic notification is on, and reloads the
// card.
func (a *App) afterWrite(ctx context.Context, svc lpa.Service, maxSeq int, autoNotify bool) (apply func(), sent, unsent int, err error) {
	// The write is done: later steps must not be cancelled by the caller.
	ctx = context.WithoutCancel(ctx)
	if autoNotify {
		list, err := svc.NotificationList(ctx)
		if err != nil {
			return nil, 0, 0, err
		}
		for _, n := range list {
			if n.SeqNumber <= maxSeq {
				continue
			}
			if err := svc.NotificationProcess(ctx, n.SeqNumber, true); err != nil {
				a.Logger.Warn("notification send failed", "seq", n.SeqNumber, "err", err)
				unsent++
				continue
			}
			sent++
		}
	}
	d, err := readCard(ctx, svc)
	if err != nil {
		return nil, sent, unsent, err
	}
	return a.applyCard(d), sent, unsent, nil
}

func maxSeq(ns []*lpa.Notification) int {
	m := -1
	for _, n := range ns {
		m = max(m, n.SeqNumber)
	}
	return m
}

// writeTask runs a card write followed by notification handling.
func (a *App) writeTask(kind TaskKind, write func(ctx context.Context, svc lpa.Service) error) error {
	autoNotify := a.Settings.AutoNotify
	return a.run(kind, func(ctx context.Context, t *Task) (func(), error) {
		svc, err := a.service(t)
		if err != nil {
			return nil, err
		}
		before, err := svc.NotificationList(ctx)
		if err != nil {
			return nil, err
		}
		if err := write(ctx, svc); err != nil {
			return nil, err
		}
		apply, sent, unsent, err := a.afterWrite(ctx, svc, maxSeq(before), autoNotify)
		return func() {
			if apply != nil {
				apply()
			}
			if err == nil {
				a.notice(Notice{Kind: NoticeDone, Task: kind, Sent: sent, Unsent: unsent})
			}
		}, err
	})
}

// SetProfileEnabled enables or disables a profile.
func (a *App) SetProfileEnabled(iccid string, on bool) error {
	if on {
		return a.writeTask(TaskEnable, func(ctx context.Context, svc lpa.Service) error {
			return svc.ProfileEnable(ctx, iccid)
		})
	}
	return a.writeTask(TaskDisable, func(ctx context.Context, svc lpa.Service) error {
		return svc.ProfileDisable(ctx, iccid)
	})
}

// DeleteProfile deletes a (disabled) profile.
func (a *App) DeleteProfile(iccid string) error {
	return a.writeTask(TaskDelete, func(ctx context.Context, svc lpa.Service) error {
		return svc.ProfileDelete(ctx, iccid)
	})
}

// RenameProfile sets a profile's nickname.
func (a *App) RenameProfile(iccid, nickname string) error {
	return a.run(TaskRename, func(ctx context.Context, t *Task) (func(), error) {
		svc, err := a.service(t)
		if err != nil {
			return nil, err
		}
		if err := svc.ProfileNickname(ctx, iccid, nickname); err != nil {
			return nil, err
		}
		profiles, err := svc.ProfileList(context.WithoutCancel(ctx))
		if err != nil {
			return nil, err
		}
		return func() {
			a.Profiles = profiles
			a.notice(Notice{Kind: NoticeDone, Task: TaskRename})
		}, nil
	})
}

// SetDefaultSMDP sets the card's default SM-DP+ address.
func (a *App) SetDefaultSMDP(addr string) error {
	return a.run(TaskSetDefaultSMDP, func(ctx context.Context, t *Task) (func(), error) {
		svc, err := a.service(t)
		if err != nil {
			return nil, err
		}
		if err := svc.ChipDefaultSmdp(ctx, addr); err != nil {
			return nil, err
		}
		chip, err := svc.ChipInfo(context.WithoutCancel(ctx))
		if err != nil {
			return nil, err
		}
		return func() {
			a.Chip = chip
			a.notice(Notice{Kind: NoticeDone, Task: TaskSetDefaultSMDP})
		}, nil
	})
}

// ProcessNotifications sends the notifications with the given sequence
// numbers, removing each one that was sent when remove is set.
func (a *App) ProcessNotifications(seqs []int, remove bool) error {
	return a.run(TaskSendNotifications, func(ctx context.Context, t *Task) (func(), error) {
		svc, err := a.service(t)
		if err != nil {
			return nil, err
		}
		sent := 0
		var firstErr error
		for _, seq := range seqs {
			if ctx.Err() != nil {
				break
			}
			if err := svc.NotificationProcess(ctx, seq, remove); err != nil {
				a.Logger.Warn("notification send failed", "seq", seq, "err", err)
				if firstErr == nil {
					firstErr = fmt.Errorf("#%d: %w", seq, err)
				}
				continue
			}
			sent++
		}
		list, err := svc.NotificationList(context.WithoutCancel(ctx))
		if err != nil {
			return nil, err
		}
		return func() {
			a.Notifications = list
			a.notice(Notice{Kind: NoticeSent, Task: TaskSendNotifications, Sent: sent, Total: len(seqs), Err: firstErr})
		}, nil
	})
}

// RemoveNotifications removes notifications without sending them.
func (a *App) RemoveNotifications(seqs []int) error {
	return a.run(TaskRemoveNotifications, func(ctx context.Context, t *Task) (func(), error) {
		svc, err := a.service(t)
		if err != nil {
			return nil, err
		}
		removed := 0
		for _, seq := range seqs {
			if err := svc.NotificationRemove(ctx, seq); err != nil {
				return nil, err
			}
			removed++
		}
		list, err := svc.NotificationList(context.WithoutCancel(ctx))
		if err != nil {
			return nil, err
		}
		return func() {
			a.Notifications = list
			a.notice(Notice{Kind: NoticeRemoved, Task: TaskRemoveNotifications, Total: removed})
		}, nil
	})
}

// DownloadResult describes a finished download.
type DownloadResult struct {
	ICCID string
	// NotificationSent reports that the install notification was sent
	// and removed (automatic notification on).
	NotificationSent bool
	// NotificationError is why sending it failed, if it did.
	NotificationError error
}

// Download installs a profile. The preview is offered through Confirm,
// which the UI answers with AnswerConfirm. onDone runs on the UI thread
// with the result (nil when the download failed or was cancelled).
func (a *App) Download(info lpa.PullInfo, onDone func(*DownloadResult)) error {
	if onDone == nil {
		onDone = func(*DownloadResult) {}
	}
	autoNotify := a.Settings.AutoNotify
	a.Logger.Info("download requested", "smdp", info.SMDP, "matching_id", info.MatchID,
		"confirmation_code", info.ConfirmCode, "imei_set", info.IMEI != "")
	return a.run(TaskDownload, func(ctx context.Context, t *Task) (func(), error) {
		svc, err := a.service(t)
		if err != nil {
			return func() { onDone(nil) }, err
		}
		outcome, err := svc.ProfileDownload(ctx, lpa.DownloadRequest{
			PullInfo: info,
			Confirm: func(meta lpa.ProfileMetadata) (bool, error) {
				req := &ConfirmRequest{Metadata: meta, answer: make(chan bool, 1)}
				a.dispatch(func() { a.Confirm = req })
				select {
				case ok := <-req.answer:
					return ok, nil
				case <-ctx.Done():
					return false, ctx.Err()
				}
			},
		})
		if err != nil {
			return func() { onDone(nil) }, err
		}
		res := &DownloadResult{}
		ctx = context.WithoutCancel(ctx)
		if outcome != nil && outcome.Notification != nil {
			res.ICCID = outcome.Notification.Iccid
			if autoNotify {
				a.stage(t)("send_notification")
				if err := svc.NotificationProcess(ctx, outcome.Notification.SeqNumber, true); err != nil {
					res.NotificationError = err
					a.Logger.Warn("install notification failed", "seq", outcome.Notification.SeqNumber, "err", err)
				} else {
					res.NotificationSent = true
				}
			}
		}
		d, err := readCard(ctx, svc)
		if err != nil {
			return func() { onDone(res) }, err
		}
		apply := a.applyCard(d)
		return func() {
			apply()
			a.Logger.Info("profile installed", "iccid", res.ICCID, "notification_sent", res.NotificationSent)
			onDone(res)
		}, nil
	})
}

// AnswerConfirm answers the pending download preview.
func (a *App) AnswerConfirm(install bool) {
	if a.Confirm == nil {
		return
	}
	a.Logger.Info("preview answered", "install", install, "iccid", a.Confirm.Metadata.ICCID)
	a.Confirm.answer <- install
	a.Confirm = nil
}
