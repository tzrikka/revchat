package bitbucket

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/revchat/pkg/config"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/xdg"
)

func commitCommentCreatedWorkflow(ctx workflow.Context, event RepositoryEvent) error {
	log.Warn(ctx, "Bitbucket commit comment created event - not implemented yet")
	return nil
}

func commitStatusWorkflow(ctx workflow.Context, event RepositoryEvent) error {
	cs := event.CommitStatus
	pr, err := findPRByCommit(ctx, cs.Commit.Hash)
	if err != nil {
		return err
	}
	if pr == nil {
		log.Debug(ctx, "PR not found for commit status", "hash", cs.Commit.Hash)
		// Not an error: the commit may not belong to any open PR,
		// or may be obsoleted by a newer commit in the snapshot.
		return nil
	}

	eventType := strings.TrimPrefix(event.Type, "commit_status_")
	if eventType != "created" && eventType != "updated" {
		log.Error(ctx, "unrecognized Bitbucket commit status type", "event", event.Type)
		return errors.New("unrecognized Bitbucket commit status type: " + event.Type)
	}

	log.Info(ctx, "Bitbucket commit status "+eventType,
		"pr_url", prURL(pr), "build_name", cs.Name, "build_state", cs.State,
		"build_desc", cs.Description, "build_url", cs.URL)

	return nil
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
