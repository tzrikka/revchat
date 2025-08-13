package bitbucket

import (
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/slack"
	"github.com/tzrikka/revchat/pkg/users"
)

// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Pull-request-events
func (b Bitbucket) handlePullRequestEvent(ctx workflow.Context, event PullRequestEvent) {
	switch event.Type {
	case "created":
		b.prCreated(ctx, event)
	case "updated":
		b.prUpdated(ctx, event)

	case "approved", "unapproved":
		b.prReviewed(ctx, event)
	case "changes_request_created", "changes_request_removed":
		b.prReviewed(ctx, event)

	case "fulfilled", "rejected":
		b.prClosed(ctx, event)

	case "comment_created":
		b.prCommentCreated()
	case "comment_updated":
		b.prCommentUpdated()
	case "comment_deleted":
		b.prCommentDeleted()
	case "comment_resolved":
		b.prCommentResolved()
	case "comment_reopened":
		b.prCommentReopened()

	default:
		workflow.GetLogger(ctx).Error("unrecognized Bitbucket PR event type", "event_type", event.Type)
	}
}

// A new PR was created (or marked as ready for review - see [Bitbucket.prUpdated]).
func (b Bitbucket) prCreated(ctx workflow.Context, event PullRequestEvent) {
	// Ignore drafts until they're marked as ready for review.
	if event.PullRequest.Draft {
		msg := "ignoring Bitbucket event - the PR is a draft"
		workflow.GetLogger(ctx).Debug(msg, "event_type", event.Type, "pr_url", event.PullRequest.Links["html"].HRef)
		return
	}

	// Wait for workflow completion before returning, to ensure we handle
	// subsequent PR initialization events appropriately (e.g. check states).
	_ = b.executeWorkflow(ctx, "bitbucket.initChannel", event).Get(ctx, nil)
}

func (b Bitbucket) prUpdated(ctx workflow.Context, event PullRequestEvent) {
}

func (b Bitbucket) prReviewed(ctx workflow.Context, event PullRequestEvent) {
	// If we're not tracking this PR, there's no need/way to announce this event.
	channelID, found := lookupChannel(ctx, event.Type, event.PullRequest)
	if !found {
		return
	}

	msg := users.BitbucketToSlackRef(ctx, b.cmd, event.Actor.AccountID, event.Actor.DisplayName)
	switch event.Type {
	case "approved":
		msg += " approved this PR :+1:"
	case "unapproved":
		msg += " unapproved this PR :-1:"
	case "changes_request_created":
		msg += " requested changes in this PR :warning:"

	// Ignored event type.
	case "changes_request_removed":

	default:
		workflow.GetLogger(ctx).Error("unrecognized Bitbucket PR review event type", "event_type", event.Type)
	}

	req := slack.ChatPostMessageRequest{Channel: channelID, MarkdownText: msg}
	slack.PostChatMessageActivityAsync(ctx, b.cmd, req)
}

// A PR was closed, i.e. merged or rejected (possibly a draft).
func (b Bitbucket) prClosed(ctx workflow.Context, event PullRequestEvent) {
	// Ignore drafts - they don't have an active Slack channel anyway.
	if event.PullRequest.Draft {
		msg := "ignoring Bitbucket event - the PR is a draft"
		workflow.GetLogger(ctx).Debug(msg, "event_type", event.Type, "pr_url", event.PullRequest.Links["html"].HRef)
		return
	}

	b.executeWorkflow(ctx, "bitbucket.archiveChannel", event)
}

func (b Bitbucket) prCommentCreated() {
}

func (b Bitbucket) prCommentUpdated() {
}

func (b Bitbucket) prCommentDeleted() {
}

func (b Bitbucket) prCommentResolved() {
}

func (b Bitbucket) prCommentReopened() {
}
