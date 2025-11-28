package files

import (
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/timpani-api/pkg/bitbucket"
)

func getBitbucketSourceFile(ctx workflow.Context, workspace, repo, commit, path string) string {
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

	return file
}
