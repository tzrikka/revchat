package data

import (
	"testing"
)

func TestOptIn(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)
	pathCache = map[string]string{} // Reset global state.

	email := "email@example.com"
	accountID := "BitbucketAccountID"
	login := "GitHubLogin"

	// Before opting in.
	optedIn, err := IsOptedIn(email)
	want := false
	if err != nil {
		t.Fatalf("IsOptedIn() error = %v", err)
	}
	if optedIn != want {
		t.Fatalf("IsOptedIn() = %v, want %v", optedIn, want)
	}

	// Opt in.
	if err := OptInBitbucketUser("SlackUserID", accountID, email, "linkID"); err != nil {
		t.Fatalf("OptInBitbucketUser() error = %v", err)
	}
	if err := OptInGitHubUser("SlackUserID", login, email, "linkID"); err != nil {
		t.Fatalf("OptInGitHubUser() error = %v", err)
	}

	// After opting in.
	optedIn, err = IsOptedIn(email)
	want = true
	if err != nil {
		t.Fatalf("IsOptedIn() error = %v", err)
	}
	if optedIn != want {
		t.Fatalf("IsOptedIn() = %v, want %v", optedIn, want)
	}

	// Opt out.
	if err := OptOut(email); err != nil {
		t.Fatalf("OptOut() error = %v", err)
	}

	// After opting out.
	optedIn, err = IsOptedIn(email)
	want = false
	if err != nil {
		t.Fatalf("IsOptedIn() error = %v", err)
	}
	if optedIn != want {
		t.Fatalf("IsOptedIn() = %v, want %v", optedIn, want)
	}

	// User ID/email mapping still exists.
	gotUser, err := SelectUserByBitbucketID(accountID)
	if err != nil {
		t.Errorf("SelectUserByBitbucketID() error = %v", err)
	}
	if gotUser.Email != email {
		t.Errorf("SelectUserByBitbucketID() email = %q, want %q", gotUser.Email, email)
	}

	gotUser, err = SelectUserByEmail(email)
	if err != nil {
		t.Errorf("SelectUserByEmail() error = %v", err)
	}
	if gotUser.GitHubID != login {
		t.Errorf("SelectUserByEmail() GitHub ID = %q, want %q", gotUser.GitHubID, login)
	}
}
