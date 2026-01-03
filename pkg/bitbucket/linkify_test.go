package bitbucket

import (
	"testing"
)

func TestLinkifyTitle(t *testing.T) {
	tests := []struct {
		name string
		text string
		cfg  map[string]string
		want string
	}{
		{
			name: "empty_title",
			cfg:  map[string]string{"default": "https://domain.atlassian.net/browse/"},
		},
		{
			name: "no_ids",
			cfg:  map[string]string{"default": "https://domain.atlassian.net/browse/"},
			text: "This is a PR title",
			want: "This is a PR title",
		},
		{
			name: "single_id",
			cfg:  map[string]string{"default": "https://domain.atlassian.net/browse/"},
			text: "PROJ-1234: blah",
			want: "<https://domain.atlassian.net/browse/PROJ-1234|PROJ-1234>: blah",
		},
		{
			name: "multiple_ids",
			cfg:  map[string]string{"default": "https://domain.atlassian.net/browse/"},
			text: "Blah [PROJ-1234] and PROJ-5678",
			want: "Blah [<https://domain.atlassian.net/browse/PROJ-1234|PROJ-1234>] and <https://domain.atlassian.net/browse/PROJ-5678|PROJ-5678>",
		},
		{
			name: "multiple_no_default",
			cfg:  map[string]string{"FOO": "https://domain.atlassian.net/browse/"},
			text: "FOO-1234 and [BAR-5678]",
			want: "<https://domain.atlassian.net/browse/FOO-1234|FOO-1234> and [BAR-5678]",
		},
		{
			name: "pr_in_this_repo",
			text: "Foo #123 bar",
			want: "Foo <https://bitbucket.org/workspace/repo/pull-requests/123|#123> bar",
		},
		{
			name: "pr_in_other_repo",
			text: "Foo other#123 bar",
			want: "Foo <https://bitbucket.org/workspace/other/pull-requests/123|other#123> bar",
		},
		{
			name: "pr_in_other_project_1",
			text: "Foo proj/other#123 bar",
			want: "Foo <https://bitbucket.org/proj/other/pull-requests/123|proj/other#123> bar",
		},
		{
			name: "pr_in_other_project_2",
			text: "proj/other#123 xxx proj/other#123",
			want: "<https://bitbucket.org/proj/other/pull-requests/123|proj/other#123> xxx <https://bitbucket.org/proj/other/pull-requests/123|proj/other#123>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prURL := "https://bitbucket.org/workspace/repo/pull-requests/98765"
			if got := LinkifyTitle(nil, tt.cfg, prURL, tt.text); got != tt.want {
				t.Errorf("LinkifyTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}
