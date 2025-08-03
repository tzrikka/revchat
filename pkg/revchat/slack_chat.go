package revchat

import (
	"go.temporal.io/sdk/workflow"
)

func mentionGitHubUserInMessage(ctx workflow.Context, channel string, user User, msg string) string {
	return mentionGitHubUserInReply(ctx, channel, "", user, msg)
}

func mentionGitHubUserInReply(ctx workflow.Context, channel, commentURL string, user User, msg string) string {
	return ""
}
