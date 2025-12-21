package slack

import (
	"fmt"
	"log/slog"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
)

func (c *Config) memberJoinedWorkflow(ctx workflow.Context, event memberEventWrapper) error {
	e := event.InnerEvent
	if !prChannel(ctx, e.Channel) || selfTriggeredMemberEvent(ctx, event.Authorizations, e) {
		return nil
	}

	user, err := data.SelectUserBySlackID(e.User)
	if err != nil {
		logger.Error(ctx, "failed to load user by Slack ID", err, slog.String("user_id", e.User))
		return err
	}

	// If a user was added by someone else, check if the invitee is opted-in.
	if data.IsOptedIn(user) {
		return nil
	}

	// If not, send them an ephemeral message explaining how to opt-in.
	logger.Warn(ctx, "user joined Slack channel but is not opted-in",
		slog.String("user_id", e.User), slog.String("channel_id", e.Channel))
	msg := ":wave: Hi <@%s>! You have joined a RevChat channel, but you have to opt-in for this to work! Please "
	_, err = PostMessage(ctx, e.User, fmt.Sprintf(msg, e.User)+"run this slash command:\n\n```/revchat opt-in```")
	return err
}

func (c *Config) memberLeftWorkflow(ctx workflow.Context, event memberEventWrapper) error {
	e := event.InnerEvent
	if !prChannel(ctx, e.Channel) || selfTriggeredMemberEvent(ctx, event.Authorizations, e) {
		return nil
	}

	logger.Warn(ctx, "member left Slack channel - event handler not implemented yet")
	return nil
}
