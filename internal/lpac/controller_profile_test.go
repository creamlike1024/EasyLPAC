package lpac

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestController_ProfileList(t *testing.T) {
	controller := LoadFixture("profile-list")
	profiles, err := controller.ProfileList()
	assert.NoError(t, err)
	assert.Len(t, profiles, 1)
	expectedProfile := &Profile{
		ICCID:               "8944478600003029338",
		ProfileState:        "enabled",
		ServiceProviderName: "BetterRoaming",
		ProfileName:         "BetterRoaming",
		ProfileClass:        "operational",
	}
	assert.Equal(t, expectedProfile, profiles[0])
}

func TestController_ProfileEnable(t *testing.T) {
	controller := LoadFixture("profile-enable")
	assert.NoError(t, controller.ProfileEnable("8944478600003029338"))
	controller = LoadFixture("profile-enabled")
	assert.Equal(t, controller.ProfileEnable("8944478600003029338"), &Error{
		FunctionName: "es10c_enable_profile",
		Details:      "profile not in disabled state",
	})
}

func TestController_ProfileDisable(t *testing.T) {
	controller := LoadFixture("profile-disable")
	assert.NoError(t, controller.ProfileDisable("8944478600003029338"))
	controller = LoadFixture("profile-disabled")
	assert.Equal(t, controller.ProfileDisable("8944478600003029338"), &Error{
		FunctionName: "es10c_disable_profile",
		Details:      "profile not in enabled state",
	})
}

func TestController_ProfileDelete(t *testing.T) {
	controller := LoadFixture("profile-delete")
	assert.NoError(t, controller.ProfileDelete("8944478600003241313"))
	controller = LoadFixture("profile-deleted")
	assert.Equal(t, controller.ProfileDelete("8944478600003029338"), &Error{
		FunctionName: "es10c_disable_profile",
		Details:      "iccid or aid not found",
	})
}

func TestController_ProfileDownload(t *testing.T) {
	controller := LoadFixture("profile-download")
	assert.NoError(t, controller.ProfileDownload(nil, &ProfileDownloadOptions{
		Host:       "rsp.truphone.com",
		MatchingID: "QR-G-5C-KR-1PCDWP9",
	}))
}

func TestController_SetProfileNickname(t *testing.T) {
	controller := LoadFixture("set-profile-nickname")
	assert.NoError(t, controller.SetProfileNickname("8944478600003029338", "BetterRoaming"))
}
