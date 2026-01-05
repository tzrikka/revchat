package github

import (
	"slices"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/users"
)

// ChannelMembers returns a list of opted-in Slack user IDs who are PR authors/reviewers/followers.
// The output is guaranteed to be sorted, without repetitions, and not contain teams/apps.
func ChannelMembers(ctx workflow.Context, pr PullRequest) []string {
	us := []User{pr.User}

	if !pr.Draft {
		us = append(append(us, pr.RequestedReviewers...), pr.Assignees...)
	}

	slackIDs := loginsToSlackIDs(ctx, userLogins(us))
	slackIDs = append(slackIDs, data.SelectUserByGitHubID(ctx, pr.User.Login).Followers...)

	slices.Sort(slackIDs)
	return slices.Compact(slackIDs)
}

func loginsToSlackIDs(ctx workflow.Context, logins []string) []string {
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
