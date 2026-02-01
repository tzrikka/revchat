package bitbucket

import (
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack/activities"
	"github.com/tzrikka/revchat/pkg/users"
	tslack "github.com/tzrikka/timpani-api/pkg/slack"
)

// No need for thread safety here: this is set only once per process, and even
// if multiple workflows set it concurrently, the value will be the same anyway.
var workspaceURL = ""

func MentionUserInMsg(ctx workflow.Context, channelID string, user Account, msg string) {
	// Don't use fmt.Sprintf() here to avoid issues with % signs in the text.
	msg = strings.Replace(msg, "%s", SlackDisplayName(ctx, user), 1)

	// Failures here are already logged, and never a reason to abort the calling workflows.
	_, _ = activities.PostReply(ctx, channelID, "", msg)
}

func MentionUserInReply(ctx workflow.Context, parentURL string, user Account, msg string) error {
	ids, err := msgIDsForCommentURL(ctx, parentURL)
	if err != nil || ids == nil {
		return err
	}

	// Don't use fmt.Sprintf() here to avoid issues with % signs in the text.
	msg = strings.Replace(msg, "%s", SlackDisplayName(ctx, user), 1)

	_, err = activities.PostReply(ctx, ids[0], ids[1], msg)
	return err
}

// SlackDisplayName returns the Slack display name corresponding to the given Bitbucket user, linked
// to their Slack profile if possible. We use this instead of [users.BitbucketIDToSlackRef] in cases
// where we want to mention the user without actually spamming them with echoes of their own actions.
func SlackDisplayName(ctx workflow.Context, user Account) string {
	slackUserID := users.BitbucketIDToSlackID(ctx, user.AccountID, false)
	displayName := users.SlackIDToDisplayName(ctx, slackUserID)
	if displayName == "" {
		displayName = user.DisplayName
	}

	if workspaceURL == "" {
		if resp, err := tslack.AuthTest(ctx); err == nil {
			workspaceURL = resp.URL
		}
	}

	// And if possible convert it into a profile link that LOOKS like a mention
	// (but doesn't trigger Slack to attach a preview of the profile card to the message).
	if workspaceURL != "" && slackUserID != "" {
		displayName = fmt.Sprintf("<%steam/%s?preview=no|%s>", workspaceURL, slackUserID, displayName)
	}

	return displayName
}

func ImpersonateUserInMsg(ctx workflow.Context, url, channelID, alertsChannel string, user Account, msg string, diff []byte) error {
	return impersonateUserInBoth(ctx, url, channelID, "", channelID, alertsChannel, msg, user, diff)
}

func ImpersonateUserInReply(ctx workflow.Context, url, parentURL, alertsChannel string, user Account, msg string, diff []byte) error {
	ids, err := msgIDsForCommentURL(ctx, parentURL)
	if err != nil || ids == nil {
		return err
	}

	return impersonateUserInBoth(ctx, url, ids[0], ids[1], fmt.Sprintf("%s/%s", ids[0], ids[1]), alertsChannel, msg, user, diff)
}

// impersonateUserInBoth posts a Slack message or reply, impersonating the given user.
// If diff is non-nil, it uploads it to Slack and modifies the message to reference it.
// It also sets mappings between all of these, but doesn't return an error if any mapping fails.
func impersonateUserInBoth(ctx workflow.Context, url, channelID, threadTS, idPrefix, alertsChannel, msg string, user Account, diff []byte) error {
	fileID := ""
	if diff != nil {
		msg, fileID = uploadDiff(ctx, diff, url, msg)
	}

	ts, err := postAsUser(ctx, msg, channelID, threadTS, fileID, user)
	if err != nil {
		return err
	}

	slackIDs := fmt.Sprintf("%s/%s", idPrefix, ts)
	_ = activities.AlertError(ctx, alertsChannel, "failed to set mapping between a new Slack message and its Bitbucket URL",
		data.MapURLAndID(ctx, url, slackIDs), "Comment URL", url, "Slack IDs", slackIDs)

	if fileID == "" {
		return nil
	}

	// Also remember to delete diff files later, if we update or delete the message.
	fakeURL := url + "/slack_file_id"
	slackIDs = fmt.Sprintf("%s/%s", slackIDs, fileID)
	_ = activities.AlertError(ctx, alertsChannel, "failed to set mapping between a PR code suggestion and its diff file in Slack",
		data.MapURLAndID(ctx, fakeURL, slackIDs), "Fake URL", fakeURL, "Slack IDs", slackIDs)

	return nil
}

// uploadDiff uploads the given diff content to Slack and modifies the message
// to reference this uploaded file instead of Bitbucket's minimalistic code block.
// It returns the modified message and the uploaded file ID (or an empty string).
func uploadDiff(ctx workflow.Context, diff []byte, url, msg string) (string, string) {
	parts := strings.Split(url, "-")
	filename := parts[len(parts)-1] + ".diff"
	title := "Diff " + parts[len(parts)-1]

	file, err := activities.Upload(ctx, diff, filename, title, "diff", "text/x-diff", "", "")
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
func postAsUser(ctx workflow.Context, msg, channelID, threadTS, fileID string, user Account) (string, error) {
	var resp *tslack.ChatPostMessageResponse
	var err error

	if fileID != "" {
		// Don't use fmt.Sprintf() here to avoid issues with % signs in the text.
		msg = strings.Replace(ImpersonationToMention(msg), "%s", SlackDisplayName(ctx, user), 1)
		resp, err = activities.PostReply(ctx, channelID, threadTS, msg)
	} else {
		displayName, icon := impersonateUser(ctx, user)
		resp, err = activities.PostReplyAsUser(ctx, channelID, threadTS, displayName, icon, msg)
	}

	if err != nil {
		return "", err
	}
	return resp.TS, nil
}

var prefixPattern = regexp.MustCompile(`^<([^|]+)\|(File|Inline) comment>`)

// ImpersonationToMention converts a message that was meant to be used in
// [ImpersonateUserInMsg] or [ImpersonateUserInReply] into a message that can
// be used in [MentionUserInMsg] or [MentionUserInReply], by adjusting the prefix.
//
// This is potentially relevant only for file and inline comments: if the message
// includes a file (i.e. contains a Slack file permalink), we can't impersonate the
// user, because the Slack API won't allow us to update or delete the message later.
func ImpersonationToMention(msg string) string {
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
	id := users.BitbucketIDToSlackID(ctx, user.AccountID, false)
	if id == "" {
		return user.DisplayName, ""
	}

	return users.SlackIDToDisplayName(ctx, id), users.SlackIDToIcon(ctx, id)
}

func DeleteSlackMsg(ctx workflow.Context, url string) error {
	ids, err := msgIDsForCommentURL(ctx, url)
	if err != nil || ids == nil {
		return err
	}

	data.DeleteURLAndIDMapping(ctx, url)

	return activities.DeleteMessage(ctx, ids[0], ids[len(ids)-1])
}

func EditSlackMsg(ctx workflow.Context, url, msg string) error {
	ids, err := msgIDsForCommentURL(ctx, url)
	if err != nil || ids == nil {
		return err
	}

	return activities.UpdateMessage(ctx, ids[0], ids[len(ids)-1], msg)
}

func msgIDsForCommentURL(ctx workflow.Context, url string) ([]string, error) {
	ids, err := data.SwitchURLAndID(ctx, url)
	if err != nil {
		return nil, err
	}
	if ids == "" {
		// When calling this function, we know the event is relevant, but we don't
		// know if the PR is older than RevChat's history, which is beyond our control.
		// Even so, if we can't find the mapping, it's likely that something is wrong.
		logger.From(ctx).Debug("didn't find PR comment's Slack message IDs", slog.String("comment_url", url))
		return nil, errors.New("didn't find PR comment's Slack message IDs")
	}

	parts := strings.Split(ids, "/")
	if len(parts) < 2 {
		logger.From(ctx).Error("failed to parse Slack message IDs",
			slog.String("comment_url", url), slog.String("slack_ids", ids))
		return nil, errors.New("invalid Slack message IDs: " + ids)
	}

	return parts, nil
}
