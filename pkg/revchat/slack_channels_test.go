package revchat

import (
	"testing"

	"github.com/urfave/cli/v3"

	"github.com/tzrikka/revchat/pkg/config"
)

func TestSlackNormalizeChannelName(t *testing.T) {
	s := Slack{cmd: &cli.Command{
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:  "slack-max-channel-name-length",
				Value: config.DefaultMaxChannelNameLength,
			},
		},
	}}

	tests := []struct {
		name string
		s    string
		want string
	}{
		{
			name: "empty",
		},
		{
			name: "remove_annotations",
			s:    "[a][bb][CCC][DDDD-1234] [DDDD 1234] Foo[] Bar [Baz]",
			want: "foo-bar",
		},
		{
			name: "lower_case",
			s:    "ABC-DEF",
			want: "abc-def",
		},
		{
			name: "remove_apostrophes",
			s:    "can't-don`t",
			want: "cant-dont",
		},
		{
			name: "remove_special_chars",
			s:    "`a ~1!2@3#4$5%6^7&8*9(0)-_=+ []{}|\\ ;:'\" ,<.>/?",
			want: "a-1-2-3-4-5-6-7-8-9-0",
		},
		{
			name: "minimize_separators",
			s:    "a  2--3__4",
			want: "a-2-3-4",
		},
		{
			name: "prefixes_and_suffixes_1",
			s:    "-foo-",
			want: "foo",
		},
		{
			name: "prefixes_and_suffixes_2",
			s:    "__bar__",
			want: "bar",
		},
		{
			name: "max_length",
			s:    "a-very-long-channel-name-that-exceeds-the-maximum-length-of-50-characters",
			want: "a-very-long-channel-name-that-exceeds-the-maximum",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.normalizeChannelName(tt.s); got != tt.want {
				t.Errorf("normalizeChannelName() = %q, want %q", got, tt.want)
			}
		})
	}
}
