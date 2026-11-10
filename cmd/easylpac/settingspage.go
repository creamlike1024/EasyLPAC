package main

import (
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"EasyLPAC/internal/app"
	"EasyLPAC/internal/settings"
	"EasyLPAC/locales"
)

// aidPresets are the ISD-R AIDs to choose from, by name.
var aidPresets = []struct {
	label func() string
	aid   string
}{
	{locales.SettingsAidStandard, settings.AIDDefault},
	{func() string { return "5ber" }, settings.AID5ber},
	{func() string { return "eSIM.me" }, settings.AIDESIMme},
	{func() string { return "Xesim" }, settings.AIDXesim},
}

var baudRates = []string{"9600", "19200", "38400", "57600", "115200", "230400", "460800", "921600"}

func (g *gui) settingsPage(c *ui.Context) {
	cfg := g.app.Settings
	busy := g.app.Busy()

	sectionTitle(c, locales.SettingsConnection())
	card(c, func() {
		kind := 0
		if cfg.Backend == settings.BackendAT {
			kind = 1
		}
		ui.Field(c, locales.SettingsConnectThrough(), func() {
			if ui.Segmented(c, &kind, locales.SettingsBackendPcsc(), locales.SettingsBackendAt()).AlignSelf(ui.Start).Disabled(busy).Changed() {
				if kind == 1 {
					g.app.SelectBackend(settings.BackendAT)
				} else {
					g.app.SelectBackend(settings.BackendPCSC)
				}
			}
		})
		switch cfg.Backend {
		case settings.BackendAT:
			g.atSettings(c, busy)
		default:
			g.readerSettings(c, busy)
		}
	})

	sectionTitle(c, locales.SettingsEuicc())
	card(c, func() {
		g.aidSettings(c, busy)
	})

	sectionTitle(c, locales.SettingsBehavior())
	card(c, func() {
		toggleRow(c, locales.SettingsAutoNotify(), locales.SettingsAutoNotifyBody(),
			cfg.AutoNotify, func(on bool) { g.app.UpdateSettings(func(s *settings.Config) { s.AutoNotify = on }) })
		if !cfg.AutoNotify {
			g.manualNotifyWarning(c)
		}
		ui.Divider(c)
		toggleRow(c, locales.SettingsHideIds(), locales.SettingsHideIdsBody(),
			cfg.HideIdentifiers, func(on bool) { g.app.UpdateSettings(func(s *settings.Config) { s.HideIdentifiers = on }) })
	})

	sectionTitle(c, locales.SettingsAppearance())
	card(c, func() {
		themeSettings(c)
		ui.Divider(c)
		g.languageSettings(c)
	})

	sectionTitle(c, locales.SettingsDiagnostics())
	card(c, func() {
		toggleRow(c, locales.SettingsLogApdu(), locales.SettingsLogApduBody(),
			cfg.DebugAPDU, func(on bool) { g.app.UpdateSettings(func(s *settings.Config) { s.DebugAPDU = on }) })
		ui.Divider(c)
		toggleRow(c, locales.SettingsLogHttp(), locales.SettingsLogHttpBody(),
			cfg.DebugHTTP, func(on bool) { g.app.UpdateSettings(func(s *settings.Config) { s.DebugHTTP = on }) })
	})
}

func (g *gui) readerSettings(c *ui.Context, busy bool) {
	t := c.Theme()
	cfg := g.app.Settings
	b := cfg.Backends[settings.BackendPCSC]
	var names []string
	for _, d := range g.app.Devices {
		names = append(names, d.ID)
	}
	current := b.Reader
	if current != "" && !slices.Contains(names, current) {
		names = append(names, current)
	}
	ui.Field(c, locales.SettingsReader(), func() {
		ui.Row(c).Gap(8).Children(func() {
			switch {
			case g.app.Discovering && len(names) == 0:
				ui.Text(c, locales.SettingsLooking()).TextColor(t.TextMuted).Grow(1)
			case len(names) == 0:
				ui.Text(c, locales.SettingsNoReader()).TextColor(t.TextMuted).Grow(1)
			case current == "":
				// Nothing chosen yet: a prompt as the select's value.
				pick := locales.SettingsChooseReader()
				if ui.Select(c, &pick, append([]string{pick}, names...)).Label(locales.SettingsReader()).Disabled(busy).Grow(1).Changed() && pick != locales.SettingsChooseReader() {
					g.app.SelectDevice(pick)
				}
			case ui.Select(c, &current, names).Label(locales.SettingsReader()).Disabled(busy).Grow(1).Changed():
				g.app.SelectDevice(current)
			}
			g.findDevicesButton(c, locales.SettingsFindReaders())
		})
	})
	if current != "" && !g.app.DevicePresent() {
		ui.Text(c, locales.SettingsReaderAbsent(current)).FontSize(12).TextColor(t.Warning)
	}
	on := !b.DisableTerminalCapability
	toggleRow(c, locales.SettingsTerminalCap(), locales.SettingsTerminalCapBody(), on, func(on bool) {
		g.app.UpdateSettings(func(s *settings.Config) {
			bb := s.Backends[settings.BackendPCSC]
			bb.DisableTerminalCapability = !on
			s.Backends[settings.BackendPCSC] = bb
		})
	})
}

func (g *gui) atSettings(c *ui.Context, busy bool) {
	t := c.Theme()
	cfg := g.app.Settings
	b := cfg.Backends[settings.BackendAT]
	var ports []string
	for _, d := range g.app.Devices {
		ports = append(ports, d.ID)
	}
	if g.atPort == "" {
		g.atPort = b.Port
	}
	ui.Field(c, locales.SettingsSerialPort(), func() {
		ui.Row(c).Gap(8).Children(func() {
			if len(ports) > 0 {
				sel := b.Port
				if !slices.Contains(ports, sel) {
					sel = ""
				}
				if ui.Select(c, &sel, ports).Label(locales.SettingsDetectedPorts()).Disabled(busy).Width(240).Changed() {
					g.atPort = sel
					g.app.SelectDevice(sel)
				}
			}
			g.findDevicesButton(c, locales.SettingsFindPorts())
			submit := ui.TextInput(c, &g.atPort).Placeholder("/dev/ttyUSB2, COM5").Label(locales.SettingsPort()).Grow(1).Submitted()
			if ui.Button(c, locales.SettingsUse()).Radius(14).Disabled(busy || strings.TrimSpace(g.atPort) == "" || g.atPort == b.Port).Clicked() || submit {
				g.app.SelectDevice(strings.TrimSpace(g.atPort))
			}
		})
	})
	baud := strconv.Itoa(b.Baud)
	if b.Baud == 0 {
		baud = strconv.Itoa(settings.DefaultATBaud)
	}
	ui.Field(c, locales.SettingsBaud(), func() {
		if ui.Select(c, &baud, baudRates).Label(locales.SettingsBaud()).Disabled(busy).Width(160).Changed() {
			n, _ := strconv.Atoi(baud)
			g.app.UpdateSettings(func(s *settings.Config) {
				bb := s.Backends[settings.BackendAT]
				bb.Baud = n
				s.Backends[settings.BackendAT] = bb
			})
		}
	})
	ui.Text(c, locales.SettingsAtRequirement()).FontSize(12).TextColor(t.TextMuted)
}

func (g *gui) aidSettings(c *ui.Context, busy bool) {
	t := c.Theme()
	cfg := g.app.Settings
	preset := ""
	names := []string{}
	for _, p := range aidPresets {
		names = append(names, p.label())
		if strings.EqualFold(p.aid, cfg.AID) {
			preset = p.label()
		}
	}
	aidCustom := locales.SettingsAidCustom()
	names = append(names, aidCustom)
	if !g.aidInit {
		g.aidInit = true
		g.aidCustomMode = preset == ""
		g.customAID = cfg.AID
	}
	choice := preset
	if g.aidCustomMode || preset == "" {
		choice = aidCustom
	}
	custom := g.aidCustomMode
	_, aidErr := app.ParseAID(g.customAID)
	ui.Field(c, locales.SettingsAid(), func() {
		ui.Row(c).Gap(8).Children(func() {
			if ui.Select(c, &choice, names).Label(locales.SettingsAid()).Disabled(busy).Width(200).Shrink(0).Changed() {
				if choice == aidCustom {
					// Custom starts from what the field shows.
					g.aidCustomMode = true
				} else {
					g.aidCustomMode = false
					for _, p := range aidPresets {
						if p.label() == choice {
							g.app.UpdateSettings(func(s *settings.Config) { s.AID = p.aid })
							g.customAID = p.aid
						}
					}
				}
			}
			in := ui.TextInput(c, &g.customAID).Placeholder(locales.SettingsAidPlaceholder()).Label(locales.SettingsCustomAid()).
				Font("monospace").MaxLength(32).Disabled(busy || !custom).Grow(1)
			if custom && aidErr != nil && g.customAID != "" {
				in.Invalid(true)
			}
			if custom {
				apply := ui.Button(c, locales.SettingsApply()).Radius(14).Disabled(busy || aidErr != nil || strings.EqualFold(g.customAID, cfg.AID))
				if apply.Clicked() || (in.Submitted() && aidErr == nil) {
					aid := strings.ToUpper(g.customAID)
					g.app.UpdateSettings(func(s *settings.Config) { s.AID = aid })
					c.Toast(locales.SettingsAidSaved())
				}
			}
		})
	})
	switch {
	case custom && aidErr != nil && g.customAID != "":
		ui.Text(c, locales.SettingsAidInvalid()).FontSize(12).TextColor(t.Danger)
	case custom && !strings.EqualFold(g.customAID, cfg.AID):
		ui.Text(c, locales.SettingsAidNotApplied(cfg.AID)).FontSize(12).Font("monospace").TextColor(t.TextMuted)
	}
	ui.Text(c, locales.SettingsAidHint()).FontSize(12).TextColor(t.TextMuted)
}

// themeSettings picks the appearance. MyGo keeps it until the app quits:
// each launch follows the system again.
func themeSettings(c *ui.Context) {
	choice := slices.Index(themeSources, mygo.Theme.Source())
	ui.Field(c, locales.SettingsTheme(), func() {
		if ui.Segmented(c, &choice, locales.SettingsThemeSystem(), locales.SettingsThemeLight(), locales.SettingsThemeDark()).AlignSelf(ui.Start).Changed() {
			mygo.Theme.SetSource(themeSources[choice])
		}
	})
}

// languageSettings picks the language the app shows, or the user's
// languages, which MyGo keeps for the next launches. Each language is
// named in itself, so a reader lost in another language finds theirs.
func (g *gui) languageSettings(c *ui.Context) {
	tags := append([]string{""}, mygo.App.Languages()...)
	names := []string{locales.SettingsLanguageSystem()}
	for _, tag := range tags[1:] {
		name := i18n.Autonym(tag)
		if name == "" {
			name = tag
		}
		names = append(names, name)
	}
	choice := names[max(slices.Index(tags, g.locale), 0)]
	ui.Field(c, locales.SettingsLanguage(), func() {
		if ui.Select(c, &choice, names).Label(locales.SettingsLanguage()).Width(200).Changed() {
			g.locale = tags[slices.Index(names, choice)]
			mygo.App.SetLocale(g.locale)
		}
	})
}

// manualNotifyWarning tells what turning automatic notifications off means.
func (g *gui) manualNotifyWarning(c *ui.Context) {
	t := c.Theme()
	pending := len(g.app.Notifications)
	ui.Row(c).Padding(10, 12).Gap(10).Radius(10).AlignItems(ui.Start).
		Background(t.Warning.Alpha(0.14)).Border(1, t.Warning.Alpha(0.55)).Children(func() {
		ui.Icon(c, iconAlert).Size(18, 18).TextColor(t.Warning).Shrink(0)
		ui.Column(c).Grow(1).MinWidth(0).Gap(6).Children(func() {
			ui.Text(c, locales.SettingsManualNotify()).
				FontSize(12)
			if pending > 0 {
				ui.Text(c, locales.SettingsPending(pending)).FontSize(12).Bold()
			}
		})
	})
}

// toggleRow is a setting with a title, a description and a switch.
func toggleRow(c *ui.Context, title, desc string, on bool, set func(bool)) {
	t := c.Theme()
	ui.Row(c).Gap(16).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, title)
			ui.Text(c, desc).FontSize(12).TextColor(t.TextMuted)
		})
		v := on
		if ui.Switch(c, &v).Label(title).Changed() {
			set(v)
		}
	})
}

// findDevicesButton looks for devices again: the only way the device list
// changes after launch, as nothing polls.
func (g *gui) findDevicesButton(c *ui.Context, label string) {
	b := ui.Button(c, "").Label(label).Tooltip(label).Radius(16).Disabled(g.app.Discovering).Children(func() {
		if g.app.Discovering {
			ui.Spinner(c).Size(16, 16)
		} else {
			ui.Icon(c, iconRefresh).Size(16, 16)
		}
	})
	if b.Clicked() {
		g.app.RefreshDevices()
	}
}
