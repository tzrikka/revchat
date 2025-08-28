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
