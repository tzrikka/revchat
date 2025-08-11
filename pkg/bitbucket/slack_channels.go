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
	// If we're not tracking this PR, there's no channel to archive.
	channelID, found := lookupChannel(ctx, event.Type, event.PullRequest)
	if !found {
		return nil
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

	if err := slack.ArchiveChannelActivity(ctx, b.cmd, channelID); err != nil {
		state = strings.Replace(state, " this PR", "", 1)
		msg := "Failed to archive this channel, even though its PR was " + state
		req := slack.ChatPostMessageRequest{Channel: channelID, MarkdownText: msg}
		slack.PostChatMessageActivityAsync(ctx, b.cmd, req)

		return err
	}

	return nil
}

// lookupChannel returns the ID of a channel associated
// with a PR, if the PR is active and the channel is found.
func lookupChannel(ctx workflow.Context, eventType string, pr PullRequest) (string, bool) {
	l := workflow.GetLogger(ctx)
	url := pr.Links["html"].HRef

	if pr.Draft {
		l.Debug("ignoring Bitbucket event - the PR is a draft", "event_type", eventType, "pr_url", url)
		return "", false
	}

	channelID, err := data.ConvertURLToChannel(url)
	if err != nil {
		l.Error("failed to retrieve Bitbucket PR's Slack channel ID", "error", err, "event_type", eventType, "pr_url", url)
		return "", false
	}

	if channelID == "" {
		l.Debug("Bitbucket PR's Slack channel ID is empty", "event_type", eventType, "pr_url", url)
		return "", false
	}

	return channelID, true
}

func (b Bitbucket) initChannelWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	pr := event.PullRequest
	url := pr.Links["html"].HRef

	channelID, err := b.createChannel(ctx, pr)
	if err != nil {
		b.reportCreationErrorToAuthor(ctx, event.Actor.AccountID, url)
		return err
	}

	// Map the PR to the Slack channel ID, for 2-way event syncs.
	l := workflow.GetLogger(ctx)
	if err := data.MapURLToChannel(url, channelID); err != nil {
		msg := "failed to save PR URL / Slack channel mapping"
		l.Error(msg, "error", err, "channel_id", channelID, "pr_url", url)
		return err
	}

	// Channel cosmetics.
	slack.SetChannelTopicActivity(ctx, b.cmd, channelID, url)
	slack.SetChannelDescriptionActivity(ctx, b.cmd, channelID, pr.Title, url)
	b.setChannelBookmarks(ctx, channelID, url, pr)

	b.postIntroMessage(ctx, channelID, event.Type, pr, event.Actor)

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

		id, retry, err := slack.CreateChannelActivity(ctx, b.cmd, name, url)
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
	l.Error(msg, "pr_url", url)
	return "", errors.New(msg)
}

func (b Bitbucket) reportCreationErrorToAuthor(ctx workflow.Context, accountID, url string) {
	l := workflow.GetLogger(ctx)

	email, err := data.BitbucketUserEmailByID(accountID)
	if err != nil {
		l.Error("failed to load Bitbucket user email", "error", err, "account_id", accountID)
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
	slack.PostChatMessageActivityAsync(ctx, b.cmd, req)
}

func (b Bitbucket) setChannelBookmarks(ctx workflow.Context, channelID, url string, pr PullRequest) {
	files := 0
	commits := 0

	slack.AddBookmarkActivity(ctx, b.cmd, channelID, fmt.Sprintf("Conversation (%d)", pr.CommentCount), url+"/overview")
	slack.AddBookmarkActivity(ctx, b.cmd, channelID, fmt.Sprintf("Files changed (%d)", files), url+"/diff")
	slack.AddBookmarkActivity(ctx, b.cmd, channelID, fmt.Sprintf("Commits (%d)", commits), url+"/commits")
}

func (b Bitbucket) postIntroMessage(ctx workflow.Context, channelID, eventType string, pr PullRequest, actor Account) {
	action := "created"
	if eventType == "pullrequest.updated" {
		action = "marked as ready for review"
	}

	msg := fmt.Sprintf("%%s %s %s: `%s`", action, pr.Links["html"].HRef, pr.Title)
	if text := strings.TrimSpace(pr.Description); text != "" {
		msg += "\n\n" + markdown.BitbucketToSlack(ctx, b.cmd, text)
	}

	_, _ = b.mentionUserInMsg(ctx, channelID, actor, msg)
}
