package data

import "testing"

func TestUsers(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", d)

	id := "user_id"
	email := "email@example.com"

	// Before adding the user.
	gotEmail, err := BitbucketUserEmailByID(id)
	if err != nil {
		t.Fatalf("BitbucketUserEmail() error = %v", err)
	}
	if gotEmail != "" {
		t.Fatalf("BitbucketUserEmail() = %q, want %q", gotEmail, email)
	}

	// Add the user.
	if err := AddBitbucketUser(id, email); err != nil {
		t.Fatalf("AddBitbucketUser() error = %v", err)
	}

	// After adding the user.
	gotEmail, err = BitbucketUserEmailByID(id)
	if err != nil {
		t.Errorf("BitbucketUserEmail() error = %v", err)
	}
	if gotEmail != email {
		t.Errorf("BitbucketUserEmail() = %q, want %q", gotEmail, email)
	}

	gotEmail, err = GitHubUserEmailByID(id)
	if err != nil {
		t.Errorf("GitHubUserEmail() error = %v", err)
	}
	if gotEmail != "" {
		t.Errorf("GitHubUserEmail() = %q, want %q", gotEmail, email)
	}

	gotEmail, err = SlackUserEmailByID(id)
	if err != nil {
		t.Errorf("SlackUserEmail() error = %v", err)
	}
	if gotEmail != "" {
		t.Errorf("SlackUserEmail() = %q, want %q", gotEmail, email)
	}

	// Remove the user.
	if err := RemoveBitbucketUser(email); err != nil {
		t.Fatalf("RemoveBitbucketUser() error = %v", err)
	}

	// After removing the usr.
	gotEmail, err = BitbucketUserEmailByID(id)
	if err != nil {
		t.Errorf("BitbucketUserEmail() error = %v", err)
	}
	if gotEmail != "" {
		t.Errorf("BitbucketUserEmail() = %q, want %q", gotEmail, email)
	}

	gotEmail, err = GitHubUserEmailByID(id)
	if err != nil {
		t.Errorf("GitHubUserEmail() error = %v", err)
	}
	if gotEmail != "" {
		t.Errorf("GitHubUserEmail() = %q, want %q", gotEmail, email)
	}

	gotEmail, err = SlackUserEmailByID(id)
	if err != nil {
		t.Errorf("SlackUserEmail() error = %v", err)
	}
	if gotEmail != "" {
		t.Errorf("SlackUserEmail() = %q, want %q", gotEmail, email)
	}
}
