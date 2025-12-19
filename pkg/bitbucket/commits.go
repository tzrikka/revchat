package bitbucket

import (
	"regexp"
	"strconv"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/timpani-api/pkg/bitbucket"
)

func commits(ctx workflow.Context, event PullRequestEvent) []Commit {
	workspace, repo, found := strings.Cut(event.Repository.FullName, "/")
	if !found {
		log.Error(ctx, "failed to parse Bitbucket workspace and repository name",
			"full_name", event.Repository.FullName)
		return nil
	}

	cs, err := bitbucket.PullRequestsListCommits(ctx, bitbucket.PullRequestsListCommitsRequest{
		PullRequestsRequest: bitbucket.PullRequestsRequest{
			Workspace:     workspace,
			RepoSlug:      repo,
			PullRequestID: strconv.Itoa(event.PullRequest.ID),
		},
	})
	if err != nil {
		log.Error(ctx, "failed to list Bitbucket PR commits", "error", err, "pr_url", htmlURL(event.PullRequest.Links),
			"workspace", workspace, "repo", repo, "pr_id", event.PullRequest.ID)
		return nil
	}

	return cs
}

var diffURLPattern = regexp.MustCompile(`/([^/]+)/([^/]+)/diff/([^?]+)(\?path=(.*))?$`)

func sourceFile(ctx workflow.Context, diffURL, hash string) string {
	matches := diffURLPattern.FindStringSubmatch(diffURL)
	if len(matches) < 6 {
		log.Error(ctx, "failed to parse Bitbucket diff URL", "diff_url", diffURL)
		return ""
	}

	text, err := bitbucket.SourceGetFile(ctx, bitbucket.SourceGetRequest{
		Workspace: matches[1],
		RepoSlug:  matches[2],
		Commit:    hash,
		Path:      matches[5],
	})
	if err != nil {
		log.Error(ctx, "failed to get Bitbucket file content", "error", err, "commit", hash, "path", matches[5])
		return ""
	}

	return text
}

// func fileDiff(ctx workflow.Context, url string) string {
// 	matches := diffURLPattern.FindStringSubmatch(url)
// 	if len(matches) < 4 {
// 		log.Error(ctx, "failed to parse Bitbucket diff URL", "diff_url", url)
// 		return ""
// 	}
// 	if len(matches) < 6 {
// 		matches = append(matches, "", "")
// 	}
//
// 	text, err := bitbucket.CommitsDiff(ctx, bitbucket.CommitsDiffRequest{
// 		Workspace: matches[1],
// 		RepoSlug:  matches[2],
// 		Spec:      matches[3],
// 		Path:      matches[5],
// 	})
// 	if err != nil {
// 		log.Error(ctx, "failed to get Bitbucket PR diff", "error", err, "diff_url", url)
// 		return ""
// 	}
//
// 	_, text, _ = strings.Cut(text, "@@")
// 	return strings.TrimSpace(strings.ReplaceAll(text, `\ No newline at end of file`, ""))
// }

func diffstat(ctx workflow.Context, event PullRequestEvent) []bitbucket.Diffstat {
	workspace, repo, found := strings.Cut(event.Repository.FullName, "/")
	if !found {
		log.Error(ctx, "failed to parse Bitbucket workspace and repository name", "full_name", event.Repository.FullName)
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
		log.Error(ctx, "failed to get Bitbucket PR diffstat", "error", err, "pr_url", htmlURL(event.PullRequest.Links),
			"workspace", workspace, "repo", repo, "pr_id", event.PullRequest.ID)
		return nil
	}

	return ds
}
