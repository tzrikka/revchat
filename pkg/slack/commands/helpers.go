package commands

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack/activities"
)

var (
	bitbucketURLPattern = regexp.MustCompile(`^https://[^/]+/([\w-]+)/([\w-]+)/pull-requests/(\d+)`)
	userIDsPattern      = regexp.MustCompile(`<(@)(\w+)(\|[^>]*)?>`)
	userOrTeamIDPattern = regexp.MustCompile(`<(@|!subteam\^)(\w+)(\|[^>]*)?>`)
)

func expandSubteams(_ workflow.Context, ids []string) []string {
	if len(ids) == 0 {
		return nil
	}

	expanded := make([]string, 0, len(ids))
	for _, id := range ids {
		if !strings.HasPrefix(id, "S") {
			expanded = append(expanded, id)
			continue
		}
	}

	return expanded
}

func extractAtLeastOneUserID(ctx workflow.Context, event SlashCommandEvent, pattern *regexp.Regexp) []string {
	matches := pattern.FindAllStringSubmatch(event.Text, -1)
	if len(matches) == 0 {
		PostEphemeralError(ctx, event, "you need to mention at least one `@user`.")
		return nil
	}

	var ids []string
	for _, match := range matches {
		ids = append(ids, strings.ToUpper(match[2]))
	}

	return ids
}

func PostEphemeralError(ctx workflow.Context, event SlashCommandEvent, msg string) {
	// We're already reporting another error, there's nothing to do if this fails.
	_ = activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, ":warning: Error: "+msg)
}

// prDetailsFromChannel extracts the PR details based on the Slack channel's ID.
// This also ensures that the slash command is being run inside a RevChat channel, and
// indirectly that the user is opted-in since these channels are accessible only to them.
func prDetailsFromChannel(ctx workflow.Context, event SlashCommandEvent) []string {
	url, err := data.SwitchURLAndID(event.ChannelID)
	if err != nil {
		logger.From(ctx).Error("failed to convert Slack channel to PR URL",
			slog.Any("error", err), slog.String("channel_id", event.ChannelID))
		PostEphemeralError(ctx, event, "failed to read internal data about the PR.")
		return nil
	}

	if url == "" {
		PostEphemeralError(ctx, event, "this command can only be used inside RevChat channels.")
		return nil // Not a server error as far as we're concerned.
	}

	match := bitbucketURLPattern.FindStringSubmatch(url)
	if len(match) != 4 {
		logger.From(ctx).Error("failed to parse Bitbucket PR URL", slog.String("pr_url", url))
		PostEphemeralError(ctx, event, "this command can only be used inside RevChat channels.")
		return nil
	}

	return match
}

func reviewerData(ctx workflow.Context, event SlashCommandEvent) (url, paths []string, pr map[string]any, err error) {
	url = prDetailsFromChannel(ctx, event)
	if url == nil {
		return nil, nil, nil, nil
	}

	pr, err = data.LoadBitbucketPR(url[0])
	if err != nil {
		PostEphemeralError(ctx, event, "failed to load PR snapshot.")
		return url, nil, nil, err
	}

	paths = data.ReadBitbucketDiffstatPaths(url[0])
	if len(paths) == 0 {
		PostEphemeralError(ctx, event, "no file paths found in PR diffstat.")
		return url, nil, pr, nil
	}

	return url, paths, pr, nil
}

// UserDetails retrieves the user details from internal data based on
// their Slack ID, and (based on that) whether they are opted-in or not.
func UserDetails(ctx workflow.Context, event SlashCommandEvent, userID string) (data.User, bool, error) {
	user, err := data.SelectUserBySlackID(userID)
	if err != nil {
		logger.From(ctx).Error("failed to load user by Slack ID", slog.Any("error", err), slog.String("user_id", userID))
		PostEphemeralError(ctx, event, fmt.Sprintf("failed to read internal data about <@%s>.", userID))
		return data.User{}, false, err
	}

	return user, data.IsOptedIn(user), nil
}
