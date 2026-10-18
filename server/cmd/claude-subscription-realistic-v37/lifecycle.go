package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

const v37LedgerSchema = "silver-mediums.claude-subscription-realistic-ledger.v37"

type v37StopClass string

const (
	v37Competence v37StopClass = "competence_failure"
	v37Protocol   v37StopClass = "response_or_tool_protocol_failure"
	v37Resource   v37StopClass = "observation_resource_exhaustion"
	v37Transport  v37StopClass = "provider_transport_invalid"
	v37Credential v37StopClass = "credential_failure"
	v37Unsafe     v37StopClass = "unsafe_external_action"
	v37Evidence   v37StopClass = "evidence_corruption"
	v37Campaign   v37StopClass = "campaign_cap"
)

func (kind v37StopClass) global() bool {
	return kind == v37Credential || kind == v37Unsafe || kind == v37Evidence || kind == v37Campaign
}

type v37Failure struct {
	Kind  v37StopClass
	Cause error
}

func (failure *v37Failure) Error() string { return string(failure.Kind) + ": " + failure.Cause.Error() }
func (failure *v37Failure) Unwrap() error { return failure.Cause }

func failV37(kind v37StopClass, err error) error { return &v37Failure{Kind: kind, Cause: err} }

func classifyV37(err error) v37StopClass {
	var typed *v37Failure
	if errors.As(err, &typed) {
		return typed.Kind
	}
	if errors.Is(err, agentqualification.ErrRealisticCredentialFailure) {
		return v37Credential
	}
	if errors.Is(err, agentqualification.ErrRealisticBudgetBreach) {
		return v37Campaign
	}
	return v37Transport
}

type v37Usage struct {
	ProviderInvocations     int   `json:"provider_invocations"`
	InputTokens             int64 `json:"input_tokens"`
	OutputTokens            int64 `json:"output_tokens"`
	CostMicros              int64 `json:"cost_equivalent_micros"`
	AmbiguousInvocations    int   `json:"ambiguous_invocations"`
	ReservedReportedTokens  int64 `json:"reserved_reported_tokens"`
	ReservedGeneratedTokens int64 `json:"reserved_generated_tokens"`
	ReservedCostMicros      int64 `json:"reserved_cost_equivalent_micros"`
}

type v37Claim struct {
	CaseID          string   `json:"case_id"`
	CaseVersion     string   `json:"case_version"`
	RunFamilyID     string   `json:"run_family_id"`
	Status          string   `json:"status"`
	Outcome         string   `json:"outcome,omitempty"`
	FailureClass    string   `json:"failure_class,omitempty"`
	Phase           int      `json:"phase"`
	InFlight        bool     `json:"in_flight"`
	RequestSHA256   string   `json:"request_sha256,omitempty"`
	AdmissionSHA256 string   `json:"admission_sha256,omitempty"`
	RuntimeSHA256   string   `json:"runtime_sha256,omitempty"`
	ExecutionSHA256 string   `json:"execution_sha256,omitempty"`
	GradeSHA256     string   `json:"grade_sha256,omitempty"`
	ResultSHA256    string   `json:"result_sha256,omitempty"`
	Usage           v37Usage `json:"usage"`
	Retries         int      `json:"retries"`
}

type v37Ledger struct {
	SchemaVersion       string    `json:"schema_version"`
	BatchID             string    `json:"batch_id"`
	DefinitionSHA256    string    `json:"definition_sha256"`
	Status              string    `json:"status"`
	Claim               *v37Claim `json:"claim,omitempty"`
	CandidateEliminated bool      `json:"candidate_eliminated"`
	GlobalHalt          bool      `json:"global_halt"`
	GlobalHaltReason    string    `json:"global_halt_reason,omitempty"`
}

type v37Runtime struct {
	Workspace agentqualification.RealisticCaseWorkspace           `json:"workspace"`
	State     agentqualification.ClaudeRealisticV37ExecutionState `json:"state"`
}

type v37Result struct {
	Claim v37Claim                                    `json:"claim"`
	Grade *agentqualification.RealisticWorkspaceGrade `json:"grade,omitempty"`
}

type v37PhaseProvider interface {
	Plan(context.Context, agentqualification.ClaudeScreeningPlan, agentqualification.RealisticProviderRequest) (agentqualification.ClaudeRealisticV37Plan, []byte, error)
}

func executeV37(ctx context.Context, root string, provider v37PhaseProvider, gate func() error) (v37Ledger, error) {
	if gate == nil {
		return v37Ledger{}, fmt.Errorf("v37 review gate missing")
	}
	if err := gate(); err != nil {
		return v37Ledger{}, err
	}
	d, err := agentqualification.ClaudeSubscriptionRealisticV37()
	if err != nil {
		return v37Ledger{}, err
	}
	if !filepath.IsAbs(root) || provider == nil {
		return v37Ledger{}, fmt.Errorf("absolute durable root and provider required")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return v37Ledger{}, err
	}
	ledgerPath := filepath.Join(root, "ledger.json")
	ledger, err := loadV37Ledger(ledgerPath, d)
	if err != nil {
		return ledger, err
	}
	if ledger.Status == "complete" || ledger.GlobalHalt {
		return ledger, nil
	}
	cell := filepath.Join(root, "observations", "R03")
	runtimePath := filepath.Join(cell, "runtime.json")
	var runtimeState v37Runtime
	if ledger.Claim == nil {
		if err := os.MkdirAll(filepath.Join(cell, "workspace"), 0o700); err != nil {
			return ledger, err
		}
		runFamily := hashV37(d.BatchID + "|R03:v2")
		workspace, prepareErr := agentqualification.PrepareRealisticCaseWorkspaceForRun(filepath.Join(cell, "workspace"), "R03", runFamily)
		if prepareErr != nil {
			return haltV37(ledgerPath, ledger, v37Evidence, prepareErr)
		}
		runtimeState = v37Runtime{Workspace: workspace}
		if err := writeExclusiveV37(runtimePath, runtimeState); err != nil {
			return haltV37(ledgerPath, ledger, v37Evidence, err)
		}
		runtimeHash, _ := fileHashV37(runtimePath)
		admission := struct {
			SchemaVersion    string                                    `json:"schema_version"`
			BatchID          string                                    `json:"batch_id"`
			DefinitionSHA256 string                                    `json:"definition_sha256"`
			CaseID           string                                    `json:"case_id"`
			CaseVersion      string                                    `json:"case_version"`
			RunFamilyID      string                                    `json:"run_family_id"`
			Caps             agentqualification.ClaudeRealisticV37Caps `json:"caps"`
		}{SchemaVersion: "silver-mediums.claude-subscription-realistic-admission.v37", BatchID: d.BatchID, DefinitionSHA256: agentqualification.ClaudeSubscriptionRealisticV37SHA256(), CaseID: "R03", CaseVersion: "v2", RunFamilyID: runFamily, Caps: d.Caps}
		admissionPath := filepath.Join(cell, "admission.json")
		if err := writeExclusiveV37(admissionPath, admission); err != nil {
			return haltV37(ledgerPath, ledger, v37Evidence, err)
		}
		admissionHash, _ := fileHashV37(admissionPath)
		ledger.Claim = &v37Claim{CaseID: "R03", CaseVersion: "v2", RunFamilyID: runFamily, Status: "pending", AdmissionSHA256: admissionHash, RuntimeSHA256: runtimeHash, Retries: 0}
		if err := writeAtomicV37(ledgerPath, ledger); err != nil {
			return ledger, err
		}
	} else {
		if err := readJSONV37(runtimePath, &runtimeState); err != nil {
			return haltV37(ledgerPath, ledger, v37Evidence, err)
		}
		if ledger.Claim.Status != "pending" {
			return ledger, fmt.Errorf("v37 nonterminal ledger has closed claim")
		}
		if ledger.Claim.InFlight && runtimeState.State.PendingPlan == nil {
			ledger.Claim.Usage = v37Usage{ProviderInvocations: d.Caps.MaxProviderInvocations, AmbiguousInvocations: d.Caps.MaxProviderInvocations, ReservedReportedTokens: d.Caps.MaxTotalReportedTokens, ReservedGeneratedTokens: d.Caps.MaxGeneratedTokens, ReservedCostMicros: d.Caps.MaxCostMicros}
			return closeV37(root, ledgerPath, ledger, "invalid", v37Transport, nil)
		}
	}

	for {
		claim := ledger.Claim
		if runtimeState.State.Controller.TerminalState == "completed" {
			return completeV37(root, ledgerPath, ledger, &runtimeState)
		}
		if runtimeState.State.PendingPlan != nil {
			resumed, resumeErr := agentqualification.ExecuteClaudeSubscriptionRealisticV37Phase(runtimeState.Workspace, claim.RunFamilyID, runtimeState.State.PendingPhase, *runtimeState.State.PendingPlan, runtimeState.State.PendingReceipt, runtimeState.State, func(state agentqualification.ClaudeRealisticV37ExecutionState) error {
				runtimeState.State = state
				return checkpointRuntimeV37(runtimePath, &runtimeState, claim)
			})
			runtimeState.State = resumed
			if resumeErr != nil {
				return haltV37(ledgerPath, ledger, v37Evidence, resumeErr)
			}
			claim.InFlight = false
			if resumed.Controller.TerminalState == "failed" {
				return closeV37(root, ledgerPath, ledger, "failed", v37Protocol, nil)
			}
			if err := checkpointRuntimeV37(runtimePath, &runtimeState, claim); err != nil {
				return haltV37(ledgerPath, ledger, v37Evidence, err)
			}
			continue
		}
		if runtimeState.State.CompletedPhases >= d.Caps.MaxProviderInvocations {
			return closeV37(root, ledgerPath, ledger, "failed", v37Protocol, nil)
		}
		transportPlan, request, requestErr := agentqualification.BuildClaudeSubscriptionRealisticV37PhaseRequestFromState(runtimeState.Workspace, claim.RunFamilyID, runtimeState.State)
		if requestErr != nil {
			return haltV37(ledgerPath, ledger, v37Evidence, requestErr)
		}
		requestRaw, _ := json.Marshal(request)
		claim.Phase = request.ProviderTurn
		claim.InFlight = true
		claim.RequestSHA256 = hashV37Bytes(requestRaw)
		if err := writeAtomicV37(ledgerPath, ledger); err != nil {
			return ledger, err
		}
		candidate, receipt, callErr := provider.Plan(ctx, transportPlan, request)
		if callErr != nil {
			kind := classifyV37(callErr)
			if kind.global() {
				closed, closeErr := closeV37(root, ledgerPath, ledger, "invalid", kind, nil)
				if closeErr != nil {
					return haltV37(ledgerPath, ledger, v37Evidence, closeErr)
				}
				return haltV37(ledgerPath, closed, kind, callErr)
			}
			outcome := "failed"
			if kind == v37Transport {
				outcome = "invalid"
			}
			return closeV37(root, ledgerPath, ledger, outcome, kind, nil)
		}
		artifact := struct {
			Request agentqualification.RealisticProviderRequest `json:"request"`
			Plan    agentqualification.ClaudeRealisticV37Plan   `json:"plan"`
			Receipt json.RawMessage                             `json:"receipt"`
		}{Request: request, Plan: candidate, Receipt: append(json.RawMessage(nil), receipt...)}
		phasePath := filepath.Join(cell, fmt.Sprintf("phase-%02d-paid.json", request.ProviderTurn))
		if err := writeExclusiveV37(phasePath, artifact); err != nil {
			return haltV37(ledgerPath, ledger, v37Evidence, err)
		}
		next, executeErr := agentqualification.ExecuteClaudeSubscriptionRealisticV37Phase(runtimeState.Workspace, claim.RunFamilyID, request.ProviderTurn, candidate, receipt, runtimeState.State, func(state agentqualification.ClaudeRealisticV37ExecutionState) error {
			runtimeState.State = state
			return checkpointRuntimeV37(runtimePath, &runtimeState, claim)
		})
		runtimeState.State = next
		if executeErr != nil {
			kind := v37Evidence
			if strings.Contains(executeErr.Error(), "resource ceiling") {
				kind = v37Resource
			} else if strings.Contains(executeErr.Error(), "action count") || strings.Contains(executeErr.Error(), "not allowed") || strings.Contains(executeErr.Error(), "finish policy") {
				kind = v37Protocol
			}
			if kind.global() {
				return haltV37(ledgerPath, ledger, kind, executeErr)
			}
			return closeV37(root, ledgerPath, ledger, "failed", kind, nil)
		}
		claim.InFlight = false
		claim.Usage = usageV37(next.Controller)
		if err := checkpointRuntimeV37(runtimePath, &runtimeState, claim); err != nil {
			return haltV37(ledgerPath, ledger, v37Evidence, err)
		}
	}
}

func completeV37(root, ledgerPath string, ledger v37Ledger, runtimeState *v37Runtime) (v37Ledger, error) {
	claim := ledger.Claim
	cell := filepath.Join(root, "observations", "R03")
	evidenceRoot := filepath.Join(cell, "evidence")
	receipt, receiptPath, err := agentqualification.SealClaudeSubscriptionRealisticV37Execution(evidenceRoot, runtimeState.Workspace, runtimeState.State.Controller)
	if err != nil {
		return haltV37(ledgerPath, ledger, v37Evidence, err)
	}
	claim.ExecutionSHA256, _ = fileHashV37(receiptPath)
	grade, err := agentqualification.GradeClaudeSubscriptionRealisticV37(runtimeState.Workspace, receipt, runtimeState.State.Controller.FinalSummary)
	if err != nil {
		return haltV37(ledgerPath, ledger, v37Evidence, err)
	}
	if err := writeExclusiveV37(filepath.Join(cell, "grade.json"), grade); err != nil {
		return haltV37(ledgerPath, ledger, v37Evidence, err)
	}
	claim.GradeSHA256, _ = fileHashV37(filepath.Join(cell, "grade.json"))
	if grade.Verdict != "PASS" {
		return closeV37(root, ledgerPath, ledger, "failed", v37Competence, &grade)
	}
	return closeV37(root, ledgerPath, ledger, "pass", "", &grade)
}

func closeV37(root, ledgerPath string, ledger v37Ledger, outcome string, kind v37StopClass, grade *agentqualification.RealisticWorkspaceGrade) (v37Ledger, error) {
	claim := ledger.Claim
	claim.Status, claim.Outcome, claim.FailureClass, claim.InFlight = "completed", outcome, string(kind), false
	claim.Usage = usageV37FromClaim(claim.Usage)
	result := v37Result{Claim: *claim, Grade: grade}
	resultPath := filepath.Join(root, "observations", "R03", "result.json")
	if err := writeExclusiveV37(resultPath, result); err != nil {
		return haltV37(ledgerPath, ledger, v37Evidence, err)
	}
	claim.ResultSHA256, _ = fileHashV37(resultPath)
	if kind == v37Competence || kind == v37Protocol || kind == v37Resource {
		ledger.CandidateEliminated = true
	}
	ledger.Status = "complete"
	if err := writeAtomicV37(ledgerPath, ledger); err != nil {
		return ledger, err
	}
	return ledger, nil
}

func haltV37(path string, ledger v37Ledger, kind v37StopClass, cause error) (v37Ledger, error) {
	ledger.GlobalHalt, ledger.GlobalHaltReason, ledger.Status = true, string(kind), "halted"
	_ = writeAtomicV37(path, ledger)
	if cause == nil {
		cause = errors.New(string(kind))
	}
	return ledger, failV37(kind, cause)
}

func loadV37Ledger(path string, d agentqualification.ClaudeSubscriptionRealisticV37Definition) (v37Ledger, error) {
	var ledger v37Ledger
	if err := readJSONV37(path, &ledger); err == nil {
		if ledger.SchemaVersion != v37LedgerSchema || ledger.BatchID != d.BatchID || ledger.DefinitionSHA256 != agentqualification.ClaudeSubscriptionRealisticV37SHA256() {
			return ledger, fmt.Errorf("v37 ledger identity drifted")
		}
		return ledger, nil
	} else if !os.IsNotExist(err) {
		return ledger, err
	}
	ledger = v37Ledger{SchemaVersion: v37LedgerSchema, BatchID: d.BatchID, DefinitionSHA256: agentqualification.ClaudeSubscriptionRealisticV37SHA256(), Status: "running"}
	return ledger, writeExclusiveV37(path, ledger)
}

func checkpointRuntimeV37(path string, state *v37Runtime, claim *v37Claim) error {
	if err := writeAtomicV37(path, state); err != nil {
		return err
	}
	claim.RuntimeSHA256, _ = fileHashV37(path)
	return nil
}

func usageV37(result agentqualification.RealisticControllerResult) v37Usage {
	return v37Usage{ProviderInvocations: result.ProviderCalls, InputTokens: result.InputTokens, OutputTokens: result.OutputTokens, CostMicros: result.CostMicros}
}

func usageV37FromClaim(usage v37Usage) v37Usage { return usage }

func readJSONV37(path string, out any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	return nil
}

func writeExclusiveV37(path string, value any) error {
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

func writeAtomicV37(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".v37-*.tmp")
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

func fileHashV37(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return hashV37Bytes(raw), nil
}

func hashV37(value string) string { return hashV37Bytes([]byte(value)) }
func hashV37Bytes(raw []byte) string {
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}
