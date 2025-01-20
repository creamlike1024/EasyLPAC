package main

import (
	_ "embed"
	"encoding/json"
	"strings"
)

type PullInfo struct {
	SMDP        string
	MatchID     string
	ObjectID    string
	ConfirmCode string
	IMEI        string
	OnlyPreview bool
}

type LpacReturnValue struct {
	Type    string `json:"type"`
	Payload struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	} `json:"payload"`
}

type EuiccInfo struct {
	EidValue                 string `json:"eidValue"`
	EuiccConfiguredAddresses struct {
		DefaultDpAddress any    `json:"defaultDpAddress"`
		RootDsAddress    string `json:"rootDsAddress"`
	} `json:"EuiccConfiguredAddresses"`
	EUICCInfo2 struct {
		ProfileVersion   string `json:"profileVersion"`
		Svn              string `json:"svn"`
		EuiccFirmwareVer string `json:"euiccFirmwareVer"`
		ExtCardResource  struct {
			InstalledApplication  int `json:"installedApplication"`
			FreeNonVolatileMemory int `json:"freeNonVolatileMemory"`
			FreeVolatileMemory    int `json:"freeVolatileMemory"`
		} `json:"extCardResource"`
		UiccCapability                 []string `json:"uiccCapability"`
		JavacardVersion                string   `json:"javacardVersion"`
		GlobalplatformVersion          string   `json:"globalplatformVersion"`
		RspCapability                  []string `json:"rspCapability"`
		EuiccCiPKIDListForVerification []string `json:"euiccCiPKIdListForVerification"`
		EuiccCiPKIDListForSigning      []string `json:"euiccCiPKIdListForSigning"`
		EuiccCategory                  any      `json:"euiccCategory"`
		ForbiddenProfilePolicyRules    []string `json:"forbiddenProfilePolicyRules"`
		PpVersion                      string   `json:"ppVersion"`
		SasAcreditationNumber          string   `json:"sasAcreditationNumber"`
		CertificationDataObject        struct {
			PlatformLabel    string `json:"platformLabel"`
			DiscoveryBaseURL string `json:"discoveryBaseURL"`
		} `json:"certificationDataObject"`
	} `json:"EUICCInfo2"`
}

type Profile struct {
	Iccid               string  `json:"iccid"`
	IsdpAid             string  `json:"isdpAid"`
	ProfileState        string  `json:"profileState"`
	ProfileNickname     *string `json:"profileNickname"`
	ServiceProviderName string  `json:"serviceProviderName"`
	ProfileName         string  `json:"profileName"`
	IconType            string  `json:"iconType"`
	Icon                []byte  `json:"icon"`
	ProfileClass        string  `json:"profileClass"`
}

func (p *Profile) MaskedICCID() string {
	return p.Iccid[0:7] + strings.Repeat("*", len(p.Iccid)-7)
}

func (p *Profile) CapitalizedState() string {
	return strings.ToUpper(p.ProfileState[0:1]) + p.ProfileState[1:]
}

type Notification struct {
	SeqNumber                  int    `json:"seqNumber"`
	ProfileManagementOperation string `json:"profileManagementOperation"`
	NotificationAddress        string `json:"notificationAddress"`
	Iccid                      string `json:"iccid"`
}

func (n *Notification) MaskedICCID() string {
	return n.Iccid[0:7] + strings.Repeat("*", len(n.Iccid)-7)
}

func (n *Notification) CapitalizedOperation() string {
	return strings.ToUpper(n.ProfileManagementOperation[0:1]) + n.ProfileManagementOperation[1:]
}

type ApduDriver struct {
	Env  string `json:"env"`
	Name string `json:"name"`
}

var Profiles []*Profile
var Notifications []*Notification
var ChipInfo *EuiccInfo
var ApduDrivers []*ApduDriver
