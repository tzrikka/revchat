package workflows

import (
	"errors"
	"log/slog"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/github"
)

// PullRequestReviewCommentWorkflow is an entrypoint to mirror all GitHub pull request review comment events in the
// PR's Slack channel: https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request_review_comment
func PullRequestReviewCommentWorkflow(ctx workflow.Context, event github.PullRequestReviewCommentEvent) error {
	switch event.Action {
	case "created":
		return prReviewCommentCreated(ctx)
	case "edited":
		return prReviewCommentEdited(ctx)
	case "deleted":
		return prReviewCommentDeleted(ctx)
	default:
		logger.From(ctx).Error("unrecognized GitHub PR review comment event action", slog.String("action", event.Action))
		return errors.New("unrecognized GitHub PR review comment event action: " + event.Action)
	}
}

// A comment on a pull request diff was created.
func prReviewCommentCreated(ctx workflow.Context) error {
	logger.From(ctx).Warn("GitHub PR review comment created event - not implemented yet")
	return nil
}

// The content of a comment on a pull request diff was changed.
func prReviewCommentEdited(ctx workflow.Context) error {
	logger.From(ctx).Warn("GitHub PR review comment edited event - not implemented yet")
	return nil
}

// A comment on a pull request diff was deleted.
func prReviewCommentDeleted(ctx workflow.Context) error {
	logger.From(ctx).Warn("GitHub PR review comment deleted event - not implemented yet")
	return nil
}
