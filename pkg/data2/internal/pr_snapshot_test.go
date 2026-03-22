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

func TestURLFromPR(t *testing.T) {
	tests := []struct {
		name string
		pr   map[string]any
		want string
	}{
		// Bitbucket.
		{
			name: "happy_path",
			pr: map[string]any{"links": map[string]any{"html": map[string]any{
				"href": "https://bitbucket.org/example/repo/pull-requests/1",
			}}},
			want: "https://bitbucket.org/example/repo/pull-requests/1",
		},
		{
			name: "missing_links",
			pr:   map[string]any{},
		},
		{
			name: "missing_html",
			pr:   map[string]any{"links": map[string]any{}},
		},
		{
			name: "missing_href",
			pr:   map[string]any{"links": map[string]any{"html": map[string]any{}}},
		},
		// GitHub.
		{
			name: "happy_path_github",
			pr:   map[string]any{"html_url": "https://github.com/example/repo/pull/1"},
			want: "https://github.com/example/repo/pull/1",
		},
		{
			name: "missing_html_url",
			pr:   map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := urlFromPR(tt.pr); got != tt.want {
				t.Errorf("urlFromPR() = %q, want %q", got, tt.want)
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
