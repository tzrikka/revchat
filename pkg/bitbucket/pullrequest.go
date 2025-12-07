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
	"github.com/tzrikka/revchat/pkg/users"
)

// A new PR was created.
func (c Config) prCreatedWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	return c.initChannel(ctx, event)
}

// A PR was closed, i.e. merged or declined/rejected.
func prClosedWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	return archiveChannel(ctx, event)
}

func (c Config) prUpdatedWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	channelID, found := lookupChannel(ctx, event.Type, event.PullRequest)
	if !found {
		return nil
	}

	cmts := commits(ctx, event)
	event.PullRequest.CommitCount = len(cmts)

	url := event.PullRequest.Links["html"].HRef
	snapshot, err := switchSnapshot(ctx, url, event.PullRequest)
	if err != nil {
		return err
	}

	// Support PR data recovery.
	if snapshot == nil {
		initPRData(ctx, event, channelID)
		return nil
	}

	defer updateChannelBookmarks(ctx, event, channelID, snapshot)

	// Announce transitions between drafts and ready to review.
	if !snapshot.Draft && event.PullRequest.Draft {
		return mentionUserInMsg(ctx, channelID, event.Actor, "%s marked this PR as a draft. :construction:")
	}
	if snapshot.Draft && !event.PullRequest.Draft {
		_ = mentionUserInMsg(ctx, channelID, event.Actor, "%s marked this PR as ready for review. :eyes:")
		snapshot.Reviewers = nil // Force re-adding any reviewers that were added while the PR was a draft.
	}

	// Title edited.
	if snapshot.Title != event.PullRequest.Title {
		_ = mentionUserInMsg(ctx, channelID, event.Actor, ":pencil2: %s edited the PR title.")
		slack.SetChannelDescription(ctx, channelID, event.PullRequest.Title, url)
		if msg := c.linkifyIDs(ctx, event.PullRequest.Title); msg != "" {
			_, _ = slack.PostMessage(ctx, channelID, msg)
		}

		err = c.renameChannel(ctx, event.PullRequest, channelID)
	}

	// Description edited.
	if snapshot.Description != event.PullRequest.Description {
		msg := ":pencil2: %s deleted the PR description."
		if text := strings.TrimSpace(event.PullRequest.Description); text != "" {
			msg = ":pencil2: %s edited the PR description:\n\n" + markdown.BitbucketToSlack(ctx, text, url)
		}

		err = mentionUserInMsg(ctx, channelID, event.Actor, msg)
	}

	// Reviewers added/removed.
	added, removed := reviewersDiff(*snapshot, event.PullRequest)
	if len(added)+len(removed) > 0 {
		_ = mentionUserInMsg(ctx, channelID, event.Actor, reviewerMentions(ctx, added, removed))
		if !event.PullRequest.Draft {
			_ = slack.InviteUsersToChannel(ctx, channelID, bitbucketToSlackIDs(ctx, added))
		}
		_ = slack.KickUsersFromChannel(ctx, channelID, bitbucketToSlackIDs(ctx, removed))
	}

	for _, id := range added {
		email, err := users.BitbucketToEmail(ctx, id)
		if err != nil {
			continue
		}
		if err := data.AddReviewerToPR(url, email); err != nil {
			log.Error(ctx, "failed to add reviewer to Bitbucket PR's attention state", "error", err, "pr_url", url)
		}
	}

	for _, id := range removed {
		email, err := users.BitbucketToEmail(ctx, id)
		if err != nil {
			continue
		}
		if err := data.RemoveFromTurn(url, email); err != nil {
			log.Error(ctx, "failed to remove reviewers from Bitbucket PR's attention state", "error", err, "pr_url", url)
		}
	}

	// Commit(s) pushed to the PR branch.
	if event.PullRequest.CommitCount > 0 && snapshot.Source.Commit.Hash != event.PullRequest.Source.Commit.Hash {
		if err := data.UpdateBitbucketDiffstat(url, diffstat(ctx, event)); err != nil {
			log.Error(ctx, "failed to update Bitbucket PR's diffstat", "error", err, "pr_url", url)
			// Continue anyway.
		}

		slices.Reverse(cmts) // Switch from reverse order to chronological order.

		prevCount := snapshot.CommitCount
		if prevCount >= event.PullRequest.CommitCount {
			// Handle the unlikely ">" case where RevChat missed a commit push,
			// but more likely the "==" case where the user force-pushed a new head
			// (i.e. same number of commits) - by announcing just the last commit.
			prevCount = event.PullRequest.CommitCount - 1
		}
		cmts = cmts[prevCount:]

		msg := fmt.Sprintf("pushed <%s/commits|%d commit", url, len(cmts))
		if len(cmts) != 1 {
			msg += "s"
		}

		msg += "> to this PR:"
		for _, c := range cmts {
			title, _, _ := strings.Cut(c.Message, "\n")
			msg += fmt.Sprintf("\n  •  <%s|`%s`> %s", c.Links["html"].HRef, c.Hash[:7], title)
		}
		err = mentionUserInMsg(ctx, channelID, event.Actor, "%s "+msg)
	}

	// Retargeted destination branch.
	oldBranch := snapshot.Destination.Branch.Name
	newBranch := event.PullRequest.Destination.Branch.Name
	if oldBranch != newBranch {
		repoURL := event.Repository.Links["html"].HRef
		msg := "changed the target branch from <%s/branch/%s|`%s`> to <%s/branch/%s|`%s`>."
		msg = fmt.Sprintf(msg, repoURL, oldBranch, oldBranch, repoURL, newBranch, newBranch)
		err = mentionUserInMsg(ctx, channelID, event.Actor, "%s "+msg)
	}

	return err
}

func prReviewedWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	channelID, found := lookupChannel(ctx, event.Type, event.PullRequest)
	if !found {
		return nil
	}

	defer updateChannelBookmarks(ctx, event, channelID, nil)

	pr := event.PullRequest
	url := pr.Links["html"].HRef
	email, _ := users.BitbucketToEmail(ctx, event.Actor.AccountID)
	msg := "%s "

	switch event.Type {
	case "approved":
		if err := data.RemoveFromTurn(url, email); err != nil {
			log.Error(ctx, "failed to remove user from Bitbucket PR's attention state", "error", err, "pr_url", url, "email", email)
		}
		msg += "approved this PR. :+1:"

	case "unapproved":
		if err := data.AddReviewerToPR(url, email); err != nil {
			log.Error(ctx, "failed to add user back to Bitbucket PR's attention state", "error", err, "pr_url", url, "email", email)
		}
		if err := data.SwitchTurn(url, email); err != nil {
			log.Error(ctx, "failed to switch Bitbucket PR's attention state", "error", err, "pr_url", url, "email", email)
		}
		msg += "unapproved this PR. :-1:"

	case "changes_request_created":
		pr.ChangeRequestCount++
		if _, err := switchSnapshot(ctx, url, pr); err != nil {
			log.Error(ctx, "failed to update change-request count in PR snapshot",
				"error", err, "pr_url", url, "new_count", pr.ChangeRequestCount)
		}

		if err := data.SwitchTurn(url, email); err != nil {
			log.Error(ctx, "failed to switch Bitbucket PR's attention state", "error", err, "pr_url", url, "email", email)
		}
		msg += "requested changes in this PR. :warning:"

	case "changes_request_removed":
		pr.ChangeRequestCount--
		if pr.ChangeRequestCount < 0 {
			pr.ChangeRequestCount = 0 // Should not happen, but just in case.
		}
		if _, err := switchSnapshot(ctx, url, pr); err != nil {
			log.Error(ctx, "failed to update change-request count in PR snapshot",
				"error", err, "pr_url", url, "new_count", pr.ChangeRequestCount)
		}
		return nil

	default:
		log.Error(ctx, "unrecognized Bitbucket PR review event type", "event_type", event.Type)
		return nil
	}

	return mentionUserInMsg(ctx, channelID, event.Actor, msg)
}

func prCommentCreatedWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	pr := event.PullRequest
	channelID, found := lookupChannel(ctx, event.Type, pr)
	if !found {
		return nil
	}

	defer updateChannelBookmarks(ctx, event, channelID, nil)

	prURL := pr.Links["html"].HRef
	email, _ := users.BitbucketToEmail(ctx, event.Actor.AccountID)
	if err := data.SwitchTurn(prURL, email); err != nil {
		log.Error(ctx, "failed to switch Bitbucket PR's attention state", "error", err, "pr_url", prURL)
		// Don't abort - we still want to post the comment.
	}

	// If the comment was created by RevChat, don't repost it.
	if strings.HasSuffix(event.Comment.Content.Raw, "\n\n[This comment was created by RevChat]: #") {
		log.Debug(ctx, "ignoring self-triggered Bitbucket event")
		return nil
	}

	msg := markdown.BitbucketToSlack(ctx, event.Comment.Content.Raw, prURL)
	var diff []byte
	if event.Comment.Inline != nil {
		msg, diff = beautifyInlineComment(ctx, event, msg, event.Comment.Content.Raw)
	}

	var err error
	commentURL := htmlURL(event.Comment.Links)
	if event.Comment.Parent == nil {
		err = impersonateUserInMsg(ctx, commentURL, channelID, event.Comment.User, msg, diff)
	} else {
		parentURL := htmlURL(event.Comment.Parent.Links)
		err = impersonateUserInReply(ctx, commentURL, parentURL, event.Comment.User, msg, diff)
	}
	return err
}

func prCommentUpdatedWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	channelID, found := lookupChannel(ctx, event.Type, event.PullRequest)
	if !found {
		return nil
	}

	defer updateChannelBookmarks(ctx, event, channelID, nil)

	commentURL := htmlURL(event.Comment.Links)
	msg := markdown.BitbucketToSlack(ctx, event.Comment.Content.Raw, htmlURL(event.PullRequest.Links))
	if event.Comment.Inline != nil {
		msg, _ = beautifyInlineComment(ctx, event, msg, event.Comment.Content.Raw)
	}

	return editMsg(ctx, commentURL, msg)
}

func prCommentDeletedWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	channelID, found := lookupChannel(ctx, event.Type, event.PullRequest)
	if !found {
		return nil
	}

	defer updateChannelBookmarks(ctx, event, channelID, nil)

	if fileID, found := lookupSlackFileID(ctx, event.Comment); found {
		slack.DeleteFile(ctx, fileID)
	}

	return deleteMsg(ctx, htmlURL(event.Comment.Links))
}

func prCommentResolvedWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	channelID, found := lookupChannel(ctx, event.Type, event.PullRequest)
	if !found {
		return nil
	}

	defer updateChannelBookmarks(ctx, event, channelID, nil)

	url := htmlURL(event.Comment.Links)
	addOKReaction(ctx, url)
	return mentionUserInReplyByURL(ctx, url, event.Actor, "%s resolved this comment. :ok:")
}

func prCommentReopenedWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	channelID, found := lookupChannel(ctx, event.Type, event.PullRequest)
	if !found {
		return nil
	}

	defer updateChannelBookmarks(ctx, event, channelID, nil)

	url := htmlURL(event.Comment.Links)
	removeOKReaction(ctx, url)
	return mentionUserInReplyByURL(ctx, url, event.Actor, "%s reopened this comment. :no_good:")
}

// switchSnapshot stores the given new PR snapshot, and returns the previous one (if any).
func switchSnapshot(ctx workflow.Context, url string, snapshot PullRequest) (*PullRequest, error) {
	defer func() { _ = data.StoreBitbucketPR(url, snapshot) }()

	prev, err := data.LoadBitbucketPR(url)
	if err != nil {
		log.Error(ctx, "failed to load Bitbucket PR snapshot", "error", err, "pr_url", url)
		return nil, err
	}

	if prev == nil {
		return nil, nil
	}

	pr := new(PullRequest)
	if err := mapToStruct(prev, pr); err != nil {
		log.Error(ctx, "previous snapshot of Bitbucket PR is invalid", "error", err, "pr_url", url)
		return nil, err
	}

	// the "CommitCount" and "ChangeRequestCount" fields are populated and used by RevChat, not Bitbucket.
	// Persist them across snapshots (before the deferred call to [data.StoreBitbucketPR]).
	if snapshot.CommitCount == 0 {
		snapshot.CommitCount = pr.CommitCount
	}
	if snapshot.ChangeRequestCount == 0 {
		snapshot.ChangeRequestCount = pr.ChangeRequestCount
	}

	return pr, nil
}

// mapToStruct converts a map-based representation of JSON data into a [PullRequest] struct.
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

func htmlURL(links map[string]Link) string {
	return links["html"].HRef
}
