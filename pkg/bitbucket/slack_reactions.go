package bitbucket

import (
	"errors"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack"
)

func (c Config) addReaction(ctx workflow.Context, url, emoji string) error {
	l := workflow.GetLogger(ctx)
	ids, err := data.SwitchURLAndID(url)
	if err != nil {
		l.Error("failed to retrieve PR comment's Slack IDs", "error", err, "url", url)
		return err
	}

	id := strings.Split(ids, "/")
	if len(id) < 2 {
		l.Error("can't add reaction to Slack message - missing/bad IDs", "bitbucket_url", url, "slack_ids", ids)
		return errors.New("missing/bad Slack IDs")
	}

	return slack.AddReactionActivity(ctx, c.Cmd, id[0], id[len(id)-1], emoji)
}

func (c Config) removeReaction(ctx workflow.Context, url, emoji string) error {
	l := workflow.GetLogger(ctx)
	ids, err := data.SwitchURLAndID(url)
	if err != nil {
		l.Error("failed to retrieve PR comment's Slack IDs", "url", url, "error", err)
		return err
	}

	id := strings.Split(ids, "/")
	if len(id) < 2 {
		l.Error("can't remove reaction from Slack message - missing/bad IDs", "bitbucket_url", url, "slack_ids", ids)
		return errors.New("missing/bad Slack IDs")
	}

	return slack.RemoveReactionActivity(ctx, c.Cmd, id[0], id[len(id)-1], emoji)
}
