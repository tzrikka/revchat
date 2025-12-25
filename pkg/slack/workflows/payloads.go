package workflows

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
	// https://docs.slack.dev/reference/events/message/#hidden_subtypes
	// Hidden bool `json:"hidden,omitempty"`

	User     string `json:"user,omitempty"`
	BotID    string `json:"bot_id,omitempty"`
	Username string `json:"username,omitempty"` // Customized display name, when bot_id is present.
	// ParentUserID string `json:"parent_user_id,omitempty"` // Unnecessary.

	Team       string `json:"team,omitempty"`
	SourceTeam string `json:"source_team,omitempty"`
	UserTeam   string `json:"user_team,omitempty"`

	Channel     string `json:"channel,omitempty"`
	ChannelType string `json:"channel_type,omitempty"`

	Text   string           `json:"text,omitempty"`
	Blocks []map[string]any `json:"blocks,omitempty"`

	Edited          *Edited       `json:"edited,omitempty"`           // Subtype = "message_changed".
	Message         *MessageEvent `json:"message,omitempty"`          // Subtype = "message_changed".
	PreviousMessage *MessageEvent `json:"previous_message,omitempty"` // Subtype = "message_changed" or "message_deleted".
	Root            *MessageEvent `json:"root,omitempty"`             // Subtype = "thread_broadcast".

	TS        string `json:"ts"`
	EventTS   string `json:"event_ts,omitempty"`
	DeletedTS string `json:"deleted_ts,omitempty"` // Subtype = "message_deleted".
	ThreadTS  string `json:"thread_ts,omitempty"`  // Reply, or subtype = "thread_broadcast".

	LatestReply     string   `json:"latest_reply,omitempty"`
	ReplyCount      int      `json:"reply_count,omitempty"`
	ReplyUsers      []string `json:"reply_users,omitempty"`
	ReplyUsersCount int      `json:"reply_users_count,omitempty"`

	// LastRead           string `json:"last_read,omitempty"`
	// UnreadCount        int    `json:"unread_count,omitempty"`
	// UnreadCountDisplay int    `json:"unread_count_display,omitempty"`

	// https://docs.slack.dev/reference/events/message/file_share
	Files        []File `json:"files,omitempty"`
	Upload       bool   `json:"upload,omitempty"`
	DisplayAsBot bool   `json:"display_as_bot,omitempty"`

	// https://docs.slack.dev/reference/events/message/#stars
	IsStarred bool       `json:"is_starred,omitempty"`
	PinnedTo  []string   `json:"pinned_to,omitempty"`
	Reactions []Reaction `json:"reactions,omitempty"`

	// IsLocked   bool `json:"is_locked,omitempty"`
	// Subscribed bool `json:"subscribed,omitempty"`

	// ClientMsgID  string `json:"client_msg_id,omitempty"` // Unnecessary.
}

type Edited struct {
	User string `json:"user"`
	TS   string `json:"ts"`
}

// https://docs.slack.dev/reference/events/message/file_share
// https://docs.slack.dev/reference/objects/file-object
type File struct {
	ID string `json:"id"`

	// Created   int    `json:"created"`
	// Updated   int    `json:"updated"`
	// User      string `json:"user"`
	// UserTeam  string `json:"user_team"`

	Name       string `json:"name"`
	Title      string `json:"title"`
	MimeType   string `json:"mimetype"`
	FileType   string `json:"filetype"` // https://docs.slack.dev/reference/objects/file-object#types
	PrettyType string `json:"pretty_type"`

	Size      int `json:"size"`
	OriginalW int `json:"original_w"`
	OriginalH int `json:"original_h"`

	Mode             string `json:"mode"` // One of: "hosted", "external", "snippet", or "post".
	Editable         bool   `json:"editable"`
	IsExternal       bool   `json:"is_external"`
	IsPublic         bool   `json:"is_public"`
	PublicURLShared  bool   `json:"public_url_shared"`
	ExternalType     string `json:"external_type"`
	MediaDisplayType string `json:"media_display_type"`

	Username     string `json:"username"`
	DisplayAsBot bool   `json:"display_as_bot"`

	URLPrivate         string `json:"url_private"`
	URLPrivateDownload string `json:"url_private_download"`
	Permalink          string `json:"permalink"`
	PermalinkPublic    string `json:"permalink_public"`
	EditLink           string `json:"edit_link,omitempty"`

	Thumb64     string `json:"thumb_64,omitempty"`
	Thumb80     string `json:"thumb_80,omitempty"`
	Thumb160    string `json:"thumb_160,omitempty"`
	Thumb360    string `json:"thumb_360,omitempty"`
	Thumb360Gif string `json:"thumb_360_gif,omitempty"`
	Thumb480    string `json:"thumb_480,omitempty"`
	Thumb720    string `json:"thumb_720,omitempty"`
	Thumb800    string `json:"thumb_800,omitempty"`
	Thumb960    string `json:"thumb_960,omitempty"`
	Thumb1024   string `json:"thumb_1024,omitempty"`
	ThumbTiny   string `json:"thumb_tiny,omitempty"`

	Preview            string `json:"preview,omitempty"`
	PreviewHighlight   string `json:"preview_highlight,omitempty"`
	PreviewIsTruncated bool   `json:"preview_is_truncated,omitempty"`
	Lines              int    `json:"lines,omitempty"`
	LinesMore          int    `json:"lines_more,omitempty"`

	SkippedShares  bool   `json:"skipped_shares,omitempty"`
	HasRichPreview bool   `json:"has_rich_preview,omitempty"`
	FileAccess     string `json:"file_access,omitempty"`
}

// https://docs.slack.dev/reference/events/message/#stars
type Reaction struct {
	Name  string   `json:"name,omitempty"`
	Users []string `json:"users,omitempty"`
	Count int      `json:"count,omitempty"`
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
