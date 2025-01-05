package main

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/dialog"
	"os"
	"path/filepath"
)

const Version = "development"
const EUICCDataVersion = "unknown"

var App fyne.App

func init() {
	App = app.New()
	App.Settings().SetTheme(&MyTheme{})

	if err := LoadConfig(); err != nil {
		panic(err)
	}
	if _, err := os.Stat(ConfigInstance.LogDir); os.IsNotExist(err) {
		err := os.Mkdir(ConfigInstance.LogDir, 0755)
		if err != nil {
			panic(err)
		}
	}
}

func main() {
	var err error
	ConfigInstance.LogFile, err = os.Create(filepath.Join(ConfigInstance.LogDir, ConfigInstance.LogFilename))
	if err != nil {
		panic(err)
	}
	defer ConfigInstance.LogFile.Close()

	InitWidgets()
	go UpdateStatusBarListener()
	go LockButtonListener()

	WMain = InitMainWindow()

	_, err = os.Stat(filepath.Join(ConfigInstance.LpacDir, ConfigInstance.EXEName))
	if err != nil {
		d := dialog.NewError(fmt.Errorf(" lpac not found\nPlease make sure you have put lpac binary in the `lpac` folder"), WMain)
		d.SetOnClosed(func() {
			os.Exit(127)
		})
		d.Show()
	} else {
		if version, err2 := LpacVersion(); err2 != nil {
			LpacVersionLabel.SetText("lpac Version: unknown")
		} else {
			LpacVersionLabel.SetText("lpac Version: " + version)
		}
		RefreshApduDriver()
		if ApduDrivers != nil {
			ApduDriverSelect.SetSelectedIndex(0)
		}
	}

	WMain.Show()
	App.Run()
}
