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

func newBookmarkTitles(pr PullRequest, files int) []string {
	return []string{
		fmt.Sprintf("Reviewers (%d)", len(accountIDs(pr.Reviewers))),
		fmt.Sprintf("Comments (%d)", pr.CommentCount),
		fmt.Sprintf("Tasks (%d)", pr.TaskCount),
		fmt.Sprintf("Approvals (%d)", countApprovals(pr)),
		fmt.Sprintf("Commits (%d)", pr.CommitCount),
		fmt.Sprintf("Files changed (%d)", files),
		"",
	}
}

func SetChannelBookmarks(ctx workflow.Context, channelID, prURL string, pr PullRequest) {
	titles := newBookmarkTitles(pr, len(data.ReadBitbucketDiffstatPaths(prURL)))
	_ = slack.BookmarksAdd(ctx, channelID, titles[0], prURL+"/overview", ":eyes:")
	_ = slack.BookmarksAdd(ctx, channelID, titles[1], prURL+"/overview", ":speech_balloon:")
	_ = slack.BookmarksAdd(ctx, channelID, titles[2], prURL+"/overview", ":white_check_mark:")
	_ = slack.BookmarksAdd(ctx, channelID, titles[3], prURL+"/overview", ":+1:")
	_ = slack.BookmarksAdd(ctx, channelID, titles[4], prURL+"/commits", ":pushpin:")
	_ = slack.BookmarksAdd(ctx, channelID, titles[5], prURL+"/diff", ":open_file_folder:")
	_ = slack.BookmarksAdd(ctx, channelID, "Builds: no results", prURL+"/overview", ":vertical_traffic_light:")
}

// UpdateChannelBookmarks updates the bookmarks in the PR's Slack channel, based on the latest PR event.
// This is a deferred call that doesn't return an error, because handling the event itself is more important.
func UpdateChannelBookmarks(ctx workflow.Context, pr PullRequest, prURL, channelID string) {
	bookmarks, err := slack.BookmarksList(ctx, channelID)
	if err != nil {
		logger.From(ctx).Error("failed to list Slack channel bookmarks", slog.Any("error", err))
		return
	}

	newTitles := newBookmarkTitles(pr, len(data.ReadBitbucketDiffstatPaths(prURL)))
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

// UpdateChannelBuildsBookmark updates the "Builds" bookmark in the PR's Slack channel, based on the latest repository
// event. This is a deferred call that doesn't return an error, because handling the event itself is more important.
func UpdateChannelBuildsBookmark(ctx workflow.Context, channelID, prURL string) {
	bookmarks, err := slack.BookmarksList(ctx, channelID)
	if err != nil {
		logger.From(ctx).Error("failed to list Slack channel bookmarks", slog.Any("error", err))
		return
	}
	if len(bookmarks) < 7 {
		return
	}

	prStatus := data.ReadBitbucketBuilds(ctx, prURL)

	sb := new(strings.Builder)
	sb.WriteString("Builds: ")
	if len(prStatus.Builds) == 0 {
		sb.WriteString("no results")
	}

	keys := slices.Sorted(maps.Keys(prStatus.Builds))
	for i, k := range keys {
		if i > 0 {
			sb.WriteString(" ")
		}

		b := prStatus.Builds[k]
		desc := regexp.MustCompile(`\.+$`).ReplaceAllString(strings.TrimSpace(b.Desc), "")
		fmt.Fprintf(sb, "%s %s", buildState(b.State), desc)
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
