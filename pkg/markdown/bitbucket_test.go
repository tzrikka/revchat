package markdown

import (
	"testing"
)

func TestBitbucketToSlack(t *testing.T) {
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
			want: "*H1*",
		},
		{
			name: "h2",
			text: "## H2",
			want: "*H2*",
		},
		{
			name: "h3",
			text: "### H3",
			want: "*H3*",
		},
		{
			name: "multiple_headers",
			text: "# Title 1\n\nFoo\n\n## Subtitle 2\nBar",
			want: "*Title 1*\n\nFoo\n\n*Subtitle 2*\nBar",
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
			name: "italic_inside_bold_1",
			text: "**this _is_ nested**",
			want: "*this _is_ nested*",
		},
		{
			name: "italic_inside_bold_2",
			text: "**this *is* nested**",
			want: "*this _is_ nested*",
		},
		{
			name: "italic_inside_bold_3",
			text: "__this _is_ nested__",
			want: "*this _is_ nested*",
		},
		{
			name: "bold_inside_italic_1",
			text: "_this **is** nested_",
			want: "_this *is* nested_",
		},
		// {
		// 	name: "bold_inside_italic_2",
		// 	text: "*this **is** nested*",
		// 	want: "_this *is* nested_",
		// },

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
		// Simple lists.
		{
			name: "simple_list_1",
			text: "- 111\n- 222\n- 333",
			want: "  •  111\n  •  222\n  •  333",
		},
		{
			name: "simple_list_2",
			text: "+ 111\n+ 222\n+ 333",
			want: "  •  111\n  •  222\n  •  333",
		},
		// {
		// 	name: "simple_list_3",
		// 	text: "* 111\n* 222\n* 333",
		// 	want: "  •  111\n  •  222\n  •  333",
		// },

		// Embedded lists.
		{
			name: "embedded_list_1",
			text: "- 111\n  - 222\n  - 333\n- 444",
			want: "  •  111\n          ◦   222\n          ◦   333\n  •  444",
		},
		{
			name: "embedded_list_2",
			text: "+ 111\n  + 222\n  + 333\n+ 444",
			want: "  •  111\n          ◦   222\n          ◦   333\n  •  444",
		},
		// {
		// 	name: "embedded_list_3",
		// 	text: "* 111\n  * 222\n  * 333\n* 444",
		// 	want: "  •  111\n          ◦   222\n          ◦   333\n  •  444",
		// },
		{
			name: "embedded_list_4",
			text: "- 111\n\n    - 222\n    \n    - 333\n    \n- 444\n- 555",
			want: "  •  111\n          ◦   222\n          ◦   333\n  •  444\n  •  555",
		},

		// User mentions.
		// {
		// 	name: "mention_slack_user_found",
		// 	text: "@{123456:12345678-90ab-cdef-0123-456789abcdef}",
		// 	want: "<@123>",
		// },
		// {
		// 	name: "mention_slack_user_not_found",
		// 	text: "@{123456:12345678-90ab-cdef-0123-456789abcdef}",
		// 	want: "Display Name",
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BitbucketToSlack(nil, nil, tt.text); got != tt.want {
				t.Errorf("BitbucketToSlack() = %q, want %q", got, tt.want)
			}
		})
	}
}
