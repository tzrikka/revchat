package commands

import (
	"fmt"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/slack/activities"
)

func Invite(ctx workflow.Context, event SlashCommandEvent) error {
	users := extractAtLeastOneUserID(ctx, event)
	if len(users) == 0 {
		return nil
	}

	done := make([]string, 0, len(users))
	for _, userID := range users {
		_, optedIn, err := UserDetails(ctx, event, userID)
		if err != nil {
			continue
		}
		if optedIn {
			msg := fmt.Sprintf(":bell: <@%s> is already opted-in.", userID)
			_ = activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
			continue
		}

		msg := ":wave: <@%s> is inviting you to use RevChat. Please run this slash command:\n\n```%s opt-in```"
		if err := activities.PostMessage(ctx, userID, fmt.Sprintf(msg, event.UserID, event.Command)); err != nil {
			PostEphemeralError(ctx, event, fmt.Sprintf("failed to send an invite to <@%s>.", userID))
			continue
		}

		done = append(done, userID)
	}

	if len(done) == 0 {
		return nil
	}

	msg := "Sent invite"
	if len(done) > 1 {
		msg += "s"
	}
	msg = fmt.Sprintf("%s to: <@%s>.", msg, strings.Join(done, ">, <@"))
	return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}
