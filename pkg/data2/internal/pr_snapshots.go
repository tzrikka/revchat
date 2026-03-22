package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/tzrikka/revchat/pkg/config"
	"github.com/tzrikka/xdg"
)

const (
	PRSnapshotFileSuffix = "_snapshot.json"
)

// WritePRSnapshot writes a snapshot of a PR, which is used to detect and analyze metadata changes.
func WritePRSnapshot(_ context.Context, prURL string, pr any) error {
	mu := dataFileMutexes.Get(prURL + PRSnapshotFileSuffix)
	mu.Lock()
	defer mu.Unlock()

	path, err := dataPath(prURL + PRSnapshotFileSuffix)
	if err != nil {
		return fmt.Errorf("failed to get PR snapshot path: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600) //gosec:disable G304 // Verified URL, and suffix is hardcoded.
	if err != nil {
		return fmt.Errorf("failed to open PR snapshot: %w", err)
	}
	defer f.Close()

	e := json.NewEncoder(f)
	e.SetIndent("", "  ")

	if err := e.Encode(pr); err != nil {
		return fmt.Errorf("failed to write PR snapshot: %w", err)
	}

	return nil
}

// ReadPRSnapshot reads a snapshot of a PR, which is used to detect and analyze metadata
// changes. If a snapshot doesn't exist, this function returns a nil map and no error.
func ReadPRSnapshot(_ context.Context, prURL string) (map[string]any, error) {
	mu := dataFileMutexes.Get(prURL + PRSnapshotFileSuffix)
	mu.Lock()
	defer mu.Unlock()

	path, err := dataPath(prURL + PRSnapshotFileSuffix)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR snapshot path: %w", err)
	}

	f, err := os.Open(path) //gosec:disable G304 // URL received from signature-verified 3rd-party, suffix is hardcoded.
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open PR snapshot: %w", err)
	}
	defer f.Close()

	pr := map[string]any{}
	if err := json.NewDecoder(f).Decode(&pr); err != nil {
		return nil, fmt.Errorf("failed to read PR snapshot: %w", err)
	}

	if len(pr) == 0 {
		return nil, nil
	}
	return pr, nil
}

// FindPRsByCommit returns all (0 or more) the PR snapshots that are currently associated with the given
// commit hash. This is used when processing commit events, to identify the relevant PR(s). To do this,
// this function scans through all the PR snapshots and checks their current commit hashes.
func FindPRsByCommit(ctx context.Context, hash string) (prs []map[string]any, err error) {
	root, err := xdg.CreateDir(xdg.DataHome, config.DirName)
	if err != nil {
		return nil, err
	}

	err = fs.WalkDir(os.DirFS(root), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(d.Name(), PRSnapshotFileSuffix) {
			return nil
		}

		prURL := "https://" + strings.TrimSuffix(path, PRSnapshotFileSuffix)
		snapshot, err := ReadPRSnapshot(ctx, prURL)
		if err != nil || snapshot == nil {
			return nil
		}

		// The hash from the event (hash) is always the full commit hash, but the one in the snapshot (prHash) may be truncated.
		if prHash := prCommitHash(snapshot); prHash != "" && strings.HasPrefix(hash, prHash) {
			prs = append(prs, snapshot)
		}

		return nil
	})

	return prs, err
}

func prCommitHash(pr map[string]any) string {
	if hash := prCommitHashBitbucket(pr); hash != "" {
		return hash
	}
	return prCommitHashGitHub(pr)
}

func prCommitHashBitbucket(pr map[string]any) string {
	source, ok := pr["source"].(map[string]any)
	if !ok {
		return ""
	}
	commit, ok := source["commit"].(map[string]any)
	if !ok {
		return ""
	}
	hash, ok := commit["hash"].(string)
	if !ok {
		return ""
	}

	return hash
}

func prCommitHashGitHub(pr map[string]any) string {
	head, ok := pr["head"].(map[string]any)
	if !ok {
		return ""
	}
	sha, ok := head["sha"].(string)
	if !ok {
		return ""
	}

	return sha
}

func urlFromPR(pr map[string]any) string {
	if url := urlFromPRBitbucket(pr); url != "" {
		return url
	}
	return urlFromPRGitHub(pr)
}

func urlFromPRBitbucket(pr map[string]any) string {
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

func urlFromPRGitHub(pr map[string]any) string {
	html, ok := pr["html_url"].(string)
	if !ok {
		return ""
	}

	return html
}
