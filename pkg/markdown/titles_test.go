package markdown

import (
	"testing"
)

func TestLinkifyTitle(t *testing.T) {
	tests := []struct {
		name  string
		cfg   map[string]string
		prURL string
		title string
		want  string
	}{
		{
			name: "empty_title",
			cfg:  map[string]string{"default": "https://domain.atlassian.net/browse/"},
		},
		{
			name:  "no_ids_1",
			cfg:   map[string]string{"default": "https://domain.atlassian.net/browse/"},
			title: "This is a PR title",
			want:  "This is a PR title",
		},
		{
			name:  "no_ids_2",
			cfg:   map[string]string{"default": "https://domain.atlassian.net/browse/"},
			title: "Your grade is A-",
			want:  "Your grade is A-",
		},
		{
			name:  "single_id",
			cfg:   map[string]string{"default": "https://domain.atlassian.net/browse/"},
			title: "PROJ-12345: blah",
			want:  "<https://domain.atlassian.net/browse/PROJ-12345|PROJ-12345>: blah",
		},
		{
			name:  "single_id_twice",
			cfg:   map[string]string{"default": "https://domain.atlassian.net/browse/"},
			title: "PROJ-12345 and PROJ-12345 too",
			want:  "<https://domain.atlassian.net/browse/PROJ-12345|PROJ-12345> and <https://domain.atlassian.net/browse/PROJ-12345|PROJ-12345> too",
		},
		{
			name:  "multiple_ids",
			cfg:   map[string]string{"default": "https://domain.atlassian.net/browse/"},
			title: "Concerning [PROJ-1234] and PROJ-5678",
			want:  "Concerning [<https://domain.atlassian.net/browse/PROJ-1234|PROJ-1234>] and <https://domain.atlassian.net/browse/PROJ-5678|PROJ-5678>",
		},
		{
			name:  "multiple_no_default",
			cfg:   map[string]string{"FOO": "https://qwe.atlassian.net/browse/", "BAR": "https://rty.atlassian.net/browse/"},
			title: "[FOO-12] and BAR-34 and BAZ-56",
			want:  "[<https://qwe.atlassian.net/browse/FOO-12|FOO-12>] and <https://rty.atlassian.net/browse/BAR-34|BAR-34> and BAZ-56",
		},
		{
			name:  "bitbucket_pr_in_this_repo",
			prURL: "https://bitbucket.org/workspace/repo/pull-requests/98765",
			title: "--> #123 <--",
			want:  "--> <https://bitbucket.org/workspace/repo/pull-requests/123|#123> <--",
		},
		{
			name:  "github_pr_in_this_repo",
			prURL: "https://github.com/workspace/repo/pull/98765",
			title: "--> #123 <--",
			want:  "--> <https://github.com/workspace/repo/pull/123|#123> <--",
		},
		{
			name:  "bitbucket_pr_in_other_repo",
			prURL: "https://bitbucket.org/workspace/repo/pull-requests/98765",
			title: "--> other#123 <--",
			want:  "--> <https://bitbucket.org/workspace/other/pull-requests/123|other#123> <--",
		},
		{
			name:  "github_pr_in_other_repo",
			prURL: "https://github.com/workspace/repo/pull/98765",
			title: "--> other#123 <--",
			want:  "--> <https://github.com/workspace/other/pull/123|other#123> <--",
		},
		{
			name:  "bitbucket_pr_in_other_proj_1",
			prURL: "https://bitbucket.org/workspace/repo/pull-requests/98765",
			title: "--> proj/other#123 <--",
			want:  "--> <https://bitbucket.org/proj/other/pull-requests/123|proj/other#123> <--",
		},
		{
			name:  "github_pr_in_other_org_1",
			prURL: "https://github.com/owner/repo/pull/98765",
			title: "--> org/other#123 <--",
			want:  "--> <https://github.com/org/other/pull/123|org/other#123> <--",
		},
		{
			name:  "bitbucket_pr_in_other_proj_twice",
			prURL: "https://bitbucket.org/workspace/repo/pull-requests/98765",
			title: "--> proj/other#123 X proj/other#123 <--",
			want:  "--> <https://bitbucket.org/proj/other/pull-requests/123|proj/other#123> X <https://bitbucket.org/proj/other/pull-requests/123|proj/other#123> <--",
		},
		{
			name:  "github_pr_in_other_proj_twice",
			prURL: "https://github.com/workspace/repo/pull/98765",
			title: "--> proj/other#123 X proj/other#123 <--",
			want:  "--> <https://github.com/proj/other/pull/123|proj/other#123> X <https://github.com/proj/other/pull/123|proj/other#123> <--",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LinkifyTitle(nil, tt.cfg, tt.prURL, tt.title); got != tt.want {
				t.Errorf("LinkifyTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}
