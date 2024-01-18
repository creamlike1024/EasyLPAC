package main

import (
	"encoding/base64"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/atotto/clipboard"
	"github.com/mattn/go-runewidth"
	"golang.org/x/net/publicsuffix"
	"image/color"
	"strings"
	"time"
)

var StatusProcessBar *widget.ProgressBarInfinite
var StatusLabel *widget.Label
var SetNicknameButton *widget.Button
var DownloadButton *widget.Button
var DiscoveryButton *widget.Button
var DeleteButton *widget.Button
var SwitchStateButton *widget.Button
var ProcessNotificationButton *widget.Button
var RemoveNotificationButton *widget.Button

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
	widget.ShowPopUpMenuAtPosition(menu, fyne.CurrentApp().Driver().AllWindows()[0].Canvas(), ev.AbsolutePosition)
}

func NewReadOnlyEntry() *ReadOnlyEntry {
	entry := &ReadOnlyEntry{}
	entry.ExtendBaseWidget(entry) // 确保自定义的 widget 被正确地初始化
	entry.MultiLine = true        // 支持多行文本
	entry.TextStyle = fyne.TextStyle{Italic: true}
	entry.Wrapping = fyne.TextWrapOff
	return entry
}

func InitWidgets() {
	StatusProcessBar = widget.NewProgressBarInfinite()
	StatusProcessBar.Stop()
	StatusProcessBar.Hide()

	StatusLabel = widget.NewLabel("Ready.")

	DownloadButton = &widget.Button{Text: "Download", OnTapped: downloadButtonFunc, Icon: theme.DownloadIcon()}

	DiscoveryButton = &widget.Button{Text: "Discovery", OnTapped: discoveryButtonFunc, Icon: theme.SearchIcon()}

	SetNicknameButton = &widget.Button{Text: "Nickname", OnTapped: setNicknameButtonFunc, Icon: theme.DocumentCreateIcon()}

	DeleteButton = &widget.Button{Text: "Delete", OnTapped: deleteButtonFunc, Icon: theme.DeleteIcon()}

	SwitchStateButton = &widget.Button{Text: "Enable", OnTapped: switchStateButtonFunc, Icon: theme.ConfirmIcon()}

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

	ProcessNotificationButton = &widget.Button{Text: "Process", OnTapped: processNotificationButtonFunc, Icon: theme.MediaPlayIcon()}

	RemoveNotificationButton = &widget.Button{Text: "Remove", OnTapped: removeNotificationButtonFunc, Icon: theme.DeleteIcon()}

	FreeSpaceLabel = widget.NewLabel("")

	OpenLogButton = &widget.Button{Text: "Open Log", OnTapped: OpenLog, Icon: theme.FolderOpenIcon()}

	RefreshButton = &widget.Button{Text: "Refresh", OnTapped: Refresh, Icon: theme.ViewRefreshIcon()}

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
	CopyEidButton = &widget.Button{Text: "Copy", OnTapped: copyEidButtonFunc, Icon: theme.ContentCopyIcon()}
	CopyEidButton.Hide()
	SetDefaultSmdpButton = &widget.Button{OnTapped: setDefaultSmdpButtonFunc, Icon: theme.DocumentCreateIcon()}
	SetDefaultSmdpButton.Hide()
	ApduDriverSelect = widget.NewSelect([]string{}, func(s string) {
		SetDriverIfid(s)
	})
	ApduDriverRefreshButton = &widget.Button{OnTapped: func() { RefreshApduDriver() }, Icon: theme.SearchReplaceIcon()}
}

func downloadButtonFunc() {
	if ConfigInstance.DriverIFID == "" {
		SelectCardReaderDialog()
		return
	}
	d := InitDownloadDialog()
	d.Show()
}

func discoveryButtonFunc() {
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
			return &widget.Label{TextStyle: fyne.TextStyle{Monospace: true}}
		}, func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(fmt.Sprintf("%-6s\t\t%s", data[i].EventID, data[i].RspServerAddres))
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

func switchStateButtonFunc() {
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
	if ProfileStateAllowDisable {
		if err := LpacProfileDisable(Profiles[SelectedProfile].Iccid); err != nil {
			ErrDialog(err)
		}
	} else {
		if err := LpacProfileEnable(Profiles[SelectedProfile].Iccid); err != nil {
			ErrDialog(err)
		}
	}
	RefreshProfile()
	RefreshNotification()
	RefreshChipInfo()
	if ProfileStateAllowDisable {
		SwitchStateButton.SetText("Enable")
		SwitchStateButton.SetIcon(theme.ConfirmIcon())
	}
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
		notificationText := fmt.Sprintf("%d\t\t%s\t\t%s\t\t%s\n\n",
			Notifications[SelectedNotification].SeqNumber,
			Notifications[SelectedNotification].Iccid,
			Notifications[SelectedNotification].ProfileManagementOperation,
			Notifications[SelectedNotification].NotificationAddress)
		d := dialog.NewCustomConfirm("Remove Notification",
			"Remove",
			"Not Now",
			container.NewVBox(
				&widget.Label{Text: "Successfully processed notification.", Alignment: fyne.TextAlignCenter},
				&widget.Label{Text: "Do you want to remove this notification now?", Alignment: fyne.TextAlignCenter},
				&widget.Label{Text: notificationText, TextStyle: fyne.TextStyle{Monospace: true}}),
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
	return &widget.List{
		Length: func() int {
			return len(Profiles)
		},
		CreateItem: func() fyne.CanvasObject {
			spacer := canvas.NewRectangle(color.Transparent)
			spacer.SetMinSize(fyne.NewSize(1, 1))
			iccidLabel := &widget.Label{TextStyle: fyne.TextStyle{Monospace: true}}
			stateLabel := &widget.Label{TextStyle: fyne.TextStyle{Monospace: true}}
			enabledIcon := widget.NewIcon(theme.ConfirmIcon())
			enabledIcon.Hide()
			stateFillLabel := widget.NewLabel("")
			profileIcon := widget.NewIcon(theme.FileImageIcon())
			stateContainer := container.NewHBox(stateLabel, enabledIcon, spacer, stateFillLabel, profileIcon)
			providerLabel := &widget.Label{TextStyle: fyne.TextStyle{Monospace: true}}
			nicknameLabel := &widget.Label{TextStyle: fyne.TextStyle{Monospace: true}}
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
				// tab space 随字体变化
				width := runewidth.StringWidth(Profiles[i].ServiceProviderName)
				tabNum := 6 - width/6
				// Fixme: 使用更合理的方法排版
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
		}}
}

func initNotificationList() *widget.List {
	return &widget.List{
		Length: func() int {
			return len(Notifications)
		},
		CreateItem: func() fyne.CanvasObject {
			seqLabel := &widget.Label{TextStyle: fyne.TextStyle{Monospace: true}}
			iccidLabel := &widget.Label{TextStyle: fyne.TextStyle{Monospace: true}}
			operationLabel := &widget.Label{TextStyle: fyne.TextStyle{Monospace: true}}
			notificationAddress := &widget.Label{TextStyle: fyne.TextStyle{Monospace: true}}
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
			o.(*fyne.Container).Objects[1].(*widget.Label).SetText(fmt.Sprintf("%s\t\t", iccid))
			o.(*fyne.Container).Objects[2].(*widget.Label).SetText(fmt.Sprintf("%s\t\t\t", Notifications[i].ProfileManagementOperation))
			o.(*fyne.Container).Objects[3].(*widget.Label).SetText(fmt.Sprintf("%s", notificationAddress))
		},
		OnSelected: func(id widget.ListItemID) {
			SelectedNotification = id
		}}
}
