package bitbucket

import (
	"bytes"
	"encoding/json"
	"fmt"
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
			err, "PR", prURL, "Channel", fmt.Sprintf("`%s` (<#%s>)", prChannelID, prChannelID))
	}

	data.StorePRSnapshot(ctx, prURL, event.PullRequest)
	data.StoreDiffstat(ctx, prURL, Diffstat(ctx, event))

	email := users.BitbucketActorToEmail(ctx, event.Actor)
	if email == "" {
		logger.From(ctx).Error("initializing Bitbucket PR data without author's email",
			slog.String("pr_url", prURL), slog.String("account_id", event.Actor.AccountID))
		activities.AlertWarn(ctx, slackAlertsChannel, "Failed to determine a PR author's email address",
			"Bitbucket account ID", event.Actor.AccountID)
		return
	}

	data.InitTurns(ctx, prURL, email)
}

// accountIDs extracts the IDs from a slice of [Account]s. The output is guaranteed
// to be sorted, without repetitions, and not contain apps/bots or teams.
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

// LoadPRSnapshot reads a snapshot of a PR, which is used to detect and analyze metadata
// changes. If a snapshot doesn't exist, this function returns a nil map and no error.
func LoadPRSnapshot(ctx workflow.Context, prURL string) (*PullRequest, error) {
	prev, err := data.LoadPRSnapshot(ctx, prURL)
	if err != nil || prev == nil {
		return nil, err // Error may or may not be nil, but in either case there's no snapshot to return.
	}

	pr := new(PullRequest)
	if err := mapToStruct(prev, pr); err != nil {
		logger.From(ctx).Error("previous snapshot of Bitbucket PR is invalid",
			slog.Any("error", err), slog.String("pr_url", prURL))
		return nil, err
	}

	return pr, nil
}

// SwitchPRSnapshot stores the given new PR snapshot, and returns the previous one (if there is one).
func SwitchPRSnapshot(ctx workflow.Context, prURL string, snapshot PullRequest) (*PullRequest, error) {
	defer data.StorePRSnapshot(ctx, prURL, &snapshot) // Pointer - to include potential updates below.

	pr, err := LoadPRSnapshot(ctx, prURL)
	if err != nil || pr == nil {
		return nil, err // Error may or may not be nil, but in either case there's no snapshot to return.
	}

	// The "CommitCount" and "ChangeRequestCount" fields are populated and used by RevChat, not Bitbucket.
	// Persist them across snapshots (before the deferred call to [data.StorePRSnapshot]).
	if snapshot.CommitCount == 0 {
		snapshot.CommitCount = pr.CommitCount
	}
	if snapshot.ChangeRequestCount == 0 {
		snapshot.ChangeRequestCount = pr.ChangeRequestCount
	}

	return pr, nil
}

// FindPRsByCommit returns all (0 or more) the PR snapshots that are currently associated with the given commit hash.
func FindPRsByCommit(ctx workflow.Context, hash string) ([]*PullRequest, error) {
	ms, err := data.FindPRsByCommit(ctx, hash)
	if err != nil || ms == nil {
		return nil, err
	}

	prs := make([]*PullRequest, 0, len(ms))
	for _, m := range ms {
		pr := new(PullRequest)
		if err := mapToStruct(m, pr); err != nil {
			logger.From(ctx).Error("snapshot of Bitbucket PR is invalid",
				slog.Any("error", err), slog.String("commit_hash", hash))
			continue
		}
		prs = append(prs, pr)
	}

	return prs, nil
}

// mapToStruct converts a map-based representation of JSON data into a [PullRequest] struct.
func mapToStruct(m any, pr *PullRequest) error {
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(m); err != nil {
		return err
	}

	if err := json.NewDecoder(buf).Decode(pr); err != nil {
		return err
	}

	return nil
}
