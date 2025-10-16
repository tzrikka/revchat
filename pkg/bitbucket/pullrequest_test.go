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

func TestReviewersDiffEmpty(t *testing.T) {
	prev := PullRequest{}
	curr := PullRequest{}

	added, removed := reviewersDiff(prev, curr)

	if len(added) != 0 {
		t.Errorf("reviewersDiff() added = %v, want %v", added, []string{})
	}
	if len(removed) != 0 {
		t.Errorf("reviewersDiff() removed = %v, want %v", removed, []string{})
	}
}

func TestReviewersDiffAdded1(t *testing.T) {
	prev := PullRequest{}
	curr := PullRequest{
		Reviewers: []Account{
			{AccountID: "AAA"},
		},
	}

	added, removed := reviewersDiff(prev, curr)
	if len(added) != 1 || added[0] != "AAA" {
		t.Errorf("reviewersDiff() added = %v, want %v", added, []string{"AAA"})
	}
	if len(removed) != 0 {
		t.Errorf("reviewersDiff() removed = %v, want %v", removed, []string{})
	}
}

func TestReviewersDiffAdded3(t *testing.T) {
	prev := PullRequest{}
	curr := PullRequest{
		Reviewers: []Account{
			{AccountID: "BBB"},
			{AccountID: "AAA"},
			{AccountID: "CCC"},
		},
	}

	added, removed := reviewersDiff(prev, curr)
	if len(added) != 3 || added[0] != "AAA" || added[1] != "BBB" || added[2] != "CCC" {
		t.Errorf("reviewersDiff() added = %v, want %v", added, []string{"AAA", "BBB", "CCC"})
	}
	if len(removed) != 0 {
		t.Errorf("reviewersDiff() removed = %v, want %v", removed, []string{})
	}
}

func TestReviewersDiffRemoved1(t *testing.T) {
	prev := PullRequest{
		Reviewers: []Account{
			{AccountID: "AAA"},
		},
	}
	curr := PullRequest{}

	added, removed := reviewersDiff(prev, curr)
	if len(added) != 0 {
		t.Errorf("reviewersDiff() added = %v, want %v", added, []string{})
	}
	if len(removed) != 1 || removed[0] != "AAA" {
		t.Errorf("reviewersDiff() removed = %v, want %v", removed, []string{"AAA"})
	}
}

func TestReviewersDiffRemoved3(t *testing.T) {
	prev := PullRequest{
		Reviewers: []Account{
			{AccountID: "BBB"},
			{AccountID: "AAA"},
			{AccountID: "CCC"},
		},
	}
	curr := PullRequest{}

	added, removed := reviewersDiff(prev, curr)
	if len(added) != 0 {
		t.Errorf("reviewersDiff() added = %v, want %v", added, []string{})
	}
	if len(removed) != 3 || removed[0] != "AAA" || removed[1] != "BBB" || removed[2] != "CCC" {
		t.Errorf("reviewersDiff() removed = %v, want %v", removed, []string{"AAA", "BBB", "CCC"})
	}
}

func TestReviewersDiffMixed(t *testing.T) {
	prev := PullRequest{
		Reviewers: []Account{
			{AccountID: "AAA"},
			{AccountID: "BBB"},
		},
	}
	curr := PullRequest{
		Reviewers: []Account{
			{AccountID: "CCC"},
			{AccountID: "DDD"},
		},
	}

	added, removed := reviewersDiff(prev, curr)

	if len(added) != 2 || added[0] != "CCC" || added[1] != "DDD" {
		t.Errorf("reviewersDiff() added = %v, want %v", added, []string{"Charlie"})
	}
	if len(removed) != 2 || removed[0] != "AAA" || removed[1] != "BBB" {
		t.Errorf("reviewersDiff() removed = %v, want %v", removed, []string{"Alice"})
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
