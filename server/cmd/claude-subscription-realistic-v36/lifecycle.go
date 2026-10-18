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
	"reflect"
	"strings"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

const v36LedgerSchema = "silver-mediums.claude-subscription-realistic-ledger.v36"

type v36Usage struct {
	ProviderTurns                  int   `json:"provider_turns"`
	InputTokens                    int64 `json:"input_tokens"`
	OutputTokens                   int64 `json:"output_tokens"`
	ReasoningTokens                int64 `json:"reasoning_tokens"`
	CostMicros                     int64 `json:"cost_equivalent_micros"`
	UnknownTurns                   int   `json:"unknown_turns"`
	ConservativeReservedTokens     int64 `json:"conservative_reserved_tokens"`
	ConservativeReservedGenerated  int64 `json:"conservative_reserved_generated_tokens"`
	ConservativeReservedCostMicros int64 `json:"conservative_reserved_cost_micros"`
}
type v36Claim struct {
	CaseID           string   `json:"case_id"`
	CaseVersion      string   `json:"case_version"`
	RunFamilyID      string   `json:"run_family_id"`
	Status           string   `json:"status"`
	Outcome          string   `json:"outcome,omitempty"`
	FailureClass     string   `json:"failure_class,omitempty"`
	RuntimeSHA256    string   `json:"runtime_sha256,omitempty"`
	ControllerSHA256 string   `json:"controller_sha256,omitempty"`
	ReceiptSHA256    string   `json:"receipt_sha256,omitempty"`
	GradeSHA256      string   `json:"grade_sha256,omitempty"`
	ResultSHA256     string   `json:"result_sha256,omitempty"`
	Usage            v36Usage `json:"usage"`
	Attempts         int      `json:"attempts"`
	Retries          int      `json:"retries"`
}
type v36Ledger struct {
	SchemaVersion       string              `json:"schema_version"`
	BatchID             string              `json:"batch_id"`
	DefinitionSHA256    string              `json:"definition_sha256"`
	Status              string              `json:"status"`
	Claims              map[string]v36Claim `json:"claims"`
	Usage               v36Usage            `json:"usage"`
	CandidateEliminated bool                `json:"candidate_eliminated"`
	GlobalHalt          bool                `json:"global_halt"`
	GlobalHaltReason    string              `json:"global_halt_reason,omitempty"`
}
type v36Runtime struct {
	Workspace  agentqualification.RealisticCaseWorkspace    `json:"workspace"`
	Controller agentqualification.RealisticControllerResult `json:"controller"`
}
type v36Result struct {
	Claim v36Claim                                    `json:"claim"`
	Grade *agentqualification.RealisticWorkspaceGrade `json:"grade,omitempty"`
}

type v36FailureKind string

const (
	v36Credential         v36FailureKind = "credential_failure"
	v36Unsafe             v36FailureKind = "unsafe_external_action"
	v36Evidence           v36FailureKind = "evidence_corruption"
	v36CampaignCap        v36FailureKind = "campaign_cap"
	v36ObservationInvalid v36FailureKind = "provider_transport_invalid"
)

type v36Failure struct {
	Kind  v36FailureKind
	Cause error
}

func (e *v36Failure) Error() string                { return string(e.Kind) + ": " + e.Cause.Error() }
func (e *v36Failure) Unwrap() error                { return e.Cause }
func failV36(kind v36FailureKind, err error) error { return &v36Failure{Kind: kind, Cause: err} }
func kindV36(err error) (v36FailureKind, bool) {
	var typed *v36Failure
	ok := errors.As(err, &typed)
	if !ok {
		return "", false
	}
	return typed.Kind, true
}

func executeV36(ctx context.Context, root string, provider agentqualification.RealisticProvider, gate func() error) (v36Ledger, error) {
	if gate == nil {
		return v36Ledger{}, fmt.Errorf("v36 freeze gate missing")
	}
	if err := gate(); err != nil {
		return v36Ledger{}, err
	}
	d, err := agentqualification.ClaudeSubscriptionRealisticV36()
	if err != nil {
		return v36Ledger{}, err
	}
	if !filepath.IsAbs(root) || provider == nil {
		return v36Ledger{}, fmt.Errorf("absolute evidence root and provider required")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return v36Ledger{}, err
	}
	ledgerPath := filepath.Join(root, "ledger.json")
	ledger, err := loadLedgerV36(ledgerPath, d)
	if err != nil {
		return ledger, err
	}
	if ledger.GlobalHalt || ledger.Status == "complete" {
		return ledger, nil
	}
	if pendingID := pendingClaimV36(ledger); pendingID != "" {
		if err := reconcilePendingV36(root, &ledger, pendingID, d); err != nil {
			return haltV36(ledgerPath, ledger, string(v36Evidence)+": "+err.Error())
		}
		if err := writeJSONAtomicV36(ledgerPath, ledger); err != nil {
			return ledger, err
		}
		return haltV36(ledgerPath, ledger, string(v36Evidence)+": pending claim conservatively closed; no later case admitted")
	}
	for index, ref := range d.ExecutionOrder {
		parts := strings.Split(ref, ":")
		caseID, version := parts[0], parts[1]
		if _, ok := ledger.Claims[caseID]; ok {
			continue
		}
		if caseID == "R04" && ledger.CandidateEliminated {
			claim := v36Claim{CaseID: caseID, CaseVersion: version, Status: "skipped", Outcome: "candidate_eliminated", Retries: 0}
			ledger.Claims[caseID] = claim
			if err := sealSimpleV36(root, &claim, nil); err != nil {
				return haltV36(ledgerPath, ledger, string(v36Evidence)+": "+err.Error())
			}
			ledger.Claims[caseID] = claim
			if err := writeJSONAtomicV36(ledgerPath, ledger); err != nil {
				return ledger, err
			}
			continue
		}
		if _, err := replayV36(root); err != nil {
			return haltV36(ledgerPath, ledger, string(v36Evidence)+": "+err.Error())
		}
		caps := d.CaseCaps[ref]
		admitted, arithmeticOK := admitV36(ledger.Usage, caps, d.CampaignCaps)
		if !arithmeticOK {
			return haltV36(ledgerPath, ledger, string(v36Evidence)+": cap admission arithmetic overflow")
		}
		if !admitted {
			return haltV36(ledgerPath, ledger, string(v36CampaignCap))
		}
		runFamily := hashV36(fmt.Sprintf("%s|%02d|%s", d.BatchID, index, ref))
		cell := filepath.Join(root, "observations", caseID)
		if err := os.MkdirAll(cell, 0o700); err != nil {
			return ledger, err
		}
		workspaceParent := filepath.Join(cell, "workspace")
		if err := os.MkdirAll(workspaceParent, 0o700); err != nil {
			return ledger, err
		}
		workspace, err := agentqualification.PrepareRealisticCaseWorkspaceForRun(workspaceParent, caseID, runFamily)
		if err != nil {
			return haltV36(ledgerPath, ledger, string(v36Evidence)+": "+err.Error())
		}
		claim := v36Claim{CaseID: caseID, CaseVersion: version, RunFamilyID: runFamily, Status: "pending", Retries: 0}
		ledger.Claims[caseID] = claim
		runtime := v36Runtime{Workspace: workspace}
		if err := writeJSONExclusiveV36(filepath.Join(cell, "runtime.json"), runtime); err != nil {
			return haltV36(ledgerPath, ledger, string(v36Evidence)+": "+err.Error())
		}
		claim.RuntimeSHA256, _ = fileHashV36(filepath.Join(cell, "runtime.json"))
		ledger.Claims[caseID] = claim
		if err := writeJSONAtomicV36(ledgerPath, ledger); err != nil {
			return ledger, err
		}
		capped := &v36CappedProvider{inner: provider, caseCaps: caps, campaignCaps: d.CampaignCaps, campaignPrior: ledger.Usage}
		checkpoint := func(result agentqualification.RealisticControllerResult) error {
			runtime.Controller = result
			return checkpointV36(cell, &runtime)
		}
		var controller agentqualification.RealisticControllerResult
		controller, runErr := agentqualification.RunRealisticCandidate(ctx, workspace, runFamily, 1, capped, nil, checkpoint)
		if runErr == nil && caseID == "R04" && controller.TerminalState == "interrupted" {
			controller, runErr = agentqualification.ResumeRealisticCandidate(ctx, workspace, controller, capped, nil, checkpoint)
		}
		claim.Attempts = controller.Attempt
		claim.Usage, err = usageFromControllerV36(controller)
		if err != nil {
			return haltV36(ledgerPath, ledger, string(v36Evidence)+": "+err.Error())
		}
		if claim.Usage.UnknownTurns > 0 {
			if err := reserveRemainingV36(&claim.Usage, caps); err != nil {
				return haltV36(ledgerPath, ledger, string(v36Evidence)+": "+err.Error())
			}
		}
		if err := addUsageV36(&ledger.Usage, claim.Usage); err != nil {
			return haltV36(ledgerPath, ledger, string(v36Evidence)+": usage overflow")
		}
		if err := writeJSONExclusiveV36(filepath.Join(cell, "controller-result.json"), controller); err != nil {
			return haltV36(ledgerPath, ledger, string(v36Evidence)+": "+err.Error())
		}
		claim.ControllerSHA256, _ = fileHashV36(filepath.Join(cell, "controller-result.json"))
		if capped.campaignExceeded {
			claim.Status = "completed"
			claim.Outcome = "invalid"
			claim.FailureClass = string(v36CampaignCap)
			ledger.Claims[caseID] = claim
			_ = sealSimpleV36(root, &claim, nil)
			ledger.Claims[caseID] = claim
			return haltV36(ledgerPath, ledger, string(v36CampaignCap))
		}
		if runErr != nil {
			kind, typed := kindV36(runErr)
			if !typed {
				if errors.Is(runErr, agentqualification.ErrRealisticCredentialFailure) {
					kind = v36Credential
				} else if errors.Is(runErr, agentqualification.ErrRealisticBudgetBreach) {
					kind = v36CampaignCap
				} else {
					kind = v36ObservationInvalid
				}
			}
			claim.Status = "completed"
			claim.Outcome = "invalid"
			claim.FailureClass = string(kind)
			if kind == v36Credential || kind == v36Unsafe || kind == v36Evidence || kind == v36CampaignCap {
				ledger.Claims[caseID] = claim
				_ = sealSimpleV36(root, &claim, nil)
				ledger.Claims[caseID] = claim
				return haltV36(ledgerPath, ledger, string(kind))
			}
			ledger.Claims[caseID] = claim
			if err := sealSimpleV36(root, &claim, nil); err != nil {
				return haltV36(ledgerPath, ledger, string(v36Evidence))
			}
			ledger.Claims[caseID] = claim
			if err := writeJSONAtomicV36(ledgerPath, ledger); err != nil {
				return ledger, err
			}
			continue
		}
		if controller.TerminalState != "completed" {
			claim.Status = "completed"
			claim.Outcome = "failed"
			claim.FailureClass = "response_or_tool_protocol_failure"
			if capped.resourceExceeded {
				claim.FailureClass = "observation_resource_exhaustion"
			}
			ledger.CandidateEliminated = true
			ledger.Claims[caseID] = claim
			if err := sealSimpleV36(root, &claim, nil); err != nil {
				return haltV36(ledgerPath, ledger, string(v36Evidence))
			}
			ledger.Claims[caseID] = claim
			if err := writeJSONAtomicV36(ledgerPath, ledger); err != nil {
				return ledger, err
			}
			continue
		}
		evidenceRoot := filepath.Join(cell, "evidence")
		receipt, receiptPath, sealErr := agentqualification.SealClaudeSubscriptionRealisticV36Execution(evidenceRoot, workspace, controller)
		if sealErr != nil {
			return haltV36(ledgerPath, ledger, string(v36Evidence)+": "+sealErr.Error())
		}
		claim.ReceiptSHA256, _ = fileHashV36(receiptPath)
		grade, gradeErr := agentqualification.GradeClaudeSubscriptionRealisticV36(workspace, evidenceRoot, receipt, controller.FinalSummary)
		if gradeErr != nil {
			return haltV36(ledgerPath, ledger, string(v36Evidence)+": "+gradeErr.Error())
		}
		if err := writeJSONExclusiveV36(filepath.Join(cell, "grade.json"), grade); err != nil {
			return haltV36(ledgerPath, ledger, string(v36Evidence)+": "+err.Error())
		}
		claim.GradeSHA256, _ = fileHashV36(filepath.Join(cell, "grade.json"))
		claim.Status = "completed"
		claim.Outcome = strings.ToLower(grade.Verdict)
		if grade.Verdict != "PASS" {
			claim.FailureClass = "competence_failure"
			ledger.CandidateEliminated = true
		}
		ledger.Claims[caseID] = claim
		if err := sealSimpleV36(root, &claim, &grade); err != nil {
			return haltV36(ledgerPath, ledger, string(v36Evidence)+": "+err.Error())
		}
		ledger.Claims[caseID] = claim
		if err := writeJSONAtomicV36(ledgerPath, ledger); err != nil {
			return ledger, err
		}
	}
	ledger.Status = "complete"
	if err := writeJSONAtomicV36(ledgerPath, ledger); err != nil {
		return ledger, err
	}
	return ledger, nil
}

type v36CappedProvider struct {
	inner                              agentqualification.RealisticProvider
	caseCaps, campaignCaps             agentqualification.ClaudeRealisticV36Caps
	campaignPrior                      v36Usage
	current                            v36Usage
	resourceExceeded, campaignExceeded bool
}

func (p *v36CappedProvider) Next(ctx context.Context, req agentqualification.RealisticProviderRequest) (agentqualification.RealisticProviderResponse, error) {
	campaignTurns, ok := checkedAddIntV36(p.campaignPrior.ProviderTurns, p.current.ProviderTurns)
	if !ok || p.current.ProviderTurns >= p.caseCaps.MaxProviderTurns || campaignTurns >= p.campaignCaps.MaxProviderTurns {
		p.campaignExceeded = !ok
		return agentqualification.RealisticProviderResponse{}, agentqualification.ErrRealisticBudgetBreach
	}
	r, err := p.inner.Next(ctx, req)
	if err != nil {
		return r, err
	}
	if r.InputTokens < 0 || r.OutputTokens < 0 || r.ReasoningTokens < 0 || r.CostMicros < 0 {
		return r, failV36(v36Evidence, fmt.Errorf("negative provider usage"))
	}
	p.current.ProviderTurns, ok = checkedAddIntV36(p.current.ProviderTurns, 1)
	if !ok || addUsageFieldsV36(&p.current, v36Usage{InputTokens: r.InputTokens, OutputTokens: r.OutputTokens, ReasoningTokens: r.ReasoningTokens, CostMicros: r.CostMicros}) != nil {
		return r, failV36(v36Evidence, fmt.Errorf("provider usage overflow"))
	}
	overCase, arithmeticOK := overV36(p.current, p.caseCaps)
	if !arithmeticOK {
		return r, failV36(v36Evidence, fmt.Errorf("case cap arithmetic overflow"))
	}
	if overCase {
		p.resourceExceeded = true
		r.ProtocolFailure = "v36 observation resource ceiling exceeded"
	}
	total := p.campaignPrior
	if err := addUsageV36(&total, p.current); err != nil {
		p.campaignExceeded = true
		return r, failV36(v36Evidence, fmt.Errorf("campaign usage overflow"))
	}
	overCampaign, arithmeticOK := overV36(total, p.campaignCaps)
	if !arithmeticOK {
		p.campaignExceeded = true
		return r, failV36(v36Evidence, fmt.Errorf("campaign cap arithmetic overflow"))
	}
	if overCampaign {
		p.campaignExceeded = true
		r.ProtocolFailure = "v36 campaign ceiling exceeded"
	}
	return r, nil
}
func overV36(u v36Usage, c agentqualification.ClaudeRealisticV36Caps) (bool, bool) {
	total, ok := checkedSumInt64V36(u.InputTokens, u.OutputTokens, u.ReasoningTokens, u.ConservativeReservedTokens)
	if !ok {
		return false, false
	}
	generated, ok := checkedSumInt64V36(u.OutputTokens, u.ReasoningTokens, u.ConservativeReservedGenerated)
	if !ok {
		return false, false
	}
	cost, ok := checkedSumInt64V36(u.CostMicros, u.ConservativeReservedCostMicros)
	if !ok {
		return false, false
	}
	return u.ProviderTurns > c.MaxProviderTurns || total > c.MaxTotalReportedTokens || generated > c.MaxGeneratedTokens || cost > c.MaxCostEquivalentMicros, true
}
func admitV36(u v36Usage, c, all agentqualification.ClaudeRealisticV36Caps) (bool, bool) {
	if c.MaxProviderTurns < 0 || c.MaxTotalReportedTokens < 0 || c.MaxGeneratedTokens < 0 || c.MaxCostEquivalentMicros < 0 || all.MaxProviderTurns < c.MaxProviderTurns || all.MaxTotalReportedTokens < c.MaxTotalReportedTokens || all.MaxGeneratedTokens < c.MaxGeneratedTokens || all.MaxCostEquivalentMicros < c.MaxCostEquivalentMicros {
		return false, false
	}
	turns, ok := checkedAddIntV36(u.ProviderTurns, c.MaxProviderTurns)
	if !ok {
		return false, false
	}
	total, ok := checkedSumInt64V36(u.InputTokens, u.OutputTokens, u.ReasoningTokens, u.ConservativeReservedTokens, c.MaxTotalReportedTokens)
	if !ok {
		return false, false
	}
	generated, ok := checkedSumInt64V36(u.OutputTokens, u.ReasoningTokens, u.ConservativeReservedGenerated, c.MaxGeneratedTokens)
	if !ok {
		return false, false
	}
	cost, ok := checkedSumInt64V36(u.CostMicros, u.ConservativeReservedCostMicros, c.MaxCostEquivalentMicros)
	if !ok {
		return false, false
	}
	return turns <= all.MaxProviderTurns && total <= all.MaxTotalReportedTokens && generated <= all.MaxGeneratedTokens && cost <= all.MaxCostEquivalentMicros, true
}
func usageFromControllerV36(r agentqualification.RealisticControllerResult) (v36Usage, error) {
	u := v36Usage{ProviderTurns: r.ProviderCalls, InputTokens: r.InputTokens, OutputTokens: r.OutputTokens, ReasoningTokens: r.ReasoningTokens, CostMicros: r.CostMicros, UnknownTurns: r.AmbiguousProviderCalls}
	if err := validateUsageV36(u); err != nil || u.UnknownTurns > u.ProviderTurns {
		return v36Usage{}, fmt.Errorf("invalid controller usage")
	}
	return u, nil
}
func addUsageV36(dst *v36Usage, u v36Usage) error {
	if err := validateUsageV36(*dst); err != nil {
		return err
	}
	if err := validateUsageV36(u); err != nil {
		return err
	}
	turns, ok := checkedAddIntV36(dst.ProviderTurns, u.ProviderTurns)
	if !ok {
		return fmt.Errorf("provider-turn overflow")
	}
	unknown, ok := checkedAddIntV36(dst.UnknownTurns, u.UnknownTurns)
	if !ok {
		return fmt.Errorf("unknown-turn overflow")
	}
	copy := *dst
	copy.ProviderTurns, copy.UnknownTurns = turns, unknown
	if err := addUsageFieldsV36(&copy, u); err != nil {
		return err
	}
	*dst = copy
	return nil
}

func addUsageFieldsV36(dst *v36Usage, u v36Usage) error {
	fields := [][2]*int64{{&dst.InputTokens, &u.InputTokens}, {&dst.OutputTokens, &u.OutputTokens}, {&dst.ReasoningTokens, &u.ReasoningTokens}, {&dst.CostMicros, &u.CostMicros}, {&dst.ConservativeReservedTokens, &u.ConservativeReservedTokens}, {&dst.ConservativeReservedGenerated, &u.ConservativeReservedGenerated}, {&dst.ConservativeReservedCostMicros, &u.ConservativeReservedCostMicros}}
	for _, field := range fields {
		next, ok := checkedAddInt64V36(*field[0], *field[1])
		if !ok {
			return fmt.Errorf("usage overflow")
		}
		*field[0] = next
	}
	return nil
}

func validateUsageV36(u v36Usage) error {
	if u.ProviderTurns < 0 || u.InputTokens < 0 || u.OutputTokens < 0 || u.ReasoningTokens < 0 || u.CostMicros < 0 || u.UnknownTurns < 0 || u.ConservativeReservedTokens < 0 || u.ConservativeReservedGenerated < 0 || u.ConservativeReservedCostMicros < 0 {
		return fmt.Errorf("negative usage")
	}
	return nil
}

func checkedAddInt64V36(a, b int64) (int64, bool) {
	if a < 0 || b < 0 || a > int64(^uint64(0)>>1)-b {
		return 0, false
	}
	return a + b, true
}

func checkedSumInt64V36(values ...int64) (int64, bool) {
	var total int64
	for _, value := range values {
		var ok bool
		total, ok = checkedAddInt64V36(total, value)
		if !ok {
			return 0, false
		}
	}
	return total, true
}

func checkedAddIntV36(a, b int) (int, bool) {
	if a < 0 || b < 0 || a > int(^uint(0)>>1)-b {
		return 0, false
	}
	return a + b, true
}

func reserveRemainingV36(u *v36Usage, caps agentqualification.ClaudeRealisticV36Caps) error {
	total, ok := checkedSumInt64V36(u.InputTokens, u.OutputTokens, u.ReasoningTokens)
	if !ok || total > caps.MaxTotalReportedTokens {
		return fmt.Errorf("reported-token reservation overflow")
	}
	generated, ok := checkedSumInt64V36(u.OutputTokens, u.ReasoningTokens)
	if !ok || generated > caps.MaxGeneratedTokens || u.CostMicros > caps.MaxCostEquivalentMicros || u.ProviderTurns > caps.MaxProviderTurns {
		return fmt.Errorf("case usage already exceeds reservation")
	}
	u.ConservativeReservedTokens = caps.MaxTotalReportedTokens - total
	u.ConservativeReservedGenerated = caps.MaxGeneratedTokens - generated
	u.ConservativeReservedCostMicros = caps.MaxCostEquivalentMicros - u.CostMicros
	u.UnknownTurns = caps.MaxProviderTurns - (u.ProviderTurns - u.UnknownTurns)
	u.ProviderTurns = caps.MaxProviderTurns
	return validateUsageV36(*u)
}

func loadLedgerV36(path string, d agentqualification.ClaudeSubscriptionRealisticV36Definition) (v36Ledger, error) {
	l := v36Ledger{SchemaVersion: v36LedgerSchema, BatchID: d.BatchID, DefinitionSHA256: agentqualification.ClaudeSubscriptionRealisticV36SHA256(), Status: "active", Claims: map[string]v36Claim{}}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if err := writeJSONExclusiveV36(path, l); err != nil {
			return l, err
		}
		return l, nil
	}
	if err != nil {
		return l, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&l) != nil || l.SchemaVersion != v36LedgerSchema || l.BatchID != d.BatchID || l.DefinitionSHA256 != agentqualification.ClaudeSubscriptionRealisticV36SHA256() || l.Claims == nil {
		return l, fmt.Errorf("v36 ledger evidence corruption")
	}
	if err := validateLedgerV36(l, d, true); err != nil {
		return l, err
	}
	return l, nil
}

func validateLedgerV36(l v36Ledger, d agentqualification.ClaudeSubscriptionRealisticV36Definition, allowPending bool) error {
	if l.SchemaVersion != v36LedgerSchema || l.BatchID != d.BatchID || l.DefinitionSHA256 != agentqualification.ClaudeSubscriptionRealisticV36SHA256() || l.Claims == nil || !containsV36([]string{"active", "complete", "halted"}, l.Status) {
		return fmt.Errorf("ledger binding drift")
	}
	known := map[string]string{"R03": "v2", "R04": "v1"}
	for id, claim := range l.Claims {
		if known[id] == "" || claim.CaseID != id || claim.CaseVersion != known[id] || claim.Retries != 0 || !containsV36([]string{"pending", "completed", "skipped"}, claim.Status) {
			return fmt.Errorf("ledger claim identity drift: %s", id)
		}
		if claim.Status == "pending" && !allowPending {
			return fmt.Errorf("pending claim is not replayable: %s", id)
		}
	}
	recomputed, err := recomputeUsageV36(l.Claims)
	if err != nil || recomputed != l.Usage {
		return fmt.Errorf("ledger accounting drift")
	}
	over, ok := overV36(l.Usage, d.CampaignCaps)
	if !ok || over {
		return fmt.Errorf("ledger cap or accounting drift")
	}
	return nil
}

func recomputeUsageV36(claims map[string]v36Claim) (v36Usage, error) {
	var total v36Usage
	for _, claim := range claims {
		if err := addUsageV36(&total, claim.Usage); err != nil {
			return v36Usage{}, err
		}
	}
	return total, nil
}

func pendingClaimV36(l v36Ledger) string {
	for _, id := range []string{"R03", "R04"} {
		if claim, ok := l.Claims[id]; ok && claim.Status == "pending" {
			return id
		}
	}
	return ""
}

// reconcilePendingV36 never calls the provider. A crash can occur after the
// durable in-flight checkpoint but before the response is sealed, so replay
// cannot prove that a subscription turn was or was not billed. The only safe
// recovery is to preserve all known receipts, reserve the complete case cap,
// close the observation invalid, and globally stop before another case.
func reconcilePendingV36(root string, ledger *v36Ledger, caseID string, d agentqualification.ClaudeSubscriptionRealisticV36Definition) error {
	claim := ledger.Claims[caseID]
	if claim.Status != "pending" {
		return fmt.Errorf("claim is not pending")
	}
	cell := filepath.Join(root, "observations", caseID)
	if _, err := os.Stat(filepath.Join(cell, "result.json")); err == nil {
		return fmt.Errorf("pending ledger claim already has a result")
	} else if !os.IsNotExist(err) {
		return err
	}
	var runtimeState v36Runtime
	if err := readJSONV36(filepath.Join(cell, "runtime.json"), &runtimeState); err != nil {
		return fmt.Errorf("pending runtime unreadable: %w", err)
	}
	if claim.RuntimeSHA256 == "" {
		return fmt.Errorf("pending runtime hash missing")
	}
	if err := verifyHashV36(filepath.Join(cell, "runtime.json"), claim.RuntimeSHA256); err != nil {
		return err
	}
	checkpointPath := filepath.Join(cell, "checkpoint.json")
	if _, err := os.Stat(checkpointPath); err == nil {
		if err := readJSONV36(checkpointPath, &runtimeState); err != nil {
			return fmt.Errorf("pending checkpoint unreadable: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	controller := runtimeState.Controller
	if controller.CaseID != "" {
		if controller.CaseID != caseID || controller.RunFamilyID != claim.RunFamilyID {
			return fmt.Errorf("pending controller identity drift")
		}
		if err := validateProviderTurnsV36(cell, claim, controller); err != nil {
			return err
		}
		controllerPath := filepath.Join(cell, "controller-result.json")
		if err := writeJSONExclusiveV36(controllerPath, controller); err != nil {
			return err
		}
		claim.ControllerSHA256, _ = fileHashV36(controllerPath)
		claim.Attempts = controller.Attempt
	}
	ref := caseID + ":" + claim.CaseVersion
	caps, ok := d.CaseCaps[ref]
	if !ok {
		return fmt.Errorf("pending case cap missing")
	}
	known, err := knownReceiptUsageV36(controller.ProviderReceipts)
	if controller.CaseID == "" {
		known = v36Usage{}
		err = nil
	}
	if err != nil {
		return err
	}
	if err := reserveRemainingV36(&known, caps); err != nil {
		return err
	}
	claim.Usage = known
	claim.Status = "completed"
	claim.Outcome = "invalid"
	claim.FailureClass = "pending_claim_conservatively_closed"
	ledger.Claims[caseID] = claim
	ledger.Usage, err = recomputeUsageV36(ledger.Claims)
	if err != nil {
		return err
	}
	if err := sealSimpleV36(root, &claim, nil); err != nil {
		return err
	}
	ledger.Claims[caseID] = claim
	return nil
}

func knownReceiptUsageV36(receipts [][]byte) (v36Usage, error) {
	var usage v36Usage
	for _, raw := range receipts {
		receipt, err := agentqualification.ParseClaudeRealisticV36ProviderReceipt(raw)
		if err != nil {
			return v36Usage{}, err
		}
		turns, ok := checkedAddIntV36(usage.ProviderTurns, 1)
		if !ok {
			return v36Usage{}, fmt.Errorf("provider-turn overflow")
		}
		usage.ProviderTurns = turns
		if err := addUsageFieldsV36(&usage, v36Usage{InputTokens: receipt.InputTokens, OutputTokens: receipt.OutputTokens, CostMicros: receipt.CostMicros}); err != nil {
			return v36Usage{}, err
		}
	}
	return usage, nil
}
func checkpointV36(cell string, runtime *v36Runtime) error {
	for i, raw := range runtime.Controller.ProviderReceipts {
		path := filepath.Join(cell, fmt.Sprintf("provider-turn-%02d.json", i+1))
		if err := writeOrVerifyV36(path, raw); err != nil {
			return err
		}
	}
	return writeJSONAtomicV36(filepath.Join(cell, "checkpoint.json"), runtime)
}
func sealSimpleV36(root string, claim *v36Claim, grade *agentqualification.RealisticWorkspaceGrade) error {
	path := filepath.Join(root, "observations", claim.CaseID, "result.json")
	if err := writeJSONExclusiveV36(path, v36Result{Claim: *claim, Grade: grade}); err != nil {
		return err
	}
	claim.ResultSHA256, _ = fileHashV36(path)
	return nil
}
func replayV36(root string) (map[string]any, error) {
	var l v36Ledger
	if err := readJSONV36(filepath.Join(root, "ledger.json"), &l); err != nil {
		return nil, err
	}
	d, err := agentqualification.ClaudeSubscriptionRealisticV36()
	if err != nil {
		return nil, err
	}
	if err := validateLedgerV36(l, d, false); err != nil {
		return nil, err
	}
	for _, id := range []string{"R03", "R04"} {
		c, exists := l.Claims[id]
		if !exists {
			continue
		}
		cell := filepath.Join(root, "observations", id)
		if c.ResultSHA256 == "" {
			return nil, fmt.Errorf("result hash missing: %s", id)
		}
		resultPath := filepath.Join(cell, "result.json")
		if err := verifyHashV36(resultPath, c.ResultSHA256); err != nil {
			return nil, err
		}
		var result v36Result
		if err := readJSONV36(resultPath, &result); err != nil {
			return nil, err
		}
		resultClaim, ledgerClaim := result.Claim, c
		resultClaim.ResultSHA256, ledgerClaim.ResultSHA256 = "", ""
		if !reflect.DeepEqual(resultClaim, ledgerClaim) {
			return nil, fmt.Errorf("result claim does not match ledger: %s", id)
		}
		if c.Status == "skipped" {
			if c.Usage != (v36Usage{}) || result.Grade != nil || c.ControllerSHA256 != "" || c.ReceiptSHA256 != "" || c.GradeSHA256 != "" {
				return nil, fmt.Errorf("skipped claim contains execution evidence: %s", id)
			}
			continue
		}
		if c.RuntimeSHA256 == "" || verifyHashV36(filepath.Join(cell, "runtime.json"), c.RuntimeSHA256) != nil {
			return nil, fmt.Errorf("runtime binding invalid: %s", id)
		}
		var runtimeState v36Runtime
		if err := readJSONV36(filepath.Join(cell, "runtime.json"), &runtimeState); err != nil || runtimeState.Workspace.CaseID != id || runtimeState.Workspace.CaseVersion != c.CaseVersion {
			return nil, fmt.Errorf("runtime identity invalid: %s", id)
		}
		var controller agentqualification.RealisticControllerResult
		if c.ControllerSHA256 != "" {
			controllerPath := filepath.Join(cell, "controller-result.json")
			if err := verifyHashV36(controllerPath, c.ControllerSHA256); err != nil || readJSONV36(controllerPath, &controller) != nil {
				return nil, fmt.Errorf("controller binding invalid: %s", id)
			}
			if err := validateProviderTurnsV36(cell, c, controller); err != nil {
				return nil, err
			}
		}
		if c.ReceiptSHA256 == "" {
			if c.GradeSHA256 != "" || result.Grade != nil || c.Outcome == "pass" {
				return nil, fmt.Errorf("unsealed result claims a grade: %s", id)
			}
			continue
		}
		if c.ControllerSHA256 == "" || c.GradeSHA256 == "" || result.Grade == nil {
			return nil, fmt.Errorf("completed evidence set incomplete: %s", id)
		}
		evidenceRoot := filepath.Join(cell, "evidence")
		receipt, receiptPath, err := agentqualification.SealClaudeSubscriptionRealisticV36Execution(evidenceRoot, runtimeState.Workspace, controller)
		if err != nil || verifyHashV36(receiptPath, c.ReceiptSHA256) != nil {
			return nil, fmt.Errorf("execution receipt replay failed: %s: %v", id, err)
		}
		grade, err := agentqualification.GradeClaudeSubscriptionRealisticV36(runtimeState.Workspace, evidenceRoot, receipt, controller.FinalSummary)
		if err != nil {
			return nil, fmt.Errorf("grade recompute failed: %s: %w", id, err)
		}
		gradePath := filepath.Join(cell, "grade.json")
		var storedGrade agentqualification.RealisticWorkspaceGrade
		if verifyHashV36(gradePath, c.GradeSHA256) != nil || readJSONV36(gradePath, &storedGrade) != nil || !sameJSONV36(storedGrade, grade) || !sameJSONV36(*result.Grade, grade) || strings.ToLower(grade.Verdict) != c.Outcome {
			return nil, fmt.Errorf("grade or result replay mismatch: %s", id)
		}
	}
	return map[string]any{"status": "replayed", "provider_called": false, "claims": len(l.Claims), "usage": l.Usage}, nil
}

func validateProviderTurnsV36(cell string, claim v36Claim, controller agentqualification.RealisticControllerResult) error {
	if controller.CaseID != claim.CaseID || controller.RunFamilyID != claim.RunFamilyID || controller.ProviderCalls < 0 || controller.AmbiguousProviderCalls < 0 || controller.AmbiguousProviderCalls > controller.ProviderCalls || len(controller.ProviderReceipts) != len(controller.ProviderReceiptHashes) || len(controller.ProviderReceipts) > controller.ProviderCalls {
		return fmt.Errorf("controller provider identity/accounting invalid: %s", claim.CaseID)
	}
	paths, err := filepath.Glob(filepath.Join(cell, "provider-turn-*.json"))
	if err != nil || len(paths) != len(controller.ProviderReceipts) {
		return fmt.Errorf("provider-turn artifact count mismatch: %s", claim.CaseID)
	}
	var known v36Usage
	for i, raw := range controller.ProviderReceipts {
		path := filepath.Join(cell, fmt.Sprintf("provider-turn-%02d.json", i+1))
		stored, err := os.ReadFile(path)
		if err != nil || !reflect.DeepEqual(stored, raw) || hashV36(string(raw)) != controller.ProviderReceiptHashes[i] {
			return fmt.Errorf("provider-turn hash/order mismatch: %s turn %d", claim.CaseID, i+1)
		}
		receipt, err := agentqualification.ParseClaudeRealisticV36ProviderReceipt(raw)
		if err != nil {
			return err
		}
		wantAttempt := 1
		if claim.CaseID == "R04" {
			wantAttempt = 2
		}
		wantID := fmt.Sprintf("%s:attempt-%d:turn-%d", claim.RunFamilyID, wantAttempt, i+1)
		if receipt.ProviderCallID != wantID {
			return fmt.Errorf("provider request ID/order mismatch: %s turn %d", claim.CaseID, i+1)
		}
		known.ProviderTurns++
		if err := addUsageFieldsV36(&known, v36Usage{InputTokens: receipt.InputTokens, OutputTokens: receipt.OutputTokens, CostMicros: receipt.CostMicros}); err != nil {
			return err
		}
	}
	if controller.ProviderAccountingStatus == "exact" {
		if len(controller.ProviderReceipts) != controller.ProviderCalls || controller.AmbiguousProviderCalls != 0 || known.InputTokens != controller.InputTokens || known.OutputTokens != controller.OutputTokens || controller.ReasoningTokens != 0 || known.CostMicros != controller.CostMicros {
			return fmt.Errorf("controller exact accounting mismatch: %s", claim.CaseID)
		}
	}
	if claim.Status != "pending" && claim.Usage.ConservativeReservedTokens == 0 && claim.Usage.ConservativeReservedGenerated == 0 && claim.Usage.ConservativeReservedCostMicros == 0 {
		controllerUsage, err := usageFromControllerV36(controller)
		if err != nil || controllerUsage != claim.Usage {
			return fmt.Errorf("claim/controller usage mismatch: %s", claim.CaseID)
		}
	}
	return nil
}

func containsV36(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func sameJSONV36(a, b any) bool {
	left, leftErr := json.Marshal(a)
	right, rightErr := json.Marshal(b)
	return leftErr == nil && rightErr == nil && reflect.DeepEqual(left, right)
}
func haltV36(path string, l v36Ledger, reason string) (v36Ledger, error) {
	l.GlobalHalt = true
	l.Status = "halted"
	l.GlobalHaltReason = reason
	if err := writeJSONAtomicV36(path, l); err != nil {
		return l, err
	}
	return l, errors.New(reason)
}
func hashV36(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func fileHashV36(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return hashV36(string(raw)), nil
}
func verifyHashV36(path, want string) error {
	got, err := fileHashV36(path)
	if err != nil || got != want {
		return fmt.Errorf("sealed artifact drift: %s", filepath.Base(path))
	}
	return nil
}
func readJSONV36(path string, v any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}
func writeJSONExclusiveV36(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeExclusiveV36(path, append(raw, '\n'))
}
func writeExclusiveV36(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
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
func writeOrVerifyV36(path string, raw []byte) error {
	existing, err := os.ReadFile(path)
	if err == nil {
		if hashV36(string(existing)) != hashV36(string(raw)) {
			return fmt.Errorf("write-once drift")
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	return writeExclusiveV36(path, raw)
}
func writeJSONAtomicV36(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
