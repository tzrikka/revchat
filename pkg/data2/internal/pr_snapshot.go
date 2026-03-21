package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// StorePRSnapshot writes a snapshot of a PR, which is used to detect and analyze metadata changes.
func StorePRSnapshot(_ context.Context, urlWithSuffix string, pr any) error {
	path, err := dataPath(urlWithSuffix)
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

// LoadPRSnapshot reads a snapshot of a PR, which is used to detect and analyze metadata
// changes. If a snapshot doesn't exist, this function returns a nil map and no error.
func LoadPRSnapshot(_ context.Context, urlWithSuffix string) (map[string]any, error) {
	path, err := dataPath(urlWithSuffix)
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
