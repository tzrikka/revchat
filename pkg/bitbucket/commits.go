package bitbucket

import (
	"log/slog"
	"strconv"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/timpani-api/pkg/bitbucket"
)

func Commits(ctx workflow.Context, event PullRequestEvent) []Commit {
	workspace, repo, found := strings.Cut(event.Repository.FullName, "/")
	if !found {
		logger.From(ctx).Error("failed to parse Bitbucket workspace and repository name",
			slog.String("full_name", event.Repository.FullName))
		return nil
	}

	user := data.SelectUserByBitbucketID(ctx, event.Actor.AccountID)

	cs, err := bitbucket.PullRequestsListCommits(ctx, user.ThrippyLink, workspace, repo, strconv.Itoa(event.PullRequest.ID))
	if err != nil {
		logger.From(ctx).Error("failed to list Bitbucket PR's commits", slog.Any("error", err),
			slog.String("thrippy_id", user.ThrippyLink), slog.String("pr_url", HTMLURL(event.PullRequest.Links)))
		return nil
	}

	return cs
}
