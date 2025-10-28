package slack

import (
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
)

type reactionEventWrapper struct {
	eventWrapper

	InnerEvent ReactionEvent `json:"event"`
}

// https://docs.slack.dev/reference/events/reaction_added/
// https://docs.slack.dev/reference/events/reaction_removed/
type ReactionEvent struct {
	// Type string `json:"type"`

	User     string `json:"user"`
	Reaction string `json:"reaction"`

	Item struct {
		Type        string `json:"type"`
		Channel     string `json:"channel,omitempty"`
		TS          string `json:"ts,omitempty"`
		File        string `json:"file,omitempty"`
		FileComment string `json:"file_comment,omitempty"`
	} `json:"item"`

	ItemUser string `json:"item_user,omitempty"`

	EventTS string `json:"event_ts"`
}

// https://docs.slack.dev/reference/events/reaction_added/
func (c *Config) reactionAddedWorkflow(ctx workflow.Context, event reactionEventWrapper) error {
	if isSelfTriggeredEvent(ctx, event.Authorizations, event.InnerEvent.User) {
		return nil
	}

	log.Warn(ctx, "reaction added event", "event", event)
	return nil
}

// https://docs.slack.dev/reference/events/reaction_removed/
func (c *Config) reactionRemovedWorkflow(ctx workflow.Context, event reactionEventWrapper) error {
	if isSelfTriggeredEvent(ctx, event.Authorizations, event.InnerEvent.User) {
		return nil
	}

	log.Warn(ctx, "reaction removed event", "event", event)
	return nil
}
