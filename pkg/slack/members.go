package slack

import (
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
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
	ContextEnterpriseID *string `json:"context_enterprise_id"`

	// Type string `json:"type"` // Always "event_callback".

	EventContext       string `json:"event_context"`
	EventID            string `json:"event_id"`
	EventTime          int    `json:"event_time"`
	IsExtSharedChannel bool   `json:"is_ext_shared_channel"`

	Authorizations []eventAuth `json:"authorizations"`
}

// https://docs.slack.dev/apis/events-api/#authorizations
type eventAuth struct {
	EnterpriseID        *string `json:"enterprise_id"`
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

func (c Config) memberJoinedWorkflow(ctx workflow.Context, event memberEventWrapper) error {
	if isSelfTriggeredEvent(ctx, event.Authorizations, event.InnerEvent.User) {
		return nil
	}

	log.Warn(ctx, "member joined event", "event", event)
	return nil
}

func (c Config) memberLeftWorkflow(ctx workflow.Context, event memberEventWrapper) error {
	if isSelfTriggeredEvent(ctx, event.Authorizations, event.InnerEvent.User) {
		return nil
	}

	log.Warn(ctx, "member left event", "event", event)
	return nil
}

func isSelfTriggeredEvent(ctx workflow.Context, as []eventAuth, user string) bool {
	for _, a := range as {
		if a.IsBot && a.UserID == user {
			log.Debug(ctx, "ignoring self-triggered Slack event")
			return true
		}
	}

	return false
}
