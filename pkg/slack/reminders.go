package slack

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/config"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/files"
	"github.com/tzrikka/revchat/pkg/users"
	"github.com/tzrikka/xdg"
)

const (
	dateTimeLayout = time.DateOnly + " " + time.Kitchen
)

func remindersWorkflow(ctx workflow.Context) error {
	startTime := workflow.Now(ctx).UTC().Truncate(time.Minute)

	prs := loadPRTurns(ctx)
	if len(prs) == 0 {
		return nil
	}

	reminders, err := data.ListReminders()
	if err != nil {
		return err
	}

	for userID, r := range reminders {
		now, reminderTime, err := reminderTimes(ctx, startTime, userID, r)
		if err != nil {
			continue
		}

		// Send a reminder to the user if their reminder time matches
		// the current time, and there are reminders to be sent to them.
		if userPRs := prs[userID]; reminderTime.Equal(now) && len(userPRs) > 0 {
			logger.From(ctx).Info("sending scheduled Slack reminder to user",
				slog.String("user_id", userID), slog.Int("pr_count", len(userPRs)))
			slices.Sort(userPRs)

			var msg strings.Builder
			msg.WriteString(":bell: This is your scheduled daily reminder to take action on these PRs:")
			for _, url := range userPRs {
				msg.WriteString(prDetails(ctx, url, userID))
			}

			msg.WriteString("\n\n:information_source: Slash command tips:")
			msg.WriteString("\n  •  `/revchat status` - updated report at any time")
			msg.WriteString("\n  •  `/revchat reminder <time in 12h/24h format>` - change time or timezone")
			msg.WriteString("\n  •  `/revchat who` / `[not] my turn` / `[un]freeze` - only in PR channels")
			msg.WriteString("\n  •  `/revchat explain` - who needs to approve each file, and have they?")

			_, _ = PostMessage(ctx, userID, msg.String())
		}
	}

	return nil
}

func loadPRTurns(ctx workflow.Context) map[string][]string {
	// Walk through all stored PR states to find which users need to be reminded.
	root := filepath.Join(xdg.MustDataHome(), config.DirName)
	slackUserIDs := map[string][]string{}

	err := fs.WalkDir(os.DirFS(root), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(d.Name(), "_turn.json") {
			return nil
		}

		url := "https://" + strings.TrimSuffix(path, "_turn.json")
		emails, err := data.GetCurrentTurn(url)
		if err != nil {
			logger.From(ctx).Error("failed to get current attention state for PR",
				slog.Any("error", err), slog.String("pr_url", url))
			return nil // Continue walking.
		}

		slackIDs := make([]string, 0, len(emails))
		for _, email := range emails {
			if id := users.EmailToSlackID(ctx, email); id != "" {
				slackIDs = append(slackIDs, id)
				continue
			}
			logger.From(ctx).Warn("Slack email lookup error - removing from turn",
				slog.String("missing_email", email), slog.String("pr_url", url))
			_ = data.RemoveFromTurn(url, email) // Example: user deactivated after being added to the PR.
		}

		slackUserIDs[url] = slackIDs
		return nil
	})
	if err != nil {
		logger.From(ctx).Error("failed to get current attention state for PRs", slog.Any("error", err))
		return nil
	}

	// Invert the map to be keyed by Slack user IDs instead of PR URLs.
	prs := map[string][]string{}
	for url, ids := range slackUserIDs {
		for _, id := range ids {
			prs[id] = append(prs[id], url)
		}
	}

	return prs
}

func reminderTimes(ctx workflow.Context, startTime time.Time, userID, reminder string) (parsed, now time.Time, err error) {
	// Read and parse the daily reminder time for each user.
	kitchenTime, tz, found := strings.Cut(reminder, " ")
	if !found {
		logger.From(ctx).Error("invalid Slack reminder", slog.String("user_id", userID), slog.String("text", reminder))
		err = fmt.Errorf("invalid Slack reminder for Slack user %q: %q", userID, reminder)
		return parsed, now, err
	}

	loc, err := time.LoadLocation(tz)
	if err != nil {
		logger.From(ctx).Error("invalid timezone in Slack reminder", slog.Any("error", err),
			slog.String("user_id", userID), slog.String("time", reminder), slog.String("tz", tz))
		return parsed, now, err
	}

	now = startTime.In(loc)
	today := now.Format(time.DateOnly)
	rt := fmt.Sprintf("%s %s", today, kitchenTime)
	parsed, err = time.ParseInLocation(dateTimeLayout, rt, loc)
	if err != nil {
		logger.From(ctx).Error("invalid time in Slack reminder", slog.Any("error", err),
			slog.String("user_id", userID), slog.String("date_time", rt))
		return parsed, now, err
	}

	return parsed, now, err
}

func prDetails(ctx workflow.Context, url, userID string) string {
	var summary strings.Builder

	// Title.
	title := fmt.Sprintf("\n\n  •  *<%s>*", url)
	pr, err := data.LoadBitbucketPR(url)
	if err != nil {
		logger.From(ctx).Error("failed to load Bitbucket PR snapshot for reminder",
			slog.Any("error", err), slog.String("pr_url", url))
	} else if t, ok := pr["title"].(string); ok && len(t) > 0 {
		title = fmt.Sprintf("\n\n  •  <%s|*%s*>", url, t)
	}

	// Draft indicator.
	if draft, ok := pr["draft"].(bool); ok && draft {
		title = strings.Replace(title, "•  ", "•  :construction: ", 1)
		if !strings.Contains(strings.ToLower(title), "draft") {
			title += " (draft)"
		}
	}
	summary.WriteString(title)

	// Author.
	if author, ok := pr["author"].(map[string]any); ok {
		if id, ok := author["account_id"].(string); ok {
			name, ok := author["display_name"].(string)
			if !ok {
				name = ""
			}
			summary.WriteString(" by " + users.BitbucketToSlackRef(ctx, id, name))
		}
	}

	// Slack channel link.
	channelID, err := data.SwitchURLAndID(url)
	if err != nil {
		logger.From(ctx).Error("failed to retrieve Slack channel ID for reminder",
			slog.Any("error", err), slog.String("pr_url", url))
	} else {
		summary.WriteString(fmt.Sprintf("\n          ◦   <#%s>", channelID))
	}

	// PR details.
	now := time.Now().UTC()
	created := timeSince(now, pr["created_on"])
	summary.WriteString(fmt.Sprintf("\n          ◦   Created `%s` ago", created))

	if updated := timeSince(now, pr["updated_on"]); updated != "" {
		summary.WriteString(fmt.Sprintf(", updated `%s` ago", updated))
	}

	if b := data.SummarizeBitbucketBuilds(url); b != "" {
		summary.WriteString(", build states: ")
		summary.WriteString(b)
	}

	// User-specific details.
	paths := data.ReadBitbucketDiffstatPaths(url)
	if len(paths) == 0 {
		return summary.String()
	}

	workspace, repo, branch, commit := destinationDetails(pr)
	owner := files.CountOwnedFiles(ctx, workspace, repo, branch, commit, users.SlackIDToRealName(ctx, userID), paths)
	highRisk := files.CountHighRiskFiles(ctx, workspace, repo, branch, commit, paths)
	approvals, names := approversForReminder(ctx, pr)

	if owner+highRisk+approvals > 0 {
		summary.WriteString("\n          ◦   ")
	}
	if owner > 0 {
		summary.WriteString(fmt.Sprintf("Code owner: *%d* file", owner))
		if owner > 1 {
			summary.WriteString("s")
		}
		if highRisk > 0 {
			summary.WriteString(", h")
		}
	}
	if highRisk > 0 {
		if owner == 0 {
			summary.WriteString("H")
		}
		summary.WriteString(fmt.Sprintf("igh risk: *%d* file", highRisk))
		if highRisk > 1 {
			summary.WriteString("s")
		}
	}
	if approvals > 0 {
		if owner+highRisk > 0 {
			summary.WriteString(", a")
		} else {
			summary.WriteString("A")
		}
		summary.WriteString(fmt.Sprintf("pprovals: *%d* (%s)", approvals, names))
	}

	// list.WriteString("\n          ◦   TODO: You haven't commented on it yet | Your last review was `XXX` ago")

	return summary.String()
}

func approversForReminder(ctx workflow.Context, pr map[string]any) (int, string) {
	participants, ok := pr["participants"].([]any)
	if !ok {
		return 0, ""
	}

	count := 0
	names := strings.Builder{}
	for _, p := range participants {
		participant, ok := p.(map[string]any)
		if !ok {
			continue
		}
		approved, ok := participant["approved"].(bool)
		if !ok || !approved {
			continue
		}

		count++
		user, ok := participant["user"].(map[string]any)
		if !ok {
			continue
		}
		accountID, ok := user["account_id"].(string)
		if !ok {
			continue
		}
		if count > 1 {
			names.WriteString(", ")
		}

		names.WriteString(users.BitbucketToSlackRef(ctx, accountID, ""))
	}

	return count, names.String()
}
