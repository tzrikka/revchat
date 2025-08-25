package slack

import (
	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/workflow"
)

// https://docs.slack.dev/reference/methods/reactions.add
type ReactionsAddRequest struct {
	Channel   string `json:"channel"`
	Timestamp string `json:"timestamp"`
	Name      string `json:"name"`
}

// https://docs.slack.dev/reference/methods/reactions.add
func AddReactionActivityAsync(ctx workflow.Context, cmd *cli.Command, req ReactionsAddRequest) workflow.Future {
	return executeTimpaniActivity(ctx, cmd, "slack.reactions.add", req)
}

// https://docs.slack.dev/reference/methods/reactions.remove
type ReactionsRemoveRequest struct {
	Name string `json:"name"`

	Channel     string `json:"channel,omitempty"`
	File        string `json:"file,omitempty"`
	FileComment string `json:"file_comment,omitempty"`
	Timestamp   string `json:"timestamp,omitempty"`
}

// https://docs.slack.dev/reference/methods/reactions.remove
func RemoveReactionActivityAsync(ctx workflow.Context, cmd *cli.Command, req ReactionsRemoveRequest) workflow.Future {
	return executeTimpaniActivity(ctx, cmd, "slack.reactions.remove", req)
}
