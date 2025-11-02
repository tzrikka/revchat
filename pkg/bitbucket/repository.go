package bitbucket

import (
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
)

func commitCommentCreatedWorkflow(ctx workflow.Context, event RepositoryEvent) error {
	log.Warn(ctx, "commit comment created event - not implemented yet", "event", event)
	return nil
}

func commitStatusCreatedWorkflow(ctx workflow.Context, event RepositoryEvent) error {
	log.Warn(ctx, "commit status created event - not implemented yet", "event", event)
	return nil
}

func commitStatusUpdatedWorkflow(ctx workflow.Context, event RepositoryEvent) error {
	log.Warn(ctx, "commit status updated event - not implemented yet", "event", event)
	return nil
}
