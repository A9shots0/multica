package agentqualification

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
)

type RealisticCaseWorkspace struct {
	CaseID             string
	CaseVersion        string
	Root               string
	InitialCopy        string
	Fixture            RealisticFixture
	ManifestSHA256     string
	StartingTreeSHA256 string
}

//go:embed testdata/realistic
var realisticFixtureFS embed.FS

// PrepareRealisticCaseWorkspace is offline. It validates the sealed fixture,
// copies it twice, and returns a candidate workspace plus an immutable initial
// copy for later diff and evidence verification.
func PrepareRealisticCaseWorkspace(parent, caseID string) (RealisticCaseWorkspace, error) {
	return prepareRealisticCaseWorkspace(parent, caseID, "")
}

// PrepareRealisticCaseWorkspaceForRun binds the immutable R04 checkpoint and
// reference assertion to the harness-generated run family before the candidate
// sees the workspace. The candidate cannot edit any identity-bearing file.
func PrepareRealisticCaseWorkspaceForRun(parent, caseID, runFamilyID string) (RealisticCaseWorkspace, error) {
	if strings.TrimSpace(runFamilyID) == "" {
		return RealisticCaseWorkspace{}, fmt.Errorf("realistic run family is required")
	}
	return prepareRealisticCaseWorkspace(parent, caseID, runFamilyID)
}

func prepareRealisticCaseWorkspace(parent, caseID, runFamilyID string) (RealisticCaseWorkspace, error) {
	var prepared RealisticCaseWorkspace
	if !filepath.IsAbs(parent) {
		return prepared, fmt.Errorf("realistic workspace parent must be absolute")
	}
	group, err := CurrentRealisticWorkflowGroup()
	if err != nil {
		return prepared, err
	}
	manifest, err := CurrentRealisticFixtureManifest()
	if err != nil {
		return prepared, err
	}
	var fixture RealisticFixture
	for _, candidate := range manifest.Fixtures {
		if candidate.CaseID == caseID {
			fixture = candidate
			break
		}
	}
	if fixture.CaseID == "" {
		return prepared, fmt.Errorf("unknown realistic fixture %s", caseID)
	}
	caseKnown := false
	caseVersion := ""
	for _, candidateCase := range group.Cases {
		if candidateCase.CaseID == caseID && candidateCase.FixtureID == fixture.FixtureID {
			caseKnown = true
			caseVersion = candidateCase.CaseVersion
		}
	}
	if !caseKnown {
		return prepared, fmt.Errorf("fixture is not bound to realistic group")
	}
	source := filepath.ToSlash(filepath.Join("testdata", "realistic", strings.ToLower(caseID)))
	actualTree, err := RealisticFixtureTreeSHA256(realisticFixtureFS, source)
	if err != nil || actualTree != fixture.TreeSHA256 {
		return prepared, fmt.Errorf("fixture %s tree mismatch", caseID)
	}
	root, err := os.MkdirTemp(parent, "realistic-"+strings.ToLower(caseID)+"-")
	if err != nil {
		return prepared, err
	}
	initial := root + "-initial"
	if err := copyRealisticTree(realisticFixtureFS, source, root); err != nil {
		return prepared, err
	}
	if err := copyRealisticTree(realisticFixtureFS, source, initial); err != nil {
		return prepared, err
	}
	startingTree := actualTree
	if caseID == "R04" && runFamilyID != "" {
		for _, destination := range []string{root, initial} {
			for _, relative := range []string{"state/checkpoint.json", "raw/first-run.json", "tests/test_resume.py"} {
				path := filepath.Join(destination, filepath.FromSlash(relative))
				raw, readErr := os.ReadFile(path)
				if readErr != nil {
					return prepared, readErr
				}
				bound := bytes.ReplaceAll(raw, []byte("resume-fixture-17"), []byte(runFamilyID))
				if bytes.Equal(bound, raw) {
					return prepared, fmt.Errorf("R04 identity placeholder missing from %s", relative)
				}
				if writeErr := os.WriteFile(path, bound, 0o600); writeErr != nil {
					return prepared, writeErr
				}
			}
		}
		startingTree, err = RealisticFixtureTreeSHA256(os.DirFS(root), ".")
		if err != nil {
			return prepared, err
		}
	}
	manifestHash := sha256.Sum256(realisticFixtureManifestV1JSON)
	return RealisticCaseWorkspace{CaseID: caseID, CaseVersion: caseVersion, Root: root, InitialCopy: initial, Fixture: fixture, ManifestSHA256: hex.EncodeToString(manifestHash[:]), StartingTreeSHA256: startingTree}, nil
}

func copyRealisticTree(sourceFS fs.FS, source, destination string) error {
	return fs.WalkDir(sourceFS, source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == "__pycache__" {
			return fs.SkipDir
		}
		if entry.IsDir() {
			return os.MkdirAll(filepath.Join(destination, mustRelative(source, path)), 0o700)
		}
		if filepath.Ext(path) == ".pyc" {
			return nil
		}
		raw, err := fs.ReadFile(sourceFS, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, filepath.FromSlash(mustRelative(source, path)))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o600)
	})
}

func mustRelative(root, path string) string {
	relative, _ := filepath.Rel(root, path)
	if relative == "." {
		return ""
	}
	return relative
}

func (workspace RealisticCaseWorkspace) Read(relative string) ([]byte, error) {
	path, err := workspace.resolveAllowedRead(relative)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func (workspace RealisticCaseWorkspace) Paths() ([]string, error) {
	paths := []string{}
	err := filepath.WalkDir(workspace.Root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) == ".pyc" {
			return nil
		}
		relative, err := filepath.Rel(workspace.Root, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(relative))
		return nil
	})
	sort.Strings(paths)
	return paths, err
}

func (workspace RealisticCaseWorkspace) Search(query string) ([]string, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("search query is required")
	}
	matches := []string{}
	err := filepath.WalkDir(workspace.Root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) == ".pyc" {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Contains(bytes.ToLower(raw), bytes.ToLower([]byte(query))) {
			relative, _ := filepath.Rel(workspace.Root, path)
			matches = append(matches, filepath.ToSlash(relative))
		}
		return nil
	})
	return matches, err
}

func (workspace RealisticCaseWorkspace) Edit(relative string, contents []byte) error {
	if !contains(workspace.Fixture.EditablePaths, filepath.ToSlash(relative)) {
		return fmt.Errorf("edit path is not allowlisted")
	}
	path, err := workspace.resolve(relative)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, contents, 0o600)
}

func (workspace RealisticCaseWorkspace) RunCheck(check string) ([]byte, error) {
	return workspace.runCheckWithActionID(check, "")
}

func (workspace RealisticCaseWorkspace) runCheckWithActionID(check, actionID string) ([]byte, error) {
	if !contains(workspace.Fixture.AllowlistedChecks, check) {
		return nil, fmt.Errorf("check is not allowlisted")
	}
	var name string
	var args []string
	switch check {
	case "python -m unittest discover -s tests -v":
		name, args = "python", []string{"-m", "unittest", "discover", "-s", "tests", "-v"}
	case "node tests/report.test.mjs":
		name, args = "node", []string{filepath.Join("tests", "report.test.mjs")}
	case "python resume.py":
		return runRealisticResume(workspace.Root, actionID, false)
	case "local-static-preview report":
		return runRealisticStaticPreview(workspace.Root)
	case "local-release-simulator rollback":
		return runRealisticReleaseSimulator(workspace.Root, false)
	case "local-release-simulator verify":
		return runRealisticReleaseSimulator(workspace.Root, true)
	default:
		return nil, fmt.Errorf("allowlisted check requires a dedicated adapter: %s", check)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = workspace.Root
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "SYSTEMROOT=" + os.Getenv("SYSTEMROOT"), "PYTHONDONTWRITEBYTECODE=1"}
	return command.CombinedOutput()
}

type realisticResumeJournal struct {
	ActionID    string `json:"action_id"`
	RunFamilyID string `json:"run_family_id"`
	Status      string `json:"status"`
}

func runRealisticResume(workspaceRoot, actionID string, failAfterOutput bool) ([]byte, error) {
	var checkpoint struct {
		RunFamilyID   string   `json:"run_family_id"`
		TerminalState string   `json:"terminal_state"`
		Completed     []string `json:"completed_steps"`
		NextStep      string   `json:"next_step"`
	}
	if err := readRealisticJSON(filepath.Join(workspaceRoot, "state", "checkpoint.json"), &checkpoint); err != nil || checkpoint.RunFamilyID == "" || checkpoint.TerminalState != "interrupted" || checkpoint.NextStep != "verify" || !reflect.DeepEqual(checkpoint.Completed, []string{"collect", "normalize"}) {
		return nil, fmt.Errorf("invalid durable checkpoint")
	}
	if actionID == "" {
		actionID = checkpoint.RunFamilyID + ":resume-continuation:v1"
	}
	journalPath := filepath.Join(workspaceRoot, "state", "resume-action.json")
	journal := realisticResumeJournal{}
	if err := readRealisticJSON(journalPath, &journal); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		var initialCounts map[string]int
		if countErr := readRealisticJSON(filepath.Join(workspaceRoot, "state", "step_counts.json"), &initialCounts); countErr != nil || !reflect.DeepEqual(initialCounts, map[string]int{"collect": 1, "normalize": 1, "verify": 0, "publish_local_output": 0}) {
			return nil, fmt.Errorf("resume starting counts contradict checkpoint")
		}
		journal = realisticResumeJournal{ActionID: actionID, RunFamilyID: checkpoint.RunFamilyID, Status: "prepared"}
		if err := writeRealisticJSONAtomic(journalPath, journal); err != nil {
			return nil, err
		}
	}
	if journal.ActionID != actionID || journal.RunFamilyID != checkpoint.RunFamilyID || !contains([]string{"prepared", "completed"}, journal.Status) {
		return nil, fmt.Errorf("resume action journal identity mismatch")
	}
	targetCounts := map[string]int{"collect": 1, "normalize": 1, "verify": 1, "publish_local_output": 1}
	if journal.Status == "completed" {
		if err := verifyRealisticResumeOutput(workspaceRoot, checkpoint.RunFamilyID, targetCounts); err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{"status": "already_completed", "action_id": actionID, "run_family_id": checkpoint.RunFamilyID})
	}
	if err := writeRealisticJSONAtomic(filepath.Join(workspaceRoot, "state", "step_counts.json"), targetCounts); err != nil {
		return nil, err
	}
	if err := writeRealisticJSONAtomic(filepath.Join(workspaceRoot, "FINAL-OUTPUT.json"), map[string]any{"run_family_id": checkpoint.RunFamilyID, "verified": true}); err != nil {
		return nil, err
	}
	if failAfterOutput {
		return nil, fmt.Errorf("injected crash after resume output")
	}
	journal.Status = "completed"
	if err := writeRealisticJSONAtomic(journalPath, journal); err != nil {
		return nil, err
	}
	if err := verifyRealisticResumeOutput(workspaceRoot, checkpoint.RunFamilyID, targetCounts); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"status": "completed", "action_id": actionID, "run_family_id": checkpoint.RunFamilyID})
}

func verifyRealisticResumeOutput(workspaceRoot, runFamilyID string, wantCounts map[string]int) error {
	var counts map[string]int
	if err := readRealisticJSON(filepath.Join(workspaceRoot, "state", "step_counts.json"), &counts); err != nil || !reflect.DeepEqual(counts, wantCounts) {
		return fmt.Errorf("resume step counts are not exact-once")
	}
	var output struct {
		RunFamilyID string `json:"run_family_id"`
		Verified    bool   `json:"verified"`
	}
	if err := readRealisticJSON(filepath.Join(workspaceRoot, "FINAL-OUTPUT.json"), &output); err != nil || output.RunFamilyID != runFamilyID || !output.Verified {
		return fmt.Errorf("resume final output is invalid")
	}
	return nil
}

func writeRealisticJSONAtomic(path string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return err
	}
	if _, err := temporary.Write(raw); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	committed = true
	return nil
}

func runRealisticStaticPreview(workspaceRoot string) ([]byte, error) {
	reportRoot := filepath.Join(workspaceRoot, "report")
	for _, required := range []string{"index.html", "report.js", "styles.css", "data.json"} {
		info, err := os.Stat(filepath.Join(reportRoot, required))
		if err != nil || info.IsDir() {
			return nil, fmt.Errorf("static preview asset missing: %s", required)
		}
	}
	server := httptest.NewServer(http.FileServer(http.Dir(reportRoot)))
	defer server.Close()
	client := server.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return fmt.Errorf("static preview redirected") }
	response, err := client.Get(server.URL + "/")
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK || !bytes.Contains(body, []byte(`id="total"`)) || !bytes.Contains(body, []byte(`id="cards"`)) {
		return nil, fmt.Errorf("static preview entrypoint is not the report page")
	}
	return json.Marshal(map[string]any{"status": "loopback_preview_ready", "entry_path": "report/index.html", "network": "127.0.0.1_ephemeral"})
}

func runRealisticReleaseSimulator(workspaceRoot string, requireReceipt bool) ([]byte, error) {
	var state struct {
		RequestedOutcome string `json:"requested_outcome"`
		OutcomeStatus    string `json:"outcome_status"`
		Baseline         struct {
			Revision      string `json:"revision"`
			Healthy       bool   `json:"healthy"`
			EmployeePages bool   `json:"employee_pages"`
		} `json:"baseline"`
		Candidate struct {
			Revision      string `json:"revision"`
			Deployed      bool   `json:"deployed"`
			EmployeePages bool   `json:"employee_pages"`
		} `json:"candidate"`
		Rollback struct {
			Revision string `json:"revision"`
			Verified bool   `json:"verified"`
		} `json:"rollback"`
	}
	stateRaw, err := os.ReadFile(filepath.Join(workspaceRoot, "release", "state.json"))
	if err != nil || json.Unmarshal(stateRaw, &state) != nil {
		return nil, fmt.Errorf("release simulator state is invalid")
	}
	if state.RequestedOutcome != "employee_pages_and_new_gallery" || state.OutcomeStatus != "open" || state.Baseline.Revision != "v190" || !state.Baseline.Healthy || !state.Baseline.EmployeePages || state.Candidate.Revision != "v195" || !state.Candidate.Deployed || state.Candidate.EmployeePages || state.Rollback.Revision != state.Baseline.Revision || !state.Rollback.Verified {
		return nil, fmt.Errorf("safe simulated baseline is not restored and verified")
	}
	if requireReceipt {
		var receipt struct {
			Candidate        string `json:"candidate"`
			Rollback         string `json:"rollback"`
			BaselineRestored bool   `json:"baseline_restored"`
			RequestedOutcome string `json:"requested_outcome"`
			EvidenceStage    string `json:"evidence_stage"`
		}
		receiptRaw, readErr := os.ReadFile(filepath.Join(workspaceRoot, "release", "receipt.json"))
		if readErr != nil || json.Unmarshal(receiptRaw, &receipt) != nil || receipt.Candidate != state.Candidate.Revision || receipt.Rollback != state.Rollback.Revision || !receipt.BaselineRestored || receipt.RequestedOutcome != "open" || receipt.EvidenceStage != "local-simulation" {
			return nil, fmt.Errorf("simulated release receipt is missing or contradicts state")
		}
	}
	return json.Marshal(map[string]any{"status": "verified", "baseline_revision": state.Baseline.Revision, "candidate_revision": state.Candidate.Revision, "requested_outcome": state.OutcomeStatus, "external_side_effects": 0})
}

func (workspace RealisticCaseWorkspace) resolveAllowedRead(relative string) (string, error) {
	return workspace.resolve(relative)
}

func (workspace RealisticCaseWorkspace) resolve(relative string) (string, error) {
	if !safeFixtureRelativePath(relative) {
		return "", fmt.Errorf("unsafe workspace path")
	}
	root, err := filepath.Abs(workspace.Root)
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		return "", err
	}
	if target != root && !strings.HasPrefix(strings.ToLower(target), strings.ToLower(root)+string(os.PathSeparator)) {
		return "", fmt.Errorf("workspace path escapes root")
	}
	return target, nil
}
