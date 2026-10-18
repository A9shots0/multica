package agentqualification

import "testing"

func TestRealisticMatrixIsCappedAndCandidateFailuresAreLocal(t *testing.T) {
	matrix, err := CurrentRealisticRunMatrix()
	if err != nil {
		t.Fatal(err)
	}
	if matrix.Status != "frozen_reviewed" || matrix.MaxIncrementalSpendMicros != 5000000 || matrix.Retries != 0 || !contains(matrix.CandidateEliminationReasons, "competence_failure") || contains(matrix.GlobalHaltReasons, "competence_failure") {
		t.Fatalf("realistic matrix is not safely capped: %+v", matrix)
	}
}

func TestRealisticMatrixRejectsFallbacksAndBroadHalt(t *testing.T) {
	matrix, _ := CurrentRealisticRunMatrix()
	matrix.ProviderFallbacks = true
	if err := ValidateRealisticRunMatrix(matrix); err == nil {
		t.Fatal("matrix allowed provider fallback")
	}
	matrix, _ = CurrentRealisticRunMatrix()
	matrix.GlobalHaltReasons = append(matrix.GlobalHaltReasons, "competence_failure")
	if err := ValidateRealisticRunMatrix(matrix); err == nil {
		t.Fatal("matrix allowed candidate competence failure to halt the batch")
	}
}

func TestNextRealisticMatrixUsesOnlyUnseenR01Configurations(t *testing.T) {
	matrix, err := CurrentRealisticRunMatrixByID("realistic-foundation-screen-v2")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"openrouter/mistralai/mistral-small-2603",
		"openrouter/nvidia/nemotron-3.5-lightning",
		"openrouter/openai/gpt-oss-20b",
		"openrouter/openai/gpt-5.4",
	}
	if matrix.MatrixID != "realistic-foundation-screen-v2" || len(matrix.Candidates) != len(want) {
		t.Fatalf("unexpected next matrix: %+v", matrix)
	}
	for index, candidate := range matrix.Candidates {
		if candidate.Model != want[index] {
			t.Fatalf("candidate %d drifted: %+v", index, candidate)
		}
		allowed, err := IsFrozenRealisticCandidate(candidate)
		if err != nil || !allowed {
			t.Fatalf("v2 candidate is not recognized by shared sealing/transport admission: %+v err=%v", candidate, err)
		}
	}
}

func TestThirdRealisticMatrixPinsRemainingCheapConfigurations(t *testing.T) {
	matrix, err := CurrentRealisticRunMatrixByID("realistic-foundation-screen-v3")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"openrouter/inclusionai/ling-3.0-flash",
		"openrouter/minimax/minimax-m2.7",
		"openrouter/xiaomi/mimo-v2.5",
		"openrouter/nvidia/nemotron-3-super-120b-a12b:free",
	}
	for index, candidate := range matrix.Candidates {
		if candidate.Model != want[index] {
			t.Fatalf("candidate %d drifted: %+v", index, candidate)
		}
		allowed, err := IsFrozenRealisticCandidate(candidate)
		if err != nil || !allowed {
			t.Fatalf("v3 candidate is not recognized by shared sealing/transport admission: %+v err=%v", candidate, err)
		}
	}
}

func TestFourthRealisticMatrixRerunsOnlyAffectedR01Configurations(t *testing.T) {
	matrix, err := CurrentRealisticRunMatrixByID("realistic-foundation-screen-v4")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"realistic.mercury25.default.r01v2.v4",
		"realistic.claudesonnet5.medium.anchor.r01v2.v4",
		"realistic.gpt54.medium.anchor.r01v2.v4",
		"realistic.nemotron3super.free.low.r01v2.v4",
	}
	if len(matrix.Candidates) != len(want) {
		t.Fatalf("corrected R01 matrix size drifted: %+v", matrix.Candidates)
	}
	for index, configurationID := range want {
		if matrix.Candidates[index].ConfigurationID != configurationID {
			t.Fatalf("corrected R01 candidate %d drifted: %+v", index, matrix.Candidates[index])
		}
		allowed, allowedErr := IsFrozenRealisticCandidate(matrix.Candidates[index])
		if allowedErr != nil || !allowed {
			t.Fatalf("v4 candidate is not recognized by admission: %+v err=%v", matrix.Candidates[index], allowedErr)
		}
	}
}

func TestReliabilityRepeatMatricesPreserveExactConfigurationAndGates(t *testing.T) {
	for _, tc := range []struct {
		matrixID string
		caseID   string
		wantID   string
	}{
		{"realistic-foundation-screen-v10", "R01", "realistic.claudesonnet5.medium.anchor.repeat1.v10"},
		{"realistic-foundation-screen-v11", "R02", "realistic.claudesonnet5.medium.anchor.r02.repeat1.v11"},
	} {
		matrix, err := CurrentRealisticRunMatrixByID(tc.matrixID)
		if err != nil {
			t.Fatal(err)
		}
		if matrix.Status != "frozen_reviewed" || len(matrix.Candidates) != 1 || matrix.Candidates[0].ConfigurationID != tc.wantID || matrix.Candidates[0].Model != "openrouter/anthropic/claude-sonnet-5" || matrix.Candidates[0].Effort != "medium" || matrix.Candidates[0].ServingProvider != "anthropic" {
			t.Fatalf("reliability matrix drifted: %+v", matrix)
		}
		if matrix.Retries != 0 || matrix.OwnerInterventions != 0 || matrix.ProviderFallbacks || contains(matrix.GlobalHaltReasons, "competence_failure") {
			t.Fatalf("reliability safeguards drifted: %+v", matrix)
		}
		allowed, allowedErr := IsFrozenRealisticCandidate(matrix.Candidates[0])
		if allowedErr != nil || !allowed {
			t.Fatalf("reliability candidate is not recognized by admission: %+v err=%v", matrix.Candidates[0], allowedErr)
		}
	}
}

func TestLunaProRealisticCapabilityMatrixIsCappedAndFrozenAfterReview(t *testing.T) {
	matrix, err := CurrentRealisticRunMatrixByID("realistic-foundation-screen-v12")
	if err != nil {
		t.Fatal(err)
	}
	if matrix.Status != "frozen_reviewed" || len(matrix.Candidates) != 1 {
		t.Fatalf("unexpected v12 reviewed matrix: %+v", matrix)
	}
	candidate := matrix.Candidates[0]
	if candidate.ConfigurationID != "realistic.gpt56lunapro.medium.openrouter.r01r03r04.v12" || candidate.Model != "openrouter/openai/gpt-5.6-luna-pro" || candidate.Effort != "medium" || candidate.ServingProvider != "openai" {
		t.Fatalf("v12 candidate drifted: %+v", candidate)
	}
	if matrix.Retries != 0 || matrix.OwnerInterventions != 0 || matrix.ProviderFallbacks || matrix.MaxTotalTokens != 225000 || matrix.PerObservationTotalTokenReservation != 75000 || contains(matrix.GlobalHaltReasons, "competence_failure") {
		t.Fatalf("v12 safeguards drifted: %+v", matrix)
	}
	allowed, allowedErr := IsFrozenRealisticCandidate(candidate)
	if allowedErr != nil || !allowed {
		t.Fatalf("v12 candidate is not recognized by admission: %+v err=%v", candidate, allowedErr)
	}
}

func TestV26SurvivorCapabilityMatrixIsCappedAndFrozenAfterReview(t *testing.T) {
	matrix, err := CurrentRealisticRunMatrixByID("realistic-foundation-screen-v13")
	if err != nil {
		t.Fatal(err)
	}
	if matrix.Status != "frozen_reviewed" || len(matrix.Candidates) != 4 || matrix.MaxTotalTokens != 900000 || matrix.PerObservationTotalTokenReservation != 75000 {
		t.Fatalf("unexpected v13 matrix: %+v", matrix)
	}
	want := []RealisticCandidate{
		{"realistic.gemini35flashlite.default.openrouter.r01r03r04.v13", "openrouter/google/gemini-3.5-flash-lite", "screening_default", "google-ai-studio"},
		{"realistic.nexn25pro.free.openrouter.r01r03r04.v13", "openrouter/nex-agi/nex-n2.5-pro:free", "screening_default", "nex-agi"},
		{"realistic.qwen3codernext.default.openrouter.r01r03r04.v13", "openrouter/qwen/qwen3-coder-next", "screening_default", "alibaba"},
		{"realistic.solarpro4.default.openrouter.r01r03r04.v13", "openrouter/upstage/solar-pro4", "screening_default", "upstage"},
	}
	for index := range want {
		if matrix.Candidates[index] != want[index] {
			t.Fatalf("v13 candidate %d drifted: %+v", index, matrix.Candidates[index])
		}
	}
	if matrix.Retries != 0 || matrix.OwnerInterventions != 0 || matrix.ProviderFallbacks || contains(matrix.GlobalHaltReasons, "test_invalidity") || contains(matrix.GlobalHaltReasons, "provider_transport_failure") {
		t.Fatalf("v13 stop or retry policy drifted: %+v", matrix)
	}
}

func TestAffectedClaudeR03ReliabilityMatrixIsLockedAndCapped(t *testing.T) {
	matrix, err := CurrentRealisticRunMatrixByID("realistic-foundation-screen-v14")
	if err != nil {
		t.Fatal(err)
	}
	want := RealisticCandidate{"realistic.claudesonnet5.medium.anchor.r03.parser-corrected.v14", "openrouter/anthropic/claude-sonnet-5", "medium", "anthropic"}
	if matrix.Status != "frozen_reviewed" || len(matrix.Candidates) != 1 || matrix.Candidates[0] != want {
		t.Fatalf("unexpected affected reliability matrix: %+v", matrix)
	}
	if !exactSet(matrix.ExecutionOrder, "R03") || !exactSet(matrix.DeferredCases, "R01", "R02", "R04") || matrix.MaxIncrementalSpendMicros != 500000 || matrix.MaxTotalTokens != 75000 || matrix.PerObservationTotalTokenReservation != 75000 {
		t.Fatalf("affected reliability scope or budget drifted: %+v", matrix)
	}
	if matrix.Retries != 0 || matrix.OwnerInterventions != 0 || matrix.ProviderFallbacks || contains(matrix.GlobalHaltReasons, "test_invalidity") || contains(matrix.GlobalHaltReasons, "provider_transport_failure") {
		t.Fatalf("affected reliability safeguards drifted: %+v", matrix)
	}
	allowed, allowedErr := IsFrozenRealisticCandidate(want)
	if allowedErr != nil || !allowed {
		t.Fatalf("affected reliability candidate is not recognized by admission: %+v err=%v", want, allowedErr)
	}
}
