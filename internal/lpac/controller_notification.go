package lpac

import (
	"encoding/json"
	"strconv"
)

func (c *Controller) NotificationList() (notifications []*Notification, err error) {
	payload, err := c.invoke("notification", "list")
	if err != nil {
		return
	}
	notifications = make([]*Notification, 0)
	err = json.Unmarshal(payload, &notifications)
	return
}

func (c *Controller) NotificationProcess(index int) (err error) {
	_, err = c.invoke("notification", "process", strconv.Itoa(index))
	return
}

func (c *Controller) NotificationRemove(index int) (err error) {
	_, err = c.invoke("notification", "remove", strconv.Itoa(index))
	return
}
