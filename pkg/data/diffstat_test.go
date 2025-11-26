package data

import (
	"reflect"
	"testing"

	"github.com/tzrikka/timpani-api/pkg/bitbucket"
)

func TestDiffstatPaths(t *testing.T) {
	tests := []struct {
		name string
		ds   []bitbucket.DiffStat
		want []string
	}{
		{
			name: "nil",
		},
		{
			name: "empty",
			ds:   []bitbucket.DiffStat{},
		},
		{
			name: "single_new_path",
			ds: []bitbucket.DiffStat{
				{New: &bitbucket.CommitFile{Path: "path/new.txt"}},
			},
			want: []string{"path/new.txt"},
		},
		{
			name: "single_old_path",
			ds: []bitbucket.DiffStat{
				{Old: &bitbucket.CommitFile{Path: "path/old.txt"}},
			},
			want: []string{"path/old.txt"},
		},
		{
			name: "single_old_and_new_path",
			ds: []bitbucket.DiffStat{
				{New: &bitbucket.CommitFile{Path: "path/file"}},
				{Old: &bitbucket.CommitFile{Path: "path/file"}},
			},
			want: []string{"path/file"},
		},
		{
			name: "multiple_paths",
			ds: []bitbucket.DiffStat{
				{New: &bitbucket.CommitFile{Path: "1"}},
				{Old: &bitbucket.CommitFile{Path: "1"}},
				{New: &bitbucket.CommitFile{Path: "3"}},
				{Old: &bitbucket.CommitFile{Path: "2"}},
			},
			want: []string{"1", "2", "3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := diffstatPaths(tt.ds)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("diffstatPaths() = %q, want %q", got, tt.want)
			}
		})
	}
}
