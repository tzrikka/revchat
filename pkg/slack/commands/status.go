package commands

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack"
	"github.com/tzrikka/revchat/pkg/slack/activities"
)

// SelfStatus is similar to [StatusOfOthers] but lists all the PRs that require the calling user's attention,
// i.e. PRs where it's their turn to review or respond. The user must be opted-in to use this command.
func SelfStatus(ctx workflow.Context, event SlashCommandEvent, showDrafts bool, alertsChannel string) error {
	users := data.ListPRsPerSlackUser(ctx, true, true, true)
	for user, prs := range users {
		if strings.HasSuffix(user, data.SlackIDNotFound) {
			details := make([]any, 0, 2*len(prs)+2)
			details = append(details, "Email", strings.TrimSuffix(user, data.SlackIDNotFound))
			for i, prURL := range prs {
				details = append(details, fmt.Sprintf("PR URL %d", i+1), prURL)
			}
			activities.AlertWarn(ctx, alertsChannel, "Slack email lookup failed - removed email from turn(s)", details...)
		}
	}

	if len(users) == 0 {
		return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID,
			":joy: No PRs require your attention at this time!")
	}
	prs := users[event.UserID]
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
		prDetails := slack.PRDetails(ctx, url, singleUser, true, showDrafts, false, "")

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
func StatusOfOthers(ctx workflow.Context, event SlashCommandEvent, showDrafts bool, thrippyID, alertsChannel string) error {
	showDrafts = showDraftsOption(showDrafts, event.Text)
	showTasks := strings.Contains(event.Text, " tasks")

	users := extractAtLeastOneUserID(ctx, event)
	if len(users) == 0 {
		err := errors.New("failed to use user/group mentions from Slack command")
		return errors.Join(err, activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID,
			":warning: Failed to use user/group mentions in the Slack command."))
	}

	authors, reviewers := statusMode(event.Text)
	allUserPRs := data.ListPRsPerSlackUser(ctx, false, authors, reviewers)
	for user, prs := range allUserPRs {
		if strings.HasSuffix(user, data.SlackIDNotFound) {
			details := make([]any, 0, 2*len(prs)+2)
			details = append(details, "Email", strings.TrimSuffix(user, data.SlackIDNotFound))
			for i, prURL := range prs {
				details = append(details, fmt.Sprintf("PR URL %d", i+1), prURL)
			}
			activities.AlertWarn(ctx, alertsChannel, "Slack email lookup failed - removed email from turn(s)", details...)
		}
	}

	if len(allUserPRs) == 0 {
		return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID,
			":joy: No PRs require your attention at this time!")
	}

	var filteredPRs []string
	for _, userID := range users {
		filteredPRs = append(filteredPRs, allUserPRs[userID]...)
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
	fmt.Fprintf(list, " by <@%s>:", strings.Join(users, ">, <@"))
	header := list.String()

	for _, url := range filteredPRs {
		prDetails := slack.PRDetails(ctx, url, users, false, showDrafts, showTasks, thrippyID)

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
