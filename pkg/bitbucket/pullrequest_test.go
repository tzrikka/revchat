package bitbucket

import (
	"testing"
)

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

func TestInlineCommentPrefix(t *testing.T) {
	from, to := 1, 2

	tests := []struct {
		name string
		url  string
		i    *Inline
		want string
	}{
		{
			name: "no_from",
			url:  "http://example.com",
			i:    &Inline{Path: "test.txt"},
			want: "<http://example.com|File comment> in the file `test.txt`:\n",
		},
		{
			name: "from_only",
			url:  "http://example.com",
			i:    &Inline{From: &from, Path: "test.txt"},
			want: "<http://example.com|Line comment> in line 1 in the file `test.txt`:\n",
		},
		{
			name: "from_and_to",
			url:  "http://example.com",
			i:    &Inline{From: &from, To: &to, Path: "test.txt"},
			want: "<http://example.com|Line comment> in lines 1-2 in the file `test.txt`:\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inlineCommentPrefix(tt.url, tt.i); got != tt.want {
				t.Errorf("inlineCommentPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}
