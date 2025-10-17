package bitbucket

import (
	"fmt"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/timpani-api/pkg/slack"
)

// updateChannelBookmarks updates the PR's Slack channel bookmarks based on the latest PR event. This
// is a deferred call that doesn't return an error, because handling the event itself is more important.
func (c Config) updateChannelBookmarks(ctx workflow.Context, event PullRequestEvent, channelID string, snapshot *PullRequest) {
	// PR update events already load the previous snapshot, so reuse it in that case.
	countCommits := true
	if snapshot == nil {
		countCommits = false
		url := event.PullRequest.Links["html"].HRef
		snapshot, _ = switchSnapshot(ctx, url, event.PullRequest)
		if snapshot == nil {
			return
		}
	}

	bookmarks, err := slack.BookmarksListActivity(ctx, channelID)
	if err != nil {
		log.Error(ctx, "failed to list Slack channel bookmarks", "error", err)
		return
	}

	if len(bookmarks) > 0 && snapshot.CommentCount != event.PullRequest.CommentCount {
		title := fmt.Sprintf("Comments (%d)", event.PullRequest.CommentCount)
		if err := slack.BookmarksEditTitleActivity(ctx, channelID, bookmarks[0].ID, title); err != nil {
			log.Error(ctx, "failed to update Slack channel's comments bookmark", "error", err)
		}
	}

	if len(bookmarks) > 1 && snapshot.TaskCount != event.PullRequest.TaskCount {
		title := fmt.Sprintf("Tasks (%d)", event.PullRequest.TaskCount)
		if err := slack.BookmarksEditTitleActivity(ctx, channelID, bookmarks[1].ID, title); err != nil {
			log.Error(ctx, "failed to update Slack channel's tasks bookmark", "error", err)
		}
	}

	if len(bookmarks) > 2 && countApprovals(*snapshot) != countApprovals(event.PullRequest) {
		title := fmt.Sprintf("Approvals (%d)", countApprovals(event.PullRequest))
		if err := slack.BookmarksEditTitleActivity(ctx, channelID, bookmarks[2].ID, title); err != nil {
			log.Error(ctx, "failed to update Slack channel's approvals bookmark", "error", err)
		}
	}

	if len(bookmarks) > 3 && countCommits {
		log.Warn(ctx, "count commits via API")
	}
}

func countApprovals(pr PullRequest) int {
	count := 0
	for _, p := range pr.Participants {
		if p.Approved {
			count++
		}
	}
	return count
}
