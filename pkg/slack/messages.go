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
	userID := c.extractUserID(ctx, &event.InnerEvent)
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
	case "message_deleted":
		return c.deleteMessage(ctx, event.InnerEvent)
	}

	log.Warn(ctx, "unhandled Slack message event", "event", event.InnerEvent)
	return nil
}

// extractUserID determines the user ID of the user/app that triggered a Slack message event.
// This ID is located in different places depending on the event subtype and the user type.
func (c Config) extractUserID(ctx workflow.Context, msg *MessageEvent) string {
	if msg == nil {
		return ""
	}

	if msg.Edited != nil && msg.Edited.User != "" {
		return msg.Edited.User
	}

	if msg.BotID != "" {
		return c.convertBotIDToUserID(ctx, msg.BotID)
	}

	if user := c.extractUserID(ctx, msg.Message); user != "" {
		return user
	}

	if user := c.extractUserID(ctx, msg.PreviousMessage); user != "" {
		return user
	}

	return msg.User
}

// convertBotIDToUserID uses a cache to convert Slack bot IDs to a user IDs.
func (c Config) convertBotIDToUserID(ctx workflow.Context, botID string) string {
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
	return c.addMessageInBitbucket(ctx, event)
}

func (c Config) deleteMessage(ctx workflow.Context, event MessageEvent) error {
	return c.deleteMessageInBitbucket(ctx, event)
}

// addMessageInBitbucket mirrors in Bitbucket the creation of a Slack message/reply/broadcast.
func (c Config) addMessageInBitbucket(ctx workflow.Context, event MessageEvent) error {
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

// deleteMessageInBitbucket mirrors in Bitbucket the deletion of a Slack message/reply/broadcast.
func (c Config) deleteMessageInBitbucket(ctx workflow.Context, event MessageEvent) error {
	prev := event.PreviousMessage
	if prev.ThreadTS == "" {
		log.Warn(ctx, "message deleted", "channel", event.Channel, "ts", event.DeletedTS, "text", prev.Text)
	} else {
		log.Warn(ctx, "reply deleted", "channel", event.Channel, "thread_ts", prev.ThreadTS, "ts", event.DeletedTS, "text", prev.Text)
	}

	return nil
}

var commentURLPattern = regexp.MustCompile(`[a-z]/(.+?)/(.+?)/pull-requests/(\d+)(.+comment-(\d+))?`)

const Submatches = 6

func createPRComment(ctx workflow.Context, url, msg, slackID string) error {
	sub := commentURLPattern.FindStringSubmatch(url)
	if len(sub) != Submatches {
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
