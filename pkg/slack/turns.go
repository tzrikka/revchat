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

// PRDetails returns a summary of a Bitbucket or GitHub PR's metadata and activity.
// The output is empty if the PR is in draft mode and the reportDrafts parameter is false.
func PRDetails(ctx workflow.Context, url string, userIDs []string, selfReport, reportDrafts bool) string {
	summary := new(strings.Builder)
	pr, err := data.LoadPRSnapshot(ctx, url)
	if err != nil {
		fmt.Fprintf(summary, "\n\n<%s|*%s*>", url, url)
		if selfReport {
			if channelID, _ := data.SwitchURLAndID(ctx, url); channelID != "" {
				fmt.Fprintf(summary, "\n> <#%s>", channelID)
			}
		}
		summary.WriteString("\n> :warning: Failed to read internal data about this PR")
		return summary.String()
	}

	// Abort if this is a draft but RevChat isn't configured to report drafts.
	t, draft := title(url, pr)
	if draft && !reportDrafts {
		return ""
	}

	summary.WriteString(t + author(ctx, url, pr))

	// Slack channel link (unless this is a status report about other users).
	if selfReport {
		if channelID, _ := data.SwitchURLAndID(ctx, url); channelID != "" {
			fmt.Fprintf(summary, "\n> <#%s>", channelID)
		}
	}

	// General PR details.
	now := workflow.Now(ctx).UTC()
	created, updated := times(now, url, pr)
	summary.WriteString(fmt.Sprintf("\n> Created `%s` ago", created))
	if updated != "" {
		fmt.Fprintf(summary, ", updated `%s` ago", updated)
	}

	if len(userIDs) == 1 {
		if activity := data.GetActivityTime(ctx, url, users.SlackIDToEmail(ctx, userIDs[0])); !activity.IsZero() {
			fmt.Fprintf(summary, ", touched by you `%s` ago", timeSince(now, activity))
		}
	}

	summary.WriteString(branch(url, pr))
	if s := states(ctx, url); s != "" {
		summary.WriteString(s)
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
		summary.WriteString("\n> ")
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

func isBitbucketPR(url string) bool {
	return strings.HasPrefix(url, "https://bitbucket.org/")
}

func title(url string, pr map[string]any) (string, bool) {
	prefix := "\n\n"

	draft, ok := pr["draft"].(bool)
	if ok && draft {
		prefix += ":construction: "
	}

	title, ok := pr["title"].(string)
	if !ok || len(strings.TrimSpace(title)) == 0 {
		title = url
	}

	title = strings.ReplaceAll(strings.TrimSpace(title), ">", "&gt;")
	return fmt.Sprintf("%s<%s|*%s*>", prefix, url, title), draft
}

func author(ctx workflow.Context, url string, pr map[string]any) string {
	if isBitbucketPR(url) {
		author, ok := pr["author"].(map[string]any)
		if !ok {
			return ""
		}
		id, ok := author["account_id"].(string)
		if !ok {
			return ""
		}
		name, ok := author["display_name"].(string)
		if !ok {
			name = ""
		}

		return " by " + users.BitbucketIDToSlackRef(ctx, id, name)
	}

	// GitHub.
	author, ok := pr["user"].(map[string]any)
	if !ok {
		return ""
	}
	login, ok := author["login"].(string)
	if !ok {
		return ""
	}
	userURL, ok := author["html_url"].(string)
	if !ok {
		userURL = ""
	}
	userType, ok := author["type"].(string)
	if !ok {
		userType = "User"
	}

	return " by " + users.GitHubIDToSlackRef(ctx, login, userURL, userType)
}

func times(now time.Time, url string, pr map[string]any) (created, updated string) {
	keySuffix := "at"
	if isBitbucketPR(url) {
		keySuffix = "on"
	}

	var ok bool
	created, ok = pr["created_"+keySuffix].(string)
	if !ok || created == "" {
		return "unknown", ""
	}
	updated, ok = pr["updated_"+keySuffix].(string)
	if !ok {
		return timeSince(now, created), ""
	}

	return timeSince(now, created), timeSince(now, updated)
}

func branch(url string, pr map[string]any) string {
	if isBitbucketPR(url) {
		prefix := "\n> Target branch: `"
		dest, ok := pr["destination"].(map[string]any)
		if !ok {
			return prefix + "unknown`"
		}
		branch, ok := dest["branch"].(map[string]any)
		if !ok {
			return prefix + "unknown`"
		}
		name, ok := branch["name"].(string)
		if !ok {
			return prefix + "unknown`"
		}

		return fmt.Sprintf("%s%s`", prefix, name)
	}

	// GitHub.
	prefix := "\n> Base branch: `"
	base, ok := pr["base"].(map[string]any)
	if !ok {
		return prefix + "unknown`"
	}
	ref, ok := base["ref"].(string)
	if !ok {
		return prefix + "unknown`"
	}

	return fmt.Sprintf("%s%s`", prefix, ref)
}

func states(ctx workflow.Context, url string) string {
	if isBitbucketPR(url) {
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

		if len(summary) > 0 {
			return fmt.Sprintf(", builds: :%s:", strings.Join(summary, ": :"))
		}
		return ""
	}

	// GitHub.
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
