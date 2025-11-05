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
	tslack "github.com/tzrikka/timpani-api/pkg/slack"
)

func mentionUserInReplyByURL(ctx workflow.Context, parentURL string, user Account, msg string) error {
	ids, err := data.SwitchURLAndID(parentURL)
	if err != nil {
		log.Error(ctx, "failed to load PR comment's Slack IDs", "error", err, "url", parentURL)
		return err
	}

	id := strings.Split(ids, "/")
	if len(id) < 2 {
		log.Warn(ctx, "can't post Slack reply message - missing/bad IDs", "bitbucket_url", parentURL, "slack_ids", ids)
		return errors.New("missing/bad Slack IDs")
	}

	msg = fmt.Sprintf(msg, users.BitbucketToSlackRef(ctx, user.AccountID, user.DisplayName))
	_, err = slack.PostReply(ctx, id[0], id[1], msg)
	return err
}

func mentionUserInMsg(ctx workflow.Context, channelID string, user Account, msg string) error {
	return mentionUserInReply(ctx, channelID, "", user, msg)
}

func mentionUserInReply(ctx workflow.Context, channelID, threadTS string, user Account, msg string) error {
	msg = fmt.Sprintf(msg, users.BitbucketToSlackRef(ctx, user.AccountID, user.DisplayName))
	_, err := slack.PostReply(ctx, channelID, threadTS, msg)
	return err
}

func impersonateUserInMsg(ctx workflow.Context, url, channelID string, user Account, msg string) error {
	name, icon := impersonateUser(ctx, user)
	resp, err := slack.PostMessageAsUser(ctx, channelID, name, icon, msg)
	if err != nil {
		return err
	}

	id := fmt.Sprintf("%s/%s", channelID, resp.TS)
	if err := data.MapURLAndID(url, id); err != nil {
		log.Error(ctx, "failed to save PR comment URL / Slack IDs mapping", "error", err, "url", url, "slack_id", id)
		// Don't return the error - the message is already posted in Slack, so we
		// don't want to retry and post it again, even though this is problematic.
	}

	return nil
}

func impersonateUserInReply(ctx workflow.Context, url, parentURL string, user Account, msg string) error {
	ids, err := data.SwitchURLAndID(parentURL)
	if err != nil {
		log.Error(ctx, "failed to load PR comment's Slack IDs", "error", err, "url", parentURL)
		return err
	}

	id := strings.Split(ids, "/")
	if len(id) < 2 {
		log.Warn(ctx, "can't post Slack reply message - missing/bad IDs", "bitbucket_url", parentURL, "slack_ids", ids)
		return errors.New("missing/bad Slack IDs")
	}

	name, icon := impersonateUser(ctx, user)
	resp, err := slack.PostReplyAsUser(ctx, id[0], id[1], name, icon, msg)
	if err != nil {
		return err
	}

	sid := fmt.Sprintf("%s/%s/%s", id[0], id[1], resp.TS)
	if err := data.MapURLAndID(url, sid); err != nil {
		log.Error(ctx, "failed to save PR comment URL / Slack IDs mapping", "error", err, "url", url, "slack_id", sid)
		// Don't return the error - the message is already posted in Slack, so we
		// don't want to retry and post it again, even though this is problematic.
	}

	return nil
}

func impersonateUser(ctx workflow.Context, user Account) (name, icon string) {
	id := users.BitbucketToSlackID(ctx, user.AccountID, false)
	if id == "" {
		name = user.DisplayName
		return
	}

	profile, err := tslack.UsersProfileGetActivity(ctx, id)
	if err != nil {
		name = user.DisplayName
		return
	}

	name = profile.DisplayName
	icon = profile.Image48
	return
}

func deleteMsg(ctx workflow.Context, url string) error {
	ids, err := data.SwitchURLAndID(url)
	if err != nil {
		log.Error(ctx, "failed to retrieve PR comment's Slack IDs", "error", err, "url", url)
		return err
	}
	if ids == "" {
		log.Debug(ctx, "no Slack IDs found for Bitbucket URL (already deleted)", "url", url)
		return nil
	}

	id := strings.Split(ids, "/")
	if len(id) < 2 {
		log.Warn(ctx, "can't delete Slack message - missing/bad IDs", "bitbucket_url", url, "slack_ids", ids)
		return errors.New("missing/bad Slack IDs")
	}

	if err := data.DeleteURLAndIDMapping(url); err != nil {
		log.Error(ctx, "failed to delete URL/Slack mappings", "error", err, "comment_url", url)
		// Don't abort - we still want to attempt to delete the Slack message.
	}

	return slack.DeleteMessage(ctx, id[0], id[len(id)-1])
}

func editMsg(ctx workflow.Context, url, msg string) error {
	ids, err := data.SwitchURLAndID(url)
	if err != nil {
		log.Error(ctx, "failed to retrieve PR comment's Slack IDs", "error", err, "url", url)
		return err
	}

	id := strings.Split(ids, "/")
	if len(id) < 2 {
		log.Warn(ctx, "can't update Slack message - missing/bad IDs", "bitbucket_url", url, "slack_ids", ids)
		return errors.New("missing/bad Slack IDs")
	}

	return slack.UpdateMessage(ctx, id[0], id[len(id)-1], msg)
}
