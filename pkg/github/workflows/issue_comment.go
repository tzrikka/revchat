package workflows

import (
	"errors"
	"log/slog"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/github"
)

// IssueCommentWorkflow is an entrypoint to mirror all GitHub issue comment events in the PR's
// Slack channel: https://docs.github.com/en/webhooks/webhook-events-and-payloads#issue_comment
func IssueCommentWorkflow(ctx workflow.Context, event github.IssueCommentEvent) error {
	switch event.Action {
	case "created":
		return issueCommentCreated(ctx)
	case "edited":
		return issueCommentEdited(ctx)
	case "deleted":
		return issueCommentDeleted(ctx)
	default:
		logger.From(ctx).Error("unrecognized GitHub issue comment event action", slog.String("action", event.Action))
		return errors.New("unrecognized GitHub issue comment event action: " + event.Action)
	}
}

// A comment on an issue or pull request was created.
func issueCommentCreated(ctx workflow.Context) error {
	logger.From(ctx).Warn("GitHub issue comment created event - not implemented yet")
	return nil
}

// A comment on an issue or pull request was edited.
func issueCommentEdited(ctx workflow.Context) error {
	logger.From(ctx).Warn("GitHub issue comment edited event - not implemented yet")
	return nil
}

// A comment on an issue or pull request was deleted.
func issueCommentDeleted(ctx workflow.Context) error {
	logger.From(ctx).Warn("GitHub issue comment deleted event - not implemented yet")
	return nil
}
