package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"runtime"
	"sort"

	fyne "fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const StatusProcess = 1
const StatusReady = 0
const Unselected = -1

var SelectedProfile = Unselected
var SelectedNotification = Unselected

var RefreshNeeded = true
var ProfileMaskNeeded bool
var NotificationMaskNeeded bool
var ProfileStateAllowDisable bool

var StatusChan = make(chan int)
var LockButtonChan = make(chan bool)

func RefreshProfile() error {
	var err error
	Profiles, err = LpacProfileList()
	if err != nil {
		return err
	}
	fyne.Do(func() {
		ProfileList.Refresh()
		ProfileList.UnselectAll()
		SwitchStateButton.SetText(TR.Trans("label.switch_state_button_enable"))
		SwitchStateButton.SetIcon(theme.ConfirmIcon())
	})
	return nil
}

func RefreshNotification() error {
	var err error
	Notifications, err = LpacNotificationList()
	if err != nil {
		return err
	}
	sort.Slice(Notifications, func(i, j int) bool {
		return Notifications[i].SeqNumber < Notifications[j].SeqNumber
	})
	fyne.Do(func() {
		NotificationList.Refresh()
		NotificationList.UnselectAll()
	})
	return nil
}

func RefreshChipInfo() error {
	var err error
	ChipInfo, err = LpacChipInfo()
	if err != nil {
		return err
	}
	if ChipInfo == nil {
		return nil
	}

	convertToString := func(value interface{}) string {
		if value == nil {
			return TR.Trans("label.not_set")
		}
		if str, ok := value.(string); ok {
			return str
		}
		return TR.Trans("label.not_set")
	}

	fyne.Do(func() {
		EidLabel.SetText(fmt.Sprintf(TR.Trans("label.info_eid")+" %s", ChipInfo.EidValue))
		DefaultDpAddressLabel.SetText(fmt.Sprintf(TR.Trans("label.default_smdp_address")+"  %s", convertToString(ChipInfo.EuiccConfiguredAddresses.DefaultDpAddress)))
		RootDsAddressLabel.SetText(fmt.Sprintf(TR.Trans("label.root_smds_address")+"  %s", convertToString(ChipInfo.EuiccConfiguredAddresses.RootDsAddress)))
		if eum := GetEUM(ChipInfo.EidValue); eum != nil {
			manufacturer := fmt.Sprint(eum.Manufacturer, " ", CountryCodeToEmoji(eum.Country))
			EUICCManufacturerLabel.SetText(TR.Trans("label.manufacturer") + " " + manufacturer)
		} else {
			EUICCManufacturerLabel.SetText(TR.Trans("label.manufacturer_unknown"))
		}
		bytes, err := json.MarshalIndent(ChipInfo.EUICCInfo2, "", "  ")
		if err != nil {
			ShowLpacErrDialog(fmt.Errorf(TR.Trans("message.failed_to_decode_euiccinfo2")+"\n%s", err))
		}
		EuiccInfo2Entry.SetText(string(bytes))
		freeSpace := float64(ChipInfo.EUICCInfo2.ExtCardResource.FreeNonVolatileMemory) / 1024
		FreeSpaceLabel.SetText(fmt.Sprintf(TR.Trans("label.free_space")+" %.2f KiB", math.Round(freeSpace*100)/100))

		CopyEidButton.Show()
		SetDefaultSmdpButton.Show()
		EuiccInfo2Entry.Show()
		ViewCertInfoButton.Show()
		EUICCManufacturerLabel.Show()
		CopyEuiccInfo2Button.Show()
	})
	return nil
}

// DiscoverDrivers queries available APDU drivers from lpac
func DiscoverDrivers() error {
	var err error
	AvailableDrivers, err = LpacDriverList()
	if err != nil {
		return err
	}
	return nil
}

func OpenLog() {
	if err := OpenProgram(ConfigInstance.LogDir); err != nil {
		d := dialog.NewError(err, WMain)
		d.Show()
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
		return fmt.Errorf("unsupported platform, failed to open")
	}
	return exec.Command(launcher, name).Start()
}

func Refresh() {
	if !isApduConfigured() {
		showApduNotConfiguredDialog()
		return
	}
	err := RefreshProfile()
	if err != nil {
		ShowLpacErrDialog(err)
		return
	}
	err = RefreshNotification()
	if err != nil {
		ShowLpacErrDialog(err)
		return
	}
	err = RefreshChipInfo()
	if err != nil {
		ShowLpacErrDialog(err)
		return
	}
	RefreshNeeded = false
}

func UpdateStatusBarListener() {
	for {
		status := <-StatusChan
		fyne.Do(func() {
			switch status {
			case StatusProcess:
				StatusLabel.SetText(TR.Trans("label.status_processing"))
				StatusProcessBar.Start()
				StatusProcessBar.Show()
			case StatusReady:
				StatusLabel.SetText(TR.Trans("label.status_ready"))
				StatusProcessBar.Stop()
				StatusProcessBar.Hide()
			}
		})
	}
}

func LockButtonListener() {
	buttons := []*widget.Button{
		RefreshButton, DownloadButton, SetNicknameButton, SwitchStateButton, DeleteProfileButton,
		ProcessNotificationButton, ProcessAllNotificationButton, RemoveNotificationButton, BatchRemoveNotificationButton,
		SetDefaultSmdpButton, DeviceSelectRefresh,
	}
	checks := []*widget.Check{
		ProfileMaskCheck, NotificationMaskCheck,
	}
	for {
		lock := <-LockButtonChan
		fyne.Do(func() {
			if lock {
				for _, button := range buttons {
					if button != nil {
						button.Disable()
					}
				}
				for _, check := range checks {
					if check != nil {
						check.Disable()
					}
				}
				if ApduBackendSelect != nil {
					ApduBackendSelect.Disable()
				}
				if DeviceSelect != nil {
					DeviceSelect.Disable()
				}
				if DeviceEntry != nil {
					DeviceEntry.Disable()
				}
				if UimSlotEntry != nil {
					UimSlotEntry.Disable()
				}
			} else {
				for _, button := range buttons {
					if button != nil {
						button.Enable()
					}
				}
				for _, check := range checks {
					if check != nil {
						check.Enable()
					}
				}
				if ApduBackendSelect != nil {
					ApduBackendSelect.Enable()
				}
				if DeviceSelect != nil {
					DeviceSelect.Enable()
				}
				if DeviceEntry != nil {
					DeviceEntry.Enable()
				}
				if UimSlotEntry != nil {
					UimSlotEntry.Enable()
				}
			}
		})
	}
}
