package agentqualification

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func sealFixtureObservation(observation RealisticObservation) RealisticObservation {
	hash := strings.Repeat("a", 64)
	observation.ExecutionReceiptSHA256 = hash
	observation.GradeSHA256 = hash
	observation.ControllerResultSHA256 = hash
	observation.ProviderAccountingSHA256 = hash
	observation.FailureEvidenceSHA256 = hash
	observation.RawProviderReceiptsSHA256 = hash
	return observation
}

func TestRealisticCandidateFailureIsLocalAndSharedFailurePausesCase(t *testing.T) {
	matrix, _ := CurrentRealisticRunMatrix()
	state := NewRealisticCampaignState()
	state, _ = ClaimRealisticAssignment(state, matrix, matrix.Candidates[0].ConfigurationID, "R01", "claim-1")
	first := sealFixtureObservation(RealisticObservation{ClaimID: "claim-1", ConfigurationID: matrix.Candidates[0].ConfigurationID, CaseID: "R01", Outcome: "candidate_fail", FailureClass: "competence_failure", ProviderCalls: 3, TotalTokens: 100, CostMicros: 20})
	state, err := RecordRealisticObservation(state, matrix, first)
	if err != nil || state.GlobalHaltReason != "" || len(state.Eliminated) != 1 || len(state.PausedCases) != 0 {
		t.Fatalf("first candidate failure was not local: state=%+v err=%v", state, err)
	}
	state, _ = ClaimRealisticAssignment(state, matrix, matrix.Candidates[1].ConfigurationID, "R01", "claim-2")
	second := sealFixtureObservation(RealisticObservation{ClaimID: "claim-2", ConfigurationID: matrix.Candidates[1].ConfigurationID, CaseID: "R01", Outcome: "candidate_fail", FailureClass: "hard_disqualifier", ProviderCalls: 2, TotalTokens: 80, CostMicros: 10})
	state, err = RecordRealisticObservation(state, matrix, second)
	if err != nil || state.GlobalHaltReason != "" || state.PausedCases["R01"] == "" {
		t.Fatalf("shared failure did not pause only the case: state=%+v err=%v", state, err)
	}
}

func TestContinuationCampaignLogsSharedFailuresWithoutPausingUnaffectedCandidate(t *testing.T) {
	matrix, err := CurrentRealisticRunMatrixByID("realistic-foundation-screen-v5")
	if err != nil {
		t.Fatal(err)
	}
	state := NewRealisticCampaignState()
	for index := 0; index < 2; index++ {
		candidate := matrix.Candidates[index]
		claim := "claim-" + candidate.ConfigurationID
		state, err = ClaimRealisticAssignment(state, matrix, candidate.ConfigurationID, "R03", claim)
		if err == nil {
			state, err = RecordRealisticObservation(state, matrix, sealFixtureObservation(RealisticObservation{ClaimID: claim, ConfigurationID: candidate.ConfigurationID, CaseID: "R03", Outcome: "candidate_fail", FailureClass: "competence_failure", ProviderCalls: 1, TotalTokens: 1, CostMicros: 1}))
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, paused := state.PausedCases["R03"]; paused {
		t.Fatal("continuation campaign paused unaffected candidates after shared competence failures")
	}
	next, err := NextRealisticAssignments(state, matrix)
	if err != nil || len(next) != 1 || next[0].Candidate.ConfigurationID != matrix.Candidates[2].ConfigurationID || next[0].CaseID != "R03" {
		t.Fatalf("unaffected continuation candidate was not preserved: next=%+v err=%v", next, err)
	}
}

func TestRealisticOnlyGlobalReasonsOrBudgetHaltCampaign(t *testing.T) {
	matrix, _ := CurrentRealisticRunMatrix()
	state := NewRealisticCampaignState()
	state, _ = ClaimRealisticAssignment(state, matrix, matrix.Candidates[0].ConfigurationID, "R01", "claim-invalid")
	invalid := RealisticObservation{ClaimID: "claim-invalid", ConfigurationID: matrix.Candidates[0].ConfigurationID, CaseID: "R01", Outcome: "global_halt", FailureClass: "competence_failure", ProviderCalls: 1, TotalTokens: 1, CostMicros: 1}
	if _, err := RecordRealisticObservation(state, matrix, invalid); err == nil {
		t.Fatal("competence failure halted the campaign")
	}
	overState := NewRealisticCampaignState()
	overState, _ = ClaimRealisticAssignment(overState, matrix, matrix.Candidates[0].ConfigurationID, "R01", "claim-budget")
	overBudget := sealFixtureObservation(RealisticObservation{ClaimID: "claim-budget", ConfigurationID: matrix.Candidates[0].ConfigurationID, CaseID: "R01", Outcome: "pass", ProviderCalls: 1, TotalTokens: matrix.MaxTotalTokens + 1, CostMicros: 1})
	state, err := RecordRealisticObservation(overState, matrix, overBudget)
	if err != nil || state.GlobalHaltReason != "budget_breach" {
		t.Fatalf("budget breach did not halt: state=%+v err=%v", state, err)
	}
}

func TestNextRealisticAssignmentsCompletesCaseWaveBeforeAdvancing(t *testing.T) {
	matrix, err := CurrentRealisticRunMatrix()
	if err != nil {
		t.Fatal(err)
	}
	state := NewRealisticCampaignState()
	next, err := NextRealisticAssignments(state, matrix)
	if err != nil || len(next) != 4 || next[0].CaseID != "R01" {
		t.Fatalf("unexpected first wave: %+v err=%v", next, err)
	}
	for _, candidate := range matrix.Candidates {
		claimID := "claim-" + candidate.ConfigurationID
		state, err = ClaimRealisticAssignment(state, matrix, candidate.ConfigurationID, "R01", claimID)
		if err == nil {
			state, err = RecordRealisticObservation(state, matrix, sealFixtureObservation(RealisticObservation{ClaimID: claimID, ConfigurationID: candidate.ConfigurationID, CaseID: "R01", Outcome: "pass", ProviderCalls: 1, TotalTokens: 10, CostMicros: 1}))
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	next, err = NextRealisticAssignments(state, matrix)
	if err != nil || len(next) != 4 || next[0].CaseID != "R03" {
		t.Fatalf("funnel advanced incorrectly: %+v err=%v", next, err)
	}
}

func TestPausedPrerequisiteAdvancesOnlyCandidatesThatAlreadyPassedIt(t *testing.T) {
	matrix, err := CurrentRealisticRunMatrix()
	if err != nil {
		t.Fatal(err)
	}
	state := NewRealisticCampaignState()
	state.Observations = []RealisticObservation{
		{ConfigurationID: matrix.Candidates[0].ConfigurationID, CaseID: "R01", Outcome: "pass", ProviderCalls: 1},
		{ConfigurationID: matrix.Candidates[1].ConfigurationID, CaseID: "R01", Outcome: "candidate_fail", FailureClass: "competence_failure", ProviderCalls: 1},
		{ConfigurationID: matrix.Candidates[2].ConfigurationID, CaseID: "R01", Outcome: "candidate_fail", FailureClass: "hard_disqualifier", ProviderCalls: 1},
	}
	state.Eliminated[matrix.Candidates[1].ConfigurationID] = "R01:competence_failure"
	state.Eliminated[matrix.Candidates[2].ConfigurationID] = "R01:hard_disqualifier"
	state.PausedCases["R01"] = "shared_failure_requires_fixture_grader_runner_audit"
	next, err := NextRealisticAssignments(state, matrix)
	if err != nil {
		t.Fatal(err)
	}
	if len(next) != 1 || next[0].Candidate.ConfigurationID != matrix.Candidates[0].ConfigurationID || next[0].CaseID != "R03" {
		t.Fatalf("paused prerequisite leaked an unproven candidate forward: %+v", next)
	}
}

func TestSealRealisticObservationBindsGradeResultAndAccounting(t *testing.T) {
	root, _ := filepath.Abs(t.TempDir())
	hash := strings.Repeat("a", 64)
	receipt := RealisticExecutionReceipt{
		ConfigurationID: "candidate-1", CaseID: "R01", ProviderCalls: 1, InputTokens: 10, OutputTokens: 4, ReasoningTokens: 2, CostMicros: 7,
		ProviderReceiptHashes: []string{hash}, ProviderAccountingStatus: "exact", ProviderTokenAccountingStatus: "exact", ProviderCostAccountingStatus: "exact", ControllerResultSHA256: hash,
	}
	receiptRaw, _ := json.MarshalIndent(receipt, "", "  ")
	receiptPath := filepath.Join(root, "execution-receipt.json")
	gradePath := filepath.Join(root, "grade.json")
	if err := os.WriteFile(receiptPath, receiptRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	gradeRaw, _ := json.MarshalIndent(BlindGrade{SchemaVersion: "fixture", CaseID: "R01", GraderVersion: "fixture", Verdict: "PASS"}, "", "  ")
	if err := os.WriteFile(gradePath, gradeRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	observation := RealisticObservation{ConfigurationID: "candidate-1", CaseID: "R01", Outcome: "pass", ProviderCalls: 1, TotalTokens: 16, CostMicros: 7}
	sealed, path, err := SealRealisticObservation(root, receiptPath, gradePath, observation, receipt)
	if err != nil {
		t.Fatal(err)
	}
	if !validSHA256(sealed.ExecutionReceiptSHA256) || !validSHA256(sealed.GradeSHA256) || sealed.ControllerResultSHA256 != hash || !validSHA256(sealed.ProviderAccountingSHA256) {
		t.Fatalf("observation hashes were not sealed: %+v", sealed)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestSealRealisticObservationRejectsMutableAccounting(t *testing.T) {
	root, _ := filepath.Abs(t.TempDir())
	hash := strings.Repeat("b", 64)
	receipt := RealisticExecutionReceipt{ConfigurationID: "candidate-1", CaseID: "R01", ProviderCalls: 1, InputTokens: 10, OutputTokens: 4, ReasoningTokens: 2, CostMicros: 7, ProviderReceiptHashes: []string{hash}, ProviderAccountingStatus: "exact", ProviderTokenAccountingStatus: "exact", ProviderCostAccountingStatus: "exact", ControllerResultSHA256: hash}
	receiptRaw, _ := json.MarshalIndent(receipt, "", "  ")
	receiptPath, gradePath := filepath.Join(root, "execution-receipt.json"), filepath.Join(root, "grade.json")
	_ = os.WriteFile(receiptPath, receiptRaw, 0o600)
	gradeRaw, _ := json.Marshal(BlindGrade{CaseID: "R01", Verdict: "PASS"})
	_ = os.WriteFile(gradePath, gradeRaw, 0o600)
	observation := RealisticObservation{ConfigurationID: "candidate-1", CaseID: "R01", Outcome: "pass", ProviderCalls: 1, TotalTokens: 17, CostMicros: 7}
	if _, _, err := SealRealisticObservation(root, receiptPath, gradePath, observation, receipt); err == nil || !strings.Contains(err.Error(), "mutable accounting") {
		t.Fatalf("mutable observation accounting was accepted: %v", err)
	}
}

func TestSealOrReuseRealisticObservationAcceptsOnlyExactCrashArtifact(t *testing.T) {
	root, _ := filepath.Abs(t.TempDir())
	hash := strings.Repeat("c", 64)
	receipt := RealisticExecutionReceipt{ConfigurationID: "candidate-1", CaseID: "R01", ProviderCalls: 1, InputTokens: 10, OutputTokens: 4, ReasoningTokens: 2, CostMicros: 7, ProviderReceiptHashes: []string{hash}, ProviderAccountingStatus: "exact", ProviderTokenAccountingStatus: "exact", ProviderCostAccountingStatus: "exact", ControllerResultSHA256: hash}
	receiptRaw, _ := json.MarshalIndent(receipt, "", "  ")
	receiptPath, gradePath := filepath.Join(root, "execution-receipt.json"), filepath.Join(root, "grade.json")
	_ = os.WriteFile(receiptPath, receiptRaw, 0o600)
	gradeRaw, _ := json.MarshalIndent(BlindGrade{CaseID: "R01", Verdict: "PASS"}, "", "  ")
	_ = os.WriteFile(gradePath, gradeRaw, 0o600)
	observation := RealisticObservation{ClaimID: "claim", ConfigurationID: "candidate-1", CaseID: "R01", Outcome: "pass", ProviderCalls: 1, TotalTokens: 16, CostMicros: 7}
	first, path, err := SealOrReuseRealisticObservation(root, receiptPath, gradePath, observation, receipt)
	if err != nil {
		t.Fatal(err)
	}
	second, secondPath, err := SealOrReuseRealisticObservation(root, receiptPath, gradePath, observation, receipt)
	if err != nil || secondPath != path || !reflect.DeepEqual(first, second) {
		t.Fatalf("exact sealed observation was not reusable: first=%+v second=%+v err=%v", first, second, err)
	}
	observation.CostMicros++
	if _, _, err := SealOrReuseRealisticObservation(root, receiptPath, gradePath, observation, receipt); err == nil {
		t.Fatal("conflicting crash artifact was accepted")
	}
}

func TestRecordRealisticObservationIsIdempotentAfterCampaignWrite(t *testing.T) {
	matrix, _ := CurrentRealisticRunMatrix()
	candidateID := matrix.Candidates[0].ConfigurationID
	state := NewRealisticCampaignState()
	state.ActiveClaims[candidateID+":R01"] = "claim"
	hash := strings.Repeat("d", 64)
	observation := RealisticObservation{ClaimID: "claim", ConfigurationID: candidateID, CaseID: "R01", Outcome: "pass", ProviderCalls: 1, TotalTokens: 10, CostMicros: 5, ExecutionReceiptSHA256: hash, GradeSHA256: hash, ControllerResultSHA256: hash, ProviderAccountingSHA256: hash}
	first, err := RecordRealisticObservation(state, matrix, observation)
	if err != nil {
		t.Fatal(err)
	}
	second, err := RecordRealisticObservation(first, matrix, observation)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Observations) != 1 || second.TotalProviderCalls != 1 || second.TotalTokens != 10 || second.TotalCostMicros != 5 {
		t.Fatalf("campaign retry duplicated accounting: %+v", second)
	}
}

func TestRealisticObservationRequiresExclusiveMatchingClaim(t *testing.T) {
	matrix, _ := CurrentRealisticRunMatrix()
	candidateID := matrix.Candidates[0].ConfigurationID
	state := NewRealisticCampaignState()
	observation := RealisticObservation{ClaimID: "forged", ConfigurationID: candidateID, CaseID: "R01", Outcome: "pass", ProviderCalls: 1}
	if _, err := RecordRealisticObservation(state, matrix, observation); err == nil {
		t.Fatal("observation without an exclusive claim was accepted")
	}
	state, err := ClaimRealisticAssignment(state, matrix, candidateID, "R01", "owner-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ClaimRealisticAssignment(state, matrix, candidateID, "R01", "owner-2"); err == nil {
		t.Fatal("duplicate observation claim was accepted")
	}
	observation.ClaimID = "owner-2"
	if _, err := RecordRealisticObservation(state, matrix, observation); err == nil {
		t.Fatal("stale claim recorded observation accounting")
	}
	observation.ClaimID = "owner-1"
	if _, err := RecordRealisticObservation(state, matrix, observation); err == nil {
		t.Fatal("unsealed graded observation was accepted")
	}
	observation = sealFixtureObservation(observation)
	state, err = RecordRealisticObservation(state, matrix, observation)
	if err != nil || len(state.Observations) != 1 || state.TotalProviderCalls != 1 || len(state.ActiveClaims) != 0 {
		t.Fatalf("matching claim was not consumed exactly once: state=%+v err=%v", state, err)
	}
}

func TestReliabilityBrowserRequiresAllV10Passes(t *testing.T) {
	matrix, err := CurrentRealisticRunMatrixByID("realistic-foundation-screen-v11")
	if err != nil {
		t.Fatal(err)
	}
	state := NewRealisticCampaignState()
	if next, nextErr := NextRealisticAssignments(state, matrix); nextErr != nil || len(next) != 0 {
		t.Fatalf("V11 started without V10 evidence: next=%+v err=%v", next, nextErr)
	}
	for _, caseID := range []string{"R01", "R03", "R04"} {
		state.Observations = append(state.Observations, RealisticObservation{ConfigurationID: "realistic.claudesonnet5.medium.anchor.repeat1.v10", CaseID: caseID, Outcome: "pass", ProviderCalls: 1})
	}
	next, err := NextRealisticAssignments(state, matrix)
	if err != nil || len(next) != 1 || next[0].CaseID != "R02" {
		t.Fatalf("V11 did not unlock after the complete V10 pass set: next=%+v err=%v", next, err)
	}
}

func TestReliabilityClaimReservesWorstCaseTokensBeforeProviderLaunch(t *testing.T) {
	matrix, err := CurrentRealisticRunMatrixByID("realistic-foundation-screen-v10")
	if err != nil {
		t.Fatal(err)
	}
	state := NewRealisticCampaignState()
	state.TotalTokens = matrix.MaxTotalTokens - matrix.PerObservationTotalTokenReservation + 1
	_, err = ClaimRealisticAssignment(state, matrix, matrix.Candidates[0].ConfigurationID, "R01", "over-budget-claim")
	if err == nil || !strings.Contains(err.Error(), "reservation does not fit") {
		t.Fatalf("provider launch was not blocked before the campaign ceiling: %v", err)
	}
	state.TotalTokens--
	claimed, err := ClaimRealisticAssignment(state, matrix, matrix.Candidates[0].ConfigurationID, "R01", "fits-exactly")
	if err != nil || claimed.ActiveClaims[matrix.Candidates[0].ConfigurationID+":R01"] != "fits-exactly" {
		t.Fatalf("exactly fitting token reservation was rejected: state=%+v err=%v", claimed, err)
	}
}

func TestObservationLocalInvalidDoesNotHaltOrEliminateCandidate(t *testing.T) {
	matrix, err := CurrentRealisticRunMatrixByID("realistic-foundation-screen-v13")
	if err != nil {
		t.Fatal(err)
	}
	state := NewRealisticCampaignState()
	candidate := matrix.Candidates[0]
	state.ActiveClaims[candidate.ConfigurationID+":R01"] = "invalid-r01"
	observation := sealFixtureObservation(RealisticObservation{
		ClaimID: "invalid-r01", ConfigurationID: candidate.ConfigurationID, CaseID: "R01",
		Outcome: "invalid", FailureClass: "provider_transport_failure", ProviderCalls: 1,
	})
	state, err = RecordRealisticObservation(state, matrix, observation)
	if err != nil {
		t.Fatal(err)
	}
	if state.GlobalHaltReason != "" || len(state.Eliminated) != 0 || len(state.PausedCases) != 0 {
		t.Fatalf("local invalid changed campaign eligibility globally: %+v", state)
	}
	next, err := NextRealisticAssignments(state, matrix)
	if err != nil {
		t.Fatal(err)
	}
	if len(next) != 3 || next[0].CaseID != "R01" {
		t.Fatalf("unaffected R01 candidates did not continue: %+v", next)
	}
}
