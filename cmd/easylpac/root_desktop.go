//go:build !android

package main

import "github.com/egoist/mygo/ui"

func (g *gui) rootView(c *ui.Context) { g.view(c) }
