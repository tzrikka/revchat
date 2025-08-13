package github

import (
	"go.temporal.io/sdk/workflow"
)

// https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request_review_thread
func (g GitHub) handlePullRequestReviewThreadEvent(ctx workflow.Context, event PullRequestReviewThreadEvent) {
	switch event.Action {
	case "resolved":
		g.reviewThreadResolved()
	case "deleted":
		g.reviewThreadUnresolved()

	default:
		workflow.GetLogger(ctx).Error("unrecognized GitHub PR review thread event action", "action", event.Action)
	}
}

// A comment thread on a pull request was marked as resolved.
func (g GitHub) reviewThreadResolved() {
}

// A previously resolved comment thread on a pull request was marked as unresolved.
func (g GitHub) reviewThreadUnresolved() {
}
