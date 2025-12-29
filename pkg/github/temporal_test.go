package github

import (
	"testing"
)

func TestTrimURLPrefix(t *testing.T) {
	tests := []struct {
		name   string
		rawURL string
		want   string
	}{
		{
			name: "empty",
		},
		{
			name:   "not_url",
			rawURL: "not a url",
			want:   "not a url",
		},
		{
			name:   "root_url",
			rawURL: "https://example.com/",
			want:   "",
		},
		{
			name:   "simple_path",
			rawURL: "https://example.com/path",
			want:   "path",
		},
		{
			name:   "simple_path_with_port",
			rawURL: "https://example.com:8080/path",
			want:   "path",
		},
		{
			name:   "longer_path",
			rawURL: "https://example.com/a/b/c/d/e",
			want:   "a/b/c/d/e",
		},
		{
			name:   "simple_query_only",
			rawURL: "https://example.com/?key=val",
			want:   "?key=val",
		},
		{
			name:   "path_with_query",
			rawURL: "https://example.com/foo/bar?aaa=111&bbb=222",
			want:   "foo/bar?aaa=111&bbb=222",
		},
		{
			name:   "fragment_only",
			rawURL: "https://example.com/#fragment",
			want:   "#fragment",
		},
		{
			name:   "path_with_query_and_fragment",
			rawURL: "https://example.com/foo/bar?aaa=111&bbb=222#fragment",
			want:   "foo/bar?aaa=111&bbb=222#fragment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := trimURLPrefix(tt.rawURL); got != tt.want {
				t.Errorf("trimURLPrefix() = %q, want %q", got, tt.want)
			}
		})
	}
}
