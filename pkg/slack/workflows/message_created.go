package workflows

import (
	"fmt"
	"strconv"
	"strings"

	"go.temporal.io/sdk/workflow"

	bitbucket "github.com/tzrikka/revchat/pkg/bitbucket/activities"
	"github.com/tzrikka/revchat/pkg/data"
	github "github.com/tzrikka/revchat/pkg/github/activities"
	"github.com/tzrikka/revchat/pkg/markdown"
	"github.com/tzrikka/revchat/pkg/slack/activities"
)

// createMessageInBitbucket mirrors the creation of a Slack message/reply in Bitbucket or
// GitHub. Broadcast replies are treated as normal replies, not as new top-level messages.
func (c *Config) createMessage(ctx workflow.Context, event MessageEvent, userID string, isBitbucket bool) error {
	// Impersonating the user who posted the Slack message is not an option with bots.
	if event.Subtype == "bot_message" {
		return nil
	}

	thrippyID, err := c.thrippyLinkID(ctx, userID, event.Channel)
	if err != nil || thrippyID == "" {
		return err
	}

	// Start with the Slack ID(s) of the message's "parent": the channel's PR, or a thread's root comment.
	slackIDs := event.Channel
	if event.ThreadTS != "" {
		slackIDs = fmt.Sprintf("%s/%s", slackIDs, event.ThreadTS)
	}

	parentURL, err := c.urlParts(ctx, slackIDs)
	if err != nil {
		return err
	}

	var newCommentURL string
	switch {
	case isBitbucket:
		newCommentURL, err = createCommentInBitbucket(ctx, event, thrippyID, parentURL)
	case event.ThreadTS == "":
		newCommentURL, err = createReviewInGitHub(ctx, event, thrippyID, parentURL)
	default:
		newCommentURL, err = createReplyInGitHub(ctx, event, thrippyID, parentURL)
	}
	if err != nil {
		return err
	}

	slackIDs = fmt.Sprintf("%s/%s", slackIDs, event.TS)
	return activities.AlertError(ctx, c.AlertsChannel, "failed to set mapping between a new PR comment and its Slack IDs",
		data.MapURLAndID(ctx, newCommentURL, slackIDs), "Comment URL", newCommentURL, "Slack IDs", slackIDs)
}

func createCommentInBitbucket(ctx workflow.Context, event MessageEvent, thrippyID string, url []string) (string, error) {
	msg := markdown.SlackToBitbucket(ctx, event.Text) + fileLinks(event.Files, true)
	msg += "\n\n[This comment was created by RevChat]: #"

	return bitbucket.CreatePullRequestComment(ctx, thrippyID, url[2], url[3], url[5], url[7], msg)
}

func createReviewInGitHub(ctx workflow.Context, event MessageEvent, thrippyID string, url []string) (string, error) {
	msg := markdown.SlackToGitHub(ctx, event.Text) + fileLinks(event.Files, false)
	msg += "\n\n[This comment was created by RevChat]: #"

	prID, err := strconv.Atoi(url[5])
	if err != nil {
		return "", fmt.Errorf("failed to parse PR number %q: %w", url[5], err)
	}

	return github.CreateFileReviewComment(ctx, thrippyID, url[2], url[3], prID, msg)
}

func createReplyInGitHub(ctx workflow.Context, event MessageEvent, thrippyID string, url []string) (string, error) {
	msg := markdown.SlackToGitHub(ctx, event.Text) + fileLinks(event.Files, false)
	msg += "\n\n[This comment was created by RevChat]: #"

	prID, err := strconv.Atoi(url[5])
	if err != nil {
		return "", fmt.Errorf("failed to parse PR number %q: %w", url[5], err)
	}

	commentID, err := strconv.Atoi(url[7])
	if err != nil {
		return "", fmt.Errorf("failed to parse comment ID %q: %w", url[7], err)
	}

	return github.CreateReviewCommentReply(ctx, thrippyID, url[2], url[3], prID, commentID, msg)
}

func fileLinks(files []File, isBitbucket bool) string {
	if len(files) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\nAttached files:\n")
	for _, f := range files {
		sb.WriteString(fmt.Sprintf("\n- :%s: [%s](%s)", fileTypeEmoji(f, isBitbucket), f.Name, f.Permalink))
	}

	return sb.String()
}

// https://docs.slack.dev/reference/objects/file-object/#types
func fileTypeEmoji(f File, isBitbucket bool) string {
	switch strings.ToLower(f.FileType) {
	// Multimedia.
	case "aac", "m4a", "mp3", "ogg", "wav":
		return "headphones"
	case "avi", "flv", "mkv", "mov", "mp4", "mpg", "ogv", "webm", "wmv":
		return "clapper"
	case "bmp", "dgraw", "eps", "odg", "odi", "psd", "svg", "tiff":
		if isBitbucket {
			return "frame_photo"
		}
		return "framed_picture"
	case "gif", "jpg", "jpeg", "png", "webp":
		return "camera"

	// Documents.
	case "text", "csv", "diff", "doc", "docx", "dotx", "gdoc", "json", "markdown", "odt", "rtf", "xml", "yaml":
		if isBitbucket {
			return "pencil"
		}
		return "memo"
	case "eml", "epub", "html", "latex", "mhtml", "pdf":
		return "book"
	case "gsheet", "ods", "xls", "xlsb", "xlsm", "xlsx", "xltx":
		return "bar_chart"
	case "gpres", "odp", "ppt", "pptx":
		return "chart_with_upwards_trend"
	case "gz", "gzip", "tar", "zip":
		return "file_cabinet"

	// Source code.
	case "apk", "c", "csharp", "cpp", "css", "dockerfile", "go", "java", "javascript", "js", "kotlin", "lua":
		fallthrough
	case "powershell", "python", "rust", "sql", "shell":
		if isBitbucket {
			return "code"
		}
		return "robot_face"

	// Everything else.
	default:
		return "paperclip"
	}
}
