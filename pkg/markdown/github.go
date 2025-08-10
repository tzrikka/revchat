package markdown

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/users"
)

// GitHubToSlack converts a GitHub PR's body to Slack markdown text.
//
// Based on:
//   - https://docs.github.com/en/get-started/writing-on-github/getting-started-with-writing-and-formatting-on-github/basic-writing-and-formatting-syntax
//   - https://docs.slack.dev/messaging/formatting-message-text/
func GitHubToSlack(ctx workflow.Context, cmd *cli.Command, text, prURL string) string {
	// Header lines --> bold lines.
	text = regexp.MustCompile(`(?m)^#+\s+(.+)`).ReplaceAllString(text, "**${1}**")

	// Bold and italic text together: "*** ... ***" --> "_* ... *_".
	text = regexp.MustCompile(`\*\*\*(.+?)\*\*\*`).ReplaceAllString(text, "_**${1}**_")

	// Italic text: "*" --> "_" ("_" -> "_" is a no-op).
	text = regexp.MustCompile(`(^|[^*])\*([^*]+?)\*`).ReplaceAllString(text, "${1}_${2}_")

	// Bold text: "**" or "__" --> "*".
	text = regexp.MustCompile(`\*\*(.+?)\*\*`).ReplaceAllString(text, "*${1}*")
	text = regexp.MustCompile(`__(.+?)__`).ReplaceAllString(text, "*${1}*")

	// Strikethrough text: "~~" --> "~" ("~" -> "~" is a no-op).
	text = regexp.MustCompile(`~~(.+?)~~`).ReplaceAllString(text, "~${1}~")

	// Links: "[text](url)" --> "<url|text>".
	// Images: "![text](url)" --> "!<url|text>" --> "Image: <url|text>".
	text = regexp.MustCompile(`\[(.*?)\]\((.*?)\)`).ReplaceAllString(text, "<${2}|${1}>")
	text = regexp.MustCompile(`!<(.*?)>`).ReplaceAllString(text, "Image: <${1}>")

	// Lists (up to 2 levels): "-" or "*" or "+" --> "•" and "◦".
	for _, bullet := range []string{"-", `\+`} {
		pattern := fmt.Sprintf(`(?m)^%s\s*`, bullet)
		text = regexp.MustCompile(pattern).ReplaceAllString(text, "  •  ")

		pattern = fmt.Sprintf(`(?m)^\s{2,4}%s\s*`, bullet)
		text = regexp.MustCompile(pattern).ReplaceAllString(text, "          ◦   ")
	}

	// Mentions: "@user" --> "<@U123>" or "<https://github.com/user|@user>",
	// "@org/team" --> "<https://github.com/org/teams/team|@org/team>".
	for _, ghRef := range regexp.MustCompile(`@[\w/-]+`).FindAllString(text, -1) {
		u, err := url.Parse(prURL)
		if err != nil {
			break
		}
		username := strings.Replace(ghRef[1:], "/", "/teams/", 1)
		profile := fmt.Sprintf("%s://%s/%s", u.Scheme, u.Host, username)
		slackRef := users.GitHubToSlackRef(ctx, cmd, username, profile)

		text = regexp.MustCompile(ghRef).ReplaceAllString(text, slackRef)
	}

	// PR references: "#123" --> "<PR URL|#123>" (works for issues too).
	prefix := "<" + regexp.MustCompile(`/pull/\d+$`).ReplaceAllString(prURL, "/pull")
	text = regexp.MustCompile(`#(\d+)`).ReplaceAllString(text, prefix+"/${1}|#${1}>")

	// # Hide HTML comments.
	text = regexp.MustCompile(`(?s)<!--.+?-->`).ReplaceAllString(text, "")

	return text
}
