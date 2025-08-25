package slack

import (
	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/workflow"
)

// https://docs.slack.dev/reference/methods/chat.delete
type ChatDeleteRequest struct {
	Channel string `json:"channel"`
	TS      string `json:"ts"`

	AsUser bool `json:"as_user,omitempty"`
}

// https://docs.slack.dev/reference/methods/chat.delete
func DeleteChatMessageActivityAsync(ctx workflow.Context, cmd *cli.Command, req ChatDeleteRequest) workflow.Future {
	return executeTimpaniActivity(ctx, cmd, "slack.chat.delete", req)
}

// https://docs.slack.dev/reference/methods/chat.postMessage
type ChatPostMessageRequest struct {
	Channel string `json:"channel"`

	Blocks       []map[string]any `json:"blocks,omitempty"`
	Attachments  []map[string]any `json:"attachments,omitempty"`
	MarkdownText string           `json:"markdown_text,omitempty"`
	Text         string           `json:"text,omitempty"`

	ThreadTS       string `json:"thread_ts,omitempty"`
	ReplyBroadcast bool   `json:"reply_broadcast,omitempty"`

	IconEmoji string         `json:"icon_emoji,omitempty"`
	IconURL   string         `json:"icon_url,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`

	LinkNames bool `json:"link_names,omitempty"`
	// Ignoring "mrkdwn" for now, because it has an unusual default value (true).
	Parse       string `json:"parse,omitempty"`
	UnfurlLinks bool   `json:"unfurl_links,omitempty"`
	UnfurlMedia bool   `json:"unfurl_media,omitempty"`
	Username    string `json:"username,omitempty"`
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

// https://docs.slack.dev/reference/methods/chat.update
//
// https://docs.slack.dev/reference/methods/chat.postMessage#channels
type ChatUpdateRequest struct {
	Channel string `json:"channel"`
	TS      string `json:"ts"`

	Blocks       []map[string]any `json:"blocks,omitempty"`
	Attachments  []map[string]any `json:"attachments,omitempty"`
	MarkdownText string           `json:"markdown_text,omitempty"`
	Text         string           `json:"text,omitempty"`

	AsUser         bool           `json:"as_user,omitempty"`
	FileIDs        []string       `json:"file_ids,omitempty"`
	LinkNames      bool           `json:"link_names,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	Parse          string         `json:"parse,omitempty"`
	ReplyBroadcast bool           `json:"reply_broadcast,omitempty"`
}

// https://docs.slack.dev/reference/methods/chat.update
func UpdateChatMessageActivityAsync(ctx workflow.Context, cmd *cli.Command, req ChatUpdateRequest) workflow.Future {
	return executeTimpaniActivity(ctx, cmd, "slack.chat.update", req)
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
