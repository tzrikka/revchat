package workflows

import (
	"errors"
	"log/slog"
	"strings"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/github"
	"github.com/tzrikka/revchat/pkg/markdown"
	"github.com/tzrikka/revchat/pkg/slack"
	"github.com/tzrikka/revchat/pkg/slack/activities"
	"github.com/tzrikka/revchat/pkg/users"
)

// PullRequestWorkflow is an entrypoint to handle all GitHub pull request events:
// https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request
func (c Config) PullRequestWorkflow(ctx workflow.Context, event github.PullRequestEvent) error {
	switch event.Action {
	case "opened":
		return c.prOpened(ctx, event)
	case "closed":
		return prClosed(ctx, event)
	// case "reopened":
	// 	return c.prReopened(ctx, event)

	// case "converted_to_draft":
	// 	return c.prConvertedToDraft(ctx, event)
	// case "ready_for_review":
	// 	return c.prReadyForReview(ctx, event)

	// case "review_requested", "review_request_removed", "assigned", "unassigned":
	// 	return c.prReviewRequests(ctx, event)

	// case "edited": // Title, body, base branch.
	// 	return c.prEdited(ctx, event)
	// case "synchronize": // Head branch.
	// 	return c.prSynchronized(ctx, event)

	// Ignored actions.
	case "auto_merge_enabled", "auto_merge_disabled":
	case "enqueued", "dequeued":
	case "labeled", "unlabeled":
	case "locked", "unlocked":
	case "milestoned", "demilestoned":

	default:
		logger.From(ctx).Error("unrecognized GitHub PR event action", slog.String("action", event.Action))
		return errors.New("unrecognized GitHub PR event action: " + event.Action)
	}

	return nil
}

// prOpened initializes a new Slack channel for a newly-created or reopened PR.
func (c Config) prOpened(ctx workflow.Context, event github.PullRequestEvent) error {
	pr := event.PullRequest

	maxLen, prefix, private := c.SlackChannelNameMaxLength, c.SlackChannelNamePrefix, c.SlackChannelsArePrivate
	channelID, err := slack.CreateChannel(ctx, pr.Number, pr.Title, pr.HTMLURL, maxLen, prefix, private)
	if err != nil {
		// True = send this DM only if the user is opted-in.
		if userID := users.GitHubIDToSlackID(ctx, event.Sender.Login, true); userID != "" {
			_ = activities.PostMessage(ctx, userID, "Failed to create Slack channel for "+pr.HTMLURL)
		}
		return err
	}

	github.InitPRData(ctx, event, channelID)

	// Channel cosmetics.
	activities.SetChannelTopic(ctx, channelID, pr.HTMLURL)
	activities.SetChannelDescription(ctx, channelID, pr.Title, pr.HTMLURL)
	github.SetChannelBookmarks(ctx, channelID, pr.HTMLURL, pr)

	msg := "%s created this PR: " + markdown.LinkifyTitle(ctx, c.LinkifyMap, pr.HTMLURL, pr.Title)
	if event.Action == "reopened" {
		msg = strings.Replace(msg, "created", "reopened", 1)
	}
	if pr.Body != nil && strings.TrimSpace(*pr.Body) != "" && *pr.Body != pr.Title {
		msg += "\n\n" + markdown.GitHubToSlack(ctx, *pr.Body, pr.HTMLURL)
	}
	github.MentionUserInMsg(ctx, channelID, event.Sender, msg)

	err = activities.InviteUsersToChannel(ctx, channelID, pr.HTMLURL, github.ChannelMembers(ctx, pr))
	if err != nil {
		// True = send this DM only if the user is opted-in.
		if userID := users.GitHubIDToSlackID(ctx, event.Sender.Login, true); userID != "" {
			_ = activities.PostMessage(ctx, userID, "Failed to create Slack channel for "+pr.HTMLURL)
		}
		// Undo channel creation and PR data initialization.
		_ = activities.ArchiveChannel(ctx, channelID, pr.HTMLURL)
		data.CleanupPRData(ctx, channelID, pr.HTMLURL)
		return err
	}

	return nil
}

// lookupChannel returns the ID of a Slack channel associated with the given PR, if it exists.
func lookupChannel(ctx workflow.Context, pr github.PullRequest) (string, bool) {
	if pr.State == "closed" {
		logger.From(ctx).Debug("ignoring GitHub event - the PR is closed", slog.String("pr_url", pr.HTMLURL))
		return "", false
	}
	return activities.LookupChannel(ctx, pr.HTMLURL)
}

// prClosed archives a PR's Slack channel when the PR is closed.
func prClosed(ctx workflow.Context, event github.PullRequestEvent) error {
	// If we're not tracking this PR, there's no channel to archive.
	channelID, found := lookupChannel(ctx, event.PullRequest)
	if !found {
		return nil
	}

	// Wait for a few seconds to handle other asynchronous events
	// (e.g. a PR closure comment) before archiving the channel.
	_ = workflow.Sleep(ctx, 3*time.Second)

	msg := "%s closed this PR."
	if event.PullRequest.Merged {
		msg = "%s merged this PR."
	}
	github.MentionUserInMsg(ctx, channelID, event.Sender, msg)

	data.CleanupPRData(ctx, channelID, event.PullRequest.HTMLURL)

	if err := activities.ArchiveChannel(ctx, channelID, event.PullRequest.HTMLURL); err != nil {
		msg = strings.Replace(msg, " this PR", "", 1)
		_ = activities.PostMessage(ctx, channelID, ":boom: Failed to archive this channel, even though its PR was "+msg)
		return err
	}

	return nil
}
