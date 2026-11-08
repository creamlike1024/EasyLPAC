package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"EasyLPAC/internal/app"
	"EasyLPAC/internal/lpa"
	"EasyLPAC/internal/registry"
	"EasyLPAC/locales"
)

func (g *gui) cardPage(c *ui.Context) {
	if g.connectionHint(c) {
		return
	}
	t := c.Theme()
	chip := g.app.Chip
	info := chip.EUICCInfo2

	// The EID, on glass tinted with the accent. It is shown as one run of
	// digits: SGP.29 defines no grouping (its EUM identification number is
	// of variable length).
	ui.Column(c).Padding(20, 22).Gap(8).Radius(18).Material(glass.Glass{Tint: t.Accent.Alpha(0.18), Clarity: 0.3}).Children(func() {
		ui.Text(c, locales.CardEid()).FontSize(13).FontWeight(600).TextColor(t.TextMuted)
		ui.Row(c).Gap(10).Children(func() {
			ui.Text(c, g.identifier(chip.EidValue)).FontSize(21).Font("monospace").Selectable().MinWidth(0)
			if iconButton(c, iconCopy, locales.CardCopyEid()).Clicked() {
				g.clip.WriteText(chip.EidValue)
				c.Toast(locales.CardEidCopied())
			}
		})
		if !app.ValidEID(chip.EidValue) {
			ui.Text(c, locales.CardEidInvalid()).FontSize(12).TextColor(t.Warning)
		}
	})

	// The figures that matter most, as tiles: the EUM by the EID, and the
	// production site by the SAS accreditation number, each on its own.
	free := info.ExtCardResource.FreeNonVolatileMemory
	ui.Grid(c).ColumnTracks(ui.Fr(1), ui.Fr(1), ui.Fr(1), ui.Fr(1)).Gap(12).Children(func() {
		tile(c, locales.CardFreeSpace(), formatBytes(free), locales.CardEsimsInstalled(len(g.app.Profiles)))
		tile(c, "SGP.22", orDash(info.Svn), locales.CardRspVersion())
		eum := eumTile(chip.EidValue)
		tile(c, locales.CardEum(), eum.name, eum.detail).Tooltip(eum.full)
		site := siteTile(info.SasAcreditationNumber)
		tile(c, locales.CardSasSite(), site.name, site.detail).Tooltip(site.full)
	})

	sectionTitle(c, locales.CardServers())
	card(c, func() {
		dp := defaultDP(g.app.Chip.EuiccConfiguredAddresses.DefaultDpAddress)
		kv(c, locales.CardDefaultSmdp(), orText(dp, locales.CommonNotSet()), func() {
			if ui.Button(c, locales.CardChange()).Radius(14).Disabled(g.app.Busy()).Clicked() {
				g.smdpText, g.editSMDP = dp, true
			}
		})
		ui.Divider(c)
		kv(c, locales.CardRootSmds(), orText(chip.EuiccConfiguredAddresses.RootDsAddress, locales.CommonNotSet()), nil)
	})

	g.euiccInfo2(c, info)

	ui.Modal(c, &g.editSMDP, func() {
		ui.Column(c).Width(380).Gap(12).Children(func() {
			ui.Text(c, locales.CardDefaultSmdp()).FontSize(16).Bold()
			ui.Text(c, locales.CardSmdpBody()).FontSize(13).TextColor(t.TextMuted)
			submit := ui.TextInput(c, &g.smdpText).Placeholder("smdp.example.com").Label(locales.CardAddress()).AutoFocus().Submitted()
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, locales.CommonCancel()).Clicked() {
					g.editSMDP = false
				}
				if ui.PrimaryButton(c, locales.CommonSave()).Disabled(g.app.Busy()).Clicked() || submit {
					_ = g.app.SetDefaultSMDP(strings.TrimSpace(g.smdpText))
					g.editSMDP = false
				}
			})
		})
	})
}

// euiccInfo2 shows EUICCInfo2 as a tree of its fields, with the issuers
// of the CI key IDs and the supplier of the SAS site named.
func (g *gui) euiccInfo2(c *ui.Context, info lpa.EUICCInfo2) {
	t := c.Theme()
	raw := app.EUICCInfo2JSON(info)
	if g.infoJSON != string(raw) {
		g.infoJSON = string(raw)
		g.infoTree, _ = parseJSONTree(raw)
		if g.infoOpen == nil {
			g.infoOpen = map[string]bool{"$.extCardResource": true}
		}
	}
	ui.Row(c).Gap(8).Padding(8, 4, 0).Children(func() {
		ui.Text(c, "EUICCInfo2").FontSize(13).FontWeight(600).TextColor(t.TextMuted).Grow(1)
		if ui.Button(c, locales.CardExpandAll()).Radius(14).Clicked() {
			setTreeOpen(g.infoTree, g.infoOpen, true)
		}
		if ui.Button(c, locales.CardCollapseAll()).Radius(14).Clicked() {
			setTreeOpen(g.infoTree, g.infoOpen, false)
		}
		copyJSON := ui.Button(c, "").Radius(14).Children(func() {
			ui.Icon(c, iconCopy).Size(16, 16)
			ui.Text(c, locales.CardCopyJson()).SingleLine()
		})
		if copyJSON.Clicked() {
			g.clip.WriteText(g.infoJSON)
			c.Toast(locales.CardJsonCopied())
		}
	})
	if g.infoTree == nil {
		return
	}
	ui.Column(c).Padding(6).Radius(14).Background(t.Surface).Border(1, t.Border).Children(func() {
		g.jsonTree(c, g.infoTree, g.infoOpen, euiccInfo2Note, func(n *jnode) { g.euiccInfo2Action(c, n) })
	})
}

// euiccInfo2Action offers the certificate of a CI key ID.
func (g *gui) euiccInfo2Action(c *ui.Context, n *jnode) {
	if n.kind != 's' || !strings.HasPrefix(n.path, "$.euiccCiPKIdListFor") {
		return
	}
	ci := registry.LookupIssuer(n.value)
	if ci == nil || ci.Certificate == nil {
		return
	}
	b := ui.Button(c.Key("cert:"+n.path), locales.CardViewCertificate()).Padding(2, 8).Radius(10).FontSize(12)
	if b.Clicked() {
		g.openCertificate(ci, n.value)
	}
}

// euiccInfo2Note explains values of EUICCInfo2 from the registry.
func euiccInfo2Note(n *jnode) string {
	switch {
	case n.kind != 's':
		return ""
	case strings.HasPrefix(n.path, "$.euiccCiPKIdListFor"):
		if ci := registry.LookupIssuer(n.value); ci != nil {
			if cat := categoryLabel(ci.Category); cat != "" {
				return ci.Name + " · " + cat
			}
			return ci.Name
		}
		return locales.CardUnknownIssuer()
	case n.path == "$.sasAcreditationNumber":
		if s, country := registry.LookupSite(n.value); s != nil {
			return s.ShortName() + " · " + registry.CountryFlag(country) + " " + country
		}
	case n.path == "$.extCardResource.freeNonVolatileMemory" || n.path == "$.extCardResource.freeVolatileMemory":
		if v, err := strconv.Atoi(n.value); err == nil {
			return formatBytes(v)
		}
	}
	return ""
}

// supplierTile is a supplier as a Card page tile shows it.
type supplierTile struct {
	name   string // the country's flag and the short name
	detail string // the line under it
	full   string // the full name, as a tooltip
}

func describe(s *registry.Supplier, country, detail string) supplierTile {
	if s.Deprecated != "" {
		detail = strings.TrimPrefix(detail+" · "+s.Deprecated, " · ")
	}
	return supplierTile{registry.CountryFlag(country) + " " + s.ShortName(), detail, s.Name}
}

// eumTile is the eUICC manufacturer the EID's EUM identification number
// names (SGP.29), with its own country.
func eumTile(eid string) supplierTile {
	s, prefix := registry.LookupEUMPrefix(eid)
	if s == nil {
		return supplierTile{name: locales.CardMakerUnknown()}
	}
	return describe(s, s.Country, prefix)
}

// siteTile is the SAS-UP accredited site that produced the eUICC, from
// its SAS accreditation number, with the site's country.
func siteTile(sas string) supplierTile {
	s, _, country := registry.LookupSiteCode(sas)
	if s == nil {
		return supplierTile{name: locales.CardMakerUnknown(), detail: sas}
	}
	return describe(s, country, strings.ToUpper(strings.TrimSpace(sas)))
}

func tile(c *ui.Context, title, value, sub string) ui.Element {
	t := c.Theme()
	return ui.Column(c).Padding(14, 16).Gap(4).Radius(14).Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Text(c, title).FontSize(12).TextColor(t.TextMuted).SingleLine()
		ui.Text(c, value).FontSize(19).Bold().SingleLine()
		ui.Text(c, sub).FontSize(11).TextColor(t.TextMuted).SingleLine()
	})
}

// kv is a labeled value with an optional trailing control.
func kv(c *ui.Context, key, value string, trailing func()) {
	t := c.Theme()
	ui.Row(c).Gap(12).Children(func() {
		ui.Text(c, key).TextColor(t.TextMuted).Width(160).Shrink(0)
		ui.Text(c, value).Grow(1).MinWidth(0).Selectable()
		if trailing != nil {
			trailing()
		}
	})
}

func defaultDP(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func orDash(s string) string { return orText(s, "—") }

func orText(s, alt string) string {
	if strings.TrimSpace(s) == "" {
		return alt
	}
	return s
}

// formatBytes shows a byte count in binary units (1 KiB = 1024 bytes), as
// ExtCardResource counts memory in bytes (ETSI TS 102 226).
func formatBytes(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KiB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}
