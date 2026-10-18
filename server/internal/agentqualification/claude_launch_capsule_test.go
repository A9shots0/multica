package agentqualification

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func claudeCapsuleTestBatch(t *testing.T) ScreeningBatchDefinition {
	t.Helper()
	suite, err := CurrentMinimumScreeningSuite()
	if err != nil {
		t.Fatal(err)
	}
	first := claudeTestConfiguration()
	second := claudeTestConfiguration()
	second.ID = "screen.generalist.claude.other.medium.v1"
	second.Model = "claude-haiku-4-5"
	batch := ScreeningBatchDefinition{
		SchemaVersion: ScreeningBatchSchemaVersion, BatchID: "screen-claude-capsule-fixture-v1", Status: "planned_not_launched",
		SuiteVersion: suite.SchemaVersion, ExecutionMode: "isolated_capability", CandidateTransport: ClaudeCodeToolFreeTransport,
		ExecutionOrderPolicy: "seeded_permutation_v1", ExecutionOrderSeed: "claude-capsule-fixture-v1",
		GraderBlindToCandidate: true, GraderConfigurationID: "deterministic-minimum-suite.v1", Cases: []ScreeningCase{suite.Cases[0]},
		Candidates: []Configuration{first, second}, Repeats: 1, TokenCeiling: 20000, PerLaunchTokenReservation: 10000,
		CapacityBudgets:       []CapacityBudget{{Provider: "claude", LimitID: "max", AccountLane: "fixture", MaxUsedIncreaseBasisPoints: 100, MaxSnapshotAgeSeconds: 120, RequireFreshAuthoritativeRead: true}},
		MaxOwnerInterventions: 0, ExcludedCapabilities: []string{"tools", "production_release", "external_messaging"},
		StopRules: []string{"stop_before_launch_when_subscription_capacity_read_is_missing_or_stale", "stop_before_launch_when_token_reservation_does_not_fit"},
	}
	if err := ValidateScreeningBatch(batch); err != nil {
		t.Fatal(err)
	}
	if err := ValidateScreeningInstructionCompatibility(batch); err != nil {
		t.Fatal(err)
	}
	return batch
}

func TestReserveClaudeLaunchCapsuleSealsNoInferenceArtifact(t *testing.T) {
	batch := claudeCapsuleTestBatch(t)
	ledger := BatchUsageLedger{SchemaVersion: BatchUsageSchemaVersion, BatchID: batch.BatchID, TokenCeiling: batch.TokenCeiling}
	capacity := CapacitySnapshot{Provider: "claude", LimitID: "max", AccountLane: "fixture", WindowID: "fixture-window", UsedBasisPoints: 0, ObservedAt: "2026-09-11T12:00:00Z", Source: "authoritative_account_usage"}
	prepared, reservation, capsule, dir, err := ReserveClaudeLaunchCapsule(t.TempDir(), batch, ledger, capacity, capacity, "2026-09-11T12:00:30Z", "2026-09-11T12:00:31Z")
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Assignment == nil || reservation.State != "reserved" || capsule.State != "ready_no_inference" || capsule.ProviderCallAllowed || capsule.ReservationSHA256 == "" {
		t.Fatalf("capsule incomplete or fail-open: %+v %+v %+v", prepared, reservation, capsule)
	}
	if _, err := os.Stat(filepath.Join(dir, "launch-capsule.json")); err != nil {
		t.Fatalf("launch capsule not sealed: %v", err)
	}
	if _, _, _, _, err := ReserveClaudeLaunchCapsule(filepath.Dir(dir), batch, ledger, capacity, capacity, "2026-09-11T12:00:30Z", "2026-09-11T12:00:32Z"); !errors.Is(err, ErrScreeningAssignmentReserved) {
		t.Fatalf("duplicate launch capsule was not blocked: %v", err)
	}
}

func TestReserveClaudeLaunchCapsuleRequiresFreshAuthoritativeCapacity(t *testing.T) {
	batch := claudeCapsuleTestBatch(t)
	ledger := BatchUsageLedger{SchemaVersion: BatchUsageSchemaVersion, BatchID: batch.BatchID, TokenCeiling: batch.TokenCeiling}
	stale := CapacitySnapshot{Provider: "claude", LimitID: "max", AccountLane: "fixture", WindowID: "fixture-window", UsedBasisPoints: 0, ObservedAt: "2026-09-11T11:00:00Z", Source: "authoritative_account_usage"}
	root := t.TempDir()
	prepared, _, _, _, err := ReserveClaudeLaunchCapsule(root, batch, ledger, stale, stale, "2026-09-11T12:00:30Z", "2026-09-11T12:00:31Z")
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Admission.CanLaunch {
		t.Fatalf("stale capacity passed: %+v", prepared.Admission)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("stale capacity created reservation artifacts: %v", entries)
	}
}
