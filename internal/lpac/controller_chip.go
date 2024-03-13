package lpac

import (
	"bytes"
	"encoding/json"
)

func (c *Controller) ChipInfo() (details *EUICCDetails, err error) {
	payload, err := c.invoke("chip", "info")
	if err == nil {
		details = new(EUICCDetails)
		err = json.Unmarshal(payload, details)
	}
	if details != nil && details.Info != nil {
		var dst bytes.Buffer
		if err = json.Indent(&dst, details.Info, "", "  "); err != nil {
			return
		}
		details.Info = dst.Bytes()
	}
	return
}

func (c *Controller) SetDefaultSMDP(smdp string) (err error) {
	_, err = c.invoke("chip", "defaultsmdp", smdp)
	return
}
