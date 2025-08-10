package github

import (
	"fmt"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/slack"
	"github.com/tzrikka/revchat/pkg/users"
)

func (g GitHub) mentionUserInMsg(ctx workflow.Context, channelID string, u User, msg string) (string, error) {
	msg = fmt.Sprintf(msg, users.GitHubToSlackRef(ctx, g.cmd, u.Login, u.HTMLURL))

	req := slack.ChatPostMessageRequest{Channel: channelID, MarkdownText: msg}
	a := g.executeTimpaniActivity(ctx, slack.ChatPostMessageActivity, req)

	resp := slack.ChatPostMessageResponse{}
	if err := a.Get(ctx, &resp); err != nil {
		msg := "failed to post Slack message"
		workflow.GetLogger(ctx).Error(msg, "error", err, "channel_id", channelID)
		return "", err
	}

	return resp.TS, nil
}

// func (g GitHub) mentionUserInReply(ctx workflow.Context, channelID, commentURL string, user User, msg string) string {
// 	return ""
// }
