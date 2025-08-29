package slack

import (
	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
)

// https://docs.slack.dev/reference/methods/bookmarks.add
type bookmarksAddRequest struct {
	ChannelID string `json:"channel_id"`
	Title     string `json:"title"`
	Type      string `json:"type"`

	Link        string `json:"link,omitempty"`
	Emoji       string `json:"emoji,omitempty"`
	EntityID    string `json:"entity_id,omitempty"`
	AccessLevel string `json:"access_level,omitempty"`
	ParentID    string `json:"parent_id,omitempty"`
}

// https://docs.slack.dev/reference/methods/bookmarks.add
func AddBookmarkActivity(ctx workflow.Context, cmd *cli.Command, channelID, title, url string) {
	req := bookmarksAddRequest{ChannelID: channelID, Title: title, Type: "link", Link: url}
	a := executeTimpaniActivity(ctx, cmd, "slack.bookmarks.add", req)

	if err := a.Get(ctx, nil); err != nil {
		msg := "failed to add new bookmark in Slack channel"
		log.Error(ctx, msg, "error", err, "channel_id", channelID, "title", title)
	}
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
