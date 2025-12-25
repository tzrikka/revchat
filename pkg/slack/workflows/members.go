package workflows

import (
	"fmt"
	"log/slog"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack/activities"
)

// MemberJoinedWorkflow ensures that users who are added to RevChat channels by others
// are opted-in: https://docs.slack.dev/reference/events/member_joined_channel/
func MemberJoinedWorkflow(ctx workflow.Context, event memberEventWrapper) error {
	e := event.InnerEvent
	if !isRevChatChannel(ctx, e.Channel) || selfTriggeredMemberEvent(ctx, event.Authorizations, e) {
		return nil
	}

	user, err := data.SelectUserBySlackID(e.User)
	if err != nil {
		logger.From(ctx).Error("failed to load user by Slack ID",
			slog.Any("error", err), slog.String("user_id", e.User))
		return err
	}

	// If a user was added by someone else, check if the invitee is opted-in.
	if data.IsOptedIn(user) {
		return nil
	}

	// If not, send them an ephemeral message explaining how to opt-in.
	msg := ":wave: Hi <@%s>! You have joined a RevChat channel, but you have to opt-in for this to work! Please run "
	_, err = activities.PostMessage(ctx, e.User, fmt.Sprintf(msg, e.User)+"this slash command:\n\n```/revchat opt-in```")
	logger.From(ctx).Warn("user joined Slack channel but is not opted-in",
		slog.String("user_id", e.User), slog.String("channel_id", e.Channel))
	return err
}

// MemberLeftWorkflow is (or rather will be) based on:
// https://docs.slack.dev/reference/events/member_left_channel/
func MemberLeftWorkflow(ctx workflow.Context, event memberEventWrapper) error {
	e := event.InnerEvent
	if !isRevChatChannel(ctx, e.Channel) || selfTriggeredMemberEvent(ctx, event.Authorizations, e) {
		return nil
	}

	logger.From(ctx).Debug("Slack member left channel event - not implemented yet")
	return nil
}
