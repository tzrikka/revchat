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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LinkifyTitle(nil, tt.cfg, tt.text); got != tt.want {
				t.Errorf("LinkifyTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}
