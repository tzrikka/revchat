package commands

import (
	"fmt"
	"log/slog"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack/activities"
)

func Follow(ctx workflow.Context, event SlashCommandEvent) error {
	// Ensure that the calling user is opted-in, i.e. authorized us & allowed to join PR channels.
	_, optedIn, err := UserDetails(ctx, event, event.UserID)
	if err != nil {
		return nil // Not a server error as far as we're concerned.
	}
	if !optedIn {
		PostEphemeralError(ctx, event, "you need to opt-in first.")
		return nil // Not a server error as far as we're concerned.
	}

	users := extractAtLeastOneUserID(ctx, event)
	if len(users) == 0 {
		return nil
	}

	done := make([]string, 0, len(users))
	for _, userID := range users {
		_, optedIn, err := UserDetails(ctx, event, userID)
		if err != nil {
			continue
		}
		if !optedIn {
			PostEphemeralError(ctx, event, fmt.Sprintf(":no_bell: <@%s> isn't opted-in to use RevChat.", userID))
			continue
		}

		if err := data.FollowUser(event.UserID, userID); err != nil {
			logger.From(ctx).Error("failed to follow user", slog.Any("error", err),
				slog.String("follower_id", event.UserID), slog.String("followed_id", userID))
			PostEphemeralError(ctx, event, fmt.Sprintf("failed to follow <@%s>.", userID))
			continue
		}

		done = append(done, userID)
	}

	if len(done) == 0 {
		return nil
	}

	msg := fmt.Sprintf("You will now be added to channels for PRs authored by: <@%s>.", strings.Join(done, ">, <@"))
	return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}

func Unfollow(ctx workflow.Context, event SlashCommandEvent) error {
	// Ensure that the calling user is opted-in, i.e. authorized us & allowed to join PR channels.
	_, optedIn, err := UserDetails(ctx, event, event.UserID)
	if err != nil {
		return nil // Not a server error as far as we're concerned.
	}
	if !optedIn {
		PostEphemeralError(ctx, event, "you need to opt-in first.")
		return nil // Not a server error as far as we're concerned.
	}

	users := extractAtLeastOneUserID(ctx, event)
	if len(users) == 0 {
		return nil
	}

	done := make([]string, 0, len(users))
	for _, userID := range users {
		_, optedIn, err := UserDetails(ctx, event, userID)
		if err != nil {
			continue
		}
		if !optedIn {
			PostEphemeralError(ctx, event, fmt.Sprintf(":no_bell: <@%s> isn't opted-in to use RevChat.", userID))
			continue
		}

		if err := data.UnfollowUser(event.UserID, userID); err != nil {
			logger.From(ctx).Error("failed to unfollow user", slog.Any("error", err),
				slog.String("unfollower_id", event.UserID), slog.String("followed_id", userID))
			PostEphemeralError(ctx, event, fmt.Sprintf("failed to unfollow <@%s>.", userID))
			continue
		}

		done = append(done, userID)
	}

	if len(done) == 0 {
		return nil
	}

	msg := fmt.Sprintf("You will no longer be added to channels for PRs authored by: <@%s>.", strings.Join(done, ">, <@"))
	return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}
