package slack

import (
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/revchat/pkg/data"
)

func destinationDetails(pr map[string]any) (workspace, repo, branch, commit string) {
	// Workspace and repo slug.
	dest, ok := pr["destination"].(map[string]any)
	if !ok {
		return
	}
	m, ok := dest["repository"].(map[string]any)
	if !ok {
		return
	}
	fullName, ok := m["full_name"].(string)
	if !ok {
		return
	}
	workspace, repo, ok = strings.Cut(fullName, "/")
	if !ok {
		return
	}

	// Branch name.
	m, ok = dest["branch"].(map[string]any)
	if !ok {
		return
	}
	branch, ok = m["name"].(string)
	if !ok {
		return
	}

	// Commit hash.
	m, ok = dest["commit"].(map[string]any)
	if !ok {
		return
	}
	commit, _ = m["hash"].(string)

	return
}

func selfTriggeredMemberEvent(ctx workflow.Context, auth []eventAuth, event MemberEvent) bool {
	for _, a := range auth {
		if a.IsBot && (a.UserID == event.User || a.UserID == event.Inviter) {
			log.Debug(ctx, "ignoring self-triggered Slack event")
			return true
		}
	}
	return false
}

func selfTriggeredEvent(ctx workflow.Context, auth []eventAuth, userID string) bool {
	for _, a := range auth {
		if a.IsBot && a.UserID == userID {
			log.Debug(ctx, "ignoring self-triggered Slack event")
			return true
		}
	}
	return false
}

func commentURL(ctx workflow.Context, ids string) (string, error) {
	url, err := data.SwitchURLAndID(ids)
	if err != nil {
		log.Error(ctx, "failed to retrieve Slack message's PR comment URL", "error", err, "ids", ids)
		return "", err
	}

	if url == "" {
		log.Debug(ctx, "Slack message's PR comment URL is empty", "ids", ids)
	}

	return url, nil
}

func prChannel(ctx workflow.Context, channelID string) bool {
	url, _ := commentURL(ctx, channelID)
	return url != ""
}
