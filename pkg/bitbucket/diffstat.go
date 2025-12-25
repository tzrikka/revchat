package bitbucket

import (
	"log/slog"
	"strconv"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/timpani-api/pkg/bitbucket"
)

func Diffstat(ctx workflow.Context, event PullRequestEvent) []bitbucket.Diffstat {
	workspace, repo, found := strings.Cut(event.Repository.FullName, "/")
	if !found {
		logger.From(ctx).Error("failed to parse Bitbucket workspace and repository name",
			slog.String("full_name", event.Repository.FullName))
		return nil
	}

	ds, err := bitbucket.PullRequestsDiffstat(ctx, bitbucket.PullRequestsDiffstatRequest{
		PullRequestsRequest: bitbucket.PullRequestsRequest{
			Workspace:     workspace,
			RepoSlug:      repo,
			PullRequestID: strconv.Itoa(event.PullRequest.ID),
		},
	})
	if err != nil {
		logger.From(ctx).Error("failed to get Bitbucket PR diffstat", slog.Any("error", err),
			slog.String("pr_url", HTMLURL(event.PullRequest.Links)), slog.String("workspace", workspace),
			slog.String("repo", repo), slog.Int("pr_id", event.PullRequest.ID))
		return nil
	}

	return ds
}
