package slack

import (
	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/workflow"
)

// https://docs.slack.dev/reference/methods/reactions.add
type reactionsAddRequest struct {
	Channel   string `json:"channel"`
	Timestamp string `json:"timestamp"`
	Name      string `json:"name"`
}

// https://docs.slack.dev/reference/methods/reactions.add
func AddReactionActivity(ctx workflow.Context, cmd *cli.Command, channelID, timestamp, emoji string) error {
	req := reactionsAddRequest{Channel: channelID, Timestamp: timestamp, Name: emoji}
	a := executeTimpaniActivity(ctx, cmd, "slack.reactions.add", req)

	if err := a.Get(ctx, nil); err != nil {
		msg := "failed to add reaction to Slack message"
		workflow.GetLogger(ctx).Error(msg, "error", err, "channel_id", channelID, "timestamp", timestamp, "emoji", emoji)
		return err
	}

	return nil
}

// https://docs.slack.dev/reference/methods/reactions.remove
type reactionsRemoveRequest struct {
	Name string `json:"name"`

	Channel     string `json:"channel,omitempty"`
	File        string `json:"file,omitempty"`
	FileComment string `json:"file_comment,omitempty"`
	Timestamp   string `json:"timestamp,omitempty"`
}

// https://docs.slack.dev/reference/methods/reactions.remove
func RemoveReactionActivity(ctx workflow.Context, cmd *cli.Command, channelID, timestamp, emoji string) error {
	req := reactionsRemoveRequest{Channel: channelID, Timestamp: timestamp, Name: emoji}
	a := executeTimpaniActivity(ctx, cmd, "slack.reactions.remove", req)

	if err := a.Get(ctx, nil); err != nil {
		msg := "failed to remove reaction from Slack message"
		workflow.GetLogger(ctx).Error(msg, "error", err, "channel_id", channelID, "timestamp", timestamp, "emoji", emoji)
		return err
	}

	return nil
}
