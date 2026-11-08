package main

import (
	"github.com/egoist/mygo/ui"

	"EasyLPAC/assets"
	"EasyLPAC/internal/app"
	"EasyLPAC/internal/registry"
	"EasyLPAC/locales"
)

// author is shown on the About page.
const author = "creamlike1024"

func (g *gui) aboutPage(c *ui.Context) {
	t := c.Theme()
	if g.appIcon == nil {
		g.appIcon, _ = ui.DecodeImage(assets.Icon)
	}
	card(c, func() {
		ui.Row(c).Gap(16).Children(func() {
			if g.appIcon != nil {
				ui.Image(c, g.appIcon).Size(64, 64).Shrink(0)
			}
			ui.Column(c).Gap(3).Children(func() {
				ui.Text(c, "EasyLPAC").FontSize(22).Bold()
				externalLink(c, locales.AboutRepo(), "https://github.com/"+author+"/EasyLPAC").AlignSelf(ui.Start)
				ui.Row(c).Gap(16).Children(func() {
					ui.Text(c, locales.AboutAuthor(author)).TextColor(t.TextMuted)
					ui.Text(c, locales.AboutVersion(Version)).TextColor(t.TextMuted)
				})
			})
		})
	})

	sectionTitle(c, locales.AboutComponents())
	card(c, func() {
		commit := app.LpacCommit()
		kv(c, "libeuicc", locales.AboutLibeuicc(app.LibeuiccVersion()), func() {
			externalLink(c, "estkme-group/lpac", "https://github.com/estkme-group/lpac/commit/"+commit)
		})
		kv(c, locales.AboutEuiccData(), registry.DataVersion, func() {
			externalLink(c, "euicc-manual.osmocom.org", "https://euicc-manual.osmocom.org/docs/pki/eum/")
		})
		kv(c, locales.AboutInterface(), "MyGo", func() {
			externalLink(c, "github.com/egoist/mygo", "https://github.com/egoist/mygo")
		})
	})

	// Each component under its own license: EasyLPAC's, and the static
	// libeuicc's with what the LGPL asks of a static link.
	sectionTitle(c, locales.AboutLicense())
	card(c, func() {
		licenseRow(c, "EasyLPAC", "MIT", "https://github.com/"+author+"/EasyLPAC/blob/master/LICENSE")
		ui.Divider(c)
		licenseRow(c, "libeuicc", "LGPL-2.1-only", "https://github.com/estkme-group/lpac/blob/"+app.LpacCommit()+"/euicc/LICENSE")
		ui.Divider(c)
		licenseRow(c, "cJSON", "MIT", "https://github.com/DaveGamble/cJSON/blob/master/LICENSE")
		ui.Divider(c)
		licenseRow(c, "MyGo", "MIT", "https://github.com/egoist/mygo/blob/main/LICENSE")
	})
}

// licenseRow names a component and its license (SPDX), with a link to
// the license's text.
func licenseRow(c *ui.Context, component, license, url string) {
	ui.Row(c).Gap(8).Children(func() {
		ui.Text(c, component).Bold()
		ui.Badge(c, license)
		ui.Spacer(c)
		externalLink(c, locales.AboutLicenseText(), url)
	})
}

// externalLink is a link that opens a page outside the app, marked so by
// an icon after its text (a Link holds text alone).
func externalLink(c *ui.Context, label, url string) ui.Element {
	t := c.Theme()
	link := ui.ButtonBase(c.Key(url)).Role(ui.RoleLink).Label(label).Gap(4).Cursor(ui.CursorPointer)
	link.Children(func() {
		text := ui.Text(c, label).TextColor(t.Accent)
		if link.Hovered() {
			text.Underline()
		}
		ui.Icon(c, iconExternal).Size(13, 13).TextColor(t.Accent)
	})
	if link.Clicked() {
		c.OpenURL(url)
	}
	return link
}
