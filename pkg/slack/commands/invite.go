package commands

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/slack/activities"
)

func Invite(ctx workflow.Context, event SlashCommandEvent) error {
	users := extractAtLeastOneUserID(ctx, event, userIDsPattern)
	if len(users) == 0 {
		return nil // Not a server error as far as we're concerned.
	}

	sent := make(map[string]bool, len(users))
	for _, userID := range users {
		// Avoid duplicate invitations, and check that the user isn't already opted-in.
		if sent[userID] {
			continue
		}

		_, optedIn, err := UserDetails(ctx, event, userID)
		if optedIn {
			msg := fmt.Sprintf(":bell: <@%s> is already opted-in.", userID)
			_ = activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
		}
		if err != nil || optedIn {
			continue
		}

		msg := ":wave: <@%s> is inviting you to use RevChat. Please run this slash command:\n\n```%s opt-in```"
		if _, err := activities.PostMessage(ctx, userID, fmt.Sprintf(msg, event.UserID, event.Command)); err != nil {
			PostEphemeralError(ctx, event, fmt.Sprintf("failed to send an invite to <@%s>.", userID))
			continue
		}
		sent[userID] = true
	}

	if len(sent) == 0 {
		return nil
	}
	msg := "Sent invite"
	if len(sent) > 1 {
		msg += "s"
	}
	msg = fmt.Sprintf("%s to: <@%s>", msg, strings.Join(slices.Sorted(maps.Keys(sent)), ">, <@"))
	return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}
