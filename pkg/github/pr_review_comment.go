package github

import (
	"go.temporal.io/sdk/workflow"
)

// https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request_review_comment
// https://docs.github.com/pull-requests/collaborating-with-pull-requests/reviewing-changes-in-pull-requests/commenting-on-a-pull-request#adding-line-comments-to-a-pull-request
func (g GitHub) handlePullRequestReviewCommentEvent(ctx workflow.Context, event PullRequestReviewCommentEvent) {
	switch event.Action {
	case "created":
		g.reviewCommentCreated()
	case "edited":
		g.reviewCommentEdited()
	case "deleted":
		g.reviewCommentDeleted()

	default:
		workflow.GetLogger(ctx).Error("unrecognized GitHub PR review comment event action", "action", event.Action)
	}
}

// A comment on a pull request diff was created.
func (g GitHub) reviewCommentCreated() {
}

// The content of a comment on a pull request diff was changed.
func (g GitHub) reviewCommentEdited() {
}

// A comment on a pull request diff was deleted.
func (g GitHub) reviewCommentDeleted() {
}
