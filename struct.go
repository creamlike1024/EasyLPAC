package main

import "encoding/json"

type PullInfo struct {
	smdp        string
	matchID     string
	confirmCode string
	imei        string
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
	Iccid               string `json:"iccid"`
	IsdpAid             string `json:"isdpAid"`
	ProfileState        string `json:"profileState"`
	ProfileNickname     any    `json:"profileNickname"`
	ServiceProviderName string `json:"serviceProviderName"`
	ProfileName         string `json:"profileName"`
	IconType            string `json:"iconType"`
	Icon                any    `json:"icon"`
	ProfileClass        string `json:"profileClass"`
}

type Notification struct {
	SeqNumber                  int    `json:"seqNumber"`
	ProfileManagementOperation string `json:"profileManagementOperation"`
	NotificationAddress        string `json:"notificationAddress"`
	Iccid                      string `json:"iccid"`
}

var Profiles []Profile
var Notifications []Notification
var ChipInfo EuiccInfo
