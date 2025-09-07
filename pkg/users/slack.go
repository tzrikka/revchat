package users

import (
	"errors"

	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/revchat/pkg/data"
)

// EmailToSlackID looks up a Slack user based on an email address. This
// function assumes this mapping does not exist yet, so it saves it if found.
func EmailToSlackID(ctx workflow.Context, cmd *cli.Command, email string) string {
	user := slackLookupUserByEmailActivity(ctx, cmd, email)
	if user == nil {
		return ""
	}

	if err := data.AddSlackUser(user.ID, email); err != nil {
		log.Error(ctx, "failed to save Slack user ID/email mapping", "error", err, "user_id", user.ID, "email", email)
	}

	return user.ID
}

// SlackIDToEmail retrieves a Slack user's info based on their ID, and returns their email address.
// This function uses both caching and API calls. Not finding an email address is considered an error.
func SlackIDToEmail(ctx workflow.Context, cmd *cli.Command, userID string) (string, error) {
	email, err := data.SlackUserEmailByID(userID)
	if err != nil {
		log.Error(ctx, "failed to load Slack user email", "error", err, "user_id", userID)
		// Don't abort - try to use the Slack API as a fallback.
	}
	if email != "" {
		return email, nil
	}

	user, err := SlackUserInfoActivity(ctx, cmd, userID)
	if err != nil {
		return "", err
	}
	if user.Profile.Email != "" {
		return user.Profile.Email, nil
	}

	log.Error(ctx, "Slack user has no email address", "user_id", userID, "real_name", user.RealName)
	return "", errors.New("Slack user has no email address")
}

// https://docs.slack.dev/reference/methods/users.info
type slackUsersInfoRequest struct {
	User string `json:"user"`

	IncludeLocale bool `json:"include_locale,omitempty"`
}

// https://docs.slack.dev/reference/methods/users.info
type slackUsersInfoResponse struct {
	slackResponse

	User *SlackUser `json:"user,omitempty"`
}

// https://docs.slack.dev/reference/methods/users.lookupByEmail
type slackUsersLookupByEmailRequest struct {
	Email string `json:"email"`
}

// https://docs.slack.dev/reference/methods/users.lookupByEmail
type slackUsersLookupByEmailResponse struct {
	slackResponse

	User *SlackUser `json:"user,omitempty"`
}

// https://docs.slack.dev/reference/methods/users.profile.get
type usersProfileGetRequest struct {
	User string `json:"user"`

	IncludeLabels bool `json:"include_labels,omitempty"`
}

// https://docs.slack.dev/reference/methods/users.profile.get
type usersProfileGetResponse struct {
	slackResponse

	Profile *SlackProfile `json:"profile,omitempty"`
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

// https://docs.slack.dev/reference/objects/user-object/
type SlackUser struct {
	ID       string `json:"id"`
	TeamID   string `json:"team_id"`
	RealName string `json:"real_name"`
	IsBot    bool   `json:"is_bot"`

	TZ       string `json:"tz"`
	TZLabel  string `json:"tz_label"`
	TZOffset int    `json:"tz_offset"`

	Updated int `json:"updated"`

	Profile SlackProfile `json:"profile"`
}

// https://docs.slack.dev/reference/objects/user-object/#profile
type SlackProfile struct {
	DisplayName           string `json:"display_name"`
	DisplayNameNormalized string `json:"display_name_normalized"`

	FirstName          string `json:"first_name"`
	LastName           string `json:"last_name"`
	RealName           string `json:"real_name"`
	RealNameNormalized string `json:"real_name_normalized"`

	Email string `json:"email"`
	Team  string `json:"team"`

	Image24  string `json:"image_24"`
	Image32  string `json:"image_32"`
	Image48  string `json:"image_48"`
	Image72  string `json:"image_72"`
	Image192 string `json:"image_192"`
	Image512 string `json:"image_512"`

	APIAppID     string `json:"api_app_id,omitempty"`
	BotID        string `json:"bot_id,omitempty"`
	AlwaysActive bool   `json:"always_active,omitempty"`

	// https://docs.slack.dev/reference/methods/users.profile.set#custom_profile
}

// https://docs.slack.dev/reference/methods/users.lookupByEmail
func slackLookupUserByEmailActivity(ctx workflow.Context, cmd *cli.Command, email string) *SlackUser {
	req := slackUsersLookupByEmailRequest{Email: email}
	a := executeTimpaniActivity(ctx, cmd, "slack.users.lookupByEmail", req)

	resp := slackUsersLookupByEmailResponse{}
	if err := a.Get(ctx, &resp); err != nil {
		log.Error(ctx, "failed to lookup Slack user by email", "error", err, "email", email)
		return nil
	}

	return resp.User
}

// https://docs.slack.dev/reference/methods/users.info
func SlackUserInfoActivity(ctx workflow.Context, cmd *cli.Command, userID string) (*SlackUser, error) {
	a := executeTimpaniActivity(ctx, cmd, "slack.users.info", slackUsersInfoRequest{User: userID})

	resp := &slackUsersInfoResponse{}
	if err := a.Get(ctx, resp); err != nil {
		log.Error(ctx, "failed to retrieve Slack user info", "error", err, "user_id", userID)
		return nil, err
	}

	return resp.User, nil
}

// https://docs.slack.dev/reference/methods/users.profile.get
func SlackUserProfileActivity(ctx workflow.Context, cmd *cli.Command, userID string) (*SlackProfile, error) {
	a := executeTimpaniActivity(ctx, cmd, "slack.users.profile.get", usersProfileGetRequest{User: userID})

	resp := &usersProfileGetResponse{}
	if err := a.Get(ctx, resp); err != nil {
		log.Error(ctx, "failed to retrieve Slack user profile", "error", err, "user_id", userID)
		return nil, err
	}

	return resp.Profile, nil
}
