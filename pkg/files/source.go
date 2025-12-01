package files

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/cache"
	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/timpani-api/pkg/bitbucket"
)

// fileCache is a cache for "CODEOWNERS" and "highrisk.txt" files.
// It substantially reduces API calls in workflows that call [CountOwnedFiles]
// and [CountHighRiskFiles] and iterate over many users with many PRs,
// but on the other hand we don't want this data to become stale.
var fileCache = cache.New(15*time.Minute, cache.NoCleanup)

func getBitbucketSourceFile(ctx workflow.Context, workspace, repo, commit, path string) string {
	key := fmt.Sprintf("%s:%s:%s:%s", workspace, repo, commit, path)
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
		log.Error(ctx, "failed to read Bitbucket source file", "error", err,
			"workspace", workspace, "repo", repo, "commit", commit, "path", path)
		return ""
	}

	fileCache.Set(key, file, cache.DefaultExpiration)
	return file
}
