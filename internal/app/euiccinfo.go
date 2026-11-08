package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"EasyLPAC/internal/lpa"
	"EasyLPAC/internal/registry"
)

// euiccInfo2JSON mirrors EUICCInfo2 (SGP.22 ES10b.GetEUICCInfo) with
// lpac's field names and order, so that copied text matches what lpac
// prints.
type euiccInfo2JSON struct {
	ProfileVersion   string `json:"profileVersion"`
	Svn              string `json:"svn"`
	EuiccFirmwareVer string `json:"euiccFirmwareVer"`
	ExtCardResource  struct {
		InstalledApplication  int `json:"installedApplication"`
		FreeNonVolatileMemory int `json:"freeNonVolatileMemory"`
		FreeVolatileMemory    int `json:"freeVolatileMemory"`
	} `json:"extCardResource"`
	UiccCapability                 []string `json:"uiccCapability"`
	Ts102241Version                string   `json:"ts102241Version"`
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
}

// EUICCInfo2JSON returns EUICCInfo2 as indented JSON.
func EUICCInfo2JSON(i lpa.EUICCInfo2) []byte {
	var j euiccInfo2JSON
	j.ProfileVersion, j.Svn, j.EuiccFirmwareVer = i.ProfileVersion, i.Svn, i.EuiccFirmwareVer
	j.ExtCardResource.InstalledApplication = i.ExtCardResource.InstalledApplication
	j.ExtCardResource.FreeNonVolatileMemory = i.ExtCardResource.FreeNonVolatileMemory
	j.ExtCardResource.FreeVolatileMemory = i.ExtCardResource.FreeVolatileMemory
	j.UiccCapability, j.RspCapability = i.UiccCapability, i.RspCapability
	j.Ts102241Version, j.GlobalplatformVersion = i.Ts102241Version, i.GlobalplatformVersion
	j.EuiccCiPKIDListForVerification = i.EuiccCiPKIDListForVerification
	j.EuiccCiPKIDListForSigning = i.EuiccCiPKIDListForSigning
	j.EuiccCategory = i.EuiccCategory
	j.ForbiddenProfilePolicyRules = i.ForbiddenProfilePolicyRules
	j.PpVersion, j.SasAcreditationNumber = i.PpVersion, i.SasAcreditationNumber
	j.CertificationDataObject.PlatformLabel = i.CertificationDataObject.PlatformLabel
	j.CertificationDataObject.DiscoveryBaseURL = i.CertificationDataObject.DiscoveryBaseURL
	out, _ := json.MarshalIndent(j, "", "  ")
	return out
}

// ValidEID reports whether an EID is 32 digits whose check digits verify:
// the number modulo 97 is 1 (SGP.29 §10).
func ValidEID(eid string) bool {
	if len(eid) != 32 {
		return false
	}
	for _, r := range eid {
		if r < '0' || r > '9' {
			return false
		}
	}
	n, ok := new(big.Int).SetString(eid, 10)
	return ok && new(big.Int).Mod(n, big.NewInt(97)).Int64() == 1
}

// ServerError explains an SM-DP+ error: the meaning of its codes from the
// eUICC Manual, else the server's message, and the codes.
func ServerError(err error) (summary, codes string, ok bool) {
	var e *lpa.ES9Error
	if !errors.As(err, &e) {
		return "", "", false
	}
	summary = registry.LookupErrorCode(e.Subject, e.Reason)
	if summary == "" {
		summary = e.Message
	}
	if summary == "" {
		summary = "The SM-DP+ server refused the request"
	}
	return summary, fmt.Sprintf("subject %s, reason %s", e.Subject, e.Reason), true
}

// DescribeError is a one-line explanation of err for the user.
func DescribeError(err error) string {
	if summary, codes, ok := ServerError(err); ok {
		return "SM-DP+: " + summary + " (" + codes + ")"
	}
	return err.Error()
}
