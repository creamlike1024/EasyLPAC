package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"image/color"
)

var WMain fyne.Window

func InitMainWindow() fyne.Window {
	w := App.NewWindow("EasyLPAC")
	w.Resize(fyne.Size{
		Width:  820,
		Height: 515,
	})
	w.SetMaster()

	statusBar := container.NewGridWrap(fyne.Size{
		Width:  120,
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
			container.NewHBox(DownloadButton, spacer, DiscoveryButton, spacer, SetNicknameButton, spacer, EnableButton, spacer, DeleteButton),
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
			container.NewHBox(ProcessNotificationButton, spacer, RemoveNotificationButton),
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
			container.NewScroll(EuiccInfo2TextGrid)))
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
	smdp := widget.NewEntry()
	smdp.PlaceHolder = "Leave it empty to use default SM-DP+"
	matchID := widget.NewEntry()
	matchID.PlaceHolder = "Activation code. Optional"
	confirmCode := widget.NewEntry()
	confirmCode.PlaceHolder = "Optional"
	imei := widget.NewEntry()
	imei.PlaceHolder = "The IMEI sent to SM-DP+. Optional"

	form := []*widget.FormItem{
		{Text: "SM-DP+", Widget: smdp},
		{Text: "Matching ID", Widget: matchID},
		{Text: "Confirm Code", Widget: confirmCode},
		{Text: "IMEI", Widget: imei},
	}

	d := dialog.NewForm("Download", "Submit", "Cancel", form, func(b bool) {
		if b {
			var pullConfig PullInfo
			pullConfig.SMDP = smdp.Text
			pullConfig.MatchID = matchID.Text
			pullConfig.ConfirmCode = confirmCode.Text
			pullConfig.IMEI = imei.Text
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
	entry := widget.NewEntry()
	entry.SetPlaceHolder("Leave it empty to remove nickname")
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
	entry := widget.NewEntry()
	entry.SetPlaceHolder("Leave it empty to remove default SM-DP+ setting")
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
	d := dialog.NewError(err, WMain)
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
