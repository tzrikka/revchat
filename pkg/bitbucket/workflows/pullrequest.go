package workflows

import (
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/bitbucket"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/markdown"
	"github.com/tzrikka/revchat/pkg/slack/activities"
	"github.com/tzrikka/revchat/pkg/users"
)

// PullRequestCreatedWorkflow initializes a new Slack channel for a newly-created PR:
// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Created.1
func (c Config) PullRequestCreatedWorkflow(ctx workflow.Context, event bitbucket.PullRequestEvent) error {
	pr := event.PullRequest
	prURL := bitbucket.HTMLURL(pr.Links)
	pr.CommitCount = len(bitbucket.Commits(ctx, event))

	maxLen, prefix, private := c.SlackChannelNameMaxLength, c.SlackChannelNamePrefix, c.SlackChannelsArePrivate
	channelID, err := bitbucket.CreateSlackChannel(ctx, pr, maxLen, prefix, private)
	if err != nil {
		if userID := users.BitbucketToSlackID(ctx, event.Actor.AccountID, true); userID != "" {
			_, _ = activities.PostMessage(ctx, userID, "Failed to create Slack channel for "+prURL)
		}
		return err
	}

	bitbucket.InitPRData(ctx, event, channelID)

	// Channel cosmetics.
	activities.SetChannelTopic(ctx, channelID, prURL)
	activities.SetChannelDescription(ctx, channelID, pr.Title, prURL)
	bitbucket.SetChannelBookmarks(ctx, channelID, prURL, pr)

	msg := "%s created this PR: " + bitbucket.LinkifyTitle(ctx, c.LinkifyMap, pr.Title)
	if desc := strings.TrimSpace(pr.Description); desc != "" {
		msg += "\n\n" + markdown.BitbucketToSlack(ctx, desc, prURL)
	}
	bitbucket.MentionUserInMsg(ctx, channelID, event.Actor, msg)
	err = activities.InviteUsersToChannel(ctx, channelID, bitbucket.Invitees(ctx, pr))
	if err != nil {
		if userID := users.BitbucketToSlackID(ctx, event.Actor.AccountID, true); userID != "" {
			_, _ = activities.PostMessage(ctx, userID, "Failed to create Slack channel for "+prURL)
		}
		// Undo channel creation and PR data initialization.
		_ = activities.ArchiveChannel(ctx, channelID, prURL)
		data.CleanupPRData(ctx, channelID, prURL)
		return err
	}

	return nil
}

// PullRequestClosedWorkflow archives a PR's Slack channel when the PR is merged or declined/rejected:
//   - https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Merged
//   - https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Declined
func PullRequestClosedWorkflow(ctx workflow.Context, event bitbucket.PullRequestEvent) error {
	// If we're not tracking this PR, there's no channel to archive.
	prURL := bitbucket.HTMLURL(event.PullRequest.Links)
	channelID, found := bitbucket.LookupSlackChannel(ctx, event.Type, prURL)
	if !found {
		return nil
	}

	// Wait for a few seconds to handle other asynchronous events
	// (e.g. a PR closure comment) before archiving the channel.
	_ = workflow.Sleep(ctx, 3*time.Second)

	state := "closed this PR"
	if event.Type == "fulfilled" {
		state = "merged this PR"
	}
	if reason := event.PullRequest.Reason; reason != "" {
		state = fmt.Sprintf("%s with this reason: `%s`", state, reason)
	} else {
		state += "."
	}
	bitbucket.MentionUserInMsg(ctx, channelID, event.Actor, "%s "+state)

	data.CleanupPRData(ctx, channelID, prURL)

	if err := activities.ArchiveChannel(ctx, channelID, prURL); err != nil {
		state = strings.Replace(state, " this PR", "", 1)
		_, _ = activities.PostMessage(ctx, channelID, ":boom: Failed to archive this channel, even though its PR was "+state)
		return err
	}

	return nil
}

// PullRequestUpdatedWorkflow mirrors various PR updates in the PR's Slack channel
// (such as title/description edits, reviewer changes, commit pushes, and branch retargeting):
// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Updated.2
func (c Config) PullRequestUpdatedWorkflow(ctx workflow.Context, event bitbucket.PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	prURL := bitbucket.HTMLURL(event.PullRequest.Links)
	channelID, found := bitbucket.LookupSlackChannel(ctx, event.Type, prURL)
	if !found {
		return nil
	}

	commits := bitbucket.Commits(ctx, event)
	event.PullRequest.CommitCount = len(commits)

	snapshot, err := bitbucket.SwitchSnapshot(ctx, prURL, event.PullRequest)
	if err != nil {
		return err
	}

	// Support PR data recovery.
	if snapshot == nil {
		bitbucket.InitPRData(ctx, event, channelID)
		return nil
	}

	defer bitbucket.UpdateChannelBookmarks(ctx, event, channelID, snapshot)

	// Announce transitions between drafts and ready to review.
	if !snapshot.Draft && event.PullRequest.Draft {
		bitbucket.MentionUserInMsg(ctx, channelID, event.Actor, "%s marked this PR as a draft. :construction:")
		return nil
	}
	if snapshot.Draft && !event.PullRequest.Draft {
		bitbucket.MentionUserInMsg(ctx, channelID, event.Actor, "%s marked this PR as ready for review. :eyes:")
		snapshot.Reviewers = nil // Force re-adding any reviewers that were added while the PR was a draft.
	}

	// Title edited.
	if snapshot.Title != event.PullRequest.Title {
		msg := ":pencil2: %s edited the PR title: " + bitbucket.LinkifyTitle(ctx, c.LinkifyMap, event.PullRequest.Title)
		bitbucket.MentionUserInMsg(ctx, channelID, event.Actor, msg)
		activities.SetChannelDescription(ctx, channelID, event.PullRequest.Title, prURL)
		err = bitbucket.RenameSlackChannel(ctx, event.PullRequest, channelID, c.SlackChannelNameMaxLength, c.SlackChannelNamePrefix)
	}

	// Description edited.
	if snapshot.Description != event.PullRequest.Description {
		msg := ":pencil2: %s deleted the PR description."
		if text := strings.TrimSpace(event.PullRequest.Description); text != "" {
			msg = ":pencil2: %s edited the PR description:\n\n" + markdown.BitbucketToSlack(ctx, text, prURL)
		}

		bitbucket.MentionUserInMsg(ctx, channelID, event.Actor, msg)
	}

	// Reviewers added/removed.
	added, removed := bitbucket.ReviewersDiff(*snapshot, event.PullRequest)
	if len(added)+len(removed) > 0 {
		bitbucket.MentionUserInMsg(ctx, channelID, event.Actor, bitbucket.ReviewerMentions(ctx, added, removed))
		if !event.PullRequest.Draft {
			_ = activities.InviteUsersToChannel(ctx, channelID, bitbucket.BitbucketToSlackIDs(ctx, added))
		}
		_ = activities.KickUsersFromChannel(ctx, channelID, bitbucket.BitbucketToSlackIDs(ctx, removed))
	}

	for _, id := range added {
		email, err := users.BitbucketToEmail(ctx, id)
		if err != nil {
			continue
		}
		if err := data.AddReviewerToPR(prURL, email); err != nil {
			logger.From(ctx).Error("failed to add reviewer to Bitbucket PR's attention state",
				slog.Any("error", err), slog.String("pr_url", prURL))
		}
	}

	for _, id := range removed {
		email, err := users.BitbucketToEmail(ctx, id)
		if err != nil {
			continue
		}
		if err := data.RemoveFromTurn(prURL, email); err != nil {
			logger.From(ctx).Error("failed to remove reviewers from Bitbucket PR's attention state",
				slog.Any("error", err), slog.String("pr_url", prURL))
		}
	}

	// Commit(s) pushed to the PR branch.
	if event.PullRequest.CommitCount > 0 && snapshot.Source.Commit.Hash != event.PullRequest.Source.Commit.Hash {
		if err := data.UpdateBitbucketDiffstat(prURL, bitbucket.Diffstat(ctx, event)); err != nil {
			logger.From(ctx).Error("failed to update Bitbucket PR's diffstat",
				slog.Any("error", err), slog.String("pr_url", prURL))
			// Don't abort - it's more important to announce this, even if our internal state is stale.
		}

		slices.Reverse(commits) // Switch from reverse order to chronological order.

		prevCount := snapshot.CommitCount
		if prevCount >= event.PullRequest.CommitCount {
			// Handle the unlikely ">" case where RevChat missed a commit push,
			// but more likely the "==" case where the user force-pushed a new head
			// (i.e. same number of commits) - by announcing just the last commit.
			prevCount = event.PullRequest.CommitCount - 1
		}
		commits = commits[prevCount:]

		var msg strings.Builder
		msg.WriteString(fmt.Sprintf("pushed <%s/commits|%d commit", prURL, len(commits)))
		if len(commits) != 1 {
			msg.WriteString("s")
		}

		msg.WriteString("> to this PR:")
		for _, c := range commits {
			title, _, _ := strings.Cut(c.Message, "\n")
			msg.WriteString(fmt.Sprintf("\n  •  <%s|`%s`> %s", c.Links["html"].HRef, c.Hash[:7], title))
		}

		bitbucket.MentionUserInMsg(ctx, channelID, event.Actor, "%s "+msg.String())
	}

	// Retargeted destination branch.
	oldBranch := snapshot.Destination.Branch.Name
	newBranch := event.PullRequest.Destination.Branch.Name
	if oldBranch != newBranch {
		repoURL := bitbucket.HTMLURL(event.Repository.Links)
		msg := "changed the target branch from <%s/branch/%s|`%s`> to <%s/branch/%s|`%s`>."
		msg = fmt.Sprintf(msg, repoURL, oldBranch, oldBranch, repoURL, newBranch, newBranch)
		bitbucket.MentionUserInMsg(ctx, channelID, event.Actor, "%s "+msg)
	}

	return err
}

// PullRequestReviewedWorkflow mirrors PR review results in the PR's Slack channel:
//   - https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Approved
//   - https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Approval-removed
//   - https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Changes-Request-created
//   - https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Changes-Request-removed
func PullRequestReviewedWorkflow(ctx workflow.Context, event bitbucket.PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	pr := event.PullRequest
	prURL := bitbucket.HTMLURL(pr.Links)
	channelID, found := bitbucket.LookupSlackChannel(ctx, event.Type, prURL)
	if !found {
		return nil
	}

	defer bitbucket.UpdateChannelBookmarks(ctx, event, channelID, nil)

	email, _ := users.BitbucketToEmail(ctx, event.Actor.AccountID)
	msg := "%s "

	switch event.Type {
	case "approved":
		if err := data.RemoveFromTurn(prURL, email); err != nil {
			logger.From(ctx).Error("failed to remove user from Bitbucket PR's attention state", slog.Any("error", err),
				slog.String("pr_url", prURL), slog.String("email", email), slog.String("account_id", event.Actor.AccountID))
			// Don't abort - it's more important to announce this, even if our internal state is stale.
		}
		msg += "approved this PR. :+1:"

	case "unapproved":
		if err := data.AddReviewerToPR(prURL, email); err != nil {
			logger.From(ctx).Error("failed to add user back to Bitbucket PR's attention state", slog.Any("error", err),
				slog.String("pr_url", prURL), slog.String("email", email), slog.String("account_id", event.Actor.AccountID))
			// Don't abort - it's more important to announce this, even if our internal state is stale.
		}
		if err := data.SwitchTurn(prURL, email); err != nil {
			logger.From(ctx).Error("failed to switch Bitbucket PR's attention state", slog.Any("error", err),
				slog.String("pr_url", prURL), slog.String("email", email), slog.String("account_id", event.Actor.AccountID))
			// Don't abort - it's more important to announce this, even if our internal state is stale.
		}
		msg += "unapproved this PR. :-1:"

	case "changes_request_created":
		pr.ChangeRequestCount++
		if _, err := bitbucket.SwitchSnapshot(ctx, prURL, pr); err != nil {
			logger.From(ctx).Error("failed to update change-request count in PR snapshot",
				slog.Any("error", err), slog.String("pr_url", prURL), slog.Int("new_count", pr.ChangeRequestCount))
			// Don't abort - it's more important to announce this, even if our internal state is stale.
		}

		if err := data.SwitchTurn(prURL, email); err != nil {
			logger.From(ctx).Error("failed to switch Bitbucket PR's attention state",
				slog.Any("error", err), slog.String("pr_url", prURL), slog.String("email", email))
			// Don't abort - it's more important to announce this, even if our internal state is stale.
		}
		msg += "requested changes in this PR. :warning:"

	case "changes_request_removed":
		pr.ChangeRequestCount--
		if pr.ChangeRequestCount < 0 {
			pr.ChangeRequestCount = 0 // Should not happen, but just in case.
		}
		if _, err := bitbucket.SwitchSnapshot(ctx, prURL, pr); err != nil {
			logger.From(ctx).Error("failed to update change-request count in PR snapshot",
				slog.Any("error", err), slog.String("pr_url", prURL), slog.Int("new_count", pr.ChangeRequestCount))
			// Don't abort - it's more important to announce this, even if our internal state is stale.
		}
		return nil

	default:
		logger.From(ctx).Error("unrecognized Bitbucket PR review event type", slog.String("event_type", event.Type))
		return nil
	}

	bitbucket.MentionUserInMsg(ctx, channelID, event.Actor, msg)
	return nil
}
