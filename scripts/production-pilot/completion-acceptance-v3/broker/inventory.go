package main

// Dated captured-evidence schemas only. No repository, filesystem inventory or Git execution.

type inventoryDocument struct {
	SchemaVersion string              `json:"schema_version"`
	Snapshot      inventorySnapshot   `json:"snapshot"`
	Roots         []inventoryRoot     `json:"roots"`
	Worktrees     []inventoryWorktree `json:"worktrees"`
	Totals        inventoryTotals     `json:"totals"`
	Commands      []string            `json:"commands"`
	Limitations   []string            `json:"limitations"`
}

type inventorySnapshot struct {
	TimeUTC  string `json:"time_utc"`
	Host     string `json:"host"`
	Platform string `json:"platform"`
	Boundary string `json:"boundary"`
}

type inventoryRoot struct {
	Index               int    `json:"index"`
	Label               string `json:"label"`
	Path                string `json:"path"`
	Exists              bool   `json:"exists"`
	IsGitRepo           bool   `json:"is_git_repo"`
	Branch              string `json:"branch"`
	HeadSHA             string `json:"head_sha"`
	RemoteOrigin        string `json:"remote_origin"`
	TrackedChangeCount  int64  `json:"tracked_change_count"`
	UntrackedFileCount  int64  `json:"untracked_file_count"`
	Dirty               bool   `json:"dirty"`
	WorktreeCount       int64  `json:"worktree_count"`
	LockedWorktreeCount int64  `json:"locked_worktree_count"`
	PrunableCount       int64  `json:"prunable_worktree_count"`
	DetachedCount       int64  `json:"detached_worktree_count"`
}

type inventoryWorktree struct {
	RootPath       string `json:"root_path"`
	Path           string `json:"path"`
	Branch         string `json:"branch"`
	HeadSHA        string `json:"head_sha"`
	Detached       bool   `json:"detached"`
	Locked         bool   `json:"locked"`
	LockReason     string `json:"lock_reason,omitempty"`
	Prunable       bool   `json:"prunable"`
	PrunableReason string `json:"prunable_reason,omitempty"`
	PathChecked    bool   `json:"path_checked"`
	PathPresent    bool   `json:"path_present"`
}

type inventoryTotals struct {
	RootCount           int64 `json:"root_count"`
	DirtyRootCount      int64 `json:"dirty_root_count"`
	CleanRootCount      int64 `json:"clean_root_count"`
	WorktreeCount       int64 `json:"worktree_record_count"`
	RequiredPathChecks  int64 `json:"required_path_check_count"`
	CompletedPathChecks int64 `json:"completed_path_check_count"`
	PresentPaths        int64 `json:"present_path_count"`
	AbsentPaths         int64 `json:"absent_path_count"`
	LockedWorktrees     int64 `json:"locked_worktree_count"`
	PrunableWorktrees   int64 `json:"prunable_worktree_count"`
	DetachedWorktrees   int64 `json:"detached_worktree_count"`
}

type artifactEvidencePacket struct {
	SchemaVersion          int32                      `json:"schema_version"`
	Contract               string                     `json:"contract"`
	Artifacts              []artifactDigest           `json:"artifacts"`
	Inventory              inventoryEvidenceTotals    `json:"inventory"`
	Roots                  []artifactEvidenceRoot     `json:"roots"`
	Worktrees              []artifactEvidenceWorktree `json:"worktrees"`
	InventoryRecordsSHA256 string                     `json:"inventory_records_sha256"`
}

type artifactDigest struct {
	Path           string `json:"path"`
	DeclaredSHA256 string `json:"declared_sha256"`
	ObservedSHA256 string `json:"observed_sha256"`
}

type inventoryEvidenceTotals struct {
	RootCount               int64 `json:"root_count"`
	DirtyRootCount          int64 `json:"dirty_root_count"`
	CleanRootCount          int64 `json:"clean_root_count"`
	WorktreeCount           int64 `json:"worktree_count"`
	PerWorktreeRecordCount  int64 `json:"per_worktree_record_count"`
	RequiredPathCheckCount  int64 `json:"required_path_check_count"`
	CompletedPathCheckCount int64 `json:"completed_path_check_count"`
	PresentPathCount        int64 `json:"present_path_count"`
	AbsentPathCount         int64 `json:"absent_path_count"`
}

type artifactEvidenceRoot struct {
	Path               string `json:"path"`
	Branch             string `json:"branch"`
	HeadSHA            string `json:"head_sha"`
	TrackedChangeCount int64  `json:"tracked_change_count"`
	UntrackedFileCount int64  `json:"untracked_file_count"`
	Dirty              bool   `json:"dirty"`
}

type artifactEvidenceWorktree struct {
	RootPath    string `json:"root_path"`
	Path        string `json:"path"`
	Branch      string `json:"branch"`
	HeadSHA     string `json:"head_sha"`
	PathChecked bool   `json:"path_checked"`
	PathPresent bool   `json:"path_present"`
}
