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

	// Pre-initialized state.
	got, err := GetCurrentTurn(url)
	if err == nil {
		t.Fatal("GetCurrentTurn() error = nil, want = true")
	}
	if got != nil {
		t.Fatalf("GetCurrentTurn() = %v, want %v", got, nil)
	}

	// Initialize state.
	err = InitTurn(url, "author", []string{})
	if err != nil {
		t.Fatalf("InitTurn() error = %v", err)
	}

	got, err = GetCurrentTurn(url)
	if err != nil {
		t.Fatalf("GetCurrentTurn() error = %v", err)
	}
	want := []string{"author"}
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
