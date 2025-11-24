package slack

import (
	"errors"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/timpani-api/pkg/slack"
)

func Upload(ctx workflow.Context, content []byte, filename, title, snippetType, mimeType, channelID, threadTS string) (*slack.File, error) {
	uploadURL, fileID, err := slack.FilesGetUploadURLExternalActivity(ctx, len(content), filename, snippetType, "")
	if err != nil {
		log.Error(ctx, "failed to get Slack URL to upload file", "error", err, "filename", filename)
		return nil, err
	}

	if err := slack.TimpaniUploadExternalActivity(ctx, uploadURL, mimeType, content); err != nil {
		log.Error(ctx, "failed to upload file to Slack", "error", err, "filename", filename)
		return nil, err
	}

	files, err := slack.FilesCompleteUploadExternalActivity(ctx, slack.FilesCompleteUploadExternalRequest{
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
