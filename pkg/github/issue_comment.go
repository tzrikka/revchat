package github

import (
	"errors"

	"go.temporal.io/sdk/workflow"
)

// https://docs.github.com/en/webhooks/webhook-events-and-payloads#issue_comment
func (c Config) issueCommentWorkflow(ctx workflow.Context, event IssueCommentEvent) error {
	switch event.Action {
	case "created":
		return c.issueCommentCreated()
	case "edited":
		return c.issueCommentEdited()
	case "deleted":
		return c.issueCommentDeleted()

	default:
		workflow.GetLogger(ctx).Error("unrecognized GitHub issue comment event action", "action", event.Action)
		return errors.New("unrecognized GitHub issue comment event action: " + event.Action)
	}
}

// A comment on an issue or pull request was created.
func (c Config) issueCommentCreated() error {
	return nil
}

// A comment on an issue or pull request was edited.
func (c Config) issueCommentEdited() error {
	return nil
}

// A comment on an issue or pull request was deleted.
func (c Config) issueCommentDeleted() error {
	return nil
}
