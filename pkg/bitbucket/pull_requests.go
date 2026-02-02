package bitbucket

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"slices"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack/activities"
	"github.com/tzrikka/revchat/pkg/users"
)

// InitPRData saves the initial state of a new PR: snapshots of PR metadata,
// and a 2-way ID mapping for syncs between Bitbucket and Slack. If there are
// errors, they are logged but ignored, as we can try to recreate the data later.
func InitPRData(ctx workflow.Context, event PullRequestEvent, prChannelID, slackAlertsChannel string) {
	prURL := HTMLURL(event.PullRequest.Links)
	if err := data.MapURLAndID(ctx, prURL, prChannelID); err != nil {
		_ = activities.AlertError(ctx, slackAlertsChannel, "failed to set mapping between a PR and its Slack channel",
			err, "PR URL", prURL, "Slack channel ID", prChannelID)
	}

	data.StoreBitbucketPR(ctx, prURL, event.PullRequest)
	if err := data.UpdateBitbucketDiffstat(prURL, Diffstat(ctx, event)); err != nil {
		logger.From(ctx).Error("failed to create Bitbucket PR diffstat",
			slog.Any("error", err), slog.String("pr_url", prURL))
	}

	email := users.BitbucketIDToEmail(ctx, event.Actor.AccountID)
	if email == "" {
		logger.From(ctx).Error("initializing Bitbucket PR data without author's email",
			slog.String("pr_url", prURL), slog.String("account_id", event.Actor.AccountID))
		activities.AlertWarn(ctx, slackAlertsChannel, "Failed to determine a PR author's email address!",
			"Bitbucket account ID", event.Actor.AccountID)
		return
	}

	data.InitTurns(ctx, prURL, email)
}

// accountIDs extracts the IDs from a slice of [Account]s.
// The output is guaranteed to be sorted, without repetitions, and not contain teams/apps.
func accountIDs(accounts []Account) []string {
	if len(accounts) == 0 {
		return nil
	}

	ids := make([]string, 0, len(accounts))
	for _, a := range accounts {
		if a.Type == "user" || a.Type == "" {
			ids = append(ids, a.AccountID)
		}
	}

	slices.Sort(ids)
	return slices.Compact(ids)
}

func HTMLURL(links map[string]Link) string {
	return links["html"].HRef
}

// MapToStruct converts a map-based representation of JSON data into a [PullRequest] struct.
func MapToStruct(m any, pr *PullRequest) error {
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(m); err != nil {
		return err
	}

	if err := json.NewDecoder(buf).Decode(pr); err != nil {
		return err
	}

	return nil
}

// SwitchSnapshot stores the given new PR snapshot, and returns the previous one (if there is one).
func SwitchSnapshot(ctx workflow.Context, prURL string, snapshot PullRequest) (*PullRequest, error) {
	defer data.StoreBitbucketPR(ctx, prURL, snapshot)

	prev, err := data.LoadBitbucketPR(ctx, prURL)
	if err != nil {
		return nil, err
	}
	if prev == nil {
		return nil, nil
	}

	pr := new(PullRequest)
	if err := MapToStruct(prev, pr); err != nil {
		logger.From(ctx).Error("previous snapshot of Bitbucket PR is invalid",
			slog.Any("error", err), slog.String("pr_url", prURL))
		return nil, err
	}

	// the "CommitCount" and "ChangeRequestCount" fields are populated and used by RevChat, not Bitbucket.
	// Persist them across snapshots (before the deferred call to [data.StoreBitbucketPR]).
	if snapshot.CommitCount == 0 {
		snapshot.CommitCount = pr.CommitCount
	}
	if snapshot.ChangeRequestCount == 0 {
		snapshot.ChangeRequestCount = pr.ChangeRequestCount
	}

	return pr, nil
}
