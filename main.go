package main

import (
	"encoding/json"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/dialog"
	"os"
	"path/filepath"
)

const Version = "0.6.7"
const EUICCDataVersion = "2024-02-06"

var App fyne.App

func init() {
	App = app.New()
	App.Settings().SetTheme(&myTheme{})

	StatusChan = make(chan int)
	LockButtonChan = make(chan bool)
	SelectedProfile = Unselected
	SelectedNotification = Unselected
	RefreshNeeded = true
	if err := json.Unmarshal(CIRegistryByte, &CIRegistry); err != nil {
		panic(err)
	}
	if err := json.Unmarshal(EUMRegistryByte, &EUMRegistry); err != nil {
		panic(err)
	}
	if err := LoadConfig(); err != nil {
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

	_, err = os.Stat(filepath.Join(ConfigInstance.LpacDir, ConfigInstance.EXEName))
	if err != nil {
		d := dialog.NewError(fmt.Errorf("lpac not found\nPlease make sure you have put lpac binary in the `lpac` folder"), WMain)
		d.SetOnClosed(func() {
			os.Exit(127)
		})
		d.Show()
	} else {
		RefreshApduDriver()
		if ApduDrivers != nil {
			ApduDriverSelect.SetSelectedIndex(0)
		}
	}

	WMain.Show()
	App.Run()
}
