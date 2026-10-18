// Command implementation-lane-bridge lets a Multica issue start the Silver
// Mediums implementation lane.
//
// The local daemon runs it in place of the Claude CLI (MULTICA_CLAUDE_PATH) for
// the ordinary squad turns of a lane workflow. It speaks just enough of the
// Claude stream-json protocol for the daemon to record a terminal result, and
// does the real work through the lane and the Multica API with the task-scoped
// token the daemon issued for this one task:
//
//   - leader turn: delegate the lane task to the implementer once, so the
//     implementer's task belongs to the squad workflow.
//   - implementer turn: read the lane task spec from the issue, run the lane
//     (isolated worktree, executor, harness checks, Codex reviewer, at most one
//     correction, teardown), record the evidence, publish the outcome as a
//     comment and emit implementation_ready. A failed lane run fails the task,
//     so the failure shows on every surface.
//
// The reviewer turn is not handled here. The controller routes it as a bounded,
// tool-free workflow task, which runs under a workflow-broker grant through
// cmd/implementation-lane-review-broker.
//
// Eligibility is policy, not standing authority: each run still gets the
// lane's fresh, expiring, revoked-at-cleanup authority record and the daemon's
// task-scoped token.
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/brokerreply"
	"github.com/multica-ai/multica/server/pkg/laneevidence"
)

// Reported to the daemon's version probe; the daemon treats this as a Claude CLI.
const bridgeVersion = "2.1.272"

var (
	issueInPrompt = regexp.MustCompile(`(?i)issue id is:?\s*([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})`)
	specBlock     = regexp.MustCompile("(?s)```lane-task\\s*\\n(.*?)\\n```")
)

type config struct {
	Python    string   // python executable that runs the lane
	RunTask   string   // scripts/implementation-lane/run_task.py
	StateDir  string   // bridge records, one per issue
	Repos     []string // repositories a lane task may target
	ServerURL string
	Token     string
	AgentID   string
	TaskID    string
}

type laneSpec struct {
	TaskID              string          `json:"task_id"`
	Title               string          `json:"title"`
	Repository          string          `json:"repository"`
	AllowedPaths        []string        `json:"allowed_paths"`
	VerificationCommand []string        `json:"verification_command"`
	Objective           string          `json:"objective"`
	Raw                 json.RawMessage `json:"-"`
}

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println(bridgeVersion)
		return
	}
	session := uuid.NewString()
	message, err := run()
	if err != nil {
		emit(session, "implementation lane bridge: "+err.Error(), true)
		fmt.Fprintln(os.Stderr, "implementation lane bridge:", err)
		os.Exit(1)
	}
	emit(session, message, false)
}

// emit writes the minimal Claude stream-json frames the daemon needs to record
// a terminal result: an assistant message and a result with the same text.
func emit(session, text string, isError bool) {
	enc := json.NewEncoder(os.Stdout)
	_ = enc.Encode(map[string]any{"type": "system", "subtype": "init", "session_id": session})
	_ = enc.Encode(map[string]any{"type": "assistant", "session_id": session, "message": map[string]any{
		"role": "assistant", "content": []map[string]any{{"type": "text", "text": text}}}})
	subtype := "success"
	if isError {
		subtype = "error_during_execution"
	}
	_ = enc.Encode(map[string]any{"type": "result", "subtype": subtype, "is_error": isError,
		"result": text, "session_id": session, "num_turns": 1})
}

func run() (string, error) {
	prompt, control, err := readPrompt(os.Stdin)
	if err != nil {
		return "", err
	}
	if control != "" {
		// The daemon's model listing probe; the bridge has no models to list.
		return "", fmt.Errorf("unsupported control request %s", control)
	}
	cfg, err := loadConfig()
	if err != nil {
		return "", err
	}
	match := issueInPrompt.FindStringSubmatch(prompt)
	if match == nil {
		return "", errors.New("the task prompt names no issue")
	}
	issueID := match[1]
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Minute)
	defer cancel()

	var plan workflowPlan
	if err := cfg.api(ctx, http.MethodGet, "/api/issues/"+issueID+"/workflow-plan", nil, &plan); err != nil {
		return "", fmt.Errorf("read workflow plan: %w", err)
	}
	switch {
	case cfg.AgentID == plan.ImplementerID:
		return implement(ctx, cfg, issueID)
	case cfg.AgentID == plan.ReviewerID:
		// The reviewer's turn is a controller-routed bounded workflow task. It
		// runs under a workflow-broker grant through the lane review broker
		// (cmd/implementation-lane-review-broker), never through this bridge:
		// a bounded turn is tool-free and has no task token for this process.
		return "", errors.New("the reviewer turn runs under the lane review broker grant, not the issue bridge")
	default:
		return lead(ctx, cfg, issueID, plan)
	}
}

type workflowPlan struct {
	CurrentState  string `json:"current_state"`
	SquadID       string `json:"squad_id"`
	ImplementerID string `json:"implementer_id"`
	ReviewerID    string `json:"reviewer_id"`
}

type taskRun struct {
	AgentID string `json:"agent_id"`
}

// lead is the squad leader's turn. The product starts a squad workflow with a
// squad-scoped leader task, and a worker only belongs to that workflow when the
// leader delegates to it, because the worker task then inherits the leader's
// squad. So the leader delegates exactly once, by mentioning the implementer.
// Every later wake-up of the leader (a hand-off, an incident) is answered
// without a comment: a leader comment there would only route more work, and a
// failed leader task would stop a workflow that is still healthy.
func lead(ctx context.Context, cfg config, issueID string, plan workflowPlan) (string, error) {
	if uuid.Validate(plan.SquadID) != nil {
		return "", errors.New("the workflow plan names no squad")
	}
	var squad struct {
		LeaderID string `json:"leader_id"`
	}
	if err := cfg.api(ctx, http.MethodGet, "/api/squads/"+plan.SquadID, nil, &squad); err != nil {
		return "", fmt.Errorf("read workflow squad: %w", err)
	}
	if squad.LeaderID != cfg.AgentID {
		return "", errors.New("agent is neither the implementer, the reviewer nor the squad leader of this issue's workflow")
	}
	var raw json.RawMessage
	if err := cfg.api(ctx, http.MethodGet, "/api/issues/"+issueID+"/task-runs", nil, &raw); err != nil {
		return "", fmt.Errorf("read issue task runs: %w", err)
	}
	var runs []taskRun
	if json.Unmarshal(raw, &runs) != nil {
		var wrapped struct {
			Tasks []taskRun `json:"tasks"`
		}
		if err := json.Unmarshal(raw, &wrapped); err != nil {
			return "", fmt.Errorf("read issue task runs: %w", err)
		}
		runs = wrapped.Tasks
	}
	for _, run := range runs {
		if run.AgentID == plan.ImplementerID {
			return "Squad leader: the lane task is already delegated to the implementer; nothing to route.", nil
		}
	}
	if plan.CurrentState != "working" {
		return "Squad leader: the workflow is " + plan.CurrentState + "; nothing to route.", nil
	}
	if uuid.Validate(plan.ImplementerID) != nil {
		return "", errors.New("the workflow plan names no implementer")
	}
	content := "Delegating this issue's lane task to the implementer. [@Implementer](mention://agent/" + plan.ImplementerID + ")"
	if err := cfg.comment(ctx, issueID, content); err != nil {
		return "", fmt.Errorf("delegate to the implementer: %w", err)
	}
	return "Squad leader delegated the lane task to the implementer.", nil
}

func readPrompt(r io.Reader) (prompt, control string, err error) {
	line, err := bufio.NewReaderSize(r, 1<<20).ReadBytes('\n')
	if err != nil && err != io.EOF {
		return "", "", err
	}
	var frame struct {
		Type    string `json:"type"`
		Request struct {
			Subtype string `json:"subtype"`
		} `json:"request"`
		Message struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	}
	if json.Unmarshal(line, &frame) != nil {
		return string(line), "", nil
	}
	if frame.Type == "control_request" {
		return "", frame.Request.Subtype, nil
	}
	var text string
	if json.Unmarshal(frame.Message.Content, &text) == nil {
		return text, "", nil
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	_ = json.Unmarshal(frame.Message.Content, &blocks)
	var b strings.Builder
	for _, block := range blocks {
		b.WriteString(block.Text)
		b.WriteString("\n")
	}
	return b.String(), "", nil
}

func loadConfig() (config, error) {
	cfg := config{
		Python:    strings.TrimSpace(os.Getenv("LANE_BRIDGE_PYTHON")),
		RunTask:   strings.TrimSpace(os.Getenv("LANE_BRIDGE_RUN_TASK")),
		StateDir:  strings.TrimSpace(os.Getenv("LANE_BRIDGE_STATE_DIR")),
		ServerURL: strings.TrimRight(strings.TrimSpace(os.Getenv("MULTICA_SERVER_URL")), "/"),
		Token:     strings.TrimSpace(os.Getenv("MULTICA_TOKEN")),
		AgentID:   strings.TrimSpace(os.Getenv("MULTICA_AGENT_ID")),
		TaskID:    strings.TrimSpace(os.Getenv("MULTICA_TASK_ID")),
	}
	for _, repo := range strings.Split(os.Getenv("LANE_BRIDGE_REPOSITORIES"), ";") {
		if repo = strings.TrimSpace(repo); repo != "" {
			cfg.Repos = append(cfg.Repos, filepath.Clean(repo))
		}
	}
	if cfg.Python == "" || cfg.RunTask == "" || cfg.StateDir == "" || len(cfg.Repos) == 0 {
		return cfg, errors.New("bridge is not configured (LANE_BRIDGE_PYTHON, LANE_BRIDGE_RUN_TASK, LANE_BRIDGE_STATE_DIR, LANE_BRIDGE_REPOSITORIES)")
	}
	server, err := url.Parse(cfg.ServerURL)
	if err != nil || server.Scheme != "http" || server.Hostname() != "127.0.0.1" || server.Path != "" {
		return cfg, errors.New("bridge only talks to a loopback Multica server")
	}
	if !strings.HasPrefix(cfg.Token, "mat_") {
		return cfg, errors.New("bridge requires the daemon's task-scoped token")
	}
	if uuid.Validate(cfg.AgentID) != nil || uuid.Validate(cfg.TaskID) != nil {
		return cfg, errors.New("task or agent identity is unavailable")
	}
	return cfg, os.MkdirAll(cfg.StateDir, 0o700)
}

func (cfg config) api(ctx context.Context, method, path string, body, out any) error {
	var payload io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, cfg.ServerURL+path, payload)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-ID", cfg.AgentID)
	req.Header.Set("X-Task-ID", cfg.TaskID)
	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s returned HTTP %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(raw[:min(len(raw), 300)])))
	}
	if out != nil && len(bytes.TrimSpace(raw)) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}

func (cfg config) comment(ctx context.Context, issueID, content string) error {
	body, err := brokerreply.CommentBody(ctx, func(ctx context.Context, path string, out any) error {
		return cfg.api(ctx, http.MethodGet, path, nil, out)
	}, issueID, cfg.TaskID, cfg.AgentID, content)
	if err != nil {
		return err
	}
	return cfg.api(ctx, http.MethodPost, "/api/issues/"+issueID+"/comments", body, nil)
}

func (cfg config) event(ctx context.Context, issueID string, body map[string]any) error {
	return cfg.api(ctx, http.MethodPost, "/api/issues/"+issueID+"/workflow-events", body, nil)
}

// parseSpec extracts the one lane task spec a human put in the issue and checks
// it against the bridge's eligibility policy.
func parseSpec(description string, cfg config) (laneSpec, []byte, error) {
	blocks := specBlock.FindAllStringSubmatch(description, -1)
	if len(blocks) != 1 {
		return laneSpec{}, nil, errors.New("the issue must contain exactly one ```lane-task JSON block")
	}
	raw := []byte(blocks[0][1])
	var spec laneSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return spec, nil, fmt.Errorf("lane task spec is not valid JSON: %w", err)
	}
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9-]{2,80}$`).MatchString(spec.TaskID) {
		return spec, nil, errors.New("lane task_id must be lowercase letters, digits and dashes")
	}
	if len(spec.AllowedPaths) == 0 || len(spec.VerificationCommand) == 0 || strings.TrimSpace(spec.Objective) == "" {
		return spec, nil, errors.New("lane task spec needs allowed_paths, verification_command and objective")
	}
	repo := filepath.Clean(spec.Repository)
	allowed := false
	for _, candidate := range cfg.Repos {
		if strings.EqualFold(candidate, repo) {
			allowed = true
		}
	}
	if !allowed {
		return spec, nil, fmt.Errorf("repository %q is not eligible for the lane", spec.Repository)
	}
	return spec, raw, nil
}

func implement(ctx context.Context, cfg config, issueID string) (string, error) {
	recordPath := laneevidence.RecordPath(cfg.StateDir, issueID)
	if _, err := os.Stat(recordPath); err == nil {
		// The lane already used its one correction inside the run. A second
		// implementer turn for the same issue (for example the correction a
		// failed review requests) is refused, never re-run. The refusal is a
		// terminal error result, so the server records a delegated failure and
		// raises the owner-visible technical stop instead of stalling.
		return "", errors.New("this issue already consumed its lane run; a further implementation turn is not authorized")
	}
	var issue struct {
		Description string `json:"description"`
	}
	if err := cfg.api(ctx, http.MethodGet, "/api/issues/"+issueID, nil, &issue); err != nil {
		return "", fmt.Errorf("read issue: %w", err)
	}
	spec, raw, err := parseSpec(issue.Description, cfg)
	if err != nil {
		_ = cfg.comment(ctx, issueID, "Implementation lane could not start: "+err.Error())
		return "", err
	}
	workDir := filepath.Join(cfg.StateDir, issueID)
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		return "", err
	}
	specPath, resultPath := filepath.Join(workDir, "task.json"), filepath.Join(workDir, "result.json")
	if err := os.WriteFile(specPath, raw, 0o600); err != nil {
		return "", err
	}
	started := time.Now()
	logFile, err := os.Create(filepath.Join(workDir, "lane.log"))
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, cfg.Python, cfg.RunTask, specPath, "--result-file", resultPath)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	runErr := cmd.Run()
	_ = logFile.Close()
	elapsed := time.Since(started).Seconds()

	var result struct {
		RunID       string `json:"run_id"`
		EvidenceDir string `json:"evidence_dir"`
		Succeeded   bool   `json:"succeeded"`
		HostBlocked bool   `json:"host_blocked"`
		Failure     string `json:"failure"`
	}
	resultRaw, readErr := os.ReadFile(resultPath)
	if readErr != nil || json.Unmarshal(resultRaw, &result) != nil {
		msg := fmt.Sprintf("Implementation lane ended without a result (exit: %v). Nothing was published.", runErr)
		_ = cfg.comment(ctx, issueID, msg)
		return "", errors.New(msg)
	}
	outcomePath := filepath.Join(result.EvidenceDir, "outcome.json")
	outcomeRaw, _ := os.ReadFile(outcomePath)
	summary := laneSummary(outcomeRaw, elapsed)
	if !result.Succeeded {
		reason := failureReason(result.Failure, result.EvidenceDir)
		msg := "Implementation lane run " + result.RunID + " did not pass: " + reason + "\n\n" + summary
		_ = cfg.comment(ctx, issueID, msg)
		return "", errors.New("lane run did not pass: " + reason)
	}
	patchPath := filepath.Join(result.EvidenceDir, "final.patch")
	patchRaw, err := os.ReadFile(patchPath)
	if err != nil {
		return "", fmt.Errorf("lane passed but its patch is missing: %w", err)
	}
	record := laneevidence.Record{IssueID: issueID, TaskID: spec.TaskID, RunID: result.RunID,
		EvidenceDir: result.EvidenceDir, Repository: spec.Repository, PatchSHA256: digest(patchRaw),
		OutcomeSHA256: digest(outcomeRaw), RecordedAt: time.Now().UTC(), ElapsedSeconds: elapsed}
	recordRaw, _ := json.MarshalIndent(record, "", "  ")
	if err := os.WriteFile(recordPath, recordRaw, 0o600); err != nil {
		return "", err
	}
	packet, err := evidencePacket(spec.Repository, []string{patchPath, outcomePath})
	if err != nil {
		return "", fmt.Errorf("build artifact evidence: %w", err)
	}
	content := "Implementation lane run " + result.RunID + " passed. The change is a patch; nothing was committed, pushed or deployed.\n\n" +
		summary + "\n\nPatch SHA256: `" + record.PatchSHA256 + "`\nEvidence: `" + result.EvidenceDir + "`"
	if err := cfg.comment(ctx, issueID, content); err != nil {
		return "", fmt.Errorf("publish lane result: %w", err)
	}
	reason := "Implementation lane run " + result.RunID + " passed every harness gate and its Codex review."
	if err := cfg.event(ctx, issueID, map[string]any{"event": "implementation_ready", "reason": reason, "artifact_evidence": packet}); err != nil {
		return "", fmt.Errorf("emit implementation_ready: %w", err)
	}
	return reason, nil
}

// failureReason names why a lane run did not pass. The lane records a failure
// string for harness failures, but a run whose independent review did not pass
// ends with no failure string at all; the owner then saw "did not pass: " with
// nothing after it. The last review verdict in the evidence says why.
func failureReason(failure, evidenceDir string) string {
	if strings.TrimSpace(failure) != "" {
		return failure
	}
	for _, name := range []string{"04-review-after-correction.verdict.json", "02-reviewer.verdict.json"} {
		raw, err := os.ReadFile(filepath.Join(evidenceDir, name))
		if err != nil {
			continue
		}
		var verdict struct {
			Verdict  string   `json:"verdict"`
			Findings []string `json:"findings"`
		}
		if json.Unmarshal(raw, &verdict) != nil || verdict.Verdict == "pass" {
			continue
		}
		if len(verdict.Findings) == 0 {
			return "the independent review did not pass"
		}
		return "the independent review did not pass: " + strings.Join(verdict.Findings, " ")
	}
	return "the lane recorded no reason"
}

func laneSummary(outcomeRaw []byte, elapsed float64) string {
	var o map[string]any
	if json.Unmarshal(outcomeRaw, &o) != nil {
		return fmt.Sprintf("Lane time: %.1f min. Outcome record unavailable.", elapsed/60)
	}
	lines := []string{fmt.Sprintf("- Lane time: %.1f min", elapsed/60)}
	for _, key := range []string{"first_attempt_passed", "correction_used", "verification_passed", "test_fails_before_change",
		"regression_passed", "reviewer_passed", "server_contract_warnings", "dependency_install_unchanged", "authority_revoked", "worktree_removed"} {
		if value, ok := o[key]; ok {
			lines = append(lines, fmt.Sprintf("- %s: %v", key, value))
		}
	}
	lines = append(lines, "- "+laneevidence.ReviewerLine(outcomeRaw))
	if memory, ok := o["memory"].(map[string]any); ok {
		lines = append(lines, fmt.Sprintf("- Peak lane memory: %v MB", memory["peak_lane_process_tree_mb"]))
	}
	return strings.Join(lines, "\n")
}

func digest(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }

type evidenceRoot struct {
	Path               string `json:"path"`
	Branch             string `json:"branch"`
	HeadSHA            string `json:"head_sha"`
	TrackedChangeCount int64  `json:"tracked_change_count"`
	UntrackedFileCount int64  `json:"untracked_file_count"`
	Dirty              bool   `json:"dirty"`
}

type evidenceWorktree struct {
	RootPath    string `json:"root_path"`
	Path        string `json:"path"`
	Branch      string `json:"branch"`
	HeadSHA     string `json:"head_sha"`
	PathChecked bool   `json:"path_checked"`
	PathPresent bool   `json:"path_present"`
}

// evidencePacket builds repository_worktree_inventory.v1 evidence from git for
// the task repository: one root record and one record per registered worktree,
// each with an explicit path check, plus digests of the published artifacts.
func evidencePacket(repository string, artifacts []string) (map[string]any, error) {
	git := func(args ...string) (string, error) {
		out, err := exec.Command("git", append([]string{"-C", repository}, args...)...).Output()
		return string(out), err
	}
	head, err := git("rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	branch, _ := git("rev-parse", "--abbrev-ref", "HEAD")
	status, err := git("status", "--porcelain=v1", "--untracked-files=normal")
	if err != nil {
		return nil, err
	}
	var tracked, untracked int64
	for _, line := range strings.Split(strings.TrimRight(status, "\n"), "\n") {
		switch {
		case line == "":
		case strings.HasPrefix(line, "??"):
			untracked++
		default:
			tracked++
		}
	}
	rootPath := filepath.ToSlash(repository)
	roots := []evidenceRoot{{Path: rootPath, Branch: strings.TrimSpace(branch), HeadSHA: strings.TrimSpace(head),
		TrackedChangeCount: tracked, UntrackedFileCount: untracked, Dirty: tracked+untracked > 0}}
	listing, err := git("worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var worktrees []evidenceWorktree
	var current *evidenceWorktree
	flush := func() {
		if current != nil {
			if current.Branch == "" {
				current.Branch = "(detached)"
			}
			_, statErr := os.Stat(current.Path)
			current.PathChecked, current.PathPresent = true, statErr == nil
			worktrees = append(worktrees, *current)
		}
		current = nil
	}
	for _, line := range strings.Split(listing, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			flush()
			current = &evidenceWorktree{RootPath: rootPath, Path: filepath.ToSlash(strings.TrimPrefix(line, "worktree "))}
		case strings.HasPrefix(line, "HEAD ") && current != nil:
			current.HeadSHA = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch ") && current != nil:
			current.Branch = strings.TrimPrefix(strings.TrimPrefix(line, "branch "), "refs/heads/")
		}
	}
	flush()
	var present int64
	for _, w := range worktrees {
		if w.PathPresent {
			present++
		}
	}
	var digests []map[string]string
	for _, path := range artifacts {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		sum := digest(raw)
		digests = append(digests, map[string]string{"path": filepath.ToSlash(path), "declared_sha256": sum, "observed_sha256": sum})
	}
	records, err := json.Marshal(struct {
		Roots     []evidenceRoot     `json:"roots"`
		Worktrees []evidenceWorktree `json:"worktrees"`
	}{roots, worktrees})
	if err != nil {
		return nil, err
	}
	dirty := int64(0)
	if roots[0].Dirty {
		dirty = 1
	}
	n := int64(len(worktrees))
	return map[string]any{
		"schema_version": 1, "contract": "repository_worktree_inventory.v1", "artifacts": digests,
		"inventory": map[string]any{"root_count": 1, "dirty_root_count": dirty, "clean_root_count": 1 - dirty,
			"worktree_count": n, "per_worktree_record_count": n, "required_path_check_count": n,
			"completed_path_check_count": n, "present_path_count": present, "absent_path_count": n - present},
		"roots": roots, "worktrees": worktrees, "inventory_records_sha256": digest(records),
	}, nil
}
