package slack

import (
	"slices"
	"strings"
)

func DestinationDetails(pr map[string]any) (workspace, repo, branch, commit string) {
	// Workspace and repo slug.
	dest, ok := pr["destination"].(map[string]any)
	if !ok {
		return "", "", "", ""
	}
	m, ok := dest["repository"].(map[string]any)
	if !ok {
		return "", "", "", ""
	}
	fullName, ok := m["full_name"].(string)
	if !ok {
		return "", "", "", ""
	}
	workspace, repo, ok = strings.Cut(fullName, "/")
	if !ok {
		return "", "", "", ""
	}

	// Branch name.
	m, ok = dest["branch"].(map[string]any)
	if !ok {
		return workspace, repo, "", ""
	}
	branch, ok = m["name"].(string)
	if !ok {
		return workspace, repo, "", ""
	}

	// Commit hash.
	m, ok = dest["commit"].(map[string]any)
	if !ok {
		return workspace, repo, branch, ""
	}
	commit, _ = m["hash"].(string)

	return workspace, repo, branch, commit
}

func RequiredReviewers(paths []string, owners map[string][]string) []string {
	var required []string

	for _, p := range paths {
		required = append(required, owners[p]...)
	}

	slices.Sort(required)
	return slices.Compact(required)
}
