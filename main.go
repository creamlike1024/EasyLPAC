package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"os"
	"path/filepath"
)

var App fyne.App

func init() {
	App = app.New()

	StatusChan = make(chan int)
	LockButtonChan = make(chan bool)
	SelectedProfile = -1
	SelectedNotification = -1

	err := LoadConfig()
	if err != nil {
		panic(err)
	}

	if _, err := os.Stat(ConfigInstance.logDir); os.IsNotExist(err) {
		os.Mkdir(ConfigInstance.logDir, 0755)
	}
}

func main() {
	go UpdateStatusBar()
	go LockButton()
	InitWidgets()

	var err error
	ConfigInstance.LogFile, err = os.Create(filepath.Join(ConfigInstance.logDir, ConfigInstance.logFilename))
	if err != nil {
		panic(err)
	}
	defer ConfigInstance.LogFile.Close()

	WMain = InitMainWindow()
	WMain.Show()

	App.Run()
}
