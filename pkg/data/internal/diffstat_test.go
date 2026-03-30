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
