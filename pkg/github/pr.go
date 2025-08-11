package github

import (
	"fmt"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/slack"
	"github.com/tzrikka/revchat/pkg/users"
)

func (g GitHub) handlePullRequestEvent(ctx workflow.Context, event PullRequestEvent) {
	switch event.Action {
	case "opened":
		g.prOpened(ctx, event.Action, event.PullRequest, event.Sender)
	case "closed":
		g.prClosed(ctx, event.Action, event.PullRequest, event.Sender)
	case "reopened":
		g.prReopened(ctx, event.Action, event.PullRequest, event.Sender)

	case "converted_to_draft":
		g.prConvertedToDraft(ctx, event.Action, event.PullRequest, event.Sender)
	case "ready_for_review":
		g.prReadyForReview(ctx, event.Action, event.PullRequest, event.Sender)

	case "review_requested":
		g.prReviewRequested(ctx, event)
	case "review_request_removed":
		g.prReviewRequestRemoved(ctx, event)

	case "assigned":
		g.prAssigned(ctx, event.Action, event.PullRequest, *event.Assignee, event.Sender)
	case "unassigned":
		g.prUnassigned(ctx, event.Action, event.PullRequest, *event.Assignee, event.Sender)

	case "edited": // Title, body, base branch.
		g.prEdited(ctx, event.Action, event.PullRequest, *event.Changes, event.Sender)
	case "synchronize": // Head branch.
		g.prSynchronized(ctx, event.Action, event.PullRequest, event.Sender)

	case "locked":
		g.prLocked()
	case "unlocked":
		g.prUnlocked()
	}

	// Ignored actions:
	//  - auto_merge_enabled, auto_merge_disabled
	//  - enqueued, dequeued
	//  - labeled, unlabeled
	//  - milestoned, demilestoned
}

// A new PR was created (or reopened, or marked as ready for review).
// See also [GitHub.prReopened] and [GitHub.prReadyForReview] which wrap it.
func (g GitHub) prOpened(ctx workflow.Context, action string, pr PullRequest, sender User) {
	// Ignore drafts until they're marked as ready for review.
	if pr.Draft {
		msg := "ignoring GitHub event - the PR is a draft"
		workflow.GetLogger(ctx).Debug(msg, "action", action, "pr_url", pr.HTMLURL)
		return
	}

	// Wait for workflow completion before returning, to ensure we handle
	// subsequent PR initialization events appropriately (e.g. check states).
	req := PullRequestEvent{Action: action, PullRequest: pr, Sender: sender}
	_ = g.executeWorkflow(ctx, "github.initChannel", req).Get(ctx, nil)
}

// A PR (possibly a draft) was closed.
// If "merged" is false in the webhook payload, the PR was
// closed with unmerged commits. Otherwise, the PR was merged.
func (g GitHub) prClosed(ctx workflow.Context, action string, pr PullRequest, sender User) {
	// Ignore drafts - they don't have an active Slack channel anyway.
	if pr.Draft {
		msg := "ignoring GitHub event - the PR is a draft"
		workflow.GetLogger(ctx).Debug(msg, "action", action, "pr_url", pr.HTMLURL)
		return
	}

	req := PullRequestEvent{Action: action, PullRequest: pr, Sender: sender}
	g.executeWorkflow(ctx, "github.archiveChannel", req)
}

// A previously closed PR (possibly a draft) was reopened.
func (g GitHub) prReopened(ctx workflow.Context, action string, pr PullRequest, sender User) {
	// Ignore drafts - they don't have an active Slack channel anyway.
	if pr.Draft {
		msg := "ignoring GitHub event - the PR is a draft"
		workflow.GetLogger(ctx).Debug(msg, "action", action, "pr_url", pr.HTMLURL)
		return
	}

	// Slack bug notice from https://docs.slack.dev/reference/methods/conversations.unarchive:
	// bot tokens ("xoxb-...") cannot currently be used to unarchive conversations. For now,
	// use a user token ("xoxp-...") to unarchive the conversation rather than a bot token.
	// Workaround for the Slack unarchive bug: treat this as a new PR.
	g.prOpened(ctx, action, pr, sender)
}

// A PR was converted to a draft.
// For more information, see "Changing the stage of a pull request":
// https://docs.github.com/pull-requests/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/changing-the-stage-of-a-pull-request
func (g GitHub) prConvertedToDraft(ctx workflow.Context, action string, pr PullRequest, sender User) {
	req := PullRequestEvent{Action: action, PullRequest: pr, Sender: sender}
	g.executeWorkflow(ctx, "github.archiveChannel", req)
}

// A draft PR was marked as ready for review.
// For more information, see "Changing the stage of a pull request":
// https://docs.github.com/pull-requests/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/changing-the-stage-of-a-pull-request
func (g GitHub) prReadyForReview(ctx workflow.Context, action string, pr PullRequest, sender User) {
	// Slack bug notice from https://docs.slack.dev/reference/methods/conversations.unarchive:
	// bot tokens ("xoxb-...") cannot currently be used to unarchive conversations. For now,
	// use a user token ("xoxp-...") to unarchive the conversation rather than a bot token.
	// Workaround for the Slack unarchive bug: treat this as a new PR.
	g.prOpened(ctx, action, pr, sender)
}

// Review by a person or team was requested for a PR.
// For more information, see "Requesting a pull request review":
// https://docs.github.com/pull-requests/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/requesting-a-pull-request-review
func (g GitHub) prReviewRequested(ctx workflow.Context, event PullRequestEvent) {
	// If we're not tracking this PR, there's no need/way to announce this event.
	channelID, found := lookupChannel(ctx, event.Action, event.PullRequest)
	if !found {
		return
	}

	if reviewer := event.RequestedReviewer; reviewer != nil {
		g.prReviewRequestedUser(ctx, channelID, *reviewer, event.Sender, "reviewer")
	}
	if team := event.RequestedTeam; team != nil {
		ref := users.GitHubToSlackRef(ctx, g.cmd, event.Sender.Login, event.Sender.HTMLURL)
		msg := fmt.Sprintf("%s added the <%s|%s> team as a reviewer", ref, team.HTMLURL, team.Name)
		req := slack.ChatPostMessageRequest{Channel: channelID, MarkdownText: msg}
		slack.PostChatMessageActivityAsync(ctx, g.cmd, req)
	}
}

func (g GitHub) prReviewRequestedUser(ctx workflow.Context, channelID string, reviewer, sender User, role string) {
}

// A request for review by a person or team was removed from a PR.
func (g GitHub) prReviewRequestRemoved(ctx workflow.Context, event PullRequestEvent) {
	// If we're not tracking this PR, there's no need/way to announce this event.
	channelID, found := lookupChannel(ctx, event.Action, event.PullRequest)
	if !found {
		return
	}

	if reviewer := event.RequestedReviewer; reviewer != nil {
		g.prReviewRequestRemovedUser(ctx, channelID, *reviewer, event.Sender, "reviewer")
	}
	if team := event.RequestedTeam; team != nil {
		ref := users.GitHubToSlackRef(ctx, g.cmd, event.Sender.Login, event.Sender.HTMLURL)
		msg := fmt.Sprintf("%s removed the <%s|%s> team as a reviewer", ref, team.HTMLURL, team.Name)
		req := slack.ChatPostMessageRequest{Channel: channelID, MarkdownText: msg}
		slack.PostChatMessageActivityAsync(ctx, g.cmd, req)
	}
}

func (g GitHub) prReviewRequestRemovedUser(ctx workflow.Context, channelID string, reviewer, sender User, role string) {
}

// A PR was assigned to a user.
func (g GitHub) prAssigned(ctx workflow.Context, action string, pr PullRequest, assignee, sender User) {
	// If we're not tracking this PR, there's no need/way to announce this event.
	channelID, found := lookupChannel(ctx, action, pr)
	if !found {
		return
	}

	g.prReviewRequestedUser(ctx, channelID, assignee, sender, "assignee")
}

// A user was unassigned from a PR.
func (g GitHub) prUnassigned(ctx workflow.Context, action string, pr PullRequest, assignee, sender User) {
	// If we're not tracking this PR, there's no need/way to announce this event.
	channelID, found := lookupChannel(ctx, action, pr)
	if !found {
		return
	}

	g.prReviewRequestRemovedUser(ctx, channelID, assignee, sender, "assignee")
}

// The title or body of a PR was edited, or the base branch was changed.
func (g GitHub) prEdited(ctx workflow.Context, action string, pr PullRequest, changes Changes, sender User) {
}

// A PR's head branch was updated. For example, the head branch was updated
// from the base branch or new commits were pushed to the head branch.
func (g GitHub) prSynchronized(ctx workflow.Context, action string, pr PullRequest, sender User) {
	// If we're not tracking this PR, there's no need/way to announce this event.
	channelID, found := lookupChannel(ctx, action, pr)
	if !found {
		return
	}

	ref := users.GitHubToSlackRef(ctx, g.cmd, sender.Login, sender.HTMLURL)
	msg := fmt.Sprintf("%s updated the head branch (see the [PR commits](%s/commits))", ref, pr.HTMLURL)
	req := slack.ChatPostMessageRequest{Channel: channelID, MarkdownText: msg}
	slack.PostChatMessageActivityAsync(ctx, g.cmd, req)
}

// Conversation on a PR was locked. For more information, see "Locking conversations":
// https://docs.github.com/en/communities/moderating-comments-and-conversations/locking-conversations
func (g GitHub) prLocked() {
}

// Conversation on a pull request was unlocked. For more information, see "Locking conversations":
// https://docs.github.com/en/communities/moderating-comments-and-conversations/locking-conversations
func (g GitHub) prUnlocked() {
}
