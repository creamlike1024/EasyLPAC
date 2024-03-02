package main

import (
	"encoding/json"
	"fmt"
	"fyne.io/fyne/v2/widget"
	"math"
	"os/exec"
	"runtime"
)

const StatusProcess = 1
const StatusReady = 0
const Unselected = -1

var SelectedProfile int
var SelectedNotification int

var RefreshNeeded bool
var ProfileMaskNeeded bool
var NotificationMaskNeeded bool
var ProfileStateAllowDisable bool

var StatusChan chan int
var LockButtonChan chan bool

func RefreshProfile() {
	var err error
	Profiles, err = LpacProfileList()
	if err != nil {
		ShowLpacErrDialog(err)
	}
	// 刷新 List
	ProfileList.Refresh()
	ProfileList.UnselectAll()
}

func RefreshNotification() {
	var err error
	Notifications, err = LpacNotificationList()
	if err != nil {
		ShowLpacErrDialog(err)
	}
	// 刷新 List
	NotificationList.Refresh()
	NotificationList.UnselectAll()
}

func RefreshChipInfo() {
	var err error
	ChipInfo, err = LpacChipInfo()
	if err != nil {
		ShowLpacErrDialog(err)
	}

	convertToString := func(value interface{}) string {
		if value == nil {
			return "<not set>"
		}
		if str, ok := value.(string); ok {
			return str
		}
		return "<not set>"
	}

	EidLabel.SetText(fmt.Sprintf("EID: %s", ChipInfo.EidValue))
	DefaultDpAddressLabel.SetText(fmt.Sprintf("Default SM-DP+ Address:  %s", convertToString(ChipInfo.EuiccConfiguredAddresses.DefaultDpAddress)))
	RootDsAddressLabel.SetText(fmt.Sprintf("Root SM-DS Address:  %s", convertToString(ChipInfo.EuiccConfiguredAddresses.RootDsAddress)))
	// eUICC Manufacturer Label
	var EUICCManufacturerLabelContent string
	// 仅在获取到 EidValue 时进行切片
	if eum := GetEUM(ChipInfo.EidValue); eum != nil {
		EUICCManufacturerLabelContent = fmt.Sprint(
			"Manufacturer: ",
			eum.Manufacturer,
			" ",
			CountryCodeToEmoji(eum.Country),
		)
	}
	if EUICCManufacturerLabelContent == "" {
		EUICCManufacturerLabel.ParseMarkdown("Manufacturer: Unknown")
	} else {
		EUICCManufacturerLabel.ParseMarkdown(EUICCManufacturerLabelContent)
	}
	// EUICCInfo2 entry
	bytes, err := json.MarshalIndent(ChipInfo.EUICCInfo2, "", "  ")
	if err != nil {
		ShowLpacErrDialog(fmt.Errorf("chip Info: failed to decode EUICCInfo2\n%s", err))
	}
	EuiccInfo2Entry.SetText(string(bytes))
	// 计算剩余空间
	freeSpace := float64(ChipInfo.EUICCInfo2.ExtCardResource.FreeNonVolatileMemory) / 1024
	FreeSpaceLabel.SetText(fmt.Sprintf("Free space: %.2f KB", math.Round(freeSpace*100)/100))

	CopyEidButton.Show()
	SetDefaultSmdpButton.Show()
	EuiccInfo2Entry.Show()
	ViewCertInfoButton.Show()
	EUICCManufacturerLabel.Show()
	CopyEuiccInfo2Button.Show()
}

func RefreshApduDriver() {
	var err error
	ApduDrivers, err = LpacDriverApduList()
	if err != nil {
		ShowLpacErrDialog(err)
	}
	var options []string
	for _, d := range ApduDrivers {
		options = append(options, d.Name)
	}
	ApduDriverSelect.SetOptions(options)
	ApduDriverSelect.ClearSelected()
	ConfigInstance.DriverIFID = ""
	ApduDriverSelect.Refresh()
}

func OpenLog() {
	if err := OpenProgram(ConfigInstance.LogDir); err != nil {
		ShowLpacErrDialog(err)
	}
}

func OpenProgram(name string) error {
	var launcher string
	switch runtime.GOOS {
	case "windows":
		launcher = "explorer"
	case "darwin":
		launcher = "open"
	case "linux":
		launcher = "xdg-open"
	}
	if launcher == "" {
		return fmt.Errorf("unsupported platform, please open log file manually")
	}
	return exec.Command(launcher, name).Start()
}

func Refresh() {
	if ConfigInstance.DriverIFID == "" {
		ShowSelectCardReaderDialog()
		return
	}
	RefreshProfile()
	RefreshNotification()
	RefreshChipInfo()
	RefreshNeeded = false
}

func UpdateStatusBarListener() {
	for {
		status := <-StatusChan
		switch status {
		case StatusProcess:
			StatusLabel.SetText("Processing...")
			StatusProcessBar.Start()
			StatusProcessBar.Show()
			continue
		case StatusReady:
			StatusLabel.SetText("Ready.")
			StatusProcessBar.Stop()
			StatusProcessBar.Hide()
			continue
		}
	}
}

func LockButtonListener() {
	buttons := []*widget.Button{
		RefreshButton, DownloadButton, DiscoveryButton,
		SetNicknameButton, SwitchStateButton, DeleteProfileButton,
		ProcessNotificationButton, RemoveNotificationButton,
		SetDefaultSmdpButton, ApduDriverRefreshButton,
	}
	checks := []*widget.Check{
		ProfileMaskCheck, NotificationMaskCheck,
	}
	for {
		lock := <-LockButtonChan
		if lock {
			for _, button := range buttons {
				button.Disable()
			}
			for _, check := range checks {
				check.Disable()
			}
			ApduDriverSelect.Disable()
		} else {
			for _, button := range buttons {
				button.Enable()
			}
			for _, check := range checks {
				check.Enable()
			}
			ApduDriverSelect.Enable()
		}
	}
}

func SetDriverIFID(name string) {
	for _, d := range ApduDrivers {
		if name == d.Name {
			// 未选择过读卡器
			if ConfigInstance.DriverIFID == "" {
				ConfigInstance.DriverIFID = d.Env
			} else {
				// 选择过读卡器，要求刷新
				ConfigInstance.DriverIFID = d.Env
				RefreshNeeded = true
			}
		}
	}
}
