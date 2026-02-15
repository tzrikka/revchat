package activities

import (
	"errors"
	"log/slog"
	"regexp"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/timpani-api/pkg/bitbucket"
)

func CreatePullRequestComment(ctx workflow.Context, thrippyID, workspace, repo, prID, parentID, msg string) (string, error) {
	if thrippyID == "" {
		return "", errors.New("missing user authentication credentials")
	}

	resp, err := bitbucket.PullRequestsCreateComment(ctx, bitbucket.PullRequestsCreateCommentRequest{
		PullRequestsRequest: bitbucket.PullRequestsRequest{
			ThrippyLinkID: thrippyID,
			Workspace:     workspace,
			RepoSlug:      repo,
			PullRequestID: prID,
		},
		Markdown: msg,
		ParentID: parentID, // Optional.
	})
	if err != nil {
		logger.From(ctx).Error("failed to create Bitbucket PR comment", slog.Any("error", err),
			slog.String("thrippy_id", thrippyID), slog.String("workspace", workspace), slog.String("repo", repo),
			slog.String("pr_id", prID), slog.String("parent_id", parentID))
		return "", err
	}

	return resp.Links["html"].HRef, nil
}

func DeletePullRequestComment(ctx workflow.Context, thrippyID, workspace, repo, prID, commentID string) error {
	if thrippyID == "" {
		return errors.New("missing user authentication credentials")
	}

	if err := bitbucket.PullRequestsDeleteComment(ctx, thrippyID, workspace, repo, prID, commentID); err != nil {
		logger.From(ctx).Error("failed to delete Bitbucket PR comment", slog.Any("error", err),
			slog.String("workspace", workspace), slog.String("repo", repo),
			slog.String("pr_id", prID), slog.String("comment_id", commentID))
		return err
	}

	return nil
}

var commentURLPattern = regexp.MustCompile(`^https://[^/]+/([^/]+)/([^/]+)/pull-requests/(\d+)([^\s\d]+(\d+))?`)

// GetPullRequestComment allows the Thrippy link ID to be empty, even though it is encouraged to specify it.
func GetPullRequestComment(ctx workflow.Context, thrippyID, commentURL string) (*bitbucket.Comment, error) {
	url := commentURLPattern.FindStringSubmatch(commentURL)
	if len(url) != 6 {
		logger.From(ctx).Error("failed to parse Bitbucket PR comment's URL", slog.String("url", commentURL))
		return nil, errors.New("invalid Bitbucket PR comment URL: " + commentURL)
	}

	resp, err := bitbucket.PullRequestsGetComment(ctx, thrippyID, url[1], url[2], url[3], url[5])
	if err != nil {
		logger.From(ctx).Error("failed to get Bitbucket PR comment", slog.Any("error", err),
			slog.String("thrippy_id", thrippyID), slog.String("comment_url", commentURL))
		return nil, err
	}

	return resp, nil
}

// ListPullRequestTasks allows the Thrippy link ID to be empty, even though it is encouraged to specify it.
func ListPullRequestTasks(ctx workflow.Context, thrippyID, prURL string) ([]bitbucket.Task, error) {
	url := commentURLPattern.FindStringSubmatch(prURL)
	if len(url) < 4 {
		logger.From(ctx).Error("failed to parse Bitbucket PR's URL", slog.String("url", prURL))
		return nil, errors.New("invalid Bitbucket PR URL: " + prURL)
	}

	resp, err := bitbucket.PullRequestsListTasks(ctx, thrippyID, url[1], url[2], url[3])
	if err != nil {
		logger.From(ctx).Error("failed to list Bitbucket PR tasks", slog.Any("error", err),
			slog.String("thrippy_id", thrippyID), slog.String("workspace", url[1]),
			slog.String("repo", url[2]), slog.String("pr_id", url[3]))
		return nil, err
	}

	return resp, nil
}

func UpdatePullRequestComment(ctx workflow.Context, thrippyID, workspace, repo, prID, commentID, msg string) error {
	if thrippyID == "" {
		return errors.New("missing user authentication credentials")
	}

	err := bitbucket.PullRequestsUpdateComment(ctx, thrippyID, workspace, repo, prID, commentID, msg)
	if err != nil {
		logger.From(ctx).Error("failed to update Bitbucket PR comment", slog.Any("error", err),
			slog.String("thrippy_id", thrippyID), slog.String("workspace", workspace), slog.String("repo", repo),
			slog.String("pr_id", prID), slog.String("comment_id", commentID))
		return err
	}

	return nil
}
