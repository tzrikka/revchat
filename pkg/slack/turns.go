package slack

import (
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
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

func LoadPRTurns(ctx workflow.Context) map[string][]string {
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
		emails, err := data.GetCurrentTurn(ctx, url)
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
			_ = data.RemoveFromTurn(ctx, url, email) // Example: user deactivated after being added to the PR.
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

func PRDetails(ctx workflow.Context, url, userID string) string {
	var summary strings.Builder

	// Title.
	title := fmt.Sprintf("\n\n  •  *<%s>*", url)
	pr, err := data.LoadBitbucketPR(ctx, url)
	if err == nil {
		if t, ok := pr["title"].(string); ok && len(t) > 0 {
			title = fmt.Sprintf("\n\n  •  <%s|*%s*>", url, t)
		}
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
			summary.WriteString(" by " + users.BitbucketIDToSlackRef(ctx, id, name))
		}
	}

	// Slack channel link.
	if channelID, _ := data.SwitchURLAndID(ctx, url); channelID != "" {
		summary.WriteString(fmt.Sprintf("\n          ◦   <#%s>", channelID))
	}

	// PR details.
	now := time.Now().UTC()
	created := timeSince(now, pr["created_on"])
	summary.WriteString(fmt.Sprintf("\n          ◦   Created `%s` ago", created))

	if updated := timeSince(now, pr["updated_on"]); updated != "" {
		summary.WriteString(fmt.Sprintf(", updated `%s` ago", updated))
	}

	if b := summarizeBuilds(ctx, url); b != "" {
		summary.WriteString(", build states: ")
		summary.WriteString(b)
	}

	// User-specific details.
	paths := data.ReadBitbucketDiffstatPaths(url)
	if len(paths) == 0 {
		return summary.String()
	}

	workspace, repo, branch, commit := DestinationDetails(pr)
	owner := files.CountOwnedFiles(ctx, workspace, repo, branch, commit, users.SlackIDToRealName(ctx, userID), paths)
	highRisk := files.CountHighRiskFiles(ctx, workspace, repo, branch, commit, paths)
	approvals, names := approvers(ctx, pr)

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

func summarizeBuilds(ctx workflow.Context, url string) string {
	pr := data.ReadBitbucketBuilds(ctx, url)
	if pr == nil {
		return ""
	}

	keys := slices.Sorted(maps.Keys(pr.Builds))
	var summary []string
	for _, k := range keys {
		switch s := pr.Builds[k].State; s {
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
