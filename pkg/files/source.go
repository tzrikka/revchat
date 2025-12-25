package files

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/cache"
	"github.com/tzrikka/revchat/pkg/bitbucket/activities"
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

	file, err := activities.GetSourceFile(ctx, workspace, repo, branch, commit, path)
	if err != nil {
		return ""
	}

	fileCache.Set(key, file, cache.DefaultExpiration)
	return file
}
