package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tzrikka/revchat/pkg/config"
	"github.com/tzrikka/xdg"
)

func TestPRCommitHash(t *testing.T) {
	tests := []struct {
		name string
		pr   map[string]any
		want string
	}{
		// Bitbucket.
		{
			name: "happy_path_bitbucket",
			pr:   map[string]any{"source": map[string]any{"commit": map[string]any{"hash": "abc123"}}},
			want: "abc123",
		},
		{
			name: "missing_source_bitbucket",
			pr:   map[string]any{},
		},
		{
			name: "missing_commit_bitbucket",
			pr:   map[string]any{"source": map[string]any{}},
		},
		{
			name: "missing_hash_bitbucket",
			pr:   map[string]any{"source": map[string]any{"commit": map[string]any{}}},
		},
		// GitHub.
		{
			name: "happy_path_github",
			pr:   map[string]any{"head": map[string]any{"sha": "def456"}},
			want: "def456",
		},
		{
			name: "missing_head_github",
			pr:   map[string]any{},
		},
		{
			name: "missing_sha_github",
			pr:   map[string]any{"head": map[string]any{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := prCommitHash(tt.pr)
			if got != tt.want {
				t.Errorf("prCommitHash() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFindPRsByCommit(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	path, err := xdg.CreateDir(xdg.DataHome, config.DirName)
	if err != nil {
		t.Fatalf("xdg.CreateDir() error = %v", err)
	}

	pr := `{"source":{"commit":{"hash":"abc123"}}}`
	if err := os.WriteFile(filepath.Join(path, "pr1"+PRSnapshotFileSuffix), []byte(pr), xdg.NewFilePermissions); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	pr = `{"source":{"commit":{"hash":"def456"}}}`
	if err := os.WriteFile(filepath.Join(path, "pr2"+PRSnapshotFileSuffix), []byte(pr), xdg.NewFilePermissions); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, "pr3"+PRSnapshotFileSuffix), []byte(pr), xdg.NewFilePermissions); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	tests := []struct {
		name    string
		hash    string
		wantLen int
	}{
		{
			name: "not_found",
			hash: "nonexistent",
		},
		{
			name:    "found_one",
			hash:    "abc123",
			wantLen: 1,
		},
		{
			name:    "found_multiple",
			hash:    "def456",
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FindPRsByCommit(t.Context(), tt.hash)
			if err != nil {
				t.Fatalf("FindPRsByCommit() error = %v", err)
			}
			if len(got) != tt.wantLen {
				t.Errorf("FindPRsByCommit() len = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestFindPRsByCommit_pruning(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	path, err := xdg.CreateDir(xdg.DataHome, config.DirName)
	if err != nil {
		t.Fatalf("xdg.CreateDir() error = %v", err)
	}

	pr := `{
		"source": {"commit": {"hash": "abc123"}},
		"links": {"html": {"href": "url1"}, "self": {"href": "url2"}},
		"draft": true,
		"change_request_count": 1,
		"task_count": 2,
		"participants": [
			{"approved": true, "user": {"display_name": "Alice"}, "role": "REVIEWER", "state": "APPROVED"},
			{"approved": false, "user": {"display_name": "Bob"}, "role": "REVIEWER", "state": "UNAPPROVED"}
		],
		"aaa": "bbb",
		"ccc": 123,
		"ddd": true,
		"eee": null
	}`

	if err := os.WriteFile(filepath.Join(path, "pr1"+PRSnapshotFileSuffix), []byte(pr), xdg.NewFilePermissions); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	got, err := FindPRsByCommit(t.Context(), "abc123")
	if err != nil {
		t.Fatalf("FindPRsByCommit() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("FindPRsByCommit() len = %d, want %d", len(got), 1)
	}

	m := got[0]
	if len(m) != 5 {
		t.Errorf("FindPRsByCommit() map len = %d, want %d", len(m), 5)
	}

	links, ok := m["links"].(map[string]any)
	if !ok {
		t.Error("FindPRsByCommit() - missing or bad links map")
	}
	if len(links) != 1 || links["html"] == nil {
		t.Error("FindPRsByCommit() = links map not pruned correctly")
	}

	ps, ok := m["participants"].([]any)
	if !ok {
		t.Error("FindPRsByCommit() missing or bad participants block")
	}
	if len(ps) != 2 {
		t.Errorf("FindPRsByCommit() participants slice len = %d, want %d", len(ps), 2)
	}

	if _, found := m["draft"]; !found {
		t.Errorf("FindPRsByCommit() draft field not found")
	}
	if m["change_request_count"].(float64) != 1 {
		t.Errorf("FindPRsByCommit() change_request_count field = %v, want %v", m["change_request_count"], 1)
	}
	if m["task_count"].(float64) != 2 {
		t.Errorf("FindPRsByCommit() task_count field = %v, want %v", m["task_count"], 2)
	}
}
