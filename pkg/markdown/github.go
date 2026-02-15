package markdown

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/users"
)

// GitHubToSlack converts a GitHub PR's body to Slack markdown text.
//
// Based on:
//   - https://docs.github.com/en/get-started/writing-on-github/getting-started-with-writing-and-formatting-on-github/basic-writing-and-formatting-syntax
//   - https://docs.slack.dev/messaging/formatting-message-text/
func GitHubToSlack(ctx workflow.Context, text, prURL string) string {
	// Before list styling, because our fake lists rely on whitespace prefixes.
	text = gitHubToSlackWhitespaces(text)
	// Before text styling, to prevent confusion in "*"-based bullets with text that contains "*" characters.
	text = gitHubToSlackLists(text)
	text = gitHubToSlackTextStyles(text)
	text = gitHubToSlackLinks(text, prURL)

	// Hide HTML comments and <sub> tags.
	text = regexp.MustCompile(`(?s)<!--.+?-->`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?s)<sub>.+?</sub>`).ReplaceAllString(text, "")

	// Newlines: no more than 1 or 2.
	text = regexp.MustCompile(`\n{3,}`).ReplaceAllString(text, "\n\n")

	// Mentions: "@user" --> "<@U123>" or "<https://github.com/user|@user>",
	// "@org/team" --> "<https://github.com/org/teams/team|@org/team>".
	for _, ghRef := range regexp.MustCompile(`@[\w/-]+`).FindAllString(text, -1) {
		u, err := url.Parse(prURL)
		if err != nil {
			break
		}

		username := ghRef[1:]
		if strings.Contains(username, "/") {
			username = "orgs/" + strings.Replace(username, "/", "/teams/", 1)
		}

		profile := fmt.Sprintf("%s://%s/%s", u.Scheme, u.Host, username)
		slackRef := users.GitHubIDToSlackRef(ctx, username, profile, "")
		text = strings.ReplaceAll(text, ghRef, slackRef)
	}

	return text
}

func gitHubToSlackLinks(text, prURL string) string {
	// Links: "[text](url)" --> "<url|text>".
	text = regexp.MustCompile(`\[(.*?)\]\((.*?)\)`).ReplaceAllString(text, "<${2}|${1}>")

	// Images: "![text](url)" --> "!<url|text>" --> "Image: <url|text>".
	text = regexp.MustCompile(`!<(.*?)>`).ReplaceAllString(text, "Image: <${1}>")

	// PR and issue references: "#123" --> "<PR URL|#123>".
	baseURL := "<" + regexp.MustCompile(`/pull/\d+$`).ReplaceAllString(prURL, "/pull/")
	return regexp.MustCompile(`#(\d+)`).ReplaceAllString(text, baseURL+"${1}|#${1}>")
}

func gitHubToSlackLists(text string) string {
	// Up to 2 levels: "-" or "*" or "+" --> "•" and "◦".
	text = regexp.MustCompile(`(?m)^[-*+]\s+`).ReplaceAllString(text, "  •   ")
	return regexp.MustCompile(`(?m)^\s{2,4}[-*+]\s+`).ReplaceAllString(text, "          ◦   ")
}

func gitHubToSlackTextStyles(text string) string {
	// Bold and italic text together: "*** ... ***" --> "_* ... *_".
	text = regexp.MustCompile(`\*\*\*(.+?)\*\*\*`).ReplaceAllString(text, "_@REVCHAT-TEMP-BOLD@${1}@REVCHAT-TEMP-BOLD@_")

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

func gitHubToSlackWhitespaces(text string) string {
	text = strings.TrimSpace(text)

	// "\r\n" --> "\n" (e.g. tables in Copilot reviews).
	text = strings.ReplaceAll(text, "\r\n", "\n")

	return text
}

// SlackToGitHub converts Slack markdown text into GitHub markdown text.
//
// Based on:
//   - https://docs.slack.dev/messaging/formatting-message-text/
//   - https://docs.github.com/en/get-started/writing-on-github/getting-started-with-writing-and-formatting-on-github/basic-writing-and-formatting-syntax
func SlackToGitHub(_ workflow.Context, text string) string {
	return text
}
