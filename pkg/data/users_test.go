package data

import (
	"testing"
)

func TestUsers(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	id := "user_id"
	email := "email@example.com"

	// Before adding the user.
	got, err := BitbucketUserEmailByID(id)
	if err != nil {
		t.Fatalf("BitbucketUserEmail() error = %v", err)
	}
	if got != "" {
		t.Fatalf("BitbucketUserEmail() = %q, want %q", got, "")
	}

	// Add the user.
	if err := AddBitbucketUser(id, email); err != nil {
		t.Fatalf("AddBitbucketUser() error = %v", err)
	}

	// After adding the user.
	got, err = BitbucketUserEmailByID(id)
	if err != nil {
		t.Errorf("BitbucketUserEmail() error = %v", err)
	}
	if got != email {
		t.Errorf("BitbucketUserEmail() = %q, want %q", got, email)
	}

	got, err = GitHubUserEmailByID(id)
	if err != nil {
		t.Errorf("GitHubUserEmail() error = %v", err)
	}
	if got != "" {
		t.Errorf("GitHubUserEmail() = %q, want %q", got, "")
	}

	got, err = SlackUserEmailByID(id)
	if err != nil {
		t.Errorf("SlackUserEmail() error = %v", err)
	}
	if got != "" {
		t.Errorf("SlackUserEmail() = %q, want %q", got, "")
	}

	// Remove the user.
	if err := RemoveBitbucketUser(email); err != nil {
		t.Fatalf("RemoveBitbucketUser() error = %v", err)
	}

	// After removing the usr.
	got, err = BitbucketUserEmailByID(id)
	if err != nil {
		t.Errorf("BitbucketUserEmail() error = %v", err)
	}
	if got != "" {
		t.Errorf("BitbucketUserEmail() = %q, want %q", got, "")
	}

	got, err = GitHubUserEmailByID(id)
	if err != nil {
		t.Errorf("GitHubUserEmail() error = %v", err)
	}
	if got != "" {
		t.Errorf("GitHubUserEmail() = %q, want %q", got, "")
	}

	got, err = SlackUserEmailByID(id)
	if err != nil {
		t.Errorf("SlackUserEmail() error = %v", err)
	}
	if got != "" {
		t.Errorf("SlackUserEmail() = %q, want %q", got, "")
	}
}
