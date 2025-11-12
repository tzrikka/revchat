package slack

import (
	"fmt"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/revchat/pkg/data"
)

func (c *Config) memberJoinedWorkflow(ctx workflow.Context, event memberEventWrapper) error {
	e := event.InnerEvent
	if selfTriggeredMemberEvent(ctx, event.Authorizations, e) {
		return nil
	}

	user, err := data.SelectUserBySlackID(event.InnerEvent.User)
	if err != nil {
		log.Error(ctx, "failed to load user by Slack ID", "error", err, "user_id", e.User)
		return err
	}

	// If a user was added by someone else, check if the invitee is opted-in.
	if user.ThrippyLink != "" {
		return nil
	}

	// If not, send them an ephemeral message explaining how to opt-in.
	log.Warn(ctx, "user joined Slack channel but is not opted-in", "user_id", e.User, "channel_id", e.Channel)
	msg := ":wave: Hi <@%s>! You have joined a RevChat channel, but you have to opt-in for this to work! "
	_, err = PostMessage(ctx, e.User, fmt.Sprintf(msg, e.User)+"Please run this slash command:\n\n```/revchat opt-in```")
	return err
}

func (c *Config) memberLeftWorkflow(ctx workflow.Context, event memberEventWrapper) error {
	if selfTriggeredMemberEvent(ctx, event.Authorizations, event.InnerEvent) {
		return nil
	}

	log.Warn(ctx, "member left Slack channel - event handler not implemented yet")
	return nil
}

func selfTriggeredMemberEvent(ctx workflow.Context, auth []eventAuth, event MemberEvent) bool {
	for _, a := range auth {
		if a.IsBot && (a.UserID == event.User || a.UserID == event.Inviter) {
			log.Debug(ctx, "ignoring self-triggered Slack event")
			return true
		}
	}
	return false
}

func selfTriggeredEvent(ctx workflow.Context, auth []eventAuth, userID string) bool {
	for _, a := range auth {
		if a.IsBot && a.UserID == userID {
			log.Debug(ctx, "ignoring self-triggered Slack event")
			return true
		}
	}
	return false
}
