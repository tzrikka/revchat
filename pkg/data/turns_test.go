package data

import (
	"reflect"
	"testing"
)

func TestTurns(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)
	pathCache.Clear()

	url := "https://bitbucket.org/workspace/repo/pull-requests/1"

	// Pre-initialized state (missing file).
	got, err := GetCurrentTurn(nil, url)
	if err != nil {
		t.Fatal("GetCurrentTurn() error = nil, want = true")
	}
	want := []string{""}
	if len(got) != 1 && got[0] != "" {
		t.Fatalf("GetCurrentTurn() = %q, want = %q", got, want)
	}

	// Initialize state without reviewers.
	InitTurns(nil, url, "author")

	got, err = GetCurrentTurn(nil, url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"author"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	// Add reviewers.
	err = AddReviewerToTurns(nil, url, "rev1")
	if err != nil {
		t.Fatalf("AddReviewerToTurns() error = %v", err)
	}
	got, err = GetCurrentTurn(nil, url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"rev1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	err = AddReviewerToTurns(nil, url, "rev2")
	if err != nil {
		t.Fatalf("AddReviewerToTurns() error = %v", err)
	}
	got, err = GetCurrentTurn(nil, url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"rev1", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	err = AddReviewerToTurns(nil, url, "rev2") // should be a no-op.
	if err != nil {
		t.Fatalf("AddReviewerToTurns() error = %v", err)
	}
	got, err = GetCurrentTurn(nil, url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"rev1", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	err = AddReviewerToTurns(nil, url, "author") // should be a no-op.
	if err != nil {
		t.Fatalf("AddReviewerToTurns() error = %v", err)
	}
	got, err = GetCurrentTurn(nil, url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"rev1", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	// Update turn states.
	err = SwitchTurn(nil, url, "rev1", false)
	if err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}
	got, err = GetCurrentTurn(nil, url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"author", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	err = SwitchTurn(nil, url, "rev2", false)
	if err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}
	got, err = GetCurrentTurn(nil, url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"author"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	err = SwitchTurn(nil, url, "author", false)
	if err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}
	got, err = GetCurrentTurn(nil, url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"rev1", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	ok, err := FreezeTurn(nil, url, "someone")
	if err != nil {
		t.Fatalf("FreezeTurn() error = %v", err)
	}
	if !ok {
		t.Fatalf("FreezeTurn() = %v, want %v", ok, true)
	}
	ok, err = FreezeTurn(nil, url, "someone")
	if err != nil {
		t.Fatalf("FreezeTurn() error = %v", err)
	}
	if ok {
		t.Fatalf("FreezeTurn() = %v, want %v", ok, false)
	}

	err = SwitchTurn(nil, url, "rev1", false)
	if err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}
	err = SwitchTurn(nil, url, "rev2", false)
	if err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}
	got, err = GetCurrentTurn(nil, url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"rev1", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	// Force switch while frozen.
	err = SwitchTurn(nil, url, "rev1", true)
	if err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}
	got, err = GetCurrentTurn(nil, url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"author", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	// Add "rev1" back while still frozen.
	err = SwitchTurn(nil, url, "author", true)
	if err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}
	got, err = GetCurrentTurn(nil, url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"rev1", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	err = RemoveReviewerFromTurns(nil, url, "rev1", false)
	if err != nil {
		t.Fatalf("RemoveReviewerFromTurns() error = %v", err)
	}
	got, err = GetCurrentTurn(nil, url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	err = RemoveReviewerFromTurns(nil, url, "rev1", false) // Should be a no-op.
	if err != nil {
		t.Fatalf("RemoveReviewerFromTurns() error = %v", err)
	}
	got, err = GetCurrentTurn(nil, url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	ok, err = UnfreezeTurn(nil, url)
	if err != nil {
		t.Fatalf("UnfreezeTurn() error = %v", err)
	}
	if !ok {
		t.Fatalf("UnfreezeTurn() = %v, want %v", ok, true)
	}
	ok, err = UnfreezeTurn(nil, url)
	if err != nil {
		t.Fatalf("UnfreezeTurn() error = %v", err)
	}
	if ok {
		t.Fatalf("UnfreezeTurn() = %v, want %v", ok, false)
	}

	err = SwitchTurn(nil, url, "rev2", false)
	if err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}
	got, err = GetCurrentTurn(nil, url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"author"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}
}

func TestNudge(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)
	pathCache.Clear()

	url := "https://bitbucket.org/workspace/repo/pull-requests/1"

	// Initialize state.
	InitTurns(nil, url, "author")
	if err := AddReviewerToTurns(nil, url, "rev1"); err != nil {
		t.Fatalf("AddReviewerToTurns() error = %v", err)
	}
	if err := AddReviewerToTurns(nil, url, "rev2"); err != nil {
		t.Fatalf("AddReviewerToTurns() error = %v", err)
	}

	got, err := GetCurrentTurn(nil, url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want := []string{"rev1", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	// Nudge a non-reviewer.
	ok, approved, err := Nudge(nil, url, "non-reviewer")
	if err != nil {
		t.Fatalf("Nudge() error = %v", err)
	}
	if ok {
		t.Fatalf("Nudge() ok = %v, want %v", ok, false)
	}
	if approved {
		t.Fatalf("Nudge() approved = %v, want %v", approved, false)
	}

	// Rev1 reviews, author nudges rev2.
	if err := SwitchTurn(nil, url, "rev1", false); err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}

	ok, approved, err = Nudge(nil, url, "rev2")
	if err != nil {
		t.Fatalf("Nudge() error = %v", err)
	}
	if !ok {
		t.Fatalf("Nudge() = %v, want %v", ok, true)
	}
	if approved {
		t.Fatalf("Nudge() approved = %v, want %v", approved, false)
	}

	got, err = GetCurrentTurn(nil, url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"author", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	// Rev2 reviews -> it's the author's turn --> nudge the author.
	if err := SwitchTurn(nil, url, "rev2", false); err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}

	got, err = GetCurrentTurn(nil, url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"author"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	ok, approved, err = Nudge(nil, url, "author")
	if err != nil {
		t.Fatalf("Nudge() error = %v", err)
	}
	if !ok {
		t.Fatalf("Nudge() = %v, want %v", ok, true)
	}
	if approved {
		t.Fatalf("Nudge() approved = %v, want %v", approved, false)
	}

	got, err = GetCurrentTurn(nil, url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"author"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	// Author responds to comments --> it's rev1 and rev2's turn again.
	if err := SwitchTurn(nil, url, "author", false); err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}

	got, err = GetCurrentTurn(nil, url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"rev1", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	// Rev1 approves, and gets removed from the turn --> it's rev2's turn
	// (not the author, because it's currently the turn of "all the remaining reviewers").
	if err := RemoveReviewerFromTurns(nil, url, "rev1", true); err != nil {
		t.Fatalf("RemoveReviewerFromTurns() error = %v", err)
	}

	got, err = GetCurrentTurn(nil, url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	// Can't nudge rev1 anymore (still a reviewer in Bitbucket, but not tracked by RevChat in this PR).
	ok, approved, err = Nudge(nil, url, "rev1")
	if err != nil {
		t.Fatalf("Nudge() error = %v", err)
	}
	if ok {
		t.Fatalf("Nudge() = %v, want %v", ok, false)
	}
	if !approved {
		t.Fatalf("Nudge() approved = %v, want %v", approved, true)
	}

	// Rev2 nudged the author after some offline discussion.
	ok, approved, err = Nudge(nil, url, "author")
	if err != nil {
		t.Fatalf("NudgeReviewer() error = %v", err)
	}
	if !ok {
		t.Fatalf("NudgeReviewer() = %v, want %v", ok, true)
	}
	if approved {
		t.Fatalf("NudgeReviewer() approved = %v, want %v", approved, false)
	}

	got, err = GetCurrentTurn(nil, url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"author", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	// Author responds to comments --> it's rev2's turn again.
	if err := SwitchTurn(nil, url, "author", false); err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}

	got, err = GetCurrentTurn(nil, url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	// Rev2 approves too --> it's the author's turn again.
	if err := SwitchTurn(nil, url, "rev2", false); err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}

	got, err = GetCurrentTurn(nil, url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"author"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}
}

func TestNormalizeEmailAddresses(t *testing.T) {
	turn := &PRTurn{
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
