package agentqualification

import (
	"encoding/json"
	"testing"
)

func TestFirstScreeningBatchIsSmallControlledAndFitsCeiling(t *testing.T) {
	batch, err := FirstScreeningBatch()
	if err != nil {
		t.Fatal(err)
	}
	projection := ProjectScreeningBatch(batch)
	if projection.Observations != 2 || projection.ReservedTokens != 30000 || !projection.FitsDeclaredCeiling {
		t.Fatalf("projection = %+v", projection)
	}
	if batch.ProductionAllowed || batch.ExternalSideEffectsAllowed || batch.NaturalLanguageIntakeRun {
		t.Fatalf("unsafe screening boundary: %+v", batch)
	}
	if len(batch.CapacityBudgets) != 1 || batch.CapacityBudgets[0].MaxUsedIncreaseBasisPoints != 200 {
		t.Fatalf("unexpected subscription cap: %+v", batch.CapacityBudgets)
	}
}

func TestScreeningPacketSeparatesCandidateInputFromHiddenTruth(t *testing.T) {
	packet, err := CandidateScreeningPacket("T11")
	if err != nil {
		t.Fatal(err)
	}
	if containsText(string(packet), "hidden_truth") || containsText(string(packet), "best_supported_cause") {
		t.Fatal("candidate packet leaked hidden grader truth")
	}
	batch, err := FirstScreeningBatch()
	if err != nil {
		t.Fatal(err)
	}
	if !batch.GraderBlindToCandidate || batch.ExecutionOrderPolicy != "seeded_permutation_v1" || batch.ExecutionOrderSeed == "" {
		t.Fatalf("bias controls missing: %+v", batch)
	}
	if len(batch.Cases) != 1 {
		t.Fatalf("unexpected cases: %+v", batch.Cases)
	}
	candidateHash, _ := EmbeddedScreeningPacketSHA256("T11", "candidate")
	graderHash, _ := EmbeddedScreeningPacketSHA256("T11", "grader")
	if batch.Cases[0].CandidatePacketSHA256 != candidateHash || batch.Cases[0].GraderPacketSHA256 != graderHash {
		t.Fatal("batch is not pinned to the embedded screening packets")
	}
}

func TestScreeningBatchRejectsChangedPacketOrUnblindedGrader(t *testing.T) {
	batch, err := FirstScreeningBatch()
	if err != nil {
		t.Fatal(err)
	}
	batch.Cases[0].CandidatePacketSHA256 = "changed"
	if err := ValidateScreeningBatch(batch); err == nil {
		t.Fatal("changed candidate packet was accepted")
	}
	batch, _ = FirstScreeningBatch()
	batch.GraderBlindToCandidate = false
	if err := ValidateScreeningBatch(batch); err == nil {
		t.Fatal("unblinded grader was accepted")
	}
}

func containsText(value, target string) bool {
	for i := 0; i+len(target) <= len(value); i++ {
		if value[i:i+len(target)] == target {
			return true
		}
	}
	return false
}

func TestScreeningCandidatesChangeOnlyModelConfiguration(t *testing.T) {
	batch, err := FirstScreeningBatch()
	if err != nil {
		t.Fatal(err)
	}
	models := map[string]struct{}{}
	for _, candidate := range batch.Candidates {
		models[candidate.Model+":"+candidate.ReasoningEffort] = struct{}{}
	}
	if len(models) != len(batch.Candidates) {
		t.Fatalf("candidate comparison collapsed: %+v", models)
	}
}

func TestScreeningBatchRejectsExhaustiveOrFailOpenShape(t *testing.T) {
	batch, err := FirstScreeningBatch()
	if err != nil {
		t.Fatal(err)
	}
	batch.TokenCeiling = batch.PerLaunchTokenReservation
	if err := ValidateScreeningBatch(batch); err == nil {
		t.Fatal("batch whose planned reservations exceed ceiling was accepted")
	}
	batch, _ = FirstScreeningBatch()
	batch.CapacityBudgets[0].RequireFreshAuthoritativeRead = false
	if err := ValidateScreeningBatch(batch); err == nil {
		t.Fatal("batch without authoritative subscription read was accepted")
	}
}

func TestScreeningBatchCallersCannotMutateCanonicalPlan(t *testing.T) {
	first, err := FirstScreeningBatch()
	if err != nil {
		t.Fatal(err)
	}
	first.Candidates[0].SkillVersions["silver_mediums_context"] = "mutated"
	first.CapacityBudgets[0].RequireFreshAuthoritativeRead = false
	second, err := FirstScreeningBatch()
	if err != nil {
		t.Fatal(err)
	}
	if second.Candidates[0].SkillVersions["silver_mediums_context"] != "item14.v1" || !second.CapacityBudgets[0].RequireFreshAuthoritativeRead {
		t.Fatalf("canonical batch was mutated through a caller copy: %+v", second)
	}
}

func TestCapacityAdmissionStopsAtPercentageCeiling(t *testing.T) {
	budget := CapacityBudget{Provider: "codex", LimitID: "codex", AccountLane: "primary_subscription", MaxUsedIncreaseBasisPoints: 300, MaxSnapshotAgeSeconds: 120, RequireFreshAuthoritativeRead: true}
	baseline := CapacitySnapshot{Provider: "codex", LimitID: "codex", AccountLane: "primary_subscription", WindowID: "weekly-1", UsedBasisPoints: 4800, ObservedAt: "2026-09-11T12:00:00Z", Source: "authoritative_account_usage"}
	current := baseline
	current.UsedBasisPoints = 5099
	if verdict := EvaluateCapacityAdmission(budget, baseline, current, "2026-09-11T12:01:00Z"); !verdict.CanLaunch {
		t.Fatalf("below-ceiling launch rejected: %+v", verdict)
	}
	current.UsedBasisPoints = 5100
	if verdict := EvaluateCapacityAdmission(budget, baseline, current, "2026-09-11T12:01:00Z"); verdict.CanLaunch || !contains(verdict.Reasons, "capacity_increase_ceiling_reached") {
		t.Fatalf("ceiling launch admitted: %+v", verdict)
	}
}

func TestCapacityAdmissionFailsClosedAcrossWindowOrUnknownSource(t *testing.T) {
	budget := CapacityBudget{Provider: "codex", LimitID: "codex", AccountLane: "primary_subscription", MaxUsedIncreaseBasisPoints: 300, MaxSnapshotAgeSeconds: 120, RequireFreshAuthoritativeRead: true}
	baseline := CapacitySnapshot{Provider: "codex", LimitID: "codex", AccountLane: "primary_subscription", WindowID: "weekly-1", UsedBasisPoints: 4800, ObservedAt: "2026-09-11T12:00:00Z", Source: "authoritative_account_usage"}
	current := baseline
	current.WindowID = "weekly-2"
	current.Source = "guess"
	verdict := EvaluateCapacityAdmission(budget, baseline, current, "2026-09-11T12:01:00Z")
	if verdict.CanLaunch || !contains(verdict.Reasons, "capacity_window_changed") || !contains(verdict.Reasons, "capacity_snapshot_invalid") {
		t.Fatalf("incomparable capacity admitted: %+v", verdict)
	}
}

func TestCapacityAdmissionRejectsStaleRead(t *testing.T) {
	budget := CapacityBudget{Provider: "codex", LimitID: "codex", AccountLane: "primary_subscription", MaxUsedIncreaseBasisPoints: 300, MaxSnapshotAgeSeconds: 120, RequireFreshAuthoritativeRead: true}
	baseline := CapacitySnapshot{Provider: "codex", LimitID: "codex", AccountLane: "primary_subscription", WindowID: "weekly-1", UsedBasisPoints: 4800, ObservedAt: "2026-09-11T12:00:00Z", Source: "authoritative_account_usage"}
	verdict := EvaluateCapacityAdmission(budget, baseline, baseline, "2026-09-11T12:02:01Z")
	if verdict.CanLaunch || !contains(verdict.Reasons, "capacity_snapshot_stale") {
		t.Fatalf("stale capacity read admitted: %+v", verdict)
	}
}

func TestScreeningLaunchBindsPlanUsageAndCapacity(t *testing.T) {
	batch, err := CurrentScreeningBatch()
	if err != nil {
		t.Fatal(err)
	}
	ledger := BatchUsageLedger{SchemaVersion: BatchUsageSchemaVersion, BatchID: batch.BatchID, TokenCeiling: batch.TokenCeiling}
	baseline := CapacitySnapshot{Provider: "codex", LimitID: "codex", AccountLane: "primary_subscription", WindowID: "weekly-1", UsedBasisPoints: 4800, ObservedAt: "2026-09-11T12:00:00Z", Source: "authoritative_account_usage"}
	current := baseline
	verdict := EvaluateScreeningLaunch(batch, ledger, baseline, current, "2026-09-11T12:01:00Z")
	if !verdict.CanLaunch {
		t.Fatalf("clean bounded launch rejected: %+v", verdict)
	}
	ledger.TokenCeiling++
	verdict = EvaluateScreeningLaunch(batch, ledger, baseline, current, "2026-09-11T12:01:00Z")
	if verdict.CanLaunch || !contains(verdict.Reasons, "batch_usage_ledger_mismatch") {
		t.Fatalf("mismatched budget ledger admitted: %+v", verdict)
	}
}

func TestScreeningLaunchRejectsObservationOutsidePlanButStillCountsCost(t *testing.T) {
	batch, err := CurrentScreeningBatch()
	if err != nil {
		t.Fatal(err)
	}
	record := validObservation(t)
	record.CandidateID = "not-in-batch"
	ledger := BatchUsageLedger{SchemaVersion: BatchUsageSchemaVersion, BatchID: batch.BatchID, TokenCeiling: batch.TokenCeiling, ObservationReceipts: []ObservationRecord{record}}
	baseline := CapacitySnapshot{Provider: "codex", LimitID: "codex", AccountLane: "primary_subscription", WindowID: "weekly-1", UsedBasisPoints: 4800, ObservedAt: "2026-09-11T12:00:00Z", Source: "authoritative_account_usage"}
	verdict := EvaluateScreeningLaunch(batch, ledger, baseline, baseline, "2026-09-11T12:01:00Z")
	if verdict.CanLaunch || !contains(verdict.Reasons, "batch_observation_candidate_unknown:not-in-batch") {
		t.Fatalf("out-of-plan observation admitted: %+v", verdict)
	}
	if got := verdict.Usage.FreshInputTokens + verdict.Usage.CachedInputTokens + verdict.Usage.OutputTokens; got != 130 {
		t.Fatalf("out-of-plan paid work disappeared from usage: %+v", verdict)
	}
}

func TestHistoricalV1InstructionConflictCannotLaunch(t *testing.T) {
	batch, err := FirstScreeningBatch()
	if err != nil {
		t.Fatal(err)
	}
	ledger := BatchUsageLedger{SchemaVersion: BatchUsageSchemaVersion, BatchID: batch.BatchID, TokenCeiling: batch.TokenCeiling}
	capacity := CapacitySnapshot{Provider: "codex", LimitID: "codex", AccountLane: "primary_subscription", WindowID: "weekly-1", UsedBasisPoints: 5100, ObservedAt: "2026-09-11T12:00:00Z", Source: "authoritative_account_usage"}
	verdict := EvaluateScreeningLaunch(batch, ledger, capacity, capacity, "2026-09-11T12:01:00Z")
	if verdict.CanLaunch || !containsTextInList(verdict.Reasons, "screening_instruction_incompatible:") {
		t.Fatalf("historical incompatible instructions were admitted: %+v", verdict)
	}
}

func TestHistoricalV2DescriptionFetchTransportCannotLaunch(t *testing.T) {
	batch, err := SecondScreeningBatch()
	if err != nil {
		t.Fatal(err)
	}
	ledger := BatchUsageLedger{SchemaVersion: BatchUsageSchemaVersion, BatchID: batch.BatchID, TokenCeiling: batch.TokenCeiling}
	capacity := CapacitySnapshot{Provider: "codex", LimitID: "codex", AccountLane: "primary_subscription", WindowID: "weekly-1", UsedBasisPoints: 5200, ObservedAt: "2026-09-11T12:00:00Z", Source: "authoritative_account_usage"}
	verdict := EvaluateScreeningLaunch(batch, ledger, capacity, capacity, "2026-09-11T12:01:00Z")
	if verdict.CanLaunch || !containsTextInList(verdict.Reasons, "screening_instruction_incompatible:") {
		t.Fatalf("historical non-inline transport was admitted: %+v", verdict)
	}
}

func TestCurrentV5PinsCompatibleInstructionsAndEverySkillContentHash(t *testing.T) {
	batch, err := CurrentScreeningBatch()
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateScreeningInstructionCompatibility(batch); err != nil {
		t.Fatal(err)
	}
	batch.Candidates[0].InstructionSHA256 = "changed"
	if err := ValidateScreeningInstructionCompatibility(batch); err == nil {
		t.Fatal("changed instruction content hash was accepted")
	}
	batch, _ = CurrentScreeningBatch()
	batch.Candidates[0].SkillContentSHA256 = nil
	if err := ValidateScreeningInstructionCompatibility(batch); err == nil {
		t.Fatal("missing skill content hash was accepted")
	}
}

func TestHistoricalV3SkillAndPacketContractsAreIncompatible(t *testing.T) {
	batch, err := ThirdScreeningBatch()
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateScreeningInstructionCompatibility(batch); err == nil {
		t.Fatal("historical v3 instruction/skill conflict was accepted")
	}
}

func TestCurrentV5PublishesCompleteT12FieldTypes(t *testing.T) {
	packet, err := CandidateScreeningPacket("T12")
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		ResponseContract struct {
			Fields map[string]string `json:"fields"`
		} `json:"response_contract"`
	}
	if err := json.Unmarshal(packet, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"blast_radius", "remediation_plan", "verification_plan", "untouched_scope"} {
		if decoded.ResponseContract.Fields[field] != "array_of_strings" {
			t.Fatalf("field %s type is not public: %+v", field, decoded.ResponseContract.Fields)
		}
	}
}

func TestMinimumSuitePacketsAreVersionedTypedAndPublic(t *testing.T) {
	cases := []ScreeningCase{{CaseID: "T11", CaseVersion: "v4"}, {CaseID: "T12", CaseVersion: "v4"}, {CaseID: "T08", CaseVersion: "v2"}, {CaseID: "T16", CaseVersion: "v2"}}
	for _, screeningCase := range cases {
		if err := validatePublicResponseContract(screeningCase.CaseID, screeningCase.CaseVersion); err != nil {
			t.Fatalf("%s:%s: %v", screeningCase.CaseID, screeningCase.CaseVersion, err)
		}
		packet, err := CandidateScreeningPacketVersion(screeningCase.CaseID, screeningCase.CaseVersion)
		if err != nil {
			t.Fatal(err)
		}
		if containsText(string(packet), "hidden_truth") || containsText(string(packet), "automatic_disqualifiers") {
			t.Fatalf("%s leaked hidden grading data", screeningCase.CaseID)
		}
		for _, audience := range []string{"candidate", "grader"} {
			if _, err := EmbeddedScreeningPacketVersionSHA256(screeningCase.CaseID, screeningCase.CaseVersion, audience); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestHistoricalT11V1PacketRemainsImmutable(t *testing.T) {
	v1, err := CandidateScreeningPacketVersion("T11", "v1")
	if err != nil {
		t.Fatal(err)
	}
	v2, err := CandidateScreeningPacketVersion("T11", "v2")
	if err != nil {
		t.Fatal(err)
	}
	if string(v1) == string(v2) || !containsText(string(v1), "required_fields") || !containsText(string(v2), "additional_fields_allowed") {
		t.Fatal("T11 packet version separation was lost")
	}
}

func TestCalibratedPacketGenerationsDoNotOverwritePriorPackets(t *testing.T) {
	for _, item := range []struct {
		caseID string
		old    string
		latest string
	}{{"T11", "v3", "v4"}, {"T12", "v3", "v4"}, {"T08", "v1", "v2"}, {"T16", "v1", "v2"}} {
		oldPacket, err := CandidateScreeningPacketVersion(item.caseID, item.old)
		if err != nil {
			t.Fatal(err)
		}
		latestPacket, err := CandidateScreeningPacketVersion(item.caseID, item.latest)
		if err != nil {
			t.Fatal(err)
		}
		if string(oldPacket) == string(latestPacket) {
			t.Fatalf("%s %s was overwritten instead of versioned", item.caseID, item.old)
		}
	}
}

func containsTextInList(values []string, target string) bool {
	for _, value := range values {
		if len(value) >= len(target) && value[:len(target)] == target {
			return true
		}
	}
	return false
}
