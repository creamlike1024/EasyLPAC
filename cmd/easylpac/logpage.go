package main

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"EasyLPAC/locales"
)

func (g *gui) logPage(c *ui.Context) {
	t := c.Theme()
	entries, _ := g.app.Log.Snapshot()
	hide := g.app.Settings.HideIdentifiers
	if !hide {
		ui.Row(c).Padding(8, 12).Gap(8).Radius(10).Background(t.Accent.Alpha(0.10)).Border(1, t.Accent.Alpha(0.35)).Children(func() {
			ui.Icon(c, iconInfo).Size(16, 16).TextColor(t.Accent).Shrink(0)
			ui.Text(c, locales.LogHideHint()).FontSize(12).Grow(1).MinWidth(0)
		})
	}
	g.logList.FollowEnd = true
	ui.List(c, &g.logList, len(entries), func(i int) {
		e := entries[i]
		color := t.Text
		switch {
		case e.Level >= slog.LevelError:
			color = t.Danger
		case e.Level >= slog.LevelWarn:
			color = t.Warning
		case e.Level < slog.LevelInfo:
			color = t.TextMuted
		}
		ui.Row(c).PaddingX(12).PaddingY(3).Gap(10).AlignItems(ui.Start).Children(func() {
			ui.Text(c, e.Time.Format("15:04:05.000")).Font("monospace").FontSize(12).TextColor(t.TextMuted).Shrink(0)
			msg := e.Message
			if attrs := e.AttrText(hide); attrs != "" {
				msg += "  " + attrs
			}
			ui.Text(c, msg).Font("monospace").FontSize(12).TextColor(color).Grow(1).MinWidth(0)
		})
	}).Height(max(logHeight(c, !hide), 240)).Radius(12).Border(1, t.Border).Background(t.Surface).Padding(6, 0).Selectable()
}

func (g *gui) logActions(c *ui.Context) {
	if iconButton(c, iconCopy, locales.LogCopy()).Clicked() {
		var b bytes.Buffer
		_ = g.app.Log.Export(&b, g.app.Settings.HideIdentifiers)
		g.clip.WriteText(b.String())
		c.Toast(locales.LogCopied())
	}
	exp := ui.PrimaryButton(c, "").Radius(16).Children(func() {
		ui.Icon(c, iconExport).Size(16, 16)
		ui.Text(c, locales.LogExport()).SingleLine()
	})
	if exp.Clicked() {
		go g.exportLog(g.app.Settings.HideIdentifiers)
	}
}

// exportLog asks where to save the log and writes it there.
// hide is read on the UI thread when the export starts.
func (g *gui) exportLog(hide bool) {
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent:      g.win,
		Title:       locales.LogExportTitle(),
		DefaultPath: fmt.Sprintf("EasyLPAC-%s.log", time.Now().Format("20060102-150405")),
		Filters:     []mygo.FileFilter{{Name: locales.LogFileType(), Extensions: []string{"log"}}},
	})
	if err != nil || path == "" {
		return
	}
	if !strings.EqualFold(filepath.Ext(path), ".log") {
		path += ".log"
	}
	var b bytes.Buffer
	_ = g.app.Log.Export(&b, hide)
	err = os.WriteFile(path, b.Bytes(), 0o600)
	g.win.Update(func() {
		if err != nil {
			g.app.Logger.Error("log export failed", "err", err)
			g.toast = locales.LogExportFailed(err.Error())
			return
		}
		g.app.Logger.Info("log exported")
		g.toast = locales.LogExported()
	})
}

// logHeight fills the window below the page's header (68), its bottom
// padding (28) and the hint above the list when it shows.
func logHeight(c *ui.Context, hint bool) float32 {
	_, h := c.Size()
	h -= 68 + 28
	if hint {
		h -= 36 + 16
	}
	return h
}
