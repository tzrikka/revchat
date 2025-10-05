package users_test

import (
	"testing"

	"github.com/tzrikka/revchat/pkg/users"
)

func TestEmailToBitbucketID(t *testing.T) {
	got, err := users.EmailToBitbucketID(nil, "workspace", "")
	if err == nil {
		t.Error("EmailToBitbucketID() error = nil")
	}
	want := ""
	if got != want {
		t.Errorf("EmailToBitbucketID() = %q, want %q", got, want)
	}
}
