package agentqualification

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRoleQualificationExecutionV2PlanIsExactCounterbalancedAndProviderLocked(t *testing.T) {
	plan, err := BuildRoleQualificationExecutionPlanV2()
	if err != nil {
		t.Fatal(err)
	}
	if plan.ProviderCallsAllowed || plan.RuntimeWiring != "none" || len(plan.Cells) != 36 || plan.Campaign.MaxCalls != 36 {
		t.Fatalf("plan boundary drifted: %+v", plan)
	}
	seen := map[string]int{}
	ordinals := map[int]bool{}
	for _, cell := range plan.Cells {
		if cell.MaxCalls != 1 || cell.Retries != 0 || cell.Fallbacks || cell.PacketSHA256 == "" {
			t.Fatalf("cell boundary drifted: %+v", cell)
		}
		ordinals[cell.Ordinal] = true
		seen[cell.ConfigurationClass.ClassID+":"+cell.CandidateClass]++
		raw, _ := json.Marshal(cell.Packet)
		for _, hidden := range []string{"pass_requirements", "subtle_failures", "adversarial_failures", "required_concepts", "forbidden_concepts", "expected_pass"} {
			if strings.Contains(string(raw), hidden) {
				t.Errorf("cell %s leaks %s", cell.CellID, hidden)
			}
		}
	}
	if len(ordinals) != 36 || len(seen) != 12 {
		t.Fatalf("counterbalance coverage ordinals=%d pairs=%d", len(ordinals), len(seen))
	}
	for pair, count := range seen {
		if count != 3 {
			t.Errorf("%s count=%d", pair, count)
		}
	}
	again, err := BuildRoleQualificationExecutionPlanV2()
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(plan.Cells)
	b, _ := json.Marshal(again.Cells)
	if string(a) != string(b) {
		t.Fatal("execution order is not deterministic")
	}
}

func TestRoleQualificationExecutionV2StrictSchemaCanonicalizesBeforeHiddenGrade(t *testing.T) {
	plan, err := BuildRoleQualificationExecutionPlanV2()
	if err != nil {
		t.Fatal(err)
	}
	cell := plan.Cells[0]
	answer := strongFixtureAnswerV2(t, cell.CaseID)
	values := map[string]string{}
	for _, field := range cell.Packet.ResponseSchema.Required {
		values[field] = "Not applicable."
	}
	values[cell.Packet.ResponseSchema.Required[0]] = answer
	raw, _ := json.Marshal(values)
	canonical, grade, err := ValidateAndGradeRoleQualificationExecutionOutputV2(cell.CaseID, raw)
	if err != nil {
		t.Fatal(err)
	}
	if !grade.Pass || !json.Valid(canonical) {
		t.Fatalf("grade=%+v canonical=%s", grade, canonical)
	}
	delete(values, cell.Packet.ResponseSchema.Required[0])
	raw, _ = json.Marshal(values)
	if _, _, err := ValidateAndGradeRoleQualificationExecutionOutputV2(cell.CaseID, raw); err == nil {
		t.Fatal("missing schema field accepted")
	}
	values[cell.Packet.ResponseSchema.Required[0]] = answer
	values["hidden_grader"] = "leak"
	raw, _ = json.Marshal(values)
	if _, _, err := ValidateAndGradeRoleQualificationExecutionOutputV2(cell.CaseID, raw); err == nil {
		t.Fatal("additional schema field accepted")
	}
}

func TestRoleQualificationExecutionV2WriteAheadClaimReceiptReconcileReplayAndNoRetry(t *testing.T) {
	root := t.TempDir()
	plan, err := BuildRoleQualificationExecutionPlanV2()
	if err != nil {
		t.Fatal(err)
	}
	cell := plan.Cells[0]
	if err := WriteRoleQualificationClaimV2(root, cell.CellID, "2026-09-13T12:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "claims", cell.CellID+".json")); err != nil {
		t.Fatal("claim was not durable before receipt")
	}
	status, err := ReconcileRoleQualificationClaimV2(root, cell.CellID)
	if err != nil || status != "stranded_no_receipt_no_retry" {
		t.Fatalf("status=%s err=%v", status, err)
	}
	if err := WriteRoleQualificationClaimV2(root, cell.CellID, "2026-09-13T12:00:01Z"); err == nil {
		t.Fatal("stranded claim was retried")
	}
	receipt := passingReceiptV2(t, cell)
	if err := WriteRoleQualificationReceiptV2(root, receipt); err != nil {
		t.Fatal(err)
	}
	status, err = ReconcileRoleQualificationClaimV2(root, cell.CellID)
	if err != nil || status != "passed" {
		t.Fatalf("status=%s err=%v", status, err)
	}
	replay, err := RoleQualificationExecutionReplayV2(root)
	if err != nil {
		t.Fatal(err)
	}
	if replay.Claims != 1 || replay.Receipts != 1 || replay.Observations != 1 || replay.Totals.Calls != 1 || replay.GlobalHalt {
		t.Fatalf("replay=%+v", replay)
	}
	if err := WriteRoleQualificationReceiptV2(root, receipt); err == nil {
		t.Fatal("immutable receipt overwritten")
	}
	status, err = ReconcileRoleQualificationClaimV2(root, cell.CellID)
	if err != nil || status != "already_closed_no_retry" {
		t.Fatalf("second reconcile status=%s err=%v", status, err)
	}
}

func TestRoleQualificationExecutionV2CrashRecoveryRepairsLedgerWithoutDuplicateCall(t *testing.T) {
	root := t.TempDir()
	plan, _ := BuildRoleQualificationExecutionPlanV2()
	cell := plan.Cells[0]
	if err := WriteRoleQualificationClaimV2(root, cell.CellID, "2026-09-13T12:00:00Z"); err != nil {
		t.Fatal(err)
	}
	receipt := passingReceiptV2(t, cell)
	if err := WriteRoleQualificationReceiptV2(root, receipt); err != nil {
		t.Fatal(err)
	}
	observation, err := deriveRoleQualificationObservationV2(cell, receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeOnceRoleQualificationJSONV2(filepath.Join(root, "observations", cell.CellID+".json"), observation); err != nil {
		t.Fatal(err)
	}
	status, err := ReconcileRoleQualificationClaimV2(root, cell.CellID)
	if err != nil || status != "already_closed_no_retry" {
		t.Fatalf("status=%s err=%v", status, err)
	}
	replay, err := RoleQualificationExecutionReplayV2(root)
	if err != nil {
		t.Fatal(err)
	}
	if replay.Totals.Calls != 1 || replay.Observations != 1 {
		t.Fatalf("replay=%+v", replay)
	}
}

func TestRoleQualificationExecutionV2ReceiptsBindRouteCapsAndSeparateCash(t *testing.T) {
	plan, _ := BuildRoleQualificationExecutionPlanV2()
	var openrouter, subscription RoleQualificationExecutionCellV2
	for _, cell := range plan.Cells {
		if cell.ConfigurationClass.RuntimeProvider == "claude_code_subscription" {
			subscription = cell
		} else if openrouter.CellID == "" {
			openrouter = cell
		}
	}
	r := passingReceiptV2(t, openrouter)
	r.ActualModel = "fallback"
	if err := validateRoleQualificationReceiptV2(openrouter, r, plan.PerObservation); err == nil {
		t.Fatal("actual model drift accepted")
	}
	r = passingReceiptV2(t, openrouter)
	r.Usage.InputTokens = 20001
	r.Usage.TotalTokens = 20003
	if err := validateRoleQualificationReceiptV2(openrouter, r, plan.PerObservation); err == nil {
		t.Fatal("observation cap overshoot accepted")
	}
	r = passingReceiptV2(t, subscription)
	r.Usage.OpenRouterCashMicros = 1
	if err := validateRoleQualificationReceiptV2(subscription, r, plan.PerObservation); err == nil {
		t.Fatal("subscription cost conflated with marginal cash")
	}
	if _, ok := classifyRoleQualificationFailureV2("made_up"); ok {
		t.Fatal("unknown failure classification accepted")
	}
	if scope, _ := classifyRoleQualificationFailureV2("transport_failure"); scope != "global_halt" {
		t.Fatal(scope)
	}
	if scope, _ := classifyRoleQualificationFailureV2("competence_failure"); scope != "candidate_local" {
		t.Fatal(scope)
	}
}

func TestRoleQualificationExecutionV2CampaignCapsAndOverflowFailClosed(t *testing.T) {
	plan, _ := BuildRoleQualificationExecutionPlanV2()
	usage := RoleQualificationUsageV2{Calls: 36, InputTokens: 720000, OutputTokens: 144000, TotalTokens: 864000, OpenRouterCashMicros: 10000000, WallMillis: 5400000}
	if err := validateRoleQualificationCampaignUsageV2(usage, plan.Campaign); err != nil {
		t.Fatal(err)
	}
	usage.Calls++
	if err := validateRoleQualificationCampaignUsageV2(usage, plan.Campaign); err == nil {
		t.Fatal("campaign call overshoot accepted")
	}
	total := RoleQualificationUsageV2{Calls: int64(^uint64(0) >> 1)}
	if err := addRoleQualificationUsageV2(&total, RoleQualificationUsageV2{Calls: 1}); err == nil {
		t.Fatal("accounting overflow accepted")
	}
}

func TestRoleQualificationExecutionV2CandidateFailureIsPairLocalAndGlobalHaltStopsAll(t *testing.T) {
	for _, tc := range []struct {
		failure, wantScope string
		blocksOther        bool
	}{{"competence_failure", "candidate_local", false}, {"transport_failure", "global_halt", true}} {
		root := t.TempDir()
		plan, _ := BuildRoleQualificationExecutionPlanV2()
		first := plan.Cells[0]
		if err := WriteRoleQualificationClaimV2(root, first.CellID, "2026-09-13T12:00:00Z"); err != nil {
			t.Fatal(err)
		}
		receipt := passingReceiptV2(t, first)
		receipt.StructuredOutput = nil
		receipt.FailureClass = tc.failure
		if err := WriteRoleQualificationReceiptV2(root, receipt); err != nil {
			t.Fatal(err)
		}
		if _, err := ReconcileRoleQualificationClaimV2(root, first.CellID); err != nil {
			t.Fatal(err)
		}
		replay, err := RoleQualificationExecutionReplayV2(root)
		if err != nil {
			t.Fatal(err)
		}
		if tc.wantScope == "global_halt" && !replay.GlobalHalt {
			t.Fatal("global halt not preserved")
		}
		samePair := RoleQualificationExecutionCellV2{}
		otherPair := RoleQualificationExecutionCellV2{}
		for _, cell := range plan.Cells {
			if cell.CellID == first.CellID {
				continue
			}
			if cell.ConfigurationClass.ClassID == first.ConfigurationClass.ClassID && cell.CandidateClass == first.CandidateClass && samePair.CellID == "" {
				samePair = cell
			}
			if cell.ConfigurationClass.ClassID != first.ConfigurationClass.ClassID && otherPair.CellID == "" {
				otherPair = cell
			}
		}
		if err := WriteRoleQualificationClaimV2(root, samePair.CellID, "2026-09-13T12:00:02Z"); err == nil {
			t.Fatal("eliminated pair received another grant")
		}
		err = WriteRoleQualificationClaimV2(root, otherPair.CellID, "2026-09-13T12:00:03Z")
		if tc.blocksOther && err == nil {
			t.Fatal("global halt allowed unrelated grant")
		}
		if !tc.blocksOther && err != nil {
			t.Fatalf("candidate-local failure blocked unrelated pair: %v", err)
		}
	}
}

func strongFixtureAnswerV2(t *testing.T, caseID string) string {
	t.Helper()
	var fixtures roleQualificationFixturesFileV1
	if err := decodeEmbeddedRoleQualificationV1("role_qualification_calibration_fixtures_v1.json", &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures.Fixtures {
		if fixture.CaseID == caseID && fixture.Variant == "strong_pass" {
			return fixture.Answer
		}
	}
	t.Fatalf("strong fixture missing for %s", caseID)
	return ""
}
func passingReceiptV2(t *testing.T, cell RoleQualificationExecutionCellV2) RoleQualificationProviderReceiptV2 {
	t.Helper()
	values := map[string]string{}
	for _, field := range cell.Packet.ResponseSchema.Required {
		values[field] = "Not applicable."
	}
	values[cell.Packet.ResponseSchema.Required[0]] = strongFixtureAnswerV2(t, cell.CaseID)
	structured, _ := json.Marshal(values)
	raw := json.RawMessage(`{"provider":"sealed"}`)
	transport := RoleQualificationOpenCodeTransportV2
	usage := RoleQualificationUsageV2{Calls: 1, InputTokens: 10, OutputTokens: 2, ReasoningTokens: 0, TotalTokens: 12, OpenRouterCashMicros: 5, WallMillis: 100}
	if cell.ConfigurationClass.RuntimeProvider == "claude_code_subscription" {
		transport = RoleQualificationClaudeTransportV2
		usage.OpenRouterCashMicros = 0
		usage.ClaudeSubscriptionCostEquivalentMicros = 5
	}
	return RoleQualificationProviderReceiptV2{SchemaVersion: "silver-mediums.role-qualification-provider-receipt.v2", CellID: cell.CellID, ProviderCallID: "call-1", RequestedModel: cell.ConfigurationClass.Model, ActualModel: cell.ConfigurationClass.Model, RequestedProvider: cell.ConfigurationClass.ServingProvider, ActualProvider: cell.ConfigurationClass.ServingProvider, RequestedEffort: cell.ConfigurationClass.Effort, ActualEffort: cell.ConfigurationClass.Effort, RequestedTransport: transport, ActualTransport: transport, ProviderFallbackUsed: false, RetryCount: 0, StartedAt: time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano), CompletedAt: time.Date(2026, 9, 13, 12, 0, 1, 0, time.UTC).Format(time.RFC3339Nano), RawResponse: raw, RawResponseSHA256: sha256Hex(raw), StructuredOutput: structured, Usage: usage}
}
