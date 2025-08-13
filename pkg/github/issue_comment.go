package github

import (
	"go.temporal.io/sdk/workflow"
)

// https://docs.github.com/en/webhooks/webhook-events-and-payloads#issue_comment
func (g GitHub) handleIssueCommentEvent(ctx workflow.Context, event IssueCommentEvent) {
	switch event.Action {
	case "created":
		g.issueCommentCreated()
	case "edited":
		g.issueCommentEdited()
	case "deleted":
		g.issueCommentDeleted()

	default:
		workflow.GetLogger(ctx).Error("unrecognized GitHub issue comment event action", "action", event.Action)
	}
}

// A comment on an issue or pull request was created.
func (g GitHub) issueCommentCreated() {
}

// A comment on an issue or pull request was edited.
func (g GitHub) issueCommentEdited() {
}

// A comment on an issue or pull request was deleted.
func (g GitHub) issueCommentDeleted() {
}
