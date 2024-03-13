package lpac

import (
	"encoding/json"
	"fmt"
)

type _Response[T any] struct {
	Type    string `json:"type"`
	Payload T      `json:"payload"`
}

type _Payload struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type _DriverPayload struct {
	Env  string          `json:"env"`
	Data json.RawMessage `json:"data"`
}

type Error struct {
	FunctionName string
	Details      string
}

func (e *Error) Error() string {
	if e.Details == "" {
		return e.FunctionName
	}
	return fmt.Sprint(e.FunctionName, ": ", e.Details)
}

type EUICCDetails struct {
	EID                 string               `json:"eidValue"`
	ConfiguredAddresses *ConfiguredAddresses `json:"EuiccConfiguredAddresses"`
	Info                json.RawMessage      `json:"EUICCInfo2"`
}

func (d *EUICCDetails) UnmarshalInfo() (info *EUICCInfo2, err error) {
	info = new(EUICCInfo2)
	err = json.Unmarshal(d.Info, info)
	return
}

type ConfiguredAddresses struct {
	DefaultSMDP string `json:"defaultDpAddress"`
	RootSMDS    string `json:"rootDsAddress"`
}

type EUICCInfo2 struct {
	ExtCardResource struct {
		FreeNVRAM int `json:"freeNonVolatileMemory"`
	} `json:"extCardResource"`
	CIListVerification []string `json:"euiccCiPKIdListForVerification"`
	CIListSigning      []string `json:"euiccCiPKIdListForSigning"`
}

type APDUDriver struct {
	Index string `json:"env"`
	Name  string `json:"name"`
}

func (d *APDUDriver) String() string {
	return fmt.Sprintf("Index: %s, Name: %q", d.Index, d.Name)
}

type Notification struct {
	Index                      int    `json:"seqNumber"`
	ProfileManagementOperation string `json:"profileManagementOperation"`
	NotificationAddress        string `json:"notificationAddress"`
	ICCID                      string `json:"iccid"`
}

type Profile struct {
	ICCID               string `json:"iccid"`
	ProfileState        string `json:"profileState"`
	ProfileNickname     string `json:"profileNickname"`
	ServiceProviderName string `json:"serviceProviderName"`
	ProfileName         string `json:"profileName"`
	IconType            string `json:"iconType"`
	Icon                []byte `json:"icon"`
	ProfileClass        string `json:"profileClass"`
}

type ProfileDownloadOptions struct {
	Host             string
	MatchingID       string
	ObjectID         string
	ConfirmationCode string
	IMEI             string
}
