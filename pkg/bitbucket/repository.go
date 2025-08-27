package bitbucket

import (
	"go.temporal.io/sdk/workflow"
)

func (c Config) commitCommentCreatedWorkflow(ctx workflow.Context, event RepositoryEvent) error {
	return nil
}

func (c Config) buildStatusCreatedWorkflow(ctx workflow.Context, event RepositoryEvent) error {
	return nil
}

func (c Config) buildStatusUpdatedWorkflow(ctx workflow.Context, event RepositoryEvent) error {
	return nil
}
