package slack

import (
	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/workflow"
)

// https://docs.slack.dev/reference/methods/chat.postMessage
type ChatPostMessageRequest struct {
	Channel string `json:"channel"`

	Attachments  []map[string]any `json:"attachments,omitempty"`
	Blocks       []map[string]any `json:"blocks,omitempty"`
	IconEmoji    string           `json:"icon_emoji,omitempty"`
	IconURL      string           `json:"icon_url,omitempty"`
	LinkNames    bool             `json:"link_names,omitempty"`
	MarkdownText string           `json:"markdown_text,omitempty"`
	Metadata     map[string]any   `json:"metadata,omitempty"`
	// Ignoring "mrkdwn" for now, because it has an unusual default value (true).
	Parse          string `json:"parse,omitempty"`
	ReplyBroadcast bool   `json:"reply_broadcast,omitempty"`
	Text           string `json:"text,omitempty"`
	ThreadTS       string `json:"thread_ts,omitempty"`
	UnfurnLinks    bool   `json:"unfurl_links,omitempty"`
	Username       string `json:"username,omitempty"`
}

// https://docs.slack.dev/reference/methods/chat.postMessage
type ChatPostMessageResponse struct {
	slackResponse

	Channel string         `json:"channel,omitempty"`
	TS      string         `json:"ts,omitempty"`
	Message map[string]any `json:"message,omitempty"`
}

// https://docs.slack.dev/reference/methods/chat.postMessage
func PostChatMessageActivityAsync(ctx workflow.Context, cmd *cli.Command, req ChatPostMessageRequest) workflow.Future {
	return executeTimpaniActivity(ctx, cmd, "slack.chat.postMessage", req)
}

// https://docs.slack.dev/reference/methods/chat.postMessage
func PostChatMessageActivity(ctx workflow.Context, cmd *cli.Command, req ChatPostMessageRequest) (*ChatPostMessageResponse, error) {
	a := PostChatMessageActivityAsync(ctx, cmd, req)

	resp := &ChatPostMessageResponse{}
	if err := a.Get(ctx, resp); err != nil {
		msg := "failed to post Slack message"
		workflow.GetLogger(ctx).Error(msg, "error", err)
		return nil, err
	}

	return resp, nil
}

type slackResponse struct {
	OK               bool              `json:"ok"`
	Error            string            `json:"error,omitempty"`
	Needed           string            `json:"needed,omitempty"`   // Scope errors (undocumented).
	Provided         string            `json:"provided,omitempty"` // Scope errors (undocumented).
	Warning          string            `json:"warning,omitempty"`
	ResponseMetadata *responseMetadata `json:"response_metadata,omitempty"`
}

type responseMetadata struct {
	Messages   []string `json:"messages,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
	NextCursor string   `json:"next_cursor,omitempty"`
}
