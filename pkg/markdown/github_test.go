package markdown

import (
	"testing"
)

func TestGitHubToSlack(t *testing.T) {
	tests := []struct {
		name string
		text string
		url  string
		want string
	}{
		{
			name: "empty",
		},
		// Headers.
		{
			name: "h1",
			text: "# H1",
			want: "*# H1*",
		},
		{
			name: "h2",
			text: "## H2",
			want: "*## H2*",
		},
		{
			name: "h3",
			text: "### H3",
			want: "*### H3*",
		},
		{
			name: "multiple_headers",
			text: "# Title 1\n\nFoo\n\n## Subtitle 2\nBar",
			want: "*# Title 1*\n\nFoo\n\n*## Subtitle 2*\nBar",
		},
		// Basic text styles.
		{
			name: "italic_1",
			text: "_italic_",
			want: "_italic_",
		},
		{
			name: "italic_2",
			text: "*italic*",
			want: "_italic_",
		},
		{
			name: "bold_1",
			text: "__bold__",
			want: "*bold*",
		},
		{
			name: "bold_2",
			text: "**bold**",
			want: "*bold*",
		},
		{
			name: "strikethrough",
			text: "~~strike~~",
			want: "~strike~",
		},
		// Nested text styles.
		{
			name: "italic_and_bold",
			text: "***both***",
			want: "_*both*_",
		},
		{
			name: "italic_inside_bold_1",
			text: "**this *is* nested**",
			want: "*this _is_ nested*",
		},
		{
			name: "italic_inside_bold_2",
			text: "**this _is_ nested**",
			want: "*this _is_ nested*",
		},
		{
			name: "italic_inside_bold_3",
			text: "__this *is* nested__",
			want: "*this _is_ nested*",
		},
		{
			name: "italic_inside_bold_4",
			text: "__this _is_ nested__",
			want: "*this _is_ nested*",
		},
		{
			name: "bold_inside_italic_1",
			text: "*this **is** nested*",
			want: "_this *is* nested_",
		},
		{
			name: "bold_inside_italic_2",
			text: "*this __is__ nested*",
			want: "_this *is* nested_",
		},
		{
			name: "bold_inside_italic_3",
			text: "_this **is** nested_",
			want: "_this *is* nested_",
		},
		{
			name: "bold_inside_italic_4",
			text: "_this __is__ nested_",
			want: "_this *is* nested_",
		},
		// Quote blocks.
		{
			name: "block_quotes",
			text: "111\n>222\n> 333\n444",
			want: "111\n>222\n> 333\n444",
		},
		// Code blocks.
		{
			name: "inline_code",
			text: "`inline`",
			want: "`inline`",
		},
		{
			name: "code_block",
			text: "```\nmulti\nline\n```",
			want: "```\nmulti\nline\n```",
		},
		// Links.
		{
			name: "link",
			text: "[text](url)",
			want: "<url|text>",
		},
		{
			name: "image",
			text: "!<url maybe with text>",
			want: "Image: <url maybe with text>",
		},
		{
			name: "pr_ref",
			text: "#123",
			url:  "https://github.com/org/repo/pull/987",
			want: "<https://github.com/org/repo/pull/123|#123>",
		},
		// Simple lists.
		{
			name: "simple_list_1",
			text: "- 111\n- 222\n- 333",
			want: "  •   111\n  •   222\n  •   333",
		},
		{
			name: "simple_list_2",
			text: "* 111\n* 222\n* 333",
			want: "  •   111\n  •   222\n  •   333",
		},
		{
			name: "simple_list_3",
			text: "+ 111\n+ 222\n+ 333",
			want: "  •   111\n  •   222\n  •   333",
		},
		// Simple lists with confusing characters.
		{
			name: "simple_list_4",
			text: "- 111 - 222\n- 333",
			want: "  •   111 - 222\n  •   333",
		},
		{
			name: "simple_list_5",
			text: "* 111 * 222\n* 333",
			want: "  •   111 * 222\n  •   333",
		},
		{
			name: "simple_list_6",
			text: "+ 111 + 222\n+ 333",
			want: "  •   111 + 222\n  •   333",
		},
		// Embedded lists.
		{
			name: "embedded_list_1",
			text: "- 111\n  - 222\n  - 333\n- 444",
			want: "  •   111\n          ◦   222\n          ◦   333\n  •   444",
		},
		{
			name: "embedded_list_2",
			text: "+ 111\n  + 222\n  + 333\n+ 444",
			want: "  •   111\n          ◦   222\n          ◦   333\n  •   444",
		},
		{
			name: "embedded_list_3",
			text: "* 111\n  * 222\n  * 333\n* 444",
			want: "  •   111\n          ◦   222\n          ◦   333\n  •   444",
		},

		// User mentions.
		// {
		// 	name: "mention_slack_user_found",
		// 	text: "@user",
		// 	url:  "https://github.com/org/repo/pull/123",
		// 	want: "<@123>",
		// },
		// {
		// 	name: "mention_slack_user_not_found",
		// 	text: "@user",
		// 	url:  "https://github.com/org/repo/pull/123",
		// 	want: "<https://github.com/user|@user>",
		// },
		// {
		// 	name: "mention_slack_team",
		// 	text: "@org/team",
		// 	url:  "https://github.com/org/repo/pull/123",
		// 	want: "<https://github.com/org/teams/team|@org/team>",
		// },

		// HTML comments and <sub> tags.
		{
			name: "html_comment",
			text: "Foo\n<!-- hidden -->\nBar",
			want: "Foo\n\nBar",
		},
		{
			name: "html_sub",
			text: "Blah blah\n\n\n<sub>hidden\nstuff</sub>\n\nBlah",
			want: "Blah blah\n\nBlah",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GitHubToSlack(nil, tt.text, tt.url); got != tt.want {
				t.Errorf("GitHubToSlack() = %q, want %q", got, tt.want)
			}
		})
	}
}
