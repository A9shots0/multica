package agentqualification

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func openCodeCapsuleTestBatch(t *testing.T) ScreeningBatchDefinition {
	t.Helper()
	suite, err := CurrentMinimumScreeningSuite()
	if err != nil {
		t.Fatal(err)
	}
	first := openCodeTestConfiguration()
	second := openCodeTestConfiguration()
	second.ID = "screen.generalist.opencode.other.medium.v1"
	second.Model = "opencode/other-test-model"
	batch := ScreeningBatchDefinition{
		SchemaVersion: ScreeningBatchSchemaVersion, BatchID: "screen-opencode-capsule-fixture-v1", Status: "planned_not_launched",
		SuiteVersion: suite.SchemaVersion, ExecutionMode: "isolated_capability", CandidateTransport: OpenCodeToolFreeTransport,
		ExecutionOrderPolicy: "seeded_permutation_v1", ExecutionOrderSeed: "opencode-capsule-fixture-v1",
		GraderBlindToCandidate: true, GraderConfigurationID: "deterministic-minimum-suite.v1", Cases: []ScreeningCase{suite.Cases[0]},
		Candidates: []Configuration{first, second}, Repeats: 1, TokenCeiling: 20000, PerLaunchTokenReservation: 10000,
		APIBudgets:            []APIBudget{{Provider: "opencode_zen", ProjectLane: "fixture", Currency: "USD", WindowID: "fixture-month", MaxSpendMicros: 1000000, PerRequestReserveMicros: 100000, MaxSnapshotAgeSeconds: 120}},
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

func resolvedOpenCodeConfigFor(plan OpenCodeScreeningPlan) []byte {
	steps := "1"
	if plan.Transport == OpenCodeToolFreeTransportV2 || plan.Transport == OpenCodeToolFreeTransportV3 || plan.Transport == OpenCodeToolFreeTransportV4 || plan.Transport == OpenCodeToolFreeTransportV5 || plan.Transport == OpenCodeToolFreeTransportV6 {
		steps = "2"
	}
	provider := ""
	if plan.Transport == OpenCodeToolFreeTransportV6 {
		modelID := strings.TrimPrefix(plan.Model, "openrouter/")
		provider = `,"provider":{"openrouter":{"whitelist":["` + modelID + `"],"options":{"name":"OpenRouter"},"models":{"` + modelID + `":{"options":{"provider":{"allow_fallbacks":false,"order":["` + plan.ServingProvider + `"]}}}}}}`
	}
	return []byte(`{"autoupdate":false,"share":"disabled","snapshot":false,"plugin":[],"instructions":[],"permission":{"*":"deny"},"tools":{"*":false},"compaction":{"auto":false,"prune":false},"default_agent":"qualification","mcp":{},"agent":{"qualification":{"model":"` + plan.Model + `","variant":"` + plan.Effort + `","steps":` + steps + `,"permission":{"*":"deny"},"tools":{"*":false}}}` + provider + `}`)
}

func TestReserveOpenCodeLaunchCapsuleSealsNoInferenceArtifact(t *testing.T) {
	batch := openCodeCapsuleTestBatch(t)
	assignments, err := ScreeningAssignments(batch)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildOpenCodeScreeningPlan(assignments[0].Configuration, batch.Cases[0])
	if err != nil {
		t.Fatal(err)
	}
	ledger := BatchUsageLedger{SchemaVersion: BatchUsageSchemaVersion, BatchID: batch.BatchID, TokenCeiling: batch.TokenCeiling}
	spend := APISpendSnapshot{Provider: "opencode_zen", ProjectLane: "fixture", Currency: "USD", WindowID: "fixture-month", SpentMicros: 0, ObservedAt: "2026-09-11T12:00:00Z", Source: "authoritative_api_usage"}
	prepared, reservation, capsule, dir, err := ReserveOpenCodeLaunchCapsule(t.TempDir(), batch, ledger, spend, "2026-09-11T12:00:30Z", "2026-09-11T12:00:31Z", resolvedOpenCodeConfigFor(plan))
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Assignment == nil || reservation.State != "reserved" || capsule.State != "ready_no_inference" || capsule.ProviderCallAllowed || capsule.ReservationSHA256 == "" || capsule.ResolvedConfigSHA256 == "" {
		t.Fatalf("capsule incomplete or fail-open: %+v %+v %+v", prepared, reservation, capsule)
	}
	if _, err := os.Stat(filepath.Join(dir, "launch-capsule.json")); err != nil {
		t.Fatalf("launch capsule not sealed: %v", err)
	}
	if _, _, _, _, err := ReserveOpenCodeLaunchCapsule(filepath.Dir(dir), batch, ledger, spend, "2026-09-11T12:00:30Z", "2026-09-11T12:00:32Z", resolvedOpenCodeConfigFor(plan)); !errors.Is(err, ErrScreeningAssignmentReserved) {
		t.Fatalf("duplicate launch capsule was not blocked: %v", err)
	}
}

func TestReserveOpenCodeLaunchCapsuleFailsBeforeReservationOnUnsafeConfig(t *testing.T) {
	batch := openCodeCapsuleTestBatch(t)
	ledger := BatchUsageLedger{SchemaVersion: BatchUsageSchemaVersion, BatchID: batch.BatchID, TokenCeiling: batch.TokenCeiling}
	spend := APISpendSnapshot{Provider: "opencode_zen", ProjectLane: "fixture", Currency: "USD", WindowID: "fixture-month", SpentMicros: 0, ObservedAt: "2026-09-11T12:00:00Z", Source: "authoritative_api_usage"}
	root := t.TempDir()
	unsafe := []byte(`{"autoupdate":false,"share":"disabled","snapshot":false,"plugin":[],"instructions":[],"permission":{"*":"deny"},"tools":{"*":false},"compaction":{"auto":false,"prune":false},"default_agent":"qualification","mcp":{"inherited":{"type":"remote","url":"https://example.invalid"}},"agent":{"qualification":{"model":"opencode/test-model","variant":"medium","steps":1,"permission":{"*":"deny"},"tools":{"*":false}}}}`)
	if _, _, _, _, err := ReserveOpenCodeLaunchCapsule(root, batch, ledger, spend, "2026-09-11T12:00:30Z", "2026-09-11T12:00:31Z", unsafe); err == nil {
		t.Fatal("unsafe resolved config reached reservation")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("unsafe preflight created reservation artifacts: %v", entries)
	}
}

func TestReserveOpenCodeLaunchCapsuleRequiresFreshAuthoritativeAPISpend(t *testing.T) {
	batch := openCodeCapsuleTestBatch(t)
	assignments, err := ScreeningAssignments(batch)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildOpenCodeScreeningPlan(assignments[0].Configuration, batch.Cases[0])
	if err != nil {
		t.Fatal(err)
	}
	ledger := BatchUsageLedger{SchemaVersion: BatchUsageSchemaVersion, BatchID: batch.BatchID, TokenCeiling: batch.TokenCeiling}
	stale := APISpendSnapshot{Provider: "opencode_zen", ProjectLane: "fixture", Currency: "USD", WindowID: "fixture-month", SpentMicros: 0, ObservedAt: "2026-09-11T11:00:00Z", Source: "authoritative_api_usage"}
	root := t.TempDir()
	prepared, _, _, _, err := ReserveOpenCodeLaunchCapsule(root, batch, ledger, stale, "2026-09-11T12:00:30Z", "2026-09-11T12:00:31Z", resolvedOpenCodeConfigFor(plan))
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Admission.CanLaunch || !containsText(strings.Join(prepared.Admission.Reasons, " "), "api_budget:api_snapshot_stale") {
		t.Fatalf("stale API spend did not fail closed: %+v", prepared.Admission)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("stale spend created reservation artifacts: %v", entries)
	}
}
