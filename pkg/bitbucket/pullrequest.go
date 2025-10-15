package bitbucket

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/markdown"
	"github.com/tzrikka/revchat/pkg/slack"
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
	url := event.PullRequest.Links["html"].HRef
	snapshot, err := switchSnapshot(ctx, url, event.PullRequest)
	if err != nil {
		return err
	}

	// PR converted to a draft.
	if snapshot != nil && event.PullRequest.Draft {
		event.PullRequest.Draft = false
		return c.prClosedWorkflow(ctx, event)
	}

	// Ignore drafts until they're marked as ready for review.
	if snapshot == nil && event.PullRequest.Draft {
		return nil
	}

	// PR converted from a draft to ready-for-review.
	if snapshot == nil && !event.PullRequest.Draft {
		return c.prCreatedWorkflow(ctx, event)
	}

	// If we're not tracking this PR, there's no need/way to announce this event.
	channelID, found := lookupChannel(ctx, event.Type, event.PullRequest)
	if !found {
		return nil
	}

	// Title edited.
	if snapshot.Title != event.PullRequest.Title {
		_ = c.mentionUserInMsg(ctx, channelID, event.Actor, "%s edited the PR title.")
		slack.SetChannelDescription(ctx, channelID, event.PullRequest.Title, url)
		if msg := c.linkifyIDs(ctx, event.PullRequest.Title); msg != "" {
			_, _ = slack.PostMessage(ctx, channelID, msg)
		}

		return nil
	}

	// Description edited.
	if snapshot.Description != event.PullRequest.Description {
		msg := "%s deleted the PR description."
		if text := strings.TrimSpace(event.PullRequest.Description); text != "" {
			msg = "%s edited the PR description:\n\n" + markdown.BitbucketToSlack(ctx, c.Cmd, text, url)
		}

		err := c.mentionUserInMsg(ctx, channelID, event.Actor, msg)
		if msg := c.linkifyIDs(ctx, event.PullRequest.Description); msg != "" {
			_, err = slack.PostMessage(ctx, channelID, msg)
		}

		return err
	}

	// Reviewers added/removed.
	added, removed := reviewersDiff(*snapshot, event.PullRequest)
	if len(added)+len(removed) > 0 {
		msg := reviewerMentions(added, removed)
		_ = c.mentionUserInMsg(ctx, channelID, event.Actor, msg+".")
		return nil
	}

	// Commit(s) pushed to the PR branch.
	if snapshot.Source.Commit.Hash != event.PullRequest.Source.Commit.Hash {
		count := 0

		msg := fmt.Sprintf("%%s pushed <%s/commits|%d commit", url, count)
		if count != 1 {
			msg += "s"
		}
		return c.mentionUserInMsg(ctx, channelID, event.Actor, msg+"> to this PR.")
	}

	// Retargeted destination branch.
	oldBranch := snapshot.Destination.Branch.Name
	newBranch := event.PullRequest.Destination.Branch.Name
	if oldBranch != newBranch {
		repoURL := event.Repository.Links["html"].HRef
		msg := "%%s changed the target branch from <%s/branch/%s|`%s`> to <%s/branch/%s|`%s`>."
		msg = fmt.Sprintf(msg, repoURL, oldBranch, oldBranch, repoURL, newBranch, newBranch)
		return c.mentionUserInMsg(ctx, channelID, event.Actor, msg)
	}

	log.Warn(ctx, "unhandled Bitbucket PR update event", "url", url)
	return nil
}

func switchSnapshot(ctx workflow.Context, url string, snapshot PullRequest) (*PullRequest, error) {
	defer data.StoreBitbucketPR(url, snapshot)

	prev, err := data.LoadBitbucketPR(url)
	if err != nil {
		log.Error(ctx, "failed to load Bitbucket PR snapshot", "error", err, "url", url)
		return nil, err
	}

	if prev == nil {
		return nil, nil
	}

	pr := new(PullRequest)
	if err := mapToStruct(prev, pr); err != nil {
		log.Error(ctx, "previous snapshot of Bitbucket PR is invalid", "error", err, "url", url)
		return nil, err
	}

	return pr, nil
}

func mapToStruct(m any, pr *PullRequest) error {
	buf := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(buf).Encode(m); err != nil {
		return err
	}

	if err := json.NewDecoder(buf).Decode(pr); err != nil {
		return err
	}

	return nil
}

func reviewers(pr PullRequest, includeParticipants bool) []string {
	var accountIDs []string
	for _, r := range pr.Reviewers {
		accountIDs = append(accountIDs, r.AccountID)
	}

	if !includeParticipants {
		for _, p := range pr.Participants {
			accountIDs = append(accountIDs, p.User.AccountID)
		}
	}

	slices.Sort(accountIDs)
	return slices.Compact(accountIDs)
}

func reviewersDiff(prev, curr PullRequest) (added, removed []string) {
	prevIDs := reviewers(prev, false)
	currIDs := reviewers(curr, false)

	for _, id := range currIDs {
		if !slices.Contains(prevIDs, id) {
			added = append(added, id)
		}
	}

	for _, id := range prevIDs {
		if !slices.Contains(currIDs, id) {
			removed = append(removed, id)
		}
	}

	return
}

func reviewerMentions(added, removed []string) string {
	msg := "%s "
	if len(added) > 0 {
		msg += "added"
		for _, a := range added {
			msg += fmt.Sprintf(" %s", a)
		}
		if len(added) == 1 {
			msg += " as a reviewer"
		} else {
			msg += " as reviewers"
		}
	}

	if len(added) > 0 && len(removed) > 0 {
		msg += ", and "
	}

	if len(removed) > 0 {
		msg += "removed"
		for _, r := range removed {
			msg += fmt.Sprintf(" %s", r)
		}
		if len(removed) == 1 {
			msg += " as a reviewer"
		} else {
			msg += " as reviewers"
		}
	}

	return msg
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
