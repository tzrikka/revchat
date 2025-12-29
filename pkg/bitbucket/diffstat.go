package bitbucket

import (
	"log/slog"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/timpani-api/pkg/bitbucket"
)

func Diffstat(ctx workflow.Context, event PullRequestEvent) []bitbucket.Diffstat {
	user, err := data.SelectUserByBitbucketID(event.Actor.AccountID)
	if err != nil {
		// user.ThrippyLink == "", which is still usable for our purposes.
		logger.From(ctx).Warn("unexpected but not critical: failed to load user by Bitbucket ID",
			slog.Any("error", err), slog.String("user_id", event.Actor.AccountID))
	}

	workspace, repo, found := strings.Cut(event.Repository.FullName, "/")
	if !found {
		logger.From(ctx).Error("failed to parse Bitbucket workspace and repository name",
			slog.String("full_name", event.Repository.FullName))
		return nil
	}

	ds, err := bitbucket.PullRequestsDiffstat(ctx, user.ThrippyLink, workspace, repo, event.PullRequest.ID)
	if err != nil {
		logger.From(ctx).Error("failed to get Bitbucket PR diffstat", slog.Any("error", err),
			slog.String("thrippy_link_id", user.ThrippyLink), slog.String("pr_url", HTMLURL(event.PullRequest.Links)))
		return nil
	}

	return ds
}
