package slack

import (
	"testing"
)

func TestSelfTriggeredMemberEvent(t *testing.T) {
	tests := []struct {
		name  string
		auth  []eventAuth
		event MemberEvent
		want  bool
	}{
		{
			name:  "bot_is_invitee",
			auth:  []eventAuth{{IsBot: true, UserID: "B1"}},
			event: MemberEvent{User: "B1", Inviter: "U2"},
			want:  true,
		},
		{
			name:  "bot_is_inviter",
			auth:  []eventAuth{{IsBot: true, UserID: "B1"}},
			event: MemberEvent{User: "U2", Inviter: "B1"},
			want:  true,
		},
		{
			name:  "different_bot",
			auth:  []eventAuth{{IsBot: true, UserID: "B1"}},
			event: MemberEvent{User: "U2", Inviter: "U3"},
			want:  false,
		},
		{
			name:  "different_user",
			auth:  []eventAuth{{IsBot: false, UserID: "U1"}},
			event: MemberEvent{User: "U2", Inviter: "U3"},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selfTriggeredMemberEvent(nil, tt.auth, tt.event)
			if got != tt.want {
				t.Fatalf("selfTriggeredMemberEvent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelfTriggeredEvent(t *testing.T) {
	tests := []struct {
		name string
		auth []eventAuth
		id   string
		want bool
	}{
		{
			name: "bot_is_user",
			auth: []eventAuth{{IsBot: true, UserID: "B1"}},
			id:   "B1",
			want: true,
		},
		{
			name: "different_bot",
			auth: []eventAuth{{IsBot: true, UserID: "B1"}},
			id:   "U2",
			want: false,
		},
		{
			name: "different_user",
			auth: []eventAuth{{IsBot: false, UserID: "U1"}},
			id:   "U2",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selfTriggeredEvent(nil, tt.auth, tt.id)
			if got != tt.want {
				t.Fatalf("selfTriggeredEvent() = %v, want %v", got, tt.want)
			}
		})
	}
}
