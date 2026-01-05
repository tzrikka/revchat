package github

import (
	"fmt"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/timpani-api/pkg/slack"
)

func SetChannelBookmarks(ctx workflow.Context, channelID, prURL string, pr PullRequest) {
	reviewers := 0
	approvals := 0

	_ = slack.BookmarksAdd(ctx, channelID, fmt.Sprintf("Reviewers (%d)", reviewers), prURL, ":eyes:")
	_ = slack.BookmarksAdd(ctx, channelID, fmt.Sprintf("Comments (%d)", pr.Comments), prURL, ":speech_balloon:")
	_ = slack.BookmarksAdd(ctx, channelID, fmt.Sprintf("Approvals (%d)", approvals), prURL, ":+1:")
	_ = slack.BookmarksAdd(ctx, channelID, fmt.Sprintf("Commits (%d)", pr.Commits), prURL+"/commits", ":pushpin:")
	_ = slack.BookmarksAdd(ctx, channelID, fmt.Sprintf("Files changed (%d)", pr.ChangedFiles), prURL+"/files", ":open_file_folder:")
	_ = slack.BookmarksAdd(ctx, channelID, fmt.Sprintf("Diffs (+%d -%d)", pr.Additions, pr.Deletions), prURL+".diff", ":hammer_and_wrench:")
	_ = slack.BookmarksAdd(ctx, channelID, "Checks (0)", prURL+"/checks", ":vertical_traffic_light:")
}
