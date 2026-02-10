package slack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTitle(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		pr        map[string]any
		wantTitle string
		wantDraft bool
	}{
		{
			name: "bitbucket_pr",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/123",
			pr: map[string]any{
				"title": "Add new feature",
			},
			wantTitle: "\n\n<https://bitbucket.org/workspace/repo/pull-requests/123|*Add new feature*>",
		},
		{
			name: "bitbucket_draft",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/456",
			pr: map[string]any{
				"title": "Work in progress",
				"draft": true,
			},
			wantTitle: "\n\n:construction: <https://bitbucket.org/workspace/repo/pull-requests/456|*Work in progress*>",
			wantDraft: true,
		},
		{
			name: "github_pr",
			url:  "https://github.com/owner/repo/pulls/123",
			pr: map[string]any{
				"title": "Add new feature",
				"draft": false,
			},
			wantTitle: "\n\n<https://github.com/owner/repo/pulls/123|*Add new feature*>",
		},
		{
			name: "github_draft",
			url:  "https://github.com/owner/repo/pulls/456",
			pr: map[string]any{
				"title": "Work -> progress",
				"draft": true,
			},
			wantTitle: "\n\n:construction: <https://github.com/owner/repo/pulls/456|*Work -&gt; progress*>",
			wantDraft: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTitle, gotDraft := title(tt.url, tt.pr)
			if gotTitle != tt.wantTitle {
				t.Errorf("title() = %q, want %q", gotTitle, tt.wantTitle)
			}
			if gotDraft != tt.wantDraft {
				t.Errorf("title() draft = %v, want %v", gotDraft, tt.wantDraft)
			}
		})
	}
}

func TestTimes(t *testing.T) {
	tests := []struct {
		name        string
		now         time.Time
		url         string
		pr          map[string]any
		wantCreated string
		wantUpdated string
	}{
		{
			name: "bitbucket_created_only",
			now:  time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
			url:  "https://bitbucket.org/workspace/repo/pull-requests/123",
			pr: map[string]any{
				"created_on": "2024-05-30T10:00:00Z",
			},
			wantCreated: "2d 2h 0m",
		},
		{
			name: "bitbucket_created_and_updated",
			now:  time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
			url:  "https://bitbucket.org/workspace/repo/pull-requests/456",
			pr: map[string]any{
				"created_on": "2024-05-30T10:00:00Z",
				"updated_on": "2024-06-01T11:00:00Z",
			},
			wantCreated: "2d 2h 0m",
			wantUpdated: "1h 0m",
		},
		{
			name: "github_created_only",
			now:  time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
			url:  "https://github.com/owner/repo/pulls/123",
			pr: map[string]any{
				"created_at": "2024-05-30T10:00:00Z",
			},
			wantCreated: "2d 2h 0m",
		},
		{
			name: "github_created_and_updated",
			now:  time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
			url:  "https://github.com/owner/repo/pulls/456",
			pr: map[string]any{
				"created_at": "2024-05-30T10:00:00Z",
				"updated_at": "2024-06-01T11:00:00Z",
			},
			wantCreated: "2d 2h 0m",
			wantUpdated: "1h 0m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCreated, gotUpdated := times(tt.now, tt.url, tt.pr)
			if gotCreated != tt.wantCreated {
				t.Errorf("times() created = %q, want %q", gotCreated, tt.wantCreated)
			}
			if gotUpdated != tt.wantUpdated {
				t.Errorf("times() updated = %q, want %q", gotUpdated, tt.wantUpdated)
			}
		})
	}
}

func TestBranch(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		pr         map[string]any
		wantBranch string
	}{
		{
			name: "bitbucket_happy_path",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/1234",
			pr: map[string]any{
				"destination": map[string]any{
					"branch": map[string]any{
						"name": "main",
					},
				},
			},
			wantBranch: "\n> Target branch: `main`",
		},
		{
			name:       "bitbucket_missing_destination",
			url:        "https://bitbucket.org/workspace/repo/pull-requests/5678",
			pr:         map[string]any{},
			wantBranch: "\n> Target branch: `unknown`",
		},
		{
			name: "bitbucket_missing_branch",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/9012",
			pr: map[string]any{
				"destination": map[string]any{},
			},
			wantBranch: "\n> Target branch: `unknown`",
		},
		{
			name: "bitbucket_missing_name",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/3456",
			pr: map[string]any{
				"destination": map[string]any{
					"branch": map[string]any{},
				},
			},
			wantBranch: "\n> Target branch: `unknown`",
		},
		{
			name: "bitbucket_non_string_name",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/7890",
			pr: map[string]any{
				"destination": map[string]any{
					"branch": map[string]any{
						"name": 123,
					},
				},
			},
			wantBranch: "\n> Target branch: `unknown`",
		},
		{
			name: "github_happy_path",
			url:  "https://github.com/owner/repo/pulls/1234",
			pr: map[string]any{
				"base": map[string]any{
					"ref": "main",
				},
			},
			wantBranch: "\n> Base branch: `main`",
		},
		{
			name:       "github_missing_base",
			url:        "https://github.com/owner/repo/pulls/5678",
			pr:         map[string]any{},
			wantBranch: "\n> Base branch: `unknown`",
		},
		{
			name: "github_missing_ref",
			url:  "https://github.com/owner/repo/pulls/9012",
			pr: map[string]any{
				"base": map[string]any{},
			},
			wantBranch: "\n> Base branch: `unknown`",
		},
		{
			name: "github_non_string_ref",
			url:  "https://github.com/owner/repo/pulls/3456",
			pr: map[string]any{
				"base": map[string]any{
					"ref": 123,
				},
			},
			wantBranch: "\n> Base branch: `unknown`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBranch := branch(tt.url, tt.pr)
			if gotBranch != tt.wantBranch {
				t.Errorf("branch() = %q, want %q", gotBranch, tt.wantBranch)
			}
		})
	}
}

func TestStates(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	// Test data for Bitbucket happy path.
	path := filepath.Join(d, "revchat", "bitbucket.org", "workspace", "repo", "pull-requests", "67890_builds.json")
	err := os.MkdirAll(filepath.Dir(path), 0o700)
	if err != nil {
		t.Fatal(err)
	}
	builds := map[string]any{
		"builds": map[string]any{
			"build1": map[string]any{"state": "SUCCESSFUL"},
			"build2": map[string]any{"state": "FAILED"},
		},
	}
	data, err := json.Marshal(builds)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(path, data, 0o644)
	if err != nil {
		t.Fatal(err)
	}

	// Test cases.
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "bitbucket_no_builds",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/12345",
		},
		{
			name: "bitbucket_builds",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/67890",
			want: ", builds: :large_green_circle: :red_circle:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := states(nil, tt.url)
			if got != tt.want {
				t.Errorf("states() = %q, want %q", got, tt.want)
			}
		})
	}
}
