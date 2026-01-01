package data

import (
	"testing"
)

func TestUsers(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)
	pathCache = map[string]string{} // Reset global state.

	id := "user_id"
	email := "email@example.com"

	// Before adding the user.
	got := SelectUserByBitbucketID(nil, id)
	if got.Email != "" {
		t.Fatalf("SelectUserByBitbucketID() email = %q, want %q", got.Email, "")
	}

	// Add the user.
	if err := UpsertUser(nil, email, "", id, "", "", ""); err != nil {
		t.Fatalf("UpsertUser() error = %v", err)
	}

	// After adding the user.
	got = SelectUserByBitbucketID(nil, id)
	if got.Email != email {
		t.Errorf("SelectUserByBitbucketID() email = %q, want %q", got.Email, email)
	}

	got, err := SelectUserByGitHubID(id)
	if err != nil {
		t.Errorf("SelectUserByGitHubID() error = %v", err)
	}
	if got.Email != "" {
		t.Errorf("SelectUserByGitHubID() email = %q, want %q", got.Email, "")
	}

	got, err = SelectUserBySlackID(id)
	if err != nil {
		t.Errorf("SelectUserBySlackID() error = %v", err)
	}
	if got.Email != "" {
		t.Errorf("SelectUserBySlackID() email = %q, want %q", got.Email, "")
	}
}
