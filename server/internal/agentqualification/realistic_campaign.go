package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
)

type RealisticObservation struct {
	ClaimID                   string `json:"claim_id"`
	ConfigurationID           string `json:"configuration_id"`
	CaseID                    string `json:"case_id"`
	Outcome                   string `json:"outcome"`
	FailureClass              string `json:"failure_class,omitempty"`
	ProviderCalls             int    `json:"provider_calls"`
	TotalTokens               int64  `json:"total_tokens"`
	CostMicros                int64  `json:"cost_micros"`
	ExecutionReceiptSHA256    string `json:"execution_receipt_sha256,omitempty"`
	GradeSHA256               string `json:"grade_sha256,omitempty"`
	ControllerResultSHA256    string `json:"controller_result_sha256,omitempty"`
	ProviderAccountingSHA256  string `json:"provider_accounting_sha256,omitempty"`
	FailureEvidenceSHA256     string `json:"failure_evidence_sha256,omitempty"`
	RawProviderReceiptsSHA256 string `json:"raw_provider_receipts_sha256,omitempty"`
}

// SealRealisticObservation binds the mutable campaign row to the independently
// written grade, execution receipt, controller result, and recomputed provider
// accounting before the row can be recorded as a completed case outcome.
func SealRealisticObservation(evidenceRoot, receiptPath, gradePath string, observation RealisticObservation, receipt RealisticExecutionReceipt) (RealisticObservation, string, error) {
	return sealRealisticObservation(evidenceRoot, receiptPath, gradePath, observation, receipt, false)
}

// SealOrReuseRealisticObservation preserves write-once semantics while making
// crash recovery idempotent. A pre-existing artifact is accepted only when its
// canonical JSON exactly matches the independently recomputed observation.
func SealOrReuseRealisticObservation(evidenceRoot, receiptPath, gradePath string, observation RealisticObservation, receipt RealisticExecutionReceipt) (RealisticObservation, string, error) {
	return sealRealisticObservation(evidenceRoot, receiptPath, gradePath, observation, receipt, true)
}

func sealRealisticObservation(evidenceRoot, receiptPath, gradePath string, observation RealisticObservation, receipt RealisticExecutionReceipt, reuseExact bool) (RealisticObservation, string, error) {
	if !filepath.IsAbs(evidenceRoot) || !filepath.IsAbs(receiptPath) || !filepath.IsAbs(gradePath) || receipt.ConfigurationID != observation.ConfigurationID || receipt.CaseID != observation.CaseID {
		return observation, "", fmt.Errorf("realistic observation identity or artifact path is invalid")
	}
	receiptRaw, err := os.ReadFile(receiptPath)
	if err != nil {
		return observation, "", err
	}
	var sealedReceipt RealisticExecutionReceipt
	argumentRaw, marshalErr := json.Marshal(receipt)
	var normalizedArgument RealisticExecutionReceipt
	if marshalErr != nil || json.Unmarshal(argumentRaw, &normalizedArgument) != nil || json.Unmarshal(receiptRaw, &sealedReceipt) != nil || !reflect.DeepEqual(sealedReceipt, normalizedArgument) {
		return observation, "", fmt.Errorf("realistic execution receipt argument does not match sealed artifact")
	}
	gradeRaw, err := os.ReadFile(gradePath)
	if err != nil {
		return observation, "", err
	}
	var grade BlindGrade
	if json.Unmarshal(gradeRaw, &grade) != nil || grade.CaseID != observation.CaseID || grade.Verdict != "PASS" && grade.Verdict != "FAIL" {
		return observation, "", fmt.Errorf("realistic grade binding is invalid")
	}
	wantOutcome := "pass"
	if grade.Verdict == "FAIL" {
		wantOutcome = "candidate_fail"
	}
	if observation.Outcome != wantOutcome {
		return observation, "", fmt.Errorf("realistic grade and observation outcome disagree")
	}
	receiptHash := sha256.Sum256(receiptRaw)
	gradeHash := sha256.Sum256(gradeRaw)
	accountingRaw, err := json.Marshal(struct {
		ProviderCalls                 int      `json:"provider_calls"`
		InputTokens                   int64    `json:"input_tokens"`
		OutputTokens                  int64    `json:"output_tokens"`
		ReasoningTokens               int64    `json:"reasoning_tokens"`
		CostMicros                    int64    `json:"cost_micros"`
		ProviderReceiptHashes         []string `json:"provider_receipt_hashes"`
		ProviderAccountingStatus      string   `json:"provider_accounting_status"`
		ProviderTokenAccountingStatus string   `json:"provider_token_accounting_status"`
		ProviderCostAccountingStatus  string   `json:"provider_cost_accounting_status"`
	}{receipt.ProviderCalls, receipt.InputTokens, receipt.OutputTokens, receipt.ReasoningTokens, receipt.CostMicros, receipt.ProviderReceiptHashes, receipt.ProviderAccountingStatus, receipt.ProviderTokenAccountingStatus, receipt.ProviderCostAccountingStatus})
	if err != nil {
		return observation, "", err
	}
	accountingHash := sha256.Sum256(accountingRaw)
	observation.ExecutionReceiptSHA256 = hex.EncodeToString(receiptHash[:])
	observation.GradeSHA256 = hex.EncodeToString(gradeHash[:])
	observation.ControllerResultSHA256 = receipt.ControllerResultSHA256
	observation.ProviderAccountingSHA256 = hex.EncodeToString(accountingHash[:])
	if observation.ProviderCalls != receipt.ProviderCalls || observation.TotalTokens != receipt.InputTokens+receipt.OutputTokens+receipt.ReasoningTokens || observation.CostMicros != receipt.CostMicros {
		return observation, "", fmt.Errorf("realistic observation mutable accounting does not match sealed receipt")
	}
	raw, err := json.MarshalIndent(observation, "", "  ")
	if err != nil {
		return observation, "", err
	}
	path := filepath.Join(evidenceRoot, "observation.json")
	if reuseExact {
		if existing, readErr := os.ReadFile(path); readErr == nil {
			var sealed RealisticObservation
			if json.Unmarshal(existing, &sealed) != nil || !reflect.DeepEqual(sealed, observation) {
				return observation, "", fmt.Errorf("persisted realistic observation disagrees with recomputed evidence")
			}
			return sealed, path, nil
		} else if !os.IsNotExist(readErr) {
			return observation, "", readErr
		}
	}
	if err := writeNewEvidenceFile(path, raw); err != nil {
		return observation, "", err
	}
	return observation, path, nil
}

type RealisticCampaignState struct {
	Revision                      int64                  `json:"revision"`
	Observations                  []RealisticObservation `json:"observations"`
	ActiveClaims                  map[string]string      `json:"active_claims"`
	Eliminated                    map[string]string      `json:"eliminated"`
	PausedCases                   map[string]string      `json:"paused_cases"`
	GlobalHaltReason              string                 `json:"global_halt_reason,omitempty"`
	TotalProviderCalls            int                    `json:"total_provider_calls"`
	TotalTokens                   int64                  `json:"total_tokens"`
	TotalCostMicros               int64                  `json:"total_cost_micros"`
	BaselineProviderSpendMicros   int64                  `json:"baseline_provider_spend_micros"`
	LatestProviderSpendMicros     int64                  `json:"latest_provider_spend_micros"`
	AuthoritativeDeltaMicros      int64                  `json:"authoritative_delta_micros"`
	BaselineProviderSpendRecorded bool                   `json:"baseline_provider_spend_recorded"`
}

type RealisticAssignment struct {
	Candidate RealisticCandidate `json:"candidate"`
	CaseID    string             `json:"case_id"`
}

func NewRealisticCampaignState() RealisticCampaignState {
	return RealisticCampaignState{Eliminated: map[string]string{}, PausedCases: map[string]string{}, ActiveClaims: map[string]string{}}
}

// NextRealisticAssignments enforces the frozen funnel: complete each case
// wave before advancing, while a shared-failure pause skips only that case.
func NextRealisticAssignments(state RealisticCampaignState, matrix RealisticRunMatrix) ([]RealisticAssignment, error) {
	if state.GlobalHaltReason != "" {
		return nil, fmt.Errorf("realistic campaign is halted")
	}
	if matrix.MatrixID == "realistic-foundation-screen-v11" && !realisticReliabilityPrerequisitesPassed(state) {
		return nil, nil
	}
	passed := map[string]map[string]bool{}
	observed := map[string]map[string]bool{}
	for _, observation := range state.Observations {
		if observed[observation.ConfigurationID] == nil {
			observed[observation.ConfigurationID] = map[string]bool{}
			passed[observation.ConfigurationID] = map[string]bool{}
		}
		observed[observation.ConfigurationID][observation.CaseID] = true
		passed[observation.ConfigurationID][observation.CaseID] = observation.Outcome == "pass"
	}
	for caseIndex, caseID := range matrix.ExecutionOrder {
		if _, paused := state.PausedCases[caseID]; paused {
			continue
		}
		assignments := []RealisticAssignment{}
		for _, candidate := range matrix.Candidates {
			if _, eliminated := state.Eliminated[candidate.ConfigurationID]; eliminated {
				continue
			}
			eligible := true
			for priorIndex := 0; priorIndex < caseIndex; priorIndex++ {
				priorCase := matrix.ExecutionOrder[priorIndex]
				if !passed[candidate.ConfigurationID][priorCase] {
					eligible = false
				}
			}
			if eligible && !observed[candidate.ConfigurationID][caseID] && state.ActiveClaims[realisticObservationKey(candidate.ConfigurationID, caseID)] == "" {
				assignments = append(assignments, RealisticAssignment{Candidate: candidate, CaseID: caseID})
			}
		}
		if len(assignments) != 0 {
			return assignments, nil
		}
	}
	return nil, nil
}

func ClaimRealisticAssignment(state RealisticCampaignState, matrix RealisticRunMatrix, candidateID, caseID, claimID string) (RealisticCampaignState, error) {
	if claimID == "" {
		return state, fmt.Errorf("realistic observation claim identity is required")
	}
	key := realisticObservationKey(candidateID, caseID)
	if existing := state.ActiveClaims[key]; existing != "" {
		if existing == claimID {
			return state, nil
		}
		return state, fmt.Errorf("realistic observation is already exclusively claimed")
	}
	next, err := NextRealisticAssignments(state, matrix)
	if err != nil {
		return state, err
	}
	allowed := false
	for _, assignment := range next {
		if assignment.Candidate.ConfigurationID == candidateID && assignment.CaseID == caseID {
			allowed = true
		}
	}
	if !allowed {
		return state, fmt.Errorf("realistic observation is not currently assignable")
	}
	if matrix.PerObservationTotalTokenReservation > 0 && state.TotalTokens+int64(len(state.ActiveClaims)+1)*matrix.PerObservationTotalTokenReservation > matrix.MaxTotalTokens {
		return state, fmt.Errorf("realistic campaign token reservation does not fit before provider launch")
	}
	if state.ActiveClaims == nil {
		state.ActiveClaims = map[string]string{}
	}
	state.ActiveClaims[key] = claimID
	return state, nil
}

func RecordRealisticObservation(state RealisticCampaignState, matrix RealisticRunMatrix, observation RealisticObservation) (RealisticCampaignState, error) {
	if state.GlobalHaltReason != "" {
		return state, fmt.Errorf("realistic campaign is halted")
	}
	zeroCallLocalInvalid := observation.Outcome == "invalid" && observation.FailureClass == "test_invalidity"
	if observation.ClaimID == "" || observation.ConfigurationID == "" || observation.CaseID == "" || observation.ProviderCalls < 0 || observation.TotalTokens < 0 || observation.CostMicros < 0 || observation.ProviderCalls < 1 && observation.Outcome != "global_halt" && !zeroCallLocalInvalid {
		return state, fmt.Errorf("realistic observation accounting is incomplete")
	}
	for _, existing := range state.Observations {
		if existing.ConfigurationID != observation.ConfigurationID || existing.CaseID != observation.CaseID {
			continue
		}
		if reflect.DeepEqual(existing, observation) {
			return state, nil
		}
		return state, fmt.Errorf("realistic observation was already recorded with different evidence")
	}
	key := realisticObservationKey(observation.ConfigurationID, observation.CaseID)
	if state.ActiveClaims[key] != observation.ClaimID {
		return state, fmt.Errorf("realistic observation claim is missing or stale")
	}
	if _, eliminated := state.Eliminated[observation.ConfigurationID]; eliminated {
		return state, fmt.Errorf("eliminated candidate cannot receive another observation")
	}
	if _, paused := state.PausedCases[observation.CaseID]; paused {
		return state, fmt.Errorf("paused case cannot receive another observation")
	}
	if !contains([]string{"pass", "candidate_fail", "invalid", "global_halt"}, observation.Outcome) {
		return state, fmt.Errorf("realistic observation outcome is invalid")
	}
	if observation.ProviderCalls == 0 {
		allowedZeroCall := observation.Outcome == "invalid" && observation.FailureClass == "test_invalidity" || observation.Outcome == "global_halt" && contains([]string{"test_invalidity", "budget_breach", "credential_failure"}, observation.FailureClass)
		if !allowedZeroCall || observation.TotalTokens != 0 || observation.CostMicros != 0 || !validSHA256(observation.FailureEvidenceSHA256) || !validSHA256(observation.RawProviderReceiptsSHA256) || !validSHA256(observation.ControllerResultSHA256) || !validSHA256(observation.ProviderAccountingSHA256) {
			return state, fmt.Errorf("zero-call realistic halt is not an approved sealed harness failure")
		}
	}
	if observation.ProviderCalls > 0 {
		gradedSeal := validSHA256(observation.ExecutionReceiptSHA256) && validSHA256(observation.GradeSHA256)
		failureSeal := validSHA256(observation.FailureEvidenceSHA256) && validSHA256(observation.RawProviderReceiptsSHA256)
		if !validSHA256(observation.ControllerResultSHA256) || !validSHA256(observation.ProviderAccountingSHA256) {
			return state, fmt.Errorf("paid realistic observation is not sealed to result and accounting")
		}
		if (observation.Outcome == "pass" || observation.FailureClass == "competence_failure") && !gradedSeal {
			return state, fmt.Errorf("graded realistic observation is not sealed to receipt, grade, result, and accounting")
		}
		if !gradedSeal && !failureSeal {
			return state, fmt.Errorf("paid failure observation is not sealed to result, raw receipts, evidence, and accounting")
		}
	}
	state.Observations = append(state.Observations, observation)
	delete(state.ActiveClaims, key)
	state.TotalProviderCalls += observation.ProviderCalls
	state.TotalTokens += observation.TotalTokens
	state.TotalCostMicros += observation.CostMicros
	if state.TotalTokens > matrix.MaxTotalTokens || state.TotalCostMicros > matrix.MaxIncrementalSpendMicros {
		state.GlobalHaltReason = "budget_breach"
		return state, nil
	}
	if observation.Outcome == "global_halt" {
		if !contains(matrix.GlobalHaltReasons, observation.FailureClass) {
			return state, fmt.Errorf("candidate failure cannot halt the campaign")
		}
		state.GlobalHaltReason = observation.FailureClass
		return state, nil
	}
	if observation.Outcome == "candidate_fail" {
		if !contains(matrix.CandidateEliminationReasons, observation.FailureClass) {
			return state, fmt.Errorf("candidate elimination reason is invalid")
		}
		state.Eliminated[observation.ConfigurationID] = observation.CaseID + ":" + observation.FailureClass
		failedCandidates := map[string]struct{}{}
		for _, prior := range state.Observations {
			if prior.CaseID == observation.CaseID && prior.Outcome == "candidate_fail" {
				failedCandidates[prior.ConfigurationID] = struct{}{}
			}
		}
		if len(failedCandidates) >= 2 && matrix.SharedFailurePolicy == "same_case_failure_by_two_unrelated_candidates_pauses_that_case_for_fixture_grader_runner_audit_without_halting_unrelated_valid_cases" {
			state.PausedCases[observation.CaseID] = "shared_failure_requires_fixture_grader_runner_audit"
		}
	}
	return state, nil
}

func realisticObservationKey(candidateID, caseID string) string { return candidateID + ":" + caseID }

func realisticReliabilityPrerequisitesPassed(state RealisticCampaignState) bool {
	const v10Candidate = "realistic.claudesonnet5.medium.anchor.repeat1.v10"
	passed := map[string]bool{}
	for _, observation := range state.Observations {
		if observation.ConfigurationID == v10Candidate && observation.Outcome == "pass" {
			passed[observation.CaseID] = true
		}
	}
	return passed["R01"] && passed["R03"] && passed["R04"]
}
