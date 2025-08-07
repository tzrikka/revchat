package bitbucket

import (
	"fmt"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/slack"
)

func (b Bitbucket) mentionUserInMessage(ctx workflow.Context, channel string, user Account, msg string) (string, error) {
	msg = fmt.Sprintf(msg, user.AccountID)

	req := slack.ChatPostMessageRequest{Channel: channel, MarkdownText: msg}
	a := b.executeTimpaniActivity(ctx, slack.ChatPostMessageActivity, req)

	resp := slack.ChatPostMessageResponse{}
	if err := a.Get(ctx, &resp); err != nil {
		msg := "failed to post Slack message"
		workflow.GetLogger(ctx).Error(msg, "error", err, "channel", channel)
		return "", err
	}

	return resp.TS, nil
}
