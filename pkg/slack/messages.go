package slack

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/markdown"
	"github.com/tzrikka/timpani-api/pkg/bitbucket"
	"github.com/tzrikka/timpani-api/pkg/slack"
)

type messageEventWrapper struct {
	eventWrapper

	InnerEvent MessageEvent `json:"event"`
}

// https://docs.slack.dev/reference/events/message
type MessageEvent struct {
	// Type string `json:"type"` // Always "message".

	Subtype string `json:"subtype,omitempty"`

	User     string `json:"user,omitempty"`
	BotID    string `json:"bot_id,omitempty"`
	Username string `json:"username,omitempty"` // Customized display name, when bot_id is present.

	Team        string `json:"team,omitempty"`
	Channel     string `json:"channel,omitempty"`
	ChannelType string `json:"channel_type,omitempty"`

	Text string `json:"text,omitempty"`
	// Blocks []map[string]any `json:"blocks"` // Text is enough.

	Edited          *edited       `json:"edited,omitempty"`           // Subtype = "message_changed".
	Message         *MessageEvent `json:"message,omitempty"`          // Subtype = "message_changed".
	PreviousMessage *MessageEvent `json:"previous_message,omitempty"` // Subtype = "message_changed" or "message_deleted".
	Root            *MessageEvent `json:"root,omitempty"`             // Subtype = "thread_broadcast".

	TS        string `json:"ts"`
	EventTS   string `json:"event_ts,omitempty"`
	DeletedTS string `json:"deleted_ts,omitempty"` // Subtype = "message_deleted".
	ThreadTS  string `json:"thread_ts,omitempty"`  // Reply, or subtype = "thread_broadcast".

	ParentUserID string `json:"parent_user_id,omitempty"` // Subtype = "thread_broadcast".
	ClientMsgID  string `json:"client_msg_id,omitempty"`
}

type edited struct {
	User string `json:"user"`
	TS   string `json:"ts"`
}

// https://docs.slack.dev/reference/events/message
func (c Config) messageWorkflow(ctx workflow.Context, event messageEventWrapper) error {
	userID := extractUserID(ctx, &event.InnerEvent)
	if userID == "" {
		msg := "could not determine who triggered a Slack message event"
		log.Error(ctx, msg)
		return errors.New(msg)
	}

	if isSelfTriggeredEvent(ctx, event.Authorizations, userID) {
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
		return c.addMessage(ctx, event.InnerEvent)
	case "message_changed":
		return c.changeMessage(ctx, event.InnerEvent)
	case "message_deleted":
		return c.deleteMessage(ctx, event.InnerEvent)
	}

	log.Warn(ctx, "unhandled Slack message event", "event", event.InnerEvent)
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
		log.Error(ctx, "failed to load Slack bot's user ID", "bot_id", botID, "error", err)
		return ""
	}

	if userID != "" {
		return userID
	}

	bot, err := slack.BotsInfoActivity(ctx, botID)
	if err != nil {
		log.Error(ctx, "failed to retrieve bot info from Slack", "bot_id", botID, "error", err)
		return ""
	}

	log.Debug(ctx, "retrieved bot info from Slack", "bot_id", botID, "user_id", bot.UserID, "name", bot.Name)
	if err := data.SetSlackBotUserID(botID, bot.UserID); err != nil {
		log.Error(ctx, "failed to save Slack bot's user ID", "bot_id", botID, "error", err)
	}

	return bot.UserID
}

func (c Config) addMessage(ctx workflow.Context, event MessageEvent) error {
	switch {
	case c.bitbucketWorkspace != "":
		return addMessageInBitbucket(ctx, event)
	default:
		log.Error(ctx, "neither Bitbucket nor GitHub are configured")
		return errors.New("neither Bitbucket nor GitHub are configured")
	}
}

func (c Config) changeMessage(ctx workflow.Context, event MessageEvent) error {
	switch {
	case c.bitbucketWorkspace != "":
		return editMessageInBitbucket(ctx, event)
	default:
		log.Error(ctx, "neither Bitbucket nor GitHub are configured")
		return errors.New("neither Bitbucket nor GitHub are configured")
	}
}

func (c Config) deleteMessage(ctx workflow.Context, event MessageEvent) error {
	switch {
	case c.bitbucketWorkspace != "":
		return deleteMessageInBitbucket(ctx, event)
	default:
		log.Error(ctx, "neither Bitbucket nor GitHub are configured")
		return errors.New("neither Bitbucket nor GitHub are configured")
	}
}

// addMessageInBitbucket mirrors in Bitbucket the creation of a Slack message/reply/broadcast.
func addMessageInBitbucket(ctx workflow.Context, event MessageEvent) error {
	if event.Subtype == "bot_message" {
		log.Error(ctx, "unexpected bot message", "bot_id", event.BotID, "username", event.Username)
	}

	id := event.Channel
	if event.ThreadTS != "" {
		id = fmt.Sprintf("%s/%s", id, event.ThreadTS)
	}

	url, err := data.SwitchURLAndID(id)
	if err != nil {
		msg := "failed to retrieve Slack message's PR comment URL"
		log.Error(ctx, msg, "error", err, "message_subtype", event.Subtype, "channel_id", event.Channel, "message_ts", event.ThreadTS)
		return err
	}
	if url == "" {
		log.Debug(ctx, "Slack message's PR comment URL is empty", "msg_subtype", event.Subtype, "channel_id", event.Channel, "msg_ts", event.ThreadTS)
		return fmt.Errorf("no associated URL for Slack channel %q and message timestamp %q", event.Channel, event.ThreadTS)
	}

	return createPRComment(ctx, url, event.Text, fmt.Sprintf("%s/%s", id, event.TS))
}

var commentURLPattern = regexp.MustCompile(`[a-z]/(.+?)/(.+?)/pull-requests/(\d+)(.+comment-(\d+))?`)

const ExpectedSubmatches = 6

// editMessageInBitbucket mirrors in Bitbucket the editing of a Slack message/reply/broadcast.
func editMessageInBitbucket(ctx workflow.Context, event MessageEvent) error {
	log.Warn(ctx, "message edit event", "event", event)
	return nil
}

// deleteMessageInBitbucket mirrors in Bitbucket the deletion of a Slack message/reply/broadcast.
func deleteMessageInBitbucket(ctx workflow.Context, event MessageEvent) error {
	id := fmt.Sprintf("%s/%s", event.Channel, event.DeletedTS)
	if tts := event.PreviousMessage.ThreadTS; tts != "" {
		id = fmt.Sprintf("%s/%s/%s", event.Channel, tts, event.DeletedTS)
	}

	url, err := data.SwitchURLAndID(id)
	if err != nil {
		msg := "failed to retrieve Slack message's PR comment URL"
		log.Error(ctx, msg, "error", err, "message_subtype", event.Subtype, "id", id, "url", url)
		return err
	}

	sub := commentURLPattern.FindStringSubmatch(url)
	if len(sub) != ExpectedSubmatches {
		msg := "failed to parse Slack message's PR comment URL"
		log.Error(ctx, msg, "message_subtype", event.Subtype, "id", id, "url", url)
		return fmt.Errorf("invalid Bitbucket PR URL: %s", url)
	}

	if err := data.DeleteURLAndIDMapping(url); err != nil {
		log.Error(ctx, "failed to delete URL/Slack mappings", "error", err, "comment_url", url)
		// Don't abort - we still want to attempt to delete the PR comment.
	}

	err = bitbucket.PullRequestsDeleteCommentActivity(ctx, bitbucket.PullRequestsDeleteCommentRequest{
		Workspace:     sub[1],
		RepoSlug:      sub[2],
		PullRequestID: sub[3],
		CommentID:     sub[5],
	})
	if err != nil {
		log.Error(ctx, "failed to delete Bitbucket PR comment", "error", err, "id", id, "url", url)
		return err
	}

	return nil
}

func createPRComment(ctx workflow.Context, url, msg, slackID string) error {
	sub := commentURLPattern.FindStringSubmatch(url)
	if len(sub) != ExpectedSubmatches {
		log.Error(ctx, "failed to parse Slack message's PR comment URL", "url", url)
		return fmt.Errorf("invalid Bitbucket PR URL: %s", url)
	}

	msg = markdown.SlackToBitbucket(ctx, msg) + "\n\n[This comment was created by RevChat]: #"

	resp, err := bitbucket.PullRequestsCreateCommentActivity(ctx, bitbucket.PullRequestsCreateCommentRequest{
		Workspace:     sub[1],
		RepoSlug:      sub[2],
		PullRequestID: sub[3],
		Markdown:      msg,
		ParentID:      sub[5], // Optional.
	})
	if err != nil {
		log.Error(ctx, "failed to create Bitbucket PR comment", "error", err, "url", url, "slack_id", slackID)
		return fmt.Errorf("failed to create Bitbucket PR comment: %w", err)
	}

	url = resp.Links["html"].HRef
	if err := data.MapURLAndID(url, slackID); err != nil {
		log.Error(ctx, "failed to save PR comment URL / Slack IDs mapping", "error", err, "url", url, "slack_id", slackID)
		// Don't return the error - the message is already created in Bitbucket, so
		// we don't want to retry and post it again, even though this is problematic.
	}

	return nil
}
