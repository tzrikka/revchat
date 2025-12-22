package slack

import (
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/markdown"
	"github.com/tzrikka/timpani-api/pkg/bitbucket"
	"github.com/tzrikka/timpani-api/pkg/slack"
)

// https://docs.slack.dev/reference/events/message/
func (c *Config) messageWorkflow(ctx workflow.Context, event messageEventWrapper) error {
	if !prChannel(ctx, event.InnerEvent.Channel) {
		return nil
	}

	userID := extractUserID(ctx, &event.InnerEvent)
	if userID == "" {
		msg := "could not determine who triggered a Slack message event"
		logger.Error(ctx, msg, nil)
		return errors.New(msg)
	}

	if selfTriggeredEvent(ctx, event.Authorizations, userID) {
		return nil
	}

	subtype := event.InnerEvent.Subtype
	if strings.HasPrefix(subtype, "channel_") || strings.HasPrefix(subtype, "group_") {
		return nil
	}
	if subtype == "reminder_add" || event.InnerEvent.User == "USLACKBOT" {
		return nil
	}

	switch subtype {
	case "", "bot_message", "file_share", "thread_broadcast":
		return c.addMessage(ctx, event.InnerEvent, userID)
	case "message_changed":
		return c.changeMessage(ctx, event.InnerEvent, userID)
	case "message_deleted":
		return c.deleteMessage(ctx, event.InnerEvent, userID)
	}

	logger.Warn(ctx, "unhandled Slack message event", slog.Any("event", event.InnerEvent))
	return nil
}

// extractUserID determines the user ID of the user/app that triggered a Slack message event.
// This ID is located in different places depending on the event subtype and the user type.
func extractUserID(ctx workflow.Context, msg *MessageEvent) string {
	if msg == nil {
		return ""
	}

	if msg.Edited != nil && msg.Edited.User != "" {
		return msg.Edited.User
	}

	if msg.BotID != "" {
		return convertBotIDToUserID(ctx, msg.BotID)
	}

	if user := extractUserID(ctx, msg.Message); user != "" {
		return user
	}

	if user := extractUserID(ctx, msg.PreviousMessage); user != "" {
		return user
	}

	return msg.User
}

// convertBotIDToUserID uses a cache to convert Slack bot IDs to a user IDs.
func convertBotIDToUserID(ctx workflow.Context, botID string) string {
	userID, err := data.GetSlackBotUserID(botID)
	if err != nil {
		logger.Error(ctx, "failed to load Slack bot's user ID", err, slog.String("bot_id", botID))
		return ""
	}

	if userID != "" {
		return userID
	}

	bot, err := slack.BotsInfo(ctx, botID)
	if err != nil {
		logger.Error(ctx, "failed to retrieve bot info from Slack", err, slog.String("bot_id", botID))
		return ""
	}

	logger.Debug(ctx, "retrieved bot info from Slack", slog.String("bot_id", botID),
		slog.String("user_id", bot.UserID), slog.String("name", bot.Name))
	if err := data.SetSlackBotUserID(botID, bot.UserID); err != nil {
		logger.Error(ctx, "failed to save Slack bot's user ID", err, slog.String("bot_id", botID))
	}

	return bot.UserID
}

func (c *Config) addMessage(ctx workflow.Context, event MessageEvent, userID string) error {
	switch {
	case c.BitbucketWorkspace != "":
		return c.addMessageBitbucket(ctx, event, userID)
	default:
		logger.Error(ctx, "neither Bitbucket nor GitHub are configured", nil)
		return errors.New("neither Bitbucket nor GitHub are configured")
	}
}

func (c *Config) changeMessage(ctx workflow.Context, event MessageEvent, userID string) error {
	switch {
	case c.BitbucketWorkspace != "":
		return c.editMessageBitbucket(ctx, event, userID)
	default:
		logger.Error(ctx, "neither Bitbucket nor GitHub are configured", nil)
		return errors.New("neither Bitbucket nor GitHub are configured")
	}
}

func (c *Config) deleteMessage(ctx workflow.Context, event MessageEvent, userID string) error {
	switch {
	case c.BitbucketWorkspace != "":
		return deleteMessageBitbucket(ctx, event, userID)
	default:
		logger.Error(ctx, "neither Bitbucket nor GitHub are configured", nil)
		return errors.New("neither Bitbucket nor GitHub are configured")
	}
}

// addMessageBitbucket mirrors in Bitbucket the creation of a Slack message/reply.
// Broadcast replies are treated as normal replies, not as new top-level messages.
func (c *Config) addMessageBitbucket(ctx workflow.Context, event MessageEvent, userID string) error {
	if event.Subtype == "bot_message" {
		return nil // Slack bot, not a real user.
	}

	ids := event.Channel
	if event.ThreadTS != "" {
		ids = fmt.Sprintf("%s/%s", event.Channel, event.ThreadTS)
	}

	// If we're not tracking this PR, there's no need/way to announce this event.
	url, err := urlElements(ctx, ids)
	if err != nil || url == nil {
		return err
	}

	// Need to impersonate in Bitbucket the user who sent the Slack message.
	linkID, err := thrippyLinkID(ctx, userID, event.Channel)
	if err != nil || linkID == "" {
		return err
	}

	msg := markdown.SlackToBitbucket(ctx, c.BitbucketWorkspace, event.Text) + c.fileLinks(ctx, event.Files)
	msg += "\n\n[This comment was created by RevChat]: #"

	resp, err := bitbucket.PullRequestsCreateComment(ctx, bitbucket.PullRequestsCreateCommentRequest{
		PullRequestsRequest: bitbucket.PullRequestsRequest{
			ThrippyLinkID: linkID,
			Workspace:     url[1],
			RepoSlug:      url[2],
			PullRequestID: url[3],
		},
		Markdown: msg,
		ParentID: url[5], // Optional.
	})
	if err != nil {
		logger.Error(ctx, "failed to create Bitbucket PR comment", err,
			slog.String("slack_ids", ids), slog.String("pr_url", url[0]))
		return fmt.Errorf("failed to create Bitbucket PR comment: %w", err)
	}

	url[0] = resp.Links["html"].HRef
	ids = fmt.Sprintf("%s/%s", ids, event.TS)

	if err := data.MapURLAndID(url[0], ids); err != nil {
		logger.Error(ctx, "failed to save PR comment URL / Slack IDs mapping", err,
			slog.String("slack_ids", ids), slog.String("comment_url", url[0]))
		// Don't return the error - the message is already created in Bitbucket, so
		// we don't want to retry and post it again, even though this is problematic.
	}

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
		logger.Error(ctx, "neither Bitbucket nor GitHub are configured", nil)
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

// editMessageBitbucket mirrors in Bitbucket the editing of a Slack message/reply.
func (c *Config) editMessageBitbucket(ctx workflow.Context, event MessageEvent, userID string) error {
	// Ignore "fake" edit events when a broadcast reply is created/deleted.
	if event.Message.Text == event.PreviousMessage.Text {
		return nil
	}

	ids := fmt.Sprintf("%s/%s", event.Channel, event.Message.TS)
	if event.Message.ThreadTS != "" && event.Message.ThreadTS != event.Message.TS {
		ids = fmt.Sprintf("%s/%s/%s", event.Channel, event.Message.ThreadTS, event.Message.TS)
	}

	// If we're not tracking this PR, there's no need/way to announce this event.
	url, err := urlElements(ctx, ids)
	if err != nil || url == nil {
		return err
	}

	// Need to impersonate in Bitbucket the user who sent the Slack message.
	linkID, err := thrippyLinkID(ctx, userID, event.Channel)
	if err != nil || linkID == "" {
		return err
	}

	msg := markdown.SlackToBitbucket(ctx, c.BitbucketWorkspace, event.Message.Text)
	err = bitbucket.PullRequestsUpdateComment(ctx, bitbucket.PullRequestsUpdateCommentRequest{
		PullRequestsRequest: bitbucket.PullRequestsRequest{
			ThrippyLinkID: linkID,
			Workspace:     url[1],
			RepoSlug:      url[2],
			PullRequestID: url[3],
		},
		CommentID: url[5],
		Markdown:  msg,
	})
	if err != nil {
		logger.Error(ctx, "failed to update Bitbucket PR comment", err,
			slog.String("slack_ids", ids), slog.String("comment_url", url[0]))
		return err
	}

	return nil
}

// deleteMessageBitbucket mirrors in Bitbucket the deletion of a Slack message/reply.
func deleteMessageBitbucket(ctx workflow.Context, event MessageEvent, userID string) error {
	// Don't delete "tombstone" messages (roots of threads).
	if event.PreviousMessage.Subtype == "tombstone" {
		return nil
	}

	ids := fmt.Sprintf("%s/%s", event.Channel, event.DeletedTS)
	if event.PreviousMessage.ThreadTS != "" && event.PreviousMessage.ThreadTS != event.DeletedTS {
		ids = fmt.Sprintf("%s/%s/%s", event.Channel, event.PreviousMessage.ThreadTS, event.DeletedTS)
	}

	// If we're not tracking this PR, there's no need/way to announce this event.
	url, err := urlElements(ctx, ids)
	if err != nil || url == nil {
		return err
	}

	if err := data.DeleteURLAndIDMapping(url[0]); err != nil {
		logger.Error(ctx, "failed to delete URL/Slack mappings", err, slog.String("comment_url", url[0]))
		// Don't abort - we still want to attempt to delete the PR comment.
	}

	// Need to impersonate in Bitbucket the user who sent the Slack message.
	linkID, err := thrippyLinkID(ctx, userID, event.Channel)
	if err != nil || linkID == "" {
		return err
	}

	err = bitbucket.PullRequestsDeleteComment(ctx, bitbucket.PullRequestsDeleteCommentRequest{
		PullRequestsRequest: bitbucket.PullRequestsRequest{
			ThrippyLinkID: linkID,
			Workspace:     url[1],
			RepoSlug:      url[2],
			PullRequestID: url[3],
		},
		CommentID: url[5],
	})
	if err != nil {
		logger.Error(ctx, "failed to delete Bitbucket PR comment", err,
			slog.String("slack_ids", ids), slog.String("comment_url", url[0]))
		return err
	}

	return nil
}

var commentURLPattern = regexp.MustCompile(`^https://[^/]+/(.+?)/(.+?)/pull-requests/(\d+)(.+comment-(\d+))?`)

const ExpectedSubmatches = 6

func urlElements(ctx workflow.Context, ids string) ([]string, error) {
	url, err := commentURL(ctx, ids)
	if err != nil || url == "" {
		return nil, err
	}

	sub := commentURLPattern.FindStringSubmatch(url)
	if len(sub) != ExpectedSubmatches {
		msg := "failed to parse Slack message's PR comment URL"
		logger.Error(ctx, msg, nil, slog.String("slack_ids", ids), slog.String("comment_url", url))
		return nil, fmt.Errorf("invalid Bitbucket PR URL: %s", url)
	}

	return sub, nil
}

func thrippyLinkID(ctx workflow.Context, userID, channelID string) (string, error) {
	if len(userID) > 0 && userID[0] == 'B' {
		return "", nil // Slack bot, not a real user.
	}

	user, err := data.SelectUserBySlackID(userID)
	if err != nil {
		logger.Error(ctx, "failed to load user by Slack ID", err, slog.String("user_id", userID))
		return "", err
	}

	if !data.IsOptedIn(user) {
		msg := ":warning: Cannot mirror this in Bitbucket, you need to run this slash command: `/revchat opt-in`"
		return "", PostEphemeralMessage(ctx, channelID, userID, msg)
	}

	return user.ThrippyLink, nil
}
