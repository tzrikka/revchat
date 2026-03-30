package commands

import (
	"log/slog"
	"slices"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/files"
	"github.com/tzrikka/revchat/pkg/slack"
	"github.com/tzrikka/timpani-api/pkg/bitbucket"
)

func Clean(ctx workflow.Context, event SlashCommandEvent) error {
	url, paths, pr, err := reviewerData(ctx, event)
	if err != nil || len(url) < 6 || len(paths) == 0 {
		return err
	}

	workspace, repo, branch, commit := slack.PRIdentifiers(ctx, url[0], pr)
	owners, _ := files.OwnersPerPath(ctx, workspace, repo, branch, commit, paths, true)
	reviewers := requiredReviewers(paths, owners)
	for i, fullName := range reviewers {
		if user := data.SelectUserByRealName(ctx, fullName); user.BitbucketID != "" {
			reviewers[i] = user.BitbucketID
		}
	}

	reviewers = append(reviewers, approversForClean(pr)...)
	reviewers = filterReviewers(pr, reviewers)

	user, _, err := UserDetails(ctx, event, event.UserID)
	if err != nil {
		return err
	}

	// Retrieve the latest PR metadata from Bitbucket, just in case our stored snapshot is outdated.
	pr, err = bitbucket.PullRequestsGet(ctx, user.ThrippyLink, workspace, repo, url[5])
	if err != nil {
		logger.From(ctx).Error("failed to get Bitbucket PR", slog.Any("error", err), slog.String("pr_url", url[0]),
			slog.String("slack_user_id", event.UserID), slog.String("thrippy_id", user.ThrippyLink))
		PostEphemeralError(ctx, event, "failed to get current PR details from Bitbucket.")
		return err
	}

	// Bitbucket API quirk: it rejects updates with the "summary.html" field.
	delete(pr, "summary")

	// Update the reviewers list in Bitbucket.
	pr["reviewers"] = make([]map[string]any, len(reviewers))
	for i, accountID := range reviewers {
		pr["reviewers"].([]map[string]any)[i] = map[string]any{ //nolint:errcheck // Type conversion always succeeds.
			"account_id": accountID,
		}
	}

	if _, err := bitbucket.PullRequestsUpdate(ctx, user.ThrippyLink, workspace, repo, url[5], pr); err != nil {
		logger.From(ctx).Error("failed to update Bitbucket PR", slog.Any("error", err), slog.String("pr_url", url[0]),
			slog.String("slack_user_id", event.UserID), slog.String("thrippy_id", user.ThrippyLink))
		PostEphemeralError(ctx, event, "failed to update PR reviewers in Bitbucket.")
		return err
	}

	return nil
}

func requiredReviewers(paths []string, owners map[string][]string) []string {
	var required []string

	for _, p := range paths {
		required = append(required, owners[p]...)
	}

	slices.Sort(required)
	return slices.Compact(required)
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
