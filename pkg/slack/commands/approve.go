package commands

import (
	"errors"
	"log/slog"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/timpani-api/pkg/bitbucket"
)

func Approve(ctx workflow.Context, event SlashCommandEvent, bitbucketWorkspace string) error {
	url := prDetailsFromChannel(ctx, event)
	if url == nil {
		return nil
	}

	var err error
	switch {
	case bitbucketWorkspace != "":
		req := bitbucket.PullRequestsApproveRequest{Workspace: url[1], RepoSlug: url[2], PullRequestID: url[3]}
		err = bitbucket.PullRequestsApprove(ctx, req)
	default:
		logger.From(ctx).Error("neither Bitbucket nor GitHub are configured")
		PostEphemeralError(ctx, event, "internal configuration error.")
		return errors.New("neither Bitbucket nor GitHub are configured")
	}

	if err != nil {
		logger.From(ctx).Error("failed to approve PR", slog.Any("error", err), slog.String("pr_url", url[0]))
		PostEphemeralError(ctx, event, "failed to approve "+url[0])
		return err
	}

	// No need to post a confirmation message or update its bookmarks,
	// the resulting Bitbucket/GitHub event will trigger that.
	return nil
}

func Unapprove(ctx workflow.Context, event SlashCommandEvent, bitbucketWorkspace string) (err error) {
	url := prDetailsFromChannel(ctx, event)
	if url == nil {
		return nil
	}

	switch {
	case bitbucketWorkspace != "":
		req := bitbucket.PullRequestsUnapproveRequest{Workspace: url[1], RepoSlug: url[2], PullRequestID: url[3]}
		err = bitbucket.PullRequestsUnapprove(ctx, req)
	default:
		logger.From(ctx).Error("neither Bitbucket nor GitHub are configured")
		PostEphemeralError(ctx, event, "internal configuration error.")
		return errors.New("neither Bitbucket nor GitHub are configured")
	}

	if err != nil {
		logger.From(ctx).Error("failed to unapprove PR", slog.Any("error", err), slog.String("pr_url", url[0]))
		PostEphemeralError(ctx, event, "failed to unapprove "+url[0])
		return err
	}

	// No need to post a confirmation message or update its bookmarks,
	// the resulting Bitbucket/GitHub event will trigger that.
	return nil
}
