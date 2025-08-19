package bitbucket

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

func (b Bitbucket) archiveChannelWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	pr := event.PullRequest

	// If we're not tracking this PR, there's no channel to archive.
	channelID, found := lookupChannel(ctx, event.Type, pr)
	if !found {
		return nil
	}

	// Wait for a few seconds to handle other asynchronous events
	// (e.g. a PR closure comment) before archiving the channel.
	_ = workflow.Sleep(ctx, 5*time.Second)

	url := pr.Links["html"].HRef
	b.cleanupPRData(ctx, url)

	state := "closed this PR"
	switch event.Type {
	case "pullrequest.fulfilled":
		state = "merged this PR"
	case "pullrequest.updated":
		state = "converted this PR to a draft"
	}

	if reason := event.PullRequest.Reason; reason != "" {
		state = fmt.Sprintf("%s with this reason: `%s`", state, reason)
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

	channelID, err := data.SwitchURLAndID(url)
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

// cleanupPRData deletes all data associated with a PR. If there are errors,
// they are logged but ignored, as they do not affect the overall workflow.
func (b Bitbucket) cleanupPRData(ctx workflow.Context, url string) {
	if err := data.DeleteBitbucketPR(url); err != nil {
		workflow.GetLogger(ctx).Error("failed to delete PR snapshot", "error", err, "pr_url", url)
	}

	if err := data.DeleteURLAndIDMapping(url); err != nil {
		workflow.GetLogger(ctx).Error("failed to delete PR URL / Slack channel mappings", "error", err, "pr_url", url)
	}
}

// initPRData saves the initial state of a new PR: a snapshot of the
// PR details, and mappings for 2-way syncs between Bitbucket and Slack.
func (b Bitbucket) initPRData(ctx workflow.Context, url string, pr PullRequest, channelID string) bool {
	l := workflow.GetLogger(ctx)

	if err := data.StoreBitbucketPR(url, pr); err != nil {
		l.Error("failed to save PR snapshot", "error", err, "channel_id", channelID, "pr_url", url)
		return false
	}

	if err := data.MapURLAndID(url, channelID); err != nil {
		l.Error("failed to save PR URL / Slack channel mapping", "error", err, "channel_id", channelID, "pr_url", url)
		return false
	}

	return true
}

func (b Bitbucket) initChannelWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	pr := event.PullRequest
	url := pr.Links["html"].HRef

	channelID, err := b.createChannel(ctx, pr)
	if err != nil {
		b.reportCreationErrorToAuthor(ctx, event.Actor.AccountID, url)
		return err
	}

	if !b.initPRData(ctx, url, pr, channelID) {
		b.reportCreationErrorToAuthor(ctx, event.Actor.AccountID, url)
		b.cleanupPRData(ctx, url)
		return errors.New("failed to initialize PR data")
	}

	// Channel cosmetics.
	slack.SetChannelTopicActivity(ctx, b.cmd, channelID, url)
	slack.SetChannelDescriptionActivity(ctx, b.cmd, channelID, pr.Title, url)
	b.setChannelBookmarks(ctx, channelID, url, pr)
	b.postIntroMessage(ctx, channelID, event.Type, pr, event.Actor)

	err = slack.InviteUsersToChannelActivity(ctx, b.cmd, channelID, b.prParticipants(ctx, pr))
	if err != nil {
		b.reportCreationErrorToAuthor(ctx, event.Actor.AccountID, url)
		b.cleanupPRData(ctx, url)
	}

	return err
}

func (b Bitbucket) createChannel(ctx workflow.Context, pr PullRequest) (string, error) {
	title := slack.NormalizeChannelName(pr.Title, b.cmd.Int("slack-max-channel-name-length"))
	url := pr.Links["html"].HRef
	l := workflow.GetLogger(ctx)

	for i := 1; i < 100; i++ {
		name := fmt.Sprintf("%s_%d_%s", b.cmd.String("slack-channel-name-prefix"), pr.ID, title)
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
	// True = don't send a DM to the user if they're opted-out.
	userID := users.BitbucketToSlackID(ctx, b.cmd, accountID, true)
	if userID == "" {
		return
	}

	msg := "Failed to create Slack channel for " + url
	req := slack.ChatPostMessageRequest{Channel: userID, MarkdownText: msg}
	slack.PostChatMessageActivityAsync(ctx, b.cmd, req)
}

func (b Bitbucket) setChannelBookmarks(ctx workflow.Context, channelID, url string, pr PullRequest) {
	files := 0
	commits := 0

	slack.AddBookmarkActivity(ctx, b.cmd, channelID, fmt.Sprintf("Comments (%d)", pr.CommentCount), url+"/overview")
	slack.AddBookmarkActivity(ctx, b.cmd, channelID, fmt.Sprintf("Commits (%d)", commits), url+"/commits")
	slack.AddBookmarkActivity(ctx, b.cmd, channelID, fmt.Sprintf("Files changed (%d)", files), url+"/diff")
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

// prParticipants returns a list of opted-in Slack user IDs (author/participants/reviewers).
// The output is guaranteed to be sorted, without teams/apps, and without repetitions.
func (b Bitbucket) prParticipants(ctx workflow.Context, pr PullRequest) []string {
	accounts := append([]Account{pr.Author}, pr.Reviewers...)
	for _, p := range pr.Participants {
		accounts = append(accounts, p.User)
	}

	accountIDs := []string{}
	for _, a := range accounts {
		if a.Type == "user" {
			accountIDs = append(accountIDs, a.AccountID)
		}
	}

	slackIDs := []string{}
	for _, aid := range accountIDs {
		// True = don't include opted-out users. They will still be mentioned
		// in the channel, but as non-members they won't be notified about it.
		if sid := users.BitbucketToSlackID(ctx, b.cmd, aid, true); sid != "" {
			slackIDs = append(slackIDs, sid)
		}
	}

	slices.Sort(slackIDs)
	return slices.Compact(slackIDs)
}
