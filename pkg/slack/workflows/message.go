package workflows

import (
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack/activities"
	"github.com/tzrikka/timpani-api/pkg/slack"
)

// urlPattern is a regular expression that supports PR and comment URLs in Bitbucket and GitHub:
//  1. Hostname (e.g., "bitbucket.org", "github.com")
//  2. Bitbucket workspace / GitHub owner
//  3. Repository
//  4. Partial PR path ("" in GitHub / "-requests" in Bitbucket)
//  5. PR number
//  6. Optional suffix for comments
//  7. Numeric comment ID (if 6 isn't empty)
var urlPattern = regexp.MustCompile(`https://([^/]+)/([^/]+)/([^/]+)/pull(-requests)?/(\d+)(\S+(\d+))?`)

// MessageWorkflow mirrors Slack message creation/editing/deletion events
// as/in PR comments: https://docs.slack.dev/reference/events/message/
func (c *Config) MessageWorkflow(ctx workflow.Context, event messageEventWrapper) error {
	// Instead of calling ![isRevChatChannel], because we also need the PR's URL below.
	prURL, _ := c.switchURLAndID(ctx, event.InnerEvent.Channel)
	if prURL == "" {
		return c.triggerNudge(ctx, event, extractUserID(ctx, &event.InnerEvent))
	}

	userID := extractUserID(ctx, &event.InnerEvent)
	if userID == "" {
		logger.From(ctx).Error("could not determine who triggered a Slack message event")
		err := errors.New("could not determine who triggered a Slack message event")
		return activities.AlertError(ctx, c.AlertsChannel, "", err)
	}
	if selfTriggeredEvent(ctx, event.Authorizations, userID) {
		return nil
	}

	subtype := event.InnerEvent.Subtype
	if strings.HasPrefix(subtype, "channel_") || strings.HasPrefix(subtype, "group_") {
		return nil
	}
	if subtype == "reminder_add" || event.InnerEvent.User == "USLACKBOT" {
		return nil
	}

	isBitbucket := strings.HasPrefix(prURL, "https://bitbucket.org/")
	switch subtype {
	case "", "bot_message", "file_share", "thread_broadcast":
		return c.createMessage(ctx, event.InnerEvent, userID, isBitbucket)
	case "message_changed":
		return c.changeMessage(ctx, event.InnerEvent, userID, isBitbucket)
	case "message_deleted":
		return c.deleteMessage(ctx, event.InnerEvent, userID, isBitbucket)
	default:
		return nil
	}
}

// extractUserID determines the user ID of the user/app that triggered a Slack message event.
// This ID is located in different places depending on the event subtype and the user type.
func extractUserID(ctx workflow.Context, msg *MessageEvent) string {
	if msg == nil {
		return ""
	}

	if msg.Edited != nil && msg.Edited.User != "" {
		return msg.Edited.User
	}

	if msg.BotID != "" {
		return convertBotIDToUserID(ctx, msg.BotID)
	}

	if user := extractUserID(ctx, msg.Message); user != "" {
		return user
	}

	if user := extractUserID(ctx, msg.PreviousMessage); user != "" {
		return user
	}

	return msg.User
}

// convertBotIDToUserID uses cached API calls to convert Slack bot IDs to a user IDs.
func convertBotIDToUserID(ctx workflow.Context, botID string) string {
	userID, err := data.GetSlackBotUserID(ctx, botID)
	if err != nil {
		logger.From(ctx).Error("failed to load Slack bot's user ID", slog.Any("error", err), slog.String("bot_id", botID))
		return ""
	}
	if userID != "" {
		return userID
	}

	bot, err := slack.BotsInfo(ctx, botID)
	if err != nil {
		logger.From(ctx).Error("failed to retrieve bot info from Slack", slog.Any("error", err), slog.String("bot_id", botID))
		return ""
	}

	logger.From(ctx).Debug("retrieved bot info from Slack",
		slog.String("bot_id", botID), slog.String("user_id", bot.UserID), slog.String("name", bot.Name))
	if err := data.SetSlackBotUserID(ctx, botID, bot.UserID); err != nil {
		logger.From(ctx).Error("failed to save Slack bot's user ID", slog.Any("error", err), slog.String("bot_id", botID))
	}

	return bot.UserID
}

// urlParts converts the given Slack ID(s) to the corresponding PR or comment
// URL, and then extracts and returns its parts, based on [urlPattern].
func (c *Config) urlParts(ctx workflow.Context, ids string) ([]string, error) {
	url, err := c.switchURLAndID(ctx, ids)
	if err != nil {
		return nil, err
	}
	if url == "" {
		// When calling this function, we already confirmed that the channel is a
		// RevChat channel and the user is opted-in, so there should be a mapping.
		logger.From(ctx).Error("didn't find Slack message's PR comment URL", slog.String("slack_ids", ids))
		err := errors.New("didn't find Slack message's PR comment URL")
		return nil, activities.AlertError(ctx, c.AlertsChannel, "", err, "Slack IDs", fmt.Sprintf("`%s`", ids))
	}

	parts := urlPattern.FindStringSubmatch(url)
	if len(parts) != 8 {
		logger.From(ctx).Error("failed to parse Slack message's PR comment URL",
			slog.String("slack_ids", ids), slog.String("comment_url", url))
		err := errors.New("invalid PR comment URL: " + url)
		return nil, activities.AlertError(ctx, c.AlertsChannel, "", err, "Slack IDs", fmt.Sprintf("`%s`", ids), "Comment URL", url)
	}

	return parts, nil
}
