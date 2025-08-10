package users

// https://docs.slack.dev/reference/methods/users.lookupByEmail
type slackUsersLookupByEmailRequest struct {
	Email string `json:"email"`
}

// https://docs.slack.dev/reference/methods/users.lookupByEmail
type slackUsersLookupByEmailResponse struct {
	slackResponse

	User map[string]any `json:"user,omitempty"`
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
