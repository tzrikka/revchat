package bitbucket

import (
	"strconv"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/timpani-api/pkg/bitbucket"
)

func commits(ctx workflow.Context, event PullRequestEvent) []Commit {
	workspace, repo, found := strings.Cut(event.Repository.FullName, "/")
	if !found {
		log.Error(ctx, "failed to parse Bitbucket workspace and repository name", "full_name", event.Repository.FullName)
		return nil
	}

	resp, err := bitbucket.PullRequestsListCommitsActivity(ctx, bitbucket.PullRequestsListCommitsRequest{
		Workspace:     workspace,
		RepoSlug:      repo,
		PullRequestID: strconv.Itoa(event.PullRequest.ID),
		AllPages:      true,
	})
	if err != nil {
		url := event.PullRequest.Links["html"].HRef
		log.Error(ctx, "failed to list Bitbucket PR commits", "error", err, "pr_url", url,
			"workspace", workspace, "repo", repo, "pr_id", event.PullRequest.ID)
		return nil
	}

	return resp.Values
}
