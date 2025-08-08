package bitbucket

import (
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/data"
)

func (b Bitbucket) handlePullRequestEvent(ctx workflow.Context, event PullRequestEvent) {
	switch event.Type {
	case "pullrequest.created":
		b.prCreated(ctx, event.Type, event.PullRequest, event.Actor)
	case "pullrequest.updated":
		b.prUpdated(ctx, event)

	case "pullrequest.approved":
		b.prApproved(ctx, event)
	case "pullrequest.unapproved": // A.k.a. approval removed.
		b.prUnapproved(ctx, event)

	case "pullrequest.changes_request_created":
	case "pullrequest.changes_request_removed":

	case "pullrequest.fulfilled": // A.k.a. merged.
		b.prClosed(ctx, event.Type, event.PullRequest, event.Actor)
	case "pullrequest.rejected": // A.k.a. declined.
		b.prClosed(ctx, event.Type, event.PullRequest, event.Actor)

	case "pullrequest.comment_created":
	case "pullrequest.comment_updated":
	case "pullrequest.comment_deleted":
	case "pullrequest.comment_resolved":
	case "pullrequest.comment_reopened":

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

func (b Bitbucket) prApproved(ctx workflow.Context, event PullRequestEvent) {
}

func (b Bitbucket) prUnapproved(ctx workflow.Context, event PullRequestEvent) {
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

// lookupChannel returns the ID of a channel associated
// with a PR, if the PR is active and the channel is found.
func lookupChannel(ctx workflow.Context, eventType string, pr PullRequest) (string, bool) {
	l := workflow.GetLogger(ctx)
	url := pr.Links["html"].HRef

	if pr.Draft {
		l.Debug("ignoring Bitbucket event - the PR is a draft", "event_type", eventType, "pr_url", url)
		return "", false
	}

	channelID, err := data.ConvertURLToChannel(url)
	if err != nil {
		msg := "failed to retrieve Bitbucket PR's Slack channel ID"
		l.Error(msg, "error", err, "event_type", eventType, "pr_url", url)
		return "", false
	}

	if channelID == "" {
		msg := "Bitbucket PR's Slack channel ID is empty"
		l.Debug(msg, "event_type", eventType, "pr_url", url)
		return "", false
	}

	return channelID, true
}
