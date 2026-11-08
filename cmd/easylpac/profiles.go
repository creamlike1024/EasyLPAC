package main

import (
	"strings"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"EasyLPAC/internal/lpa"
	"EasyLPAC/locales"
)

func (g *gui) profilesPage(c *ui.Context) {
	if g.connectionHint(c) {
		return
	}
	if len(g.app.Profiles) == 0 {
		emptyState(c, iconSIM, locales.ProfilesEmptyTitle(),
			locales.ProfilesEmptyBody(), func() {
				if ui.PrimaryButton(c, locales.ActionAddEsim()).Clicked() {
					g.openDownload()
				}
			})
		return
	}
	var active, others []*lpa.Profile
	for _, p := range g.app.Profiles {
		if p.ProfileState == "enabled" {
			active = append(active, p)
		} else {
			others = append(others, p)
		}
	}
	if len(active) > 0 {
		sectionTitle(c, locales.ProfilesInUse())
		for _, p := range active {
			g.profileCard(c, p)
		}
	}
	if len(others) > 0 {
		sectionTitle(c, locales.ProfilesOther())
		for _, p := range others {
			g.profileCard(c, p)
		}
	}
}

// profileName is what a profile is called: its nickname, else its name.
func profileName(p *lpa.Profile) string {
	if p.ProfileNickname != nil && *p.ProfileNickname != "" {
		return *p.ProfileNickname
	}
	if p.ProfileName != "" {
		return p.ProfileName
	}
	return locales.ProfileUnnamed()
}

func (g *gui) profileCard(c *ui.Context, p *lpa.Profile) {
	t := c.Theme()
	enabled := p.ProfileState == "enabled"
	busy := g.app.Busy()
	row := ui.Row(c.Key(p.Iccid)).Padding(14, 16).Gap(14).Radius(14).Background(t.Surface).Border(1, t.Border)
	if enabled {
		row.Border(1, t.Accent.Alpha(0.6))
	}
	row.ContextMenu(func(m *ui.Menu) { g.profileMenu(m, p) })
	row.Children(func() {
		g.profileIcon(c, p.Iccid, p.Icon, profileName(p), 44)
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			ui.Row(c).Gap(8).Children(func() {
				ui.Text(c, profileName(p)).FontSize(15).Bold().SingleLine()
				if enabled {
					ui.Badge(c, locales.ProfileActive())
				}
			})
			var sub []string
			if p.ServiceProviderName != "" {
				sub = append(sub, p.ServiceProviderName)
			}
			if p.ProfileNickname != nil && *p.ProfileNickname != "" && p.ProfileName != "" {
				sub = append(sub, p.ProfileName)
			}
			if len(sub) > 0 {
				ui.Text(c, strings.Join(sub, " · ")).FontSize(13).TextColor(t.TextMuted).SingleLine()
			}
			ui.Row(c).Gap(4).Children(func() {
				ui.Text(c, locales.ProfileIccid(g.identifier(p.Iccid))).FontSize(12).Font("monospace").TextColor(t.TextMuted).Selectable()
				if copyButton(c, p.Iccid, locales.ProfileMenuCopyIccid()).Clicked() {
					g.clip.WriteText(p.Iccid)
					c.Toast(locales.ProfileIccidCopied())
				}
			})
		})
		on := enabled
		sw := ui.Switch(c, &on).Label(locales.ProfileUse(profileName(p))).Disabled(busy)
		if sw.Changed() {
			_ = g.app.SetProfileEnabled(p.Iccid, on)
		}
		iconButton(c, iconDots, locales.ProfileMore(profileName(p))).Menu(func(m *ui.Menu) { g.profileMenu(m, p) })
	})
}

func (g *gui) profileMenu(m *ui.Menu, p *lpa.Profile) {
	busy := g.app.Busy()
	enabled := p.ProfileState == "enabled"
	if m.Item(locales.ProfileMenuRename()).Disabled(busy).Chosen() {
		g.renameICCID, g.renaming = p.Iccid, true
		g.renameText = ""
		if p.ProfileNickname != nil {
			g.renameText = *p.ProfileNickname
		}
	}
	if m.Item(locales.ProfileMenuCopyIccid()).Chosen() {
		g.clip.WriteText(p.Iccid)
		g.toast = locales.ProfileIccidCopied()
	}
	m.Separator()
	del := m.Item(locales.ProfileMenuDelete()).Danger().Disabled(busy || enabled)
	if enabled {
		del.Tooltip(locales.ProfileDeleteDisabledHint())
	}
	if del.Chosen() {
		g.deleteICCID, g.deleting = p.Iccid, true
	}
}

// profileIcon shows the profile's own icon, or its initial on a tint.
func (g *gui) profileIcon(c *ui.Context, key string, data []byte, name string, size float32) {
	t := c.Theme()
	if g.icons == nil {
		g.icons = map[string]ui.ImageSource{}
	}
	src, ok := g.icons[key]
	if !ok && len(data) > 0 {
		src, _ = ui.DecodeImage(data)
		g.icons[key] = src
	}
	if src != nil {
		ui.Image(c, src).Size(size, size).Radius(size / 4).Shrink(0)
		return
	}
	initial := "?"
	if r := []rune(strings.TrimSpace(name)); len(r) > 0 {
		initial = strings.ToUpper(string(r[0]))
	}
	ui.Box(c).Size(size, size).Radius(size / 4).Shrink(0).Center().
		Material(glass.Glass{Tint: t.Accent.Alpha(0.35)}).Children(func() {
		ui.Text(c, initial).FontSize(size * 0.42).Bold().TextColor(t.Accent)
	})
}

// profileDialogs are the rename and delete dialogs.
func (g *gui) profileDialogs(c *ui.Context) {
	ui.Modal(c, &g.renaming, func() {
		ui.Column(c).Width(360).Gap(12).Children(func() {
			ui.Text(c, locales.ProfileRenameTitle()).FontSize(16).Bold()
			ui.Text(c, locales.ProfileRenameBody()).FontSize(13).TextColor(c.Theme().TextMuted)
			submit := ui.TextInput(c, &g.renameText).Placeholder(locales.ProfileNickname()).Label(locales.ProfileNickname()).MaxLength(64).AutoFocus().Submitted()
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, locales.CommonCancel()).Clicked() {
					g.renaming = false
				}
				if ui.PrimaryButton(c, locales.CommonSave()).Disabled(g.app.Busy()).Clicked() || submit {
					_ = g.app.RenameProfile(g.renameICCID, strings.TrimSpace(g.renameText))
					g.renaming = false
				}
			})
		})
	})

	title := locales.ProfileDeleteTitleUnnamed()
	for _, p := range g.app.Profiles {
		if p.Iccid == g.deleteICCID {
			title = locales.ProfileDeleteTitle(profileName(p))
		}
	}
	switch ui.AlertDialog(c, &g.deleting, title, locales.ProfileDeleteBody(),
		locales.CommonCancel(), locales.ProfileDeleteConfirm()) {
	case 1:
		_ = g.app.DeleteProfile(g.deleteICCID)
	}
}
