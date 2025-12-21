package github

import (
	"errors"
	"log/slog"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
)

// https://docs.github.com/en/webhooks/webhook-events-and-payloads#issue_comment
func (c Config) issueCommentWorkflow(ctx workflow.Context, event IssueCommentEvent) error {
	switch event.Action {
	case "created":
		return c.issueCommentCreated(ctx)
	case "edited":
		return c.issueCommentEdited(ctx)
	case "deleted":
		return c.issueCommentDeleted(ctx)

	default:
		logger.Error(ctx, "unrecognized GitHub issue comment event action", nil, slog.String("action", event.Action))
		return errors.New("unrecognized GitHub issue comment event action: " + event.Action)
	}
}

// A comment on an issue or pull request was created.
func (c Config) issueCommentCreated(ctx workflow.Context) error {
	logger.Warn(ctx, "GitHub issue comment created - event handler not implemented yet")
	return nil
}

// A comment on an issue or pull request was edited.
func (c Config) issueCommentEdited(ctx workflow.Context) error {
	logger.Warn(ctx, "GitHub issue comment edited - event handler not implemented yet")
	return nil
}

// A comment on an issue or pull request was deleted.
func (c Config) issueCommentDeleted(ctx workflow.Context) error {
	logger.Warn(ctx, "GitHub issue comment deleted - event handler not implemented yet")
	return nil
}
