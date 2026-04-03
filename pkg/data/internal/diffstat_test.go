package internal

import (
	"reflect"
	"testing"
)

func TestExtractFilePaths(t *testing.T) {
	tests := []struct {
		name  string
		files []map[string]any
		want  []string
	}{
		{
			name: "nil",
		},
		{
			name:  "empty",
			files: []map[string]any{},
		},
		// Bitbucket.
		{
			name: "single_new_path_bitbucket",
			files: []map[string]any{
				{"new": map[string]any{"path": "path/new.txt"}},
			},
			want: []string{"path/new.txt"},
		},
		{
			name: "single_old_path_bitbucket",
			files: []map[string]any{
				{"old": map[string]any{"path": "path/old.txt"}},
			},
			want: []string{"path/old.txt"},
		},
		{
			name: "single_old_and_new_path_bitbucket",
			files: []map[string]any{
				{"new": map[string]any{"path": "path/file"}},
				{"old": map[string]any{"path": "path/file"}},
			},
			want: []string{"path/file"},
		},
		{
			name: "multiple_paths_bitbucket",
			files: []map[string]any{
				{"new": map[string]any{"path": "1"}},
				{"old": map[string]any{"path": "1"}},
				{"new": map[string]any{"path": "3"}},
				{"old": map[string]any{"path": "2"}},
			},
			want: []string{"1", "2", "3"},
		},
		// GitHub.
		{
			name: "single_path_github",
			files: []map[string]any{
				{"filename": "path/file.txt"},
			},
			want: []string{"path/file.txt"},
		},
		{
			name: "multiple_paths_github",
			files: []map[string]any{
				{"filename": "1"},
				{"filename": "3"},
				{"filename": "2"},
			},
			want: []string{"1", "2", "3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractFilePaths(tt.files)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("extractFilePaths() = %q, want %q", got, tt.want)
			}
		})
	}
}

// The unit tests below are the same as in data/diffstat_test.go.

func TestDiffstat(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	// Initial state.
	got, err := ReadDiffstatPaths(t.Context(), "url")
	if err != nil {
		t.Fatalf("ReadDiffstatPaths() error = %v", err)
	}
	if got != nil {
		t.Fatalf("ReadDiffstatPaths() = %#v, want %v", got, nil)
	}

	// New PR.
	tests := []struct {
		name  string
		files []map[string]any
		want  []string
	}{
		{
			name: "github_pr_created",
			files: []map[string]any{
				{"filename": "file1"},
				{"filename": "file2"},
			},
			want: []string{"file1", "file2"},
		},
		{
			name: "github_pr_updated",
			files: []map[string]any{
				{"old": map[string]any{"path": "file4"}},
				{"new": map[string]any{"path": "file3"}},
			},
			want: []string{"file3", "file4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := WriteDiffstat(t.Context(), "url", tt.files); err != nil {
				t.Fatalf("WriteDiffstat() error = %v", err)
			}

			got, err := ReadDiffstatPaths(t.Context(), "url")
			if err != nil {
				t.Fatalf("ReadDiffstatPaths() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ReadDiffstatPaths() = %#v, want %v", got, tt.want)
			}
		})
	}

	// PR closed.
	if err := DeleteGenericPRFile(t.Context(), "url"+DiffstatFileSuffix); err != nil {
		t.Fatalf("DeleteGenericPRFile() error = %v", err)
	}

	got, err = ReadDiffstatPaths(t.Context(), "url")
	if err != nil {
		t.Fatalf("ReadDiffstatPaths() error = %v", err)
	}
	if got != nil {
		t.Fatalf("ReadDiffstatPaths() = %#v, want %v", got, nil)
	}
}
