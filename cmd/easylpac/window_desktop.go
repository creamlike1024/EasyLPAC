//go:build !android && !phone

package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// openWindows opens the desktop's one window, with its sidebar.
func (g *gui) openWindows(title string) {
	g.win = mygo.NewWindow(mygo.WindowOptions{
		Title:          title,
		Width:          1040,
		Height:         700,
		MinWidth:       760,
		MinHeight:      520,
		TitleBarStyle:  mygo.TitleBarHiddenInset,
		TitleBarHeight: titleBarHeight,
		Vibrancy:       mygo.VibrancySidebar,
		Content:        ui.View(g.view),
	})
	g.wins = []*mygo.Window{g.win}
	g.lead.Store(g.win)
}

// shownWins are the windows on screen: the desktop's one.
func (g *gui) shownWins() []*mygo.Window { return g.wins }
