package agentqualification

import "testing"

func TestAPIBudgetAdmissionKeepsAPISpendSeparateAndBounded(t *testing.T) {
	budget := APIBudget{Provider: "openai", ProjectLane: "qualification-project", Currency: "USD", WindowID: "batch-v1", MaxSpendMicros: 2_000_000, PerRequestReserveMicros: 250_000, MaxSnapshotAgeSeconds: 120}
	snapshot := APISpendSnapshot{Provider: "openai", ProjectLane: "qualification-project", Currency: "USD", WindowID: "batch-v1", SpentMicros: 1_700_000, ObservedAt: "2026-09-11T12:00:00Z", Source: "authoritative_api_usage"}
	if result := EvaluateAPIBudgetAdmission(budget, snapshot, "2026-09-11T12:01:00Z"); !result.CanLaunch {
		t.Fatalf("bounded API request rejected: %+v", result)
	}
	snapshot.SpentMicros = 1_800_000
	if result := EvaluateAPIBudgetAdmission(budget, snapshot, "2026-09-11T12:01:00Z"); result.CanLaunch || !contains(result.Reasons, "api_spend_ceiling_reached") {
		t.Fatalf("over-budget API request admitted: %+v", result)
	}
}

func TestAPIBudgetAdmissionRejectsSubscriptionOrStaleEvidence(t *testing.T) {
	budget := APIBudget{Provider: "openai", ProjectLane: "qualification-project", Currency: "USD", WindowID: "batch-v1", MaxSpendMicros: 2_000_000, PerRequestReserveMicros: 250_000, MaxSnapshotAgeSeconds: 120}
	snapshot := APISpendSnapshot{Provider: "openai", ProjectLane: "qualification-project", Currency: "USD", WindowID: "batch-v1", SpentMicros: 0, ObservedAt: "2026-09-11T11:00:00Z", Source: "authoritative_account_usage"}
	result := EvaluateAPIBudgetAdmission(budget, snapshot, "2026-09-11T12:01:00Z")
	if result.CanLaunch || !contains(result.Reasons, "api_snapshot_invalid") || !contains(result.Reasons, "api_snapshot_stale") {
		t.Fatalf("wrong-lane evidence admitted: %+v", result)
	}
}
