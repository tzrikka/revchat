package workflows

import (
	"errors"
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
	"github.com/tzrikka/revchat/pkg/slack"
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
	channelID, err := slack.CreateChannel(ctx, pr.ID, pr.Title, prURL, maxLen, prefix, private)
	if err != nil {
		// True = send this DM only if the user is opted-in.
		if userID := users.BitbucketIDToSlackID(ctx, event.Actor.AccountID, true); userID != "" {
			_ = activities.PostMessage(ctx, userID, "Failed to create Slack channel for "+prURL)
		}
		return err
	}

	bitbucket.InitPRData(ctx, event, channelID)

	// Channel cosmetics (before inviting users).
	activities.SetChannelTopic(ctx, channelID, prURL)
	activities.SetChannelDescription(ctx, channelID, pr.Title, prURL)
	bitbucket.SetChannelBookmarks(ctx, channelID, prURL, pr)

	msg := "%s created this PR: " + markdown.LinkifyTitle(ctx, c.LinkifyMap, prURL, pr.Title)
	if desc := strings.TrimSpace(pr.Description); desc != "" && desc != pr.Title {
		msg += "\n\n" + markdown.BitbucketToSlack(ctx, desc, prURL)
	}
	bitbucket.MentionUserInMsg(ctx, channelID, event.Actor, msg)

	followerIDs := data.SelectUserByBitbucketID(ctx, pr.Author.AccountID).Followers
	err = activities.InviteUsersToChannel(ctx, channelID, prURL, bitbucket.ChannelMembers(ctx, pr), followerIDs)
	if err != nil {
		// True = send this DM only if the user is opted-in.
		if userID := users.BitbucketIDToSlackID(ctx, event.Actor.AccountID, true); userID != "" {
			_ = activities.PostMessage(ctx, userID, "Failed to create Slack channel for "+prURL)
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
	channelID, found := activities.LookupChannel(ctx, prURL)
	if !found {
		return nil
	}

	// Wait for a few seconds to handle other asynchronous events
	// (e.g. a PR closure comment) before archiving the channel.
	_ = workflow.Sleep(ctx, 3*time.Second)

	msg := "%s closed this PR"
	if event.Type == "fulfilled" {
		msg = "%s merged this PR"
	}
	if reason := event.PullRequest.Reason; reason != "" {
		msg = fmt.Sprintf("%s with this reason: `%s`", msg, reason)
	} else {
		msg += "."
	}
	bitbucket.MentionUserInMsg(ctx, channelID, event.Actor, msg)
	data.CleanupPRData(ctx, channelID, prURL)

	if err := activities.ArchiveChannel(ctx, channelID, prURL); err != nil {
		msg = strings.Replace(msg, " this PR", "", 1)
		_ = activities.PostMessage(ctx, channelID, ":boom: Failed to archive this channel, even though its PR was "+msg)
		return err
	}

	return nil
}

// PullRequestUpdatedWorkflow mirrors various PR updates in the PR's Slack channel
// (such as title/description edits, reviewer changes, commit pushes, and branch retargeting):
// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Updated.2
func (c Config) PullRequestUpdatedWorkflow(ctx workflow.Context, event bitbucket.PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	pr := event.PullRequest
	prURL := bitbucket.HTMLURL(pr.Links)
	channelID, found := activities.LookupChannel(ctx, prURL)
	if !found {
		return nil
	}

	commits := bitbucket.Commits(ctx, event)
	pr.CommitCount = len(commits)

	snapshot, err := bitbucket.SwitchSnapshot(ctx, prURL, pr)
	if err != nil {
		return err
	}

	// Support PR data recovery.
	if snapshot == nil {
		bitbucket.InitPRData(ctx, event, channelID)
		return nil
	}

	defer bitbucket.UpdateChannelBookmarks(ctx, pr, prURL, channelID)

	var errs []error

	// Announce transitions between draft and ready-to-review modes.
	if !snapshot.Draft && pr.Draft {
		bitbucket.MentionUserInMsg(ctx, channelID, event.Actor, "%s marked this PR as a draft. :construction:")
	} else if snapshot.Draft && !pr.Draft {
		bitbucket.MentionUserInMsg(ctx, channelID, event.Actor, "%s marked this PR as ready for review. :eyes:")
		errs = append(errs, activities.InviteUsersToChannel(ctx, channelID, prURL, bitbucket.ChannelMembers(ctx, pr), nil))
	}

	// Title edited.
	if snapshot.Title != pr.Title {
		msg := ":pencil2: %s edited the PR title: " + markdown.LinkifyTitle(ctx, c.LinkifyMap, prURL, pr.Title)
		bitbucket.MentionUserInMsg(ctx, channelID, event.Actor, msg)
		activities.SetChannelDescription(ctx, channelID, pr.Title, prURL)
		err := slack.RenameChannel(ctx, pr.ID, pr.Title, prURL, channelID, c.SlackChannelNameMaxLength, c.SlackChannelNamePrefix)
		errs = append(errs, err)
	}

	// Description edited.
	if snapshot.Description != pr.Description {
		msg := ":pencil2: %s deleted the PR description."
		if text := strings.TrimSpace(pr.Description); text != "" {
			msg = ":pencil2: %s edited the PR description:\n\n" + markdown.BitbucketToSlack(ctx, text, prURL)
		}

		bitbucket.MentionUserInMsg(ctx, channelID, event.Actor, msg)
	}

	// Reviewers added/removed.
	added, removed := bitbucket.ReviewersDiff(*snapshot, pr)
	if len(added)+len(removed) > 0 {
		bitbucket.MentionUserInMsg(ctx, channelID, event.Actor, bitbucket.ReviewerMentions(ctx, added, removed))
		if !pr.Draft {
			errs = append(errs, activities.InviteUsersToChannel(ctx, channelID, prURL, bitbucket.BitbucketToSlackIDs(ctx, added), nil))
		}
		errs = append(errs, activities.KickUsersFromChannel(ctx, channelID, prURL, bitbucket.BitbucketToSlackIDs(ctx, removed)))
	}

	// Retargeted destination branch.
	oldBranch := snapshot.Destination.Branch.Name
	newBranch := pr.Destination.Branch.Name
	if oldBranch != newBranch {
		repoURL := bitbucket.HTMLURL(event.Repository.Links)
		msg := "changed the target branch from <%s/branch/%s|`%s`> to <%s/branch/%s|`%s`>."
		msg = fmt.Sprintf(msg, repoURL, oldBranch, oldBranch, repoURL, newBranch, newBranch)
		bitbucket.MentionUserInMsg(ctx, channelID, event.Actor, "%s "+msg)
	}

	// Commit(s) pushed to the PR branch.
	if pr.CommitCount > 0 && snapshot.Source.Commit.Hash != pr.Source.Commit.Hash {
		if err := data.UpdateBitbucketDiffstat(prURL, bitbucket.Diffstat(ctx, event)); err != nil {
			logger.From(ctx).Error("failed to update Bitbucket PR diffstat",
				slog.Any("error", err), slog.String("pr_url", prURL))
			errs = append(errs, err)
			// Don't abort - it's more important to announce this, even if our internal state is stale.
		}

		slices.Reverse(commits) // Switch from reverse order to chronological order.

		prevCount := snapshot.CommitCount
		if prevCount >= pr.CommitCount {
			// Handle the unlikely ">" case where RevChat missed a commit push,
			// but more likely the "==" case where the user force-pushed a new head
			// (i.e. same number of commits) - by announcing just the last commit.
			prevCount = pr.CommitCount - 1
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

	return errors.Join(errs...)
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
	channelID, found := activities.LookupChannel(ctx, prURL)
	if !found {
		return nil
	}

	defer bitbucket.UpdateChannelBookmarks(ctx, event.PullRequest, prURL, channelID)

	email := users.BitbucketIDToEmail(ctx, event.Actor.AccountID)
	msg := "%s "
	var err error

	switch event.Type {
	case "approved":
		msg += "approved this PR. :+1:"
		err = data.RemoveReviewerFromTurns(ctx, prURL, email, true)
		// Don't abort - it's more important to announce this, even if our internal state is stale.

	case "unapproved":
		msg += "unapproved this PR. :-1:"
		// If the user isn't opted-in, or isn't a member of the Slack channel, don't add them back to the
		// PR's attention state (just like the logic in other places, e.g. PR creation and PR updates).
		if user := data.SelectUserByEmail(ctx, email); user.IsOptedIn() {
			err = activities.InviteUsersToChannel(ctx, channelID, prURL, []string{users.EmailToSlackID(ctx, email)}, nil)
			// Don't abort - it's more important to announce this, even if our internal state is stale.
		}

	case "changes_request_created":
		pr.ChangeRequestCount++
		msg += "requested changes in this PR. :warning:"
		_, err = bitbucket.SwitchSnapshot(ctx, prURL, pr)
		if err != nil {
			logger.From(ctx).Error("failed to update change-request count in PR snapshot",
				slog.Any("error", err), slog.String("pr_url", prURL), slog.Int("new_count", pr.ChangeRequestCount))
			// Don't abort - it's more important to announce this, even if our internal state is stale.
		}
		if err2 := data.SwitchTurn(ctx, prURL, email); err2 != nil {
			err = errors.Join(err, err2)
			// Don't abort - it's more important to announce this, even if our internal state is stale.
		}

	case "changes_request_removed":
		pr.ChangeRequestCount--
		if pr.ChangeRequestCount < 0 {
			pr.ChangeRequestCount = 0 // Should not happen, but just in case.
		}
		_, err = bitbucket.SwitchSnapshot(ctx, prURL, pr)
		if err != nil {
			logger.From(ctx).Error("failed to update change-request count in PR snapshot",
				slog.Any("error", err), slog.String("pr_url", prURL), slog.Int("new_count", pr.ChangeRequestCount))
			// Don't abort - it's more important to announce this, even if our internal state is stale.
		}

	default:
		logger.From(ctx).Error("unrecognized Bitbucket PR review event type", slog.String("event_type", event.Type))
		return nil
	}

	bitbucket.MentionUserInMsg(ctx, channelID, event.Actor, msg)
	return err
}
