package bitbucket

import (
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack"
)

func (b Bitbucket) addReactionAsync(ctx workflow.Context, url, emoji string) {
	l := workflow.GetLogger(ctx)
	ids, err := data.SwitchURLAndID(url)
	if err != nil {
		l.Error("failed to retrieve PR comment's Slack IDs", "url", url, "error", err)
		return
	}

	id := strings.Split(ids, "/")
	if len(id) < 2 {
		l.Error("can't add reaction to Slack message - missing/bad IDs", "bitbucket_url", url, "slack_ids", ids)
		return
	}

	req := slack.ReactionsAddRequest{Channel: id[0], Timestamp: id[len(id)-1], Name: emoji}
	slack.AddReactionActivityAsync(ctx, b.cmd, req)
}

func (b Bitbucket) removeReactionAsync(ctx workflow.Context, url, emoji string) {
	l := workflow.GetLogger(ctx)
	ids, err := data.SwitchURLAndID(url)
	if err != nil {
		l.Error("failed to retrieve PR comment's Slack IDs", "url", url, "error", err)
		return
	}

	id := strings.Split(ids, "/")
	if len(id) < 2 {
		l.Error("can't remove reaction from Slack message - missing/bad IDs", "bitbucket_url", url, "slack_ids", ids)
		return
	}

	req := slack.ReactionsRemoveRequest{Channel: id[0], Timestamp: id[len(id)-1], Name: emoji}
	slack.RemoveReactionActivityAsync(ctx, b.cmd, req)
}
