package slack

import (
	"fmt"
	"log/slog"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/timpani-api/pkg/slack"
)

func DeleteMessage(ctx workflow.Context, channelID, timestamp string) error {
	if err := slack.ChatDelete(ctx, channelID, timestamp); err != nil {
		logger.Error(ctx, "failed to delete Slack message", err,
			slog.String("channel_id", channelID), slog.String("timestamp", timestamp))
		return err
	}
	return nil
}

func PostEphemeralMessage(ctx workflow.Context, channelID, userID, msg string) error {
	req := slack.ChatPostEphemeralRequest{Channel: channelID, User: userID, Text: msg}
	if err := slack.ChatPostEphemeral(ctx, req); err != nil {
		if e := err.Error(); strings.Contains(e, "channel_not_found") || strings.Contains(e, "not_in_channel") {
			_, err = PostMessage(ctx, userID, fmt.Sprintf("Couldn't send you this message in <#%s>:\n\n%s", channelID, msg))
		} else {
			logger.Error(ctx, "failed to post Slack ephemeral message", err,
				slog.String("channel_id", channelID), slog.String("user_id", userID))
		}
		return err
	}
	return nil
}

func PostMessage(ctx workflow.Context, channelID, msg string) (*slack.ChatPostMessageResponse, error) {
	return PostReplyAsUser(ctx, channelID, "", "", "", msg)
}

func PostReply(ctx workflow.Context, channelID, timestamp, msg string) (*slack.ChatPostMessageResponse, error) {
	return PostReplyAsUser(ctx, channelID, timestamp, "", "", msg)
}

func PostReplyAsUser(ctx workflow.Context, channelID, timestamp, name, icon, msg string) (*slack.ChatPostMessageResponse, error) {
	resp, err := slack.ChatPostMessage(ctx, slack.ChatPostMessageRequest{
		Channel:  channelID,
		ThreadTS: timestamp,
		Username: name,
		IconURL:  icon,
		Text:     msg,
	})
	if err != nil {
		// If the channel is archived but we still store data for it, clean it up.
		if strings.Contains(err.Error(), "is_archived") {
			url, _ := data.SwitchURLAndID(channelID)
			data.FullPRCleanup(ctx, channelID, url)
		}
		logger.Error(ctx, "failed to post Slack message", err,
			slog.String("channel_id", channelID), slog.String("thread_ts", timestamp))
		return nil, err
	}
	return resp, nil
}

func UpdateMessage(ctx workflow.Context, channelID, timestamp, msg string) error {
	req := slack.ChatUpdateRequest{Channel: channelID, TS: timestamp, Text: msg}
	if err := slack.ChatUpdate(ctx, req); err != nil {
		logger.Error(ctx, "failed to update Slack message", err,
			slog.String("channel_id", channelID), slog.String("timestamp", timestamp))
		return err
	}
	return nil
}
