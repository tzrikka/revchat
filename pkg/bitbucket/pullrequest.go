package bitbucket

import (
	"fmt"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/revchat/pkg/markdown"
)

// A new PR was created (or marked as ready for review - see [Config.prUpdatedWorkflow]).
func (c Config) prCreatedWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	// Ignore drafts until they're marked as ready for review.
	if event.PullRequest.Draft {
		return nil
	}

	return c.initChannel(ctx, event)
}

func (c Config) prUpdatedWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	return nil
}

func (c Config) prReviewedWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	channelID, found := lookupChannel(ctx, event.Type, event.PullRequest)
	if !found {
		return nil
	}

	msg := "%s "
	switch event.Type {
	case "approved":
		msg += "approved this PR :+1:"
	case "unapproved":
		msg += "unapproved this PR :-1:"
	case "changes_request_created":
		msg += "requested changes in this PR :warning:"

	// Ignored event type.
	case "changes_request_removed":

	default:
		log.Error(ctx, "unrecognized Bitbucket PR review event type", "event_type", event.Type)
	}

	return c.mentionUserInMsg(ctx, channelID, event.Actor, msg)
}

// A PR was closed, i.e. merged or rejected (possibly a draft).
func (c Config) prClosedWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	// Ignore drafts - they don't have an active Slack channel anyway.
	if event.PullRequest.Draft {
		return nil
	}

	return c.archiveChannel(ctx, event)
}

func (c Config) prCommentCreatedWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	pr := event.PullRequest
	channelID, found := lookupChannel(ctx, event.Type, pr)
	if !found {
		return nil
	}

	// If the comment was created by RevChat, don't repost it.
	if strings.HasSuffix(event.Comment.Content.Raw, "\n\n[This comment was created by RevChat]: #") {
		log.Debug(ctx, "ignoring self-triggered Bitbucket event")
		return nil
	}

	commentURL := htmlURL(event.Comment.Links)
	msg := markdown.BitbucketToSlack(ctx, c.Cmd, event.Comment.Content.Raw, htmlURL(pr.Links))
	if inline := event.Comment.Inline; inline != nil {
		msg = inlineCommentPrefix(commentURL, inline) + msg
	}

	var err error
	if event.Comment.Parent == nil {
		err = c.impersonateUserInMsg(ctx, commentURL, channelID, event.Comment.User, msg)
	} else {
		parentURL := htmlURL(event.Comment.Parent.Links)
		err = c.impersonateUserInReply(ctx, commentURL, parentURL, event.Comment.User, msg)
	}
	return err
}

func (c Config) prCommentUpdatedWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	pr := event.PullRequest
	_, found := lookupChannel(ctx, event.Type, pr)
	if !found {
		return nil
	}

	commentURL := htmlURL(event.Comment.Links)
	msg := markdown.BitbucketToSlack(ctx, c.Cmd, event.Comment.Content.Raw, htmlURL(pr.Links))
	if inline := event.Comment.Inline; inline != nil {
		msg = inlineCommentPrefix(commentURL, inline) + msg
	}

	return c.editMsg(ctx, commentURL, msg)
}

func (c Config) prCommentDeletedWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	_, found := lookupChannel(ctx, event.Type, event.PullRequest)
	if !found {
		return nil
	}

	return c.deleteMsg(ctx, htmlURL(event.Comment.Links))
}

func (c Config) prCommentResolvedWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	_, found := lookupChannel(ctx, event.Type, event.PullRequest)
	if !found {
		return nil
	}

	url := htmlURL(event.Comment.Links)
	_ = c.addReaction(ctx, url, "ok")
	return c.mentionUserInReplyByURL(ctx, url, event.Actor, "%s resolved this comment :ok:")
}

func (c Config) prCommentReopenedWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	_, found := lookupChannel(ctx, event.Type, event.PullRequest)
	if !found {
		return nil
	}

	url := htmlURL(event.Comment.Links)
	_ = c.removeReaction(ctx, url, "ok")
	return c.mentionUserInReplyByURL(ctx, url, event.Actor, "%s reopened this comment :no_good:")
}

func htmlURL(links map[string]Link) string {
	return links["html"].HRef
}

func inlineCommentPrefix(url string, i *Inline) string {
	subject := "File"
	location := "the"

	if i.From != nil {
		subject = "Line"
		location = fmt.Sprintf("line %d in the", *i.From)

		if i.To != nil {
			location = fmt.Sprintf("lines %d-%d in the", *i.From, *i.To)
		}
	}

	return fmt.Sprintf("<%s|%s comment> in %s file `%s`:\n", url, subject, location, i.Path)
}
