package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/mattn/go-runewidth"
	"golang.org/x/net/publicsuffix"
	"image/color"
	"slices"
	"strings"
	"time"
)

const FontTabWidth = 4

var StatusProcessBar *widget.ProgressBarInfinite
var StatusLabel *widget.Label
var SetNicknameButton *widget.Button
var DownloadButton *widget.Button
var DiscoveryButton *widget.Button
var DeleteProfileButton *widget.Button
var SwitchStateButton *widget.Button
var ProcessNotificationButton *widget.Button
var RemoveNotificationButton *widget.Button
var RemoveAllNotificationButton *widget.Button

var ProfileList *widget.List
var NotificationList *widget.List

var ProfileListTitle *fyne.Container
var NotificationListTitle *fyne.Container

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
var EUICCManufacturerLabel *widget.RichText
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

	DownloadButton = &widget.Button{Text: "Download", OnTapped: func() { go downloadButtonFunc() }, Icon: theme.DownloadIcon()}

	DiscoveryButton = &widget.Button{Text: "Discovery", OnTapped: func() { go discoveryButtonFunc() }, Icon: theme.SearchIcon()}

	SetNicknameButton = &widget.Button{Text: "Nickname", OnTapped: func() { go setNicknameButtonFunc() }, Icon: theme.DocumentCreateIcon()}

	DeleteProfileButton = &widget.Button{Text: "Delete", OnTapped: func() { go deleteProfileButtonFunc() }, Icon: theme.DeleteIcon()}

	SwitchStateButton = &widget.Button{Text: "Enable", OnTapped: func() { go switchStateButtonFunc() }, Icon: theme.ConfirmIcon()}

	ProfileList = initProfileList()

	ProfileListTitle = container.NewHBox(&widget.Label{Text: "ICCID\t\t\t\t\t", TextStyle: fyne.TextStyle{Bold: true}},
		&widget.Label{Text: "Profile State\t\t", TextStyle: fyne.TextStyle{Bold: true}},
		&widget.Label{Text: "Provider\t\t\t\t\t", TextStyle: fyne.TextStyle{Bold: true}},
		&widget.Label{Text: "Nickname", TextStyle: fyne.TextStyle{Bold: true}})

	NotificationList = initNotificationList()

	NotificationListTitle = container.NewHBox(&widget.Label{Text: "Seq\t\t", TextStyle: fyne.TextStyle{Bold: true}},
		&widget.Label{Text: "ICCID\t\t\t\t\t", TextStyle: fyne.TextStyle{Bold: true}},
		&widget.Label{Text: "Operation\t\t\t", TextStyle: fyne.TextStyle{Bold: true}},
		&widget.Label{Text: "Server", TextStyle: fyne.TextStyle{Bold: true}})

	ProcessNotificationButton = &widget.Button{Text: "Process", OnTapped: func() { go processNotificationButtonFunc() }, Icon: theme.MediaPlayIcon()}

	RemoveNotificationButton = &widget.Button{Text: "Remove", OnTapped: func() { go removeNotificationButtonFunc() }, Icon: theme.ContentRemoveIcon()}

	RemoveAllNotificationButton = &widget.Button{Text: "Remove All", OnTapped: func() { go removeAllNotificationButtonFunc() }, Icon: theme.DeleteIcon()}

	FreeSpaceLabel = widget.NewLabel("")

	OpenLogButton = &widget.Button{Text: "Open Log", OnTapped: func() { go OpenLog() }, Icon: theme.FolderOpenIcon()}

	RefreshButton = &widget.Button{Text: "Refresh", OnTapped: func() { go Refresh() }, Icon: theme.ViewRefreshIcon()}

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
	CopyEidButton = &widget.Button{Text: "Copy", OnTapped: func() { go copyEidButtonFunc() }, Icon: theme.ContentCopyIcon()}
	CopyEidButton.Hide()
	SetDefaultSmdpButton = &widget.Button{OnTapped: func() { go setDefaultSmdpButtonFunc() }, Icon: theme.DocumentCreateIcon()}
	SetDefaultSmdpButton.Hide()
	ViewCertInfoButton = &widget.Button{Text: "Certificate Issuer", OnTapped: func() { go viewCertInfoButtonFunc() }, Icon: theme.InfoIcon()}
	ViewCertInfoButton.Hide()
	EUICCManufacturerLabel = widget.NewRichText()
	EUICCManufacturerLabel.Hide()
	CopyEuiccInfo2Button = &widget.Button{Text: "Copy eUICCInfo2", OnTapped: func() { go copyEuiccInfo2ButtonFunc() }, Icon: theme.ContentCopyIcon()}
	CopyEuiccInfo2Button.Hide()
	ApduDriverSelect = widget.NewSelect([]string{}, func(s string) { SetDriverIFID(s) })
	ApduDriverRefreshButton = &widget.Button{OnTapped: func() { go RefreshApduDriver() }, Icon: theme.SearchReplaceIcon()}
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

func discoveryButtonFunc() {
	if ConfigInstance.DriverIFID == "" {
		ShowSelectCardReaderDialog()
		return
	}
	if RefreshNeeded == true {
		ShowRefreshNeededDialog()
		return
	}
	discoveryFunc := func() {
		ch := make(chan bool)
		var data []DiscoveryResult
		var err error
		go func() {
			data, err = LpacProfileDiscovery()
			ch <- true
		}()
		<-ch
		if err != nil {
			ShowLpacErrDialog(err)
			return
		}
		if len(data) != 0 {
			var d *dialog.CustomDialog
			selectedProfile := Unselected
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
				return &widget.Label{TextStyle: fyne.TextStyle{Monospace: true}}
			}, func(i widget.ListItemID, o fyne.CanvasObject) {
				o.(*widget.Label).SetText(fmt.Sprintf("%-6s\t\t%s", data[i].EventID, data[i].RspServerAddres))
			})
			discoveredEsimList.OnSelected = func(id widget.ListItemID) {
				selectedProfile = id
			}
			downloadButton := widget.NewButton("Download", func() {
				if selectedProfile == Unselected {
					ShowSelectItemDialog()
				} else {
					d.Hide()
					go LpacProfileDownload(PullInfo{
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
			d := dialog.NewInformation("Result", "No eSIM profile found.\n", WMain)
			d.Show()
		}
	}
	d := dialog.NewInformation("Info", "Discovery has not been actually tested yet.\n"+
		"If you have any discoverable profiles and try to use the discovery function,\n"+
		"regardless of whether it is successful,\n"+
		"please open an issue to report logs and program behavior.\n"+
		"Thank you very much\n", WMain)
	d.SetOnClosed(func() {
		go discoveryFunc()
	})
	d.Show()
}

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
	profileText := fmt.Sprintf("%s\t\t%s",
		Profiles[SelectedProfile].Iccid,
		Profiles[SelectedProfile].ServiceProviderName)
	if Profiles[SelectedProfile].ProfileNickname != nil {
		profileText += fmt.Sprintf("\t\t%s\n\n", Profiles[SelectedProfile].ProfileNickname)
	} else {
		profileText += "\n\n"
	}
	d := dialog.NewCustomConfirm("Confirm",
		"Confirm",
		"Cancel",
		container.NewVBox(container.NewCenter(widget.NewLabel("Are you sure you want to delete this profile?")), &widget.Label{Text: profileText, TextStyle: fyne.TextStyle{Monospace: true}}),
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
	notificationText := fmt.Sprintf("%d\t\t%s\t\t%s\t\t%s\n\n",
		Notifications[SelectedNotification].SeqNumber,
		Notifications[SelectedNotification].Iccid,
		Notifications[SelectedNotification].ProfileManagementOperation,
		Notifications[SelectedNotification].NotificationAddress)
	d := dialog.NewCustomConfirm("Confirm",
		"Confirm",
		"Cancel",
		container.NewVBox(container.NewCenter(widget.NewLabel("Are you sure you want to remove this notification?")),
			&widget.Label{Text: notificationText, TextStyle: fyne.TextStyle{Monospace: true}}),
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
	entry := &widget.Entry{TextStyle: fyne.TextStyle{Monospace: true}, PlaceHolder: "Confirm"}
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
		Country    string
		CommonName string
		KeyID      string
	}
	var ciWidgetEls []ciWidgetEl
	// chipinfo 中 signing 和 verification 同时存在则有效
	for _, keyId := range ChipInfo.EUICCInfo2.EuiccCiPKIDListForSigning {
		if !slices.Contains(ChipInfo.EUICCInfo2.EuiccCiPKIDListForVerification, keyId) {
			continue
		}
		var element ciWidgetEl
		element.CommonName = "Unknown"
		element.KeyID = keyId
		if issuer, found := issuerRegistry[keyId]; found {
			element.Country = issuer.Country
			element.CommonName = issuer.CommonName
		}
		ciWidgetEls = append(ciWidgetEls, element)
	}
	list := &widget.List{
		Length: func() int {
			return len(ciWidgetEls)
		},
		CreateItem: func() fyne.CanvasObject {
			return container.NewVBox(container.NewBorder(nil, nil, &widget.Label{TextStyle: fyne.TextStyle{Monospace: true}}, &widget.Label{}),
				&widget.Label{TextStyle: fyne.TextStyle{Monospace: true}})
		},
		UpdateItem: func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*fyne.Container).Objects[0].(*fyne.Container).Objects[0].(*widget.Label).SetText(fmt.Sprintf("CN: %s", ciWidgetEls[i].CommonName))
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
		} else if issuer, found := issuerRegistry[ciWidgetEls[selectedCI].KeyID]; !found || issuer.Text == "" {
			d := dialog.NewInformation("No Data",
				"The information of this certificate is not included.\n"+
					"If you have any information about this certificate,\n"+
					"you can report it to <euicc-dev-manual@septs.pw>\n"+
					"Thank you",
				WMain)
			d.Show()
		} else {
			entry := NewReadOnlyEntry()
			entry.SetText(issuer.Text)
			w := App.NewWindow(issuer.KeyID)
			w.Resize(fyne.Size{Width: 550, Height: 600})
			w.SetContent(entry)
			w.Show()
		}
	}
	certDataButton := &widget.Button{
		Text:     "Certificate Info",
		OnTapped: certDataButtonFunc,
		Icon:     theme.InfoIcon(),
	}
	d := dialog.NewCustom("Certificate Issuer", "OK", container.NewBorder(nil, container.NewCenter(certDataButton), nil, nil, list), WMain)
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
			spacer := canvas.NewRectangle(color.Transparent)
			spacer.SetMinSize(fyne.NewSize(1, 1))
			iccidLabel := &widget.Label{TextStyle: fyne.TextStyle{Monospace: true, TabWidth: FontTabWidth}}
			stateLabel := &widget.Label{TextStyle: fyne.TextStyle{Monospace: true, TabWidth: FontTabWidth}}
			enabledIcon := widget.NewIcon(theme.ConfirmIcon())
			enabledIcon.Hide()
			stateFillLabel := widget.NewLabel("")
			profileIcon := widget.NewIcon(theme.FileImageIcon())
			stateContainer := container.NewHBox(stateLabel, enabledIcon, spacer, stateFillLabel, profileIcon)
			providerLabel := &widget.Label{TextStyle: fyne.TextStyle{Monospace: true, TabWidth: FontTabWidth}}
			nicknameLabel := &widget.Label{TextStyle: fyne.TextStyle{Monospace: true, TabWidth: FontTabWidth}}
			return container.NewHBox(iccidLabel, stateContainer, providerLabel, nicknameLabel)
		},
		UpdateItem: func(i widget.ListItemID, o fyne.CanvasObject) {
			c := o.(*fyne.Container)
			if ProfileMaskNeeded {
				var iccidMasked string
				for x := 0; x < len(Profiles[i].Iccid); x++ {
					if x < 7 {
						iccidMasked += string(Profiles[i].Iccid[x])
					} else {
						iccidMasked += "*"
					}
				}
				iccidMasked += "\t\t"
				c.Objects[0].(*widget.Label).SetText(iccidMasked)
			} else {
				c.Objects[0].(*widget.Label).SetText(fmt.Sprintf("%s\t\t", Profiles[i].Iccid))
			}
			c.Objects[1].(*fyne.Container).Objects[0].(*widget.Label).SetText(fmt.Sprintf("%s", Profiles[i].ProfileState))
			if Profiles[i].ProfileState == "enabled" {
				c.Objects[1].(*fyne.Container).Objects[1].(*widget.Icon).Show()
				c.Objects[1].(*fyne.Container).Objects[2].(*canvas.Rectangle).SetMinSize(fyne.Size{Width: 24, Height: 1})
				c.Objects[1].(*fyne.Container).Objects[3].(*widget.Label).SetText("\t")
			} else {
				c.Objects[1].(*fyne.Container).Objects[1].(*widget.Icon).Hide()
				c.Objects[1].(*fyne.Container).Objects[2].(*canvas.Rectangle).SetMinSize(fyne.Size{Width: 0, Height: 1})
				c.Objects[1].(*fyne.Container).Objects[3].(*widget.Label).SetText("\t\t")
			}
			if Profiles[i].Icon != nil {
				iconData, err := base64.StdEncoding.DecodeString(Profiles[i].Icon.(string))
				if err == nil {
					// 创建一个 fyne.Resource 对象
					iconResource := fyne.NewStaticResource(Profiles[i].ProfileName, iconData)
					c.Objects[1].(*fyne.Container).Objects[4].(*widget.Icon).SetResource(iconResource)
					// 刷新状态
					c.Objects[1].(*fyne.Container).Objects[4].(*widget.Icon).Show()
					// 重设控件间距
					if Profiles[i].ProfileState == "enabled" {
						c.Objects[1].(*fyne.Container).Objects[2].(*canvas.Rectangle).SetMinSize(fyne.Size{Width: 0, Height: 1})
					} else {
						c.Objects[1].(*fyne.Container).Objects[2].(*canvas.Rectangle).SetMinSize(fyne.Size{Width: 16, Height: 1})
						c.Objects[1].(*fyne.Container).Objects[3].(*widget.Label).SetText("\t")
					}
				}
			} else {
				// 恢复默认图标
				c.Objects[1].(*fyne.Container).Objects[4].(*widget.Icon).SetResource(theme.FileImageIcon())
				c.Objects[1].(*fyne.Container).Objects[4].(*widget.Icon).Hide()
			}
			providerName := Profiles[i].ServiceProviderName
			if Profiles[i].ProfileNickname != nil {
				width := runewidth.StringWidth(Profiles[i].ServiceProviderName)
				tabNum := 6 - width/6
				// Fixme: 使用 Grid Layout 排版
				if width == 23 || width == 29 {
					tabNum -= 1
				}
				for x := 1; x <= tabNum; x++ {
					providerName += "\t"
				}
				c.Objects[2].(*widget.Label).SetText(providerName)
				c.Objects[3].(*widget.Label).SetText(fmt.Sprintf("%s", Profiles[i].ProfileNickname))
			} else {
				c.Objects[2].(*widget.Label).SetText(providerName)
				c.Objects[3].(*widget.Label).SetText("") // 必须刷新
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
			seqLabel := &widget.Label{TextStyle: fyne.TextStyle{Monospace: true, TabWidth: FontTabWidth}}
			iccidLabel := &widget.Label{TextStyle: fyne.TextStyle{Monospace: true, TabWidth: FontTabWidth}}
			operationLabel := &widget.Label{TextStyle: fyne.TextStyle{Monospace: true, TabWidth: FontTabWidth}}
			notificationAddress := &widget.Label{TextStyle: fyne.TextStyle{Monospace: true, TabWidth: FontTabWidth}}
			return container.NewHBox(seqLabel, iccidLabel, operationLabel, notificationAddress)
		},
		UpdateItem: func(i widget.ListItemID, o fyne.CanvasObject) {
			var iccid, notificationAddress string
			maskFQDNExceptPublicSuffix := func(fqdn string) string {
				suffix, _ := publicsuffix.PublicSuffix(fqdn)
				parts := strings.Split(fqdn, ".")
				suffixParts := strings.Split(suffix, ".")
				// 如果域名部分少于后缀部分，说明域名不合法或者是一个裸域名，直接返回掩码后的顶级域名
				if len(parts) <= len(suffixParts) {
					return strings.Repeat("x", len(parts[0])) + "." + suffix
				}
				// 掩盖除了后缀之外的所有部分
				for i := 0; i < len(parts)-len(suffixParts); i++ {
					parts[i] = strings.Repeat("x", len(parts[i]))
				}
				return strings.Join(parts, ".")
			}
			if NotificationMaskNeeded {
				for x := 0; x < len(Notifications[i].Iccid); x++ {
					if x < 7 {
						iccid += string(Notifications[i].Iccid[x])
					} else {
						{
							iccid += "*"
						}
					}
				}
				notificationAddress = maskFQDNExceptPublicSuffix(Notifications[i].NotificationAddress)
			} else {
				iccid = Notifications[i].Iccid
				notificationAddress = Notifications[i].NotificationAddress
			}
			o.(*fyne.Container).Objects[0].(*widget.Label).SetText(fmt.Sprintf("%-6d\t", Notifications[i].SeqNumber))
			if iccid == "" {
				o.(*fyne.Container).Objects[1].(*widget.Label).SetText(fmt.Sprintf("No ICCID!\t\t\t\t"))
			} else {
				o.(*fyne.Container).Objects[1].(*widget.Label).SetText(fmt.Sprintf("%s\t\t", iccid))
			}
			o.(*fyne.Container).Objects[2].(*widget.Label).SetText(fmt.Sprintf("%s\t\t\t", Notifications[i].ProfileManagementOperation))
			o.(*fyne.Container).Objects[3].(*widget.Label).SetText(fmt.Sprintf("%s", notificationAddress))
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
		notificationText := fmt.Sprintf("%d\t\t%s\t\t%s\t\t%s\n",
			notification.SeqNumber,
			notification.Iccid,
			notification.ProfileManagementOperation,
			notification.NotificationAddress)
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
					&widget.Label{Text: "Successfully processed notification.", Alignment: fyne.TextAlignCenter},
					&widget.Label{Text: "Do you want to remove this notification now?", Alignment: fyne.TextAlignCenter},
					&widget.Label{Text: notificationText, TextStyle: fyne.TextStyle{Monospace: true, TabWidth: FontTabWidth}})),
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
