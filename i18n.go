package main

import (
	"embed"
	"github.com/Xuanwo/go-locale"
	"github.com/fullpipe/icu-mf/mf"
	"golang.org/x/text/language"
)

//go:embed i18n/*.yaml
var i18nDir embed.FS
var TR mf.Translator
var LanguageTag string

func detectSystemLanguate() string {
	tag, err := locale.Detect()
	if err != nil {
		return "en"
	}
	return tag.String()
}

func InitI18n() {
	bundle, err := mf.NewBundle(
		mf.WithDefaultLangFallback(language.English),
		mf.WithYamlProvider(i18nDir))
	if err != nil {
		panic(err)
	}
	LanguageTag = detectSystemLanguate()
	TR = bundle.Translator(LanguageTag)
}
