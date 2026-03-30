package data_test

import (
	"testing"

	"github.com/tzrikka/revchat/pkg/data"
)

func TestSlackBots(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	tests := []struct {
		name   string
		botID  string
		userID string
		want1  string
		want2  string
	}{
		{
			name: "initial_state",
		},
		{
			name:   "set_and_get",
			botID:  "bot1",
			userID: "user1",
			want1:  "user1",
		},
		{
			name:   "another_set_and_get",
			botID:  "bot2",
			userID: "user2",
			want1:  "user1",
			want2:  "user2",
		},
		{
			name:   "update_and_get",
			botID:  "bot1",
			userID: "user3",
			want1:  "user3",
			want2:  "user2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.botID != "" {
				data.SetSlackBotUserID(nil, tt.botID, tt.userID)
			}

			got1 := data.GetSlackBotUserID(nil, "bot1")
			if got1 != tt.want1 {
				t.Errorf("GetSlackBotUserID(bot1) = %q, want %q", got1, tt.want1)
			}

			got2 := data.GetSlackBotUserID(nil, "bot2")
			if got2 != tt.want2 {
				t.Errorf("GetSlackBotUserID(bot2) = %q, want %q", got2, tt.want2)
			}
		})
	}
}
