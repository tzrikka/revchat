package workflows

import (
	"slices"
	"testing"
)

func TestIntersect(t *testing.T) {
	tests := []struct {
		name   string
		slice1 []string
		slice2 []string
		want   []string
	}{
		{
			name: "both_nil",
		},
		{
			name:   "both_empty",
			slice1: []string{},
			slice2: []string{},
		},
		{
			name:   "no_intersection",
			slice1: []string{"A", "B", "C"},
			slice2: []string{"D", "E", "F"},
		},
		{
			name:   "some_intersection",
			slice1: []string{"A", "B", "C", "D"},
			slice2: []string{"C", "D", "E", "F"},
			want:   []string{"C", "D"},
		},
		{
			name:   "identical_slices",
			slice1: []string{"A", "B", "C"},
			slice2: []string{"A", "B", "C"},
			want:   []string{"A", "B", "C"},
		},
		{
			name:   "with_duplicates",
			slice1: []string{"A", "B", "B", "C"},
			slice2: []string{"B", "B", "C", "D"},
			want:   []string{"B", "C"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := intersect(tt.slice1, tt.slice2)
			if !slices.Equal(got, tt.want) {
				t.Errorf("intersect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractPullRequestURLs(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "no_match",
			text: "This is a test message without any PR URLs.",
		},
		{
			name: "single_match",
			text: "Check out this PR: https://example.com/org/repo/pull/123 for more details.",
			want: []string{"https://example.com/org/repo/pull/123"},
		},
		{
			name: "multiple_matches",
			text: "Refer to https://example.com/org/repo/pull/123 and https://example.com/org/repo/pull/456 for context.",
			want: []string{"https://example.com/org/repo/pull/123", "https://example.com/org/repo/pull/456"},
		},
		{
			name: "repeated_url",
			text: "https://foo.com/org/repo/pull/456 and https://foo.com/org/repo/pull/456 and https://bar.com/org/repo/pull/123",
			want: []string{"https://bar.com/org/repo/pull/123", "https://foo.com/org/repo/pull/456"},
		},
		{
			name: "with_comment_suffix",
			text: "See https://example.com/org/repo/pull/123#comment-789 for the discussion.",
			want: []string{"https://example.com/org/repo/pull/123"},
		},
		{
			name: "mixed_content",
			text: "Here are some links: https://example.com/org/repo/pull/123, https://notaprurl.com, and https://example.com/org/repo/pull/456#comment-101112.",
			want: []string{"https://example.com/org/repo/pull/123", "https://example.com/org/repo/pull/456"},
		},
		{
			name: "bitbucket_style",
			text: "Check this Bitbucket PR: https://bitbucket.org/workspace/repo/pull-requests/789",
			want: []string{"https://bitbucket.org/workspace/repo/pull-requests/789"},
		},
		{
			name: "bitbucket_overview",
			text: "Check this Bitbucket PR: https://bitbucket.org/workspace/repo/pull-requests/1/overview",
			want: []string{"https://bitbucket.org/workspace/repo/pull-requests/1"},
		},
		{
			name: "github_style",
			text: "Check this GitHub PR: https://github.com/org/repo/pull/123",
			want: []string{"https://github.com/org/repo/pull/123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPullRequestURLs(tt.text)
			if !slices.Equal(got, tt.want) {
				t.Errorf("extractPullRequestURLs() = %v, want %v", got, tt.want)
			}
		})
	}
}
