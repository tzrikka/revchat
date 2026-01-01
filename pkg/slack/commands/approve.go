package commands

import (
	"errors"
	"log/slog"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/timpani-api/pkg/bitbucket"
)

func Approve(ctx workflow.Context, event SlashCommandEvent, bitbucketWorkspace string) error {
	url, err := prDetailsFromChannel(ctx, event)
	if url == nil {
		return err // May or may not be nil.
	}

	user, _, err := UserDetails(ctx, event, event.UserID)
	if err != nil {
		return err
	}

	switch {
	case bitbucketWorkspace != "":
		err = bitbucket.PullRequestsApprove(ctx, user.ThrippyLink, url[1], url[2], url[3])
	default:
		logger.From(ctx).Error("neither Bitbucket nor GitHub are configured")
		PostEphemeralError(ctx, event, "internal configuration error.")
		return errors.New("neither Bitbucket nor GitHub are configured")
	}

	if err != nil {
		logger.From(ctx).Error("failed to approve PR", slog.Any("error", err), slog.String("pr_url", url[0]),
			slog.String("slack_user_id", event.UserID), slog.String("thrippy_link_id", user.ThrippyLink))
		PostEphemeralError(ctx, event, "failed to approve "+url[0])
		return err
	}

	// No need to post a confirmation message or update its bookmarks,
	// the resulting Bitbucket/GitHub event will trigger that.
	return nil
}

func Unapprove(ctx workflow.Context, event SlashCommandEvent, bitbucketWorkspace string) (err error) {
	url, err := prDetailsFromChannel(ctx, event)
	if url == nil {
		return err // May or may not be nil.
	}

	user, _, err := UserDetails(ctx, event, event.UserID)
	if err != nil {
		return err
	}

	switch {
	case bitbucketWorkspace != "":
		err = bitbucket.PullRequestsUnapprove(ctx, user.ThrippyLink, url[1], url[2], url[3])
	default:
		logger.From(ctx).Error("neither Bitbucket nor GitHub are configured")
		PostEphemeralError(ctx, event, "internal configuration error.")
		return errors.New("neither Bitbucket nor GitHub are configured")
	}

	if err != nil {
		logger.From(ctx).Error("failed to unapprove PR", slog.Any("error", err), slog.String("pr_url", url[0]),
			slog.String("slack_user_id", event.UserID), slog.String("thrippy_link_id", user.ThrippyLink))
		PostEphemeralError(ctx, event, "failed to unapprove "+url[0])
		return err
	}

	// No need to post a confirmation message or update its bookmarks,
	// the resulting Bitbucket/GitHub event will trigger that.
	return nil
}
