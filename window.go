package main

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"image/color"
)

var WMain fyne.Window

func InitMainWindow() fyne.Window {
	w := App.NewWindow("EasyLPAC")
	w.Resize(fyne.Size{
		Width:  850,
		Height: 545,
	})
	w.SetMaster()

	statusBar := container.NewGridWrap(fyne.Size{
		Width:  100,
		Height: 36,
	}, StatusLabel, StatusProcessBar)

	spacer := canvas.NewRectangle(color.Transparent)
	spacer.SetMinSize(fyne.NewSize(1, 36))

	topToolBar := container.NewBorder(
		layout.NewSpacer(),
		nil,
		container.New(layout.NewHBoxLayout(), OpenLogButton, spacer, RefreshButton, spacer),
		FreeSpaceLabel,
		container.NewBorder(
			nil,
			nil,
			widget.NewLabel("Card Reader:"),
			nil,
			container.NewHBox(container.NewGridWrap(fyne.Size{
				Width:  260,
				Height: 36,
			}, ApduDriverSelect), ApduDriverRefreshButton)),
	)

	profileTabContent := container.NewBorder(
		topToolBar,
		container.NewBorder(
			nil,
			nil,
			nil,
			container.NewHBox(ProfileMaskCheck, DownloadButton, spacer, DiscoveryButton, spacer, SetNicknameButton, spacer, SwitchStateButton, spacer, DeleteButton),
			statusBar),
		nil,
		nil,
		container.NewBorder(
			ProfileListTitle,
			nil,
			nil,
			nil,
			ProfileList))
	ProfileTab = container.NewTabItem("Profile", profileTabContent)

	notificationTabContent := container.NewBorder(
		topToolBar,
		container.NewBorder(
			nil,
			nil,
			nil,
			container.NewHBox(NotificationMaskCheck, spacer, ProcessNotificationButton, spacer, RemoveNotificationButton),
			statusBar),
		nil,
		nil,
		container.NewBorder(
			NotificationListTitle,
			nil,
			nil,
			nil,
			NotificationList))
	NotificationTab = container.NewTabItem("Notification", notificationTabContent)

	chipInfoTabContent := container.NewBorder(
		topToolBar,
		container.NewBorder(
			nil,
			nil,
			nil,
			nil,
			statusBar),
		nil,
		nil,
		container.NewBorder(
			container.NewVBox(container.NewHBox(EidLabel, CopyEidButton), container.NewHBox(DefaultDpAddressLabel, SetDefaultSmdpButton), RootDsAddressLabel),
			nil,
			nil,
			nil,
			container.NewScroll(EuiccInfo2Entry),
		))
	ChipInfoTab = container.NewTabItem("Chip Info", chipInfoTabContent)

	thankstoText := widget.NewRichTextFromMarkdown(`
# Thanks to

[lpac](https://github.com/estkme-group/lpac) C-based eUICC LPA

[fyne](https://github.com/fyne-io/fyne) Material Design GUI toolkit`)

	aboutText := widget.NewRichTextFromMarkdown(`
# EasyLPAC

lpac GUI Frontend

[Github](https://github.com/creamlike1024/EasyLPAC) Repo `)

	aboutTabContent := container.NewBorder(
		layout.NewSpacer(),
		layout.NewSpacer(),
		layout.NewSpacer(),
		layout.NewSpacer(),
		container.NewCenter(container.NewVBox(thankstoText, aboutText)))
	AboutTab = container.NewTabItem("About", aboutTabContent)

	Tabs = container.NewAppTabs(ProfileTab, NotificationTab, ChipInfoTab, AboutTab)

	w.SetContent(Tabs)

	return w
}

func InitDownloadDialog() dialog.Dialog {
	smdp := &widget.Entry{PlaceHolder: "Leave it empty to use default SM-DP+"}
	matchID := &widget.Entry{PlaceHolder: "Activation code. Optional"}
	confirmCode := &widget.Entry{PlaceHolder: "Optional"}
	imei := &widget.Entry{PlaceHolder: "The IMEI sent to SM-DP+. Optional"}

	form := []*widget.FormItem{
		{Text: "SM-DP+", Widget: smdp},
		{Text: "Matching ID", Widget: matchID},
		{Text: "Confirm Code", Widget: confirmCode},
		{Text: "IMEI", Widget: imei},
	}

	d := dialog.NewForm("Download", "Submit", "Cancel", form, func(b bool) {
		if b {
			pullConfig := PullInfo{
				SMDP:        smdp.Text,
				MatchID:     matchID.Text,
				ConfirmCode: confirmCode.Text,
				IMEI:        imei.Text,
			}
			LpacProfileDownload(pullConfig)
			RefreshProfile()
			RefreshNotification()
			RefreshChipInfo()
		}
	}, WMain)
	d.Resize(fyne.Size{
		Width:  500,
		Height: 300,
	})
	return d
}

func InitSetNicknameDialog() dialog.Dialog {
	entry := &widget.Entry{PlaceHolder: "Leave it empty to remove nickname", TextStyle: fyne.TextStyle{Monospace: true}}
	form := []*widget.FormItem{
		{Text: "Nickname", Widget: entry},
	}
	d := dialog.NewForm("Set Nickname", "Submit", "Cancel", form, func(b bool) {
		if b {
			if err := LpacProfileNickname(Profiles[SelectedProfile].Iccid, entry.Text); err != nil {
				ErrDialog(err)
			}
			RefreshProfile()
		}
	}, WMain)
	d.Resize(fyne.Size{
		Width:  400,
		Height: 200,
	})
	return d
}

func InitSetDefaultSmdpDialog() dialog.Dialog {
	entry := &widget.Entry{PlaceHolder: "Leave it empty to remove default SM-DP+ setting"}
	form := []*widget.FormItem{
		{Text: "Default SM-DP+", Widget: entry},
	}
	d := dialog.NewForm("Set Default SM-DP+", "Submit", "Cancel", form, func(b bool) {
		if b {
			if err := LpacChipDefaultSmdp(entry.Text); err != nil {
				ErrDialog(err)
			}
			RefreshChipInfo()
		}
	}, WMain)
	d.Resize(fyne.Size{
		Width:  510,
		Height: 200,
	})
	return d
}

func ErrDialog(err error) {
	l := &widget.Label{Text: fmt.Sprintf("%v", err), TextStyle: fyne.TextStyle{Monospace: true}}
	content := container.NewVBox(container.NewCenter(container.NewHBox(widget.NewIcon(theme.ErrorIcon()), widget.NewLabel("lpac error:"))), container.NewCenter(l))
	d := dialog.NewCustom("Error", "OK", content, WMain)
	d.Show()
}

func SelectItemDialog() {
	d := dialog.NewInformation("Info", "Please select a item.", WMain)
	d.Resize(fyne.Size{
		Width:  220,
		Height: 160,
	})
	d.Show()
}

func SelectCardReaderDialog() {
	d := dialog.NewInformation("Info", "Please select a card reader.", WMain)
	d.Show()
}

func RefreshNeededDialog() {
	d := dialog.NewInformation("Info", "Card reader changed.\nPlease refresh before proceeding.", WMain)
	d.Show()
}
