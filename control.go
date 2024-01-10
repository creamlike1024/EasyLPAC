package main

import (
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
	chipInfo, err := LpacChipInfo()
	if err != nil {
		ErrDialog(err)
	}
	// 计算剩余空间
	freeSpace := float32(chipInfo.Euiccinfo2.FreeNvram) / 1024
	FreeSpaceLabel.SetText(fmt.Sprintf("Free space: %.2f kb", freeSpace))
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

	chipInfo, err := LpacChipInfo()
	if err != nil {
		ErrDialog(err)
	}
	// 计算剩余空间
	freeSpace := float32(chipInfo.Euiccinfo2.FreeNvram) / 1024
	FreeSpaceLabel.SetText(fmt.Sprintf("Free space: %.2f kb", freeSpace))
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
			RefreshProfileButton.Disable()
			RefreshNotificationButton.Disable()
			EnableButton.Disable()
			DeleteButton.Disable()
			ProcessNotificationButton.Disable()
			RemoveNotificationButton.Disable()
		} else {
			DownloadButton.Enable()
			RefreshProfileButton.Enable()
			RefreshNotificationButton.Enable()
			EnableButton.Enable()
			DeleteButton.Enable()
			ProcessNotificationButton.Enable()
			RemoveNotificationButton.Enable()
		}
	}
}
