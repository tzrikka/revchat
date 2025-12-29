package github

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

	Assignee          *User `json:"assignee,omitempty"`
	RequestedReviewer *User `json:"requested_reviewer,omitempty"`
	RequestedTeam     *Team `json:"requested_team,omitempty"`

	Changes *Changes `json:"changes,omitempty"`
	Before  *string  `json:"before,omitempty"`
	After   *string  `json:"after,omitempty"`

	// Repository   `json:"repository"`
	// Organization `json:"organization"`
	// Installation `json:"installation"`
}

// PullRequestReviewEvent is based on:
// https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request_review
type PullRequestReviewEvent struct {
	Action      string      `json:"action"`
	PullRequest PullRequest `json:"pull_request"`
	Review      Review      `json:"review"`
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
	PullRequest PullRequest `json:"pull_request"`
	Comment     PullComment `json:"comment"`
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
	PullRequest PullRequest `json:"pull_request"`
	Thread      Thread      `json:"thread"`
	Sender      User        `json:"sender"`

	// Repository   `json:"repository"`
	// Organization `json:"organization"`
	// Installation `json:"installation"`
}

type AutoMerge struct {
	EnabledBy     User   `json:"enabled_by"`
	MergeMethod   string `json:"merge_method"`
	CommitTitle   string `json:"commit_title"`
	CommitMessage string `json:"commit_message"`
}

type Branch struct {
	Label string     `json:"label"`
	Ref   string     `json:"ref"`
	SHA   string     `json:"sha"`
	Repo  Repository `json:"repo"`
	User  User       `json:"user"`
}

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
	ID int64 `json:"id"`
}

// Issue is based on:
// https://docs.github.com/en/rest/issues/issues?apiVersion=2022-11-28#get-an-issue
type Issue = map[string]any

// IssueComment is based on:
// https://docs.github.com/en/rest/issues/comments?apiVersion=2022-11-28#get-an-issue-comment
type IssueComment struct {
	ID      int    `json:"id"`
	HTMLURL string `json:"html_url"`
	User    User   `json:"user"`

	Body      string     `json:"body"`
	Reactions *Reactions `json:"reactions,omitempty"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type Organization struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

// PullComment is based on:
// https://docs.github.com/en/rest/pulls/comments?apiVersion=2022-11-28#get-a-review-comment-for-a-pull-request
type PullComment struct {
	ID                  int    `json:"id"`
	PullRequestReviewID int    `json:"pull_request_review_id"`
	InReplyTo           *int   `json:"in_reply_to_id,omitempty"`
	HTMLURL             string `json:"html_url"`
	User                User   `json:"user"`

	CommitID         string `json:"commit_id"`
	OriginalCommitID string `json:"original_commit_id"`
	Path             string `json:"path"`

	Body        string     `json:"body"`
	DiffHunk    string     `json:"diff_hunk"`
	SubjectType string     `json:"subject_type,omitempty"`
	Reactions   *Reactions `json:"reactions,omitempty"`

	StartLine         *int    `json:"start_line,omitempty"`
	OriginalStartLine *int    `json:"original_start_line,omitempty"`
	StartSide         *string `json:"start_side,omitempty"`
	Line              *int    `json:"line,omitempty"`
	OriginalLine      *int    `json:"original_line,omitempty"`
	Side              *string `json:"side,omitempty"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// PullRequest is based on:
//   - https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request
//   - https://docs.github.com/en/rest/pulls/pulls?apiVersion=2022-11-28#get-a-pull-request
type PullRequest struct {
	ID     int64 `json:"id"`
	Number int   `json:"number"`

	HTMLURL  string `json:"html_url"`
	DiffURL  string `json:"diff_url"`
	PatchURL string `json:"patch_url"`

	Title string  `json:"title"`
	Body  *string `json:"body"`
	State string  `json:"state"`

	User               User   `json:"user"`
	Assignee           *User  `json:"assignee"`
	Assignees          []User `json:"assignees"`
	RequestedReviewers []User `json:"requested_reviewers"`
	RequestedTeams     []Team `json:"requested_teams"`

	Head Branch `json:"head"`
	Base Branch `json:"base"`

	CreatedAt      string     `json:"created_at"`
	UpdatedAt      string     `json:"updated_at"`
	ClosedAt       *string    `json:"closed_at"`
	MergedAt       *string    `json:"merged_at"`
	MergedBy       *User      `json:"merged_by"`
	MergeCommitSHA *string    `json:"merge_commit_sha"`
	AutoMerge      *AutoMerge `json:"auto_merge"`

	// Labels    []Label    `json:"labels"`
	// Milestone *Milestone `json:"milestone"`

	ActiveLockReason *string `json:"active_lock_reason"`
	Draft            bool    `json:"draft"`
	Locked           bool    `json:"locked"`
	Merged           bool    `json:"merged"`
	Mergeable        *bool   `json:"mergeable"`
	Rebaseable       *bool   `json:"rebaseable"`
	MergeableState   string  `json:"mergeable_state"`
	Comments         int     `json:"comments"`
	ReviewComments   int     `json:"review_comments"`
	Commits          int     `json:"commits"`
	Additions        int     `json:"additions"`
	Deletions        int     `json:"deletions"`
	ChangedFiles     int     `json:"changed_files"`
}

type Reactions struct {
	URL string `json:"url"`

	TotalCount int `json:"total_count"`
	PlusOne    int `json:"+1"`
	MinusOne   int `json:"-1"`
	Laugh      int `json:"laugh"`
	Confused   int `json:"confused"`
	Heart      int `json:"heart"`
	Hooray     int `json:"hooray"`
	Eyes       int `json:"eyes"`
	Rocket     int `json:"rocket"`
}

type Repository struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	HTMLURL  string `json:"html_url"`

	// Owner User `json:"owner"` // Unnecessary.
}

// Review is based on:
// https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request_review
// https://docs.github.com/en/rest/pulls/reviews?apiVersion=2022-11-28#get-a-review-for-a-pull-request
type Review struct {
	ID      int    `json:"id"`
	HTMLURL string `json:"html_url"`
	User    *User  `json:"user"`

	Body  string `json:"body"`
	State string `json:"state"`

	SubmittedAt string `json:"submitted_at"`
	UpdatedAt   string `json:"updated_at,omitempty"`
	CommitID    string `json:"commit_id"`
}

type Team struct {
	ID          int64   `json:"id"`
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	HTMLURL     string  `json:"html_url"`
}

// Thread is based on:
// https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request_review_thread
type Thread = map[string]any

type User struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	HTMLURL   string `json:"html_url"`
	Type      string `json:"type"`
	SiteAdmin bool   `json:"site_admin"`
}
