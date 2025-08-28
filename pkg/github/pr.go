package github

import (
	"errors"
	"fmt"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/revchat/pkg/markdown"
)

// https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request
// https://docs.github.com/pull-requests/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/about-pull-requests
func (c Config) pullRequestWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	switch event.Action {
	case "opened":
		return c.prOpened(ctx, event)
	case "closed":
		return c.prClosed(ctx, event)
	case "reopened":
		return c.prReopened(ctx, event)

	case "converted_to_draft":
		return c.prConvertedToDraft(ctx, event)
	case "ready_for_review":
		return c.prReadyForReview(ctx, event)

	case "review_requested", "review_request_removed", "assigned", "unassigned":
		return c.prReviewRequests(ctx, event)

	case "edited": // Title, body, base branch.
		return c.prEdited(ctx, event)
	case "synchronize": // Head branch.
		return c.prSynchronized(ctx, event)

	case "locked":
		return c.prLocked()
	case "unlocked":
		return c.prUnlocked()

	// Ignored actions.
	case "auto_merge_enabled", "auto_merge_disabled":
	case "enqueued", "dequeued":
	case "labeled", "unlabeled":
	case "milestoned", "demilestoned":

	default:
		log.Error(ctx, "unrecognized GitHub PR event action", "action", event.Action)
		return errors.New("unrecognized GitHub PR event action: " + event.Action)
	}

	return nil
}

// A new PR was created (or reopened, or marked as ready for review).
// See also [Config.prReopened] and [Config.prReadyForReview] which wrap it.
func (c Config) prOpened(ctx workflow.Context, event PullRequestEvent) error {
	// Ignore drafts until they're marked as ready for review.
	if event.PullRequest.Draft {
		return nil
	}

	return c.initChannel(ctx, event)
}

// A PR (possibly a draft) was closed.
// If "merged" is false in the webhook payload, the PR was
// closed with unmerged commits. Otherwise, the PR was merged.
func (c Config) prClosed(ctx workflow.Context, event PullRequestEvent) error {
	// Ignore drafts - they don't have an active Slack channel anyway.
	if event.PullRequest.Draft {
		return nil
	}

	return c.archiveChannel(ctx, event)
}

// A previously closed PR (possibly a draft) was reopened.
func (c Config) prReopened(ctx workflow.Context, event PullRequestEvent) error {
	// Slack bug notice from https://docs.slack.dev/reference/methods/conversations.unarchive:
	// bot tokens ("xoxb-...") cannot currently be used to unarchive conversations. For now,
	// use a user token ("xoxp-...") to unarchive the conversation rather than a bot token.
	// Workaround for the Slack unarchive bug: treat this as a new PR.
	return c.prOpened(ctx, event)
}

// A PR was converted to a draft.
// For more information, see "Changing the stage of a pull request":
// https://docs.github.com/pull-requests/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/changing-the-stage-of-a-pull-request
func (c Config) prConvertedToDraft(ctx workflow.Context, event PullRequestEvent) error {
	return c.archiveChannel(ctx, event)
}

// A draft PR was marked as ready for review.
// For more information, see "Changing the stage of a pull request":
// https://docs.github.com/pull-requests/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/changing-the-stage-of-a-pull-request
func (c Config) prReadyForReview(ctx workflow.Context, event PullRequestEvent) error {
	// Slack bug notice from https://docs.slack.dev/reference/methods/conversations.unarchive:
	// bot tokens ("xoxb-...") cannot currently be used to unarchive conversations. For now,
	// use a user token ("xoxp-...") to unarchive the conversation rather than a bot token.
	// Workaround for the Slack unarchive bug: treat this as a new PR.
	return c.prOpened(ctx, event)
}

// Review by a person or team was requested or removed for a PR.
// For more information, see "Requesting a pull request review":
// https://docs.github.com/pull-requests/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/requesting-a-pull-request-review
func (c Config) prReviewRequests(ctx workflow.Context, event PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	if _, found := lookupChannel(ctx, event.Action, event.PullRequest); !found {
		return nil
	}

	return c.updateMembers(ctx, event)
}

// The title or body of a PR was edited, or the base branch was changed.
func (c Config) prEdited(ctx workflow.Context, event PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	pr := event.PullRequest
	channelID, found := lookupChannel(ctx, event.Action, pr)
	if !found {
		return nil
	}

	// PR base branch was changed.
	if event.Changes.Base != nil {
		msg := fmt.Sprintf("changed the base branch from `%s` to `%s`", event.Changes.Base.Ref, pr.Base.Ref)
		if err := c.mentionUserInMsg(ctx, channelID, event.Sender, "%s "+msg); err != nil {
			return err
		}
	}

	// PR description was changed.
	if event.Changes.Body != nil {
		msg := "%s "
		if *pr.Body != "" {
			msg += "updated the PR description to:\n\n" + markdown.GitHubToSlack(ctx, c.Cmd, *pr.Body, pr.HTMLURL)
		} else {
			msg += "deleted the PR description"
		}
		if err := c.mentionUserInMsg(ctx, channelID, event.Sender, msg); err != nil {
			return err
		}
	}

	// PR title was changed.
	if event.Changes.Title != nil {
		msg := fmt.Sprintf("edited the PR title to: `%s`", pr.Title)
		if err := c.mentionUserInMsg(ctx, channelID, event.Sender, "%s "+msg); err != nil {
			return err
		}
	}

	return nil
}

// A PR's head branch was updated. For example, the head branch was updated
// from the base branch or new commits were pushed to the head branch.
func (c Config) prSynchronized(ctx workflow.Context, event PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	pr := event.PullRequest
	channelID, found := lookupChannel(ctx, event.Action, pr)
	if !found {
		return nil
	}

	after := *event.After
	msg := fmt.Sprintf("pushed commit [`%s`](%s/commits/%s) into the head branch", after[:7], pr.HTMLURL, after)
	return c.mentionUserInMsg(ctx, channelID, event.Sender, "%s "+msg)
}

// Conversation on a PR was locked. For more information, see "Locking conversations":
// https://docs.github.com/en/communities/moderating-comments-and-conversations/locking-conversations
func (c Config) prLocked() error {
	return nil
}

// Conversation on a pull request was unlocked. For more information, see "Locking conversations":
// https://docs.github.com/en/communities/moderating-comments-and-conversations/locking-conversations
func (c Config) prUnlocked() error {
	return nil
}
