package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestExternalIncidentInputValidationAndMachineAuth(t *testing.T) {
	valid := func() map[string]any {
		return externalIncidentEvent("incident:multica:"+externalIncidentDigest("event-1"), "railway-crm-health", 1, "new", "high", time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC))
	}

	t.Run("machine credential is required before the body is processed", func(t *testing.T) {
		req := externalIncidentOccurrenceRequest(http.MethodPost, "not-json", "intelligence", "mul_human_pat_is_not_machine_auth")
		testutil.Call(t, testHandler.PostExternalIncidentOccurrence, req).Want(http.StatusUnauthorized)
	})

	projectID := dbfx.Project(t, "Validation incidents")
	sourceKey := fmt.Sprintf("intelligence-validation-%d", time.Now().UnixNano())
	credential, _ := configureExternalIncidentSource(t, sourceKey, projectID, true)
	cleanupExternalIncidentSource(t, sourceKey)

	t.Run("unknown fields are rejected", func(t *testing.T) {
		body := valid()
		body["unexpected"] = true
		testutil.Call(t, testHandler.PostExternalIncidentOccurrence,
			externalIncidentOccurrenceRequest(http.MethodPost, body, sourceKey, credential)).Want(http.StatusBadRequest)
	})

	t.Run("required boolean is enforced", func(t *testing.T) {
		body := valid()
		delete(body, "action_required")
		testutil.Call(t, testHandler.PostExternalIncidentOccurrence,
			externalIncidentOccurrenceRequest(http.MethodPost, body, sourceKey, credential)).Want(http.StatusBadRequest)
	})
}

func TestExternalIncidentLifecycleReplayConflictAndNoNotificationSideEffects(t *testing.T) {
	projectID := dbfx.Project(t, "Intelligence incidents")
	sourceKey := fmt.Sprintf("intelligence-%d", time.Now().UnixNano())
	credential, creatorAgentID := configureExternalIncidentSource(t, sourceKey, projectID, true)
	cleanupExternalIncidentSource(t, sourceKey)

	var notificationsBefore, actionReceiptsBefore int
	dbfx.QueryRow(t, `SELECT count(*) FROM owner_notification WHERE workspace_id=$1`, testWorkspaceID).Scan(&notificationsBefore)
	dbfx.QueryRow(t, `SELECT count(*) FROM issue_workflow_action_receipt WHERE workspace_id=$1`, testWorkspaceID).Scan(&actionReceiptsBefore)

	base := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	states := []struct {
		state    string
		severity string
	}{
		{"new", "medium"}, {"worsened", "high"}, {"needs_human", "high"},
		{"recovering", "medium"}, {"recovered", "low"},
	}
	var issueID string
	previousEventKey := ""
	for i, item := range states {
		eventKey := fmt.Sprintf("incident:multica:%s", externalIncidentDigest(fmt.Sprintf("evt-%d", i+1)))
		body := externalIncidentSuccessorEvent(eventKey, "railway-crm-health", 1, int64(i+1), previousEventKey, item.state, item.severity, base.Add(time.Duration(i)*time.Minute))
		var got map[string]any
		testutil.Call(t, testHandler.PostExternalIncidentOccurrence,
			externalIncidentOccurrenceRequest(http.MethodPost, body, sourceKey, credential)).Want(http.StatusCreated).JSON(&got)
		if i == 0 {
			issueID, _ = got["issue_id"].(string)
		} else if got["issue_id"] != issueID {
			t.Fatalf("state %s issue_id = %v, want %s", item.state, got["issue_id"], issueID)
		}
		previousEventKey = eventKey
	}

	replayBody := externalIncidentSuccessorEvent("incident:multica:"+externalIncidentDigest("evt-5"), "railway-crm-health", 1, 5, "incident:multica:"+externalIncidentDigest("evt-4"), "recovered", "low", base.Add(4*time.Minute))
	var replay map[string]any
	testutil.Call(t, testHandler.PostExternalIncidentOccurrence,
		externalIncidentOccurrenceRequest(http.MethodPost, replayBody, sourceKey, credential)).Want(http.StatusOK).JSON(&replay)
	if replay["replayed"] != true || replay["issue_id"] != issueID {
		t.Fatalf("replay = %#v, want same issue and replayed=true", replay)
	}

	changed := externalIncidentSuccessorEvent("incident:multica:"+externalIncidentDigest("evt-5"), "railway-crm-health", 1, 5, "incident:multica:"+externalIncidentDigest("evt-4"), "recovered", "low", base.Add(4*time.Minute))
	changed["summary"] = "same delivery key, changed content"
	testutil.Call(t, testHandler.PostExternalIncidentOccurrence,
		externalIncidentOccurrenceRequest(http.MethodPost, changed, sourceKey, credential)).Want(http.StatusConflict)

	retiredFinding := "railway-retired-service"
	retiredOpenKey := "incident:multica:" + externalIncidentDigest("retired-new")
	testutil.Call(t, testHandler.PostExternalIncidentOccurrence, externalIncidentOccurrenceRequest(http.MethodPost,
		externalIncidentSuccessorEvent(retiredOpenKey, retiredFinding, 1, 1, "", "new", "low", base), sourceKey, credential)).Want(http.StatusCreated)
	var retired map[string]any
	testutil.Call(t, testHandler.PostExternalIncidentOccurrence, externalIncidentOccurrenceRequest(http.MethodPost,
		externalIncidentSuccessorEvent("incident:multica:"+externalIncidentDigest("retired-final"), retiredFinding, 1, 2, retiredOpenKey, "retired", "low", base.Add(time.Minute)), sourceKey, credential)).Want(http.StatusCreated).JSON(&retired)

	if got := dbfx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1 AND origin_type='external_incident'`, testWorkspaceID); got != 2 {
		t.Fatalf("external incident issues = %d, want one per episode (2)", got)
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM external_incident_quarantine WHERE workspace_id=$1 AND source_key=$2`, testWorkspaceID, sourceKey); got != 1 {
		t.Fatalf("quarantine rows = %d, want 1", got)
	}
	var status, creatorType, creatorID string
	dbfx.QueryRow(t, `SELECT status, creator_type, creator_id::text FROM issue WHERE id=$1`, issueID).Scan(&status, &creatorType, &creatorID)
	if status != "done" || retired["transition"] != "retired" {
		t.Fatalf("terminal states = (%s, %v), want done/retired", status, retired["transition"])
	}
	if creatorType != "agent" || creatorID != creatorAgentID {
		t.Fatalf("creator = %s/%s, want machine agent %s", creatorType, creatorID, creatorAgentID)
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM owner_notification WHERE workspace_id=$1`, testWorkspaceID); got != notificationsBefore {
		t.Fatalf("owner notifications changed from %d to %d", notificationsBefore, got)
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM issue_workflow_action_receipt WHERE workspace_id=$1`, testWorkspaceID); got != actionReceiptsBefore {
		t.Fatalf("workflow action receipts changed from %d to %d", actionReceiptsBefore, got)
	}
}

func TestExternalIncidentConcurrentExactReplayCreatesOneIssue(t *testing.T) {
	projectID := dbfx.Project(t, "Concurrent incident")
	sourceKey := fmt.Sprintf("intelligence-race-%d", time.Now().UnixNano())
	credential, _ := configureExternalIncidentSource(t, sourceKey, projectID, true)
	cleanupExternalIncidentSource(t, sourceKey)

	body := externalIncidentEvent("incident:multica:"+externalIncidentDigest("race-event"), "railway-race", 1, "new", "high", time.Now().UTC().Truncate(time.Second))
	body["summary"] = "Concurrent watcher finding"
	statuses := make(chan int, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp := testutil.Call(t, testHandler.PostExternalIncidentOccurrence, externalIncidentOccurrenceRequest(http.MethodPost, body, sourceKey, credential))
			statuses <- resp.Code
		}()
	}
	wg.Wait()
	close(statuses)
	for status := range statuses {
		if status != http.StatusCreated && status != http.StatusOK && status != http.StatusConflict {
			t.Fatalf("concurrent response status = %d", status)
		}
	}
	testutil.Call(t, testHandler.PostExternalIncidentOccurrence,
		externalIncidentOccurrenceRequest(http.MethodPost, body, sourceKey, credential)).Want(http.StatusOK)
	if got := dbfx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1 AND origin_type='external_incident' AND title='Concurrent watcher finding'`, testWorkspaceID); got != 1 {
		t.Fatalf("concurrent issues = %d, want 1", got)
	}
}

func TestExternalIncidentCredentialRotationAndRejectedReceiptBoundaries(t *testing.T) {
	projectID := dbfx.Project(t, "Binding boundary")
	sourceKey := fmt.Sprintf("intelligence-boundary-%d", time.Now().UnixNano())
	credential, creatorAgentID := configureExternalIncidentSource(t, sourceKey, projectID, true)
	cleanupExternalIncidentSource(t, sourceKey)

	wrong := externalIncidentEvent("incident:multica:"+externalIncidentDigest("wrong-credential"), "railway-auth", 1, "new", "high", time.Now().UTC())
	testutil.Call(t, testHandler.PostExternalIncidentOccurrence,
		externalIncidentOccurrenceRequest(http.MethodPost, wrong, sourceKey, "mei_"+externalIncidentDigest("wrong"))).Want(http.StatusUnauthorized)
	if got := dbfx.Count(t, `SELECT count(*) FROM external_incident_occurrence WHERE workspace_id=$1 AND source_key=$2`, testWorkspaceID, sourceKey); got != 0 {
		t.Fatalf("wrong-credential receipts = %d, want 0", got)
	}

	var rotated externalIncidentSourceBindingResponse
	var bindingRevision int64
	dbfx.QueryRow(t, `SELECT revision FROM external_incident_source_binding WHERE workspace_id=$1 AND source_key=$2`, testWorkspaceID, sourceKey).Scan(&bindingRevision)
	config := map[string]any{"project_id": projectID, "creator_agent_id": creatorAgentID, "enabled": true, "rotate_credential": true, "expected_revision": bindingRevision}
	testutil.Call(t, testHandler.PutExternalIncidentSourceBinding,
		externalIncidentConfigRequest(http.MethodPut, config, sourceKey)).Want(http.StatusOK).JSON(&rotated)
	if rotated.Credential == "" || rotated.Credential == credential {
		t.Fatal("credential rotation did not return a new one-time credential")
	}
	testutil.Call(t, testHandler.PostExternalIncidentOccurrence,
		externalIncidentOccurrenceRequest(http.MethodPost, wrong, sourceKey, credential)).Want(http.StatusUnauthorized)

	terminalFirst := externalIncidentEvent("incident:multica:"+externalIncidentDigest("terminal-first"), "railway-no-active-episode", 1, "recovered", "low", time.Now().UTC())
	testutil.Call(t, testHandler.PostExternalIncidentOccurrence,
		externalIncidentOccurrenceRequest(http.MethodPost, terminalFirst, sourceKey, rotated.Credential)).Want(http.StatusUnprocessableEntity)
	testutil.Call(t, testHandler.PostExternalIncidentOccurrence,
		externalIncidentOccurrenceRequest(http.MethodPost, terminalFirst, sourceKey, rotated.Credential)).Want(http.StatusUnprocessableEntity)
	var state, errorCode string
	dbfx.QueryRow(t, `SELECT state, error_code FROM external_incident_occurrence WHERE workspace_id=$1 AND source_key=$2`, testWorkspaceID, sourceKey).Scan(&state, &errorCode)
	if state != "rejected" || errorCode != "terminal_event_requires_active_episode" {
		t.Fatalf("rejected receipt = (%s, %s)", state, errorCode)
	}
}

// This body is the Intelligence incident_outbox payload plus its row's
// idempotency_key as event_key and the required producer-derived ordering
// fields. Intelligence must add those fields before enabling delivery.
func TestExternalIncidentAcceptsPlannedIntelligenceOutboxContract(t *testing.T) {
	projectID := dbfx.Project(t, "Outbox contract")
	sourceKey := fmt.Sprintf("intelligence-contract-%d", time.Now().UnixNano())
	credential, _ := configureExternalIncidentSource(t, sourceKey, projectID, true)
	cleanupExternalIncidentSource(t, sourceKey)
	body := externalIncidentEvent("incident:multica:"+externalIncidentDigest("outbox-row"), "current-intelligence-finding", 1, "suppressed", "medium", time.Now().UTC())
	body["action_required"] = false
	body["summary"] = "The deterministic check is intentionally suppressed."
	body["next_step"] = "Review the suppression when the source rule changes."
	var result map[string]any
	testutil.Call(t, testHandler.PostExternalIncidentOccurrence,
		externalIncidentOccurrenceRequest(http.MethodPost, body, sourceKey, credential)).Want(http.StatusCreated).JSON(&result)
	if result["transition"] != "suppressed" {
		t.Fatalf("result = %#v", result)
	}
}

func TestExternalIncidentDoesNotInventRulesBeyondIntelligenceOutbox(t *testing.T) {
	projectID := dbfx.Project(t, "Producer rule compatibility")
	sourceKey := fmt.Sprintf("intelligence-rules-%d", time.Now().UnixNano())
	credential, _ := configureExternalIncidentSource(t, sourceKey, projectID, true)
	cleanupExternalIncidentSource(t, sourceKey)
	observedAt := time.Now().UTC().Truncate(time.Second)

	first := externalIncidentEvent("incident:multica:"+externalIncidentDigest("rule-first"), "producer-rule-finding", 1, "new", "critical", observedAt)
	testutil.Call(t, testHandler.PostExternalIncidentOccurrence,
		externalIncidentOccurrenceRequest(http.MethodPost, first, sourceKey, credential)).Want(http.StatusCreated)
	firstKey := "incident:multica:" + externalIncidentDigest("rule-first")
	mismatch := externalIncidentSuccessorEvent("incident:multica:"+externalIncidentDigest("rule-mismatch"), "producer-rule-finding", 1, 2, firstKey, "worsened", "medium", observedAt)
	mismatch["incident_key"] = externalIncidentDigest("different-incident")
	testutil.Call(t, testHandler.PostExternalIncidentOccurrence,
		externalIncidentOccurrenceRequest(http.MethodPost, mismatch, sourceKey, credential)).Want(http.StatusUnprocessableEntity)

	// Intelligence permits an equal observed_at and does not infer severity
	// direction from the word "worsened". The consumer must not reject an
	// event the authoritative producer already applied and queued.
	second := externalIncidentSuccessorEvent("incident:multica:"+externalIncidentDigest("rule-second"), "producer-rule-finding", 1, 3, firstKey, "worsened", "medium", observedAt)
	testutil.Call(t, testHandler.PostExternalIncidentOccurrence,
		externalIncidentOccurrenceRequest(http.MethodPost, second, sourceKey, credential)).Want(http.StatusCreated)
}

func TestExternalIncidentPendingRetryReconcilesAcrossEpisodes(t *testing.T) {
	projectID := dbfx.Project(t, "Ordered incident delivery")
	sourceKey := fmt.Sprintf("intelligence-order-%d", time.Now().UnixNano())
	credential, _ := configureExternalIncidentSource(t, sourceKey, projectID, true)
	cleanupExternalIncidentSource(t, sourceKey)
	base := time.Now().UTC().Truncate(time.Second)
	finding := "railway-ordering"
	firstKey := "incident:multica:" + externalIncidentDigest("ordered-open")
	secondKey := "incident:multica:" + externalIncidentDigest("ordered-recovered")
	thirdKey := "incident:multica:" + externalIncidentDigest("ordered-reopened")
	second := externalIncidentSuccessorEvent(secondKey, finding, 1, 30, firstKey, "recovered", "low", base.Add(time.Minute))
	third := externalIncidentSuccessorEvent(thirdKey, finding, 2, 55, secondKey, "new", "critical", base.Add(2*time.Minute))

	var firstPending, replayPending map[string]any
	testutil.Call(t, testHandler.PostExternalIncidentOccurrence,
		externalIncidentOccurrenceRequest(http.MethodPost, third, sourceKey, credential)).Want(http.StatusAccepted).JSON(&firstPending)
	testutil.Call(t, testHandler.PostExternalIncidentOccurrence,
		externalIncidentOccurrenceRequest(http.MethodPost, third, sourceKey, credential)).Want(http.StatusAccepted).JSON(&replayPending)
	for _, field := range []string{"event_key", "finding_key", "episode_generation", "transition", "receipt_state"} {
		if replayPending[field] != firstPending[field] {
			t.Fatalf("pending retry changed %s: first=%v retry=%v", field, firstPending[field], replayPending[field])
		}
	}
	if firstPending["receipt_state"] != "pending_predecessor" {
		t.Fatalf("pending receipt state = %v", firstPending["receipt_state"])
	}
	testutil.Call(t, testHandler.PostExternalIncidentOccurrence,
		externalIncidentOccurrenceRequest(http.MethodPost, second, sourceKey, credential)).Want(http.StatusAccepted)

	first := externalIncidentSuccessorEvent(firstKey, finding, 1, 10, "", "new", "high", base)
	testutil.Call(t, testHandler.PostExternalIncidentOccurrence,
		externalIncidentOccurrenceRequest(http.MethodPost, first, sourceKey, credential)).Want(http.StatusCreated)
	if got := dbfx.Count(t, `SELECT count(*) FROM external_incident_occurrence WHERE workspace_id=$1 AND source_key=$2 AND state='pending_predecessor'`, testWorkspaceID, sourceKey); got != 2 {
		t.Fatalf("pending receipts after predecessor commit = %d, want 2; post-commit work must not be the durability mechanism", got)
	}

	statuses := make(chan int, 2)
	var wg sync.WaitGroup
	for _, body := range []map[string]any{second, third} {
		body := body
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses <- testutil.Call(t, testHandler.PostExternalIncidentOccurrence,
				externalIncidentOccurrenceRequest(http.MethodPost, body, sourceKey, credential)).Code
		}()
	}
	wg.Wait()
	close(statuses)
	for status := range statuses {
		if status != http.StatusCreated && status != http.StatusOK && status != http.StatusAccepted && status != http.StatusConflict {
			t.Fatalf("concurrent ordered delivery status = %d", status)
		}
	}
	// A 202 is durable retry work, not delivery success. Retrying the exact
	// successor after its predecessor applies must claim and apply it.
	testutil.Call(t, testHandler.PostExternalIncidentOccurrence,
		externalIncidentOccurrenceRequest(http.MethodPost, second, sourceKey, credential)).Want(http.StatusOK)
	thirdRetry := testutil.Call(t, testHandler.PostExternalIncidentOccurrence,
		externalIncidentOccurrenceRequest(http.MethodPost, third, sourceKey, credential))
	if thirdRetry.Code != http.StatusCreated && thirdRetry.Code != http.StatusOK {
		t.Fatalf("third retry status = %d, want applied 201 or replay 200", thirdRetry.Code)
	}

	var state string
	var sequence int64
	dbfx.QueryRow(t, `SELECT current_state, current_source_sequence FROM external_incident_episode
		WHERE workspace_id=$1 AND source_key=$2 AND finding_key=$3 AND generation=2`,
		testWorkspaceID, sourceKey, externalIncidentDigest(finding)).Scan(&state, &sequence)
	if state != "new" || sequence != 55 {
		t.Fatalf("reconciled generation 2 head = (%s,%d), want (new,55)", state, sequence)
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM external_incident_occurrence WHERE workspace_id=$1 AND source_key=$2 AND state='applied'`, testWorkspaceID, sourceKey); got != 3 {
		t.Fatalf("applied ordered receipts = %d, want 3", got)
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1 AND origin_type='external_incident'`, testWorkspaceID); got != 2 {
		t.Fatalf("issues across two episodes = %d, want 2", got)
	}
}

func TestExternalIncidentHumanOwnedFieldsRemainUntouched(t *testing.T) {
	projectID := dbfx.Project(t, "Human owned incident fields")
	sourceKey := fmt.Sprintf("intelligence-human-%d", time.Now().UnixNano())
	credential, _ := configureExternalIncidentSource(t, sourceKey, projectID, true)
	cleanupExternalIncidentSource(t, sourceKey)
	base := time.Now().UTC().Truncate(time.Second)
	finding := "railway-human-touch"
	firstKey := "incident:multica:" + externalIncidentDigest("human-open")
	var opened map[string]any
	testutil.Call(t, testHandler.PostExternalIncidentOccurrence, externalIncidentOccurrenceRequest(http.MethodPost,
		externalIncidentSuccessorEvent(firstKey, finding, 1, 1, "", "new", "medium", base), sourceKey, credential)).Want(http.StatusCreated).JSON(&opened)
	issueID := opened["issue_id"].(string)
	if _, err := testPool.Exec(context.Background(), `UPDATE issue SET status='blocked', priority='urgent', revision=revision+1 WHERE id=$1`, issueID); err != nil {
		t.Fatal(err)
	}
	secondKey := "incident:multica:" + externalIncidentDigest("human-worsened")
	second := externalIncidentSuccessorEvent(secondKey, finding, 1, 2, firstKey, "worsened", "high", base.Add(time.Minute))
	second["summary"] = "Machine refreshed title"
	second["next_step"] = "Machine refreshed next step"
	testutil.Call(t, testHandler.PostExternalIncidentOccurrence,
		externalIncidentOccurrenceRequest(http.MethodPost, second, sourceKey, credential)).Want(http.StatusCreated)
	var title, description, status, priority string
	dbfx.QueryRow(t, `SELECT title, description, status, priority FROM issue WHERE id=$1`, issueID).Scan(&title, &description, &status, &priority)
	if title != "Machine refreshed title" || status != "blocked" || priority != "urgent" {
		t.Fatalf("partial human ownership result = (%q,%q,%q), want machine title and human status/priority", title, status, priority)
	}
	if _, err := testPool.Exec(context.Background(), `UPDATE issue SET title='Human title', description='Human runbook', revision=revision+1 WHERE id=$1`, issueID); err != nil {
		t.Fatal(err)
	}
	third := externalIncidentSuccessorEvent("incident:multica:"+externalIncidentDigest("human-recovered"), finding, 1, 3, secondKey, "recovered", "low", base.Add(2*time.Minute))
	third["summary"] = "Machine recovery title"
	third["next_step"] = "Machine recovery next step"
	testutil.Call(t, testHandler.PostExternalIncidentOccurrence,
		externalIncidentOccurrenceRequest(http.MethodPost, third, sourceKey, credential)).Want(http.StatusCreated)
	dbfx.QueryRow(t, `SELECT title, description, status, priority FROM issue WHERE id=$1`, issueID).Scan(&title, &description, &status, &priority)
	if title != "Human title" || description != "Human runbook" || status != "blocked" || priority != "urgent" {
		t.Fatalf("recovery overwrote human fields: (%q,%q,%q,%q)", title, description, status, priority)
	}
	var titleOwned, descriptionOwned, statusOwned, priorityOwned bool
	dbfx.QueryRow(t, `SELECT title_human_owned, description_human_owned, status_human_owned, priority_human_owned
		FROM external_incident_episode WHERE workspace_id=$1 AND source_key=$2 AND finding_key=$3 AND generation=1`,
		testWorkspaceID, sourceKey, externalIncidentDigest(finding)).Scan(&titleOwned, &descriptionOwned, &statusOwned, &priorityOwned)
	if !titleOwned || !descriptionOwned || !statusOwned || !priorityOwned {
		t.Fatalf("sticky human ownership = (%v,%v,%v,%v), want all true", titleOwned, descriptionOwned, statusOwned, priorityOwned)
	}
}

func TestExternalIncidentRealtimePayloadIsCanonicalWithoutOwnerNotification(t *testing.T) {
	projectID := dbfx.Project(t, "Realtime incident")
	sourceKey := fmt.Sprintf("intelligence-realtime-%d", time.Now().UnixNano())
	credential, _ := configureExternalIncidentSource(t, sourceKey, projectID, true)
	cleanupExternalIncidentSource(t, sourceKey)
	createdEvents := make(chan events.Event, 1)
	updatedEvents := make(chan events.Event, 1)
	isThisSource := func(payload map[string]any) bool {
		issue, _ := payload["issue"].(map[string]any)
		metadata, _ := issue["metadata"].(map[string]any)
		incident, _ := metadata["external_incident"].(map[string]any)
		return incident["source_key"] == sourceKey
	}
	testHandler.Bus.Subscribe(protocol.EventIssueCreated, func(event events.Event) {
		if payload, ok := event.Payload.(map[string]any); ok && payload["external_incident"] == true && isThisSource(payload) {
			createdEvents <- event
		}
	})
	testHandler.Bus.Subscribe(protocol.EventIssueUpdated, func(event events.Event) {
		if payload, ok := event.Payload.(map[string]any); ok && payload["external_incident"] == true && isThisSource(payload) {
			updatedEvents <- event
		}
	})
	before := dbfx.Count(t, `SELECT count(*) FROM owner_notification WHERE workspace_id=$1`, testWorkspaceID)
	base := time.Now().UTC().Truncate(time.Second)
	finding := "railway-realtime"
	firstKey := "incident:multica:" + externalIncidentDigest("realtime-open")
	testutil.Call(t, testHandler.PostExternalIncidentOccurrence, externalIncidentOccurrenceRequest(http.MethodPost,
		externalIncidentSuccessorEvent(firstKey, finding, 1, 1, "", "new", "medium", base), sourceKey, credential)).Want(http.StatusCreated)
	created := <-createdEvents
	createdPayload := created.Payload.(map[string]any)
	createdIssue, ok := createdPayload["issue"].(map[string]any)
	if !ok || createdIssue["id"] == "" || createdIssue["workspace_id"] != testWorkspaceID || createdIssue["revision"] == nil {
		t.Fatalf("created realtime payload is not a canonical issue snapshot: %#v", createdPayload)
	}
	testutil.Call(t, testHandler.PostExternalIncidentOccurrence, externalIncidentOccurrenceRequest(http.MethodPost,
		externalIncidentSuccessorEvent("incident:multica:"+externalIncidentDigest("realtime-recovered"), finding, 1, 2, firstKey, "recovered", "low", base.Add(time.Minute)), sourceKey, credential)).Want(http.StatusCreated)
	updated := <-updatedEvents
	updatedPayload := updated.Payload.(map[string]any)
	if _, ok := updatedPayload["issue"].(map[string]any); !ok || updatedPayload["status_changed"] != true || updatedPayload["prev_status"] != "in_progress" {
		t.Fatalf("updated realtime payload = %#v", updatedPayload)
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM owner_notification WHERE workspace_id=$1`, testWorkspaceID); got != before {
		t.Fatalf("owner notifications changed from %d to %d", before, got)
	}
}

func TestExternalIncidentSourceCredentialCreationAndRotationAreSerialized(t *testing.T) {
	projectID := dbfx.Project(t, "Credential serialization")
	agentID := dbfx.Agent(t, "Credential serialization agent", "")
	sourceKey := fmt.Sprintf("intelligence-credential-race-%d", time.Now().UnixNano())
	cleanupExternalIncidentSource(t, sourceKey)
	body := map[string]any{"project_id": projectID, "creator_agent_id": agentID, "enabled": true}
	type configResult struct {
		status int
		body   externalIncidentSourceBindingResponse
	}
	results := make(chan configResult, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			response := testutil.Call(t, testHandler.PutExternalIncidentSourceBinding, externalIncidentConfigRequest(http.MethodPut, body, sourceKey))
			var decoded externalIncidentSourceBindingResponse
			_ = json.Unmarshal(response.Body.Bytes(), &decoded)
			results <- configResult{status: response.Code, body: decoded}
		}()
	}
	wg.Wait()
	close(results)
	credentials := []string{}
	var revision int64
	for result := range results {
		if result.status != http.StatusOK {
			t.Fatalf("concurrent creation status = %d", result.status)
		}
		if result.body.Credential != "" {
			credentials = append(credentials, result.body.Credential)
		}
		revision = result.body.BindingRevision
	}
	if len(credentials) != 1 {
		t.Fatalf("live credentials returned by concurrent creation = %d, want 1", len(credentials))
	}
	if _, err := testHandler.ExternalIncidentService.AuthenticateSource(context.Background(), parseUUID(testWorkspaceID), sourceKey, credentials[0]); err != nil {
		t.Fatalf("returned creation credential is not live: %v", err)
	}
	rotate := map[string]any{"project_id": projectID, "creator_agent_id": agentID, "enabled": true, "rotate_credential": true, "expected_revision": revision}
	rotationResults := make(chan configResult, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			response := testutil.Call(t, testHandler.PutExternalIncidentSourceBinding, externalIncidentConfigRequest(http.MethodPut, rotate, sourceKey))
			var decoded externalIncidentSourceBindingResponse
			_ = json.Unmarshal(response.Body.Bytes(), &decoded)
			rotationResults <- configResult{status: response.Code, body: decoded}
		}()
	}
	wg.Wait()
	close(rotationResults)
	rotatedCredentials := []string{}
	conflicts := 0
	for result := range rotationResults {
		if result.status == http.StatusConflict {
			conflicts++
		}
		if result.status == http.StatusOK && result.body.Credential != "" {
			rotatedCredentials = append(rotatedCredentials, result.body.Credential)
		}
	}
	if len(rotatedCredentials) != 1 || conflicts != 1 {
		t.Fatalf("concurrent rotations returned %d credentials and %d conflicts, want 1/1", len(rotatedCredentials), conflicts)
	}
	if _, err := testHandler.ExternalIncidentService.AuthenticateSource(context.Background(), parseUUID(testWorkspaceID), sourceKey, rotatedCredentials[0]); err != nil {
		t.Fatalf("returned rotation credential is not live: %v", err)
	}
}

func configureExternalIncidentSource(t *testing.T, sourceKey, projectID string, enabled bool) (credential, agentID string) {
	t.Helper()
	agentID = dbfx.Agent(t, "Silver Mediums Intelligence "+sourceKey, "")
	var configured externalIncidentSourceBindingResponse
	body := map[string]any{"project_id": projectID, "creator_agent_id": agentID, "enabled": enabled}
	testutil.Call(t, testHandler.PutExternalIncidentSourceBinding,
		externalIncidentConfigRequest(http.MethodPut, body, sourceKey)).Want(http.StatusOK).JSON(&configured)
	if configured.Credential == "" {
		t.Fatal("first source configuration did not return its one-time machine credential")
	}
	return configured.Credential, agentID
}

func cleanupExternalIncidentSource(t *testing.T, sourceKey string) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = testPool.Exec(ctx, `DELETE FROM external_incident_quarantine WHERE workspace_id=$1 AND source_key=$2`, testWorkspaceID, sourceKey)
		_, _ = testPool.Exec(ctx, `DELETE FROM external_incident_occurrence WHERE workspace_id=$1 AND source_key=$2`, testWorkspaceID, sourceKey)
		_, _ = testPool.Exec(ctx, `DELETE FROM external_incident_episode WHERE workspace_id=$1 AND source_key=$2`, testWorkspaceID, sourceKey)
		_, _ = testPool.Exec(ctx, `DELETE FROM external_incident_source_binding WHERE workspace_id=$1 AND source_key=$2`, testWorkspaceID, sourceKey)
		_, _ = testPool.Exec(ctx, `DELETE FROM issue WHERE workspace_id=$1 AND origin_type='external_incident'`, testWorkspaceID)
	})
}

func externalIncidentEvent(eventKey, findingSeed string, generation int64, transition, severity string, observedAt time.Time) map[string]any {
	return externalIncidentSuccessorEvent(eventKey, findingSeed, generation, 1, "", transition, severity, observedAt)
}

func externalIncidentSuccessorEvent(eventKey, findingSeed string, generation, sourceSequence int64, predecessorEventKey, transition, severity string, observedAt time.Time) map[string]any {
	return map[string]any{
		"event_key": eventKey, "finding_key": externalIncidentDigest(findingSeed),
		"incident_key": externalIncidentDigest("incident:" + findingSeed), "episode_generation": generation,
		"source_sequence": sourceSequence, "predecessor_event_key": predecessorEventKey,
		"transition": transition, "severity": severity, "action_required": true,
		"summary":       "Deterministic watcher evidence changed.",
		"next_step":     "Open the evidence and correct the first deterministic error.",
		"observed_at":   observedAt.Format(time.RFC3339Nano),
		"evidence_refs": []string{"railway-log-window:123"},
	}
}

func externalIncidentDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func externalIncidentOccurrenceRequest(method string, body any, sourceKey, credential string) *http.Request {
	req := testutil.JSONRequest(method, "/api/workspaces/"+testWorkspaceID+"/external-incident-sources/"+sourceKey+"/occurrences", body)
	req = testutil.WithHeaders(req, "Authorization", "Bearer "+credential)
	return testutil.WithURLParams(req, "id", testWorkspaceID, "sourceKey", sourceKey)
}

func externalIncidentConfigRequest(method string, body any, sourceKey string) *http.Request {
	req := testutil.JSONRequest(method, "/api/workspaces/"+testWorkspaceID+"/external-incident-sources/"+sourceKey, body)
	req = testutil.WithHeaders(req, "X-User-ID", testUserID)
	return testutil.WithURLParams(req, "id", testWorkspaceID, "sourceKey", sourceKey)
}
