package main

import (
	"embed"
	"github.com/fullpipe/icu-mf/mf"
	"github.com/jeandeaual/go-locale"
	"golang.org/x/text/language"
)

//go:embed i18n/*.yaml
var i18nDir embed.FS
var TR mf.Translator

func detectSystemLanguate() string {
	userLanguage, err := locale.GetLanguage()
	if err != nil {
		return "en"
	} else {
		return userLanguage
	}
}

func init() {
	bundle, err := mf.NewBundle(
		mf.WithDefaultLangFallback(language.English),
		mf.WithYamlProvider(i18nDir))
	if err != nil {
		panic(err)
	}
	TR = bundle.Translator(detectSystemLanguate())

}
