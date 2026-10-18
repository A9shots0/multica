package agentqualification

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const R02RepeatAdmissionSchemaVersion = "silver-mediums.realistic-r02-repeat-admission.v1"

//go:embed realistic_r02_repeat_admission_v1.json
var realisticR02RepeatAdmissionRaw []byte

type R02RepeatAcceptedEvidence struct {
	Label                  string `json:"label"`
	CaseID                 string `json:"case_id"`
	ConfigurationID        string `json:"configuration_id"`
	ObservationPath        string `json:"observation_path"`
	ObservationSHA256      string `json:"observation_sha256"`
	ExecutionReceiptPath   string `json:"execution_receipt_path"`
	ExecutionReceiptSHA256 string `json:"execution_receipt_sha256"`
	GradePath              string `json:"grade_path"`
	GradeSHA256            string `json:"grade_sha256"`
}

type R02RepeatAdmission struct {
	SchemaVersion                  string                      `json:"schema_version"`
	AdmissionID                    string                      `json:"admission_id"`
	Status                         string                      `json:"status"`
	ProviderCallsAllowed           bool                        `json:"provider_calls_allowed"`
	MatrixID                       string                      `json:"matrix_id"`
	MatrixPath                     string                      `json:"matrix_path"`
	MatrixSHA256                   string                      `json:"matrix_sha256"`
	ConfigurationID                string                      `json:"configuration_id"`
	Model                          string                      `json:"model"`
	Effort                         string                      `json:"effort"`
	ServingProvider                string                      `json:"serving_provider"`
	CaseID                         string                      `json:"case_id"`
	MaxActions                     int                         `json:"max_actions"`
	MaxTotalTokens                 int64                       `json:"max_total_tokens"`
	MaxGeneratedTokens             int64                       `json:"max_generated_tokens"`
	MaxSpendMicros                 int64                       `json:"max_spend_micros"`
	Retries                        int                         `json:"retries"`
	ProviderFallbacks              bool                        `json:"provider_fallbacks"`
	OwnerInterventions             int                         `json:"owner_interventions"`
	GlobalHaltReasons              []string                    `json:"global_halt_reasons"`
	ObservationLocalInvalidReasons []string                    `json:"observation_local_invalid_reasons"`
	CandidateLocalFailureReasons   []string                    `json:"candidate_local_failure_reasons"`
	AcceptedEvidence               []R02RepeatAcceptedEvidence `json:"accepted_evidence"`
}

func CurrentR02RepeatAdmission() (R02RepeatAdmission, string, error) {
	var admission R02RepeatAdmission
	if err := json.Unmarshal(realisticR02RepeatAdmissionRaw, &admission); err != nil {
		return admission, "", err
	}
	hash := sha256.Sum256(realisticR02RepeatAdmissionRaw)
	return admission, hex.EncodeToString(hash[:]), ValidateR02RepeatAdmissionContract(admission)
}

func ValidateR02RepeatAdmissionContract(admission R02RepeatAdmission) error {
	if admission.SchemaVersion != R02RepeatAdmissionSchemaVersion || admission.AdmissionID != "realistic-r02-reliability-repeat-v1" {
		return fmt.Errorf("R02 repeat admission identity drifted")
	}
	if admission.Status != "review_pending" && admission.Status != "frozen_reviewed" || admission.ProviderCallsAllowed != (admission.Status == "frozen_reviewed") {
		return fmt.Errorf("R02 repeat review lock is invalid")
	}
	if admission.MatrixID != "realistic-foundation-screen-v11" || admission.MatrixPath != "server/internal/agentqualification/realistic_matrix_v11.json" || !validSHA256(admission.MatrixSHA256) {
		return fmt.Errorf("R02 repeat matrix binding drifted")
	}
	if admission.ConfigurationID != "realistic.claudesonnet5.medium.anchor.r02.repeat1.v11" || admission.Model != "openrouter/anthropic/claude-sonnet-5" || admission.Effort != "medium" || admission.ServingProvider != "anthropic" || admission.CaseID != "R02" {
		return fmt.Errorf("R02 repeat candidate tuple drifted")
	}
	if admission.MaxActions != 18 || admission.MaxTotalTokens != 75_000 || admission.MaxGeneratedTokens != 20_000 || admission.MaxSpendMicros != 350_000 || admission.Retries != 0 || admission.ProviderFallbacks || admission.OwnerInterventions != 0 {
		return fmt.Errorf("R02 repeat budget or retry envelope drifted")
	}
	if !exactSet(admission.GlobalHaltReasons, "credential_failure", "uncontrolled_spending", "unsafe_external_action", "evidence_corruption") || !exactSet(admission.ObservationLocalInvalidReasons, "non_safety_harness_imperfection", "provider_transport_imperfection") || !exactSet(admission.CandidateLocalFailureReasons, "competence_failure", "hard_disqualifier", "response_or_tool_protocol_failure") {
		return fmt.Errorf("R02 repeat stop policy drifted")
	}
	want := map[string]string{"accepted_r01": "R01", "accepted_r03": "R03", "accepted_r04": "R04", "accepted_browser_r02": "R02"}
	if len(admission.AcceptedEvidence) != len(want) {
		return fmt.Errorf("R02 repeat accepted evidence set drifted")
	}
	for _, item := range admission.AcceptedEvidence {
		if want[item.Label] != item.CaseID || item.ConfigurationID == "" || !safeFixtureRelativePath(item.ObservationPath) || !safeFixtureRelativePath(item.ExecutionReceiptPath) || !safeFixtureRelativePath(item.GradePath) || !validSHA256(item.ObservationSHA256) || !validSHA256(item.ExecutionReceiptSHA256) || !validSHA256(item.GradeSHA256) {
			return fmt.Errorf("R02 repeat accepted evidence contract is invalid: %s", item.Label)
		}
		delete(want, item.Label)
	}
	if len(want) != 0 {
		return fmt.Errorf("R02 repeat accepted evidence is incomplete")
	}
	return nil
}

// VerifyR02RepeatAdmissionEvidence proves the new generation's admission from
// immutable accepted receipts. It does not synthesize V10 campaign rows.
func VerifyR02RepeatAdmissionEvidence(repoRoot string, admission R02RepeatAdmission) error {
	if err := ValidateR02RepeatAdmissionContract(admission); err != nil {
		return err
	}
	if !filepath.IsAbs(repoRoot) {
		return fmt.Errorf("R02 repeat repository root must be absolute")
	}
	if err := verifyR02RepeatFileHash(repoRoot, admission.MatrixPath, admission.MatrixSHA256); err != nil {
		return err
	}
	matrix, err := CurrentRealisticRunMatrixByID(admission.MatrixID)
	if err != nil {
		return err
	}
	if len(matrix.Candidates) != 1 || matrix.Candidates[0].ConfigurationID != admission.ConfigurationID || matrix.Candidates[0].Model != admission.Model || matrix.Candidates[0].Effort != admission.Effort || matrix.Candidates[0].ServingProvider != admission.ServingProvider || len(matrix.ExecutionOrder) != 1 || matrix.ExecutionOrder[0] != "R02" || matrix.PerObservationTotalTokenReservation != admission.MaxTotalTokens || matrix.PerObservationGeneratedTokenCeiling != admission.MaxGeneratedTokens || matrix.Retries != 0 || matrix.ProviderFallbacks {
		return fmt.Errorf("unchanged V11 matrix does not match the R02 repeat admission")
	}
	for _, item := range admission.AcceptedEvidence {
		if err := verifyR02RepeatFileHash(repoRoot, item.ObservationPath, item.ObservationSHA256); err != nil {
			return err
		}
		if err := verifyR02RepeatFileHash(repoRoot, item.ExecutionReceiptPath, item.ExecutionReceiptSHA256); err != nil {
			return err
		}
		if err := verifyR02RepeatFileHash(repoRoot, item.GradePath, item.GradeSHA256); err != nil {
			return err
		}
		var observation RealisticObservation
		if err := readR02RepeatJSON(repoRoot, item.ObservationPath, &observation); err != nil || observation.CaseID != item.CaseID || observation.ConfigurationID != item.ConfigurationID || observation.ExecutionReceiptSHA256 != item.ExecutionReceiptSHA256 || observation.GradeSHA256 != item.GradeSHA256 {
			return fmt.Errorf("accepted observation identity is not hash-bound: %s", item.Label)
		}
		var receipt RealisticExecutionReceipt
		if err := readR02RepeatJSON(repoRoot, item.ExecutionReceiptPath, &receipt); err != nil || receipt.CaseID != item.CaseID || receipt.ConfigurationID != item.ConfigurationID || receipt.TerminalState != "completed" || receipt.Retries != 0 || receipt.OwnerInterventions != 0 || len(receipt.ExternalSideEffects) != 0 {
			return fmt.Errorf("accepted execution receipt is invalid: %s", item.Label)
		}
		evidenceRoot := filepath.Dir(filepath.Join(repoRoot, filepath.FromSlash(item.ExecutionReceiptPath)))
		controller, err := verifyR02RepeatProviderProvenance(evidenceRoot, receipt)
		if err != nil {
			return fmt.Errorf("accepted provider provenance is invalid (%s): %w", item.Label, err)
		}
		var grade BlindGrade
		if err := readR02RepeatJSON(repoRoot, item.GradePath, &grade); err != nil || grade.CaseID != item.CaseID || grade.Verdict != "PASS" {
			// R03's immutable original FAIL is intentionally retained. Its known
			// phrase-matcher defect was corrected only after the batch, so prove
			// acceptance by recomputing the current hidden verifier below.
			if item.Label != "accepted_r03" || observation.Outcome != "candidate_fail" || grade.Verdict != "FAIL" {
				return fmt.Errorf("accepted grade is not PASS: %s", item.Label)
			}
		} else if observation.Outcome != "pass" {
			return fmt.Errorf("accepted PASS grade disagrees with observation: %s", item.Label)
		}
		if item.Label == "accepted_r03" {
			if reasons := evaluateRealisticDeclarations("R03", receipt, controller.FinalSummary); len(reasons) != 0 {
				return fmt.Errorf("accepted R03 does not pass the corrected frozen declaration verifier: %v", reasons)
			}
		}
		if item.Label == "accepted_browser_r02" {
			seen := map[string]bool{}
			for _, proof := range receipt.BrowserProofs {
				if proof.RenderedTotal == 47 && proof.RenderedCardCount == 4 && !proof.HorizontalOverflow {
					seen[proof.Viewport] = true
				}
			}
			if !seen["narrow"] || !seen["medium"] || !seen["wide"] {
				return fmt.Errorf("accepted Browser receipt lacks the three passing viewports")
			}
		}
	}
	return nil
}

// verifyR02RepeatProviderProvenance independently binds each accepted receipt
// to the exact OpenRouter -> Anthropic Claude Sonnet 5 medium route and
// recomputes accounting from the immutable raw provider receipts. This avoids
// trusting mutable summary fields in either the observation or admission.
func verifyR02RepeatProviderProvenance(evidenceRoot string, receipt RealisticExecutionReceipt) (RealisticControllerResult, error) {
	var controller RealisticControllerResult
	if receipt.ProviderCalls < 1 || receipt.ProviderAccountingStatus != "exact" || receipt.ProviderTokenAccountingStatus != "exact" || receipt.ProviderCostAccountingStatus != "exact" {
		return controller, fmt.Errorf("receipt accounting is not exact")
	}
	controllerRaw, err := os.ReadFile(filepath.Join(evidenceRoot, filepath.FromSlash(receipt.ControllerResultArtifactPath)))
	if err != nil {
		return controller, err
	}
	if digestR02Repeat(controllerRaw) != receipt.ControllerResultSHA256 || json.Unmarshal(controllerRaw, &controller) != nil {
		return controller, fmt.Errorf("controller result is not hash-bound")
	}
	receiptsRaw, err := os.ReadFile(filepath.Join(evidenceRoot, filepath.FromSlash(receipt.ProviderReceiptArtifactPath)))
	if err != nil {
		return controller, err
	}
	if digestR02Repeat(receiptsRaw) != receipt.ProviderReceiptSHA256 {
		return controller, fmt.Errorf("provider receipt artifact is not hash-bound")
	}
	var sealed []sealedRealisticProviderReceipt
	if json.Unmarshal(receiptsRaw, &sealed) != nil || len(sealed) != receipt.ProviderCalls || len(receipt.ProviderReceiptHashes) != receipt.ProviderCalls {
		return controller, fmt.Errorf("provider receipt cardinality is invalid")
	}
	accounting := realisticProviderAccounting{}
	for index, entry := range sealed {
		digest := digestR02Repeat(entry.Raw)
		if entry.SHA256 != digest || receipt.ProviderReceiptHashes[index] != digest {
			return controller, fmt.Errorf("provider receipt %d hash mismatch", index+1)
		}
		if err := verifyR02RepeatRawRoute(entry.Raw); err != nil {
			return controller, fmt.Errorf("provider receipt %d: %w", index+1, err)
		}
		usage, tokensKnown, costKnown := recomputeKnownFailureUsage(entry.Raw)
		if !tokensKnown || !costKnown {
			return controller, fmt.Errorf("provider receipt %d accounting is not recomputable", index+1)
		}
		accounting.InputTokens += usage.InputTokens
		accounting.OutputTokens += usage.OutputTokens
		accounting.ReasoningTokens += usage.ReasoningTokens
		accounting.CostMicros += usage.CostMicros
	}
	if accounting.InputTokens != receipt.InputTokens || accounting.OutputTokens != receipt.OutputTokens || accounting.ReasoningTokens != receipt.ReasoningTokens || accounting.CostMicros != receipt.CostMicros {
		return controller, fmt.Errorf("provider accounting disagrees with raw receipts")
	}
	if controller.ProviderCalls != receipt.ProviderCalls || controller.InputTokens != receipt.InputTokens || controller.OutputTokens != receipt.OutputTokens || controller.ReasoningTokens != receipt.ReasoningTokens || controller.CostMicros != receipt.CostMicros || !equalStrings(controller.ProviderReceiptHashes, receipt.ProviderReceiptHashes) || controller.ProviderAccountingStatus != "exact" || controller.ProviderTokenAccountingStatus != "exact" || controller.ProviderCostAccountingStatus != "exact" {
		return controller, fmt.Errorf("controller accounting disagrees with execution receipt")
	}
	return controller, nil
}

func verifyR02RepeatRawRoute(raw []byte) error {
	var envelope struct {
		Export struct {
			Info struct {
				Model struct {
					ID         string `json:"id"`
					ProviderID string `json:"providerID"`
					Variant    string `json:"variant"`
				} `json:"model"`
			} `json:"info"`
		} `json:"export"`
		Routing struct {
			DirectProvider              string   `json:"direct_provider"`
			RequestedUpstreamProvider   string   `json:"requested_upstream_provider"`
			ProviderOrder               []string `json:"provider_order"`
			AllowFallbacks              bool     `json:"allow_fallbacks"`
			ActualUpstreamProviderState string   `json:"actual_upstream_provider_state"`
		} `json:"routing"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("raw receipt is not JSON")
	}
	model := envelope.Export.Info.Model
	route := envelope.Routing
	if model.ID != "anthropic/claude-sonnet-5" || model.ProviderID != "openrouter" || model.Variant != "medium" {
		return fmt.Errorf("model or effort route drifted")
	}
	if route.DirectProvider != "openrouter" || route.RequestedUpstreamProvider != "anthropic" || route.AllowFallbacks || len(route.ProviderOrder) != 1 || route.ProviderOrder[0] != "anthropic" {
		return fmt.Errorf("provider route or fallback policy drifted")
	}
	if route.ActualUpstreamProviderState != "unavailable_from_provider_metadata" && !bytes.EqualFold([]byte(route.ActualUpstreamProviderState), []byte("anthropic")) {
		return fmt.Errorf("actual serving provider disagrees with Anthropic route")
	}
	return nil
}

func digestR02Repeat(raw []byte) string {
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}

func R02RepeatBoundedMatrix(admission R02RepeatAdmission) (RealisticRunMatrix, error) {
	if err := ValidateR02RepeatAdmissionContract(admission); err != nil {
		return RealisticRunMatrix{}, err
	}
	matrix, err := CurrentRealisticRunMatrixByID(admission.MatrixID)
	if err != nil {
		return matrix, err
	}
	// The frozen V11 file remains byte-for-byte unchanged. This generation-owned
	// envelope only tightens its $5 maximum to the approved $0.35 ceiling.
	matrix.MaxIncrementalSpendMicros = admission.MaxSpendMicros
	matrix.GlobalHaltReasons = append([]string(nil), admission.GlobalHaltReasons...)
	matrix.CandidateEliminationReasons = append([]string(nil), admission.CandidateLocalFailureReasons...)
	if matrix.MaxIncrementalSpendMicros <= 0 || matrix.MaxIncrementalSpendMicros > 350_000 || matrix.MaxTotalTokens != 75_000 || matrix.PerObservationGeneratedTokenCeiling != 20_000 || matrix.PerObservationTotalTokenReservation != 75_000 || matrix.Retries != 0 || matrix.ProviderFallbacks || matrix.OwnerInterventions != 0 {
		return matrix, fmt.Errorf("R02 repeat bounded matrix envelope is invalid")
	}
	return matrix, nil
}

func RequireActiveR02RepeatAdmission(repoRoot string) (R02RepeatAdmission, string, error) {
	admission, hash, err := CurrentR02RepeatAdmission()
	if err != nil {
		return admission, hash, err
	}
	if err := VerifyR02RepeatAdmissionEvidence(repoRoot, admission); err != nil {
		return admission, hash, err
	}
	if admission.Status != "frozen_reviewed" || !admission.ProviderCallsAllowed {
		return admission, hash, fmt.Errorf("R02 repeat provider execution is locked pending independent review and activation")
	}
	return admission, hash, nil
}

func verifyR02RepeatFileHash(root, relative, want string) error {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		return fmt.Errorf("read R02 repeat prerequisite %s: %w", relative, err)
	}
	hash := sha256.Sum256(raw)
	if hex.EncodeToString(hash[:]) != want {
		return fmt.Errorf("R02 repeat prerequisite hash drifted: %s", relative)
	}
	return nil
}

func readR02RepeatJSON(root, relative string, target any) error {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}
