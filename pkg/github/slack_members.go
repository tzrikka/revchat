package github

import (
	"slices"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/users"
)

// ChannelMembers returns a list of opted-in Slack user IDs who are PR authors/reviewers/followers.
// The output is guaranteed to be sorted, without repetitions, and not contain (unresolved) teams or bots.
func ChannelMembers(ctx workflow.Context, pr PullRequest) []string {
	us := []User{pr.User}

	if !pr.Draft {
		us = append(append(us, pr.RequestedReviewers...), pr.Assignees...)
	}

	slackIDs := LoginsToSlackIDs(ctx, userLogins(us))

	slices.Sort(slackIDs)
	return slices.Compact(slackIDs)
}

// ReviewerMentions returns a Slack message mentioning 1 or more added or removed reviewers.
func ReviewerMentions(ctx workflow.Context, action, role string, reviewers []User) string {
	var msg strings.Builder
	msg.WriteString(":bust_in_silhouette: %s ")

	msg.WriteString(action)
	switch len(reviewers) {
	case 1:
		msg.WriteString(" this ")
	default:
		msg.WriteString(" these ")
	}

	msg.WriteString(role)
	if len(reviewers) > 1 {
		msg.WriteString("s")
	}

	msg.WriteString(":")
	for _, user := range reviewers {
		if mention := users.GitHubIDToSlackRef(ctx, user.Login, user.HTMLURL, user.Type); mention != "" {
			msg.WriteString(" " + mention)
		}
	}

	msg.WriteString(".")
	return msg.String()
}

func LoginsToSlackIDs(ctx workflow.Context, logins []string) []string {
	slackIDs := make([]string, 0, len(logins))
	for _, githubID := range logins {
		// True = don't include opted-out users. They will still be mentioned
		// in the channel, but as non-members they won't be notified about it.
		if slackID := users.GitHubIDToSlackID(ctx, githubID, true); slackID != "" {
			slackIDs = append(slackIDs, slackID)
		}
	}

	slices.Sort(slackIDs)
	return slices.Compact(slackIDs)
}
