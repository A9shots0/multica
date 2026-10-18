package agentqualification

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRoleQualificationExecutionV3ReservationStrandedClaimBlocksAll(t *testing.T) {
	root := t.TempDir()
	plan, _ := BuildRoleQualificationExecutionPlanV3()
	first := plan.Cells[0]
	claim, err := PrepareRoleQualificationGrantV3(root, first.CellID, "2026-09-13T12:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if claim.Reserved.Calls != 1 || claim.Reserved.InputTokens != 20000 || claim.Reserved.OutputTokens != 4000 || claim.Reserved.TotalTokens != 24000 || claim.Reserved.OpenRouterCashMicros != 120000 {
		t.Fatalf("reservation=%+v", claim.Reserved)
	}
	replay, err := RoleQualificationExecutionReplayV3(root)
	if err != nil {
		t.Fatal(err)
	}
	if !replay.GlobalHalt || replay.GlobalHaltClass != "evidence_transport_uncertainty" || len(replay.StrandedNoAuthoritativeReceipt) != 1 {
		t.Fatalf("replay=%+v", replay)
	}
	if _, err := PrepareRoleQualificationGrantV3(root, plan.Cells[4].CellID, "2026-09-13T12:00:01Z"); err == nil {
		t.Fatal("unrelated grant allowed around stranded claim")
	}
}

func TestRoleQualificationExecutionV3RawDerivedReplayAndForgeryRejection(t *testing.T) {
	root := t.TempDir()
	plan, _ := BuildRoleQualificationExecutionPlanV3()
	cell := plan.Cells[0]
	if _, err := PrepareRoleQualificationGrantV3(root, cell.CellID, "2026-09-13T12:00:00Z"); err != nil {
		t.Fatal(err)
	}
	r := passingReceiptV3(t, cell)
	if err := SealRoleQualificationRawReceiptV3(root, r); err != nil {
		t.Fatal(err)
	}
	status, err := ReconcileRoleQualificationClaimV3(root, cell.CellID)
	if err != nil || status != "passed" {
		t.Fatalf("status=%s err=%v", status, err)
	}
	replay, err := RoleQualificationExecutionReplayV3(root)
	if err != nil {
		t.Fatal(err)
	}
	if replay.Observations != 1 || replay.Totals.Calls != 1 || replay.GlobalHalt {
		t.Fatalf("replay=%+v", replay)
	}
	path := filepath.Join(root, "observations", cell.CellID+".json")
	raw, _ := os.ReadFile(path)
	var o RoleQualificationObservationV3
	_ = json.Unmarshal(raw, &o)
	o.Grade.Pass = false
	forged, _ := json.Marshal(o)
	if err := os.WriteFile(path, forged, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := RoleQualificationExecutionReplayV3(root); err == nil || !strings.Contains(err.Error(), "derived evidence drift") {
		t.Fatalf("forged observation accepted: %v", err)
	}
}

func TestRoleQualificationExecutionV3SealsOverspendBeforeTypedGlobalHalt(t *testing.T) {
	root := t.TempDir()
	plan, _ := BuildRoleQualificationExecutionPlanV3()
	cell := plan.Cells[0]
	_, _ = PrepareRoleQualificationGrantV3(root, cell.CellID, "2026-09-13T12:00:00Z")
	r := passingReceiptV3(t, cell)
	var p RoleQualificationRawProviderResponseV3
	if err := strictJSON(r.RawResponse, &p); err != nil {
		t.Fatal(err)
	}
	p.Usage.InputTokens = 20001
	p.Usage.TotalTokens = p.Usage.InputTokens + p.Usage.OutputTokens + p.Usage.ReasoningTokens
	r.RawResponse, _ = json.Marshal(p)
	r.RawResponseSHA256 = sha256Hex(r.RawResponse)
	if err := SealRoleQualificationRawReceiptV3(root, r); err != nil {
		t.Fatalf("raw must seal before cap evaluation: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "receipts", cell.CellID+".json")); err != nil {
		t.Fatal("raw receipt not sealed")
	}
	if _, err := ReconcileRoleQualificationClaimV3(root, cell.CellID); err != nil {
		t.Fatal(err)
	}
	replay, err := RoleQualificationExecutionReplayV3(root)
	if err != nil {
		t.Fatal(err)
	}
	if !replay.GlobalHalt || replay.GlobalHaltClass != "observation_token_cap_breach" {
		t.Fatalf("replay=%+v", replay)
	}
}

func TestRoleQualificationExecutionV3RejectsUnclaimedSubstitutedAndRouteDrift(t *testing.T) {
	root := t.TempDir()
	plan, _ := BuildRoleQualificationExecutionPlanV3()
	cell := plan.Cells[0]
	r := passingReceiptV3(t, cell)
	if err := SealRoleQualificationRawReceiptV3(root, r); err == nil {
		t.Fatal("unclaimed receipt accepted")
	}
	_, _ = PrepareRoleQualificationGrantV3(root, cell.CellID, "2026-09-13T12:00:00Z")
	r.FrozenRequestSHA256 = "forged"
	if err := SealRoleQualificationRawReceiptV3(root, r); err == nil {
		t.Fatal("substituted request accepted")
	}
	r = passingReceiptV3(t, cell)
	var p RoleQualificationRawProviderResponseV3
	_ = strictJSON(r.RawResponse, &p)
	p.ActualModel = "fallback"
	r.RawResponse, _ = json.Marshal(p)
	r.RawResponseSHA256 = sha256Hex(r.RawResponse)
	if err := SealRoleQualificationRawReceiptV3(root, r); err == nil {
		t.Fatal("route drift accepted")
	}
}

func TestRoleQualificationExecutionV3ReservationAndOverflowCaps(t *testing.T) {
	plan, _ := BuildRoleQualificationExecutionPlanV3()
	var subscription, free RoleQualificationExecutionCellV3
	for _, c := range plan.Cells {
		if c.ConfigurationClass.RuntimeProvider == "claude_code_subscription" {
			subscription = c
		}
		if strings.HasSuffix(c.ConfigurationClass.Model, ":free") {
			free = c
		}
	}
	if got := reservationForRoleQualificationCellV3(subscription, plan.PerObservation); got.ClaudeSubscriptionCostEquivalentMicros != 120000 || got.OpenRouterCashMicros != 0 {
		t.Fatalf("subscription=%+v", got)
	}
	if got := reservationForRoleQualificationCellV3(free, plan.PerObservation); got.OpenRouterCashMicros != 0 || got.ClaudeSubscriptionCostEquivalentMicros != 0 {
		t.Fatalf("free=%+v", got)
	}
	total := RoleQualificationUsageV3{Calls: 1<<63 - 1}
	if err := addRoleQualificationUsageV3(&total, RoleQualificationUsageV3{Calls: 1}); err == nil {
		t.Fatal("overflow accepted")
	}
}

func passingReceiptV3(t *testing.T, cell RoleQualificationExecutionCellV3) RoleQualificationProviderReceiptV3 {
	t.Helper()
	answer := structuredPassingAnswerV3(t, cell, false)
	structured, _ := json.Marshal(answer)
	usage := RoleQualificationUsageV3{Calls: 1, InputTokens: 100, OutputTokens: 50, ReasoningTokens: 10, TotalTokens: 160, WallMillis: 1000}
	if cell.ConfigurationClass.RuntimeProvider == "claude_code_subscription" {
		usage.ClaudeSubscriptionCostEquivalentMicros = 1000
	} else {
		usage.OpenRouterCashMicros = 1000
	}
	p := RoleQualificationRawProviderResponseV3{"silver-mediums.role-qualification-raw-provider-response.v3", "call-1", cell.ConfigurationClass.Model, cell.ConfigurationClass.ServingProvider, cell.ConfigurationClass.Effort, transportForRoleQualificationCellV3(cell), false, 0, "2026-09-13T12:00:00Z", "2026-09-13T12:00:01Z", structured, "", usage}
	raw, _ := json.Marshal(p)
	return RoleQualificationProviderReceiptV3{"silver-mediums.role-qualification-provider-receipt.v3", cell.CellID, cell.RequestSHA256, raw, sha256Hex(raw), "2026-09-13T12:00:02Z"}
}
