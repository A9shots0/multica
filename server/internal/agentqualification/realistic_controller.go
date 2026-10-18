package agentqualification

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const RealisticControllerVersion = "realistic-controller.v1"

var ErrRealisticBrowserEvidencePending = errors.New("independent Browser evidence is pending")
var ErrRealisticBudgetBreach = errors.New("realistic campaign budget breached")
var ErrRealisticCandidateProtocolFailure = errors.New("realistic candidate response protocol failure")
var ErrRealisticCredentialFailure = errors.New("realistic provider credential failure")
var ErrRealisticProviderPreflight = errors.New("realistic provider preflight failure")
var ErrRealisticEvidenceIntegrity = errors.New("realistic evidence integrity failure")
var ErrRealisticCheckpointDurability = errors.New("realistic controller checkpoint was not durably recorded")
var ErrRealisticProviderOutcomeAmbiguous = errors.New("realistic provider call outcome is ambiguous")

type RealisticCandidateAction struct {
	Kind     string `json:"kind"`
	Path     string `json:"path,omitempty"`
	Query    string `json:"query,omitempty"`
	Contents string `json:"contents,omitempty"`
	Check    string `json:"check,omitempty"`
	Summary  string `json:"summary,omitempty"`
}

type RealisticProviderRequest struct {
	ProviderCallID  string                `json:"provider_call_id"`
	ProviderTurn    int                   `json:"provider_turn"`
	CaseID          string                `json:"case_id"`
	CaseVersion     string                `json:"case_version"`
	RunFamilyID     string                `json:"run_family_id"`
	Attempt         int                   `json:"attempt"`
	Task            string                `json:"task"`
	WorkspacePaths  []string              `json:"workspace_paths"`
	PreviousResults []RealisticToolResult `json:"previous_results"`
	AllowedActions  []string              `json:"allowed_actions"`
	AllowedChecks   []string              `json:"allowed_checks"`
	MaxSteps        int                   `json:"max_steps"`
}

type RealisticProviderResponse struct {
	Action                RealisticCandidateAction `json:"action"`
	RawReceipt            []byte                   `json:"-"`
	RawReceiptSHA256      string                   `json:"raw_receipt_sha256,omitempty"`
	InputTokens           int64                    `json:"input_tokens"`
	OutputTokens          int64                    `json:"output_tokens"`
	ReasoningTokens       int64                    `json:"reasoning_tokens"`
	CostMicros            int64                    `json:"cost_micros"`
	ProtocolFailure       string                   `json:"protocol_failure,omitempty"`
	AccountingStatus      string                   `json:"accounting_status,omitempty"`
	TokenAccountingStatus string                   `json:"token_accounting_status,omitempty"`
	CostAccountingStatus  string                   `json:"cost_accounting_status,omitempty"`
}

type RealisticToolResult struct {
	Sequence   int    `json:"sequence"`
	ActionKind string `json:"action_kind"`
	Success    bool   `json:"success"`
	Output     string `json:"output"`
	OutputHash string `json:"output_hash"`
}

type RealisticProvider interface {
	Next(context.Context, RealisticProviderRequest) (RealisticProviderResponse, error)
}

type RealisticBrowserCollector interface {
	Capture(context.Context, string, string) ([]RealisticBrowserProof, error)
}

// RealisticControllerCheckpoint is supplied by the harness, which owns the
// durable run-state path. A successful return means the complete controller
// state has been atomically committed before execution may continue.
type RealisticControllerCheckpoint func(RealisticControllerResult) error

type RealisticPaidTurnCheckpoint struct {
	Action          RealisticCandidateAction `json:"action"`
	ProtocolFailure string                   `json:"protocol_failure,omitempty"`
}

type RealisticProviderCallCheckpoint struct {
	ProviderCallID string `json:"provider_call_id"`
	ProviderTurn   int    `json:"provider_turn"`
	RequestSHA256  string `json:"request_sha256"`
}

type RealisticControllerResult struct {
	CaseID                        string
	RunFamilyID                   string
	Attempt                       int
	TerminalState                 string
	FailureClass                  string
	ToolEvents                    []RealisticToolEvent
	ToolResults                   []RealisticToolResult
	BrowserProofs                 []RealisticBrowserProof
	ProviderCalls                 int
	InputTokens                   int64
	OutputTokens                  int64
	ReasoningTokens               int64
	CostMicros                    int64
	FinalSummary                  string
	ProviderReceiptHashes         []string
	ProviderReceipts              [][]byte
	PendingAction                 *RealisticCandidateAction
	PendingPaidTurn               *RealisticPaidTurnCheckpoint
	PendingProviderCall           *RealisticProviderCallCheckpoint
	ProviderAccountingStatus      string
	ProviderTokenAccountingStatus string
	ProviderCostAccountingStatus  string
	AmbiguousProviderCalls        int
	AttemptHistory                []RealisticAttemptCheckpoint
}

type RealisticAttemptCheckpoint struct {
	Attempt             int    `json:"attempt"`
	TerminalState       string `json:"terminal_state"`
	ProviderCalls       int    `json:"provider_calls_cumulative"`
	TotalTokens         int64  `json:"total_tokens_cumulative"`
	CostMicros          int64  `json:"cost_micros_cumulative"`
	WorkspaceTreeSHA256 string `json:"workspace_tree_sha256"`
}

// RunRealisticCandidate is the provider entrypoint. It cannot run while any
// group, fixture, matrix, runner, verifier, receipt, or policy remains pending.
func RunRealisticCandidate(ctx context.Context, workspace RealisticCaseWorkspace, runFamilyID string, attempt int, provider RealisticProvider, browser RealisticBrowserCollector, checkpoint RealisticControllerCheckpoint) (RealisticControllerResult, error) {
	if err := RequireCompleteRealisticFreeze(); err != nil {
		return RealisticControllerResult{}, err
	}
	return runRealisticCandidateFrozen(ctx, workspace, runFamilyID, attempt, provider, browser, checkpoint)
}

func runRealisticCandidateFrozen(ctx context.Context, workspace RealisticCaseWorkspace, runFamilyID string, attempt int, provider RealisticProvider, browser RealisticBrowserCollector, checkpoints ...RealisticControllerCheckpoint) (RealisticControllerResult, error) {
	result := RealisticControllerResult{CaseID: workspace.CaseID, RunFamilyID: runFamilyID, Attempt: attempt}
	checkpoint := acceptInMemoryRealisticCheckpoint
	if len(checkpoints) > 0 {
		checkpoint = checkpoints[0]
	}
	if provider == nil || checkpoint == nil || strings.TrimSpace(runFamilyID) == "" || attempt < 1 {
		return result, fmt.Errorf("realistic run identity or provider missing")
	}
	if workspace.CaseID == "R04" && attempt == 1 {
		treeHash, err := RealisticFixtureTreeSHA256(os.DirFS(workspace.Root), ".")
		if err != nil {
			return result, err
		}
		result.TerminalState = "interrupted"
		result.AttemptHistory = append(result.AttemptHistory, RealisticAttemptCheckpoint{Attempt: 1, TerminalState: "interrupted", WorkspaceTreeSHA256: treeHash})
		if err := persistRealisticControllerCheckpoint(checkpoint, result); err != nil {
			return result, err
		}
		return result, nil
	}
	return continueRealisticCandidate(ctx, workspace, runFamilyID, attempt, provider, browser, checkpoint, result)
}

// ResumeRealisticCandidate resumes only a sealed Browser handoff in the same
// attempt or the single forced R04 interruption in attempt two. Prior usage,
// tool results, and raw provider receipt hashes remain cumulative.
func ResumeRealisticCandidate(ctx context.Context, workspace RealisticCaseWorkspace, prior RealisticControllerResult, provider RealisticProvider, browser RealisticBrowserCollector, checkpoint RealisticControllerCheckpoint) (RealisticControllerResult, error) {
	if err := RequireCompleteRealisticFreeze(); err != nil {
		return RealisticControllerResult{}, err
	}
	return resumeRealisticCandidateFrozen(ctx, workspace, prior, provider, browser, checkpoint)
}

func resumeRealisticCandidateFrozen(ctx context.Context, workspace RealisticCaseWorkspace, prior RealisticControllerResult, provider RealisticProvider, browser RealisticBrowserCollector, checkpoints ...RealisticControllerCheckpoint) (RealisticControllerResult, error) {
	checkpoint := acceptInMemoryRealisticCheckpoint
	if len(checkpoints) > 0 {
		checkpoint = checkpoints[0]
	}
	if prior.CaseID != workspace.CaseID || prior.RunFamilyID == "" || provider == nil || checkpoint == nil {
		return prior, fmt.Errorf("realistic continuation identity mismatch")
	}
	attempt := prior.Attempt
	switch prior.TerminalState {
	case "awaiting_browser_evidence":
		if workspace.CaseID != "R02" || prior.PendingAction == nil || prior.PendingAction.Kind != "browser" || browser == nil {
			return prior, fmt.Errorf("Browser continuation is incomplete")
		}
	case "interrupted":
		if workspace.CaseID != "R04" || prior.Attempt != 1 || prior.PendingAction != nil {
			return prior, fmt.Errorf("interruption continuation is invalid")
		}
		attempt = 2
	case "":
		if prior.PendingProviderCall != nil {
			prior.TerminalState, prior.FailureClass = "invalid", "provider_transport_failure"
			prior.ProviderAccountingStatus = "ambiguous"
			prior.ProviderTokenAccountingStatus = "unknown"
			prior.ProviderCostAccountingStatus = "unknown"
			prior.AmbiguousProviderCalls++
			if err := persistRealisticControllerCheckpoint(checkpoint, prior); err != nil {
				return prior, err
			}
			return prior, ErrRealisticProviderOutcomeAmbiguous
		}
		if prior.ProviderCalls < 1 {
			return prior, fmt.Errorf("nonterminal realistic continuation has no recorded provider progress")
		}
	default:
		return prior, fmt.Errorf("terminal realistic result cannot resume")
	}
	prior.Attempt = attempt
	prior.TerminalState = ""
	return continueRealisticCandidate(ctx, workspace, prior.RunFamilyID, attempt, provider, browser, checkpoint, prior)
}

func continueRealisticCandidate(ctx context.Context, workspace RealisticCaseWorkspace, runFamilyID string, attempt int, provider RealisticProvider, browser RealisticBrowserCollector, checkpoint RealisticControllerCheckpoint, result RealisticControllerResult) (RealisticControllerResult, error) {
	if workspace.CaseVersion == "" {
		group, groupErr := CurrentRealisticWorkflowGroup()
		if groupErr != nil {
			return result, groupErr
		}
		for _, candidateCase := range group.Cases {
			if candidateCase.CaseID == workspace.CaseID {
				workspace.CaseVersion = candidateCase.CaseVersion
				break
			}
		}
	}
	taskRaw, err := workspace.Read("TASK.md")
	if err != nil {
		return result, err
	}
	workspacePaths, err := workspace.Paths()
	if err != nil {
		return result, err
	}
	maxSteps := 15
	if workspace.CaseID == "R02" {
		// R02 can legitimately require a failed Browser pass, one correction,
		// a second Browser pass, an evidence write, and a final receipt.
		maxSteps = 18
	}
	if result.PendingAction != nil {
		proofs, captureErr := browser.Capture(ctx, workspace.Root, "report/index.html")
		if captureErr != nil {
			if errors.Is(captureErr, ErrRealisticBrowserEvidencePending) {
				result.TerminalState = "awaiting_browser_evidence"
				return result, nil
			}
			result.TerminalState, result.FailureClass = "invalid", "evidence_integrity_failure"
			return result, captureErr
		}
		result.PendingAction = nil
		result.BrowserProofs = proofs
		appendRealisticToolResult(&result, "browser", realisticBrowserToolOutput(proofs), nil, nil, nil)
		if err := persistRealisticControllerCheckpoint(checkpoint, result); err != nil {
			return result, err
		}
	}
	if result.PendingPaidTurn != nil {
		if done, err := applyRealisticPaidTurn(ctx, workspace, attempt, browser, checkpoint, &result); done || err != nil {
			return result, err
		}
	}
	for step := result.ProviderCalls + 1; step <= maxSteps; step++ {
		providerCallID := fmt.Sprintf("%s:attempt-%d:turn-%d", runFamilyID, attempt, step)
		request := RealisticProviderRequest{
			ProviderCallID: providerCallID, ProviderTurn: step,
			CaseID: workspace.CaseID, CaseVersion: workspace.CaseVersion, RunFamilyID: runFamilyID, Attempt: attempt, Task: string(taskRaw), WorkspacePaths: append([]string(nil), workspacePaths...),
			PreviousResults: append([]RealisticToolResult(nil), result.ToolResults...),
			AllowedActions:  []string{"read", "search", "edit", "run_check", "browser", "checkpoint_and_interrupt", "finish"}, MaxSteps: maxSteps,
			AllowedChecks: append([]string(nil), workspace.Fixture.AllowlistedChecks...),
		}
		requestRaw, marshalErr := json.Marshal(request)
		if marshalErr != nil {
			return result, marshalErr
		}
		requestHash := sha256.Sum256(requestRaw)
		result.ProviderCalls++
		result.ProviderAccountingStatus = "in_flight"
		result.ProviderTokenAccountingStatus = "in_flight"
		result.ProviderCostAccountingStatus = "in_flight"
		result.PendingProviderCall = &RealisticProviderCallCheckpoint{ProviderCallID: providerCallID, ProviderTurn: step, RequestSHA256: hex.EncodeToString(requestHash[:])}
		if checkpointErr := persistRealisticControllerCheckpoint(checkpoint, result); checkpointErr != nil {
			return result, checkpointErr
		}
		response, err := provider.Next(ctx, request)
		if err != nil {
			if errors.Is(err, ErrRealisticCredentialFailure) {
				result.ProviderCalls--
				result.PendingProviderCall = nil
				result.ProviderAccountingStatus = "exact"
				result.ProviderTokenAccountingStatus = "exact"
				result.ProviderCostAccountingStatus = "exact"
				result.TerminalState, result.FailureClass = "invalid", "credential_failure"
				if checkpointErr := persistRealisticControllerCheckpoint(checkpoint, result); checkpointErr != nil {
					return result, checkpointErr
				}
				return result, err
			}
			if errors.Is(err, ErrRealisticProviderPreflight) {
				result.ProviderCalls--
				result.PendingProviderCall = nil
				result.ProviderAccountingStatus = "exact"
				result.ProviderTokenAccountingStatus = "exact"
				result.ProviderCostAccountingStatus = "exact"
				result.TerminalState, result.FailureClass = "invalid", "test_invalidity"
				if checkpointErr := persistRealisticControllerCheckpoint(checkpoint, result); checkpointErr != nil {
					return result, checkpointErr
				}
				return result, err
			}
			result.TerminalState, result.FailureClass = "invalid", "provider_transport_failure"
			if errors.Is(err, ErrRealisticBudgetBreach) {
				result.ProviderCalls--
				result.PendingProviderCall = nil
				result.ProviderAccountingStatus = "exact"
				result.ProviderTokenAccountingStatus = "exact"
				result.ProviderCostAccountingStatus = "exact"
				result.FailureClass = "budget_breach"
			} else {
				recordRealisticAmbiguousTransport(&result, response)
				if errors.Is(err, ErrRealisticEvidenceIntegrity) {
					result.FailureClass = "evidence_integrity_failure"
				}
			}
			if checkpointErr := persistRealisticControllerCheckpoint(checkpoint, result); checkpointErr != nil {
				return result, checkpointErr
			}
			return result, err
		}
		result.PendingProviderCall = nil
		result.ProviderAccountingStatus = "exact"
		result.ProviderTokenAccountingStatus = "exact"
		result.ProviderCostAccountingStatus = "exact"
		if len(response.RawReceipt) == 0 {
			result.TerminalState, result.FailureClass = "invalid", "evidence_integrity_failure"
			if checkpointErr := persistRealisticControllerCheckpoint(checkpoint, result); checkpointErr != nil {
				return result, checkpointErr
			}
			return result, fmt.Errorf("provider receipt is missing")
		}
		rawReceiptHash := sha256.Sum256(response.RawReceipt)
		result.ProviderReceiptHashes = append(result.ProviderReceiptHashes, hex.EncodeToString(rawReceiptHash[:]))
		result.ProviderReceipts = append(result.ProviderReceipts, append([]byte(nil), response.RawReceipt...))
		if response.InputTokens < 0 || response.OutputTokens < 0 || response.ReasoningTokens < 0 || response.CostMicros < 0 {
			result.TerminalState, result.FailureClass = "invalid", "evidence_integrity_failure"
			if checkpointErr := persistRealisticControllerCheckpoint(checkpoint, result); checkpointErr != nil {
				return result, checkpointErr
			}
			return result, fmt.Errorf("provider usage receipt is invalid")
		}
		result.InputTokens += response.InputTokens
		result.OutputTokens += response.OutputTokens
		result.ReasoningTokens += response.ReasoningTokens
		result.CostMicros += response.CostMicros
		result.PendingPaidTurn = &RealisticPaidTurnCheckpoint{Action: response.Action, ProtocolFailure: response.ProtocolFailure}
		if err := persistRealisticControllerCheckpoint(checkpoint, result); err != nil {
			return result, err
		}
		if result.CostMicros > 5000000 || result.OutputTokens+result.ReasoningTokens > 20000 {
			result.TerminalState, result.FailureClass = "invalid", "budget_breach"
			result.PendingPaidTurn = nil
			if checkpointErr := persistRealisticControllerCheckpoint(checkpoint, result); checkpointErr != nil {
				return result, checkpointErr
			}
			return result, fmt.Errorf("realistic run budget breached")
		}
		if result.InputTokens+result.OutputTokens+result.ReasoningTokens > 75000 {
			result.TerminalState, result.FailureClass = "failed", "response_or_tool_protocol_failure"
			result.PendingPaidTurn = nil
			if checkpointErr := persistRealisticControllerCheckpoint(checkpoint, result); checkpointErr != nil {
				return result, checkpointErr
			}
			return result, nil
		}
		if done, err := applyRealisticPaidTurn(ctx, workspace, attempt, browser, checkpoint, &result); done || err != nil {
			return result, err
		}
	}
	result.TerminalState, result.FailureClass = "failed", "response_or_tool_protocol_failure"
	if err := persistRealisticControllerCheckpoint(checkpoint, result); err != nil {
		return result, err
	}
	return result, nil
}

func applyRealisticPaidTurn(ctx context.Context, workspace RealisticCaseWorkspace, attempt int, browser RealisticBrowserCollector, checkpoint RealisticControllerCheckpoint, result *RealisticControllerResult) (bool, error) {
	turn := result.PendingPaidTurn
	if turn == nil {
		return false, nil
	}
	if turn.ProtocolFailure != "" {
		result.PendingPaidTurn = nil
		result.TerminalState, result.FailureClass = "failed", "response_or_tool_protocol_failure"
		return true, persistRealisticControllerCheckpoint(checkpoint, *result)
	}
	action := turn.Action
	if action.Kind == "finish" {
		result.PendingPaidTurn = nil
		result.TerminalState, result.FinalSummary = "completed", action.Summary
		treeHash, treeErr := RealisticFixtureTreeSHA256(os.DirFS(workspace.Root), ".")
		if treeErr != nil {
			result.TerminalState, result.FailureClass = "invalid", "evidence_integrity_failure"
			return true, treeErr
		}
		result.AttemptHistory = append(result.AttemptHistory, RealisticAttemptCheckpoint{Attempt: attempt, TerminalState: "completed", ProviderCalls: result.ProviderCalls, TotalTokens: result.InputTokens + result.OutputTokens + result.ReasoningTokens, CostMicros: result.CostMicros, WorkspaceTreeSHA256: treeHash})
		return true, persistRealisticControllerCheckpoint(checkpoint, *result)
	}
	if action.Kind == "checkpoint_and_interrupt" {
		result.PendingPaidTurn = nil
		result.TerminalState, result.FailureClass = "failed", "response_or_tool_protocol_failure"
		return true, persistRealisticControllerCheckpoint(checkpoint, *result)
	}
	started := time.Now().UTC()
	touched := []string{}
	arguments := []string{}
	var output []byte
	var err error
	switch action.Kind {
	case "read":
		output, err = workspace.Read(action.Path)
	case "search":
		var matches []string
		matches, err = workspace.Search(action.Query)
		output = []byte(strings.Join(matches, "\n"))
	case "edit":
		err = workspace.Edit(action.Path, []byte(action.Contents))
		touched = []string{action.Path}
		output = []byte("edited " + action.Path)
	case "run_check":
		actionID := result.RunFamilyID + ":attempt:" + fmt.Sprint(attempt) + ":check:" + action.Check
		output, err = workspace.runCheckWithActionID(action.Check, actionID)
		arguments = strings.Fields(action.Check)
		if workspace.CaseID == "R04" && action.Check == "python resume.py" {
			touched = []string{"state/resume-action.json", "state/step_counts.json", "FINAL-OUTPUT.json"}
		}
	case "browser":
		if workspace.CaseID != "R02" || browser == nil {
			result.PendingPaidTurn = nil
			result.TerminalState, result.FailureClass = "invalid", "test_invalidity"
			return true, fmt.Errorf("Browser collector is unavailable for Browser-required case")
		} else {
			// R02 has one fixture-owned local entrypoint. The model decides when
			// the page is ready; the deterministic controller owns the exact path.
			result.BrowserProofs, err = browser.Capture(ctx, workspace.Root, "report/index.html")
			if errors.Is(err, ErrRealisticBrowserEvidencePending) {
				copy := action
				result.PendingPaidTurn = nil
				result.PendingAction = &copy
				result.TerminalState = "awaiting_browser_evidence"
				return true, persistRealisticControllerCheckpoint(checkpoint, *result)
			}
			if err != nil {
				result.TerminalState, result.FailureClass = "invalid", "evidence_integrity_failure"
				return true, err
			}
			output = realisticBrowserToolOutput(result.BrowserProofs)
		}
	default:
		result.PendingPaidTurn = nil
		result.TerminalState, result.FailureClass = "failed", "response_or_tool_protocol_failure"
		return true, persistRealisticControllerCheckpoint(checkpoint, *result)
	}
	appendRealisticToolResultAt(result, action.Kind, output, err, touched, arguments, started)
	result.PendingPaidTurn = nil
	if checkpointErr := persistRealisticControllerCheckpoint(checkpoint, *result); checkpointErr != nil {
		return true, checkpointErr
	}
	return false, nil
}

func realisticBrowserToolOutput(proofs []RealisticBrowserProof) []byte {
	allUsable := len(proofs) == 3
	for _, proof := range proofs {
		if proof.RenderedTotal != 47 || proof.RenderedCardCount != 4 || proof.HorizontalOverflow {
			allUsable = false
		}
	}
	status := "failed_verification_correct_and_capture_again_before_finishing"
	if allUsable {
		status = "passed_verification_save_exact_proof_hashes"
	}
	raw, err := json.Marshal(map[string]any{"status": status, "proofs": proofs})
	if err != nil {
		return []byte(`{"status":"browser_result_encoding_failed"}`)
	}
	return raw
}

func persistRealisticControllerCheckpoint(checkpoint RealisticControllerCheckpoint, result RealisticControllerResult) error {
	if checkpoint == nil {
		return fmt.Errorf("%w: checkpoint writer missing", ErrRealisticCheckpointDurability)
	}
	if err := checkpoint(result); err != nil {
		return fmt.Errorf("%w: %v", ErrRealisticCheckpointDurability, err)
	}
	return nil
}

func recordRealisticAmbiguousTransport(result *RealisticControllerResult, response RealisticProviderResponse) {
	result.PendingProviderCall = nil
	result.AmbiguousProviderCalls++
	result.ProviderAccountingStatus = response.AccountingStatus
	if result.ProviderAccountingStatus != "partial" {
		result.ProviderAccountingStatus = "ambiguous"
	}
	result.ProviderTokenAccountingStatus = response.TokenAccountingStatus
	if result.ProviderTokenAccountingStatus == "" {
		result.ProviderTokenAccountingStatus = "unknown"
	}
	result.ProviderCostAccountingStatus = response.CostAccountingStatus
	if result.ProviderCostAccountingStatus == "" {
		result.ProviderCostAccountingStatus = "unknown"
	}
	if len(response.RawReceipt) > 0 {
		rawReceiptHash := sha256.Sum256(response.RawReceipt)
		result.ProviderReceiptHashes = append(result.ProviderReceiptHashes, hex.EncodeToString(rawReceiptHash[:]))
		result.ProviderReceipts = append(result.ProviderReceipts, append([]byte(nil), response.RawReceipt...))
	}
	if response.InputTokens >= 0 {
		result.InputTokens += response.InputTokens
	}
	if response.OutputTokens >= 0 {
		result.OutputTokens += response.OutputTokens
	}
	if response.ReasoningTokens >= 0 {
		result.ReasoningTokens += response.ReasoningTokens
	}
	if response.CostMicros >= 0 {
		result.CostMicros += response.CostMicros
	}
}

func acceptInMemoryRealisticCheckpoint(RealisticControllerResult) error { return nil }

func appendRealisticToolResult(result *RealisticControllerResult, actionKind string, output []byte, err error, touched, arguments []string) {
	appendRealisticToolResultAt(result, actionKind, output, err, touched, arguments, time.Now().UTC())
}

func appendRealisticToolResultAt(result *RealisticControllerResult, actionKind string, output []byte, err error, touched, arguments []string, started time.Time) {
	completed := time.Now().UTC()
	toolResult := RealisticToolResult{Sequence: len(result.ToolResults) + 1, ActionKind: actionKind, Success: err == nil, Output: string(output)}
	if err != nil {
		toolResult.Output = err.Error()
	}
	outputHash := sha256.Sum256([]byte(toolResult.Output))
	toolResult.OutputHash = hex.EncodeToString(outputHash[:])
	result.ToolResults = append(result.ToolResults, toolResult)
	toolName := map[string]string{"read": "read", "search": "search", "edit": "edit_within_workspace", "run_check": "run_allowlisted_local_checks", "browser": "browser_on_local_preview_when_case_requires"}[actionKind]
	result.ToolEvents = append(result.ToolEvents, RealisticToolEvent{Sequence: toolResult.Sequence, Tool: toolName, Arguments: arguments, ExitCode: boolExit(err == nil), StartedAt: started.Format(time.RFC3339Nano), CompletedAt: completed.Format(time.RFC3339Nano), OutputSHA256: toolResult.OutputHash, TouchedPaths: touched})
}

func boolExit(success bool) int {
	if success {
		return 0
	}
	return 1
}
