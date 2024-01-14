package main

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/atotto/clipboard"
	"time"
)

var StatusProcessBar *widget.ProgressBarInfinite
var StatusLabel *widget.Label
var SetNicknameButton *widget.Button
var DownloadButton *widget.Button
var DiscoveryButton *widget.Button
var DeleteButton *widget.Button
var EnableButton *widget.Button
var ProfileList *widget.List
var NotificationList *widget.List

var ProfileListTitle *widget.Label
var NotificationListTitle *widget.Label

var FreeSpaceLabel *widget.Label
var OpenLogButton *widget.Button
var RefreshButton *widget.Button
var ProcessNotificationButton *widget.Button
var RemoveNotificationButton *widget.Button

var EidLabel *widget.Label
var DefaultDpAddressLabel *widget.Label
var RootDsAddressLabel *widget.Label
var EuiccInfo2TextGrid *widget.TextGrid
var CopyEidButton *widget.Button
var SetDefaultSmdpButton *widget.Button

var ApduDriverSelect *widget.Select
var ApduDriverRefreshButton *widget.Button

var Tabs *container.AppTabs
var ProfileTab *container.TabItem
var NotificationTab *container.TabItem
var ChipInfoTab *container.TabItem
var AboutTab *container.TabItem

func InitWidgets() {
	StatusProcessBar = widget.NewProgressBarInfinite()
	StatusProcessBar.Stop()
	StatusProcessBar.Hide()

	StatusLabel = widget.NewLabel("Ready.")

	DownloadButton = widget.NewButton("Download", downloadButtonFunc)
	DownloadButton.SetIcon(theme.DownloadIcon())

	DiscoveryButton = widget.NewButton("Discovery", discoveryButtonnFunc)
	DiscoveryButton.SetIcon(theme.SearchIcon())

	SetNicknameButton = widget.NewButton("Nickname", setNicknameButtonFunc)
	SetNicknameButton.SetIcon(theme.DocumentCreateIcon())

	DeleteButton = widget.NewButton("Delete", deleteButtonFunc)
	DeleteButton.SetIcon(theme.DeleteIcon())

	EnableButton = widget.NewButton("Enable", enableButtonFunc)
	EnableButton.SetIcon(theme.ConfirmIcon())

	ProfileList = initProfileList()
	ProfileList.OnSelected = func(id widget.ListItemID) {
		SelectedProfile = id
	}

	ProfileListTitle = widget.NewLabel(fmt.Sprintf("%s\t\t\t\t\t\t%s\t\t%s\t\t\t\t%s", "ICCID", "Profile State", "Provider", "Nickname"))
	ProfileListTitle.TextStyle = fyne.TextStyle{Bold: true}

	NotificationList = initNotificationList()
	NotificationList.OnSelected = func(id widget.ListItemID) {
		SelectedNotification = id
	}

	NotificationListTitle = widget.NewLabel(fmt.Sprintf("%s\t\t%s\t\t\t\t\t\t%s\t\t\t%s", "Seq", "ICCID", "Operation", "Server"))
	NotificationListTitle.TextStyle = fyne.TextStyle{Bold: true}

	ProcessNotificationButton = widget.NewButton("Process", processNotificationButtonFunc)
	ProcessNotificationButton.SetIcon(theme.MediaPlayIcon())

	RemoveNotificationButton = widget.NewButton("Remove", removeNotificationButtonFunc)
	RemoveNotificationButton.SetIcon(theme.DeleteIcon())

	FreeSpaceLabel = widget.NewLabel("")

	OpenLogButton = widget.NewButton("Open Log", OpenLog)
	OpenLogButton.SetIcon(theme.FolderOpenIcon())

	RefreshButton = widget.NewButton("Refresh", Refresh)
	RefreshButton.SetIcon(theme.ViewRefreshIcon())

	EidLabel = widget.NewLabel("")
	DefaultDpAddressLabel = widget.NewLabel("")
	RootDsAddressLabel = widget.NewLabel("")
	EuiccInfo2TextGrid = widget.NewTextGrid()
	CopyEidButton = widget.NewButton("Copy", copyEidButtonFunc)
	CopyEidButton.SetIcon(theme.ContentCopyIcon())
	CopyEidButton.Hide()
	SetDefaultSmdpButton = widget.NewButton("", setDefaultSmdpButtonFunc)
	SetDefaultSmdpButton.SetIcon(theme.DocumentCreateIcon())
	SetDefaultSmdpButton.Hide()
	ApduDriverSelect = widget.NewSelect([]string{}, func(s string) {
		SetDriverIfid(s)
	})
	ApduDriverRefreshButton = widget.NewButton("", func() {
		RefreshApduDriver()
	})
	ApduDriverRefreshButton.SetIcon(theme.SearchReplaceIcon())
}

func downloadButtonFunc() {
	if ConfigInstance.DriverIFID == "" {
		SelectCardReaderDialog()
		return
	}
	d := InitDownloadDialog()
	d.Show()
}

func discoveryButtonnFunc() {
	if ConfigInstance.DriverIFID == "" {
		SelectCardReaderDialog()
		return
	}
	data, err := LpacProfileDiscovery()
	if err != nil {
		ErrDialog(err)
		return
	}
	if len(data) != 0 {
		var d *dialog.CustomDialog
		selectedProfile := -1
		foundLabel := widget.NewLabel("")
		if len(data) == 1 {
			foundLabel.SetText(fmt.Sprintf("%d profile found.", len(data)))
		} else {
			foundLabel.SetText(fmt.Sprintf("%d profiles found.", len(data)))
		}
		discoveredEsimListTitle := widget.NewLabel("EventID\t\tRSP Server Address")
		discoveredEsimListTitle.TextStyle = fyne.TextStyle{Bold: true}
		discoveredEsimList := widget.NewList(func() int {
			return len(data)
		}, func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.TextStyle = fyne.TextStyle{Monospace: true}
			return label
		}, func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(fmt.Sprintf("%-5s\t\t%s", data[i].EventID, data[i].RspServerAddres))
		})
		discoveredEsimList.OnSelected = func(id widget.ListItemID) {
			selectedProfile = id
		}
		downloadButton := widget.NewButton("Download", func() {
			if selectedProfile == -1 {
				SelectItemDialog()
			} else {
				d.Hide()
				LpacProfileDownload(PullInfo{
					SMDP:        data[selectedProfile].RspServerAddres,
					MatchID:     "",
					ConfirmCode: "",
					IMEI:        "",
				})
			}
		})
		downloadButton.Importance = widget.HighImportance
		downloadButton.SetIcon(theme.DownloadIcon())
		dismissButton := widget.NewButton("Dismiss", func() {
			d.Hide()
		})
		dismissButton.SetIcon(theme.CancelIcon())
		content := container.NewBorder(
			foundLabel,
			nil,
			nil,
			nil,
			container.NewBorder(
				discoveredEsimListTitle,
				nil,
				nil,
				nil,
				discoveredEsimList))
		d = dialog.NewCustomWithoutButtons("Result", content, WMain)
		d.Resize(fyne.Size{
			Width:  550,
			Height: 400,
		})
		d.SetButtons([]fyne.CanvasObject{dismissButton, downloadButton})
		d.Show()

	} else {
		d := dialog.NewInformation("Result", "No eSIM profile found.", WMain)
		d.Show()
	}
}

func setNicknameButtonFunc() {
	if ConfigInstance.DriverIFID == "" {
		SelectCardReaderDialog()
		return
	}
	if RefreshProfileNeeded {
		RefreshNeededDialog()
		return
	}
	if SelectedProfile < 0 || SelectedProfile >= len(Profiles) {
		SelectItemDialog()
		return
	}
	d := InitSetNicknameDialog()
	d.Show()
}

func deleteButtonFunc() {
	if ConfigInstance.DriverIFID == "" {
		SelectCardReaderDialog()
		return
	}
	if RefreshProfileNeeded {
		RefreshNeededDialog()
		return
	}
	if SelectedProfile < 0 || SelectedProfile >= len(Profiles) {
		SelectItemDialog()
		return
	}
	if Profiles[SelectedProfile].ProfileState == "enabled" {
		d := dialog.NewInformation("Hint", "You should disable the profile before deleting it.", WMain)
		d.Resize(fyne.Size{
			Width:  360,
			Height: 170,
		})
		d.Show()
		return
	}
	dialogText := fmt.Sprintf("Are you sure you want to delete this profile?\n\n%s\t\t%s",
		Profiles[SelectedProfile].Iccid,
		Profiles[SelectedProfile].ServiceProviderName)
	if Profiles[SelectedProfile].ProfileNickname != nil {
		dialogText += fmt.Sprintf("\t\t%s\n\n", Profiles[SelectedProfile].ProfileNickname)
	} else {
		dialogText += "\n\n"
	}
	d := dialog.NewConfirm("Confirm",
		dialogText,
		func(b bool) {
			if b {
				if err := LpacProfileDelete(Profiles[SelectedProfile].Iccid); err != nil {
					ErrDialog(err)
				}
				RefreshProfile()
				RefreshNotification()
				RefreshChipInfo()
			} else {
				return
			}
		}, WMain)
	d.Show()
}

func enableButtonFunc() {
	if ConfigInstance.DriverIFID == "" {
		SelectCardReaderDialog()
		return
	}
	if RefreshProfileNeeded {
		RefreshNeededDialog()
		return
	}
	if SelectedProfile < 0 || SelectedProfile >= len(Profiles) {
		SelectItemDialog()
		return
	}
	if err := LpacProfileEnable(Profiles[SelectedProfile].Iccid); err != nil {
		ErrDialog(err)
	}
	RefreshProfile()
	RefreshNotification()
	RefreshChipInfo()
}

func processNotificationButtonFunc() {
	if ConfigInstance.DriverIFID == "" {
		SelectCardReaderDialog()
		return
	}
	if RefreshNotificationNeeded {
		RefreshNeededDialog()
		return
	}
	if SelectedNotification < 0 || SelectedNotification >= len(Notifications) {
		SelectItemDialog()
		return
	}
	seq := Notifications[SelectedNotification].SeqNumber
	if err := LpacNotificationProcess(seq); err != nil {
		ErrDialog(err)
		RefreshNotification()
		// RefreshChipInfo()
	} else {
		dialogText := fmt.Sprintf("Successfully processed notification.\nDo you want to remove this notification now?\n\n%d\t\t%s\t\t%s\t\t%s\n\n",
			Notifications[SelectedNotification].SeqNumber,
			Notifications[SelectedNotification].Iccid,
			Notifications[SelectedNotification].ProfileManagementOperation,
			Notifications[SelectedNotification].NotificationAddress)
		d := dialog.NewConfirm("Remove Notification",
			dialogText,
			func(b bool) {
				if b {
					if err := LpacNotificationRemove(seq); err != nil {
						ErrDialog(err)
					}
				}
				RefreshNotification()
			}, WMain)
		d.Show()
	}
}

func removeNotificationButtonFunc() {
	if ConfigInstance.DriverIFID == "" {
		SelectCardReaderDialog()
		return
	}
	if RefreshNotificationNeeded {
		RefreshNeededDialog()
		return
	}
	if SelectedNotification < 0 || SelectedNotification >= len(Notifications) {
		SelectItemDialog()
		return
	}
	dialogText := fmt.Sprintf("Are you sure you want to remove this notification?\n\n%d\t\t%s\t\t%s\t\t%s\n\n",
		Notifications[SelectedNotification].SeqNumber,
		Notifications[SelectedNotification].Iccid,
		Notifications[SelectedNotification].ProfileManagementOperation,
		Notifications[SelectedNotification].NotificationAddress)
	d := dialog.NewConfirm("Confirm",
		dialogText,
		func(b bool) {
			if b {
				if err := LpacNotificationRemove(Notifications[SelectedNotification].SeqNumber); err != nil {
					ErrDialog(err)
				}
				RefreshNotification()
				RefreshChipInfo()
			} else {
				return
			}
		}, WMain)
	d.Show()
}

func copyEidButtonFunc() {
	err := clipboard.WriteAll(ChipInfo.EidValue)
	if err != nil {
		ErrDialog(err)
	} else {
		go func() {
			CopyEidButton.SetText("Copied!")
			time.Sleep(2 * time.Second)
			CopyEidButton.SetText("Copy")
		}()
	}
}

func setDefaultSmdpButtonFunc() {
	if ConfigInstance.DriverIFID == "" {
		SelectCardReaderDialog()
		return
	}
	if RefreshChipInfoNeeded {
		RefreshNeededDialog()
		return
	}
	d := InitSetDefaultSmdpDialog()
	d.Show()
}

func initProfileList() *widget.List {
	return widget.NewList(
		func() int {
			return len(Profiles)
		},
		func() fyne.CanvasObject {
			return widget.NewRichText()
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			var text string
			text = fmt.Sprintf("%s\t\t", Profiles[i].Iccid)
			if Profiles[i].ProfileState == "enabled" {
				text += fmt.Sprintf("*%s*", Profiles[i].ProfileState)
			} else {
				text += fmt.Sprintf("%s", Profiles[i].ProfileState)
			}
			text += fmt.Sprintf("\t\t\t%s", Profiles[i].ServiceProviderName)
			if Profiles[i].ProfileNickname != nil {
				// fyne tab space 为 5
				tabNum := 5 - len(Profiles[i].ServiceProviderName)/5
				for x := 1; x <= tabNum; x++ {
					text += "\t"
				}
				text += fmt.Sprintf("%s", Profiles[i].ProfileNickname)
			}
			text = "`" + text + "`"
			o.(*widget.RichText).ParseMarkdown(text)
		})
}

func initNotificationList() *widget.List {
	return widget.NewList(
		func() int {
			return len(Notifications)
		},
		func() fyne.CanvasObject {
			return widget.NewRichText()
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			text := fmt.Sprintf("%-5d\t%s\t\t%s",
				Notifications[i].SeqNumber,
				Notifications[i].Iccid,
				Notifications[i].ProfileManagementOperation)
			text += fmt.Sprintf("\t\t\t%s", Notifications[i].NotificationAddress)
			text = "`" + text + "`"
			o.(*widget.RichText).ParseMarkdown(text)
		})
}
