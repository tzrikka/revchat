package github

import (
	"fmt"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/slack"
	"github.com/tzrikka/revchat/pkg/users"
)

func (g GitHub) mentionUserInMsgAsync(ctx workflow.Context, channelID string, u User, msg string) {
	msg = fmt.Sprintf(msg, users.GitHubToSlackRef(ctx, g.cmd, u.Login, u.HTMLURL))
	req := slack.ChatPostMessageRequest{Channel: channelID, MarkdownText: msg}
	slack.PostChatMessageActivityAsync(ctx, g.cmd, req)
}

func (g GitHub) mentionUserInMsg(ctx workflow.Context, channelID string, u User, msg string) (string, error) {
	msg = fmt.Sprintf(msg, users.GitHubToSlackRef(ctx, g.cmd, u.Login, u.HTMLURL))
	req := slack.ChatPostMessageRequest{Channel: channelID, MarkdownText: msg}
	resp, err := slack.PostChatMessageActivity(ctx, g.cmd, req)
	if err != nil {
		return "", err
	}

	return resp.TS, nil
}

// func (g GitHub) mentionUserInReply(ctx workflow.Context, channelID, commentURL string, user User, msg string) string {
// 	return ""
// }
