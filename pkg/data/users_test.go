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
	gotUser, err := SelectUserByBitbucketID(id)
	if err != nil {
		t.Fatalf("SelectUserByBitbucketID() error = %v", err)
	}
	if gotUser.Email != "" {
		t.Fatalf("SelectUserByBitbucketID() email = %q, want %q", gotUser.Email, "")
	}

	// Add the user.
	if err := UpsertUser(email, id, "", "", "", "", ""); err != nil {
		t.Fatalf("UpsertUser() error = %v", err)
	}

	// After adding the user.
	gotUser, err = SelectUserByBitbucketID(id)
	if err != nil {
		t.Errorf("SelectUserByBitbucketID() error = %v", err)
	}
	if gotUser.Email != email {
		t.Errorf("SelectUserByBitbucketID() email = %q, want %q", gotUser.Email, email)
	}

	gotUser, err = SelectUserByGitHubID(id)
	if err != nil {
		t.Errorf("SelectUserByGitHubID() error = %v", err)
	}
	if gotUser.Email != "" {
		t.Errorf("SelectUserByGitHubID() email = %q, want %q", gotUser.Email, "")
	}

	gotUser, err = SelectUserBySlackID(id)
	if err != nil {
		t.Errorf("SelectUserBySlackID() error = %v", err)
	}
	if gotUser.Email != "" {
		t.Errorf("SelectUserBySlackID() email = %q, want %q", gotUser.Email, "")
	}
}
