package revchat

// https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request
type PullRequestEvent struct {
	Action       string       `json:"action"`
	Installation Installation `json:"installation"`
	Number       int          `json:"number"`
	Organization Organization `json:"organization"`
	PullRequest  PullRequest  `json:"pull_request"`
	Repository   Repository   `json:"repository"`
	Sender       User         `json:"sender"`

	Assignee          *User `json:"assignee"`
	RequestedReviewer *User `json:"requested_reviewer"`
	RequestedTeam     *Team `json:"requested_team"`

	Changes *Changes `json:"changes"`
	Before  *string  `json:"before"`
	After   *string  `json:"after"`
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
	Base struct {
		Ref Change `json:"ref"`
		SHA Change `json:"sha"`
	} `json:"base"`
	Body  Change `json:"body"`
	Title Change `json:"title"`
}

type Change struct {
	From string `json:"from"`
}

type Installation struct {
	ID int64 `json:"id"`
}

type Organization struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

type PullRequest struct {
	ID     int64 `json:"id"`
	Number int   `json:"number"`

	HTMLURL  string  `json:"html_url"`
	DiffURL  string  `json:"diff_url"`
	PatchURL string  `json:"patch_url"`
	Title    string  `json:"title"`
	Body     *string `json:"body"`
	State    string  `json:"state"`

	AuthorAssociation  string `json:"author_association"`
	User               User   `json:"user"`
	Assignee           *User  `json:"assignee"`
	Assignees          []User `json:"assignees"`
	RequestedReviewers []User `json:"requested_reviewers"`
	RequestedTeams     []Team `json:"requested_teams"`

	CreatedAt      string     `json:"created_at"`
	UpdatedAt      string     `json:"updated_at"`
	ClosedAt       *string    `json:"closed_at"`
	MergedAt       *string    `json:"merged_at"`
	MergedBy       *User      `json:"merged_by"`
	MergeCommitSHA *string    `json:"merge_commit_sha"`
	AutoMerge      *AutoMerge `json:"auto_merge"`

	// Labels    []Label    `json:"labels"`
	// Milestone *Milestone `json:"milestone"`
	Head Branch `json:"head"`
	Base Branch `json:"base"`

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

type Repository struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	HTMLURL  string `json:"html_url"`
	Owner    User   `json:"owner"`
}

type Team struct {
	ID          int64   `json:"id"`
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	HTMLURL     string  `json:"html_url"`
}

type User struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	HTMLURL   string `json:"html_url"`
	Type      string `json:"type"`
	SiteAdmin bool   `json:"site_admin"`
}
