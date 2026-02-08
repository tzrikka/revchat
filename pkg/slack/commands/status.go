package commands

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/slack"
	"github.com/tzrikka/revchat/pkg/slack/activities"
)

func SelfStatus(ctx workflow.Context, event SlashCommandEvent) error {
	prs := slack.LoadPRTurns(ctx, true)[event.UserID]
	if len(prs) == 0 {
		return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID,
			":joy: No PRs require your attention at this time!")
	}

	var msg strings.Builder
	msg.WriteString(":eyes: These PRs currently require your attention:")

	slices.Sort(prs)
	for _, url := range prs {
		msg.WriteString(slack.PRDetails(ctx, url, []string{event.UserID}))
	}

	return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg.String())
}

// StatusOfOthers is similar to [SelfStatus] but lists all the PRs associated with the given users
// and/or groups. This is the only command that doesn't require the calling user to be opted-in.
func StatusOfOthers(ctx workflow.Context, event SlashCommandEvent) error {
	users := extractAtLeastOneUserID(ctx, event)
	if len(users) == 0 {
		err := errors.New("failed to use user/group mentions from Slack command")
		return errors.Join(err, activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID,
			":warning: Failed to use user/group mentions in the Slack command."))
	}

	allPRs := slack.LoadPRTurns(ctx, false)
	if len(allPRs) == 0 {
		return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID,
			":joy: No PRs require your attention at this time!")
	}

	var filteredPRs []string
	for _, userID := range users {
		filteredPRs = append(filteredPRs, allPRs[userID]...)
	}
	if len(filteredPRs) == 0 {
		return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID,
			":joy: No PRs require your attention at this time!")
	}
	slices.Sort(filteredPRs)
	filteredPRs = slices.Compact(filteredPRs)

	msg := new(strings.Builder)
	fmt.Fprintf(msg, "These open PRs were/are created/reviewed by <@%s>:", strings.Join(users, ">, <@"))

	for _, url := range filteredPRs {
		msg.WriteString(slack.PRDetails(ctx, url, users))
	}

	return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg.String())
}
