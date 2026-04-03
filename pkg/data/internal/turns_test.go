package internal

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestNormalizeEmailAddresses(t *testing.T) {
	turn := &PRTurns{
		Author:   "AUTHOR",
		FrozenBy: "FrozenBy",
		Reviewers: map[string]bool{
			"Rev1": true,
			"rev2": false,
			"REV3": true,
		},
	}

	normalizeEmailAddresses(turn)

	if turn.Author != "author" {
		t.Fatalf("normalizeEmailAddresses() Author = %q, want %q", turn.Author, "author")
	}
	if turn.FrozenBy != "frozenby" {
		t.Fatalf("normalizeEmailAddresses() FrozenBy = %q, want %q", turn.FrozenBy, "frozenby")
	}
	wantReviewers := map[string]bool{
		"rev1": true,
		"rev2": false,
		"rev3": true,
	}
	if !reflect.DeepEqual(turn.Reviewers, wantReviewers) {
		t.Fatalf("normalizeEmailAddresses() Reviewers = %v, want %v", turn.Reviewers, wantReviewers)
	}
}

func TestListOf(t *testing.T) {
	tests := []struct {
		name string
		pr   map[string]any
		key  string
		want []any
	}{
		{
			name: "missing_key",
			pr:   map[string]any{},
			key:  "key",
			want: []any{},
		},
		{
			name: "good_key",
			pr: map[string]any{
				"key": []any{"a", "b", "c"},
			},
			key:  "key",
			want: []any{"a", "b", "c"},
		},
		{
			name: "wrong_type",
			pr: map[string]any{
				"key": "not a list",
			},
			key:  "key",
			want: []any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := listOf(tt.pr, tt.key); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("listOf() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserActivity(t *testing.T) {
	tests := []struct {
		name         string
		detailsMap   any
		wantApproved bool
		wantZero     bool
	}{
		{
			name:       "invalid_participant",
			detailsMap: "not a map",
			wantZero:   true,
		},
		{
			name: "invalid_user",
			detailsMap: map[string]any{
				"user": "not a map",
			},
			wantZero: true,
		},
		{
			name: "missing_approved",
			detailsMap: map[string]any{
				"user": map[string]any{},
			},
			wantZero: true,
		},
		{
			name: "invalid_approved",
			detailsMap: map[string]any{
				"user":     map[string]any{},
				"approved": "not a bool",
			},
			wantZero: true,
		},
		{
			name: "approved",
			detailsMap: map[string]any{
				"user":            map[string]any{},
				"approved":        true,
				"participated_on": time.Now().UTC().Format(time.RFC3339),
			},
			wantApproved: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			email, approved, ts := userActivity(t.Context(), tt.detailsMap)
			if email != "" {
				t.Errorf("userActivity() email = %q, want %q", email, "")
			}
			if approved != tt.wantApproved {
				t.Errorf("userActivity() approved = %v, want %v", approved, tt.wantApproved)
			}
			if ts.IsZero() != tt.wantZero {
				t.Errorf("userActivity() timestamp = %v, want zero: %v", ts, tt.wantZero)
			}
		})
	}
}

// The unit tests below are the same as in data/turns_test.go.

func TestInitTurns(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	url := "https://bitbucket.org/workspace/repo/pull-requests/1"

	// Pre-initialized state (missing file).
	got, err := ReadCurrentTurnEmails(t.Context(), url)
	if err == nil {
		t.Fatal("ReadCurrentTurnEmails() error = nil, want = true")
	}
	want := []string{}
	if len(got) != 0 {
		t.Fatalf("ReadCurrentTurnEmails() = %q, want = %q", got, want)
	}

	// Initialize PR without reviewers.
	if err := InitTurns(url, "author@example.com"); err != nil {
		t.Fatalf("InitTurns() error = %v", err)
	}

	got, err = ReadCurrentTurnEmails(t.Context(), url)
	if err != nil {
		t.Fatalf("ReadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"author@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadCurrentTurnEmails() = %v, want %v", got, want)
	}

	// Initialize another PR without reviewers.
	url = "https://bitbucket.org/workspace/repo/pull-requests/2"
	if err := InitTurns(url, "bot"); err != nil {
		t.Fatalf("InitTurns() error = %v", err)
	}

	got, err = ReadCurrentTurnEmails(t.Context(), url)
	if err != nil {
		t.Fatalf("ReadCurrentTurnEmails() error = %v", err)
	}
	want = []string{}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadCurrentTurnEmails() = %v, want %v", got, want)
	}
}

func TestDeleteTurns(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	url := "https://github.com/owner/repo/pull/1"
	if err := InitTurns(url, "author@example.com"); err != nil {
		t.Fatalf("InitTurns() error = %v", err)
	}

	if err := DeleteGenericPRFile(t.Context(), url+TurnsFileSuffix); err != nil {
		t.Fatalf("DeleteTurns() error = %v", err)
	}

	if _, err := ReadCurrentTurnEmails(t.Context(), url); err == nil {
		t.Fatalf("ReadCurrentTurnEmails() error = nil")
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
			if err := InitTurns(url, tt.author); err != nil {
				t.Fatalf("InitTurns() error = %v", err)
			}

			if tt.reviewer != "" {
				gotStates, gotErr := SetReviewerTurn(t.Context(), url, tt.reviewer, false)
				if gotErr != nil {
					t.Fatalf("SetReviewerTurn() error = %v", gotErr)
				}
				if !gotStates[0] {
					t.Fatalf("SetReviewerTurn() done = %v, want %v", gotStates[0], true)
				}
				if gotStates[1] {
					t.Fatalf("SetReviewerTurn() approved = %v, want %v", gotStates[1], false)
				}
			}

			got, err := ReadCurrentTurnEmails(t.Context(), url)
			if err != nil {
				t.Fatalf("ReadCurrentTurnEmails() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ReadCurrentTurnEmails() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTurns(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	url := "https://bitbucket.org/workspace/repo/pull-requests/1"

	// Initialize state without reviewers.
	if err := InitTurns(url, "author@example.com"); err != nil {
		t.Fatalf("InitTurns() error = %v", err)
	}

	got, err := ReadCurrentTurnEmails(t.Context(), url)
	if err != nil {
		t.Fatalf("ReadCurrentTurnEmails() error = %v", err)
	}
	want := []string{"author@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadCurrentTurnEmails() = %v, want %v", got, want)
	}

	// Add reviewers.
	states, err := SetReviewerTurn(t.Context(), url, "rev1", false)
	if err != nil {
		t.Fatalf("SetReviewerTurn() error = %v", err)
	}
	if !states[0] {
		t.Fatalf("SetReviewerTurn() done = %v, want %v", states[0], true)
	}
	if states[1] {
		t.Fatalf("SetReviewerTurn() approved = %v, want %v", states[1], false)
	}

	got, err = ReadCurrentTurnEmails(t.Context(), url)
	if err != nil {
		t.Fatalf("ReadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"rev1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadCurrentTurnEmails() = %v, want %v", got, want)
	}

	states, err = SetReviewerTurn(t.Context(), url, "rev2", false)
	if err != nil {
		t.Fatalf("SetReviewerTurn() error = %v", err)
	}
	if !states[0] {
		t.Fatalf("SetReviewerTurn() done = %v, want %v", states[0], true)
	}
	if states[1] {
		t.Fatalf("SetReviewerTurn() approved = %v, want %v", states[1], false)
	}

	got, err = ReadCurrentTurnEmails(t.Context(), url)
	if err != nil {
		t.Fatalf("ReadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"rev1", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadCurrentTurnEmails() = %v, want %v", got, want)
	}

	states, err = SetReviewerTurn(t.Context(), url, "rev2", false) // should be a no-op.
	if err != nil {
		t.Fatalf("SetReviewerTurn() error = %v", err)
	}
	if !states[0] {
		t.Fatalf("SetReviewerTurn() done = %v, want %v", states[0], true)
	}
	if states[1] {
		t.Fatalf("SetReviewerTurn() approved = %v, want %v", states[1], false)
	}

	got, err = ReadCurrentTurnEmails(t.Context(), url)
	if err != nil {
		t.Fatalf("ReadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"rev1", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadCurrentTurnEmails() = %v, want %v", got, want)
	}

	states, err = SetReviewerTurn(t.Context(), url, "author@example.com", false) // should be a no-op.
	if err != nil {
		t.Fatalf("SetReviewerTurn() error = %v", err)
	}
	if !states[0] {
		t.Fatalf("SetReviewerTurn() done = %v, want %v", states[0], true)
	}
	if states[1] {
		t.Fatalf("SetReviewerTurn() approved = %v, want %v", states[1], false)
	}

	got, err = ReadCurrentTurnEmails(t.Context(), url)
	if err != nil {
		t.Fatalf("ReadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"rev1", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadCurrentTurnEmails() = %v, want %v", got, want)
	}

	// Update turn states.
	err = SwitchTurn(t.Context(), url, "rev1", false)
	if err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}
	got, err = ReadCurrentTurnEmails(t.Context(), url)
	if err != nil {
		t.Fatalf("ReadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"author@example.com", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadCurrentTurnEmails() = %v, want %v", got, want)
	}

	err = SwitchTurn(t.Context(), url, "rev2", false)
	if err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}
	got, err = ReadCurrentTurnEmails(t.Context(), url)
	if err != nil {
		t.Fatalf("ReadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"author@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadCurrentTurnEmails() = %v, want %v", got, want)
	}

	err = SwitchTurn(t.Context(), url, "author@example.com", false)
	if err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}
	got, err = ReadCurrentTurnEmails(t.Context(), url)
	if err != nil {
		t.Fatalf("ReadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"rev1", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadCurrentTurnEmails() = %v, want %v", got, want)
	}

	ok, err := FreezeTurns(t.Context(), url, "someone")
	if err != nil {
		t.Fatalf("FreezeTurns() error = %v", err)
	}
	if !ok {
		t.Fatalf("FreezeTurns() = %v, want %v", ok, true)
	}
	ok, err = FreezeTurns(t.Context(), url, "someone")
	if err != nil {
		t.Fatalf("FreezeTurns() error = %v", err)
	}
	if ok {
		t.Fatalf("FreezeTurns() = %v, want %v", ok, false)
	}

	err = SwitchTurn(t.Context(), url, "rev1", false)
	if err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}
	err = SwitchTurn(t.Context(), url, "rev2", false)
	if err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}
	got, err = ReadCurrentTurnEmails(t.Context(), url)
	if err != nil {
		t.Fatalf("ReadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"rev1", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadCurrentTurnEmails() = %v, want %v", got, want)
	}

	// Force switch while frozen.
	err = SwitchTurn(t.Context(), url, "rev1", true)
	if err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}
	got, err = ReadCurrentTurnEmails(t.Context(), url)
	if err != nil {
		t.Fatalf("ReadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"author@example.com", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadCurrentTurnEmails() = %v, want %v", got, want)
	}

	// Add "rev1" back while still frozen.
	err = SwitchTurn(t.Context(), url, "author@example.com", true)
	if err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}
	got, err = ReadCurrentTurnEmails(t.Context(), url)
	if err != nil {
		t.Fatalf("ReadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"rev1", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadCurrentTurnEmails() = %v, want %v", got, want)
	}

	err = RemoveReviewerFromTurns(t.Context(), url, "rev1", false)
	if err != nil {
		t.Fatalf("RemoveReviewerFromTurns() error = %v", err)
	}
	got, err = ReadCurrentTurnEmails(t.Context(), url)
	if err != nil {
		t.Fatalf("ReadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadCurrentTurnEmails() = %v, want %v", got, want)
	}

	err = RemoveReviewerFromTurns(t.Context(), url, "rev1", false) // Should be a no-op.
	if err != nil {
		t.Fatalf("RemoveReviewerFromTurns() error = %v", err)
	}
	got, err = ReadCurrentTurnEmails(t.Context(), url)
	if err != nil {
		t.Fatalf("ReadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadCurrentTurnEmails() = %v, want %v", got, want)
	}

	ok, err = UnfreezeTurns(t.Context(), url)
	if err != nil {
		t.Fatalf("UnfreezeTurns() error = %v", err)
	}
	if !ok {
		t.Fatalf("UnfreezeTurns() = %v, want %v", ok, true)
	}
	ok, err = UnfreezeTurns(t.Context(), url)
	if err != nil {
		t.Fatalf("UnfreezeTurns() error = %v", err)
	}
	if ok {
		t.Fatalf("UnfreezeTurns() = %v, want %v", ok, false)
	}

	err = SwitchTurn(t.Context(), url, "rev2", false)
	if err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}
	got, err = ReadCurrentTurnEmails(t.Context(), url)
	if err != nil {
		t.Fatalf("ReadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"author@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadCurrentTurnEmails() = %v, want %v", got, want)
	}
}

func TestFrozen(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	url := "https://bitbucket.org/workspace/repo/pull-requests/1"

	if err := InitTurns(url, "author@example.com"); err != nil {
		t.Fatalf("InitTurns() error = %v", err)
	}

	frozen, _ := IsFrozen(t.Context(), url)
	if !frozen.At.IsZero() {
		t.Fatalf("Frozen() at = %v, want zero time", frozen.At)
	}
	if frozen.By != "" {
		t.Fatalf("Frozen() by = %q, want empty string", frozen.By)
	}

	email := "freezer"
	_, err := FreezeTurns(t.Context(), url, email)
	if err != nil {
		t.Fatalf("FreezeTurns() error = %v", err)
	}

	frozen, _ = IsFrozen(t.Context(), url)
	if frozen.At.IsZero() {
		t.Fatalf("Frozen() at = zero time, want non-zero time")
	}
	if frozen.By != email {
		t.Fatalf("Frozen() by = %q, want %q", frozen.By, email)
	}
}

func TestNudge(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	url := "https://bitbucket.org/workspace/repo/pull-requests/1"

	// Initialize state.
	if err := InitTurns(url, "author@example.com"); err != nil {
		t.Fatalf("InitTurns() error = %v", err)
	}

	states, err := SetReviewerTurn(t.Context(), url, "rev1", false)
	if err != nil {
		t.Fatalf("SetReviewerTurn() error = %v", err)
	}
	if !states[0] {
		t.Fatalf("SetReviewerTurn() done = %v, want %v", states[0], true)
	}
	if states[1] {
		t.Fatalf("SetReviewerTurn() approved = %v, want %v", states[1], false)
	}
	states, err = SetReviewerTurn(t.Context(), url, "rev2", false)
	if err != nil {
		t.Fatalf("SetReviewerTurn() error = %v", err)
	}
	if !states[0] {
		t.Fatalf("SetReviewerTurn() done = %v, want %v", states[0], true)
	}
	if states[1] {
		t.Fatalf("SetReviewerTurn() approved = %v, want %v", states[1], false)
	}

	got, err := ReadCurrentTurnEmails(t.Context(), url)
	if err != nil {
		t.Fatalf("ReadCurrentTurnEmails() error = %v", err)
	}
	want := []string{"rev1", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadCurrentTurnEmails() = %v, want %v", got, want)
	}

	// Nudge a non-reviewer.
	states, err = SetReviewerTurn(t.Context(), url, "non-reviewer", true)
	if err != nil {
		t.Fatalf("SetReviewerTurn() error = %v", err)
	}
	if states[0] {
		t.Fatalf("SetReviewerTurn() ok = %v, want %v", states[0], false)
	}
	if states[1] {
		t.Fatalf("SetReviewerTurn() approved = %v, want %v", states[1], false)
	}

	// Rev1 reviews, author nudges rev2.
	if err := SwitchTurn(t.Context(), url, "rev1", false); err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}

	states, err = SetReviewerTurn(t.Context(), url, "rev2", true)
	if err != nil {
		t.Fatalf("SetReviewerTurn() error = %v", err)
	}
	if !states[0] {
		t.Fatalf("SetReviewerTurn() = %v, want %v", states[0], true)
	}
	if states[1] {
		t.Fatalf("SetReviewerTurn() approved = %v, want %v", states[1], false)
	}

	got, err = ReadCurrentTurnEmails(t.Context(), url)
	if err != nil {
		t.Fatalf("ReadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"author@example.com", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadCurrentTurnEmails() = %v, want %v", got, want)
	}

	// Rev2 reviews -> it's the author's turn --> nudge the author.
	if err := SwitchTurn(t.Context(), url, "rev2", false); err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}

	got, err = ReadCurrentTurnEmails(t.Context(), url)
	if err != nil {
		t.Fatalf("ReadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"author@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadCurrentTurnEmails() = %v, want %v", got, want)
	}

	states, err = SetReviewerTurn(t.Context(), url, "author@example.com", true)
	if err != nil {
		t.Fatalf("SetReviewerTurn() error = %v", err)
	}
	if !states[0] {
		t.Fatalf("SetReviewerTurn() = %v, want %v", states[0], true)
	}
	if states[1] {
		t.Fatalf("SetReviewerTurn() approved = %v, want %v", states[1], false)
	}

	got, err = ReadCurrentTurnEmails(t.Context(), url)
	if err != nil {
		t.Fatalf("ReadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"author@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadCurrentTurnEmails() = %v, want %v", got, want)
	}

	// Author responds to comments --> it's rev1 and rev2's turn again.
	if err := SwitchTurn(t.Context(), url, "author@example.com", false); err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}

	got, err = ReadCurrentTurnEmails(t.Context(), url)
	if err != nil {
		t.Fatalf("ReadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"rev1", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadCurrentTurnEmails() = %v, want %v", got, want)
	}

	// Rev1 approves, and gets removed from the turn --> it's rev2's turn
	// (not the author, because it's currently the turn of "all the remaining reviewers").
	if err := RemoveReviewerFromTurns(t.Context(), url, "rev1", true); err != nil {
		t.Fatalf("RemoveReviewerFromTurns() error = %v", err)
	}

	got, err = ReadCurrentTurnEmails(t.Context(), url)
	if err != nil {
		t.Fatalf("ReadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadCurrentTurnEmails() = %v, want %v", got, want)
	}

	// Can't nudge rev1 anymore (still a reviewer in Bitbucket, but not tracked by RevChat in this PR).
	states, err = SetReviewerTurn(t.Context(), url, "rev1", true)
	if err != nil {
		t.Fatalf("SetReviewerTurn() error = %v", err)
	}
	if states[0] {
		t.Fatalf("SetReviewerTurn() = %v, want %v", states[0], false)
	}
	if !states[1] {
		t.Fatalf("SetReviewerTurn() approved = %v, want %v", states[1], true)
	}

	// Rev2 nudged the author after some offline discussion.
	states, err = SetReviewerTurn(t.Context(), url, "author@example.com", true)
	if err != nil {
		t.Fatalf("SetReviewerTurn() error = %v", err)
	}
	if !states[0] {
		t.Fatalf("SetReviewerTurn() = %v, want %v", states[0], true)
	}
	if states[1] {
		t.Fatalf("SetReviewerTurn() approved = %v, want %v", states[1], false)
	}

	got, err = ReadCurrentTurnEmails(t.Context(), url)
	if err != nil {
		t.Fatalf("ReadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"author@example.com", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadCurrentTurnEmails() = %v, want %v", got, want)
	}

	// Author responds to comments --> it's rev2's turn again.
	if err := SwitchTurn(t.Context(), url, "author@example.com", false); err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}

	got, err = ReadCurrentTurnEmails(t.Context(), url)
	if err != nil {
		t.Fatalf("ReadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadCurrentTurnEmails() = %v, want %v", got, want)
	}

	// Rev2 approves too --> it's the author's turn again.
	if err := SwitchTurn(t.Context(), url, "rev2", false); err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}

	got, err = ReadCurrentTurnEmails(t.Context(), url)
	if err != nil {
		t.Fatalf("ReadCurrentTurnEmails() error = %v", err)
	}
	want = []string{"author@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadCurrentTurnEmails() = %v, want %v", got, want)
	}
}
