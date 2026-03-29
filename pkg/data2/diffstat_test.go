package data2_test

import (
	"reflect"
	"testing"

	"github.com/tzrikka/revchat/pkg/data2"
)

func TestDiffstat(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	// Initial state.
	got := data2.LoadDiffstatPaths(nil, "url")
	if got != nil {
		t.Fatalf("LoadDiffstatPaths() = %#v, want %v", got, nil)
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
			data2.StoreDiffstat(nil, "url", tt.files)
			got := data2.LoadDiffstatPaths(nil, "url")
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("LoadDiffstatPaths() = %#v, want %v", got, tt.want)
			}
		})
	}

	// PR closed.
	data2.DeleteDiffstat(nil, "url")

	got = data2.LoadDiffstatPaths(nil, "url")
	if got != nil {
		t.Fatalf("LoadDiffstatPaths() = %#v, want %v", got, nil)
	}
}
