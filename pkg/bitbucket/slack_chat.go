package bitbucket

import (
	"fmt"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack"
)

func (b Bitbucket) mentionUserInMessage(ctx workflow.Context, channel string, user Account, msg string) (string, error) {
	msg = fmt.Sprintf(msg, b.slackUserRef(ctx, user))

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

func (b Bitbucket) slackUserRef(ctx workflow.Context, user Account) string {
	l := workflow.GetLogger(ctx)

	email, err := data.BitbucketUserEmailByID(user.AccountID)
	if err != nil {
		l.Error("failed to read Bitbucket user email", "error", err)
		return user.DisplayName
	}

	if email == "" {
		return user.DisplayName
	}

	id, err := data.SlackUserIDByEmail(email)
	if err != nil {
		l.Error("failed to read Slack user ID", "error", err)

		req := slack.UsersLookupByEmailRequest{Email: email}
		a := b.executeTimpaniActivity(ctx, slack.UsersLookupByEmailActivity, req)

		resp := slack.UsersLookupByEmailResponse{}
		if err := a.Get(ctx, &resp); err != nil {
			l.Error("failed to lookup user email in Slack", "error", err, "email", email)
			return user.DisplayName
		}

		var ok bool
		if id, ok = resp.User["id"].(string); !ok {
			return user.DisplayName
		}
	}

	if id == "" {
		l.Error("got an empty Slack user ID", "email", email)
		return user.DisplayName
	}

	return fmt.Sprintf("<@%s>", id)
}
