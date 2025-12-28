package activities

import (
	"fmt"
	"log/slog"
	"regexp"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/timpani-api/pkg/bitbucket"
)

func CreatePullRequestComment(ctx workflow.Context, thrippyLinkID, workspace, repo, prID, parentID, msg string) (string, error) {
	resp, err := bitbucket.PullRequestsCreateComment(ctx, bitbucket.PullRequestsCreateCommentRequest{
		PullRequestsRequest: bitbucket.PullRequestsRequest{
			ThrippyLinkID: thrippyLinkID,
			Workspace:     workspace,
			RepoSlug:      repo,
			PullRequestID: prID,
		},
		Markdown: msg,
		ParentID: parentID, // Optional.
	})
	if err != nil {
		logger.From(ctx).Error("failed to create Bitbucket PR comment", slog.Any("error", err),
			slog.String("workspace", workspace), slog.String("repo", repo), slog.String("pr_id", prID))
		return "", err
	}

	return resp.Links["html"].HRef, nil
}

func DeletePullRequestComment(ctx workflow.Context, thrippyLinkID, workspace, repo, prID, commentID string) error {
	err := bitbucket.PullRequestsDeleteComment(ctx, bitbucket.PullRequestsDeleteCommentRequest{
		PullRequestsRequest: bitbucket.PullRequestsRequest{
			ThrippyLinkID: thrippyLinkID,
			Workspace:     workspace,
			RepoSlug:      repo,
			PullRequestID: prID,
		},
		CommentID: commentID,
	})
	if err != nil {
		logger.From(ctx).Error("failed to delete Bitbucket PR comment", slog.Any("error", err),
			slog.String("workspace", workspace), slog.String("repo", repo),
			slog.String("pr_id", prID), slog.String("comment_id", commentID))
		return err
	}

	return nil
}

var commentURLPattern = regexp.MustCompile(`^https://[^/]+/([^/]+)/([^/]+)/pull-requests/(\d+)(.+comment-(\d+))?`)

const expectedSubmatches = 6

func GetPullRequestComment(ctx workflow.Context, thrippyLinkID, commentURL string) (*bitbucket.Comment, error) {
	url := commentURLPattern.FindStringSubmatch(commentURL)
	if len(url) != expectedSubmatches {
		logger.From(ctx).Error("failed to parse Bitbucket PR comment's URL", slog.String("url", commentURL))
		return nil, fmt.Errorf("invalid Bitbucket PR comment URL: %s", commentURL)
	}

	resp, err := bitbucket.PullRequestsGetComment(ctx, bitbucket.PullRequestsGetCommentRequest{
		PullRequestsRequest: bitbucket.PullRequestsRequest{
			ThrippyLinkID: thrippyLinkID,
			Workspace:     url[1],
			RepoSlug:      url[2],
			PullRequestID: url[3],
		},
		CommentID: url[5],
	})
	if err != nil {
		logger.From(ctx).Error("failed to get Bitbucket PR comment", slog.Any("error", err),
			slog.String("workspace", url[1]), slog.String("repo", url[2]),
			slog.String("pr_id", url[3]), slog.String("comment_id", url[5]))
		return nil, err
	}

	return resp, nil
}

func UpdatePullRequestComment(ctx workflow.Context, thrippyLinkID, workspace, repo, prID, commentID, msg string) error {
	err := bitbucket.PullRequestsUpdateComment(ctx, bitbucket.PullRequestsUpdateCommentRequest{
		PullRequestsRequest: bitbucket.PullRequestsRequest{
			ThrippyLinkID: thrippyLinkID,
			Workspace:     workspace,
			RepoSlug:      repo,
			PullRequestID: prID,
		},
		CommentID: commentID,
		Markdown:  msg,
	})
	if err != nil {
		logger.From(ctx).Error("failed to update Bitbucket PR comment", slog.Any("error", err),
			slog.String("workspace", workspace), slog.String("repo", repo),
			slog.String("pr_id", prID), slog.String("comment_id", commentID))
		return err
	}

	return nil
}
