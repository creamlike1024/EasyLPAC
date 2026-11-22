//go:build android

package main

import "github.com/egoist/mygo/ui"

// rootView is the read-only reader test until Android has an interface of
// its own; the demo keeps the desktop's.
func (g *gui) rootView(c *ui.Context) {
	if g.demo {
		g.view(c)
		return
	}
	g.readOnlyView(c)
}
