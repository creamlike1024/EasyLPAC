package lpac

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestController_NotificationList(t *testing.T) {
	controller := LoadFixture("notification-list")
	notifications, err := controller.NotificationList()
	assert.NoError(t, err)
	assert.Len(t, notifications, 6)
}

func TestController_NotificationProcess(t *testing.T) {
	controller := LoadFixture("notification-process")
	assert.NoError(t, controller.NotificationProcess(3))
	controller = LoadFixture("notification-process-not-found")
	assert.Equal(t, controller.NotificationProcess(100), &Error{
		FunctionName: "es10b_retrieve_notifications_list",
	})
}

func TestController_NotificationRemove(t *testing.T) {
	controller := LoadFixture("notification-remove")
	assert.NoError(t, controller.NotificationRemove(3))
	controller = LoadFixture("notification-remove-not-found")
	assert.Equal(t, controller.NotificationRemove(100), &Error{
		FunctionName: "es10b_remove_notification_from_list",
		Details:      "seqNumber not found",
	})
}
