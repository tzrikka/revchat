package slack

import (
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/timpani-api/pkg/slack"
)

func DeleteMessage(ctx workflow.Context, channelID, timestamp string) error {
	if err := slack.ChatDeleteActivity(ctx, channelID, timestamp); err != nil {
		log.Error(ctx, "failed to delete Slack message", "error", err, "channel_id", channelID, "timestamp", timestamp)
		return err
	}
	return nil
}

func PostEphemeralMessage(ctx workflow.Context, channelID, userID, msg string) error {
	req := slack.ChatPostEphemeralRequest{Channel: channelID, User: userID, Text: msg}
	if err := slack.ChatPostEphemeralActivity(ctx, req); err != nil {
		log.Error(ctx, "failed to post Slack ephemeral message", "error", err)
		return err
	}
	return nil
}

func PostMessage(ctx workflow.Context, channelID, msg string) (*slack.ChatPostMessageResponse, error) {
	return PostReplyAsUser(ctx, channelID, "", "", "", msg)
}

func PostMessageAsUser(ctx workflow.Context, channelID, name, icon, msg string) (*slack.ChatPostMessageResponse, error) {
	return PostReplyAsUser(ctx, channelID, "", name, icon, msg)
}

func PostReply(ctx workflow.Context, channelID, timestamp, msg string) (*slack.ChatPostMessageResponse, error) {
	return PostReplyAsUser(ctx, channelID, timestamp, "", "", msg)
}

func PostReplyAsUser(ctx workflow.Context, channelID, timestamp, name, icon, msg string) (*slack.ChatPostMessageResponse, error) {
	resp, err := slack.ChatPostMessageActivity(ctx, slack.ChatPostMessageRequest{
		Channel:  channelID,
		ThreadTS: timestamp,
		Username: name,
		IconURL:  icon,
		Text:     msg,
	})
	if err != nil {
		log.Error(ctx, "failed to post Slack message", "error", err, "channel_id", channelID, "thread_ts", timestamp)
		return nil, err
	}
	return resp, nil
}

func UpdateMessage(ctx workflow.Context, channelID, timestamp, msg string) error {
	req := slack.ChatUpdateRequest{Channel: channelID, TS: timestamp, MarkdownText: msg}
	if err := slack.ChatUpdateActivity(ctx, req); err != nil {
		log.Error(ctx, "failed to update Slack message", "error", err, "channel_id", channelID, "timestamp", timestamp)
		return err
	}
	return nil
}
