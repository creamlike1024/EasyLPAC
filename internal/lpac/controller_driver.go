package lpac

import (
	"encoding/json"
)

func (c *Controller) APDUDriverList() (drivers []*APDUDriver, err error) {
	payload, err := c.invoke("driver", "apdu", "list")
	if err != nil {
		return
	}
	err = json.Unmarshal(payload, &drivers)
	return
}
