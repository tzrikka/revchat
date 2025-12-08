package bitbucket

import (
	"testing"
)

func TestSwitchSnapshot(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	// Initial state.
	snapshot1 := PullRequest{ID: 1}
	pr, err := switchSnapshot(nil, "url", snapshot1)
	if err != nil {
		t.Fatalf("switchSnapshot() error = %v", err)
	}
	if pr != nil {
		t.Fatalf("switchSnapshot() = %v, want %v", pr, nil)
	}

	// Replace initial snapshot.
	snapshot2 := PullRequest{ID: 2}
	pr, err = switchSnapshot(nil, "url", snapshot2)
	if err != nil {
		t.Fatalf("switchSnapshot() error = %v", err)
	}
	if pr == nil {
		t.Fatalf("switchSnapshot() = %v, want %v", pr, snapshot2)
	}
	if pr.ID != snapshot1.ID {
		t.Fatalf("switchSnapshot() = %v, want %v", pr.ID, snapshot1.ID)
	}
}

func TestHTMLURL(t *testing.T) {
	tests := []struct {
		name  string
		links map[string]Link
		want  string
	}{
		{
			name: "empty",
		},
		{
			name:  "happy_path",
			links: map[string]Link{"html": {HRef: "http://example.com"}},
			want:  "http://example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := htmlURL(tt.links); got != tt.want {
				t.Errorf("htmlURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
