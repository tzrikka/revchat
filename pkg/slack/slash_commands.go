package slack

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/files"
	"github.com/tzrikka/revchat/pkg/users"
	"github.com/tzrikka/timpani-api/pkg/bitbucket"
	"github.com/tzrikka/timpani-api/pkg/slack"
)

const (
	DefaultReminderTime = "8:00AM"
)

var (
	bitbucketURLPattern = regexp.MustCompile(`^https://[^/]+/([\w-]+)/([\w-]+)/pull-requests/(\d+)`)
	remindersPattern    = regexp.MustCompile(`^reminders?(\s+at)?\s+([0-9:]+)\s*(am|pm|a|p)?`)
	userCommandsPattern = regexp.MustCompile(`^(follow|unfollow|invite|nudge|ping|poke)`)
	userIDsPattern      = regexp.MustCompile(`<(@)(\w+)(\|[^>]*)?>`)
	userOrTeamIDPattern = regexp.MustCompile(`<(@|!subteam\^)(\w+)(\|[^>]*)?>`)
)

// https://docs.slack.dev/apis/events-api/using-socket-mode#command
// https://docs.slack.dev/interactivity/implementing-slash-commands#app_command_handling
func (c *Config) slashCommandWorkflow(ctx workflow.Context, event SlashCommandEvent) error {
	event.Text = strings.ToLower(event.Text)
	switch event.Text {
	case "", "help":
		return helpSlashCommand(ctx, event)

	case "opt-in", "opt in", "optin":
		return c.optInSlashCommand(ctx, event)
	case "opt-out", "opt out", "optout":
		return c.optOutSlashCommand(ctx, event)

	case "clean":
		return cleanSlashCommand(ctx, event)
	case "explain":
		return explainSlashCommand(ctx, event)
	case "stat", "state", "status":
		return statusSlashCommand(ctx, event)

	case "who", "whose", "whose turn":
		return whoseTurnSlashCommand(ctx, event)
	case "my turn":
		return myTurnSlashCommand(ctx, event)
	case "not my turn":
		return notMyTurnSlashCommand(ctx, event)
	case "freeze", "freeze turn", "freeze turns":
		return freezeTurnSlashCommand(ctx, event)
	case "unfreeze", "unfreeze turn", "unfreeze turns":
		return unfreezeTurnSlashCommand(ctx, event)

	case "approve", "lgtm", "+1":
		return c.approveSlashCommand(ctx, event)
	case "unapprove", "-1":
		return c.unapproveSlashCommand(ctx, event)
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
		}
	}
	if remindersPattern.MatchString(event.Text) {
		return remindersSlashCommand(ctx, event)
	}

	logger.From(ctx).Warn("unrecognized Slack slash command",
		slog.String("username", event.UserName), slog.String("text", event.Text))
	postEphemeralError(ctx, event, fmt.Sprintf("unrecognized command - try `%s help`", event.Command))
	return nil
}

func helpSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	var cmds strings.Builder

	cmds.WriteString(":wave: Available general commands:\n")
	cmds.WriteString("\n  •  `%s opt-in` - opt into being added to PR channels and receiving DMs")
	cmds.WriteString("\n  •  `%s opt-out` - opt out of being added to PR channels and receiving DMs")
	cmds.WriteString("\n  •  `%s reminders at <time in 12h/24h format>` - weekdays, using your timezone")
	cmds.WriteString("\n  •  `%s status` - show your current PR states, as an author and a reviewer")
	cmds.WriteString("\n\nMore commands inside PR channels:\n")
	cmds.WriteString("\n  •  `%s who` / `whose turn` / `my turn` / `not my turn` / `[un]freeze [turns]`")
	cmds.WriteString("\n  •  `%s nudge <1 or more @users or @groups>` / `ping <...>` / `poke <...>`")
	cmds.WriteString("\n  •  `%s explain` - who needs to approve each file, and have they?")
	cmds.WriteString("\n  •  `%s clean` - remove unnecessary reviewers from the PR")
	cmds.WriteString("\n  •  `%s approve` or `lgtm` or `+1`")
	cmds.WriteString("\n  •  `%s unapprove` or `-1`")

	msg := strings.ReplaceAll(cmds.String(), "%s", event.Command)
	return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}

func cleanSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	url, paths, pr, err := reviewerData(ctx, event)
	if err != nil || url == nil || len(url) < 4 || len(paths) == 0 {
		return err
	}

	workspace, repo, branch, commit := destinationDetails(pr)
	owners, _ := files.OwnersPerPath(ctx, workspace, repo, branch, commit, paths, true)
	reviewers := requiredReviewers(paths, owners)
	for i, fullName := range reviewers {
		user, _ := data.SelectUserByRealName(fullName)
		if user.BitbucketID != "" {
			reviewers[i] = user.BitbucketID
		}
	}

	reviewers = append(reviewers, approversForClean(pr)...)
	reviewers = filterReviewers(pr, reviewers)

	// Need to impersonate in Bitbucket the user who sent this Slack command.
	linkID, err := thrippyLinkID(ctx, event.UserID, event.ChannelID)
	if err != nil || linkID == "" {
		postEphemeralError(ctx, event, "failed to get current PR details from Bitbucket.")
		return err
	}

	// Retrieve the latest PR metadata from Bitbucket, just in case our stored snapshot is outdated.
	pr, err = bitbucket.PullRequestsGet(ctx, bitbucket.PullRequestsGetRequest{
		ThrippyLinkID: linkID,
		Workspace:     workspace,
		RepoSlug:      repo,
		PullRequestID: url[3],
	})
	if err != nil {
		logger.From(ctx).Error("failed to get Bitbucket PR",
			slog.Any("error", err), slog.String("pr_url", event.UserID))
		postEphemeralError(ctx, event, "failed to get current PR details from Bitbucket.")
		return err
	}

	// Bitbucket API quirk: it rejects updates with the "summary.html" field.
	delete(pr, "summary")

	// Update the reviewers list in Bitbucket.
	pr["reviewers"] = make([]map[string]any, len(reviewers))
	for i, accountID := range reviewers {
		pr["reviewers"].([]map[string]any)[i] = map[string]any{ //nolint:errcheck
			"account_id": accountID,
		}
	}

	_, err = bitbucket.PullRequestsUpdate(ctx, bitbucket.PullRequestsUpdateRequest{
		PullRequestsRequest: bitbucket.PullRequestsRequest{
			ThrippyLinkID: linkID,
			Workspace:     workspace,
			RepoSlug:      repo,
			PullRequestID: url[3],
		},
		PullRequest: pr,
	})
	if err != nil {
		logger.From(ctx).Error("failed to update Bitbucket PR", slog.Any("error", err),
			slog.String("pr_url", event.UserID), slog.String("slack_user_id", event.UserID))
		postEphemeralError(ctx, event, "failed to update PR reviewers in Bitbucket.")
		return err
	}

	return nil
}

func reviewerData(ctx workflow.Context, event SlashCommandEvent) (url, paths []string, pr map[string]any, err error) {
	url = prDetailsFromChannel(ctx, event)
	if url == nil {
		return nil, nil, nil, nil
	}

	pr, err = data.LoadBitbucketPR(url[0])
	if err != nil {
		postEphemeralError(ctx, event, "failed to load PR snapshot.")
		return url, nil, nil, err
	}

	paths = data.ReadBitbucketDiffstatPaths(url[0])
	if len(paths) == 0 {
		postEphemeralError(ctx, event, "no file paths found in PR diffstat.")
		return url, nil, pr, nil
	}

	return url, paths, pr, nil
}

func explainSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	url, paths, pr, err := reviewerData(ctx, event)
	if err != nil || url == nil || len(paths) == 0 {
		return err
	}

	workspace, repo, branch, commit := destinationDetails(pr)
	owners, groups := files.OwnersPerPath(ctx, workspace, repo, branch, commit, paths, false)

	msg := explainCodeOwners(paths, owners, groups, approversForExplain(ctx, pr))
	return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}

func statusSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	prs := loadPRTurns(ctx)[event.UserID]
	if len(prs) == 0 {
		return PostEphemeralMessage(ctx, event.ChannelID, event.UserID,
			":joy: No PRs require your attention at this time!")
	}

	var msg strings.Builder
	msg.WriteString(":eyes: These PRs currently require your attention:")

	slices.Sort(prs)
	for _, url := range prs {
		msg.WriteString(prDetails(ctx, url, event.UserID))
	}

	return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg.String())
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

		if err := data.FollowUser(event.UserID, userID); err != nil {
			logger.From(ctx).Error("failed to follow user", slog.Any("error", err),
				slog.String("follower_id", event.UserID), slog.String("followed_id", userID))
			postEphemeralError(ctx, event, fmt.Sprintf("failed to follow <@%s>.", userID))
			continue
		}

		msg := fmt.Sprintf("You will now be added to channels for PRs authored by <@%s>.", userID)
		_ = PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
	}

	return nil
}

func unfollowSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
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

		if err := data.UnfollowUser(event.UserID, userID); err != nil {
			logger.From(ctx).Error("failed to unfollow user", slog.Any("error", err),
				slog.String("unfollower_id", event.UserID), slog.String("followed_id", userID))
			postEphemeralError(ctx, event, fmt.Sprintf("failed to unfollow <@%s>.", userID))
			continue
		}

		msg := fmt.Sprintf("You will no longer be added to channels for PRs authored by <@%s>.", userID)
		_ = PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
	}

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
		logger.From(ctx).Error("failed to nudge user", slog.Any("error", err),
			slog.String("pr_url", url), slog.String("user_id", userID))
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

	info, err := slack.UsersInfo(ctx, event.UserID)
	if err != nil {
		logger.From(ctx).Error("failed to retrieve Slack user info",
			slog.Any("error", err), slog.String("user_id", event.UserID))
		postEphemeralError(ctx, event, "failed to retrieve your user info from Slack.")
		return err
	}

	// Ensure the user's basic details are set even if they're unrecognized.
	user.Email = strings.ToLower(info.Profile.Email)
	user.RealName = info.RealName
	user.SlackID = event.UserID
	if info.IsBot {
		user.Email = "bot"
	}

	switch {
	case c.BitbucketWorkspace != "":
		return c.optInBitbucket(ctx, event, user)
	default:
		logger.From(ctx).Error("neither Bitbucket nor GitHub are configured")
		postEphemeralError(ctx, event, "internal configuration error.")
		return errors.New("neither Bitbucket nor GitHub are configured")
	}
}

func (c *Config) optInBitbucket(ctx workflow.Context, event SlashCommandEvent, user data.User) error {
	linkID, nonce, err := c.createThrippyLink(ctx)
	if err != nil {
		logger.From(ctx).Error("failed to create Thrippy link for Slack user", slog.Any("error", err))
		postEphemeralError(ctx, event, "internal authorization failure.")
		return err
	}

	msg := ":point_right: <https://%s/start?id=%s&nonce=%s|Click here> to authorize RevChat to act on your behalf in Bitbucket."
	msg = fmt.Sprintf(msg, c.ThrippyHTTPAddress, linkID, nonce)
	if err := PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg); err != nil {
		logger.From(ctx).Error("failed to post ephemeral opt-in message in Slack", slog.Any("error", err))
		_ = c.deleteThrippyLink(ctx, linkID)
		return err
	}

	err = c.waitForThrippyLinkCreds(ctx, linkID)
	if err != nil {
		_ = c.deleteThrippyLink(ctx, linkID)
		if err.Error() == ErrLinkAuthzTimeout { // For some reason errors.Is() doesn't work across Temporal.
			logger.From(ctx).Warn("user did not complete Thrippy OAuth flow in time", slog.String("email", user.Email))
			postEphemeralError(ctx, event, "Bitbucket authorization timed out - please try opting in again.")
			return nil // Not a *server* error as far as we're concerned.
		} else {
			logger.From(ctx).Error("failed to authorize Bitbucket user",
				slog.Any("error", err), slog.String("email", user.Email))
			postEphemeralError(ctx, event, "failed to authorize you in Bitbucket.")
			return err
		}
	}

	if err := data.UpsertUser(user.Email, "", "", user.SlackID, user.RealName, linkID); err != nil {
		logger.From(ctx).Error("failed to opt-in user", slog.Any("error", err), slog.String("email", user.Email))
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

	if err := data.UpsertUser(user.Email, user.BitbucketID, user.GitHubID, user.SlackID, "", "X"); err != nil {
		logger.From(ctx).Error("failed to opt-out user", slog.Any("error", err), slog.String("email", user.Email))
		postEphemeralError(ctx, event, "failed to write internal data about you.")
		return err
	}

	if err := c.deleteThrippyLink(ctx, user.ThrippyLink); err != nil {
		logger.From(ctx).Error("failed to delete Thrippy link for opted-out user",
			slog.Any("error", err), slog.String("link_id", user.ThrippyLink))
		// This is an internal error, it doesn't concern or affect the user.
	}

	msg := ":no_bell: You are now opted out of using RevChat for new PRs."
	return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}

func remindersSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	matches := remindersPattern.FindStringSubmatch(event.Text)
	if len(matches) < 3 {
		logger.From(ctx).Error("failed to parse reminders slash command - regex mismatch", slog.String("text", event.Text))
		postEphemeralError(ctx, event, "unexpected internal error while parsing command.")
		return errors.New("failed to parse reminders command - regex mismatch")
	}

	amPm := ""
	if len(matches) > 3 {
		amPm = matches[3]
	}

	kitchenTime, err := normalizeTime(matches[2], amPm)
	if err != nil {
		logger.From(ctx).Warn("failed to parse time in reminders slash command", slog.Any("error", err))
		postEphemeralError(ctx, event, err.Error())
		return nil // Not a *server* error as far as we're concerned.
	}

	if kt, _ := time.Parse(time.Kitchen, kitchenTime); kt.Minute() != 0 && kt.Minute() != 30 {
		logger.From(ctx).Warn("uncommon reminder time requested",
			slog.String("user_id", event.UserID), slog.String("time", kitchenTime))
		postEphemeralError(ctx, event, "please specify a time on the hour or half-hour.")
		return nil // Not a *server* error as far as we're concerned.
	}

	return setReminder(ctx, event, kitchenTime, false)
}

func setReminder(ctx workflow.Context, event SlashCommandEvent, t string, quiet bool) error {
	user, err := slack.UsersInfo(ctx, event.UserID)
	if err != nil {
		logger.From(ctx).Error("failed to retrieve Slack user info",
			slog.Any("error", err), slog.String("user_id", event.UserID))
		postEphemeralError(ctx, event, "failed to retrieve Slack user info.")
		return err
	}

	if user.TZ == "" {
		logger.From(ctx).Warn("Slack user has no timezone in their profile", slog.String("user_id", event.UserID))
		postEphemeralError(ctx, event, "please set a timezone in your Slack profile preferences first.")
		return nil // Not a *server* error as far as we're concerned.
	}

	if _, err := time.LoadLocation(user.TZ); err != nil {
		logger.From(ctx).Warn("unrecognized user timezone", slog.Any("error", err),
			slog.String("user_id", event.UserID), slog.String("tz", user.TZ))
		postEphemeralError(ctx, event, fmt.Sprintf("your Slack timezone is unrecognized: `%s`", user.TZ))
		return err
	}

	if err := data.SetReminder(event.UserID, t, user.TZ); err != nil {
		logger.From(ctx).Error("failed to store user reminder time", slog.Any("error", err),
			slog.String("user_id", event.UserID), slog.String("time", t), slog.String("zone", user.TZ))
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

func commonTurnData(ctx workflow.Context, event SlashCommandEvent) (string, []string, data.User, error) {
	url := prDetailsFromChannel(ctx, event)
	if url == nil {
		return "", nil, data.User{}, nil // Not a *server* error as far as we're concerned.
	}

	emails, err := data.GetCurrentTurn(url[0])
	if err != nil {
		logger.From(ctx).Error("failed to get current turn for PR",
			slog.Any("error", err), slog.String("pr_url", url[0]))
		postEphemeralError(ctx, event, "failed to read internal data about the PR.")
		return "", nil, data.User{}, err
	}

	user, err := data.SelectUserBySlackID(event.UserID)
	if err != nil {
		logger.From(ctx).Error("failed to load user by Slack ID",
			slog.Any("error", err), slog.String("user_id", event.UserID))
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
		logger.From(ctx).Error("failed to self-nudge", slog.Any("error", err),
			slog.String("pr_url", url), slog.String("email", user.Email))
		postEphemeralError(ctx, event, "failed to write internal data about this PR.")
	}
	if !ok {
		msg = ":thinking_face: I didn't think you're supposed to review this PR, thanks for letting me know!\n\n"

		if err := data.AddReviewerToPR(url, user.Email); err != nil {
			logger.From(ctx).Error("failed to add reviewer to PR", slog.Any("error", err),
				slog.String("pr_url", url), slog.String("email", user.Email))
			postEphemeralError(ctx, event, "failed to write internal data about this.")
		}
	}

	emails, err = data.GetCurrentTurn(url)
	if err != nil {
		logger.From(ctx).Error("failed to get current turn for PR after switching",
			slog.Any("error", err), slog.String("pr_url", url))
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
		msg := ":joy: I didn't think it's your turn anyway!\n\n" + whoseTurnText(ctx, currentTurn, user, "")
		return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
	}

	if err := data.SwitchTurn(url, user.Email); err != nil {
		logger.From(ctx).Error("failed to switch PR turn", slog.Any("error", err),
			slog.String("pr_url", url), slog.String("email", user.Email))
		postEphemeralError(ctx, event, "failed to write internal data about this PR.")
		return err
	}

	newTurn, err := data.GetCurrentTurn(url)
	if err != nil {
		logger.From(ctx).Error("failed to get current turn for PR after switching",
			slog.Any("error", err), slog.String("pr_url", url))
		postEphemeralError(ctx, event, "failed to read internal data about this PR.")
		return err
	}

	msg := "Thanks for letting me know!\n\n" + whoseTurnText(ctx, newTurn, user, " now")
	return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}

func freezeTurnSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	url := prDetailsFromChannel(ctx, event)
	if url == nil {
		return nil // Not a *server* error as far as we're concerned.
	}

	ok, err := data.FreezeTurn(url[0], users.SlackIDToEmail(ctx, event.UserID))
	if err != nil {
		logger.From(ctx).Error("failed to freeze PR turn", slog.Any("error", err), slog.String("pr_url", url[0]))
		postEphemeralError(ctx, event, "failed to write internal data about this PR.")
		return err
	}

	msg := ":snowflake: Turn switching is now frozen in this PR."
	if !ok {
		msg = ":snowflake: Turn switching is already frozen in this PR."
	}
	return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}

func unfreezeTurnSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	url := prDetailsFromChannel(ctx, event)
	if url == nil {
		return nil // Not a *server* error as far as we're concerned.
	}

	ok, err := data.UnfreezeTurn(url[0])
	if err != nil {
		logger.From(ctx).Error("failed to unfreeze PR turn", slog.Any("error", err), slog.String("pr_url", url[0]))
		postEphemeralError(ctx, event, "failed to write internal data about this PR.")
		return err
	}

	msg := ":sunny: Turn switching is now unfrozen in this PR."
	if !ok {
		msg = ":sunny: Turn switching is already unfrozen in this PR."
	}
	return PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
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

func (c *Config) approveSlashCommand(ctx workflow.Context, event SlashCommandEvent) error {
	url := prDetailsFromChannel(ctx, event)
	if url == nil {
		return nil
	}

	var err error
	switch {
	case c.BitbucketWorkspace != "":
		req := bitbucket.PullRequestsApproveRequest{Workspace: url[1], RepoSlug: url[2], PullRequestID: url[3]}
		err = bitbucket.PullRequestsApprove(ctx, req)
	default:
		logger.From(ctx).Error("neither Bitbucket nor GitHub are configured")
		postEphemeralError(ctx, event, "internal configuration error.")
		return errors.New("neither Bitbucket nor GitHub are configured")
	}

	if err != nil {
		logger.From(ctx).Error("failed to approve PR", slog.Any("error", err), slog.String("pr_url", url[0]))
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
	case c.BitbucketWorkspace != "":
		req := bitbucket.PullRequestsUnapproveRequest{Workspace: url[1], RepoSlug: url[2], PullRequestID: url[3]}
		err = bitbucket.PullRequestsUnapprove(ctx, req)
	default:
		logger.From(ctx).Error("neither Bitbucket nor GitHub are configured")
		postEphemeralError(ctx, event, "internal configuration error.")
		return errors.New("neither Bitbucket nor GitHub are configured")
	}

	if err != nil {
		logger.From(ctx).Error("failed to unapprove PR", slog.Any("error", err), slog.String("pr_url", url[0]))
		postEphemeralError(ctx, event, "failed to unapprove "+url[0])
		return err
	}

	// No need to post a confirmation message or update its bookmarks,
	// the resulting Bitbucket/GitHub event will trigger that.
	return nil
}

func userDetails(ctx workflow.Context, event SlashCommandEvent, userID string) (data.User, bool, error) {
	user, err := data.SelectUserBySlackID(userID)
	if err != nil {
		logger.From(ctx).Error("failed to load user by Slack ID", slog.Any("error", err), slog.String("user_id", userID))
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
		logger.From(ctx).Error("failed to convert Slack channel to PR URL",
			slog.Any("error", err), slog.String("channel_id", event.ChannelID))
		postEphemeralError(ctx, event, "failed to read internal data about the PR.")
		return nil
	}

	if url == "" {
		postEphemeralError(ctx, event, "this command can only be used inside RevChat channels.")
		return nil // Not a *server* error as far as we're concerned.
	}

	match := bitbucketURLPattern.FindStringSubmatch(url)
	if len(match) != 4 {
		logger.From(ctx).Error("failed to parse Bitbucket PR URL", slog.String("pr_url", url))
		postEphemeralError(ctx, event, "this command can only be used inside RevChat channels.")
		return nil
	}

	return match
}

func postEphemeralError(ctx workflow.Context, event SlashCommandEvent, msg string) {
	// We're already reporting another error, there's nothing to do if this fails.
	_ = PostEphemeralMessage(ctx, event.ChannelID, event.UserID, ":warning: Error: "+msg)
}
