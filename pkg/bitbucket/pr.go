package bitbucket

import (
	"go.temporal.io/sdk/workflow"
)

func (b Bitbucket) handlePullRequestEvent(ctx workflow.Context, event PullRequestEvent) {
	switch event.Type {
	case "pullrequest.created":
		b.prCreated(ctx, event.Type, event.PullRequest, event.Actor)
	case "pullrequest.updated":
		b.prUpdated(ctx, event)

	case "pullrequest.approved", "pullrequest.unapproved":
		b.prReviewed(ctx, event.Type, event.PullRequest, event.Actor)
	case "pullrequest.changes_request_created", "pullrequest.changes_request_removed":
		b.prReviewed(ctx, event.Type, event.PullRequest, event.Actor)

	case "pullrequest.fulfilled", "pullrequest.rejected":
		b.prClosed(ctx, event.Type, event.PullRequest, event.Actor)

	case "pullrequest.comment_created":
	case "pullrequest.comment_updated":
	case "pullrequest.comment_deleted":
	case "pullrequest.comment_resolved":
	case "pullrequest.comment_reopened":

	case "repo.commit_comment_created":

	default:
		workflow.GetLogger(ctx).Error("unrecognized Bitbucket PR event type", "event_type", event.Type)
	}
}

// A new PR was created (or marked as ready for review - see [Bitbucket.prUpdated]).
func (b Bitbucket) prCreated(ctx workflow.Context, eventType string, pr PullRequest, actor Account) {
	// Ignore drafts until they're marked as ready for review.
	if pr.Draft {
		msg := "ignoring Bitbucket event - the PR is a draft"
		workflow.GetLogger(ctx).Debug(msg, "event_type", eventType, "pr_url", pr.Links["html"].HRef)
		return
	}

	req := PullRequestEvent{Type: eventType, PullRequest: pr, Actor: actor}
	b.executeRevChatWorkflow(ctx, "bitbucket.initChannel", req)
}

func (b Bitbucket) prUpdated(ctx workflow.Context, event PullRequestEvent) {
}

func (b Bitbucket) prReviewed(ctx workflow.Context, eventType string, pr PullRequest, actor Account) {
	// If we're not tracking this PR, there's no need/way to announce this event.
	channelID, found := lookupChannel(ctx, eventType, pr)
	if !found {
		return
	}

	msg := "%s "
	switch eventType {
	case "pullrequest.approved":
		msg += "approved this PR :+1:"
	case "pullrequest.unapproved":
		msg += "unapproved this PR :-1:"
	case "pullrequest.changes_request_created":
		msg += "requested changes in this PR :warning:"
	}
	// Ignored event type: pullrequest.changes_request_removed.

	_, _ = b.mentionUserInMsg(ctx, channelID, actor, msg)
}

// A PR was closed, i.e. merged or rejected (possibly a draft).
func (b Bitbucket) prClosed(ctx workflow.Context, eventType string, pr PullRequest, actor Account) {
	// Ignore drafts - they don't have an active Slack channel anyway.
	if pr.Draft {
		msg := "ignoring Bitbucket event - the PR is a draft"
		workflow.GetLogger(ctx).Debug(msg, "event_type", eventType, "pr_url", pr.Links["html"].HRef)
		return
	}

	req := PullRequestEvent{Type: eventType, PullRequest: pr, Actor: actor}
	b.executeRevChatWorkflow(ctx, "bitbucket.archiveChannel", req)
}
