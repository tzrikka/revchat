package files

import (
	"fmt"
	"log/slog"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/cache"
	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/timpani-api/pkg/bitbucket"
)

// fileCache is a cache for "CODEOWNERS" and "highrisk.txt" files.
// It substantially reduces API calls in workflows that call [CountOwnedFiles]
// and [CountHighRiskFiles] and iterate over many users with many PRs,
// but on the other hand we don't want this data to become stale.
var fileCache = cache.New(10*time.Minute, cache.DefaultCleanupInterval)

func getBitbucketSourceFile(ctx workflow.Context, workspace, repo, branch, commit, path string) string {
	key := fmt.Sprintf("%s:%s:%s:%s", workspace, repo, branch, path)
	if file, ok := fileCache.Get(key); ok {
		return file
	}

	file, err := bitbucket.SourceGetFile(ctx, bitbucket.SourceGetRequest{
		Workspace: workspace,
		RepoSlug:  repo,
		Commit:    commit,
		Path:      path,
	})
	if err != nil {
		logger.From(ctx).Warn("failed to read Bitbucket source file",
			slog.Any("error", err), slog.String("workspace", workspace), slog.String("repo", repo),
			slog.String("branch", branch), slog.String("commit", commit), slog.String("path", path))
		return ""
	}

	fileCache.Set(key, file, cache.DefaultExpiration)
	return file
}
