package github

import (
	"fmt"
	"log/slog"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/timpani-api/pkg/slack"
)

func newBookmarkTitles(pr *PullRequest, issue *Issue) []string {
	switch {
	case pr != nil:
		return []string{
			"Reviewers (0)",
			fmt.Sprintf("Comments (%d)", pr.Comments+pr.ReviewComments),
			"Approvals (0)",
			fmt.Sprintf("Commits (%d)", pr.Commits),
			fmt.Sprintf("Files changed (%d)", pr.ChangedFiles),
			fmt.Sprintf("Diffs (+%d -%d)", pr.Additions, pr.Deletions),
			"",
		}

	case issue != nil:
		return []string{"", fmt.Sprintf("Comments (%d)", issue.Comments), "", "", "", "", ""}

	default:
		return nil
	}
}

func SetChannelBookmarks(ctx workflow.Context, channelID, prURL string, pr PullRequest) {
	titles := newBookmarkTitles(&pr, nil)
	_ = slack.BookmarksAdd(ctx, channelID, titles[0], prURL, ":eyes:")
	_ = slack.BookmarksAdd(ctx, channelID, titles[1], prURL, ":speech_balloon:")
	_ = slack.BookmarksAdd(ctx, channelID, titles[2], prURL, ":+1:")
	_ = slack.BookmarksAdd(ctx, channelID, titles[3], prURL+"/commits", ":pushpin:")
	_ = slack.BookmarksAdd(ctx, channelID, titles[4], prURL+"/files", ":open_file_folder:")
	_ = slack.BookmarksAdd(ctx, channelID, titles[5], prURL+".diff", ":hammer_and_wrench:")
	_ = slack.BookmarksAdd(ctx, channelID, "Checks (0)", prURL+"/checks", ":vertical_traffic_light:")
}

// UpdateChannelBookmarks updates the bookmarks in the PR's Slack channel, based on the latest PR event.
// This is a deferred call that doesn't return an error, because handling the event itself is more important.
func UpdateChannelBookmarks(ctx workflow.Context, pr *PullRequest, issue *Issue, channelID string) {
	bookmarks, err := slack.BookmarksList(ctx, channelID)
	if err != nil {
		logger.From(ctx).Error("failed to list Slack channel bookmarks", slog.Any("error", err))
		return
	}

	newTitles := newBookmarkTitles(pr, issue)
	for i, b := range bookmarks {
		if i >= len(newTitles) {
			break
		}
		if t := newTitles[i]; t != "" && t != b.Title {
			if err := slack.BookmarksEditTitle(ctx, channelID, b.ID, t); err != nil {
				logger.From(ctx).Error("failed to update Slack channel bookmark", slog.Any("error", err), slog.String("title", t))
			}
		}
	}
}
