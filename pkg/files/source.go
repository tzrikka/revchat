package files

import (
	"fmt"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/timpani-api/pkg/bitbucket"
)

var cache map[string]string

func getBitbucketSourceFile(ctx workflow.Context, workspace, repo, commit, path string) string {
	key := fmt.Sprintf("%s/%s/%s/%s", workspace, repo, commit, path)
	if file, ok := cache[key]; ok {
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

	cache[key] = file
	return file
}
