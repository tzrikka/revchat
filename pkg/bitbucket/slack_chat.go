package bitbucket

import (
	"fmt"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack"
	"github.com/tzrikka/revchat/pkg/users"
)

func (b Bitbucket) mentionUserInMsgAsync(ctx workflow.Context, channelID string, user Account, msg string) {
	msg = fmt.Sprintf(msg, users.BitbucketToSlackRef(ctx, b.cmd, user.AccountID, user.DisplayName))
	req := slack.ChatPostMessageRequest{Channel: channelID, MarkdownText: msg}
	slack.PostChatMessageActivityAsync(ctx, b.cmd, req)
}

func (b Bitbucket) mentionUserInReplyAsync(ctx workflow.Context, parentURL string, user Account, msg string) {
	l := workflow.GetLogger(ctx)
	ids, err := data.SwitchURLAndID(parentURL)
	if err != nil {
		l.Error("failed to retrieve PR comment's Slack IDs", "url", parentURL, "error", err)
		return
	}

	id := strings.Split(ids, "/")
	if len(id) < 2 {
		l.Debug("can't post Slack reply message - missing/bad IDs", "bitbucket_url", parentURL, "slack_ids", ids)
		return
	}

	msg = fmt.Sprintf(msg, users.BitbucketToSlackRef(ctx, b.cmd, user.AccountID, user.DisplayName))
	req := slack.ChatPostMessageRequest{Channel: id[0], MarkdownText: msg, ThreadTS: id[1]}
	slack.PostChatMessageActivityAsync(ctx, b.cmd, req)
}

func (b Bitbucket) mentionUserInMsg(ctx workflow.Context, channelID string, user Account, msg string) (string, error) {
	return b.mentionUserInReply(ctx, channelID, "", user, msg)
}

func (b Bitbucket) mentionUserInReply(ctx workflow.Context, channelID, threadTS string, user Account, msg string) (string, error) {
	msg = fmt.Sprintf(msg, users.BitbucketToSlackRef(ctx, b.cmd, user.AccountID, user.DisplayName))
	req := slack.ChatPostMessageRequest{Channel: channelID, MarkdownText: msg, ThreadTS: threadTS}
	resp, err := slack.PostChatMessageActivity(ctx, b.cmd, req)
	if err != nil {
		return "", err
	}

	return resp.TS, nil
}

func (b Bitbucket) impersonateUserInMsg(ctx workflow.Context, url, channelID string, user Account, msg string) {
	name, icon := b.impersonateUser(ctx, user)
	req := slack.ChatPostMessageRequest{Channel: channelID, MarkdownText: msg, Username: name, IconURL: icon}
	resp, err := slack.PostChatMessageActivity(ctx, b.cmd, req)
	if err != nil {
		return
	}

	if err := data.MapURLAndID(url, fmt.Sprintf("%s/%s", channelID, resp.TS)); err != nil {
		workflow.GetLogger(ctx).Error("failed to map PR comment URL to Slack IDs", "url", url, "error", err)
	}
}

func (b Bitbucket) impersonateUserInReply(ctx workflow.Context, url, parentURL string, user Account, msg string) {
	l := workflow.GetLogger(ctx)
	ids, err := data.SwitchURLAndID(parentURL)
	if err != nil {
		l.Error("failed to retrieve PR comment's Slack IDs", "url", parentURL, "error", err)
		return
	}

	id := strings.Split(ids, "/")
	if len(id) < 2 {
		l.Error("can't post Slack reply message - missing/bad IDs", "bitbucket_url", parentURL, "slack_ids", ids)
		return
	}

	name, icon := b.impersonateUser(ctx, user)
	req := slack.ChatPostMessageRequest{Channel: id[0], MarkdownText: msg, ThreadTS: id[1], Username: name, IconURL: icon}
	resp, err := slack.PostChatMessageActivity(ctx, b.cmd, req)
	if err != nil {
		return
	}

	if err := data.MapURLAndID(url, fmt.Sprintf("%s/%s/%s", id[0], id[1], resp.TS)); err != nil {
		l.Error("failed to map PR comment URL to Slack IDs", "url", url, "error", err)
	}
}

func (b Bitbucket) impersonateUser(ctx workflow.Context, user Account) (name, icon string) {
	id := users.BitbucketToSlackID(ctx, b.cmd, user.AccountID, false)
	if id == "" {
		name = user.DisplayName
		return
	}

	profile, err := slack.UserProfileActivity(ctx, b.cmd, id)
	if err != nil {
		name = user.DisplayName
		return
	}

	name = profile.DisplayName
	icon = profile.Image48
	return
}

func (b Bitbucket) deleteMsg(ctx workflow.Context, url string) {
	l := workflow.GetLogger(ctx)
	ids, err := data.SwitchURLAndID(url)
	if err != nil {
		l.Error("failed to retrieve PR comment's Slack IDs", "url", url, "error", err)
		return
	}

	id := strings.Split(ids, "/")
	if len(id) < 2 {
		l.Error("can't delete Slack message - missing/bad IDs", "bitbucket_url", url, "slack_ids", ids)
		return
	}

	req := slack.ChatDeleteRequest{Channel: id[0], TS: id[len(id)-1]}
	slack.DeleteChatMessageActivityAsync(ctx, b.cmd, req)

	if err := data.DeleteURLAndIDMapping(url); err != nil {
		l.Error("failed to delete URL/Slack mappings", "error", err, "comment_url", url)
	}
}

func (b Bitbucket) editMsg(ctx workflow.Context, url, msg string) {
	l := workflow.GetLogger(ctx)
	ids, err := data.SwitchURLAndID(url)
	if err != nil {
		l.Error("failed to retrieve PR comment's Slack IDs", "url", url, "error", err)
		return
	}

	id := strings.Split(ids, "/")
	if len(id) < 2 {
		l.Error("can't update Slack message - missing/bad IDs", "bitbucket_url", url, "slack_ids", ids)
		return
	}

	req := slack.ChatUpdateRequest{Channel: id[0], TS: id[len(id)-1], MarkdownText: msg}
	slack.UpdateChatMessageActivityAsync(ctx, b.cmd, req)
}
