package bitbucket

import (
	"testing"

	"github.com/urfave/cli/v3"
)

func TestConfigLinkifyIDs(t *testing.T) {
	cmdWithDefault := &cli.Command{Flags: []cli.Flag{
		&cli.StringSliceFlag{
			Name:  "linkification-map",
			Value: []string{"default=https://domain.atlassian.net/browse/"},
		},
	}}
	cmdWithoutDefault := &cli.Command{Flags: []cli.Flag{
		&cli.StringSliceFlag{
			Name:  "linkification-map",
			Value: []string{"FOO = https://domain.atlassian.net/browse/"},
		},
	}}

	tests := []struct {
		name string
		cmd  *cli.Command
		text string
		want string
	}{
		{
			name: "no IDs",
			cmd:  cmdWithDefault,
		},
		{
			name: "single_ID",
			cmd:  cmdWithDefault,
			text: "PROJ-1234",
			want: "> References in the PR:\n>  •  <https://domain.atlassian.net/browse/PROJ-1234|PROJ-1234>",
		},
		{
			name: "multiple_IDs",
			cmd:  cmdWithDefault,
			text: "[PROJ-1234] and PROJ-5678.",
			want: "> References in the PR:\n>  •  <https://domain.atlassian.net/browse/PROJ-1234|PROJ-1234>\n>  •  <https://domain.atlassian.net/browse/PROJ-5678|PROJ-5678>",
		},
		{
			name: "multiple_no_default",
			cmd:  cmdWithoutDefault,
			text: "FOO-1234 and [BAR-5678].",
			want: "> References in the PR:\n>  •  <https://domain.atlassian.net/browse/FOO-1234|FOO-1234>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Config{Cmd: tt.cmd}
			if got := c.linkifyIDs(nil, tt.text); got != tt.want {
				t.Errorf("linkifyIDs() = %q, want %q", got, tt.want)
			}
		})
	}
}
