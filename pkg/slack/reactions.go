package slack

import (
	"log/slog"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
)

// https://docs.slack.dev/reference/events/reaction_added/
func (c *Config) reactionAddedWorkflow(ctx workflow.Context, event reactionEventWrapper) error {
	if selfTriggeredEvent(ctx, event.Authorizations, event.InnerEvent.User) {
		return nil
	}

	logger.Warn(ctx, "reaction added event", slog.Any("event", event))
	return nil
}

// https://docs.slack.dev/reference/events/reaction_removed/
func (c *Config) reactionRemovedWorkflow(ctx workflow.Context, event reactionEventWrapper) error {
	if selfTriggeredEvent(ctx, event.Authorizations, event.InnerEvent.User) {
		return nil
	}

	logger.Warn(ctx, "reaction removed event", slog.Any("event", event))
	return nil
}
