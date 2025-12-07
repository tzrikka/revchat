package bitbucket

import (
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/timpani-api/pkg/slack"
)

// addOKReaction adds the ":ok:" emoji as a reaction to the
// Slack message identified by the given Bitbucket comment URL.
func addOKReaction(ctx workflow.Context, url string) {
	ids, err := data.SwitchURLAndID(url)
	if err != nil {
		log.Error(ctx, "failed to retrieve PR comment's Slack IDs", "error", err, "bitbucket_url", url)
		return
	}

	id := strings.Split(ids, "/")
	if len(id) < 2 {
		log.Warn(ctx, "can't add reaction to Slack message - missing/bad IDs", "bitbucket_url", url, "slack_ids", ids)
		return
	}

	_ = slack.ReactionsAdd(ctx, id[0], id[len(id)-1], "ok")
}

// removeOKReaction removes the ":ok:" emoji from the Slack bot's reactions
// in the Slack message identified by the given Bitbucket comment URL.
func removeOKReaction(ctx workflow.Context, url string) {
	ids, err := data.SwitchURLAndID(url)
	if err != nil {
		log.Error(ctx, "failed to retrieve PR comment's Slack IDs", "error", err, "bitbucket_url", url)
		return
	}

	id := strings.Split(ids, "/")
	if len(id) < 2 {
		log.Error(ctx, "can't remove reaction from Slack message - missing/bad IDs", "bitbucket_url", url, "slack_ids", ids)
		return
	}

	_ = slack.ReactionsRemove(ctx, id[0], id[len(id)-1], "ok")
}
