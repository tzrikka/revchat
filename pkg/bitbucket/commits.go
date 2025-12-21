package bitbucket

import (
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/timpani-api/pkg/bitbucket"
)

func commits(ctx workflow.Context, event PullRequestEvent) []Commit {
	workspace, repo, found := strings.Cut(event.Repository.FullName, "/")
	if !found {
		logger.Error(ctx, "failed to parse Bitbucket workspace and repository name",
			nil, slog.String("full_name", event.Repository.FullName))
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
		logger.Error(ctx, "failed to list Bitbucket PR commits", err,
			slog.String("pr_url", htmlURL(event.PullRequest.Links)), slog.String("workspace", workspace),
			slog.String("repo", repo), slog.Int("pr_id", event.PullRequest.ID))
		return nil
	}

	return cs
}

var diffURLPattern = regexp.MustCompile(`/([^/]+)/([^/]+)/diff/([^?]+)(\?path=(.*))?$`)

func sourceFile(ctx workflow.Context, diffURL, hash string) string {
	matches := diffURLPattern.FindStringSubmatch(diffURL)
	if len(matches) < 6 {
		logger.Error(ctx, "failed to parse Bitbucket diff URL", nil, slog.String("diff_url", diffURL))
		return ""
	}

	text, err := bitbucket.SourceGetFile(ctx, bitbucket.SourceGetRequest{
		Workspace: matches[1],
		RepoSlug:  matches[2],
		Commit:    hash,
		Path:      matches[5],
	})
	if err != nil {
		logger.Error(ctx, "failed to get Bitbucket file content", err,
			slog.String("commit", hash), slog.String("path", matches[5]))
		return ""
	}

	return text
}

// func fileDiff(ctx workflow.Context, url string) string {
// 	matches := diffURLPattern.FindStringSubmatch(url)
// 	if len(matches) < 4 {
// 		logger.Error(ctx, "failed to parse Bitbucket diff URL", nil, slog.String("diff_url", url))
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
// 		logger.Error(ctx, "failed to get Bitbucket PR diff", err, slog.String("diff_url", url))
// 		return ""
// 	}
//
// 	_, text, _ = strings.Cut(text, "@@")
// 	return strings.TrimSpace(strings.ReplaceAll(text, `\ No newline at end of file`, ""))
// }

func diffstat(ctx workflow.Context, event PullRequestEvent) []bitbucket.Diffstat {
	workspace, repo, found := strings.Cut(event.Repository.FullName, "/")
	if !found {
		logger.Error(ctx, "failed to parse Bitbucket workspace and repository name",
			nil, slog.String("full_name", event.Repository.FullName))
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
		logger.Error(ctx, "failed to get Bitbucket PR diffstat", err, slog.String("pr_url",
			htmlURL(event.PullRequest.Links)), slog.String("workspace", workspace),
			slog.String("repo", repo), slog.Int("pr_id", event.PullRequest.ID))
		return nil
	}

	return ds
}
