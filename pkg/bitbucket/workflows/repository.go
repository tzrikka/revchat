package workflows

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/cache"
	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/bitbucket"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack/activities"
)

// We don't want to spam the channel with "ready to merge" messages in times of frequent
// builds, so we throttle these messages to at most once per hour per PR. This is not
// a critical or common need, so a non-persistent in-memory cache is good enough.
var mergeReadiness = cache.New[bool](time.Hour, cache.DefaultCleanupInterval)

// CommitCommentCreatedWorkflow (will) handle (in the future) this event:
// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Commit-comment-created
func CommitCommentCreatedWorkflow(ctx workflow.Context, _ bitbucket.RepositoryEvent) error {
	logger.From(ctx).Debug("Bitbucket commit comment created event - not implemented yet")
	return nil
}

// CommitStatusWorkflow mirrors build/commit status updates in the corresponding PR's Slack channel:
//   - https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Build-status-created
//   - https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Build-status-updated
func (c Config) CommitStatusWorkflow(ctx workflow.Context, event bitbucket.RepositoryEvent) error {
	// Commit status --> commit hash --> 0 or more [bitbucket.PullRequest] instances.
	cs := event.CommitStatus
	prs, err := bitbucket.FindPRsByCommit(ctx, cs.Commit.Hash)
	if err != nil {
		return activities.AlertError(ctx, c.SlackAlertsChannel, "failed to associate commit hash with PR", err)
	}

	if len(prs) == 0 {
		logger.From(ctx).Debug("PR not found for commit status", slog.String("hash", cs.Commit.Hash),
			slog.String("build_name", cs.Name), slog.String("build_url", cs.URL))
		// This is not a problem: the commit may not belong to any open PR,
		// or may be obsoleted by a newer commit in the snapshot.
		return nil
	}

	for _, pr := range prs {
		err = errors.Join(err, updateCommitStatus(ctx, cs, pr))
	}

	return err
}

func updateCommitStatus(ctx workflow.Context, cs *bitbucket.CommitStatus, pr *bitbucket.PullRequest) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	prURL := bitbucket.HTMLURL(pr.Links)
	channelID, found := activities.LookupChannel(ctx, prURL)
	if !found {
		return nil
	}

	defer bitbucket.UpdateChannelBuildsBookmark(ctx, channelID, prURL)

	status := data.CommitStatus{Name: cs.Name, State: cs.State, Desc: cs.Description, URL: cs.URL}
	data.UpdateBitbucketBuilds(ctx, prURL, cs.Commit.Hash, cs.Key, status)

	desc, _, _ := strings.Cut(cs.Description, "\n")
	msg := fmt.Sprintf(`%s "%s" build status: <%s|%s>`, buildStateEmoji(cs.State), cs.Name, cs.URL, desc)
	err := activities.PostMessage(ctx, channelID, msg)

	// If the channel is archived but we still store data for it, clean it up. We don't consider this a server error.
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

	if announced, _ := mergeReadiness.Get(prURL); announced {
		return err
	}

	logger.From(ctx).Info("Bitbucket PR is ready to be merged", slog.String("pr_url", prURL))
	if err := activities.PostMessage(ctx, channelID, "<!here> this PR is ready to be merged! :tada:"); err != nil {
		return err
	}

	mergeReadiness.Set(prURL, true, cache.DefaultExpiration)
	return nil
}

// IssueCreatedWorkflow (will) handle (in the future) this event:
// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Created
func IssueCreatedWorkflow(ctx workflow.Context, _ bitbucket.RepositoryEvent) error {
	logger.From(ctx).Debug("Bitbucket issue created event - not implemented yet")
	return nil
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
	if len(prStatus.Builds) < 2 {
		return false
	}

	for _, build := range prStatus.Builds {
		if build.State != "SUCCESSFUL" {
			return false
		}
	}

	return true
}
