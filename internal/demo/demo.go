// Package demo is a simulated eUICC and SM-DP+ for trying the app
// without a card: an lpa.Service over in-memory state, with delays like a
// real card's and failures on request. Nothing it does reaches a card or
// a server.
//
// Downloads take any activation code; words in the matching ID choose
// what happens (see Scenarios).
package demo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/big"
	"strings"
	"sync"
	"time"

	"EasyLPAC/internal/lpa"
	"EasyLPAC/internal/transport"
)

// ReaderName is the simulated device.
const ReaderName = "Demo eUICC (simulated)"

// Scenario is a download outcome chosen by a word in the matching ID.
type Scenario struct {
	Code        string // an activation code that triggers it
	Description string
}

// Scenarios lists the simulated download outcomes.
var Scenarios = []Scenario{
	{"LPA:1$smdp.demo.example$DEMO-OK", "Installs an eSIM"},
	{"LPA:1$smdp.demo.example$DEMO-CONFIRM$$1", "Needs confirmation code 1234"},
	{"LPA:1$smdp.demo.example$DEMO-REFUSED", "The server refuses the matching ID (8.2.6 / 3.8)"},
	{"LPA:1$smdp.demo.example$DEMO-EXPIRED", "The download order has expired (8.8.5 / 4.1)"},
	{"LPA:1$smdp.demo.example$DEMO-FULL", "The card runs out of space while installing"},
	{"LPA:1$smdp.demo.example$DEMO-SLOW", "Takes its time, to try cancelling"},
}

// Card is the simulated eUICC; it outlives the services the app makes.
type Card struct {
	mu            sync.Mutex
	chip          lpa.EuiccInfo
	profiles      []*lpa.Profile
	notifications []*lpa.Notification
	seq           int
	installed     int
	// Speed scales the delays (1 is like a real card; 0 skips them).
	Speed float64
}

// New returns a card with three profiles and two pending notifications.
func New() *Card {
	c := &Card{Speed: 1}
	c.chip.EidValue = withCheckDigits("890490320000010000003456789012")
	c.chip.EuiccConfiguredAddresses.RootDsAddress = "lpa.ds.gsma.com"
	i := &c.chip.EUICCInfo2
	i.ProfileVersion, i.Svn, i.EuiccFirmwareVer = "2.3.1", "2.2.2", "4.6.0"
	i.ExtCardResource.InstalledApplication = 3
	i.ExtCardResource.FreeNonVolatileMemory = 412000
	i.ExtCardResource.FreeVolatileMemory = 8480
	i.UiccCapability = []string{"contactlessSupport", "usimSupport", "isimSupport", "csimSupport", "akaMilenage", "akaTuak128", "gbaAuthenUsim", "eapClient", "javacard", "multipleUsimSupport"}
	i.Ts102241Version, i.GlobalplatformVersion = "9.2.0", "2.3.0"
	i.RspCapability = []string{"additionalProfile", "testProfileSupport"}
	i.EuiccCiPKIDListForVerification = []string{"81370f5125d0b1d408d4c3b232e6d25e795bebfb", "c0bc70ba36929d43b467ff57570530e57ab8fcd8"}
	i.EuiccCiPKIDListForSigning = []string{"81370f5125d0b1d408d4c3b232e6d25e795bebfb"}
	i.EuiccCategory = "other"
	i.PpVersion, i.SasAcreditationNumber = "1.0.0", "GD-MH-UP-0001"
	i.CertificationDataObject.PlatformLabel = "Demo eUICC"
	i.CertificationDataObject.DiscoveryBaseURL = "https://lpa.ds.gsma.com"

	c.profiles = []*lpa.Profile{
		{Iccid: "8985203000000000017", IsdpAid: "a0000005591010ffffffff8900001100", ProfileState: "enabled",
			ProfileName: "Travel 10 GB", ServiceProviderName: "Example Mobile", ProfileClass: "operational",
			IconType: "png", Icon: iconPNG(color.RGBA{0x25, 0x63, 0xeb, 0xff})},
		{Iccid: "8944110000000000025", IsdpAid: "a0000005591010ffffffff8900001200", ProfileState: "disabled",
			ProfileName: "Home", ServiceProviderName: "Carrier Two", ProfileClass: "operational", ProfileNickname: ptr("Work line")},
		{Iccid: "8900100000000000033", IsdpAid: "a0000005591010ffffffff8900001300", ProfileState: "disabled",
			ProfileName: "GSMA Test Profile", ServiceProviderName: "GSMA", ProfileClass: "test",
			IconType: "png", Icon: iconPNG(color.RGBA{0x16, 0xa3, 0x4a, 0xff})},
	}
	// More of them, so that the list scrolls.
	extra := []struct {
		name, provider string
		nick           string
		col            color.RGBA
	}{
		{"Japan 5 GB", "Sakura Telecom", "", color.RGBA{0xdc, 0x26, 0x26, 0xff}},
		{"Europe Unlimited", "EuroRoam", "Holiday", color.RGBA{0x7c, 0x3a, 0xed, 0xff}},
		{"US Prepaid", "Liberty Wireless", "", color.RGBA{0x0e, 0xa5, 0xe9, 0xff}},
		{"IoT Data", "Machine Net", "Tracker", color.RGBA{0x64, 0x74, 0x8b, 0xff}},
		{"Hong Kong 30 days", "Harbour Mobile", "", color.RGBA{0xea, 0x58, 0x0c, 0xff}},
		{"Backup line", "Carrier Two", "", color.RGBA{0x05, 0x96, 0x69, 0xff}},
		{"Korea Travel", "Han River Mobile", "", color.RGBA{0xdb, 0x27, 0x77, 0xff}},
	}
	for i, e := range extra {
		p := &lpa.Profile{
			Iccid:        fmt.Sprintf("89%02d00000000000%04d", 10+i*7, 41+i*8),
			IsdpAid:      fmt.Sprintf("a0000005591010ffffffff89000014%02x", i),
			ProfileState: "disabled", ProfileName: e.name, ServiceProviderName: e.provider,
			ProfileClass: "operational", IconType: "png", Icon: iconPNG(e.col),
		}
		if e.nick != "" {
			p.ProfileNickname = ptr(e.nick)
		}
		c.profiles = append(c.profiles, p)
	}
	c.chip.EUICCInfo2.ExtCardResource.InstalledApplication += len(extra)
	c.addNotification("install", "8944110000000000025", "rsp.carrier-two.example")
	c.addNotification("delete", "8981100000000000041", "unreachable.invalid")
	return c
}

func ptr(s string) *string { return &s }

// withCheckDigits appends the SGP.29 check digits to 30 digits.
func withCheckDigits(base string) string {
	n, _ := new(big.Int).SetString(base+"00", 10)
	cd := 98 - new(big.Int).Mod(n, big.NewInt(97)).Int64()
	return fmt.Sprintf("%s%02d", base, cd)
}

// iconPNG draws a profile icon: a disc of col with a lighter ring.
func iconPNG(col color.RGBA) []byte {
	const size = 64
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			d := math.Hypot(float64(x)-31.5, float64(y)-31.5)
			switch {
			case d < 18:
				img.Set(x, y, color.RGBA{0xff, 0xff, 0xff, 0xff})
			case d < 31:
				img.Set(x, y, col)
			}
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}

func (c *Card) addNotification(op, iccid, addr string) *lpa.Notification {
	c.seq++
	n := &lpa.Notification{SeqNumber: c.seq, ProfileManagementOperation: op, Iccid: iccid, NotificationAddress: addr}
	c.notifications = append(c.notifications, n)
	return n
}

// Discover lists the simulated device.
func Discover(context.Context, string) ([]transport.Device, error) {
	return []transport.Device{{ID: ReaderName, Label: ReaderName}}, nil
}

// Service returns an lpa.Service over the card, reporting download
// stages to onStage.
func (c *Card) Service(onStage func(string)) lpa.Service {
	return &service{card: c, onStage: onStage}
}

type service struct {
	card    *Card
	onStage func(string)
}

// wait sleeps d scaled by the card's speed, or until ctx ends.
func (s *service) wait(ctx context.Context, d time.Duration) error {
	d = time.Duration(float64(d) * s.card.Speed)
	if d <= 0 {
		return ctx.Err()
	}
	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *service) stage(name string) {
	if s.onStage != nil {
		s.onStage(name)
	}
}

func (s *service) Backends(context.Context) ([]string, error) { return []string{"demo"}, nil }
func (s *service) Devices(ctx context.Context, _ string) ([]lpa.Device, error) {
	return []lpa.Device{{Env: ReaderName, Name: ReaderName}}, nil
}
func (s *service) Version(context.Context) (string, error) { return "demo", nil }

func (s *service) ChipInfo(ctx context.Context) (*lpa.EuiccInfo, error) {
	if err := s.wait(ctx, 250*time.Millisecond); err != nil {
		return nil, err
	}
	c := s.card
	c.mu.Lock()
	defer c.mu.Unlock()
	info := c.chip
	return &info, nil
}

func (s *service) ProfileList(ctx context.Context) ([]*lpa.Profile, error) {
	if err := s.wait(ctx, 200*time.Millisecond); err != nil {
		return nil, err
	}
	c := s.card
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*lpa.Profile, len(c.profiles))
	for i, p := range c.profiles {
		cp := *p
		out[i] = &cp
	}
	return out, nil
}

func (s *service) NotificationList(ctx context.Context) ([]*lpa.Notification, error) {
	if err := s.wait(ctx, 150*time.Millisecond); err != nil {
		return nil, err
	}
	c := s.card
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*lpa.Notification, len(c.notifications))
	for i, n := range c.notifications {
		cp := *n
		out[i] = &cp
	}
	return out, nil
}

func (c *Card) find(iccid string) *lpa.Profile {
	for _, p := range c.profiles {
		if p.Iccid == iccid {
			return p
		}
	}
	return nil
}

func opErr(stage lpa.Stage, op, msg string) error {
	return &lpa.OpError{Stage: stage, BackendID: "demo", Op: op, Cause: errors.New(msg), StateKnown: true}
}

func (s *service) ProfileEnable(ctx context.Context, iccid string) error {
	if err := s.wait(ctx, 700*time.Millisecond); err != nil {
		return err
	}
	c := s.card
	c.mu.Lock()
	defer c.mu.Unlock()
	p := c.find(iccid)
	switch {
	case p == nil:
		return opErr(lpa.StageProfileEnable, "enable", "iccidOrAidNotFound")
	case p.ProfileState == "enabled":
		return opErr(lpa.StageProfileEnable, "enable", "profileNotInDisabledState")
	}
	for _, o := range c.profiles {
		if o.ProfileState == "enabled" {
			o.ProfileState = "disabled"
			c.addNotification("disable", o.Iccid, addressFor(o))
		}
	}
	p.ProfileState = "enabled"
	c.addNotification("enable", p.Iccid, addressFor(p))
	return nil
}

func (s *service) ProfileDisable(ctx context.Context, iccid string) error {
	if err := s.wait(ctx, 600*time.Millisecond); err != nil {
		return err
	}
	c := s.card
	c.mu.Lock()
	defer c.mu.Unlock()
	p := c.find(iccid)
	switch {
	case p == nil:
		return opErr(lpa.StageProfileDisable, "disable", "iccidOrAidNotFound")
	case p.ProfileState != "enabled":
		return opErr(lpa.StageProfileDisable, "disable", "profileNotInEnabledState")
	}
	p.ProfileState = "disabled"
	c.addNotification("disable", p.Iccid, addressFor(p))
	return nil
}

func (s *service) ProfileDelete(ctx context.Context, iccid string) error {
	if err := s.wait(ctx, 900*time.Millisecond); err != nil {
		return err
	}
	c := s.card
	c.mu.Lock()
	defer c.mu.Unlock()
	p := c.find(iccid)
	switch {
	case p == nil:
		return opErr(lpa.StageProfileDelete, "delete", "iccidOrAidNotFound")
	case p.ProfileState == "enabled":
		return opErr(lpa.StageProfileDelete, "delete", "profileNotInDisabledState")
	}
	for i, o := range c.profiles {
		if o == p {
			c.profiles = append(c.profiles[:i], c.profiles[i+1:]...)
		}
	}
	c.chip.EUICCInfo2.ExtCardResource.FreeNonVolatileMemory += 60000
	c.chip.EUICCInfo2.ExtCardResource.InstalledApplication--
	c.addNotification("delete", p.Iccid, addressFor(p))
	return nil
}

func (s *service) ProfileNickname(ctx context.Context, iccid, nickname string) error {
	if err := s.wait(ctx, 300*time.Millisecond); err != nil {
		return err
	}
	c := s.card
	c.mu.Lock()
	defer c.mu.Unlock()
	p := c.find(iccid)
	if p == nil {
		return opErr(lpa.StageProfileNickname, "nickname", "iccidNotFound")
	}
	if nickname == "" {
		p.ProfileNickname = nil
	} else {
		p.ProfileNickname = ptr(nickname)
	}
	return nil
}

func (s *service) ChipDefaultSmdp(ctx context.Context, addr string) error {
	if err := s.wait(ctx, 300*time.Millisecond); err != nil {
		return err
	}
	c := s.card
	c.mu.Lock()
	defer c.mu.Unlock()
	if addr == "" {
		c.chip.EuiccConfiguredAddresses.DefaultDpAddress = nil
	} else {
		c.chip.EuiccConfiguredAddresses.DefaultDpAddress = addr
	}
	return nil
}

func (s *service) NotificationProcess(ctx context.Context, seq int, remove bool) error {
	c := s.card
	c.mu.Lock()
	var n *lpa.Notification
	for _, x := range c.notifications {
		if x.SeqNumber == seq {
			n = x
		}
	}
	c.mu.Unlock()
	if n == nil {
		return opErr(lpa.StageNotifyProcess, "process", fmt.Sprintf("notification %d not found", seq))
	}
	if err := s.wait(ctx, 800*time.Millisecond); err != nil {
		return err
	}
	if strings.HasSuffix(n.NotificationAddress, ".invalid") {
		return opErr(lpa.StageNotifyProcess, "send", "dial tcp: lookup "+n.NotificationAddress+": no such host")
	}
	if remove {
		return s.NotificationRemove(ctx, seq)
	}
	return nil
}

func (s *service) NotificationRemove(ctx context.Context, seq int) error {
	if err := s.wait(ctx, 200*time.Millisecond); err != nil {
		return err
	}
	c := s.card
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, n := range c.notifications {
		if n.SeqNumber == seq {
			c.notifications = append(c.notifications[:i], c.notifications[i+1:]...)
			return nil
		}
	}
	return opErr(lpa.StageNotifyRemove, "remove", fmt.Sprintf("notification %d not found", seq))
}

func addressFor(p *lpa.Profile) string {
	switch p.ServiceProviderName {
	case "Example Mobile":
		return "rsp.example-mobile.example"
	case "Carrier Two":
		return "rsp.carrier-two.example"
	}
	return "smdp.demo.example"
}

// The download stages, as the real service reports them.
var stages = []string{
	"es10b_get_euicc_challenge_and_info",
	"es9p_initiate_authentication",
	"es10b_authenticate_server",
	"es9p_authenticate_client",
	"preview",
	"es10b_prepare_download",
	"es9p_get_bound_profile_package",
	"es10b_load_bound_profile_package",
}

func (s *service) ProfileDownload(ctx context.Context, req lpa.DownloadRequest) (*lpa.DownloadOutcome, error) {
	fail := func(stage string, err error) (*lpa.DownloadOutcome, error) {
		s.stage("cancel")
		return nil, &lpa.OpError{Stage: lpa.StageDownload, BackendID: "demo", Op: "download", Cause: err, StateKnown: true}
	}
	es9 := func(stage, subject, reason, msg string) (*lpa.DownloadOutcome, error) {
		return fail(stage, &lpa.ES9Error{Stage: stage, Subject: subject, Reason: reason, Message: msg})
	}
	if strings.TrimSpace(req.SMDP) == "" {
		return fail("", errors.New("no SM-DP+ address"))
	}
	scenario := strings.ToUpper(req.MatchID)
	step := 450 * time.Millisecond
	if strings.Contains(scenario, "SLOW") {
		step = 2500 * time.Millisecond
	}
	c := s.card
	c.mu.Lock()
	c.installed++
	n := c.installed
	c.mu.Unlock()
	meta := lpa.ProfileMetadata{
		ICCID:               fmt.Sprintf("89860%014d", 1000+n*7),
		ServiceProviderName: "Demo Carrier",
		ProfileName:         fmt.Sprintf("Demo Plan %d", n),
		ProfileClass:        "operational",
		IconType:            "png",
		Icon:                iconPNG(color.RGBA{0xdb, 0x27, 0x77, 0xff}),
	}

	for _, st := range stages {
		if err := ctx.Err(); err != nil {
			return fail(st, err)
		}
		s.stage(st)
		switch st {
		case "preview":
			if req.Confirm != nil {
				ok, err := req.Confirm(meta)
				if err != nil {
					return fail(st, err)
				}
				if !ok {
					return fail(st, lpa.ErrUserCancelled)
				}
			}
			continue
		}
		if err := s.wait(ctx, step); err != nil {
			return fail(st, err)
		}
		switch {
		case st == "es9p_initiate_authentication" && strings.Contains(scenario, "REFUSED"):
			return es9(st, "8.2.6", "3.8", "MatchingID refused")
		case st == "es9p_authenticate_client" && strings.Contains(scenario, "EXPIRED"):
			return es9(st, "8.8.5", "4.1", "Download order expired")
		case st == "es9p_authenticate_client" && strings.Contains(scenario, "CONFIRM") && req.ConfirmCode == "":
			return es9(st, "8.2.7", "2.2", "Confirmation code missing")
		case st == "es9p_get_bound_profile_package" && strings.Contains(scenario, "CONFIRM") && req.ConfirmCode != "1234":
			return es9(st, "8.2.7", "3.8", "Confirmation code refused")
		case st == "es10b_load_bound_profile_package" && strings.Contains(scenario, "FULL"):
			return fail(st, errors.New("load bound profile package: installFailedDueToInsufficientMemoryForProfile (bppCommandId=3, errorReason=19)"))
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.profiles = append(c.profiles, &lpa.Profile{
		Iccid: meta.ICCID, ProfileState: "disabled", ProfileName: meta.ProfileName,
		ServiceProviderName: meta.ServiceProviderName, ProfileClass: meta.ProfileClass,
		IconType: meta.IconType, Icon: meta.Icon,
	})
	c.chip.EUICCInfo2.ExtCardResource.FreeNonVolatileMemory -= 60000
	c.chip.EUICCInfo2.ExtCardResource.InstalledApplication++
	note := c.addNotification("install", meta.ICCID, req.SMDP)
	cp := *note
	return &lpa.DownloadOutcome{Notification: &cp}, nil
}
