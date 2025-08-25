package bitbucket

import (
	"fmt"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/markdown"
)

// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Pull-request-events
func (b Bitbucket) handlePullRequestEvent(ctx workflow.Context, event PullRequestEvent) {
	switch event.Type {
	case "created":
		b.prCreated(ctx, event)
	case "updated":
		b.prUpdated(ctx, event)

	case "approved", "unapproved", "changes_request_created", "changes_request_removed":
		b.prReviewed(ctx, event)

	case "fulfilled", "rejected":
		b.prClosed(ctx, event)

	case "comment_created":
		b.prCommentCreated(ctx, event)
	case "comment_updated":
		b.prCommentUpdated(ctx, event)
	case "comment_deleted":
		b.prCommentDeleted(ctx, event)
	case "comment_resolved":
		b.prCommentResolved(ctx, event)
	case "comment_reopened":
		b.prCommentReopened(ctx, event)

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

	msg := "%s "
	switch event.Type {
	case "approved":
		msg += "approved this PR :+1:"
	case "unapproved":
		msg += "unapproved this PR :-1:"
	case "changes_request_created":
		msg += "requested changes in this PR :warning:"

	// Ignored event type.
	case "changes_request_removed":

	default:
		workflow.GetLogger(ctx).Error("unrecognized Bitbucket PR review event type", "event_type", event.Type)
	}

	b.mentionUserInMsgAsync(ctx, channelID, event.Actor, msg)
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

func (b Bitbucket) prCommentCreated(ctx workflow.Context, event PullRequestEvent) {
	// If we're not tracking this PR, there's no need/way to announce this event.
	channelID, found := lookupChannel(ctx, event.Type, event.PullRequest)
	if !found {
		return
	}

	url := event.Comment.Links["html"].HRef
	msg := markdown.BitbucketToSlack(ctx, b.cmd, event.Comment.Content.Raw)
	if inline := event.Comment.Inline; inline != nil {
		msg = inlineCommentPrefix(url, inline) + msg
	}

	if event.Comment.Parent == nil {
		b.impersonateUserInMsg(ctx, url, channelID, event.Comment.User, msg)
	} else {
		parentURL := event.Comment.Parent.Links["html"].HRef
		b.impersonateUserInReply(ctx, url, parentURL, event.Comment.User, msg)
	}
}

func (b Bitbucket) prCommentUpdated(ctx workflow.Context, event PullRequestEvent) {
	// If we're not tracking this PR, there's no need/way to announce this event.
	_, found := lookupChannel(ctx, event.Type, event.PullRequest)
	if !found {
		return
	}

	url := event.Comment.Links["html"].HRef
	msg := markdown.BitbucketToSlack(ctx, b.cmd, event.Comment.Content.Raw)
	if inline := event.Comment.Inline; inline != nil {
		msg = inlineCommentPrefix(url, inline) + msg
	}

	b.editMsg(ctx, url, msg)
}

func (b Bitbucket) prCommentDeleted(ctx workflow.Context, event PullRequestEvent) {
	// If we're not tracking this PR, there's no need/way to announce this event.
	_, found := lookupChannel(ctx, event.Type, event.PullRequest)
	if !found {
		return
	}

	url := event.Comment.Links["html"].HRef
	b.deleteMsg(ctx, url)
}

func inlineCommentPrefix(url string, i *Inline) string {
	subject := "File"
	location := "the"

	if i.From != nil {
		subject = "Line"
		location = fmt.Sprintf("line %d in the", *i.From)

		if i.To != nil {
			location = fmt.Sprintf("lines %d-%d in the", *i.From, *i.To)
		}
	}

	return fmt.Sprintf("<%s|%s comment> in %s file `%s`:\n", url, subject, location, i.Path)
}

func (b Bitbucket) prCommentResolved(ctx workflow.Context, event PullRequestEvent) {
	// If we're not tracking this PR, there's no need/way to announce this event.
	_, found := lookupChannel(ctx, event.Type, event.PullRequest)
	if !found {
		return
	}

	url := event.Comment.Links["html"].HRef
	b.mentionUserInReplyAsync(ctx, url, event.Actor, "%s resolved this comment :ok:")
	b.addReactionAsync(ctx, url, "ok")
}

func (b Bitbucket) prCommentReopened(ctx workflow.Context, event PullRequestEvent) {
	// If we're not tracking this PR, there's no need/way to announce this event.
	_, found := lookupChannel(ctx, event.Type, event.PullRequest)
	if !found {
		return
	}

	url := event.Comment.Links["html"].HRef
	b.mentionUserInReplyAsync(ctx, url, event.Actor, "%s reopened this comment :no_good:")
	b.removeReactionAsync(ctx, url, "ok")
}
