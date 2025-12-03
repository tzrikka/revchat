package files

import (
	"strings"

	"go.temporal.io/sdk/workflow"
)

// CountHighRiskFiles counts how many of the given file paths are considered high risk,
// according to the "highrisk.txt" file in the given branch (a PR's destination).
func CountHighRiskFiles(ctx workflow.Context, workspace, repo, branch, commit string, paths []string) int {
	hr := parseHighRiskFile(getBitbucketSourceFile(ctx, workspace, repo, branch, commit, "highrisk.txt"))

	count := 0
	for _, p := range paths {
		if isHighRisk(hr, p) {
			count++
		}
	}
	return count
}

func parseHighRiskFile(fileContent string) []string {
	if fileContent == "" {
		return nil
	}

	var paths []string
	for line := range strings.Lines(fileContent) {
		if line := strings.TrimSpace(line); line != "" {
			paths = append(paths, line)
		}
	}
	return paths
}

func isHighRisk(highRisk []string, filePath string) bool {
	for _, highRiskPath := range highRisk {
		if strings.HasPrefix(filePath, highRiskPath) {
			return true
		}
	}
	return false
}
