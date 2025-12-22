package github

import (
	"errors"
	"log/slog"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
)

// https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request_review_comment
// https://docs.github.com/pull-requests/collaborating-with-pull-requests/reviewing-changes-in-pull-requests/commenting-on-a-pull-request#adding-line-comments-to-a-pull-request
func (c Config) prReviewCommentWorkflow(ctx workflow.Context, event PullRequestReviewCommentEvent) error {
	switch event.Action {
	case "created":
		return c.reviewCommentCreated(ctx)
	case "edited":
		return c.reviewCommentEdited(ctx)
	case "deleted":
		return c.reviewCommentDeleted(ctx)

	default:
		logger.From(ctx).Error("unrecognized GitHub PR review comment event action", slog.String("action", event.Action))
		return errors.New("unrecognized GitHub PR review comment event action: " + event.Action)
	}
}

// A comment on a pull request diff was created.
func (c Config) reviewCommentCreated(ctx workflow.Context) error {
	logger.From(ctx).Warn("GitHub PR review comment created - event handler not implemented yet")
	return nil
}

// The content of a comment on a pull request diff was changed.
func (c Config) reviewCommentEdited(ctx workflow.Context) error {
	logger.From(ctx).Warn("GitHub PR review comment edited - event handler not implemented yet")
	return nil
}

// A comment on a pull request diff was deleted.
func (c Config) reviewCommentDeleted(ctx workflow.Context) error {
	logger.From(ctx).Warn("GitHub PR review comment deleted - event handler not implemented yet")
	return nil
}
