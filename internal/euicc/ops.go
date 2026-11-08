package euicc

/*
#include <stdlib.h>
#include "shim.h"
*/
import "C"

import (
	"encoding/base64"
	"unsafe"
)

// EUICCInfo2 mirrors the ES10c GetEuiccInfo2 subset we consume.
type EUICCInfo2 struct {
	ProfileVersion    string
	Svn               string
	EuiccFirmwareVer  string
	ExtCardResource   ExtCardResource
	UiccCapability    []string
	Ts102241Version   string
	GlobalplatformVer string
	RspCapability     []string
	CiPKIDForVerify   []string
	CiPKIDForSigning  []string
	EuiccCategory     string
	ForbiddenPPR      []string
	PpVersion         string
	SasAcreditation   string
	PlatformLabel     string
	DiscoveryBaseURL  string
}

// ExtCardResource is the card memory inventory.
type ExtCardResource struct {
	InstalledApplication  uint32
	FreeNonVolatileMemory uint32
	FreeVolatileMemory    uint32
}

// ConfiguredAddresses are the SM-DP+/SM-DS defaults stored on the card.
// Empty strings mean "not set".
type ConfiguredAddresses struct {
	DefaultDpAddress string
	RootDsAddress    string
}

// ProfileState values as returned by es10c.
const (
	ProfileStateDisabled  = 0
	ProfileStateEnabled   = 1
	ProfileStateUndefined = 255
)

// ProfileClass values as returned by es10c.
const (
	ProfileClassTest         = 0
	ProfileClassProvisioning = 1
	ProfileClassOperational  = 2
	ProfileClassUndefined    = 255
)

// IconType values as returned by es10c.
const (
	IconTypeJpeg      = 0
	IconTypePng       = 1
	IconTypeUndefined = 255
)

// Profile is one entry of ES10c GetProfilesInfo.
type Profile struct {
	ICCID               string
	ISDPAID             string
	State               int32 // -1 (null) or ProfileState*
	Class               int32
	Nickname            string // empty = absent
	HasNickname         bool
	ServiceProviderName string
	ProfileName         string
	IconType            int32
	Icon                []byte // decoded PNG/JPEG, nil if absent/undecodable
}

// Notification is one entry of ES10b ListNotification.
type Notification struct {
	SeqNumber                  uint64
	ProfileManagementOperation int32
	NotificationAddress        string
	ICCID                      string
}

// Notification operation values (ES10b).
const (
	NotificationOpInstall   = 0x80
	NotificationOpEnable    = 0x40
	NotificationOpDisable   = 0x20
	NotificationOpDelete    = 0x10
	NotificationOpUndefined = 0xFF
)

// GetConfiguredAddresses returns the configured SM-DP+/SM-DS addresses.
func (c *Context) GetConfiguredAddresses() (ConfiguredAddresses, error) {
	var dp, ds *C.char
	if rc := C.elpac_get_configured_addresses(c.h, &dp, &ds); rc != 0 {
		return ConfiguredAddresses{}, c.failed("es10a_get_euicc_configured_addresses")
	}
	defer C.elpac_string_free(dp)
	defer C.elpac_string_free(ds)
	return ConfiguredAddresses{
		DefaultDpAddress: goStringOwned(dp),
		RootDsAddress:    goStringOwned(ds),
	}, nil
}

// GetEUICCInfo2 reads and copies EUICCInfo2.
func (c *Context) GetEUICCInfo2() (*EUICCInfo2, error) {
	var cinfo C.struct_elpac_euiccinfo2
	if rc := C.elpac_get_euiccinfo2(c.h, &cinfo); rc != 0 {
		return nil, c.failed("es10c_ex_get_euiccinfo2")
	}
	defer C.elpac_euiccinfo2_free(&cinfo)

	return &EUICCInfo2{
		ProfileVersion:   goStringOwned(cinfo.profileVersion),
		Svn:              goStringOwned(cinfo.svn),
		EuiccFirmwareVer: goStringOwned(cinfo.euiccFirmwareVer),
		ExtCardResource: ExtCardResource{
			InstalledApplication:  uint32(cinfo.installedApplication),
			FreeNonVolatileMemory: uint32(cinfo.freeNonVolatileMemory),
			FreeVolatileMemory:    uint32(cinfo.freeVolatileMemory),
		},
		UiccCapability:    goStringArray(cinfo.uiccCapability),
		Ts102241Version:   goStringOwned(cinfo.ts102241Version),
		GlobalplatformVer: goStringOwned(cinfo.globalplatformVersion),
		RspCapability:     goStringArray(cinfo.rspCapability),
		CiPKIDForVerify:   goStringArray(cinfo.euiccCiPKIdListForVerification),
		CiPKIDForSigning:  goStringArray(cinfo.euiccCiPKIdListForSigning),
		EuiccCategory:     goStringOwned(cinfo.euiccCategory),
		ForbiddenPPR:      goStringArray(cinfo.forbiddenProfilePolicyRules),
		PpVersion:         goStringOwned(cinfo.ppVersion),
		SasAcreditation:   goStringOwned(cinfo.sasAcreditationNumber),
		PlatformLabel:     goStringOwned(cinfo.platformLabel),
		DiscoveryBaseURL:  goStringOwned(cinfo.discoveryBaseURL),
	}, nil
}

// GetProfiles reads the profile list.
func (c *Context) GetProfiles() ([]Profile, error) {
	var clist C.struct_elpac_profile_list
	if rc := C.elpac_get_profiles(c.h, &clist); rc != 0 {
		return nil, c.failed("es10c_get_profiles_info")
	}
	defer C.elpac_profile_list_free(&clist)

	n := int(clist.count)
	if n == 0 {
		return nil, nil
	}
	items := unsafe.Slice((*C.struct_elpac_profile)(unsafe.Pointer(clist.items)), n)

	out := make([]Profile, 0, n)
	for i := range items {
		it := &items[i]
		p := Profile{
			ICCID:               goStringOwned(it.iccid),
			ISDPAID:             goStringOwned(it.isdpAid),
			State:               int32(it.profileState),
			Class:               int32(it.profileClass),
			ServiceProviderName: goStringOwned(it.serviceProviderName),
			ProfileName:         goStringOwned(it.profileName),
			IconType:            int32(it.iconType),
		}
		if it.profileNickname != nil {
			p.Nickname = C.GoString(it.profileNickname)
			p.HasNickname = true
		}
		if it.iconBase64 != nil {
			// libeuicc returns base64; a malformed icon does not fail
			// the whole list (deliberate difference from the legacy
			// JSON path, which failed wholesale).
			if raw, err := base64.StdEncoding.DecodeString(C.GoString(it.iconBase64)); err == nil {
				p.Icon = raw
			}
		}
		out = append(out, p)
	}
	return out, nil
}

// GetNotifications reads the pending notification list.
func (c *Context) GetNotifications() ([]Notification, error) {
	var clist C.struct_elpac_notification_list
	if rc := C.elpac_get_notifications(c.h, &clist); rc != 0 {
		return nil, c.failed("es10b_list_notification")
	}
	defer C.elpac_notification_list_free(&clist)

	n := int(clist.count)
	if n == 0 {
		return nil, nil
	}
	items := unsafe.Slice((*C.struct_elpac_notification)(unsafe.Pointer(clist.items)), n)

	out := make([]Notification, 0, n)
	for i := range items {
		it := &items[i]
		out = append(out, Notification{
			SeqNumber:                  uint64(it.seqNumber),
			ProfileManagementOperation: int32(it.profileManagementOperation),
			NotificationAddress:        goStringOwned(it.notificationAddress),
			ICCID:                      goStringOwned(it.iccid),
		})
	}
	return out, nil
}

func goStringOwned(p *C.char) string {
	if p == nil {
		return ""
	}
	return C.GoString(p)
}

// goStringArray walks a NULL-terminated array of C strings.
func goStringArray(arr **C.char) []string {
	if arr == nil {
		return nil
	}
	var out []string
	for i := 0; ; i++ {
		p := *(**C.char)(unsafe.Add(unsafe.Pointer(arr), uintptr(i)*unsafe.Sizeof(arr)))
		if p == nil {
			break
		}
		out = append(out, C.GoString(p))
	}
	return out
}
