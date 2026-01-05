package workflows

import (
	"errors"
	"log/slog"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/github"
)

// PullRequestReviewThreadWorkflow is an entrypoint to mirror all GitHub pull request review thread (i.e. comment resolution)
// events in the PR's Slack channel: https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request_review_thread
func PullRequestReviewThreadWorkflow(ctx workflow.Context, event github.PullRequestReviewThreadEvent) error {
	switch event.Action {
	case "resolved":
		return reviewThreadResolved(ctx)
	case "unresolved":
		return reviewThreadUnresolved(ctx)
	default:
		logger.From(ctx).Error("unrecognized GitHub PR review thread event action", slog.String("action", event.Action))
		return errors.New("unrecognized GitHub PR review thread event action: " + event.Action)
	}
}

// A comment thread on a pull request was marked as resolved.
func reviewThreadResolved(ctx workflow.Context) error {
	logger.From(ctx).Warn("GitHub PR review thread resolved - event handler not implemented yet")
	return nil
}

// A previously resolved comment thread on a pull request was marked as unresolved.
func reviewThreadUnresolved(ctx workflow.Context) error {
	logger.From(ctx).Warn("GitHub PR review thread unresolved - event handler not implemented yet")
	return nil
}
