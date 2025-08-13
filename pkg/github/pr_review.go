package github

import (
	"go.temporal.io/sdk/workflow"
)

// https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request_review
// https://docs.github.com/pull-requests/collaborating-with-pull-requests/reviewing-changes-in-pull-requests/about-pull-request-reviews
func (g GitHub) handlePullRequestReviewEvent(ctx workflow.Context, event PullRequestReviewEvent) {
	switch event.Action {
	case "submitted":
		g.reviewSubmitted()
	case "edited":
		g.reviewEdited()
	case "dismissed":
		g.reviewDismissed()

	default:
		workflow.GetLogger(ctx).Error("unrecognized GitHub PR review event action", "action", event.Action)
	}
}

// A review on a pull request was submitted. This is interesting when
// the review state is "approved", and/or the review body isn't empty.
func (g GitHub) reviewSubmitted() {
}

// The body comment on a pull request review was edited.
func (g GitHub) reviewEdited() {
}

// A review on a pull request was dismissed.
func (g GitHub) reviewDismissed() {
}
