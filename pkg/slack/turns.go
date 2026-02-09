package slack

import (
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"os"
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

// LoadPRTurns scans all stored PR turn files, and returns a map of
// Slack user IDs to all the PR URLs they need to be reminded about.
func LoadPRTurns(ctx workflow.Context, onlyCurrent bool) map[string][]string {
	root, err := xdg.CreateDir(xdg.DataHome, config.DirName)
	if err != nil {
		return nil
	}

	usersToPRs := map[string][]string{}
	err = fs.WalkDir(os.DirFS(root), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(d.Name(), data.TurnsFileSuffix) {
			return nil
		}

		prURL := "https://" + strings.TrimSuffix(path, data.TurnsFileSuffix)
		var emails []string
		if onlyCurrent {
			emails, err = data.GetCurrentTurns(ctx, prURL)
		} else {
			emails, err = data.GetAllTurns(ctx, prURL)
		}
		if err != nil {
			return nil
		}

		for _, email := range emails {
			if id := users.EmailToSlackID(ctx, email); id != "" {
				usersToPRs[id] = append(usersToPRs[id], prURL)
				continue
			}
			logger.From(ctx).Warn("Slack email lookup error - removing from turn",
				slog.String("missing_email", email), slog.String("pr_url", prURL))

			// Example: user deactivated after being added to the PR.
			_ = data.RemoveReviewerFromTurns(ctx, prURL, email, false)
		}

		return nil
	})
	if err != nil {
		logger.From(ctx).Error("failed to get current attention state for PRs", slog.Any("error", err))
		return nil
	}

	return usersToPRs
}

func PRDetails(ctx workflow.Context, url string, userIDs []string) string {
	summary := new(strings.Builder)

	// Title + optional draft indicator.
	title := url
	draftEmoji := ""
	pr, err := data.LoadPRSnapshot(ctx, url)
	if err == nil {
		if draft, ok := pr["draft"].(bool); ok && draft {
			draftEmoji = ":construction: "
		}

		if t, ok := pr["title"].(string); ok && len(t) > 0 {
			title = fmt.Sprintf("<%s|*%s*>", url, strings.ReplaceAll(t, ">", "&gt;"))
		}
	}
	fmt.Fprintf(summary, "\n\n  •  %s%s", draftEmoji, title)

	// Author.
	if author, ok := pr["author"].(map[string]any); ok {
		if id, ok := author["account_id"].(string); ok {
			name, ok := author["display_name"].(string)
			if !ok {
				name = ""
			}
			summary.WriteString(" by " + users.BitbucketIDToSlackRef(ctx, id, name))
		}
	}

	// Slack channel link (unless this is a status report about other users).
	if len(userIDs) == 1 {
		if channelID, _ := data.SwitchURLAndID(ctx, url); channelID != "" {
			fmt.Fprintf(summary, "\n          ◦   <#%s>", channelID)
		}
	}

	// PR & user-specific details.
	now := time.Now().UTC()
	created := timeSince(now, pr["created_on"])
	fmt.Fprintf(summary, "\n          ◦   Created `%s` ago", created)

	if updated := timeSince(now, pr["updated_on"]); updated != "" {
		fmt.Fprintf(summary, ", updated `%s` ago", updated)
	}

	if len(userIDs) == 1 {
		if activity := data.GetActivityTime(ctx, url, users.SlackIDToEmail(ctx, userIDs[0])); !activity.IsZero() {
			fmt.Fprintf(summary, ", touched by you `%s` ago", timeSince(now, activity))
		}
	}

	if b := summarizeBuilds(ctx, url); b != "" {
		summary.WriteString(", builds: " + b)
	}

	// User-specific details.
	fullNames := make([]string, 0, len(userIDs))
	for _, userID := range userIDs {
		if name := users.SlackIDToRealName(ctx, userID); name != "" {
			fullNames = append(fullNames, name)
		}
	}

	paths := data.ReadBitbucketDiffstatPaths(url)
	if len(paths) == 0 {
		return summary.String()
	}

	workspace, repo, branch, commit := DestinationDetails(pr)
	owner := files.CountOwnedFiles(ctx, workspace, repo, branch, commit, fullNames, paths)
	highRisk := files.CountHighRiskFiles(ctx, workspace, repo, branch, commit, paths)
	approvals, names := approvers(ctx, pr)

	if owner+highRisk+approvals > 0 {
		summary.WriteString("\n          ◦   ")
	}
	if owner > 0 {
		fmt.Fprintf(summary, "Code owners: *%d* file", owner)
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
		fmt.Fprintf(summary, "igh risk: *%d* file", highRisk)
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
		fmt.Fprintf(summary, "pprovals: *%d* (%s)", approvals, names)
	}

	return summary.String()
}

func summarizeBuilds(ctx workflow.Context, url string) string {
	prStatus := data.ReadBitbucketBuilds(ctx, url)
	keys := slices.Sorted(maps.Keys(prStatus.Builds))
	var summary []string
	for _, k := range keys {
		switch s := prStatus.Builds[k].State; s {
		case "INPROGRESS":
			// Don't show in-progress builds in summary.
		case "SUCCESSFUL":
			summary = append(summary, "large_green_circle")
		default: // "FAILED", "STOPPED".
			summary = append(summary, "red_circle")
		}
	}

	// Returns a sequence of space-separated emoji.
	if len(summary) > 0 {
		return fmt.Sprintf(":%s:", strings.Join(summary, ": :"))
	}
	return ""
}

func approvers(ctx workflow.Context, pr map[string]any) (int, string) {
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

		names.WriteString(users.BitbucketIDToSlackRef(ctx, accountID, ""))
	}

	return count, names.String()
}
