package commands

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack/activities"
	"github.com/tzrikka/revchat/pkg/users"
)

func commonTurnData(ctx workflow.Context, opts client.Options, event SlashCommandEvent) (string, []string, data.User, error) {
	url, err := prDetailsFromChannel(ctx, event)
	if url == nil {
		return "", nil, data.User{}, err // The error may or may not be nil.
	}

	emails, err := data.LoadCurrentTurnEmails(ctx, opts, url[0])
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

func MyTurn(ctx workflow.Context, opts client.Options, event SlashCommandEvent) error {
	url, emails, user, err := commonTurnData(ctx, opts, event)
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

	ok, _, err := data.SetReviewerTurn(ctx, opts, url, user.Email, true)
	if err != nil {
		PostEphemeralError(ctx, event, "failed to write internal data about this PR.")
	}
	if !ok {
		msg = ":thinking_face: I didn't think you're supposed to review this PR, thanks for letting me know!\n\n"
		if _, _, err := data.SetReviewerTurn(ctx, opts, url, user.Email, false); err != nil {
			PostEphemeralError(ctx, event, "failed to write internal data about this.")
		}
	}

	emails, err = data.LoadCurrentTurnEmails(ctx, opts, url)
	if err != nil {
		PostEphemeralError(ctx, event, "failed to read internal data about this PR.")
		return err
	}

	msg += whoseTurnText(ctx, emails, user, " now")
	return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}

func NotMyTurn(ctx workflow.Context, opts client.Options, event SlashCommandEvent) error {
	url, currentTurn, user, err := commonTurnData(ctx, opts, event)
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

	if err := data.SwitchTurn(ctx, opts, url, user.Email, true); err != nil {
		PostEphemeralError(ctx, event, "failed to write internal data about this PR.")
		return err
	}

	newTurn, err := data.LoadCurrentTurnEmails(ctx, opts, url)
	if err != nil {
		PostEphemeralError(ctx, event, "failed to read internal data about this PR.")
		return err
	}

	msg := "Thanks for letting me know!\n\n" + whoseTurnText(ctx, newTurn, user, " now")
	return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}

func FreezeTurns(ctx workflow.Context, opts client.Options, event SlashCommandEvent) error {
	url, err := prDetailsFromChannel(ctx, event)
	if url == nil {
		return err // May or may not be nil.
	}

	ok, err := data.FreezeTurns(ctx, opts, url[0], users.SlackIDToEmail(ctx, event.UserID))
	if err != nil {
		PostEphemeralError(ctx, event, "failed to write internal data about this PR.")
		return err
	}

	// Also switch turns to the user who froze them (if possible), but ignore errors.
	_, _, err = data.SetReviewerTurn(ctx, opts, url[0], users.SlackIDToEmail(ctx, event.UserID), true)

	msg := ":snowflake: Turn switching is now frozen in this PR."
	if !ok {
		msg = ":snowflake: Turn switching is already frozen in this PR."
	}
	return errors.Join(err, activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg))
}

func UnfreezeTurns(ctx workflow.Context, opts client.Options, event SlashCommandEvent) error {
	url, err := prDetailsFromChannel(ctx, event)
	if url == nil {
		return err // May or may not be nil.
	}

	ok, err := data.UnfreezeTurns(ctx, opts, url[0])
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

func WhoseTurn(ctx workflow.Context, opts client.Options, event SlashCommandEvent) error {
	url, emails, user, err := commonTurnData(ctx, opts, event)
	if err != nil {
		return err
	}
	if url == "" {
		return nil // Not a server error as far as we're concerned.
	}

	msg := whoseTurnText(ctx, emails, user, "")

	if at, by := data.IsFrozen(ctx, opts, url); !at.IsZero() {
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
// The emails slice must be deduped, but may contain invalid/bot email addresses (which are removed).
func whoseTurnText(ctx workflow.Context, emails []string, user data.User, tweak string) string {
	// Ignore invalid/bot email addresses.
	if i := slices.Index(emails, ""); i > -1 {
		emails = slices.Delete(emails, i, i+1)
	}
	if i := slices.Index(emails, "bot"); i > -1 {
		emails = slices.Delete(emails, i, i+1)
	}

	// If the user who ran the command is in the list, highlight that to them.
	var msg strings.Builder
	i := slices.Index(emails, user.Email)
	if i > -1 {
		msg.WriteString(":eyes: ")
	}

	msg.WriteString("I think it's")
	msg.WriteString(tweak)

	withOthers := false
	switch {
	case i == -1 && len(emails) == 0:
		msg.WriteString(" no one's turn")
	case i == -1 && len(emails) > 0:
		msg.WriteString(" the turn of ")
	default:
		msg.WriteString(" *your* turn")
		emails = slices.Delete(emails, i, i+1)
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
