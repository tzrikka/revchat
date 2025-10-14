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
			text: "[label](url)",
			want: "<url|label>",
		},
		{
			name: "image",
			text: "!<url maybe with text>",
			want: ":camera: <url maybe with text>",
		},
		{
			name: "reverse_link",
			text: "<relative_path|label>",
			want: `&lt;relative_path|label>`,
		},
		{
			name: "pr_ref_in_same_repo",
			text: "#9",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/123",
			want: "<https://bitbucket.org/workspace/repo/pull-requests/9|#9>",
		},
		{
			name: "pr_ref_in_other_repo",
			text: "bar#12",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/123",
			want: "<https://bitbucket.org/workspace/bar/pull-requests/12|bar#12>",
		},
		{
			name: "pr_ref_in_other_workspace",
			text: "foo-bar/blah-2-blah#3456",
			url:  "https://bitbucket.org/workspace/repo/pull-requests/123",
			want: "<https://bitbucket.org/foo-bar/blah-2-blah/pull-requests/3456|foo-bar/blah-2-blah#3456>",
		},
		// Simple lists.
		{
			name: "simple_list_1",
			text: "- 111\n- 222\n- 333",
			want: "  •  111\n  •  222\n  •  333",
		},
		{
			name: "simple_list_2",
			text: "* 111\n* 222\n* 333",
			want: "  •  111\n  •  222\n  •  333",
		},
		{
			name: "simple_list_3",
			text: "+ 111\n+ 222\n+ 333",
			want: "  •  111\n  •  222\n  •  333",
		},
		// Simple lists with confusing characters.
		{
			name: "simple_list_4",
			text: "- 111 - 222\n- 333",
			want: "  •  111 - 222\n  •  333",
		},
		{
			name: "simple_list_5",
			text: "* 111 * 222\n* 333",
			want: "  •  111 * 222\n  •  333",
		},
		{
			name: "simple_list_6",
			text: "+ 111 + 222\n+ 333",
			want: "  •  111 + 222\n  •  333",
		},
		// Embedded lists.
		{
			name: "embedded_list_1",
			text: "- 111\n  - 222\n  - 333\n- 444",
			want: "  •  111\n          ◦   222\n          ◦   333\n  •  444",
		},
		{
			name: "embedded_list_2",
			text: "* 111\n  * 222\n  * 333\n* 444",
			want: "  •  111\n          ◦   222\n          ◦   333\n  •  444",
		},
		{
			name: "embedded_list_3",
			text: "* 111\n  * 222\n  * 333\n* 444",
			want: "  •  111\n          ◦   222\n          ◦   333\n  •  444",
		},
		{
			name: "embedded_list_4",
			text: "XXX\n\n* 111\n\n    * 222\n    * 333\n    \n* 444\n\nYYY",
			want: "XXX\n\n  •  111\n          ◦   222\n          ◦   333\n  •  444\n\nYYY",
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
			if got := BitbucketToSlack(nil, nil, tt.text, tt.url); got != tt.want {
				t.Errorf("BitbucketToSlack() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSlackToBitbucket(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "empty",
		},
		// Basic text styles.
		{
			name: "italic",
			text: "_italic_",
			want: "_italic_",
		},
		{
			name: "bold",
			text: "*bold*",
			want: "**bold**",
		},
		{
			name: "strikethrough",
			text: "~strike~",
			want: "~~strike~~",
		},
		// Nested text styles.
		{
			name: "italic_inside_bold",
			text: "*this _is_ nested*",
			want: "**this _is_ nested**",
		},
		{
			name: "bold_inside_italic",
			text: "_this *is* nested_",
			want: "_this **is** nested_",
		},
		// Quote blocks.
		{
			name: "block_quotes_1",
			text: "111\n> 222\n> 333\n444",
			want: "111  \n> 222  \n> 333  \n\n444",
		},
		{
			name: "block_quotes_2",
			text: "111\n&gt; 222\n&gt; 333\n444",
			want: "111  \n> 222  \n> 333  \n\n444",
		},
		{
			name: "block_quotes_3",
			text: "111\n\n&gt; 222\n&gt; 333\n\n444",
			want: "111  \n\n> 222  \n> 333  \n\n444",
		},
		// Code blocks.
		{
			name: "inline_code",
			text: "`inline`",
			want: "`inline`",
		},
		{
			name: "code_block",
			text: "```multi\nline```",
			want: "```\nmulti  \nline\n```",
		},
		// Links.
		{
			name: "link_1",
			text: "<url|label>",
			want: "[label](url)",
		},
		{
			name: "link_2",
			text: "<url>",
			want: "[url](url)",
		},
		{
			name: "reverse_link",
			text: "[label](url)",
			want: `\[label](url)`,
		},
		// Simple lists.
		{
			name: "simple_list_1",
			text: "• 111\n• 222\n• 333",
			want: "-  111  \n-  222  \n-  333",
		},
		{
			name: "simple_list_2",
			text: "aaa\n• 111\n• 222\n• 333\nbbb",
			want: "aaa  \n\n-  111  \n-  222  \n-  333  \n\nbbb",
		},
		// Simple lists with confusing characters.
		{
			name: "simple_list_3",
			text: "• 111 - 222\n• 333",
			want: "-  111 - 222  \n-  333",
		},
		{
			name: "simple_list_4",
			text: "• 111 + 222\n• 333",
			want: "-  111 + 222  \n-  333",
		},
		{
			name: "simple_list_5",
			text: "• 111 * 222\n• 333",
			want: "-  111 * 222  \n-  333",
		},
		// Embedded lists.
		{
			name: "embedded_list_1",
			text: "• 111\n    ◦ 222\n    ◦ 333\n• 444",
			want: "-  111  \n    -  222  \n    -  333  \n-  444",
		},
		{
			name: "embedded_list_2",
			text: "aaa\nbbb\n• 111\n    ◦ 222\n    ◦ 333\n• 444\nccc\nddd",
			want: "aaa  \nbbb  \n\n-  111  \n    -  222  \n    -  333  \n-  444  \n\nccc  \nddd",
		},
		// User mentions.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SlackToBitbucket(nil, "workspace", tt.text); got != tt.want {
				t.Errorf("SlackToBitbucket() = %q, want %q", got, tt.want)
			}
		})
	}
}
