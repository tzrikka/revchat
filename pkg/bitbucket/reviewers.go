package bitbucket

import (
	"slices"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/users"
)

// reviewers returns the list of reviewer account IDs, and possibly participants too.
// The output is guaranteed to be sorted, without teams/apps, and without repetitions.
func reviewers(pr PullRequest, includeParticipants bool) []string {
	var accountIDs []string
	for _, r := range pr.Reviewers {
		accountIDs = append(accountIDs, r.AccountID)
	}

	if includeParticipants {
		for _, p := range pr.Participants {
			accountIDs = append(accountIDs, p.User.AccountID)
		}
	}

	slices.Sort(accountIDs)
	return slices.Compact(accountIDs)
}

// reviewerDiff returns the lists of added and removed reviewers
// (not participants), compared to the previous snapshot of the PR.
// The output is guaranteed to be sorted, without teams/apps, and without repetitions.
func reviewersDiff(prev, curr PullRequest) (added, removed []string) {
	prevIDs := reviewers(prev, false)
	currIDs := reviewers(curr, false)

	for _, id := range currIDs {
		if !slices.Contains(prevIDs, id) {
			added = append(added, id)
		}
	}

	for _, id := range prevIDs {
		if !slices.Contains(currIDs, id) {
			removed = append(removed, id)
		}
	}

	return
}

// reviewerMentions returns a Slack message mentioning all the newly added/removed reviewers.
func reviewerMentions(ctx workflow.Context, added, removed []string) string {
	var sb strings.Builder
	sb.WriteString(":bust_in_silhouette: %s ")

	switch len(added) {
	case 0:
		// Do nothing.
	case 1:
		sb.WriteString("added this reviewer:")
		sb.WriteString(bitbucketAccountsToSlackMentions(ctx, added))
	default:
		sb.WriteString("added these reviewers:")
		sb.WriteString(bitbucketAccountsToSlackMentions(ctx, added))
	}

	if len(added) > 0 && len(removed) > 0 {
		sb.WriteString(", and ")
	}

	switch len(removed) {
	case 0:
		// Do nothing.
	case 1:
		sb.WriteString("removed this reviewer:")
		sb.WriteString(bitbucketAccountsToSlackMentions(ctx, removed))
	default:
		sb.WriteString("removed these reviewers:")
		sb.WriteString(bitbucketAccountsToSlackMentions(ctx, removed))
	}

	sb.WriteString(".")
	return sb.String()
}

func bitbucketAccountsToSlackMentions(ctx workflow.Context, accountIDs []string) string {
	var sb strings.Builder
	for _, a := range accountIDs {
		if ref := users.BitbucketToSlackRef(ctx, a, ""); ref != "" {
			sb.WriteString(" " + ref)
		}
	}
	return sb.String()
}

func bitbucketToSlackIDs(ctx workflow.Context, accountIDs []string) []string {
	slackIDs := []string{}
	for _, aid := range accountIDs {
		// True = don't include opted-out users. They will still be mentioned
		// in the channel, but as non-members they won't be notified about it.
		if sid := users.BitbucketToSlackID(ctx, aid, true); sid != "" {
			slackIDs = append(slackIDs, sid)
		}
	}
	return slackIDs
}
