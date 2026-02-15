package commands

import (
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/slack/activities"
)

func Help(ctx workflow.Context, event SlashCommandEvent) error {
	var cmds strings.Builder

	cmds.WriteString(":wave: Available general commands:\n")
	cmds.WriteString("\n  •   `%s opt-in` - opt into being added to PR channels and receiving DMs")
	cmds.WriteString("\n  •   `%s opt-out` - opt out of being added to PR channels and receiving DMs")
	cmds.WriteString("\n  •   `%s reminders at <time in 12h/24h format>` - weekdays, using your timezone")
	cmds.WriteString("\n  •   `%s follow <1 or more @users or @groups>` - auto add yourself to PRs they create")
	cmds.WriteString("\n  •   `%s unfollow <1 or more @users or @groups>` - stop following their PR channels")
	cmds.WriteString("\n  •   `%s status` - all the PRs you need to look at, as an author or a reviewer")
	cmds.WriteString("\n\nMore commands inside PR channels:\n")
	cmds.WriteString("\n  •   `%s who` / `whose turn` / `my turn` / `not my turn` / `[un]freeze [turns]`")
	cmds.WriteString("\n  •   `%s nudge <1 or more @users or @groups>` / `ping <...>` / `poke <...>`")
	cmds.WriteString("\n  •   `%s explain` - who needs to approve each file, and have they?")
	cmds.WriteString("\n  •   `%s clean` - remove unnecessary reviewers from the PR")
	cmds.WriteString("\n  •   `%s approve` or `lgtm` or `+1`")
	cmds.WriteString("\n  •   `%s unapprove` or `-1`")

	msg := strings.ReplaceAll(cmds.String(), "%s", event.Command)
	return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}
