package slack

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

type archiveEventWrapper struct {
	eventWrapper

	InnerEvent ArchiveEvent `json:"event"`
}

// https://docs.slack.dev/reference/events/channel_archive/
// https://docs.slack.dev/reference/events/channel_unarchive/
// https://docs.slack.dev/reference/events/group_archive/
// https://docs.slack.dev/reference/events/group_unarchive/
type ArchiveEvent struct {
	// Type string `json:"type"`

	Channel string `json:"channel"`
	User    string `json:"user"`

	IsMoved int    `json:"is_moved,omitempty"`
	EventTS string `json:"event_ts"`
}

type memberEventWrapper struct {
	eventWrapper

	InnerEvent MemberEvent `json:"event"`
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

type messageEventWrapper struct {
	eventWrapper

	InnerEvent MessageEvent `json:"event"`
}

// https://docs.slack.dev/reference/events/message/
type MessageEvent struct {
	// Type string `json:"type"` // Always "message".

	Subtype string `json:"subtype,omitempty"`

	User     string `json:"user,omitempty"`
	BotID    string `json:"bot_id,omitempty"`
	Username string `json:"username,omitempty"` // Customized display name, when bot_id is present.

	Team        string `json:"team,omitempty"`
	Channel     string `json:"channel,omitempty"`
	ChannelType string `json:"channel_type,omitempty"`

	Text string `json:"text,omitempty"`
	// Blocks []map[string]any `json:"blocks"` // Text is enough.

	Edited          *Edited       `json:"edited,omitempty"`           // Subtype = "message_changed".
	Message         *MessageEvent `json:"message,omitempty"`          // Subtype = "message_changed".
	PreviousMessage *MessageEvent `json:"previous_message,omitempty"` // Subtype = "message_changed" or "message_deleted".
	Root            *MessageEvent `json:"root,omitempty"`             // Subtype = "thread_broadcast".

	TS        string `json:"ts"`
	EventTS   string `json:"event_ts,omitempty"`
	DeletedTS string `json:"deleted_ts,omitempty"` // Subtype = "message_deleted".
	ThreadTS  string `json:"thread_ts,omitempty"`  // Reply, or subtype = "thread_broadcast".

	ParentUserID string `json:"parent_user_id,omitempty"` // Subtype = "thread_broadcast".
	ClientMsgID  string `json:"client_msg_id,omitempty"`
}

type Edited struct {
	User string `json:"user"`
	TS   string `json:"ts"`
}

type reactionEventWrapper struct {
	eventWrapper

	InnerEvent ReactionEvent `json:"event"`
}

// https://docs.slack.dev/reference/events/reaction_added/
// https://docs.slack.dev/reference/events/reaction_removed/
type ReactionEvent struct {
	// Type string `json:"type"`

	User     string `json:"user"`
	Reaction string `json:"reaction"`

	Item struct {
		Type        string `json:"type"`
		Channel     string `json:"channel,omitempty"`
		TS          string `json:"ts,omitempty"`
		File        string `json:"file,omitempty"`
		FileComment string `json:"file_comment,omitempty"`
	} `json:"item"`

	ItemUser string `json:"item_user,omitempty"`

	EventTS string `json:"event_ts"`
}

// https://docs.slack.dev/interactivity/implementing-slash-commands/#app_command_handling
// https://docs.slack.dev/apis/events-api/using-socket-mode#command
type SlashCommandEvent struct {
	APIAppID string `json:"api_app_id"`

	IsEnterpriseInstall string `json:"is_enterprise_install"`
	EnterpriseID        string `json:"enterprise_id,omitempty"`
	EnterpriseName      string `json:"enterprise_name,omitempty"`
	TeamID              string `json:"team_id"`
	TeamDomain          string `json:"team_domain"`

	ChannelID   string `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	UserID      string `json:"user_id"`
	UserName    string `json:"user_name"`

	Command string `json:"command"`
	Text    string `json:"text"`

	ResponseURL string `json:"response_url"`
	TriggerID   string `json:"trigger_id"`
}
