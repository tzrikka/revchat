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
	tslack "github.com/tzrikka/timpani-api/pkg/slack"
)

// MessageWorkflow mirrors Slack message creation/editing/deletion events
// as/in PR comments: https://docs.slack.dev/reference/events/message/
func (c *Config) MessageWorkflow(ctx workflow.Context, event messageEventWrapper) error {
	if !isRevChatChannel(ctx, event.InnerEvent.Channel) {
		return nil
	}

	userID := extractUserID(ctx, &event.InnerEvent)
	if userID == "" {
		logger.From(ctx).Error("could not determine who triggered a Slack message event")
		return errors.New("could not determine who triggered a Slack message event")
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

	switch subtype {
	case "", "bot_message", "file_share", "thread_broadcast":
		return c.createMessage(ctx, event.InnerEvent, userID)
	case "message_changed":
		return c.changeMessage(ctx, event.InnerEvent, userID)
	case "message_deleted":
		return c.deleteMessage(ctx, event.InnerEvent, userID)
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
	userID, err := data.GetSlackBotUserID(botID)
	if err != nil {
		logger.From(ctx).Error("failed to load Slack bot's user ID", slog.Any("error", err), slog.String("bot_id", botID))
		return ""
	}

	if userID != "" {
		return userID
	}

	bot, err := tslack.BotsInfo(ctx, botID)
	if err != nil {
		logger.From(ctx).Error("failed to retrieve bot info from Slack", slog.Any("error", err), slog.String("bot_id", botID))
		return ""
	}

	logger.From(ctx).Debug("retrieved bot info from Slack",
		slog.String("bot_id", botID), slog.String("user_id", bot.UserID), slog.String("name", bot.Name))
	if err := data.SetSlackBotUserID(botID, bot.UserID); err != nil {
		logger.From(ctx).Error("failed to save Slack bot's user ID", slog.Any("error", err), slog.String("bot_id", botID))
	}

	return bot.UserID
}

var commentURLPattern = regexp.MustCompile(`^https://[^/]+/([^/]+)/([^/]+)/pull-requests/(\d+)(.+comment-(\d+))?`)

const expectedSubmatches = 6

func urlParts(ctx workflow.Context, ids string) ([]string, error) {
	url, err := commentURL(ctx, ids)
	if err != nil || url == "" {
		return nil, err
	}

	parts := commentURLPattern.FindStringSubmatch(url)
	if len(parts) != expectedSubmatches {
		logger.From(ctx).Error("failed to parse Slack message's PR comment URL",
			slog.String("slack_ids", ids), slog.String("comment_url", url))
		return nil, fmt.Errorf("invalid Bitbucket PR URL: %s", url)
	}

	return parts, nil
}
