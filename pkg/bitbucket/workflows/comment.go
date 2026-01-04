package workflows

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"strings"
	"time"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/bitbucket"
	bact "github.com/tzrikka/revchat/pkg/bitbucket/activities"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/markdown"
	sact "github.com/tzrikka/revchat/pkg/slack/activities"
	"github.com/tzrikka/revchat/pkg/users"
)

// CommentCreatedWorkflow mirrors the creation of a new PR comment in the PR's Slack channel:
// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Comment-created.1
func (c Config) CommentCreatedWorkflow(ctx workflow.Context, event bitbucket.PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to mirror this event.
	prURL := bitbucket.HTMLURL(event.PullRequest.Links)
	channelID, found := sact.LookupChannel(ctx, event.Type, prURL)
	if !found {
		return nil
	}

	defer bitbucket.UpdateChannelBookmarks(ctx, event, channelID, nil)

	// Don't abort if this fails - it's more important to post the comment.
	email := users.BitbucketIDToEmail(ctx, event.Actor.AccountID)
	_ = data.SwitchTurn(ctx, prURL, email)

	// If the comment was created by RevChat, don't repost it. However, we still
	// need to poll for updates (comment created in Slack, but edited in Bitbucket).
	commentURL := bitbucket.HTMLURL(event.Comment.Links)
	if strings.HasSuffix(event.Comment.Content.Raw, "\n\n[This comment was created by RevChat]: #") {
		logger.From(ctx).Debug("ignoring self-triggered Bitbucket event")
		c.pollCommentForUpdates(ctx, event.Comment.User.AccountID, commentURL, event.Comment.Content.Raw)
		return nil
	}

	msg := markdown.BitbucketToSlack(ctx, event.Comment.Content.Raw, prURL)
	var diff []byte
	if event.Comment.Inline != nil {
		msg, diff = bitbucket.BeautifyInlineComment(ctx, event.Comment, msg)
	}

	var err error
	if event.Comment.Parent == nil {
		err = bitbucket.ImpersonateUserInMsg(ctx, commentURL, channelID, event.Comment.User, msg, diff)
	} else {
		parentURL := bitbucket.HTMLURL(event.Comment.Parent.Links)
		err = bitbucket.ImpersonateUserInReply(ctx, commentURL, parentURL, event.Comment.User, msg, diff)
	}

	// If the comment posting failed, there's no point in polling for updates (but don't ignore that error).
	if err == nil {
		c.pollCommentForUpdates(ctx, event.Comment.User.AccountID, commentURL, event.Comment.Content.Raw)
	}
	return err
}

// CommentUpdatedWorkflow mirrors an edit of an existing PR comment in the PR's Slack channel:
// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Comment-updated
//
// Note: these events are not reported by Bitbucket if they occur within a [10-minute window]
// after the creation or last update of the same PR comment. As a workaround, we actively
// poll Bitbucket to detect text changes within these windows: see [Config.PollCommentWorkflow].
//
// [10-minute window]: https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Comment-updated
func (c Config) CommentUpdatedWorkflow(ctx workflow.Context, event bitbucket.PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to mirror this event.
	channelID, found := sact.LookupChannel(ctx, event.Type, bitbucket.HTMLURL(event.PullRequest.Links))
	if !found {
		return nil
	}

	defer bitbucket.UpdateChannelBookmarks(ctx, event, channelID, nil)

	return c.updateCommentInWorkflow(ctx, event.Comment)
}

func (c Config) updateCommentInWorkflow(ctx workflow.Context, comment *bitbucket.Comment) error {
	// If the comment previously had an attached diff file, delete it - it's obsolete now.
	if fileID, _ := data.SwitchURLAndID(ctx, bitbucket.HTMLURL(comment.Links)+"/slack_file_id"); fileID != "" {
		sact.DeleteFile(ctx, fileID)
	}

	commentURL := bitbucket.HTMLURL(comment.Links)
	msg := markdown.BitbucketToSlack(ctx, comment.Content.Raw, commentURL)
	var diff []byte
	if comment.Inline != nil {
		msg, diff = bitbucket.BeautifyInlineComment(ctx, comment, msg)
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
		msg = strings.Replace(bitbucket.ImpersonationToMention(buf.String()), "%s", bitbucket.SlackDisplayName(ctx, comment.User), 1)
	}

	// Unlike comment creation, even if mirroring this update in Slack fails, we still need to poll for updates.
	c.pollCommentForUpdates(ctx, comment.User.AccountID, commentURL, comment.Content.Raw)

	return bitbucket.EditMsg(ctx, commentURL, msg)
}

// CommentDeletedWorkflow mirrors the deletion of a PR comment in the PR's Slack channel:
// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Comment-deleted
func (c Config) CommentDeletedWorkflow(ctx workflow.Context, event bitbucket.PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to mirror this event.
	channelID, found := sact.LookupChannel(ctx, event.Type, bitbucket.HTMLURL(event.PullRequest.Links))
	if !found {
		return nil
	}

	defer bitbucket.UpdateChannelBookmarks(ctx, event, channelID, nil)

	commentURL := bitbucket.HTMLURL(event.Comment.Links)
	defer c.stopPollingComment(ctx, commentURL)

	if fileID, _ := data.SwitchURLAndID(ctx, commentURL+"/slack_file_id"); fileID != "" {
		sact.DeleteFile(ctx, fileID)
	}

	return bitbucket.DeleteMsg(ctx, commentURL)
}

// CommentResolvedWorkflow mirrors the resolution of a PR comment in the PR's Slack channel:
// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Comment-resolved
func CommentResolvedWorkflow(ctx workflow.Context, event bitbucket.PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	channelID, found := sact.LookupChannel(ctx, event.Type, bitbucket.HTMLURL(event.PullRequest.Links))
	if !found {
		return nil
	}

	defer bitbucket.UpdateChannelBookmarks(ctx, event, channelID, nil)

	url := bitbucket.HTMLURL(event.Comment.Links)
	sact.AddOKReaction(ctx, url) // The mention below is more important than this reaction.
	return bitbucket.MentionUserInReply(ctx, url, event.Actor, "%s resolved this comment. :ok:")
}

// CommentReopenedWorkflow mirrors the reopening of a resolved PR comment in the PR's Slack channel:
// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Comment-reopened
func CommentReopenedWorkflow(ctx workflow.Context, event bitbucket.PullRequestEvent) error {
	// If we're not tracking this PR, there's no need/way to announce this event.
	channelID, found := sact.LookupChannel(ctx, event.Type, bitbucket.HTMLURL(event.PullRequest.Links))
	if !found {
		return nil
	}

	defer bitbucket.UpdateChannelBookmarks(ctx, event, channelID, nil)

	url := bitbucket.HTMLURL(event.Comment.Links)
	sact.RemoveOKReaction(ctx, url) // The mention below is more important than this reaction.
	return bitbucket.MentionUserInReply(ctx, url, event.Actor, "%s reopened this comment. :no_good:")
}

const (
	CommentPollingInterval = 10 * time.Second
	CommentPollingWindow   = 10 * time.Minute
)

func checksum(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

// pollCommentForUpdates is a convenience wrapper for [setScheduleActivity].
func (c Config) pollCommentForUpdates(ctx workflow.Context, accountID, commentURL, rawText string) {
	user := data.SelectUserByBitbucketID(ctx, accountID)

	ctx = workflow.WithLocalActivityOptions(ctx, workflow.LocalActivityOptions{
		StartToCloseTimeout: CommentPollingInterval,
	})
	fut := workflow.ExecuteLocalActivity(ctx, c.setScheduleActivity, user.ThrippyLink, commentURL, checksum(rawText))
	_ = fut.Get(ctx, nil)
}

// setScheduleActivity is a Temporal local activity that creates or updates a Temporal schedule to poll
// a specific PR comment in order to detect edits made within Bitbucket's [10-minute silent window].
// This schedule will run [Config.PollCommentWorkflow] every [CommentPollingInterval] during
// [CommentPollingWindow], or until the comment is deleted.
//
// [10-minute window]: https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Comment-updated
func (c Config) setScheduleActivity(ctx context.Context, linkID, commentURL, checksum string) error {
	l := activity.GetLogger(ctx)
	cli, err := client.Dial(c.Opts)
	if err != nil {
		l.Error("failed to dial Temporal", slog.Any("error", err))
		return err
	}
	defer cli.Close()

	// Common parameters for the schedule, whether we create or update it.
	sched := &client.Schedule{
		Action: &client.ScheduleWorkflowAction{
			ID:        trimURLPrefix(commentURL),
			Workflow:  Schedules[0],
			Args:      []any{linkID, commentURL, checksum},
			TaskQueue: c.TaskQueue,
		},
		Spec: &client.ScheduleSpec{
			Intervals: []client.ScheduleIntervalSpec{
				{Every: CommentPollingInterval - time.Second}, // See jitter comment below.
			},
			Jitter: 2 * time.Second, // Exactly 10s --> Approximately 9s + (0-2s jitter).
			EndAt:  time.Now().UTC().Add(CommentPollingWindow).Add(5 * time.Second),
		},
		Policy: &client.SchedulePolicies{
			Overlap:       enums.SCHEDULE_OVERLAP_POLICY_SKIP,
			CatchupWindow: CommentPollingInterval,
		},
	}

	// Try to update an existing schedule first.
	handle := cli.ScheduleClient().GetHandle(ctx, trimURLPrefix(commentURL))
	if _, err := handle.Describe(ctx); err == nil {
		err = handle.Update(ctx, client.ScheduleUpdateOptions{
			DoUpdate: func(input client.ScheduleUpdateInput) (*client.ScheduleUpdate, error) {
				sched.State = input.Description.Schedule.State
				return &client.ScheduleUpdate{Schedule: sched}, nil
			},
		})
		if err == nil {
			l.Info("restarted Bitbucket PR comment polling schedule", slog.String("comment_url", commentURL))
			return nil
		}
	}

	// If updating failed (regardless of whether that schedule exists or not), create a new schedule.
	_, err = cli.ScheduleClient().Create(ctx, client.ScheduleOptions{
		ID:            trimURLPrefix(commentURL),
		Spec:          *sched.Spec,
		Action:        sched.Action,
		Overlap:       sched.Policy.Overlap,
		CatchupWindow: sched.Policy.CatchupWindow,
	})
	if err != nil {
		l.Error("failed to create Bitbucket PR comment polling schedule",
			slog.Any("error", err), slog.String("comment_url", commentURL))
		return err
	}

	l.Info("started new Bitbucket PR comment polling schedule", slog.String("comment_url", commentURL))
	return nil
}

func trimURLPrefix(commentURL string) string {
	return strings.TrimPrefix(commentURL, "https://bitbucket.org/")
}

// PollCommentWorkflow checks a specific PR comment to detect and mirror edits made within Bitbucket's
// [10-minute silent window] after its creation or last update, instead of [CommentUpdatedWorkflow].
// This workflow runs in a Temporal schedule, every [CommentPollingInterval] during
// [CommentPollingWindow], or until the comment is deleted.
//
// This workflow uses a checksum of the comment's text for privacy and efficiency reasons.
//
// [10-minute window]: https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Comment-updated
func (c Config) PollCommentWorkflow(ctx workflow.Context, linkID, commentURL, oldChecksum string) error {
	comment, err := bact.GetPullRequestComment(ctx, linkID, commentURL)
	if err != nil {
		return err
	}

	if comment.Deleted {
		logger.From(ctx).Info("Bitbucket PR comment deleted, stopping polling schedule", slog.String("comment_url", commentURL))
		c.stopPollingComment(ctx, commentURL)
		return nil
	}

	if checksum(comment.Content.Raw) != oldChecksum {
		logger.From(ctx).Info("Bitbucket PR comment text changed, updating Slack message and polling schedule",
			slog.String("comment_url", commentURL))
		return c.updateCommentInWorkflow(ctx, comment)
	}

	return nil
}

// stopPollingComment is a convenience wrapper for [unsetScheduleActivity].
func (c Config) stopPollingComment(ctx workflow.Context, commentURL string) {
	ctx = workflow.WithLocalActivityOptions(ctx, workflow.LocalActivityOptions{
		StartToCloseTimeout: CommentPollingInterval,
	})
	_ = workflow.ExecuteLocalActivity(ctx, c.unsetScheduleActivity, commentURL).Get(ctx, nil)
}

func (c Config) unsetScheduleActivity(ctx context.Context, commentURL string) error {
	l := activity.GetLogger(ctx)
	cli, err := client.Dial(c.Opts)
	if err != nil {
		l.Error("failed to dial Temporal", slog.Any("error", err))
		return err
	}
	defer cli.Close()

	handle := cli.ScheduleClient().GetHandle(ctx, trimURLPrefix(commentURL))
	if _, err := handle.Describe(ctx); err == nil {
		if err := handle.Delete(ctx); err != nil {
			l.Error("failed to delete Bitbucket PR comment polling schedule",
				slog.Any("error", err), slog.String("comment_url", commentURL))
			return err
		}
	}

	return nil
}
