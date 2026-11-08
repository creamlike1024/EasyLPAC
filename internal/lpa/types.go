// Package lpa defines the domain model and service boundary between the
// application layer and the eUICC backends. It must not import any UI
// framework or cgo types, or parse any CLI output. See docs/refactor-architecture.md.
package lpa

// PullInfo carries the user input for a profile download.
type PullInfo struct {
	SMDP        string
	MatchID     string
	ObjectID    string
	ConfirmCode string
	IMEI        string
}

// DownloadRequest is the user input for a download plus the optional
// preview confirmation hook.
type DownloadRequest struct {
	PullInfo
	// Confirm is called with the metadata of the profile about to be
	// installed. Returning false aborts the download (a protocol
	// cancel is attempted before the package is fetched). nil means
	// install without preview.
	Confirm func(ProfileMetadata) (bool, error)
}

// ProfileMetadata is the preview of a profile package.
type ProfileMetadata struct {
	ICCID               string
	ServiceProviderName string
	ProfileName         string
	IconType            string
	Icon                []byte
	ProfileClass        string
}

// Profile is one eUICC profile entry.
type Profile struct {
	Iccid               string
	IsdpAid             string
	ProfileState        string // "enabled" | "disabled"
	ProfileNickname     *string
	ServiceProviderName string
	ProfileName         string
	IconType            string
	Icon                []byte
	ProfileClass        string
}

// Notification is one pending notification on the eUICC.
type Notification struct {
	SeqNumber                  int
	ProfileManagementOperation string // "install" | "enable" | "disable" | "delete"
	NotificationAddress        string
	Iccid                      string
}

// EuiccInfo mirrors the chip information shown in the UI.
type EuiccInfo struct {
	EidValue                 string
	EuiccConfiguredAddresses struct {
		DefaultDpAddress any
		RootDsAddress    string
	}
	EUICCInfo2 EUICCInfo2
}

// EUICCInfo2 is the decoded EUICCInfo2 structure.
type EUICCInfo2 struct {
	ProfileVersion   string
	Svn              string
	EuiccFirmwareVer string
	ExtCardResource  struct {
		InstalledApplication  int
		FreeNonVolatileMemory int
		FreeVolatileMemory    int
	}
	UiccCapability                 []string
	Ts102241Version                string
	GlobalplatformVersion          string
	RspCapability                  []string
	EuiccCiPKIDListForVerification []string
	EuiccCiPKIDListForSigning      []string
	EuiccCategory                  any
	ForbiddenProfilePolicyRules    []string
	PpVersion                      string
	SasAcreditationNumber          string
	CertificationDataObject        struct {
		PlatformLabel    string
		DiscoveryBaseURL string
	}
}

// Device is a discovered APDU device entry for one backend.
type Device struct {
	Env  string // backend-internal identifier (e.g. PC/SC reader IFID)
	Name string // human-readable label
}

// DownloadOutcome reports the result of a completed download.
// Written profile packages are never reported as failed because of
// later notification or UI cancellation errors.
type DownloadOutcome struct {
	// Notification is the install notification produced by this
	// download, as reported by the card during the install; nil when
	// the implementation cannot correlate one.
	Notification *Notification
}

// NotificationOutcome reports the result of processing one notification.
type NotificationOutcome struct {
	Seq     int
	Removed bool
}
