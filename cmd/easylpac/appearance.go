package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
)

// themeSources are the appearances to choose from, in the order shown.
var themeSources = []mygo.ThemeSource{mygo.ThemeSystem, mygo.ThemeLight, mygo.ThemeDark}

// chosenLocale returns the language the app was set to show, or "" when it
// follows the user's languages. MyGo does not tell which, so a language
// that is the one the user's languages give anyway counts as following
// them: the app shows the same either way.
func chosenLocale() string {
	shown := mygo.App.Locale()
	langs := mygo.App.Languages()
	system := i18n.Match(mygo.App.PreferredLanguages(), langs)
	if system == "" && len(langs) > 0 {
		system = langs[0]
	}
	if shown == system {
		return ""
	}
	return shown
}
