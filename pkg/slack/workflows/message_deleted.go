package workflows

import (
	"errors"
	"fmt"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/bitbucket/activities"
	"github.com/tzrikka/revchat/pkg/data"
)

func (c *Config) deleteMessage(ctx workflow.Context, event MessageEvent, userID string) error {
	switch {
	case c.BitbucketWorkspace != "":
		return deleteMessageBitbucket(ctx, event, userID)
	default:
		logger.From(ctx).Error("neither Bitbucket nor GitHub are configured")
		return errors.New("neither Bitbucket nor GitHub are configured")
	}
}

func deleteMessageBitbucket(ctx workflow.Context, event MessageEvent, userID string) error {
	// Don't delete "tombstone" messages (roots of threads).
	if event.PreviousMessage.Subtype == "tombstone" {
		return nil
	}

	// Need to impersonate in Bitbucket the user who deleted the Slack message.
	linkID, err := thrippyLinkID(ctx, userID, event.Channel)
	if err != nil || linkID == "" {
		return err
	}

	ids := fmt.Sprintf("%s/%s", event.Channel, event.DeletedTS)
	if event.PreviousMessage.ThreadTS != "" && event.PreviousMessage.ThreadTS != event.DeletedTS {
		ids = fmt.Sprintf("%s/%s/%s", event.Channel, event.PreviousMessage.ThreadTS, event.DeletedTS)
	}

	url, err := urlParts(ctx, ids)
	if err != nil {
		return err
	}

	data.DeleteURLAndIDMapping(ctx, url[0])

	return activities.DeletePullRequestComment(ctx, linkID, url[1], url[2], url[3], url[5])
}
