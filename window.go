package main

import (
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/validation"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/makiuchi-d/gozxing"
	nativeDialog "github.com/sqweek/dialog"
	"golang.design/x/clipboard"
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

	statusBar := container.NewGridWrap(fyne.Size{
		Width:  100,
		Height: DownloadButton.MinSize().Height,
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
			widget.NewLabel(TR.Trans("label.card_reader")),
			nil,
			container.NewHBox(container.NewGridWrap(fyne.Size{
				Width:  280,
				Height: ApduDriverSelect.MinSize().Height,
			}, ApduDriverSelect), ApduDriverRefreshButton)),
	)

	profileTabContent := container.NewBorder(
		topToolBar,
		container.NewBorder(
			nil,
			nil,
			nil,
			container.NewHBox(ProfileMaskCheck, DownloadButton,
				// spacer, DiscoveryButton,
				spacer, SetNicknameButton,
				spacer, SwitchStateButton,
				spacer, DeleteProfileButton),
			statusBar),
		nil,
		nil,
		ProfileList)
	ProfileTab = container.NewTabItem(TR.Trans("tab_bar.profile"), profileTabContent)

	notificationTabContent := container.NewBorder(
		topToolBar,
		container.NewBorder(
			nil,
			nil,
			nil,
			container.NewHBox(NotificationMaskCheck,
				spacer, ProcessNotificationButton,
				spacer, ProcessAllNotificationButton,
				spacer, BatchRemoveNotificationButton,
				spacer, RemoveNotificationButton),
			statusBar),
		nil,
		nil,
		NotificationList)
	NotificationTab = container.NewTabItem(TR.Trans("tab_bar.notification"), notificationTabContent)

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
			container.NewVBox(
				container.NewHBox(
					EidLabel, CopyEidButton, layout.NewSpacer(), EUICCManufacturerLabel),
				container.NewHBox(
					DefaultDpAddressLabel, SetDefaultSmdpButton, layout.NewSpacer(), ViewCertInfoButton),
				container.NewHBox(
					RootDsAddressLabel, layout.NewSpacer(), CopyEuiccInfo2Button)),
			nil,
			nil,
			nil,
			container.NewScroll(EuiccInfo2Entry),
		))
	ChipInfoTab = container.NewTabItem(TR.Trans("tab_bar.chip_info"), chipInfoTabContent)

	aidEntryHint := &widget.Label{Text: TR.Trans("label.aid_valid")}
	aidEntry := &widget.Entry{
		Text: ConfigInstance.LpacAID,
		Validator: validation.NewAllStrings(
			validation.NewRegexp(`^.{32}$`, TR.Trans("message.aid_length_illegal")),
			validation.NewRegexp(`[[:xdigit:]]{32}`, TR.Trans("message.aid_not_hex")),
		),
	}
	aidEntry.OnChanged = func(s string) {
		val := aidEntry.Validate()
		if val != nil {
			aidEntryHint.SetText(val.Error())
		} else {
			// Use last known good value only
			ConfigInstance.LpacAID = s
			aidEntryHint.SetText(TR.Trans("label.aid_valid"))
		}
	}
	setToDefaultAidButton := widget.NewButton(
		TR.Trans("label.aid_default_button"),
		func() {
			aidEntry.SetText(AID_DEFAULT)
		})
	setTo5berAidButton := widget.NewButton(
		TR.Trans("label.aid_5ber_button"),
		func() {
			aidEntry.SetText(AID_5BER)
		})
	setToEsimmeAidButton := widget.NewButton(
		TR.Trans("label.aid_esimme_button"),
		func() {
			aidEntry.SetText(AID_ESIMME)
		})
	setToXesimAidButton := widget.NewButton(
		TR.Trans("label.aid_xesim_button"),
		func() {
			aidEntry.SetText(AID_XESIM)
		})

	settingsTabContent := container.NewVBox(
		&widget.Label{Text: TR.Trans("label.lpac_isdr_aid"), TextStyle: fyne.TextStyle{Bold: true}},
		container.NewHBox(container.NewGridWrap(
			fyne.Size{
				Width:  320,
				Height: aidEntry.MinSize().Height,
			}, aidEntry),
			setToDefaultAidButton,
			setTo5berAidButton,
			setToEsimmeAidButton,
			setToXesimAidButton),
		aidEntryHint,

		&widget.Label{Text: TR.Trans("label.lpac_debug_output"), TextStyle: fyne.TextStyle{Bold: true}},
		&widget.Check{
			Text:    TR.Trans("label.enable_env_LIBEUICC_DEBUG_HTTP_check"),
			Checked: false,
			OnChanged: func(b bool) {
				ConfigInstance.DebugHTTP = b
			},
		},
		&widget.Check{
			Text:    TR.Trans("label.enable_env_LIBEUICC_DEBUG_APDU_check"),
			Checked: false,
			OnChanged: func(b bool) {
				ConfigInstance.DebugAPDU = b
			},
		},

		&widget.Label{Text: TR.Trans("label.easylpac_settings"), TextStyle: fyne.TextStyle{Bold: true}},
		&widget.Check{
			Text:    TR.Trans("label.auto_process_notification_check"),
			Checked: true,
			OnChanged: func(b bool) {
				ConfigInstance.AutoMode = b
			},
		})
	SettingsTab = container.NewTabItem(TR.Trans("tab_bar.settings"), settingsTabContent)

	thankstoText := widget.NewRichTextFromMarkdown(TR.Trans("thanks_to"))

	aboutText := widget.NewRichTextFromMarkdown(TR.Trans("about"))

	aboutTabContent := container.NewBorder(
		nil,
		container.NewBorder(nil, nil,
			container.NewHBox(
				widget.NewLabel(fmt.Sprintf(TR.Trans("label.version")+" %s", Version)),
				LpacVersionLabel),
			widget.NewLabel(fmt.Sprintf(TR.Trans("label.euicc_data")+" %s", EUICCDataVersion))),
		nil,
		nil,
		container.NewCenter(container.NewVBox(thankstoText, aboutText)))
	AboutTab = container.NewTabItem(TR.Trans("tab_bar.about"), aboutTabContent)

	Tabs = container.NewAppTabs(ProfileTab, NotificationTab, ChipInfoTab, SettingsTab, AboutTab)

	w.SetContent(Tabs)

	return w
}

func InitDownloadDialog() dialog.Dialog {
	smdpEntry := &widget.Entry{PlaceHolder: TR.Trans("label.smdp_entry_placeholder")}
	matchIDEntry := &widget.Entry{PlaceHolder: TR.Trans("label.match_id_entry_placeholder")}
	confirmCodeEntry := &widget.Entry{PlaceHolder: TR.Trans("label.confirm_code_entry_placeholder")}
	imeiEntry := &widget.Entry{PlaceHolder: TR.Trans("label.imei_entry_placeholder")}

	formItems := []*widget.FormItem{
		{Text: TR.Trans("label.smdp"), Widget: smdpEntry},
		{Text: TR.Trans("label.match_id"), Widget: matchIDEntry},
		{Text: TR.Trans("label.confirm_code"), Widget: confirmCodeEntry},
		{Text: TR.Trans("label.imei"), Widget: imeiEntry},
	}

	form := widget.NewForm(formItems...)
	var d dialog.Dialog
	showConfirmCodeNeededDialog := func() {
		dialog.ShowInformation(TR.Trans("dialog.confirm_code_required"),
			TR.Trans("message.confirm_code_required"), WMain)
	}
	cancelButton := &widget.Button{
		Text: TR.Trans("dialog.cancel"),
		Icon: theme.CancelIcon(),
		OnTapped: func() {
			d.Hide()
		},
	}
	downloadButton := &widget.Button{
		Text:       TR.Trans("label.download_profile_button"),
		Icon:       theme.ConfirmIcon(),
		Importance: widget.HighImportance,
		OnTapped: func() {
			d.Hide()
			pullConfig := PullInfo{
				SMDP:        strings.TrimSpace(smdpEntry.Text),
				MatchID:     strings.TrimSpace(matchIDEntry.Text),
				ConfirmCode: strings.TrimSpace(confirmCodeEntry.Text),
				IMEI:        strings.TrimSpace(imeiEntry.Text),
			}
			go func() {
				err := RefreshNotification()
				if err != nil {
					ShowLpacErrDialog(err)
					return
				}
				LpacProfileDownload(pullConfig)
			}()
		},
	}
	// 回调函数需要操作这两个 Button，预先声明
	var selectQRCodeButton *widget.Button
	var pasteFromClipboardButton *widget.Button
	disableButtons := func() {
		cancelButton.Disable()
		downloadButton.Disable()
		selectQRCodeButton.Disable()
		pasteFromClipboardButton.Disable()
	}
	enableButtons := func() {
		cancelButton.Enable()
		downloadButton.Enable()
		selectQRCodeButton.Enable()
		pasteFromClipboardButton.Enable()
	}

	selectQRCodeButton = &widget.Button{
		Text: TR.Trans("label.select_qrcode_button"),
		Icon: theme.FileImageIcon(),
		OnTapped: func() {
			go func() {
				disableButtons()
				defer enableButtons()
				fileBuilder := nativeDialog.File().Title(TR.Trans("dialog.select_qrcode"))
				fileBuilder.Filters = []nativeDialog.FileFilter{
					{
						Desc:       TR.Trans("dialog.image_desc") + " (*.PNG, *.png, *.JPG, *.jpg, *.JPEG, *.jpeg)",
						Extensions: []string{"PNG", "png", "JPG", "jpg", "JPEG", "jpeg"},
					},
					{
						Desc:       TR.Trans("dialog.all_files_desc") + " (*.*)",
						Extensions: []string{"*"},
					},
				}

				filename, err := fileBuilder.Load()
				if err != nil {
					if err.Error() != "Cancelled" {
						panic(err)
					}
				} else {
					result, err := ScanQRCodeImageFile(filename)
					if err != nil {
						dialog.ShowError(err, WMain)
					} else {
						pullInfo, confirmCodeNeeded, err2 := DecodeLpaActivationCode(result.String())
						if err2 != nil {
							dialog.ShowError(err2, WMain)
						} else {
							smdpEntry.SetText(pullInfo.SMDP)
							matchIDEntry.SetText(pullInfo.MatchID)
							if confirmCodeNeeded {
								go showConfirmCodeNeededDialog()
							}
						}
					}
				}
			}()
		},
	}
	pasteFromClipboardButton = &widget.Button{
		Text: TR.Trans("label.paste_from_clipboard_button"),
		Icon: theme.ContentPasteIcon(),
		OnTapped: func() {
			go func() {
				disableButtons()
				defer enableButtons()
				var err error
				var pullInfo PullInfo
				var confirmCodeNeeded bool
				var qrResult *gozxing.Result

				format, result, err := PasteFromClipboard()
				if err != nil {
					dialog.ShowError(err, WMain)
					return
				}
				switch format {
				case clipboard.FmtImage:
					qrResult, err = ScanQRCodeImageBytes(result)
					if err != nil {
						dialog.ShowError(err, WMain)
						return
					}
					pullInfo, confirmCodeNeeded, err = DecodeLpaActivationCode(qrResult.String())
				case clipboard.FmtText:
					pullInfo, confirmCodeNeeded, err = DecodeLpaActivationCode(CompleteActivationCode(string(result)))
				default:
					// Unreachable, should not be here.
					panic("unexpected clipboard format")
				}
				if err != nil {
					dialog.ShowError(err, WMain)
					return
				}
				smdpEntry.SetText(pullInfo.SMDP)
				matchIDEntry.SetText(pullInfo.MatchID)
				if confirmCodeNeeded {
					go showConfirmCodeNeededDialog()
				}
			}()
		},
	}
	d = dialog.NewCustomWithoutButtons(TR.Trans("label.download_profile_button"), container.NewBorder(
		nil,
		container.NewVBox(spacer, container.NewCenter(selectQRCodeButton), spacer,
			container.NewCenter(pasteFromClipboardButton), spacer,
			container.NewCenter(container.NewHBox(cancelButton, spacer, downloadButton))),
		nil,
		nil,
		form), WMain)
	d.Resize(fyne.Size{
		Width:  520,
		Height: 380,
	})
	return d
}

func InitSetNicknameDialog() dialog.Dialog {
	entry := &widget.Entry{PlaceHolder: TR.Trans("label.set_nickname_entry_placeholder")}
	form := []*widget.FormItem{
		{Text: TR.Trans("label.set_nickname_button"), Widget: entry},
	}
	d := dialog.NewForm(TR.Trans("label.set_nickname_form"), TR.Trans("dialog.submit"), TR.Trans("dialog.cancel"), form, func(b bool) {
		if b {
			if err := LpacProfileNickname(Profiles[SelectedProfile].Iccid, entry.Text); err != nil {
				ShowLpacErrDialog(err)
			}
			err := RefreshProfile()
			if err != nil {
				ShowLpacErrDialog(err)
			}
		}
	}, WMain)
	d.Resize(fyne.Size{
		Width:  400,
		Height: 180,
	})
	return d
}

func InitSetDefaultSmdpDialog() dialog.Dialog {
	entry := &widget.Entry{PlaceHolder: TR.Trans("label.set_default_smdp_entry_placeholder")}
	form := []*widget.FormItem{
		{Text: TR.Trans("label.default_smdp"), Widget: entry},
	}
	d := dialog.NewForm(TR.Trans("label.set_default_smdp_form"), TR.Trans("dialog.submit"), TR.Trans("dialog.cancel"), form, func(b bool) {
		if b {
			if err := LpacChipDefaultSmdp(entry.Text); err != nil {
				ShowLpacErrDialog(err)
			}
			err := RefreshChipInfo()
			if err != nil {
				ShowLpacErrDialog(err)
			}
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
		l := &widget.Label{Text: fmt.Sprintf("%v", err)}
		content := container.NewVBox(
			container.NewCenter(container.NewHBox(
				widget.NewIcon(theme.ErrorIcon()),
				widget.NewLabel(TR.Trans("dialog.lpac_error")))),
			container.NewCenter(l),
			container.NewCenter(widget.NewLabel(TR.Trans("message.lpac_error"))))
		dialog.ShowCustom(TR.Trans("dialog.error"), TR.Trans("dialog.ok"), content, WMain)
	}()
}

func ShowSelectItemDialog() {
	go func() {
		d := dialog.NewInformation(TR.Trans("dialog.info"), TR.Trans("message.select_item"), WMain)
		d.Show()
	}()
}

func ShowSelectCardReaderDialog() {
	go func() {
		dialog.ShowInformation(TR.Trans("dialog.info"), TR.Trans("message.select_card_reader"), WMain)
	}()
}

func ShowRefreshNeededDialog() {
	go func() {
		dialog.ShowInformation(TR.Trans("dialog.info"), TR.Trans("message.refresh_required")+"\n", WMain)
	}()
}
