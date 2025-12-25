package slack

import (
	"testing"
)

func TestRequiredReviewers(t *testing.T) {
	owners := map[string][]string{
		"file1.go": {"alice", "bob"},
		"file2.go": {"bob", "carol"},
		"file3.go": {"dave"},
	}
	paths := []string{"file1.go", "file2.go", "file3.go"}

	want := []string{"alice", "bob", "carol", "dave"}
	got := RequiredReviewers(paths, owners)

	if len(got) != len(want) {
		t.Fatalf("RequiredReviewers() = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("RequiredReviewers()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
