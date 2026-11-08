//go:build !demo

package main

import (
	"github.com/egoist/mygo/ui"

	"EasyLPAC/internal/app"
)

// demoBuild is false in release builds; see demo.go.
const demoBuild = false

func useDemo(*app.Options) {}

func (g *gui) demoCodes(*ui.Context) {}
