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
		Width:  800,
		Height: 500,
	})
	w.SetMaster()

	statusBar := container.NewGridWrap(fyne.Size{
		Width:  120,
		Height: 36,
	}, StatusLabel, StatusProcessBar)

	spacer := canvas.NewRectangle(color.Transparent)
	spacer.SetMinSize(fyne.NewSize(1, 36))

	profileTabContent := container.NewBorder(
		container.NewBorder(
			layout.NewSpacer(),
			nil,
			container.New(layout.NewHBoxLayout(), OpenLogButton, spacer, RefreshProfileButton),
			FreeSpaceLabel),
		container.NewBorder(
			nil,
			nil,
			nil,
			container.NewHBox(DownloadButton, spacer, EnableButton, spacer, DeleteButton),
			statusBar),
		nil,
		nil,
		container.NewBorder(
			ProfileListTitle,
			nil,
			nil,
			nil,
			ProfileList))
	profileTab := container.NewTabItem("Profile", profileTabContent)

	notificationTabContent := container.NewBorder(
		container.NewBorder(
			layout.NewSpacer(),
			nil,
			container.NewHBox(OpenLogButton, spacer, RefreshNotificationButton),
			FreeSpaceLabel),
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
	notificationTab := container.NewTabItem("Notification", notificationTabContent)

	chipInfoTabContent := container.NewBorder(
		container.NewBorder(
			layout.NewSpacer(),
			nil,
			container.NewHBox(OpenLogButton, spacer, RefreshChipInfoButton),
			FreeSpaceLabel),
		container.NewBorder(
			nil,
			nil,
			nil,
			nil,
			statusBar),
		nil,
		nil,
		container.NewBorder(
			container.NewVBox(container.NewHBox(EidLabel, CopyEidButton), DefaultDpAddressLabel, RootDsAddressLabel),
			nil,
			nil,
			nil,
			container.NewScroll(EuiccInfo2TextGrid)))
	chipInfoTab := container.NewTabItem("Chip Info", chipInfoTabContent)

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
	aboutTab := container.NewTabItem("About", aboutTabContent)

	tabs := container.NewAppTabs(profileTab, notificationTab, chipInfoTab, aboutTab)

	w.SetContent(tabs)

	return w
}

func InitDownloadWindow() fyne.Window {
	LockButtonChan <- true
	w := App.NewWindow("Download")
	w.Resize(fyne.Size{
		Width:  500,
		Height: 200,
	})
	w.RequestFocus()
	w.SetOnClosed(func() {
		LockButtonChan <- false
	})
	smdp := widget.NewEntry()
	smdp.PlaceHolder = "Leave it empty to use default smdp"
	matchID := widget.NewEntry()
	matchID.PlaceHolder = "Activation code. Optional"
	confirmCode := widget.NewEntry()
	confirmCode.PlaceHolder = "Optional"
	imei := widget.NewEntry()
	imei.PlaceHolder = "The IMEI sent to SM-DP. Optional"

	// appendText := widget.NewTextGridFromString(
	// 	"SM-DP+: Leave it empty to use default smdp.\n" +
	// 		"Matching ID: activation code. optional.\n" +
	// 		"Confirm Code: optional.\n" +
	// 		"IMEI: The IMEI of the device to which Profile is to be downloaded, optional.")
	// appendText := widget.NewRichTextWithText("Placeholder")

	form := &widget.Form{
		Items: []*widget.FormItem{ // we can specify items in the constructor
			{Text: "SM-DP+", Widget: smdp},
			{Text: "Matching ID", Widget: matchID},
			{Text: "Confirm Code", Widget: confirmCode},
			{Text: "IMEI", Widget: imei}},
		OnSubmit: func() { // optional, handle form submission
			var pullConfig PullInfo
			pullConfig.smdp = smdp.Text
			pullConfig.matchID = matchID.Text
			pullConfig.confirmCode = confirmCode.Text
			pullConfig.imei = imei.Text
			w.Close()
			LpacProfileDownload(pullConfig)
		},
		OnCancel: func() {
			w.Close()
		},
	}
	// form.Append("", appendText)
	w.SetContent(form)
	return w
}

func ErrDialog(err error) {
	d := dialog.NewError(err, WMain)
	d.Show()
}

func SelectItemDialog() {
	d := dialog.NewInformation("Info", "Please select a item.", WMain)
	d.Show()
}
