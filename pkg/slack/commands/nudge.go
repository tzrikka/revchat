package commands

import (
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack/activities"
)

func Nudge(ctx workflow.Context, event SlashCommandEvent) error {
	url := prDetailsFromChannel(ctx, event)
	if url == nil {
		return nil // Not a server error as far as we're concerned.
	}

	users := extractAtLeastOneUserID(ctx, event, userIDsPattern)
	if len(users) == 0 {
		return nil // Not a server error as far as we're concerned.
	}

	sent := make(map[string]bool, len(users))
	for _, userID := range users {
		// Avoid duplicate nudges, and check that the user is eligible to be nudged.
		if sent[userID] || !checkUserBeforeNudging(ctx, event, url[0], userID) {
			continue
		}

		msg := ":pleading_face: <@%s> is asking you to review <#%s> :pray:"
		if _, err := activities.PostMessage(ctx, userID, fmt.Sprintf(msg, event.UserID, event.ChannelID)); err != nil {
			PostEphemeralError(ctx, event, fmt.Sprintf("failed to send a nudge to <@%s>.", userID))
			continue
		}
		sent[userID] = true
	}

	if len(sent) == 0 {
		return nil
	}
	msg := "Sent nudge"
	if len(sent) > 1 {
		msg += "s"
	}
	msg = fmt.Sprintf("%s to: <@%s>", msg, strings.Join(slices.Sorted(maps.Keys(sent)), ">, <@"))
	return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}

// checkUserBeforeNudging ensures that the user exists, is opted-in, and is a reviewer of the PR.
func checkUserBeforeNudging(ctx workflow.Context, event SlashCommandEvent, url, userID string) bool {
	user, optedIn, err := UserDetails(ctx, event, userID)
	if err != nil {
		return false
	}
	if !optedIn {
		msg := fmt.Sprintf(":no_bell: <@%s> is not opted-in to use RevChat.", userID)
		_ = activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
		return false
	}

	ok, err := data.Nudge(url, user.Email)
	if err != nil {
		logger.From(ctx).Error("failed to nudge user", slog.Any("error", err),
			slog.String("pr_url", url), slog.String("user_id", userID))
		PostEphemeralError(ctx, event, fmt.Sprintf("internal data error while nudging <@%s>.", userID))
		return ok // May be true despite the error: a valid reviewer, but failed to save it.
	}
	if !ok {
		msg := fmt.Sprintf(":no_good: <@%s> is not a tracked reviewer of this PR.", userID)
		_ = activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
	}
	return ok
}
