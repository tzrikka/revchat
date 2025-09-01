package slack

import (
	"errors"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/revchat/pkg/data"
)

type messageEventWrapper struct {
	eventWrapper

	InnerEvent MessageEvent `json:"event"`
}

// https://docs.slack.dev/reference/events/message/
type MessageEvent struct {
	// Type string `json:"type"` // Always "message".

	Subtype string `json:"subtype,omitempty"`
	Hidden  bool   `json:"hidden,omitempty"` // For example, when subtype = "message_changed" or "message_deleted".

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

	DeletedTS    string `json:"deleted_ts,omitempty"`     // Subtype = "message_deleted".
	ThreadTS     string `json:"thread_ts,omitempty"`      // Reply, or subtype = "thread_broadcast".
	ParentUserID string `json:"parent_user_id,omitempty"` // Subtype = "thread_broadcast".

	ClientMsgID string `json:"client_msg_id,omitempty"`
	EventTS     string `json:"event_ts,omitempty"`
	TS          string `json:"ts"`
}

type edited struct {
	User string `json:"user"`
	TS   string `json:"ts"`
}

// https://docs.slack.dev/reference/events/message/
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

	log.Warn(ctx, "message event", "event", event)
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
		log.Error(ctx, "failed to read Slack bot's user ID", "bot_id", botID, "error", err)
		return ""
	}

	if userID != "" {
		return userID
	}

	bi, err := BotsInfoActivity(ctx, c.Cmd, botID, "")
	if err != nil {
		log.Error(ctx, "failed to retrieve bot info from Slack", "bot_id", botID, "error", err)
		return ""
	}

	log.Debug(ctx, "retrieved bot info from Slack", "bot_id", botID, "user_id", bi.UserID, "name", bi.Name)
	if err := data.SetSlackBotUserID(botID, bi.UserID); err != nil {
		log.Error(ctx, "failed to cache Slack bot's user ID", "bot_id", botID, "error", err)
	}
	return bi.UserID
}
