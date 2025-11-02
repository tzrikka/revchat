package slack

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/revchat/pkg/config"
	"github.com/tzrikka/revchat/pkg/data"
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
			slices.Sort(userPRs)
			log.Info(ctx, "sending scheduled Slack reminder to user", "user_id", userID, "pr_count", len(userPRs))
			msg := ":bell: This is your scheduled daily reminder to check these PRs:"
			for _, url := range userPRs {
				msg += prDetails(ctx, url)
			}
			_, _ = PostMessage(ctx, userID, msg)
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
			log.Error(ctx, "failed to get current attention state for PR", "error", err, "pr_url", url)
			return nil // Continue walking.
		}

		slackIDs := make([]string, 0, len(emails))
		for _, email := range emails {
			if id := users.EmailToSlackID(ctx, email); id != "" {
				slackIDs = append(slackIDs, id)
			}
		}

		slackUserIDs[url] = slackIDs
		return nil
	})
	if err != nil {
		log.Error(ctx, "failed to get current attention state for PRs", "error", err)
		return nil
	}

	// Invert the map to be keyed by Slack user IDs instead of PR URLs.
	prs := map[string][]string{}
	for url, ids := range slackUserIDs {
		for _, id := range ids {
			prs[id] = append(prs[id], url)
		}
	}

	log.Debug(ctx, "loaded PR attention states", "pr_count", len(slackUserIDs), "user_count", len(prs))
	return prs
}

func reminderTimes(ctx workflow.Context, startTime time.Time, userID, reminder string) (parsed, now time.Time, err error) {
	// Read and parse the daily reminder time for each user.
	kitchenTime, tz, found := strings.Cut(reminder, " ")
	if !found {
		log.Error(ctx, "invalid Slack reminder", "user_id", userID, "text", reminder)
		err = fmt.Errorf("invalid Slack reminder for Slack user %q: %q", userID, reminder)
		return
	}

	loc, err := time.LoadLocation(tz)
	if err != nil {
		log.Error(ctx, "invalid timezone in Slack reminder", "error", err, "user_id", userID, "time", reminder, "tz", tz)
		return
	}

	now = startTime.In(loc)
	today := now.Format(time.DateOnly)
	rt := fmt.Sprintf("%s %s", today, kitchenTime)
	parsed, err = time.ParseInLocation(dateTimeLayout, rt, loc)
	if err != nil {
		log.Error(ctx, "invalid time in Slack reminder", "error", err, "user_id", userID, "date_time", rt)
		return
	}

	return
}

func prDetails(ctx workflow.Context, url string) string {
	sb := strings.Builder{}

	// Title.
	title := "\n\n  •  *(Unnamed PR)*"
	pr, err := data.LoadBitbucketPR(url)
	if err != nil {
		log.Error(ctx, "failed to load Bitbucket PR snapshot for reminder", "error", err, "pr_url", url)
	} else {
		title = fmt.Sprintf("\n\n  •  *%v*", pr["title"])
	}
	sb.WriteString(title)

	// Slack channel link.
	channelID, err := data.SwitchURLAndID(url)
	if err != nil {
		log.Error(ctx, "failed to get Slack channel ID for reminder", "error", err, "pr_url", url)
	} else {
		sb.WriteString(fmt.Sprintf("\n          ◦   <#%s>", channelID))
	}

	// PR URL.
	sb.WriteString("\n          ◦   ")
	sb.WriteString(url)

	// User-specific details.
	sb.WriteString("\n          ◦   TODO: Details... (time since last update/review, owner/high-risk, checks)")

	// Approvals.
	count, names := approvals(ctx, pr)
	if count > 0 {
		sb.WriteString(fmt.Sprintf("\n          ◦   Approvals: %d (%s)", count, names))
	}

	return sb.String()
}

func approvals(ctx workflow.Context, pr map[string]any) (int, string) {
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
			names.WriteString(" ")
		}

		names.WriteString(users.BitbucketToSlackRef(ctx, accountID, ""))
	}

	return count, names.String()
}
