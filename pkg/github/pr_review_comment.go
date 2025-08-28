package github

import (
	"errors"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
)

// https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request_review_comment
// https://docs.github.com/pull-requests/collaborating-with-pull-requests/reviewing-changes-in-pull-requests/commenting-on-a-pull-request#adding-line-comments-to-a-pull-request
func (c Config) prReviewCommentWorkflow(ctx workflow.Context, event PullRequestReviewCommentEvent) error {
	switch event.Action {
	case "created":
		return c.reviewCommentCreated()
	case "edited":
		return c.reviewCommentEdited()
	case "deleted":
		return c.reviewCommentDeleted()

	default:
		log.Error(ctx, "unrecognized GitHub PR review comment event action", "action", event.Action)
		return errors.New("unrecognized GitHub PR review comment event action: " + event.Action)
	}
}

// A comment on a pull request diff was created.
func (c Config) reviewCommentCreated() error {
	return nil
}

// The content of a comment on a pull request diff was changed.
func (c Config) reviewCommentEdited() error {
	return nil
}

// A comment on a pull request diff was deleted.
func (c Config) reviewCommentDeleted() error {
	return nil
}
