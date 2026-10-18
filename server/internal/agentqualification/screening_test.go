package agentqualification

import (
	"encoding/json"
	"testing"
)

func TestScreeningAssignmentsAreSeededCompleteAndCandidateSafe(t *testing.T) {
	batch, err := CurrentScreeningBatch()
	if err != nil {
		t.Fatal(err)
	}
	first, err := ScreeningAssignments(batch)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ScreeningAssignments(batch)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("assignment count = %d/%d", len(first), len(second))
	}
	for index := range first {
		if first[index].EvaluationBinding == "" || first[index].EvaluationBinding != second[index].EvaluationBinding || first[index].Configuration.ID != second[index].Configuration.ID {
			t.Fatalf("order/binding is not reproducible: %+v %+v", first, second)
		}
		if containsText(string(first[index].CandidatePacket), "hidden_truth") || containsText(string(first[index].CandidatePacket), "automatic_disqualifiers") {
			t.Fatal("candidate assignment leaked grader-only content")
		}
	}
	if first[0].Configuration.ID == first[1].Configuration.ID {
		t.Fatal("seeded plan omitted a candidate")
	}
}

func TestBlindSubmissionWithholdsCandidateIdentity(t *testing.T) {
	batch, _ := CurrentScreeningBatch()
	assignments, _ := ScreeningAssignments(batch)
	response := []byte(`{"owner_observation":"preserved","hypotheses":[],"first_divergence":"task cadence","root_cause":"scheduled task"}`)
	submission, binding, err := BuildBlindGradingSubmission(batch, assignments[0], response)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(submission)
	if err != nil {
		t.Fatal(err)
	}
	if containsText(string(raw), assignments[0].Configuration.ID) || containsText(string(raw), assignments[0].Configuration.Model) || containsText(string(raw), "reasoning_effort") {
		t.Fatalf("blind payload leaked candidate identity: %s", raw)
	}
	if binding.CandidateID != assignments[0].Configuration.ID || binding.SubmissionID != submission.SubmissionID || binding.EvaluationBinding != assignments[0].EvaluationBinding {
		t.Fatalf("sealed binding mismatch: %+v %+v", submission, binding)
	}
}

func TestBlindSubmissionRejectsInvalidOrOutOfPlanWork(t *testing.T) {
	batch, _ := CurrentScreeningBatch()
	assignments, _ := ScreeningAssignments(batch)
	if _, _, err := BuildBlindGradingSubmission(batch, assignments[0], []byte("not json")); err == nil {
		t.Fatal("invalid response was accepted")
	}
	changed := assignments[0]
	changed.EvaluationBinding = "changed"
	if _, _, err := BuildBlindGradingSubmission(batch, changed, []byte(`{}`)); err == nil {
		t.Fatal("out-of-plan assignment was accepted")
	}
}

func TestPrepareNextScreeningLaunchAppliesGateBeforeReturningAssignment(t *testing.T) {
	batch, _ := CurrentScreeningBatch()
	ledger := BatchUsageLedger{SchemaVersion: BatchUsageSchemaVersion, BatchID: batch.BatchID, TokenCeiling: batch.TokenCeiling}
	baseline := CapacitySnapshot{Provider: "codex", LimitID: "codex", AccountLane: "primary_subscription", WindowID: "weekly-1", UsedBasisPoints: 4800, ObservedAt: "2026-09-11T12:00:00Z", Source: "authoritative_account_usage"}
	prepared, err := PrepareNextScreeningLaunch(batch, ledger, baseline, baseline, "2026-09-11T12:01:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if !prepared.Admission.CanLaunch || prepared.Assignment == nil || prepared.Assignment.EvaluationBinding == "" {
		t.Fatalf("admitted launch missing assignment: %+v", prepared)
	}

	stale := baseline
	stale.ObservedAt = "2026-09-11T11:00:00Z"
	prepared, err = PrepareNextScreeningLaunch(batch, ledger, baseline, stale, "2026-09-11T12:01:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Admission.CanLaunch || prepared.Assignment != nil {
		t.Fatalf("rejected launch exposed a paid assignment: %+v", prepared)
	}
}

func TestPrepareNextScreeningLaunchPreservesInvalidAndContinuesUnaffectedCandidate(t *testing.T) {
	batch, _ := CurrentScreeningBatch()
	assignments, _ := ScreeningAssignments(batch)
	record := observationForScreeningAssignment(t, assignments[0], "invalid")
	ledger := BatchUsageLedger{SchemaVersion: BatchUsageSchemaVersion, BatchID: batch.BatchID, TokenCeiling: batch.TokenCeiling, ObservationReceipts: []ObservationRecord{record}}
	capacity := CapacitySnapshot{Provider: "codex", LimitID: "codex", AccountLane: "primary_subscription", WindowID: "weekly-1", UsedBasisPoints: 4800, ObservedAt: "2026-09-11T12:00:00Z", Source: "authoritative_account_usage"}
	prepared, err := PrepareNextScreeningLaunch(batch, ledger, capacity, capacity, "2026-09-11T12:01:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if !prepared.Admission.CanLaunch || prepared.Assignment == nil || prepared.Assignment.Configuration.ID == record.CandidateID {
		t.Fatalf("invalid observation did not continue an unaffected candidate: %+v", prepared)
	}
	if !contains(prepared.Admission.InvalidCandidateIDs, record.CandidateID) {
		t.Fatalf("invalid candidate was not preserved separately: %+v", prepared.Admission)
	}
}

func TestPrepareNextScreeningLaunchContinuesSameCandidateAfterObservationLocalInvalid(t *testing.T) {
	batch, _ := TwentySixthOpenRouterScreeningBatchPlan()
	assignments, _ := ScreeningAssignments(batch)
	invalidCandidateID := assignments[0].Configuration.ID
	records := []ObservationRecord{observationForScreeningAssignment(t, assignments[0], "invalid")}
	eliminated := make(map[string]struct{})
	for _, assignment := range assignments[1:] {
		if assignment.Configuration.ID == invalidCandidateID {
			continue
		}
		if _, exists := eliminated[assignment.Configuration.ID]; exists {
			continue
		}
		record := observationForScreeningAssignment(t, assignment, "failed")
		record.FailureClass = "model"
		sealObservation(t, &record)
		records = append(records, record)
		eliminated[assignment.Configuration.ID] = struct{}{}
	}
	ledger := BatchUsageLedger{SchemaVersion: BatchUsageSchemaVersion, BatchID: batch.BatchID, TokenCeiling: batch.TokenCeiling, ObservationReceipts: records}
	prepared, err := prepareScreeningAssignment(batch, ledger, ScreeningLaunchAdmission{CanLaunch: true})
	if err != nil {
		t.Fatal(err)
	}
	if !prepared.Admission.CanLaunch || prepared.Assignment == nil || prepared.Assignment.Configuration.ID != invalidCandidateID {
		t.Fatalf("observation-local invalid incorrectly eliminated its candidate: %+v", prepared)
	}
	if prepared.Assignment.EvaluationBinding == assignments[0].EvaluationBinding {
		t.Fatalf("invalid observation binding was selected for retry: %+v", prepared.Assignment)
	}
}

func TestPrepareNextScreeningLaunchUsesAssignmentBindingForCompletion(t *testing.T) {
	batch, _ := CurrentScreeningBatch()
	assignments, _ := ScreeningAssignments(batch)
	record := observationForScreeningAssignment(t, assignments[0], "passed")
	ledger := BatchUsageLedger{SchemaVersion: BatchUsageSchemaVersion, BatchID: batch.BatchID, TokenCeiling: batch.TokenCeiling, ObservationReceipts: []ObservationRecord{record}}
	capacity := CapacitySnapshot{Provider: "codex", LimitID: "codex", AccountLane: "primary_subscription", WindowID: "weekly-1", UsedBasisPoints: 4800, ObservedAt: "2026-09-11T12:00:00Z", Source: "authoritative_account_usage"}
	prepared, err := PrepareNextScreeningLaunch(batch, ledger, capacity, capacity, "2026-09-11T12:01:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Assignment == nil || prepared.Assignment.EvaluationBinding == assignments[0].EvaluationBinding {
		t.Fatalf("completed assignment was selected again: %+v", prepared)
	}
}

func observationForScreeningAssignment(t *testing.T, assignment ScreeningAssignment, status string) ObservationRecord {
	t.Helper()
	hash, err := ConfigurationSHA256(assignment.Configuration)
	if err != nil {
		t.Fatal(err)
	}
	failureClass, severity := "none", "none"
	if status != "passed" {
		failureClass, severity = "infrastructure", "p1"
	}
	observationID := "obs-" + assignment.Configuration.ID + "-" + assignment.CaseID
	record := ObservationRecord{
		SchemaVersion: ObservationSchemaVersion, ObservationID: observationID,
		EvaluationBinding: assignment.EvaluationBinding, CandidateID: assignment.Configuration.ID,
		CandidateVersion: "v1", CaseID: assignment.CaseID, CaseVersion: assignment.CaseVersion,
		HarnessVersion: "direct.v1", EnvironmentVersion: "isolated.v1", GraderVersion: "grader.v1",
		HumanCalibrationStatus: "calibrated", TrialStatus: status, FailureClass: failureClass, Severity: severity, Confidence: "high",
		UnaffectedControls: []string{"no external effects"}, DeterministicChecks: []EvidenceRef{{Kind: "fixture", SHA256: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}},
		StartedAt: "2026-09-11T12:00:00Z", CompletedAt: "2026-09-11T12:00:10Z",
		Configurations: []Configuration{assignment.Configuration}, ExpectedRuns: []ExpectedRun{{RunID: "run-1", Stage: "screen", ConfigurationID: assignment.Configuration.ID}},
		RunReceipts:    []RunReceipt{{RunID: "run-1", FamilyRootID: observationID, AgentID: "candidate", ConfigurationID: assignment.Configuration.ID, ConfigurationSHA256: hash, Provider: "openai", ReportedModelVersion: assignment.Configuration.Model, EvaluationBinding: assignment.EvaluationBinding, UsageSource: "provider_receipt", UsageBoundary: "subscription_reported_tokens", Usage: Int64Measurement{FreshInputTokens: i64(100), CachedInputTokens: i64(0), OutputTokens: i64(10), WallTimeMillis: i64(10000)}}},
		RawTraceSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", OutcomeEvidenceSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	sealObservation(t, &record)
	return record
}

func TestCandidateDisqualifierDoesNotHideUnseenComparisonCandidate(t *testing.T) {
	batch, err := NinthOpenRouterScreeningBatch()
	if err != nil {
		t.Fatal(err)
	}
	assignments, err := ScreeningAssignments(batch)
	if err != nil {
		t.Fatal(err)
	}
	failed := observationForScreeningAssignment(t, assignments[0], "failed")
	failed.FailureClass = "model"
	failed.Severity = "p2"
	failed.Disqualifiers = []string{"ignored_consumer"}
	sealObservation(t, &failed)
	ledger := BatchUsageLedger{SchemaVersion: BatchUsageSchemaVersion, BatchID: batch.BatchID, TokenCeiling: batch.TokenCeiling, ObservationReceipts: []ObservationRecord{failed}}
	snapshot := APISpendSnapshot{Provider: "openrouter", ProjectLane: "opencode_local_openrouter", Currency: "USD", WindowID: "openrouter_account_lifetime", SpentMicros: 102075332, ObservedAt: "2026-09-12T02:10:00Z", Source: "authoritative_api_usage"}
	prepared, err := PrepareNextScreeningAPILaunch(batch, ledger, snapshot, "2026-09-12T02:10:01Z")
	if err != nil {
		t.Fatal(err)
	}
	if !prepared.Admission.CanLaunch || prepared.Assignment == nil || prepared.Assignment.Configuration.ID != assignments[1].Configuration.ID {
		t.Fatalf("candidate-specific failure hid unseen candidate: %+v", prepared)
	}
}

func TestCandidateDisqualifierSkipsThatCandidateRemainingCases(t *testing.T) {
	batch, err := ThirteenthOpenRouterScreeningBatchPlan()
	if err != nil {
		t.Fatal(err)
	}
	assignments, err := ScreeningAssignments(batch)
	if err != nil {
		t.Fatal(err)
	}
	const eliminatedID = "screen.generalist.minimaxm27.default.openrouter.v13"
	var failedAssignment ScreeningAssignment
	for _, assignment := range assignments {
		if assignment.Configuration.ID == eliminatedID {
			failedAssignment = assignment
			break
		}
	}
	if failedAssignment.EvaluationBinding == "" {
		t.Fatal("MiniMax assignment not found")
	}
	failed := observationForScreeningAssignment(t, failedAssignment, "failed")
	failed.FailureClass = "model"
	failed.Severity = "p2"
	failed.Disqualifiers = []string{"invalid_response_format"}
	sealObservation(t, &failed)
	ledger := BatchUsageLedger{SchemaVersion: BatchUsageSchemaVersion, BatchID: batch.BatchID, TokenCeiling: batch.TokenCeiling, ObservationReceipts: []ObservationRecord{failed}}
	snapshot := APISpendSnapshot{Provider: "openrouter", ProjectLane: "opencode_local_openrouter", Currency: "USD", WindowID: "openrouter_account_lifetime", SpentMicros: 102075659, ObservedAt: "2026-09-12T02:12:00Z", Source: "authoritative_api_usage"}
	prepared, err := PrepareNextScreeningAPILaunch(batch, ledger, snapshot, "2026-09-12T02:12:01Z")
	if err != nil {
		t.Fatal(err)
	}
	if !prepared.Admission.CanLaunch || prepared.Assignment == nil {
		t.Fatalf("unrelated candidates did not continue: %+v", prepared)
	}
	if prepared.Assignment.Configuration.ID == eliminatedID {
		t.Fatalf("eliminated candidate received another case: %+v", prepared.Assignment)
	}
	if !contains(prepared.Admission.EliminatedCandidateIDs, eliminatedID) || prepared.Admission.SkippedAssignmentCount != 4 {
		t.Fatalf("elimination accounting is wrong: %+v", prepared.Admission)
	}
}

func TestBatchCompletesWhenEliminatedCandidateAssignmentsArePolicySkipped(t *testing.T) {
	batch, err := ThirteenthOpenRouterScreeningBatchPlan()
	if err != nil {
		t.Fatal(err)
	}
	assignments, err := ScreeningAssignments(batch)
	if err != nil {
		t.Fatal(err)
	}
	const eliminatedID = "screen.generalist.minimaxm27.default.openrouter.v13"
	records := make([]ObservationRecord, 0, 16)
	eliminatedRecorded := false
	for _, assignment := range assignments {
		if assignment.Configuration.ID == eliminatedID {
			if eliminatedRecorded {
				continue
			}
			failed := observationForScreeningAssignment(t, assignment, "failed")
			failed.FailureClass = "model"
			failed.Severity = "p2"
			failed.Disqualifiers = []string{"invalid_response_format"}
			sealObservation(t, &failed)
			records = append(records, failed)
			eliminatedRecorded = true
			continue
		}
		records = append(records, observationForScreeningAssignment(t, assignment, "passed"))
	}
	ledger := BatchUsageLedger{SchemaVersion: BatchUsageSchemaVersion, BatchID: batch.BatchID, TokenCeiling: batch.TokenCeiling, ObservationReceipts: records}
	snapshot := APISpendSnapshot{Provider: "openrouter", ProjectLane: "opencode_local_openrouter", Currency: "USD", WindowID: "openrouter_account_lifetime", SpentMicros: 102075659, ObservedAt: "2026-09-12T02:12:00Z", Source: "authoritative_api_usage"}
	prepared, err := PrepareNextScreeningAPILaunch(batch, ledger, snapshot, "2026-09-12T02:12:01Z")
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Admission.CanLaunch || prepared.Assignment != nil {
		t.Fatalf("completed batch still admitted a launch: %+v", prepared)
	}
	if prepared.Admission.CompletedCount != 16 || prepared.Admission.SkippedAssignmentCount != 4 || prepared.Admission.TerminalAssignmentCount != 20 {
		t.Fatalf("terminal assignment accounting is wrong: %+v", prepared.Admission)
	}
	if !contains(prepared.Admission.Reasons, "screening_batch_complete_with_candidate_eliminations") {
		t.Fatalf("completion reason missing: %+v", prepared.Admission)
	}
}

func TestUnsafeExecutionStillHaltsEntireBatch(t *testing.T) {
	batch, err := ThirteenthOpenRouterScreeningBatchPlan()
	if err != nil {
		t.Fatal(err)
	}
	assignments, err := ScreeningAssignments(batch)
	if err != nil {
		t.Fatal(err)
	}
	unsafe := observationForScreeningAssignment(t, assignments[0], "failed")
	unsafe.FailureClass = "unsafe_execution"
	unsafe.Severity = "p0"
	sealObservation(t, &unsafe)
	ledger := BatchUsageLedger{SchemaVersion: BatchUsageSchemaVersion, BatchID: batch.BatchID, TokenCeiling: batch.TokenCeiling, ObservationReceipts: []ObservationRecord{unsafe}}
	snapshot := APISpendSnapshot{Provider: "openrouter", ProjectLane: "opencode_local_openrouter", Currency: "USD", WindowID: "openrouter_account_lifetime", SpentMicros: 102075659, ObservedAt: "2026-09-12T02:12:00Z", Source: "authoritative_api_usage"}
	prepared, err := PrepareNextScreeningAPILaunch(batch, ledger, snapshot, "2026-09-12T02:12:01Z")
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Admission.CanLaunch || prepared.Assignment != nil || !contains(prepared.Admission.Reasons, "batch_fatal_unsafe_execution:"+unsafe.ObservationID) {
		t.Fatalf("unsafe execution did not halt batch: %+v", prepared)
	}
}

func TestGeneratedTokenBudgetBreachHaltsEntireBatch(t *testing.T) {
	batch, err := ThirteenthOpenRouterScreeningBatchPlan()
	if err != nil {
		t.Fatal(err)
	}
	assignments, err := ScreeningAssignments(batch)
	if err != nil {
		t.Fatal(err)
	}
	overBudget := observationForScreeningAssignment(t, assignments[0], "failed")
	generated := int64(OpenCodeQualificationOutputTokenMaxV4 + 1)
	overBudget.RunReceipts[0].Usage.OutputTokens = &generated
	sealObservation(t, &overBudget)
	ledger := BatchUsageLedger{SchemaVersion: BatchUsageSchemaVersion, BatchID: batch.BatchID, TokenCeiling: batch.TokenCeiling, ObservationReceipts: []ObservationRecord{overBudget}}
	snapshot := APISpendSnapshot{Provider: "openrouter", ProjectLane: "opencode_local_openrouter", Currency: "USD", WindowID: "openrouter_account_lifetime", SpentMicros: 102075659, ObservedAt: "2026-09-12T02:12:00Z", Source: "authoritative_api_usage"}
	prepared, err := PrepareNextScreeningAPILaunch(batch, ledger, snapshot, "2026-09-12T02:12:01Z")
	if err != nil {
		t.Fatal(err)
	}
	want := "batch_generated_token_ceiling_breached:" + overBudget.ObservationID
	if prepared.Admission.CanLaunch || prepared.Assignment != nil || !contains(prepared.Admission.Reasons, want) {
		t.Fatalf("generated-token budget breach did not halt batch: %+v", prepared)
	}
}

func TestSharedFailureAcrossThreeCandidatesTriggersAuditWithoutHidingCheapComparisons(t *testing.T) {
	batch, err := ThirteenthOpenRouterScreeningBatchPlan()
	if err != nil {
		t.Fatal(err)
	}
	assignments, err := ScreeningAssignments(batch)
	if err != nil {
		t.Fatal(err)
	}
	records := []ObservationRecord{}
	seenCandidates := map[string]struct{}{}
	for _, assignment := range assignments {
		if assignment.CaseID != "T04" {
			continue
		}
		if _, seen := seenCandidates[assignment.Configuration.ID]; seen {
			continue
		}
		failed := observationForScreeningAssignment(t, assignment, "failed")
		failed.FailureClass = "model"
		failed.Severity = "p2"
		failed.Disqualifiers = []string{"patched_ui_only"}
		uniqueRunID := "run-" + assignment.Configuration.ID
		failed.ExpectedRuns[0].RunID = uniqueRunID
		failed.RunReceipts[0].RunID = uniqueRunID
		sealObservation(t, &failed)
		records = append(records, failed)
		seenCandidates[assignment.Configuration.ID] = struct{}{}
		if len(records) == 3 {
			break
		}
	}
	ledger := BatchUsageLedger{SchemaVersion: BatchUsageSchemaVersion, BatchID: batch.BatchID, TokenCeiling: batch.TokenCeiling, ObservationReceipts: records}
	admission := evaluateScreeningLaunchCore(batch, ledger)
	if !admission.CanLaunch || !contains(admission.AuditTriggers, "shared_model_failure_requires_case_audit:T04") {
		t.Fatalf("shared failure did not trigger a non-destructive test audit: %+v", admission)
	}
}
