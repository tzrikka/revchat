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

func (g GitHub) archiveChannelWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	// If we're not tracking this PR, there's no channel to archive.
	channelID, found := lookupChannel(ctx, event.Action, event.PullRequest)
	if !found {
		return nil
	}

	// Wait for a few seconds to handle other asynchronous events
	// (e.g. a PR closure comment) before archiving the channel.
	_ = workflow.Sleep(ctx, 5*time.Second)

	l := workflow.GetLogger(ctx)
	url := event.PullRequest.HTMLURL
	if err := data.RemoveURLToChannelMapping(url); err != nil {
		msg := "failed to remove PR URL / Slack channel mapping"
		l.Error(msg, "error", err, "action", event.Action, "channel_id", channelID, "pr_url", url)
		// Ignore this error, don't abort.
	}

	state := event.Action + " this PR"
	if event.Action == "closed" && event.PullRequest.Merged {
		state = "merged this PR"
	}
	if event.Action == "converted_to_draft" {
		state = "converted this PR to a draft"
	}

	_, _ = g.mentionUserInMsg(ctx, channelID, event.Sender, "%s "+state)

	if err := slack.ArchiveChannelActivity(ctx, g.cmd, channelID); err != nil {
		state = strings.Replace(state, " this PR", "", 1)
		msg := "Failed to archive this channel, even though its PR was " + state
		req := slack.ChatPostMessageRequest{Channel: channelID, MarkdownText: msg}
		slack.PostChatMessageActivityAsync(ctx, g.cmd, req)

		return err
	}

	return nil
}

// lookupChannel returns the ID of a channel associated
// with a PR, if the PR is active and the channel is found.
func lookupChannel(ctx workflow.Context, action string, pr PullRequest) (string, bool) {
	l := workflow.GetLogger(ctx)

	if pr.Draft {
		l.Debug("ignoring GitHub event - the PR is a draft", "action", action, "pr_url", pr.HTMLURL)
		return "", false
	}
	// case pr.State != "open":
	// 	l.Debug("ignoring GitHub event - the PR isn't open", "action", action, "url", pr.HTMLURL)
	// 	return "", false

	channelID, err := data.ConvertURLToChannel(pr.HTMLURL)
	if err != nil {
		l.Error("failed to retrieve GitHub PR's Slack channel ID", "error", err, "action", action, "pr_url", pr.HTMLURL)
		return "", false
	}

	if channelID == "" {
		l.Debug("GitHub PR's Slack channel ID is empty", "action", action, "pr_url", pr.HTMLURL)
		return "", false
	}

	return channelID, true
}

func (g GitHub) initChannelWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	pr := event.PullRequest
	channelID, err := g.createChannel(ctx, pr)
	if err != nil {
		g.reportCreationErrorToAuthor(ctx, event.Sender.Login, pr.HTMLURL)
		return err
	}

	// Map the PR to the Slack channel ID, for 2-way event syncs.
	l := workflow.GetLogger(ctx)
	if err := data.MapURLToChannel(pr.HTMLURL, channelID); err != nil {
		msg := "failed to save PR URL / Slack channel mapping"
		l.Error(msg, "error", err, "channel_id", channelID, "pr_url", pr.HTMLURL)
		return err
	}

	// Channel cosmetics.
	slack.SetChannelTopicActivity(ctx, g.cmd, channelID, pr.HTMLURL)
	slack.SetChannelDescriptionActivity(ctx, g.cmd, channelID, pr.Title, pr.HTMLURL)
	g.setChannelBookmarks(ctx, channelID, pr.HTMLURL, pr)

	g.postIntroMessage(ctx, channelID, event.Action, pr, event.Sender)
	return slack.InviteUsersToChannelActivity(ctx, g.cmd, channelID, g.prParticipants(ctx, pr))
}

func (g GitHub) createChannel(ctx workflow.Context, pr PullRequest) (string, error) {
	title := slack.NormalizeChannelName(pr.Title, g.cmd.Int("slack-max-channel-name-length"))
	l := workflow.GetLogger(ctx)

	for i := 1; i < 100; i++ {
		name := fmt.Sprintf("%s_%s", g.cmd.String("slack-channel-name-prefix"), title)
		if i > 1 {
			name = fmt.Sprintf("%s_%d", name, i)
		}

		id, retry, err := slack.CreateChannelActivity(ctx, g.cmd, name, pr.HTMLURL)
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

func (g GitHub) reportCreationErrorToAuthor(ctx workflow.Context, username, url string) {
	// True = don't send a DM to the user if they're opted-out.
	userID := users.GitHubToSlackID(ctx, g.cmd, username, true)
	if userID == "" {
		return
	}

	msg := "Failed to create Slack channel for " + url
	req := slack.ChatPostMessageRequest{Channel: userID, MarkdownText: msg}
	slack.PostChatMessageActivityAsync(ctx, g.cmd, req)
}

func (g GitHub) setChannelBookmarks(ctx workflow.Context, channelID, url string, pr PullRequest) {
	checks := 0

	slack.AddBookmarkActivity(ctx, g.cmd, channelID, fmt.Sprintf("Conversation (%d)", pr.Comments), url)
	slack.AddBookmarkActivity(ctx, g.cmd, channelID, fmt.Sprintf("Commits (%d)", pr.Commits), url+"/commits")
	slack.AddBookmarkActivity(ctx, g.cmd, channelID, fmt.Sprintf("Checks (%d)", checks), url+"/checks")
	slack.AddBookmarkActivity(ctx, g.cmd, channelID, fmt.Sprintf("Files changed (%d)", pr.ChangedFiles), url+"/files")
	slack.AddBookmarkActivity(ctx, g.cmd, channelID, fmt.Sprintf("Diffs (+%d -%d)", pr.Additions, pr.Deletions), url+".diff")
}

func (g GitHub) postIntroMessage(ctx workflow.Context, channelID, action string, pr PullRequest, sender User) {
	if action == "ready_for_review" {
		action = "marked as ready for review"
	}

	msg := fmt.Sprintf("%%s %s %s: `%s`", action, pr.HTMLURL, pr.Title)
	if pr.Body != nil && strings.TrimSpace(*pr.Body) != "" {
		msg += "\n\n" + markdown.GitHubToSlack(ctx, g.cmd, *pr.Body, pr.HTMLURL)
	}

	_, _ = g.mentionUserInMsg(ctx, channelID, sender, msg)
}

// prParticipants returns a list of opted-in Slack user IDs (author/reviewers/assignees).
// The output is guaranteed to be sorted, without teams, and without repetitions.
func (g GitHub) prParticipants(ctx workflow.Context, pr PullRequest) []string {
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
		if id := users.GitHubToSlackID(ctx, g.cmd, u, true); id != "" {
			slackIDs = append(slackIDs, id)
		}
	}

	slices.Sort(slackIDs)
	return slices.Compact(slackIDs)
}
