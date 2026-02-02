package workflows

import (
	"errors"
	"fmt"
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
	case "opened", "reopened":
		return c.prOpened(ctx, event)
	case "closed":
		return c.prClosed(ctx, event)

	case "converted_to_draft":
		return prConvertedToDraft(ctx, event)
	case "ready_for_review":
		return prReadyForReview(ctx, event)

	case "assigned", "unassigned":
		fallthrough
	case "review_requested", "review_request_removed":
		return prReviewRequests(ctx, event)

	case "edited": // Title, body, base branch.
		return c.prEdited(ctx, event)
	case "synchronize": // Head branch.
		return prSynchronized(ctx, event)

	case "locked", "unlocked":
		return prLocked(ctx, event)

	// Ignored actions.
	case "auto_merge_enabled", "auto_merge_disabled":
	case "enqueued", "dequeued":
	case "labeled", "unlabeled":
	case "milestoned", "demilestoned":

	default:
		logger.From(ctx).Error("unrecognized GitHub PR event action", slog.String("action", event.Action))
		return errors.New("unrecognized GitHub PR event action: " + event.Action)
	}

	return nil
}

// prOpened initializes a new Slack channel for a newly-created or reopened PR.
//
// Why are reopened PRs handled here too? See this Slack bug notice
// in https://docs.slack.dev/reference/methods/conversations.unarchive:
// bot tokens ("xoxb-...") cannot currently be used to unarchive conversations.
// Use a user token ("xoxp-...") to unarchive conversations rather than a bot token.
//
// Partial workaround: treat "reopened" events as "opened". Drawback: losing pre-archiving channel history.
func (c Config) prOpened(ctx workflow.Context, event github.PullRequestEvent) error {
	pr := event.PullRequest

	maxLen, prefix, private := c.SlackChannelNameMaxLength, c.SlackChannelNamePrefix, c.SlackChannelsArePrivate
	channelID, err := slack.CreateChannel(ctx, pr.Number, pr.Title, pr.HTMLURL, maxLen, prefix, private)
	if err != nil {
		// True = send an error DM only if the user is opted-in.
		if userID := users.GitHubIDToSlackID(ctx, event.Sender.Login, true); userID != "" {
			err = errors.Join(err, activities.PostMessage(ctx, userID, "Failed to create a Slack channel for "+pr.HTMLURL))
		}
		return activities.AlertError(ctx, c.SlackAlertsChannel, "failed to create Slack channel for "+pr.HTMLURL, err)
	}

	github.InitPRData(ctx, event, channelID, c.SlackAlertsChannel)

	// Channel cosmetics.
	activities.SetChannelTopic(ctx, channelID, pr.HTMLURL)
	activities.SetChannelDescription(ctx, channelID, pr.Title, pr.HTMLURL, "")
	github.SetChannelBookmarks(ctx, channelID, pr.HTMLURL, pr)

	msg := "%s created this PR: " + markdown.LinkifyTitle(ctx, c.LinkifyMap, pr.HTMLURL, pr.Title)
	if event.Action == "reopened" {
		msg = strings.Replace(msg, "created", "reopened", 1)
	}
	if body := strings.TrimSpace(pr.Body); body != "" && body != pr.Title {
		msg += "\n\n" + markdown.GitHubToSlack(ctx, body, pr.HTMLURL)
	}
	github.MentionUserInMsg(ctx, channelID, event.Sender, msg)

	followerIDs := data.SelectUserByGitHubID(ctx, pr.User.Login).Followers
	err = activities.InviteUsersToChannel(ctx, channelID, pr.HTMLURL, github.ChannelMembers(ctx, pr), followerIDs)
	if err != nil {
		// True = send an error DM only if the user is opted-in.
		if userID := users.GitHubIDToSlackID(ctx, event.Sender.Login, true); userID != "" {
			err = errors.Join(err, activities.PostMessage(ctx, userID, "Failed to initialize a Slack channel for "+pr.HTMLURL))
		}
		// Undo channel creation and PR data initialization.
		err = errors.Join(err, activities.ArchiveChannel(ctx, channelID, pr.HTMLURL))
		data.CleanupPRData(ctx, channelID, pr.HTMLURL)
		return activities.AlertError(ctx, c.SlackAlertsChannel, "failed to invite users to Slack channel for "+pr.HTMLURL, err)
	}

	return nil
}

// prClosed archives a PR's Slack channel when the PR is closed.
func (c Config) prClosed(ctx workflow.Context, event github.PullRequestEvent) error {
	// If we're not tracking this PR, there's no channel to archive.
	prURL := event.PullRequest.HTMLURL
	channelID, found := activities.LookupChannel(ctx, prURL)
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

	data.CleanupPRData(ctx, channelID, prURL)

	if err := activities.ArchiveChannel(ctx, channelID, prURL); err != nil {
		msg = ":boom: Failed to archive this channel, even though its PR was " + strings.Replace(msg, " this PR", "", 1)
		err = errors.Join(err, activities.PostMessage(ctx, channelID, msg))
		return activities.AlertError(ctx, c.SlackAlertsChannel, "failed to archive Slack channel for "+prURL, err)
	}

	return nil
}

// prConvertedToDraft announces that a PR was converted to a draft.
// For more information, see "Changing the stage of a pull request":
// https://docs.github.com/pull-requests/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/changing-the-stage-of-a-pull-request
func prConvertedToDraft(ctx workflow.Context, event github.PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	channelID, found := activities.LookupChannel(ctx, event.PullRequest.HTMLURL)
	if !found {
		return nil
	}

	github.MentionUserInMsg(ctx, channelID, event.Sender, "%s marked this PR as a draft. :construction:")
	_, _, _ = data.Nudge(ctx, event.PullRequest.HTMLURL, users.GitHubIDToEmail(ctx, event.PullRequest.User.Login))
	return nil
}

// prReadyForReview announces that a draft PR was marked as ready for review.
// For more information, see "Changing the stage of a pull request":
// https://docs.github.com/pull-requests/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/changing-the-stage-of-a-pull-request
func prReadyForReview(ctx workflow.Context, event github.PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	channelID, found := activities.LookupChannel(ctx, event.PullRequest.HTMLURL)
	if !found {
		return nil
	}

	github.MentionUserInMsg(ctx, channelID, event.Sender, "%s marked this PR as ready for review. :eyes:")
	_ = data.SwitchTurn(ctx, event.PullRequest.HTMLURL, users.GitHubIDToEmail(ctx, event.Sender.Login), true)

	return activities.InviteUsersToChannel(ctx, channelID, event.PullRequest.HTMLURL, github.ChannelMembers(ctx, event.PullRequest), nil)
}

// lookupChannel returns the ID of a Slack channel associated with the given PR, if it exists.
func lookupChannel(ctx workflow.Context, pr github.PullRequest) (string, bool) {
	if pr.State == "closed" {
		logger.From(ctx).Debug("ignoring GitHub event - the PR is closed", slog.String("pr_url", pr.HTMLURL))
		return "", false
	}
	return activities.LookupChannel(ctx, pr.HTMLURL)
}

// Review by a person or team was requested for or removed from a PR, or un/assigned
// to/from a specific person. For more information, see "Requesting a pull request review":
// https://docs.github.com/pull-requests/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/requesting-a-pull-request-review
func prReviewRequests(ctx workflow.Context, event github.PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	channelID, found := lookupChannel(ctx, event.PullRequest)
	if !found {
		return nil
	}

	defer github.UpdateChannelBookmarks(ctx, &event.PullRequest, nil, channelID)

	prURL := event.PullRequest.HTMLURL
	var errs []error

	// Individual assignee.
	if user := event.Assignee; user != nil {
		switch event.Action {
		case "assigned":
			msg := github.ReviewerMentions(ctx, "set", "assignee", []github.User{*user})
			github.MentionUserInMsg(ctx, channelID, event.Sender, msg)
			if !event.PullRequest.Draft {
				id := github.LoginsToSlackIDs(ctx, []string{user.Login})
				errs = append(errs, activities.InviteUsersToChannel(ctx, channelID, prURL, id, nil))
			}
		case "unassigned":
			msg := github.ReviewerMentions(ctx, "removed", "assignee", []github.User{*user})
			github.MentionUserInMsg(ctx, channelID, event.Sender, msg)
		}
	}

	// Individual reviewer.
	if user := event.RequestedReviewer; user != nil {
		ids := github.LoginsToSlackIDs(ctx, []string{user.Login})

		switch event.Action {
		case "review_requested":
			msg := github.ReviewerMentions(ctx, "added", "reviewer", []github.User{*user})
			github.MentionUserInMsg(ctx, channelID, event.Sender, msg)
			if !event.PullRequest.Draft {
				errs = append(errs, activities.InviteUsersToChannel(ctx, channelID, prURL, ids, nil))
			}
		case "review_request_removed":
			msg := github.ReviewerMentions(ctx, "removed", "reviewer", []github.User{*user})
			github.MentionUserInMsg(ctx, channelID, event.Sender, msg)
			errs = append(errs, activities.KickUsersFromChannel(ctx, channelID, prURL, ids))
		}
	}

	// Reviewing team.
	if team := event.RequestedTeam; team != nil {
		ids := []string{}

		switch event.Action {
		case "review_requested":
			msg := ":bust_in_silhouette: %s added this team as reviewers: "
			msg = fmt.Sprintf("%s <%s?preview=no|%s>.", msg, team.HTMLURL, team.Name)
			github.MentionUserInMsg(ctx, channelID, event.Sender, msg)
			if !event.PullRequest.Draft {
				errs = append(errs, activities.InviteUsersToChannel(ctx, channelID, prURL, ids, nil))
			}
		case "review_request_removed":
			msg := ":bust_in_silhouette: %s removed this team as reviewers: "
			msg = fmt.Sprintf("%s <%s?preview=no|%s>.", msg, team.HTMLURL, team.Name)
			github.MentionUserInMsg(ctx, channelID, event.Sender, msg)
			errs = append(errs, activities.KickUsersFromChannel(ctx, channelID, prURL, ids))
		}
	}

	return errors.Join(errs...)
}

// prEdited announces that the title or body of a PR was edited, or the base branch was changed.
func (c Config) prEdited(ctx workflow.Context, event github.PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	pr := event.PullRequest
	channelID, found := lookupChannel(ctx, pr)
	if !found {
		return nil
	}

	defer github.UpdateChannelBookmarks(ctx, &pr, nil, channelID)

	email := users.GitHubIDToEmail(ctx, event.Sender.Login)

	// Title was changed.
	var err error
	if event.Changes.Title != nil {
		msg := ":pencil2: %s edited the PR title: " + markdown.LinkifyTitle(ctx, c.LinkifyMap, pr.HTMLURL, pr.Title)
		github.MentionUserInMsg(ctx, channelID, event.Sender, msg)

		activities.SetChannelDescription(ctx, channelID, pr.Title, pr.HTMLURL, email)

		maxLen, prefix := c.SlackChannelNameMaxLength, c.SlackChannelNamePrefix
		err = slack.RenameChannel(ctx, pr.Number, pr.Title, pr.HTMLURL, channelID, maxLen, prefix)
	}

	// Description body was changed.
	if event.Changes.Body != nil {
		msg := ":pencil2: %s deleted the PR description."
		if body := strings.TrimSpace(pr.Body); body != "" {
			msg = ":pencil2: %s edited the PR description:\n\n" + markdown.GitHubToSlack(ctx, body, pr.HTMLURL)
		}
		github.MentionUserInMsg(ctx, channelID, event.Sender, msg)
		data.UpdateActivityTime(ctx, pr.HTMLURL, email)
	}

	// Base branch was changed.
	if event.Changes.Base != nil {
		msg := "changed the base branch from <%s/tree/%s|`%s`> to <%s/tree/%s|`%s`>."
		repoURL, oldBranch, newBranch := pr.Base.Repo.HTMLURL, event.Changes.Base.Ref, pr.Base.Ref
		msg = fmt.Sprintf(msg, repoURL, oldBranch, oldBranch, repoURL, newBranch, newBranch)
		github.MentionUserInMsg(ctx, channelID, event.Sender, "%s "+msg)
		data.UpdateActivityTime(ctx, pr.HTMLURL, email)
	}

	return err
}

// prSynchronized announces that a PR's head branch was updated. For example, the head
// branch was updated from the base branch or new commits were pushed to the head branch.
func prSynchronized(ctx workflow.Context, event github.PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	channelID, found := lookupChannel(ctx, event.PullRequest)
	if !found {
		return nil
	}

	data.UpdateActivityTime(ctx, event.PullRequest.HTMLURL, users.GitHubIDToEmail(ctx, event.Sender.Login))
	defer github.UpdateChannelBookmarks(ctx, &event.PullRequest, nil, channelID)

	if event.After == nil {
		logger.From(ctx).Warn("'after' field in GitHub PR synchronize event is nil")
		return nil
	}

	after := *event.After
	msg := fmt.Sprintf("pushed commit [`%s`](%s/commits/%s) into the head branch", after[:7], event.PullRequest.HTMLURL, after)
	github.MentionUserInMsg(ctx, channelID, event.Sender, "%s "+msg)

	return nil
}

// prLocked announces that conversation on a PR was locked or unlocked. For more information, see "Locking conversations":
// https://docs.github.com/en/communities/moderating-comments-and-conversations/locking-conversations
func prLocked(ctx workflow.Context, event github.PullRequestEvent) error {
	logger.From(ctx).Warn(fmt.Sprintf("GitHub PR %s event - not implemented yet", event.Action))
	return nil
}
