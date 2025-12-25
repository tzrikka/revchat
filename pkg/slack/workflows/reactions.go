package workflows

import (
	"log/slog"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
)

// ReactionAddedWorkflow mirrors the addition of a reaction to a Slack message in
// the corresponding PR comment: https://docs.slack.dev/reference/events/reaction_added/
func ReactionAddedWorkflow(ctx workflow.Context, event reactionEventWrapper) error {
	if selfTriggeredEvent(ctx, event.Authorizations, event.InnerEvent.User) {
		return nil
	}

	logger.From(ctx).Debug("Slack reaction added event - not implemented yet", slog.Any("event", event))
	return nil
}

// ReactionRemovedWorkflow mirrors the removal of a reaction from a Slack message in
// the corresponding PR comment: https://docs.slack.dev/reference/events/reaction_removed/
func ReactionRemovedWorkflow(ctx workflow.Context, event reactionEventWrapper) error {
	if selfTriggeredEvent(ctx, event.Authorizations, event.InnerEvent.User) {
		return nil
	}

	logger.From(ctx).Debug("Slack reaction removed event - not implemented yet", slog.Any("event", event))
	return nil
}
