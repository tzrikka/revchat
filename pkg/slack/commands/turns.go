package commands

import (
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack/activities"
	"github.com/tzrikka/revchat/pkg/users"
)

func commonTurnData(ctx workflow.Context, event SlashCommandEvent) (string, []string, data.User, error) {
	url := prDetailsFromChannel(ctx, event)
	if url == nil {
		return "", nil, data.User{}, nil // Not a server error as far as we're concerned.
	}

	emails, err := data.GetCurrentTurn(url[0])
	if err != nil {
		logger.From(ctx).Error("failed to get current turn for PR",
			slog.Any("error", err), slog.String("pr_url", url[0]))
		PostEphemeralError(ctx, event, "failed to read internal data about the PR.")
		return "", nil, data.User{}, err
	}

	user, err := data.SelectUserBySlackID(event.UserID)
	if err != nil {
		logger.From(ctx).Error("failed to load user by Slack ID",
			slog.Any("error", err), slog.String("user_id", event.UserID))
		PostEphemeralError(ctx, event, "failed to read internal data about you.")
		return "", nil, data.User{}, err
	}

	return url[0], emails, user, nil
}

func MyTurn(ctx workflow.Context, event SlashCommandEvent) error {
	url, emails, user, err := commonTurnData(ctx, event)
	if err != nil {
		return err
	}
	if url == "" {
		return nil
	}

	// If this is a no-op, inform the user.
	if slices.Contains(emails, user.Email) {
		msg := whoseTurnText(ctx, emails, user, " already")
		return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
	}

	msg := "Thanks for letting me know!\n\n"

	ok, err := data.Nudge(url, user.Email)
	if err != nil {
		logger.From(ctx).Error("failed to self-nudge", slog.Any("error", err),
			slog.String("pr_url", url), slog.String("email", user.Email))
		PostEphemeralError(ctx, event, "failed to write internal data about this PR.")
	}
	if !ok {
		msg = ":thinking_face: I didn't think you're supposed to review this PR, thanks for letting me know!\n\n"

		if err := data.AddReviewerToPR(url, user.Email); err != nil {
			logger.From(ctx).Error("failed to add reviewer to PR", slog.Any("error", err),
				slog.String("pr_url", url), slog.String("email", user.Email))
			PostEphemeralError(ctx, event, "failed to write internal data about this.")
		}
	}

	emails, err = data.GetCurrentTurn(url)
	if err != nil {
		logger.From(ctx).Error("failed to get current turn for PR after switching",
			slog.Any("error", err), slog.String("pr_url", url))
		PostEphemeralError(ctx, event, "failed to read internal data about this PR.")
		return err
	}

	msg += whoseTurnText(ctx, emails, user, " now")
	return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}

func NotMyTurn(ctx workflow.Context, event SlashCommandEvent) error {
	url, currentTurn, user, err := commonTurnData(ctx, event)
	if err != nil {
		return err
	}
	if url == "" {
		return nil
	}

	// If this is a no-op, inform the user.
	if !slices.Contains(currentTurn, user.Email) {
		msg := ":joy: I didn't think it's your turn anyway!\n\n" + whoseTurnText(ctx, currentTurn, user, "")
		return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
	}

	if err := data.SwitchTurn(url, user.Email); err != nil {
		logger.From(ctx).Error("failed to switch PR turn", slog.Any("error", err),
			slog.String("pr_url", url), slog.String("email", user.Email))
		PostEphemeralError(ctx, event, "failed to write internal data about this PR.")
		return err
	}

	newTurn, err := data.GetCurrentTurn(url)
	if err != nil {
		logger.From(ctx).Error("failed to get current turn for PR after switching",
			slog.Any("error", err), slog.String("pr_url", url))
		PostEphemeralError(ctx, event, "failed to read internal data about this PR.")
		return err
	}

	msg := "Thanks for letting me know!\n\n" + whoseTurnText(ctx, newTurn, user, " now")
	return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}

func FreezeTurns(ctx workflow.Context, event SlashCommandEvent) error {
	url := prDetailsFromChannel(ctx, event)
	if url == nil {
		return nil // Not a server error as far as we're concerned.
	}

	ok, err := data.FreezeTurn(url[0], users.SlackIDToEmail(ctx, event.UserID))
	if err != nil {
		logger.From(ctx).Error("failed to freeze PR turn", slog.Any("error", err), slog.String("pr_url", url[0]))
		PostEphemeralError(ctx, event, "failed to write internal data about this PR.")
		return err
	}

	msg := ":snowflake: Turn switching is now frozen in this PR."
	if !ok {
		msg = ":snowflake: Turn switching is already frozen in this PR."
	}
	return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}

func UnfreezeTurns(ctx workflow.Context, event SlashCommandEvent) error {
	url := prDetailsFromChannel(ctx, event)
	if url == nil {
		return nil // Not a server error as far as we're concerned.
	}

	ok, err := data.UnfreezeTurn(url[0])
	if err != nil {
		logger.From(ctx).Error("failed to unfreeze PR turn", slog.Any("error", err), slog.String("pr_url", url[0]))
		PostEphemeralError(ctx, event, "failed to write internal data about this PR.")
		return err
	}

	msg := ":sunny: Turn switching is now unfrozen in this PR."
	if !ok {
		msg = ":sunny: Turn switching is already unfrozen in this PR."
	}
	return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}

func WhoseTurn(ctx workflow.Context, event SlashCommandEvent) error {
	url, emails, user, err := commonTurnData(ctx, event)
	if err != nil {
		return err
	}
	if url == "" {
		return nil
	}

	msg := whoseTurnText(ctx, emails, user, "")
	return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}

// whoseTurnText builds a "whose turn is it" summary message, reused by multiple slash commands.
func whoseTurnText(ctx workflow.Context, emails []string, user data.User, tweak string) string {
	// If the user who ran the command is in the list, highlight that to them.
	var msg strings.Builder
	i := slices.Index(emails, user.Email)
	if i > -1 {
		msg.WriteString(":eyes: ")
	}

	msg.WriteString("I think it's")
	msg.WriteString(tweak)

	comma := false
	if i > -1 {
		msg.WriteString(" *your* turn")
		emails = slices.Delete(emails, i, i+1)
		if len(emails) > 0 {
			msg.WriteString(", along with")
			comma = true
		}
	} else {
		msg.WriteString(" the turn of")
	}

	for _, email := range emails {
		u, err := data.SelectUserByEmail(email)
		if err != nil {
			logger.From(ctx).Error("failed to load user by email", slog.Any("error", err), slog.String("email", email))
			msg.WriteString(fmt.Sprintf(" `%s`", email))
			continue
		}
		msg.WriteString(fmt.Sprintf(" <@%s>", u.SlackID))
	}

	if comma {
		msg.WriteString(",")
	}
	msg.WriteString(" to review this PR.")

	// if i > -1 {
	// 	msg.WriteString("\n\nYou haven't reviewed this PR yet.")
	// 	msg.WriteString("\n\nIt's been `TODO` since your last activity in this PR.")
	// }

	return msg.String()
}
