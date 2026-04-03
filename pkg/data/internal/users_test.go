package internal_test

import (
	"testing"

	"github.com/tzrikka/revchat/pkg/data/internal"
)

func TestUsers(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	id := "user_id"
	name := "First Last"
	email := "email@example.com"

	// Before adding the user.
	gotUser, gotErr := internal.SelectUser(t.Context(), internal.IndexByBitbucketID, id)
	if gotErr != nil {
		t.Fatalf("SelectUser() error = %v", gotErr)
	}
	if gotUser.Email != "" {
		t.Fatalf("SelectUser() email = %q, want %q", gotUser.Email, "")
	}

	// Add the user.
	if _, err := internal.UpsertUser(t.Context(), email, name, id, "", "", ""); err != nil {
		t.Fatalf("UpsertUser() error = %v", err)
	}

	// After adding the user.
	gotUser, gotErr = internal.SelectUser(t.Context(), internal.IndexByBitbucketID, id)
	if gotErr != nil {
		t.Fatalf("SelectUser() error = %v", gotErr)
	}
	if gotUser.Email != email {
		t.Errorf("SelectUser() email = %q, want %q", gotUser.Email, email)
	}
	if gotUser.RealName != name {
		t.Errorf("SelectUser() name = %q, want %q", gotUser.RealName, name)
	}

	gotUser, gotErr = internal.SelectUser(t.Context(), internal.IndexByGitHubID, id)
	if gotErr != nil {
		t.Fatalf("SelectUser() error = %v", gotErr)
	}
	if gotUser.Email != "" {
		t.Errorf("SelectUser() email = %q, want %q", gotUser.Email, "")
	}

	gotUser, gotErr = internal.SelectUser(t.Context(), internal.IndexBySlackID, id)
	if gotErr != nil {
		t.Fatalf("SelectUser() error = %v", gotErr)
	}
	if gotUser.Email != "" {
		t.Errorf("SelectUser() email = %q, want %q", gotUser.Email, "")
	}
	if gotUser.IsOptedIn() {
		t.Errorf("SelectUser() optedIn = %v, want %v", gotUser.IsOptedIn(), false)
	}
}
