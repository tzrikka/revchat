package users_test

import (
	"testing"

	"github.com/tzrikka/revchat/pkg/users"
)

func TestEmailToBitbucketID(t *testing.T) {
	got := users.EmailToBitbucketID(nil, "workspace", "")
	want := ""
	if got != want {
		t.Errorf("EmailToBitbucketID() = %q, want %q", got, want)
	}
}
