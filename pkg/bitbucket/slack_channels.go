package bitbucket

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

func (b Bitbucket) archiveChannelWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	channelID, found := lookupChannel(ctx, event.Type, event.PullRequest)
	if !found {
		return nil // If we're not tracking the PR, there's no channel to archive.
	}

	// Wait for a few seconds to handle other asynchronous events
	// (e.g. a PR closure comment) before archiving the channel.
	_ = workflow.Sleep(ctx, 5*time.Second)

	l := workflow.GetLogger(ctx)
	url := event.PullRequest.Links["html"].HRef
	if err := data.RemoveURLToChannelMapping(url); err != nil {
		msg := "failed to remove PR URL / Slack channel mapping"
		l.Error(msg, "error", err, "event_type", event.Type, "channel_id", channelID, "pr_url", url)
		// Ignore this error, don't abort.
	}

	state := "closed this PR"
	switch event.Type {
	case "pullrequest.fulfilled":
		state = "merged this PR"
	case "pullrequest.updated":
		state = "converted this PR to a draft"
	}

	_, _ = b.mentionUserInMsg(ctx, channelID, event.Actor, "%s "+state)

	req := slack.ConversationsArchiveRequest{Channel: channelID}
	a := b.executeTimpaniActivity(ctx, slack.ConversationsArchiveActivity, req)

	if err := a.Get(ctx, nil); err != nil {
		msg := "failed to archive Slack channel"
		l.Error(msg, "error", err, "event_type", event.Type, "channel_id", channelID, "pr_url", url)

		state = strings.Replace(state, " this PR", "", 1)
		msg = "Failed to archive this channel, even though its PR was " + state
		req := slack.ChatPostMessageRequest{Channel: channelID, MarkdownText: msg}
		b.executeTimpaniActivity(ctx, slack.ChatPostMessageActivity, req)

		return err
	}

	return nil
}

func (b Bitbucket) initChannelWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	url := event.PullRequest.Links["html"].HRef

	channelID, err := b.createChannel(ctx, event.PullRequest)
	if err != nil {
		b.reportCreationErrorToAuthor(ctx, event.Actor.AccountID, url)
		return err
	}

	// Channel cosmetics.
	b.setChannelTopic(ctx, channelID, url)
	b.setChannelDescription(ctx, channelID, event.PullRequest.Title, url)
	b.postIntroMessage(ctx, channelID, event.Type, event.PullRequest, event.Actor)

	// Map the PR to the Slack channel ID, for 2-way event syncs.
	l := workflow.GetLogger(ctx)
	if err := data.MapURLToChannel(url, channelID); err != nil {
		msg := "failed to map PR to Slack channel"
		l.Error(msg, "error", err, "channel_id", channelID, "pr_url", url)
		return err
	}

	return nil
}

func (b Bitbucket) createChannel(ctx workflow.Context, pr PullRequest) (string, error) {
	title := slack.NormalizeChannelName(pr.Title, b.cmd.Int("slack-max-channel-name-length"))
	url := pr.Links["html"].HRef
	l := workflow.GetLogger(ctx)

	for i := 1; i < 100; i++ {
		name := fmt.Sprintf("%s_%s", b.cmd.String("slack-channel-name-prefix"), title)
		if i > 1 {
			name = fmt.Sprintf("%s_%d", name, i)
		}

		req := slack.ConversationsCreateRequest{Name: name}
		a := b.executeTimpaniActivity(ctx, slack.ConversationsCreateActivity, req)

		resp := &slack.ConversationsCreateResponse{}
		if err := a.Get(ctx, resp); err != nil {
			msg := "failed to create Slack channel"
			if !strings.Contains(err.Error(), "name_taken") {
				l.Error(msg, "error", err, "name", name, "pr_url", url)
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

		l.Info("created Slack channel", "channel_id", id, "channel_name", name, "pr_url", url)
		return id, nil
	}

	msg := "too many failed attempts to create Slack channel"
	l.Error(msg, "pr_url", url)
	return "", errors.New(msg)
}

func (b Bitbucket) reportCreationErrorToAuthor(ctx workflow.Context, accountID, url string) {
	l := workflow.GetLogger(ctx)

	email, err := data.BitbucketUserEmailByID(accountID)
	if err != nil {
		l.Error("failed to read Bitbucket user email", "error", err)
		return
	}

	// Don't send a DM to the user if they're opted-out.
	if email == "" {
		return
	}

	user, err := data.SlackUserIDByEmail(email)
	if err != nil || user == "" {
		l.Error("failed to read Slack user ID", "error", err, "email", email)
		return
	}

	msg := "Failed to create Slack channel for " + url
	req := slack.ChatPostMessageRequest{Channel: user, MarkdownText: msg}
	b.executeTimpaniActivity(ctx, slack.ChatPostMessageActivity, req)
}

func (b Bitbucket) setChannelTopic(ctx workflow.Context, channelID, url string) {
	t := url
	if len(t) > slack.MaxMetadataLen {
		t = t[:slack.MaxMetadataLen-4] + " ..."
	}

	req := slack.ConversationsSetTopicRequest{Channel: channelID, Topic: t}
	a := b.executeTimpaniActivity(ctx, slack.ConversationsSetTopicActivity, req)

	if err := a.Get(ctx, nil); err != nil {
		msg := "failed to set Slack channel topic"
		workflow.GetLogger(ctx).Error(msg, "error", err, "channel_id", channelID, "pr_url", url)
	}
}

func (b Bitbucket) setChannelDescription(ctx workflow.Context, channelID, title, url string) {
	t := fmt.Sprintf("`%s`", title)
	if len(t) > slack.MaxMetadataLen {
		t = t[:slack.MaxMetadataLen-4] + "`..."
	}

	req := slack.ConversationsSetPurposeRequest{Channel: channelID, Purpose: t}
	a := b.executeTimpaniActivity(ctx, slack.ConversationsSetPurposeActivity, req)

	if err := a.Get(ctx, nil); err != nil {
		msg := "failed to set Slack channel description"
		workflow.GetLogger(ctx).Error(msg, "error", err, "channel_id", channelID, "pr_url", url)
	}
}

func (b Bitbucket) postIntroMessage(ctx workflow.Context, channelID, eventType string, pr PullRequest, actor Account) {
	action := "created"
	if eventType == "pullrequest.updated" {
		action = "marked as ready for review"
	}

	msg := fmt.Sprintf("%%s %s %s: `%s`", action, pr.Links["html"].HRef, pr.Title)
	if strings.TrimSpace(pr.Description) != "" {
		msg += "\n\n" + markdown.BitbucketToSlack(pr.Description)
	}

	_, _ = b.mentionUserInMsg(ctx, channelID, actor, msg)
}
