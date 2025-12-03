package bitbucket

import (
	"fmt"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack"
	"github.com/tzrikka/revchat/pkg/users"
	tslack "github.com/tzrikka/timpani-api/pkg/slack"
)

// No need for thread safety, this should only be set once per process, and even
// if multiple workflows set it concurrently, the value will be the same anyway.
var workspaceURL = ""

func mentionUserInMsg(ctx workflow.Context, channelID string, user Account, msg string) error {
	return mentionUserInReply(ctx, channelID, "", user, msg)
}

func mentionUserInReplyByURL(ctx workflow.Context, parentURL string, user Account, msg string) error {
	ids, err := data.SwitchURLAndID(parentURL)
	if err != nil {
		log.Error(ctx, "failed to load PR comment's Slack IDs", "error", err, "url", parentURL)
		return err
	}

	id := strings.Split(ids, "/")
	if len(id) < 2 {
		log.Warn(ctx, "can't post Slack reply message - missing/bad IDs", "bitbucket_url", parentURL, "slack_ids", ids)
		return nil
	}

	return mentionUserInReply(ctx, id[0], id[1], user, msg)
}

func mentionUserInReply(ctx workflow.Context, channelID, threadTS string, user Account, msg string) error {
	// We don't want to use a Slack mention here, because that would spam the user in
	// Slack with a narration of their own actions. So we just write their name instead
	// of "users.BitbucketToSlackRef(ctx, user.AccountID, user.DisplayName)".
	slackUserID := users.BitbucketToSlackID(ctx, user.AccountID, false)
	displayName := users.SlackIDToDisplayName(ctx, slackUserID)
	if displayName == "" {
		displayName = user.DisplayName
	}

	if workspaceURL == "" {
		if resp, err := tslack.AuthTest(ctx); err == nil {
			workspaceURL = resp.URL
		}
	}

	// And if possible convert it into a profile link that LOOKS like a mention.
	if workspaceURL != "" && slackUserID != "" {
		displayName = fmt.Sprintf("<%steam/%s?preview=no|%s>", workspaceURL, slackUserID, displayName)
	}

	_, err := slack.PostReply(ctx, channelID, threadTS, fmt.Sprintf(msg, displayName))
	return err
}

func impersonateUserInMsg(ctx workflow.Context, url, channelID string, user Account, msg string, diff []byte) error {
	if diff != nil {
		msg = uploadDiff(ctx, diff, url, msg)
	}

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

func impersonateUserInReply(ctx workflow.Context, url, parentURL string, user Account, msg string, diff []byte) error {
	if diff != nil {
		msg = uploadDiff(ctx, diff, url, msg)
	}

	ids, err := data.SwitchURLAndID(parentURL)
	if err != nil {
		log.Error(ctx, "failed to load PR comment's Slack IDs", "error", err, "url", parentURL)
		return err
	}

	id := strings.Split(ids, "/")
	if len(id) < 2 {
		log.Warn(ctx, "can't post Slack reply message - missing/bad IDs", "bitbucket_url", parentURL, "slack_ids", ids)
		return nil
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

func uploadDiff(ctx workflow.Context, diff []byte, url, msg string) string {
	parts := strings.Split(url, "-")
	filename := parts[len(parts)-1] + ".diff"
	title := "Diff " + parts[len(parts)-1]

	file, err := slack.Upload(ctx, diff, filename, title, "diff", "text/x-diff", "", "")
	if err != nil || file == nil {
		return msg // File upload failed, return the original message unmodified.
	}

	// Success: replace the code block in the message with a prettier rendering of the file.
	parts = strings.Split(msg, "\n```")
	msg = fmt.Sprintf("%s<%s| >", parts[0], file.Permalink)
	if len(parts) > 2 {
		msg += parts[2]
	}

	return msg
}

func impersonateUser(ctx workflow.Context, user Account) (name, icon string) {
	id := users.BitbucketToSlackID(ctx, user.AccountID, false)
	if id == "" {
		name = user.DisplayName
		return
	}

	profile, err := tslack.UsersProfileGet(ctx, id)
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

	id := strings.Split(ids, "/")
	if len(id) < 2 {
		log.Warn(ctx, "can't delete Slack message - missing/bad IDs", "bitbucket_url", url, "slack_ids", ids)
		return nil
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
		return nil
	}

	return slack.UpdateMessage(ctx, id[0], id[len(id)-1], msg)
}
