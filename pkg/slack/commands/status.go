package commands

import (
	"slices"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/slack"
	"github.com/tzrikka/revchat/pkg/slack/activities"
)

func Status(ctx workflow.Context, event SlashCommandEvent) error {
	prs := slack.LoadPRTurns(ctx)[event.UserID]
	if len(prs) == 0 {
		return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID,
			":joy: No PRs require your attention at this time!")
	}

	var msg strings.Builder
	msg.WriteString(":eyes: These PRs currently require your attention:")

	slices.Sort(prs)
	for _, url := range prs {
		msg.WriteString(slack.PRDetails(ctx, url, event.UserID))
	}

	return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg.String())
}
