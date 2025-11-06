package bitbucket

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/revchat/pkg/config"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack"
	"github.com/tzrikka/xdg"
)

func commitCommentCreatedWorkflow(ctx workflow.Context, event RepositoryEvent) error {
	log.Warn(ctx, "Bitbucket commit comment created event - not implemented yet")
	return nil
}

func commitStatusWorkflow(ctx workflow.Context, event RepositoryEvent) error {
	// Commit status --> commit hash --> PR snapshot (JSON map) --> [PullRequest] struct.
	cs := event.CommitStatus
	m, err := findPRByCommit(ctx, cs.Commit.Hash)
	if err != nil {
		return err
	}
	if m == nil {
		log.Debug(ctx, "PR not found for commit status", "hash", cs.Commit.Hash, "build_name", cs.Name, "build_url", cs.URL)
		// Not an error: the commit may not belong to any open PR,
		// or may be obsoleted by a newer commit in the snapshot.
		return nil
	}

	pr := new(PullRequest)
	if err := mapToStruct(m, pr); err != nil {
		log.Error(ctx, "invalid Bitbucket PR", "error", err, "pr_url", prURL(m))
		return err
	}

	// If we're not tracking this PR, there's no need/way to announce this event.
	channelID, found := lookupChannel(ctx, event.Type, *pr)
	if !found {
		return nil
	}

	url := pr.Links["html"].HRef
	status := data.CommitStatus{State: cs.State, Desc: cs.Description, URL: cs.URL}
	if err := data.UpdateBitbucketBuilds(url, cs.Commit.Hash, cs.Name, status); err != nil {
		log.Error(ctx, "failed to update Bitbucket build states", "error", err, "pr_url", url, "commit_hash", cs.Commit.Hash)
		// Continue anyway.
	}

	var msg string
	switch cs.State {
	case "INPROGRESS":
		msg = ":hourglass_flowing_sand:"
	case "SUCCESSFUL":
		msg = ":large_green_circle:"
	default: // "FAILED", "STOPPED".
		msg = ":red_circle:"
	}
	msg = fmt.Sprintf(`%s "%s" build status: <%s|%s>`, msg, cs.Name, cs.URL, cs.Description)
	_, err = slack.PostMessage(ctx, channelID, msg)
	return err
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
		snapshot, err := data.LoadBitbucketPR(url)
		if err != nil {
			log.Error(ctx, "failed to load Bitbucket PR snapshot for reminder", "error", err, "pr_url", url)
			return nil // Continue walking.
		}

		prHash, ok := prCommitHash(snapshot)
		if !ok {
			return nil
		}
		if strings.HasPrefix(eventHash, prHash) {
			if pr != nil {
				log.Warn(ctx, "commit hash collision", "hash", eventHash, "existing_pr", prURL(pr), "new_pr", prURL(snapshot))
				return nil // Continue walking.
			}
			pr = snapshot
		}

		return nil
	})

	return
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

func prURL(pr map[string]any) string {
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
