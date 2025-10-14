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

// BitbucketToSlack converts Bitbucket markdown text into Slack markdown text.
//
// Based on:
//   - https://confluence.atlassian.com/bitbucketserver/markdown-syntax-guide-776639995.html
//   - https://bitbucket.org/tutorials/markdowndemo/src/master/
//   - https://docs.slack.dev/messaging/formatting-message-text/
func BitbucketToSlack(ctx workflow.Context, cmd *cli.Command, text, prURL string) string {
	// Before list styling, because our fake lists rely on whitespace prefixes.
	text = bitbucketToSlackWhitespaces(text)
	// Before text styling, to prevent confusion in "*"-based bullets with text that contains "*" characters.
	text = bitbucketToSlackLists(text)
	text = bitbucketToSlackTextStyles(text)
	text = bitbucketToSlackLinks(text, prURL)

	// Mentions: "@{account:uuid}" --> "<@U123>" or "Display Name",
	for _, bbRef := range regexp.MustCompile(`@\{[\w:-]+\}`).FindAllString(text, -1) {
		accountID := bbRef[2 : len(bbRef)-1]
		text = strings.ReplaceAll(text, bbRef, users.BitbucketToSlackRef(ctx, cmd, accountID, ""))
	}

	return text
}

func bitbucketToSlackLinks(text, prURL string) string {
	// Keep Slack-style links as-is (unlinkified).
	text = regexp.MustCompile(`<(\S+?)(\|.*?)?>`).ReplaceAllString(text, `&lt;${1}${2}>`)

	// Links: "[text](url){ ... }" --> "<url|text>".
	text = regexp.MustCompile(`\[(.*?)\]\((.*?)\)(\{.*?\})?`).ReplaceAllString(text, "<${2}|${1}>")

	// Images: "![text](url){ ... }" --> "!<url|text>" --> ":camera: <url|text>".
	text = regexp.MustCompile(`!(<|&lt;)(.*?)>`).ReplaceAllString(text, ":camera: <${2}>")

	// PR references: "#123" --> "<PR URL|#123>".
	baseURL := "<" + regexp.MustCompile(`/pull-requests/\d+$`).ReplaceAllString(prURL, "/pull-requests/")
	text = regexp.MustCompile(`#(\d+)`).ReplaceAllString(text, baseURL+"${1}|#${1}>")

	// PR references in another repo: "repo#123" --> "<PR URL|repo#123>".
	pattern := `([\w-]+)<(.*?)/([\w-]+)/pull-requests/(\d+)\|`
	text = regexp.MustCompile(pattern).ReplaceAllString(text, "<${2}/${1}/pull-requests/${4}|${1}")

	// PR references in another workspace: "proj/repo#123" --> "<PR URL|proj/repo#123>".
	pattern = `([\w-]+)/<(.*?)/([\w-]+)/([\w-]+)/pull-requests/(\d+)\|`
	return regexp.MustCompile(pattern).ReplaceAllString(text, "<${2}/${1}/${4}/pull-requests/${5}|${1}/")
}

func bitbucketToSlackLists(text string) string {
	// The Bitbucket web UI injects superfluous whitespaces between different levels.
	text = regexp.MustCompile(`(?m)\n\n\s{4}([-*+])`).ReplaceAllString(text, "\n    ${1}")
	text = regexp.MustCompile(`(?m)\n\s{4}\n([-*+])`).ReplaceAllString(text, "\n${1}")

	// Up to 2 levels: "-" or "+" or "*" --> "•" and "◦".
	text = regexp.MustCompile(`(?m)^[-*+]\s+`).ReplaceAllString(text, "  •  ")
	return regexp.MustCompile(`(?m)^\s{2,4}[-*+]\s+`).ReplaceAllString(text, "          ◦   ")
}

func bitbucketToSlackTextStyles(text string) string {
	// Bold text: "**" or "__" --> "*".
	text = regexp.MustCompile(`\*\*(.+?)\*\*`).ReplaceAllString(text, "@REVCHAT-TEMP-BOLD@${1}@REVCHAT-TEMP-BOLD@")
	text = regexp.MustCompile(`__(.+?)__`).ReplaceAllString(text, "@REVCHAT-TEMP-BOLD@${1}@REVCHAT-TEMP-BOLD@")

	// Italic text: "*" --> "_" ("_" -> "_" is a no-op).
	// This is why we use a temporary marker for bold text.
	text = regexp.MustCompile(`\*(.+?)\*`).ReplaceAllString(text, "_${1}_")

	// Finalize the transformation of bold text.
	text = strings.ReplaceAll(text, "@REVCHAT-TEMP-BOLD@", "*")

	// Strikethrough text: "~~" --> "~" ("~" -> "~" is a no-op).
	text = regexp.MustCompile(`~~(.+?)~~`).ReplaceAllString(text, "~${1}~")

	// Header lines --> bold lines: "# ..." --> "*# ...*".
	return regexp.MustCompile(`(?m)^(#+)\s+(.+)`).ReplaceAllString(text, "*${1} ${2}*")
}

func bitbucketToSlackWhitespaces(text string) string {
	text = strings.TrimSpace(text)

	// Newlines: no more than 1 or 2.
	return regexp.MustCompile(`\n{3,}`).ReplaceAllString(text, "\n\n")
}

// SlackToBitbucket converts Slack markdown text into Bitbucket markdown text.
//
// Based on:
//   - https://docs.slack.dev/messaging/formatting-message-text/
//   - https://confluence.atlassian.com/bitbucketserver/markdown-syntax-guide-776639995.html
//   - https://bitbucket.org/tutorials/markdowndemo/src/master/
func SlackToBitbucket(ctx workflow.Context, bitbucketWorkspace, text string) string {
	// Before the rest because they undo a few whitespace changes.
	text = slackToBitbucketWhitespaces(text)

	text = slackToBitbucketBlocks(text)
	text = slackToBitbucketLists(text)
	text = slackToBitbucketTextStyles(text)
	text = slackToBitbucketReferences(ctx, bitbucketWorkspace, text)

	// Links: "<url>" and "<url|text>" --> "[text](url)".
	text = regexp.MustCompile(`<(.*?)(\|(.*?))?>`).ReplaceAllString(text, "[${3}](${1})")
	return regexp.MustCompile(`\[\]\((.*?)\)`).ReplaceAllString(text, "[$1]($1)")
}

func slackToBitbucketBlocks(text string) string {
	// Quote blocks: "&gt;" --> ">" (">" --> ">" is a no-op).
	text = regexp.MustCompile(`(?m)^&gt;`).ReplaceAllString(text, ">")
	// Add a second newline after the last line of a quote block, if missing.
	text = regexp.MustCompile(`(?m)(^>.+)\n([^\n>])`).ReplaceAllString(text, "${1}\n\n${2}")

	// Code blocks: "```...```" --> "```\n...\n```".
	return regexp.MustCompile("(?s)```(.+?)```").ReplaceAllString(text, "```\n${1}\n```")
}

func slackToBitbucketLists(text string) string {
	// Up to 3 levels: "•" and "◦" and "▪︎" --> "-".
	text = regexp.MustCompile(`(?m)^• (\S)`).ReplaceAllString(text, "-  ${1}")
	text = regexp.MustCompile(`(?m)^    ◦ (\S)`).ReplaceAllString(text, "    -  ${1}")
	text = regexp.MustCompile(`(?m)^        ▪︎ (\S)`).ReplaceAllString(text, "        -  ${1}")

	// Add a second newline before and after (but not between) list items, if missing.
	text = regexp.MustCompile(`(\S  )\n(\s*-)`).ReplaceAllString(text, "${1}\n\n${2}")
	text = regexp.MustCompile(`(?m)(^\s*-.+)\n\n(\s*-)`).ReplaceAllString(text, "${1}\n${2}")
	text = regexp.MustCompile(`(?m)(^\s*-.+)\n\n(\s*-)`).ReplaceAllString(text, "${1}\n${2}")
	return regexp.MustCompile(`(?m)(^\s*-.+)\n([^\n\s-])`).ReplaceAllString(text, "${1}\n\n${2}")
}

func slackToBitbucketReferences(ctx workflow.Context, bitbucketWorkspace, text string) string {
	// User mentions: "<@U123>" --> "@{account:uuid}" or "Display Name".
	for _, slackRef := range regexp.MustCompile(`<@[A-Z0-9]+>`).FindAllString(text, -1) {
		bbRef := users.SlackToBitbucketRef(ctx, bitbucketWorkspace, slackRef)
		text = strings.ReplaceAll(text, slackRef, bbRef)
	}

	// Special mentions: "<!...>" --> "@...".
	text = strings.ReplaceAll(text, "<!here>", "@here")
	text = strings.ReplaceAll(text, "<!channel>", "@channel")

	// Channel references: "<#C123|>" --> "<link|@name>".
	for _, slackRef := range regexp.MustCompile(`<#([A-Z0-9]+)\|?>`).FindAllStringSubmatch(text, -1) {
		if len(slackRef) > 1 {
			id := slackRef[1]
			name := slackChannelIDToName(ctx, id)
			u, _ := url.JoinPath(slackBaseURL(ctx), "archives", id) // "" on error.
			text = strings.ReplaceAll(text, slackRef[0], fmt.Sprintf("<%s|#%s>", u, name))
		}
	}

	return text
}

func slackToBitbucketTextStyles(text string) string {
	// Bold text: "*" --> "**".
	text = regexp.MustCompile(`\*(.+?)\*`).ReplaceAllString(text, "**${1}**")

	// Italic text: "_" --> "_" is a no-op.

	// Strikethrough text: "~" --> "~~".
	return regexp.MustCompile(`~(.+?)~`).ReplaceAllString(text, "~~${1}~~")
}

func slackToBitbucketWhitespaces(text string) string {
	text = strings.TrimSpace(text)

	// Newlines in Bitbucket: "\n" on its own is not enough.
	return regexp.MustCompile(`(\S)\n`).ReplaceAllString(text, "${1}  \n")
}
