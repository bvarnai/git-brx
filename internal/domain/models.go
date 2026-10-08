package domain

// BranchType defines recognized topic and mainline branch categories.
type BranchType string

const (
	BranchTypeIssue   BranchType = "issue"
	BranchTypeFeature BranchType = "feature"
	BranchTypeEpic    BranchType = "epic"
	BranchTypeRelease BranchType = "release"
	BranchTypeMaster  BranchType = "master"
	BranchTypeCustom  BranchType = "custom"
)

// BranchRecord encapsulates details of an inspected Git branch ref.
type BranchRecord struct {
	Name        string     `json:"name"`
	Type        BranchType `json:"type"`
	IssueKey    string     `json:"issue_key,omitempty"`
	UpstreamRef string     `json:"upstream_ref,omitempty"`
	CommitHash  string     `json:"commit_hash,omitempty"`
	IsDetached  bool       `json:"is_detached"`
}

// BranchDelta captures ahead/behind commit divergence against upstream.
type BranchDelta struct {
	LocalBranch  string `json:"local_branch"`
	RemoteBranch string `json:"remote_branch"`
	AheadCount   int    `json:"ahead_count"`
	BehindCount  int    `json:"behind_count"`
}

// IsInSync returns true if the branch has neither unpushed nor unmerged commits.
func (d *BranchDelta) IsInSync() bool {
	return d.AheadCount == 0 && d.BehindCount == 0
}

// RepositoryState represents the topology and active transient operations of a repository.
type RepositoryState struct {
	RootPath      string        `json:"root_path"`
	CurrentBranch *BranchRecord `json:"current_branch,omitempty"`
	IsShallow     bool          `json:"is_shallow"`
	IsBare        bool          `json:"is_bare"`
	HasOrigin     bool          `json:"has_origin"`
	OriginURL     string        `json:"origin_url,omitempty"`
	RebaseActive  bool          `json:"rebase_active"`
	MergeActive   bool          `json:"merge_active"`
	WorktreeClean bool          `json:"worktree_clean"`
}

// JiraIssue contains issue metadata fetched from the JIRA REST API.
type JiraIssue struct {
	Key                 string   `json:"key"`
	Summary             string   `json:"summary"`
	IssueType           string   `json:"issue_type"`
	Status              string   `json:"status"`
	AssigneeName        string   `json:"assignee_name,omitempty"`
	AssigneeDisplayName string   `json:"assignee_display_name,omitempty"`
	Components          []string `json:"components,omitempty"`
}

// PullRequest models pull request payload and response from Bitbucket Server.
type PullRequest struct {
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	SourceBranch string   `json:"source_branch"`
	TargetBranch string   `json:"target_branch"`
	ProjectKey   string   `json:"project_key"`
	RepoSlug     string   `json:"repo_slug"`
	Reviewers    []string `json:"reviewers"`
	State        string   `json:"state"`
	URL          string   `json:"url,omitempty"`
}
