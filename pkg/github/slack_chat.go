package github

import (
	"fmt"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/slack"
	"github.com/tzrikka/revchat/pkg/users"
)

func (c Config) mentionUserInMsg(ctx workflow.Context, channelID string, u User, msg string) error {
	msg = fmt.Sprintf(msg, users.GitHubToSlackRef(ctx, c.Cmd, u.Login, u.HTMLURL))
	req := slack.ChatPostMessageRequest{Channel: channelID, MarkdownText: msg}
	_, err := slack.PostChatMessageActivity(ctx, c.Cmd, req)
	return err
}
