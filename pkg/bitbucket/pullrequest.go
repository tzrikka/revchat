package bitbucket

import (
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

	url := htmlURL(event.PullRequest.Links)
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
		mentionUserInMsg(ctx, channelID, event.Actor, "%s marked this PR as a draft. :construction:")
		return nil
	}
	if snapshot.Draft && !event.PullRequest.Draft {
		mentionUserInMsg(ctx, channelID, event.Actor, "%s marked this PR as ready for review. :eyes:")
		snapshot.Reviewers = nil // Force re-adding any reviewers that were added while the PR was a draft.
	}

	// Title edited.
	if snapshot.Title != event.PullRequest.Title {
		msg := ":pencil2: %s edited the PR title: " + c.linkifyTitle(ctx, event.PullRequest.Title)
		mentionUserInMsg(ctx, channelID, event.Actor, msg)
		slack.SetChannelDescription(ctx, channelID, event.PullRequest.Title, url)
		err = c.renameChannel(ctx, event.PullRequest, channelID)
	}

	// Description edited.
	if snapshot.Description != event.PullRequest.Description {
		msg := ":pencil2: %s deleted the PR description."
		if text := strings.TrimSpace(event.PullRequest.Description); text != "" {
			msg = ":pencil2: %s edited the PR description:\n\n" + markdown.BitbucketToSlack(ctx, text, url)
		}

		mentionUserInMsg(ctx, channelID, event.Actor, msg)
	}

	// Reviewers added/removed.
	added, removed := reviewersDiff(*snapshot, event.PullRequest)
	if len(added)+len(removed) > 0 {
		mentionUserInMsg(ctx, channelID, event.Actor, reviewerMentions(ctx, added, removed))
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
		mentionUserInMsg(ctx, channelID, event.Actor, "%s "+msg)
	}

	// Retargeted destination branch.
	oldBranch := snapshot.Destination.Branch.Name
	newBranch := event.PullRequest.Destination.Branch.Name
	if oldBranch != newBranch {
		repoURL := htmlURL(event.Repository.Links)
		msg := "changed the target branch from <%s/branch/%s|`%s`> to <%s/branch/%s|`%s`>."
		msg = fmt.Sprintf(msg, repoURL, oldBranch, oldBranch, repoURL, newBranch, newBranch)
		mentionUserInMsg(ctx, channelID, event.Actor, "%s "+msg)
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
	url := htmlURL(pr.Links)
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

	mentionUserInMsg(ctx, channelID, event.Actor, msg)
	return nil
}

func prCommentCreatedWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	pr := event.PullRequest
	channelID, found := lookupChannel(ctx, event.Type, pr)
	if !found {
		return nil
	}

	defer updateChannelBookmarks(ctx, event, channelID, nil)

	prURL := htmlURL(pr.Links)
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

	// If the comment previously had an attached diff file, delete it - it's obsolete now.
	if fileID, found := lookupSlackFileID(ctx, event.Comment); found {
		slack.DeleteFile(ctx, fileID)
	}

	msg := markdown.BitbucketToSlack(ctx, event.Comment.Content.Raw, htmlURL(event.PullRequest.Links))
	var diff []byte
	if event.Comment.Inline != nil {
		msg, diff = beautifyInlineComment(ctx, event, msg, event.Comment.Content.Raw)
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
		msg = strings.Replace(impersonationToMention(buf.String()), "%s", slackDisplayName(ctx, event.Actor), 1)
	}

	return editMsg(ctx, htmlURL(event.Comment.Links), msg)
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
