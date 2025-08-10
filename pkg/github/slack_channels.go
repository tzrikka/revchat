package github

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/markdown"
	"github.com/tzrikka/revchat/pkg/slack"
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

	req := slack.ConversationsArchiveRequest{Channel: channelID}
	a := g.executeTimpaniActivity(ctx, slack.ConversationsArchiveActivity, req)

	if err := a.Get(ctx, nil); err != nil {
		msg := "failed to archive Slack channel"
		l.Error(msg, "error", err, "action", event.Action, "channel_id", channelID, "pr_url", url)

		state = strings.Replace(state, " this PR", "", 1)
		msg = "Failed to archive this channel, even though its PR was " + state
		req := slack.ChatPostMessageRequest{Channel: channelID, MarkdownText: msg}
		g.executeTimpaniActivity(ctx, slack.ChatPostMessageActivity, req)

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
	channelID, err := g.createChannel(ctx, event.PullRequest)
	if err != nil {
		g.reportCreationErrorToAuthor(ctx, event.Sender.Login, event.PullRequest.HTMLURL)
		return err
	}

	// Channel cosmetics.
	g.setChannelTopic(ctx, channelID, event.PullRequest.HTMLURL)
	g.setChannelDescription(ctx, channelID, event.PullRequest.Title, event.PullRequest.HTMLURL)
	g.postIntroMessage(ctx, channelID, event.Action, event.PullRequest, event.Sender)

	// Map the PR to the Slack channel ID, for 2-way event syncs.
	l := workflow.GetLogger(ctx)
	if err := data.MapURLToChannel(event.PullRequest.HTMLURL, channelID); err != nil {
		msg := "failed to save PR URL / Slack channel mapping"
		l.Error(msg, "error", err, "channel_id", channelID, "pr_url", event.PullRequest.HTMLURL)
		return err
	}

	return nil
}

func (g GitHub) createChannel(ctx workflow.Context, pr PullRequest) (string, error) {
	title := slack.NormalizeChannelName(pr.Title, g.cmd.Int("slack-max-channel-name-length"))
	l := workflow.GetLogger(ctx)

	for i := 1; i < 100; i++ {
		name := fmt.Sprintf("%s_%s", g.cmd.String("slack-channel-name-prefix"), title)
		if i > 1 {
			name = fmt.Sprintf("%s_%d", name, i)
		}

		req := slack.ConversationsCreateRequest{Name: name}
		a := g.executeTimpaniActivity(ctx, slack.ConversationsCreateActivity, req)

		resp := &slack.ConversationsCreateResponse{}
		if err := a.Get(ctx, resp); err != nil {
			msg := "failed to create Slack channel"
			if !strings.Contains(err.Error(), "name_taken") {
				l.Error(msg, "error", err, "name", name, "pr_url", pr.HTMLURL)
				return "", err
			}

			l.Debug(msg+" - already exists", "name", name)
			continue // Retry with a different name.
		}

		channelID, ok := resp.Channel["id"]
		if !ok {
			msg := "created Slack channel without ID"
			l.Error(msg, "resp", resp)
			return "", errors.New(msg)
		}

		id, ok := channelID.(string)
		if !ok || len(id) == 0 {
			msg := "created Slack channel with invalid ID"
			l.Error(msg, "resp", resp)
			return "", errors.New(msg)
		}

		l.Info("created Slack channel", "channel_id", id, "channel_name", name, "pr_url", pr.HTMLURL)
		return id, nil
	}

	msg := "too many failed attempts to create Slack channel"
	l.Error(msg, "pr_url", pr.HTMLURL)
	return "", errors.New(msg)
}

func (g GitHub) reportCreationErrorToAuthor(ctx workflow.Context, username, url string) {
	l := workflow.GetLogger(ctx)

	email, err := data.GitHubUserEmailByID(username)
	if err != nil {
		l.Error("failed to load GitHub user email", "error", err, "username", username)
		return
	}

	// Don't send a DM to the user if they're opted-out.
	optedIn, err := data.IsOptedIn(email)
	if err != nil {
		l.Error("failed to load user opt-in status", "error", err, "email", email)
		return
	}
	if !optedIn {
		return
	}

	userID, err := data.SlackUserIDByEmail(email)
	if err != nil || userID == "" {
		l.Error("failed to load Slack user ID", "error", err, "email", email)
		return
	}

	msg := "Failed to create Slack channel for " + url
	req := slack.ChatPostMessageRequest{Channel: userID, MarkdownText: msg}
	g.executeTimpaniActivity(ctx, slack.ChatPostMessageActivity, req)
}

func (g GitHub) setChannelTopic(ctx workflow.Context, channelID, url string) {
	t := url
	if len(t) > slack.MaxMetadataLen {
		t = t[:slack.MaxMetadataLen-4] + " ..."
	}

	req := slack.ConversationsSetTopicRequest{Channel: channelID, Topic: t}
	a := g.executeTimpaniActivity(ctx, slack.ConversationsSetTopicActivity, req)

	if err := a.Get(ctx, nil); err != nil {
		msg := "failed to set Slack channel topic"
		workflow.GetLogger(ctx).Error(msg, "error", err, "channel_id", channelID, "pr_url", url)
	}
}

func (g GitHub) setChannelDescription(ctx workflow.Context, channelID, title, url string) {
	t := fmt.Sprintf("`%s`", title)
	if len(t) > slack.MaxMetadataLen {
		t = t[:slack.MaxMetadataLen-4] + "`..."
	}

	req := slack.ConversationsSetPurposeRequest{Channel: channelID, Purpose: t}
	a := g.executeTimpaniActivity(ctx, slack.ConversationsSetPurposeActivity, req)

	if err := a.Get(ctx, nil); err != nil {
		msg := "failed to set Slack channel description"
		workflow.GetLogger(ctx).Error(msg, "error", err, "channel_id", channelID, "pr_url", url)
	}
}

func (g GitHub) postIntroMessage(ctx workflow.Context, channelID, action string, pr PullRequest, sender User) {
	if action == "ready_for_review" {
		action = "marked as ready for review"
	}

	msg := fmt.Sprintf("%%s %s %s: `%s`", action, pr.HTMLURL, pr.Title)
	if text := strings.TrimSpace(*pr.Body); text != "" {
		msg += "\n\n" + markdown.GitHubToSlack(ctx, g.cmd, text, pr.HTMLURL)
	}

	_, _ = g.mentionUserInMsg(ctx, channelID, sender, msg)
}
