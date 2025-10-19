package github

import (
	"errors"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
)

// https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request_review_thread
func (c Config) prReviewThreadWorkflow(ctx workflow.Context, event PullRequestReviewThreadEvent) error {
	switch event.Action {
	case "resolved":
		return c.reviewThreadResolved(ctx)
	case "unresolved":
		return c.reviewThreadUnresolved(ctx)

	default:
		log.Error(ctx, "unrecognized GitHub PR review thread event action", "action", event.Action)
		return errors.New("unrecognized GitHub PR review thread event action: " + event.Action)
	}
}

// A comment thread on a pull request was marked as resolved.
func (c Config) reviewThreadResolved(ctx workflow.Context) error {
	log.Warn(ctx, "GitHub PR review thread resolved - event handler not implemented yet")
	return nil
}

// A previously resolved comment thread on a pull request was marked as unresolved.
func (c Config) reviewThreadUnresolved(ctx workflow.Context) error {
	log.Warn(ctx, "GitHub PR review thread unresolved - event handler not implemented yet")
	return nil
}
