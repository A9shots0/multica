package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

const gitExecutable = `C:\Program Files\Git\cmd\git.exe`

var allowedRoots = []rootSpec{
	{Index: 1, Label: "CRM", Path: `C:\Dev\CRM`},
	{Index: 2, Label: "LinkHub", Path: `C:\Dev\linkaggregator`},
	{Index: 3, Label: "Viral Queue", Path: `C:\Dev\telegrambots\viralcontentsharer`},
	{Index: 4, Label: "Canopy frontend", Path: `A:\OneDrive\Desktop\Browser`},
	{Index: 5, Label: "Canopy backend", Path: `A:\OneDrive\Desktop\Notion\twitter`},
	{Index: 6, Label: "Telegram photobot", Path: `A:\OneDrive\Desktop\Notion\telegram\photobot`},
	{Index: 7, Label: "Multica owner checkout", Path: `C:\Dev\SMK-multica-selfhost-pilot`},
	{Index: 8, Label: "Silver Mediums Knowledge", Path: `C:\Dev\Silver-Mediums-Knowledge`},
	{Index: 9, Label: "Silver Mediums Intelligence", Path: `A:\Silver-Mediums-Workspace\repos-future\silver-mediums-intelligence`},
	{Index: 10, Label: "Multica isolated replay worktree", Path: `A:\Silver-Mediums-Workspace\worktrees\multica\integration-preservation-pilot-20260914T232814Z`},
}

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
			Boundary: "Windows local Git metadata and path existence for the ten frozen roots only; no network, production, outbound action, or Phone Farm access.",
		},
		Commands: []string{
			"git -C <root> rev-parse --is-inside-work-tree",
			"git -C <root> symbolic-ref --quiet --short HEAD",
			"git -C <root> rev-parse HEAD",
			"git -C <root> remote get-url origin",
			"git -C <root> status --porcelain",
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
	status, err := runGit(spec.Path, "status", "--porcelain")
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

var inventoryGit = func(root string, args ...string) (string, error) {
	return runGitWithExecutable(gitExecutable, root, args...)
}

func runGit(root string, args ...string) (string, error) { return inventoryGit(root, args...) }

func runGitWithExecutable(executable, root string, args ...string) (string, error) {
	argv := append([]string{"-c", "credential.interactive=never", "-C", root}, args...)
	cmd := exec.Command(executable, argv...)
	cmd.Env = []string{"GIT_TERMINAL_PROMPT=0", "SYSTEMROOT=" + os.Getenv("SYSTEMROOT"), "PATH=" + filepath.Dir(executable)}
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

func writeArtifacts(directory string, prepared inventoryDocument, correction bool) (brokerActionEvidence, error) {
	if directory != authorizedReportDirectory {
		return brokerActionEvidence{}, fmt.Errorf("report directory is outside frozen authority")
	}
	if correction {
		if err := requireExactArtifactDirectory(directory); err != nil {
			return brokerActionEvidence{}, err
		}
		fresh, err := collectInventory(time.Now().UTC())
		if err != nil {
			return brokerActionEvidence{}, err
		}
		prepared = fresh
	} else {
		if _, err := os.Stat(directory); !os.IsNotExist(err) {
			return brokerActionEvidence{}, fmt.Errorf("fresh replay report directory already exists")
		}
		if err := os.Mkdir(directory, 0o700); err != nil {
			return brokerActionEvidence{}, err
		}
		anchor := filepath.Join(directory, ".snapshot-anchor")
		if err := os.WriteFile(anchor, []byte("item14 replay snapshot anchor\n"), 0o600); err != nil {
			return brokerActionEvidence{}, err
		}
		fresh, err := collectInventory(time.Now().UTC())
		if err != nil {
			return brokerActionEvidence{}, err
		}
		prepared = fresh
		if err := os.Remove(anchor); err != nil {
			return brokerActionEvidence{}, err
		}
	}
	// The implementation recollection runs with an anchor so Git observes the
	// same untracked directory cardinality as the final three files. Correction
	// recollects the already-populated directory directly.

	inventoryBytes, err := json.MarshalIndent(prepared, "", "  ")
	if err != nil {
		return brokerActionEvidence{}, err
	}
	inventoryBytes = append(inventoryBytes, '\n')
	ownerBytes := renderOwnerReport(prepared)
	inventoryHash := hashBytes(inventoryBytes)
	ownerHash := hashBytes(ownerBytes)
	evidenceBytes := renderEvidenceReport(prepared, inventoryHash, ownerHash)
	files := map[string][]byte{
		"inventory.json": inventoryBytes,
		"WINDOWS-REPOSITORY-WORKTREE-INVENTORY.md": ownerBytes,
		"EVIDENCE.md": evidenceBytes,
	}
	for _, name := range []string{"inventory.json", "WINDOWS-REPOSITORY-WORKTREE-INVENTORY.md", "EVIDENCE.md"} {
		if err := writeFileAtomically(filepath.Join(directory, name), files[name]); err != nil {
			return brokerActionEvidence{}, err
		}
	}
	if err := requireExactArtifactDirectory(directory); err != nil {
		return brokerActionEvidence{}, err
	}
	packet, err := buildArtifactEvidence(directory, prepared, files)
	if err != nil {
		return brokerActionEvidence{}, err
	}
	reason := fmt.Sprintf("verified %d roots and %d/%d worktree path checks; inventory records sha256 %s",
		prepared.Totals.RootCount, prepared.Totals.CompletedPathChecks, prepared.Totals.RequiredPathChecks, packet.InventoryRecordsSHA256)
	return brokerActionEvidence{Reason: reason, Packet: packet}, nil
}

func validateInventoryTotals(doc inventoryDocument) error {
	var totals inventoryTotals
	totals.RootCount = int64(len(doc.Roots))
	totals.WorktreeCount = int64(len(doc.Worktrees))
	totals.RequiredPathChecks = totals.WorktreeCount
	for _, root := range doc.Roots {
		if root.Dirty != (root.TrackedChangeCount+root.UntrackedFileCount > 0) {
			return fmt.Errorf("root dirty total is inconsistent")
		}
		if root.Dirty {
			totals.DirtyRootCount++
		}
		totals.LockedWorktrees += root.LockedWorktreeCount
		totals.PrunableWorktrees += root.PrunableCount
		totals.DetachedWorktrees += root.DetachedCount
	}
	totals.CleanRootCount = totals.RootCount - totals.DirtyRootCount
	rootWorktrees := map[string]int64{}
	for _, worktree := range doc.Worktrees {
		rootWorktrees[worktree.RootPath]++
		if !worktree.PathChecked {
			return fmt.Errorf("worktree path check is incomplete")
		}
		totals.CompletedPathChecks++
		if worktree.PathPresent {
			totals.PresentPaths++
		} else {
			totals.AbsentPaths++
		}
	}
	for _, root := range doc.Roots {
		if root.WorktreeCount != rootWorktrees[root.Path] {
			return fmt.Errorf("root worktree total is inconsistent")
		}
	}
	if totals != doc.Totals {
		return fmt.Errorf("inventory totals are not canonically derived")
	}
	return nil
}

func renderOwnerReport(doc inventoryDocument) []byte {
	var b strings.Builder
	b.WriteString("# Windows repository and worktree inventory\n\n")
	fmt.Fprintf(&b, "Snapshot: `%s` on `%s`  \n", doc.Snapshot.TimeUTC, doc.Snapshot.Host)
	fmt.Fprintf(&b, "Coverage: **%d/%d roots** and **%d/%d worktree path checks**.\n\n", len(doc.Roots), len(allowedRoots), doc.Totals.CompletedPathChecks, doc.Totals.RequiredPathChecks)
	b.WriteString("| # | Root | Branch | Tracked | Untracked | Worktree records |\n|---:|---|---|---:|---:|---:|\n")
	for _, root := range doc.Roots {
		fmt.Fprintf(&b, "| %d | `%s` | `%s` | %d | %d | %d |\n", root.Index, root.Path, root.Branch, root.TrackedChangeCount, root.UntrackedFileCount, root.WorktreeCount)
	}
	fmt.Fprintf(&b, "\nTotals: %d dirty and %d clean roots; %d worktree records; %d present and %d absent paths; %d locked, %d prunable, and %d detached records.\n\n",
		doc.Totals.DirtyRootCount, doc.Totals.CleanRootCount, doc.Totals.WorktreeCount, doc.Totals.PresentPaths, doc.Totals.AbsentPaths, doc.Totals.LockedWorktrees, doc.Totals.PrunableWorktrees, doc.Totals.DetachedWorktrees)
	b.WriteString("This is local Windows metadata evidence, not cleanup authority or proof of remote, deployed, Mac, Phone Farm runtime, or business state. No Phone Farm surface was accessed in this replay.\n")
	return []byte(b.String())
}

func renderEvidenceReport(doc inventoryDocument, inventoryHash, ownerHash string) []byte {
	var b strings.Builder
	b.WriteString("# Evidence - Item 14 bounded integration replay\n\n")
	fmt.Fprintf(&b, "Snapshot: `%s`  \nHost: `%s`  \nBoundary: %s\n\n", doc.Snapshot.TimeUTC, doc.Snapshot.Host, doc.Snapshot.Boundary)
	b.WriteString("## Artifact hashes\n\n")
	fmt.Fprintf(&b, "- `inventory.json`: `%s`\n- `WINDOWS-REPOSITORY-WORKTREE-INVENTORY.md`: `%s`\n", inventoryHash, ownerHash)
	b.WriteString("- `EVIDENCE.md`: recorded in the controller-accepted workflow event because a file cannot contain its own stable digest.\n\n")
	b.WriteString("## Exact read-only commands\n\n```text\n")
	for _, command := range doc.Commands {
		b.WriteString(command + "\n")
	}
	b.WriteString("```\n\n")
	fmt.Fprintf(&b, "All %d roots existed and were valid Git worktrees. All %d emitted worktree records received an explicit filesystem path check: %d present and %d absent. Important rates: root coverage %d/%d; path-check coverage %d/%d.\n\n",
		len(doc.Roots), len(doc.Worktrees), doc.Totals.PresentPaths, doc.Totals.AbsentPaths, len(doc.Roots), len(allowedRoots), doc.Totals.CompletedPathChecks, doc.Totals.RequiredPathChecks)
	b.WriteString("## Confidence and limitations\n\n")
	for _, limitation := range doc.Limitations {
		b.WriteString("- " + limitation + "\n")
	}
	b.WriteString("\nPrioritized safe next action: preserve this evidence and require owner approval before any worktree move, prune, repair, deletion, or migration.\n")
	return []byte(b.String())
}

func buildArtifactEvidence(directory string, doc inventoryDocument, files map[string][]byte) (artifactEvidencePacket, error) {
	packet := artifactEvidencePacket{
		SchemaVersion: 1, Contract: "repository_worktree_inventory.v1",
		Inventory: inventoryEvidenceTotals{
			RootCount: doc.Totals.RootCount, DirtyRootCount: doc.Totals.DirtyRootCount, CleanRootCount: doc.Totals.CleanRootCount,
			WorktreeCount: doc.Totals.WorktreeCount, PerWorktreeRecordCount: doc.Totals.WorktreeCount,
			RequiredPathCheckCount: doc.Totals.RequiredPathChecks, CompletedPathCheckCount: doc.Totals.CompletedPathChecks,
			PresentPathCount: doc.Totals.PresentPaths, AbsentPathCount: doc.Totals.AbsentPaths,
		},
	}
	for _, name := range []string{"inventory.json", "WINDOWS-REPOSITORY-WORKTREE-INVENTORY.md", "EVIDENCE.md"} {
		digest := hashBytes(files[name])
		packet.Artifacts = append(packet.Artifacts, artifactDigest{Path: filepath.Join(directory, name), DeclaredSHA256: digest, ObservedSHA256: digest})
	}
	for _, root := range doc.Roots {
		packet.Roots = append(packet.Roots, artifactEvidenceRoot{
			Path: root.Path, Branch: root.Branch, HeadSHA: root.HeadSHA,
			TrackedChangeCount: root.TrackedChangeCount, UntrackedFileCount: root.UntrackedFileCount, Dirty: root.Dirty,
		})
	}
	for _, worktree := range doc.Worktrees {
		packet.Worktrees = append(packet.Worktrees, artifactEvidenceWorktree{
			RootPath: worktree.RootPath, Path: worktree.Path, Branch: worktree.Branch, HeadSHA: worktree.HeadSHA,
			PathChecked: worktree.PathChecked, PathPresent: worktree.PathPresent,
		})
	}
	digest, err := inventoryRecordsDigest(packet.Roots, packet.Worktrees)
	if err != nil {
		return artifactEvidencePacket{}, err
	}
	packet.InventoryRecordsSHA256 = digest
	return packet, nil
}

func inventoryRecordsDigest(roots []artifactEvidenceRoot, worktrees []artifactEvidenceWorktree) (string, error) {
	raw, err := json.Marshal(struct {
		Roots     []artifactEvidenceRoot     `json:"roots"`
		Worktrees []artifactEvidenceWorktree `json:"worktrees"`
	}{Roots: roots, Worktrees: worktrees})
	if err != nil {
		return "", err
	}
	return hashBytes(raw), nil
}

func verifyArtifacts(directory string) (verificationResult, error) {
	if err := requireExactArtifactDirectory(directory); err != nil {
		return verificationResult{}, err
	}
	inventoryRaw, err := os.ReadFile(filepath.Join(directory, "inventory.json"))
	if err != nil {
		return verificationResult{}, err
	}
	var observed inventoryDocument
	if err := json.Unmarshal(inventoryRaw, &observed); err != nil {
		return verificationResult{}, err
	}
	if observed.SchemaVersion != "2.0.0" || len(observed.Roots) != len(allowedRoots) || len(observed.Worktrees) == 0 {
		return verificationResult{}, fmt.Errorf("inventory schema or coverage is incomplete")
	}
	if err := validateInventoryTotals(observed); err != nil {
		return verificationResult{}, err
	}
	canonicalInventory, err := json.MarshalIndent(observed, "", "  ")
	if err != nil {
		return verificationResult{}, err
	}
	canonicalInventory = append(canonicalInventory, '\n')
	if !bytes.Equal(inventoryRaw, canonicalInventory) {
		return verificationResult{}, fmt.Errorf("inventory JSON is not canonical")
	}
	for i, spec := range allowedRoots {
		if observed.Roots[i].Index != spec.Index || observed.Roots[i].Path != spec.Path || !observed.Roots[i].Exists || !observed.Roots[i].IsGitRepo {
			return verificationResult{}, fmt.Errorf("root %d does not match the frozen allowlist", spec.Index)
		}
	}
	current, err := collectInventory(time.Now().UTC())
	if err != nil {
		return verificationResult{}, err
	}
	left, _ := json.Marshal(struct {
		Roots     []inventoryRoot     `json:"roots"`
		Worktrees []inventoryWorktree `json:"worktrees"`
	}{observed.Roots, observed.Worktrees})
	right, _ := json.Marshal(struct {
		Roots     []inventoryRoot     `json:"roots"`
		Worktrees []inventoryWorktree `json:"worktrees"`
	}{current.Roots, current.Worktrees})
	if !bytes.Equal(left, right) {
		return verificationResult{Pass: false, Summary: "fresh independent Git/path recomputation differs from the recorded snapshot"}, nil
	}
	files := map[string][]byte{"inventory.json": inventoryRaw}
	for _, name := range []string{"WINDOWS-REPOSITORY-WORKTREE-INVENTORY.md", "EVIDENCE.md"} {
		files[name], err = os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			return verificationResult{}, err
		}
	}
	if !bytes.Equal(files["WINDOWS-REPOSITORY-WORKTREE-INVENTORY.md"], renderOwnerReport(observed)) {
		return verificationResult{}, fmt.Errorf("owner report does not match canonical inventory rendering")
	}
	expectedEvidence := renderEvidenceReport(observed, hashBytes(inventoryRaw), hashBytes(files["WINDOWS-REPOSITORY-WORKTREE-INVENTORY.md"]))
	if !bytes.Equal(files["EVIDENCE.md"], expectedEvidence) {
		return verificationResult{}, fmt.Errorf("evidence report does not match canonical inventory rendering")
	}
	packet, err := buildArtifactEvidence(directory, observed, files)
	if err != nil {
		return verificationResult{}, err
	}
	if !bytes.Contains(files["EVIDENCE.md"], []byte(hashBytes(files["inventory.json"]))) ||
		!bytes.Contains(files["EVIDENCE.md"], []byte(hashBytes(files["WINDOWS-REPOSITORY-WORKTREE-INVENTORY.md"]))) {
		return verificationResult{}, fmt.Errorf("evidence report does not bind artifact hashes")
	}
	summary := fmt.Sprintf("PASS: %d/%d roots, %d/%d path checks, three exact artifacts, records sha256 %s",
		observed.Totals.RootCount, len(allowedRoots), observed.Totals.CompletedPathChecks, observed.Totals.RequiredPathChecks, packet.InventoryRecordsSHA256)
	return verificationResult{Pass: true, Summary: summary}, nil
}

func requireExactArtifactDirectory(directory string) error {
	if directory != authorizedReportDirectory {
		return fmt.Errorf("artifact directory is outside frozen authority")
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	want := map[string]bool{"inventory.json": true, "WINDOWS-REPOSITORY-WORKTREE-INVENTORY.md": true, "EVIDENCE.md": true}
	for _, entry := range entries {
		if entry.IsDir() || !want[entry.Name()] {
			return fmt.Errorf("unexpected artifact directory entry")
		}
		delete(want, entry.Name())
	}
	if len(want) != 0 {
		return fmt.Errorf("artifact directory does not contain exactly three files")
	}
	return nil
}

func writeFileAtomically(path string, content []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".item14-write-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tempPath)
		}
	}()
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(content); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	ok = true
	return nil
}

func hashBytes(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func sortedArtifactNames(files map[string][]byte) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
