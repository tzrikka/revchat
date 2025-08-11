package markdown

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/users"
)

// BitbucketToSlack converts a Bitbucket PR's description to Slack markdown text.
//
// Based on:
//   - https://confluence.atlassian.com/bitbucketserver/markdown-syntax-guide-776639995.html
//   - https://bitbucket.org/tutorials/markdowndemo/src/master/
//   - https://docs.slack.dev/messaging/formatting-message-text/
func BitbucketToSlack(ctx workflow.Context, cmd *cli.Command, text string) string {
	text = strings.TrimSpace(text)

	// Header lines --> bold lines.
	text = regexp.MustCompile(`(?m)^#+\s+(.+)`).ReplaceAllString(text, "**${1}**")

	// Italic text: "*" --> "_" ("_" -> "_" is a no-op).
	text = regexp.MustCompile(`(^|[^*])\*([^*]+?)\*`).ReplaceAllString(text, "${1}_${2}_")

	// Bold text: "**" or "__" --> "*".
	text = regexp.MustCompile(`\*\*(.+?)\*\*`).ReplaceAllString(text, "*${1}*")
	text = regexp.MustCompile(`__(.+?)__`).ReplaceAllString(text, "*${1}*")

	// Strikethrough text: "~~" --> "~".
	text = regexp.MustCompile(`~~(.+?)~~`).ReplaceAllString(text, "~${1}~")

	// Links: "[text](url){: ... }" --> "<url|text>".
	// Images: "![text](url){: ... }" --> "!<url|text>" --> "Image: <url|text>".
	text = regexp.MustCompile(`\[(.*?)\]\((.*?)\)(\{:.*?\})?`).ReplaceAllString(text, "<${2}|${1}>")
	text = regexp.MustCompile(`!<(.*?)>`).ReplaceAllString(text, "Image: <${1}>")

	// Unordered lists (up to 2 levels): "-" or "*" or "+" --> "•" and "◦".
	for _, bullet := range []string{"-", `\+`} {
		// When editing the PR description in the Bitbucket web UI, it injects
		// superfluous "\n" and whitespaces between different levels of embedding.
		text = regexp.MustCompile(`(?m)\n\n\s{4}`+bullet).ReplaceAllString(text, "\n    "+bullet)
		text = regexp.MustCompile(`(?m)\n\s{4}\n`).ReplaceAllString(text, "\n")

		pattern := fmt.Sprintf(`(?m)^%s\s*`, bullet)
		text = regexp.MustCompile(pattern).ReplaceAllString(text, "  •  ")

		pattern = fmt.Sprintf(`(?m)^\s{2,4}%s\s*`, bullet)
		text = regexp.MustCompile(pattern).ReplaceAllString(text, "          ◦   ")
	}

	// Mentions: "@{account:uuid}" --> "<@U123>" or "Display Name",
	for _, bbRef := range regexp.MustCompile(`@\{[\w:-]+\}`).FindAllString(text, -1) {
		accountID := strings.TrimSuffix(bbRef[2:], "}")
		slackRef := users.BitbucketToSlackRef(ctx, cmd, accountID, "")
		text = strings.ReplaceAll(text, bbRef, slackRef)
	}

	return text
}
