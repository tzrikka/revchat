package github

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
	"github.com/tzrikka/timpani-api/pkg/slack"
)

// No need for thread safety here: this is set only once per process, and even
// if multiple workflows set it concurrently, the value will be the same anyway.
var workspaceURL = ""

func MentionUserInMsg(ctx workflow.Context, channelID string, user User, msg string) {
	// Don't use fmt.Sprintf() here to avoid issues with % signs in the text.
	msg = strings.Replace(msg, "%s", SlackDisplayName(ctx, user), 1)

	// Failures here are already logged, and never a reason to abort the calling workflows.
	_, _ = activities.PostReply(ctx, channelID, "", msg)
}

func MentionUserInReply(ctx workflow.Context, parentURL string, user User, msg string) error {
	ids, err := msgIDsForCommentURL(ctx, parentURL)
	if err != nil || ids == nil {
		return err
	}

	// Don't use fmt.Sprintf() here to avoid issues with % signs in the text.
	msg = strings.Replace(msg, "%s", SlackDisplayName(ctx, user), 1)

	_, err = activities.PostReply(ctx, ids[0], ids[1], msg)
	return err
}

// SlackDisplayName returns the Slack display name corresponding to the given GitHub user, linked
// to their Slack profile if possible. We use this instead of [users.GitHubIDToSlackRef] in cases
// where we want to mention the user without actually spamming them with echoes of their own actions.
func SlackDisplayName(ctx workflow.Context, user User) string {
	// Exception: GitHub teams can only be mentioned by their GitHub name and link.
	if strings.Contains(user.Login, "/") {
		return fmt.Sprintf("<%s?preview=no|@%s>", user.HTMLURL, user.Login)
	}

	u := data.SelectUserByGitHubID(ctx, user.Login)
	if u.SlackID == "" {
		// Workaround in case only the user's GitHub account ID isn't stored yet, but the rest is.
		u = data.SelectUserByEmail(ctx, users.GitHubIDToEmail(ctx, user.Login))
	}

	displayName := users.SlackIDToDisplayName(ctx, u.SlackID)
	if displayName == "" {
		displayName = u.RealName
	}
	if displayName == "" {
		displayName = "@" + user.Login
	}

	if workspaceURL == "" {
		if resp, err := slack.AuthTest(ctx); err == nil {
			workspaceURL = resp.URL
		}
	}

	// And if possible convert it into a profile link that LOOKS like a mention
	// (but doesn't trigger Slack to attach a preview of the profile card to the message).
	if workspaceURL != "" && u.SlackID != "" {
		displayName = fmt.Sprintf("<%steam/%s?preview=no|%s>", workspaceURL, u.SlackID, displayName)
	} else {
		// Fallback: link to the GitHub user profile.
		displayName = fmt.Sprintf("<%s?preview=no|%s>", user.HTMLURL, displayName)
	}

	return displayName
}

func ImpersonateUserInMsg(ctx workflow.Context, url, channelID string, user User, msg string, diff []byte) error {
	return impersonateUserInBoth(ctx, url, channelID, "", channelID, msg, user, diff)
}

func ImpersonateUserInReply(ctx workflow.Context, url, parentURL string, user User, msg string, diff []byte) error {
	ids, err := msgIDsForCommentURL(ctx, parentURL)
	if err != nil || ids == nil {
		return err
	}

	return impersonateUserInBoth(ctx, url, ids[0], ids[1], fmt.Sprintf("%s/%s", ids[0], ids[1]), msg, user, diff)
}

func impersonateUserInBoth(ctx workflow.Context, url, channelID, threadTS, idPrefix, msg string, user User, diff []byte) error {
	fileID := ""
	if diff != nil {
		msg, fileID = uploadDiff(ctx, diff, url, msg)
	}

	ts, err := postAsUser(ctx, msg, channelID, threadTS, fileID, user)
	if err != nil {
		return err
	}

	_ = data.MapURLAndID(ctx, url, fmt.Sprintf("%s/%s", idPrefix, ts))

	if fileID == "" {
		return nil
	}

	// Also remember to delete diff files later, if we update or delete the message.
	_ = data.MapURLAndID(ctx, url+"/slack_file_id", fmt.Sprintf("%s/%s/%s", idPrefix, ts, fileID))
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
func postAsUser(ctx workflow.Context, msg, channelID, threadTS, fileID string, user User) (string, error) {
	var resp *slack.ChatPostMessageResponse
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

func impersonateUser(ctx workflow.Context, user User) (displayName, icon string) {
	id := users.GitHubIDToSlackID(ctx, user.Login, false)
	if id == "" {
		return user.Login, ""
	}

	return users.SlackIDToDisplayName(ctx, id), users.SlackIDToIcon(ctx, id)
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
