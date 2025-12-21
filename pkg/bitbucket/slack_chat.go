package bitbucket

import (
	"fmt"
	"regexp"
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

func mentionUserInMsg(ctx workflow.Context, channelID string, user Account, msg string) {
	// Don't use fmt.Sprintf() here to avoid issues with % signs in the text.
	msg = strings.Replace(msg, "%s", slackDisplayName(ctx, user), 1)

	// Failures here are already logged, and never a reason to abort the calling workflows.
	_, _ = slack.PostReply(ctx, channelID, "", msg)
}

func mentionUserInReplyByURL(ctx workflow.Context, parentURL string, user Account, msg string) error {
	ids, err := msgIDsForCommentURL(ctx, parentURL, "post")
	if err != nil || ids == nil {
		return err
	}

	// Don't use fmt.Sprintf() here to avoid issues with % signs in the text.
	msg = strings.Replace(msg, "%s", slackDisplayName(ctx, user), 1)

	_, err = slack.PostReply(ctx, ids[0], ids[1], msg)
	return err
}

func slackDisplayName(ctx workflow.Context, user Account) string {
	// We don't want to use a Slack mention, because that would spam the user in
	// Slack with echoes of their own actions. So we just write their name instead
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

	return displayName
}

func impersonateUserInMsg(ctx workflow.Context, url, channelID string, user Account, msg string, diff []byte) error {
	fileID := ""
	if diff != nil {
		msg, fileID = uploadDiff(ctx, diff, url, msg)
	}

	resp, err := postAsUser(ctx, msg, channelID, "", fileID, user)
	if err != nil {
		return err
	}

	id := fmt.Sprintf("%s/%s", channelID, resp.TS)
	if err := data.MapURLAndID(url, id); err != nil {
		log.Error(ctx, "failed to save PR comment URL / Slack IDs mapping",
			"error", err, "bitbucket_url", url, "slack_id", id)
		// Don't return the error - the message is already posted in Slack, so we
		// don't want to retry and post it again, even though this is problematic.
	}

	// Also remember to delete diff files later, if we update or delete the message.
	if fileID != "" {
		if err := data.MapURLAndID(url+"/slack_file_id", fmt.Sprintf("%s/%s", id, fileID)); err != nil {
			log.Error(ctx, "failed to save Slack file mapping", "error", err,
				"bitbucket_url", url, "slack_id", id, "file_id", fileID)
			// Don't return the error - the message is already posted in Slack, so we
			// don't want to retry and post it again, even though this is problematic.
		}
	}

	return nil
}

func impersonateUserInReply(ctx workflow.Context, url, parentURL string, user Account, msg string, diff []byte) error {
	ids, err := msgIDsForCommentURL(ctx, parentURL, "post")
	if err != nil || ids == nil {
		return err
	}

	fileID := ""
	if diff != nil {
		msg, fileID = uploadDiff(ctx, diff, url, msg)
	}

	resp, err := postAsUser(ctx, msg, ids[0], ids[1], fileID, user)
	if err != nil {
		return err
	}

	sid := fmt.Sprintf("%s/%s/%s", ids[0], ids[1], resp.TS)
	if err := data.MapURLAndID(url, sid); err != nil {
		log.Error(ctx, "failed to save PR comment URL / Slack IDs mapping",
			"error", err, "bitbucket_url", url, "slack_id", sid)
		// Don't return the error - the message is already posted in Slack, so we
		// don't want to retry and post it again, even though this is problematic.
	}

	// Also remember to delete diff files later, if we update or delete the message.
	if fileID != "" {
		if err := data.MapURLAndID(url+"/slack_file_id", fmt.Sprintf("%s/%s", sid, fileID)); err != nil {
			log.Error(ctx, "failed to save Slack file mapping", "error", err,
				"bitbucket_url", url, "slack_id", sid, "file_id", fileID)
			// Don't return the error - the message is already posted in Slack, so we
			// don't want to retry and post it again, even though this is problematic.
		}
	}

	return nil
}

// uploadDiff uploads the given diff content to Slack and modifies the message
// to reference this uploaded file instead of Bitbucket's minimalistic code block.
// It returns the modified message and the uploaded file ID (or an empty string).
func uploadDiff(ctx workflow.Context, diff []byte, url, msg string) (string, string) {
	parts := strings.Split(url, "-")
	filename := parts[len(parts)-1] + ".diff"
	title := "Diff " + parts[len(parts)-1]

	file, err := slack.Upload(ctx, diff, filename, title, "diff", "text/x-diff", "", "")
	if err != nil || file == nil {
		return msg, "" // File upload failed, return the original message unmodified.
	}

	// Success: replace the code block in the message with a prettier rendering of the file.
	parts = strings.Split(msg, "\n```")
	msg = fmt.Sprintf("%s<%s| >", parts[0], file.Permalink)
	if len(parts) > 2 {
		msg += parts[2]
	}

	return msg, file.ID
}

// postAsUser posts a Slack message or a reply, either impersonating the given user
// (if fileID is empty) or mentioning them (if fileID is non-empty).
//
// This distinction is necessary due to a limitation in the Slack API: messages posted as
// another user ("impersonated") cannot be updated or deleted if they include file attachments.
func postAsUser(ctx workflow.Context, msg, channelID, threadTS, fileID string, user Account) (*tslack.ChatPostMessageResponse, error) {
	if fileID != "" {
		// Don't use fmt.Sprintf() here to avoid issues with % signs in the text.
		msg = strings.Replace(impersonationToMention(msg), "%s", slackDisplayName(ctx, user), 1)
		return slack.PostReply(ctx, channelID, threadTS, msg)
	}

	displayName, icon := impersonateUser(ctx, user)
	return slack.PostReplyAsUser(ctx, channelID, threadTS, displayName, icon, msg)
}

var prefixPattern = regexp.MustCompile(`^<([^|]+)\|(File|Inline) comment>`)

// impersonationToMention converts a message that was meant to be used in
// [impersonateUserInMsg] or [impersonateUserInReply] into a message that can
// be used in [mentionUserInMsg] or [mentionUserInReply], by adjusting the prefix.
//
// This is potentially relevant only for file and inline comments: if the message
// includes a file (i.e. contains a Slack file permalink), we can't impersonate the
// user, because the Slack API won't allow us to update or delete the message later.
func impersonationToMention(msg string) string {
	match := prefixPattern.FindStringSubmatch(msg)
	if len(match) < 3 {
		return msg
	}

	lower := strings.ToLower(match[2])
	article := "a"
	if lower == "inline" {
		article = "an"
	}

	newPrefix := fmt.Sprintf("posted %s <%s|%s comment>", article, match[1], lower)
	return strings.Replace(msg, match[0], "%s "+newPrefix, 1)
}

func impersonateUser(ctx workflow.Context, user Account) (displayName, icon string) {
	id := users.BitbucketToSlackID(ctx, user.AccountID, false)
	if id == "" {
		return user.DisplayName, ""
	}

	return users.SlackIDToDisplayName(ctx, id), users.SlackIDToIcon(ctx, id)
}

func deleteMsg(ctx workflow.Context, url string) error {
	ids, err := msgIDsForCommentURL(ctx, url, "delete")
	if err != nil || ids == nil {
		return err
	}

	if err := data.DeleteURLAndIDMapping(url); err != nil {
		log.Error(ctx, "failed to delete URL/Slack mappings", "error", err, "comment_url", url)
		// Don't abort - we still want to attempt to delete the Slack message.
	}

	return slack.DeleteMessage(ctx, ids[0], ids[len(ids)-1])
}

func editMsg(ctx workflow.Context, url, msg string) error {
	ids, err := msgIDsForCommentURL(ctx, url, "update")
	if err != nil || ids == nil {
		return err
	}

	return slack.UpdateMessage(ctx, ids[0], ids[len(ids)-1], msg)
}

func msgIDsForCommentURL(ctx workflow.Context, url, action string) ([]string, error) {
	ids, err := data.SwitchURLAndID(url)
	if err != nil {
		log.Error(ctx, "failed to load PR comment's Slack IDs", "error", err, "url", url)
		return nil, err
	}

	parts := strings.Split(ids, "/")
	if len(parts) < 2 {
		msg := fmt.Sprintf("can't %s Slack message - missing/bad IDs", action)
		log.Warn(ctx, msg, "bitbucket_url", url, "slack_ids", ids)
		return nil, nil
	}

	return parts, nil
}
