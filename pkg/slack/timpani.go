package slack

const (
	ChatPostMessageActivity         = "slack.chat.postMessage"
	ConversationsArchiveActivity    = "slack.conversations.archive"
	ConversationsCreateActivity     = "slack.conversations.create"
	ConversationsInviteActivity     = "slack.conversations.invite"
	ConversationsSetPurposeActivity = "slack.conversations.setPurpose"
	ConversationsSetTopicActivity   = "slack.conversations.setTopic"
	ConversationsUnarchiveActivity  = "slack.conversations.unarchive"
)

// https://docs.slack.dev/reference/methods/chat.postMessage
type ChatPostMessageRequest struct {
	Channel string `json:"channel"`

	Attachments  []map[string]any `json:"attachments,omitempty"`
	Blocks       []map[string]any `json:"blocks,omitempty"`
	IconEmoji    string           `json:"icon_emoji,omitempty"`
	IconURL      string           `json:"icon_url,omitempty"`
	LinkNames    bool             `json:"link_names,omitempty"`
	MarkdownText string           `json:"markdown_text,omitempty"`
	Metadata     map[string]any   `json:"metadata,omitempty"`
	// Ignoring "mrkdwn" for now, because it has an unusual default value (true).
	Parse          string `json:"parse,omitempty"`
	ReplyBroadcast bool   `json:"reply_broadcast,omitempty"`
	Text           string `json:"text,omitempty"`
	ThreadTS       string `json:"thread_ts,omitempty"`
	UnfurnLinks    bool   `json:"unfurl_links,omitempty"`
	Username       string `json:"username,omitempty"`
}

// https://docs.slack.dev/reference/methods/chat.postMessage
type ChatPostMessageResponse struct {
	slackResponse

	Channel string         `json:"channel,omitempty"`
	TS      string         `json:"ts,omitempty"`
	Message map[string]any `json:"message,omitempty"`
}

// https://docs.slack.dev/reference/methods/conversations.archive
type ConversationsArchiveRequest struct {
	Channel string `json:"channel"`
}

// https://docs.slack.dev/reference/methods/conversations.archive
type ConversationsArchiveResponse struct {
	slackResponse
}

// https://docs.slack.dev/reference/methods/conversations.create
type ConversationsCreateRequest struct {
	Name string `json:"name"`

	IsPrivate bool   `json:"is_private,omitempty"`
	TeamID    string `json:"team_id,omitempty"`
}

// https://docs.slack.dev/reference/methods/conversations.create
type ConversationsCreateResponse struct {
	slackResponse

	Channel map[string]any `json:"channel,omitempty"`
}

// https://docs.slack.dev/reference/methods/conversations.invite
type ConversationsInviteRequest struct {
	Channel string `json:"channel"`
	Users   string `json:"users"`

	Force bool `json:"force,omitempty"`
}

// https://docs.slack.dev/reference/methods/conversations.invite
type ConversationsInviteResponse struct {
	slackResponse

	Channel map[string]any   `json:"channel,omitempty"`
	Errors  []map[string]any `json:"errors,omitempty"`
}

// https://docs.slack.dev/reference/methods/conversations.setPurpose
type ConversationsSetPurposeRequest struct {
	Channel string `json:"channel"`
	Purpose string `json:"purpose"`
}

// https://docs.slack.dev/reference/methods/conversations.setTopic
type ConversationsSetTopicRequest struct {
	Channel string `json:"channel"`
	Topic   string `json:"topic"`
}

// https://docs.slack.dev/reference/methods/conversations.unarchive
type ConversationsUnarchiveRequest struct {
	Channel string `json:"channel"`
}

// https://docs.slack.dev/reference/methods/conversations.unarchive
type ConversationsUnarchiveResponse struct {
	slackResponse
}

type slackResponse struct {
	OK               bool              `json:"ok"`
	Error            string            `json:"error,omitempty"`
	Needed           string            `json:"needed,omitempty"`   // Scope errors (undocumented).
	Provided         string            `json:"provided,omitempty"` // Scope errors (undocumented).
	Warning          string            `json:"warning,omitempty"`
	ResponseMetadata *responseMetadata `json:"response_metadata,omitempty"`
}

type responseMetadata struct {
	Messages   []string `json:"messages,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
	NextCursor string   `json:"next_cursor,omitempty"`
}
