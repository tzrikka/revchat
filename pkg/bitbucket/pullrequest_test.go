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
	tests := []struct {
		name string
		url  string
		i    *Inline
		want string
	}{
		{
			name: "file_comment",
			url:  "http://example.com",
			i:    &Inline{Path: "test.txt"},
			want: "<http://example.com|File comment> in `test.txt`:\n",
		},
		{
			name: "single_from_line",
			url:  "http://example.com",
			i:    &Inline{From: intPtr(1), Path: "test.txt"},
			want: "<http://example.com|Inline comment> in line 1 in `test.txt`:\n",
		},
		{
			name: "single_to_line",
			url:  "http://example.com",
			i:    &Inline{To: intPtr(1), Path: "test.txt"},
			want: "<http://example.com|Inline comment> in line 1 in `test.txt`:\n",
		},
		{
			name: "multiple_lines",
			url:  "http://example.com",
			i:    &Inline{StartTo: intPtr(2), To: intPtr(3), Path: "test.txt"},
			want: "<http://example.com|Inline comment> in lines 2-3 in `test.txt`:\n",
		},
		{
			name: "multiple_to_and_from_lines",
			url:  "http://example.com",
			i:    &Inline{StartFrom: intPtr(2), StartTo: intPtr(3), From: intPtr(4), To: intPtr(5), Path: "test.txt"},
			want: "<http://example.com|Inline comment> in lines 2-5 in `test.txt`:\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inlineCommentPrefix(tt.url, tt.i); got != tt.want {
				t.Errorf("inlineCommentPrefix() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSpliceSuggestion(t *testing.T) {
	tests := []struct {
		name       string
		in         *Inline
		suggestion string
		srcFile    string
		want       string
	}{
		// Replace.
		{
			name:       "replace_first_line_in_original_file",
			in:         &Inline{From: intPtr(1)},
			suggestion: "New 1",
			srcFile:    "Line 1\nLine 2\nLine 3\nLine 4",
			want:       "@@ -1,1 +1,1 @@\n-Line 1\n+New 1\n",
		},
		{
			name:       "replace_first_line_in_changed_code",
			in:         &Inline{To: intPtr(1)},
			suggestion: "New 1",
			srcFile:    "Line 1\nLine 2\nLine 3\nLine 4",
			want:       "@@ -1,1 +1,1 @@\n-Line 1\n+New 1\n",
		},
		{
			name:       "replace_last_2_lines_in_original_file",
			in:         &Inline{StartFrom: intPtr(3), From: intPtr(4)},
			suggestion: "New 3\nNew 4",
			srcFile:    "Line 1\nLine 2\nLine 3\nLine 4",
			want:       "@@ -3,2 +3,2 @@\n-Line 3\n-Line 4\n+New 3\n+New 4\n",
		},
		{
			name:       "replace_last_2_lines_in_changed_code",
			in:         &Inline{StartTo: intPtr(3), To: intPtr(4)},
			suggestion: "New 3\nNew 4",
			srcFile:    "Line 1\nLine 2\nLine 3\nLine 4",
			want:       "@@ -3,2 +3,2 @@\n-Line 3\n-Line 4\n+New 3\n+New 4\n",
		},
		// Add.
		{
			name:       "add_1_line_in_original_file",
			in:         &Inline{From: intPtr(2)},
			suggestion: "New 2\nNew A",
			srcFile:    "Line 1\nLine 2\nLine 3\nLine 4",
			want:       "@@ -2,1 +2,2 @@\n-Line 2\n+New 2\n+New A\n",
		},
		{
			name:       "add_1_line_in_changed_code",
			in:         &Inline{To: intPtr(2)},
			suggestion: "New 2\nNew A",
			srcFile:    "Line 1\nLine 2\nLine 3\nLine 4",
			want:       "@@ -2,1 +2,2 @@\n-Line 2\n+New 2\n+New A\n",
		},
		{
			name:       "add_2_lines_in_original_file",
			in:         &Inline{StartFrom: intPtr(2), From: intPtr(3)},
			suggestion: "Line 2\nNew A\nLine 3\nNew B",
			srcFile:    "Line 1\nLine 2\nLine 3\nLine 4",
			want:       "@@ -2,2 +2,4 @@\n-Line 2\n-Line 3\n+Line 2\n+New A\n+Line 3\n+New B\n",
		},
		{
			name:       "add_2_lines_in_changed_code",
			in:         &Inline{StartTo: intPtr(2), To: intPtr(3)},
			suggestion: "Line 2\nLine 3\nNew A\nNew B",
			srcFile:    "Line 1\nLine 2\nLine 3\nLine 4",
			want:       "@@ -2,2 +2,4 @@\n-Line 2\n-Line 3\n+Line 2\n+Line 3\n+New A\n+New B\n",
		},
		// Delete.
		{
			name:    "delete_last_line_in_original_file",
			in:      &Inline{From: intPtr(4)},
			srcFile: "Line 1\nLine 2\nLine 3\nLine 4",
			want:    "@@ -4,1 @@\n-Line 4\n",
		},
		{
			name:    "delete_last_line_in_changed_code",
			in:      &Inline{To: intPtr(4)},
			srcFile: "Line 1\nLine 2\nLine 3\nLine 4",
			want:    "@@ -4,1 @@\n-Line 4\n",
		},
		{
			name:    "delete_last_2_lines_in_original_file",
			in:      &Inline{StartFrom: intPtr(3), From: intPtr(4)},
			srcFile: "Line 1\nLine 2\nLine 3\nLine 4",
			want:    "@@ -3,2 @@\n-Line 3\n-Line 4\n",
		},
		{
			name:    "delete_last_2_lines_in_changed_code",
			in:      &Inline{StartTo: intPtr(3), To: intPtr(4)},
			srcFile: "Line 1\nLine 2\nLine 3\nLine 4",
			want:    "@@ -3,2 @@\n-Line 3\n-Line 4\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := spliceSuggestion(nil, tt.in, tt.suggestion, tt.srcFile)
			if string(got) != tt.want {
				t.Errorf("spliceSuggestion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func intPtr(i int) *int {
	return &i
}
