package commands

import (
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack/activities"
	"github.com/tzrikka/timpani-api/pkg/slack"
)

var (
	bitbucketURLPattern  = regexp.MustCompile(`^https://[^/]+/([\w-]+)/([\w-]+)/pull-requests/(\d+)`)
	userOrGroupIDPattern = regexp.MustCompile(`<(@|!subteam\^)(\w+)(\|[^>]*)?>`)
)

// extractAtLeastOneUserID extracts at least one user or group ID from the slash command text.
// Groups are expanded into their member IDs. If no IDs are found, a message is sent to the user.
func extractAtLeastOneUserID(ctx workflow.Context, event SlashCommandEvent) []string {
	matches := userOrGroupIDPattern.FindAllStringSubmatch(event.Text, -1)
	if len(matches) == 0 {
		PostEphemeralError(ctx, event, "you need to mention at least one `@user` or `@group`.")
		return nil
	}

	var ids []string
	for _, match := range matches {
		if match[1] == "@" {
			ids = append(ids, strings.ToUpper(match[2]))
			continue
		}
		members, err := slack.UserGroupsUsersList(ctx, strings.ToUpper(match[2]), false)
		if err != nil {
			logger.From(ctx).Error("failed to expand Slack user group",
				slog.Any("error", err), slog.String("subteam_id", match[2]))
			PostEphemeralError(ctx, event, fmt.Sprintf("failed to expand the user group `<!subteam^%s>`.", match[2]))
			continue
		}
		ids = append(ids, members...)
	}

	slices.Sort(ids)
	return slices.Compact(ids)
}

func PostEphemeralError(ctx workflow.Context, event SlashCommandEvent, msg string) {
	// We're already reporting another error, there's nothing to do if this fails.
	_ = activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, ":warning: Error: "+msg)
}

// prDetailsFromChannel extracts the PR details based on the Slack channel's ID.
// This also ensures that the slash command is being run inside a RevChat channel, and
// indirectly that the user is opted-in since these channels are accessible only to them.
func prDetailsFromChannel(ctx workflow.Context, event SlashCommandEvent) ([]string, error) {
	url, err := data.SwitchURLAndID(ctx, event.ChannelID)
	if err != nil {
		PostEphemeralError(ctx, event, "failed to read internal data about this channel.")
		return nil, err
	}
	if url == "" {
		PostEphemeralError(ctx, event, "this command can only be used inside RevChat channels.")
		return nil, nil // Not a server error as far as we're concerned.
	}

	parts := bitbucketURLPattern.FindStringSubmatch(url)
	if len(parts) != 4 {
		logger.From(ctx).Error("failed to parse PR URL", slog.String("pr_url", url))
		PostEphemeralError(ctx, event, "failed to determine the context of this channel.")
		return nil, fmt.Errorf("failed to parse PR URL: %s", url)
	}

	return parts, nil
}

func reviewerData(ctx workflow.Context, event SlashCommandEvent) (url, paths []string, pr map[string]any, err error) {
	url, err = prDetailsFromChannel(ctx, event)
	if url == nil {
		return nil, nil, nil, err // The error may or may not be nil.
	}

	pr, err = data.LoadBitbucketPR(ctx, url[0])
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
	user, optedIn, err := data.SelectUserBySlackID(ctx, userID)
	if err != nil {
		msg := "failed to read internal data about you."
		if userID != event.UserID {
			msg = fmt.Sprintf("failed to read internal data about <@%s>.", userID)
		}
		PostEphemeralError(ctx, event, msg)
		return data.User{}, false, err
	}

	return user, optedIn, nil
}
