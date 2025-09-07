package slack

import (
	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
)

// https://docs.slack.dev/reference/methods/chat.delete
type chatDeleteRequest struct {
	Channel string `json:"channel"`
	TS      string `json:"ts"`

	AsUser bool `json:"as_user,omitempty"`
}

// https://docs.slack.dev/reference/methods/chat.delete
func DeleteChatMessageActivity(ctx workflow.Context, cmd *cli.Command, channelID, timestamp string) error {
	a := executeTimpaniActivity(ctx, cmd, "slack.chat.delete", chatDeleteRequest{Channel: channelID, TS: timestamp})

	if err := a.Get(ctx, nil); err != nil {
		log.Error(ctx, "failed to delete Slack message", "error", err)
		return err
	}

	return nil
}

// https://docs.slack.dev/reference/methods/chat.postEphemeral
type chatPostEphemeralRequest struct {
	Channel string `json:"channel"`
	User    string `json:"user"`

	Blocks       []map[string]any `json:"blocks,omitempty"`
	Attachments  []map[string]any `json:"attachments,omitempty"`
	MarkdownText string           `json:"markdown_text,omitempty"`
	Text         string           `json:"text,omitempty"`
}

// https://docs.slack.dev/reference/methods/chat.postEphemeral
func PostEphemeralMessageActivity(ctx workflow.Context, cmd *cli.Command, channelID, userID, msg string) error {
	req := chatPostEphemeralRequest{Channel: channelID, User: userID, MarkdownText: msg}
	a := executeTimpaniActivity(ctx, cmd, "slack.chat.postEphemeral", req)

	if err := a.Get(ctx, nil); err != nil {
		log.Error(ctx, "failed to post Slack ephemeral message", "error", err)
		return err
	}

	return nil
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
func PostChatMessageActivity(ctx workflow.Context, cmd *cli.Command, req ChatPostMessageRequest) (*ChatPostMessageResponse, error) {
	a := executeTimpaniActivity(ctx, cmd, "slack.chat.postMessage", req)

	resp := &ChatPostMessageResponse{}
	if err := a.Get(ctx, resp); err != nil {
		log.Error(ctx, "failed to post Slack message", "error", err)
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
func UpdateChatMessageActivity(ctx workflow.Context, cmd *cli.Command, req ChatUpdateRequest) error {
	a := executeTimpaniActivity(ctx, cmd, "slack.chat.update", req)

	if err := a.Get(ctx, nil); err != nil {
		log.Error(ctx, "failed to update Slack message", "error", err)
		return err
	}

	return nil
}
