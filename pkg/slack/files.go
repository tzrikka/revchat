package slack

import (
	"errors"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/timpani-api/pkg/slack"
)

func DeleteFile(ctx workflow.Context, ids string) {
	parts := strings.Split(ids, "/")
	if len(parts) < 3 {
		log.Warn(ctx, "can't delete Slack file - missing/bad IDs", "slack_ids", ids)
		return
	}

	fileID := parts[len(parts)-1]
	if err := slack.FilesDelete(ctx, fileID); err != nil {
		log.Error(ctx, "failed to delete uploaded Slack file", "error", err, "file_id", fileID)
	}

	if err := data.DeleteURLAndIDMapping(ids); err != nil {
		log.Error(ctx, "failed to delete Slack file mapping", "error", err, "ids", ids)
	}
}

func Upload(ctx workflow.Context, content []byte, filename, title, snippetType, mimeType, channelID, threadTS string) (*slack.File, error) {
	uploadURL, fileID, err := slack.FilesGetUploadURLExternal(ctx, len(content), filename, snippetType, "")
	if err != nil {
		log.Error(ctx, "failed to get Slack URL to upload file", "error", err, "filename", filename)
		return nil, err
	}

	if err := slack.TimpaniUploadExternal(ctx, uploadURL, mimeType, content); err != nil {
		log.Error(ctx, "failed to upload file to Slack", "error", err, "filename", filename)
		return nil, err
	}

	files, err := slack.FilesCompleteUploadExternal(ctx, slack.FilesCompleteUploadExternalRequest{
		Files:     []slack.File{{ID: fileID, Title: title}},
		ChannelID: channelID,
		ThreadTS:  threadTS,
	})
	if err != nil {
		log.Error(ctx, "failed to complete Slack file upload", "error", err, "filename", filename)
		return nil, err
	}

	if len(files) == 0 {
		log.Error(ctx, "no files returned after completing Slack file upload", "file_id", fileID)
		return nil, errors.New("no files returned after completing Slack file upload")
	}

	return &files[0], nil
}
