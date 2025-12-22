package slack

import (
	"log/slog"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
)

func destinationDetails(pr map[string]any) (workspace, repo, branch, commit string) {
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

func selfTriggeredMemberEvent(ctx workflow.Context, auth []eventAuth, event MemberEvent) bool {
	for _, a := range auth {
		if a.IsBot && (a.UserID == event.User || a.UserID == event.Inviter) {
			logger.From(ctx).Debug("ignoring self-triggered Slack event")
			return true
		}
	}
	return false
}

func selfTriggeredEvent(ctx workflow.Context, auth []eventAuth, userID string) bool {
	for _, a := range auth {
		if a.IsBot && a.UserID == userID {
			logger.From(ctx).Debug("ignoring self-triggered Slack event")
			return true
		}
	}
	return false
}

func commentURL(ctx workflow.Context, ids string) (string, error) {
	url, err := data.SwitchURLAndID(ids)
	if err != nil {
		logger.From(ctx).Error("failed to retrieve Slack message's PR comment URL",
			slog.Any("error", err), slog.String("ids", ids))
		return "", err
	}

	if url == "" {
		logger.From(ctx).Debug("Slack message's PR comment URL is empty", slog.String("ids", ids))
	}

	return url, nil
}

func prChannel(ctx workflow.Context, channelID string) bool {
	url, _ := commentURL(ctx, channelID)
	return url != ""
}
