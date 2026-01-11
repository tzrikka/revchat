package workflows

import (
	"errors"
	"fmt"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/bitbucket/activities"
	"github.com/tzrikka/revchat/pkg/markdown"
)

func (c *Config) changeMessage(ctx workflow.Context, event MessageEvent, userID string) error {
	switch {
	case c.BitbucketWorkspace != "":
		return editMessageBitbucket(ctx, event, userID)
	default:
		logger.From(ctx).Error("neither Bitbucket nor GitHub are configured")
		return errors.New("neither Bitbucket nor GitHub are configured")
	}
}

func editMessageBitbucket(ctx workflow.Context, event MessageEvent, userID string) error {
	// Ignore "fake" edit events when a broadcast reply is created/deleted.
	if event.Message.Text == event.PreviousMessage.Text {
		return nil
	}

	// Need to impersonate in Bitbucket the user who edited the Slack message.
	linkID, err := thrippyLinkID(ctx, userID, event.Channel)
	if err != nil || linkID == "" {
		return err
	}

	ids := fmt.Sprintf("%s/%s", event.Channel, event.Message.TS)
	if event.Message.ThreadTS != "" && event.Message.ThreadTS != event.Message.TS {
		ids = fmt.Sprintf("%s/%s/%s", event.Channel, event.Message.ThreadTS, event.Message.TS)
	}

	url, err := urlParts(ctx, ids)
	if err != nil {
		return err
	}

	msg := markdown.SlackToBitbucket(ctx, event.Message.Text)
	return activities.UpdatePullRequestComment(ctx, linkID, url[1], url[2], url[3], url[5], msg)
}
