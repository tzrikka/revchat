package github

import (
	"time"

	"github.com/tzrikka/timpani-api/pkg/github"
)

// IssueCommentEvent is based on:
// https://docs.github.com/en/webhooks/webhook-events-and-payloads#issue_comment
type IssueCommentEvent struct {
	Action  string       `json:"action"`
	Issue   Issue        `json:"issue"`
	Comment IssueComment `json:"comment"`
	Sender  User         `json:"sender"`

	Changes *Changes `json:"changes,omitempty"`

	// Repository   `json:"repository"`
	// Organization `json:"organization"`
	// Installation `json:"installation"`
}

// PullRequestEvent is based on:
// https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request
type PullRequestEvent struct {
	Action      string      `json:"action"`
	Number      int         `json:"number"`
	PullRequest PullRequest `json:"pull_request"`
	Sender      User        `json:"sender"`

	Assignee          *User    `json:"assignee,omitempty"`
	RequestedReviewer *User    `json:"requested_reviewer,omitempty"`
	RequestedTeam     *Team    `json:"requested_team,omitempty"`
	Changes           *Changes `json:"changes,omitempty"`
	Before            *string  `json:"before,omitempty"`
	After             *string  `json:"after,omitempty"`

	// Repository   `json:"repository"`
	// Organization `json:"organization"`
	// Installation `json:"installation"`
}

// PullRequestReviewEvent is based on:
// https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request_review
type PullRequestReviewEvent struct {
	Action      string      `json:"action"`
	Review      Review      `json:"review"`
	PullRequest PullRequest `json:"pull_request"`
	Sender      User        `json:"sender"`

	Changes *Changes `json:"changes,omitempty"`

	// Repository   `json:"repository"`
	// Organization `json:"organization"`
	// Installation `json:"installation"`
}

// PullRequestReviewCommentEvent is based on:
// https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request_review_comment
type PullRequestReviewCommentEvent struct {
	Action      string      `json:"action"`
	Comment     PullComment `json:"comment"`
	PullRequest PullRequest `json:"pull_request"`
	Sender      User        `json:"sender"`

	Changes *Changes `json:"changes,omitempty"`

	// Repository   `json:"repository"`
	// Organization `json:"organization"`
	// Installation `json:"installation"`
}

// PullRequestReviewThreadEvent is based on:
// https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request_review_thread
type PullRequestReviewThreadEvent struct {
	Action      string      `json:"action"`
	Thread      Thread      `json:"thread"`
	PullRequest PullRequest `json:"pull_request"`
	Sender      User        `json:"sender"`

	UpdatedAt time.Time `json:"updated_at,omitzero"`

	// Repository   `json:"repository"`
	// Organization `json:"organization"`
	// Installation `json:"installation"`
}

type (
	Issue        = github.Issue
	IssueComment = github.IssueComment
	PullComment  = github.PullComment
	PullRequest  = github.PullRequest
	Review       = github.Review
	Team         = github.Team
	User         = github.User
)

type Changes struct {
	Title *ChangeFrom `json:"title,omitempty"`
	Body  *ChangeFrom `json:"body,omitempty"`
	Base  *ChangeBase `json:"base,omitempty"`
}

type ChangeBase struct {
	Ref ChangeFrom `json:"ref"`
	SHA ChangeFrom `json:"sha"`
}

type ChangeFrom struct {
	From string `json:"from"`
}

type Installation struct {
	ID     int64  `json:"id"`
	NodeID string `json:"node_id"`
}

type Organization struct {
	ID     int64  `json:"id"`
	NodeID string `json:"node_id"`
	Login  string `json:"login"`
}

// Thread is based on:
// https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request_review_thread
type Thread struct {
	NodeID   string        `json:"node_id"`
	Comments []PullComment `json:"comments"`
}
