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

	if _, err := os.Stat(ConfigInstance.LogDir); os.IsNotExist(err) {
		os.Mkdir(ConfigInstance.LogDir, 0755)
	}
}

func main() {
	go UpdateStatusBar()
	go LockButton()
	InitWidgets()

	var err error
	ConfigInstance.LogFile, err = os.Create(filepath.Join(ConfigInstance.LogDir, ConfigInstance.LogFilename))
	if err != nil {
		panic(err)
	}
	defer ConfigInstance.LogFile.Close()

	WMain = InitMainWindow()
	RefreshApduDriver()
	if ApduDrivers != nil {
		ApduDriverSelect.SetSelectedIndex(0)
	}
	WMain.Show()

	App.Run()
}
