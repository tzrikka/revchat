package workflows

import (
	"testing"
)

func TestPRCommitHash(t *testing.T) {
	want := "abc123"
	prSnapshot := map[string]any{
		"source": map[string]any{
			"commit": map[string]any{
				"hash": want,
			},
		},
	}

	got, ok := prCommitHash(prSnapshot)
	if got != want {
		t.Errorf("prCommitHash() = %q, want %q", got, want)
	}
	if !ok {
		t.Errorf("prCommitHash() bool = %v, want %v", ok, true)
	}
}

func TestURLFromPR(t *testing.T) {
	want := "https://bitbucket.org/example/repo/pull-requests/1"
	prSnapshot := map[string]any{
		"links": map[string]any{
			"html": map[string]any{
				"href": want,
			},
		},
	}

	got := urlFromPR(prSnapshot)
	if got != want {
		t.Errorf("urlFromPR() = %q, want %q", got, want)
	}
}
