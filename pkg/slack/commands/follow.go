package commands

import (
	"fmt"
	"slices"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack/activities"
)

func Follow(ctx workflow.Context, event SlashCommandEvent) error {
	users := extractFollowedUsers(ctx, event)
	if len(users) == 0 {
		return nil
	}

	done := make([]string, 0, len(users))
	for _, userID := range users {
		if !checkFollowedUser(ctx, event, userID) {
			continue
		}
		if !data.FollowUser(ctx, event.UserID, userID) {
			PostEphemeralError(ctx, event, fmt.Sprintf("failed to follow <@%s>.", userID))
			continue
		}
		done = append(done, userID)
	}

	return reportFollowResults(ctx, event, done, "now")
}

func Unfollow(ctx workflow.Context, event SlashCommandEvent) error {
	users := extractFollowedUsers(ctx, event)
	if len(users) == 0 {
		return nil
	}

	done := make([]string, 0, len(users))
	for _, userID := range users {
		if !checkFollowedUser(ctx, event, userID) {
			continue
		}
		if !data.UnfollowUser(ctx, event.UserID, userID) {
			PostEphemeralError(ctx, event, fmt.Sprintf("failed to unfollow <@%s>.", userID))
			continue
		}
		done = append(done, userID)
	}

	return reportFollowResults(ctx, event, done, "no longer")
}

func extractFollowedUsers(ctx workflow.Context, event SlashCommandEvent) []string {
	// Ensure that the calling user is opted-in, i.e. has authorized RevChat & is allowed to join PR channels.
	_, optedIn, err := UserDetails(ctx, event, event.UserID)
	if err != nil {
		return nil // Not a server error as far as we're concerned.
	}
	if !optedIn {
		PostEphemeralError(ctx, event, "you need to opt-in first.")
		return nil // Not a server error as far as we're concerned.
	}

	ids := extractAtLeastOneUserID(ctx, event)
	i := slices.Index(ids, event.UserID)
	if i == -1 {
		return ids
	}

	// Remove the calling user from the list of users to un/follow.
	ids = slices.Delete(ids, i, i+1)
	if len(ids) == 0 {
		PostEphemeralError(ctx, event, "you can't un/follow yourself.")
		return nil
	}
	return ids
}

func checkFollowedUser(ctx workflow.Context, event SlashCommandEvent, userID string) bool {
	_, optedIn, err := UserDetails(ctx, event, userID)
	if err != nil {
		return false
	}
	if !optedIn {
		PostEphemeralError(ctx, event, fmt.Sprintf(":no_bell: <@%s> isn't opted-in to use RevChat.", userID))
		return false
	}

	return true
}

func reportFollowResults(ctx workflow.Context, event SlashCommandEvent, users []string, action string) error {
	if len(users) == 0 {
		return nil
	}

	msg := fmt.Sprintf("You will %s be added to channels for PRs authored by: <@%s>.", action, strings.Join(users, ">, <@"))
	return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}
