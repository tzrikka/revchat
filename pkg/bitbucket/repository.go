package bitbucket

import (
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
)

func (c Config) commitCommentCreatedWorkflow(ctx workflow.Context, event RepositoryEvent) error {
	log.Warn(ctx, "commit comment created event - not implemented yet", "event", event)
	return nil
}

func (c Config) commitStatusCreatedWorkflow(ctx workflow.Context, event RepositoryEvent) error {
	log.Warn(ctx, "commit status created event - not implemented yet", "event", event)
	return nil
}

func (c Config) commitStatusUpdatedWorkflow(ctx workflow.Context, event RepositoryEvent) error {
	log.Warn(ctx, "commit status updated event - not implemented yet", "event", event)
	return nil
}
