package activities

import (
	"errors"
	"log/slog"
	"regexp"
	"strconv"

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

var commentURLPattern = regexp.MustCompile(`^https://[^/]+/([^/]+)/([^/]+)/pull/(\d+)([^\s\d]+(\d+))?`)

// GetPullRequest allows the Thrippy link ID to be empty, even though it is encouraged to specify it.
func GetPullRequest(ctx workflow.Context, thrippyID, prURL string) (*github.PullRequest, error) {
	url := commentURLPattern.FindStringSubmatch(prURL)
	if len(url) < 4 {
		logger.From(ctx).Error("failed to parse GitHub PR's URL", slog.String("url", prURL))
		return nil, errors.New("invalid GitHub PR URL: " + prURL)
	}

	id, err := strconv.Atoi(url[3])
	if err != nil {
		logger.From(ctx).Error("failed to convert PR ID to integer", slog.Any("error", err),
			slog.String("thrippy_id", thrippyID), slog.String("pr_id", url[3]))
		return nil, errors.New("invalid PR ID in GitHub URL: " + url[3])
	}

	resp, err := github.PullRequestsGet(ctx, thrippyID, url[1], url[2], id)
	if err != nil {
		logger.From(ctx).Error("failed to get GitHub PR", slog.Any("error", err),
			slog.String("thrippy_id", thrippyID), slog.String("owner", url[1]),
			slog.String("repo", url[2]), slog.String("pr_id", url[3]))
		return nil, err
	}

	return resp, nil
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
