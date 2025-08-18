package github

import (
	"fmt"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/markdown"
)

// https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request
// https://docs.github.com/pull-requests/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/about-pull-requests
func (g GitHub) handlePullRequestEvent(ctx workflow.Context, event PullRequestEvent) {
	switch event.Action {
	case "opened":
		g.prOpened(ctx, event)
	case "closed":
		g.prClosed(ctx, event)
	case "reopened":
		g.prReopened(ctx, event)

	case "converted_to_draft":
		g.prConvertedToDraft(ctx, event)
	case "ready_for_review":
		g.prReadyForReview(ctx, event)

	case "review_requested", "review_request_removed", "assigned", "unassigned":
		g.prReviewRequests(ctx, event)

	case "edited": // Title, body, base branch.
		g.prEdited(ctx, event)
	case "synchronize": // Head branch.
		g.prSynchronized(ctx, event)

	case "locked":
		g.prLocked()
	case "unlocked":
		g.prUnlocked()

	// Ignored actions.
	case "auto_merge_enabled", "auto_merge_disabled":
	case "enqueued", "dequeued":
	case "labeled", "unlabeled":
	case "milestoned", "demilestoned":

	default:
		workflow.GetLogger(ctx).Error("unrecognized GitHub PR event action", "action", event.Action)
	}
}

// A new PR was created (or reopened, or marked as ready for review).
// See also [GitHub.prReopened] and [GitHub.prReadyForReview] which wrap it.
func (g GitHub) prOpened(ctx workflow.Context, event PullRequestEvent) {
	// Ignore drafts until they're marked as ready for review.
	if event.PullRequest.Draft {
		msg := "ignoring GitHub event - the PR is a draft"
		workflow.GetLogger(ctx).Debug(msg, "action", event.Action, "pr_url", event.PullRequest.HTMLURL)
		return
	}

	// Wait for workflow completion before returning, to ensure we handle
	// subsequent PR initialization events appropriately (e.g. check states).
	_ = g.executeWorkflow(ctx, "github.initChannel", event).Get(ctx, nil)
}

// A PR (possibly a draft) was closed.
// If "merged" is false in the webhook payload, the PR was
// closed with unmerged commits. Otherwise, the PR was merged.
func (g GitHub) prClosed(ctx workflow.Context, event PullRequestEvent) {
	// Ignore drafts - they don't have an active Slack channel anyway.
	if event.PullRequest.Draft {
		msg := "ignoring GitHub event - the PR is a draft"
		workflow.GetLogger(ctx).Debug(msg, "action", event.Action, "pr_url", event.PullRequest.HTMLURL)
		return
	}

	g.executeWorkflow(ctx, "github.archiveChannel", event)
}

// A previously closed PR (possibly a draft) was reopened.
func (g GitHub) prReopened(ctx workflow.Context, event PullRequestEvent) {
	// Slack bug notice from https://docs.slack.dev/reference/methods/conversations.unarchive:
	// bot tokens ("xoxb-...") cannot currently be used to unarchive conversations. For now,
	// use a user token ("xoxp-...") to unarchive the conversation rather than a bot token.
	// Workaround for the Slack unarchive bug: treat this as a new PR.
	g.prOpened(ctx, event)
}

// A PR was converted to a draft.
// For more information, see "Changing the stage of a pull request":
// https://docs.github.com/pull-requests/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/changing-the-stage-of-a-pull-request
func (g GitHub) prConvertedToDraft(ctx workflow.Context, event PullRequestEvent) {
	g.executeWorkflow(ctx, "github.archiveChannel", event)
}

// A draft PR was marked as ready for review.
// For more information, see "Changing the stage of a pull request":
// https://docs.github.com/pull-requests/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/changing-the-stage-of-a-pull-request
func (g GitHub) prReadyForReview(ctx workflow.Context, event PullRequestEvent) {
	// Slack bug notice from https://docs.slack.dev/reference/methods/conversations.unarchive:
	// bot tokens ("xoxb-...") cannot currently be used to unarchive conversations. For now,
	// use a user token ("xoxp-...") to unarchive the conversation rather than a bot token.
	// Workaround for the Slack unarchive bug: treat this as a new PR.
	g.prOpened(ctx, event)
}

// Review by a person or team was requested or removed for a PR.
// For more information, see "Requesting a pull request review":
// https://docs.github.com/pull-requests/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/requesting-a-pull-request-review
func (g GitHub) prReviewRequests(ctx workflow.Context, event PullRequestEvent) {
	// If we're not tracking this PR, there's no need/way to announce this event.
	if _, found := lookupChannel(ctx, event.Action, event.PullRequest); found {
		g.executeWorkflow(ctx, "github.updateMembers", event)
	}
}

// The title or body of a PR was edited, or the base branch was changed.
func (g GitHub) prEdited(ctx workflow.Context, event PullRequestEvent) {
	pr := event.PullRequest

	// If we're not tracking this PR, there's no need/way to announce this event.
	channelID, found := lookupChannel(ctx, event.Action, pr)
	if !found {
		return
	}

	// PR base branch was changed.
	if event.Changes.Base != nil {
		msg := fmt.Sprintf("changed the base branch from `%s` to `%s`", event.Changes.Base.Ref, pr.Base.Ref)
		g.mentionUserInMsgAsync(ctx, channelID, event.Sender, "%s "+msg)
	}

	// PR description was changed.
	if event.Changes.Body != nil {
		msg := "%s "
		if *pr.Body != "" {
			msg += "updated the PR description to:\n\n" + markdown.GitHubToSlack(ctx, g.cmd, *pr.Body, pr.HTMLURL)
		} else {
			msg += "deleted the PR description"
		}
		g.mentionUserInMsgAsync(ctx, channelID, event.Sender, msg)
	}

	// PR title was changed.
	if event.Changes.Title != nil {
		msg := fmt.Sprintf("edited the PR title to: `%s`", pr.Title)
		g.mentionUserInMsgAsync(ctx, channelID, event.Sender, "%s "+msg)
	}
}

// A PR's head branch was updated. For example, the head branch was updated
// from the base branch or new commits were pushed to the head branch.
func (g GitHub) prSynchronized(ctx workflow.Context, event PullRequestEvent) {
	pr := event.PullRequest
	after := *event.After

	// If we're not tracking this PR, there's no need/way to announce this event.
	channelID, found := lookupChannel(ctx, event.Action, pr)
	if !found {
		return
	}

	msg := fmt.Sprintf("pushed commit [`%s`](%s/commits/%s) into the head branch", after[:7], pr.HTMLURL, after)
	g.mentionUserInMsgAsync(ctx, channelID, event.Sender, "%s "+msg)
}

// Conversation on a PR was locked. For more information, see "Locking conversations":
// https://docs.github.com/en/communities/moderating-comments-and-conversations/locking-conversations
func (g GitHub) prLocked() {
}

// Conversation on a pull request was unlocked. For more information, see "Locking conversations":
// https://docs.github.com/en/communities/moderating-comments-and-conversations/locking-conversations
func (g GitHub) prUnlocked() {
}
