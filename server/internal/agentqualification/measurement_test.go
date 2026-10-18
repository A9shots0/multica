package agentqualification

import (
	"reflect"
	"testing"
)

func i64(value int64) *int64 { return &value }

func sealObservation(t *testing.T, record *ObservationRecord) {
	t.Helper()
	pre, err := ExpectedRunsSHA256(record.ExpectedRuns, false)
	if err != nil {
		t.Fatal(err)
	}
	final, err := ExpectedRunsSHA256(record.ExpectedRuns, true)
	if err != nil {
		t.Fatal(err)
	}
	record.PreExecutionManifestSHA256 = pre
	record.FinalManifestSHA256 = final
	runIDs := make([]string, 0, len(record.RunReceipts))
	for _, receipt := range record.RunReceipts {
		runIDs = append(runIDs, receipt.RunID)
	}
	record.AuthoritativeLedger = RunLedgerSnapshot{
		Source: "controller_run_ledger", SnapshotID: "ledger-" + record.ObservationID,
		FamilyRootID: record.ObservationID, ClosedAt: "2026-09-11T12:01:01Z", RunIDs: runIDs,
	}
	hash, err := RunLedgerSHA256(record.AuthoritativeLedger)
	if err != nil {
		t.Fatal(err)
	}
	record.AuthoritativeLedger.SHA256 = hash
}

func validObservation(t *testing.T) ObservationRecord {
	t.Helper()
	configuration := Configuration{
		ID: "single-agent.sol.medium.v1", RoleID: "shared_windows_executor",
		Model: "gpt-5.6-sol", ReasoningEffort: "medium",
		InstructionVersion: "builder.v1", SkillVersions: map[string]string{"builder": "v1"},
		PermissionVersion: "shadow.v1", RuntimeProvider: "codex",
	}
	hash, err := ConfigurationSHA256(configuration)
	if err != nil {
		t.Fatal(err)
	}
	record := ObservationRecord{
		SchemaVersion: ObservationSchemaVersion, ObservationID: "obs-001",
		EvaluationBinding: "obs-001:T01",
		CandidateID:       "single-agent", CandidateVersion: "v1", CaseID: "T01", CaseVersion: "v1",
		HarnessVersion: "harness.v1", EnvironmentVersion: "env.v1", GraderVersion: "grader.v1",
		HumanCalibrationStatus: "calibrated", TrialStatus: "passed", FailureClass: "none", Severity: "none", Confidence: "high",
		UnaffectedControls:  []string{"control-1"},
		DeterministicChecks: []EvidenceRef{{Kind: "focused_test", SHA256: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}},
		StartedAt:           "2026-09-11T12:00:00Z", CompletedAt: "2026-09-11T12:01:00Z",
		Configurations: []Configuration{configuration},
		ExpectedRuns:   []ExpectedRun{{RunID: "run-1", Stage: "execute", ConfigurationID: configuration.ID}},
		RunReceipts: []RunReceipt{{
			RunID: "run-1", FamilyRootID: "obs-001", AgentID: "agent-1",
			ConfigurationID: configuration.ID, ConfigurationSHA256: hash,
			Provider: "openai", ReportedModelVersion: "gpt-5.6-sol",
			EvaluationBinding: "obs-001:T01", UsageSource: "provider_receipt",
			UsageBoundary: "subscription_reported_tokens",
			Usage:         Int64Measurement{FreshInputTokens: i64(100), CachedInputTokens: i64(20), OutputTokens: i64(10), WallTimeMillis: i64(5000)},
		}},
		RawTraceSHA256:        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		OutcomeEvidenceSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	sealObservation(t, &record)
	return record
}

func TestValidateObservationBindsReceiptsToImmutableAssignment(t *testing.T) {
	record := validObservation(t)
	record.EvaluationBinding = "assignment-binding"
	result := ValidateObservation(record)
	if result.Valid || !contains(result.Reasons, "run_evaluation_unbound:run-1") {
		t.Fatalf("mismatched assignment binding passed: %+v", result)
	}
}

func TestValidateObservationAcceptsCompleteSingleAgentAttempt(t *testing.T) {
	record := validObservation(t)
	result := ValidateObservation(record)
	if !result.Valid || !result.CostBaselineEligible {
		t.Fatalf("valid observation rejected: %+v", result)
	}
	if result.Totals.Runs != 1 || result.Totals.FreshInputTokens != 100 || result.Totals.CachedInputTokens != 20 || result.Totals.OutputTokens != 10 {
		t.Fatalf("totals = %+v", result.Totals)
	}
}

func TestValidateObservationIncludesEveryControllerRunAndRetry(t *testing.T) {
	record := validObservation(t)
	configuration := record.Configurations[0]
	hash, _ := ConfigurationSHA256(configuration)
	record.ExpectedRuns = append(record.ExpectedRuns, ExpectedRun{RunID: "run-2", Stage: "review", ConfigurationID: configuration.ID})
	record.RunReceipts = append(record.RunReceipts, RunReceipt{
		RunID: "run-2", FamilyRootID: record.ObservationID, ParentRunID: "run-1", AgentID: "reviewer-1",
		ConfigurationID: configuration.ID, ConfigurationSHA256: hash, Provider: "openai",
		ReportedModelVersion: "gpt-5.6-sol", EvaluationBinding: "obs-001:T01",
		UsageSource: "provider_receipt", UsageBoundary: "subscription_reported_tokens", Retry: true,
		Usage: Int64Measurement{FreshInputTokens: i64(50), CachedInputTokens: i64(5), OutputTokens: i64(7), WallTimeMillis: i64(2500)},
	})
	sealObservation(t, &record)
	result := ValidateObservation(record)
	if !result.Valid || result.Totals.Runs != 2 || result.Totals.Retries != 1 || result.Totals.FreshInputTokens != 150 || result.Totals.RunComputeMillis != 7500 || result.Totals.EndToEndMillis != 60000 {
		t.Fatalf("complete family not measured: %+v", result)
	}
}

func TestValidateObservationRejectsMissingAndUnexpectedRuns(t *testing.T) {
	record := validObservation(t)
	record.ExpectedRuns[0].RunID = "expected-run"
	sealObservation(t, &record)
	result := ValidateObservation(record)
	want := []string{"expected_run_receipt_missing:expected-run", "unexpected_run_receipt:run-1"}
	for _, reason := range want {
		if !contains(result.Reasons, reason) {
			t.Fatalf("reasons = %v, missing %q", result.Reasons, reason)
		}
	}
	if result.CostBaselineEligible {
		t.Fatal("incomplete run family became cost eligible")
	}
}

func TestValidateObservationRejectsRunHiddenFromManifest(t *testing.T) {
	record := validObservation(t)
	record.AuthoritativeLedger.RunIDs = append(record.AuthoritativeLedger.RunIDs, "hidden-run")
	hash, err := RunLedgerSHA256(record.AuthoritativeLedger)
	if err != nil {
		t.Fatal(err)
	}
	record.AuthoritativeLedger.SHA256 = hash
	result := ValidateObservation(record)
	if !contains(result.Reasons, "authoritative_ledger_unmanifested_run:hidden-run") {
		t.Fatalf("hidden run was not detected: %v", result.Reasons)
	}
}

func TestValidateObservationRejectsManifestMutation(t *testing.T) {
	record := validObservation(t)
	record.ExpectedRuns[0].Stage = "quietly-mutated"
	result := ValidateObservation(record)
	if !contains(result.Reasons, "pre_execution_manifest_hash_invalid") || !contains(result.Reasons, "final_manifest_hash_invalid") {
		t.Fatalf("manifest mutation was not detected: %v", result.Reasons)
	}
}

func TestValidateObservationRejectsContextlessOrIncompleteUsage(t *testing.T) {
	record := validObservation(t)
	record.RunReceipts[0].EvaluationBinding = ""
	record.RunReceipts[0].Usage.CachedInputTokens = nil
	result := ValidateObservation(record)
	want := []string{"run_cached_input_tokens_unavailable:run-1", "run_evaluation_unbound:run-1"}
	for _, reason := range want {
		if !contains(result.Reasons, reason) {
			t.Fatalf("reasons = %v, missing %q", result.Reasons, reason)
		}
	}
}

func TestConfigurationSHAChangesWithEffortOrInstructions(t *testing.T) {
	record := validObservation(t)
	base, _ := ConfigurationSHA256(record.Configurations[0])
	changed := record.Configurations[0]
	changed.ReasoningEffort = "high"
	effort, _ := ConfigurationSHA256(changed)
	changed = record.Configurations[0]
	changed.InstructionVersion = "builder.v2"
	instructions, _ := ConfigurationSHA256(changed)
	if base == effort || base == instructions || reflect.DeepEqual(effort, instructions) {
		t.Fatalf("configuration versions collapsed: base=%s effort=%s instructions=%s", base, effort, instructions)
	}
}

func TestBatchUsageCountsInvalidPaidWorkAndEnforcesCeiling(t *testing.T) {
	record := validObservation(t)
	record.TrialStatus = "invalid"
	record.RawTraceSHA256 = "bad"
	ledger := BatchUsageLedger{
		SchemaVersion: BatchUsageSchemaVersion, BatchID: "batch-1", TokenCeiling: 200,
		ObservationReceipts: []ObservationRecord{record},
	}
	verdict := EvaluateBatchUsage(ledger, 80)
	if verdict.CanLaunchNext || !contains(verdict.Reasons, "batch_token_ceiling_reached") {
		t.Fatalf("invalid paid work escaped ceiling: %+v", verdict)
	}
	if got := verdict.Totals.FreshInputTokens + verdict.Totals.CachedInputTokens + verdict.Totals.OutputTokens; got != 130 {
		t.Fatalf("spent tokens = %d, want 130", got)
	}
}

func TestBatchUsageFailsClosedWhenReceiptUsageUnknown(t *testing.T) {
	record := validObservation(t)
	record.RunReceipts[0].Usage.OutputTokens = nil
	ledger := BatchUsageLedger{
		SchemaVersion: BatchUsageSchemaVersion, BatchID: "batch-1", TokenCeiling: 1000,
		ObservationReceipts: []ObservationRecord{record},
	}
	verdict := EvaluateBatchUsage(ledger, 1)
	if verdict.CanLaunchNext || !contains(verdict.Reasons, "batch_run_usage_unknown:run-1") {
		t.Fatalf("unknown usage did not stop next launch: %+v", verdict)
	}
}

func TestBatchUsageFailsClosedWhenAuthoritativeRunHasNoReceipt(t *testing.T) {
	record := validObservation(t)
	record.AuthoritativeLedger.RunIDs = append(record.AuthoritativeLedger.RunIDs, "unreceipted-paid-run")
	hash, err := RunLedgerSHA256(record.AuthoritativeLedger)
	if err != nil {
		t.Fatal(err)
	}
	record.AuthoritativeLedger.SHA256 = hash
	ledger := BatchUsageLedger{
		SchemaVersion: BatchUsageSchemaVersion, BatchID: "batch-1", TokenCeiling: 1000,
		ObservationReceipts: []ObservationRecord{record},
	}
	verdict := EvaluateBatchUsage(ledger, 1)
	if verdict.CanLaunchNext || !contains(verdict.Reasons, "batch_authoritative_run_usage_missing:unreceipted-paid-run") {
		t.Fatalf("authoritative run without usage did not stop launch: %+v", verdict)
	}
}
