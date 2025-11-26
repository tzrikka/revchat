package data

import (
	"reflect"
	"testing"
)

func TestTurns(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)
	pathCache = map[string]string{} // Reset global state.

	url := "https://bitbucket.org/workspace/repo/pull-requests/1"

	// Pre-initialized state (missing file).
	got, err := GetCurrentTurn(url)
	if err != nil {
		t.Fatal("GetCurrentTurn() error = nil, want = true")
	}
	want := []string{""}
	if len(got) != 1 && got[0] != "" {
		t.Fatalf("GetCurrentTurn() = %q, want = %q", got, want)
	}

	// Initialize state without reviewers.
	if err := InitTurns(url, "author", []string{}); err != nil {
		t.Fatalf("InitTurns() error = %v", err)
	}

	got, err = GetCurrentTurn(url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"author"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	// Add reviewers.
	err = AddReviewerToPR(url, "rev1")
	if err != nil {
		t.Fatalf("AddReviewerToPR() error = %v", err)
	}
	got, err = GetCurrentTurn(url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"rev1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	err = AddReviewerToPR(url, "rev2")
	if err != nil {
		t.Fatalf("AddReviewerToPR() error = %v", err)
	}
	got, err = GetCurrentTurn(url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"rev1", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	err = AddReviewerToPR(url, "rev2") // should be a no-op.
	if err != nil {
		t.Fatalf("AddReviewerToPR() error = %v", err)
	}
	got, err = GetCurrentTurn(url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"rev1", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	err = AddReviewerToPR(url, "author") // should be a no-op.
	if err != nil {
		t.Fatalf("AddReviewerToPR() error = %v", err)
	}
	got, err = GetCurrentTurn(url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"rev1", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	// Update turn states.
	err = SwitchTurn(url, "rev1")
	if err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}
	got, err = GetCurrentTurn(url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"author", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	err = SwitchTurn(url, "rev2")
	if err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}
	got, err = GetCurrentTurn(url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"author"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	err = SwitchTurn(url, "author")
	if err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}
	got, err = GetCurrentTurn(url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"rev1", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	err = SetTurn(url, []string{"rev2", "rev3"})
	if err != nil {
		t.Fatalf("SetTurn() error = %v", err)
	}
	got, err = GetCurrentTurn(url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"rev2", "rev3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	err = RemoveFromTurn(url, "rev3")
	if err != nil {
		t.Fatalf("RemoveFromTurn() error = %v", err)
	}
	got, err = GetCurrentTurn(url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	err = RemoveFromTurn(url, "rev3") // Should be a no-op.
	if err != nil {
		t.Fatalf("RemoveFromTurn() error = %v", err)
	}
	got, err = GetCurrentTurn(url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	err = SwitchTurn(url, "rev2")
	if err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}
	got, err = GetCurrentTurn(url)
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
	pathCache = map[string]string{} // Reset global state.

	url := "https://bitbucket.org/workspace/repo/pull-requests/1"

	// Initialize state.
	if err := InitTurns(url, "author", []string{"rev1", "rev2"}); err != nil {
		t.Fatalf("InitTurns() error = %v", err)
	}

	got, err := GetCurrentTurn(url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want := []string{"rev1", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	// Nudge a non-reviewer.
	ok, err := Nudge(url, "non-reviewer")
	if err != nil {
		t.Fatalf("Nudge() error = %v", err)
	}
	if ok {
		t.Fatalf("Nudge() = %v, want %v", ok, false)
	}

	// Rev1 reviews, author nudges rev2.
	if err := SwitchTurn(url, "rev1"); err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}

	ok, err = Nudge(url, "rev2")
	if err != nil {
		t.Fatalf("Nudge() error = %v", err)
	}
	if !ok {
		t.Fatalf("Nudge() = %v, want %v", ok, true)
	}

	got, err = GetCurrentTurn(url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"author", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	// Rev2 reviews -> it's the author's turn --> nudge the author.
	if err := SwitchTurn(url, "rev2"); err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}

	got, err = GetCurrentTurn(url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"author"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	ok, err = Nudge(url, "author")
	if err != nil {
		t.Fatalf("Nudge() error = %v", err)
	}
	if !ok {
		t.Fatalf("Nudge() = %v, want %v", ok, true)
	}

	got, err = GetCurrentTurn(url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"author"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	// Author responds to comments --> it's rev1 and rev2's turn again.
	if err := SwitchTurn(url, "author"); err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}

	got, err = GetCurrentTurn(url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"rev1", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	// Rev1 approves, and gets removed from the turn --> it's rev2's turn
	// (not the author, because it's currently the turn of "all the remaining reviewers").
	if err := RemoveFromTurn(url, "rev1"); err != nil {
		t.Fatalf("RemoveFromTurn() error = %v", err)
	}

	got, err = GetCurrentTurn(url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	// Can't nudge rev1 anymore (still a reviewer in Bitbucket, but not tracked by RevChat in this PR).
	ok, err = Nudge(url, "rev1")
	if err != nil {
		t.Fatalf("Nudge() error = %v", err)
	}
	if ok {
		t.Fatalf("Nudge() = %v, want %v", ok, false)
	}

	// Rev2 nudged the author after some offline discussion.
	ok, err = Nudge(url, "author")
	if err != nil {
		t.Fatalf("NudgeReviewer() error = %v", err)
	}
	if !ok {
		t.Fatalf("NudgeReviewer() = %v, want %v", ok, true)
	}

	got, err = GetCurrentTurn(url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"author", "rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	// Author responds to comments --> it's rev2's turn again.
	if err := SwitchTurn(url, "author"); err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}

	got, err = GetCurrentTurn(url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"rev2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}

	// Rev2 approves too --> it's the author's turn again.
	if err := SwitchTurn(url, "rev2"); err != nil {
		t.Fatalf("SwitchTurn() error = %v", err)
	}

	got, err = GetCurrentTurn(url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want = []string{"author"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, want)
	}
}
