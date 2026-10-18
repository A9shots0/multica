package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

const r02RepeatBrowserInvalidationNote = "The current ChatGPT in-app Browser control surface could not create the frozen 390/900/1440 responsive viewports or capture the required DOM evidence, and its security policy explicitly rejected raw CDP or workaround access. The observation is unusable; no retry or additional provider call occurred."

type r02RepeatPostBatchCorrection struct {
	SchemaVersion          string                         `json:"schema_version"`
	AdmissionID            string                         `json:"admission_id"`
	AdmissionSHA256        string                         `json:"admission_sha256"`
	RecordedAt             string                         `json:"recorded_at"`
	Outcome                string                         `json:"outcome"`
	FailureClass           string                         `json:"failure_class"`
	Reason                 string                         `json:"reason"`
	OriginalRunStateSHA256 string                         `json:"original_run_state_sha256"`
	WorkspaceTreeSHA256    string                         `json:"workspace_tree_sha256"`
	ProviderCalls          int                            `json:"provider_calls"`
	InputTokens            int64                          `json:"input_tokens"`
	OutputTokens           int64                          `json:"output_tokens"`
	ReasoningTokens        int64                          `json:"reasoning_tokens"`
	CostMicros             int64                          `json:"cost_micros"`
	ProviderReceiptHashes  []string                       `json:"provider_receipt_hashes"`
	FailureEvidenceSHA256  string                         `json:"failure_evidence_sha256"`
	NoProviderCall         bool                           `json:"no_provider_call"`
	NoRetry                bool                           `json:"no_retry"`
	AccidentalExecutions   []r02RepeatAccidentalExecution `json:"accidental_executions"`
	TotalAccidentalCostUSD string                         `json:"total_accidental_cost_usd"`
}

type r02RepeatAccidentalExecution struct {
	RunFamilyID             string   `json:"run_family_id"`
	EvidenceSource          string   `json:"evidence_source"`
	SessionIDs              []string `json:"session_ids"`
	CompletedCalls          int      `json:"completed_calls"`
	PendingZeroTokenCall    int      `json:"pending_zero_token_call"`
	InputTokens             int64    `json:"input_tokens_including_cache"`
	OutputTokens            int64    `json:"output_tokens"`
	ReasoningTokens         int64    `json:"reasoning_tokens"`
	CostUSD                 string   `json:"cost_usd"`
	PreservedRunStatePath   string   `json:"preserved_run_state_path,omitempty"`
	PreservedRunStateSHA256 string   `json:"preserved_run_state_sha256,omitempty"`
}

func invalidateR02RepeatForBrowserCapability(artifactRoot string) (map[string]any, error) {
	admission, admissionHash, err := agentqualification.RequireActiveR02RepeatAdmission(repositoryRootForR02Repeat())
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(artifactRoot) {
		return nil, fmt.Errorf("R02 repeat artifact root must be absolute")
	}
	campaignPath := filepath.Join(artifactRoot, "r02-repeat-campaign.json")
	var campaign r02RepeatCampaignState
	if err := readJSONFile(campaignPath, &campaign); err != nil {
		return nil, err
	}
	statePath := filepath.Join(artifactRoot, "observations", safeArtifactName(admission.ConfigurationID), "r02", "run-state.json")
	runStateRaw, err := os.ReadFile(statePath)
	if err != nil {
		return nil, err
	}
	var state realisticRunState
	if err := readJSONFile(statePath, &state); err != nil {
		return nil, err
	}
	if err := validateR02RepeatBrowserInvalidation(admission, admissionHash, campaign, state); err != nil {
		return nil, err
	}
	workspaceTree, err := agentqualification.RealisticFixtureTreeSHA256(os.DirFS(state.Workspace.Root), ".")
	if err != nil {
		return nil, err
	}
	record, artifacts, err := finalizeR02RepeatObservation(state, admission, admissionHash, fmt.Errorf("%s", r02RepeatBrowserInvalidationNote))
	if err != nil {
		return nil, err
	}
	if record.Outcome != "invalid" || record.FailureClass != "non_safety_harness_imperfection" {
		return nil, fmt.Errorf("R02 repeat Browser invalidation classified unexpectedly")
	}
	if err := agentqualification.ValidateR02RepeatRunRecord(admission, record, state.Controller); err != nil {
		return nil, err
	}
	runStateHash := sha256.Sum256(runStateRaw)
	correction := r02RepeatPostBatchCorrection{
		SchemaVersion: "silver-mediums.realistic-r02-repeat-post-batch-correction.v1", AdmissionID: admission.AdmissionID, AdmissionSHA256: admissionHash,
		RecordedAt: time.Now().UTC().Format(time.RFC3339Nano), Outcome: record.Outcome, FailureClass: record.FailureClass, Reason: r02RepeatBrowserInvalidationNote,
		OriginalRunStateSHA256: hex.EncodeToString(runStateHash[:]), WorkspaceTreeSHA256: workspaceTree,
		ProviderCalls: state.Controller.ProviderCalls, InputTokens: state.Controller.InputTokens, OutputTokens: state.Controller.OutputTokens, ReasoningTokens: state.Controller.ReasoningTokens, CostMicros: state.Controller.CostMicros,
		ProviderReceiptHashes: append([]string(nil), state.Controller.ProviderReceiptHashes...), FailureEvidenceSHA256: record.Observation.FailureEvidenceSHA256,
		NoProviderCall: true, NoRetry: true,
	}
	correction.AccidentalExecutions, correction.TotalAccidentalCostUSD, err = auditedR02RepeatAccidentalExecutions()
	if err != nil {
		return nil, err
	}
	correctionPath := filepath.Join(state.EvidenceRoot, "post-batch-browser-capability-correction.json")
	if err := writeJSONExclusive(correctionPath, correction); err != nil {
		return nil, err
	}
	campaign.Result = &record
	// Preserve the original provider spend snapshots: this administrative
	// command does not query or call the provider. Exact observation accounting
	// is carried by the sealed failure receipt and correction record.
	if err := writeJSONAtomic(campaignPath, campaign); err != nil {
		return nil, err
	}
	artifacts["status"] = record.Outcome
	artifacts["failure_class"] = record.FailureClass
	artifacts["campaign"] = campaignPath
	artifacts["post_batch_correction"] = correctionPath
	artifacts["provider_called_by_command"] = false
	artifacts["retry"] = false
	return artifacts, nil
}

func auditedR02RepeatAccidentalExecutions() ([]r02RepeatAccidentalExecution, string, error) {
	first := r02RepeatAccidentalExecution{
		RunFamilyID:    "realistic.claudesonnet5.medium.anchor.r02.repeat1.v11-r02-10adde92fbac094a",
		EvidenceSource: "read-only OpenCode opencode.db session ledger audited 2026-09-13T01:26:31Z",
		SessionIDs:     []string{"ses_f67a41a16ffexke8NALFHLtJWm", "ses_f67a400e6ffej97Kth9NTxDnrt", "ses_f67a3e69bffeh5L1KhXUVaLtk4", "ses_f67a3cc60ffejV64fJEFQNQheK", "ses_f67a3b050ffeegEZlgkxULuu5P", "ses_f67a38cd0ffeDCC3ramTiK6D73", "ses_f67a36e48ffeLMpDqe0VKWdiyN"},
		CompletedCalls: 7, InputTokens: 20_716, OutputTokens: 502, ReasoningTokens: 225, CostUSD: "0.0590565",
	}
	secondPath := filepath.Join(os.TempDir(), "TestR02RepeatPaidPathFailsClosedBeforeBaselineOrProvider1312114765", "001", "observations", "realistic.claudesonnet5.medium.anchor.r02.repeat1.v11", "r02", "run-state.json")
	raw, err := os.ReadFile(secondPath)
	if err != nil {
		return nil, "", fmt.Errorf("read preserved second accidental run state: %w", err)
	}
	var state realisticRunState
	if err := readJSONFile(secondPath, &state); err != nil {
		return nil, "", err
	}
	c := state.Controller
	if c.RunFamilyID != "realistic.claudesonnet5.medium.anchor.r02.repeat1.v11-r02-61d0c2e3fd5d1cb9" || c.ProviderCalls != 8 || len(c.ProviderReceipts) != 7 || len(c.ProviderReceiptHashes) != 7 || c.InputTokens != 20_152 || c.OutputTokens != 468 || c.ReasoningTokens != 521 || c.CostMicros != 60_269 || c.PendingProviderCall == nil || c.ProviderAccountingStatus != "in_flight" {
		return nil, "", fmt.Errorf("second accidental execution no longer matches audited pending state")
	}
	hash := sha256.Sum256(raw)
	second := r02RepeatAccidentalExecution{
		RunFamilyID: c.RunFamilyID, EvidenceSource: "preserved interrupted run-state plus OpenCode session ledger",
		SessionIDs:     []string{"ses_f67a39b68ffe7Flh4N8Ak5AQq4", "ses_f67a38079ffeZO5Bo6YVEp6WpX", "ses_f67a3657fffehdiKGiZwmEa6nV", "ses_f67a34980ffeAYDiQv6NuJaiGd", "ses_f67a324b8ffeRmMxiAJpq2Zj8H", "ses_f67a2fce7ffedbljPlpH7CEGx6", "ses_f67a2da24ffeLDULhoOU3Hn3vg", "ses_f67a2b801ffeXfLzd59SCm30rx"},
		CompletedCalls: 7, PendingZeroTokenCall: 1, InputTokens: c.InputTokens, OutputTokens: c.OutputTokens, ReasoningTokens: c.ReasoningTokens, CostUSD: "0.060269",
		PreservedRunStatePath: secondPath, PreservedRunStateSHA256: hex.EncodeToString(hash[:]),
	}
	return []r02RepeatAccidentalExecution{first, second}, "0.1193255", nil
}

func validateR02RepeatBrowserInvalidation(admission agentqualification.R02RepeatAdmission, admissionHash string, campaign r02RepeatCampaignState, state realisticRunState) error {
	if campaign.AdmissionID != admission.AdmissionID || campaign.AdmissionSHA256 != admissionHash || campaign.Result != nil {
		return fmt.Errorf("R02 repeat campaign is not the exact active unfinished admission")
	}
	controller := state.Controller
	if state.Candidate.ConfigurationID != admission.ConfigurationID || state.Candidate.Model != admission.Model || state.Candidate.Effort != admission.Effort || state.Candidate.ServingProvider != admission.ServingProvider || state.Workspace.CaseID != admission.CaseID || state.ClaimID == "" || controller.RunFamilyID != state.ClaimID || controller.CaseID != admission.CaseID {
		return fmt.Errorf("R02 repeat Browser invalidation identity is not exact")
	}
	if controller.TerminalState != "awaiting_browser_evidence" || controller.FailureClass != "" || len(controller.BrowserProofs) != 0 {
		return fmt.Errorf("R02 repeat is not the exact Browser-blocked state without proofs")
	}
	if controller.ProviderCalls < 1 || controller.ProviderCalls != len(controller.ProviderReceipts) || controller.ProviderCalls != len(controller.ProviderReceiptHashes) || controller.ProviderAccountingStatus != "exact" || controller.ProviderTokenAccountingStatus != "exact" || controller.ProviderCostAccountingStatus != "exact" || controller.AmbiguousProviderCalls != 0 || controller.PendingProviderCall != nil || controller.PendingPaidTurn != nil {
		return fmt.Errorf("R02 repeat paid accounting is not exact and closed")
	}
	if controller.InputTokens+controller.OutputTokens+controller.ReasoningTokens > admission.MaxTotalTokens || controller.OutputTokens+controller.ReasoningTokens > admission.MaxGeneratedTokens || controller.CostMicros > admission.MaxSpendMicros || len(controller.ToolEvents) > admission.MaxActions || controller.Attempt != 1 || len(controller.AttemptHistory) != 0 {
		return fmt.Errorf("R02 repeat state is outside the admitted one-attempt envelope")
	}
	return nil
}
