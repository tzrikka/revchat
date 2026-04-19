package commands

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack"
	"github.com/tzrikka/revchat/pkg/slack/activities"
)

// SelfStatus is similar to [StatusOfOthers] but lists all the PRs that require the calling user's attention,
// i.e. PRs where it's their turn to review or respond. The user must be opted-in to use this command.
func SelfStatus(ctx workflow.Context, opts client.Options, event SlashCommandEvent, alertsChannel string, showDrafts bool) error {
	userPRs, userAlerts := data.ListPRsPerSlackUser(ctx, opts, true, true, true, []string{event.UserID})
	for _, details := range userAlerts {
		activities.AlertWarn(ctx, alertsChannel, "Slack email lookup failed - removed email from turn(s)", details...)
	}
	if len(userPRs) == 0 {
		return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID,
			":joy: No PRs require your attention at this time!")
	}
	prs := userPRs[event.UserID]
	if len(prs) == 0 {
		return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID,
			":joy: No PRs require your attention at this time!")
	}

	var list strings.Builder
	list.WriteString(":eyes: These PRs currently require your attention:")
	header := list.String()

	slices.Sort(prs)
	singleUser := []string{event.UserID}

	for _, url := range prs {
		prDetails := slack.PRDetails(ctx, opts, url, singleUser, true, showDrafts, false, "")

		// If the message becomes too long, split it into multiple chunks,
		// even if the Slack API could technically handle a bit more.
		if list.Len()+len(prDetails) > 3900 {
			if err := activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, list.String()); err != nil {
				return err
			}
			list.Reset()
		}

		list.WriteString(prDetails)
	}

	msg := list.String()
	if msg == header {
		msg = "\n:joy: No PRs require your attention at this time!"
	}

	return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}

// StatusOfOthers is similar to [SelfStatus] but lists all the PRs associated with the given users
// and/or groups. This is the only command that doesn't require the calling user to be opted-in.
func StatusOfOthers(ctx workflow.Context, opts client.Options, event SlashCommandEvent, showDrafts bool, thrippyID, alertsChannel string) error {
	// Sanity check: Slack can send the app an event from a channel that the app can't access.
	if _, err := activities.ChannelInfo(ctx, event.ChannelID, false, false); err != nil {
		_ = activities.AlertError(ctx, alertsChannel, "received status command from inaccessible channel", err,
			"Channel", fmt.Sprintf("`%s` (<#%s>)", event.ChannelID, event.ChannelID), "Initiator", fmt.Sprintf("<@%s>", event.UserID))
		msg := fmt.Sprintf(":warning: Received a `status` command from you in an inaccessible channel: <#%s>.", event.ChannelID)
		msg += "\n\nPlease add this app to that channel, or run your command here."
		return activities.PostMessage(ctx, event.UserID, msg)
	}

	showDrafts = showDraftsOption(showDrafts, event.Text)
	showTasks := strings.Contains(event.Text, " tasks")

	users := extractAtLeastOneUserID(ctx, event)
	if len(users) == 0 {
		err := errors.New("failed to use user/group mentions from Slack command")
		return errors.Join(err, activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID,
			":warning: Failed to use user/group mentions in the Slack command."))
	}

	authors, reviewers := statusMode(event.Text)
	userPRs, userAlerts := data.ListPRsPerSlackUser(ctx, opts, false, authors, reviewers, users)
	for _, details := range userAlerts {
		activities.AlertWarn(ctx, alertsChannel, "Slack email lookup failed - removed email from turn(s)", details...)
	}
	if len(userPRs) == 0 {
		return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID,
			":joy: No PRs require your attention at this time!")
	}

	var filteredPRs []string
	for _, userID := range users {
		filteredPRs = append(filteredPRs, userPRs[userID]...)
	}
	if len(filteredPRs) == 0 {
		return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID,
			":joy: No PRs require your attention at this time!")
	}
	slices.Sort(filteredPRs)
	filteredPRs = slices.Compact(filteredPRs)

	list := new(strings.Builder)
	switch {
	case authors && reviewers:
		list.WriteString("These PRs were created / need to be reviewed")
	case authors:
		list.WriteString("These PRs were created")
	case reviewers:
		list.WriteString("These PRs need to be reviewed")
	}
	fmt.Fprintf(list, " by <@%s>:", strings.Join(users, ">, <@")) //workflowcheck:ignore // Deterministic output, not a file.
	header := list.String()

	for _, url := range filteredPRs {
		prDetails := slack.PRDetails(ctx, opts, url, users, false, showDrafts, showTasks, thrippyID)

		// If the message becomes too long, split it into multiple chunks, even if the Slack API
		// could technically handle a bit more. Why not 4000? To leave a buffer for encoding.
		if list.Len()+len(prDetails) > 3800 {
			if err := activities.PostMessage(ctx, event.ChannelID, list.String()); err != nil {
				return err
			}
			list.Reset()
		}

		list.WriteString(prDetails)
	}

	msg := list.String()
	if msg == header {
		return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID,
			":joy: No PRs require your attention at this time!")
	}

	return activities.PostMessage(ctx, event.ChannelID, msg)
}

func showDraftsOption(defaultValue bool, text string) bool {
	if !defaultValue && strings.Contains(text, " draft") {
		defaultValue = true
	}

	if defaultValue && strings.Contains(text, " no draft") {
		defaultValue = false
	}

	return defaultValue
}

func statusMode(text string) (authors, reviewers bool) {
	authors = strings.Contains(text, " author")
	reviewers = strings.Contains(text, " reviewer")

	if !authors && !reviewers {
		authors, reviewers = true, true
	}

	return authors, reviewers
}
