package main

import (
	"errors"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"golang.org/x/net/publicsuffix"
	"slices"
	"strings"
	"time"
)

var StatusProcessBar *widget.ProgressBarInfinite
var StatusLabel *widget.Label
var SetNicknameButton *widget.Button
var DownloadButton *widget.Button

// var DiscoveryButton *widget.Button

var DeleteProfileButton *widget.Button
var SwitchStateButton *widget.Button
var ProcessNotificationButton *widget.Button
var RemoveNotificationButton *widget.Button
var RemoveAllNotificationButton *widget.Button

var ProfileList *widget.List
var NotificationList *widget.List

var FreeSpaceLabel *widget.Label
var OpenLogButton *widget.Button
var RefreshButton *widget.Button
var ProfileMaskCheck *widget.Check
var NotificationMaskCheck *widget.Check

var EidLabel *widget.Label
var DefaultDpAddressLabel *widget.Label
var RootDsAddressLabel *widget.Label
var EuiccInfo2Entry *ReadOnlyEntry
var CopyEidButton *widget.Button
var SetDefaultSmdpButton *widget.Button
var ViewCertInfoButton *widget.Button
var EUICCManufacturerLabel *widget.Label
var CopyEuiccInfo2Button *widget.Button

var ApduDriverSelect *widget.Select
var ApduDriverRefreshButton *widget.Button

var Tabs *container.AppTabs
var ProfileTab *container.TabItem
var NotificationTab *container.TabItem
var ChipInfoTab *container.TabItem
var AboutTab *container.TabItem

type ReadOnlyEntry struct{ widget.Entry }

func (entry *ReadOnlyEntry) TypedRune(r rune)            {}
func (entry *ReadOnlyEntry) TypedKey(key *fyne.KeyEvent) {}
func (entry *ReadOnlyEntry) TypedShortcut(shortcut fyne.Shortcut) {
	switch shortcut := shortcut.(type) {
	case *fyne.ShortcutCopy:
		entry.Entry.TypedShortcut(shortcut)
	}
}

func (entry *ReadOnlyEntry) TappedSecondary(ev *fyne.PointEvent) {
	c := fyne.CurrentApp().Driver().AllWindows()[0].Clipboard()
	copyItem := fyne.NewMenuItem("Copy", func() {
		c.SetContent(entry.SelectedText())
	})
	menu := fyne.NewMenu("", copyItem)
	widget.ShowPopUpMenuAtPosition(menu, fyne.CurrentApp().Driver().CanvasForObject(entry), ev.AbsolutePosition)
}

func NewReadOnlyEntry() *ReadOnlyEntry {
	entry := &ReadOnlyEntry{}
	entry.ExtendBaseWidget(entry) // 确保自定义的 widget 被正确地初始化
	entry.MultiLine = true        // 支持多行文本
	entry.TextStyle = fyne.TextStyle{Monospace: true}
	entry.Wrapping = fyne.TextWrapOff
	return entry
}

func InitWidgets() {
	StatusProcessBar = widget.NewProgressBarInfinite()
	StatusProcessBar.Stop()
	StatusProcessBar.Hide()

	StatusLabel = widget.NewLabel("Ready.")

	DownloadButton = &widget.Button{Text: "Download",
		OnTapped: func() { go downloadButtonFunc() },
		Icon:     theme.DownloadIcon()}

	// DiscoveryButton = &widget.Button{Text: "Discovery",
	// 	OnTapped: func() { go discoveryButtonFunc() },
	// 	Icon:     theme.SearchIcon()}

	SetNicknameButton = &widget.Button{Text: "Nickname",
		OnTapped: func() { go setNicknameButtonFunc() },
		Icon:     theme.DocumentCreateIcon()}

	DeleteProfileButton = &widget.Button{Text: "Delete",
		OnTapped: func() { go deleteProfileButtonFunc() },
		Icon:     theme.DeleteIcon()}

	SwitchStateButton = &widget.Button{Text: "Enable", OnTapped: func() { go switchStateButtonFunc() },
		Icon: theme.ConfirmIcon()}

	ProfileList = initProfileList()
	NotificationList = initNotificationList()

	ProcessNotificationButton = &widget.Button{Text: "Process",
		OnTapped: func() { go processNotificationButtonFunc() },
		Icon:     theme.MediaPlayIcon()}

	RemoveNotificationButton = &widget.Button{Text: "Remove",
		OnTapped: func() { go removeNotificationButtonFunc() },
		Icon:     theme.ContentRemoveIcon()}

	RemoveAllNotificationButton = &widget.Button{Text: "Remove All",
		OnTapped: func() { go removeAllNotificationButtonFunc() },
		Icon:     theme.DeleteIcon()}

	FreeSpaceLabel = widget.NewLabel("")

	OpenLogButton = &widget.Button{Text: "Open Log",
		OnTapped: func() { go OpenLog() },
		Icon:     theme.FolderOpenIcon()}

	RefreshButton = &widget.Button{Text: "Refresh",
		OnTapped: func() { go Refresh() },
		Icon:     theme.ViewRefreshIcon()}

	ProfileMaskCheck = widget.NewCheck("Mask", func(b bool) {
		if b {
			ProfileMaskNeeded = true
			ProfileList.Refresh()
		} else {
			ProfileMaskNeeded = false
			ProfileList.Refresh()
		}
	})
	NotificationMaskCheck = widget.NewCheck("Mask", func(b bool) {
		if b {
			NotificationMaskNeeded = true
			NotificationList.Refresh()
		} else {
			NotificationMaskNeeded = false
			NotificationList.Refresh()
		}
	})

	EidLabel = widget.NewLabel("")
	DefaultDpAddressLabel = widget.NewLabel("")
	RootDsAddressLabel = widget.NewLabel("")
	EuiccInfo2Entry = NewReadOnlyEntry()
	EuiccInfo2Entry.Hide()
	CopyEidButton = &widget.Button{Text: "Copy",
		OnTapped: func() { go copyEidButtonFunc() },
		Icon:     theme.ContentCopyIcon()}
	CopyEidButton.Hide()
	SetDefaultSmdpButton = &widget.Button{OnTapped: func() { go setDefaultSmdpButtonFunc() },
		Icon: theme.DocumentCreateIcon()}
	SetDefaultSmdpButton.Hide()
	ViewCertInfoButton = &widget.Button{Text: "Certificate Issuer",
		OnTapped: func() { go viewCertInfoButtonFunc() },
		Icon:     theme.InfoIcon()}
	ViewCertInfoButton.Hide()
	EUICCManufacturerLabel = &widget.Label{}
	EUICCManufacturerLabel.Hide()
	CopyEuiccInfo2Button = &widget.Button{Text: "Copy eUICCInfo2",
		OnTapped: func() { go copyEuiccInfo2ButtonFunc() },
		Icon:     theme.ContentCopyIcon()}
	CopyEuiccInfo2Button.Hide()
	ApduDriverSelect = widget.NewSelect([]string{}, func(s string) { SetDriverIFID(s) })
	ApduDriverRefreshButton = &widget.Button{OnTapped: func() { go RefreshApduDriver() },
		Icon: theme.SearchReplaceIcon()}
}

func downloadButtonFunc() {
	if ConfigInstance.DriverIFID == "" {
		ShowSelectCardReaderDialog()
		return
	}
	if RefreshNeeded == true {
		ShowRefreshNeededDialog()
		return
	}
	d := InitDownloadDialog()
	d.Show()
}

// func discoveryButtonFunc() {
// 	if ConfigInstance.DriverIFID == "" {
// 		ShowSelectCardReaderDialog()
// 		return
// 	}
// 	if RefreshNeeded == true {
// 		ShowRefreshNeededDialog()
// 		return
// 	}
// 	discoveryFunc := func() {
// 		ch := make(chan bool)
// 		var data []DiscoveryResult
// 		var err error
// 		go func() {
// 			data, err = LpacProfileDiscovery()
// 			ch <- true
// 		}()
// 		<-ch
// 		if err != nil {
// 			ShowLpacErrDialog(err)
// 			return
// 		}
// 		if len(data) != 0 {
// 			var d *dialog.CustomDialog
// 			selectedProfile := Unselected
// 			foundLabel := widget.NewLabel("")
// 			if len(data) == 1 {
// 				foundLabel.SetText(fmt.Sprintf("%d profile found.", len(data)))
// 			} else {
// 				foundLabel.SetText(fmt.Sprintf("%d profiles found.", len(data)))
// 			}
// 			discoveredEsimListTitle := widget.NewLabel("EventID\t\tRSP Server Address")
// 			discoveredEsimListTitle.TextStyle = fyne.TextStyle{Bold: true}
// 			discoveredEsimList := widget.NewList(func() int {
// 				return len(data)
// 			}, func() fyne.CanvasObject {
// 				return &widget.Label{TextStyle: fyne.TextStyle{Monospace: true}}
// 			}, func(i widget.ListItemID, o fyne.CanvasObject) {
// 				o.(*widget.Label).SetText(fmt.Sprintf("%-6s\t\t%s", data[i].EventID, data[i].RspServerAddress))
// 			})
// 			discoveredEsimList.OnSelected = func(id widget.ListItemID) {
// 				selectedProfile = id
// 			}
// 			downloadButton := widget.NewButton("Download", func() {
// 				if selectedProfile == Unselected {
// 					ShowSelectItemDialog()
// 				} else {
// 					d.Hide()
// 					go LpacProfileDownload(PullInfo{
// 						SMDP:        data[selectedProfile].RspServerAddress,
// 						MatchID:     "",
// 						ConfirmCode: "",
// 						IMEI:        "",
// 					})
// 				}
// 			})
// 			downloadButton.Importance = widget.HighImportance
// 			downloadButton.SetIcon(theme.DownloadIcon())
// 			dismissButton := widget.NewButton("Dismiss", func() {
// 				d.Hide()
// 			})
// 			dismissButton.SetIcon(theme.CancelIcon())
// 			content := container.NewBorder(
// 				foundLabel,
// 				nil,
// 				nil,
// 				nil,
// 				container.NewBorder(
// 					discoveredEsimListTitle,
// 					nil,
// 					nil,
// 					nil,
// 					discoveredEsimList))
// 			d = dialog.NewCustomWithoutButtons("Result", content, WMain)
// 			d.Resize(fyne.Size{
// 				Width:  550,
// 				Height: 400,
// 			})
// 			d.SetButtons([]fyne.CanvasObject{dismissButton, downloadButton})
// 			d.Show()
//
// 		} else {
// 			d := dialog.NewInformation("Result", "No eSIM profile found.\n", WMain)
// 			d.Show()
// 		}
// 	}
// 	d := dialog.NewInformation("Info", "Discovery has not been actually tested yet.\n"+
// 		"If you have any discoverable profiles and try to use the discovery function,\n"+
// 		"regardless of whether it is successful,\n"+
// 		"please open an issue to report logs and program behavior.\n"+
// 		"Thank you very much\n", WMain)
// 	d.SetOnClosed(func() {
// 		go discoveryFunc()
// 	})
// 	d.Show()
// }

func setNicknameButtonFunc() {
	if ConfigInstance.DriverIFID == "" {
		ShowSelectCardReaderDialog()
		return
	}
	if RefreshNeeded {
		ShowRefreshNeededDialog()
		return
	}
	if SelectedProfile == Unselected {
		ShowSelectItemDialog()
		return
	}
	d := InitSetNicknameDialog()
	d.Show()
}

func deleteProfileButtonFunc() {
	if ConfigInstance.DriverIFID == "" {
		ShowSelectCardReaderDialog()
		return
	}
	if RefreshNeeded {
		ShowRefreshNeededDialog()
		return
	}
	if SelectedProfile == Unselected {
		ShowSelectItemDialog()
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
	profileText := fmt.Sprint(
		"ICCID: ", Profiles[SelectedProfile].Iccid, "\n",
		"Provider: ", Profiles[SelectedProfile].ServiceProviderName, "\n",
	)
	if name := Profiles[SelectedProfile].ProfileNickname; name != nil {
		profileText += fmt.Sprint("Nickname: ", name, "\n")
	}
	d := dialog.NewCustomConfirm("Confirm",
		"Confirm",
		"Cancel",
		container.NewVBox(container.NewCenter(widget.NewLabel("Are you sure you want to delete this profile?")),
			&widget.Label{Text: profileText}),
		func(b bool) {
			if b {
				go func() {
					notificationOrigin := Notifications
					if err := LpacProfileDelete(Profiles[SelectedProfile].Iccid); err != nil {
						ShowLpacErrDialog(err)
						Refresh()
					} else {
						Refresh()
						d := dialog.NewConfirm("Delete Successful",
							"The profile has been successfully deleted\nSend the delete notification now?\n",
							func(b bool) {
								if b {
									deleteNotification := findNewNotification(notificationOrigin, Notifications)
									go processNotification(deleteNotification.SeqNumber)
								}
							},
							WMain)
						d.Show()
					}
				}()
			}
		}, WMain)
	d.Show()
}

func switchStateButtonFunc() {
	if ConfigInstance.DriverIFID == "" {
		ShowSelectCardReaderDialog()
		return
	}
	if RefreshNeeded {
		ShowRefreshNeededDialog()
		return
	}
	if SelectedProfile == Unselected {
		ShowSelectItemDialog()
		return
	}
	if ProfileStateAllowDisable {
		if err := LpacProfileDisable(Profiles[SelectedProfile].Iccid); err != nil {
			ShowLpacErrDialog(err)
		}
	} else {
		if err := LpacProfileEnable(Profiles[SelectedProfile].Iccid); err != nil {
			ShowLpacErrDialog(err)
		}
	}
	Refresh()
	if ProfileStateAllowDisable {
		SwitchStateButton.SetText("Enable")
		SwitchStateButton.SetIcon(theme.ConfirmIcon())
	}
}

func processNotificationButtonFunc() {
	if ConfigInstance.DriverIFID == "" {
		ShowSelectCardReaderDialog()
		return
	}
	if RefreshNeeded {
		ShowRefreshNeededDialog()
		return
	}
	if SelectedNotification == Unselected {
		ShowSelectItemDialog()
		return
	}
	seq := Notifications[SelectedNotification].SeqNumber
	go processNotification(seq)
}

func removeNotificationButtonFunc() {
	if ConfigInstance.DriverIFID == "" {
		ShowSelectCardReaderDialog()
		return
	}
	if RefreshNeeded {
		ShowRefreshNeededDialog()
		return
	}
	if SelectedNotification == Unselected {
		ShowSelectItemDialog()
		return
	}
	d := dialog.NewCustomConfirm("Confirm",
		"Confirm",
		"Cancel",
		&widget.Label{Text: "Are you sure you want to remove this notification?\n",
			Alignment: fyne.TextAlignCenter},
		func(b bool) {
			if b {
				if err := LpacNotificationRemove(Notifications[SelectedNotification].SeqNumber); err != nil {
					ShowLpacErrDialog(err)
				}
				RefreshNotification()
				RefreshChipInfo()
			} else {
				return
			}
		}, WMain)
	d.Show()
}

func removeAllNotificationButtonFunc() {
	if ConfigInstance.DriverIFID == "" {
		ShowSelectCardReaderDialog()
		return
	}
	if RefreshNeeded {
		ShowRefreshNeededDialog()
		return
	}
	var d dialog.Dialog
	entry := &widget.Entry{PlaceHolder: "Confirm"}
	content := container.NewBorder(container.NewVBox(
		&widget.Label{Alignment: fyne.TextAlignCenter, Text: "Are you sure you want to delete all notifications?\n" +
			"This operation is not recoverable"},
		container.NewCenter(widget.NewRichTextFromMarkdown("Enter **Confirm** to proceed")),
	),
		container.NewCenter(container.NewHBox(
			&widget.Button{Text: "Cancel", Icon: theme.CancelIcon(), OnTapped: func() { d.Hide() }},
			spacer,
			&widget.Button{Text: "OK", Icon: theme.ConfirmIcon(), OnTapped: func() {
				d.Hide()
				if strings.TrimSpace(entry.Text) != "Confirm" {
					dError := dialog.NewError(errors.New("input mismatch, cancel operation"), WMain)
					dError.Show()
				} else {
					for _, notification := range Notifications {
						err := LpacNotificationRemove(notification.SeqNumber)
						if err != nil {
							ShowLpacErrDialog(err)
						}
						RefreshNotification()
					}
					dInfo := dialog.NewInformation("Info", "Operation finished", WMain)
					dInfo.Show()
				}
			}})),
		nil,
		nil,
		entry)
	d = dialog.NewCustomWithoutButtons("Remove All Notification?", content, WMain)
	d.Show()
}

func copyEidButtonFunc() {
	WMain.Clipboard().SetContent(ChipInfo.EidValue)
	CopyEidButton.SetText("Copied!")
	time.Sleep(2 * time.Second)
	CopyEidButton.SetText("Copy")
}

func copyEuiccInfo2ButtonFunc() {
	WMain.Clipboard().SetContent(EuiccInfo2Entry.Text)
	CopyEuiccInfo2Button.SetText("Copied eUICCInfo2!")
	time.Sleep(2 * time.Second)
	CopyEuiccInfo2Button.SetText("Copy eUICCInfo2")
}

func setDefaultSmdpButtonFunc() {
	if ConfigInstance.DriverIFID == "" {
		ShowSelectCardReaderDialog()
		return
	}
	if RefreshNeeded {
		ShowRefreshNeededDialog()
		return
	}
	d := InitSetDefaultSmdpDialog()
	d.Show()
}

func viewCertInfoButtonFunc() {
	selectedCI := Unselected
	type ciWidgetEl struct {
		Country string
		Name    string
		KeyID   string
	}
	var ciWidgetEls []ciWidgetEl
	// ChipInfo 中 signing 和 verification 同时存在则有效
	for _, keyId := range ChipInfo.EUICCInfo2.EuiccCiPKIDListForSigning {
		if !slices.Contains(ChipInfo.EUICCInfo2.EuiccCiPKIDListForVerification, keyId) {
			continue
		}
		var element ciWidgetEl
		element.KeyID = keyId
		element.Name = "Unknown"
		if issuer := GetIssuer(keyId); issuer != nil {
			element.Country = issuer.Country
			element.Name = issuer.Name
		}
		ciWidgetEls = append(ciWidgetEls, element)
	}
	list := &widget.List{
		Length: func() int {
			return len(ciWidgetEls)
		},
		CreateItem: func() fyne.CanvasObject {
			return container.NewVBox(container.NewBorder(nil, nil,
				&widget.Label{}, &widget.Label{}),
				&widget.Label{})
		},
		UpdateItem: func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*fyne.Container).Objects[0].(*fyne.Container).Objects[0].(*widget.Label).SetText(ciWidgetEls[i].Name)
			o.(*fyne.Container).Objects[0].(*fyne.Container).Objects[1].(*widget.Label).SetText(CountryCodeToEmoji(ciWidgetEls[i].Country))
			o.(*fyne.Container).Objects[1].(*widget.Label).SetText(fmt.Sprintf("KeyID: %s", ciWidgetEls[i].KeyID))
		},
		OnSelected: func(id widget.ListItemID) {
			selectedCI = id
		},
		OnUnselected: func(id widget.ListItemID) {
			selectedCI = Unselected
		},
	}
	certDataButtonFunc := func() {
		if selectedCI == Unselected {
			ShowSelectItemDialog()
		} else if issuer := GetIssuer(ciWidgetEls[selectedCI].KeyID); issuer == nil {
			d := dialog.NewInformation("No Data",
				"The information of this certificate is not included.\n"+
					"If you have any information about this certificate,\n"+
					"you can report it to <euicc-dev-manual@septs.pw>\n"+
					"Thank you",
				WMain)
			d.Show()
		} else {
			const CiUrl = "https://euicc-manual.septs.app/docs/pki/ci/files/"
			certificateURL := fmt.Sprint(CiUrl, issuer.KeyID, ".txt")
			if err := OpenProgram(certificateURL); err != nil {
				ShowLpacErrDialog(err)
			}
		}
	}
	certDataButton := &widget.Button{
		Text:     "Certificate Info",
		OnTapped: certDataButtonFunc,
		Icon:     theme.InfoIcon(),
	}
	d := dialog.NewCustom("Certificate Issuer", "OK",
		container.NewBorder(nil, container.NewCenter(certDataButton), nil, nil, list), WMain)
	d.Resize(fyne.Size{
		Width:  600,
		Height: 500,
	})
	d.Show()
}

func initProfileList() *widget.List {
	return &widget.List{
		Length: func() int {
			return len(Profiles)
		},
		CreateItem: func() fyne.CanvasObject {
			iccidLabel := &widget.Label{}
			profileNameLabel := &widget.Label{}
			stateLabel := &widget.Label{TextStyle: fyne.TextStyle{Bold: true}}
			enabledIcon := widget.NewIcon(theme.ConfirmIcon())
			profileIcon := widget.NewIcon(theme.FileImageIcon())
			providerLabel := &widget.Label{}
			nicknameLabel := &widget.Label{}
			return container.NewVBox(
				container.NewBorder(nil, nil, iccidLabel, profileNameLabel),
				container.NewBorder(nil, nil, container.NewHBox(stateLabel, enabledIcon, providerLabel, profileIcon), nicknameLabel))
		},
		UpdateItem: func(i widget.ListItemID, o fyne.CanvasObject) {
			r1 := o.(*fyne.Container).Objects[0].(*fyne.Container)
			r2 := o.(*fyne.Container).Objects[1].(*fyne.Container)
			iccidLabel := r1.Objects[0].(*widget.Label)
			profileNameLabel := r1.Objects[1].(*widget.Label)
			stateLabel := r2.Objects[0].(*fyne.Container).Objects[0].(*widget.Label)
			enabledIcon := r2.Objects[0].(*fyne.Container).Objects[1].(*widget.Icon)
			providerLabel := r2.Objects[0].(*fyne.Container).Objects[2].(*widget.Label)
			profileIcon := r2.Objects[0].(*fyne.Container).Objects[3].(*widget.Icon)
			nicknameLabel := r2.Objects[1].(*widget.Label)

			iccid := Profiles[i].Iccid
			if ProfileMaskNeeded {
				iccid = Profiles[i].MaskedICCID()
			}
			iccidLabel.SetText(fmt.Sprintf("ICCID: %s", iccid))
			profileNameLabel.SetText(Profiles[i].ProfileName)
			stateLabel.SetText(strings.ToUpper(Profiles[i].ProfileState))
			if Profiles[i].ProfileState == "enabled" {
				enabledIcon.Show()
			} else {
				enabledIcon.Hide()
			}

			if Profiles[i].Icon != nil {
				profileIcon.SetResource(fyne.NewStaticResource(Profiles[i].Iccid, Profiles[i].Icon))
				profileIcon.Show()
			} else {
				profileIcon.Hide()
			}

			providerLabel.SetText("Provider: " + Profiles[i].ServiceProviderName)
			if Profiles[i].ProfileNickname != nil {
				nicknameLabel.SetText(*Profiles[i].ProfileNickname)
			} else {
				nicknameLabel.SetText("")
			}
		},
		OnSelected: func(id widget.ListItemID) {
			SelectedProfile = id
			if Profiles[SelectedProfile].ProfileState == "enabled" {
				ProfileStateAllowDisable = true
				SwitchStateButton.SetText("Disable")
				SwitchStateButton.SetIcon(theme.CancelIcon())
			} else {
				ProfileStateAllowDisable = false
				SwitchStateButton.SetText("Enable")
				SwitchStateButton.SetIcon(theme.ConfirmIcon())
			}
		},
		OnUnselected: func(id widget.ListItemID) {
			SelectedProfile = Unselected
		}}
}

func initNotificationList() *widget.List {
	return &widget.List{
		Length: func() int {
			return len(Notifications)
		},
		CreateItem: func() fyne.CanvasObject {
			notificationAddressLabel := &widget.Label{}
			seqLabel := &widget.Label{}
			operationLabel := &widget.Label{TextStyle: fyne.TextStyle{Bold: true}}
			providerLaber := &widget.Label{}
			iccidLabel := &widget.Label{}
			providerIcon := widget.NewIcon(theme.FileImageIcon())
			return container.NewVBox(
				container.NewBorder(nil, nil, notificationAddressLabel, seqLabel),
				container.NewHBox(operationLabel, providerIcon, providerLaber, iccidLabel),
			)
		},
		UpdateItem: func(i widget.ListItemID, o fyne.CanvasObject) {
			iccid := Notifications[i].Iccid
			notificationAddress := Notifications[i].NotificationAddress
			maskFQDNExceptPublicSuffix := func(fqdn string) string {
				suffix, _ := publicsuffix.PublicSuffix(fqdn)
				parts := strings.Split(fqdn, ".")
				suffixParts := strings.Split(suffix, ".")
				// 如果域名部分少于后缀部分，说明域名不合法或者是一个裸域名，直接返回掩码后的顶级域名
				if len(parts) <= len(suffixParts) {
					return strings.Repeat("x", len(parts[0])) + "." + suffix
				}
				// 掩盖除了后缀之外的所有部分
				for x := 0; x < len(parts)-len(suffixParts); x++ {
					parts[x] = strings.Repeat("x", len(parts[x]))
				}
				return strings.Join(parts, ".")
			}
			if NotificationMaskNeeded {
				if iccid != "" {
					iccid = Notifications[i].MaskedICCID()
				}
				notificationAddress = maskFQDNExceptPublicSuffix(Notifications[i].NotificationAddress)
			}
			// ICCID
			if iccid == "" {
				iccid = "No ICCID!"
			}
			o.(*fyne.Container).Objects[1].(*fyne.Container).Objects[3].(*widget.Label).
				SetText(fmt.Sprint("(", iccid, ")"))
			// Notification Address
			o.(*fyne.Container).Objects[0].(*fyne.Container).Objects[0].(*widget.Label).
				SetText(notificationAddress)
			// Seq number
			o.(*fyne.Container).Objects[0].(*fyne.Container).Objects[1].(*widget.Label).
				SetText(fmt.Sprint("Seq: ", Notifications[i].SeqNumber))
			// Operation
			o.(*fyne.Container).Objects[1].(*fyne.Container).Objects[0].(*widget.Label).
				SetText(strings.ToTitle(Notifications[i].ProfileManagementOperation))
			// Provider
			profile, err := findProfileByIccid(Notifications[i].Iccid)
			if err != nil {
				o.(*fyne.Container).Objects[1].(*fyne.Container).Objects[2].(*widget.Label).SetText("?deleted profile")
				o.(*fyne.Container).Objects[1].(*fyne.Container).Objects[1].(*widget.Icon).Hide()
			} else {
				name := profile.ServiceProviderName
				if profile.ProfileNickname != nil {
					name = *profile.ProfileNickname
				}
				o.(*fyne.Container).Objects[1].(*fyne.Container).Objects[2].(*widget.Label).SetText(name)
				icon := o.(*fyne.Container).Objects[1].(*fyne.Container).Objects[1].(*widget.Icon)
				if profile.Icon != nil {
					icon.SetResource(fyne.NewStaticResource(profile.Iccid, profile.Icon))
					icon.Show()
				} else {
					icon.Hide()
				}
			}
		},
		OnSelected: func(id widget.ListItemID) {
			SelectedNotification = id
		},
		OnUnselected: func(id widget.ListItemID) {
			SelectedNotification = Unselected
		}}
}

func processNotification(seq int) {
	if err := LpacNotificationProcess(seq); err != nil {
		ShowLpacErrDialog(err)
		RefreshNotification()
	} else {
		notification := Notification{}
		for _, n := range Notifications {
			if n.SeqNumber == seq {
				notification = n
			}
		}
		var d *dialog.CustomDialog
		notNowButton := &widget.Button{
			Text: "Not Now",
			Icon: theme.CancelIcon(),
			OnTapped: func() {
				d.Hide()
			},
		}
		removeButton := &widget.Button{
			Text: "Remove",
			Icon: theme.DeleteIcon(),
			OnTapped: func() {
				go func() {
					d.Hide()
					if err := LpacNotificationRemove(seq); err != nil {
						ShowLpacErrDialog(err)
					}
					RefreshNotification()
					RefreshChipInfo()
				}()
			},
		}
		d = dialog.NewCustomWithoutButtons("Remove Notification",
			container.NewBorder(
				nil,
				container.NewCenter(container.NewHBox(notNowButton, spacer, removeButton)),
				nil,
				nil,
				container.NewVBox(
					&widget.Label{Text: "Successfully processed notification.\nDo you want to remove this notification now?",
						Alignment: fyne.TextAlignCenter},
					&widget.Label{Text: fmt.Sprintf("Seq: %d\nICCID: %s\nOperation: %s\nAddress: %s\n",
						notification.SeqNumber, notification.Iccid,
						notification.ProfileManagementOperation, notification.NotificationAddress)})),
			WMain)
		d.Show()
	}
}

func findNewNotification(first, second []Notification) Notification {
	exists := make(map[int]bool)
	for _, notification := range first {
		exists[notification.SeqNumber] = true
	}
	for _, notification := range second {
		if !exists[notification.SeqNumber] {
			return notification
		}
	}
	return Notification{}
}

func findProfileByIccid(iccid string) (Profile, error) {
	for _, profile := range Profiles {
		if iccid == profile.Iccid {
			return profile, nil
		}
	}
	return Profile{}, errors.New("profile not found")
}
