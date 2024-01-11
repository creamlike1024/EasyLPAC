package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
)

const StatusProcess = 1
const StatusReady = 0

var SelectedProfile int
var SelectedNotification int

var StatusChan chan int
var LockButtonChan chan bool

func RefreshProfile() {
	var err error
	Profiles, err = LpacProfileList()
	if err != nil {
		ErrDialog(err)
	}
	// 刷新 List
	ProfileList.Refresh()
	ProfileList.UnselectAll()
	SelectedProfile = -1
}

func RefreshNotification() {
	var err error
	Notifications, err = LpacNotificationList()
	if err != nil {
		ErrDialog(err)
	}
	// 刷新 List
	NotificationList.Refresh()
	NotificationList.UnselectAll()
	SelectedNotification = -1
}

func RefreshChipInfo() {
	var err error
	ChipInfo, err = LpacChipInfo()
	if err != nil {
		ErrDialog(err)
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
	DefaultDpAddressLabel.SetText(fmt.Sprintf("Default DP Address:  %s", convertToString(ChipInfo.EuiccConfiguredAddresses.DefaultDpAddress)))
	RootDsAddressLabel.SetText(fmt.Sprintf("Root DS Address:  %s", convertToString(ChipInfo.EuiccConfiguredAddresses.RootDsAddress)))
	bytes, err := json.MarshalIndent(ChipInfo.EUICCInfo2, "", "  ")
	if err != nil {
		ErrDialog(fmt.Errorf("chip Info: failed to decode EUICCInfo2\n%s", err))
	}
	CopyEidButton.Show()

	EuiccInfo2TextGrid.SetText(string(bytes))

	// 计算剩余空间
	freeSpace := float32(ChipInfo.EUICCInfo2.ExtCardResource.FreeNonVolatileMemory) / 1024
	FreeSpaceLabel.SetText(fmt.Sprintf("Free space: %.2f KB", freeSpace))
}

func OpenLog() {
	var err error

	switch runtime.GOOS {
	case "windows":
		err = exec.Command("explorer", ConfigInstance.logDir).Start()
	case "darwin":
		err = exec.Command("open", "-R", ConfigInstance.logDir).Start()
	case "linux":
		err = exec.Command("xdg-open", ConfigInstance.logDir).Start()
	default:
		err = fmt.Errorf("unsupported platform, please open log file manually")
		ErrDialog(err)
	}

	if err != nil {
		ErrDialog(err)
	}
}

func UpdateStatusBar() {
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

func LockButton() {
	for {
		lock := <-LockButtonChan
		if lock {
			DownloadButton.Disable()
			DiscoveryButton.Disable()
			SetNicknameButton.Disable()
			RefreshProfileButton.Disable()
			RefreshNotificationButton.Disable()
			EnableButton.Disable()
			DeleteButton.Disable()
			ProcessNotificationButton.Disable()
			RemoveNotificationButton.Disable()
		} else {
			DownloadButton.Enable()
			DiscoveryButton.Enable()
			SetNicknameButton.Enable()
			RefreshProfileButton.Enable()
			RefreshNotificationButton.Enable()
			EnableButton.Enable()
			DeleteButton.Enable()
			ProcessNotificationButton.Enable()
			RemoveNotificationButton.Enable()
		}
	}
}
