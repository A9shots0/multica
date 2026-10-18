package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

const v38LedgerSchema = "silver-mediums.claude-subscription-realistic-ledger.v38"

type v38StopClass string

const (
	v38Competence v38StopClass = "competence_failure"
	v38Protocol   v38StopClass = "response_or_tool_protocol_failure"
	v38Resource   v38StopClass = "observation_resource_exhaustion"
	v38Transport  v38StopClass = "provider_transport_invalid"
	v38Credential v38StopClass = "credential_failure"
	v38Unsafe     v38StopClass = "unsafe_external_action"
	v38Evidence   v38StopClass = "evidence_corruption"
	v38Campaign   v38StopClass = "campaign_cap"
)

func (kind v38StopClass) global() bool {
	return kind == v38Credential || kind == v38Unsafe || kind == v38Evidence || kind == v38Campaign
}

type v38Failure struct {
	Kind  v38StopClass
	Cause error
}

func (failure *v38Failure) Error() string { return string(failure.Kind) + ": " + failure.Cause.Error() }
func (failure *v38Failure) Unwrap() error { return failure.Cause }
func failV38(kind v38StopClass, err error) error {
	return &v38Failure{Kind: kind, Cause: err}
}

func classifyV38(err error) v38StopClass {
	var typed *v38Failure
	if errors.As(err, &typed) {
		return typed.Kind
	}
	if errors.Is(err, agentqualification.ErrRealisticCredentialFailure) {
		return v38Credential
	}
	if errors.Is(err, agentqualification.ErrRealisticBudgetBreach) {
		return v38Campaign
	}
	return v38Transport
}

type v38Usage struct {
	ProviderInvocations     int   `json:"provider_invocations"`
	InputTokens             int64 `json:"input_tokens"`
	OutputTokens            int64 `json:"output_tokens"`
	CostMicros              int64 `json:"cost_equivalent_micros"`
	AmbiguousInvocations    int   `json:"ambiguous_invocations"`
	ReservedReportedTokens  int64 `json:"reserved_reported_tokens"`
	ReservedGeneratedTokens int64 `json:"reserved_generated_tokens"`
	ReservedCostMicros      int64 `json:"reserved_cost_equivalent_micros"`
}

type v38PaidPhaseBinding struct {
	Phase         int    `json:"phase"`
	Path          string `json:"path"`
	SHA256        string `json:"sha256"`
	RequestSHA256 string `json:"request_sha256"`
}

type v38ProviderFailureReceipt struct {
	SchemaVersion       string          `json:"schema_version"`
	ProviderCallID      string          `json:"provider_call_id"`
	RequestSHA256       string          `json:"request_sha256"`
	PromptSHA256        string          `json:"prompt_sha256"`
	OutputSchemaSHA256  string          `json:"output_schema_sha256"`
	Model               string          `json:"model"`
	Effort              string          `json:"effort"`
	ServingProvider     string          `json:"serving_provider"`
	Transport           string          `json:"transport"`
	StartedAt           string          `json:"started_at"`
	CompletedAt         string          `json:"completed_at"`
	FailureClass        string          `json:"failure_class"`
	AccountingStatus    string          `json:"accounting_status"`
	RawStdout           []byte          `json:"raw_stdout"`
	RawStdoutSHA256     string          `json:"raw_stdout_sha256"`
	RawStderr           []byte          `json:"raw_stderr"`
	RawStderrSHA256     string          `json:"raw_stderr_sha256"`
	InputTokens         int64           `json:"input_tokens"`
	OutputTokens        int64           `json:"output_tokens"`
	CostMicros          int64           `json:"cost_equivalent_micros"`
	ReservedTotalTokens int64           `json:"reserved_total_tokens"`
	ReservedOutput      int64           `json:"reserved_output_tokens"`
	ReservedCostMicros  int64           `json:"reserved_cost_equivalent_micros"`
	RequestJSON         json.RawMessage `json:"request_json"`
}

type v38PaidPhaseArtifact struct {
	Request agentqualification.RealisticProviderRequest `json:"request"`
	Plan    agentqualification.ClaudeRealisticV37Plan   `json:"plan"`
	// Receipt is bytes rather than RawMessage so the exact provider-receipt
	// bytes survive nesting in the phase artifact without JSON normalization.
	Receipt []byte `json:"receipt"`
}

type v38Claim struct {
	CaseID               string                `json:"case_id"`
	CaseVersion          string                `json:"case_version"`
	RunFamilyID          string                `json:"run_family_id"`
	Status               string                `json:"status"`
	Outcome              string                `json:"outcome,omitempty"`
	FailureClass         string                `json:"failure_class,omitempty"`
	Phase                int                   `json:"phase"`
	InFlight             bool                  `json:"in_flight"`
	RequestSHA256        string                `json:"request_sha256,omitempty"`
	AdmissionSHA256      string                `json:"admission_sha256,omitempty"`
	RuntimeSHA256        string                `json:"runtime_sha256,omitempty"`
	ExecutionSHA256      string                `json:"execution_sha256,omitempty"`
	GradeSHA256          string                `json:"grade_sha256,omitempty"`
	ResultSHA256         string                `json:"result_sha256,omitempty"`
	FailureReceiptSHA256 string                `json:"failure_receipt_sha256,omitempty"`
	PaidPhases           []v38PaidPhaseBinding `json:"paid_phases,omitempty"`
	Usage                v38Usage              `json:"usage"`
	Retries              int                   `json:"retries"`
}

type v38Ledger struct {
	SchemaVersion       string    `json:"schema_version"`
	BatchID             string    `json:"batch_id"`
	DefinitionSHA256    string    `json:"definition_sha256"`
	Status              string    `json:"status"`
	Claim               *v38Claim `json:"claim,omitempty"`
	CandidateEliminated bool      `json:"candidate_eliminated"`
	GlobalHalt          bool      `json:"global_halt"`
	GlobalHaltReason    string    `json:"global_halt_reason,omitempty"`
}

type v38Runtime struct {
	Workspace agentqualification.RealisticCaseWorkspace           `json:"workspace"`
	State     agentqualification.ClaudeRealisticV37ExecutionState `json:"state"`
}

type v38Result struct {
	Claim v38Claim                                    `json:"claim"`
	Grade *agentqualification.RealisticWorkspaceGrade `json:"grade,omitempty"`
}

type v38PhaseProvider interface {
	Plan(context.Context, agentqualification.ClaudeScreeningPlan, agentqualification.RealisticProviderRequest) (agentqualification.ClaudeRealisticV37Plan, []byte, error)
}

func executeV38(ctx context.Context, root string, provider v38PhaseProvider, gate func() error) (v38Ledger, error) {
	if gate == nil {
		return v38Ledger{}, fmt.Errorf("v38 review gate missing")
	}
	if err := gate(); err != nil {
		return v38Ledger{}, err
	}
	d, err := agentqualification.ClaudeSubscriptionRealisticV38()
	if err != nil {
		return v38Ledger{}, err
	}
	if !filepath.IsAbs(root) || provider == nil {
		return v38Ledger{}, fmt.Errorf("absolute durable root and provider required")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return v38Ledger{}, err
	}
	ledgerPath := filepath.Join(root, "ledger.json")
	ledger, err := loadV38Ledger(ledgerPath, d)
	if err != nil {
		return ledger, err
	}
	if ledger.Status == "complete" || ledger.GlobalHalt {
		if err := replayClosedV38(root, ledger); err != nil {
			return haltV38(ledgerPath, ledger, v38Evidence, err)
		}
		return ledger, nil
	}
	cell := filepath.Join(root, "observations", "R04")
	runtimePath := filepath.Join(cell, "runtime.json")
	var runtimeState v38Runtime
	if ledger.Claim == nil {
		if err := os.MkdirAll(filepath.Join(cell, "workspace"), 0o700); err != nil {
			return ledger, err
		}
		runFamily := hashV38(d.BatchID + "|R04:v1")
		workspace, prepareErr := agentqualification.PrepareRealisticCaseWorkspaceForRun(filepath.Join(cell, "workspace"), "R04", runFamily)
		if prepareErr != nil {
			return haltV38(ledgerPath, ledger, v38Evidence, prepareErr)
		}
		state, initializeErr := agentqualification.InitializeClaudeSubscriptionRealisticV38(workspace, runFamily, func(agentqualification.RealisticControllerResult) error { return nil })
		if initializeErr != nil {
			return haltV38(ledgerPath, ledger, v38Evidence, initializeErr)
		}
		runtimeState = v38Runtime{Workspace: workspace, State: state}
		if err := writeExclusiveV38(runtimePath, runtimeState); err != nil {
			return haltV38(ledgerPath, ledger, v38Evidence, err)
		}
		runtimeHash, _ := fileHashV38(runtimePath)
		admission := struct {
			SchemaVersion    string                                    `json:"schema_version"`
			BatchID          string                                    `json:"batch_id"`
			DefinitionSHA256 string                                    `json:"definition_sha256"`
			CaseID           string                                    `json:"case_id"`
			CaseVersion      string                                    `json:"case_version"`
			RunFamilyID      string                                    `json:"run_family_id"`
			Caps             agentqualification.ClaudeRealisticV37Caps `json:"caps"`
			Attempt1         string                                    `json:"attempt_1"`
			Attempt2         string                                    `json:"attempt_2"`
		}{"silver-mediums.claude-subscription-realistic-admission.v38", d.BatchID, agentqualification.ClaudeSubscriptionRealisticV38SHA256(), "R04", "v1", runFamily, d.Caps, d.Attempt1, d.Attempt2}
		admissionPath := filepath.Join(cell, "admission.json")
		if err := writeExclusiveV38(admissionPath, admission); err != nil {
			return haltV38(ledgerPath, ledger, v38Evidence, err)
		}
		admissionHash, _ := fileHashV38(admissionPath)
		ledger.Claim = &v38Claim{CaseID: "R04", CaseVersion: "v1", RunFamilyID: runFamily, Status: "pending", AdmissionSHA256: admissionHash, RuntimeSHA256: runtimeHash, Retries: 0}
		if err := writeAtomicV38(ledgerPath, ledger); err != nil {
			return ledger, err
		}
	} else {
		if err := readJSONV38(runtimePath, &runtimeState); err != nil {
			return haltV38(ledgerPath, ledger, v38Evidence, err)
		}
		if ledger.Claim.Status != "pending" || ledger.Claim.CaseID != "R04" || ledger.Claim.CaseVersion != "v1" {
			return haltV38(ledgerPath, ledger, v38Evidence, fmt.Errorf("v38 claim identity drift"))
		}
		runtimeHash, runtimeHashErr := fileHashV38(runtimePath)
		admissionHash, admissionHashErr := fileHashV38(filepath.Join(cell, "admission.json"))
		if runtimeHashErr != nil || admissionHashErr != nil || runtimeHash != ledger.Claim.RuntimeSHA256 || admissionHash != ledger.Claim.AdmissionSHA256 {
			return haltV38(ledgerPath, ledger, v38Evidence, fmt.Errorf("v38 durable artifact hash drift"))
		}
		if err := verifyV38PaidPhases(cell, ledger.Claim, runtimeState.State); err != nil {
			return haltV38(ledgerPath, ledger, v38Evidence, err)
		}
		if err := verifyV38RunningUsage(d, ledger.Claim, runtimeState.State); err != nil {
			return haltV38(ledgerPath, ledger, v38Evidence, err)
		}
		if ledger.Claim.InFlight && runtimeState.State.PendingPlan == nil {
			recovered, recoverErr := recoverV38PaidPhase(cell, runtimePath, ledgerPath, &ledger, &runtimeState)
			if recoverErr != nil && recovered {
				kind := classifyV38Execution(recoverErr)
				if kind.global() {
					return haltV38(ledgerPath, ledger, kind, recoverErr)
				}
				return closeV38(root, ledgerPath, ledger, "failed", kind, nil)
			}
			if recoverErr == nil && recovered {
				ledger.Claim.InFlight = false
				if err := checkpointRuntimeV38(runtimePath, ledgerPath, &runtimeState, &ledger); err != nil {
					return haltV38(ledgerPath, ledger, v38Evidence, err)
				}
			} else {
				ledger.Claim.Usage = conservativeV38Usage(d, runtimeState.State.Controller)
				return closeV38(root, ledgerPath, ledger, "invalid", v38Transport, nil)
			}
		}
	}

	for {
		claim := ledger.Claim
		if runtimeState.State.Controller.TerminalState == "failed" {
			return closeV38(root, ledgerPath, ledger, "failed", v38Protocol, nil)
		}
		if runtimeState.State.PendingPlan != nil {
			resumed, resumeErr := agentqualification.ExecuteClaudeSubscriptionRealisticV38Phase(runtimeState.Workspace, claim.RunFamilyID, *runtimeState.State.PendingPlan, runtimeState.State.PendingReceipt, runtimeState.State, func(state agentqualification.ClaudeRealisticV37ExecutionState) error {
				runtimeState.State = state
				return checkpointRuntimeV38(runtimePath, ledgerPath, &runtimeState, &ledger)
			})
			runtimeState.State = resumed
			if resumeErr != nil {
				return haltV38(ledgerPath, ledger, v38Evidence, resumeErr)
			}
			claim.InFlight = false
			if resumed.Controller.TerminalState == "failed" {
				return closeV38(root, ledgerPath, ledger, "failed", v38Protocol, nil)
			}
			if err := checkpointRuntimeV38(runtimePath, ledgerPath, &runtimeState, &ledger); err != nil {
				return haltV38(ledgerPath, ledger, v38Evidence, err)
			}
			continue
		}
		if runtimeState.State.Controller.TerminalState == "completed" {
			return completeV38(root, ledgerPath, ledger, &runtimeState)
		}
		if runtimeState.State.CompletedPhases >= d.Caps.MaxProviderInvocations {
			return closeV38(root, ledgerPath, ledger, "failed", v38Protocol, nil)
		}
		transport, request, requestErr := agentqualification.BuildClaudeSubscriptionRealisticV38PhaseRequest(runtimeState.Workspace, claim.RunFamilyID, runtimeState.State)
		if requestErr != nil {
			return haltV38(ledgerPath, ledger, v38Evidence, requestErr)
		}
		requestRaw, _ := json.Marshal(request)
		reserved, admissionErr := admitV38ProviderCall(d, runtimeState.State.Controller)
		if admissionErr != nil {
			claim.Usage = usageV38(runtimeState.State.Controller)
			return closeV38(root, ledgerPath, ledger, "failed", v38Resource, nil)
		}
		claim.Phase, claim.InFlight, claim.RequestSHA256 = request.ProviderTurn, true, hashV38Bytes(requestRaw)
		claim.Usage = reserved
		if err := writeAtomicV38(ledgerPath, ledger); err != nil {
			return ledger, err
		}
		candidate, receipt, callErr := provider.Plan(ctx, transport, request)
		if callErr != nil {
			kind := classifyV38(callErr)
			if len(receipt) != 0 {
				if err := recordV38FailureReceipt(cell, request.ProviderTurn, receipt, claim, d); err != nil {
					return haltV38(ledgerPath, ledger, v38Evidence, err)
				}
				if err := writeAtomicV38(ledgerPath, ledger); err != nil {
					return haltV38(ledgerPath, ledger, v38Evidence, err)
				}
			} else {
				// Without a durable raw response, the post-launch billing outcome is
				// unknowable. Retain the full remaining reservation and never retry.
				claim.Usage = conservativeV38Usage(d, runtimeState.State.Controller)
			}
			outcome := "failed"
			if kind == v38Transport || kind.global() {
				outcome = "invalid"
			}
			closed, closeErr := closeV38(root, ledgerPath, ledger, outcome, kind, nil)
			if closeErr != nil {
				return haltV38(ledgerPath, ledger, v38Evidence, closeErr)
			}
			if kind.global() {
				return haltV38(ledgerPath, closed, kind, callErr)
			}
			return closed, nil
		}
		artifact := v38PaidPhaseArtifact{request, candidate, append(json.RawMessage(nil), receipt...)}
		phasePath := filepath.Join(cell, fmt.Sprintf("phase-%02d-paid.json", request.ProviderTurn))
		if err := writeExclusiveV38(phasePath, artifact); err != nil {
			return haltV38(ledgerPath, ledger, v38Evidence, err)
		}
		phaseHash, _ := fileHashV38(phasePath)
		claim.PaidPhases = append(claim.PaidPhases, v38PaidPhaseBinding{Phase: request.ProviderTurn, Path: filepath.Base(phasePath), SHA256: phaseHash, RequestSHA256: claim.RequestSHA256})
		if err := writeAtomicV38(ledgerPath, ledger); err != nil {
			return haltV38(ledgerPath, ledger, v38Evidence, err)
		}
		next, executeErr := agentqualification.ExecuteClaudeSubscriptionRealisticV38Phase(runtimeState.Workspace, claim.RunFamilyID, candidate, receipt, runtimeState.State, func(state agentqualification.ClaudeRealisticV37ExecutionState) error {
			runtimeState.State = state
			return checkpointRuntimeV38(runtimePath, ledgerPath, &runtimeState, &ledger)
		})
		runtimeState.State = next
		if executeErr != nil {
			kind := classifyV38Execution(executeErr)
			if kind.global() {
				return haltV38(ledgerPath, ledger, kind, executeErr)
			}
			return closeV38(root, ledgerPath, ledger, "failed", kind, nil)
		}
		claim.InFlight = false
		claim.Usage = usageV38(next.Controller)
		if err := checkpointRuntimeV38(runtimePath, ledgerPath, &runtimeState, &ledger); err != nil {
			return haltV38(ledgerPath, ledger, v38Evidence, err)
		}
	}
}

func completeV38(root, ledgerPath string, ledger v38Ledger, runtimeState *v38Runtime) (v38Ledger, error) {
	claim := ledger.Claim
	cell := filepath.Join(root, "observations", "R04")
	receipt, receiptPath, err := agentqualification.SealClaudeSubscriptionRealisticV38Execution(filepath.Join(cell, "evidence"), runtimeState.Workspace, runtimeState.State.Controller)
	if err != nil {
		return haltV38(ledgerPath, ledger, v38Evidence, err)
	}
	claim.ExecutionSHA256, _ = fileHashV38(receiptPath)
	grade, err := agentqualification.GradeClaudeSubscriptionRealisticV38(runtimeState.Workspace, receipt, runtimeState.State.Controller.FinalSummary)
	if err != nil {
		return haltV38(ledgerPath, ledger, v38Evidence, err)
	}
	if err := writeExclusiveV38(filepath.Join(cell, "grade.json"), grade); err != nil {
		return haltV38(ledgerPath, ledger, v38Evidence, err)
	}
	claim.GradeSHA256, _ = fileHashV38(filepath.Join(cell, "grade.json"))
	if grade.Verdict != "PASS" {
		return closeV38(root, ledgerPath, ledger, "failed", v38Competence, &grade)
	}
	return closeV38(root, ledgerPath, ledger, "pass", "", &grade)
}

func closeV38(root, ledgerPath string, ledger v38Ledger, outcome string, kind v38StopClass, grade *agentqualification.RealisticWorkspaceGrade) (v38Ledger, error) {
	claim := ledger.Claim
	claim.Status, claim.Outcome, claim.FailureClass, claim.InFlight = "completed", outcome, string(kind), false
	resultPath := filepath.Join(root, "observations", "R04", "result.json")
	if err := writeExclusiveV38(resultPath, v38Result{Claim: *claim, Grade: grade}); err != nil {
		return haltV38(ledgerPath, ledger, v38Evidence, err)
	}
	claim.ResultSHA256, _ = fileHashV38(resultPath)
	if kind == v38Competence || kind == v38Protocol || kind == v38Resource {
		ledger.CandidateEliminated = true
	}
	ledger.Status = "complete"
	if err := writeAtomicV38(ledgerPath, ledger); err != nil {
		return ledger, err
	}
	return ledger, nil
}

func haltV38(path string, ledger v38Ledger, kind v38StopClass, cause error) (v38Ledger, error) {
	ledger.GlobalHalt, ledger.GlobalHaltReason, ledger.Status = true, string(kind), "halted"
	if writeErr := writeAtomicV38(path, ledger); writeErr != nil {
		if cause == nil {
			cause = writeErr
		} else {
			cause = fmt.Errorf("%v; durable halt write failed: %w", cause, writeErr)
		}
	}
	if cause == nil {
		cause = errors.New(string(kind))
	}
	return ledger, failV38(kind, cause)
}

func loadV38Ledger(path string, d agentqualification.ClaudeSubscriptionRealisticV38Definition) (v38Ledger, error) {
	var ledger v38Ledger
	if err := readJSONV38(path, &ledger); err == nil {
		if ledger.SchemaVersion != v38LedgerSchema || ledger.BatchID != d.BatchID || ledger.DefinitionSHA256 != agentqualification.ClaudeSubscriptionRealisticV38SHA256() {
			return ledger, fmt.Errorf("v38 ledger identity drifted")
		}
		return ledger, nil
	} else if !os.IsNotExist(err) {
		return ledger, err
	}
	ledger = v38Ledger{SchemaVersion: v38LedgerSchema, BatchID: d.BatchID, DefinitionSHA256: agentqualification.ClaudeSubscriptionRealisticV38SHA256(), Status: "running"}
	return ledger, writeExclusiveV38(path, ledger)
}

func checkpointRuntimeV38(runtimePath, ledgerPath string, runtimeState *v38Runtime, ledger *v38Ledger) error {
	if err := writeAtomicV38(runtimePath, runtimeState); err != nil {
		return err
	}
	ledger.Claim.RuntimeSHA256, _ = fileHashV38(runtimePath)
	ledger.Claim.Usage = usageV38(runtimeState.State.Controller)
	return writeAtomicV38(ledgerPath, *ledger)
}

func usageV38(result agentqualification.RealisticControllerResult) v38Usage {
	return v38Usage{ProviderInvocations: result.ProviderCalls, InputTokens: result.InputTokens, OutputTokens: result.OutputTokens, CostMicros: result.CostMicros}
}

func checkedAddV38Int64(left, right int64) (int64, bool) {
	if left < 0 || right < 0 || left > int64(^uint64(0)>>1)-right {
		return 0, false
	}
	return left + right, true
}

func admitV38ProviderCall(d agentqualification.ClaudeSubscriptionRealisticV38Definition, controller agentqualification.RealisticControllerResult) (v38Usage, error) {
	total, ok := checkedAddV38Int64(controller.InputTokens, controller.OutputTokens)
	if !ok || controller.ProviderCalls < 0 || controller.ProviderCalls >= d.Caps.MaxProviderInvocations || total < 0 || total >= d.Caps.MaxTotalReportedTokens || controller.OutputTokens < 0 || controller.OutputTokens >= d.Caps.MaxGeneratedTokens || controller.CostMicros < 0 || controller.CostMicros >= d.Caps.MaxCostMicros {
		return v38Usage{}, failV38(v38Resource, fmt.Errorf("v38 pre-call case/campaign admission denied"))
	}
	usage := usageV38(controller)
	usage.ReservedReportedTokens = d.Caps.MaxTotalReportedTokens - total
	usage.ReservedGeneratedTokens = d.Caps.MaxGeneratedTokens - controller.OutputTokens
	usage.ReservedCostMicros = d.Caps.MaxCostMicros - controller.CostMicros
	return usage, nil
}

func conservativeV38Usage(d agentqualification.ClaudeSubscriptionRealisticV38Definition, controller agentqualification.RealisticControllerResult) v38Usage {
	usage := usageV38(controller)
	total, ok := checkedAddV38Int64(controller.InputTokens, controller.OutputTokens)
	if !ok || total > d.Caps.MaxTotalReportedTokens {
		total = d.Caps.MaxTotalReportedTokens
	}
	usage.AmbiguousInvocations = 1
	usage.ReservedReportedTokens = d.Caps.MaxTotalReportedTokens - total
	usage.ReservedGeneratedTokens = d.Caps.MaxGeneratedTokens - controller.OutputTokens
	usage.ReservedCostMicros = d.Caps.MaxCostMicros - controller.CostMicros
	if usage.ReservedGeneratedTokens < 0 {
		usage.ReservedGeneratedTokens = 0
	}
	if usage.ReservedCostMicros < 0 {
		usage.ReservedCostMicros = 0
	}
	return usage
}

func classifyV38Execution(err error) v38StopClass {
	var typed *agentqualification.ClaudeRealisticV38Error
	if !errors.As(err, &typed) {
		return v38Evidence
	}
	switch typed.Class {
	case agentqualification.ClaudeRealisticV38FailureProtocol:
		return v38Protocol
	case agentqualification.ClaudeRealisticV38FailureResource:
		return v38Resource
	default:
		return v38Evidence
	}
}

func recordV38FailureReceipt(cell string, phase int, raw []byte, claim *v38Claim, d agentqualification.ClaudeSubscriptionRealisticV38Definition) error {
	var receipt v38ProviderFailureReceipt
	if err := decodeV38FailureReceipt(raw, &receipt, claim, phase, d); err != nil {
		return err
	}
	path := filepath.Join(cell, fmt.Sprintf("phase-%02d-failure.json", phase))
	if err := writeExclusiveV38(path, receipt); err != nil {
		return err
	}
	claim.FailureReceiptSHA256, _ = fileHashV38(path)
	if receipt.AccountingStatus == "exact" {
		input, ok := checkedAddV38Int64(claim.Usage.InputTokens, receipt.InputTokens)
		if !ok {
			return fmt.Errorf("v38 failed-call input overflow")
		}
		output, ok := checkedAddV38Int64(claim.Usage.OutputTokens, receipt.OutputTokens)
		if !ok {
			return fmt.Errorf("v38 failed-call output overflow")
		}
		cost, ok := checkedAddV38Int64(claim.Usage.CostMicros, receipt.CostMicros)
		if !ok {
			return fmt.Errorf("v38 failed-call cost overflow")
		}
		claim.Usage = v38Usage{ProviderInvocations: claim.Usage.ProviderInvocations + 1, InputTokens: input, OutputTokens: output, CostMicros: cost}
	} else if receipt.AccountingStatus == "conservative" {
		claim.Usage.AmbiguousInvocations = 1
	} else {
		return fmt.Errorf("v38 failed-call accounting status invalid")
	}
	return nil
}

func decodeV38FailureReceipt(raw []byte, receipt *v38ProviderFailureReceipt, claim *v38Claim, phase int, d agentqualification.ClaudeSubscriptionRealisticV38Definition) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(receipt) != nil || decoder.Decode(&struct{}{}) != io.EOF || receipt.SchemaVersion != "silver-mediums.claude-subscription-realistic-provider-failure.v38" || receipt.ProviderCallID != fmt.Sprintf("%s:attempt-2:phase-%d", claim.RunFamilyID, phase) || receipt.RequestSHA256 != claim.RequestSHA256 || receipt.RawStdoutSHA256 != hashV38Bytes(receipt.RawStdout) || receipt.RawStderrSHA256 != hashV38Bytes(receipt.RawStderr) || receipt.Model != d.Configuration.Model || receipt.Effort != d.Configuration.ReasoningEffort || receipt.ServingProvider != d.Configuration.ServingProvider || receipt.Transport != d.Transport || receipt.InputTokens < 0 || receipt.OutputTokens < 0 || receipt.CostMicros < 0 {
		return fmt.Errorf("v38 provider failure receipt invalid")
	}
	if !containsV38([]string{string(v38Protocol), string(v38Transport), string(v38Credential), string(v38Campaign)}, receipt.FailureClass) {
		return fmt.Errorf("v38 provider failure class invalid")
	}
	var request agentqualification.RealisticProviderRequest
	requestDecoder := json.NewDecoder(bytes.NewReader(receipt.RequestJSON))
	requestDecoder.DisallowUnknownFields()
	if requestDecoder.Decode(&request) != nil || requestDecoder.Decode(&struct{}{}) != io.EOF || hashV38Bytes(mustJSONV38(request)) != receipt.RequestSHA256 || request.ProviderCallID != receipt.ProviderCallID || request.ProviderTurn != phase || request.RunFamilyID != claim.RunFamilyID || request.CaseID != "R04" || request.CaseVersion != "v1" || request.Attempt != 2 {
		return fmt.Errorf("v38 provider failure request invalid")
	}
	if receipt.AccountingStatus == "exact" {
		if receipt.ReservedTotalTokens != 0 || receipt.ReservedOutput != 0 || receipt.ReservedCostMicros != 0 {
			return fmt.Errorf("v38 exact failure receipt retained reservation")
		}
	} else if receipt.AccountingStatus == "conservative" {
		if receipt.ReservedTotalTokens != d.Caps.MaxTotalReportedTokens || receipt.ReservedOutput != d.Caps.MaxGeneratedTokens || receipt.ReservedCostMicros != d.Caps.MaxCostMicros {
			return fmt.Errorf("v38 conservative failure reservation invalid")
		}
	} else {
		return fmt.Errorf("v38 failed-call accounting status invalid")
	}
	return nil
}

func verifyV38RunningUsage(d agentqualification.ClaudeSubscriptionRealisticV38Definition, claim *v38Claim, state agentqualification.ClaudeRealisticV37ExecutionState) error {
	var want v38Usage
	var err error
	if claim.InFlight && state.PendingPlan == nil {
		want, err = admitV38ProviderCall(d, state.Controller)
	} else {
		want = usageV38(state.Controller)
	}
	if err != nil || claim.Usage != want {
		return fmt.Errorf("v38 running usage/admission drift")
	}
	return nil
}

func verifyV38PaidPhases(cell string, claim *v38Claim, state agentqualification.ClaudeRealisticV37ExecutionState) error {
	controller := state.Controller
	if len(claim.PaidPhases) < controller.ProviderCalls || len(claim.PaidPhases) > controller.ProviderCalls+1 || len(controller.ProviderReceipts) != controller.ProviderCalls || len(controller.ProviderReceiptHashes) != controller.ProviderCalls {
		return fmt.Errorf("v38 paid-phase/controller count drift")
	}
	for index, binding := range claim.PaidPhases {
		if binding.Phase != index+1 || binding.Path != fmt.Sprintf("phase-%02d-paid.json", index+1) {
			return fmt.Errorf("v38 paid-phase order drift")
		}
		path := filepath.Join(cell, binding.Path)
		hash, err := fileHashV38(path)
		if err != nil || hash != binding.SHA256 {
			return fmt.Errorf("v38 paid-phase hash drift")
		}
		artifact, err := readV38PaidPhase(path)
		if err != nil || artifact.Request.ProviderTurn != binding.Phase || artifact.Request.RunFamilyID != claim.RunFamilyID || hashV38Bytes(mustJSONV38(artifact.Request)) != binding.RequestSHA256 {
			return fmt.Errorf("v38 paid-phase request drift")
		}
		_, replayed, err := agentqualification.ParseClaudeRealisticV38ProviderReceipt(artifact.Receipt)
		if err != nil || !equalJSONV38(replayed, artifact.Plan) {
			return fmt.Errorf("v38 paid-phase replay drift: %w", err)
		}
		if index < controller.ProviderCalls {
			if !bytes.Equal(controller.ProviderReceipts[index], artifact.Receipt) || controller.ProviderReceiptHashes[index] != hashV38Bytes(artifact.Receipt) {
				return fmt.Errorf("v38 controller receipt drift")
			}
		}
	}
	if len(claim.PaidPhases) == controller.ProviderCalls+1 && (!claim.InFlight || state.PendingPlan != nil) {
		return fmt.Errorf("v38 unreconciled paid phase state invalid")
	}
	return nil
}

func recoverV38PaidPhase(cell, runtimePath, ledgerPath string, ledger *v38Ledger, runtimeState *v38Runtime) (bool, error) {
	claim := ledger.Claim
	path := filepath.Join(cell, fmt.Sprintf("phase-%02d-paid.json", claim.Phase))
	if _, err := os.Stat(path); err != nil {
		return false, err
	}
	artifact, err := readV38PaidPhase(path)
	if err != nil || artifact.Request.ProviderTurn != claim.Phase || artifact.Request.RunFamilyID != claim.RunFamilyID || hashV38Bytes(mustJSONV38(artifact.Request)) != claim.RequestSHA256 {
		return false, fmt.Errorf("v38 paid phase cannot be reconciled")
	}
	_, replayed, err := agentqualification.ParseClaudeRealisticV38ProviderReceipt(artifact.Receipt)
	if err != nil || !equalJSONV38(replayed, artifact.Plan) {
		return false, fmt.Errorf("v38 paid phase replay failed")
	}
	hash, _ := fileHashV38(path)
	if len(claim.PaidPhases) == claim.Phase-1 {
		claim.PaidPhases = append(claim.PaidPhases, v38PaidPhaseBinding{Phase: claim.Phase, Path: filepath.Base(path), SHA256: hash, RequestSHA256: claim.RequestSHA256})
	}
	if err := writeAtomicV38(ledgerPath, *ledger); err != nil {
		return false, err
	}
	next, err := agentqualification.ExecuteClaudeSubscriptionRealisticV38Phase(runtimeState.Workspace, claim.RunFamilyID, artifact.Plan, artifact.Receipt, runtimeState.State, func(state agentqualification.ClaudeRealisticV37ExecutionState) error {
		runtimeState.State = state
		return checkpointRuntimeV38(runtimePath, ledgerPath, runtimeState, ledger)
	})
	runtimeState.State = next
	return true, err
}

func readV38PaidPhase(path string) (v38PaidPhaseArtifact, error) {
	var artifact v38PaidPhaseArtifact
	return artifact, readJSONV38(path, &artifact)
}

func mustJSONV38(value any) []byte {
	raw, _ := json.Marshal(value)
	return raw
}

func equalJSONV38(left, right any) bool {
	return string(mustJSONV38(left)) == string(mustJSONV38(right))
}

func replayClosedV38(root string, ledger v38Ledger) error {
	d, err := agentqualification.ClaudeSubscriptionRealisticV38()
	if err != nil {
		return err
	}
	if ledger.Claim == nil || ledger.Claim.Status != "completed" || !containsV38([]string{"pass", "failed", "invalid"}, ledger.Claim.Outcome) {
		return fmt.Errorf("v38 closed claim missing")
	}
	cell := filepath.Join(root, "observations", "R04")
	var runtimeState v38Runtime
	runtimePath := filepath.Join(cell, "runtime.json")
	if err := readJSONV38(runtimePath, &runtimeState); err != nil {
		return err
	}
	runtimeHash, _ := fileHashV38(runtimePath)
	admissionHash, _ := fileHashV38(filepath.Join(cell, "admission.json"))
	if runtimeHash != ledger.Claim.RuntimeSHA256 || admissionHash != ledger.Claim.AdmissionSHA256 {
		return fmt.Errorf("v38 closed durable hash drift")
	}
	if err := verifyV38PaidPhases(cell, ledger.Claim, runtimeState.State); err != nil {
		return err
	}
	resultPath := filepath.Join(cell, "result.json")
	resultHash, err := fileHashV38(resultPath)
	if err != nil || resultHash != ledger.Claim.ResultSHA256 {
		return fmt.Errorf("v38 result hash drift")
	}
	var result v38Result
	if err := readJSONV38(resultPath, &result); err != nil {
		return err
	}
	wantClaim := *ledger.Claim
	wantClaim.ResultSHA256 = ""
	if !equalJSONV38(result.Claim, wantClaim) {
		return fmt.Errorf("v38 result/ledger drift")
	}
	if ledger.Claim.FailureReceiptSHA256 != "" {
		failurePath := filepath.Join(cell, fmt.Sprintf("phase-%02d-failure.json", ledger.Claim.Phase))
		failureHash, err := fileHashV38(failurePath)
		if err != nil || failureHash != ledger.Claim.FailureReceiptSHA256 {
			return fmt.Errorf("v38 failure receipt hash drift")
		}
		failureRaw, err := os.ReadFile(failurePath)
		if err != nil {
			return err
		}
		var failure v38ProviderFailureReceipt
		if err := decodeV38FailureReceipt(failureRaw, &failure, ledger.Claim, ledger.Claim.Phase, d); err != nil {
			return err
		}
		if failure.FailureClass != ledger.Claim.FailureClass {
			return fmt.Errorf("v38 provider failure class drift")
		}
		var failureRequest agentqualification.RealisticProviderRequest
		if err := json.Unmarshal(failure.RequestJSON, &failureRequest); err != nil {
			return err
		}
		transport, request, err := agentqualification.BuildClaudeSubscriptionRealisticV38PhaseRequest(runtimeState.Workspace, ledger.Claim.RunFamilyID, runtimeState.State)
		if err != nil || !equalJSONV38(request, failureRequest) || failure.PromptSHA256 != transport.PromptSHA256 || failure.OutputSchemaSHA256 != transport.SchemaSHA256 {
			return fmt.Errorf("v38 failed provider call replay drift: %w", err)
		}
		wantUsage := usageV38(runtimeState.State.Controller)
		if failure.AccountingStatus == "exact" {
			input, inputOK := checkedAddV38Int64(wantUsage.InputTokens, failure.InputTokens)
			output, outputOK := checkedAddV38Int64(wantUsage.OutputTokens, failure.OutputTokens)
			cost, costOK := checkedAddV38Int64(wantUsage.CostMicros, failure.CostMicros)
			if !inputOK || !outputOK || !costOK {
				return fmt.Errorf("v38 failed provider accounting overflow")
			}
			wantUsage.ProviderInvocations++
			wantUsage.InputTokens, wantUsage.OutputTokens, wantUsage.CostMicros = input, output, cost
		} else {
			wantUsage = conservativeV38Usage(d, runtimeState.State.Controller)
		}
		if ledger.Claim.Usage != wantUsage {
			return fmt.Errorf("v38 failed provider accounting drift")
		}
	} else {
		wantUsage := usageV38(runtimeState.State.Controller)
		if ledger.Claim.Outcome == "invalid" && ledger.Claim.Usage.AmbiguousInvocations == 1 {
			wantUsage = conservativeV38Usage(d, runtimeState.State.Controller)
		}
		if ledger.Claim.Usage != wantUsage {
			return fmt.Errorf("v38 closed accounting drift")
		}
	}
	if ledger.Claim.ExecutionSHA256 == "" {
		if ledger.Claim.GradeSHA256 != "" || result.Grade != nil {
			return fmt.Errorf("v38 unexecuted result has grade")
		}
		return nil
	}
	receiptPath := filepath.Join(cell, "evidence", "execution-receipt.json")
	receiptHash, err := fileHashV38(receiptPath)
	if err != nil || receiptHash != ledger.Claim.ExecutionSHA256 {
		return fmt.Errorf("v38 execution receipt hash drift")
	}
	var receipt agentqualification.RealisticExecutionReceipt
	if err := readJSONV38(receiptPath, &receipt); err != nil {
		return err
	}
	sealed, _, err := agentqualification.SealClaudeSubscriptionRealisticV38Execution(filepath.Join(cell, "evidence"), runtimeState.Workspace, runtimeState.State.Controller)
	if err != nil || !equalJSONV38(sealed, receipt) {
		return fmt.Errorf("v38 execution receipt replay failed: %w", err)
	}
	gradePath := filepath.Join(cell, "grade.json")
	gradeHash, err := fileHashV38(gradePath)
	if err != nil || gradeHash != ledger.Claim.GradeSHA256 {
		return fmt.Errorf("v38 grade hash drift")
	}
	var grade agentqualification.RealisticWorkspaceGrade
	if err := readJSONV38(gradePath, &grade); err != nil {
		return err
	}
	replayedGrade, err := agentqualification.GradeClaudeSubscriptionRealisticV38(runtimeState.Workspace, &receipt, runtimeState.State.Controller.FinalSummary)
	if err != nil || !equalJSONV38(replayedGrade, grade) || result.Grade == nil || !equalJSONV38(*result.Grade, grade) {
		return fmt.Errorf("v38 grade replay failed: %w", err)
	}
	return nil
}

func containsV38(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func readJSONV38(path string, out any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err == io.EOF {
		return nil
	}
	return fmt.Errorf("v38 JSON has trailing data")
}

func writeExclusiveV38(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.Write(append(raw, '\n')); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func writeAtomicV38(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".v38-*.tmp")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(append(raw, '\n')); err != nil {
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
	return os.Rename(name, path)
}

func fileHashV38(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return hashV38Bytes(raw), nil
}

func hashV38(value string) string { return hashV38Bytes([]byte(value)) }
func hashV38Bytes(raw []byte) string {
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}
