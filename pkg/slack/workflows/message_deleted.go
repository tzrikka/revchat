package workflows

import (
	"errors"
	"fmt"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	bitbucket "github.com/tzrikka/revchat/pkg/bitbucket/activities"
	"github.com/tzrikka/revchat/pkg/data"
)

func deleteMessage(ctx workflow.Context, event MessageEvent, userID string, isBitbucket bool) error {
	// Don't delete "tombstone" messages (roots of threads).
	if event.PreviousMessage.Subtype == "tombstone" {
		return nil
	}

	thrippyID, err := thrippyLinkID(ctx, userID, event.Channel)
	if err != nil || thrippyID == "" {
		return err
	}

	slackIDs := fmt.Sprintf("%s/%s", event.Channel, event.DeletedTS)
	if event.PreviousMessage.ThreadTS != "" && event.PreviousMessage.ThreadTS != event.DeletedTS {
		slackIDs = fmt.Sprintf("%s/%s/%s", event.Channel, event.PreviousMessage.ThreadTS, event.DeletedTS)
	}

	url, err := urlParts(ctx, slackIDs)
	if err != nil {
		return err
	}

	if isBitbucket {
		return deleteMessageInBitbucket(ctx, thrippyID, url)
	}
	return deleteMessageInGitHub(ctx, thrippyID, url)
}

func deleteMessageInBitbucket(ctx workflow.Context, thrippyID string, url []string) error {
	data.DeleteURLAndIDMapping(ctx, url[0])
	return bitbucket.DeletePullRequestComment(ctx, thrippyID, url[2], url[3], url[5], url[7])
}

func deleteMessageInGitHub(ctx workflow.Context, _ string, _ []string) error {
	logger.From(ctx).Error("delete Slack message in GitHub - not implemented yet")
	return errors.New("delete Slack message in GitHub - not implemented yet")
}
