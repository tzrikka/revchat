package slack

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/timpani-api/pkg/bitbucket"
	"github.com/tzrikka/timpani-api/pkg/slack"
)

const (
	DefaultReminderTime = "8:00AM"
)

var (
	bitbucketURLPattern = regexp.MustCompile(`^https://[^/]+/([\w-]+)/([\w-]+)/pull-requests/(\d+)`)
	remindersPattern    = regexp.MustCompile(`^reminders?(\s+at)?\s+([0-9:]+)\s*(am|pm|a|p)?`)
	userCommandsPattern = regexp.MustCompile(`^(follow|unfollow|invite|nudge|ping|poke|set turn to)`)
	userIDsPattern      = regexp.MustCompile(`<(@)(\w+)(\|[^>]*)?>`)
	userOrTeamIDPattern = regexp.MustCompile(`<(@|!subteam\^)(\w+)(\|[^>]*)?>`)
)

// https://docs.slack.dev/interactivity/implementing-slash-commands#app_command_handling
// https://docs.slack.dev/apis/events-api/using-socket-mode#command
func (c *Config) slashCommandWorkflow(ctx workflow.Context, event SlashCommandEvent) error {
	event.Text = strings.ToLower(event.Text)
	switch event.Text {
	case "", "help":
		return helpSlashCommand(ctx, event)
	case "opt-in", "opt in", "optin":
		return c.optInSlashCommand(ctx, event)
	case "opt-out", "opt out", "optout":
		return c.optOutSlashCommand(ctx, event)
	case "stat", "state", "status":
		return statusSlashCommand(ctx, event)
	case "who", "whose", "whose turn":
		return whoseTurnSlashCommand(ctx, event)
	case "my turn":
		return myTurnSlashCommand(ctx, event)
	case "not my turn":
		return notMyTurnSlashCommand(ctx, event)
	case "approve", "lgtm", "+1":
		return c.approveSlashCommand(ctx, event)
	case "unapprove", "-1":
		return c.unapproveSlashCommand(ctx, event)
	case "update", "update channel":
		return updateChannelSlashCommand(ctx, event)
	}

	if remindersPattern.MatchString(event.Text) {
		return remindersSlashCommand(ctx, event)
	}
	if cmd := userCommandsPattern.FindStringSubmatch(event.Text); cmd != nil {
		switch cmd[1] {
		case "follow":
			return followSlashCommand(ctx, event)
		case "unfollow":
			return unfollowSlashCommand(ctx, event)
		case "invite":
			return inviteSlashCommand(ctx, event)
		case "nudge", "ping", "poke":
			return nudgeSlashCommand(ctx, event)
		default:
			return setTurnSlashCommand(ctx, event)
		}
	}

	log.Warn(ctx, "unrecognized Slack slash command", "username", event.UserName, "text", event.Text)
	postEphemeralError(ctx, event, fmt.Sprintf("unrecognized command - try `%s help`", event.Command))
	return nil
}

func helpSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	msg := ":wave: Available slash commands for `%s`:\n\n"
	msg += "  •  `%s opt-in` - opt into being added to PR channels and receiving DMs\n"
	msg += "  •  `%s opt-out` - opt out of being added to PR channels and receiving DMs\n"
	msg += "  •  `%s reminders at <time in 12h/24h format>` (weekdays, using your timezone)\n"
	msg += "  •  `%s status` - show your current PR status, as an author and reviewer\n\n"
	msg += "More commands inside PR channels:\n\n"
	msg += "  •  `%s who` / `whose turn` / `my turn` / `not my turn` / `set turn to <1 or more @users>`\n"
	msg += "  •  `%s nudge <1 or more @users>` / `ping <1 or more @users>` / `poke <...>`\n"
	msg += "  •  `%s approve` or `lgtm` or `+1`\n"
	msg += "  •  `%s unapprove` or `-1`\n"
	msg += "  •  `%s update channel`\n"
	msg = strings.ReplaceAll(msg, "%s", event.Command)

	return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}

func extractAtLeastOneUserID(ctx workflow.Context, event SlashCommandEvent, pattern *regexp.Regexp) ([]string, map[string]bool) {
	matches := pattern.FindAllStringSubmatch(event.Text, -1)
	if len(matches) == 0 {
		postEphemeralError(ctx, event, "you need to mention at least one `@user`.")
		return nil, nil
	}

	var ids []string
	for _, match := range matches {
		ids = append(ids, strings.ToUpper(match[2]))
	}

	// The returned map is intentionally empty - the caller is responsible for populating it.
	return ids, make(map[string]bool, len(ids))
}

func followSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	// Ensure that the calling user is opted-in, i.e. authorized us & allowed to join PR channels.
	_, optedIn, err := userDetails(ctx, event, event.UserID)
	if err != nil {
		return nil // Not a *server* error as far as we're concerned.
	}
	if !optedIn {
		postEphemeralError(ctx, event, "you need to opt-in first.")
		return nil // Not a *server* error as far as we're concerned.
	}

	users, _ := extractAtLeastOneUserID(ctx, event, userOrTeamIDPattern)
	for _, userID := range expandSubteams(ctx, users) {
		_, optedIn, err := userDetails(ctx, event, userID)
		if err != nil {
			continue
		}
		if !optedIn {
			postEphemeralError(ctx, event, fmt.Sprintf("<@%s> isn't opted-in yet.", userID))
			continue
		}
	}

	return nil
}

func unfollowSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	postEphemeralError(ctx, event, "this command is not implemented yet.")
	return nil
}

func expandSubteams(_ workflow.Context, ids []string) []string {
	if len(ids) == 0 {
		return nil
	}

	expanded := make([]string, 0, len(ids))
	for _, id := range ids {
		if !strings.HasPrefix(id, "S") {
			expanded = append(expanded, id)
			continue
		}
	}

	return expanded
}

func inviteSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	users, sent := extractAtLeastOneUserID(ctx, event, userIDsPattern)
	if len(users) == 0 {
		return nil // Not a *server* error as far as we're concerned.
	}

	for _, userID := range users {
		// Avoid duplicate invitations, and check that the user isn't already opted-in.
		if sent[userID] {
			continue
		}

		_, optedIn, err := userDetails(ctx, event, userID)
		if optedIn {
			msg := fmt.Sprintf(":bell: <@%s> is already opted-in.", userID)
			_ = PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
		}
		if err != nil || optedIn {
			continue
		}

		msg := ":wave: <@%s> is inviting you to use RevChat. Please run this slash command:\n\n```%s opt-in```"
		if _, err := PostMessage(ctx, userID, fmt.Sprintf(msg, event.UserID, event.Command)); err != nil {
			postEphemeralError(ctx, event, fmt.Sprintf("failed to send an invite to <@%s>.", userID))
			continue
		}
		sent[userID] = true
	}

	if len(sent) == 0 {
		return nil
	}
	msg := fmt.Sprintf("Sent invites to: <@%s>", strings.Join(slices.Sorted(maps.Keys(sent)), ">, <@"))
	return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}

func nudgeSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	url := prDetailsFromChannel(ctx, event)
	if url == nil {
		return nil // Not a *server* error as far as we're concerned.
	}

	users, sent := extractAtLeastOneUserID(ctx, event, userIDsPattern)
	if len(users) == 0 {
		return nil // Not a *server* error as far as we're concerned.
	}

	for _, userID := range users {
		// Avoid duplicate nudges, and check that the user is eligible to be nudged.
		if sent[userID] || !checkUserBeforeNudging(ctx, event, url[0], userID) {
			continue
		}

		msg := ":pleading_face: <@%s> is asking you to review <#%s> :pray:"
		if _, err := PostMessage(ctx, userID, fmt.Sprintf(msg, event.UserID, event.ChannelID)); err != nil {
			postEphemeralError(ctx, event, fmt.Sprintf("failed to send a nudge to <@%s>.", userID))
			continue
		}
		sent[userID] = true
	}

	if len(sent) == 0 {
		return nil
	}
	msg := fmt.Sprintf("Sent nudges to: <@%s>", strings.Join(slices.Sorted(maps.Keys(sent)), ">, <@"))
	return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}

// checkUserBeforeNudging ensures that the user exists, is opted-in, and is a reviewer of the PR.
func checkUserBeforeNudging(ctx workflow.Context, event SlashCommandEvent, url, userID string) bool {
	user, optedIn, err := userDetails(ctx, event, userID)
	if err != nil {
		return false
	}
	if !optedIn {
		msg := fmt.Sprintf(":no_bell: <@%s> is not opted-in to use RevChat.", userID)
		_ = PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
		return false
	}

	ok, err := data.Nudge(url, user.Email)
	if err != nil {
		log.Error(ctx, "failed to nudge user", "error", err, "pr_url", url, "user_id", userID)
		postEphemeralError(ctx, event, fmt.Sprintf("internal data error while nudging <@%s>.", userID))
		return ok // May be true: valid reviewer, but failed to save it.
	}
	if !ok {
		msg := fmt.Sprintf(":no_good: <@%s> is not a tracked reviewer of this PR.", userID)
		_ = PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
	}
	return ok
}

func (c *Config) optInSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	user, optedIn, err := userDetails(ctx, event, event.UserID)
	if err != nil {
		return err
	}
	if optedIn {
		return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, ":bell: You're already opted in.")
	}

	user.SlackID = event.UserID // Ensure the user's Slack ID is set even if the user is unrecognized.

	switch {
	case c.bitbucketWorkspace != "":
		return c.optInBitbucket(ctx, event, user)
	default:
		log.Error(ctx, "neither Bitbucket nor GitHub are configured")
		postEphemeralError(ctx, event, "internal configuration error.")
		return errors.New("neither Bitbucket nor GitHub are configured")
	}
}

func (c *Config) optInBitbucket(ctx workflow.Context, event SlashCommandEvent, user data.User) error {
	linkID, nonce, err := c.createThrippyLink(ctx)
	if err != nil {
		log.Error(ctx, "failed to create Thrippy link for Slack user", "error", err)
		postEphemeralError(ctx, event, "internal authorization failure.")
		return err
	}

	msg := ":point_right: <https://%s/start?id=%s&nonce=%s|Click here> to authorize RevChat to act on your behalf in Bitbucket."
	msg = fmt.Sprintf(msg, c.thrippyHTTPAddress, linkID, nonce)
	if err := PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg); err != nil {
		log.Error(ctx, "failed to post ephemeral opt-in message in Slack", "error", err)
		_ = c.deleteThrippyLink(ctx, linkID)
		return err
	}

	err = c.waitForThrippyLinkCreds(ctx, linkID)
	if err != nil {
		_ = c.deleteThrippyLink(ctx, linkID)
		if err.Error() == ErrLinkAuthzTimeout { // For some reason errors.Is() doesn't work across Temporal.
			log.Warn(ctx, "user did not complete Thrippy OAuth flow in time", "email", user.Email)
			postEphemeralError(ctx, event, "Bitbucket authorization timed out - please try opting in again.")
			return nil // Not a *server* error as far as we're concerned.
		} else {
			log.Error(ctx, "failed to authorize Bitbucket user", "error", err, "email", user.Email)
			postEphemeralError(ctx, event, "failed to authorize you in Bitbucket.")
			return err
		}
	}

	if err := data.UpsertUser("", "", "", user.SlackID, "", "", linkID); err != nil {
		log.Error(ctx, "failed to opt-in user", "error", err, "email", user.Email)
		postEphemeralError(ctx, event, "failed to write internal data about you.")
		_ = c.deleteThrippyLink(ctx, linkID)
		return err
	}

	if err := setReminder(ctx, event, DefaultReminderTime, true); err != nil {
		_ = c.deleteThrippyLink(ctx, linkID)
		return err
	}

	msg = ":bell: You are now opted into using RevChat.\n\n"
	msg += ":alarm_clock: Default time for weekday reminders = *8 AM* (in your current timezone). "
	msg += "To change it, run this slash command:\n\n```%s reminders at <time in 12h or 24h format>```"
	return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, fmt.Sprintf(msg, event.Command))
}

func (c *Config) optOutSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	user, optedIn, err := userDetails(ctx, event, event.UserID)
	if err != nil {
		return err
	}
	if !optedIn {
		return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, ":no_bell: You're already opted out.")
	}

	if err := data.UpsertUser(user.Email, user.BitbucketID, user.GitHubID, user.SlackID, "", "", "X"); err != nil {
		log.Error(ctx, "failed to opt-out user", "error", err, "email", user.Email)
		postEphemeralError(ctx, event, "failed to write internal data about you.")
		return err
	}

	if err := c.deleteThrippyLink(ctx, user.ThrippyLink); err != nil {
		log.Error(ctx, "failed to delete Thrippy link for opted-out user", "error", err, "link_id", user.ThrippyLink)
		// This is an internal error, it doesn't concern or affect the user.
	}

	msg := ":no_bell: You are now opted out of using RevChat for new PRs."
	return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}

func remindersSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	matches := remindersPattern.FindStringSubmatch(event.Text)
	if len(matches) < 3 {
		log.Error(ctx, "failed to parse reminders slash command - regex mismatch", "text", event.Text)
		postEphemeralError(ctx, event, "unexpected internal error while parsing command.")
		return errors.New("failed to parse reminders command - regex mismatch")
	}

	amPm := ""
	if len(matches) > 3 {
		amPm = matches[3]
	}

	kitchenTime, err := normalizeTime(matches[2], amPm)
	if err != nil {
		log.Warn(ctx, "failed to parse time in reminders slash command", "error", err)
		postEphemeralError(ctx, event, err.Error())
		return nil // Not a *server* error as far as we're concerned.
	}

	if kt, _ := time.Parse(time.Kitchen, kitchenTime); kt.Minute() != 0 && kt.Minute() != 30 {
		log.Warn(ctx, "uncommon reminder time requested", "user_id", event.UserID, "time", kitchenTime)
		postEphemeralError(ctx, event, "please specify a time on the hour or half-hour.")
		return nil // Not a *server* error as far as we're concerned.
	}

	return setReminder(ctx, event, kitchenTime, false)
}

func setReminder(ctx workflow.Context, event SlashCommandEvent, t string, quiet bool) error {
	user, err := slack.UsersInfo(ctx, event.UserID)
	if err != nil {
		log.Error(ctx, "failed to retrieve Slack user info", "error", err, "user_id", event.UserID)
		postEphemeralError(ctx, event, "failed to retrieve Slack user info.")
		return err
	}

	if user.TZ == "" {
		log.Warn(ctx, "Slack user has no timezone in their profile", "user_id", event.UserID)
		postEphemeralError(ctx, event, "please set a timezone in your Slack profile preferences first.")
		return nil // Not a *server* error as far as we're concerned.
	}

	if _, err := time.LoadLocation(user.TZ); err != nil {
		log.Warn(ctx, "unrecognized user timezone", "error", err, "user_id", event.UserID, "tz", user.TZ)
		postEphemeralError(ctx, event, fmt.Sprintf("your Slack timezone is unrecognized: `%s`", user.TZ))
		return err
	}

	if err := data.SetReminder(event.UserID, t, user.TZ); err != nil {
		log.Error(ctx, "failed to store user reminder time", "error", err, "user_id", event.UserID, "time", t, "zone", user.TZ)
		postEphemeralError(ctx, event, "failed to write internal data about you.")
		return err
	}

	if !quiet {
		t = fmt.Sprintf("%s %s", t[:len(t)-2], t[len(t)-2:]) // Insert space before AM/PM suffix.
		msg := fmt.Sprintf(":alarm_clock: Your daily reminder time is set to *%s* _(%s)_ on weekdays.", t, user.TZ)
		return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
	}

	return nil
}

func statusSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	var msg strings.Builder
	prs := loadPRTurns(ctx)[event.UserID]
	slices.Sort(prs)

	if len(prs) == 0 {
		msg.WriteString(":joy: No PRs require your attention at this time!")
	} else {
		msg.WriteString(":eyes: These PRs currently require your attention:")
	}

	for _, url := range prs {
		msg.WriteString(prDetails(ctx, url, event.UserID))
	}

	msg.WriteString("\n\n:information_source: Slash command tips:\n  •  `")
	msg.WriteString(event.Command)
	msg.WriteString(" status` - updated report at any time\n  •  `")
	msg.WriteString(event.Command)
	msg.WriteString(" reminder <time in 12h/24h format>` - change time or timezone\n  •  `")
	msg.WriteString(event.Command)
	msg.WriteString(" who` / `my turn` / `not my turn` - only in RevChat channels")

	return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg.String())
}

func commonTurnData(ctx workflow.Context, event SlashCommandEvent) (string, []string, data.User, error) {
	url := prDetailsFromChannel(ctx, event)
	if url == nil {
		return "", nil, data.User{}, nil // Not a *server* error as far as we're concerned.
	}

	emails, err := data.GetCurrentTurn(url[0])
	if err != nil {
		log.Error(ctx, "failed to get current turn for PR", "error", err, "pr_url", url[0])
		postEphemeralError(ctx, event, "failed to read internal data about the PR.")
		return "", nil, data.User{}, err
	}

	user, err := data.SelectUserBySlackID(event.UserID)
	if err != nil {
		log.Error(ctx, "failed to load user by Slack ID", "error", err, "user_id", event.UserID)
		postEphemeralError(ctx, event, "failed to read internal data about you.")
		return "", nil, data.User{}, err
	}

	return url[0], emails, user, nil
}

func myTurnSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
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
		return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
	}

	msg := "Thanks for letting me know!\n\n"

	ok, err := data.Nudge(url, user.Email)
	if err != nil {
		log.Error(ctx, "failed to self-nudge", "error", err, "pr_url", url, "email", user.Email)
		postEphemeralError(ctx, event, "failed to write internal data about this PR.")
	}
	if !ok {
		msg = ":thinking_face: I didn't think you're supposed to review this PR, thanks for letting me know!\n\n"

		if err := data.AddReviewerToPR(url, user.Email); err != nil {
			log.Error(ctx, "failed to add reviewer to PR", "error", err, "pr_url", url, "email", user.Email)
			postEphemeralError(ctx, event, "failed to write internal data about this.")
		}
	}

	emails, err = data.GetCurrentTurn(url)
	if err != nil {
		log.Error(ctx, "failed to get current turn for PR after switching", "error", err, "pr_url", url)
		postEphemeralError(ctx, event, "failed to read internal data about this PR.")
		return err
	}

	msg += whoseTurnText(ctx, emails, user, " now")
	return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}

func notMyTurnSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	url, currentTurn, user, err := commonTurnData(ctx, event)
	if err != nil {
		return err
	}
	if url == "" {
		return nil
	}

	// If this is a no-op, inform the user.
	if !slices.Contains(currentTurn, user.Email) {
		msg := ":joy: I didn't think it's your turn anyway!\n\n" + whoseTurnText(ctx, currentTurn, user, " already")
		return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
	}

	if err := data.SwitchTurn(url, user.Email); err != nil {
		log.Error(ctx, "failed to switch PR turn", "error", err, "pr_url", url, "email", user.Email)
		postEphemeralError(ctx, event, "failed to write internal data about this PR.")
		return err
	}

	newTurn, err := data.GetCurrentTurn(url)
	if err != nil {
		log.Error(ctx, "failed to get current turn for PR after switching", "error", err, "pr_url", url)
		postEphemeralError(ctx, event, "failed to read internal data about this PR.")
		return err
	}

	msg := "Thanks for letting me know!\n\n" + whoseTurnText(ctx, newTurn, user, " now")
	return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}

func setTurnSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	url := prDetailsFromChannel(ctx, event)
	if url == nil {
		return nil // Not a *server* error as far as we're concerned.
	}

	users, _ := extractAtLeastOneUserID(ctx, event, userIDsPattern)
	if len(users) == 0 {
		return nil // Not a *server* error as far as we're concerned.
	}

	postEphemeralError(ctx, event, "this command is not implemented yet.")
	return nil
}

func whoseTurnSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	url, emails, user, err := commonTurnData(ctx, event)
	if err != nil {
		return err
	}
	if url == "" {
		return nil
	}

	msg := whoseTurnText(ctx, emails, user, "")
	return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
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
			log.Error(ctx, "failed to load user by email", "error", err, "email", email)
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

func (c *Config) approveSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	url := prDetailsFromChannel(ctx, event)
	if url == nil {
		return nil
	}

	var err error
	switch {
	case c.bitbucketWorkspace != "":
		req := bitbucket.PullRequestsApproveRequest{Workspace: url[1], RepoSlug: url[2], PullRequestID: url[3]}
		err = bitbucket.PullRequestsApprove(ctx, req)
	default:
		log.Error(ctx, "neither Bitbucket nor GitHub are configured")
		postEphemeralError(ctx, event, "internal configuration error.")
		return errors.New("neither Bitbucket nor GitHub are configured")
	}

	if err != nil {
		log.Error(ctx, "failed to approve PR", "error", err, "pr_url", url[0])
		postEphemeralError(ctx, event, "failed to approve "+url[0])
		return err
	}

	// No need to post a confirmation message or update its bookmarks,
	// the resulting Bitbucket/GitHub event will trigger that.
	return nil
}

func (c *Config) unapproveSlashCommand(ctx workflow.Context, event SlashCommandEvent) (err error) {
	url := prDetailsFromChannel(ctx, event)
	if url == nil {
		return nil
	}

	switch {
	case c.bitbucketWorkspace != "":
		req := bitbucket.PullRequestsUnapproveRequest{Workspace: url[1], RepoSlug: url[2], PullRequestID: url[3]}
		err = bitbucket.PullRequestsUnapprove(ctx, req)
	default:
		log.Error(ctx, "neither Bitbucket nor GitHub are configured")
		postEphemeralError(ctx, event, "internal configuration error.")
		return errors.New("neither Bitbucket nor GitHub are configured")
	}

	if err != nil {
		log.Error(ctx, "failed to unapprove PR", "error", err, "pr_url", url[0])
		postEphemeralError(ctx, event, "failed to unapprove "+url[0])
		return err
	}

	// No need to post a confirmation message or update its bookmarks,
	// the resulting Bitbucket/GitHub event will trigger that.
	return nil
}

func updateChannelSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	url := prDetailsFromChannel(ctx, event)
	if url == nil {
		return nil
	}

	log.Warn(ctx, "this slash command is not implemented yet")
	postEphemeralError(ctx, event, "this command is not implemented yet.")
	return nil
}

func userDetails(ctx workflow.Context, event SlashCommandEvent, userID string) (data.User, bool, error) {
	user, err := data.SelectUserBySlackID(userID)
	if err != nil {
		log.Error(ctx, "failed to load user by Slack ID", "error", err, "user_id", userID)
		postEphemeralError(ctx, event, fmt.Sprintf("failed to read internal data about <@%s>.", userID))
		return data.User{}, false, err
	}

	return user, data.IsOptedIn(user), nil
}

// prDetailsFromChannel extracts the PR details based on the Slack channel's ID.
// This also ensures that the slash command is being run inside a RevChat channel.
func prDetailsFromChannel(ctx workflow.Context, event SlashCommandEvent) []string {
	url, err := data.SwitchURLAndID(event.ChannelID)
	if err != nil {
		log.Error(ctx, "failed to convert Slack channel to PR URL", "error", err, "channel_id", event.ChannelID)
		postEphemeralError(ctx, event, "failed to read internal data about the PR.")
		return nil
	}

	if url == "" {
		postEphemeralError(ctx, event, "this command can only be used inside RevChat channels.")
		return nil // Not a *server* error as far as we're concerned.
	}

	match := bitbucketURLPattern.FindStringSubmatch(url)
	if len(match) != 4 {
		log.Error(ctx, "failed to parse Bitbucket PR URL", "pr_url", url)
		postEphemeralError(ctx, event, "this command can only be used inside RevChat channels.")
		return nil
	}

	return match
}

func postEphemeralError(ctx workflow.Context, event SlashCommandEvent, msg string) {
	// We're already reporting another error, there's nothing to do if this fails.
	_ = PostEphemeralMessage(ctx, event.ChannelID, event.UserID, ":warning: Error: "+msg)
}
