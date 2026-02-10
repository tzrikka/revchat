package commands

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/slack"
	"github.com/tzrikka/revchat/pkg/slack/activities"
)

// SelfStatus is similar to [StatusOfOthers] but lists all the PRs that require the calling user's attention,
// i.e. PRs where it's their turn to review or respond. The user must be opted-in to use this command.
func SelfStatus(ctx workflow.Context, event SlashCommandEvent, reportDrafts bool) error {
	prs := slack.LoadPRTurns(ctx, true, true, true)[event.UserID]
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
		prDetails := slack.PRDetails(ctx, url, singleUser, true, reportDrafts)

		// If the message becomes too long, split it into multiple chunks,
		// even if the Slack API could technically handle a bit more.
		if list.Len()+len(prDetails) > 4000 {
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
func StatusOfOthers(ctx workflow.Context, event SlashCommandEvent, reportDrafts bool) error {
	users := extractAtLeastOneUserID(ctx, event)
	if len(users) == 0 {
		err := errors.New("failed to use user/group mentions from Slack command")
		return errors.Join(err, activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID,
			":warning: Failed to use user/group mentions in the Slack command."))
	}

	authors, reviewers := statusMode(event.Text)
	allPRs := slack.LoadPRTurns(ctx, false, authors, reviewers)
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
		prDetails := slack.PRDetails(ctx, url, users, false, reportDrafts)

		// If the message becomes too long, split it into multiple chunks,
		// even if the Slack API could technically handle a bit more.
		if list.Len()+len(prDetails) > 4000 {
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

var statusPattern = regexp.MustCompile(`^stat(e|us)?([\s-](auth|rev))?`)

func statusMode(text string) (authors, reviewers bool) {
	match := statusPattern.FindStringSubmatch(text)
	// Can't panic because [StatusOfOthers] is registered with a more restrictive pattern.
	switch match[3] {
	case "auth":
		return true, false
	case "rev":
		return false, true
	default:
		return true, true
	}
}
