package workflows

import (
	"log/slog"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/bitbucket"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/markdown"
	"github.com/tzrikka/revchat/pkg/slack"
	"github.com/tzrikka/revchat/pkg/slack/activities"
	"github.com/tzrikka/revchat/pkg/users"
)

// CommentCreatedWorkflow mirrors the creation of a new PR comment in the PR's Slack channel:
// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Comment-created.1
func CommentCreatedWorkflow(ctx workflow.Context, event bitbucket.PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to mirror this event.
	prURL := bitbucket.HTMLURL(event.PullRequest.Links)
	channelID, found := bitbucket.LookupSlackChannel(ctx, event.Type, prURL)
	if !found {
		return nil
	}

	defer bitbucket.UpdateChannelBookmarks(ctx, event, channelID, nil)

	email, _ := users.BitbucketToEmail(ctx, event.Actor.AccountID)
	if err := data.SwitchTurn(prURL, email); err != nil {
		logger.From(ctx).Error("failed to switch Bitbucket PR's attention state",
			slog.Any("error", err), slog.String("pr_url", prURL), slog.String("account_id", event.Actor.AccountID))
		// Don't abort - it's more important to post the comment, even if our internal state is stale.
	}

	// If the comment was created by RevChat, don't repost it.
	if strings.HasSuffix(event.Comment.Content.Raw, "\n\n[This comment was created by RevChat]: #") {
		logger.From(ctx).Debug("ignoring self-triggered Bitbucket event")
		return nil
	}

	msg := markdown.BitbucketToSlack(ctx, event.Comment.Content.Raw, prURL)
	var diff []byte
	if event.Comment.Inline != nil {
		msg, diff = bitbucket.BeautifyInlineComment(ctx, event, msg, event.Comment.Content.Raw)
	}

	var err error
	commentURL := bitbucket.HTMLURL(event.Comment.Links)
	if event.Comment.Parent == nil {
		err = bitbucket.ImpersonateUserInMsg(ctx, commentURL, channelID, event.Comment.User, msg, diff)
	} else {
		parentURL := bitbucket.HTMLURL(event.Comment.Parent.Links)
		err = bitbucket.ImpersonateUserInReply(ctx, commentURL, parentURL, event.Comment.User, msg, diff)
	}
	return err
}

// CommentUpdatedWorkflow mirrors an edit of an existing PR comment in the PR's Slack channel:
// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Comment-updated
//
// Note: these events are not reported by Bitbucket if they occur within a 10-minute window since the creation or
// last update of the same PR comment. RevChat actively polls Bitbucket to detect these events within these windows.
func CommentUpdatedWorkflow(ctx workflow.Context, event bitbucket.PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to mirror this event.
	prURL := bitbucket.HTMLURL(event.PullRequest.Links)
	channelID, found := bitbucket.LookupSlackChannel(ctx, event.Type, prURL)
	if !found {
		return nil
	}

	defer bitbucket.UpdateChannelBookmarks(ctx, event, channelID, nil)

	// If the comment previously had an attached diff file, delete it - it's obsolete now.
	if fileID, found := bitbucket.LookupSlackFileID(ctx, event.Comment); found {
		slack.DeleteFile(ctx, fileID)
	}

	msg := markdown.BitbucketToSlack(ctx, event.Comment.Content.Raw, prURL)
	var diff []byte
	if event.Comment.Inline != nil {
		msg, diff = bitbucket.BeautifyInlineComment(ctx, event, msg, event.Comment.Content.Raw)
	}

	// We can't upload a file to an existing impersonated message - that would disable future updates/deletion
	// of that message. We also can't replace an existing file attachment with a new upload in a seamless way.
	// So we simply replace the suggestion block with a slightly better diff block.
	if diff != nil {
		parts := strings.Split(msg, "\n```")

		var buf strings.Builder
		buf.WriteString(parts[0])
		buf.WriteString("\n```")
		buf.Write(diff)
		buf.WriteString("```")
		if len(parts) > 2 {
			if suffix := strings.TrimSpace(parts[2]); suffix != "" {
				buf.WriteString("\n")
				buf.WriteString(suffix)
			}
		}
		// Don't use fmt.Sprintf() here to avoid issues with % signs in the diff.
		msg = strings.Replace(bitbucket.ImpersonationToMention(buf.String()), "%s", bitbucket.SlackDisplayName(ctx, event.Actor), 1)
	}

	return bitbucket.EditMsg(ctx, bitbucket.HTMLURL(event.Comment.Links), msg)
}

// CommentDeletedWorkflow mirrors the deletion of a PR comment in the PR's Slack channel:
// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Comment-deleted
func CommentDeletedWorkflow(ctx workflow.Context, event bitbucket.PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to mirror this event.
	channelID, found := bitbucket.LookupSlackChannel(ctx, event.Type, bitbucket.HTMLURL(event.PullRequest.Links))
	if !found {
		return nil
	}

	defer bitbucket.UpdateChannelBookmarks(ctx, event, channelID, nil)

	if fileID, found := bitbucket.LookupSlackFileID(ctx, event.Comment); found {
		slack.DeleteFile(ctx, fileID)
	}

	return bitbucket.DeleteMsg(ctx, bitbucket.HTMLURL(event.Comment.Links))
}

// CommentResolvedWorkflow mirrors the resolution of a PR comment in the PR's Slack channel:
// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Comment-resolved
func CommentResolvedWorkflow(ctx workflow.Context, event bitbucket.PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	channelID, found := bitbucket.LookupSlackChannel(ctx, event.Type, bitbucket.HTMLURL(event.PullRequest.Links))
	if !found {
		return nil
	}

	defer bitbucket.UpdateChannelBookmarks(ctx, event, channelID, nil)

	url := bitbucket.HTMLURL(event.Comment.Links)
	activities.AddOKReaction(ctx, url) // The mention below is more important than this reaction.
	return bitbucket.MentionUserInReply(ctx, url, event.Actor, "%s resolved this comment. :ok:")
}

// CommentReopenedWorkflow mirrors the reopening of a resolved PR comment in the PR's Slack channel:
// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Comment-reopened
func CommentReopenedWorkflow(ctx workflow.Context, event bitbucket.PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	channelID, found := bitbucket.LookupSlackChannel(ctx, event.Type, bitbucket.HTMLURL(event.PullRequest.Links))
	if !found {
		return nil
	}

	defer bitbucket.UpdateChannelBookmarks(ctx, event, channelID, nil)

	url := bitbucket.HTMLURL(event.Comment.Links)
	activities.RemoveOKReaction(ctx, url) // The mention below is more important than this reaction.
	return bitbucket.MentionUserInReply(ctx, url, event.Actor, "%s reopened this comment. :no_good:")
}
