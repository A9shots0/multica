package agentqualification

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const RealisticNextHarnessVersion = "realistic-next-harness.v1"

var ErrRealisticNextUnsafeExecution = errors.New("realistic next harness unsafe execution")

type RealisticNextLimits struct {
	MaxSteps           int
	MaxInputTokens     int64
	MaxOutputTokens    int64
	MaxReasoningTokens int64
	MaxCostMicros      int64
}

type RealisticNextWorkspace struct {
	CaseID        string
	CaseVersion   string
	Root          string
	InitialCopy   string
	StartingHash  string
	EditablePaths []string
	AllowedChecks []string
	r05           *R05V1HarnessEvidence
	r06           *R06V1Workspace
}

type RealisticNextRunResult struct {
	Controller   RealisticControllerResult `json:"controller"`
	GlobalHalt   bool                      `json:"global_halt"`
	HaltReason   string                    `json:"halt_reason,omitempty"`
	StartingHash string                    `json:"starting_tree_sha256"`
	FinalHash    string                    `json:"final_tree_sha256"`
	ReceiptPath  string                    `json:"receipt_path,omitempty"`
	ReceiptHash  string                    `json:"receipt_sha256,omitempty"`
}

type RealisticNextBinding struct {
	Workspace   RealisticNextWorkspace
	RunFamilyID string
	Provider    RealisticProvider
}

type RealisticNextProviderPreflight interface {
	Preflight(context.Context) error
}

type RealisticNextBatchResult struct {
	Runs            []RealisticNextRunResult `json:"runs"`
	Halted          bool                     `json:"halted"`
	HaltReason      string                   `json:"halt_reason,omitempty"`
	ProviderCalls   int                      `json:"provider_calls"`
	InputTokens     int64                    `json:"input_tokens"`
	OutputTokens    int64                    `json:"output_tokens"`
	ReasoningTokens int64                    `json:"reasoning_tokens"`
	CostMicros      int64                    `json:"cost_micros"`
}

func PrepareRealisticNextR05V1(parent string) (RealisticNextWorkspace, error) {
	if !filepath.IsAbs(parent) {
		return RealisticNextWorkspace{}, fmt.Errorf("R05 workspace parent must be absolute")
	}
	fixture, err := CurrentR05V1SyntheticFixture()
	if err != nil {
		return RealisticNextWorkspace{}, err
	}
	root, err := CopyR05V1SyntheticFixture(parent)
	if err != nil {
		return RealisticNextWorkspace{}, err
	}
	initial, err := CopyR05V1SyntheticFixture(parent)
	if err != nil {
		return RealisticNextWorkspace{}, err
	}
	hash, err := RealisticFixtureTreeSHA256(os.DirFS(root), ".")
	if err != nil {
		return RealisticNextWorkspace{}, err
	}
	return RealisticNextWorkspace{CaseID: fixture.CaseID, CaseVersion: fixture.CaseVersion, Root: root, InitialCopy: initial, StartingHash: hash, EditablePaths: append([]string(nil), fixture.EditablePaths...), AllowedChecks: append([]string(nil), fixture.AllowlistedChecks...), r05: &R05V1HarnessEvidence{}}, nil
}

func PrepareRealisticNextR06V1(parent string) (RealisticNextWorkspace, error) {
	workspace, err := PrepareR06V1Workspace(parent)
	if err != nil {
		return RealisticNextWorkspace{}, err
	}
	checks := []string{R06V1ContractCheck, R06V1ReleaseCRM, R06V1ReleaseLinkHub, R06V1ReleaseCanopy, R06V1AuthenticatedVerify, R06V1Rollback, R06V1VerifyBaseline}
	return RealisticNextWorkspace{CaseID: R06V1CaseID, CaseVersion: R06V1CaseVersion, Root: workspace.Root, InitialCopy: workspace.InitialCopy, StartingHash: workspace.StartingTreeSHA256, EditablePaths: append([]string(nil), workspace.EditablePaths...), AllowedChecks: checks, r06: &workspace}, nil
}

func RunRealisticNextBatch(ctx context.Context, bindings []RealisticNextBinding, limits RealisticNextLimits, receiptRoot string, checkpoint RealisticControllerCheckpoint) (RealisticNextBatchResult, error) {
	var batch RealisticNextBatchResult
	for _, binding := range bindings {
		if realisticNextBatchBudgetExhausted(limits, batch) {
			batch.Halted, batch.HaltReason = true, "budget_breach"
			break
		}
		runLimits := realisticNextRemainingLimits(limits, batch)
		run, err := RunRealisticNextCandidate(ctx, binding.Workspace, binding.RunFamilyID, binding.Provider, runLimits, checkpoint)
		if err != nil {
			return batch, err
		}
		if receiptRoot != "" {
			path, hash, err := SealRealisticNextReceipt(receiptRoot, run)
			if err != nil {
				return batch, fmt.Errorf("%w: %v", ErrRealisticEvidenceIntegrity, err)
			}
			run.ReceiptPath, run.ReceiptHash = path, hash
		}
		batch.Runs = append(batch.Runs, run)
		batch.ProviderCalls += run.Controller.ProviderCalls
		batch.InputTokens += run.Controller.InputTokens
		batch.OutputTokens += run.Controller.OutputTokens
		batch.ReasoningTokens += run.Controller.ReasoningTokens
		batch.CostMicros += run.Controller.CostMicros
		if run.GlobalHalt {
			batch.Halted, batch.HaltReason = true, run.HaltReason
			break
		}
	}
	return batch, nil
}

func RunRealisticNextCandidate(ctx context.Context, workspace RealisticNextWorkspace, runFamilyID string, provider RealisticProvider, limits RealisticNextLimits, checkpoint RealisticControllerCheckpoint) (RealisticNextRunResult, error) {
	run := RealisticNextRunResult{Controller: RealisticControllerResult{CaseID: workspace.CaseID, RunFamilyID: runFamilyID, Attempt: 1}, StartingHash: workspace.StartingHash}
	if provider == nil || checkpoint == nil || strings.TrimSpace(runFamilyID) == "" || workspace.CaseID == "" || workspace.Root == "" {
		return run, fmt.Errorf("realistic next run identity, provider, checkpoint, or workspace missing")
	}
	if limits.MaxSteps <= 0 || limits.MaxSteps > 64 {
		return run, fmt.Errorf("realistic next max steps must be within 1..64")
	}
	if verdict := realisticNextEvaluate(workspace); verdict.Verdict == RealisticNextFixtureInvalid {
		run.Controller.TerminalState, run.Controller.FailureClass = "invalid", "evidence_integrity_failure"
		run.Controller.FinalSummary = strings.Join(verdict.Reasons, ";")
		realisticNextCloseZeroCallAccounting(&run.Controller)
		run.GlobalHalt, run.HaltReason = true, "evidence_integrity_failure"
		if err := persistRealisticControllerCheckpoint(checkpoint, run.Controller); err != nil {
			return run, err
		}
		finalHash, hashErr := RealisticFixtureTreeSHA256(os.DirFS(workspace.Root), ".")
		run.FinalHash = finalHash
		return run, hashErr
	}
	if preflight, ok := provider.(RealisticNextProviderPreflight); ok {
		if preflightErr := preflight.Preflight(ctx); preflightErr != nil {
			realisticNextClassifyProviderError(&run, RealisticProviderResponse{}, preflightErr)
			if err := persistRealisticControllerCheckpoint(checkpoint, run.Controller); err != nil {
				return run, err
			}
			finalHash, hashErr := RealisticFixtureTreeSHA256(os.DirFS(workspace.Root), ".")
			run.FinalHash = finalHash
			return run, hashErr
		}
	}
	task, err := os.ReadFile(filepath.Join(workspace.Root, "TASK.md"))
	if err != nil {
		return run, err
	}
	paths, err := realisticNextPaths(workspace.Root)
	if err != nil {
		return run, err
	}
	for turn := 1; turn <= limits.MaxSteps; turn++ {
		request := RealisticProviderRequest{ProviderCallID: fmt.Sprintf("%s-%02d", runFamilyID, turn), ProviderTurn: turn, CaseID: workspace.CaseID, CaseVersion: workspace.CaseVersion, RunFamilyID: runFamilyID, Attempt: 1, Task: string(task), WorkspacePaths: paths, PreviousResults: append([]RealisticToolResult(nil), run.Controller.ToolResults...), AllowedActions: []string{"read", "search", "edit", "run_check", "finish"}, AllowedChecks: append([]string(nil), workspace.AllowedChecks...), MaxSteps: limits.MaxSteps}
		requestRaw, _ := json.Marshal(request)
		requestHash := sha256.Sum256(requestRaw)
		run.Controller.PendingProviderCall = &RealisticProviderCallCheckpoint{ProviderCallID: request.ProviderCallID, ProviderTurn: turn, RequestSHA256: hex.EncodeToString(requestHash[:])}
		if err := persistRealisticControllerCheckpoint(checkpoint, run.Controller); err != nil {
			return run, err
		}
		response, callErr := provider.Next(ctx, request)
		run.Controller.ProviderCalls++
		if callErr != nil {
			realisticNextClassifyProviderError(&run, response, callErr)
			if err := persistRealisticControllerCheckpoint(checkpoint, run.Controller); err != nil {
				return run, err
			}
			break
		}
		if err := realisticNextRecordResponse(&run, response); err != nil {
			run.Controller.TerminalState, run.Controller.FailureClass = "invalid", "evidence_integrity_failure"
			run.GlobalHalt, run.HaltReason = true, "evidence_integrity_failure"
			if err := persistRealisticControllerCheckpoint(checkpoint, run.Controller); err != nil {
				return run, err
			}
			break
		}
		if realisticNextOverBudget(run.Controller, limits) {
			run.Controller.TerminalState, run.Controller.FailureClass = "invalid", "budget_breach"
			run.GlobalHalt, run.HaltReason = true, "budget_breach"
			if err := persistRealisticControllerCheckpoint(checkpoint, run.Controller); err != nil {
				return run, err
			}
			break
		}
		if response.ProtocolFailure != "" {
			run.Controller.TerminalState, run.Controller.FailureClass = "failed", "response_or_tool_protocol_failure"
			if err := persistRealisticControllerCheckpoint(checkpoint, run.Controller); err != nil {
				return run, err
			}
			break
		}
		action := response.Action
		run.Controller.PendingAction = &action
		if err := persistRealisticControllerCheckpoint(checkpoint, run.Controller); err != nil {
			return run, err
		}
		if action.Kind == "finish" {
			if strings.TrimSpace(action.Summary) == "" {
				run.Controller.TerminalState, run.Controller.FailureClass = "failed", "response_or_tool_protocol_failure"
			} else {
				verdict := realisticNextEvaluate(workspace)
				switch verdict.Verdict {
				case RealisticNextPass:
					run.Controller.TerminalState, run.Controller.FinalSummary = "passed", action.Summary
				case RealisticNextFixtureInvalid:
					run.Controller.TerminalState, run.Controller.FailureClass = "invalid", "evidence_integrity_failure"
					run.Controller.FinalSummary = strings.Join(verdict.Reasons, ";")
					run.GlobalHalt, run.HaltReason = true, "evidence_integrity_failure"
				default:
					run.Controller.TerminalState, run.Controller.FailureClass = "failed", "competence_failure"
					run.Controller.FinalSummary = strings.Join(verdict.Reasons, ";")
				}
			}
			run.Controller.PendingAction = nil
			if err := persistRealisticControllerCheckpoint(checkpoint, run.Controller); err != nil {
				return run, err
			}
			break
		}
		started := time.Now().UTC()
		output, touched, arguments, actionErr, protocol := realisticNextExecute(ctx, workspace, action)
		appendRealisticToolResultAt(&run.Controller, action.Kind, output, actionErr, touched, arguments, started)
		run.Controller.PendingAction = nil
		if protocol {
			run.Controller.TerminalState, run.Controller.FailureClass = "failed", "response_or_tool_protocol_failure"
		}
		if err := persistRealisticControllerCheckpoint(checkpoint, run.Controller); err != nil {
			return run, err
		}
		if protocol {
			break
		}
	}
	if run.Controller.TerminalState == "" {
		run.Controller.TerminalState, run.Controller.FailureClass = "failed", "response_or_tool_protocol_failure"
		run.Controller.FinalSummary = "bounded step ceiling reached"
		if err := persistRealisticControllerCheckpoint(checkpoint, run.Controller); err != nil {
			return run, err
		}
	}
	run.FinalHash, err = RealisticFixtureTreeSHA256(os.DirFS(workspace.Root), ".")
	if err != nil {
		return run, err
	}
	return run, nil
}

func realisticNextRecordResponse(run *RealisticNextRunResult, response RealisticProviderResponse) error {
	run.Controller.PendingProviderCall = nil
	if len(response.RawReceipt) == 0 {
		return ErrRealisticEvidenceIntegrity
	}
	hash := sha256.Sum256(response.RawReceipt)
	actual := hex.EncodeToString(hash[:])
	if response.RawReceiptSHA256 != "" && response.RawReceiptSHA256 != actual {
		return ErrRealisticEvidenceIntegrity
	}
	receipts := append(append([][]byte(nil), run.Controller.ProviderReceipts...), append([]byte(nil), response.RawReceipt...))
	accounting, err := recomputeRealisticProviderAccounting(receipts)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRealisticEvidenceIntegrity, err)
	}
	if accounting.ProviderCalls != run.Controller.ProviderCalls {
		return fmt.Errorf("%w: receipt count does not close provider calls", ErrRealisticEvidenceIntegrity)
	}
	run.Controller.ProviderReceiptHashes = accounting.ReceiptHashes
	run.Controller.ProviderReceipts = receipts
	run.Controller.InputTokens = accounting.InputTokens
	run.Controller.OutputTokens = accounting.OutputTokens
	run.Controller.ReasoningTokens = accounting.ReasoningTokens
	run.Controller.CostMicros = accounting.CostMicros
	run.Controller.ProviderAccountingStatus = "exact"
	run.Controller.ProviderTokenAccountingStatus = "exact"
	run.Controller.ProviderCostAccountingStatus = "exact"
	return nil
}

func realisticNextClassifyProviderError(run *RealisticNextRunResult, response RealisticProviderResponse, err error) {
	run.Controller.PendingProviderCall = nil
	switch {
	case errors.Is(err, ErrRealisticCredentialFailure):
		if run.Controller.ProviderCalls > 0 {
			run.Controller.ProviderCalls--
		}
		realisticNextCloseZeroCallAccounting(&run.Controller)
		run.Controller.TerminalState, run.Controller.FailureClass, run.GlobalHalt, run.HaltReason = "invalid", "credential_failure", true, "credential_failure"
	case errors.Is(err, ErrRealisticBudgetBreach):
		if run.Controller.ProviderCalls > 0 {
			run.Controller.ProviderCalls--
		}
		realisticNextCloseZeroCallAccounting(&run.Controller)
		run.Controller.TerminalState, run.Controller.FailureClass, run.GlobalHalt, run.HaltReason = "invalid", "budget_breach", true, "budget_breach"
	case errors.Is(err, ErrRealisticNextUnsafeExecution):
		run.Controller.TerminalState, run.Controller.FailureClass, run.GlobalHalt, run.HaltReason = "invalid", "unsafe_execution", true, "unsafe_execution"
	case errors.Is(err, ErrRealisticEvidenceIntegrity):
		run.Controller.TerminalState, run.Controller.FailureClass, run.GlobalHalt, run.HaltReason = "invalid", "evidence_integrity_failure", true, "evidence_integrity_failure"
	case errors.Is(err, ErrRealisticProviderOutcomeAmbiguous):
		recordRealisticAmbiguousTransport(&run.Controller, response)
		run.Controller.TerminalState, run.Controller.FailureClass, run.GlobalHalt, run.HaltReason = "invalid", "ambiguous_transport", true, "ambiguous_transport"
	case errors.Is(err, ErrRealisticProviderPreflight):
		if run.Controller.ProviderCalls > 0 {
			run.Controller.ProviderCalls--
		}
		realisticNextCloseZeroCallAccounting(&run.Controller)
		run.Controller.TerminalState, run.Controller.FailureClass = "invalid", "provider_preflight_failure"
	default:
		recordRealisticAmbiguousTransport(&run.Controller, response)
		run.Controller.TerminalState, run.Controller.FailureClass, run.GlobalHalt, run.HaltReason = "invalid", "ambiguous_transport", true, "ambiguous_transport"
	}
}

func realisticNextCloseZeroCallAccounting(result *RealisticControllerResult) {
	result.ProviderAccountingStatus = "exact"
	result.ProviderTokenAccountingStatus = "exact"
	result.ProviderCostAccountingStatus = "exact"
}

func realisticNextOverBudget(result RealisticControllerResult, limits RealisticNextLimits) bool {
	return limits.MaxInputTokens > 0 && result.InputTokens > limits.MaxInputTokens || limits.MaxOutputTokens > 0 && result.OutputTokens > limits.MaxOutputTokens || limits.MaxReasoningTokens > 0 && result.ReasoningTokens > limits.MaxReasoningTokens || limits.MaxCostMicros > 0 && result.CostMicros > limits.MaxCostMicros
}

func realisticNextRemainingLimits(limits RealisticNextLimits, batch RealisticNextBatchResult) RealisticNextLimits {
	remaining := limits
	if remaining.MaxInputTokens > 0 {
		remaining.MaxInputTokens -= batch.InputTokens
	}
	if remaining.MaxOutputTokens > 0 {
		remaining.MaxOutputTokens -= batch.OutputTokens
	}
	if remaining.MaxReasoningTokens > 0 {
		remaining.MaxReasoningTokens -= batch.ReasoningTokens
	}
	if remaining.MaxCostMicros > 0 {
		remaining.MaxCostMicros -= batch.CostMicros
	}
	return remaining
}

func realisticNextBatchBudgetExhausted(limits RealisticNextLimits, batch RealisticNextBatchResult) bool {
	return limits.MaxInputTokens > 0 && batch.InputTokens >= limits.MaxInputTokens ||
		limits.MaxOutputTokens > 0 && batch.OutputTokens >= limits.MaxOutputTokens ||
		limits.MaxReasoningTokens > 0 && batch.ReasoningTokens >= limits.MaxReasoningTokens ||
		limits.MaxCostMicros > 0 && batch.CostMicros >= limits.MaxCostMicros
}

func realisticNextExecute(ctx context.Context, workspace RealisticNextWorkspace, action RealisticCandidateAction) ([]byte, []string, []string, error, bool) {
	switch action.Kind {
	case "read":
		path, err := realisticNextResolve(workspace.Root, action.Path)
		if err != nil {
			return nil, nil, []string{action.Path}, err, true
		}
		raw, err := os.ReadFile(path)
		return raw, nil, []string{action.Path}, err, false
	case "search":
		if strings.TrimSpace(action.Query) == "" {
			return nil, nil, nil, fmt.Errorf("search query missing"), true
		}
		rows, err := realisticNextSearch(workspace.Root, action.Query)
		return []byte(strings.Join(rows, "\n")), nil, []string{action.Query}, err, false
	case "edit":
		rel := filepath.ToSlash(filepath.Clean(action.Path))
		if !contains(workspace.EditablePaths, rel) {
			return nil, nil, []string{action.Path}, fmt.Errorf("edit path is not allowlisted"), true
		}
		if workspace.r06 != nil {
			if err := workspace.r06.Edit(rel, []byte(action.Contents)); err != nil {
				return nil, nil, []string{rel}, err, true
			}
		} else {
			path, err := realisticNextResolve(workspace.Root, rel)
			if err != nil {
				return nil, nil, []string{rel}, err, true
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return nil, nil, []string{rel}, err, false
			}
			if err := os.WriteFile(path, []byte(action.Contents), 0o600); err != nil {
				return nil, nil, []string{rel}, err, false
			}
		}
		return []byte("edited"), []string{rel}, []string{rel}, nil, false
	case "run_check":
		if !contains(workspace.AllowedChecks, action.Check) {
			return nil, nil, []string{action.Check}, fmt.Errorf("check is not allowlisted"), true
		}
		touched := realisticNextCheckTouchedPaths(workspace.CaseID, action.Check)
		if workspace.r06 != nil {
			out, err := workspace.r06.RunCheck(action.Check)
			return out, touched, []string{action.Check}, err, false
		}
		out, err := r05V1RunHarnessCheck(workspace.Root, action.Check, workspace.r05)
		return out, touched, []string{action.Check}, err, false
	default:
		return nil, nil, []string{action.Kind}, fmt.Errorf("action kind is not allowlisted"), true
	}
}

func realisticNextCheckTouchedPaths(caseID, check string) []string {
	if caseID == "R05" && check == R05V1SimulateCheck {
		return []string{"state/final-state.json"}
	}
	if caseID != "R06" {
		return nil
	}
	switch check {
	case R06V1ReleaseCRM, R06V1ReleaseLinkHub, R06V1ReleaseCanopy, R06V1AuthenticatedVerify, R06V1VerifyBaseline:
		return []string{"release/state.json"}
	case R06V1Rollback:
		return []string{"release/state.json", "release/receipt.json"}
	default:
		return nil
	}
}

func realisticNextEvaluate(workspace RealisticNextWorkspace) RealisticNextCalibrationResult {
	if workspace.CaseID == "R05" {
		return EvaluateRealisticNextR05(workspace.InitialCopy, workspace.Root, workspace.r05)
	}
	if workspace.CaseID == "R06" && workspace.r06 != nil {
		return EvaluateRealisticNextR06(*workspace.r06)
	}
	return realisticNextResult(workspace.CaseID, RealisticNextFixtureInvalid, "unsupported_case")
}

func realisticNextResolve(root, relative string) (string, error) {
	if !safeFixtureRelativePath(relative) {
		return "", fmt.Errorf("unsafe workspace path")
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(filepath.Join(absoluteRoot, filepath.FromSlash(relative)))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absoluteRoot, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("workspace path escapes root")
	}
	return target, nil
}

func realisticNextPaths(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(paths)
	return paths, err
}

func realisticNextSearch(root, query string) ([]string, error) {
	paths, err := realisticNextPaths(root)
	if err != nil {
		return nil, err
	}
	var rows []string
	for _, path := range paths {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return nil, err
		}
		for number, line := range strings.Split(string(raw), "\n") {
			if strings.Contains(line, query) {
				rows = append(rows, fmt.Sprintf("%s:%d:%s", path, number+1, line))
			}
		}
	}
	return rows, nil
}

func SealRealisticNextReceipt(root string, run RealisticNextRunResult) (string, string, error) {
	if !filepath.IsAbs(root) || strings.TrimSpace(run.Controller.RunFamilyID) == "" || run.Controller.TerminalState == "" {
		return "", "", fmt.Errorf("receipt root or completed run missing")
	}
	// Use a stable case/run digest without exposing provider-supplied path text.
	digest := sha256.Sum256([]byte(run.Controller.CaseID + "\x00" + run.Controller.RunFamilyID))
	directory := filepath.Join(root, hex.EncodeToString(digest[:]))
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", "", err
	}
	path := filepath.Join(directory, "receipt.json")
	copyForSeal := run
	copyForSeal.ReceiptPath, copyForSeal.ReceiptHash = "", ""
	raw, err := json.MarshalIndent(struct {
		Schema string                 `json:"schema"`
		Run    RealisticNextRunResult `json:"run"`
	}{Schema: RealisticNextHarnessVersion, Run: copyForSeal}, "", "  ")
	if err != nil {
		return "", "", err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", "", err
	}
	written := false
	defer func() {
		if !written {
			_ = os.Remove(path)
		}
	}()
	if _, err := file.Write(raw); err != nil {
		_ = file.Close()
		return "", "", err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return "", "", err
	}
	if err := file.Close(); err != nil {
		return "", "", err
	}
	written = true
	hash := sha256.Sum256(raw)
	return path, hex.EncodeToString(hash[:]), nil
}
