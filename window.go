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
	nativeDialog "github.com/sqweek/dialog"
	"image/color"
)

var WMain fyne.Window
var spacer *canvas.Rectangle

func InitMainWindow() fyne.Window {
	w := App.NewWindow("EasyLPAC")
	w.Resize(fyne.Size{
		Width:  850,
		Height: 545,
	})
	w.SetMaster()
	SetFixedWindowSize(&w)

	statusBar := container.NewGridWrap(fyne.Size{
		Width:  100,
		Height: 36,
	}, StatusLabel, StatusProcessBar)

	spacer = canvas.NewRectangle(color.Transparent)
	spacer.SetMinSize(fyne.NewSize(1, 1))

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
			container.NewHBox(ProfileMaskCheck, DownloadButton, spacer, DiscoveryButton, spacer, SetNicknameButton, spacer, SwitchStateButton, spacer, DeleteProfileButton),
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
			container.NewVBox(container.NewBorder(nil, nil, container.NewHBox(EidLabel, CopyEidButton), EUICCManufacturerLabel),
				container.NewHBox(DefaultDpAddressLabel, SetDefaultSmdpButton),
				container.NewBorder(nil, nil, RootDsAddressLabel, ViewCertInfoButton)),
			nil,
			nil,
			nil,
			container.NewScroll(EuiccInfo2Entry),
		))
	ChipInfoTab = container.NewTabItem("Chip Info", chipInfoTabContent)

	thankstoText := widget.NewRichTextFromMarkdown(`
# Thanks to

[lpac](https://github.com/estkme-group/lpac) C-based eUICC LPA

[eUICC Manual](https://euicc-manual.septs.app) eUICC Developer Manual

[fyne](https://github.com/fyne-io/fyne) Material Design GUI toolkit`)

	aboutText := widget.NewRichTextFromMarkdown(`
# EasyLPAC

lpac GUI Frontend

[Github](https://github.com/creamlike1024/EasyLPAC) Repo `)

	aboutTabContent := container.NewBorder(
		nil,
		container.NewBorder(nil, nil,
			widget.NewLabel(fmt.Sprintf("Version: %s", Version)),
			widget.NewLabel(fmt.Sprintf("eUICC Data: %s", EUICCDataVersion))),
		nil,
		nil,
		container.NewCenter(container.NewVBox(thankstoText, aboutText)))
	AboutTab = container.NewTabItem("About", aboutTabContent)

	Tabs = container.NewAppTabs(ProfileTab, NotificationTab, ChipInfoTab, AboutTab)

	w.SetContent(Tabs)

	return w
}

func InitDownloadDialog() dialog.Dialog {
	smdpEntry := &widget.Entry{PlaceHolder: "Leave it empty to use default SM-DP+"}
	matchIDEntry := &widget.Entry{PlaceHolder: "Activation code. Optional"}
	confirmCodeEntry := &widget.Entry{PlaceHolder: "Optional"}
	imeiEntry := &widget.Entry{PlaceHolder: "The IMEI sent to SM-DP+. Optional"}

	formItems := []*widget.FormItem{
		{Text: "SM-DP+", Widget: smdpEntry},
		{Text: "Matching ID", Widget: matchIDEntry},
		{Text: "Confirm Code", Widget: confirmCodeEntry},
		{Text: "IMEI", Widget: imeiEntry},
	}

	form := widget.NewForm(formItems...)
	var d dialog.Dialog
	cancelButton := &widget.Button{
		Text: "Cancel",
		Icon: theme.CancelIcon(),
		OnTapped: func() {
			d.Hide()
		},
	}
	downloadButton := &widget.Button{
		Text:       "Download",
		Icon:       theme.ConfirmIcon(),
		Importance: widget.HighImportance,
		OnTapped: func() {
			pullConfig := PullInfo{
				SMDP:        smdpEntry.Text,
				MatchID:     matchIDEntry.Text,
				ConfirmCode: confirmCodeEntry.Text,
				IMEI:        imeiEntry.Text,
			}
			go func() {
				RefreshNotification()
				LpacProfileDownload(pullConfig)
			}()
			d.Hide()
		},
	}
	// 回调函数需要操作 selectQRCodeButton，预先声明
	var selectQRCodeButton *widget.Button
	selectQRCodeButton = &widget.Button{
		Text: "Scan image file",
		Icon: theme.FileImageIcon(),
		OnTapped: func() {
			go func() {
				fileBuilder := nativeDialog.File().Title("Select a QR Code image file")
				fileBuilder.Filters = []nativeDialog.FileFilter{
					{
						Desc:       "Image (*.png, *.jpg, *.jpeg)",
						Extensions: []string{"PNG", "JPG", "JPEG"},
					},
					{
						Desc:       "All files (*.*)",
						Extensions: []string{"*"},
					},
				}

				selectQRCodeButton.Disable()
				cancelButton.Disable()
				downloadButton.Disable()

				filename, err := fileBuilder.Load()
				if err != nil {
					if err.Error() != "Cancelled" {
						panic(err)
					}
				} else {
					result, err := ScanQRCodeImageFile(filename)
					if err != nil {
						dError := dialog.NewError(err, WMain)
						dError.Show()
					} else {
						pullInfo, err := DecodeLPADownloadConfig(result.String())
						if err != nil {
							dError := dialog.NewError(err, WMain)
							dError.Show()
						} else {
							smdpEntry.SetText(pullInfo.SMDP)
							matchIDEntry.SetText(pullInfo.MatchID)
						}
					}
				}

				selectQRCodeButton.Enable()
				cancelButton.Enable()
				downloadButton.Enable()
			}()
		},
	}
	d = dialog.NewCustomWithoutButtons("Download", container.NewBorder(
		nil,
		container.NewVBox(spacer, container.NewCenter(selectQRCodeButton), spacer, container.NewCenter(container.NewHBox(cancelButton, spacer, downloadButton))),
		nil,
		nil,
		form), WMain)
	d.Resize(fyne.Size{
		Width:  500,
		Height: 340,
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
				ShowLpacErrDialog(err)
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
				ShowLpacErrDialog(err)
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

func ShowLpacErrDialog(err error) {
	go func() {
		l := &widget.Label{Text: fmt.Sprintf("%v", err), TextStyle: fyne.TextStyle{Monospace: true}}
		content := container.NewVBox(container.NewCenter(container.NewHBox(widget.NewIcon(theme.ErrorIcon()), widget.NewLabel("lpac error"))),
			container.NewCenter(l),
			container.NewCenter(widget.NewLabel("Please check the log for details")))
		d := dialog.NewCustom("Error", "OK", content, WMain)
		d.Show()
	}()
}

func ShowSelectItemDialog() {
	go func() {
		d := dialog.NewInformation("Info", "Please select a item.", WMain)
		d.Resize(fyne.Size{
			Width:  220,
			Height: 160,
		})
		d.Show()
	}()
}

func ShowSelectCardReaderDialog() {
	go func() {
		d := dialog.NewInformation("Info", "Please select a card reader.", WMain)
		d.Show()
	}()
}

func ShowRefreshNeededDialog() {
	go func() {
		d := dialog.NewInformation("Info", "Please refresh before proceeding.\n", WMain)
		d.Show()
	}()
}
