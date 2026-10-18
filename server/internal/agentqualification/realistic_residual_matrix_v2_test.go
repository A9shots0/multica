package agentqualification

import "testing"

func TestRealisticResidualRunMatrixV2IsNarrowCappedAndReviewed(t *testing.T) {
	matrix, err := CurrentRealisticResidualRunMatrixV2()
	if err != nil {
		t.Fatal(err)
	}
	if matrix.Status != "frozen_reviewed" || len(matrix.Candidates) != 1 || matrix.Candidates[0].Model != "openrouter/openai/gpt-5.4" || matrix.Candidates[0].Effort != "medium" {
		t.Fatalf("unexpected residual matrix: %+v", matrix)
	}
	if matrix.Retries != 0 || matrix.ProviderFallbacks || matrix.MaxIncrementalSpendMicros != 1_000_000 || matrix.MaxTotalTokens != 150_000 {
		t.Fatalf("residual matrix caps drifted: %+v", matrix)
	}
}

func TestRealisticResidualRunMatrixV2KeepsLocalExhaustionOutOfGlobalHalts(t *testing.T) {
	matrix, _ := CurrentRealisticResidualRunMatrixV2()
	if !exactSet(matrix.GlobalHaltReasons, "credential_failure", "runtime_or_config_failure", "credit_failure", "transport_failure", "unsafe_execution", "campaign_budget_breach", "uncontrolled_spending", "evidence_corruption_making_further_results_unusable") {
		t.Fatalf("global halts drifted: %+v", matrix.GlobalHaltReasons)
	}
	if !contains(matrix.CandidateEliminationReasons, "candidate_resource_exhaustion") {
		t.Fatalf("candidate exhaustion is not local: %+v", matrix.CandidateEliminationReasons)
	}
}
