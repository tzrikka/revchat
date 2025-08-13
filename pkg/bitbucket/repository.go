package bitbucket

import (
	"go.temporal.io/sdk/workflow"
)

// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Repository-events
func (b Bitbucket) handleRepositoryEvent(ctx workflow.Context, event RepositoryEvent) {
	switch event.Type {
	case "commit_comment_created":
		b.commitCommentCreated()

	case "build_status_created":
		b.buildStatusCreated()
	case "build_status_updated":
		b.buildStatusUpdated()

	default:
		workflow.GetLogger(ctx).Error("unrecognized Bitbucket repo event type", "event_type", event.Type)
	}
}

func (b Bitbucket) commitCommentCreated() {
}

func (b Bitbucket) buildStatusCreated() {
}

func (b Bitbucket) buildStatusUpdated() {
}
