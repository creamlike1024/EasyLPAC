package lpac

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestController_ChipInfo(t *testing.T) {
	controller := LoadFixture("chip-info")
	chipInfo, err := controller.ChipInfo()
	assert.NoError(t, err)
	info, err := chipInfo.UnmarshalInfo()
	assert.NoError(t, err)
	assert.Equal(t, "89049032005008882600049725952373", chipInfo.EID)
	assert.Empty(t, chipInfo.ConfiguredAddresses.DefaultSMDP)
	assert.Equal(t, "testrootsmds.gsma.com", chipInfo.ConfiguredAddresses.RootSMDS)
	assert.Equal(t, info.CIListSigning, info.CIListVerification)
	assert.Equal(t, []string{"81370f5125d0b1d408d4c3b232e6d25e795bebfb"}, info.CIListVerification)
}

func TestController_SetDefaultSMDP(t *testing.T) {
	controller := LoadFixture("set-default-smdp")
	assert.NoError(t, controller.SetDefaultSMDP("smdp.io"))
}
