package activities

import (
	"errors"
	"log/slog"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/timpani-api/pkg/github"
)

func CreateIssueComment(ctx workflow.Context, thrippyID, owner, repo string, issue int, msg string) (string, error) {
	if thrippyID == "" {
		return "", errors.New("missing user authentication credentials")
	}

	resp, err := github.IssuesCommentsCreate(ctx, thrippyID, owner, repo, issue, msg)
	if err != nil {
		logger.From(ctx).Error("failed to create GitHub issue comment", slog.Any("error", err),
			slog.String("thrippy_id", thrippyID), slog.String("owner", owner),
			slog.String("repo", repo), slog.Int("issue_id", issue))
		return "", err
	}

	return resp.HTMLURL, nil
}
