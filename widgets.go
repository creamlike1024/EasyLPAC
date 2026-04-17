package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	fyne "fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/fullpipe/icu-mf/mf"
	"golang.org/x/net/publicsuffix"
)

var StatusProcessBar *widget.ProgressBarInfinite
var StatusLabel *widget.Label
var SetNicknameButton *widget.Button
var DownloadButton *widget.Button
var DeleteProfileButton *widget.Button
var SwitchStateButton *widget.Button
var ProcessNotificationButton *widget.Button
var ProcessAllNotificationButton *widget.Button
var RemoveNotificationButton *widget.Button
var BatchRemoveNotificationButton *widget.Button

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

// Driver selection widgets
var ApduBackendSelect *widget.Select
var DriverConfigContainer *fyne.Container

// Driver-specific config widgets (created dynamically)
var DeviceSelect *widget.Select       // For drivers with enumeration (pcsc, at)
var DeviceSelectRefresh *widget.Button
var DeviceEntry *widget.Entry         // For drivers with device path (mbim, qmi, at_csim)
var UimSlotEntry *widget.Entry        // For drivers with UIM slot (mbim, qmi)

var Tabs *container.AppTabs
var ProfileTab *container.TabItem
var NotificationTab *container.TabItem
var ChipInfoTab *container.TabItem
var SettingsTab *container.TabItem
var AboutTab *container.TabItem

var LpacVersionLabel *widget.Label

type ReadOnlyEntry struct{ widget.Entry }

func (entry *ReadOnlyEntry) TypedRune(_ rune)          {}
func (entry *ReadOnlyEntry) TypedKey(_ *fyne.KeyEvent) {}
func (entry *ReadOnlyEntry) TypedShortcut(shortcut fyne.Shortcut) {
	switch shortcut := shortcut.(type) {
	case *fyne.ShortcutCopy:
		entry.Entry.TypedShortcut(shortcut)
	}
}

func (entry *ReadOnlyEntry) TappedSecondary(ev *fyne.PointEvent) {
	c := fyne.CurrentApp().Driver().AllWindows()[0].Clipboard()
	copyItem := fyne.NewMenuItem(TR.Trans("label.menu_copy"), func() {
		c.SetContent(entry.SelectedText())
	})
	menu := fyne.NewMenu("", copyItem)
	widget.ShowPopUpMenuAtPosition(menu, fyne.CurrentApp().Driver().CanvasForObject(entry), ev.AbsolutePosition)
}

func NewReadOnlyEntry() *ReadOnlyEntry {
	entry := &ReadOnlyEntry{}
	entry.ExtendBaseWidget(entry)
	entry.MultiLine = true
	entry.TextStyle = fyne.TextStyle{Monospace: true}
	entry.Wrapping = fyne.TextWrapOff
	return entry
}

func InitWidgets() {
	StatusProcessBar = widget.NewProgressBarInfinite()
	StatusProcessBar.Stop()
	StatusProcessBar.Hide()

	StatusLabel = widget.NewLabel(TR.Trans("label.status_ready"))

	DownloadButton = &widget.Button{Text: TR.Trans("label.download_profile_button"),
		OnTapped: func() { go downloadButtonFunc() },
		Icon:     theme.DownloadIcon()}

	SetNicknameButton = &widget.Button{Text: TR.Trans("label.set_nickname_button"),
		OnTapped: func() { go setNicknameButtonFunc() },
		Icon:     theme.DocumentCreateIcon()}

	DeleteProfileButton = &widget.Button{Text: TR.Trans("label.delete_profile_button"),
		OnTapped: func() { go deleteProfileButtonFunc() },
		Icon:     theme.DeleteIcon()}

	SwitchStateButton = &widget.Button{Text: TR.Trans("label.switch_state_button_enable"),
		OnTapped: func() { go switchStateButtonFunc() },
		Icon:     theme.ConfirmIcon()}

	ProfileList = initProfileList()
	NotificationList = initNotificationList()

	ProcessNotificationButton = &widget.Button{Text: TR.Trans("label.process_notification_button"),
		OnTapped: func() { go processNotificationButtonFunc() },
		Icon:     theme.MediaPlayIcon()}

	ProcessAllNotificationButton = &widget.Button{Text: TR.Trans("label.process_all_notification_button"),
		OnTapped: func() { go processAllNotificationButtonFunc() },
		Icon:     theme.MediaReplayIcon()}

	RemoveNotificationButton = &widget.Button{Text: TR.Trans("label.remove_notification_button"),
		OnTapped: func() { go removeNotificationButtonFunc() },
		Icon:     theme.ContentRemoveIcon()}

	BatchRemoveNotificationButton = &widget.Button{Text: TR.Trans("label.batch_remove_notification_button"),
		OnTapped: func() { go batchRemoveNotificationButtonFunc() },
		Icon:     theme.DeleteIcon()}

	FreeSpaceLabel = widget.NewLabel("")

	OpenLogButton = &widget.Button{Text: TR.Trans("label.open_log_button"),
		OnTapped: func() { go OpenLog() },
		Icon:     theme.FolderOpenIcon()}

	RefreshButton = &widget.Button{Text: TR.Trans("label.refresh_button"),
		OnTapped: func() { go Refresh() },
		Icon:     theme.ViewRefreshIcon()}

	ProfileMaskCheck = widget.NewCheck(TR.Trans("label.profile_mask_check"), func(b bool) {
		ProfileMaskNeeded = b
		ProfileList.Refresh()
	})
	NotificationMaskCheck = widget.NewCheck(TR.Trans("label.notification_mask_check"), func(b bool) {
		NotificationMaskNeeded = b
		NotificationList.Refresh()
	})

	EidLabel = widget.NewLabel("")
	DefaultDpAddressLabel = widget.NewLabel("")
	RootDsAddressLabel = widget.NewLabel("")
	EuiccInfo2Entry = NewReadOnlyEntry()
	EuiccInfo2Entry.Hide()
	CopyEidButton = &widget.Button{Text: TR.Trans("label.copy_eid_button"),
		OnTapped: func() { go copyEidButtonFunc() },
		Icon:     theme.ContentCopyIcon()}
	CopyEidButton.Hide()
	SetDefaultSmdpButton = &widget.Button{OnTapped: func() { go setDefaultSmdpButtonFunc() },
		Icon: theme.DocumentCreateIcon()}
	SetDefaultSmdpButton.Hide()
	ViewCertInfoButton = &widget.Button{Text: TR.Trans("label.view_cert_info_button"),
		OnTapped: func() { go viewCertInfoButtonFunc() },
		Icon:     theme.InfoIcon()}
	ViewCertInfoButton.Hide()
	EUICCManufacturerLabel = &widget.Label{}
	EUICCManufacturerLabel.Hide()
	CopyEuiccInfo2Button = &widget.Button{Text: TR.Trans("label.copy_euicc_info2_button"),
		OnTapped: func() { go copyEuiccInfo2ButtonFunc() },
		Icon:     theme.ContentCopyIcon()}
	CopyEuiccInfo2Button.Hide()
	LpacVersionLabel = &widget.Label{}

	// Initialize driver config widgets
	DeviceSelect = widget.NewSelect([]string{}, func(s string) {
		onDeviceSelected(s)
	})
	DeviceSelectRefresh = &widget.Button{
		OnTapped: func() { go RefreshDeviceList() },
		Icon:     theme.SearchReplaceIcon(),
	}
	DeviceEntry = &widget.Entry{
		OnChanged: func(s string) {
			onDevicePathChanged(s)
		},
	}
	UimSlotEntry = &widget.Entry{
		OnChanged: func(s string) {
			// Filter to numeric only and enforce minimum of 1
			filtered := filterNumeric(s)
			if filtered != s {
				UimSlotEntry.SetText(filtered)
				return
			}
			onUimSlotChanged(filtered)
		},
	}
	UimSlotEntry.SetText("1")

	// Initialize backend selector (will be populated after driver discovery)
	ApduBackendSelect = widget.NewSelect([]string{}, func(s string) {
		onBackendSelected(s)
	})

	// Container for driver-specific config (populated dynamically)
	DriverConfigContainer = container.NewHBox()
}

// onBackendSelected handles backend driver selection
func onBackendSelected(driverName string) {
	ConfigInstance.ApduBackend = driverName
	RefreshNeeded = true
	updateDriverConfigUI()
}

// onDeviceSelected handles device selection from dropdown (pcsc, at)
func onDeviceSelected(deviceName string) {
	if deviceName == "" {
		return
	}
	config := GetCurrentDriverConfig()
	if config == nil {
		return
	}

	// Find the env value for this device name
	for _, d := range ApduDrivers {
		if d.Name == deviceName {
			switch ConfigInstance.ApduBackend {
			case "pcsc":
				// Env is a reader index; store it alongside the full name.
				// Both are needed to work around the lpac 2.x name-filter bug.
				config.DriverIFID = d.Env
				config.DriverName = d.Name
			default:
				// For "at" and any future enumeration drivers, Env is a device path.
				config.DevicePath = d.Env
			}
			SetCurrentDriverConfig(*config)
			RefreshNeeded = true
			return
		}
	}
}

// onDevicePathChanged handles device path entry changes
func onDevicePathChanged(path string) {
	config := GetCurrentDriverConfig()
	if config == nil {
		return
	}
	config.DevicePath = path
	SetCurrentDriverConfig(*config)
	RefreshNeeded = true
}

// onUimSlotChanged handles UIM slot entry changes
func onUimSlotChanged(slotStr string) {
	config := GetCurrentDriverConfig()
	if config == nil {
		return
	}
	if slotStr == "" {
		config.UimSlot = 1
		SetCurrentDriverConfig(*config)
		RefreshNeeded = true
		return
	}
	if slot, err := strconv.Atoi(slotStr); err == nil && slot > 0 {
		config.UimSlot = slot
		SetCurrentDriverConfig(*config)
		RefreshNeeded = true
	}
}

// filterNumeric filters a string to only contain digits, returns "1" if empty or zero
func filterNumeric(s string) string {
	var result strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			result.WriteRune(r)
		}
	}
	filtered := result.String()
	// Remove leading zeros but keep at least one digit
	filtered = strings.TrimLeft(filtered, "0")
	if filtered == "" {
		return "1"
	}
	return filtered
}

// updateDriverConfigUI updates the driver config UI based on selected backend
func updateDriverConfigUI() {
	if DriverConfigContainer == nil {
		return
	}

	driver := ConfigInstance.ApduBackend
	config := GetCurrentDriverConfig()

	// Clear current config UI
	DriverConfigContainer.Objects = nil

	// Skip unknown drivers
	if !IsKnownDriver(driver) {
		DriverConfigContainer.Refresh()
		return
	}

	if DriversNoConfig[driver] {
		// No config needed for gbinder, gbinder_hidl
		DriverConfigContainer.Objects = []fyne.CanvasObject{
			widget.NewLabel(TR.Trans("label.no_config_needed")),
		}
		DriverConfigContainer.Refresh()
		return
	}

	if DriversWithEnumeration[driver] {
		// Show device dropdown with refresh button
		DeviceSelect.ClearSelected()
		DeviceSelect.SetOptions([]string{})
		if config != nil && config.DriverIFID != "" {
			// Try to find and select the current device
			for _, d := range ApduDrivers {
				if d.Env == config.DriverIFID {
					DeviceSelect.SetSelected(d.Name)
					break
				}
			}
		}

		DriverConfigContainer.Objects = []fyne.CanvasObject{
			widget.NewLabel(TR.Trans("label.device")),
			container.NewGridWrap(fyne.Size{Width: 280, Height: DeviceSelect.MinSize().Height}, DeviceSelect),
			DeviceSelectRefresh,
		}

		// Auto-refresh device list
		go RefreshDeviceList()
	} else if DriversWithDevicePath[driver] {
		// Show device path entry - only set text if explicitly configured
		if config != nil && config.DevicePath != "" {
			DeviceEntry.SetText(config.DevicePath)
		} else {
			DeviceEntry.SetText("")
		}
		DeviceEntry.SetPlaceHolder(GetDefaultDevicePath(driver))

		objects := []fyne.CanvasObject{
			widget.NewLabel(TR.Trans("label.device")),
			container.NewGridWrap(fyne.Size{Width: 200, Height: DeviceEntry.MinSize().Height}, DeviceEntry),
		}

		// Add UIM slot for drivers that need it
		if DriversWithUimSlot[driver] {
			if config != nil && config.UimSlot > 0 {
				UimSlotEntry.SetText(strconv.Itoa(config.UimSlot))
			} else {
				UimSlotEntry.SetText("1")
			}
			UimSlotEntry.SetPlaceHolder("1")
			objects = append(objects,
				widget.NewLabel(TR.Trans("label.uim_slot")),
				container.NewGridWrap(fyne.Size{Width: 50, Height: UimSlotEntry.MinSize().Height}, UimSlotEntry),
			)
		}
		DriverConfigContainer.Objects = objects
	}

	DriverConfigContainer.Refresh()
}

// PopulateBackendSelect populates the backend selector with available drivers
func PopulateBackendSelect() {
	var options []string
	for _, driver := range AvailableDrivers {
		// Skip unknown drivers
		if !IsKnownDriver(driver) {
			continue
		}
		options = append(options, driver)
	}
	ApduBackendSelect.SetOptions(options)

	// Select first available driver if none selected
	if ConfigInstance.ApduBackend == "" && len(options) > 0 {
		// Prefer pcsc if available
		for _, opt := range options {
			if opt == "pcsc" {
				ApduBackendSelect.SetSelected("pcsc")
				return
			}
		}
		ApduBackendSelect.SetSelected(options[0])
	} else if ConfigInstance.ApduBackend != "" {
		ApduBackendSelect.SetSelected(ConfigInstance.ApduBackend)
	}
}

// RefreshDeviceList refreshes the device list for drivers with enumeration
func RefreshDeviceList() {
	driver := ConfigInstance.ApduBackend
	if !DriversWithEnumeration[driver] {
		return
	}

	var err error
	ApduDrivers, err = LpacDriverApduListForDriver(driver)
	if err != nil {
		ShowLpacErrDialog(err)
		return
	}

	var options []string
	for _, d := range ApduDrivers {
		// Exclude YubiKey and CanoKey
		if strings.Contains(d.Name, "canokeys.org") || strings.Contains(d.Name, "YubiKey") {
			continue
		}
		// Workaround: lpac shows an empty driver when no card reader inserted under macOS
		if d.Name == "" {
			continue
		}
		options = append(options, d.Name)
	}

	fyne.Do(func() {
		DeviceSelect.SetOptions(options)
		DeviceSelect.ClearSelected()

		// Clear the driver IFID since list changed
		config := GetCurrentDriverConfig()
		if config != nil {
			config.DriverIFID = ""
			SetCurrentDriverConfig(*config)
		}
		DeviceSelect.Refresh()
	})
}

// isApduConfigured checks if the current driver is properly configured
func isApduConfigured() bool {
	driver := ConfigInstance.ApduBackend
	if driver == "" {
		return false
	}

	config := GetCurrentDriverConfig()
	if config == nil {
		return false
	}

	if DriversNoConfig[driver] {
		return true
	}

	if DriversWithEnumeration[driver] {
		return config.DriverIFID != ""
	}

	if DriversWithDevicePath[driver] {
		return config.DevicePath != ""
	}

	return false
}

// showApduNotConfiguredDialog shows appropriate dialog based on driver type
func showApduNotConfiguredDialog() {
	driver := ConfigInstance.ApduBackend
	if driver == "" {
		ShowSelectBackendDialog()
		return
	}

	if DriversWithEnumeration[driver] {
		ShowSelectCardReaderDialog()
	} else if DriversWithDevicePath[driver] {
		ShowEnterDevicePathDialog()
	}
}

func downloadButtonFunc() {
	if !isApduConfigured() {
		showApduNotConfiguredDialog()
		return
	}
	if RefreshNeeded {
		ShowRefreshNeededDialog()
		return
	}
	InitDownloadDialog().Show()
}

func setNicknameButtonFunc() {
	if !isApduConfigured() {
		showApduNotConfiguredDialog()
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
	InitSetNicknameDialog().Show()
}

func deleteProfileButtonFunc() {
	if !isApduConfigured() {
		showApduNotConfiguredDialog()
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
		d := dialog.NewInformation(TR.Trans("dialog.hint"), TR.Trans("message.disable_profile_before_delete"), WMain)
		d.Resize(fyne.Size{
			Width:  360,
			Height: 170,
		})
		d.Show()
		return
	}
	profileText := fmt.Sprint(
		TR.Trans("label.info_iccid")+" ", Profiles[SelectedProfile].Iccid, "\n",
		TR.Trans("label.info_provider")+" ", Profiles[SelectedProfile].ServiceProviderName, "\n",
	)
	if Profiles[SelectedProfile].ProfileNickname != nil {
		profileText += fmt.Sprint(TR.Trans("label.info_nickname")+" ", *Profiles[SelectedProfile].ProfileNickname, "\n")
	}
	dialog.ShowCustomConfirm(TR.Trans("dialog.confirm"),
		TR.Trans("dialog.confirm"),
		TR.Trans("dialog.cancel"),
		container.NewVBox(container.NewCenter(widget.NewLabel(TR.Trans("message.delete_profile_confirm"))),
			&widget.Label{Text: profileText}),
		func(b bool) {
			if b {
				go func() {
					if err := LpacProfileDelete(Profiles[SelectedProfile].Iccid); err != nil {
						ShowLpacErrDialog(err)
						Refresh()
					} else {
						notificationOrigin := Notifications
						Refresh()
						deleteNotification := findNewNotification(notificationOrigin, Notifications)
						if deleteNotification == nil {
							dialog.ShowError(errors.New(TR.Trans("message.notification_not_found")), WMain)
							return
						}
						if ConfigInstance.AutoMode {
							if err2 := LpacNotificationProcess(deleteNotification.SeqNumber, false); err2 != nil {
								dialog.ShowError(errors.New(TR.Trans("message.successfully_delete_profile_failed_send_notification")), WMain)
							} else {
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
											if err3 := LpacNotificationRemove(deleteNotification.SeqNumber); err3 != nil {
												ShowLpacErrDialog(err3)
											}
											if err3 := RefreshNotification(); err3 != nil {
												ShowLpacErrDialog(err3)
												return
											}
											if err3 := RefreshChipInfo(); err3 != nil {
												ShowLpacErrDialog(err3)
												return
											}
										}()
									},
								}
								d = dialog.NewCustomWithoutButtons(TR.Trans("dialog.delete_profile_remove_notification"),
									container.NewBorder(
										nil,
										container.NewCenter(container.NewHBox(notNowButton, spacer, removeButton)),
										nil,
										nil,
										container.NewVBox(
											&widget.Label{Text: TR.Trans("message.successfully_delete_profile_ask_remove_notification"),
												Alignment: fyne.TextAlignCenter},
											&widget.Label{Text: fmt.Sprintf(TR.Trans("label.info_seq")+" %d\n"+
												TR.Trans("label.info_iccid")+" %s\n"+
												TR.Trans("label.info_operation")+" %s\n"+
												TR.Trans("label.info_address")+" %s\n",
												deleteNotification.SeqNumber, deleteNotification.Iccid,
												deleteNotification.ProfileManagementOperation, deleteNotification.NotificationAddress)})),
									WMain)
								d.Show()
							}
						} else {
							dialog.ShowConfirm(TR.Trans("dialog.delete_profile_successfully"),
								TR.Trans("dialog.successfully_delete_profile_ask_send_notification"),
								func(b bool) {
									if b {
										go processNotificationManually(deleteNotification.SeqNumber)
									}
								},
								WMain)
						}
					}
				}()
			}
		}, WMain)
}

func switchStateButtonFunc() {
	if !isApduConfigured() {
		showApduNotConfiguredDialog()
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
	if ConfigInstance.AutoMode {
		notificationsOrigin := Notifications
		Refresh()
		switchNotifications := findNewNotifications(notificationsOrigin, Notifications)
		if switchNotifications == nil || len(switchNotifications) > 2 {
			dialog.ShowError(errors.New(TR.Trans("message.notification_not_found")), WMain)
		} else {
			dialogText := TR.Trans("message.successfully_enable_profile") + "\n"
			var hasError bool
			for _, notification := range switchNotifications {
				if err2 := LpacNotificationProcess(notification.SeqNumber, true); err2 != nil {
					hasError = true
					switch notification.ProfileManagementOperation {
					case "enable":
						dialogText += TR.Trans("message.failed_process_enable_notification") + "\n"
					case "disable":
						dialogText += TR.Trans("message.failed_process_disable_notification") + "\n"
					}
				}
			}
			if hasError {
				dialog.ShowError(errors.New(dialogText), WMain)
			}
		}
	}
	Refresh()
	if ProfileStateAllowDisable {
		SwitchStateButton.SetText(TR.Trans("label.switch_state_button_enable"))
		SwitchStateButton.SetIcon(theme.ConfirmIcon())
	}
}

func processNotificationButtonFunc() {
	if !isApduConfigured() {
		showApduNotConfiguredDialog()
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
	go processNotificationManually(seq)
}

func processAllNotificationButtonFunc() {
	if !isApduConfigured() {
		showApduNotConfiguredDialog()
		return
	}
	if RefreshNeeded {
		ShowRefreshNeededDialog()
		return
	}
	config := map[string]bool{
		"enable":  true,
		"disable": true,
		"install": true,
		"delete":  false,
	}
	enableCheck := &widget.Check{
		Text:    TR.Trans("label.notification_operation_enable"),
		Checked: true,
		OnChanged: func(b bool) {
			config["enable"] = b
		},
	}
	disableCheck := &widget.Check{
		Text:    TR.Trans("label.notification_operation_disable"),
		Checked: true,
		OnChanged: func(b bool) {
			config["disable"] = b
		},
	}
	installCheck := &widget.Check{
		Text:    TR.Trans("label.notification_operation_install"),
		Checked: true,
		OnChanged: func(b bool) {
			config["install"] = b
		},
	}
	deleteCheck := &widget.Check{
		Text:    TR.Trans("label.notification_operation_delete"),
		Checked: false,
		OnChanged: func(b bool) {
			config["delete"] = b
		},
	}
	fyne.Do(func() {
		dialog.ShowCustomConfirm(TR.Trans("dialog.process_all_notification"),
			TR.Trans("dialog.ok"),
			TR.Trans("dialog.cancel"),
			container.NewVBox(
				&widget.Label{Text: TR.Trans("message.select_remove_notification_type")},
				enableCheck,
				disableCheck,
				installCheck,
				deleteCheck,
			),
			func(b bool) {
				if b {
					go func() {
						total := len(Notifications)
						var count int
						for _, notification := range Notifications {
							switch notification.ProfileManagementOperation {
							case "enable":
								if err := LpacNotificationProcess(notification.SeqNumber, config["enable"]); err != nil {
									count++
								}
							case "disable":
								if err := LpacNotificationProcess(notification.SeqNumber, config["disable"]); err != nil {
									count++
								}
							case "install":
								if err := LpacNotificationProcess(notification.SeqNumber, config["install"]); err != nil {
									count++
								}
							case "delete":
								if err := LpacNotificationProcess(notification.SeqNumber, config["delete"]); err != nil {
									count++
								}
							}
						}
						if err := RefreshNotification(); err != nil {
							ShowLpacErrDialog(err)
						}
						fyne.Do(func() {
							dialog.ShowCustom(TR.Trans("dialog.process_all_notification_finished"),
								"OK",
								&widget.Label{Text: TR.Trans("message.process_all_notification_result",
									mf.Arg("total", total),
									mf.Arg("success", total-count),
									mf.Arg("fail", count))},
								WMain)
						})
					}()
				}
			}, WMain)
	})
}

func removeNotificationButtonFunc() {
	if !isApduConfigured() {
		showApduNotConfiguredDialog()
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
	dialog.ShowCustomConfirm(TR.Trans("dialog.confirm"),
		TR.Trans("dialog.confirm"),
		TR.Trans("dialog.cancel"),
		&widget.Label{Text: TR.Trans("message.remove_notification_confirm") + "\n",
			Alignment: fyne.TextAlignCenter},
		func(b bool) {
			if b {
				if err := LpacNotificationRemove(Notifications[SelectedNotification].SeqNumber); err != nil {
					ShowLpacErrDialog(err)
				}

				if err := RefreshNotification(); err != nil {
					ShowLpacErrDialog(err)
					return
				}

				if err := RefreshChipInfo(); err != nil {
					ShowLpacErrDialog(err)
					return
				}
			}
		}, WMain)
}

func batchRemoveNotificationButtonFunc() {
	if !isApduConfigured() {
		showApduNotConfiguredDialog()
		return
	}
	if RefreshNeeded {
		ShowRefreshNeededDialog()
		return
	}
	config := map[string]bool{
		"enable":  true,
		"disable": true,
		"install": true,
		"delete":  false,
	}
	enableCheck := &widget.Check{
		Text:    TR.Trans("label.notification_operation_enable"),
		Checked: true,
		OnChanged: func(b bool) {
			config["enable"] = b
		},
	}
	disableCheck := &widget.Check{
		Text:    TR.Trans("label.notification_operation_disable"),
		Checked: true,
		OnChanged: func(b bool) {
			config["disable"] = b
		},
	}
	installCheck := &widget.Check{
		Text:    TR.Trans("label.notification_operation_install"),
		Checked: true,
		OnChanged: func(b bool) {
			config["install"] = b
		},
	}
	deleteCheck := &widget.Check{
		Text:    TR.Trans("label.notification_operation_delete"),
		Checked: false,
		OnChanged: func(b bool) {
			config["delete"] = b
		},
	}
	fyne.Do(func() {
		dialog.ShowCustomConfirm(TR.Trans("dialog.batch_remove_notification"),
			TR.Trans("dialog.confirm"),
			TR.Trans("dialog.cancel"),
			container.NewVBox(
				&widget.Label{Text: TR.Trans("message.select_batch_remove_notification_type")},
				enableCheck,
				disableCheck,
				installCheck,
				deleteCheck),
			func(b bool) {
				if b {
					go func() {
						var failedCount int
						var total int
						for _, notification := range Notifications {
							switch notification.ProfileManagementOperation {
							case "enable":
								if err := LpacNotificationRemove(notification.SeqNumber); err != nil {
									failedCount++
								}
								total++
							case "disable":
								if err := LpacNotificationProcess(notification.SeqNumber, config["disable"]); err != nil {
									failedCount++
								}
								total++
							case "install":
								if err := LpacNotificationProcess(notification.SeqNumber, config["install"]); err != nil {
									failedCount++
								}
								total++
							case "delete":
								if err := LpacNotificationProcess(notification.SeqNumber, config["delete"]); err == nil {
									failedCount++
								}
								total++
							}
						}
						if err := RefreshNotification(); err != nil {
							ShowLpacErrDialog(err)
						}
						fyne.Do(func() {
							dialog.ShowCustom(TR.Trans("dialog.batch_remove_notification_finished"),
								TR.Trans("dialog.ok"),
								&widget.Label{Text: TR.Trans("message.batch_remove_notification_result",
									mf.Arg("total", total),
									mf.Arg("success", total-failedCount),
									mf.Arg("fail", failedCount))},
								WMain)
						})
					}()
				}
			}, WMain)
	})
}

func copyEidButtonFunc() {
	WMain.Clipboard().SetContent(ChipInfo.EidValue)
	CopyEidButton.SetText(TR.Trans("label.copy_eid_button_copied"))
	time.Sleep(2 * time.Second)
	CopyEidButton.SetText(TR.Trans("label.copy_eid_button"))
}

func copyEuiccInfo2ButtonFunc() {
	WMain.Clipboard().SetContent(EuiccInfo2Entry.Text)
	CopyEuiccInfo2Button.SetText(TR.Trans("label.copy_euicc_info2_button_copied"))
	time.Sleep(2 * time.Second)
	CopyEuiccInfo2Button.SetText(TR.Trans("label.copy_euicc_info2_button"))
}

func setDefaultSmdpButtonFunc() {
	if !isApduConfigured() {
		showApduNotConfiguredDialog()
		return
	}
	if RefreshNeeded {
		ShowRefreshNeededDialog()
		return
	}
	InitSetDefaultSmdpDialog().Show()
}

func viewCertInfoButtonFunc() {
	selectedCI := Unselected
	type ciWidgetEl struct {
		Country string
		Name    string
		KeyID   string
	}
	var ciWidgetEls []ciWidgetEl
	for _, keyId := range ChipInfo.EUICCInfo2.EuiccCiPKIDListForSigning {
		if !sliceContains(ChipInfo.EUICCInfo2.EuiccCiPKIDListForVerification, keyId) {
			continue
		}
		var element ciWidgetEl
		element.KeyID = keyId
		element.Name = TR.Trans("label.ci_name_unknown")
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
			o.(*fyne.Container).Objects[1].(*widget.Label).SetText(fmt.Sprintf(TR.Trans("label.ci_info_keyid")+" %s", ciWidgetEls[i].KeyID))
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
			dialog.ShowInformation(TR.Trans("dialog.ci_no_data"),
				TR.Trans("message.ci_no_data"),
				WMain)
		} else {
			const CiUrl = "https://euicc-manual.osmocom.org/docs/pki/ci/files/"
			certificateURL := fmt.Sprint(CiUrl, issuer.KeyID, ".txt")
			if err := OpenProgram(certificateURL); err != nil {
				dialog.ShowError(err, WMain)
			}
		}
	}
	certDataButton := &widget.Button{
		Text:     TR.Trans("label.cert_data_button"),
		OnTapped: certDataButtonFunc,
		Icon:     theme.InfoIcon(),
	}
	d := dialog.NewCustom(TR.Trans("dialog.ci"), TR.Trans("dialog.ok"),
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
			nameLabel := &widget.Label{}
			stateLabel := &widget.Label{TextStyle: fyne.TextStyle{Bold: true}}
			enabledIcon := widget.NewIcon(theme.ConfirmIcon())
			profileIcon := widget.NewIcon(theme.FileImageIcon())
			providerLabel := &widget.Label{}
			return container.NewVBox(
				container.NewHBox(iccidLabel, layout.NewSpacer(), nameLabel),
				container.NewHBox(container.NewVBox(layout.NewSpacer(), stateLabel),
					enabledIcon, providerLabel, profileIcon, layout.NewSpacer()))
		},
		UpdateItem: func(i widget.ListItemID, o fyne.CanvasObject) {
			r1 := o.(*fyne.Container).Objects[0].(*fyne.Container)
			r2 := o.(*fyne.Container).Objects[1].(*fyne.Container)
			iccidLabel := r1.Objects[0].(*widget.Label)
			nameLabel := r1.Objects[2].(*widget.Label)
			stateLabel := r2.Objects[0].(*fyne.Container).Objects[1].(*widget.Label)
			enabledIcon := r2.Objects[1].(*widget.Icon)
			providerLabel := r2.Objects[2].(*widget.Label)
			profileIcon := r2.Objects[3].(*widget.Icon)

			iccid := Profiles[i].Iccid
			if ProfileMaskNeeded {
				iccid = Profiles[i].MaskedICCID()
			}
			iccidLabel.SetText(fmt.Sprintf(TR.Trans("label.info_iccid")+" %s", iccid))
			if Profiles[i].ProfileNickname != nil {
				nameLabel.SetText(*Profiles[i].ProfileNickname)
			} else {
				nameLabel.SetText(Profiles[i].ProfileName)
			}
			switch Profiles[i].ProfileState {
			case "enabled":
				stateLabel.SetText(TR.Trans("label.profile_status_enabled"))
			case "disabled":
				stateLabel.SetText(TR.Trans("label.profile_status_disabled"))
			}
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

			providerLabel.SetText(TR.Trans("label.info_provider") + " " + Profiles[i].ServiceProviderName)
		},
		OnSelected: func(id widget.ListItemID) {
			SelectedProfile = id
			if Profiles[SelectedProfile].ProfileState == "enabled" {
				ProfileStateAllowDisable = true
				SwitchStateButton.SetText(TR.Trans("label.switch_state_button_disable"))
				SwitchStateButton.SetIcon(theme.CancelIcon())
			} else {
				ProfileStateAllowDisable = false
				SwitchStateButton.SetText(TR.Trans("label.switch_state_button_enable"))
				SwitchStateButton.SetIcon(theme.ConfirmIcon())
			}
		},
		OnUnselected: func(id widget.ListItemID) {
			SelectedProfile = Unselected
		}}
}

func initNotificationList() *widget.List {
	maskFQDNExceptPublicSuffix := func(fqdn string) string {
		suffix, _ := publicsuffix.PublicSuffix(fqdn)
		parts := strings.Split(fqdn, ".")
		suffixParts := strings.Split(suffix, ".")
		if len(parts) <= len(suffixParts) {
			return strings.Repeat("x", len(parts[0])) + "." + suffix
		}
		for x := 0; x < len(parts)-len(suffixParts); x++ {
			parts[x] = strings.Repeat("x", len(parts[x]))
		}
		return strings.Join(parts, ".")
	}

	return &widget.List{
		Length: func() int {
			return len(Notifications)
		},
		CreateItem: func() fyne.CanvasObject {
			notificationAddressLabel := &widget.Label{}
			seqLabel := &widget.Label{}
			operationLabel := &widget.Label{TextStyle: fyne.TextStyle{Bold: true}}
			providerLabel := &widget.Label{}
			iccidLabel := &widget.Label{}
			providerIcon := widget.NewIcon(theme.FileImageIcon())
			return container.NewVBox(
				container.NewHBox(notificationAddressLabel, layout.NewSpacer(), seqLabel),
				container.NewHBox(container.NewVBox(layout.NewSpacer(), operationLabel), providerLabel, providerIcon, iccidLabel),
			)
		},
		UpdateItem: func(i widget.ListItemID, o fyne.CanvasObject) {
			notificationAddressLabel := o.(*fyne.Container).Objects[0].(*fyne.Container).Objects[0].(*widget.Label)
			seqLabel := o.(*fyne.Container).Objects[0].(*fyne.Container).Objects[2].(*widget.Label)
			iccidLabel := o.(*fyne.Container).Objects[1].(*fyne.Container).Objects[3].(*widget.Label)
			operationLabel := o.(*fyne.Container).Objects[1].(*fyne.Container).Objects[0].(*fyne.Container).Objects[1].(*widget.Label)
			providerLabel := o.(*fyne.Container).Objects[1].(*fyne.Container).Objects[1].(*widget.Label)
			providerIcon := o.(*fyne.Container).Objects[1].(*fyne.Container).Objects[2].(*widget.Icon)

			iccid := Notifications[i].Iccid
			notificationAddress := Notifications[i].NotificationAddress
			if NotificationMaskNeeded {
				if iccid != "" {
					iccid = Notifications[i].MaskedICCID()
				}
				notificationAddress = maskFQDNExceptPublicSuffix(Notifications[i].NotificationAddress)
			}
			if iccid == "" {
				iccid = TR.Trans("label.no_iccid")
			}
			iccidLabel.SetText(fmt.Sprint("(", iccid, ")"))
			notificationAddressLabel.SetText(notificationAddress)
			seqLabel.SetText(fmt.Sprint(TR.Trans("label.info_seq")+" ", Notifications[i].SeqNumber))
			switch Notifications[i].ProfileManagementOperation {
			case "enable":
				operationLabel.SetText(TR.Trans("label.notification_operation_enable"))
			case "disable":
				operationLabel.SetText(TR.Trans("label.notification_operation_disable"))
			case "install":
				operationLabel.SetText(TR.Trans("label.notification_operation_install"))
			case "delete":
				operationLabel.SetText(TR.Trans("label.notification_operation_delete"))
			}
			profile, err := findProfileByIccid(Notifications[i].Iccid)
			if err != nil {
				providerLabel.SetText(TR.Trans("label.deleted_profile"))
				providerIcon.Hide()
			} else {
				name := profile.ServiceProviderName
				if profile.ProfileNickname != nil {
					name = *profile.ProfileNickname
				}
				providerLabel.SetText(name)
				if profile.Icon != nil {
					providerIcon.SetResource(fyne.NewStaticResource(profile.Iccid, profile.Icon))
					providerIcon.Show()
				} else {
					providerIcon.Hide()
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

func processNotificationManually(seq int) {
	if err := LpacNotificationProcess(seq, false); err != nil {
		ShowLpacErrDialog(err)
		err2 := RefreshNotification()
		if err2 != nil {
			ShowLpacErrDialog(err2)
		}
	} else {
		var notification *Notification
		for _, n := range Notifications {
			if n.SeqNumber == seq {
				notification = n
				break
			}
		}
		if notification == nil {
			dialog.ShowError(errors.New(TR.Trans("message.notification_not_found")), WMain)
			return
		}
		var d *dialog.CustomDialog
		notNowButton := &widget.Button{
			Text: TR.Trans("dialog.not_now"),
			Icon: theme.CancelIcon(),
			OnTapped: func() {
				d.Hide()
			},
		}
		removeButton := &widget.Button{
			Text: TR.Trans("label.remove_notification_button"),
			Icon: theme.DeleteIcon(),
			OnTapped: func() {
				go func() {
					d.Hide()
					if err2 := LpacNotificationRemove(seq); err2 != nil {
						ShowLpacErrDialog(err2)
					}
					if err2 := RefreshNotification(); err2 != nil {
						ShowLpacErrDialog(err2)
						return
					}
					if err2 := RefreshChipInfo(); err2 != nil {
						ShowLpacErrDialog(err2)
						return
					}
				}()
			},
		}
		d = dialog.NewCustomWithoutButtons(TR.Trans("dialog.process_notification_remove_notification"),
			container.NewBorder(
				nil,
				container.NewCenter(container.NewHBox(notNowButton, spacer, removeButton)),
				nil,
				nil,
				container.NewVBox(
					&widget.Label{Text: TR.Trans("message.process_notification_ask_remove_notification"),
						Alignment: fyne.TextAlignCenter},
					&widget.Label{Text: fmt.Sprintf(TR.Trans("label.info_seq")+" %d\n"+
						TR.Trans("label.info_iccid")+" %s\n"+
						TR.Trans("label.info_operation")+" %s\n"+
						TR.Trans("label.info_address")+" %s\n",
						notification.SeqNumber, notification.Iccid,
						notification.ProfileManagementOperation, notification.NotificationAddress)})),
			WMain)
		d.Show()
	}
}

func findNewNotification(origin, new []*Notification) *Notification {
	exists := make(map[int]bool)
	for _, notification := range origin {
		exists[notification.SeqNumber] = true
	}
	for _, notification := range new {
		if !exists[notification.SeqNumber] {
			return notification
		}
	}
	return nil
}

func findNewNotifications(origin, new []*Notification) []*Notification {
	exists := make(map[int]bool)
	var foundNotifications []*Notification
	for _, notification := range origin {
		exists[notification.SeqNumber] = true
	}
	for _, notification := range new {
		if !exists[notification.SeqNumber] {
			foundNotifications = append(foundNotifications, notification)
		}
	}
	return foundNotifications
}

func findProfileByIccid(iccid string) (*Profile, error) {
	for _, profile := range Profiles {
		if iccid == profile.Iccid {
			return profile, nil
		}
	}
	return nil, errors.New(TR.Trans("message.profile_not_found"))
}

func sliceContains[T comparable](slice []T, element T) bool {
	for _, v := range slice {
		if v == element {
			return true
		}
	}
	return false
}
