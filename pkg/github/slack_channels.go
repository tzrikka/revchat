package github

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/markdown"
	"github.com/tzrikka/revchat/pkg/slack"
	"github.com/tzrikka/revchat/pkg/users"
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

	if err := slack.ArchiveChannelActivity(ctx, c.Cmd, channelID); err != nil {
		state = strings.Replace(state, " this PR", "", 1)
		msg := "Failed to archive this channel, even though its PR was " + state
		req := slack.ChatPostMessageRequest{Channel: channelID, MarkdownText: msg}
		_, _ = slack.PostChatMessageActivity(ctx, c.Cmd, req)

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
		msg := "failed to retrieve GitHub PR's Slack channel ID"
		workflow.GetLogger(ctx).Error(msg, "error", err, "action", action, "pr_url", pr.HTMLURL)
		return "", false
	}

	if channelID == "" {
		msg := "GitHub PR's Slack channel ID not found"
		workflow.GetLogger(ctx).Debug(msg, "action", action, "pr_url", pr.HTMLURL)
		return "", false
	}

	return channelID, true
}

// cleanupPRData deletes all data associated with a PR. If there are errors,
// they are logged but ignored, as they do not affect the overall workflow.
func (c Config) cleanupPRData(ctx workflow.Context, url string) {
	if err := data.DeleteURLAndIDMapping(url); err != nil {
		msg := "failed to delete PR URL / Slack channel mappings"
		workflow.GetLogger(ctx).Error(msg, "error", err, "pr_url", url)
	}
}

func (c Config) initChannel(ctx workflow.Context, event PullRequestEvent) error {
	pr := event.PullRequest
	channelID, err := c.createChannel(ctx, pr)
	if err != nil {
		c.reportCreationErrorToAuthor(ctx, event.Sender.Login, pr.HTMLURL)
		return err
	}

	// Map the PR to the Slack channel ID, for 2-way event syncs.
	if err := data.MapURLAndID(pr.HTMLURL, channelID); err != nil {
		msg := "failed to save PR URL / Slack channel mapping"
		workflow.GetLogger(ctx).Error(msg, "error", err, "channel_id", channelID, "pr_url", pr.HTMLURL)
		c.reportCreationErrorToAuthor(ctx, event.Sender.Login, pr.HTMLURL)
		c.cleanupPRData(ctx, pr.HTMLURL)

		return err
	}

	// Channel cosmetics.
	slack.SetChannelTopicActivity(ctx, c.Cmd, channelID, pr.HTMLURL)
	slack.SetChannelDescriptionActivity(ctx, c.Cmd, channelID, pr.Title, pr.HTMLURL)
	c.setChannelBookmarks(ctx, channelID, pr.HTMLURL, pr)
	c.postIntroMessage(ctx, channelID, event.Action, pr, event.Sender)

	err = slack.InviteUsersToChannelActivity(ctx, c.Cmd, channelID, c.prParticipants(ctx, pr))
	if err != nil {
		c.reportCreationErrorToAuthor(ctx, event.Sender.Login, pr.HTMLURL)
		c.cleanupPRData(ctx, pr.HTMLURL)
	}

	return err
}

func (c Config) createChannel(ctx workflow.Context, pr PullRequest) (string, error) {
	title := slack.NormalizeChannelName(pr.Title, c.Cmd.Int("slack-max-channel-name-length"))
	l := workflow.GetLogger(ctx)

	for i := 1; i < 50; i++ {
		name := fmt.Sprintf("%s_%d_%s", c.Cmd.String("slack-channel-name-prefix"), pr.Number, title)
		if i > 1 {
			name = fmt.Sprintf("%s_%d", name, i)
		}

		id, retry, err := slack.CreateChannelActivity(ctx, c.Cmd, name, pr.HTMLURL)
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
	l.Error(msg, "pr_url", pr.HTMLURL)
	return "", errors.New(msg)
}

func (c Config) reportCreationErrorToAuthor(ctx workflow.Context, username, url string) {
	// True = don't send a DM to the user if they're opted-out.
	userID := users.GitHubToSlackID(ctx, c.Cmd, username, true)
	if userID == "" {
		return
	}

	msg := "Failed to create Slack channel for " + url
	req := slack.ChatPostMessageRequest{Channel: userID, MarkdownText: msg}
	_, _ = slack.PostChatMessageActivity(ctx, c.Cmd, req)
}

func (c Config) setChannelBookmarks(ctx workflow.Context, channelID, url string, pr PullRequest) {
	checks := 0

	slack.AddBookmarkActivity(ctx, c.Cmd, channelID, fmt.Sprintf("Comments (%d)", pr.Comments), url)
	slack.AddBookmarkActivity(ctx, c.Cmd, channelID, fmt.Sprintf("Commits (%d)", pr.Commits), url+"/commits")
	slack.AddBookmarkActivity(ctx, c.Cmd, channelID, fmt.Sprintf("Checks (%d)", checks), url+"/checks")
	slack.AddBookmarkActivity(ctx, c.Cmd, channelID, fmt.Sprintf("Files changed (%d)", pr.ChangedFiles), url+"/files")
	slack.AddBookmarkActivity(ctx, c.Cmd, channelID, fmt.Sprintf("Diffs (+%d -%d)", pr.Additions, pr.Deletions), url+".diff")
}

func (c Config) postIntroMessage(ctx workflow.Context, channelID, action string, pr PullRequest, sender User) {
	if action == "ready_for_review" {
		action = "marked as ready for review"
	}

	msg := fmt.Sprintf("%%s %s %s: `%s`", action, pr.HTMLURL, pr.Title)
	if pr.Body != nil && strings.TrimSpace(*pr.Body) != "" {
		msg += "\n\n" + markdown.GitHubToSlack(ctx, c.Cmd, *pr.Body, pr.HTMLURL)
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
		if id := users.GitHubToSlackID(ctx, c.Cmd, u, true); id != "" {
			slackIDs = append(slackIDs, id)
		}
	}

	slices.Sort(slackIDs)
	return slices.Compact(slackIDs)
}
