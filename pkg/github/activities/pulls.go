package activities

import (
	"errors"
	"log/slog"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/timpani-api/pkg/github"
)

func CreateFileReviewComment(ctx workflow.Context, thrippyID, owner, repo string, prID int, msg string) (string, error) {
	if thrippyID == "" {
		return "", errors.New("missing user authentication credentials")
	}

	files, err := ListPullRequestFiles(ctx, thrippyID, owner, repo, prID)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", errors.New("no files found in GitHub PR to comment on")
	}

	pr := github.PullRequestsRequest{ThrippyLinkID: thrippyID, Owner: owner, Repo: repo, PullNumber: prID}
	resp3, err := github.PullRequestsCommentsCreate(ctx, github.PullRequestsCommentsCreateRequest{
		PullRequestsRequest: pr, Body: msg,
		CommitID:    "HEAD",
		Path:        files[0].Filename,
		SubjectType: "file",
	})
	if err != nil {
		logger.From(ctx).Error("failed to create GitHub PR review comment", slog.Any("error", err),
			slog.String("thrippy_id", thrippyID), slog.String("owner", owner),
			slog.String("repo", repo), slog.Int("pr_id", prID))
		return "", err
	}

	return resp3.HTMLURL, nil
}

func CreateReviewCommentReply(ctx workflow.Context, thrippyID, owner, repo string, prID, commentID int, msg string) (string, error) {
	if thrippyID == "" {
		return "", errors.New("missing user authentication credentials")
	}

	pr := github.PullRequestsRequest{ThrippyLinkID: thrippyID, Owner: owner, Repo: repo, PullNumber: prID}
	resp, err := github.PullRequestsCommentsCreateReply(ctx, github.PullRequestsCommentsCreateReplyRequest{
		PullRequestsRequest: pr,
		CommentID:           commentID,
		Body:                msg,
	})
	if err != nil {
		logger.From(ctx).Error("failed to create GitHub review comment reply", slog.Any("error", err),
			slog.String("thrippy_id", thrippyID), slog.String("owner", owner), slog.String("repo", repo),
			slog.Int("pr_id", prID), slog.Int("comment_id", commentID))
		return "", err
	}

	return resp.HTMLURL, nil
}

func ListPullRequestFiles(ctx workflow.Context, thrippyID, owner, repo string, prID int) ([]github.File, error) {
	if thrippyID == "" {
		return nil, errors.New("missing user authentication credentials")
	}

	files, err := github.PullRequestsListFiles(ctx, thrippyID, owner, repo, prID)
	if err != nil {
		logger.From(ctx).Error("failed to list GitHub PR's files", slog.Any("error", err),
			slog.String("thrippy_id", thrippyID), slog.String("owner", owner),
			slog.String("repo", repo), slog.Int("pr_id", prID))
		return nil, err
	}

	return files, nil
}
