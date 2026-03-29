package data2

import (
	"fmt"
	"reflect"
	"testing"
)

func TestInitTurns(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	url := "https://bitbucket.org/workspace/repo/pull-requests/1"

	// Pre-initialized state (missing file).
	got, err := LoadCurrentTurnEmails(nil, url)
	if err == nil {
		t.Fatal("LoadCurrentTurnEmails() error = nil, want = true")
	}
	want := []string{}
	if len(got) != 0 {
		t.Fatalf("LoadCurrentTurnEmails() = %q, want = %q", got, want)
	}

	// Initialize PR without reviewers.
	InitTurns(nil, url, "author@example.com")

	got, err = LoadCurrentTurnEmails(nil, url)
	if err != nil {
		t.Fatalf("LoadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"author@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadCurrentTurnEmails() = %v, want %v", got, want)
	}

	// Initialize another PR without reviewers.
	url = "https://bitbucket.org/workspace/repo/pull-requests/2"
	InitTurns(nil, url, "bot")

	got, err = LoadCurrentTurnEmails(nil, url)
	if err != nil {
		t.Fatalf("LoadCurrentTurnEmails() error = %v", err)
	}
	want = []string{}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadCurrentTurnEmails() = %v, want %v", got, want)
	}
}

func TestDeleteTurns(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	url := "https://github.com/owner/repo/pull/1"
	InitTurns(nil, url, "author@example.com")

	DeleteTurns(nil, url)

	_, err := LoadCurrentTurnEmails(nil, url)
	if err == nil {
		t.Fatalf("LoadCurrentTurnEmails() error = nil, want = true")
	}
}

func TestLoadCurrentTurnEmails(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	tests := []struct {
		name     string
		author   string
		reviewer string
		approver string
		want     []string
	}{
		{
			name:   "normal_author",
			author: "author@example.com",
			want:   []string{"author@example.com"},
		},
		{
			name:   "bot_author",
			author: "bot",
			want:   []string{},
		},
		// With reviewer.
		{
			name:     "normal_author_with_reviewer",
			author:   "author@example.com",
			reviewer: "reviewer@example.com",
			want:     []string{"reviewer@example.com"},
		},
		{
			name:     "bot_author_with_reviewer",
			author:   "bot",
			reviewer: "reviewer@example.com",
			want:     []string{"reviewer@example.com"},
		},
		// With approver (i.e. no longer a reviewer).
		{
			name:     "normal_author_with_approver",
			author:   "author@example.com",
			approver: "approver@example.com",
			want:     []string{"author@example.com"},
		},
		{
			name:     "bot_author_with_approver",
			author:   "bot",
			approver: "approver@example.com",
			want:     []string{},
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := fmt.Sprintf("https://github.com/owner/repo/pull/%d", i+1)
			InitTurns(nil, url, tt.author)

			if tt.reviewer != "" {
				gotDone, gotApproved, gotErr := SetReviewerTurn(nil, url, tt.reviewer, false)
				if gotErr != nil {
					t.Fatalf("SetReviewerTurn() error = %v", gotErr)
				}
				if !gotDone {
					t.Fatalf("SetReviewerTurn() done = %v, want %v", gotDone, true)
				}
				if gotApproved {
					t.Fatalf("SetReviewerTurn() approved = %v, want %v", gotApproved, false)
				}
			}

			got, err := LoadCurrentTurnEmails(nil, url)
			if err != nil {
				t.Fatalf("LoadCurrentTurnEmails() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("LoadCurrentTurnEmails() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTurns(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	url := "https://bitbucket.org/workspace/repo/pull-requests/1"

	// Initialize state without reviewers.
	InitTurns(nil, url, "author@example.com")

	got, err := LoadCurrentTurnEmails(nil, url)
	if err != nil {
		t.Fatalf("LoadCurrentTurnEmails() error = %v", err)
	}
	want := []string{"author@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadCurrentTurnEmails() = %v, want %v", got, want)
	}

	// Add reviewers.
	done, approved, err := SetReviewerTurn(nil, url, "rev1", false)
	if err != nil {
		t.Fatalf("SetReviewerTurn() error = %v", err)
	}
	if !done {
		t.Fatalf("SetReviewerTurn() done = %v, want %v", done, true)
	}
	if approved {
		t.Fatalf("SetReviewerTurn() approved = %v, want %v", approved, false)
	}

	got, err = LoadCurrentTurnEmails(nil, url)
	if err != nil {
		t.Fatalf("LoadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"rev1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadCurrentTurnEmails() = %v, want %v", got, want)
	}

	done, approved, err = SetReviewerTurn(nil, url, "rev2", false)
	if err != nil {
		t.Fatalf("SetReviewerTurn() error = %v", err)
	}
	if !done {
		t.Fatalf("SetReviewerTurn() done = %v, want %v", done, true)
	}
	if approved {
		t.Fatalf("SetReviewerTurn() approved = %v, want %v", approved, false)
	}

	got, err = LoadCurrentTurnEmails(nil, url)
	if err != nil {
		t.Fatalf("LoadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"rev1", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadCurrentTurnEmails() = %v, want %v", got, want)
	}

	done, approved, err = SetReviewerTurn(nil, url, "rev2", false) // should be a no-op.
	if err != nil {
		t.Fatalf("SetReviewerTurn() error = %v", err)
	}
	if !done {
		t.Fatalf("SetReviewerTurn() done = %v, want %v", done, true)
	}
	if approved {
		t.Fatalf("SetReviewerTurn() approved = %v, want %v", approved, false)
	}

	got, err = LoadCurrentTurnEmails(nil, url)
	if err != nil {
		t.Fatalf("LoadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"rev1", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadCurrentTurnEmails() = %v, want %v", got, want)
	}

	done, approved, err = SetReviewerTurn(nil, url, "author@example.com", false) // should be a no-op.
	if err != nil {
		t.Fatalf("SetReviewerTurn() error = %v", err)
	}
	if !done {
		t.Fatalf("SetReviewerTurn() done = %v, want %v", done, true)
	}
	if approved {
		t.Fatalf("SetReviewerTurn() approved = %v, want %v", approved, false)
	}

	got, err = LoadCurrentTurnEmails(nil, url)
	if err != nil {
		t.Fatalf("LoadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"rev1", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadCurrentTurnEmails() = %v, want %v", got, want)
	}

	// Update turn states.
	err = SwitchTurn(nil, url, "rev1", false)
	if err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}
	got, err = LoadCurrentTurnEmails(nil, url)
	if err != nil {
		t.Fatalf("LoadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"author@example.com", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadCurrentTurnEmails() = %v, want %v", got, want)
	}

	err = SwitchTurn(nil, url, "rev2", false)
	if err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}
	got, err = LoadCurrentTurnEmails(nil, url)
	if err != nil {
		t.Fatalf("LoadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"author@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadCurrentTurnEmails() = %v, want %v", got, want)
	}

	err = SwitchTurn(nil, url, "author@example.com", false)
	if err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}
	got, err = LoadCurrentTurnEmails(nil, url)
	if err != nil {
		t.Fatalf("LoadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"rev1", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadCurrentTurnEmails() = %v, want %v", got, want)
	}

	ok, err := FreezeTurns(nil, url, "someone")
	if err != nil {
		t.Fatalf("FreezeTurns() error = %v", err)
	}
	if !ok {
		t.Fatalf("FreezeTurns() = %v, want %v", ok, true)
	}
	ok, err = FreezeTurns(nil, url, "someone")
	if err != nil {
		t.Fatalf("FreezeTurns() error = %v", err)
	}
	if ok {
		t.Fatalf("FreezeTurns() = %v, want %v", ok, false)
	}

	err = SwitchTurn(nil, url, "rev1", false)
	if err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}
	err = SwitchTurn(nil, url, "rev2", false)
	if err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}
	got, err = LoadCurrentTurnEmails(nil, url)
	if err != nil {
		t.Fatalf("LoadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"rev1", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadCurrentTurnEmails() = %v, want %v", got, want)
	}

	// Force switch while frozen.
	err = SwitchTurn(nil, url, "rev1", true)
	if err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}
	got, err = LoadCurrentTurnEmails(nil, url)
	if err != nil {
		t.Fatalf("LoadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"author@example.com", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadCurrentTurnEmails() = %v, want %v", got, want)
	}

	// Add "rev1" back while still frozen.
	err = SwitchTurn(nil, url, "author@example.com", true)
	if err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}
	got, err = LoadCurrentTurnEmails(nil, url)
	if err != nil {
		t.Fatalf("LoadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"rev1", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadCurrentTurnEmails() = %v, want %v", got, want)
	}

	err = RemoveReviewerFromTurns(nil, url, "rev1", false)
	if err != nil {
		t.Fatalf("RemoveReviewerFromTurns() error = %v", err)
	}
	got, err = LoadCurrentTurnEmails(nil, url)
	if err != nil {
		t.Fatalf("LoadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadCurrentTurnEmails() = %v, want %v", got, want)
	}

	err = RemoveReviewerFromTurns(nil, url, "rev1", false) // Should be a no-op.
	if err != nil {
		t.Fatalf("RemoveReviewerFromTurns() error = %v", err)
	}
	got, err = LoadCurrentTurnEmails(nil, url)
	if err != nil {
		t.Fatalf("LoadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadCurrentTurnEmails() = %v, want %v", got, want)
	}

	ok, err = UnfreezeTurns(nil, url)
	if err != nil {
		t.Fatalf("UnfreezeTurns() error = %v", err)
	}
	if !ok {
		t.Fatalf("UnfreezeTurns() = %v, want %v", ok, true)
	}
	ok, err = UnfreezeTurns(nil, url)
	if err != nil {
		t.Fatalf("UnfreezeTurns() error = %v", err)
	}
	if ok {
		t.Fatalf("UnfreezeTurns() = %v, want %v", ok, false)
	}

	err = SwitchTurn(nil, url, "rev2", false)
	if err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}
	got, err = LoadCurrentTurnEmails(nil, url)
	if err != nil {
		t.Fatalf("LoadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"author@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadCurrentTurnEmails() = %v, want %v", got, want)
	}
}

func TestFrozen(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	url := "https://bitbucket.org/workspace/repo/pull-requests/1"

	InitTurns(nil, url, "author@example.com")

	at, by := IsFrozen(nil, url)
	if !at.IsZero() {
		t.Fatalf("Frozen() at = %v, want zero time", at)
	}
	if by != "" {
		t.Fatalf("Frozen() by = %q, want empty string", by)
	}

	email := "freezer"
	_, err := FreezeTurns(nil, url, email)
	if err != nil {
		t.Fatalf("FreezeTurns() error = %v", err)
	}

	at, by = IsFrozen(nil, url)
	if at.IsZero() {
		t.Fatalf("Frozen() at = zero time, want non-zero time")
	}
	if by != email {
		t.Fatalf("Frozen() by = %q, want %q", by, email)
	}
}

func TestNudge(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	url := "https://bitbucket.org/workspace/repo/pull-requests/1"

	// Initialize state.
	InitTurns(nil, url, "author@example.com")
	done, approved, err := SetReviewerTurn(nil, url, "rev1", false)
	if err != nil {
		t.Fatalf("SetReviewerTurn() error = %v", err)
	}
	if !done {
		t.Fatalf("SetReviewerTurn() done = %v, want %v", done, true)
	}
	if approved {
		t.Fatalf("SetReviewerTurn() approved = %v, want %v", approved, false)
	}
	done, approved, err = SetReviewerTurn(nil, url, "rev2", false)
	if err != nil {
		t.Fatalf("SetReviewerTurn() error = %v", err)
	}
	if !done {
		t.Fatalf("SetReviewerTurn() done = %v, want %v", done, true)
	}
	if approved {
		t.Fatalf("SetReviewerTurn() approved = %v, want %v", approved, false)
	}

	got, err := LoadCurrentTurnEmails(nil, url)
	if err != nil {
		t.Fatalf("LoadCurrentTurnEmails() error = %v", err)
	}
	want := []string{"rev1", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurns() = %v, want %v", got, want)
	}

	// Nudge a non-reviewer.
	ok, approved, err := SetReviewerTurn(nil, url, "non-reviewer", true)
	if err != nil {
		t.Fatalf("SetReviewerTurn() error = %v", err)
	}
	if ok {
		t.Fatalf("SetReviewerTurn() ok = %v, want %v", ok, false)
	}
	if approved {
		t.Fatalf("SetReviewerTurn() approved = %v, want %v", approved, false)
	}

	// Rev1 reviews, author nudges rev2.
	if err := SwitchTurn(nil, url, "rev1", false); err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}

	ok, approved, err = SetReviewerTurn(nil, url, "rev2", true)
	if err != nil {
		t.Fatalf("SetReviewerTurn() error = %v", err)
	}
	if !ok {
		t.Fatalf("SetReviewerTurn() = %v, want %v", ok, true)
	}
	if approved {
		t.Fatalf("SetReviewerTurn() approved = %v, want %v", approved, false)
	}

	got, err = LoadCurrentTurnEmails(nil, url)
	if err != nil {
		t.Fatalf("LoadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"author@example.com", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadCurrentTurnEmails() = %v, want %v", got, want)
	}

	// Rev2 reviews -> it's the author's turn --> nudge the author.
	if err := SwitchTurn(nil, url, "rev2", false); err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}

	got, err = LoadCurrentTurnEmails(nil, url)
	if err != nil {
		t.Fatalf("LoadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"author@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadCurrentTurnEmails() = %v, want %v", got, want)
	}

	ok, approved, err = SetReviewerTurn(nil, url, "author@example.com", true)
	if err != nil {
		t.Fatalf("SetReviewerTurn() error = %v", err)
	}
	if !ok {
		t.Fatalf("SetReviewerTurn() = %v, want %v", ok, true)
	}
	if approved {
		t.Fatalf("SetReviewerTurn() approved = %v, want %v", approved, false)
	}

	got, err = LoadCurrentTurnEmails(nil, url)
	if err != nil {
		t.Fatalf("LoadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"author@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadCurrentTurnEmails() = %v, want %v", got, want)
	}

	// Author responds to comments --> it's rev1 and rev2's turn again.
	if err := SwitchTurn(nil, url, "author@example.com", false); err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}

	got, err = LoadCurrentTurnEmails(nil, url)
	if err != nil {
		t.Fatalf("LoadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"rev1", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadCurrentTurnEmails() = %v, want %v", got, want)
	}

	// Rev1 approves, and gets removed from the turn --> it's rev2's turn
	// (not the author, because it's currently the turn of "all the remaining reviewers").
	if err := RemoveReviewerFromTurns(nil, url, "rev1", true); err != nil {
		t.Fatalf("RemoveReviewerFromTurns() error = %v", err)
	}

	got, err = LoadCurrentTurnEmails(nil, url)
	if err != nil {
		t.Fatalf("LoadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadCurrentTurnEmails() = %v, want %v", got, want)
	}

	// Can't nudge rev1 anymore (still a reviewer in Bitbucket, but not tracked by RevChat in this PR).
	ok, approved, err = SetReviewerTurn(nil, url, "rev1", true)
	if err != nil {
		t.Fatalf("SetReviewerTurn() error = %v", err)
	}
	if ok {
		t.Fatalf("SetReviewerTurn() = %v, want %v", ok, false)
	}
	if !approved {
		t.Fatalf("SetReviewerTurn() approved = %v, want %v", approved, true)
	}

	// Rev2 nudged the author after some offline discussion.
	ok, approved, err = SetReviewerTurn(nil, url, "author@example.com", true)
	if err != nil {
		t.Fatalf("SetReviewerTurn() error = %v", err)
	}
	if !ok {
		t.Fatalf("SetReviewerTurn() = %v, want %v", ok, true)
	}
	if approved {
		t.Fatalf("SetReviewerTurn() approved = %v, want %v", approved, false)
	}

	got, err = LoadCurrentTurnEmails(nil, url)
	if err != nil {
		t.Fatalf("LoadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"author@example.com", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadCurrentTurnEmails() = %v, want %v", got, want)
	}

	// Author responds to comments --> it's rev2's turn again.
	if err := SwitchTurn(nil, url, "author@example.com", false); err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}

	got, err = LoadCurrentTurnEmails(nil, url)
	if err != nil {
		t.Fatalf("LoadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadCurrentTurnEmails() = %v, want %v", got, want)
	}

	// Rev2 approves too --> it's the author's turn again.
	if err := SwitchTurn(nil, url, "rev2", false); err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}

	got, err = LoadCurrentTurnEmails(nil, url)
	if err != nil {
		t.Fatalf("LoadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"author@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadCurrentTurnEmails() = %v, want %v", got, want)
	}
}
