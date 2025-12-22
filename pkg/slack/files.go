package slack

import (
	"errors"
	"log/slog"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/timpani-api/pkg/slack"
)

func DeleteFile(ctx workflow.Context, ids string) {
	parts := strings.Split(ids, "/")
	if len(parts) < 3 {
		logger.From(ctx).Warn("can't delete Slack file - missing/bad IDs", slog.String("slack_ids", ids))
		return
	}

	fileID := parts[len(parts)-1]
	if err := slack.FilesDelete(ctx, fileID); err != nil {
		logger.From(ctx).Error("failed to delete uploaded Slack file",
			slog.Any("error", err), slog.String("file_id", fileID))
	}

	if err := data.DeleteURLAndIDMapping(ids); err != nil {
		logger.From(ctx).Error("failed to delete Slack file mapping",
			slog.Any("error", err), slog.String("ids", ids))
	}
}

func Upload(ctx workflow.Context, content []byte, filename, title, snippetType, mimeType, channelID, threadTS string) (*slack.File, error) {
	uploadURL, fileID, err := slack.FilesGetUploadURLExternal(ctx, len(content), filename, snippetType, "")
	if err != nil {
		logger.From(ctx).Error("failed to get Slack URL to upload file",
			slog.Any("error", err), slog.String("filename", filename))
		return nil, err
	}

	if err := slack.TimpaniUploadExternal(ctx, uploadURL, mimeType, content); err != nil {
		logger.From(ctx).Error("failed to upload file to Slack",
			slog.Any("error", err), slog.String("filename", filename))
		return nil, err
	}

	files, err := slack.FilesCompleteUploadExternal(ctx, slack.FilesCompleteUploadExternalRequest{
		Files:     []slack.File{{ID: fileID, Title: title}},
		ChannelID: channelID,
		ThreadTS:  threadTS,
	})
	if err != nil {
		logger.From(ctx).Error("failed to complete Slack file upload",
			slog.Any("error", err), slog.String("filename", filename))
		return nil, err
	}

	if len(files) == 0 {
		logger.From(ctx).Error("no files returned after completing Slack file upload", slog.String("file_id", fileID))
		return nil, errors.New("no files returned after completing Slack file upload")
	}

	return &files[0], nil
}
