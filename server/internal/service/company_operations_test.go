package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/systemgraph"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestCompanyIncidentAggregateKeepsSupportAsEvidenceInsteadOfAnotherIncident(t *testing.T) {
	base := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	incident := db.CompanyIncident{
		Title: "Phone Farm workflow unavailable", LifecycleState: "open", Severity: "low",
		NextStep: "Investigate", LastObservedAt: timestamp(base),
	}
	latest := []db.CompanySignalEvent{
		companySignal("availability", "phonefarm-uptime", "new", "critical", true, "Public Sidecar is unavailable", nil, base),
		companySignal("application_logs", "railway-ollama-alerts", "worsened", "critical", true, "Sidecar errors are recurring", nil, base.Add(time.Minute)),
		companySignal("support_impact", "phonefarm-support", "new", "high", false, "Three employees are affected", []string{"person:one", "device:two"}, base.Add(2*time.Minute)),
	}
	aggregate := aggregateCompanyIncident(incident, latest)
	if aggregate.LifecycleState != "open" || aggregate.Severity != "critical" || !aggregate.ActionRequired {
		t.Fatalf("unexpected aggregate: %#v", aggregate)
	}
	projection := projectCompanyIncident(incident, latest)
	if len(projection.EvidenceLanes) != 3 {
		t.Fatalf("evidence lanes=%d, want 3", len(projection.EvidenceLanes))
	}
	if len(projection.AffectedRefs) != 2 {
		t.Fatalf("affected refs=%v", projection.AffectedRefs)
	}
}

func TestVerifiedRecoveryClosesIncidentWithoutWaitingForSupportLane(t *testing.T) {
	base := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	incident := db.CompanyIncident{
		Title: "Phone Farm workflow unavailable", LifecycleState: "open", Severity: "critical",
		ActionRequired: true, NextStep: "Restore service", LastObservedAt: timestamp(base),
	}
	latest := []db.CompanySignalEvent{
		companySignal("availability", "phonefarm-uptime", "recovered", "low", false, "Public Sidecar is healthy", nil, base.Add(time.Minute)),
		companySignal("application_logs", "railway-ollama-alerts", "recovered", "low", false, "Error stream recovered", nil, base.Add(2*time.Minute)),
		companySignal("support_impact", "phonefarm-support", "new", "high", false, "Existing reports remain evidence", []string{"person:one"}, base.Add(3*time.Minute)),
		companySignal("verification", "multica", "recovered", "low", false, "Employee workflow verified", nil, base.Add(4*time.Minute)),
	}
	aggregate := aggregateCompanyIncident(incident, latest)
	if aggregate.LifecycleState != "recovered" || aggregate.VerificationState != "verified" || aggregate.ActionRequired {
		t.Fatalf("unexpected recovery aggregate: %#v", aggregate)
	}
	if !aggregate.ResolvedAt.Valid || !aggregate.ResolvedAt.Time.Equal(base.Add(4*time.Minute)) {
		t.Fatalf("resolved_at=%v", aggregate.ResolvedAt)
	}
}

func TestRecoveryNeedsIndependentVerificationAndSupportAloneCannotClose(t *testing.T) {
	base := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	incident := db.CompanyIncident{
		Title: "Phone Farm workflow unavailable", LifecycleState: "open", Severity: "critical",
		ActionRequired: true, NextStep: "Restore service", LastObservedAt: timestamp(base),
	}
	supportOnly := []db.CompanySignalEvent{
		companySignal("support_impact", "phonefarm-support", "recovered", "low", false, "Reports stopped", nil, base.Add(time.Minute)),
	}
	if aggregate := aggregateCompanyIncident(incident, supportOnly); aggregate.LifecycleState != "open" || aggregate.VerificationState == "verified" {
		t.Fatalf("support evidence closed the incident: %#v", aggregate)
	}
	controlRecovered := []db.CompanySignalEvent{
		companySignal("availability", "phonefarm-uptime", "recovered", "low", false, "Service responds", nil, base.Add(time.Minute)),
		companySignal("application_logs", "railway-ollama-alerts", "recovered", "low", false, "Errors stopped", nil, base.Add(2*time.Minute)),
	}
	if aggregate := aggregateCompanyIncident(incident, controlRecovered); aggregate.LifecycleState != "recovering" || aggregate.VerificationState != "observed" {
		t.Fatalf("unverified recovery was accepted: %#v", aggregate)
	}
}

func TestCompanyOperatingPictureUsesStableJSONServiceProjection(t *testing.T) {
	projectID := pgtype.UUID{Bytes: [16]byte{1, 2, 3}, Valid: true}
	expires := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	projection := ProjectCompanyService(db.CompanyService{
		ServiceKey: "phonefarm.sidecar", DisplayName: "Phone Farm Sidecar", AreaKey: "phonefarm",
		Environment: "production", SourceSystem: "phonefarm", SourceNativeID: "sidecar",
		ProjectID: projectID, Authority: "source", FreshnessState: "current", PrivacyClass: "internal",
		ObservedAt: timestamp(time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)),
		ExpiresAt:  timestamp(expires), Metadata: []byte(`{"region":"local"}`), Revision: 2,
		SystemNodeIds: []string{"phonefarm_app"}, JourneyIds: []string{"j_phonefarm_operation"},
	})
	encoded, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["service_key"] != "phonefarm.sidecar" || payload["observed_at"] == nil || payload["project_id"] == nil {
		t.Fatalf("unstable service projection: %s", encoded)
	}
	if _, ok := payload["workspace_id"]; ok {
		t.Fatalf("workspace implementation detail leaked: %s", encoded)
	}
	if fmt.Sprint(payload["system_node_ids"]) != "[phonefarm_app]" || fmt.Sprint(payload["journey_ids"]) != "[j_phonefarm_operation]" {
		t.Fatalf("topology mapping missing: %s", encoded)
	}
}

func TestCompanyServiceTopologyMappingIsExplicitNormalizedAndFailClosed(t *testing.T) {
	input := CompanyServiceInput{
		SystemNodeIDs: []string{" phonefarm_app ", "phonefarm_app"},
		JourneyIDs:    []string{"j_phonefarm_operation", "j_phonefarm_operation"},
	}
	if err := validateCompanyServiceTopology(&input); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(input.SystemNodeIDs) != "[phonefarm_app]" || fmt.Sprint(input.JourneyIDs) != "[j_phonefarm_operation]" {
		t.Fatalf("mapping was not normalized: nodes=%v journeys=%v", input.SystemNodeIDs, input.JourneyIDs)
	}
	invalid := CompanyServiceInput{SystemNodeIDs: []string{"phonefarm-by-title"}}
	if err := validateCompanyServiceTopology(&invalid); !errors.Is(err, ErrCompanyServiceTopology) {
		t.Fatalf("invalid mapping error=%v, want ErrCompanyServiceTopology", err)
	}
}

func TestCompanyCurrentEffortSeparatesLiveWorkFromProviderReports(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	taskID := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	agentID := pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	issueID := pgtype.UUID{Bytes: [16]byte{3}, Valid: true}
	projectID := pgtype.UUID{Bytes: [16]byte{4}, Valid: true}
	receiptID := pgtype.UUID{Bytes: [16]byte{5}, Valid: true}
	items, err := projectCompanyCurrentEffort(
		[]db.ListCompanyActiveAgentTasksRow{{
			ID: taskID, Status: "running", IssueID: issueID,
			IssueTitle: pgtype.Text{String: "Implement the HQ foundation", Valid: true}, ProjectID: projectID,
			AgentID: agentID, AgentName: "Mika worker", WorkSource: "issue",
			CreatedAt: timestamp(now.Add(-time.Hour)), StartedAt: timestamp(now.Add(-10 * time.Minute)),
		}},
		[]db.ListCompanyLatestExternalWorkReceiptsRow{{
			ID: receiptID, IssueID: issueID, IssueTitle: "Verify the HQ foundation", ProjectID: projectID,
			OutcomeState: pgtype.Text{String: "open", Valid: true}, Provider: "codex", WorkID: "outside-hq",
			RunID: "run-1", RawReceipt: []byte(`{
				"work_id":"outside-hq","run_id":"run-1","receipt_revision":1,"provider":"codex",
				"objective":"Verify HQ","scope":"bounded","requested_mode":"review",
				"authority":{"reference":"observation","allowed_effects":[]},"state":"complete",
				"current_milestone":"review complete","change_since_last_receipt":"verified",
				"evidence":[{"type":"test","reference":"test:1"}],"dependencies":[],"next_action":"owner acceptance",
				"owner_decision":{"required":false,"category":null,"question":null,"options":[],"why_now":null,"invalidated_by":[]},
				"risk":"low","resource_usage":{"source":"unavailable","attempts":null,"wall_clock_seconds":null,"active_compute_seconds":null,"input_tokens":null,"output_tokens":null,"cost_usd":null,"capacity_window_ref":null},
				"effect":{"user_outcome":"unchanged","production_changed":false},
				"freshness":{"observed_at":"2026-10-18T11:55:00Z","expires_at":null},
				"acceptance":{"stage":"local","status":"accepted","criteria_remaining":[]}
			}`),
		}},
		nil,
		now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("current effort=%d, want active Multica task plus unaccepted outside report", len(items))
	}
	if items[0].State != "reported_done_unaccepted" || !items[0].ProviderReportOnly || items[0].NativeOutcomeState == nil || *items[0].NativeOutcomeState != "open" {
		t.Fatalf("outside work lost its trust boundary: %#v", items[0])
	}
	if items[1].State != "active" || items[1].ProviderReportOnly || items[1].ActorName == nil || *items[1].ActorName != "Mika worker" {
		t.Fatalf("Multica task was not projected as live work: %#v", items[1])
	}
}

func TestCompanyWorkTopologyIsExplicitVersionedAndFailClosed(t *testing.T) {
	graph, err := systemgraph.Default()
	if err != nil {
		t.Fatal(err)
	}
	input := CompanyWorkTopologyInput{
		GraphVersion: graph.Version, SystemNodeIDs: []string{" phonefarm_app ", "phonefarm_app"},
		JourneyIDs: []string{"j_phonefarm_operation"}, Authority: "cited",
		EvidenceRefs: []string{" receipt:phonefarm-1 ", "receipt:phonefarm-1"}, ObservedAt: time.Now().UTC(),
	}
	if err := validateCompanyWorkTopology(&input); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(input.SystemNodeIDs) != "[phonefarm_app]" || fmt.Sprint(input.EvidenceRefs) != "[receipt:phonefarm-1]" {
		t.Fatalf("work mapping was not normalized: %#v", input)
	}
	invalid := input
	invalid.GraphVersion = "stale-graph"
	if err := validateCompanyWorkTopology(&invalid); !errors.Is(err, ErrCompanyWorkTopology) {
		t.Fatalf("stale graph mapping error=%v, want ErrCompanyWorkTopology", err)
	}
}

func TestProviderFreePhoneFarmPilotProducesOneIncidentWithSeparateEvidenceLanes(t *testing.T) {
	base := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	incident := db.CompanyIncident{
		ID:             pgtype.UUID{Bytes: [16]byte{9, 8, 7}, Valid: true},
		CorrelationKey: strings.Repeat("a", 64), Generation: 1,
		ServiceKey: "phonefarm.sidecar", Environment: "production",
		Title: "Phone Farm workflow unavailable", LifecycleState: "open", Severity: "critical",
		ActionRequired: true, NextStep: "Restore service", VerificationState: "unverified",
		OpenedAt: timestamp(base), LastObservedAt: timestamp(base), Revision: 1,
	}
	signals := []db.CompanySignalEvent{
		companySignal("availability", "phonefarm-uptime", "recovered", "low", false, "Public Sidecar is healthy", nil, base.Add(5*time.Minute)),
		companySignal("application_logs", "railway-ollama-alerts", "recovered", "low", false, "Error stream recovered", nil, base.Add(6*time.Minute)),
		companySignal("deployment", "phonefarm-release", "recovered", "low", false, "Replacement release is active", nil, base.Add(7*time.Minute)),
		companySignal("verification", "multica", "recovered", "low", false, "Employee workflow verified", nil, base.Add(8*time.Minute)),
	}
	for index, affected := range [][]string{
		{"person:one", "device:one"}, {"person:two", "device:one"}, {"person:three", "device:two"},
	} {
		signal := companySignal("support_impact", "phonefarm-support", "new", "high", false, "Employee report", affected, base.Add(time.Duration(index+1)*time.Minute))
		signal.SourceNativeID = fmt.Sprintf("support-report-%d", index+1)
		signals = append(signals, signal)
	}

	aggregate := aggregateCompanyIncident(incident, signals)
	if aggregate.LifecycleState != "recovered" || aggregate.VerificationState != "verified" {
		t.Fatalf("pilot did not reach verified recovery: %#v", aggregate)
	}
	projection := projectCompanyIncident(incident, signals, db.CompanyService{
		ServiceKey:    incident.ServiceKey,
		SystemNodeIds: []string{"phonefarm_app"},
		JourneyIds:    []string{"j_phonefarm_operation"},
	})
	if projection.ID != util.UUIDToString(incident.ID) {
		t.Fatalf("pilot split the incident identity: %#v", projection)
	}
	if len(projection.EvidenceLanes) != 5 {
		t.Fatalf("evidence lanes=%d, want availability, logs, support, deployment, verification", len(projection.EvidenceLanes))
	}
	if len(projection.AffectedRefs) != 5 {
		t.Fatalf("affected refs=%v, want three people and two devices", projection.AffectedRefs)
	}
	if fmt.Sprint(projection.SystemNodeIDs) != "[phonefarm_app]" || fmt.Sprint(projection.JourneyIDs) != "[j_phonefarm_operation]" {
		t.Fatalf("incident lost service topology: nodes=%v journeys=%v", projection.SystemNodeIDs, projection.JourneyIDs)
	}
}

func companySignal(lane, source, lifecycle, severity string, action bool, summary string, affected []string, observed time.Time) db.CompanySignalEvent {
	affectedJSON, _ := json.Marshal(affected)
	evidenceJSON, _ := json.Marshal([]string{source + ":evidence"})
	return db.CompanySignalEvent{
		SourceLane: lane, SourceSystem: source, SourceInstance: source + "-instance", SourceNativeID: source + "-1",
		LifecycleState: lifecycle, Severity: severity, ActionRequired: action, Summary: summary,
		NextStep: "Continue deterministic handling", AffectedRefs: affectedJSON, EvidenceRefs: evidenceJSON,
		ObservedAt: timestamp(observed),
	}
}

func timestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}
