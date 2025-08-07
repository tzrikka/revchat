package markdown

import "regexp"

// BitbucketToSlack converts a Bitbucket PR's description to Slack markdown text.
// Based on: https://docs.slack.dev/messaging/formatting-message-text/.
func BitbucketToSlack(text string) string {
	// Header lines --> bold lines.
	text = regexp.MustCompile(`(?m)^#+\s+(.+)`).ReplaceAllString(text, "*${1}*")

	// Bold text: "**" --> "*".
	text = regexp.MustCompile(`\*\*(.+?)\*\*`).ReplaceAllString(text, "*${1}*")

	// Strikethrough text: "~~" --> "~".
	text = regexp.MustCompile(`~~(.+?)~~`).ReplaceAllString(text, "~${1}~")

	// Links: "[text](url){: ... }" --> "<url|text>".
	// Images: "![text](url){: ... }" --> "!<url|text>" --> "Image: <url|text>".
	text = regexp.MustCompile(`\[(.*?)\]\((.*?)\)(\{:.*?\})?`).ReplaceAllString(text, "<${2}|${1}>")
	text = regexp.MustCompile(`!<(.*?)>`).ReplaceAllString(text, "Image: <${1}>")

	return text
}
