package bitbucket

import (
	"errors"
	"fmt"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack"
	"github.com/tzrikka/revchat/pkg/users"
)

func (c Config) mentionUserInReplyByURL(ctx workflow.Context, parentURL string, user Account, msg string) error {
	ids, err := data.SwitchURLAndID(parentURL)
	if err != nil {
		log.Error(ctx, "failed to retrieve PR comment's Slack IDs", "error", err, "url", parentURL)
		return err
	}

	id := strings.Split(ids, "/")
	if len(id) < 2 {
		log.Debug(ctx, "can't post Slack reply message - missing/bad IDs", "bitbucket_url", parentURL, "slack_ids", ids)
		return errors.New("missing/bad Slack IDs")
	}

	msg = fmt.Sprintf(msg, users.BitbucketToSlackRef(ctx, c.Cmd, user.AccountID, user.DisplayName))
	req := slack.ChatPostMessageRequest{Channel: id[0], MarkdownText: msg, ThreadTS: id[1]}
	_, err = slack.PostChatMessageActivity(ctx, c.Cmd, req)
	return err
}

func (c Config) mentionUserInMsg(ctx workflow.Context, channelID string, user Account, msg string) error {
	return c.mentionUserInReply(ctx, channelID, "", user, msg)
}

func (c Config) mentionUserInReply(ctx workflow.Context, channelID, threadTS string, user Account, msg string) error {
	msg = fmt.Sprintf(msg, users.BitbucketToSlackRef(ctx, c.Cmd, user.AccountID, user.DisplayName))
	req := slack.ChatPostMessageRequest{Channel: channelID, MarkdownText: msg, ThreadTS: threadTS}
	_, err := slack.PostChatMessageActivity(ctx, c.Cmd, req)
	return err
}

func (c Config) impersonateUserInMsg(ctx workflow.Context, url, channelID string, user Account, msg string) error {
	name, icon := c.impersonateUser(ctx, user)
	req := slack.ChatPostMessageRequest{Channel: channelID, MarkdownText: msg, Username: name, IconURL: icon}
	resp, err := slack.PostChatMessageActivity(ctx, c.Cmd, req)
	if err != nil {
		return err
	}

	if err := data.MapURLAndID(url, fmt.Sprintf("%s/%s", channelID, resp.TS)); err != nil {
		log.Error(ctx, "failed to save PR comment URL / Slack IDs mapping", "error", err, "url", url)
	}

	return nil
}

func (c Config) impersonateUserInReply(ctx workflow.Context, url, parentURL string, user Account, msg string) error {
	ids, err := data.SwitchURLAndID(parentURL)
	if err != nil {
		log.Error(ctx, "failed to retrieve PR comment's Slack IDs", "error", err, "url", parentURL)
		return err
	}

	id := strings.Split(ids, "/")
	if len(id) < 2 {
		log.Error(ctx, "can't post Slack reply message - missing/bad IDs", "bitbucket_url", parentURL, "slack_ids", ids)
		return errors.New("missing/bad Slack IDs")
	}

	name, icon := c.impersonateUser(ctx, user)
	req := slack.ChatPostMessageRequest{Channel: id[0], MarkdownText: msg, ThreadTS: id[1], Username: name, IconURL: icon}
	resp, err := slack.PostChatMessageActivity(ctx, c.Cmd, req)
	if err != nil {
		return err
	}

	if err := data.MapURLAndID(url, fmt.Sprintf("%s/%s/%s", id[0], id[1], resp.TS)); err != nil {
		log.Error(ctx, "failed to save PR comment URL / Slack IDs mapping", "error", err, "url", url)
	}

	return nil
}

func (c Config) impersonateUser(ctx workflow.Context, user Account) (name, icon string) {
	id := users.BitbucketToSlackID(ctx, c.Cmd, user.AccountID, false)
	if id == "" {
		name = user.DisplayName
		return
	}

	profile, err := slack.UserProfileActivity(ctx, c.Cmd, id)
	if err != nil {
		name = user.DisplayName
		return
	}

	name = profile.DisplayName
	icon = profile.Image48
	return
}

func (c Config) deleteMsg(ctx workflow.Context, url string) error {
	ids, err := data.SwitchURLAndID(url)
	if err != nil {
		log.Error(ctx, "failed to retrieve PR comment's Slack IDs", "error", err, "url", url)
		return err
	}

	id := strings.Split(ids, "/")
	if len(id) < 2 {
		log.Error(ctx, "can't delete Slack message - missing/bad IDs", "bitbucket_url", url, "slack_ids", ids)
		return errors.New("missing/bad Slack IDs")
	}

	if err := data.DeleteURLAndIDMapping(url); err != nil {
		log.Error(ctx, "failed to delete URL/Slack mappings", "error", err, "comment_url", url)
	}

	return slack.DeleteChatMessageActivity(ctx, c.Cmd, id[0], id[len(id)-1])
}

func (c Config) editMsg(ctx workflow.Context, url, msg string) error {
	ids, err := data.SwitchURLAndID(url)
	if err != nil {
		log.Error(ctx, "failed to retrieve PR comment's Slack IDs", "error", err, "url", url)
		return err
	}

	id := strings.Split(ids, "/")
	if len(id) < 2 {
		log.Error(ctx, "can't update Slack message - missing/bad IDs", "bitbucket_url", url, "slack_ids", ids)
		return errors.New("missing/bad Slack IDs")
	}

	req := slack.ChatUpdateRequest{Channel: id[0], TS: id[len(id)-1], MarkdownText: msg}
	return slack.UpdateChatMessageActivity(ctx, c.Cmd, req)
}
