package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/companyadapter"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestCompanyOperationsPilotCorrelatesWatcherAndSupportIntoOneVerifiedIncident(t *testing.T) {
	projectID := dbfx.Project(t, "Phone Farm company operations")
	unique := time.Now().UnixNano()
	watcherSource := fmt.Sprintf("hq-watcher-%d", unique)
	supportSource := fmt.Sprintf("hq-support-%d", unique)
	verificationSource := fmt.Sprintf("hq-verification-%d", unique)
	watcherCredential, _ := configureExternalIncidentSource(t, watcherSource, projectID, true)
	supportCredential, _ := configureExternalIncidentSource(t, supportSource, projectID, true)
	verificationCredential, _ := configureExternalIncidentSource(t, verificationSource, projectID, true)
	cleanupExternalIncidentSource(t, watcherSource)
	cleanupExternalIncidentSource(t, supportSource)
	cleanupExternalIncidentSource(t, verificationSource)
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = testPool.Exec(ctx, `DELETE FROM company_signal_quarantine WHERE workspace_id=$1`, testWorkspaceID)
		_, _ = testPool.Exec(ctx, `DELETE FROM company_signal_event WHERE workspace_id=$1`, testWorkspaceID)
		_, _ = testPool.Exec(ctx, `DELETE FROM company_incident WHERE workspace_id=$1`, testWorkspaceID)
		_, _ = testPool.Exec(ctx, `DELETE FROM company_service WHERE workspace_id=$1 AND service_key='phonefarm.sidecar'`, testWorkspaceID)
	})

	serviceBody := map[string]any{
		"display_name": "Phone Farm Sidecar", "area_key": "phonefarm", "environment": "production",
		"source_system": "phonefarm", "source_native_id": "sidecar", "project_id": projectID,
		"authority": "source", "freshness_state": "current", "privacy_class": "internal",
		"observed_at": time.Now().UTC().Format(time.RFC3339Nano),
	}
	testutil.Call(t, testHandler.PutCompanyService,
		companyServiceRequestForTest(serviceBody, "phonefarm.sidecar")).Want(http.StatusOK)

	correlation := companyadapter.CorrelationKey("phonefarm.sidecar", "production", "sidecar-workflow-unavailable")
	base := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	watcherOpen := companySignalBody("watcher:sidecar:1", "railway-ollama-alerts", "mac-mini-8gb", "watcher-sidecar", "1", "application_log", "application_logs", "worsened", "critical", true, "Sidecar requests are failing", "Inspect the active release", nil, []string{"watcher:incident-sidecar"}, "", correlation, base)
	testutil.Call(t, testHandler.PostCompanySignal,
		companySignalRequestForTest(watcherOpen, watcherSource, watcherCredential)).Want(http.StatusCreated)

	for index, affected := range [][]string{
		{"person:one", "device:one"}, {"person:two", "device:one"}, {"person:three", "device:two"},
	} {
		eventID := fmt.Sprintf("support:report:%d", index+1)
		body := companySignalBody(eventID, "phonefarm-support", "phonefarm-production", fmt.Sprintf("support-%d", index+1), "1", "support_report", "support_impact", "new", "high", false, "Employee workflow report", "Keep the report linked while recovery continues", affected, []string{fmt.Sprintf("support:report-%d", index+1)}, "", correlation, base.Add(time.Duration(index+1)*time.Minute))
		testutil.Call(t, testHandler.PostCompanySignal,
			companySignalRequestForTest(body, supportSource, supportCredential)).Want(http.StatusCreated)
	}
	concurrent := companySignalBody("watcher:concurrent:1", "railway-ollama-alerts", "mac-mini-8gb", "watcher-concurrent", "1", "application_log", "application_logs", "new", "high", false, "Concurrent exact replay", "Keep one durable receipt", nil, []string{"watcher:incident-concurrent"}, "", correlation, base.Add(4*time.Minute))
	statuses := make(chan int, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			response := testutil.Call(t, testHandler.PostCompanySignal,
				companySignalRequestForTest(concurrent, watcherSource, watcherCredential))
			statuses <- response.Code
		}()
	}
	wait.Wait()
	close(statuses)
	for status := range statuses {
		if status != http.StatusCreated && status != http.StatusOK {
			t.Fatalf("concurrent exact replay status=%d, want 201 or 200", status)
		}
	}

	if got := dbfx.Count(t, `SELECT count(*) FROM company_incident WHERE workspace_id=$1 AND correlation_key=$2`, testWorkspaceID, correlation); got != 1 {
		t.Fatalf("company incidents=%d, want 1", got)
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM company_signal_event WHERE workspace_id=$1 AND incident_id IN (SELECT id FROM company_incident WHERE workspace_id=$1 AND correlation_key=$2)`, testWorkspaceID, correlation); got != 5 {
		t.Fatalf("company signals=%d, want 5", got)
	}

	testutil.Call(t, testHandler.PostCompanySignal,
		companySignalRequestForTest(watcherOpen, watcherSource, watcherCredential)).Want(http.StatusOK)
	changed := cloneMap(watcherOpen)
	changed["summary"] = "Changed content under the same event ID"
	testutil.Call(t, testHandler.PostCompanySignal,
		companySignalRequestForTest(changed, watcherSource, watcherCredential)).Want(http.StatusConflict)
	outOfOrder := companySignalBody("watcher:sidecar:2", "railway-ollama-alerts", "mac-mini-8gb", "watcher-sidecar", "2", "application_log", "application_logs", "recovered", "low", false, "Errors stopped", "Verify the employee workflow", nil, []string{"watcher:incident-sidecar"}, "", correlation, base.Add(5*time.Minute))
	testutil.Call(t, testHandler.PostCompanySignal,
		companySignalRequestForTest(outOfOrder, watcherSource, watcherCredential)).Want(http.StatusUnprocessableEntity)

	watcherRecovered := companySignalBody("watcher:sidecar:3", "railway-ollama-alerts", "mac-mini-8gb", "watcher-sidecar", "3", "application_log", "application_logs", "recovered", "low", false, "Errors stopped", "Verify the employee workflow", nil, []string{"watcher:incident-sidecar"}, "watcher:sidecar:1", correlation, base.Add(5*time.Minute))
	testutil.Call(t, testHandler.PostCompanySignal,
		companySignalRequestForTest(watcherRecovered, watcherSource, watcherCredential)).Want(http.StatusCreated)

	verification := companySignalBody("verification:sidecar:1", "multica", "multica-local", "phonefarm-sidecar-verification", "1", "verification", "verification", "recovered", "low", false, "Employee workflow verified", "Record the accepted result", nil, []string{"verification:phonefarm-sidecar-1"}, "", correlation, base.Add(6*time.Minute))
	testutil.Call(t, testHandler.PostCompanySignal,
		companySignalRequestForTest(verification, verificationSource, verificationCredential)).Want(http.StatusCreated)

	var lifecycle, verificationState string
	dbfx.QueryRow(t, `SELECT lifecycle_state, verification_state FROM company_incident WHERE workspace_id=$1 AND correlation_key=$2`, testWorkspaceID, correlation).Scan(&lifecycle, &verificationState)
	if lifecycle != "recovered" || verificationState != "verified" {
		t.Fatalf("incident state=%s/%s, want recovered/verified", lifecycle, verificationState)
	}

	var picture map[string]any
	testutil.Call(t, testHandler.GetCompanyOperatingPicture,
		companyOperatingPictureRequestForTest()).Want(http.StatusOK).JSON(&picture)
	incidents, _ := picture["incidents"].([]any)
	if len(incidents) != 1 {
		t.Fatalf("operating picture incidents=%d, want 1", len(incidents))
	}
}

func companySignalBody(eventID, sourceSystem, sourceInstance, sourceNativeID, revision, eventKind, lane, lifecycle, severity string, actionRequired bool, summary, nextStep string, affected, evidence []string, predecessor, correlation string, observedAt time.Time) map[string]any {
	return map[string]any{
		"contract_version": 1, "event_id": eventID, "source_system": sourceSystem,
		"source_instance": sourceInstance, "source_native_id": sourceNativeID, "source_revision": revision,
		"event_kind": eventKind, "source_lane": lane, "lifecycle_state": lifecycle, "severity": severity,
		"action_required": actionRequired, "summary": summary, "next_step": nextStep,
		"affected_refs": affected, "service_ref": "phonefarm.sidecar", "environment_ref": "production",
		"evidence_refs": evidence, "predecessor_event_id": predecessor, "privacy_class": "confidential",
		"raw_payload_hash": strings.Repeat("a", 64), "correlation_key": correlation,
		"incident_generation": 1, "observed_at": observedAt.Format(time.RFC3339Nano),
	}
}

func companyServiceRequestForTest(body any, serviceKey string) *http.Request {
	req := testutil.JSONRequest(http.MethodPut, "/api/workspaces/"+testWorkspaceID+"/company-services/"+serviceKey, body)
	req = testutil.WithHeaders(req, "X-User-ID", testUserID)
	return testutil.WithURLParams(req, "id", testWorkspaceID, "serviceKey", serviceKey)
}

func companySignalRequestForTest(body any, sourceKey, credential string) *http.Request {
	req := testutil.JSONRequest(http.MethodPost, "/api/workspaces/"+testWorkspaceID+"/company-signal-sources/"+sourceKey+"/events", body)
	req = testutil.WithHeaders(req, "Authorization", "Bearer "+credential)
	return testutil.WithURLParams(req, "id", testWorkspaceID, "sourceKey", sourceKey)
}

func companyOperatingPictureRequestForTest() *http.Request {
	req := testutil.JSONRequest(http.MethodGet, "/api/workspaces/"+testWorkspaceID+"/company-operating-picture", nil)
	req = testutil.WithHeaders(req, "X-User-ID", testUserID)
	return testutil.WithURLParams(req, "id", testWorkspaceID)
}

func cloneMap(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
