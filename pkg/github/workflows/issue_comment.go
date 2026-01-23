package workflows

import (
	"errors"
	"log/slog"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/github"
	"github.com/tzrikka/revchat/pkg/markdown"
	"github.com/tzrikka/revchat/pkg/slack/activities"
	"github.com/tzrikka/revchat/pkg/users"
)

// IssueCommentWorkflow is an entrypoint to mirror all GitHub issue comment events in the PR's
// Slack channel: https://docs.github.com/en/webhooks/webhook-events-and-payloads#issue_comment
func IssueCommentWorkflow(ctx workflow.Context, event github.IssueCommentEvent) error {
	switch event.Action {
	case "created":
		return issueCommentCreated(ctx, event)
	case "edited":
		return issueCommentEdited(ctx, event)
	case "deleted":
		return issueCommentDeleted(ctx, event)
	default:
		logger.From(ctx).Error("unrecognized GitHub issue comment event action", slog.String("action", event.Action))
		return errors.New("unrecognized GitHub issue comment event action: " + event.Action)
	}
}

// A comment on an issue or pull request was created.
func issueCommentCreated(ctx workflow.Context, event github.IssueCommentEvent) error {
	// If we're not tracking this PR, there's no need/way to mirror this event.
	channelID, found := activities.LookupChannel(ctx, event.Issue.HTMLURL)
	if !found {
		return nil
	}

	defer github.UpdateChannelBookmarks(ctx, nil, &event.Issue, channelID)

	// Don't abort if this fails - it's more important to post the comment.
	email := users.GitHubIDToEmail(ctx, event.Sender.Login)
	_ = data.SwitchTurn(ctx, event.Issue.HTMLURL, email, false)

	msg := markdown.GitHubToSlack(ctx, event.Comment.Body, event.Comment.HTMLURL)
	logger.From(ctx).Warn("MSG", slog.String("MSG", msg))
	return nil
}

// A comment on an issue or pull request was edited.
func issueCommentEdited(ctx workflow.Context, event github.IssueCommentEvent) error {
	// If we're not tracking this PR, there's no need/way to mirror this event.
	if _, found := activities.LookupChannel(ctx, event.Issue.HTMLURL); !found {
		return nil
	}

	msg := markdown.GitHubToSlack(ctx, event.Comment.Body, event.Comment.HTMLURL)
	logger.From(ctx).Warn("MSG", slog.String("MSG", msg))
	return nil
}

// A comment on an issue or pull request was deleted.
func issueCommentDeleted(ctx workflow.Context, event github.IssueCommentEvent) error {
	// If we're not tracking this PR, there's no need/way to mirror this event.
	channelID, found := activities.LookupChannel(ctx, event.Issue.HTMLURL)
	if !found {
		return nil
	}

	defer github.UpdateChannelBookmarks(ctx, nil, &event.Issue, channelID)

	return nil
}
