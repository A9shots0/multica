package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
	publicapiv1 "github.com/multica-ai/multica/server/pkg/publicapi/v1"
)

const (
	codexObserverStateVersion = 1
	maxCodexRolloutLineBytes  = 16 << 20
)

type codexTaskSnapshot struct {
	Name                string `json:"name,omitempty"`
	EndGoal             string `json:"end_goal,omitempty"`
	CurrentFocus        string `json:"current_focus,omitempty"`
	Remaining           string `json:"remaining,omitempty"`
	NextProof           string `json:"next_proof,omitempty"`
	RiskNote            string `json:"risk_note,omitempty"`
	OwnerAction         string `json:"owner_action,omitempty"`
	Status              string `json:"status,omitempty"`
	OwnerActionRequired bool   `json:"owner_action_required,omitempty"`
}

type codexObserverState struct {
	Version             int               `json:"version"`
	Server              string            `json:"server"`
	IssueRef            string            `json:"issue_ref"`
	RolloutPath         string            `json:"rollout_path"`
	TaskID              string            `json:"task_id"`
	WorkID              string            `json:"work_id"`
	RunID               string            `json:"run_id"`
	Offset              int64             `json:"offset"`
	LastOrdinal         int64             `json:"last_ordinal"`
	LastReceiptRevision int64             `json:"last_receipt_revision"`
	LastSemanticHash    string            `json:"last_semantic_hash,omitempty"`
	Snapshot            codexTaskSnapshot `json:"snapshot"`
}

type codexObservedEvent struct {
	Ordinal       int64
	EndOffset     int64
	ObservedAt    time.Time
	Kind          string
	Title         string
	Detail        string
	MilestoneKind string
	Snapshot      codexTaskSnapshot
}

type codexScanResult struct {
	RunID       string
	Events      []codexObservedEvent
	EndOffset   int64
	LastOrdinal int64
	Snapshot    codexTaskSnapshot
}

type codexRolloutEnvelope struct {
	Timestamp string          `json:"timestamp"`
	Ordinal   int64           `json:"ordinal"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type codexSessionMeta struct {
	ID        string `json:"id"`
	SessionID string `json:"session_id"`
}

type codexEventMessage struct {
	Type string             `json:"type"`
	Item codexCompletedItem `json:"item"`
}

type codexCompletedItem struct {
	Type      string          `json:"type"`
	Tool      string          `json:"tool"`
	Status    string          `json:"status"`
	Arguments json.RawMessage `json:"arguments"`
}

type codexTaskUpsertArguments struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	EndGoal             string `json:"endGoal"`
	CurrentFocus        string `json:"currentFocus"`
	Remaining           string `json:"remaining"`
	NextProof           string `json:"nextProof"`
	RiskNote            string `json:"riskNote"`
	OwnerAction         string `json:"ownerAction"`
	Status              string `json:"status"`
	ProgressPct         int    `json:"progressPct"`
	OwnerActionRequired bool   `json:"ownerActionRequired"`
}

type codexTaskMilestoneArguments struct {
	TaskID    string `json:"taskId"`
	Kind      string `json:"kind"`
	Title     string `json:"title"`
	Detail    string `json:"detail"`
	CreatedAt string `json:"createdAt"`
}

type codexObserverOutput struct {
	Status          string                         `json:"status"`
	IssueRef        string                         `json:"issue_ref"`
	TaskID          string                         `json:"task_id"`
	RunID           string                         `json:"run_id"`
	ObservedOrdinal int64                          `json:"observed_ordinal,omitempty"`
	Receipt         *publicapiv1.OutcomeReceiptV02 `json:"receipt,omitempty"`
	Import          *receiptImportOutput           `json:"import,omitempty"`
}

func newIssueReceiptObserveCodexCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "observe-codex <issue-ref>",
		Short: "Observe one explicitly linked Codex task without controlling it",
		Long: "Read only the named Codex rollout JSONL file and only completed Task Control updates for the exact --task-id. " +
			"The command never discovers other sessions, reads chat prose, or sends instructions to Codex. It deterministically converts meaningful updates into Outcome Receipt v0.2 observations.",
		Args: exactArgs(1),
		RunE: runIssueReceiptObserveCodex,
	}
	cmd.Flags().String("rollout-file", "", "Exact Codex rollout JSONL file to observe")
	cmd.Flags().String("task-id", "", "Exact Task Control task ID permitted for this link")
	cmd.Flags().String("work-id", "", "Stable external work ID (default: codex-task:<task-id>)")
	cmd.Flags().Bool("follow", false, "Keep watching the same file for later meaningful updates")
	cmd.Flags().Duration("poll-interval", 5*time.Second, "How often --follow checks the linked file")
	cmd.Flags().Bool("dry-run", false, "Preview the latest derived receipt without credentials, network access, or saved state")
	cmd.Flags().String("output", "text", "Output format: text or json")
	cmd.Flags().Bool("allow-external-file", false, "Allow --rollout-file to read outside the current working directory")
	return cmd
}

func runIssueReceiptObserveCodex(cmd *cobra.Command, args []string) error {
	rolloutPath, _ := cmd.Flags().GetString("rollout-file")
	taskID, _ := cmd.Flags().GetString("task-id")
	workID, _ := cmd.Flags().GetString("work-id")
	outputFormat, _ := cmd.Flags().GetString("output")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	follow, _ := cmd.Flags().GetBool("follow")
	pollInterval, _ := cmd.Flags().GetDuration("poll-interval")
	issueRef := strings.TrimSpace(args[0])
	rolloutPath = strings.TrimSpace(rolloutPath)
	taskID = strings.TrimSpace(taskID)
	workID = strings.TrimSpace(workID)

	if rolloutPath == "" || taskID == "" {
		return receiptValidationError("Choose one exact Codex rollout with --rollout-file and one exact Task Control record with --task-id.", nil)
	}
	if workID == "" {
		workID = "codex-task:" + taskID
	}
	if outputFormat != "text" && outputFormat != "json" {
		return receiptValidationError("--output must be text or json.", nil)
	}
	if follow && dryRun {
		return receiptValidationError("--dry-run is one-shot; remove --follow.", nil)
	}
	if pollInterval < time.Second {
		return receiptValidationError("--poll-interval must be at least 1s.", nil)
	}
	if err := ensureFileFlagWithinWorkdir(cmd, "rollout-file", "codex-rollout", rolloutPath); err != nil {
		return receiptValidationError("The Codex rollout file is outside this work directory.", err)
	}
	canonicalPath, err := filepath.EvalSymlinks(rolloutPath)
	if err != nil {
		return receiptValidationError("The Codex rollout file could not be resolved.", err)
	}
	canonicalPath, err = filepath.Abs(canonicalPath)
	if err != nil {
		return receiptValidationError("The Codex rollout file could not be resolved.", err)
	}
	if err := validateCodexRolloutFile(canonicalPath); err != nil {
		return err
	}

	serverIdentity := "offline"
	if !dryRun {
		serverURL := tryResolveServerURL(cmd)
		if serverURL == "" {
			return receiptValidationError("No Multica server is configured. Set --server-url, MULTICA_SERVER_URL, or the profile server_url.", nil)
		}
		serverIdentity, err = safeReceiptServerIdentity(serverURL)
		if err != nil {
			return receiptValidationError("The configured Multica server URL is invalid.", err)
		}
	}

	statePath, err := codexObserverStatePath(resolveProfile(cmd), serverIdentity, issueRef, canonicalPath, taskID, workID)
	if err != nil {
		return fmt.Errorf("prepare Codex observer state: %w", err)
	}
	var state *codexObserverState
	if !dryRun {
		state, err = loadCodexObserverState(statePath)
		if err != nil {
			return receiptValidationError("The saved Codex observer state is invalid; nothing was sent.", err)
		}
		if state != nil {
			if err := state.matches(serverIdentity, issueRef, canonicalPath, taskID, workID); err != nil {
				return receiptValidationError("The saved Codex observer state does not match this link; nothing was sent.", err)
			}
		}
	}

	for {
		updated, err := observeCodexOnce(cmd, statePath, issueRef, canonicalPath, taskID, workID, serverIdentity, outputFormat, dryRun, &state)
		if err != nil {
			return err
		}
		if !follow || dryRun {
			if !updated && !dryRun {
				return printCodexObserverOutput(cmd, outputFormat, codexObserverOutput{Status: "unchanged", IssueRef: issueRef, TaskID: taskID, RunID: state.RunID})
			}
			return nil
		}
		select {
		case <-cmd.Context().Done():
			return cmd.Context().Err()
		case <-time.After(pollInterval):
		}
	}
}

func observeCodexOnce(cmd *cobra.Command, statePath, issueRef, rolloutPath, taskID, workID, serverIdentity, outputFormat string, dryRun bool, state **codexObserverState) (bool, error) {
	scan, err := scanCodexRollout(rolloutPath, taskID, *state)
	if err != nil {
		return false, err
	}
	if *state == nil {
		*state = &codexObserverState{
			Version: codexObserverStateVersion, Server: serverIdentity, IssueRef: issueRef,
			RolloutPath: rolloutPath, TaskID: taskID, WorkID: workID, RunID: scan.RunID,
			Snapshot: scan.Snapshot,
		}
	} else if (*state).RunID != scan.RunID {
		return false, receiptValidationError("The linked rollout now reports a different Codex run ID; relink it explicitly.", nil)
	}

	events := scan.Events
	if (*state).LastReceiptRevision == 0 && len(events) > 1 {
		events = events[len(events)-1:]
	}
	updated := false
	for _, event := range events {
		semanticHash, err := codexEventSemanticHash(event)
		if err != nil {
			return updated, err
		}
		if semanticHash == (*state).LastSemanticHash {
			(*state).LastOrdinal = event.Ordinal
			(*state).Offset = event.EndOffset
			(*state).Snapshot = event.Snapshot
			continue
		}
		receipt, err := codexEventReceipt(workID, scan.RunID, taskID, (*state).LastReceiptRevision+1, event)
		if err != nil {
			return updated, err
		}
		if dryRun {
			return true, printCodexObserverOutput(cmd, outputFormat, codexObserverOutput{
				Status: "preview", IssueRef: issueRef, TaskID: taskID, RunID: scan.RunID,
				ObservedOrdinal: event.Ordinal, Receipt: &receipt,
			})
		}
		digest, err := outcomeReceiptDigest(receipt)
		if err != nil {
			return updated, err
		}
		result, err := submitOutcomeReceipt(cmd, issueRef, receipt, digest)
		if err != nil {
			return updated, err
		}
		(*state).LastReceiptRevision = receipt.ReceiptRevision
		(*state).LastSemanticHash = semanticHash
		(*state).LastOrdinal = event.Ordinal
		(*state).Offset = event.EndOffset
		(*state).Snapshot = event.Snapshot
		if err := saveCodexObserverState(statePath, *state); err != nil {
			return updated, fmt.Errorf("receipt was accepted, but the Codex observer checkpoint could not be saved; rerun this exact command safely: %w", err)
		}
		updated = true
		if err := printCodexObserverOutput(cmd, outputFormat, codexObserverOutput{
			Status: "accepted", IssueRef: issueRef, TaskID: taskID, RunID: scan.RunID,
			ObservedOrdinal: event.Ordinal, Receipt: &receipt, Import: &result,
		}); err != nil {
			return updated, err
		}
	}

	(*state).RunID = scan.RunID
	(*state).Offset = scan.EndOffset
	(*state).LastOrdinal = scan.LastOrdinal
	(*state).Snapshot = scan.Snapshot
	if !dryRun {
		if err := saveCodexObserverState(statePath, *state); err != nil {
			return updated, fmt.Errorf("save Codex observer checkpoint: %w", err)
		}
	}
	if dryRun && len(events) == 0 {
		return false, receiptValidationError("No completed Task Control update for the exact --task-id exists in this rollout.", nil)
	}
	return updated, nil
}

func validateCodexRolloutFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return receiptValidationError("The Codex rollout file could not be inspected.", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return receiptValidationError("The Codex rollout path must name one regular file.", nil)
	}
	return nil
}

func scanCodexRollout(path, taskID string, prior *codexObserverState) (codexScanResult, error) {
	result := codexScanResult{}
	startOffset := int64(0)
	if prior != nil {
		startOffset = prior.Offset
		result.RunID = prior.RunID
		result.LastOrdinal = prior.LastOrdinal
		result.Snapshot = prior.Snapshot
	}
	f, err := os.Open(path)
	if err != nil {
		return result, receiptValidationError("The Codex rollout file could not be opened.", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return result, receiptValidationError("The Codex rollout file could not be inspected.", err)
	}
	if startOffset > info.Size() {
		return result, receiptValidationError("The linked Codex rollout was truncated; relink it explicitly.", nil)
	}
	if _, err := f.Seek(startOffset, io.SeekStart); err != nil {
		return result, receiptValidationError("The Codex rollout checkpoint could not be resumed.", err)
	}
	reader := bufio.NewReaderSize(f, 64*1024)
	offset := startOffset
	for {
		lineStart := offset
		line, readErr := reader.ReadBytes('\n')
		if len(line) > maxCodexRolloutLineBytes {
			return result, receiptValidationError("A Codex rollout line exceeds the 16 MiB safety limit.", nil)
		}
		if len(line) > 0 {
			var envelope codexRolloutEnvelope
			trimmed := bytes.TrimSpace(line)
			if len(trimmed) > 0 {
				if err := json.Unmarshal(trimmed, &envelope); err != nil {
					if errors.Is(readErr, io.EOF) {
						result.EndOffset = lineStart
						break
					}
					return result, receiptValidationError("The linked Codex rollout contains invalid JSONL.", err)
				}
				if envelope.Ordinal > result.LastOrdinal {
					result.LastOrdinal = envelope.Ordinal
				}
				if envelope.Type == "session_meta" {
					var meta codexSessionMeta
					if err := json.Unmarshal(envelope.Payload, &meta); err != nil {
						return result, receiptValidationError("The linked Codex session metadata is invalid.", err)
					}
					candidate := strings.TrimSpace(meta.SessionID)
					if candidate == "" {
						candidate = strings.TrimSpace(meta.ID)
					}
					if candidate == "" {
						return result, receiptValidationError("The linked Codex rollout has no stable session ID.", nil)
					}
					if result.RunID != "" && result.RunID != candidate {
						return result, receiptValidationError("The linked Codex rollout contains conflicting session IDs.", nil)
					}
					result.RunID = candidate
				}
				event, ok, err := parseCodexTaskEvent(envelope, taskID, result.Snapshot, offset)
				if err != nil {
					return result, err
				}
				if ok {
					result.Snapshot = event.Snapshot
					result.Events = append(result.Events, event)
				}
			}
			offset += int64(len(line))
		}
		if readErr != nil {
			if !errors.Is(readErr, io.EOF) {
				return result, receiptValidationError("The linked Codex rollout could not be read.", readErr)
			}
			break
		}
	}
	result.EndOffset = offset
	if result.RunID == "" {
		return result, receiptValidationError("The linked Codex rollout has no session metadata.", nil)
	}
	return result, nil
}

func parseCodexTaskEvent(envelope codexRolloutEnvelope, taskID string, snapshot codexTaskSnapshot, endOffset int64) (codexObservedEvent, bool, error) {
	if envelope.Type != "event_msg" {
		return codexObservedEvent{}, false, nil
	}
	var message codexEventMessage
	if err := json.Unmarshal(envelope.Payload, &message); err != nil {
		return codexObservedEvent{}, false, nil
	}
	if message.Type != "item_completed" || message.Item.Type != "McpToolCall" || message.Item.Status != "completed" {
		return codexObservedEvent{}, false, nil
	}
	observedAt, err := time.Parse(time.RFC3339Nano, envelope.Timestamp)
	if err != nil {
		return codexObservedEvent{}, false, receiptValidationError("A linked Task Control update has an invalid timestamp.", err)
	}
	event := codexObservedEvent{Ordinal: envelope.Ordinal, EndOffset: endOffset, ObservedAt: observedAt, Snapshot: snapshot}
	switch message.Item.Tool {
	case "task_control.upsert_task":
		var args codexTaskUpsertArguments
		if err := json.Unmarshal(message.Item.Arguments, &args); err != nil {
			return codexObservedEvent{}, false, receiptValidationError("A linked Task Control update has invalid arguments.", err)
		}
		if strings.TrimSpace(args.ID) != taskID {
			return codexObservedEvent{}, false, nil
		}
		event.Kind = "upsert"
		event.Title = firstNonblank(args.CurrentFocus, args.Name, "Codex work updated")
		event.Detail = firstNonblank(args.CurrentFocus, args.Remaining, "The linked Codex task reported a meaningful update.")
		event.Snapshot = codexTaskSnapshot{
			Name: strings.TrimSpace(args.Name), EndGoal: strings.TrimSpace(args.EndGoal),
			CurrentFocus: strings.TrimSpace(args.CurrentFocus), Remaining: strings.TrimSpace(args.Remaining),
			NextProof: strings.TrimSpace(args.NextProof), RiskNote: strings.TrimSpace(args.RiskNote),
			OwnerAction: strings.TrimSpace(args.OwnerAction), Status: strings.TrimSpace(args.Status),
			OwnerActionRequired: args.OwnerActionRequired,
		}
		return event, true, nil
	case "task_control.record_milestone":
		var args codexTaskMilestoneArguments
		if err := json.Unmarshal(message.Item.Arguments, &args); err != nil {
			return codexObservedEvent{}, false, receiptValidationError("A linked Task Control milestone has invalid arguments.", err)
		}
		if strings.TrimSpace(args.TaskID) != taskID {
			return codexObservedEvent{}, false, nil
		}
		event.Kind = "milestone"
		event.MilestoneKind = strings.TrimSpace(args.Kind)
		event.Title = firstNonblank(args.Title, "Codex milestone")
		event.Detail = firstNonblank(args.Detail, args.Title, "The linked Codex task recorded a milestone.")
		if args.CreatedAt != "" {
			if createdAt, err := time.Parse(time.RFC3339Nano, args.CreatedAt); err == nil {
				event.ObservedAt = createdAt
			}
		}
		return event, true, nil
	default:
		return codexObservedEvent{}, false, nil
	}
}

func codexEventReceipt(workID, runID, taskID string, revision int64, event codexObservedEvent) (publicapiv1.OutcomeReceiptV02, error) {
	objective := firstNonblank(event.Snapshot.EndGoal, event.Snapshot.Name)
	if objective == "" {
		return publicapiv1.OutcomeReceiptV02{}, receiptValidationError("The linked Task Control record has no goal yet; wait for an upsert before importing milestones.", nil)
	}
	state := codexReceiptState(event)
	nextAction := firstNonblank(event.Snapshot.NextProof, event.Snapshot.Remaining, "Continue the explicitly linked Codex work.")
	criteria := []string{"Continue to the next evidence-backed milestone."}
	if state == "waiting_external" {
		nextAction = "Independently verify the Codex completion claim before accepting the Multica issue."
		criteria = []string{"Independent Multica acceptance of the reported Codex result."}
	}
	dependencies := []string{}
	if event.Snapshot.RiskNote != "" {
		dependencies = append(dependencies, event.Snapshot.RiskNote)
	}
	if event.Snapshot.OwnerActionRequired && event.Snapshot.OwnerAction != "" {
		dependencies = append(dependencies, "Owner action reported by linked task: "+event.Snapshot.OwnerAction)
	}
	risk := "low"
	if len(dependencies) > 0 || state == "blocked" {
		risk = "medium"
	}
	var supersedes *int64
	if revision > 1 {
		previous := revision - 1
		supersedes = &previous
	}
	receipt := publicapiv1.OutcomeReceiptV02{
		WorkID: workID, RunID: runID, ReceiptRevision: revision, SupersedesReceiptRevision: supersedes,
		Provider: "codex", Objective: objective,
		Scope:         "Read-only observation of the explicitly linked Codex Task Control record " + taskID + ".",
		RequestedMode: "monitor",
		Authority: publicapiv1.OutcomeReceiptAuthority{
			Reference:      "codex-rollout:" + runID + ":task:" + taskID,
			AllowedEffects: []string{},
		},
		State: state, CurrentMilestone: event.Title, ChangeSinceLastReceipt: event.Detail,
		Evidence: []publicapiv1.OutcomeReceiptEvidence{{
			Type: "controller", Reference: fmt.Sprintf("codex-rollout:%s:ordinal:%d:task:%s", runID, event.Ordinal, taskID),
		}},
		Dependencies: dependencies, NextAction: nextAction,
		OwnerDecision: publicapiv1.OutcomeReceiptOwnerDecision{Required: false, Options: []string{}, InvalidatedBy: []string{}},
		Risk:          risk,
		ResourceUsage: publicapiv1.OutcomeReceiptResourceUsage{Source: "unavailable"},
		Effect:        publicapiv1.OutcomeReceiptEffect{UserOutcome: "unchanged", ProductionChanged: false},
		Freshness:     publicapiv1.OutcomeReceiptFreshness{ObservedAt: event.ObservedAt.UTC().Format(time.RFC3339Nano)},
		Acceptance:    publicapiv1.OutcomeReceiptAcceptance{Stage: "local", Status: "partial", CriteriaRemaining: criteria},
	}
	if err := receipt.Validate(); err != nil {
		return publicapiv1.OutcomeReceiptV02{}, receiptValidationError("The linked Codex update could not produce a valid Outcome Receipt v0.2.", err)
	}
	return receipt, nil
}

func codexReceiptState(event codexObservedEvent) string {
	status := strings.ToLower(strings.TrimSpace(event.Snapshot.Status))
	kind := strings.ToLower(strings.TrimSpace(event.MilestoneKind))
	if event.Snapshot.OwnerActionRequired || status == "blocked" || status == "stalled" || kind == "blocked" || kind == "failure" {
		return "blocked"
	}
	if status == "complete" || status == "completed" || kind == "complete" || kind == "completion" {
		return "waiting_external"
	}
	if status == "paused" || status == "stopped" || status == "cancelled" {
		return "stopped"
	}
	return "working"
}

func codexEventSemanticHash(event codexObservedEvent) (string, error) {
	meaningful := struct {
		Kind          string            `json:"kind"`
		MilestoneKind string            `json:"milestone_kind"`
		Title         string            `json:"title"`
		Detail        string            `json:"detail"`
		Snapshot      codexTaskSnapshot `json:"snapshot"`
	}{event.Kind, event.MilestoneKind, event.Title, event.Detail, event.Snapshot}
	data, err := json.Marshal(meaningful)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func codexObserverStatePath(profile, server, issueRef, rolloutPath, taskID, workID string) (string, error) {
	profileDir, err := cli.ProfileDir(profile)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(strings.Join([]string{server, issueRef, rolloutPath, taskID, workID}, "\n")))
	return filepath.Join(profileDir, "receipt-observers", hex.EncodeToString(sum[:])+".json"), nil
}

func loadCodexObserverState(path string) (*codexObserverState, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		path += ".previous"
		info, err = os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > 128*1024 {
		return nil, fmt.Errorf("observer checkpoint is not a small regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var state codexObserverState
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("observer checkpoint must contain exactly one JSON object")
	}
	if state.Version != codexObserverStateVersion || state.Server == "" || state.IssueRef == "" || state.RolloutPath == "" || state.TaskID == "" || state.WorkID == "" || state.RunID == "" || state.Offset < 0 || state.LastReceiptRevision < 0 {
		return nil, fmt.Errorf("observer checkpoint is incomplete or unsupported")
	}
	return &state, nil
}

func (s *codexObserverState) matches(server, issueRef, rolloutPath, taskID, workID string) error {
	if s.Server != server || s.IssueRef != issueRef || s.RolloutPath != rolloutPath || s.TaskID != taskID || s.WorkID != workID {
		return fmt.Errorf("server, issue, rollout, task, or work ID differs")
	}
	return nil
}

func saveCodexObserverState(path string, state *codexObserverState) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".codex-observer-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	previousPath := path + ".previous"
	if _, err := os.Lstat(path); err == nil {
		_ = os.Remove(previousPath)
		if err := os.Rename(path, previousPath); err != nil {
			return err
		}
		if err := os.Rename(tmpPath, path); err != nil {
			_ = os.Rename(previousPath, path)
			return err
		}
		_ = os.Remove(previousPath)
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	_ = os.Remove(previousPath)
	return nil
}

func printCodexObserverOutput(cmd *cobra.Command, format string, output codexObserverOutput) error {
	if format == "json" {
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(output)
	}
	switch output.Status {
	case "preview":
		fmt.Fprintf(cmd.OutOrStdout(), "Previewed one meaningful update for %s. No data was sent.\nMilestone: %s\n", output.IssueRef, output.Receipt.CurrentMilestone)
	case "unchanged":
		fmt.Fprintf(cmd.OutOrStdout(), "No new meaningful Codex update for %s. Nothing was sent.\n", output.IssueRef)
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "Recorded Codex update for %s.\nMilestone: %s\nReceipt ID: %s\n", output.IssueRef, output.Receipt.CurrentMilestone, output.Import.Result.ReceiptID)
	}
	return nil
}

func firstNonblank(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
