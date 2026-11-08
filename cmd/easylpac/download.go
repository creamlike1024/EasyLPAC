package main

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"EasyLPAC/internal/activation"
	"EasyLPAC/internal/app"
	"EasyLPAC/internal/lpa"
	"EasyLPAC/locales"
)

// The steps of the Add eSIM dialog.
const (
	dlInput = iota
	dlRunning
	dlDone
	dlFailed
)

type download struct {
	open bool
	step int
	// mode is 0 for an activation code, 1 for the fields typed in.
	mode                         int
	code                         string
	smdp, matchID, confirm, imei string
	advanced                     bool
	inputError                   string
	result                       *app.DownloadResult
	err                          error
	installed                    lpa.ProfileMetadata
	// smdpShown is the server of the download under way.
	smdpShown string
}

func (g *gui) openDownload() {
	if g.app.Busy() {
		return
	}
	g.dl = download{open: true}
	// An activation code already on the clipboard is offered at once.
	if s := strings.TrimSpace(g.clip.ReadText()); strings.HasPrefix(activation.Complete(s), "LPA:1$") {
		if _, _, err := activation.Decode(activation.Complete(s)); err == nil {
			g.dl.code = activation.Complete(s)
		}
	}
}

// pullInfo is what the form describes, or why it cannot be used.
func (d *download) pullInfo() (lpa.PullInfo, bool, error) {
	if d.mode == 1 {
		if strings.TrimSpace(d.smdp) == "" {
			return lpa.PullInfo{}, false, errors.New("") // nothing typed yet: no message
		}
		return lpa.PullInfo{
			SMDP:        strings.TrimSpace(d.smdp),
			MatchID:     strings.TrimSpace(d.matchID),
			ConfirmCode: strings.TrimSpace(d.confirm),
			IMEI:        strings.TrimSpace(d.imei),
		}, false, nil
	}
	code := strings.TrimSpace(d.code)
	if code == "" {
		return lpa.PullInfo{}, false, errors.New("")
	}
	info, needCode, err := activation.Decode(activation.Complete(code))
	if err != nil {
		return info, false, errors.New(locales.DlNotACode())
	}
	info.ConfirmCode = strings.TrimSpace(d.confirm)
	info.IMEI = strings.TrimSpace(d.imei)
	return info, needCode, nil
}

func (g *gui) downloadModal(c *ui.Context) {
	d := &g.dl
	t := c.Theme()
	// Anchored at the top rather than centered: the dialog grows and
	// shrinks downwards between steps and input modes, so what is near
	// its top (the mode switch) stays under the pointer.
	m := ui.DialogBase(c, &d.open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.32)).Justify(ui.Start).Padding(96, 0, 0)
		panel.Width(508).Padding(24).Radius(16).Background(t.Background).
			Shadow(0, 12, 40, 0, ui.RGBA(0, 0, 0, 0.22)).Border(1, t.Border)
		ui.Column(c).Gap(14).Children(func() {
			switch d.step {
			case dlInput:
				g.downloadInput(c)
			case dlRunning:
				g.downloadRunning(c)
			case dlDone:
				g.downloadDone(c)
			case dlFailed:
				g.downloadFailed(c)
			}
		})
	})
	// The dialog stays while the card is written.
	if m.Dismissed() && d.step == dlRunning {
		d.open = true
	}
	if files := m.DroppedFiles(); len(files) > 0 && d.step == dlInput {
		g.scanQRFile(files[0])
	}
}

func (g *gui) downloadInput(c *ui.Context) {
	t := c.Theme()
	d := &g.dl
	info, needCode, err := d.pullInfo()
	ui.Column(c).Gap(14).Children(func() {
		ui.Row(c).Gap(8).Children(func() {
			ui.Text(c, locales.DlTitle()).FontSize(18).Bold().Grow(1)
			other := locales.DlUseManual()
			if d.mode == 1 {
				other = locales.DlUseCode()
			}
			if ui.Link(c, other, "").FontSize(13).Clicked() {
				d.mode = 1 - d.mode
				d.inputError = ""
			}
		})
		ui.Column(c).Gap(14).Children(func() {
			if d.mode == 0 {
				ui.Field(c, locales.DlModeCode(), func() {
					in := ui.TextInput(c, &d.code).Placeholder("LPA:1$smdp.example.com$MATCHING-ID").
						Label(locales.DlModeCode()).Font("monospace").AutoFocus()
					if err != nil && err.Error() != "" {
						in.Invalid(true)
					}
				})
				ui.Text(c, locales.DlCodeHint()).FontSize(12).TextColor(t.TextMuted)
				ui.Row(c).Gap(8).Children(func() {
					if ui.Button(c, "").Radius(14).Children(func() {
						ui.Icon(c, iconPaste).Size(16, 16)
						ui.Text(c, locales.DlPaste()).SingleLine()
					}).Clicked() {
						g.pasteActivation(c)
					}
					if ui.Button(c, "").Radius(14).Children(func() {
						ui.Icon(c, iconImage).Size(16, 16)
						ui.Text(c, locales.DlOpenImage()).SingleLine()
					}).Clicked() {
						go g.chooseQRFile()
					}
				})
				if err == nil {
					ui.Column(c).Padding(10, 14).Gap(4).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
						kv(c, locales.DlSmdp(), info.SMDP, nil)
						if info.MatchID != "" {
							kv(c, locales.DlMatchingId(), "•••• "+lastN(info.MatchID, 4), nil)
						}
					})
				}
			} else {
				ui.Field(c, locales.DlSmdpAddress(), func() {
					ui.TextInput(c, &d.smdp).Placeholder("smdp.example.com").Label(locales.DlSmdpAddress()).AutoFocus()
				})
				ui.Field(c, locales.DlMatchingId(), func() {
					ui.TextInput(c, &d.matchID).Placeholder(locales.CommonOptional()).Label(locales.DlMatchingId())
				})
			}
			if needCode || d.mode == 1 {
				ui.Field(c, locales.DlConfirmCode(), func() {
					placeholder := locales.CommonOptional()
					if needCode {
						placeholder = locales.DlConfirmRequired()
					}
					ui.TextInput(c, &d.confirm).Password().Placeholder(placeholder).Label(locales.DlConfirmCode())
				})
			}
			if g.demo && d.mode == 0 {
				g.demoCodes(c)
			}
			ui.Collapsible(c, locales.DlAdvanced(), &d.advanced, func() {
				// The label before the field, on its row.
				ui.Row(c).Gap(12).Children(func() {
					ui.Text(c, "IMEI").FontWeight(600).Shrink(0)
					ui.TextInput(c, &d.imei).Placeholder(locales.DlImeiPlaceholder()).Label("IMEI").Font("monospace").Grow(1)
				})
			})
		})
		if d.inputError != "" {
			ui.Text(c, d.inputError).FontSize(13).TextColor(t.Danger)
		} else if err != nil && err.Error() != "" {
			ui.Text(c, err.Error()).FontSize(13).TextColor(t.TextMuted)
		}
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if ui.Button(c, locales.CommonCancel()).Clicked() {
				d.open = false
			}
			ready := err == nil && (!needCode || strings.TrimSpace(d.confirm) != "")
			if ui.PrimaryButton(c, locales.DlDownload()).Disabled(!ready || g.app.Busy()).Clicked() {
				d.inputError, d.err, d.result = "", nil, nil
				d.smdpShown = info.SMDP
				if e := g.app.Download(info, g.downloadFinished); e != nil {
					d.inputError = e.Error()
					return
				}
				d.step = dlRunning
			}
		})
	})
}

func (g *gui) downloadFinished(res *app.DownloadResult) {
	d := &g.dl
	if res == nil {
		d.err = g.app.LastError
		if d.err == nil { // cancelled or declined
			d.step = dlInput
			return
		}
		d.step = dlFailed
		return
	}
	d.result = res
	d.step = dlDone
}

// dlStage is a step of SGP.22 profile download and installation as the
// download reports it: libeuicc's stage, the SGP.22 functions it calls,
// and what it does in words.
type dlStage struct {
	id    string
	fn    string
	label func() string
}

// downloadStages are the stages shown while downloading, in order.
var downloadStages = []dlStage{
	{"es10b_get_euicc_challenge_and_info", "ES10b.GetEUICCChallenge · ES10b.GetEUICCInfo", locales.StagePrepareCard},
	{"es9p_initiate_authentication", "ES9+.InitiateAuthentication", locales.StageContactServer},
	{"es10b_authenticate_server", "ES10b.AuthenticateServer", locales.StageCheckServer},
	{"es9p_authenticate_client", "ES9+.AuthenticateClient", locales.StageIdentifyCard},
	{"preview", "", locales.StageConfirm},
	{"es10b_prepare_download", "ES10b.PrepareDownload", locales.StagePrepareDownload},
	{"es9p_get_bound_profile_package", "ES9+.GetBoundProfilePackage", locales.StageDownload},
	{"es10b_load_bound_profile_package", "ES10b.LoadBoundProfilePackage", locales.StageInstall},
}

// notifyStage follows the download when notifications are sent.
var notifyStage = dlStage{"send_notification", "ES9+.HandleNotification", locales.StageNotify}

// cancelStage replaces the rest when a download is cancelled.
var cancelStage = dlStage{"cancel", "ES10b.CancelSession · ES9+.CancelSession", locales.StageCancel}

// stageLabel says a stage in words, for the task pill.
func stageLabel(s string) string {
	for _, st := range append(downloadStages, notifyStage, cancelStage) {
		if st.id == s {
			return st.label()
		}
	}
	return s
}

func (g *gui) downloadRunning(c *ui.Context) {
	t := c.Theme()
	task := g.app.Task
	if confirm := g.app.Confirm; confirm != nil {
		g.downloadPreview(c, confirm.Metadata)
		return
	}
	stages := downloadStages
	if g.app.Settings.AutoNotify {
		stages = append(slices.Clone(stages), notifyStage)
	}
	var marks []app.StageMark
	if task != nil {
		marks = task.Stages
	}
	// When each stage began; the last one reached is under way.
	began := map[string]time.Time{}
	at, cancelling := -1, false
	for _, m := range marks {
		began[m.Name] = m.At
		if m.Name == cancelStage.id {
			cancelling = true
		}
		for i, st := range stages {
			if st.id == m.Name {
				at = i
			}
		}
	}
	now := time.Now()
	if task != nil {
		c.After(100 * time.Millisecond) // the running stage's clock
	}

	ui.Column(c).Gap(4).Children(func() {
		ui.Text(c, locales.DlAdding()).FontSize(18).Bold()
		if g.dl.smdpShown != "" {
			ui.Text(c, "SM-DP+ "+g.dl.smdpShown).FontSize(12).Font("monospace").TextColor(t.TextMuted)
		}
	})
	ui.Progress(c, float64(at+1)/float64(len(stages)))
	ui.Column(c).Gap(2).Radius(12).Padding(6).Background(t.Surface).Border(1, t.Border).Children(func() {
		for i, st := range stages {
			if cancelling && i > at {
				break
			}
			g.stageRow(c, st, i, at, began, marks, now)
		}
		if cancelling {
			g.stageRow(c, cancelStage, at+1, at+1, began, marks, now)
		}
	})
	writing := at >= 5 // from preparing the download on, the card is being written
	ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
		if writing {
			ui.Text(c, locales.DlKeepConnected()).FontSize(12).TextColor(t.TextMuted).Grow(1)
		}
		if ui.Button(c, locales.CommonCancel()).Disabled(writing || cancelling || task == nil).Clicked() {
			task.Cancel()
		}
	})
}

// stageRow is one step: done, under way or to come, with the SGP.22
// functions it calls and the time it took.
func (g *gui) stageRow(c *ui.Context, st dlStage, i, at int, began map[string]time.Time, marks []app.StageMark, now time.Time) {
	t := c.Theme()
	ui.Row(c.Key(st.id)).Gap(10).Padding(6, 8).Radius(8).AlignItems(ui.Start).Children(func() {
		ui.Box(c).Size(16, 16).Margin(2, 0, 0).Center().Children(func() {
			switch {
			case i < at:
				ui.Icon(c, iconCheck).Size(16, 16).TextColor(t.Success)
			case i == at:
				ui.Spinner(c).Size(14, 14)
			}
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
			label := ui.Text(c, st.label())
			switch {
			case i == at:
				label.FontWeight(600)
			case i > at:
				label.TextColor(t.TextMuted)
			}
			if st.fn != "" {
				ui.Text(c, st.fn).FontSize(11).Font("monospace").TextColor(t.TextMuted).Tooltip(st.id)
			}
		})
		if start, ok := began[st.id]; ok && i <= at {
			end := now
			if i < at {
				end = stageEnd(marks, st.id, now)
			}
			ui.Text(c, formatStageTime(end.Sub(start))).FontSize(11).Font("monospace").TextColor(t.TextMuted).Shrink(0)
		}
	})
}

// stageEnd is when the stage after name began.
func stageEnd(marks []app.StageMark, name string, now time.Time) time.Time {
	for i, m := range marks {
		if m.Name == name && i+1 < len(marks) {
			return marks[i+1].At
		}
	}
	return now
}

func formatStageTime(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%d ms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1f s", d.Seconds())
}

func (g *gui) downloadPreview(c *ui.Context, meta lpa.ProfileMetadata) {
	t := c.Theme()
	ui.Text(c, locales.DlPreviewTitle()).FontSize(18).Bold()
	ui.Row(c).Padding(14, 16).Gap(14).Radius(14).Background(t.Surface).Border(1, t.Border).Children(func() {
		name := orText(meta.ProfileName, "eSIM")
		g.profileIcon(c, "preview:"+meta.ICCID, meta.Icon, name, 48)
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, name).FontSize(16).Bold()
			if meta.ServiceProviderName != "" {
				ui.Text(c, meta.ServiceProviderName).TextColor(t.TextMuted)
			}
			ui.Text(c, locales.ProfileIccid(g.identifier(meta.ICCID))).FontSize(12).Font("monospace").TextColor(t.TextMuted)
		})
		if meta.ProfileClass != "" && meta.ProfileClass != "operational" {
			ui.Badge(c, meta.ProfileClass)
		}
	})
	ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
		if ui.Button(c, locales.CommonCancel()).Clicked() {
			g.app.AnswerConfirm(false)
		}
		if ui.PrimaryButton(c, locales.DlInstall()).AutoFocus().Clicked() {
			g.dl.installed = meta
			g.app.AnswerConfirm(true)
		}
	})
}

func (g *gui) downloadDone(c *ui.Context) {
	t := c.Theme()
	d := &g.dl
	ui.Column(c).Gap(10).AlignItems(ui.Center).Padding(8, 0).Children(func() {
		ui.Box(c).Size(56, 56).Radius(28).Center().Material(glass.Glass{Tint: t.Success.Alpha(0.35)}).Children(func() {
			ui.Icon(c, iconCheck).Size(28, 28).TextColor(t.Success)
		})
		ui.Text(c, locales.DlDoneTitle()).FontSize(18).Bold()
		doneBody := locales.DlDoneBodyUnnamed()
		if d.installed.ProfileName != "" {
			doneBody = locales.DlDoneBody(d.installed.ProfileName)
		}
		ui.Text(c, doneBody).TextColor(t.TextMuted).TextAlign(ui.Center)
		switch {
		case d.result.NotificationError != nil:
			ui.Text(c, locales.DlNotifyFailed()).FontSize(12).TextColor(t.Warning).TextAlign(ui.Center)
		case d.result.NotificationSent:
			ui.Text(c, locales.DlNotified()).FontSize(12).TextColor(t.TextMuted)
		case !g.app.Settings.AutoNotify:
			ui.Text(c, locales.DlNotifyWaiting()).FontSize(12).TextColor(t.TextMuted)
		}
	})
	ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
		if ui.Button(c, locales.CommonDone()).Clicked() {
			d.open = false
		}
		iccid := d.result.ICCID
		if ui.PrimaryButton(c, locales.DlTurnOnNow()).Disabled(iccid == "" || g.app.Busy()).AutoFocus().Clicked() {
			_ = g.app.SetProfileEnabled(iccid, true)
			d.open = false
		}
	})
}

func (g *gui) downloadFailed(c *ui.Context) {
	t := c.Theme()
	d := &g.dl
	ui.Row(c).Gap(10).Children(func() {
		ui.Icon(c, iconAlert).Size(22, 22).TextColor(t.Danger)
		ui.Text(c, locales.DlFailedTitle()).FontSize(18).Bold()
	})
	if summary, codes, ok := app.ServerError(d.err); ok {
		ui.Text(c, locales.DlServerSays(summary)).Selectable()
		ui.Text(c, locales.DlErrorCode(codes)).FontSize(12).Font("monospace").TextColor(t.TextMuted).Selectable()
	} else {
		msg := locales.DlUnknownError()
		if d.err != nil {
			msg = d.err.Error()
		}
		ui.Text(c, msg).Selectable().TextColor(t.TextMuted)
	}
	ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
		if ui.Button(c, locales.CommonClose()).Clicked() {
			d.open = false
		}
		if ui.PrimaryButton(c, locales.DlRetry()).Clicked() {
			d.step = dlInput
		}
	})
}

// pasteActivation takes an activation code from the clipboard: as text,
// or as an image of the QR code.
func (g *gui) pasteActivation(c *ui.Context) {
	d := &g.dl
	d.inputError = ""
	if s := strings.TrimSpace(g.clip.ReadText()); s != "" {
		d.code = activation.Complete(s)
		return
	}
	if img := g.clip.ReadImage(); img != nil {
		text, err := activation.ScanQRCodeBytes(img)
		if err != nil {
			d.inputError = locales.DlNoQrClipboard()
			return
		}
		d.code = text
		return
	}
	d.inputError = locales.DlClipboardEmpty()
}

func (g *gui) chooseQRFile() {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:  g.win,
		Title:   locales.DlChooseImageTitle(),
		Filters: []mygo.FileFilter{{Name: locales.DlImages(), Extensions: []string{"png", "jpg", "jpeg"}}},
	})
	if err != nil || len(paths) == 0 {
		return
	}
	g.win.Update(func() { g.scanQRFile(paths[0]) })
}

func (g *gui) scanQRFile(path string) {
	d := &g.dl
	d.inputError = ""
	f, err := os.Open(path)
	if err != nil {
		d.inputError = err.Error()
		return
	}
	defer f.Close()
	text, err := activation.ScanQRCode(f)
	if err != nil {
		d.inputError = locales.DlNoQrImage()
		return
	}
	d.mode, d.code = 0, text
}

func lastN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
