package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

const v34ExecutorVersion = "claude-subscription-static-v34.v1"

type v34Usage struct {
	ProviderInvocations        int   `json:"provider_invocations"`
	InputTokens                int64 `json:"input_tokens"`
	OutputTokens               int64 `json:"output_tokens"`
	ProviderReportedCostMicros int64 `json:"provider_reported_cost_micros"`
	UnknownUsageInvocations    int   `json:"unknown_usage_invocations"`
	ConservativeReservedTokens int64 `json:"conservative_reserved_tokens"`
}
type v34Claim struct {
	BindingID          string   `json:"binding_id"`
	CaseID             string   `json:"case_id"`
	CaseVersion        string   `json:"case_version"`
	Status             string   `json:"status"`
	Result             string   `json:"result,omitempty"`
	FailureClass       string   `json:"failure_class,omitempty"`
	ReceiptSHA256      string   `json:"receipt_sha256,omitempty"`
	GradeSHA256        string   `json:"grade_sha256,omitempty"`
	InitialClaimSHA256 string   `json:"initial_claim_sha256,omitempty"`
	CapsuleSHA256      string   `json:"capsule_sha256,omitempty"`
	RawSHA256          string   `json:"raw_sha256,omitempty"`
	ResponseSHA256     string   `json:"response_sha256,omitempty"`
	ResultSHA256       string   `json:"result_sha256,omitempty"`
	UsageStatus        string   `json:"usage_status"`
	Usage              v34Usage `json:"usage"`
}
type v34Ledger struct {
	SchemaVersion       string              `json:"schema_version"`
	BatchID             string              `json:"batch_id"`
	DefinitionSHA256    string              `json:"definition_sha256"`
	FreezeSHA256        string              `json:"freeze_sha256"`
	Status              string              `json:"status"`
	Claims              map[string]v34Claim `json:"claims"`
	Usage               v34Usage            `json:"usage"`
	CandidateEliminated bool                `json:"candidate_eliminated"`
	GlobalHalt          bool                `json:"global_halt"`
	GlobalHaltReason    string              `json:"global_halt_reason,omitempty"`
}
type v34ResultEnvelope struct {
	Claim                   v34Claim `json:"claim"`
	WallMilliseconds        int64    `json:"wall_milliseconds,omitempty"`
	Retries                 int      `json:"retries"`
	ReconciledStrandedClaim bool     `json:"reconciled_stranded_claim,omitempty"`
}
type v34LaunchCapsule struct {
	BindingID         string `json:"binding_id"`
	DefinitionSHA256  string `json:"definition_sha256"`
	CandidateID       string `json:"candidate_id"`
	CaseID            string `json:"case_id"`
	CaseVersion       string `json:"case_version"`
	Model             string `json:"model"`
	Effort            string `json:"reasoning_effort"`
	ServingProvider   string `json:"serving_provider"`
	ProviderFallbacks bool   `json:"provider_fallbacks"`
	Transport         string `json:"transport"`
	PromptSHA256      string `json:"prompt_sha256"`
	SchemaSHA256      string `json:"schema_sha256"`
	MaxTotalTokens    int64  `json:"max_total_tokens"`
	MaxOutputTokens   int64  `json:"max_output_tokens"`
	MaxRetries        int    `json:"max_retries"`
	RuntimeVersion    string `json:"runtime_version"`
	ExecutableSHA256  string `json:"executable_sha256"`
	RouteEvidence     string `json:"route_evidence"`
}
type v34FailureKind string

const (
	v34CredentialFailure    v34FailureKind = "credential_failure"
	v34UnsafeExternal       v34FailureKind = "unsafe_external_action"
	v34EvidenceCorruption   v34FailureKind = "evidence_corruption"
	v34CampaignCap          v34FailureKind = "campaign_cap"
	v34ObservationTransport v34FailureKind = "observation_transport_invalid"
	v34CandidateFailure     v34FailureKind = "candidate_protocol_failure"
)

type v34Failure struct {
	Kind  v34FailureKind
	Cause error
}

func (e *v34Failure) Error() string                  { return string(e.Kind) + ": " + e.Cause.Error() }
func (e *v34Failure) Unwrap() error                  { return e.Cause }
func failV34(kind v34FailureKind, cause error) error { return &v34Failure{Kind: kind, Cause: cause} }
func kindV34(err error) (v34FailureKind, bool) {
	var typed *v34Failure
	ok := errors.As(err, &typed)
	if !ok {
		return "", false
	}
	return typed.Kind, true
}

type v34Call func(context.Context, agentqualification.ClaudeScreeningPlan) (agentqualification.ProviderExecutionResult, []byte, []byte, time.Duration, bool, error)

func executeV34(ctx context.Context, root string, call v34Call) (v34Ledger, error) {
	d, err := agentqualification.ClaudeSubscriptionStaticV34()
	if err != nil {
		return v34Ledger{}, err
	}
	if !filepath.IsAbs(root) {
		return v34Ledger{}, fmt.Errorf("v34 evidence root must be absolute")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return v34Ledger{}, err
	}
	ledgerPath := filepath.Join(root, "ledger.json")
	ledger, err := loadOrCreateLedger(ledgerPath, d)
	if err != nil {
		return v34Ledger{}, err
	}
	if ledger.GlobalHalt || ledger.Status == "complete" {
		return ledger, nil
	}
	for i, c := range d.Cases {
		binding := v34Binding(d.BatchID, i, c)
		if existing, ok := ledger.Claims[binding]; ok {
			if existing.Status == "pending" {
				if err := reconcileStrandedV34(root, &ledger, binding, d.Caps.MaxTotalReportedTokensPerCell); err != nil {
					return haltV34(ledgerPath, ledger, string(v34EvidenceCorruption)+": "+err.Error())
				}
				if total, ok := totalReportedV34(ledger.Usage); !ok || total > d.Caps.MaxCampaignReportedTokens {
					return haltV34(ledgerPath, ledger, "uncontrolled_campaign_cap")
				}
				if err := writeJSONAtomic(ledgerPath, ledger); err != nil {
					return ledger, err
				}
			}
			continue
		}
		if ledger.CandidateEliminated {
			claim := v34Claim{BindingID: binding, CaseID: c.CaseID, CaseVersion: c.CaseVersion, Status: "skipped", Result: "candidate_eliminated", UsageStatus: "not_launched"}
			claim, err = sealClaimAndResult(root, claim)
			if err != nil {
				return haltV34(ledgerPath, ledger, "evidence_corruption: "+err.Error())
			}
			ledger.Claims[binding] = claim
			if err := writeJSONAtomic(ledgerPath, ledger); err != nil {
				return ledger, err
			}
			continue
		}
		used, ok := totalReportedV34(ledger.Usage)
		if !ok || ledger.Usage.ProviderInvocations >= d.Caps.MaxProviderInvocations || used > d.Caps.MaxCampaignReportedTokens-d.Caps.MaxTotalReportedTokensPerCell {
			return haltV34(ledgerPath, ledger, "uncontrolled_campaign_cap")
		}
		plan, err := agentqualification.BuildClaudeSubscriptionScreeningPlan(d.Configuration, c)
		if err != nil {
			return haltV34(ledgerPath, ledger, "evidence_corruption: "+err.Error())
		}
		cellDir := filepath.Join(root, binding)
		if err := os.Mkdir(cellDir, 0o700); err != nil {
			return haltV34(ledgerPath, ledger, "evidence_corruption: duplicate cell directory")
		}
		claim := v34Claim{BindingID: binding, CaseID: c.CaseID, CaseVersion: c.CaseVersion, Status: "pending", UsageStatus: "pending"}
		if err := writeJSONExclusive(filepath.Join(cellDir, "claim.json"), claim); err != nil {
			return haltV34(ledgerPath, ledger, "evidence_corruption: "+err.Error())
		}
		claim.InitialClaimSHA256, _ = fileHash(filepath.Join(cellDir, "claim.json"))
		capsule := expectedV34Capsule(d, binding, c, plan)
		if err := writeJSONExclusive(filepath.Join(cellDir, "launch-capsule.json"), capsule); err != nil {
			return haltV34(ledgerPath, ledger, "evidence_corruption: "+err.Error())
		}
		claim.CapsuleSHA256, _ = fileHash(filepath.Join(cellDir, "launch-capsule.json"))
		ledger.Claims[binding] = claim
		if err := writeJSONAtomic(ledgerPath, ledger); err != nil {
			return ledger, err
		}
		providerResult, candidate, raw, wall, launched, callErr := call(ctx, plan)
		if err := writeExclusive(filepath.Join(cellDir, "provider-raw.json"), raw); err != nil {
			return haltV34(ledgerPath, ledger, "evidence_corruption: "+err.Error())
		}
		claim.RawSHA256, _ = fileHash(filepath.Join(cellDir, "provider-raw.json"))
		if callErr != nil {
			failure, typed := kindV34(callErr)
			if !typed {
				failure = v34EvidenceCorruption
			}
			claim.Status, claim.Result, claim.FailureClass, claim.UsageStatus = "completed", "invalid_unscored", string(failure), "not_launched"
			if launched {
				claim.Usage = v34Usage{ProviderInvocations: 1, UnknownUsageInvocations: 1, ConservativeReservedTokens: d.Caps.MaxTotalReportedTokensPerCell}
				claim.UsageStatus = "unknown_conservatively_reserved"
				if err := addUsage(&ledger.Usage, claim.Usage); err != nil {
					return haltV34(ledgerPath, ledger, "uncontrolled_campaign_cap")
				}
			}
			if failure == v34CandidateFailure {
				claim.Result = "failed"
				ledger.CandidateEliminated = true
			}
			resultPath := filepath.Join(cellDir, "result.json")
			if err := writeJSONExclusive(resultPath, v34ResultEnvelope{Claim: claim, WallMilliseconds: wall.Milliseconds()}); err != nil {
				return haltV34(ledgerPath, ledger, "evidence_corruption: "+err.Error())
			}
			claim.ResultSHA256, _ = fileHash(resultPath)
			ledger.Claims[binding] = claim
			if failure == v34CredentialFailure || failure == v34UnsafeExternal || failure == v34EvidenceCorruption || failure == v34CampaignCap {
				return haltV34(ledgerPath, ledger, string(failure))
			}
			if err := writeJSONAtomic(ledgerPath, ledger); err != nil {
				return ledger, err
			}
			continue
		}
		if providerResult.Usage.InputTokens == nil || providerResult.Usage.OutputTokens == nil || providerResult.Usage.ProviderReportedCostMicros == nil {
			if err := sealFailedLaunchV34(cellDir, &ledger, binding, &claim, v34Usage{ProviderInvocations: 1, UnknownUsageInvocations: 1, ConservativeReservedTokens: d.Caps.MaxTotalReportedTokensPerCell}, "unknown_conservatively_reserved", string(v34EvidenceCorruption), wall); err != nil {
				return haltV34(ledgerPath, ledger, "evidence_corruption: "+err.Error())
			}
			return haltV34(ledgerPath, ledger, "evidence_corruption: missing usage")
		}
		if !launched {
			return haltV34(ledgerPath, ledger, "evidence_corruption: completed provider result without a launch")
		}
		usage := v34Usage{ProviderInvocations: 1, InputTokens: *providerResult.Usage.InputTokens, OutputTokens: *providerResult.Usage.OutputTokens, ProviderReportedCostMicros: *providerResult.Usage.ProviderReportedCostMicros}
		if usage.InputTokens < 0 || usage.OutputTokens < 0 || usage.ProviderReportedCostMicros < 0 {
			if err := sealFailedLaunchV34(cellDir, &ledger, binding, &claim, v34Usage{ProviderInvocations: 1, UnknownUsageInvocations: 1, ConservativeReservedTokens: d.Caps.MaxTotalReportedTokensPerCell}, "unknown_conservatively_reserved", string(v34EvidenceCorruption), wall); err != nil {
				return haltV34(ledgerPath, ledger, "evidence_corruption: "+err.Error())
			}
			return haltV34(ledgerPath, ledger, "evidence_corruption: negative usage")
		}
		reservation := agentqualification.ScreeningReservation{SchemaVersion: agentqualification.ScreeningReservationSchemaVersion, BatchID: d.BatchID, EvaluationBinding: binding, CandidateID: d.Configuration.ID, CaseID: c.CaseID, AdmissionInputSHA256: agentqualification.ClaudeSubscriptionStaticV34SHA256(), ReservedAt: time.Now().UTC().Format(time.RFC3339Nano), State: "reserved"}
		assignment := agentqualification.ScreeningAssignment{BatchID: d.BatchID, CaseID: c.CaseID, CaseVersion: c.CaseVersion, Attempt: 1, Configuration: d.Configuration, CandidatePacketSHA256: c.CandidatePacketSHA256, EvaluationBinding: binding}
		adapter := agentqualification.ExecutionAdapterIdentity{Provider: "anthropic", Runtime: "claude-code", RuntimeVersion: d.RuntimeIdentity.VersionOutput, Endpoint: "local-cli-print-stdin", BillingBoundary: d.BillingBoundary, AccountLane: "owner_subscription", ToolPolicy: "none", ProviderEventPolicyVersion: "claude-code-json-result.v1", ExecutorSHA256: hashText(v34ExecutorVersion), IsolationPolicySHA256: hashText(d.Transport + "|empty-workspace|restricted|tools-empty|mcp-empty|no-session|prompt-stdin"), EnvironmentAllowlistSHA256: hashText(strings.Join(v34EnvironmentNames(), "\n"))}
		if err := writeExclusive(filepath.Join(cellDir, "candidate-response.json"), candidate); err != nil {
			return haltV34(ledgerPath, ledger, "evidence_corruption: "+err.Error())
		}
		claim.ResponseSHA256, _ = fileHash(filepath.Join(cellDir, "candidate-response.json"))
		receipt, err := agentqualification.BuildExecutionReceipt(reservation, assignment, agentqualification.ClaudeExecutionPlanBinding(d.BatchID, plan), adapter, providerResult)
		if err != nil {
			if sealErr := sealFailedLaunchV34(cellDir, &ledger, binding, &claim, usage, "exact", string(v34EvidenceCorruption), wall); sealErr != nil {
				return haltV34(ledgerPath, ledger, "evidence_corruption: "+sealErr.Error())
			}
			return haltV34(ledgerPath, ledger, "evidence_corruption: "+err.Error())
		}
		receiptPath := filepath.Join(cellDir, "execution-receipt.json")
		if err := writeJSONExclusive(receiptPath, receipt); err != nil {
			return haltV34(ledgerPath, ledger, "evidence_corruption: "+err.Error())
		}
		cellTotal, cellTotalOK := totalReportedV34(usage)
		if !cellTotalOK {
			return haltV34(ledgerPath, ledger, "evidence_corruption: usage overflow")
		}
		if cellTotal > d.Caps.MaxTotalReportedTokensPerCell || usage.OutputTokens > d.Caps.MaxOutputTokensPerCell {
			claim.Status, claim.Result, claim.FailureClass, claim.Usage, claim.UsageStatus = "completed", "invalid_unscored", "observation_resource_exhaustion", usage, "exact"
			claim.ReceiptSHA256, _ = fileHash(receiptPath)
			resultPath := filepath.Join(cellDir, "result.json")
			if err := writeJSONExclusive(resultPath, v34ResultEnvelope{Claim: claim, WallMilliseconds: wall.Milliseconds()}); err != nil {
				return haltV34(ledgerPath, ledger, "evidence_corruption: "+err.Error())
			}
			claim.ResultSHA256, _ = fileHash(resultPath)
			ledger.Claims[binding] = claim
			if err := addUsage(&ledger.Usage, usage); err != nil {
				return haltV34(ledgerPath, ledger, "uncontrolled_campaign_cap")
			}
			if total, ok := totalReportedV34(ledger.Usage); !ok || total > d.Caps.MaxCampaignReportedTokens {
				return haltV34(ledgerPath, ledger, "uncontrolled_campaign_cap")
			}
			if err := writeJSONAtomic(ledgerPath, ledger); err != nil {
				return ledger, err
			}
			continue
		}
		grade, err := agentqualification.GradeBlindVersion(c.CaseID, c.CaseVersion, candidate)
		if err != nil {
			claim.Status, claim.Result, claim.FailureClass, claim.Usage, claim.UsageStatus = "completed", "failed", "candidate_protocol_failure", usage, "exact"
			ledger.CandidateEliminated = true
		} else {
			gradePath := filepath.Join(cellDir, "blind-grade.json")
			if err := writeJSONExclusive(gradePath, grade); err != nil {
				return haltV34(ledgerPath, ledger, "evidence_corruption: "+err.Error())
			}
			claim.GradeSHA256, _ = fileHash(gradePath)
			claim.Status, claim.Result, claim.Usage, claim.UsageStatus = "completed", strings.ToLower(grade.Verdict), usage, "exact"
			if grade.Verdict != "PASS" {
				claim.FailureClass = "candidate_competence_or_disqualifier_failure"
				ledger.CandidateEliminated = true
			}
		}
		claim.ReceiptSHA256, _ = fileHash(receiptPath)
		resultPath := filepath.Join(cellDir, "result.json")
		if err := writeJSONExclusive(resultPath, v34ResultEnvelope{Claim: claim, WallMilliseconds: wall.Milliseconds()}); err != nil {
			return haltV34(ledgerPath, ledger, "evidence_corruption: "+err.Error())
		}
		claim.ResultSHA256, _ = fileHash(resultPath)
		ledger.Claims[binding] = claim
		if err := addUsage(&ledger.Usage, usage); err != nil {
			return haltV34(ledgerPath, ledger, "uncontrolled_campaign_cap")
		}
		if total, ok := totalReportedV34(ledger.Usage); !ok || total > d.Caps.MaxCampaignReportedTokens {
			return haltV34(ledgerPath, ledger, "uncontrolled_campaign_cap")
		}
		if err := writeJSONAtomic(ledgerPath, ledger); err != nil {
			return ledger, err
		}
	}
	ledger.Status = "complete"
	if err := writeJSONAtomic(ledgerPath, ledger); err != nil {
		return ledger, err
	}
	return ledger, nil
}

func expectedV34Capsule(d agentqualification.ClaudeSubscriptionStaticV34Definition, binding string, c agentqualification.ScreeningCase, plan agentqualification.ClaudeScreeningPlan) v34LaunchCapsule {
	return v34LaunchCapsule{
		BindingID: binding, DefinitionSHA256: agentqualification.ClaudeSubscriptionStaticV34SHA256(), CandidateID: d.Configuration.ID,
		CaseID: c.CaseID, CaseVersion: c.CaseVersion, Model: d.Configuration.Model, Effort: d.Configuration.ReasoningEffort,
		ServingProvider: d.Configuration.ServingProvider, ProviderFallbacks: false, Transport: d.Transport,
		PromptSHA256: plan.PromptSHA256, SchemaSHA256: plan.SchemaSHA256, MaxTotalTokens: d.Caps.MaxTotalReportedTokensPerCell,
		MaxOutputTokens: d.Caps.MaxOutputTokensPerCell, MaxRetries: 0, RuntimeVersion: d.RuntimeIdentity.VersionOutput,
		ExecutableSHA256: d.RuntimeIdentity.ExecutableSHA256, RouteEvidence: d.RuntimeIdentity.RouteEvidence,
	}
}

func realClaudeV34Call(ctx context.Context, plan agentqualification.ClaudeScreeningPlan) (agentqualification.ProviderExecutionResult, []byte, []byte, time.Duration, bool, error) {
	executable := "claude"
	if runtime.GOOS == "windows" {
		executable = "claude.cmd"
	}
	path, err := exec.LookPath(executable)
	if err != nil {
		return agentqualification.ProviderExecutionResult{}, nil, nil, 0, false, failV34(v34CredentialFailure, err)
	}
	workspace, err := os.MkdirTemp("", "sm-claude-v34-")
	if err != nil {
		return agentqualification.ProviderExecutionResult{}, nil, nil, 0, false, failV34(v34EvidenceCorruption, err)
	}
	defer os.RemoveAll(workspace)
	args := make([]string, 0, len(plan.ArgsTemplate))
	for _, arg := range plan.ArgsTemplate {
		if arg == "<generated-schema>" {
			arg = string(plan.Schema)
		}
		args = append(args, arg)
	}
	started := time.Now().UTC()
	command := exec.CommandContext(ctx, path, args...)
	command.Dir = workspace
	command.Env = v34Environment()
	command.Stdin = strings.NewReader(plan.Prompt)
	var stdout, stderr strings.Builder
	command.Stdout = &stdout
	command.Stderr = &stderr
	err = command.Run()
	wall := time.Since(started)
	raw := []byte(stdout.String())
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return agentqualification.ProviderExecutionResult{}, nil, append(raw, []byte(stderr.String())...), wall, true, failV34(v34CampaignCap, context.DeadlineExceeded)
		}
		return agentqualification.ProviderExecutionResult{}, nil, append(raw, []byte(stderr.String())...), wall, true, failV34(v34ObservationTransport, err)
	}
	result, candidate, err := agentqualification.ParseClaudeCodeSubscriptionJSONArtifact(raw, started.Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return result, candidate, raw, wall, true, failV34(v34ObservationTransport, err)
	}
	return result, candidate, raw, wall, true, nil
}

func loadOrCreateLedger(path string, d agentqualification.ClaudeSubscriptionStaticV34Definition) (v34Ledger, error) {
	var l v34Ledger
	raw, err := os.ReadFile(path)
	if err == nil {
		if json.Unmarshal(raw, &l) != nil {
			return l, errors.New("invalid ledger")
		}
		return l, validateLedger(l, d)
	}
	if !os.IsNotExist(err) {
		return l, err
	}
	l = v34Ledger{SchemaVersion: "silver-mediums.claude-subscription-static-ledger.v34", BatchID: d.BatchID, DefinitionSHA256: agentqualification.ClaudeSubscriptionStaticV34SHA256(), FreezeSHA256: agentqualification.ClaudeSubscriptionStaticV34FreezeSHA256(), Status: "active", Claims: map[string]v34Claim{}}
	return l, writeJSONExclusive(path, l)
}
func validateLedger(l v34Ledger, d agentqualification.ClaudeSubscriptionStaticV34Definition) error {
	if l.SchemaVersion != "silver-mediums.claude-subscription-static-ledger.v34" || l.BatchID != d.BatchID || l.DefinitionSHA256 != agentqualification.ClaudeSubscriptionStaticV34SHA256() || l.FreezeSHA256 != agentqualification.ClaudeSubscriptionStaticV34FreezeSHA256() {
		return errors.New("ledger identity drift")
	}
	var u v34Usage
	for _, c := range l.Claims {
		if err := addUsage(&u, c.Usage); err != nil {
			return errors.New("ledger accounting overflow")
		}
	}
	if u != l.Usage {
		return errors.New("ledger accounting drift")
	}
	if total, ok := totalReportedV34(l.Usage); !ok || total > d.Caps.MaxCampaignReportedTokens || l.Usage.ProviderInvocations > d.Caps.MaxProviderInvocations || l.Usage.UnknownUsageInvocations > l.Usage.ProviderInvocations {
		return errors.New("ledger cap/accounting drift")
	}
	return nil
}

func reconcileStrandedV34(root string, ledger *v34Ledger, binding string, reserve int64) error {
	claim := ledger.Claims[binding]
	if claim.Status != "pending" {
		return errors.New("claim is not pending")
	}
	dir := filepath.Join(root, binding)
	claimPath := filepath.Join(dir, "claim.json")
	if claim.InitialClaimSHA256 == "" {
		var err error
		claim.InitialClaimSHA256, err = fileHash(claimPath)
		if err != nil {
			return err
		}
	}
	if claim.CapsuleSHA256 == "" {
		var err error
		claim.CapsuleSHA256, err = fileHash(filepath.Join(dir, "launch-capsule.json"))
		if err != nil {
			return err
		}
	}
	rawPath := filepath.Join(dir, "provider-raw.json")
	if _, err := os.Stat(rawPath); os.IsNotExist(err) {
		if err := writeExclusive(rawPath, []byte("{\"status\":\"unavailable_after_stranded_claim\"}\n")); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	var err error
	claim.RawSHA256, err = fileHash(rawPath)
	if err != nil {
		return err
	}
	claim.Status, claim.Result, claim.FailureClass, claim.UsageStatus = "completed", "invalid_unscored", string(v34ObservationTransport), "unknown_conservatively_reserved"
	claim.Usage = v34Usage{ProviderInvocations: 1, UnknownUsageInvocations: 1, ConservativeReservedTokens: reserve}
	resultPath := filepath.Join(dir, "result.json")
	if err := writeJSONExclusive(resultPath, v34ResultEnvelope{Claim: claim, ReconciledStrandedClaim: true}); err != nil {
		return err
	}
	claim.ResultSHA256, err = fileHash(resultPath)
	if err != nil {
		return err
	}
	ledger.Claims[binding] = claim
	return addUsage(&ledger.Usage, claim.Usage)
}

func sealFailedLaunchV34(cellDir string, ledger *v34Ledger, binding string, claim *v34Claim, usage v34Usage, usageStatus, failure string, wall time.Duration) error {
	claim.Status, claim.Result, claim.FailureClass, claim.Usage, claim.UsageStatus = "completed", "invalid_unscored", failure, usage, usageStatus
	resultPath := filepath.Join(cellDir, "result.json")
	if err := writeJSONExclusive(resultPath, v34ResultEnvelope{Claim: *claim, WallMilliseconds: wall.Milliseconds()}); err != nil {
		return err
	}
	var err error
	claim.ResultSHA256, err = fileHash(resultPath)
	if err != nil {
		return err
	}
	ledger.Claims[binding] = *claim
	return addUsage(&ledger.Usage, usage)
}

func replayV34(root string) (map[string]any, error) {
	d, err := agentqualification.ClaudeSubscriptionStaticV34()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(root, "ledger.json"))
	if err != nil {
		return nil, err
	}
	var l v34Ledger
	if err = json.Unmarshal(raw, &l); err != nil {
		return nil, err
	}
	if err = validateLedger(l, d); err != nil {
		return nil, err
	}
	expected := make(map[string]agentqualification.ScreeningCase, len(d.Cases))
	for index, screeningCase := range d.Cases {
		expected[v34Binding(d.BatchID, index, screeningCase)] = screeningCase
	}
	if l.Status == "complete" && len(l.Claims) != len(expected) {
		return nil, errors.New("complete ledger does not contain every frozen claim")
	}
	for binding, c := range l.Claims {
		screeningCase, ok := expected[binding]
		if !ok || c.BindingID != binding || c.CaseID != screeningCase.CaseID || c.CaseVersion != screeningCase.CaseVersion {
			return nil, errors.New("claim identity or frozen order drift")
		}
		dir := filepath.Join(root, binding)
		if c.Status == "pending" {
			return nil, errors.New("ambiguous pending claim")
		}
		initialPath := filepath.Join(dir, "claim.json")
		if err := verifyHashV34(initialPath, c.InitialClaimSHA256, "initial claim"); err != nil {
			return nil, err
		}
		var initial v34Claim
		if err := readJSONV34(initialPath, &initial); err != nil {
			return nil, err
		}
		wantInitial := v34Claim{BindingID: binding, CaseID: c.CaseID, CaseVersion: c.CaseVersion, Status: "pending", UsageStatus: "pending"}
		if c.Status == "skipped" {
			wantInitial.Status, wantInitial.Result, wantInitial.UsageStatus = "skipped", "candidate_eliminated", "not_launched"
		}
		if !reflect.DeepEqual(initial, wantInitial) {
			return nil, errors.New("initial claim semantic drift")
		}
		resultPath := filepath.Join(dir, "result.json")
		if err := verifyHashV34(resultPath, c.ResultSHA256, "result"); err != nil {
			return nil, err
		}
		var result v34ResultEnvelope
		if err := readJSONV34(resultPath, &result); err != nil {
			return nil, err
		}
		wantResultClaim := c
		wantResultClaim.ResultSHA256 = ""
		if !reflect.DeepEqual(result.Claim, wantResultClaim) || result.Retries != 0 {
			return nil, errors.New("result and ledger claim drift")
		}
		if c.Status == "skipped" {
			if c.UsageStatus != "not_launched" || c.Usage != (v34Usage{}) || c.CapsuleSHA256 != "" || c.RawSHA256 != "" || c.ReceiptSHA256 != "" || c.ResponseSHA256 != "" || c.GradeSHA256 != "" {
				return nil, errors.New("skipped claim contains launch evidence")
			}
			continue
		}
		plan, err := agentqualification.BuildClaudeSubscriptionScreeningPlan(d.Configuration, screeningCase)
		if err != nil {
			return nil, err
		}
		capsulePath := filepath.Join(dir, "launch-capsule.json")
		if err := verifyHashV34(capsulePath, c.CapsuleSHA256, "launch capsule"); err != nil {
			return nil, err
		}
		var capsule v34LaunchCapsule
		if err := readJSONV34(capsulePath, &capsule); err != nil || !reflect.DeepEqual(capsule, expectedV34Capsule(d, binding, screeningCase, plan)) {
			return nil, errors.New("launch capsule drift")
		}
		rawPath := filepath.Join(dir, "provider-raw.json")
		if err := verifyHashV34(rawPath, c.RawSHA256, "provider raw"); err != nil {
			return nil, err
		}
		if c.UsageStatus == "not_launched" {
			if c.Usage != (v34Usage{}) || c.ReceiptSHA256 != "" || c.ResponseSHA256 != "" || c.GradeSHA256 != "" {
				return nil, errors.New("not-launched claim contains provider evidence")
			}
			continue
		}
		if c.Usage.ProviderInvocations != 1 {
			return nil, errors.New("launched claim is not counted exactly once")
		}
		if c.ReceiptSHA256 != "" {
			receiptPath := filepath.Join(dir, "execution-receipt.json")
			if err := verifyHashV34(receiptPath, c.ReceiptSHA256, "receipt"); err != nil {
				return nil, err
			}
			var receipt agentqualification.ExecutionReceipt
			if err := readJSONV34(receiptPath, &receipt); err != nil {
				return nil, err
			}
			if reasons := agentqualification.ValidateExecutionReceipt(receipt); len(reasons) != 0 || receipt.BatchID != d.BatchID || receipt.EvaluationBinding != binding || receipt.RequestedModel != d.Configuration.Model || receipt.RequestedEffort != d.Configuration.ReasoningEffort || receipt.Provider != "anthropic" || receipt.Runtime != "claude-code" || receipt.RuntimeVersion != d.RuntimeIdentity.VersionOutput || receipt.ActualServingProvider != "anthropic" || receipt.ProviderFallbacks == nil || *receipt.ProviderFallbacks || receipt.PromptSHA256 != plan.PromptSHA256 || receipt.SchemaSHA256 != plan.SchemaSHA256 {
				return nil, errors.New("receipt semantic or route drift")
			}
			if receipt.Usage.InputTokens == nil || receipt.Usage.OutputTokens == nil || receipt.Usage.ProviderReportedCostMicros == nil || *receipt.Usage.InputTokens != c.Usage.InputTokens || *receipt.Usage.OutputTokens != c.Usage.OutputTokens || *receipt.Usage.ProviderReportedCostMicros != c.Usage.ProviderReportedCostMicros || c.UsageStatus != "exact" {
				return nil, errors.New("receipt accounting drift")
			}
			if receipt.RawTraceSHA256 != c.RawSHA256 {
				return nil, errors.New("receipt raw trace drift")
			}
			responsePath := filepath.Join(dir, "candidate-response.json")
			if err := verifyHashV34(responsePath, c.ResponseSHA256, "candidate response"); err != nil {
				return nil, err
			}
			if receipt.OutputSHA256 != c.ResponseSHA256 {
				return nil, errors.New("receipt candidate output drift")
			}
		} else if c.UsageStatus == "unknown_conservatively_reserved" {
			if c.ResponseSHA256 != "" || c.GradeSHA256 != "" || c.Usage.UnknownUsageInvocations != 1 || c.Usage.ConservativeReservedTokens != d.Caps.MaxTotalReportedTokensPerCell {
				return nil, errors.New("unreceipted launch is not conservatively recorded")
			}
		} else if c.UsageStatus == "exact" && c.FailureClass == string(v34EvidenceCorruption) && c.ResponseSHA256 != "" && c.GradeSHA256 == "" {
			if err := verifyHashV34(filepath.Join(dir, "candidate-response.json"), c.ResponseSHA256, "candidate response"); err != nil {
				return nil, err
			}
		} else {
			return nil, errors.New("unreceipted launch accounting is invalid")
		}
		if c.GradeSHA256 != "" {
			gradePath := filepath.Join(dir, "blind-grade.json")
			if err := verifyHashV34(gradePath, c.GradeSHA256, "grade"); err != nil {
				return nil, err
			}
			response, err := os.ReadFile(filepath.Join(dir, "candidate-response.json"))
			if err != nil {
				return nil, err
			}
			grade, err := agentqualification.GradeBlindVersion(c.CaseID, c.CaseVersion, response)
			if err != nil {
				return nil, errors.New("sealed candidate response no longer grades")
			}
			var sealedGrade agentqualification.BlindGrade
			if err := readJSONV34(gradePath, &sealedGrade); err != nil || !reflect.DeepEqual(grade, sealedGrade) || strings.ToLower(grade.Verdict) != c.Result {
				return nil, errors.New("grade semantic drift")
			}
		}
	}
	return map[string]any{"status": "replay_valid", "provider_called": false, "ledger_sha256": hashText(string(raw)), "claims": len(l.Claims), "usage": l.Usage, "global_halt": l.GlobalHalt}, nil
}

func readJSONV34(path string, target any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("invalid sealed JSON %s: %w", filepath.Base(path), err)
	}
	return nil
}

func verifyHashV34(path, expected, label string) error {
	if expected == "" {
		return fmt.Errorf("%s hash missing", label)
	}
	h, err := fileHash(path)
	if err != nil || h != expected {
		return fmt.Errorf("%s drift", label)
	}
	return nil
}
func sealClaimAndResult(root string, c v34Claim) (v34Claim, error) {
	dir := filepath.Join(root, c.BindingID)
	if err := os.Mkdir(dir, 0o700); err != nil {
		return c, err
	}
	if err := writeJSONExclusive(filepath.Join(dir, "claim.json"), c); err != nil {
		return c, err
	}
	c.InitialClaimSHA256, _ = fileHash(filepath.Join(dir, "claim.json"))
	if err := writeJSONExclusive(filepath.Join(dir, "result.json"), v34ResultEnvelope{Claim: c}); err != nil {
		return c, err
	}
	c.ResultSHA256, _ = fileHash(filepath.Join(dir, "result.json"))
	return c, nil
}
func haltV34(path string, l v34Ledger, reason string) (v34Ledger, error) {
	l.GlobalHalt = true
	l.Status = "halted"
	l.GlobalHaltReason = reason
	if err := writeJSONAtomic(path, l); err != nil {
		return l, fmt.Errorf("%s; persist halt ledger: %w", reason, err)
	}
	return l, errors.New(reason)
}
func addUsage(dst *v34Usage, u v34Usage) error {
	if _, ok := totalReportedV34(*dst); !ok {
		return errors.New("existing usage is invalid")
	}
	if _, ok := totalReportedV34(u); !ok {
		return errors.New("incremental usage is invalid")
	}
	maxInt := int(^uint(0) >> 1)
	if dst.ProviderInvocations > maxInt-u.ProviderInvocations || dst.UnknownUsageInvocations > maxInt-u.UnknownUsageInvocations ||
		dst.InputTokens > int64(^uint64(0)>>1)-u.InputTokens || dst.OutputTokens > int64(^uint64(0)>>1)-u.OutputTokens ||
		dst.ProviderReportedCostMicros > int64(^uint64(0)>>1)-u.ProviderReportedCostMicros || dst.ConservativeReservedTokens > int64(^uint64(0)>>1)-u.ConservativeReservedTokens {
		return errors.New("usage addition overflow")
	}
	dst.ProviderInvocations += u.ProviderInvocations
	dst.InputTokens += u.InputTokens
	dst.OutputTokens += u.OutputTokens
	dst.ProviderReportedCostMicros += u.ProviderReportedCostMicros
	dst.UnknownUsageInvocations += u.UnknownUsageInvocations
	dst.ConservativeReservedTokens += u.ConservativeReservedTokens
	return nil
}
func totalReportedV34(u v34Usage) (int64, bool) {
	if u.ProviderInvocations < 0 || u.InputTokens < 0 || u.OutputTokens < 0 || u.ProviderReportedCostMicros < 0 || u.UnknownUsageInvocations < 0 || u.ConservativeReservedTokens < 0 || u.InputTokens > int64(^uint64(0)>>1)-u.OutputTokens {
		return 0, false
	}
	total := u.InputTokens + u.OutputTokens
	if total > int64(^uint64(0)>>1)-u.ConservativeReservedTokens {
		return 0, false
	}
	return total + u.ConservativeReservedTokens, true
}
func v34Binding(batch string, i int, c agentqualification.ScreeningCase) string {
	return hashText(fmt.Sprintf("%s|%02d|%s|%s", batch, i, c.CaseID, c.CaseVersion))
}
func hashText(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func fileHash(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return hashText(string(raw)), nil
}
func writeExclusive(path string, raw []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = f.Write(raw); err != nil {
		return err
	}
	return f.Sync()
}
func writeJSONExclusive(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeExclusive(path, append(raw, '\n'))
}
func writeJSONAtomic(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err = os.WriteFile(tmp, append(raw, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
func v34EnvironmentNames() []string {
	return []string{"APPDATA", "COMSPEC", "HOMEDRIVE", "HOMEPATH", "LOCALAPPDATA", "PATH", "PATHEXT", "SystemDrive", "SystemRoot", "TEMP", "TMP", "USERPROFILE", "WINDIR", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"}
}
func v34Environment() []string {
	var env []string
	for _, n := range v34EnvironmentNames() {
		if n == "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC" {
			env = append(env, n+"=1")
			continue
		}
		if v, ok := os.LookupEnv(n); ok && v != "" {
			env = append(env, n+"="+v)
		}
	}
	return env
}
