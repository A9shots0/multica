package agentqualification

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type roleQualificationProviderV5Fake struct {
	t         *testing.T
	calls     int
	freeRoute bool
}

func (f *roleQualificationProviderV5Fake) Call(_ context.Context, cell RoleQualificationExecutionCellV3) RoleQualificationProviderResultV5 {
	f.calls++
	if f.calls == 1 {
		return RoleQualificationProviderResultV5{TransportFailure: &RoleQualificationTransportFailureV5{Stage: RoleQualificationStageResponse, Cause: RoleQualificationCauseTimeout, ProviderCallStarted: true, UsageKnown: false, StartedAt: "2026-09-13T18:00:02Z", CompletedAt: "2026-09-13T18:00:03Z", ErrorDetail: "timeout", ReportedUsage: RoleQualificationUsageV3{WallMillis: 1000}, TimingKnown: true}}
	}
	r := passingReceiptV3(f.t, cell)
	if f.freeRoute {
		var raw RoleQualificationRawProviderResponseV3
		if err := strictJSON(r.RawResponse, &raw); err != nil {
			f.t.Fatal(err)
		}
		raw.Usage.OpenRouterCashMicros = 0
		r.RawResponse, _ = json.Marshal(raw)
	}
	return RoleQualificationProviderResultV5{RawResponse: r.RawResponse}
}

func v5Cell(t *testing.T, ordinal int) RoleQualificationExecutionCellV3 {
	t.Helper()
	plan, err := BuildRoleQualificationExecutionPlanV3()
	if err != nil {
		t.Fatal(err)
	}
	return plan.Cells[ordinal]
}

func seedV5ClaimAttempt(t *testing.T, cell RoleQualificationExecutionCellV3) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "preflight"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "preflight", cell.ConfigurationClass.ClassID+".json"), []byte(`{"sealed":"test"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareRoleQualificationGrantV5(root, cell.CellID, "2026-09-13T18:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareRoleQualificationAttemptV5(root, cell, "2026-09-13T18:00:01Z"); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRoleQualificationV5UnknownTransportFailureUsesFullReservationAndIsLocal(t *testing.T) {
	cell := v5Cell(t, 0)
	root := seedV5ClaimAttempt(t, cell)
	failure := RoleQualificationTransportFailureV5{Stage: RoleQualificationStageResponse, Cause: RoleQualificationCauseTimeout, ProviderCallStarted: true, UsageKnown: false, StartedAt: "2026-09-13T18:00:02Z", CompletedAt: "2026-09-13T18:00:12Z", ErrorDetail: "deadline exceeded", ReportedUsage: RoleQualificationUsageV3{WallMillis: 10000}, TimingKnown: true}
	if err := SealRoleQualificationTransportFailureV5(root, cell, failure, "2026-09-13T18:00:13Z"); err != nil {
		t.Fatal(err)
	}
	if err := ReconcileRoleQualificationClaimV5(root, cell.CellID); err != nil {
		t.Fatal(err)
	}
	replay, err := RoleQualificationExecutionReplayV5(root)
	if err != nil {
		t.Fatal(err)
	}
	if replay.GlobalHalt || replay.TransportReceipts != 1 || replay.Observations != 1 || len(replay.LocalUnscored) != 1 {
		t.Fatalf("unexpected replay %+v", replay)
	}
	want := reservationForRoleQualificationCellV3(cell, RoleQualificationObservationCapsV2{1, 20000, 4000, 24000, 180})
	if replay.Totals != want {
		t.Fatalf("unknown usage must consume full reservation: got %+v want %+v", replay.Totals, want)
	}
	if _, err := PrepareRoleQualificationAttemptV5(root, cell, "2026-09-13T18:00:14Z"); err == nil {
		t.Fatal("second attempt must fail closed")
	}
}

func TestRoleQualificationV5PreProviderFailureHasAccurateZeroCallLane(t *testing.T) {
	cell := v5Cell(t, 1)
	root := seedV5ClaimAttempt(t, cell)
	failure := RoleQualificationTransportFailureV5{Stage: RoleQualificationStagePreProvider, Cause: RoleQualificationCauseConnection, ProviderCallStarted: false, UsageKnown: true, StartedAt: "2026-09-13T18:00:02Z", CompletedAt: "2026-09-13T18:00:05Z", ErrorDetail: "server did not start", ReportedUsage: RoleQualificationUsageV3{WallMillis: 3000}, TimingKnown: true}
	if err := SealRoleQualificationTransportFailureV5(root, cell, failure, "2026-09-13T18:00:06Z"); err != nil {
		t.Fatal(err)
	}
	if err := ReconcileRoleQualificationClaimV5(root, cell.CellID); err != nil {
		t.Fatal(err)
	}
	replay, err := RoleQualificationExecutionReplayV5(root)
	if err != nil {
		t.Fatal(err)
	}
	if replay.Totals.Calls != 0 || replay.Totals.WallMillis != 3000 || replay.GlobalHalt {
		t.Fatalf("bad zero-call accounting %+v", replay)
	}
}

func TestRoleQualificationV5TypedFailureAndReceiptTamperFailClosed(t *testing.T) {
	cell := v5Cell(t, 2)
	root := seedV5ClaimAttempt(t, cell)
	bad := RoleQualificationTransportFailureV5{Stage: "made_up", Cause: RoleQualificationCauseTimeout, ProviderCallStarted: true, UsageKnown: false, StartedAt: "2026-09-13T18:00:02Z", CompletedAt: "2026-09-13T18:00:03Z", ErrorDetail: "global halt campaign breach", ReportedUsage: RoleQualificationUsageV3{WallMillis: 1000}, TimingKnown: true}
	if err := SealRoleQualificationTransportFailureV5(root, cell, bad, "2026-09-13T18:00:04Z"); err == nil {
		t.Fatal("unknown typed stage accepted")
	}
	bad.Stage = RoleQualificationStageResponse
	if err := SealRoleQualificationTransportFailureV5(root, cell, bad, "2026-09-13T18:00:04Z"); err != nil {
		t.Fatal(err)
	}
	if err := ReconcileRoleQualificationClaimV5(root, cell.CellID); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "transport-receipts", cell.CellID+".json")
	raw, _ := os.ReadFile(path)
	raw = []byte(strings.Replace(string(raw), "global halt", "candidate fail", 1))
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := RoleQualificationExecutionReplayV5(root); err == nil {
		t.Fatal("tampered error detail accepted")
	}
}

func TestRoleQualificationV5RequiresTerminalReceiptXOR(t *testing.T) {
	cell := v5Cell(t, 3)
	root := seedV5ClaimAttempt(t, cell)
	if _, err := RoleQualificationExecutionReplayV5(root); err == nil || !strings.Contains(err.Error(), "XOR") {
		t.Fatalf("stranded attempt must fail closed: %v", err)
	}
}

func TestRoleQualificationV5LegacyRecoveryExactAndNoRetry(t *testing.T) {
	legacy, next := t.TempDir(), t.TempDir()
	claim := `{"schema_version":"silver-mediums.role-qualification-claim.v3","cell_id":"rqv3-25-nex-product_ui-rq-pui-03","frozen_request_sha256":"f73eedf1c30aed8a657d76e1fc390753299f911ba0418d11f8e7f7756929754d","claimed_at":"2026-09-13T18:11:02.6227949Z","reserved":{"calls":1,"input_tokens":20000,"output_tokens":4000,"reasoning_tokens":0,"total_tokens":24000,"openrouter_cash_micros":0,"claude_subscription_cost_equivalent_micros":0,"wall_millis":180000},"retries":0,"fallbacks":false}`
	preflight := `{"schema_version":"silver-mediums.role-qualification-preflight.v4","execution_freeze_sha256":"2c747be170367ea9eafc36551ffe344e18d7d5ff0c97b60e098ac6226368b2b0","v3_freeze_sha256":"cd2f01e9ff7beaf2bbb0f4c4bfa06a4c6ebc11060a8a62fb15ef4c616cb7a0f3","configuration_class_id":"carryforward.nexn25pro.free.openrouter.floor.v1","requested_model":"openrouter/nex-agi/nex-n2.5-pro:free","requested_provider":"nex-agi","requested_effort":"screening_default","transport":"opencode_structured_v33_http_loopback","runtime_version":"1.18.18","runtime_executable_path":"C:\\Users\\a9sho\\AppData\\Roaming\\npm\\node_modules\\opencode-ai\\bin\\opencode.exe","runtime_executable_sha256":"b6efa9eb3ee1d5c25438f3cba03c471e5d8661a6121abc47f209db3b3f05f415","resolved_config_sha256":"b28ea6b75195943e178b2ed6999658f87588067111345dd426635eac20d65ca4","exact_route_validated":true,"fallbacks_disabled":true,"credential_state":"authenticated","openrouter_spend_baseline_micros":111373283,"observed_at":"2026-09-13T18:10:45.9635289Z","ready":true}`
	if sha256Hex([]byte(claim)) != LegacyV4StrandedClaimSHA256 || sha256Hex([]byte(preflight)) != LegacyV4StrandedPreflightSHA256 {
		t.Fatal("test antecedent bytes drift")
	}
	if err := os.MkdirAll(filepath.Join(legacy, "claims"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(legacy, "preflight"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "claims", LegacyV4StrandedCellID+".json"), []byte(claim), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "preflight", "carryforward.nexn25pro.free.openrouter.floor.v1.json"), []byte(preflight), 0600); err != nil {
		t.Fatal(err)
	}
	if err := RecoverLegacyRoleQualificationV4StrandedV5(legacy, next, "2026-09-13T18:14:02.6227949Z"); err != nil {
		t.Fatal(err)
	}
	replay, err := RoleQualificationExecutionReplayV5(next)
	if err != nil {
		t.Fatal(err)
	}
	if replay.GlobalHalt || replay.Totals.Calls != 1 || replay.Totals.TotalTokens != 24000 || len(replay.LocalUnscored) != 1 {
		t.Fatalf("legacy recovery replay %+v", replay)
	}
	if err := RecoverLegacyRoleQualificationV4StrandedV5(legacy, next, "2026-09-13T18:15:02.6227949Z"); err != nil {
		t.Fatal("idempotent legacy recovery failed", err)
	}
	var recovery RoleQualificationLegacyRecoveryV5
	if err := readStrictJSONFileV3(filepath.Join(next, "legacy-recovery", LegacyV4StrandedCellID+".json"), &recovery); err != nil {
		t.Fatal(err)
	}
	if recovery.Disposition != "invalid_unscored_no_retry_conservative_full_reservation" {
		t.Fatalf("bad disposition %+v", recovery)
	}
}

func TestRoleQualificationV5GlobalEvidenceFailureSealsHTTPBody(t *testing.T) {
	cell := v5Cell(t, 4)
	root := seedV5ClaimAttempt(t, cell)
	body := []byte(`{"broken":`)
	failure := RoleQualificationTransportFailureV5{Stage: RoleQualificationStageParse, Cause: RoleQualificationCauseMalformed, ProviderCallStarted: true, UsageKnown: false, StartedAt: "2026-09-13T18:00:02Z", CompletedAt: "2026-09-13T18:00:03Z", ErrorDetail: "invalid envelope", ReportedUsage: RoleQualificationUsageV3{WallMillis: 1000}, TimingKnown: true, HTTPStatus: 200, ContentType: "application/json", PartialBody: body}
	if err := SealRoleQualificationTransportFailureV5(root, cell, failure, "2026-09-13T18:00:04Z"); err != nil {
		t.Fatal(err)
	}
	if err := ReconcileRoleQualificationClaimV5(root, cell.CellID); err != nil {
		t.Fatal(err)
	}
	replay, err := RoleQualificationExecutionReplayV5(root)
	if err != nil {
		t.Fatal(err)
	}
	if !replay.GlobalHalt || replay.GlobalHaltClass != "observation_transport_malformed_evidence" || len(replay.LocalUnscored) != 0 {
		t.Fatalf("malformed evidence did not halt globally %+v", replay)
	}
	var receipt RoleQualificationTransportReceiptV5
	if err := readStrictJSONFileV3(filepath.Join(root, "transport-receipts", cell.CellID+".json"), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.HTTPStatus != 200 || receipt.ContentType != "application/json" || receipt.PartialBodySHA256 != sha256Hex(body) {
		t.Fatalf("HTTP evidence missing %+v", receipt)
	}
}

func TestRoleQualificationV5ProviderFreeOpenAttemptReconciliation(t *testing.T) {
	cell := v5Cell(t, 5)
	root := seedV5ClaimAttempt(t, cell)
	if err := ReconcileRoleQualificationOpenAttemptsV5(root, "2026-09-13T18:00:02Z"); err != nil {
		t.Fatal(err)
	}
	if err := ReconcileRoleQualificationOpenAttemptsV5(root, "2026-09-13T18:00:03Z"); err != nil {
		t.Fatal(err)
	}
	replay, err := RoleQualificationExecutionReplayV5(root)
	if err != nil {
		t.Fatal(err)
	}
	if replay.GlobalHalt || replay.TransportReceipts != 1 || replay.Totals.TotalTokens != 24000 || replay.Totals.WallMillis != 180000 {
		t.Fatalf("ambiguous recovery wrong %+v", replay)
	}
	var receipt RoleQualificationTransportReceiptV5
	if err := readStrictJSONFileV3(filepath.Join(root, "transport-receipts", cell.CellID+".json"), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.TimingKnown || receipt.ReportedUsage.WallMillis != 0 {
		t.Fatalf("unknown timing fabricated %+v", receipt)
	}
}

func TestRoleQualificationV5BatchContinuesAfterLocalTransportFailure(t *testing.T) {
	plan, _ := BuildRoleQualificationExecutionPlanV3()
	cell := plan.Cells[0]
	root := t.TempDir()
	now := time.Now().UTC()
	p := RoleQualificationPreflightV5(preflightV4Fixture(cell, now))
	p.SchemaVersion = preflightSchemaV5
	p.ExecutionFreezeSHA256 = RoleQualificationExecutionFreezeV5SHA256()
	if err := WriteRoleQualificationPreflightV5(root, p); err != nil {
		t.Fatal(err)
	}
	provider := &roleQualificationProviderV5Fake{t: t}
	replay, err := ExecuteRoleQualificationBatchV5(context.Background(), root, cell.ConfigurationClass.ClassID, now, provider)
	if err != nil {
		t.Fatal(err)
	}
	if provider.calls != 12 || replay.Observations != 12 || replay.TransportReceipts != 1 || replay.SuccessReceipts != 11 || replay.GlobalHalt || len(replay.LocalUnscored) != 1 {
		t.Fatalf("calls=%d replay=%+v", provider.calls, replay)
	}
}

func TestRoleQualificationV5RecoveredTerminalCellIsSkippedBeforeGrant(t *testing.T) {
	legacy, next := t.TempDir(), t.TempDir()
	claim := `{"schema_version":"silver-mediums.role-qualification-claim.v3","cell_id":"rqv3-25-nex-product_ui-rq-pui-03","frozen_request_sha256":"f73eedf1c30aed8a657d76e1fc390753299f911ba0418d11f8e7f7756929754d","claimed_at":"2026-09-13T18:11:02.6227949Z","reserved":{"calls":1,"input_tokens":20000,"output_tokens":4000,"reasoning_tokens":0,"total_tokens":24000,"openrouter_cash_micros":0,"claude_subscription_cost_equivalent_micros":0,"wall_millis":180000},"retries":0,"fallbacks":false}`
	preflightV4 := `{"schema_version":"silver-mediums.role-qualification-preflight.v4","execution_freeze_sha256":"2c747be170367ea9eafc36551ffe344e18d7d5ff0c97b60e098ac6226368b2b0","v3_freeze_sha256":"cd2f01e9ff7beaf2bbb0f4c4bfa06a4c6ebc11060a8a62fb15ef4c616cb7a0f3","configuration_class_id":"carryforward.nexn25pro.free.openrouter.floor.v1","requested_model":"openrouter/nex-agi/nex-n2.5-pro:free","requested_provider":"nex-agi","requested_effort":"screening_default","transport":"opencode_structured_v33_http_loopback","runtime_version":"1.18.18","runtime_executable_path":"C:\\Users\\a9sho\\AppData\\Roaming\\npm\\node_modules\\opencode-ai\\bin\\opencode.exe","runtime_executable_sha256":"b6efa9eb3ee1d5c25438f3cba03c471e5d8661a6121abc47f209db3b3f05f415","resolved_config_sha256":"b28ea6b75195943e178b2ed6999658f87588067111345dd426635eac20d65ca4","exact_route_validated":true,"fallbacks_disabled":true,"credential_state":"authenticated","openrouter_spend_baseline_micros":111373283,"observed_at":"2026-09-13T18:10:45.9635289Z","ready":true}`
	if err := os.MkdirAll(filepath.Join(legacy, "claims"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(legacy, "preflight"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "claims", LegacyV4StrandedCellID+".json"), []byte(claim), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "preflight", "carryforward.nexn25pro.free.openrouter.floor.v1.json"), []byte(preflightV4), 0600); err != nil {
		t.Fatal(err)
	}
	if err := RecoverLegacyRoleQualificationV4StrandedV5(legacy, next, "2026-09-13T18:14:02.6227949Z"); err != nil {
		t.Fatal(err)
	}
	plan, _ := BuildRoleQualificationExecutionPlanV3()
	recovered, ok := findRoleQualificationCellV3(plan, LegacyV4StrandedCellID)
	if !ok {
		t.Fatal("recovered cell missing")
	}
	now := time.Now().UTC()
	p := RoleQualificationPreflightV5(preflightV4Fixture(recovered, now))
	p.SchemaVersion = preflightSchemaV5
	p.ExecutionFreezeSHA256 = RoleQualificationExecutionFreezeV5SHA256()
	if err := WriteRoleQualificationPreflightV5(next, p); err != nil {
		t.Fatal(err)
	}
	provider := &roleQualificationProviderV5Fake{t: t, freeRoute: true}
	provider.calls = 1 // first fake call is local failure; force remaining calls to successful receipts.
	replay, err := ExecuteRoleQualificationBatchV5(context.Background(), next, recovered.ConfigurationClass.ClassID, now, provider)
	if err != nil {
		t.Fatal(err)
	}
	if provider.calls != 12 {
		t.Fatalf("recovered cell retried: new calls=%d want=11", provider.calls-1)
	}
	if replay.Claims != 12 || replay.Observations != 12 || replay.TransportReceipts != 1 || replay.SuccessReceipts != 11 || replay.GlobalHalt {
		t.Fatalf("bad recovered batch %+v", replay)
	}
}
