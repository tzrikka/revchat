package github

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/markdown"
	"github.com/tzrikka/revchat/pkg/slack"
	"github.com/tzrikka/revchat/pkg/users"
	tslack "github.com/tzrikka/timpani-api/pkg/slack"
)

func (c Config) archiveChannel(ctx workflow.Context, event PullRequestEvent) error {
	// If we're not tracking this PR, there's no channel to archive.
	channelID, found := lookupChannel(ctx, event.Action, event.PullRequest)
	if !found {
		return nil
	}

	// Wait for a few seconds to handle other asynchronous events
	// (e.g. a PR closure comment) before archiving the channel.
	_ = workflow.Sleep(ctx, 5*time.Second)

	c.cleanupPRData(ctx, event.PullRequest.HTMLURL)

	state := event.Action + " this PR"
	if event.Action == "closed" && event.PullRequest.Merged {
		state = "merged this PR"
	}
	if event.Action == "converted_to_draft" {
		state = "converted this PR to a draft"
	}

	_ = c.mentionUserInMsg(ctx, channelID, event.Sender, "%s "+state)

	if err := tslack.ConversationsArchive(ctx, channelID); err != nil {
		state = strings.Replace(state, " this PR", "", 1)
		msg := "Failed to archive this channel, even though its PR was " + state
		_, _ = slack.PostMessage(ctx, channelID, msg)

		return err
	}

	return nil
}

// lookupChannel returns the ID of a channel associated
// with a PR, if the PR is active and the channel is found.
func lookupChannel(ctx workflow.Context, action string, pr PullRequest) (string, bool) {
	if pr.Draft {
		return "", false
	}

	channelID, err := data.SwitchURLAndID(pr.HTMLURL)
	if err != nil {
		logger.From(ctx).Error("failed to retrieve PR's Slack channel ID", slog.Any("error", err),
			slog.String("action", action), slog.String("pr_url", pr.HTMLURL))
		return "", false
	}

	return channelID, channelID != ""
}

// cleanupPRData deletes all data associated with a PR. If there are errors,
// they are logged but ignored, as they do not affect the overall workflow.
func (c Config) cleanupPRData(ctx workflow.Context, url string) {
	if err := data.DeleteURLAndIDMapping(url); err != nil {
		logger.From(ctx).Error("failed to delete PR URL / Slack channel mappings",
			slog.Any("error", err), slog.String("pr_url", url))
	}
}

func (c Config) initChannel(ctx workflow.Context, event PullRequestEvent) error {
	pr := event.PullRequest
	url := pr.HTMLURL

	channelID, err := c.createChannel(ctx, pr)
	if err != nil {
		c.reportCreationErrorToAuthor(ctx, event.Sender.Login, url)
		return err
	}

	// Map the PR to the Slack channel ID, for 2-way event syncs.
	if err := data.MapURLAndID(url, channelID); err != nil {
		logger.From(ctx).Error("failed to save PR URL / Slack channel mapping",
			slog.Any("error", err), slog.String("channel_id", channelID), slog.String("pr_url", url))
		c.reportCreationErrorToAuthor(ctx, event.Sender.Login, url)
		c.cleanupPRData(ctx, url)
		return err
	}

	// Channel cosmetics.
	slack.SetChannelTopic(ctx, channelID, url)
	slack.SetChannelDescription(ctx, channelID, pr.Title, url)
	c.setChannelBookmarks(ctx, channelID, url, pr)
	c.postIntroMsg(ctx, channelID, event.Action, pr, event.Sender)

	err = slack.InviteUsersToChannel(ctx, channelID, c.prParticipants(ctx, pr))
	if err != nil {
		c.reportCreationErrorToAuthor(ctx, event.Sender.Login, url)
		c.cleanupPRData(ctx, url)
		return err
	}

	return nil
}

func (c Config) createChannel(ctx workflow.Context, pr PullRequest) (string, error) {
	title := slack.NormalizeChannelName(pr.Title, c.SlackChannelNameMaxLength)

	for i := 1; i < 50; i++ {
		name := fmt.Sprintf("%s-%d_%s", c.SlackChannelNamePrefix, pr.Number, title)
		if i > 1 {
			name = fmt.Sprintf("%s_%d", name, i)
		}

		id, retry, err := slack.CreateChannel(ctx, name, pr.HTMLURL, c.SlackChannelsArePrivate)
		if err != nil {
			if retry {
				continue
			} else {
				return "", err
			}
		}

		return id, nil
	}

	msg := "too many failed attempts to create Slack channel"
	logger.From(ctx).Error(msg, slog.String("pr_url", pr.HTMLURL))
	return "", errors.New(msg)
}

func (c Config) reportCreationErrorToAuthor(ctx workflow.Context, username, url string) {
	// True = don't send a DM to the user if they're opted-out.
	userID := users.GitHubToSlackID(ctx, username, true)
	if userID == "" {
		return
	}

	_, _ = slack.PostMessage(ctx, userID, "Failed to create Slack channel for "+url)
}

func (c Config) setChannelBookmarks(ctx workflow.Context, channelID, url string, pr PullRequest) {
	checks := 0

	_ = tslack.BookmarksAdd(ctx, channelID, fmt.Sprintf("Comments (%d)", pr.Comments), url, ":speech_balloon:")
	_ = tslack.BookmarksAdd(ctx, channelID, fmt.Sprintf("Commits (%d)", pr.Commits), url+"/commits", ":pushpin:")
	_ = tslack.BookmarksAdd(ctx, channelID, fmt.Sprintf("Checks (%d)", checks), url+"/checks", "")
	_ = tslack.BookmarksAdd(ctx, channelID, fmt.Sprintf("Files changed (%d)", pr.ChangedFiles), url+"/files", ":open_file_folder:")
	_ = tslack.BookmarksAdd(ctx, channelID, fmt.Sprintf("Diffs (+%d -%d)", pr.Additions, pr.Deletions), url+".diff", "")
}

func (c Config) postIntroMsg(ctx workflow.Context, channelID, action string, pr PullRequest, sender User) {
	if action == "ready_for_review" {
		action = "marked as ready for review"
	}

	msg := fmt.Sprintf("%%s %s %s: `%s`", action, pr.HTMLURL, pr.Title)
	if pr.Body != nil && strings.TrimSpace(*pr.Body) != "" {
		msg += "\n\n" + markdown.GitHubToSlack(ctx, *pr.Body, pr.HTMLURL)
	}

	_ = c.mentionUserInMsg(ctx, channelID, sender, msg)
}

// prParticipants returns a list of opted-in Slack user IDs (author/reviewers/assignees).
// The output is guaranteed to be sorted, without teams, and without repetitions.
func (c Config) prParticipants(ctx workflow.Context, pr PullRequest) []string {
	ghUsers := append(append([]User{pr.User}, pr.RequestedReviewers...), pr.Assignees...)
	usernames := []string{}
	for _, u := range ghUsers {
		if u.Type == "User" {
			usernames = append(usernames, u.Login)
		}
	}

	slackIDs := []string{}
	for _, u := range usernames {
		// True = don't include opted-out users. They will still be mentioned
		// in the channel, but as non-members they won't be notified about it.
		if id := users.GitHubToSlackID(ctx, u, true); id != "" {
			slackIDs = append(slackIDs, id)
		}
	}

	slices.Sort(slackIDs)
	return slices.Compact(slackIDs)
}
