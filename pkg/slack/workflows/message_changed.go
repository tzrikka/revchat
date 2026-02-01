package workflows

import (
	"errors"
	"fmt"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	bitbucket "github.com/tzrikka/revchat/pkg/bitbucket/activities"
	"github.com/tzrikka/revchat/pkg/markdown"
)

func (c *Config) changeMessage(ctx workflow.Context, event MessageEvent, userID string, isBitbucket bool) error {
	// Ignore "fake" edit events when a broadcast reply is created/deleted.
	if event.Message.Text == event.PreviousMessage.Text {
		return nil
	}

	thrippyID, err := c.thrippyLinkID(ctx, userID, event.Channel)
	if err != nil || thrippyID == "" {
		return err
	}

	slackIDs := fmt.Sprintf("%s/%s", event.Channel, event.Message.TS)
	if event.Message.ThreadTS != "" && event.Message.ThreadTS != event.Message.TS {
		slackIDs = fmt.Sprintf("%s/%s/%s", event.Channel, event.Message.ThreadTS, event.Message.TS)
	}

	url, err := c.urlParts(ctx, slackIDs)
	if err != nil {
		return err
	}

	if isBitbucket {
		return editMessageInBitbucket(ctx, event, thrippyID, url)
	}
	return editMessageInGitHub(ctx, event, thrippyID, url)
}

func editMessageInBitbucket(ctx workflow.Context, event MessageEvent, thrippyID string, url []string) error {
	msg, _ := strings.CutSuffix(event.Message.Text, "\n\n[This comment was updated by RevChat]: #")
	msg = markdown.SlackToBitbucket(ctx, msg) + "\n\n[This comment was updated by RevChat]: #"
	return bitbucket.UpdatePullRequestComment(ctx, thrippyID, url[2], url[3], url[5], url[7], msg)
}

func editMessageInGitHub(ctx workflow.Context, _ MessageEvent, _ string, _ []string) error {
	logger.From(ctx).Error("edit Slack message in GitHub - not implemented yet")
	return errors.New("edit Slack message in GitHub - not implemented yet")
}
