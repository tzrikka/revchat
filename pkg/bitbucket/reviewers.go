package bitbucket

import (
	"slices"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/users"
)

// ChannelMembers returns a list of opted-in Slack user IDs who are PR authors/reviewers/followers.
// The output is guaranteed to be sorted, without repetitions, and not contain teams/apps.
func ChannelMembers(ctx workflow.Context, pr PullRequest) []string {
	accounts := []Account{pr.Author}

	if !pr.Draft {
		accounts = append(accounts, pr.Reviewers...)
		// Include non-reviewer participants as well, if there are any; deduplication is done below.
		for _, participant := range pr.Participants {
			accounts = append(accounts, participant.User)
		}
	}

	slackIDs := BitbucketToSlackIDs(ctx, accountIDs(accounts))
	slackIDs = append(slackIDs, data.SelectUserByBitbucketID(ctx, pr.Author.AccountID).Followers...)

	slices.Sort(slackIDs)
	return slices.Compact(slackIDs)
}

// ReviewerDiff returns the lists of added and removed reviewers, compared to the previous snapshot
// of the PR. The output is guaranteed to be sorted, without repetitions, and not contain teams/apps.
func ReviewersDiff(prev, curr PullRequest) (added, removed []string) {
	prevIDs := accountIDs(prev.Reviewers)
	currIDs := accountIDs(curr.Reviewers)

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

	return added, removed
}

// ReviewerMentions returns a Slack message mentioning all the newly added/removed reviewers.
func ReviewerMentions(ctx workflow.Context, added, removed []string) string {
	var sb strings.Builder
	sb.WriteString(":bust_in_silhouette: %s ")

	switch len(added) {
	case 0:
		// Do nothing.
	case 1:
		sb.WriteString("added this reviewer:")
		sb.WriteString(bitbucketAccountIDsToSlackMentions(ctx, added))
	default:
		sb.WriteString("added these reviewers:")
		sb.WriteString(bitbucketAccountIDsToSlackMentions(ctx, added))
	}

	if len(added) > 0 && len(removed) > 0 {
		sb.WriteString(", and ")
	}

	switch len(removed) {
	case 0:
		// Do nothing.
	case 1:
		sb.WriteString("removed this reviewer:")
		sb.WriteString(bitbucketAccountIDsToSlackMentions(ctx, removed))
	default:
		sb.WriteString("removed these reviewers:")
		sb.WriteString(bitbucketAccountIDsToSlackMentions(ctx, removed))
	}

	sb.WriteString(".")
	return sb.String()
}

func bitbucketAccountIDsToSlackMentions(ctx workflow.Context, accountIDs []string) string {
	var sb strings.Builder
	for _, id := range accountIDs {
		if mention := users.BitbucketIDToSlackRef(ctx, id, ""); mention != "" {
			sb.WriteString(" " + mention)
		}
	}
	return sb.String()
}

func BitbucketToSlackIDs(ctx workflow.Context, accountIDs []string) []string {
	slackIDs := make([]string, 0, len(accountIDs))
	for _, bitbucketID := range accountIDs {
		// True = don't include opted-out users. They will still be mentioned
		// in the channel, but as non-members they won't be notified about it.
		if slackID := users.BitbucketIDToSlackID(ctx, bitbucketID, true); slackID != "" {
			slackIDs = append(slackIDs, slackID)
		}
	}
	return slackIDs
}
