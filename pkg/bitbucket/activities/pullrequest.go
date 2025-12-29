package activities

import (
	"errors"
	"fmt"
	"log/slog"
	"regexp"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/timpani-api/pkg/bitbucket"
)

func CreatePullRequestComment(ctx workflow.Context, thrippyLinkID, workspace, repo, prID, parentID, msg string) (string, error) {
	if thrippyLinkID == "" {
		return "", errors.New("missing user authentication credentials")
	}

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
			slog.String("thrippy_link_id", thrippyLinkID), slog.String("workspace", workspace),
			slog.String("repo", repo), slog.String("pr_id", prID), slog.String("parent_id", parentID))
		return "", err
	}

	return resp.Links["html"].HRef, nil
}

func DeletePullRequestComment(ctx workflow.Context, thrippyLinkID, workspace, repo, prID, commentID string) error {
	if thrippyLinkID == "" {
		return errors.New("missing user authentication credentials")
	}

	if err := bitbucket.PullRequestsDeleteComment(ctx, thrippyLinkID, workspace, repo, prID, commentID); err != nil {
		logger.From(ctx).Error("failed to delete Bitbucket PR comment", slog.Any("error", err),
			slog.String("workspace", workspace), slog.String("repo", repo),
			slog.String("pr_id", prID), slog.String("comment_id", commentID))
		return err
	}

	return nil
}

var commentURLPattern = regexp.MustCompile(`^https://[^/]+/([^/]+)/([^/]+)/pull-requests/(\d+)(.+comment-(\d+))?`)

const expectedSubmatches = 6

// GetPullRequestComment allows the Thrippy link ID to be empty, even though it is encouraged to specify it.
func GetPullRequestComment(ctx workflow.Context, thrippyLinkID, commentURL string) (*bitbucket.Comment, error) {
	url := commentURLPattern.FindStringSubmatch(commentURL)
	if len(url) != expectedSubmatches {
		logger.From(ctx).Error("failed to parse Bitbucket PR comment's URL", slog.String("url", commentURL))
		return nil, fmt.Errorf("invalid Bitbucket PR comment URL: %s", commentURL)
	}

	resp, err := bitbucket.PullRequestsGetComment(ctx, thrippyLinkID, url[1], url[2], url[3], url[5])
	if err != nil {
		logger.From(ctx).Error("failed to get Bitbucket PR comment", slog.Any("error", err),
			slog.String("thrippy_link_id", thrippyLinkID), slog.String("comment_url", commentURL))
		return nil, err
	}

	return resp, nil
}

func UpdatePullRequestComment(ctx workflow.Context, thrippyLinkID, workspace, repo, prID, commentID, msg string) error {
	if thrippyLinkID == "" {
		return errors.New("missing user authentication credentials")
	}

	err := bitbucket.PullRequestsUpdateComment(ctx, thrippyLinkID, workspace, repo, prID, commentID, msg)
	if err != nil {
		logger.From(ctx).Error("failed to update Bitbucket PR comment", slog.Any("error", err),
			slog.String("thrippy_link_id", thrippyLinkID), slog.String("workspace", workspace),
			slog.String("repo", repo), slog.String("pr_id", prID), slog.String("comment_id", commentID))
		return err
	}

	return nil
}
