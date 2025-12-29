package activities

import (
	"log/slog"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/timpani-api/pkg/bitbucket"
)

func GetSourceFile(ctx workflow.Context, workspace, repo, branch, commit, path string) (string, error) {
	file, err := bitbucket.SourceGetFile(ctx, "", workspace, repo, commit, path)
	if err != nil {
		logger.From(ctx).Warn("failed to read Bitbucket source file",
			slog.Any("error", err), slog.String("workspace", workspace), slog.String("repo", repo),
			slog.String("branch", branch), slog.String("commit", commit), slog.String("path", path))
		return "", err
	}

	return file, nil
}
