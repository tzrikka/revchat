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
		// True = send an error DM only if the user is opted-in.
		if userID := users.BitbucketIDToSlackID(ctx, event.Actor.AccountID, true); userID != "" {
			err = errors.Join(err, activities.PostMessage(ctx, userID, "Failed to create a Slack channel for "+prURL))
		}
		return activities.AlertError(ctx, c.SlackAlertsChannel, "failed to create Slack channel for "+prURL, err)
	}

	bitbucket.InitPRData(ctx, event, channelID, c.SlackAlertsChannel)

	// Channel cosmetics (before inviting users).
	activities.SetChannelTopic(ctx, channelID, prURL)
	activities.SetChannelDescription(ctx, channelID, pr.Title, prURL, "")
	bitbucket.SetChannelBookmarks(ctx, channelID, prURL, pr)

	msg := "%s created this PR: " + markdown.LinkifyTitle(ctx, c.LinkifyMap, prURL, pr.Title)
	if desc := strings.TrimSpace(pr.Description); desc != "" && desc != pr.Title {
		msg += "\n\n" + markdown.BitbucketToSlack(ctx, desc, prURL)
	}
	bitbucket.MentionUserInMsg(ctx, channelID, event.Actor, msg)

	followerIDs := data.SelectUserByBitbucketID(ctx, pr.Author.AccountID).Followers
	err = activities.InviteUsersToChannel(ctx, channelID, prURL, bitbucket.ChannelMembers(ctx, pr), followerIDs)
	if err != nil {
		// True = send an error DM only if the user is opted-in.
		if userID := users.BitbucketIDToSlackID(ctx, event.Actor.AccountID, true); userID != "" {
			err = errors.Join(err, activities.PostMessage(ctx, userID, "Failed to initialize a Slack channel for "+prURL))
		}
		// Undo channel creation and PR data initialization.
		err = errors.Join(err, activities.ArchiveChannel(ctx, channelID, prURL))
		data.CleanupPRData(ctx, channelID, prURL)
		return activities.AlertError(ctx, c.SlackAlertsChannel, "failed to invite users to Slack channel for "+prURL, err)
	}

	return nil
}

// PullRequestClosedWorkflow archives a PR's Slack channel when the PR is merged or declined/rejected:
//   - https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Merged
//   - https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Declined
func (c Config) PullRequestClosedWorkflow(ctx workflow.Context, event bitbucket.PullRequestEvent) error {
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
		msg = ":boom: Failed to archive this channel, even though its PR was " + strings.Replace(msg, " this PR", "", 1)
		err = errors.Join(err, activities.PostMessage(ctx, channelID, msg))
		return activities.AlertError(ctx, c.SlackAlertsChannel, "failed to archive Slack channel for "+prURL, err)
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
		bitbucket.InitPRData(ctx, event, channelID, c.SlackAlertsChannel)
		return nil
	}

	defer bitbucket.UpdateChannelBookmarks(ctx, pr, prURL, channelID)

	email := data.BitbucketIDToEmail(ctx, event.Actor.AccountID)
	var errs []error

	// Announce transitions between draft and ready-to-review modes.
	if !snapshot.Draft && pr.Draft {
		bitbucket.MentionUserInMsg(ctx, channelID, event.Actor, "%s marked this PR as a draft. :construction:")
		_, _, _ = data.Nudge(ctx, prURL, data.BitbucketIDToEmail(ctx, event.PullRequest.Author.AccountID))
	} else if snapshot.Draft && !pr.Draft {
		bitbucket.MentionUserInMsg(ctx, channelID, event.Actor, "%s marked this PR as ready for review. :eyes:")
		_ = data.SwitchTurn(ctx, prURL, email, true)

		errs = append(errs, activities.InviteUsersToChannel(ctx, channelID, prURL, bitbucket.ChannelMembers(ctx, pr), nil))
	}

	// Title edited.
	if snapshot.Title != pr.Title {
		msg := ":pencil2: %s edited the PR title: " + markdown.LinkifyTitle(ctx, c.LinkifyMap, prURL, pr.Title)
		bitbucket.MentionUserInMsg(ctx, channelID, event.Actor, msg)

		activities.SetChannelDescription(ctx, channelID, pr.Title, prURL, email)

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
		data.UpdateActivityTime(ctx, prURL, email)
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
		data.UpdateActivityTime(ctx, prURL, email)
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

		msg := new(strings.Builder)
		fmt.Fprintf(msg, "pushed <%s/commits|%d commit", prURL, len(commits))
		if len(commits) != 1 {
			msg.WriteString("s")
		}

		msg.WriteString("> to this PR:")
		for _, c := range commits {
			title, _, _ := strings.Cut(c.Message, "\n")
			fmt.Fprintf(msg, "\n  •  <%s|`%s`> %s", c.Links["html"].HRef, c.Hash[:7], title)
		}

		bitbucket.MentionUserInMsg(ctx, channelID, event.Actor, "%s "+msg.String())
		data.UpdateActivityTime(ctx, prURL, email)
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
		data.UpdateActivityTime(ctx, prURL, email)

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

		if err2 := data.SwitchTurn(ctx, prURL, email, false); err2 != nil {
			err = errors.Join(err, err2)
			// Don't abort - it's more important to announce this, even if our internal state is stale.
		}

	case "changes_request_removed":
		data.UpdateActivityTime(ctx, prURL, email)

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
