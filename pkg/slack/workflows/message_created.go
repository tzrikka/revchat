package workflows

import (
	"errors"
	"fmt"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/bitbucket/activities"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/markdown"
)

func (c *Config) createMessage(ctx workflow.Context, event MessageEvent, userID string) error {
	switch {
	case c.BitbucketWorkspace != "":
		return c.createMessageBitbucket(ctx, event, userID)
	default:
		logger.From(ctx).Error("neither Bitbucket nor GitHub are configured")
		return errors.New("neither Bitbucket nor GitHub are configured")
	}
}

// createMessageBitbucket mirrors in Bitbucket the creation of a Slack message/reply.
// Broadcast replies are treated as normal replies, not as new top-level messages.
func (c *Config) createMessageBitbucket(ctx workflow.Context, event MessageEvent, userID string) error {
	if event.Subtype == "bot_message" {
		return nil // Slack bot, not a real user.
	}

	// Need to impersonate in Bitbucket the user who posted the Slack message.
	linkID, err := thrippyLinkID(ctx, userID, event.Channel)
	if err != nil || linkID == "" {
		return err
	}

	ids := event.Channel
	if event.ThreadTS != "" {
		ids = fmt.Sprintf("%s/%s", event.Channel, event.ThreadTS)
	}

	url, err := urlParts(ctx, ids)
	if err != nil {
		return err
	}

	msg := markdown.SlackToBitbucket(ctx, event.Text) + c.fileLinks(ctx, event.Files)
	msg += "\n\n[This comment was created by RevChat]: #"

	newCommentURL, err := activities.CreatePullRequestComment(ctx, linkID, url[1], url[2], url[3], url[5], msg)
	if err != nil {
		return err
	}

	_ = data.MapURLAndID(ctx, newCommentURL, fmt.Sprintf("%s/%s", ids, event.TS))
	return nil
}

func (c *Config) fileLinks(ctx workflow.Context, files []File) string {
	if len(files) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\nAttached files:\n")
	for _, f := range files {
		sb.WriteString(fmt.Sprintf("\n- :%s: [%s](%s)", c.fileTypeEmoji(ctx, f), f.Name, f.Permalink))
	}
	return sb.String()
}

func (c *Config) fileTypeEmoji(ctx workflow.Context, f File) string {
	switch {
	case c.BitbucketWorkspace != "":
		return fileTypeEmojiBitbucket(f)
	default:
		logger.From(ctx).Error("neither Bitbucket nor GitHub are configured")
		return "paperclip"
	}
}

// https://docs.slack.dev/reference/objects/file-object/#types
func fileTypeEmojiBitbucket(f File) string {
	switch strings.ToLower(f.FileType) {
	// Multimedia.
	case "aac", "m4a", "mp3", "ogg", "wav":
		return "headphones"
	case "avi", "flv", "mkv", "mov", "mp4", "mpg", "ogv", "webm", "wmv":
		return "clapper"
	case "bmp", "dgraw", "eps", "odg", "odi", "psd", "svg", "tiff":
		return "frame_photo" // This is "frame_with_picture" in Slack.
	case "gif", "jpg", "jpeg", "png", "webp":
		return "camera"

	// Documents.
	case "text", "csv", "diff", "doc", "docx", "dotx", "gdoc", "json", "markdown", "odt", "rtf", "xml", "yaml":
		return "pencil"
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
		return "robot" // This is "robot_face" in Slack.
	case "powershell", "python", "rust", "sql", "shell":
		return "robot" // This is "robot_face" in Slack.

	// Everything else.
	default:
		return "paperclip"
	}
}
