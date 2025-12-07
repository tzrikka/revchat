package bitbucket

import (
	"github.com/tzrikka/timpani-api/pkg/bitbucket"
)

// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Pull-request-events
// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Common-entities-for-event-payloads
type PullRequestEvent struct {
	Type string `json:"type"` // Defined and used internally by us.

	PullRequest PullRequest `json:"pullrequest"`
	Repository  Repository  `json:"repository"`
	Actor       Account     `json:"actor"`

	Approval       *Review  `json:"approval,omitempty"`
	ChangesRequest *Review  `json:"changes_request,omitempty"`
	Comment        *Comment `json:"comment,omitempty"`
}

// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Repository-events
type RepositoryEvent struct {
	Type string `json:"type"` // Defined and used internally by us.

	Repository Repository `json:"repository"`
	Actor      Account    `json:"actor"`

	Commit       *Commit       `json:"commit,omitempty"`
	Comment      *Comment      `json:"comment,omitempty"`
	CommitStatus *CommitStatus `json:"commit_status,omitempty"`
}

type Account = bitbucket.User

type Branch struct {
	Name string `json:"name"`

	Links          map[string]Link `json:"links,omitempty"`
	SyncStrategies []string        `json:"sync_strategies,omitempty"`
}

// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Comment
type Comment struct {
	// Type string `json:"type"` // Always "pullrequest_comment".

	ID     int     `json:"id"`
	Parent *Parent `json:"parent,omitempty"`

	CreatedOn string `json:"created_on"`
	UpdatedOn string `json:"updated_on"`
	Deleted   bool   `json:"deleted"`
	Pending   bool   `json:"pending"`

	Content Rendered `json:"content"`
	Inline  *Inline  `json:"inline"`
	User    Account  `json:"user"`
	// PullRequest `json:"pullrequest"` // Unnecessary.

	Links map[string]Link `json:"links"`
}

type Commit = bitbucket.Commit

// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Build-status-created
type CommitStatus struct {
	// Type string `json:"type"` // Always "build".

	Name        string `json:"name"`
	State       string `json:"state"`
	Description string `json:"description"`
	Key         string `json:"key"`
	URL         string `json:"url"`

	Refname *string `json:"refname"`
	Commit  *Commit `json:"commit,omitempty"`
	// Repository *Repository `json:"repository,omitempty"` // Unnecessary.

	CreatedOn string `json:"created_on"`
	UpdatedOn string `json:"updated_on"`

	Links map[string]Link `json:"links"`
}

// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Comment
type Inline struct {
	Path string `json:"path"`

	StartFrom *int `json:"start_from"`
	StartTo   *int `json:"start_to"`
	From      *int `json:"from"`
	To        *int `json:"to"`

	ContextLines string `json:"context_lines"`
	Outdated     bool   `json:"outdated"`

	SrcRev  string `json:"src_rev"`
	DestRev string `json:"dest_rev"`
	// BaseRev *string `json:"base_rev"`
}

type Link struct {
	HRef string `json:"href"`
}

type Parent struct {
	ID    int             `json:"id"`
	Links map[string]Link `json:"links"`
}

type Participant struct {
	// Type string `json:"type"` // Always "participant".

	User           Account `json:"user"`
	Role           string  `json:"role"` // "PARTICIPANT" / "REVIEWER".
	Approved       bool    `json:"approved"`
	State          *string `json:"state"` // Nil / "changes_requested" / "approved".
	ParticipatedOn *string `json:"participated_on"`
}

type Project struct {
	// Type string `json:"type"` // Always "project".

	Key  string `json:"key"`
	Name string `json:"name"`
	UUID string `json:"uuid"`

	Links map[string]Link `json:"links"`
}

type PullRequest struct {
	// Type string `json:"type"` // Always "pullrequest".

	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	State       string `json:"state"` // "OPEN", "MERGED", "DECLINED".
	Draft       bool   `json:"draft"`

	// Summary     Rendered            `json:"summary"`  // Unnecessary.
	// Rendered    map[string]Rendered `json:"rendered"` // Unnecessary.
	Source      Reference `json:"source"`
	Destination Reference `json:"destination"`

	CreatedOn    string `json:"created_on"`
	UpdatedOn    string `json:"updated_on"`
	CommentCount int    `json:"comment_count"`
	TaskCount    int    `json:"task_count"`

	CommitCount        int `json:"commit_count"`         // Populated and used only in RevChat.
	ChangeRequestCount int `json:"change_request_count"` // Populated and used only in RevChat.

	Author       Account       `json:"author"`
	Participants []Participant `json:"participants"`
	Reviewers    []Account     `json:"reviewers"`
	ClosedBy     *Account      `json:"closed_by"`
	Reason       string        `json:"reason"`

	CloseSourceBranch bool    `json:"close_source_branch"`
	MergeCommit       *Commit `json:"merge_commit"`

	Links map[string]Link `json:"links"`
}

type Reference struct {
	Branch     Branch     `json:"branch"`
	Commit     Commit     `json:"commit"`
	Repository Repository `json:"repository"`
}

type Rendered struct {
	// Type string `json:"type"` // Always "rendered".

	Raw    string `json:"raw"`
	Markup string `json:"markup"`
	HTML   string `json:"html"`
}

type Repository struct {
	// Type string `json:"type"` // Always "repository".

	FullName string `json:"full_name"`
	Name     string `json:"name"`
	UUID     string `json:"uuid"`

	SCM       string `json:"scm,omitempty"`
	IsPrivate bool   `json:"is_private,omitempty"`
	Website   string `json:"website,omitempty"`

	Workspace *Workspace `json:"workspace,omitempty"`
	Project   *Project   `json:"project,omitempty"`
	Owner     *Account   `json:"owner,omitempty"`

	Links map[string]Link `json:"links"`
}

type Review struct {
	Date string  `json:"date"`
	User Account `json:"user"`
}

type Workspace struct {
	// Type string `json:"type"` // Always "workspace".

	Slug string `json:"slug"`
	Name string `json:"name"`
	UUID string `json:"uuid"`

	Links map[string]Link `json:"links"`
}
