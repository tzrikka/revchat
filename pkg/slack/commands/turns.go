package commands

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack/activities"
	"github.com/tzrikka/revchat/pkg/users"
)

func commonTurnData(ctx workflow.Context, event SlashCommandEvent) (string, []string, data.User, error) {
	url, err := prDetailsFromChannel(ctx, event)
	if url == nil {
		return "", nil, data.User{}, err // The error may or may not be nil.
	}

	emails, err := data.GetCurrentTurn(ctx, url[0])
	if err != nil {
		PostEphemeralError(ctx, event, "failed to read internal data about the PR.")
		return "", nil, data.User{}, err
	}

	user, _, err := UserDetails(ctx, event, event.UserID)
	if err != nil {
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
		return nil // Not a server error as far as we're concerned.
	}

	// If this is a no-op, inform the user.
	if slices.Contains(emails, user.Email) {
		msg := whoseTurnText(ctx, emails, user, " already")
		return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
	}

	msg := "Thanks for letting me know!\n\n"

	ok, _, err := data.Nudge(ctx, url, user.Email)
	if err != nil {
		PostEphemeralError(ctx, event, "failed to write internal data about this PR.")
	}
	if !ok {
		msg = ":thinking_face: I didn't think you're supposed to review this PR, thanks for letting me know!\n\n"

		if err := data.AddReviewerToTurns(ctx, url, user.Email); err != nil {
			PostEphemeralError(ctx, event, "failed to write internal data about this.")
		}
	}

	emails, err = data.GetCurrentTurn(ctx, url)
	if err != nil {
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
		return nil // Not a server error as far as we're concerned.
	}

	// If this is a no-op, inform the user.
	if !slices.Contains(currentTurn, user.Email) {
		msg := ":joy: I didn't think it's your turn anyway!\n\n" + whoseTurnText(ctx, currentTurn, user, "")
		return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
	}

	if err := data.SwitchTurn(ctx, url, user.Email, true); err != nil {
		PostEphemeralError(ctx, event, "failed to write internal data about this PR.")
		return err
	}

	newTurn, err := data.GetCurrentTurn(ctx, url)
	if err != nil {
		PostEphemeralError(ctx, event, "failed to read internal data about this PR.")
		return err
	}

	msg := "Thanks for letting me know!\n\n" + whoseTurnText(ctx, newTurn, user, " now")
	return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}

func FreezeTurns(ctx workflow.Context, event SlashCommandEvent) error {
	url, err := prDetailsFromChannel(ctx, event)
	if url == nil {
		return err // May or may not be nil.
	}

	ok, err := data.FreezeTurns(ctx, url[0], users.SlackIDToEmail(ctx, event.UserID))
	if err != nil {
		PostEphemeralError(ctx, event, "failed to write internal data about this PR.")
		return err
	}

	// Also switch turns to the user who froze them (if possible), but ignore errors.
	_, _, _ = data.Nudge(ctx, url[0], users.SlackIDToEmail(ctx, event.UserID))

	msg := ":snowflake: Turn switching is now frozen in this PR."
	if !ok {
		msg = ":snowflake: Turn switching is already frozen in this PR."
	}
	return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}

func UnfreezeTurns(ctx workflow.Context, event SlashCommandEvent) error {
	url, err := prDetailsFromChannel(ctx, event)
	if url == nil {
		return err // May or may not be nil.
	}

	ok, err := data.UnfreezeTurns(ctx, url[0])
	if err != nil {
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
		return nil // Not a server error as far as we're concerned.
	}

	msg := whoseTurnText(ctx, emails, user, "")

	if at, by := data.Frozen(ctx, url); !at.IsZero() {
		id := fmt.Sprintf("<@%s>", users.EmailToSlackID(ctx, by))
		if id == "<@>" {
			id = by
		}
		unix := at.Unix()
		dt := at.Format(time.DateTime)
		msg = fmt.Sprintf("%s\n\n:snowflake: Turn switching was frozen by %s <!date^%d^{ago}|at %s UTC>.", msg, id, unix, dt)
	}

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

	withOthers := false
	if i == -1 {
		msg.WriteString(" the turn of ")
	} else {
		emails = slices.Delete(emails, i, i+1)
		msg.WriteString(" *your* turn")
		if len(emails) > 0 {
			msg.WriteString(" - along with ")
			withOthers = true
		}
	}

	for j, email := range emails {
		if j > 0 {
			msg.WriteString(", ")
		}
		switch user := data.SelectUserByEmail(ctx, email); {
		case user.SlackID != "":
			msg.WriteString("<@" + user.SlackID + ">")
		case user.RealName != "":
			msg.WriteString(user.RealName)
		default:
			msg.WriteString(email)
		}
	}

	if withOthers {
		msg.WriteString(" -")
	}
	msg.WriteString(" to review this PR.")

	// if i > -1 {
	// 	msg.WriteString("\n\nYou haven't reviewed this PR yet.")
	// 	msg.WriteString("\n\nIt's been `TODO` since your last activity in this PR.")
	// }

	return msg.String()
}
