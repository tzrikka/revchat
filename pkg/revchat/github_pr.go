package revchat

import (
	"fmt"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/data"
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
		g.prAssigned(ctx, event)
	case "unassigned":
		g.prUnassigned(ctx, event)

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
		workflow.GetLogger(ctx).Debug(msg, "action", action, "url", pr.HTMLURL)
		return
	}

	req := PullRequestEvent{Action: action, PullRequest: pr, Sender: sender}
	g.executeRevChatWorkflow(ctx, "github.initChannel", req)
}

// A PR (possibly a draft) was closed.
// If "merged" is false in the webhook payload, the PR was
// closed with unmerged commits. Otherwise, the PR was merged.
func (g GitHub) prClosed(ctx workflow.Context, action string, pr PullRequest, sender User) {
	// Ignore drafts - they don't have an active Slack channel anyway.
	if pr.Draft {
		msg := "ignoring GitHub event - the PR is a draft"
		workflow.GetLogger(ctx).Debug(msg, "action", action, "url", pr.HTMLURL)
		return
	}

	req := PullRequestEvent{Action: action, PullRequest: pr, Sender: sender}
	g.executeRevChatWorkflow(ctx, "slack.archiveChannel", req)
}

// A previously closed PR (possibly a draft) was reopened.
func (g GitHub) prReopened(ctx workflow.Context, action string, pr PullRequest, sender User) {
	// Ignore drafts - they don't have an active Slack channel anyway.
	if pr.Draft {
		msg := "ignoring GitHub event - the PR is a draft"
		workflow.GetLogger(ctx).Debug(msg, "action", action, "url", pr.HTMLURL)
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
	g.executeRevChatWorkflow(ctx, "slack.archiveChannel", req)
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
	// Don't do anything if there isn't an active Slack channel anyway.
	channel, found := lookupChannel(ctx, event.Action, event.PullRequest)
	if !found {
		return
	}

	if reviewer := event.RequestedReviewer; reviewer != nil {
		g.prReviewRequestedPerson(ctx, channel, *reviewer, event.Sender, "reviewer")
	}
	if team := event.RequestedTeam; team != nil {
		g.prReviewRequestedTeam(ctx, channel, *team, event.Sender)
	}
}

func (g GitHub) prReviewRequestedPerson(ctx workflow.Context, channel string, reviewer, sender User, role string) {
}

func (g GitHub) prReviewRequestedTeam(ctx workflow.Context, channel string, team Team, sender User) {
	msg := fmt.Sprintf("added the <%s|%s> team as a reviewer", team.HTMLURL, team.Name)
	_, _ = g.mentionUserInMessage(ctx, channel, sender, "%s "+msg)
}

// A request for review by a person or team was removed from a PR.
func (g GitHub) prReviewRequestRemoved(ctx workflow.Context, event PullRequestEvent) {
	// Don't do anything if there isn't an active Slack channel anyway.
	channel, found := lookupChannel(ctx, event.Action, event.PullRequest)
	if !found {
		return
	}

	if reviewer := event.RequestedReviewer; reviewer != nil {
		g.prReviewRequestRemovedPerson(ctx, channel, *reviewer, event.Sender, "reviewer")
	}
	if team := event.RequestedTeam; team != nil {
		g.prReviewRequestRemovedTeam(ctx, channel, *team, event.Sender)
	}
}

func (g GitHub) prReviewRequestRemovedPerson(ctx workflow.Context, channel string, reviewer, sender User, role string) {
}

func (g GitHub) prReviewRequestRemovedTeam(ctx workflow.Context, channel string, team Team, sender User) {
	msg := fmt.Sprintf("removed the <%s|%s> team as a reviewer", team.HTMLURL, team.Name)
	_, _ = g.mentionUserInMessage(ctx, channel, sender, "%s "+msg)
}

// A PR was assigned to a user.
func (g GitHub) prAssigned(ctx workflow.Context, event PullRequestEvent) {
	// Don't do anything if there isn't an active Slack channel anyway.
	channel, found := lookupChannel(ctx, event.Action, event.PullRequest)
	if !found {
		return
	}

	g.prReviewRequestedPerson(ctx, channel, *event.Assignee, event.Sender, "assignee")
}

// A user was unassigned from a PR.
func (g GitHub) prUnassigned(ctx workflow.Context, event PullRequestEvent) {
	// Don't do anything if there isn't an active Slack channel anyway.
	channel, found := lookupChannel(ctx, event.Action, event.PullRequest)
	if !found {
		return
	}

	g.prReviewRequestRemovedPerson(ctx, channel, *event.Assignee, event.Sender, "assignee")
}

// The title or body of a PR was edited, or the base branch was changed.
func (g GitHub) prEdited(ctx workflow.Context, action string, pr PullRequest, changes Changes, sender User) {
}

// A PR's head branch was updated.
// For example, the head branch was updated from the base
// branch or new commits were pushed to the head branch.
func (g GitHub) prSynchronized(ctx workflow.Context, action string, pr PullRequest, sender User) {
	// Don't do anything if there isn't an active Slack channel anyway.
	channel, found := lookupChannel(ctx, action, pr)
	if !found {
		return
	}

	msg := fmt.Sprintf("updated the head branch (see the [PR commits](%s/commits))", pr.User.HTMLURL)
	_, _ = g.mentionUserInMessage(ctx, channel, sender, "%s "+msg)
}

// Conversation on a PR was locked. For more information, see "Locking conversations":
// https://docs.github.com/en/communities/moderating-comments-and-conversations/locking-conversations
func (g GitHub) prLocked() {
}

// Conversation on a pull request was unlocked. For more information, see "Locking conversations":
// https://docs.github.com/en/communities/moderating-comments-and-conversations/locking-conversations
func (g GitHub) prUnlocked() {
}

// lookupChannel returns the ID of a channel associated with
// a GitHub PR, if the PR is active and the channel is found.
func lookupChannel(ctx workflow.Context, action string, pr PullRequest) (string, bool) {
	l := workflow.GetLogger(ctx)

	if pr.Draft {
		l.Debug("ignoring GitHub event - the PR is a draft", "action", action, "url", pr.HTMLURL)
		return "", false
	}

	channel, err := data.ConvertURLToChannel(pr.HTMLURL)
	if err != nil {
		msg := "failed to retrieve GitHub PR's Slack channel ID"
		l.Error(msg, "error", err.Error(), "action", action, "url", pr.HTMLURL)
		return "", false
	}

	if channel == "" {
		msg := "GitHub PR's Slack channel ID is empty"
		l.Debug(msg, "action", action, "url", pr.HTMLURL)
		return "", false
	}

	return channel, true
}
