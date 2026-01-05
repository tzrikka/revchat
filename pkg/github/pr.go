package github

/*
import (
	"fmt"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/markdown"
)

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
	if _, found := lookupChannel(ctx, event.PullRequest); !found {
		return nil
	}

	return c.updateMembers(ctx, event)
}

// The title or body of a PR was edited, or the base branch was changed.
func (c Config) prEdited(ctx workflow.Context, event PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	pr := event.PullRequest
	channelID, found := lookupChannel(ctx, pr)
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
			msg += "updated the PR description to:\n\n" + markdown.GitHubToSlack(ctx, *pr.Body, pr.HTMLURL)
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

		// TODO: Rename channel.
		// name = f"{pr.number}_{normalize_channel_name(pr.title)}"
		// slack_helper.rename_channel(channel, name)
	}

	return nil
}

// A PR's head branch was updated. For example, the head branch was updated
// from the base branch or new commits were pushed to the head branch.
func (c Config) prSynchronized(ctx workflow.Context, event PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	pr := event.PullRequest
	channelID, found := lookupChannel(ctx, pr)
	if !found {
		return nil
	}

	after := *event.After
	msg := fmt.Sprintf("pushed commit [`%s`](%s/commits/%s) into the head branch", after[:7], pr.HTMLURL, after)
	err := c.mentionUserInMsg(ctx, channelID, event.Sender, "%s "+msg)

	// Why do we post the message before updating the bookmark, and ignore bookmark update errors, instead of
	// just reversing the order of these operations? Because posting the message is the core of this workflow.
	// TODO: Update the commits bookmark's title.

	return err
}

// Conversation on a PR was locked. For more information, see "Locking conversations":
// https://docs.github.com/en/communities/moderating-comments-and-conversations/locking-conversations
func (c Config) prLocked() error {
	// TODO: Implement.
	return nil
}

// Conversation on a pull request was unlocked. For more information, see "Locking conversations":
// https://docs.github.com/en/communities/moderating-comments-and-conversations/locking-conversations
func (c Config) prUnlocked() error {
	// TODO: Implement.
	return nil
}
*/
