package workflows

import (
	"errors"
	"log/slog"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/github"
)

// PullRequestReviewWorkflow is an entrypoint to mirror all GitHub pull request review events in the PR's
// Slack channel: https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request_review
func PullRequestReviewWorkflow(ctx workflow.Context, event github.PullRequestReviewEvent) error {
	switch event.Action {
	case "submitted":
		return prReviewSubmitted(ctx)
	case "edited":
		return prReviewEdited(ctx)
	case "dismissed":
		return prReviewDismissed(ctx)
	default:
		logger.From(ctx).Error("unrecognized GitHub PR review event action", slog.String("action", event.Action))
		return errors.New("unrecognized GitHub PR review event action: " + event.Action)
	}
}

// A review on a pull request was submitted. This is interesting when
// the review state is "approved", and/or the review body isn't empty.
func prReviewSubmitted(ctx workflow.Context) error {
	logger.From(ctx).Warn("GitHub PR review submitted event - not implemented yet")
	return nil
}

// The body comment on a pull request review was edited.
func prReviewEdited(ctx workflow.Context) error {
	logger.From(ctx).Warn("GitHub PR review edited event - not implemented yet")
	return nil
}

// A review on a pull request was dismissed.
func prReviewDismissed(ctx workflow.Context) error {
	logger.From(ctx).Warn("GitHub PR review dismissed event - not implemented yet")
	return nil
}
