package main

import (
	"runtime"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"EasyLPAC/internal/lpa"
	"EasyLPAC/locales"
)

func (g *gui) notificationsPage(c *ui.Context) {
	if g.connectionHint(c) {
		return
	}
	t := c.Theme()
	ns := g.app.Notifications
	if len(ns) == 0 {
		emptyState(c, iconBell, locales.NotifEmptyTitle(), locales.NotifEmptyBody(), nil)
		return
	}
	ui.Text(c, locales.NotifIntro()).
		FontSize(13).TextColor(t.TextMuted)

	ui.Text(c, locales.NotifSelectHint(selectModifier())).FontSize(12).TextColor(t.TextMuted)

	rows := g.sortedNotifications()
	g.notifList.Sort = &g.notifSort
	g.notifList.Selection = &g.notifSel
	g.notifList.Key = func(i int) any { return rows[i].SeqNumber }
	cols := []ui.TableColumn{
		{Title: "#", ID: "seq", Width: 64, Fixed: true, Sortable: true},
		{Title: locales.NotifColumnEvent(), ID: "event", Width: 120},
		{Title: locales.NotifColumnEsim(), ID: "esim", Width: 0, MinWidth: 140},
		{Title: "ICCID", ID: "iccid", Width: 190},
		{Title: locales.NotifColumnServer(), ID: "server", Width: 0, MinWidth: 160},
	}
	ui.Table(c, &g.notifList, cols, len(rows), func(row, col int) {
		n := rows[row]
		muted := t.TextMuted
		if g.notifSel.Has(n.SeqNumber) {
			muted = t.AccentText.Alpha(0.8)
		}
		switch col {
		case 0:
			ui.Textf(c, "%d", n.SeqNumber).Font("monospace").TextColor(muted)
		case 1:
			ui.Text(c, operationLabel(n.ProfileManagementOperation)).SingleLine()
		case 2:
			ui.Text(c, g.notificationProfile(n)).SingleLine()
		case 3:
			ui.Text(c, g.identifier(n.Iccid)).Font("monospace").FontSize(12).SingleLine().TextColor(muted)
		case 4:
			ui.Text(c, n.NotificationAddress).SingleLine().TextColor(muted)
		}
	}).Height(min(float32(len(rows))*36+40, 460)).Radius(12).Border(1, t.Border).
		ContextMenu(func(m *ui.Menu) {
			seqs := g.selectedNotifications()
			if len(seqs) == 0 {
				return
			}
			busy := g.app.Busy()
			if m.Item(locales.NotifSend()).Disabled(busy).Chosen() {
				_ = g.app.ProcessNotifications(seqs, true)
			}
			if m.Item(locales.NotifSendKeep()).Disabled(busy).Chosen() {
				_ = g.app.ProcessNotifications(seqs, false)
			}
			m.Separator()
			if m.Item(locales.NotifRemove()).Danger().Disabled(busy).Chosen() {
				_ = g.app.RemoveNotifications(seqs)
			}
		})

	switch ui.AlertDialog(c, &g.removeAll, locales.NotifRemoveAllTitle(len(ns)), locales.NotifRemoveAllBody(),
		locales.CommonCancel(), locales.NotifRemoveConfirm()) {
	case 1:
		seqs := make([]int, len(ns))
		for i, n := range ns {
			seqs[i] = n.SeqNumber
		}
		_ = g.app.RemoveNotifications(seqs)
	}
}

func (g *gui) notificationActions(c *ui.Context) {
	ns := g.app.Notifications
	if !g.app.Loaded || len(ns) == 0 {
		return
	}
	busy := g.app.Busy()
	selected := g.selectedNotifications()
	if iconButton(c, iconTrash, locales.NotifRemoveAll()).Disabled(busy).Clicked() {
		g.removeAll = true
	}
	if len(selected) > 0 {
		if ui.Button(c, locales.NotifSendSelected(len(selected))).Radius(16).Disabled(busy).Clicked() {
			_ = g.app.ProcessNotifications(selected, true)
		}
	}
	send := ui.PrimaryButton(c, "").Radius(16).Disabled(busy).Children(func() {
		ui.Icon(c, iconSend).Size(16, 16)
		ui.Text(c, locales.NotifSendAll()).SingleLine()
	})
	if send.Clicked() {
		seqs := make([]int, len(ns))
		for i, n := range ns {
			seqs[i] = n.SeqNumber
		}
		_ = g.app.ProcessNotifications(seqs, true)
	}
}

// notificationProfile names the eSIM a notification is about.
// (its nickname, else its name, else "Unnamed eSIM": also when the eSIM
// is no longer on the card, as after deleting it).
func (g *gui) notificationProfile(n *lpa.Notification) string {
	for _, p := range g.app.Profiles {
		if p.Iccid == n.Iccid {
			return profileName(p)
		}
	}
	return locales.ProfileUnnamed()
}

func operationLabel(op string) string {
	switch op {
	case "install":
		return locales.NotifOpInstall()
	case "enable":
		return locales.NotifOpEnable()
	case "disable":
		return locales.NotifOpDisable()
	case "delete":
		return locales.NotifOpDelete()
	}
	if op == "" {
		return "—"
	}
	return strings.ToUpper(op[:1]) + op[1:]
}

// sortedNotifications are the notifications as the table shows them: by
// sequence number, newest first unless the user sorted them ascending.
func (g *gui) sortedNotifications() []*lpa.Notification {
	rows := slices.Clone(g.app.Notifications)
	slices.SortFunc(rows, func(a, b *lpa.Notification) int { return a.SeqNumber - b.SeqNumber })
	if g.notifSort.Descending {
		slices.Reverse(rows)
	}
	return rows
}

// selectedNotifications are the sequence numbers chosen in the table that
// are still on the card, oldest first: the order they are sent in.
func (g *gui) selectedNotifications() []int {
	var seqs []int
	for _, n := range g.app.Notifications {
		if g.notifSel.Has(n.SeqNumber) {
			seqs = append(seqs, n.SeqNumber)
		}
	}
	slices.Sort(seqs)
	return seqs
}

// selectModifier names the key that adds a row to the choice.
func selectModifier() string {
	if runtime.GOOS == "darwin" {
		return "⌘"
	}
	return "Ctrl"
}
