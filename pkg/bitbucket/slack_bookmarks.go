package bitbucket

import (
	"fmt"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/timpani-api/pkg/slack"
)

func setChannelBookmarks(ctx workflow.Context, channelID, url string, pr PullRequest) {
	_ = slack.BookmarksAddActivity(ctx, channelID, fmt.Sprintf("Reviewers (%d)", len(reviewers(pr, false))), url+"/overview", ":eyes:")
	_ = slack.BookmarksAddActivity(ctx, channelID, fmt.Sprintf("Comments (%d)", pr.CommentCount), url+"/overview", ":speech_balloon:")
	_ = slack.BookmarksAddActivity(ctx, channelID, fmt.Sprintf("Tasks (%d)", pr.TaskCount), url+"/overview", ":white_check_mark:")
	_ = slack.BookmarksAddActivity(ctx, channelID, fmt.Sprintf("Approvals (%d)", countApprovals(pr)), url+"/overview", ":+1:")
	_ = slack.BookmarksAddActivity(ctx, channelID, fmt.Sprintf("Commits (%d)", pr.CommitCount), url+"/commits", ":pushpin:")
	_ = slack.BookmarksAddActivity(ctx, channelID, fmt.Sprintf("Files changed (%d)", 0), url+"/diff", ":open_file_folder:")
}

// updateChannelBookmarks updates the PR's Slack channel bookmarks based on the latest PR event. This
// is a deferred call that doesn't return an error, because handling the event itself is more important.
func updateChannelBookmarks(ctx workflow.Context, event PullRequestEvent, channelID string, snapshot *PullRequest) {
	// PR update events already load the previous snapshot, so reuse it in that case.
	updateCommits := true
	if snapshot == nil {
		updateCommits = false
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

	if cnt := len(reviewers(event.PullRequest, false)); len(bookmarks) > 0 && len(reviewers(*snapshot, false)) != cnt {
		title := fmt.Sprintf("Reviewers (%d)", cnt)
		if err := slack.BookmarksEditTitleActivity(ctx, channelID, bookmarks[0].ID, title); err != nil {
			log.Error(ctx, "failed to update Slack channel's reviewers bookmark", "error", err)
		}
	}

	if len(bookmarks) > 1 && snapshot.CommentCount != event.PullRequest.CommentCount {
		title := fmt.Sprintf("Comments (%d)", event.PullRequest.CommentCount)
		if err := slack.BookmarksEditTitleActivity(ctx, channelID, bookmarks[1].ID, title); err != nil {
			log.Error(ctx, "failed to update Slack channel's comments bookmark", "error", err)
		}
	}

	if len(bookmarks) > 2 && snapshot.TaskCount != event.PullRequest.TaskCount {
		title := fmt.Sprintf("Tasks (%d)", event.PullRequest.TaskCount)
		if err := slack.BookmarksEditTitleActivity(ctx, channelID, bookmarks[2].ID, title); err != nil {
			log.Error(ctx, "failed to update Slack channel's tasks bookmark", "error", err)
		}
	}

	if len(bookmarks) > 3 && countApprovals(*snapshot) != countApprovals(event.PullRequest) {
		title := fmt.Sprintf("Approvals (%d)", countApprovals(event.PullRequest))
		if err := slack.BookmarksEditTitleActivity(ctx, channelID, bookmarks[3].ID, title); err != nil {
			log.Error(ctx, "failed to update Slack channel's approvals bookmark", "error", err)
		}
	}

	if len(bookmarks) > 4 && updateCommits {
		title := fmt.Sprintf("Commits (%d)", event.PullRequest.CommitCount)
		if err := slack.BookmarksEditTitleActivity(ctx, channelID, bookmarks[4].ID, title); err != nil {
			log.Error(ctx, "failed to update Slack channel's commits bookmark", "error", err)
		}
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
