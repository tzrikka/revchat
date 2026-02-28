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
	"github.com/tzrikka/revchat/pkg/bitbucket/activities"
	"github.com/tzrikka/revchat/pkg/config"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/files"
	"github.com/tzrikka/revchat/pkg/users"
	"github.com/tzrikka/xdg"
)

// LoadPRTurns scans all stored PR turn files, and returns a map of
// Slack user IDs to all the PR URLs they need to be reminded about.
func LoadPRTurns(ctx workflow.Context, onlyCurrentTurn, authors, reviewers bool) map[string][]string {
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
		if onlyCurrentTurn {
			emails, err = data.GetCurrentTurns(ctx, prURL)
		} else {
			emails, err = data.GetAllTurns(ctx, prURL, authors, reviewers)
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
// The output is empty if the PR is in draft mode and the "reportDrafts" parameter is false.
// The "thrippyID" parameter is only used to report Bitbucket PR tasks, and can be left empty otherwise.
func PRDetails(ctx workflow.Context, url string, userIDs []string, selfReport, showDrafts, showTasks bool, thrippyID string) string {
	summary := new(strings.Builder)
	pr, err := data.LoadPRSnapshot(ctx, url)
	if err != nil {
		fmt.Fprintf(summary, "\n\n<%s|*%s*>", url, url)
		if selfReport {
			if channelID, _ := data.SwitchURLAndID(ctx, url); channelID != "" {
				fmt.Fprintf(summary, "\n><#%s>", channelID)
			}
		}
		summary.WriteString("\n>:warning: Failed to read internal data about this PR")
		return summary.String()
	}

	// Abort if this is a draft but RevChat isn't configured to report drafts.
	t, draft := title(url, pr)
	if draft && !showDrafts {
		return ""
	}

	summary.WriteString(t + author(ctx, url, pr))

	// Slack channel link (unless this is a status report about other users).
	if selfReport {
		if channelID, _ := data.SwitchURLAndID(ctx, url); channelID != "" {
			fmt.Fprintf(summary, "\n><#%s>", channelID)
		}
	}

	// Timestamps.
	now := workflow.Now(ctx).UTC()
	created, updated := times(now, url, pr)
	fmt.Fprintf(summary, "\n>Created `%s` ago", created)
	if updated != "" {
		fmt.Fprintf(summary, ", updated `%s` ago", updated)
	}

	if len(userIDs) == 1 {
		if activity := data.GetActivityTime(ctx, url, users.SlackIDToEmail(ctx, userIDs[0])); !activity.IsZero() {
			fmt.Fprintf(summary, ", touched by you `%s` ago", timeSince(now, activity))
		}
	}

	// File-related details.
	summary.WriteString(branchNameMarkdown(ctx, url, pr))
	paths := data.ReadBitbucketDiffstatPaths(url)
	if len(paths) > 0 {
		owner, repo, branch, commit := PRIdentifiers(ctx, url, pr)

		fullNames := make([]string, 0, len(userIDs))
		for _, userID := range userIDs {
			if name := users.SlackIDToRealName(ctx, userID); name != "" {
				fullNames = append(fullNames, name)
			}
		}

		if owned := files.CountOwnedFiles(ctx, owner, repo, branch, commit, fullNames, paths); owned > 0 {
			fmt.Fprintf(summary, ", code owners: *%d* file", owned)
			if owned > 1 {
				summary.WriteString("s")
			}
		}

		if highRisk := files.CountHighRiskFiles(ctx, owner, repo, branch, commit, paths); highRisk > 0 {
			fmt.Fprintf(summary, ", high risk: *%d* file", highRisk)
			if highRisk > 1 {
				summary.WriteString("s")
			}
		}
	}

	if s := states(ctx, url); s != "" {
		summary.WriteString(s)
	}

	// Review details.
	tasks := prTasks(ctx, showTasks, thrippyID, url, pr)
	approvers := prApprovers(ctx, pr)
	changeRequests := prChangeRequests()

	tasksCount := len(tasks)
	approversCount := len(approvers)
	changeRequestsCount := len(changeRequests)

	if tasksCount+approversCount+changeRequestsCount > 0 {
		summary.WriteString("\n>")
	}
	if tasksCount > 0 {
		fmt.Fprintf(summary, "Tasks: *%d*", tasksCount)
	}
	if approversCount > 0 {
		if tasksCount == 0 {
			summary.WriteString("A")
		} else {
			summary.WriteString(", a")
		}
		fmt.Fprintf(summary, "pprovals: *%d* (%s)", approversCount, strings.Join(approvers, ", "))
	}
	if changeRequestsCount > 0 {
		if tasksCount+approversCount == 0 {
			summary.WriteString("C")
		} else {
			summary.WriteString(", c")
		}
		fmt.Fprintf(summary, "hange requests: *%d* (%s)", changeRequestsCount, strings.Join(changeRequests, ", "))
	}
	if showTasks && tasksCount > 0 {
		summary.WriteString(strings.Join(tasks, ""))
	}

	return summary.String()
}

// PRIdentifiers extracts the workspace/owner, repo, destination branch, and
// destination commit hash from the given snapshot of a Bitbucket or GitHub PR.
func PRIdentifiers(ctx workflow.Context, url string, pr map[string]any) (owner, repo, branch, hash string) {
	m, ok := branchMap(ctx, url, pr)
	if !ok {
		return "", "", "", ""
	}

	owner, repo, ok = branchOwnerAndRepo(ctx, url, m)
	if !ok {
		return "", "", "", ""
	}

	branch, ok = branchName(ctx, url, m)
	if !ok {
		return owner, repo, "", ""
	}

	// Bitbucket commit hash.
	if isBitbucketPR(url) {
		commit, ok := m["commit"].(map[string]any)
		if !ok {
			logger.From(ctx).Warn("missing/invalid commit in PR snapshot", slog.String("pr_url", url))
			return owner, repo, branch, ""
		}
		hash, ok = commit["hash"].(string)
		if !ok {
			logger.From(ctx).Warn("missing/invalid commit hash in PR snapshot", slog.String("pr_url", url))
			return owner, repo, branch, ""
		}

		return owner, repo, branch, hash
	}

	// GitHub commit hash.
	hash, ok = m["sha"].(string)
	if !ok {
		logger.From(ctx).Warn("missing/invalid commit sha in PR snapshot", slog.String("pr_url", url))
		return owner, repo, branch, ""
	}

	return owner, repo, branch, hash
}

func isBitbucketPR(url string) bool {
	return strings.HasPrefix(url, "https://bitbucket.org/")
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

func branchMap(ctx workflow.Context, url string, pr map[string]any) (map[string]any, bool) {
	field := "base" // GitHub.
	if isBitbucketPR(url) {
		field = "destination"
	}

	m, ok := pr[field].(map[string]any)
	if !ok {
		logger.From(ctx).Warn("missing/invalid destination/base branch in PR snapshot",
			slog.String("pr_url", url), slog.String("field_name", field))
		return nil, false
	}

	return m, true
}

func branchName(ctx workflow.Context, url string, branch map[string]any) (string, bool) {
	if isBitbucketPR(url) {
		m, ok := branch["branch"].(map[string]any)
		if !ok {
			logger.From(ctx).Warn("missing/invalid branch subsection in PR snapshot", slog.String("pr_url", url))
			return "unknown", false
		}
		name, ok := m["name"].(string)
		if !ok {
			logger.From(ctx).Warn("missing/invalid branch name in PR snapshot", slog.String("pr_url", url))
			return "unknown", false
		}
		return name, true
	}

	// GitHub.
	ref, ok := branch["ref"].(string)
	if !ok {
		logger.From(ctx).Warn("missing/invalid branch ref in PR snapshot", slog.String("pr_url", url))
		return "unknown", false
	}
	return ref, true
}

func branchNameMarkdown(ctx workflow.Context, url string, pr map[string]any) string {
	prefix := "\n>Base branch" // GitHub.
	if isBitbucketPR(url) {
		prefix = "\n>Target branch"
	}

	m, ok := branchMap(ctx, url, pr)
	if !ok {
		return fmt.Sprintf("%s: `%s`", prefix, "unknown")
	}

	name, _ := branchName(ctx, url, m)
	return fmt.Sprintf("%s: `%s`", prefix, name)
}

func branchOwnerAndRepo(ctx workflow.Context, url string, branch map[string]any) (owner, repo string, ok bool) {
	field := "repo" // GitHub.
	if isBitbucketPR(url) {
		field = "repository"
	}

	m, ok := branch[field].(map[string]any)
	if !ok {
		logger.From(ctx).Warn("missing/invalid branch repo in PR snapshot", slog.String("pr_url", url))
		return "", "", false
	}

	fullName, ok := m["full_name"].(string)
	if !ok {
		logger.From(ctx).Warn("missing/invalid repo full name in PR snapshot", slog.String("pr_url", url))
		return "", "", false
	}

	owner, repo, ok = strings.Cut(fullName, "/")
	if !ok {
		logger.From(ctx).Warn("invalid repo full name in PR snapshot",
			slog.String("pr_url", url), slog.String("full_name", fullName))
		return "", "", false
	}

	return owner, repo, true
}

const (
	buildSuccessful = "large_green_circle"
	buildInProgress = "large_yellow_circle"
	buildFailed     = "red_circle"
)

func states(ctx workflow.Context, url string) string {
	if isBitbucketPR(url) {
		prStatus := data.ReadBitbucketBuilds(ctx, url)
		keys := slices.Sorted(maps.Keys(prStatus.Builds))
		var summary []string
		for _, k := range keys {
			switch s := prStatus.Builds[k].State; s {
			case "SUCCESSFUL":
				summary = append(summary, buildSuccessful)
			case "INPROGRESS":
				summary = append(summary, buildInProgress)
			default: // "FAILED", "STOPPED".
				summary = append(summary, buildFailed)
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

func times(now time.Time, url string, pr map[string]any) (created, updated string) {
	keySuffix := "at" // GitHub.
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

func prApprovers(ctx workflow.Context, pr map[string]any) []string {
	participants, ok := pr["participants"].([]any)
	if !ok {
		return nil
	}

	var mentions []string
	for _, p := range participants {
		participant, ok := p.(map[string]any)
		if !ok {
			continue
		}
		approved, ok := participant["approved"].(bool)
		if !ok || !approved {
			continue
		}
		user, ok := participant["user"].(map[string]any)
		if !ok {
			continue
		}
		accountID, ok := user["account_id"].(string)
		if !ok {
			continue
		}

		mention := users.BitbucketIDToSlackRef(ctx, accountID, "")
		if mention != "" {
			mentions = append(mentions, mention)
		}
	}

	return mentions
}

func prChangeRequests() []string {
	return nil
}

// prTasks returns a list of formatted strings describing the open tasks in the specified Bitbucket PR.
// It returns nil in all other cases: not a Bitbucket PR, no open tasks, or no need to enumerate them.
func prTasks(ctx workflow.Context, showTasks bool, thrippyID, url string, pr map[string]any) []string {
	if !isBitbucketPR(url) {
		return nil
	}

	// We can't use the Bitbucket API for each PR in each report due to rate limits,
	// so we rely on the stored PR snapshot as an initial filter.
	n, ok := pr["task_count"].(float64)
	if !ok {
		logger.From(ctx).Warn("missing/invalid task count in Bitbucket PR snapshot", slog.String("url", url))
		return nil
	}

	count := int(n)
	if count == 0 {
		return nil
	}
	if !showTasks {
		// Placeholder list with the correct number of tasks,
		// because we don't care about their details.
		return make([]string, count)
	}

	tasks, err := activities.ListPullRequestTasks(ctx, thrippyID, url)
	if err != nil {
		return slices.Repeat([]string{"\n> •   (Error reading task details)"}, count)
	}

	lines := make([]string, 0, count)
	for _, task := range tasks {
		if task.State == "RESOLVED" {
			continue
		}

		t := task.UpdatedOn
		if t.IsZero() {
			t = task.CreatedOn
		}
		ago := ""
		if !t.IsZero() {
			ago = fmt.Sprintf("<!date^%d^{ago}|%s ago>", t.Unix(), timeSince(workflow.Now(ctx), t))
		}

		text := task.Content.Raw
		creator := users.BitbucketIDToSlackRef(ctx, task.Creator.AccountID, task.Creator.DisplayName)
		lines = append(lines, fmt.Sprintf("\n> •   %s (by %s %s)", text, creator, ago))
	}

	return lines
}
