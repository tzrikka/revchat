package activities

import (
	"log/slog"

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
