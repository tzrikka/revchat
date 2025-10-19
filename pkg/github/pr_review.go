package github

import (
	"errors"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
)

// https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request_review
// https://docs.github.com/pull-requests/collaborating-with-pull-requests/reviewing-changes-in-pull-requests/about-pull-request-reviews
func (c Config) prReviewWorkflow(ctx workflow.Context, event PullRequestReviewEvent) error {
	switch event.Action {
	case "submitted":
		return c.reviewSubmitted(ctx)
	case "edited":
		return c.reviewEdited(ctx)
	case "dismissed":
		return c.reviewDismissed(ctx)

	default:
		log.Error(ctx, "unrecognized GitHub PR review event action", "action", event.Action)
		return errors.New("unrecognized GitHub PR review event action: " + event.Action)
	}
}

// A review on a pull request was submitted. This is interesting when
// the review state is "approved", and/or the review body isn't empty.
func (c Config) reviewSubmitted(ctx workflow.Context) error {
	log.Warn(ctx, "GitHub PR review submitted - event handler not implemented yet")
	return nil
}

// The body comment on a pull request review was edited.
func (c Config) reviewEdited(ctx workflow.Context) error {
	log.Warn(ctx, "GitHub PR review edited - event handler not implemented yet")
	return nil
}

// A review on a pull request was dismissed.
func (c Config) reviewDismissed(ctx workflow.Context) error {
	log.Warn(ctx, "GitHub PR review dismissed - event handler not implemented yet")
	return nil
}
