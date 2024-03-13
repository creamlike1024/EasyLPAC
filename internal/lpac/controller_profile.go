package lpac

import (
	"encoding/json"
)

func (c *Controller) ProfileList() (profiles []*Profile, err error) {
	payload, err := c.invoke("profile", "list")
	if err == nil {
		err = json.Unmarshal(payload, &profiles)
	}
	return
}

func (c *Controller) ProfileEnable(iccid string) (err error) {
	_, err = c.invoke("profile", "enable", iccid)
	return
}

func (c *Controller) ProfileDisable(iccid string) (err error) {
	_, err = c.invoke("profile", "disable", iccid)
	return
}

func (c *Controller) ProfileDelete(iccid string) (err error) {
	_, err = c.invoke("profile", "delete", iccid)
	return
}

func (c *Controller) ProfileDownload(steps chan<- string, options *ProfileDownloadOptions) (err error) {
	args := []string{"profile", "download"}
	if options.Host != "" {
		args = append(args, "-s", options.Host)
	}
	if options.MatchingID != "" {
		args = append(args, "-m", options.MatchingID)
	}
	if options.ConfirmationCode != "" {
		args = append(args, "-c", options.ConfirmationCode)
	}
	if options.IMEI != "" {
		args = append(args, "-i", options.IMEI)
	}
	_, err = c.invokeWithProgress(steps, args...)
	return
}

func (c *Controller) SetProfileNickname(iccid, name string) (err error) {
	_, err = c.invoke("profile", "nickname", iccid, name)
	return
}
