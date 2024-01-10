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
	Eid         string `json:"eid"`
	DefaultSmds string `json:"default_smds"`
	DefaultSmdp string `json:"default_smdp"`
	Euiccinfo2  struct {
		ProfileVersion           string `json:"profile_version"`
		Sgp22Version             string `json:"sgp22_version"`
		EuiccFirmwareVersion     string `json:"euicc_firmware_version"`
		UiccFirmwareVersion      string `json:"uicc_firmware_version"`
		GlobalPlatformVersion    string `json:"global_platform_version"`
		ProtectionProfileVersion string `json:"protection_profile_version"`
		SasAccreditationNumber   string `json:"sas_accreditation_number"`
		FreeNvram                int    `json:"free_nvram"`
		FreeRAM                  int    `json:"free_ram"`
	} `json:"euiccinfo2"`
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
