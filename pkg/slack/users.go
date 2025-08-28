package slack

import (
	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
)

// https://docs.slack.dev/reference/methods/users.info
type usersInfoRequest struct {
	User string `json:"user"`

	IncludeLocale bool `json:"include_locale,omitempty"`
}

// https://docs.slack.dev/reference/methods/users.info
type usersInfoResponse struct {
	slackResponse

	User *User `json:"user,omitempty"`
}

// https://docs.slack.dev/reference/objects/user-object/.
// A slimmer version can be found in "pkg/users/slack.go".
type User struct {
	ID       string `json:"id"`
	TeamID   string `json:"team_id"`
	RealName string `json:"real_name"`
	IsBot    bool   `json:"is_bot"`

	TZ       string `json:"tz"`
	TZLabel  string `json:"tz_label"`
	TZOffset int    `json:"tz_offset"`

	Updated int `json:"updated"`

	Profile Profile `json:"profile"`
}

// https://docs.slack.dev/reference/methods/users.profile.get
type usersProfileGetRequest struct {
	User string `json:"user"`

	IncludeLabels bool `json:"include_labels,omitempty"`
}

// https://docs.slack.dev/reference/methods/users.profile.get
type usersProfileGetResponse struct {
	slackResponse

	Profile *Profile `json:"profile,omitempty"`
}

// https://docs.slack.dev/reference/objects/user-object/#profile
type Profile struct {
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

// https://docs.slack.dev/reference/methods/users.info
func UserInfoActivity(ctx workflow.Context, cmd *cli.Command, userID string) (*User, error) {
	a := executeTimpaniActivity(ctx, cmd, "slack.users.info", usersInfoRequest{User: userID})

	resp := &usersInfoResponse{}
	if err := a.Get(ctx, resp); err != nil {
		log.Error(ctx, "failed to retrieve Slack user info", "error", err, "user_id", userID)
		return nil, err
	}

	return resp.User, nil
}

// https://docs.slack.dev/reference/methods/users.profile.get
func UserProfileActivity(ctx workflow.Context, cmd *cli.Command, userID string) (*Profile, error) {
	a := executeTimpaniActivity(ctx, cmd, "slack.users.profile.get", usersProfileGetRequest{User: userID})

	resp := &usersProfileGetResponse{}
	if err := a.Get(ctx, resp); err != nil {
		log.Error(ctx, "failed to retrieve Slack user profile", "error", err, "user_id", userID)
		return nil, err
	}

	return resp.Profile, nil
}
