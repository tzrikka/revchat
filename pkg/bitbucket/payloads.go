package bitbucket

// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Pull-request-events
// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Common-entities-for-event-payloads
type PullRequestEvent struct {
	Type string `json:"type"` // Defined and used internally by us.

	PullRequest PullRequest `json:"pullrequest"`
	Repository  Repository  `json:"repository"`
	Actor       Account     `json:"actor"`

	Approval       *Review  `json:"approval,omiyempty"`
	ChangesRequest *Review  `json:"changes_request,omiyempty"`
	Comment        *Comment `json:"comment,omiyempty"`
}

// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Repository-events
type RepositoryEvent struct {
	Type string `json:"type"` // Defined and used internally by us.

	Repository Repository `json:"repository"`
	Actor      Account    `json:"actor"`

	Commit  *Commit  `json:"commit,omiyempty"`
	Comment *Comment `json:"comment,omiyempty"`
}

type Account struct {
	Type string `json:"type"`

	DisplayName string `json:"display_name"`
	Nickname    string `json:"nickname"`
	AccountID   string `json:"account_id"`
	UUID        string `json:"uuid"`

	Links map[string]Link `json:"links"`
}

type Branch struct {
	Name string `json:"name"`

	Links          map[string]Link `json:"links"`
	SyncStrategies []string        `json:"sync_strategies"`
}

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

type Commit struct {
	// Type string `json:"type"` // Always "commit".

	Hash  string          `json:"hash"`
	Links map[string]Link `json:"links"`
}

type Inline struct {
	From         *int   `json:"from"`
	To           *int   `json:"to"`
	Path         string `json:"path"`
	Outdated     bool   `json:"outdated"`
	ContextLines string `json:"context_lines"`
	SrcRev       string `json:"src_rev"`
	DestRev      string `json:"dest_rev"`
}

type Link struct {
	HRef string `json:"href"`
}

type Parent struct {
	ID int `json:"id"`
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

	Summary     Rendered            `json:"summary"`
	Rendered    map[string]Rendered `json:"rendered"`
	Source      Reference           `json:"source"`
	Destination Reference           `json:"destination"`

	CreatedOn    string `json:"created_on"`
	UpdatedOn    string `json:"updated_on"`
	CommentCount int    `json:"comment_count"`
	TaskCount    int    `json:"task_count"`

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

	SCM       string  `json:"scm"`
	IsPrivate bool    `json:"is_private"`
	Website   *string `json:"website"`

	Workspace Workspace `json:"workspace"`
	Project   Project   `json:"project"`
	Owner     Account   `json:"owner"`

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
