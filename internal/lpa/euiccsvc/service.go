// Package euiccsvc implements lpa.Service on top of the in-process
// libeuicc binding and a card transport backend. It replaces the lpac
// CLI adapter per docs/refactor-agent-plan.md.
//
// P3 scope: read paths (chip info, profiles, notifications) are fully
// implemented; write operations are delivered by P5 and return
// ErrWriteNotImplemented until then. Sessions are opened per
// operation and serialized by the caller (single-task scheduling).
package euiccsvc

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	neturl "net/url"
	"regexp"
	"time"

	"EasyLPAC/internal/euicc"
	"EasyLPAC/internal/httptransport"
	"EasyLPAC/internal/lpa"
	"EasyLPAC/internal/transport"
)

// ErrWriteNotImplemented marks write operations that P5 delivers.
// The UI keeps using the legacy backend for writes until then.
var ErrWriteNotImplemented = errors.New("libeuicc backend: write operations are implemented in P5")

// Config binds one service instance to a backend and device.
type Config struct {
	Backend transport.Backend
	Device  transport.DeviceConfig
	// AID is the ISD-R application to select.
	AID []byte
	// DebugAPDU / DebugHTTP enable protocol-level logging: APDUs and
	// HTTP bodies at debug level to Logger, or libeuicc's own output to
	// stderr when Logger is nil.
	DebugAPDU bool
	DebugHTTP bool
	// Logger receives protocol logs (nil = none).
	Logger *slog.Logger
	// OnStage reports download stage transitions (nil = no reporting).
	OnStage func(stage string)
	// HTTPClient overrides the ES9+ HTTP client (nil = default with
	// environment proxy and no TLS certificate verification).
	HTTPClient *http.Client
}

// Service implements lpa.Service.
type Service struct {
	cfg Config
}

func New(cfg Config) (*Service, error) {
	if cfg.Backend == nil {
		return nil, errors.New("euiccsvc: no backend configured")
	}
	if len(cfg.AID) == 0 {
		return nil, errors.New("euiccsvc: no AID configured")
	}
	return &Service{cfg: cfg}, nil
}

// withContext opens a card session and a libeuicc context for one
// operation, then releases both. Errors from fn are returned as-is.
func (s *Service) withContext(ctx context.Context, fn func(*euicc.Context) error) error {
	session, err := s.cfg.Backend.Open(ctx, s.cfg.Device)
	if err != nil {
		return &lpa.OpError{Stage: lpa.StageConnect, BackendID: s.cfg.Backend.ID(),
			Op: "open", Cause: err, StateKnown: true}
	}

	adapter := &sessionAPDU{session: session}
	var httpIface euicc.HTTPInterface = httptransport.New(ctx, httptransport.Options{Client: s.cfg.HTTPClient})
	if l := s.cfg.Logger; l != nil {
		if s.cfg.DebugAPDU {
			adapter.log = l
		}
		httpIface = &loggedHTTP{inner: httpIface, log: l, bodies: s.cfg.DebugHTTP}
	}
	c, err := euicc.New(adapter, httpIface, s.cfg.AID)
	if err != nil {
		_ = session.Close() // Open returned a connected session; never leak it
		return &lpa.OpError{Stage: lpa.StageConnect, BackendID: s.cfg.Backend.ID(),
			Op: "euicc_init", Cause: err, StateKnown: true}
	}
	if s.cfg.Logger == nil {
		c.SetDebug(s.cfg.DebugAPDU, s.cfg.DebugHTTP)
	}
	defer c.Close() // euicc_fini -> disconnect callback -> session.Close
	return fn(c)
}

// wrapOp annotates err with stage/op context unless it already carries
// structured context from an inner layer.
func (s *Service) wrapOp(stage lpa.Stage, op string, err error) error {
	if err == nil {
		return nil
	}
	var opErr *lpa.OpError
	if errors.As(err, &opErr) {
		return err
	}
	return &lpa.OpError{Stage: stage, BackendID: s.cfg.Backend.ID(), Op: op, Cause: err}
}

// sessionAPDU adapts transport.CardSession to euicc.APDUInterface.
// libeuicc calls Connect once at init (the session is already
// connected) and Disconnect on teardown (which releases the session).
type sessionAPDU struct {
	session transport.CardSession
	// log receives every APDU exchange when set.
	log *slog.Logger
}

func (a *sessionAPDU) Connect() error { return nil }

func (a *sessionAPDU) Disconnect() { _ = a.session.Close() }

func (a *sessionAPDU) LogicChannelOpen(aid []byte) (uint8, error) {
	ch, err := a.session.OpenLogicalChannel(context.Background(), aid)
	if a.log != nil {
		a.log.Debug("apdu open channel", "aid", hex.EncodeToString(aid), "channel", ch, "err", err)
	}
	return ch, err
}

func (a *sessionAPDU) LogicChannelClose(channel uint8) {
	_ = a.session.CloseLogicalChannel(context.Background(), channel)
}

func (a *sessionAPDU) Transmit(apdu []byte) ([]byte, error) {
	resp, err := a.session.Transmit(context.Background(), apdu)
	if a.log != nil {
		a.log.Debug("apdu", "tx", hexCap(apdu, 512), "rx", hexCap(resp, 512), "err", err)
	}
	return resp, err
}

func hexCap(b []byte, max int) string {
	if len(b) > max {
		return hex.EncodeToString(b[:max]) + fmt.Sprintf("…(+%d bytes)", len(b)-max)
	}
	return hex.EncodeToString(b)
}

// loggedHTTP logs each ES9+ exchange: endpoint, status and duration at
// info level, and the bodies at debug level when bodies is set.
type loggedHTTP struct {
	inner  euicc.HTTPInterface
	log    *slog.Logger
	bodies bool
}

// matchingIDField hides the matching ID in logged request bodies.
var matchingIDField = regexp.MustCompile(`("matchingId"\s*:\s*")[^"]*(")`)

func (h *loggedHTTP) Transmit(url string, headers []string, tx []byte) (uint32, []byte, error) {
	start := time.Now()
	code, rx, err := h.inner.Transmit(url, headers, tx)
	endpoint := url
	if u, perr := neturl.Parse(url); perr == nil {
		endpoint = u.Host + u.Path
	}
	h.log.Info("http", "endpoint", endpoint, "status", code, "elapsed", time.Since(start).Round(time.Millisecond),
		"sent", len(tx), "received", len(rx), "err", err)
	if h.bodies {
		h.log.Debug("http request", "endpoint", endpoint, "body", textCap(matchingIDField.ReplaceAllString(string(tx), "${1}(redacted)${2}"), 2048))
		h.log.Debug("http response", "endpoint", endpoint, "body", textCap(string(rx), 2048))
	}
	return code, rx, err
}

func textCap(s string, max int) string {
	if len(s) > max {
		return s[:max] + fmt.Sprintf("…(+%d bytes)", len(s)-max)
	}
	return s
}

func (s *Service) Backends(_ context.Context) ([]string, error) {
	if !s.cfg.Backend.Available() {
		return nil, nil
	}
	return []string{s.cfg.Backend.ID()}, nil
}

func (s *Service) Devices(ctx context.Context, backend string) ([]lpa.Device, error) {
	if backend != s.cfg.Backend.ID() {
		return nil, fmt.Errorf("euiccsvc: unknown backend %q", backend)
	}
	devices, err := s.cfg.Backend.Discover(ctx)
	if err != nil {
		return nil, &lpa.OpError{Stage: lpa.StageDiscovery, BackendID: backend, Op: "discover", Cause: err}
	}
	out := make([]lpa.Device, 0, len(devices))
	for _, d := range devices {
		out = append(out, lpa.Device{Env: d.ID, Name: d.Label})
	}
	return out, nil
}

// ChipInfo composes EID (fatal on failure), configured addresses and
// EUICCInfo2 (tolerated failures), matching the legacy CLI behavior.
func (s *Service) ChipInfo(ctx context.Context) (*lpa.EuiccInfo, error) {
	var info lpa.EuiccInfo
	err := s.withContext(ctx, func(c *euicc.Context) error {
		eid, err := c.GetEID()
		if err != nil {
			return err
		}
		info.EidValue = eid

		if addr, err := c.GetConfiguredAddresses(); err == nil {
			if addr.DefaultDpAddress != "" {
				info.EuiccConfiguredAddresses.DefaultDpAddress = addr.DefaultDpAddress
			}
			info.EuiccConfiguredAddresses.RootDsAddress = addr.RootDsAddress
		}
		if info2, err := c.GetEUICCInfo2(); err == nil {
			info.EUICCInfo2 = toDomainInfo2(info2)
		}
		return nil
	})
	if err != nil {
		return nil, s.wrapOp(lpa.StageChipInfo, "chip-info", err)
	}
	return &info, nil
}

func (s *Service) ProfileList(ctx context.Context) ([]*lpa.Profile, error) {
	var profiles []*lpa.Profile
	err := s.withContext(ctx, func(c *euicc.Context) error {
		raw, err := c.GetProfiles()
		if err != nil {
			return err
		}
		for i := range raw {
			profiles = append(profiles, &lpa.Profile{
				Iccid:               raw[i].ICCID,
				IsdpAid:             raw[i].ISDPAID,
				ProfileState:        stateToString(raw[i].State),
				ProfileNickname:     nicknamePtr(raw[i]),
				ServiceProviderName: raw[i].ServiceProviderName,
				ProfileName:         raw[i].ProfileName,
				IconType:            iconTypeToString(raw[i].IconType),
				Icon:                raw[i].Icon,
				ProfileClass:        classToString(raw[i].Class),
			})
		}
		return nil
	})
	if err != nil {
		return nil, s.wrapOp(lpa.StageProfileList, "profile-list", err)
	}
	return profiles, nil
}

func (s *Service) NotificationList(ctx context.Context) ([]*lpa.Notification, error) {
	var notifications []*lpa.Notification
	err := s.withContext(ctx, func(c *euicc.Context) error {
		raw, err := c.GetNotifications()
		if err != nil {
			return err
		}
		for i := range raw {
			notifications = append(notifications, &lpa.Notification{
				SeqNumber:                  int(raw[i].SeqNumber),
				ProfileManagementOperation: operationToString(raw[i].ProfileManagementOperation),
				NotificationAddress:        raw[i].NotificationAddress,
				Iccid:                      raw[i].ICCID,
			})
		}
		return nil
	})
	if err != nil {
		return nil, s.wrapOp(lpa.StageNotifyList, "notification-list", err)
	}
	return notifications, nil
}

// Version reports the vendored protocol library version.
func (s *Service) Version(_ context.Context) (string, error) {
	return euicc.LpacShortCommit, nil
}

// --- mapping helpers ------------------------------------------------

func stateToString(state int32) string {
	switch state {
	case euicc.ProfileStateEnabled:
		return "enabled"
	case euicc.ProfileStateDisabled:
		return "disabled"
	default:
		return "undefined"
	}
}

func classToString(class int32) string {
	switch class {
	case euicc.ProfileClassTest:
		return "test"
	case euicc.ProfileClassProvisioning:
		return "provisioning"
	case euicc.ProfileClassOperational:
		return "operational"
	default:
		return "undefined"
	}
}

func iconTypeToString(iconType int32) string {
	switch iconType {
	case euicc.IconTypeJpeg:
		return "jpeg"
	case euicc.IconTypePng:
		return "png"
	default:
		return "undefined"
	}
}

func operationToString(op int32) string {
	switch op {
	case euicc.NotificationOpInstall:
		return "install"
	case euicc.NotificationOpEnable:
		return "enable"
	case euicc.NotificationOpDisable:
		return "disable"
	case euicc.NotificationOpDelete:
		return "delete"
	default:
		return "undefined"
	}
}

func nicknamePtr(p euicc.Profile) *string {
	if !p.HasNickname {
		return nil
	}
	n := p.Nickname
	return &n
}

func toDomainInfo2(info *euicc.EUICCInfo2) lpa.EUICCInfo2 {
	var out lpa.EUICCInfo2
	out.ProfileVersion = info.ProfileVersion
	out.Svn = info.Svn
	out.EuiccFirmwareVer = info.EuiccFirmwareVer
	out.ExtCardResource.InstalledApplication = int(info.ExtCardResource.InstalledApplication)
	out.ExtCardResource.FreeNonVolatileMemory = int(info.ExtCardResource.FreeNonVolatileMemory)
	out.ExtCardResource.FreeVolatileMemory = int(info.ExtCardResource.FreeVolatileMemory)
	out.UiccCapability = info.UiccCapability
	out.Ts102241Version = info.Ts102241Version
	out.GlobalplatformVersion = info.GlobalplatformVer
	out.RspCapability = info.RspCapability
	out.EuiccCiPKIDListForVerification = info.CiPKIDForVerify
	out.EuiccCiPKIDListForSigning = info.CiPKIDForSigning
	out.EuiccCategory = info.EuiccCategory
	out.ForbiddenProfilePolicyRules = info.ForbiddenPPR
	out.PpVersion = info.PpVersion
	out.SasAcreditationNumber = info.SasAcreditation
	out.CertificationDataObject.PlatformLabel = info.PlatformLabel
	out.CertificationDataObject.DiscoveryBaseURL = info.DiscoveryBaseURL
	return out
}
