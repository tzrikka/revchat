package slack

import (
	"fmt"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/revchat/pkg/data"
)

type memberEventWrapper struct {
	eventWrapper

	InnerEvent MemberEvent `json:"event"`
}

// https://docs.slack.dev/apis/events-api/#events-JSON
type eventWrapper struct {
	APIAppID            string  `json:"api_app_id"`
	TeamID              string  `json:"team_id"`
	ContextTeamID       string  `json:"context_team_id"`
	ContextEnterpriseID *string `json:"context_enterprise_id,omitempty"`

	// Type string `json:"type"` // Always "event_callback".

	EventContext       string `json:"event_context"`
	EventID            string `json:"event_id"`
	EventTime          int    `json:"event_time"`
	IsExtSharedChannel bool   `json:"is_ext_shared_channel"`

	Authorizations []eventAuth `json:"authorizations"`
}

// https://docs.slack.dev/apis/events-api/#authorizations
type eventAuth struct {
	EnterpriseID        *string `json:"enterprise_id,omitempty"`
	TeamID              string  `json:"team_id"`
	UserID              string  `json:"user_id"`
	IsBot               bool    `json:"is_bot"`
	IsEnterpriseInstall bool    `json:"is_enterprise_install"`
}

// https://docs.slack.dev/reference/events/member_joined_channel/
// https://docs.slack.dev/reference/events/member_left_channel/
type MemberEvent struct {
	// Type string `json:"type"`

	Enterprise  string `json:"enterprise,omitempty"`
	Team        string `json:"team"`
	Channel     string `json:"channel"`
	ChannelType string `json:"channel_type"`
	User        string `json:"user"`

	Inviter string `json:"inviter,omitempty"`
}

func (c *Config) memberJoinedWorkflow(ctx workflow.Context, event memberEventWrapper) error {
	e := event.InnerEvent
	if isSelfTriggeredMemberEvent(ctx, event.Authorizations, e) {
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
	if isSelfTriggeredMemberEvent(ctx, event.Authorizations, event.InnerEvent) {
		return nil
	}

	log.Warn(ctx, "member left Slack channel - event handler not implemented yet")
	return nil
}

func isSelfTriggeredMemberEvent(ctx workflow.Context, auth []eventAuth, event MemberEvent) bool {
	for _, a := range auth {
		if a.IsBot && (a.UserID == event.User || a.UserID == event.Inviter) {
			log.Debug(ctx, "ignoring self-triggered Slack event")
			return true
		}
	}
	return false
}

func isSelfTriggeredEvent(ctx workflow.Context, auth []eventAuth, userID string) bool {
	for _, a := range auth {
		if a.IsBot && a.UserID == userID {
			log.Debug(ctx, "ignoring self-triggered Slack event")
			return true
		}
	}
	return false
}
