package bitbucket

import (
	"fmt"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/slack"
	"github.com/tzrikka/revchat/pkg/users"
)

func (b Bitbucket) mentionUserInMsg(ctx workflow.Context, channelID string, user Account, msg string) (string, error) {
	msg = fmt.Sprintf(msg, users.BitbucketToSlackRef(ctx, b.cmd, user.AccountID, user.DisplayName))

	req := slack.ChatPostMessageRequest{Channel: channelID, MarkdownText: msg}
	resp, err := slack.PostChatMessageActivity(ctx, b.cmd, req)
	if err != nil {
		return "", err
	}

	return resp.TS, nil
}
