package slack

import (
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
)

// https://docs.slack.dev/reference/events/reaction_added/
func (c *Config) reactionAddedWorkflow(ctx workflow.Context, event reactionEventWrapper) error {
	if selfTriggeredEvent(ctx, event.Authorizations, event.InnerEvent.User) {
		return nil
	}

	log.Warn(ctx, "reaction added event", "event", event)
	return nil
}

// https://docs.slack.dev/reference/events/reaction_removed/
func (c *Config) reactionRemovedWorkflow(ctx workflow.Context, event reactionEventWrapper) error {
	if selfTriggeredEvent(ctx, event.Authorizations, event.InnerEvent.User) {
		return nil
	}

	log.Warn(ctx, "reaction removed event", "event", event)
	return nil
}
