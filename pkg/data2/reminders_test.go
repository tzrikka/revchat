package data2_test

import (
	"reflect"
	"testing"

	"github.com/tzrikka/revchat/pkg/data2"
)

func TestReminders(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	tests := []struct {
		name          string
		userID        string
		kitchenTime   string
		tz            string
		wantReminders map[string]string
	}{
		{
			name:          "initial_state",
			wantReminders: map[string]string{},
		},
		{
			name:          "first_set",
			userID:        "user1",
			kitchenTime:   "9:00AM",
			tz:            "America/Los_Angeles",
			wantReminders: map[string]string{"user1": "9:00AM America/Los_Angeles"},
		},
		{
			name:        "another_set",
			userID:      "user2",
			kitchenTime: "5:00PM",
			tz:          "America/Los_Angeles",
			wantReminders: map[string]string{
				"user1": "9:00AM America/Los_Angeles",
				"user2": "5:00PM America/Los_Angeles",
			},
		},
		{
			name:        "update",
			userID:      "user1",
			kitchenTime: "10:00AM",
			tz:          "America/Los_Angeles",
			wantReminders: map[string]string{
				"user1": "10:00AM America/Los_Angeles",
				"user2": "5:00PM America/Los_Angeles",
			},
		},
		{
			name:   "first_delete",
			userID: "user2",
			wantReminders: map[string]string{
				"user1": "10:00AM America/Los_Angeles",
			},
		},
		{
			name:          "last_delete",
			userID:        "user1",
			wantReminders: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.userID != "" {
				if tt.kitchenTime != "" {
					if err := data2.SetScheduledUserReminder(nil, tt.userID, tt.kitchenTime, tt.tz); err != nil {
						t.Fatalf("SetScheduledUserReminder() error = %v", err)
					}
				} else {
					data2.DeleteScheduledUserReminder(nil, tt.userID)
				}
			}

			gotReminders, err := data2.ListScheduledUserReminders(nil)
			if err != nil {
				t.Errorf("ListScheduledUserReminders() error = %v", err)
			}
			if !reflect.DeepEqual(gotReminders, tt.wantReminders) {
				t.Errorf("ListScheduledUserReminders() = %q, want %q", gotReminders, tt.wantReminders)
			}
		})
	}
}
