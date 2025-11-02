package bitbucket

import (
	"errors"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/timpani-api/pkg/slack"
)

func addReaction(ctx workflow.Context, url, emoji string) error {
	ids, err := data.SwitchURLAndID(url)
	if err != nil {
		log.Error(ctx, "failed to retrieve PR comment's Slack IDs", "error", err, "url", url)
		return err
	}

	id := strings.Split(ids, "/")
	if len(id) < 2 {
		log.Error(ctx, "can't add reaction to Slack message - missing/bad IDs", "bitbucket_url", url, "slack_ids", ids)
		return errors.New("missing/bad Slack IDs")
	}

	return slack.ReactionsAddActivity(ctx, id[0], id[len(id)-1], emoji)
}

func removeReaction(ctx workflow.Context, url, emoji string) error {
	ids, err := data.SwitchURLAndID(url)
	if err != nil {
		log.Error(ctx, "failed to retrieve PR comment's Slack IDs", "error", err, "url", url)
		return err
	}

	id := strings.Split(ids, "/")
	if len(id) < 2 {
		log.Error(ctx, "can't remove reaction from Slack message - missing/bad IDs", "bitbucket_url", url, "slack_ids", ids)
		return errors.New("missing/bad Slack IDs")
	}

	return slack.ReactionsRemoveActivity(ctx, id[0], id[len(id)-1], emoji)
}
