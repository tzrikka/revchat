package slack

import (
	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
)

// https://docs.slack.dev/reference/methods/bots.info
type botsInfoRequest struct {
	Bot string `json:"bot"`

	TeamID string `json:"team_id,omitempty"`
}

// https://docs.slack.dev/reference/methods/bots.info
type botsInfoResponse struct {
	slackResponse

	Bot *BotInfo `json:"bot,omitempty"`
}

// https://docs.slack.dev/reference/methods/bots.info
type BotInfo struct {
	ID      string `json:"id"`
	TeamID  string `json:"team_id"`
	Name    string `json:"name"`
	AppID   string `json:"app_id"`
	UserID  string `json:"user_id"`
	Deleted bool   `json:"deleted"`
	Updated int    `json:"updated"`
}

// https://docs.slack.dev/reference/methods/bots.info
func BotsInfoActivity(ctx workflow.Context, cmd *cli.Command, botID, teamID string) (*BotInfo, error) {
	a := executeTimpaniActivity(ctx, cmd, "slack.bots.info", botsInfoRequest{Bot: botID, TeamID: teamID})

	resp := &botsInfoResponse{}
	if err := a.Get(ctx, resp); err != nil {
		log.Error(ctx, "failed to get Slack bot info", "error", err)
		return nil, err
	}

	return resp.Bot, nil
}
