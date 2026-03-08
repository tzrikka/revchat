package users_test

import (
	"testing"

	"github.com/tzrikka/revchat/pkg/users"
	"github.com/tzrikka/timpani-api/pkg/bitbucket"
)

func TestEmailToBitbucketID(t *testing.T) {
	got := users.EmailToBitbucketID(nil, "")
	want := ""
	if got != want {
		t.Errorf("EmailToBitbucketID() = %q, want %q", got, want)
	}
}

func TestBitbucketActorToEmail(t *testing.T) {
	tests := []struct {
		name  string
		actor bitbucket.User
		want  string
	}{
		{
			name:  "empty_actor",
			actor: bitbucket.User{},
			want:  "",
		},
		{
			name: "app_with_account_id",
			actor: bitbucket.User{
				Type:      "app_user",
				AccountID: "account-id-123",
			},
			want: "bot",
		},
		{
			name: "app_without_account_id",
			actor: bitbucket.User{
				Type: "app_user",
			},
			want: "bot",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := users.BitbucketActorToEmail(nil, tt.actor); got != tt.want {
				t.Errorf("BitbucketActorToEmail() = %q, want %q", got, tt.want)
			}
		})
	}
}
