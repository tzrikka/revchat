package data

import (
	"testing"
)

func TestUsers(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)
	pathCache.Clear()

	id := "user_id"
	name := "First Last"
	email := "email@example.com"

	// Before adding the user.
	got := SelectUserByBitbucketID(nil, id)
	if got.Email != "" {
		t.Fatalf("SelectUserByBitbucketID() email = %q, want %q", got.Email, "")
	}

	// Add the user.
	if err := UpsertUser(nil, email, name, id, "", "", ""); err != nil {
		t.Fatalf("UpsertUser() error = %v", err)
	}

	// After adding the user.
	got = SelectUserByBitbucketID(nil, id)
	if got.Email != email {
		t.Errorf("SelectUserByBitbucketID() email = %q, want %q", got.Email, email)
	}
	if got.RealName != name {
		t.Errorf("SelectUserByBitbucketID() name = %q, want %q", got.RealName, name)
	}

	got = SelectUserByGitHubID(nil, id)
	if got.Email != "" {
		t.Errorf("SelectUserByGitHubID() email = %q, want %q", got.Email, "")
	}

	gotUser, gotOptedIn, gotErr := SelectUserBySlackID(nil, id)
	if gotErr != nil {
		t.Fatalf("SelectUserBySlackID() error = %v", gotErr)
	}
	if gotUser.Email != "" {
		t.Errorf("SelectUserBySlackID() email = %q, want %q", gotUser.Email, "")
	}
	if gotOptedIn {
		t.Errorf("SelectUserBySlackID() optedIn = %v, want %v", gotOptedIn, false)
	}
}
