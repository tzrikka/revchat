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
func (c *Config) MemberJoinedWorkflow(ctx workflow.Context, event memberEventWrapper) error {
	e := event.InnerEvent
	if selfTriggeredMemberEvent(ctx, event.Authorizations, e) || !c.isRevChatChannel(ctx, e.Channel) {
		return nil
	}

	if _, optedIn, err := data.SelectUserBySlackID(ctx, e.User); err != nil || optedIn {
		// The error is nil (and there's no alert) if the user is opted-in.
		return activities.AlertError(ctx, c.AlertsChannel, "", err, "User ID", e.User)
	}

	// If the user isn't opted-in, send them a DM explaining how to opt-in.
	mention := fmt.Sprintf("<@%s>", e.User)
	logger.From(ctx).Warn("user joined Slack channel but is not opted-in",
		slog.String("user_id", e.User), slog.String("channel_id", e.Channel))
	activities.AlertWarn(ctx, c.AlertsChannel, "User joined Slack channel but is not opted-in!",
		"User", mention, "Channel ID", e.Channel)

	msg := ":wave: Hi %s! You joined a RevChat channel, but you have to opt-in for this to work! Please run "
	return activities.PostMessage(ctx, e.User, fmt.Sprintf(msg, mention)+"this Slack command:\n\n```/revchat opt-in```")
}

// MemberLeftWorkflow is (or rather will be) based on:
// https://docs.slack.dev/reference/events/member_left_channel/
func (c *Config) MemberLeftWorkflow(ctx workflow.Context, event memberEventWrapper) error {
	e := event.InnerEvent
	if selfTriggeredMemberEvent(ctx, event.Authorizations, e) || !c.isRevChatChannel(ctx, e.Channel) {
		return nil
	}

	logger.From(ctx).Debug("Slack member left channel event - not implemented yet")
	return nil
}
