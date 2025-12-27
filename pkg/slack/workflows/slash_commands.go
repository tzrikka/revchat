package workflows

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/slack/commands"
)

var userCommandsPattern = regexp.MustCompile(`^(follow|unfollow|invite|nudge|ping|poke)`)

// SlashCommandWorkflow routes user command events to their respective handlers in the [commands] package:
//   - https://docs.slack.dev/apis/events-api/using-socket-mode#command
//   - https://docs.slack.dev/interactivity/implementing-slash-commands#app_command_handling
func (c *Config) SlashCommandWorkflow(ctx workflow.Context, event commands.SlashCommandEvent) error {
	event.Text = strings.ToLower(event.Text)
	switch event.Text {
	case "", "help":
		return commands.Help(ctx, event)

	case "opt-in", "opt in", "optin":
		return c.OptInSlashCommand(ctx, event)
	case "opt-out", "opt out", "optout":
		return c.OptOutSlashCommand(ctx, event)

	case "clean":
		return commands.Clean(ctx, event)
	case "explain":
		return commands.Explain(ctx, event)
	case "stat", "state", "status":
		return commands.Status(ctx, event)

	case "who", "whose", "whose turn":
		return commands.WhoseTurn(ctx, event)
	case "my turn":
		return commands.MyTurn(ctx, event)
	case "not my turn":
		return commands.NotMyTurn(ctx, event)
	case "freeze", "freeze turn", "freeze turns":
		return commands.FreezeTurns(ctx, event)
	case "unfreeze", "unfreeze turn", "unfreeze turns":
		return commands.UnfreezeTurns(ctx, event)

	case "approve", "lgtm", "+1":
		return commands.Approve(ctx, event, c.BitbucketWorkspace)
	case "unapprove", "-1":
		return commands.Unapprove(ctx, event, c.BitbucketWorkspace)
	}

	if cmd := userCommandsPattern.FindStringSubmatch(event.Text); cmd != nil {
		switch cmd[1] {
		case "follow":
			return commands.Follow(ctx, event)
		case "unfollow":
			return commands.Unfollow(ctx, event)
		case "invite":
			return commands.Invite(ctx, event)
		case "nudge", "ping", "poke":
			return commands.Nudge(ctx, event, c.ThrippyHTTPAddress)
		}
	}
	if commands.RemindersSyntax.MatchString(event.Text) {
		return commands.Reminders(ctx, event)
	}

	logger.From(ctx).Warn("unrecognized Slack slash command",
		slog.String("username", event.UserName), slog.String("text", event.Text))
	commands.PostEphemeralError(ctx, event, fmt.Sprintf("unrecognized command - try `%s help`", event.Command))
	return nil
}
