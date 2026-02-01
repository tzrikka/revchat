package markdown

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/users"
)

// BitbucketToSlack converts Bitbucket markdown text into Slack markdown text.
//
// Based on:
//   - https://confluence.atlassian.com/bitbucketserver/markdown-syntax-guide-776639995.html
//   - https://bitbucket.org/tutorials/markdowndemo/src/master/
//   - https://docs.slack.dev/messaging/formatting-message-text/
func BitbucketToSlack(ctx workflow.Context, text, prURL string) string {
	text = bitbucketToSlackEmoji(text)

	// Before list styling, because our fake lists rely on whitespace prefixes.
	text = bitbucketToSlackWhitespaces(text)
	// Before text styling, to prevent confusion in "*"-based bullets with text that contains "*" characters.
	text = bitbucketToSlackLists(text)
	text = bitbucketToSlackTextStyles(text)
	text = bitbucketToSlackUnescapes(text)
	text = bitbucketToSlackLinks(text, prURL)

	// Mentions: "@{account:uuid}" --> "<@U123>" or "Display Name",
	for _, bbRef := range regexp.MustCompile(`@\{[\w:-]+\}`).FindAllString(text, -1) {
		accountID := bbRef[2 : len(bbRef)-1]
		text = strings.ReplaceAll(text, bbRef, users.BitbucketIDToSlackRef(ctx, accountID, ""))
	}

	return text
}

// bitbucketToSlackEmoji is the inverse of [slackToBitbucketEmoji].
func bitbucketToSlackEmoji(text string) string {
	text = strings.ReplaceAll(text, ":frame_photo:", ":frame_with_picture:")
	text = strings.ReplaceAll(text, ":man_facepalming:", ":man-facepalming:")
	text = strings.ReplaceAll(text, ":man_gesturing_ok:", ":man-gesturing-ok:")
	text = strings.ReplaceAll(text, ":man_shrugging:", ":man-shrugging:")
	text = strings.ReplaceAll(text, ":robot:", ":robot_face:")
	text = strings.ReplaceAll(text, ":rofl:", ":rolling_on_the_floor_laughing:")
	text = strings.ReplaceAll(text, ":slight_smile:", ":slightly_smiling_face:")
	text = strings.ReplaceAll(text, ":upside_down:", ":upside_down_face:")
	text = strings.ReplaceAll(text, ":woman_facepalming:", ":woman-facepalming:")
	text = strings.ReplaceAll(text, ":woman_gesturing_ok:", ":woman-gesturing-ok:")
	return strings.ReplaceAll(text, ":woman_shrugging:", ":woman-shrugging:")
}

func bitbucketToSlackLinks(text, prURL string) string {
	// Keep Slack-style links as-is (unlinkified).
	text = regexp.MustCompile(`<(\S+?)(\|.*?)?>`).ReplaceAllString(text, `&lt;${1}${2}>`)

	// Links: "[text](url){ ... }" --> "<url|text>".
	text = regexp.MustCompile(`\[([^[]*?)\]\((.*?)\)(\{.*?\})?`).ReplaceAllString(text, "<${2}|${1}>")

	// Images: "![text](url){ ... }" --> "!<url|text>" --> ":camera: <url|text>".
	text = regexp.MustCompile(`!(<|&lt;)(.*?)>`).ReplaceAllString(text, ":camera: <${2}>")

	// PR references: "[[proj/]repo]#123" --> "<PR URL|#123>".
	baseURL := regexp.MustCompile(`^https://([^/]+)/([^/]+)/([^/]+)/pull-requests/`).FindAllStringSubmatch(prURL, -1)
	if len(baseURL) == 0 || len(baseURL[0]) < 4 {
		return text
	}

	done := map[string]bool{}
	prPattern := regexp.MustCompile(`(([\w-]+/)?([\w-]+))?#(\d+)`)
	for _, id := range prPattern.FindAllStringSubmatch(text, -1) {
		if len(id) < 5 || done[id[0]] {
			continue
		}

		if id[2] == "" {
			id[2] = baseURL[0][2]
		} else {
			id[2], _ = strings.CutSuffix(id[2], "/")
		}

		if id[3] == "" {
			id[3] = baseURL[0][3]
		}

		link := fmt.Sprintf("<https://%s/%s/%s/pull-requests/%s|%s>", baseURL[0][1], id[2], id[3], id[4], id[0])
		text = strings.ReplaceAll(text, id[0], link)
		done[id[0]] = true
	}

	return text
}

func bitbucketToSlackLists(text string) string {
	text = regexp.MustCompile(`(?m)\n\n\s{4}([-*+])`).ReplaceAllString(text, "\n    ${1}")
	text = regexp.MustCompile(`(?m)\n\s{4}\n([-*+])`).ReplaceAllString(text, "\n${1}")

	text = regexp.MustCompile(`(?m)^[-*+]\s+`).ReplaceAllString(text, "  •  ")
	text = regexp.MustCompile(`(?m)^\s{2,4}[-*+]\s+`).ReplaceAllString(text, "          ◦   ")

	return regexp.MustCompile(`(?m)^(\s*\d+)\\\.(\s+)`).ReplaceAllString(text, "${1}.${2}")
}

func bitbucketToSlackTextStyles(text string) string {
	text = regexp.MustCompile(`\*\*(.+?)\*\*`).ReplaceAllString(text, "@REVCHAT-TEMP-BOLD@${1}@REVCHAT-TEMP-BOLD@")
	text = regexp.MustCompile(`__(.+?)__`).ReplaceAllString(text, "@REVCHAT-TEMP-BOLD@${1}@REVCHAT-TEMP-BOLD@")

	text = regexp.MustCompile(`\*(.+?)\*`).ReplaceAllString(text, "_${1}_")

	text = strings.ReplaceAll(text, "@REVCHAT-TEMP-BOLD@", "*")

	text = regexp.MustCompile(`~~(.+?)~~`).ReplaceAllString(text, "~${1}~")

	return regexp.MustCompile(`(?m)^(#+)\s+(.+)`).ReplaceAllString(text, "*${1} ${2}*")
}

func bitbucketToSlackUnescapes(text string) string {
	text = strings.ReplaceAll(text, `\+`, "+")
	text = strings.ReplaceAll(text, `\_`, "_")
	text = strings.ReplaceAll(text, "\\`", "`")
	text = strings.ReplaceAll(strings.ReplaceAll(text, `\[`, "["), `\]`, "]")
	text = strings.ReplaceAll(strings.ReplaceAll(text, `\{`, "{"), `\}`, "}")
	return strings.ReplaceAll(strings.ReplaceAll(text, `\(`, "("), `\)`, ")")
}

func bitbucketToSlackWhitespaces(text string) string {
	text = strings.TrimSpace(text)
	return regexp.MustCompile(`\n{3,}`).ReplaceAllString(text, "\n\n")
}

// stripSlackEmojiSkinTones removes Slack-style emoji skin tone suffixes.
func stripSlackEmojiSkinTones(text string) string {
	re := regexp.MustCompile(`(_tone[1-6])|(::skin-tone-[1-6])`)
	return re.ReplaceAllString(text, "")
}

// SlackToBitbucket converts Slack markdown text into Bitbucket markdown text.
func SlackToBitbucket(ctx workflow.Context, text string) string {
	text = stripSlackEmojiSkinTones(text)
	text = slackToBitbucketEmoji(text)

	text = slackToBitbucketWhitespaces(text)
	text = slackToBitbucketBlocks(text)
	text = slackToBitbucketLists(text)
	text = slackToBitbucketTextStyles(text)
	text = slackToBitbucketReferences(ctx, text)
	return slackToBitbucketLinks(text)
}

func slackToBitbucketBlocks(text string) string {
	text = regexp.MustCompile(`(?m)^&gt;`).ReplaceAllString(text, ">")
	text = regexp.MustCompile(`(?m)(^>.+)\n([^\n>])`).ReplaceAllString(text, "${1}\n\n${2}")
	return regexp.MustCompile("(?s)```(.+?)```").ReplaceAllString(text, "```\n${1}\n```")
}

func slackToBitbucketEmoji(text string) string {
	text = strings.ReplaceAll(text, ":frame_with_picture:", ":frame_photo:")
	text = strings.ReplaceAll(text, ":man-facepalming:", ":man_facepalming:")
	text = strings.ReplaceAll(text, ":man-gesturing-ok:", ":man_gesturing_ok:")
	text = strings.ReplaceAll(text, ":man-shrugging:", ":man_shrugging:")
	text = strings.ReplaceAll(text, ":robot_face:", ":robot:")
	text = strings.ReplaceAll(text, ":rolling_on_the_floor_laughing:", ":rofl:")
	text = strings.ReplaceAll(text, ":shrug:", ":man_shrugging:")
	text = strings.ReplaceAll(text, ":slightly_smiling_face:", ":slight_smile:")
	text = strings.ReplaceAll(text, ":upside_down_face:", ":upside_down:")
	text = strings.ReplaceAll(text, ":woman-facepalming:", ":woman_facepalming:")
	text = strings.ReplaceAll(text, ":woman-gesturing-ok:", ":woman_gesturing_ok:")
	return strings.ReplaceAll(text, ":woman-shrugging:", ":woman_shrugging:")
}

func slackToBitbucketLinks(text string) string {
	text = regexp.MustCompile(`\[(.*?)\]\((.*?)\)`).ReplaceAllString(text, `\[${1}](${2})`)
	text = regexp.MustCompile(`<(.*?)(\|(.*?))?>`).ReplaceAllString(text, "[${3}](${1})")
	return regexp.MustCompile(`\[\]\((.*?)\)`).ReplaceAllString(text, "[$1]($1)")
}

func slackToBitbucketLists(text string) string {
	text = regexp.MustCompile(`(?m)^• (\S)`).ReplaceAllString(text, "-  ${1}")
	text = regexp.MustCompile(`(?m)^    ◦ (\S)`).ReplaceAllString(text, "    -  ${1}")
	text = regexp.MustCompile(`(?m)^        ▪︎ (\S)`).ReplaceAllString(text, "        -  ${1}")

	text = regexp.MustCompile(`(\S  )\n(\s*-)`).ReplaceAllString(text, "${1}\n\n${2}")
	text = regexp.MustCompile(`(?m)(^\s*-.+)\n\n(\s*-)`).ReplaceAllString(text, "${1}\n${2}")
	return regexp.MustCompile(`(?m)(^\s*-.+)\n([^\n\s-])`).ReplaceAllString(text, "${1}\n\n${2}")
}

func slackToBitbucketReferences(ctx workflow.Context, text string) string {
	for _, slackRef := range regexp.MustCompile(`<@[A-Z0-9]+>`).FindAllString(text, -1) {
		bbRef := users.SlackMentionToBitbucketRef(ctx, slackRef)
		text = strings.ReplaceAll(text, slackRef, bbRef)
	}

	text = regexp.MustCompile(`<!subteam\^[A-Z0-9]+\|([^>]+)>`).ReplaceAllString(text, "${1}")

	text = strings.ReplaceAll(text, "<!here>", "@here")
	text = strings.ReplaceAll(text, "<!channel>", "@channel")

	for _, slackRef := range regexp.MustCompile(`<#([A-Z0-9]+)\|?>`).FindAllStringSubmatch(text, -1) {
		if len(slackRef) > 1 {
			id := slackRef[1]
			name := slackChannelIDToName(ctx, id)
			slackURL, _ := url.JoinPath(slackBaseURL(ctx), "archives", id)
			text = strings.ReplaceAll(text, slackRef[0], fmt.Sprintf("<%s|#%s>", slackURL, name))
		}
	}

	return text
}

func slackToBitbucketTextStyles(text string) string {
	text = regexp.MustCompile(`\*(.+?)\*`).ReplaceAllString(text, "**${1}**")
	return regexp.MustCompile(`~(.+?)~`).ReplaceAllString(text, "~~${1}~~")
}

func slackToBitbucketWhitespaces(text string) string {
	text = strings.TrimSpace(text)
	return regexp.MustCompile(`(\S)\n`).ReplaceAllString(text, "${1}  \n")
}
