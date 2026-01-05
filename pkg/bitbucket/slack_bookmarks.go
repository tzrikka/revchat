package bitbucket

import (
	"fmt"
	"log/slog"
	"maps"
	"regexp"
	"slices"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/timpani-api/pkg/slack"
)

const (
	maxBookmarkTitleLen = 200
)

func SetChannelBookmarks(ctx workflow.Context, channelID, prURL string, pr PullRequest) {
	files := data.ReadBitbucketDiffstatLength(prURL)

	_ = slack.BookmarksAdd(ctx, channelID, fmt.Sprintf("Reviewers (%d)", len(accountIDs(pr.Reviewers))), prURL+"/overview", ":eyes:")
	_ = slack.BookmarksAdd(ctx, channelID, fmt.Sprintf("Comments (%d)", pr.CommentCount), prURL+"/overview", ":speech_balloon:")
	_ = slack.BookmarksAdd(ctx, channelID, fmt.Sprintf("Tasks (%d)", pr.TaskCount), prURL+"/overview", ":white_check_mark:")
	_ = slack.BookmarksAdd(ctx, channelID, fmt.Sprintf("Approvals (%d)", countApprovals(pr)), prURL+"/overview", ":+1:")
	_ = slack.BookmarksAdd(ctx, channelID, fmt.Sprintf("Commits (%d)", pr.CommitCount), prURL+"/commits", ":pushpin:")
	_ = slack.BookmarksAdd(ctx, channelID, fmt.Sprintf("Files changed (%d)", files), prURL+"/diff", ":open_file_folder:")
	_ = slack.BookmarksAdd(ctx, channelID, "Builds: no results", prURL+"/overview", ":vertical_traffic_light:")
}

// UpdateChannelBookmarks updates the bookmarks in the PR's Slack channel, based on the latest PR event.
// This is a deferred call that doesn't return an error, because handling the event itself is more important.
func UpdateChannelBookmarks(ctx workflow.Context, event PullRequestEvent, channelID string, snapshot *PullRequest) {
	prURL := HTMLURL(event.PullRequest.Links)

	// PR update events already load the previous snapshot, so reuse it in that case.
	updateCommits := true
	if snapshot == nil {
		updateCommits = false
		snapshot, _ = SwitchSnapshot(ctx, prURL, event.PullRequest)
		if snapshot == nil {
			return
		}
	}

	bookmarks, err := slack.BookmarksList(ctx, channelID)
	if err != nil {
		logger.From(ctx).Error("failed to list Slack channel's bookmarks", slog.Any("error", err))
		return
	}

	if cnt := len(accountIDs(event.PullRequest.Reviewers)); len(bookmarks) > 0 && len(accountIDs(snapshot.Reviewers)) != cnt {
		title := fmt.Sprintf("Reviewers (%d)", cnt)
		if err := slack.BookmarksEditTitle(ctx, channelID, bookmarks[0].ID, title); err != nil {
			logger.From(ctx).Error("failed to update Slack channel's reviewers bookmark", slog.Any("error", err))
		}
	}

	if len(bookmarks) > 1 && snapshot.CommentCount != event.PullRequest.CommentCount {
		title := fmt.Sprintf("Comments (%d)", event.PullRequest.CommentCount)
		if err := slack.BookmarksEditTitle(ctx, channelID, bookmarks[1].ID, title); err != nil {
			logger.From(ctx).Error("failed to update Slack channel's comments bookmark", slog.Any("error", err))
		}
	}

	if len(bookmarks) > 2 && snapshot.TaskCount != event.PullRequest.TaskCount {
		title := fmt.Sprintf("Tasks (%d)", event.PullRequest.TaskCount)
		if err := slack.BookmarksEditTitle(ctx, channelID, bookmarks[2].ID, title); err != nil {
			logger.From(ctx).Error("failed to update Slack channel's tasks bookmark", slog.Any("error", err))
		}
	}

	if len(bookmarks) > 3 && countApprovals(*snapshot) != countApprovals(event.PullRequest) {
		title := fmt.Sprintf("Approvals (%d)", countApprovals(event.PullRequest))
		if err := slack.BookmarksEditTitle(ctx, channelID, bookmarks[3].ID, title); err != nil {
			logger.From(ctx).Error("failed to update Slack channel's approvals bookmark", slog.Any("error", err))
		}
	}

	if len(bookmarks) > 4 && updateCommits {
		title := fmt.Sprintf("Commits (%d)", event.PullRequest.CommitCount)
		if title != bookmarks[4].Title {
			if err := slack.BookmarksEditTitle(ctx, channelID, bookmarks[4].ID, title); err != nil {
				logger.From(ctx).Error("failed to update Slack channel's commits bookmark", slog.Any("error", err))
			}
		}
	}

	if len(bookmarks) > 5 && updateCommits {
		title := fmt.Sprintf("Files changed (%d)", data.ReadBitbucketDiffstatLength(prURL))
		if title != bookmarks[5].Title {
			if err := slack.BookmarksEditTitle(ctx, channelID, bookmarks[5].ID, title); err != nil {
				logger.From(ctx).Error("failed to update Slack channel's files bookmark", slog.Any("error", err))
			}
		}
	}
}

// UpdateChannelBuildsBookmark updates the "Builds" bookmark in the PR's Slack channel, based on the latest repository
// event. This is a deferred call that doesn't return an error, because handling the event itself is more important.
func UpdateChannelBuildsBookmark(ctx workflow.Context, channelID, prURL string) {
	bookmarks, err := slack.BookmarksList(ctx, channelID)
	if err != nil {
		logger.From(ctx).Error("failed to list Slack channel's bookmarks", slog.Any("error", err))
		return
	}
	if len(bookmarks) < 7 {
		return
	}

	results := data.ReadBitbucketBuilds(ctx, prURL)
	if results == nil {
		return
	}

	var sb strings.Builder
	sb.WriteString("Builds: ")
	if len(results.Builds) == 0 {
		sb.WriteString("no results")
	}

	keys := slices.Sorted(maps.Keys(results.Builds))
	for i, k := range keys {
		if i > 0 {
			sb.WriteString(" ")
		}

		b := results.Builds[k]
		desc := regexp.MustCompile(`\.+$`).ReplaceAllString(strings.TrimSpace(b.Desc), "")
		sb.WriteString(fmt.Sprintf("%s %s", buildState(b.State), desc))
	}

	title := sb.String()
	if len(title) > maxBookmarkTitleLen {
		title = title[:maxBookmarkTitleLen]
	}

	if title == bookmarks[6].Title {
		return
	}

	if err := slack.BookmarksEditTitle(ctx, channelID, bookmarks[6].ID, title); err != nil {
		logger.From(ctx).Error("failed to update Slack channel's builds bookmark", slog.Any("error", err))
	}
}

func buildState(state string) string {
	switch state {
	case "INPROGRESS":
		return "[?]"
	case "SUCCESSFUL":
		return "[V]"
	default: // "FAILED", "STOPPED".
		return "[X]"
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
