package commands

import (
	"log/slog"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/files"
	"github.com/tzrikka/revchat/pkg/slack"
	"github.com/tzrikka/revchat/pkg/slack/activities"
	"github.com/tzrikka/timpani-api/pkg/bitbucket"
)

func Clean(ctx workflow.Context, event SlashCommandEvent) error {
	url, paths, pr, err := reviewerData(ctx, event)
	if err != nil || url == nil || len(url) < 4 || len(paths) == 0 {
		return err
	}

	workspace, repo, branch, commit := slack.DestinationDetails(pr)
	owners, _ := files.OwnersPerPath(ctx, workspace, repo, branch, commit, paths, true)
	reviewers := slack.RequiredReviewers(paths, owners)
	for i, fullName := range reviewers {
		user, _ := data.SelectUserByRealName(fullName)
		if user.BitbucketID != "" {
			reviewers[i] = user.BitbucketID
		}
	}

	reviewers = append(reviewers, approversForClean(pr)...)
	reviewers = filterReviewers(pr, reviewers)

	// Need to impersonate in Bitbucket the user who sent this Slack command.
	linkID, err := thrippyLinkID(ctx, event.UserID, event.ChannelID)
	if err != nil || linkID == "" {
		PostEphemeralError(ctx, event, "failed to get current PR details from Bitbucket.")
		return err
	}

	// Retrieve the latest PR metadata from Bitbucket, just in case our stored snapshot is outdated.
	pr, err = bitbucket.PullRequestsGet(ctx, bitbucket.PullRequestsGetRequest{
		ThrippyLinkID: linkID,
		Workspace:     workspace,
		RepoSlug:      repo,
		PullRequestID: url[3],
	})
	if err != nil {
		logger.From(ctx).Error("failed to get Bitbucket PR", slog.Any("error", err), slog.String("pr_url", url[0]))
		PostEphemeralError(ctx, event, "failed to get current PR details from Bitbucket.")
		return err
	}

	// Bitbucket API quirk: it rejects updates with the "summary.html" field.
	delete(pr, "summary")

	// Update the reviewers list in Bitbucket.
	pr["reviewers"] = make([]map[string]any, len(reviewers))
	for i, accountID := range reviewers {
		pr["reviewers"].([]map[string]any)[i] = map[string]any{ //nolint:errcheck
			"account_id": accountID,
		}
	}

	_, err = bitbucket.PullRequestsUpdate(ctx, bitbucket.PullRequestsUpdateRequest{
		PullRequestsRequest: bitbucket.PullRequestsRequest{
			ThrippyLinkID: linkID,
			Workspace:     workspace,
			RepoSlug:      repo,
			PullRequestID: url[3],
		},
		PullRequest: pr,
	})
	if err != nil {
		logger.From(ctx).Error("failed to update Bitbucket PR", slog.Any("error", err),
			slog.String("pr_url", url[0]), slog.String("slack_user_id", event.UserID))
		PostEphemeralError(ctx, event, "failed to update PR reviewers in Bitbucket.")
		return err
	}

	return nil
}

func approversForClean(pr map[string]any) []string {
	participants, ok := pr["participants"].([]any)
	if !ok {
		return nil
	}

	var accountIDs []string
	for _, p := range participants {
		participant, ok := p.(map[string]any)
		if !ok {
			continue
		}
		approved, ok := participant["approved"].(bool)
		if !ok || !approved {
			continue
		}

		user, ok := participant["user"].(map[string]any)
		if !ok {
			continue
		}
		accountID, ok := user["account_id"].(string)
		if !ok {
			continue
		}

		accountIDs = append(accountIDs, accountID)
	}

	return accountIDs
}

func filterReviewers(pr map[string]any, required []string) []string {
	isRequired := make(map[string]bool, len(required))
	for _, req := range required {
		isRequired[req] = true
	}

	var remaining []string
	for _, r := range allReviewers(pr) {
		if isRequired[r] { // More efficient than [slices.Contains].
			remaining = append(remaining, r)
		}
	}

	return remaining
}

func allReviewers(pr map[string]any) []string {
	reviewers, ok := pr["reviewers"].([]any)
	if !ok {
		return nil
	}

	var accountIDs []string
	for _, r := range reviewers {
		reviewer, ok := r.(map[string]any)
		if !ok {
			continue
		}

		accountID, ok := reviewer["account_id"].(string)
		if !ok {
			continue
		}

		accountIDs = append(accountIDs, accountID)
	}

	return accountIDs
}

func thrippyLinkID(ctx workflow.Context, userID, channelID string) (string, error) {
	if len(userID) > 0 && userID[0] == 'B' {
		return "", nil // Slack bot, not a real user.
	}

	user, err := data.SelectUserBySlackID(userID)
	if err != nil {
		logger.From(ctx).Error("failed to load user by Slack ID", slog.Any("error", err), slog.String("user_id", userID))
		return "", err
	}

	if !data.IsOptedIn(user) {
		msg := ":warning: Cannot mirror this in the PR, you need to run this slash command: `/revchat opt-in`"
		return "", activities.PostEphemeralMessage(ctx, channelID, userID, msg)
	}

	return user.ThrippyLink, nil
}
