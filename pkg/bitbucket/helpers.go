package bitbucket

import (
	"bytes"
	"encoding/json"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/revchat/pkg/data"
)

// switchSnapshot stores the given new PR snapshot, and returns the previous one (if any).
func switchSnapshot(ctx workflow.Context, url string, snapshot PullRequest) (*PullRequest, error) {
	defer func() { _ = data.StoreBitbucketPR(url, snapshot) }()

	prev, err := data.LoadBitbucketPR(url)
	if err != nil {
		log.Error(ctx, "failed to load Bitbucket PR snapshot", "error", err, "pr_url", url)
		return nil, err
	}

	if prev == nil {
		return nil, nil
	}

	pr := new(PullRequest)
	if err := mapToStruct(prev, pr); err != nil {
		log.Error(ctx, "previous snapshot of Bitbucket PR is invalid", "error", err, "pr_url", url)
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

func htmlURL(links map[string]Link) string {
	return links["html"].HRef
}
