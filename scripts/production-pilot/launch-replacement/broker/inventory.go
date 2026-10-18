package main

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const gitExecutable = `C:\Program Files\Git\cmd\git.exe`

var allowedRoots = []rootSpec{{Index: 1, Label: "CRM", Path: `C:\Dev\CRM`}}

var gitObjectPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

type rootSpec struct {
	Index int
	Label string
	Path  string
}

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

type brokerActionEvidence struct {
	Reason string
	Packet artifactEvidencePacket
}

type verificationResult struct {
	Pass    bool
	Summary string
}

func collectInventory(snapshot time.Time) (inventoryDocument, error) {
	host, _ := os.Hostname()
	doc := inventoryDocument{
		SchemaVersion: "2.0.0",
		Snapshot: inventorySnapshot{
			TimeUTC: snapshot.UTC().Format(time.RFC3339Nano), Host: host,
			Platform: runtime.GOOS + "/" + runtime.GOARCH,
			Boundary: "CRM local Git metadata and its linked worktree path existence only; no remote contact or CRM application access.",
		},
		Commands: []string{
			"git -C <root> rev-parse --is-inside-work-tree",
			"git -C <root> symbolic-ref --quiet --short HEAD",
			"git -C <root> rev-parse HEAD",
			"git -C <root> remote get-url origin",
			"git -C <root> status --porcelain --untracked-files=all",
			"git -C <root> worktree list --porcelain",
			"filesystem existence check for every emitted worktree path",
		},
		Limitations: []string{
			"No remotes were contacted; remote branch tips and hosted state are unknown.",
			"Phone Farm was not accessed. The Mac-only authority boundary is carried only from the frozen owner packet.",
			"Dirty, detached, locked, prunable, missing, and unreadable records are evidence, never cleanup authority.",
		},
	}
	for _, spec := range allowedRoots {
		root, worktrees, err := inspectRoot(spec)
		if err != nil {
			return inventoryDocument{}, fmt.Errorf("inspect frozen root %d: %w", spec.Index, err)
		}
		doc.Roots = append(doc.Roots, root)
		doc.Worktrees = append(doc.Worktrees, worktrees...)
		if root.Dirty {
			doc.Totals.DirtyRootCount++
		}
		doc.Totals.LockedWorktrees += root.LockedWorktreeCount
		doc.Totals.PrunableWorktrees += root.PrunableCount
		doc.Totals.DetachedWorktrees += root.DetachedCount
	}
	doc.Totals.RootCount = int64(len(doc.Roots))
	doc.Totals.CleanRootCount = doc.Totals.RootCount - doc.Totals.DirtyRootCount
	doc.Totals.WorktreeCount = int64(len(doc.Worktrees))
	doc.Totals.RequiredPathChecks = doc.Totals.WorktreeCount
	doc.Totals.CompletedPathChecks = doc.Totals.WorktreeCount
	for _, worktree := range doc.Worktrees {
		if !worktree.PathChecked {
			return inventoryDocument{}, fmt.Errorf("worktree path check is incomplete")
		}
		if worktree.PathPresent {
			doc.Totals.PresentPaths++
		} else {
			doc.Totals.AbsentPaths++
		}
	}
	if len(doc.Roots) != len(allowedRoots) || len(doc.Worktrees) == 0 {
		return inventoryDocument{}, fmt.Errorf("frozen root/worktree coverage is incomplete")
	}
	return doc, nil
}

func inspectRoot(spec rootSpec) (inventoryRoot, []inventoryWorktree, error) {
	root := inventoryRoot{Index: spec.Index, Label: spec.Label, Path: spec.Path}
	info, err := os.Stat(spec.Path)
	if err != nil || !info.IsDir() {
		return root, nil, fmt.Errorf("root is absent or not a directory")
	}
	root.Exists = true
	inside, err := runGit(spec.Path, "rev-parse", "--is-inside-work-tree")
	if err != nil || strings.TrimSpace(inside) != "true" {
		return root, nil, fmt.Errorf("root is not a Git worktree")
	}
	root.IsGitRepo = true
	branch, branchErr := runGit(spec.Path, "symbolic-ref", "--quiet", "--short", "HEAD")
	if branchErr != nil {
		branch = "(detached)"
	}
	root.Branch = strings.TrimSpace(branch)
	head, err := runGit(spec.Path, "rev-parse", "HEAD")
	if err != nil || !gitObjectPattern.MatchString(strings.TrimSpace(head)) {
		return root, nil, fmt.Errorf("HEAD is unavailable or invalid")
	}
	root.HeadSHA = strings.TrimSpace(head)
	remote, remoteErr := runGit(spec.Path, "remote", "get-url", "origin")
	if remoteErr == nil {
		root.RemoteOrigin = sanitizeRemote(strings.TrimSpace(remote))
	}
	status, err := runGit(spec.Path, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return root, nil, fmt.Errorf("read porcelain status: %w", err)
	}
	for _, line := range splitNonEmptyLines(status) {
		if strings.HasPrefix(line, "??") {
			root.UntrackedFileCount++
		} else {
			root.TrackedChangeCount++
		}
	}
	root.Dirty = root.TrackedChangeCount+root.UntrackedFileCount > 0
	porcelain, err := runGit(spec.Path, "worktree", "list", "--porcelain")
	if err != nil {
		return root, nil, fmt.Errorf("read worktree porcelain: %w", err)
	}
	worktrees, err := parseWorktrees(spec.Path, porcelain)
	if err != nil || len(worktrees) == 0 {
		return root, nil, fmt.Errorf("parse worktree porcelain: %w", err)
	}
	for i := range worktrees {
		if err := permittedWorktreePath(worktrees[i].Path); err != nil {
			return root, nil, err
		}
		worktrees[i].PathChecked = true
		if info, statErr := os.Stat(worktrees[i].Path); statErr == nil && info.IsDir() {
			worktrees[i].PathPresent = true
		}
		if worktrees[i].Locked {
			root.LockedWorktreeCount++
		}
		if worktrees[i].Prunable {
			root.PrunableCount++
		}
		if worktrees[i].Detached {
			root.DetachedCount++
		}
	}
	root.WorktreeCount = int64(len(worktrees))
	return root, worktrees, nil
}

func runGit(root string, args ...string) (string, error) {
	argv := append([]string{"-c", "credential.interactive=never", "-C", root}, args...)
	cmd := exec.Command(gitExecutable, argv...)
	cmd.Env = []string{"GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "SYSTEMROOT=" + os.Getenv("SYSTEMROOT"), "PATH=" + filepath.Dir(gitExecutable)}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	raw, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s failed", strings.Join(args, " "))
	}
	return strings.ReplaceAll(string(raw), "\r\n", "\n"), nil
}

func parseWorktrees(rootPath, raw string) ([]inventoryWorktree, error) {
	var result []inventoryWorktree
	var current *inventoryWorktree
	flush := func() error {
		if current == nil {
			return nil
		}
		if current.Path == "" || !gitObjectPattern.MatchString(current.HeadSHA) {
			return fmt.Errorf("incomplete worktree record")
		}
		if current.Branch == "" {
			current.Branch = "(detached)"
			current.Detached = true
		}
		result = append(result, *current)
		current = nil
		return nil
	}
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		if line == "" {
			if err := flush(); err != nil {
				return nil, err
			}
			continue
		}
		key, value, _ := strings.Cut(line, " ")
		switch key {
		case "worktree":
			if current != nil {
				if err := flush(); err != nil {
					return nil, err
				}
			}
			current = &inventoryWorktree{RootPath: rootPath, Path: value}
		case "HEAD":
			if current != nil {
				current.HeadSHA = value
			}
		case "branch":
			if current != nil {
				current.Branch = strings.TrimPrefix(value, "refs/heads/")
			}
		case "detached":
			if current != nil {
				current.Detached = true
				current.Branch = "(detached)"
			}
		case "locked":
			if current != nil {
				current.Locked = true
				current.LockReason = value
			}
		case "prunable":
			if current != nil {
				current.Prunable = true
				current.PrunableReason = value
			}
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return result, nil
}

func sanitizeRemote(raw string) string {
	if parsed, err := url.Parse(raw); err == nil && parsed.Scheme != "" {
		parsed.User = nil
		return parsed.String()
	}
	if at := strings.LastIndex(raw, "@"); at >= 0 && strings.Contains(raw[at+1:], ":") {
		return raw[at+1:]
	}
	return raw
}

func splitNonEmptyLines(raw string) []string {
	var result []string
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}
