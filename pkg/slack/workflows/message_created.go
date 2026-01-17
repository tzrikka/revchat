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
)

// createMessageInBitbucket mirrors the creation of a Slack message/reply in Bitbucket or
// GitHub. Broadcast replies are treated as normal replies, not as new top-level messages.
func createMessage(ctx workflow.Context, event MessageEvent, userID string, isBitbucket bool) error {
	// Impersonating the user who posted the Slack message is not an option with bots.
	if event.Subtype == "bot_message" {
		return nil
	}

	thrippyID, err := thrippyLinkID(ctx, userID, event.Channel)
	if err != nil || thrippyID == "" {
		return err
	}

	slackIDs := event.Channel
	if event.ThreadTS != "" {
		slackIDs = fmt.Sprintf("%s/%s", event.Channel, event.ThreadTS)
	}

	url, err := urlParts(ctx, slackIDs)
	if err != nil {
		return err
	}

	if isBitbucket {
		return createMessageInBitbucket(ctx, event, thrippyID, slackIDs, url)
	}
	return createMessageInGitHub(ctx, event, thrippyID, slackIDs, url)
}

func createMessageInBitbucket(ctx workflow.Context, event MessageEvent, thrippyID, slackIDs string, url []string) error {
	msg := markdown.SlackToBitbucket(ctx, event.Text) + fileLinks(event.Files, true)
	msg += "\n\n[This comment was created by RevChat]: #"

	newCommentURL, err := bitbucket.CreatePullRequestComment(ctx, thrippyID, url[2], url[3], url[5], url[7], msg)
	if err != nil {
		return err
	}

	return data.MapURLAndID(ctx, newCommentURL, fmt.Sprintf("%s/%s", slackIDs, event.TS))
}

func createMessageInGitHub(ctx workflow.Context, event MessageEvent, thrippyID, slackIDs string, url []string) error {
	msg := markdown.SlackToGitHub(ctx, event.Text) + fileLinks(event.Files, false)
	msg += "\n\n[This comment was created by RevChat]: #"

	prID, err := strconv.Atoi(url[5])
	if err != nil {
		return fmt.Errorf("failed to parse PR number %q: %w", url[5], err)
	}

	newCommentURL, err := github.CreateIssueComment(ctx, thrippyID, url[2], url[3], prID, msg)
	if err != nil {
		return err
	}

	return data.MapURLAndID(ctx, newCommentURL, fmt.Sprintf("%s/%s", slackIDs, event.TS))
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
