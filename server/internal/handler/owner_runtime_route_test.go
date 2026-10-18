package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workflowprojection"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type ownerRouteFixture struct {
	issue, intake, impl, review, release, runtime, squad string
	revision                                             int64
	request                                              service.OwnerRuntimeRouteRequest
}

func newOwnerRouteFixture(t *testing.T) ownerRouteFixture {
	t.Helper()
	if testPool == nil {
		t.Fatal("isolated database required")
	}
	f := ownerRouteFixture{runtime: handlerTestRuntimeID(t)}
	f.impl = createHandlerTestAgent(t, "Route implementer", nil)
	f.review = createHandlerTestAgent(t, "Route QC", nil)
	f.release = createHandlerTestAgent(t, "Route Fleet", nil)
	leader := createHandlerTestAgent(t, "Undeclared global leader", nil)
	f.squad = dbfx.Squad(t, "Existing owner route squad", leader)
	daemon := uuid.NewString()
	dbfx.Exec(t, `UPDATE agent_runtime SET daemon_id=$2,last_seen_at=now(),status='online',owner_id=$3 WHERE id=$1`, f.runtime, daemon, testUserID)
	for _, agent := range []string{f.impl, f.review, f.release} {
		dbfx.Exec(t, `UPDATE agent SET runtime_id=$2,owner_id=$3 WHERE id=$1`, agent, f.runtime, testUserID)
		dbfx.SquadMember(t, f.squad, "agent", agent)
	}
	f.issue = dbfx.Issue(t, "Owner route fixture", testutil.Cols{"status": "todo", "assignee_type": "agent", "assignee_id": f.impl})
	item, err := testHandler.Queries.CreateInboxItem(context.Background(), db.CreateInboxItemParams{ID: dbid.NewV7(), WorkspaceID: parseUUID(testWorkspaceID), RecipientType: "member", RecipientID: parseUUID(testUserID), Type: "chat_intake", Severity: "action_required", IssueID: parseUUID(f.issue), Title: "Actual request fixture", Details: []byte(`{"request":"Verify this issue then review and release its exact candidate","intake_state":"pending"}`)})
	if err != nil {
		t.Fatal(err)
	}
	f.intake = uuidToString(item.ID)
	if err = testPool.QueryRow(context.Background(), `SELECT revision FROM issue WHERE id=$1`, f.issue).Scan(&f.revision); err != nil {
		t.Fatal(err)
	}
	target := func(id string) service.OwnerRuntimeTarget {
		return service.OwnerRuntimeTarget{AgentID: id, RuntimeID: f.runtime}
	}
	f.request = service.OwnerRuntimeRouteRequest{SquadID: f.squad, DaemonID: daemon, Implementation: target(f.impl), Review: target(f.review), Release: target(f.release), Verification: target(f.review)}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM issue_workflow_event WHERE issue_id=$1`, f.issue)
		testPool.Exec(context.Background(), `DELETE FROM issue_workflow_plan WHERE issue_id=$1`, f.issue)
	})
	return f
}
func (f ownerRouteFixture) admit(route service.OwnerRuntimeRouteRequest) *httptest.ResponseRecorder {
	r := withURLParam(newRequest(http.MethodPost, "/api/issues/"+f.issue+"/owner-directives", map[string]any{"intake_id": f.intake, "expected_issue_revision": f.revision, "directive": "correct", "content": "Verify this issue then review and release its exact candidate", "owner_runtime_route": route}), "id", f.issue)
	w := httptest.NewRecorder()
	testHandler.PostIssueOwnerDirective(w, r)
	return w
}
func (f ownerRouteFixture) first(t *testing.T) string {
	t.Helper()
	w := f.admit(f.request)
	if w.Code != 201 {
		t.Fatalf("admit: %d %s", w.Code, w.Body)
	}
	var receipt map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &receipt)
	return receipt["task_id"].(string)
}
func startOwnerTask(t *testing.T, task string) {
	t.Helper()
	dbfx.Exec(t, `UPDATE agent_task_queue SET status='dispatched',dispatched_at=now() WHERE id=$1`, task)
	if _, err := testHandler.TaskService.StartTask(context.Background(), parseUUID(task)); err != nil {
		t.Fatal(err)
	}
}
func finishOwnerTask(t *testing.T, task string) {
	t.Helper()
	if _, err := testHandler.TaskService.CompleteTask(context.Background(), parseUUID(task), []byte(`{}`), "", "", "", false, "", ""); err != nil {
		t.Fatal(err)
	}
}
func (f ownerRouteFixture) event(task, agent, event string, extra map[string]any) *httptest.ResponseRecorder {
	body := map[string]any{"event": event, "reason": "Observed test evidence"}
	for key, value := range extra {
		body[key] = value
	}
	r := withURLParam(newRequest(http.MethodPost, "/api/issues/"+f.issue+"/workflow-events", body), "id", f.issue)
	r.Header.Set("X-Agent-ID", agent)
	r.Header.Set("X-Task-ID", task)
	w := httptest.NewRecorder()
	testHandler.PostIssueWorkflowEvent(w, r)
	return w
}
func (f ownerRouteFixture) stage(t *testing.T, want workflowprojection.OwnerIssueStage) {
	t.Helper()
	picture, err := testHandler.Queries.GetIssueCurrentPicture(context.Background(), db.GetIssueCurrentPictureParams{IssueID: parseUUID(f.issue), WorkspaceID: parseUUID(testWorkspaceID)})
	if err != nil {
		t.Fatal(err)
	}
	summary := workflowprojection.OwnerIssueSummaryFromDocument(picture.Document, picture.Revision, picture.GeneratedAt.Time)
	if summary.Stage != want {
		t.Fatalf("stage=%s want %s document=%s", summary.Stage, want, picture.Document)
	}
}
func (f ownerRouteFixture) queued(t *testing.T, agent string) string {
	t.Helper()
	var task string
	if err := testPool.QueryRow(context.Background(), `SELECT id FROM agent_task_queue WHERE issue_id=$1 AND agent_id=$2 AND status='queued'`, f.issue, agent).Scan(&task); err != nil {
		t.Fatal(err)
	}
	return task
}
func routeManifest() (*workflowArtifactEvidencePacketV1, string) {
	file := strings.Repeat("a", 64)
	sum := sha256.Sum256([]byte("actual.py\x00" + file + "\n"))
	digest := hex.EncodeToString(sum[:])
	return &workflowArtifactEvidencePacketV1{SchemaVersion: 1, Contract: "file_manifest.v1", CandidateSHA256: digest, Artifacts: []workflowArtifactDigestV1{{Path: "actual.py", DeclaredSHA256: file, ObservedSHA256: file}}}, digest
}
func routeEvidence(digest string, keys ...string) *service.OwnerRuntimeStageEvidence {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	p := &service.OwnerRuntimeStageEvidence{CandidateSHA256: digest, BuildManifestSHA256: strings.Repeat("b", 64), TargetKey: "fixture-service", DeployedRevision: "receipt-v2", Gates: map[string]workflowprojection.ReleaseEvidenceGate{}}
	for _, key := range keys {
		p.Gates[key] = workflowprojection.ReleaseEvidenceGate{State: "passed", Refs: []string{"fixture://observed/" + key}, ObservedAt: &now, Limitations: []string{}}
	}
	return p
}

func TestOwnerRuntimeRouteAtomicAdmission(t *testing.T) {
	f := newOwnerRouteFixture(t)
	bad := f.request
	bad.Verification.RuntimeID = uuid.NewString()
	if w := f.admit(bad); w.Code != 409 {
		t.Fatalf("inconsistent binding: %d %s", w.Code, w.Body)
	}
	old := testHandler.TaskService.PrepareWorkflowGuardedAuthority
	testHandler.TaskService.PrepareWorkflowGuardedAuthority = func(context.Context, *db.Queries, db.Issue, db.Agent, pgtype.UUID, string, []byte) ([]byte, bool, error) {
		return []byte(`{"guarded_dispatch":{}}`), true, nil
	}
	denied := f.admit(f.request)
	testHandler.TaskService.PrepareWorkflowGuardedAuthority = old
	if denied.Code != 409 {
		t.Fatalf("guarded conversion: %d %s", denied.Code, denied.Body)
	}
	var writes int
	if err := testPool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM issue_workflow_plan WHERE issue_id=$1)+(SELECT count(*) FROM issue_owner_directive WHERE issue_id=$1)+(SELECT count(*) FROM comment WHERE issue_id=$1)+(SELECT count(*) FROM agent_task_queue WHERE issue_id=$1)`, f.issue).Scan(&writes); err != nil || writes != 0 {
		t.Fatalf("partial writes=%d error=%v", writes, err)
	}
	var wg sync.WaitGroup
	responses := make(chan *httptest.ResponseRecorder, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); responses <- f.admit(f.request) }()
	}
	wg.Wait()
	close(responses)
	for w := range responses {
		if w.Code != 201 && w.Code != 200 {
			t.Fatalf("concurrent continuation: %d %s", w.Code, w.Body)
		}
	}
	var tasks int
	_ = testPool.QueryRow(context.Background(), `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, f.issue).Scan(&tasks)
	if tasks != 1 {
		t.Fatalf("tasks=%d", tasks)
	}
	rows, err := testHandler.Queries.ListIssuesMissingWorkflowPlan(context.Background(), 1000)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if uuidToString(row.ID) == f.issue {
			t.Fatal("owner route leaked to global reconciler")
		}
	}
	f.stage(t, workflowprojection.OwnerStageIncoming)
	changed := f.request
	changed.AllowBriefDeploymentInterruption = true
	if w := f.admit(changed); w.Code != 409 {
		t.Fatalf("changed replay: %d", w.Code)
	}
}

func TestOwnerRuntimeRouteActualStageSequence(t *testing.T) {
	f := newOwnerRouteFixture(t)
	task := f.first(t)
	startOwnerTask(t, task)
	f.stage(t, workflowprojection.OwnerStageActive)
	if w := f.event(task, f.impl, "task_started", nil); w.Code != 201 {
		t.Fatalf("start event: %d %s", w.Code, w.Body)
	}
	manifest, digest := routeManifest()
	if w := f.event(task, f.review, "implementation_ready", map[string]any{"artifact_evidence": manifest}); w.Code != 403 {
		t.Fatalf("wrong actor: %d", w.Code)
	}
	if w := f.event(task, f.impl, "implementation_ready", map[string]any{"artifact_evidence": manifest}); w.Code != 202 {
		t.Fatalf("implementation: %d %s", w.Code, w.Body)
	}
	changed, _ := routeManifest()
	changed.Artifacts[0].Path = "other.py"
	if w := f.event(task, f.impl, "implementation_ready", map[string]any{"artifact_evidence": changed}); w.Code == 201 || w.Code == 202 {
		t.Fatal("changed event replay accepted")
	}
	var count int
	_ = testPool.QueryRow(context.Background(), `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, f.issue).Scan(&count)
	if count != 1 {
		t.Fatal("successor overlapped reporting worker")
	}
	finishOwnerTask(t, task)
	task = f.queued(t, f.review)
	startOwnerTask(t, task)
	f.stage(t, workflowprojection.OwnerStageInReview)
	packet := routeEvidence(digest, "review", "build")
	if w := f.event(task, f.review, "review_passed", map[string]any{"candidate": strings.Repeat("c", 64), "stage_evidence": packet}); w.Code != 409 {
		t.Fatalf("wrong candidate: %d %s", w.Code, w.Body)
	}
	if w := f.event(task, f.review, "review_passed", map[string]any{"candidate": digest, "stage_evidence": packet}); w.Code != 202 {
		t.Fatalf("review: %d %s", w.Code, w.Body)
	}
	finishOwnerTask(t, task)
	task = f.queued(t, f.release)
	startOwnerTask(t, task)
	f.stage(t, workflowprojection.OwnerStageReleasing)
	packet = routeEvidence(digest, "deployment", "exposure")
	if w := f.event(task, f.release, "deployment_succeeded", map[string]any{"candidate": digest, "stage_evidence": packet}); w.Code != 202 {
		t.Fatalf("deploy: %d %s", w.Code, w.Body)
	}
	finishOwnerTask(t, task)
	task = f.queued(t, f.review)
	startOwnerTask(t, task)
	f.stage(t, workflowprojection.OwnerStageVerifying)
	packet = routeEvidence(digest, "runtime", "business", "regression")
	if w := f.event(task, f.review, "production_verified", map[string]any{"candidate": digest, "stage_evidence": packet}); w.Code != 201 {
		t.Fatalf("verify: %d %s", w.Code, w.Body)
	}
	finishOwnerTask(t, task)
	f.stage(t, workflowprojection.OwnerStageComplete)
	_ = testPool.QueryRow(context.Background(), `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, f.issue).Scan(&count)
	if count != 4 {
		t.Fatalf("actual task chain count=%d", count)
	}
	_ = testPool.QueryRow(context.Background(), `SELECT count(*) FROM inbox_item WHERE issue_id=$1 AND details->>'owner_category'='completion'`, f.issue).Scan(&count)
	if count != 1 {
		t.Fatalf("completion inbox=%d", count)
	}
}

func TestOwnerRuntimeRouteAuthorityAndMissingResult(t *testing.T) {
	for _, kind := range []string{"scope", "runtime", "membership", "permission", "missing_result", "failed_result"} {
		t.Run(kind, func(t *testing.T) {
			f := newOwnerRouteFixture(t)
			task := f.first(t)
			switch kind {
			case "scope":
				dbfx.Exec(t, `UPDATE issue SET title='Changed scope' WHERE id=$1`, f.issue)
			case "runtime":
				dbfx.Exec(t, `UPDATE agent_runtime SET daemon_id=$2 WHERE id=$1`, f.runtime, uuid.NewString())
			case "membership":
				dbfx.Exec(t, `DELETE FROM squad_member WHERE squad_id=$1 AND member_id=$2`, f.squad, f.review)
			case "permission":
				other := dbfx.User(t, "Other owner", uuid.NewString()+"@example.test")
				t.Cleanup(func() { dbfx.Exec(t, `UPDATE agent SET owner_id=$2 WHERE id=$1`, f.review, testUserID) })
				dbfx.Exec(t, `UPDATE agent SET permission_mode='private',owner_id=$2 WHERE id=$1`, f.review, other)
			}
			if kind != "missing_result" && kind != "failed_result" {
				row, err := testHandler.Queries.GetAgentTask(context.Background(), parseUUID(task))
				if err != nil {
					t.Fatal(err)
				}
				if _, err = testHandler.TaskService.ValidateOwnerRuntimeTask(context.Background(), testHandler.Queries, row, false); err == nil {
					t.Fatal("changed authority accepted")
				}
				return
			}
			startOwnerTask(t, task)
			if kind == "missing_result" {
				finishOwnerTask(t, task)
			} else {
				if _, err := testHandler.TaskService.FailTask(context.Background(), parseUUID(task), "actual fixture failure", "", "", "", "provider_error", false, "", ""); err != nil {
					t.Fatal(err)
				}
			}
			f.stage(t, workflowprojection.OwnerStageTechnicalStop)
			var count int
			_ = testPool.QueryRow(context.Background(), `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, f.issue).Scan(&count)
			if count != 1 {
				t.Fatalf("undeclared recovery task count=%d", count)
			}
		})
	}
}

func TestOwnerRuntimeRouteFileManifestBoundary(t *testing.T) {
	packet, digest := routeManifest()
	raw, _ := json.Marshal(packet)
	if actual, err := service.ValidateFileManifest(raw); err != nil || actual != digest {
		t.Fatal(actual, err)
	}
	for _, mutate := range []func(*workflowArtifactEvidencePacketV1){func(p *workflowArtifactEvidencePacketV1) { p.Artifacts = append(p.Artifacts, p.Artifacts[0]) }, func(p *workflowArtifactEvidencePacketV1) { p.Artifacts[0].ObservedSHA256 = strings.Repeat("0", 64) }, func(p *workflowArtifactEvidencePacketV1) { p.CandidateSHA256 = strings.Repeat("0", 64) }} {
		p, _ := routeManifest()
		mutate(p)
		raw, _ := json.Marshal(p)
		if _, err := service.ValidateFileManifest(raw); err == nil {
			t.Fatal("invalid manifest accepted")
		}
	}
	if err := validateOwnerOrLegacyArtifactRequest(issueWorkflowEventRequest{Event: "implementation_ready", ArtifactEvidence: packet}, false); err == nil {
		t.Fatal("file manifest bypassed legacy inventory contract")
	}
}

func TestOwnerRuntimeRouteQuestionResume(t *testing.T) {
	f := newOwnerRouteFixture(t)
	task := f.first(t)
	startOwnerTask(t, task)
	if w := f.event(task, f.impl, "task_started", nil); w.Code != 201 {
		t.Fatal(w.Code, w.Body)
	}
	w := f.event(task, f.impl, "owner_decision_required", map[string]any{"owner_decision": ownerDecisionPacket()})
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body)
	}
	var reported issueWorkflowEventResponse
	if err := json.Unmarshal(w.Body.Bytes(), &reported); err != nil {
		t.Fatal(err)
	}
	f.stage(t, workflowprojection.OwnerStageNeedsOwner)
	answer := func(user string) *httptest.ResponseRecorder {
		r := withURLParam(newRequest(http.MethodPost, "/api/issues/"+f.issue+"/workflow-owner-decision", map[string]any{"request_event_id": reported.EventID, "answer": "Use the reviewed copy only."}), "id", f.issue)
		r.Header.Set("X-User-ID", user)
		w := httptest.NewRecorder()
		testHandler.PostIssueWorkflowOwnerDecision(w, r)
		return w
	}
	if w := answer(testUserID); w.Code != 409 {
		t.Fatalf("running reporter answer: %d %s", w.Code, w.Body)
	}
	finishOwnerTask(t, task)
	f.stage(t, workflowprojection.OwnerStageNeedsOwner)
	foreign := dbfx.User(t, "Other member", uuid.NewString()+"@example.test")
	dbfx.Member(t, testWorkspaceID, foreign, "member")
	if w := answer(foreign); w.Code != 403 {
		t.Fatalf("foreign answer: %d %s", w.Code, w.Body)
	}
	if w := answer(testUserID); w.Code != 201 {
		t.Fatalf("owner answer: %d %s", w.Code, w.Body)
	}
	next := f.queued(t, f.impl)
	startOwnerTask(t, next)
	f.stage(t, workflowprojection.OwnerStageActive)
	var open int
	_ = testPool.QueryRow(context.Background(), `SELECT count(*) FROM inbox_item WHERE issue_id=$1 AND type='workflow_owner_decision' AND archived=false`, f.issue).Scan(&open)
	if open != 0 {
		t.Fatalf("answered question remains actionable: %d", open)
	}
}

func TestOwnerRuntimeRouteAgentCreatedIssueNotice(t *testing.T) {
	f := newOwnerRouteFixture(t)
	dbfx.Exec(t, `UPDATE issue SET creator_type='agent',creator_id=$2 WHERE id=$1`, f.issue, f.impl)
	if err := testPool.QueryRow(context.Background(), `SELECT revision FROM issue WHERE id=$1`, f.issue).Scan(&f.revision); err != nil {
		t.Fatal(err)
	}
	task := f.first(t)
	startOwnerTask(t, task)
	finishOwnerTask(t, task) // No accepted result must produce an owner-visible incident.
	f.stage(t, workflowprojection.OwnerStageTechnicalStop)
	var count int
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM inbox_item WHERE issue_id=$1 AND type='workflow_incident' AND recipient_type='member' AND recipient_id=$2`, f.issue, testUserID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("agent-created issue owner incident count=%d", count)
	}
}

func TestOwnerRuntimeRoutePreservesBoundIntakeContext(t *testing.T) {
	f := newOwnerRouteFixture(t)
	dbfx.Exec(t, `UPDATE inbox_item SET details=details-'request' WHERE id=$1`, f.intake)
	if w := f.admit(f.request); w.Code != http.StatusConflict {
		t.Fatalf("missing source request admitted: %d %s", w.Code, w.Body)
	}
	var count int
	if err := testPool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM agent_task_queue WHERE issue_id=$1)+(SELECT count(*) FROM comment WHERE issue_id=$1)+(SELECT count(*) FROM issue_owner_directive WHERE issue_id=$1)+(SELECT count(*) FROM issue_workflow_plan WHERE issue_id=$1)`, f.issue).Scan(&count); err != nil || count != 0 {
		t.Fatalf("missing request leaked admission: count=%d err=%v", count, err)
	}
	request := strings.Repeat("Existing approved scope. ", 100) + "\nattachment-id: 10000000-0000-0000-0000-000000000002 sha256: " + strings.Repeat("b", 64)
	dbfx.Exec(t, `UPDATE inbox_item SET details=jsonb_set(details,'{request}',to_jsonb($2::text)) WHERE id=$1`, f.intake, request)
	task := f.first(t)
	assertIntakeClaimContent(t, task, request, "Verify this issue then review and release its exact candidate")
}

func TestOwnerRuntimeRouteRunningProjectionIsNotAutomaticWait(t *testing.T) {
	f := newOwnerRouteFixture(t)
	task := f.first(t)
	assertProjection := func(running bool) {
		t.Helper()
		picture, err := testHandler.Queries.GetIssueCurrentPicture(context.Background(), db.GetIssueCurrentPictureParams{IssueID: parseUUID(f.issue), WorkspaceID: parseUUID(testWorkspaceID)})
		if err != nil {
			t.Fatal(err)
		}
		summary := workflowprojection.OwnerIssueSummaryFromDocument(picture.Document, picture.Revision, picture.GeneratedAt.Time)
		if !running {
			if summary.NextAction == nil || !summary.NextAction.Automatic {
				t.Fatal("queued work lost pending automatic action")
			}
			return
		}
		if picture.ExecutionState != "working" || summary.Stage != workflowprojection.OwnerStageActive || summary.NextAction != nil {
			t.Fatalf("running worker classified as automatic wait: execution=%s summary=%+v", picture.ExecutionState, summary)
		}
		for _, signal := range summary.Signals {
			if signal.Code == "waiting_automatic" || signal.Code == "automatic_continuation" {
				t.Fatalf("running worker has waiting signal: %s", signal.Code)
			}
		}
	}
	assertProjection(false)
	startOwnerTask(t, task)
	assertProjection(true) // Provider start alone already proves active work.
	if w := f.event(task, f.impl, "task_started", nil); w.Code != 201 {
		t.Fatal(w.Code, w.Body)
	}
	assertProjection(true) // Later stage event must retain the same Board truth.
}

func TestOwnerRuntimeRouteCompletionCrashRecovery(t *testing.T) {
	for _, replay := range []bool{true, false} {
		for _, accepted := range []bool{true, false} {
			t.Run(fmt.Sprintf("replay_%t_accepted_%t", replay, accepted), func(t *testing.T) {
				f := newOwnerRouteFixture(t)
				task := f.first(t)
				startOwnerTask(t, task)
				if w := f.event(task, f.impl, "task_started", nil); w.Code != 201 {
					t.Fatal(w.Code, w.Body)
				}
				if accepted {
					manifest, _ := routeManifest()
					if w := f.event(task, f.impl, "implementation_ready", map[string]any{"artifact_evidence": manifest}); w.Code != 202 {
						t.Fatal(w.Code, w.Body)
					}
				}
				reconciler := &service.WorkflowPlanReconciler{Queries: testHandler.Queries, TaskService: testHandler.TaskService, Bus: testHandler.Bus}
				// No callback/restart recovery may start a successor while its source runs.
				if err := reconciler.ReconcileOnce(context.Background()); err != nil {
					t.Fatal(err)
				}
				var count int
				_ = testPool.QueryRow(context.Background(), `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, f.issue).Scan(&count)
				if count != 1 {
					t.Fatal("recovery overlapped source worker")
				}
				// Simulate the terminal commit surviving a process crash, without any
				// in-memory post-commit callback. Both public retry and fresh reconciler
				// must recover from these durable rows, twice without duplicate work.
				dbfx.Exec(t, `UPDATE agent_task_queue SET status='completed',completed_at=now(),result='{}' WHERE id=$1`, task)
				for attempt := 0; attempt < 2; attempt++ {
					if replay {
						finishOwnerTask(t, task)
					} else if err := reconciler.ReconcileOnce(context.Background()); err != nil {
						t.Fatal(err)
					}
				}
				_ = testPool.QueryRow(context.Background(), `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, f.issue).Scan(&count)
				if accepted {
					if count != 2 {
						t.Fatalf("successor count=%d", count)
					}
					next := f.queued(t, f.review)
					row, err := testHandler.Queries.GetAgentTask(context.Background(), parseUUID(next))
					if err != nil || uuidToString(row.DelegatedFromTaskID) != task || uuidToString(row.RuntimeID) != f.runtime {
						t.Fatal("recovery lost declared source/runtime binding", err)
					}
				} else {
					if count != 1 {
						t.Fatal("missing result created unapproved successor")
					}
					f.stage(t, workflowprojection.OwnerStageTechnicalStop)
				}
			})
		}
	}
}

func TestOwnerRuntimeRouteFailureAfterAcceptedResult(t *testing.T) {
	for _, question := range []bool{false, true} {
		t.Run(fmt.Sprintf("question_%t", question), func(t *testing.T) {
			f := newOwnerRouteFixture(t)
			task := f.first(t)
			startOwnerTask(t, task)
			if w := f.event(task, f.impl, "task_started", nil); w.Code != 201 {
				t.Fatal(w.Code, w.Body)
			}
			var w *httptest.ResponseRecorder
			if question {
				w = f.event(task, f.impl, "owner_decision_required", map[string]any{"owner_decision": ownerDecisionPacket()})
			} else {
				manifest, _ := routeManifest()
				w = f.event(task, f.impl, "implementation_ready", map[string]any{"artifact_evidence": manifest})
			}
			if w.Code != 201 && w.Code != 202 {
				t.Fatal(w.Code, w.Body)
			}
			if _, err := testHandler.TaskService.FailTask(context.Background(), parseUUID(task), "provider failed after reporting", "", "", "", "provider_error", false, "", ""); err != nil {
				t.Fatal(err)
			}
			f.stage(t, workflowprojection.OwnerStageTechnicalStop)
			var tasks, questions int
			_ = testPool.QueryRow(context.Background(), `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, f.issue).Scan(&tasks)
			_ = testPool.QueryRow(context.Background(), `SELECT count(*) FROM inbox_item WHERE issue_id=$1 AND type='workflow_owner_decision' AND archived=false`, f.issue).Scan(&questions)
			if tasks != 1 || questions != 0 {
				t.Fatalf("tasks=%d questions=%d", tasks, questions)
			}
		})
	}
}
