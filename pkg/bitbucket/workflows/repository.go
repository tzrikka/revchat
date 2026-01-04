package workflows

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/bitbucket"
	"github.com/tzrikka/revchat/pkg/config"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack/activities"
	"github.com/tzrikka/xdg"
)

// CommitCommentCreatedWorkflow (will) handle (in the future) this event:
// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Commit-comment-created
func CommitCommentCreatedWorkflow(ctx workflow.Context, event bitbucket.RepositoryEvent) error {
	logger.From(ctx).Debug("Bitbucket commit comment created event - not implemented yet")
	return nil
}

// CommitStatusWorkflow mirrors build/commit status updates in the corresponding PR's Slack channel:
//   - https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Build-status-created
//   - https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Build-status-updated
func CommitStatusWorkflow(ctx workflow.Context, event bitbucket.RepositoryEvent) error {
	// Commit status --> commit hash --> PR snapshot (JSON map) --> [PullRequest] struct.
	cs := event.CommitStatus
	m, err := findPRByCommit(ctx, cs.Commit.Hash)
	if err != nil {
		return err
	}
	if m == nil {
		logger.From(ctx).Debug("PR not found for commit status", slog.String("hash", cs.Commit.Hash),
			slog.String("build_name", cs.Name), slog.String("build_url", cs.URL))
		// Not an error: the commit may not belong to any open PR,
		// or may be obsoleted by a newer commit in the snapshot.
		return nil
	}

	pr := new(bitbucket.PullRequest)
	if err := bitbucket.MapToStruct(m, pr); err != nil {
		logger.From(ctx).Error("invalid Bitbucket PR", slog.Any("error", err), slog.String("pr_url", urlFromPR(m)))
		return err
	}

	// If we're not tracking this PR, there's no need/way to announce this event.
	prURL := bitbucket.HTMLURL(pr.Links)
	channelID, found := activities.LookupChannel(ctx, event.Type, prURL)
	if !found {
		return nil
	}

	defer bitbucket.UpdateChannelBuildsBookmark(ctx, channelID, prURL)

	status := data.CommitStatus{Name: cs.Name, State: cs.State, Desc: cs.Description, URL: cs.URL}
	data.UpdateBitbucketBuilds(ctx, prURL, cs.Commit.Hash, cs.Key, status)

	desc, _, _ := strings.Cut(cs.Description, "\n")
	msg := fmt.Sprintf(`%s "%s" build status: <%s|%s>`, buildStateEmoji(cs.State), cs.Name, cs.URL, desc)
	err = activities.PostMessage(ctx, channelID, msg)

	// If the channel is archived but we still store data for it, clean it up.
	if err != nil && strings.Contains(err.Error(), "is_archived") {
		data.CleanupPRData(ctx, channelID, prURL)
		return nil
	}

	// Other than announcing this specific event, also announce if the PR is ready to be merged
	// (all builds are successful, the PR has at least 2 approvals, and no pending action items).
	if cs.State != "SUCCESSFUL" || !allBuildsSuccessful(ctx, prURL) || pr.ChangeRequestCount > 0 || pr.TaskCount > 0 {
		return err
	}
	approvers := 0
	for _, p := range pr.Participants {
		if p.Approved {
			approvers++
		}
	}
	if approvers < 2 {
		return err
	}

	logger.From(ctx).Info("Bitbucket PR is ready to be merged", slog.String("pr_url", prURL))
	return activities.PostMessage(ctx, channelID, "<!here> this PR is ready to be merged! :tada:")
}

func findPRByCommit(ctx workflow.Context, eventHash string) (pr map[string]any, err error) {
	root := filepath.Join(xdg.MustDataHome(), config.DirName)
	err = fs.WalkDir(os.DirFS(root), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(d.Name(), "_snapshot.json") {
			return nil
		}

		url := "https://" + strings.TrimSuffix(path, "_snapshot.json")
		snapshot, err := data.LoadBitbucketPR(ctx, url)
		if err != nil {
			return nil // Continue walking.
		}

		prHash, ok := prCommitHash(snapshot)
		if !ok {
			return nil
		}

		if strings.HasPrefix(eventHash, prHash) {
			if pr != nil {
				logger.From(ctx).Warn("commit hash collision", slog.String("hash", eventHash),
					slog.String("existing_pr", urlFromPR(pr)), slog.String("new_pr", urlFromPR(snapshot)))
				return nil // Continue walking.
			}
			pr = snapshot
		}

		return nil
	})

	return pr, err
}

func prCommitHash(pr map[string]any) (string, bool) {
	source, ok := pr["source"].(map[string]any)
	if !ok {
		return "", false
	}
	commit, ok := source["commit"].(map[string]any)
	if !ok {
		return "", false
	}
	hash, ok := commit["hash"].(string)
	if !ok {
		return "", false
	}

	return hash, true
}

func urlFromPR(pr map[string]any) string {
	links, ok := pr["links"].(map[string]any)
	if !ok {
		return ""
	}
	html, ok := links["html"].(map[string]any)
	if !ok {
		return ""
	}
	href, ok := html["href"].(string)
	if !ok {
		return ""
	}

	return href
}

func buildStateEmoji(state string) string {
	switch state {
	case "INPROGRESS":
		return ":hourglass_flowing_sand:"
	case "SUCCESSFUL":
		return ":large_green_circle:"
	default: // "FAILED", "STOPPED".
		return ":red_circle:"
	}
}

func allBuildsSuccessful(ctx workflow.Context, url string) bool {
	prStatus := data.ReadBitbucketBuilds(ctx, url)
	if prStatus == nil || len(prStatus.Builds) < 2 {
		return false
	}

	for _, build := range prStatus.Builds {
		if build.State != "SUCCESSFUL" {
			return false
		}
	}

	return true
}
