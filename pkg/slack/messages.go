package slack

import (
	"errors"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
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

	Team        string `json:"team,omitempty"`
	Channel     string `json:"channel"`
	ChannelType string `json:"channel_type"`
	User        string `json:"user"`

	Text string `json:"text"`
	// Blocks []map[string]any `json:"blocks"` // Text is enough.

	Edited          *edited       `json:"edited"`                     // Subtype = "message_changed".
	Message         *MessageEvent `json:"message,omitempty"`          // Subtype = "message_changed".
	PreviousMessage *MessageEvent `json:"previous_message,omitempty"` // Subtype = "message_changed" or "message_deleted".
	Root            *MessageEvent `json:"root,omitempty"`             // Subtype = "thread_broadcast".

	DeletedTS    string `json:"deleted_ts,omitempty"`     // Subtype = "message_deleted".
	ThreadTS     string `json:"thread_ts,omitempty"`      // Reply, or subtype = "thread_broadcast".
	ParentUserID string `json:"parent_user_id,omitempty"` // Subtype = "thread_broadcast".

	ClientMsgID string `json:"client_msg_id,omitempty"`
	EventTS     string `json:"event_ts"`
	TS          string `json:"ts"`
}

type edited struct {
	User string `json:"user"`
	TS   string `json:"ts"`
}

// https://docs.slack.dev/reference/events/message/
func (c Config) messageWorkflow(ctx workflow.Context, event messageEventWrapper) error {
	// First, determine who triggered this event.
	user := event.InnerEvent.User
	if msg := event.InnerEvent.Message; user == "" && msg != nil {
		user = msg.User // Thread broadcast.
		if msg.Edited != nil {
			user = msg.Edited.User // Edited message.
		}
	}
	if msg := event.InnerEvent.PreviousMessage; user == "" && msg != nil {
		user = msg.User // Deleted reply.
		if msg.Edited != nil {
			user = msg.Edited.User // Deleted message.
		}
	}
	if user == "" {
		msg := "could not determine who triggered a Slack message event"
		log.Error(ctx, msg)
		return errors.New(msg)
	}

	if isSelfTriggeredEvent(ctx, event.Authorizations, user) {
		return nil
	}

	log.Warn(ctx, "message event", "event", event)
	return nil
}
