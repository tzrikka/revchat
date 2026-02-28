package slack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestPRIdentifiers(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		pr         map[string]any
		wantOwner  string
		wantRepo   string
		wantBranch string
		wantHash   string
	}{
		{
			name: "bitbucket_missing_destination",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/123",
			pr:   map[string]any{},
		},
		{
			name: "bitbucket_missing_workspace_and_repo",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/456",
			pr: map[string]any{
				"destination": map[string]any{},
			},
		},
		{
			name: "bitbucket_missing_branch_name",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/789",
			pr: map[string]any{
				"destination": map[string]any{
					"repository": map[string]any{
						"full_name": "workspace/repo",
					},
				},
			},
			wantOwner: "workspace",
			wantRepo:  "repo",
		},
		{
			name: "bitbucket_missing_commit",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/123",
			pr: map[string]any{
				"destination": map[string]any{
					"repository": map[string]any{
						"full_name": "workspace/repo",
					},
					"branch": map[string]any{
						"name": "dev",
					},
				},
			},
			wantOwner:  "workspace",
			wantRepo:   "repo",
			wantBranch: "dev",
		},
		{
			name: "bitbucket_missing_commit_hash",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/456",
			pr: map[string]any{
				"destination": map[string]any{
					"repository": map[string]any{
						"full_name": "workspace/repo",
					},
					"branch": map[string]any{
						"name": "dev",
					},
					"commit": map[string]any{},
				},
			},
			wantOwner:  "workspace",
			wantRepo:   "repo",
			wantBranch: "dev",
		},
		{
			name: "bitbucket_happy_path",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/789",
			pr: map[string]any{
				"destination": map[string]any{
					"repository": map[string]any{
						"full_name": "workspace/repo",
					},
					"branch": map[string]any{
						"name": "dev",
					},
					"commit": map[string]any{
						"hash": "abc123",
					},
				},
			},
			wantOwner:  "workspace",
			wantRepo:   "repo",
			wantBranch: "dev",
			wantHash:   "abc123",
		},
		{
			name: "github_missing_base",
			url:  "https://github.com/owner/repo/pulls/123",
			pr:   map[string]any{},
		},
		{
			name: "github_missing_owner_and_repo",
			url:  "https://github.com/owner/repo/pulls/456",
			pr: map[string]any{
				"base": map[string]any{},
			},
		},
		{
			name: "github_missing_branch_ref",
			url:  "https://github.com/owner/repo/pulls/789",
			pr: map[string]any{
				"base": map[string]any{
					"repo": map[string]any{
						"full_name": "owner/repo",
					},
				},
			},
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			name: "github_missing_commit_sha",
			url:  "https://github.com/owner/repo/pulls/123",
			pr: map[string]any{
				"base": map[string]any{
					"repo": map[string]any{
						"full_name": "owner/repo",
					},
					"ref": "main",
				},
			},
			wantOwner:  "owner",
			wantRepo:   "repo",
			wantBranch: "main",
		},
		{
			name: "github_happy_path",
			url:  "https://github.com/owner/repo/pulls/456",
			pr: map[string]any{
				"base": map[string]any{
					"repo": map[string]any{
						"full_name": "owner/repo",
					},
					"ref": "main",
					"sha": "def456",
				},
			},
			wantOwner:  "owner",
			wantRepo:   "repo",
			wantBranch: "main",
			wantHash:   "def456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOwner, gotRepo, gotBranch, gotHash := PRIdentifiers(nil, tt.url, tt.pr)
			if gotOwner != tt.wantOwner {
				t.Errorf("PRIdentifiers() owner = %q, want %q", gotOwner, tt.wantOwner)
			}
			if gotRepo != tt.wantRepo {
				t.Errorf("PRIdentifiers() repo = %q, want %q", gotRepo, tt.wantRepo)
			}
			if gotBranch != tt.wantBranch {
				t.Errorf("PRIdentifiers() branch = %q, want %q", gotBranch, tt.wantBranch)
			}
			if gotHash != tt.wantHash {
				t.Errorf("PRIdentifiers() hash = %q, want %q", gotHash, tt.wantHash)
			}
		})
	}
}

func TestBranchMap(t *testing.T) {
	tests := []struct {
		name   string
		url    string
		pr     map[string]any
		wantOK bool
	}{
		{
			name:   "bitbucket_missing_destination",
			url:    "https://bitbucket.org/workspace/repo/pull-requests/123",
			pr:     map[string]any{},
			wantOK: false,
		},
		{
			name: "bitbucket_invalid_destination",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/456",
			pr: map[string]any{
				"destination": "not a JSON map",
			},
			wantOK: false,
		},
		{
			name: "bitbucket_valid_destination",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/789",
			pr: map[string]any{
				"destination": map[string]any{
					"dummy_data": true,
				},
			},
			wantOK: true,
		},
		{
			name:   "github_missing_base",
			url:    "https://github.com/owner/repo/pulls/123",
			pr:     map[string]any{},
			wantOK: false,
		},
		{
			name: "github_invalid_base",
			url:  "https://github.com/owner/repo/pulls/456",
			pr: map[string]any{
				"base": "not a JSON map",
			},
			wantOK: false,
		},
		{
			name: "github_valid_base",
			url:  "https://github.com/owner/repo/pulls/789",
			pr: map[string]any{
				"base": map[string]any{
					"dummy_data": true,
				},
			},
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, ok := branchMap(nil, tt.url, tt.pr)
			if ok != tt.wantOK {
				t.Errorf("branchMap() ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok && m != nil {
				t.Errorf("branchMap() = %v, want nil", m)
			}
			if ok {
				if ok1, ok2 := m["dummy_data"].(bool); !ok1 || !ok2 {
					t.Errorf("branchMap() = %v, missing dummy data", m)
				}
			}
		})
	}
}

func TestBranchNameMarkdown(t *testing.T) {
	tests := []struct {
		name string
		url  string
		pr   map[string]any
		want string
	}{
		{
			name: "bitbucket_missing_destination",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/123",
			pr:   map[string]any{},
			want: "\n>Target branch: `unknown`",
		},
		{
			name: "bitbucket_missing_branch",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/456",
			pr: map[string]any{
				"destination": map[string]any{},
			},
			want: "\n>Target branch: `unknown`",
		},
		{
			name: "bitbucket_invalid_branch",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/789",
			pr: map[string]any{
				"destination": map[string]any{
					"branch": "not a JSON map",
				},
			},
			want: "\n>Target branch: `unknown`",
		},
		{
			name: "bitbucket_missing_branch_name",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/123",
			pr: map[string]any{
				"destination": map[string]any{
					"branch": map[string]any{},
				},
			},
			want: "\n>Target branch: `unknown`",
		},
		{
			name: "bitbucket_invalid_branch_name",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/456",
			pr: map[string]any{
				"destination": map[string]any{
					"branch": map[string]any{
						"name": 123,
					},
				},
			},
			want: "\n>Target branch: `unknown`",
		},
		{
			name: "bitbucket_happy_path",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/789",
			pr: map[string]any{
				"destination": map[string]any{
					"branch": map[string]any{
						"name": "dev",
					},
				},
			},
			want: "\n>Target branch: `dev`",
		},
		{
			name: "github_missing_branch_ref",
			url:  "https://github.com/owner/repo/pulls/123",
			pr: map[string]any{
				"base": map[string]any{},
			},
			want: "\n>Base branch: `unknown`",
		},
		{
			name: "github_invalid_branch_ref",
			url:  "https://github.com/owner/repo/pulls/456",
			pr: map[string]any{
				"base": map[string]any{
					"ref": 123,
				},
			},
			want: "\n>Base branch: `unknown`",
		},
		{
			name: "github_happy_path",
			url:  "https://github.com/owner/repo/pulls/789",
			pr: map[string]any{
				"base": map[string]any{
					"ref": "main",
				},
			},
			want: "\n>Base branch: `main`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBranch := branchNameMarkdown(nil, tt.url, tt.pr)
			if gotBranch != tt.want {
				t.Errorf("branchNameMarkdown() = %q, want %q", gotBranch, tt.want)
			}
		})
	}
}

func TestBranchOwnerAndRepo(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		branch    map[string]any
		wantOwner string
		wantRepo  string
		wantOK    bool
	}{
		{
			name: "bitbucket_missing_repo",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/123",
			branch: map[string]any{
				"repo": map[string]any{
					"full_name": "owner/repo",
				},
			},
		},
		{
			name: "github_missing_repo",
			url:  "https://github.com/owner/repo/pulls/123",
			branch: map[string]any{
				"repository": map[string]any{
					"full_name": "workspace/repo",
				},
			},
		},
		{
			name: "bitbucket_missing_full_name",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/456",
			branch: map[string]any{
				"repository": map[string]any{},
			},
		},
		{
			name: "github_missing_full_name",
			url:  "https://github.com/owner/repo/pulls/456",
			branch: map[string]any{
				"repository": map[string]any{},
			},
		},
		{
			name: "bitbucket_invalid_full_name",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/789",
			branch: map[string]any{
				"repository": map[string]any{
					"full_name": "boom!",
				},
			},
		},
		{
			name: "github_invalid_full_name",
			url:  "https://github.com/owner/repo/pulls/789",
			branch: map[string]any{
				"repo": map[string]any{
					"full_name": "boom!",
				},
			},
		},
		{
			name: "bitbucket_happy_path",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/123",
			branch: map[string]any{
				"repository": map[string]any{
					"full_name": "workspace/repo",
				},
			},
			wantOwner: "workspace",
			wantRepo:  "repo",
			wantOK:    true,
		},
		{
			name: "github_happy_path",
			url:  "https://github.com/owner/repo/pulls/123",
			branch: map[string]any{
				"repo": map[string]any{
					"full_name": "owner/repo",
				},
			},
			wantOwner: "owner",
			wantRepo:  "repo",
			wantOK:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOwner, gotRepo, gotOK := branchOwnerAndRepo(nil, tt.url, tt.branch)
			if gotOwner != tt.wantOwner {
				t.Errorf("branchOwnerAndRepo() owner = %q, want %q", gotOwner, tt.wantOwner)
			}
			if gotRepo != tt.wantRepo {
				t.Errorf("branchOwnerAndRepo() repo = %q, want %q", gotRepo, tt.wantRepo)
			}
			if gotOK != tt.wantOK {
				t.Errorf("branchOwnerAndRepo() ok = %v, want %v", gotOK, tt.wantOK)
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
			"build3": map[string]any{"state": "INPROGRESS"},
		},
	}
	data, err := json.Marshal(builds)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(path, data, 0o600)
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
			want: ", builds: :large_green_circle: :red_circle: :large_yellow_circle:",
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

func TestPRTasks(t *testing.T) {
	tests := []struct {
		name string
		url  string
		pr   map[string]any
		want []string
	}{
		{
			name: "not_bitbucket_pr",
			url:  "https://github.com/owner/repo/pulls/123",
			pr:   map[string]any{},
			want: nil,
		},
		{
			name: "missing_task_count",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/123",
			pr:   map[string]any{},
			want: nil,
		},
		{
			name: "zero_tasks",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/456",
			pr: map[string]any{
				"task_count": 0.0,
			},
			want: nil,
		},
		{
			name: "one_task",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/789",
			pr: map[string]any{
				"task_count": 1.0,
			},
			want: []string{""},
		},
		{
			name: "multiple_tasks",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/101112",
			pr: map[string]any{
				"task_count": 10.0,
			},
			want: []string{"", "", "", "", "", "", "", "", "", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := prTasks(nil, false, "", tt.url, tt.pr)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("prTasks() = %q, want %q", got, tt.want)
			}
		})
	}
}
