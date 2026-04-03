package internal_test

import (
	"reflect"
	"testing"

	"github.com/tzrikka/revchat/pkg/data/internal"
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
					if err := internal.SetReminder(t.Context(), tt.userID, tt.kitchenTime, tt.tz); err != nil {
						t.Fatalf("SetReminder() error = %v", err)
					}
				} else {
					if err := internal.DeleteReminder(t.Context(), tt.userID); err != nil {
						t.Fatalf("DeleteReminder() error = %v", err)
					}
				}
			}

			gotReminders, err := internal.ListReminders(t.Context())
			if err != nil {
				t.Errorf("ListReminders() error = %v", err)
			}
			if !reflect.DeepEqual(gotReminders, tt.wantReminders) {
				t.Errorf("ListReminders() = %q, want %q", gotReminders, tt.wantReminders)
			}
		})
	}
}
