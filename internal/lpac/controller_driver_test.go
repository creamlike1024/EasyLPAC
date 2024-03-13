package lpac

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestController_APDUDriverList(t *testing.T) {
	controller := LoadFixture("chip-info")
	controller.APDUInterface = "libapduinterface_pcsc"
	drivers, err := controller.APDUDriverList()
	assert.NoError(t, err)
	assert.NotEmpty(t, drivers)
}
