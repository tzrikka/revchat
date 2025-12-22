package files

import (
	"log/slog"
	"regexp"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
)

type CodeOwners struct {
	PathList   []string
	IgnoreList []string

	Groups map[string][]string
	Paths  map[string][]string
	Users  map[string]bool
}

// CountOwnedFiles counts how many of the given file paths are owned by the specified
// user, according to the "CODEOWNERS" file in the given branch (a PR's destination).
func CountOwnedFiles(ctx workflow.Context, workspace, repo, branch, commit, userName string, paths []string) int {
	if userName == "" {
		return 0
	}

	c := parseCodeOwnersFile(ctx, getBitbucketSourceFile(ctx, workspace, repo, branch, commit, "CODEOWNERS"), true)
	if c == nil {
		return 0
	}

	if _, found := c.Users[userName]; !found {
		return 0
	}

	count := 0
	for _, p := range paths {
		owners, err := c.getOwners(p)
		if err != nil {
			logger.From(ctx).Error("failed to check CODEOWNERS path pattern",
				slog.Any("error", err), slog.String("file_path", p))
			return 0
		}
		if slices.Contains(owners, userName) {
			count++
		}
	}

	return count
}

// GotAllRequiredApprovals checks whether all required approvals are present for the given
// file paths, according to the "CODEOWNERS" file in the given branch (a PR's destination).
func GotAllRequiredApprovals(ctx workflow.Context, workspace, repo, branch, commit string, paths, approvers []string) bool {
	if len(paths) == 0 {
		return false
	}

	c := parseCodeOwnersFile(ctx, getBitbucketSourceFile(ctx, workspace, repo, branch, commit, "CODEOWNERS"), true)
	if c == nil {
		return false
	}

	for _, p := range paths {
		owners, err := c.getOwners(p)
		if err != nil {
			logger.From(ctx).Error("failed to check CODEOWNERS path pattern",
				slog.Any("error", err), slog.String("file_path", p))
			return false
		}
		if !c.allApproved(ctx, approvers, owners, true) {
			return false
		}
	}

	return true
}

// OwnersPerPath retrieves the list of code owners for each of the given file paths,
// according to the "CODEOWNERS" file in the given branch (a PR's destination).
func OwnersPerPath(ctx workflow.Context, workspace, repo, branch, commit string, paths []string, flatten bool) (owners, groups map[string][]string) {
	c := parseCodeOwnersFile(ctx, getBitbucketSourceFile(ctx, workspace, repo, branch, commit, "CODEOWNERS"), flatten)
	if c == nil {
		return nil, nil
	}

	owners = make(map[string][]string, len(paths))
	for _, p := range paths {
		os, err := c.getOwners(p)
		if err != nil {
			logger.From(ctx).Error("failed to check CODEOWNERS path pattern",
				slog.Any("error", err), slog.String("file_path", p))
			return nil, nil
		}
		owners[p] = os
	}

	if !flatten {
		groups = c.Groups
	}

	return owners, groups
}

func parseCodeOwnersFile(ctx workflow.Context, fileContent string, flatten bool) *CodeOwners {
	if fileContent == "" {
		return nil
	}

	c := &CodeOwners{
		Groups: map[string][]string{},
		Paths:  map[string][]string{},
		Users:  map[string]bool{},
	}

	for line := range strings.Lines(fileContent) {
		pathPattern, group, members := parseCodeOwnersLine(line)
		switch {
		case pathPattern != "" && strings.HasPrefix(pathPattern, "!"):
			c.IgnoreList = append(c.IgnoreList, normalizePattern(pathPattern[1:]))
		case pathPattern != "" && !strings.HasPrefix(pathPattern, "!"):
			pathPattern = normalizePattern(pathPattern)
			c.PathList = append(c.PathList, pathPattern)
			c.Paths[pathPattern] = append(c.Paths[pathPattern], members...)
		case group != "":
			c.Groups[group] = append(c.Groups[group], members...)
		}
	}

	slices.Reverse(c.PathList) // CODEOWNERS semantics: last match wins.
	if flatten {
		c.expandGroups(ctx)
	}
	return c
}

var (
	linePattern    = regexp.MustCompile(`^(@*\S+)\s*(.*)$`)
	membersPattern = regexp.MustCompile(`@{1,2}"?[\w\s]+"?`)
)

func parseCodeOwnersLine(line string) (pathPattern, group string, members []string) {
	line, _, _ = strings.Cut(line, "#")
	line = strings.TrimSpace(line)

	if len(line) == 0 || strings.HasPrefix(line, "Check(") {
		return "", "", nil
	}

	parts := linePattern.FindAllStringSubmatch(line, -1)
	if len(parts) != 1 || len(parts[0]) != 3 {
		return "", "", nil
	}
	if strings.HasPrefix(parts[0][1], "@@") {
		group = parts[0][1][2:]
	} else {
		pathPattern = parts[0][1]
	}

	members = membersPattern.FindAllString(parts[0][2], -1)
	for i, m := range members {
		members[i] = strings.Trim(strings.TrimPrefix(strings.TrimSpace(m), "@"), `"`)
	}
	return pathPattern, group, members
}

// normalizePattern ensures that the given pattern is in a form suitable for doublestar matching.
func normalizePattern(pattern string) string {
	if !strings.HasPrefix(pattern, "/") && !strings.HasPrefix(pattern, "**/") {
		pattern = "**/" + pattern
	}

	if strings.HasSuffix(pattern, "/") {
		pattern += "**/*"
	}

	if strings.HasSuffix(pattern, "/**") {
		// Inconsistency between https://mibexsoftware.bitbucket.io/codeowners-playground/
		// and https://pkg.go.dev/github.com/bmatcuk/doublestar : doublestar requires
		// "/*" at the end of the path pattern to match files under a directory.
		pattern += "/*"
	}

	return pattern
}

// expandGroups expands all group references in the CODEOWNERS files into individual members.
// It modifies the [CodeOwners.Paths] map in place. This function is idempotent.
func (c *CodeOwners) expandGroups(ctx workflow.Context) *CodeOwners {
	expandedPaths := make(map[string][]string, len(c.Paths))
	for pathPattern, members := range c.Paths {
		var expanded []string
		for _, member := range members {
			users, size := c.expandMember(ctx, member)
			if size == 0 {
				return nil
			}
			expanded = append(expanded, users...)
		}

		slices.Sort(expanded)
		expanded = slices.Compact(expanded)
		expandedPaths[pathPattern] = expanded
	}

	c.Paths = expandedPaths
	return c
}

func (c *CodeOwners) expandMember(ctx workflow.Context, name string) ([]string, int) {
	if !strings.HasPrefix(name, "@") {
		c.Users[name] = true
		return []string{name}, 1
	}

	members, found := c.Groups[name]
	if !found {
		logger.From(ctx).Error("failed to expand group in CODEOWNERS", slog.String("group_name", name))
		return nil, 0
	}

	var expanded []string
	for _, member := range members {
		users, size := c.expandMember(ctx, member)
		if size == 0 {
			return nil, 0
		}
		expanded = append(expanded, users...)
	}

	slices.Sort(expanded)
	expanded = slices.Compact(expanded)
	c.Groups[name] = expanded // Memoization for nested groups.

	return expanded, len(expanded)
}

func (c *CodeOwners) allApproved(ctx workflow.Context, approvers, owners []string, needAll bool) bool {
	if specialOwners, found := c.Groups["@FallbackOwners"]; needAll && found {
		if c.allApproved(ctx, approvers, specialOwners, false) {
			return true
		}
	}

	approvals := 0
	for _, name := range owners {
		// Approvals from individual owners.
		if !strings.HasPrefix(name, "@") {
			if slices.Contains(approvers, name) {
				approvals++
			}
			continue
		}

		// Approvals from (at least one individual user in each) CODEOWNERS group.
		members, found := c.Groups[name]
		if !found {
			logger.From(ctx).Error("group not found in CODEOWNERS", slog.String("group_name", name))
			return false
		}

		if c.allApproved(ctx, approvers, members, false) {
			approvals++
		}
	}

	return (needAll && approvals == len(owners)) || (!needAll && approvals > 0)
}

func (c *CodeOwners) getOwners(filePath string) ([]string, error) {
	if !strings.HasPrefix(filePath, "/") {
		filePath = "/" + filePath
	}

	for _, pattern := range c.PathList {
		match, err := doublestar.Match(pattern, filePath)
		if err != nil {
			return nil, err
		}
		if match && !c.ignorePath(filePath) {
			// We reversed PathList after parsing, so the FIRST match wins here.
			// But we also consider ignore patterns.
			return c.Paths[pattern], nil
		}
	}

	return nil, nil
}

func (c *CodeOwners) ignorePath(filePath string) bool {
	for _, pattern := range c.IgnoreList {
		match, err := doublestar.Match(pattern, filePath)
		if err != nil {
			continue
		}
		if match {
			return true
		}
	}

	return false
}
