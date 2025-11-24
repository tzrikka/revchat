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

// A new PR was created (or marked as ready for review - see [Config.prUpdatedWorkflow]).
func (c Config) prCreatedWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	// Ignore drafts until they're marked as ready for review.
	if event.PullRequest.Draft {
		return nil
	}

	return c.initChannel(ctx, event)
}

func (c Config) prUpdatedWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	cmts := commits(ctx, event)
	event.PullRequest.CommitCount = len(cmts)

	url := event.PullRequest.Links["html"].HRef
	snapshot, err := switchSnapshot(ctx, url, event.PullRequest)
	if err != nil {
		return err
	}

	// PR converted to a draft.
	if snapshot != nil && event.PullRequest.Draft {
		event.PullRequest.Draft = false
		return prClosedWorkflow(ctx, event)
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

	defer updateChannelBookmarks(ctx, event, channelID, snapshot)

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
		msg := reviewerMentions(ctx, added, removed)
		_ = mentionUserInMsg(ctx, channelID, event.Actor, msg+".")
		_ = slack.InviteUsersToChannel(ctx, channelID, bitbucketToSlackIDs(ctx, added))
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
		slices.Reverse(cmts) // Switch from reverse order to chronological order.

		prevCount := snapshot.CommitCount
		if prevCount >= event.PullRequest.CommitCount {
			// Handle the unlikely ">" case where RevChat missed a commit push,
			// but more likely the "==" case where the user force-pushed a new head
			// (i.e. same number of commits) - by announcing just the last commit.
			prevCount = event.PullRequest.CommitCount - 1
		}
		cmts = cmts[prevCount:]

		msg := fmt.Sprintf("%%s pushed <%s/commits|%d commit", url, len(cmts))
		if len(cmts) != 1 {
			msg += "s"
		}

		msg += "> to this PR:"
		for _, c := range cmts {
			msg += fmt.Sprintf("\n  •  <%s|`%s`> %s", c.Links["html"].HRef, c.Hash[:7], c.Message)
		}
		err = mentionUserInMsg(ctx, channelID, event.Actor, msg)
	}

	// Retargeted destination branch.
	oldBranch := snapshot.Destination.Branch.Name
	newBranch := event.PullRequest.Destination.Branch.Name
	if oldBranch != newBranch {
		repoURL := event.Repository.Links["html"].HRef
		msg := "%%s changed the target branch from <%s/branch/%s|`%s`> to <%s/branch/%s|`%s`>."
		msg = fmt.Sprintf(msg, repoURL, oldBranch, oldBranch, repoURL, newBranch, newBranch)
		err = mentionUserInMsg(ctx, channelID, event.Actor, msg)
	}

	return err
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

	// the "CommitCount" field is fake and populated by RevChat, not Bitbucket.
	// Persist it across snapshots (before the deferred call to [data.StoreBitbucketPR]).
	if snapshot.CommitCount == 0 {
		snapshot.CommitCount = pr.CommitCount
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

// reviewers returns the list of reviewer account IDs, and possibly participants too.
// The output is guaranteed to be sorted, without teams/apps, and without repetitions.
func reviewers(pr PullRequest, includeParticipants bool) []string {
	var accountIDs []string
	for _, r := range pr.Reviewers {
		accountIDs = append(accountIDs, r.AccountID)
	}

	if includeParticipants {
		for _, p := range pr.Participants {
			accountIDs = append(accountIDs, p.User.AccountID)
		}
	}

	slices.Sort(accountIDs)
	return slices.Compact(accountIDs)
}

// reviewerDiff returns the lists of added and removed reviewers
// (not participants), compared to the previous snapshot of the PR.
// The output is guaranteed to be sorted, without teams/apps, and without repetitions.
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

// reviewerMentions returns a Slack message mentioning all the newly added/removed reviewers.
func reviewerMentions(ctx workflow.Context, added, removed []string) string {
	msg := ":bust_in_silhouette: %s "
	if len(added) > 0 {
		msg += "added" + bitbucketAccountsToSlackMentions(ctx, added)
	}
	if len(added) > 0 && len(removed) > 0 {
		msg += ", and "
	}
	if len(removed) > 0 {
		msg += "removed" + bitbucketAccountsToSlackMentions(ctx, removed)
	}
	return msg
}

func bitbucketAccountsToSlackMentions(ctx workflow.Context, accountIDs []string) string {
	slackUsers := ""
	for _, a := range accountIDs {
		if ref := users.BitbucketToSlackRef(ctx, a, ""); ref != "" {
			slackUsers += " " + ref
		}
	}

	if len(accountIDs) == 1 {
		slackUsers += " as a reviewer"
	} else {
		slackUsers += " as reviewers"
	}

	return slackUsers
}

func bitbucketToSlackIDs(ctx workflow.Context, accountIDs []string) []string {
	slackIDs := []string{}
	for _, aid := range accountIDs {
		// True = don't include opted-out users. They will still be mentioned
		// in the channel, but as non-members they won't be notified about it.
		if sid := users.BitbucketToSlackID(ctx, aid, true); sid != "" {
			slackIDs = append(slackIDs, sid)
		}
	}
	return slackIDs
}

func prReviewedWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	channelID, found := lookupChannel(ctx, event.Type, event.PullRequest)
	if !found {
		return nil
	}

	defer updateChannelBookmarks(ctx, event, channelID, nil)

	url := event.PullRequest.Links["html"].HRef
	email, _ := users.BitbucketToEmail(ctx, event.Actor.AccountID)
	msg := "%s "

	switch event.Type {
	case "approved":
		if err := data.RemoveFromTurn(url, email); err != nil {
			log.Error(ctx, "failed to remove user from Bitbucket PR's attention state", "error", err, "pr_url", url, "email", email)
		}
		msg += "approved this PR :+1:"
	case "unapproved":
		if err := data.AddReviewerToPR(url, email); err != nil {
			log.Error(ctx, "failed to add user back to Bitbucket PR's attention state", "error", err, "pr_url", url, "email", email)
		}
		if err := data.SwitchTurn(url, email); err != nil {
			log.Error(ctx, "failed to switch Bitbucket PR's attention state", "error", err, "pr_url", url, "email", email)
		}
		msg += "unapproved this PR :-1:"
	case "changes_request_created":
		if err := data.SwitchTurn(url, email); err != nil {
			log.Error(ctx, "failed to switch Bitbucket PR's attention state", "error", err, "pr_url", url, "email", email)
		}
		msg += "requested changes in this PR :warning:"

	// Ignored event type.
	case "changes_request_removed":

	default:
		log.Error(ctx, "unrecognized Bitbucket PR review event type", "event_type", event.Type)
	}

	return mentionUserInMsg(ctx, channelID, event.Actor, msg)
}

// A PR was closed, i.e. merged or rejected (possibly a draft).
func prClosedWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	// Ignore drafts - they don't have an active Slack channel anyway.
	if event.PullRequest.Draft {
		return nil
	}

	return archiveChannel(ctx, event)
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
	_ = addReaction(ctx, url, "ok")
	return mentionUserInReplyByURL(ctx, url, event.Actor, "%s resolved this comment :ok:")
}

func prCommentReopenedWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	channelID, found := lookupChannel(ctx, event.Type, event.PullRequest)
	if !found {
		return nil
	}

	defer updateChannelBookmarks(ctx, event, channelID, nil)

	url := htmlURL(event.Comment.Links)
	_ = removeReaction(ctx, url, "ok")
	return mentionUserInReplyByURL(ctx, url, event.Actor, "%s reopened this comment :no_good:")
}

func htmlURL(links map[string]Link) string {
	return links["html"].HRef
}

// beautifyInlineComment adds an informative prefix to the comment's text.
// If the comment contains a suggestion code block, it removes that block
// and also generates a diff snippet to attach to the Slack message instead.
func beautifyInlineComment(ctx workflow.Context, event PullRequestEvent, msg, raw string) (string, []byte) {
	msg = inlineCommentPrefix(htmlURL(event.Comment.Links), event.Comment.Inline) + msg
	msg = strings.TrimSpace(strings.TrimSuffix(msg, "\u200c"))

	suggestion, ok := extractSuggestion(raw)
	if !ok {
		return msg, nil
	}

	diff := generateDiff(ctx, event, suggestion, event.Comment.Links["code"].HRef)
	if diff == nil {
		return msg, nil
	}

	if suggestion != "" {
		suggestion += "\n"
	}
	msg = strings.Replace(msg, fmt.Sprintf("```suggestion\n%s", suggestion), "```\n"+string(diff), 1)

	return msg, diff
}

func inlineCommentPrefix(commentURL string, in *Inline) string {
	var line1 int
	if in.StartFrom != nil {
		line1 = *in.StartFrom
		if in.StartTo != nil && *in.StartTo < line1 {
			line1 = *in.StartTo
		}
	} else if in.StartTo != nil {
		line1 = *in.StartTo
	}

	var line2 int
	if in.From != nil {
		line2 = *in.From
		if in.To != nil && *in.To > line2 {
			line2 = *in.To
		}
	} else if in.To != nil {
		line2 = *in.To
	}

	if line1 == 0 {
		line1 = line2
	}
	if line2 == 0 {
		line2 = line1
	}

	subject := "Inline"
	location := "in"
	switch line1 {
	case 0: // No line info.
		subject = "File"
	case line2: // Single line.
		location = fmt.Sprintf("in line %d in", line1)
	default: // Multiple lines.
		location = fmt.Sprintf("in lines %d-%d in", line1, line2)
	}

	return fmt.Sprintf("<%s|%s comment> %s `%s`:\n", commentURL, subject, location, in.Path)
}

// extractSuggestion extracts the suggestion code block from a PR inline comment.
func extractSuggestion(raw string) (string, bool) {
	_, s, ok := strings.Cut(raw, "```suggestion\n")
	if !ok {
		return "", false
	}

	i := strings.LastIndex(s, "```")
	if i == -1 {
		return "", false
	}

	return strings.TrimSuffix(s[:i], "\n"), true
}

func generateDiff(ctx workflow.Context, event PullRequestEvent, suggestion, diffURL string) []byte {
	src := sourceFile(ctx, diffURL, event.Comment.Inline.SrcRev)
	if src == "" {
		return nil
	}

	return spliceSuggestion(event.Comment.Inline, suggestion, src)
}

// spliceSuggestion splices the suggestion into the source
// file content, and returns the result as a diff snippet.
func spliceSuggestion(in *Inline, suggestion, srcFile string) []byte {
	var line1, line2 int
	if in.To != nil {
		line1, line2 = *in.To, *in.To
	}
	if in.StartTo != nil {
		line1 = *in.StartTo
	}

	lenFrom := line2 - line1 + 1
	lenTo := 0
	if suggestion != "" {
		lenTo = strings.Count(suggestion, "\n") + 1
	}

	var diff bytes.Buffer
	diff.WriteString(fmt.Sprintf("@@ -%d,%d ", line1, lenFrom))
	if lenTo > 0 {
		diff.WriteString(fmt.Sprintf("+%d,%d ", line1, lenTo))
	}
	diff.WriteString("@@\n")

	for _, line := range strings.Split(srcFile, "\n")[line1-1 : line2] {
		diff.WriteString(fmt.Sprintf("-%s\n", line))
	}

	if suggestion == "" {
		return diff.Bytes()
	}

	for line := range strings.SplitSeq(suggestion, "\n") {
		diff.WriteString(fmt.Sprintf("+%s\n", line))
	}

	return diff.Bytes()
}
