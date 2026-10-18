package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workflowcommand"
	v2 "github.com/multica-ai/multica/server/internal/workflowstate/v2"
)

func TestChatVersionedApprovalUsesExactDecision(t *testing.T) {
	for _, action := range []string{"approve", "reject"} {
		for _, stale := range []bool{false, true} {
			name := "current"
			if stale {
				name = "replaced"
			}
			t.Run(action+"_"+name, func(t *testing.T) {
				ctx := context.Background()
				agentID, sessionID, _, _ := setupDirectChatSession(t, ctx, "Versioned decision")
				issue := parseUUID(dbfx.Issue(t, "Approve this exact candidate"))
				ids := seedChatDecisionWorkflow(t, ctx, issue)
				controller := parseUUID(uuid.NewString())
				now := func() time.Time { return time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC) }
				h := *testHandler
				h.WorkflowCommandService = &workflowcommand.Service{Starter: testPool, Queries: h.Queries, Loader: workflowcommand.Loader{Now: now}, Now: now, ControllerID: controller}
				request := workflowcommand.Request{WorkspaceID: parseUUID(testWorkspaceID), IssueID: issue, CommandKey: "chat-request-" + name, Kind: v2.CommandRequestDecision, OperationID: uuidToString(ids["execution"]), CandidateID: uuidToString(ids["candidate"]), Expected: []v2.ExpectedRevision{{Concern: v2.ConcernExecution, Revision: 1}, {Concern: v2.ConcernAttention, Revision: 1}, {Concern: v2.ConcernEvidence, Revision: 1}}, Decision: &workflowcommand.DecisionInput{Type: v2.DecisionAuthorization, Question: "Proceed with this exact operation?", Recommendation: "Review before proceeding", Consequence: "Only this operation continues", AccountableOwnerID: parseUUID(testUserID), AuthorityGap: v2.AuthorityGap{Kind: v2.AuthorityGapOperationAuthorization, RequestedCapability: "execute proposed operation", Reason: "owner authorization required"}}}
				if _, err := h.WorkflowCommandService.Execute(ctx, workflowcommand.Principal{ActorID: controller, ActorType: "internal_controller"}, request); err != nil {
					t.Fatal(err)
				}
				var decisionID string
				dbfx.QueryRow(t, `SELECT id FROM issue_workflow_decision WHERE issue_id=$1`, issue).Scan(&decisionID)
				packet, err := h.loadWorkflowDecisionPacket(ctx, parseUUID(testWorkspaceID), issue, decisionID, "", testUserID, false)
				if err != nil {
					t.Fatal(err)
				}
				if packet.Decision["branch_id"] != "main" {
					t.Fatal("pending review must expose its suspended branch")
				}
				linkDeliveryIssue(t, sessionID, uuidToString(issue))
				other := dbfx.Issue(t, "Separate question must remain pending")
				linkDeliveryIssue(t, sessionID, other)
				otherRequest := dbfx.Insert(t, "inbox_item", testutil.Cols{"workspace_id": testWorkspaceID, "recipient_type": "member", "recipient_id": testUserID, "type": "workflow_owner_decision", "severity": "action_required", "title": "Choose a tone", "body": "Friendly or concise?", "issue_id": other, "details": `{"question_scope":"ordinary_task","intake_state":"pending"}`})
				// Inbox is the canonical owner-request address, independent of the display projection.
				requestItem := dbfx.Insert(t, "inbox_item", testutil.Cols{"workspace_id": testWorkspaceID, "recipient_type": "member", "recipient_id": testUserID, "type": "workflow_owner_decision", "severity": "action_required", "title": "Approve operation", "body": "Proceed?", "issue_id": issue, "details": `{"decision_id":"` + decisionID + `"}`})
				session, _ := h.Queries.GetChatSession(ctx, parseUUID(sessionID))
				agent, _ := h.Queries.GetAgent(ctx, parseUUID(agentID))
				target, err := service.CaptureChatWorkTarget(ctx, h.Queries, session, parseUUID(testUserID), issue)
				if err != nil {
					t.Fatal(err)
				}
				if target.DecisionID != decisionID || target.DecisionRevision != 1 {
					t.Fatalf("lost exact decision: %+v", target)
				}
				sent, err := h.TaskService.SendDirectChatMessage(ctx, session, agent, parseUUID(testUserID), action+" this specific version.", nil, "member", parseUUID(testUserID), "", target)
				if err != nil {
					t.Fatal(err)
				}
				taskID := uuidToString(sent.Task.ID)
				markTaskRunning(t, ctx, taskID)
				if stale {
					dbfx.Exec(t, `UPDATE issue_workflow_decision SET revision=revision+1 WHERE id=$1`, decisionID)
				}
				for i := 0; i < 2; i++ {
					r := withChatTestWorkspaceCtx(t, newRequest(http.MethodPost, "/respond", map[string]any{"action": action}))
					r.Header.Set("X-Actor-Source", "task_token")
					r.Header.Set("X-Agent-ID", agentID)
					r.Header.Set("X-Task-ID", taskID)
					want := 200
					if stale {
						want = 409
					}
					testutil.Call(t, h.SubmitChatWorkResponse, r).Want(want)
				}
				var count int
				dbfx.QueryRow(t, `SELECT count(*) FROM issue_workflow_decision_response WHERE decision_id=$1`, decisionID).Scan(&count)
				want := 1
				if stale {
					want = 0
				}
				if count != want {
					t.Fatalf("response count %d, want %d", count, want)
				}
				var answeredRequestArchived bool
				dbfx.QueryRow(t, `SELECT archived FROM inbox_item WHERE id=$1`, requestItem).Scan(&answeredRequestArchived)
				if answeredRequestArchived == stale {
					t.Fatal("request notification must close only when its response commits")
				}
				if !stale {
					work, err := service.LoadChatWork(ctx, h.Queries, parseUUID(testWorkspaceID), parseUUID(sessionID), pgtype.Timestamptz{}, pgtype.UUID{})
					if err != nil {
						t.Fatal(err)
					}
					for _, item := range work.Issues {
						if item.ID == uuidToString(issue) && (len(item.PendingRequests) != 0 || item.Target.DecisionID != "" || item.State == "needs_you") {
							t.Fatalf("answered decision still solicits an answer: %+v", item)
						}
					}
				}
				var archived bool
				dbfx.QueryRow(t, `SELECT archived FROM inbox_item WHERE id=$1`, otherRequest).Scan(&archived)
				if archived {
					t.Fatal("answering this issue resolved an unrelated question")
				}
				if !stale {
					opposite := "reject"
					if action == "reject" {
						opposite = "approve"
					}
					changed := withChatTestWorkspaceCtx(t, newRequest(http.MethodPost, "/respond", map[string]any{"action": opposite}))
					changed.Header.Set("X-Actor-Source", "task_token")
					changed.Header.Set("X-Agent-ID", agentID)
					changed.Header.Set("X-Task-ID", taskID)
					testutil.Call(t, h.SubmitChatWorkResponse, changed).Want(409)
					var content string
					dbfx.QueryRow(t, `SELECT content::text FROM issue_workflow_decision_response WHERE decision_id=$1`, decisionID).Scan(&content)
					if !strings.Contains(content, action+" this specific version.") {
						t.Fatalf("owner words lost: %s", content)
					}
				}
			})
		}
	}

}

func seedChatDecisionWorkflow(t *testing.T, ctx context.Context, issueID pgtype.UUID) map[string]pgtype.UUID {
	t.Helper()
	workspaceID := parseUUID(testWorkspaceID)
	conn := testPool
	t.Cleanup(func() {
		for _, table := range []string{"issue_workflow_target_head", "issue_current_picture", "issue_workflow_concern_head", "issue_workflow_decision_response", "issue_workflow_decision", "issue_workflow_action_receipt", "issue_workflow_command", "issue_workflow_outcome", "issue_workflow_scope", "issue_workflow_authority", "issue_workflow_execution", "issue_workflow_candidate", "issue_workflow_review", "issue_workflow_release", "issue_workflow_attention", "issue_workflow_evidence", "issue_workflow_budget"} {
			dbfx.Exec(t, "DELETE FROM "+table+" WHERE issue_id=$1", issueID)
		}
	})
	fresh := func(_ byte) pgtype.UUID { return parseUUID(uuid.NewString()) }
	base := byte(0)
	ids := map[string]pgtype.UUID{
		"outcome": fresh(base + 2), "scope": fresh(base + 3), "authority": fresh(base + 4),
		"execution": fresh(base + 5), "review": fresh(base + 6), "release": fresh(base + 7),
		"attention": fresh(base + 8), "evidence": fresh(base + 9), "budget": fresh(base + 10),
		"candidate": fresh(base + 11), "owner": parseUUID(testUserID), "implementer": fresh(base + 13),
		"reviewer": fresh(base + 14), "operator": fresh(base + 15), "verifier": fresh(base + 16),
		"review_evidence": fresh(base + 17),
	}
	statements := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO issue_workflow_outcome(id,workspace_id,issue_id,state,acceptance_version) VALUES($1,$2,$3,'open',1)`, []any{ids["outcome"], workspaceID, issueID}},
		{`INSERT INTO issue_workflow_scope(id,workspace_id,issue_id,revision,acceptance_version,state,owner_evidence_id,created_by,statement) VALUES($1,$2,$3,1,1,'accepted',$4,$4,'{}')`, []any{ids["scope"], workspaceID, issueID, ids["owner"]}},
		{`INSERT INTO issue_workflow_authority(id,workspace_id,issue_id,revision,scope_id,scope_revision,owner_evidence_id,state,envelope,created_by) VALUES($1,$2,$3,1,$4,1,$5,'active',$6,$5)`, []any{ids["authority"], workspaceID, issueID, ids["scope"], ids["owner"], []byte(`{"target_id":"production"}`)}},
		{`INSERT INTO issue_workflow_execution(id,workspace_id,issue_id,branch_key,operation_type,state,scope_id,scope_revision,authority_id,authority_revision,actor_id,checkpoint,expected_event,wait_reason) VALUES($1,$2,$3,'main','implementation','working',$4,1,$5,1,$6,'{}','{}','{}')`, []any{ids["execution"], workspaceID, issueID, ids["scope"], ids["authority"], ids["implementer"]}},
		{`INSERT INTO issue_workflow_candidate(id,workspace_id,issue_id,scope_id,scope_revision,candidate_type,state,artifact,build_contract,created_by) VALUES($1,$2,$3,$4,1,'source','ready','{}','{}',$5)`, []any{ids["candidate"], workspaceID, issueID, ids["scope"], ids["implementer"]}},
		{`INSERT INTO issue_workflow_review(id,workspace_id,issue_id,candidate_id,candidate_revision,scope_id,scope_revision,reviewer_id,outcome,criteria,coverage,evidence_refs) VALUES($1,$2,$3,$4,1,$5,1,$6,'passed','["acceptance"]','{"tests":true}',jsonb_build_array($7::text))`, []any{ids["review"], workspaceID, issueID, ids["candidate"], ids["scope"], ids["reviewer"], ids["review_evidence"]}},
		{`INSERT INTO issue_workflow_release(id,workspace_id,issue_id,candidate_id,candidate_revision,scope_id,scope_revision,authority_id,authority_revision,target_key,target_generation,state,facets) VALUES($1,$2,$3,$4,1,$5,1,$6,1,'production',1,'deployed',jsonb_build_object('release_operator_id',$7::text,'planned_verifier_id',$8::text))`, []any{ids["release"], workspaceID, issueID, ids["candidate"], ids["scope"], ids["authority"], ids["operator"], ids["verifier"]}},
		{`INSERT INTO issue_workflow_target_head(workspace_id,target_key,generation,release_id,issue_id,candidate_id,state) VALUES($1,'production',1,$2,$3,$4,'deployed')`, []any{workspaceID, ids["release"], issueID, ids["candidate"]}},
		{`INSERT INTO issue_workflow_attention(id,workspace_id,issue_id,attention_type,read_state,resolution_state) VALUES($1,$2,$3,'workflow','read','resolved')`, []any{ids["attention"], workspaceID, issueID}},
		{`INSERT INTO issue_workflow_evidence(id,workspace_id,issue_id,concern_type,evidence_type,level,source,producer_type,producer_id,candidate_id,target_key,observed_at,payload) VALUES($1,$2,$3,'release','runtime','runtime','test','verifier',$4,$5,'production','2026-09-10T11:00:00Z',jsonb_build_object('freshness','current','candidate_revision',1,'release_id',$6::text,'target_generation',1,'acceptance_version',1,'scope_id',$7::text,'scope_revision',1))`, []any{ids["evidence"], workspaceID, issueID, ids["verifier"], ids["candidate"], ids["release"], ids["scope"]}},
		{`INSERT INTO issue_workflow_evidence(id,workspace_id,issue_id,concern_type,evidence_type,level,source,producer_type,producer_id,candidate_id,observed_at,payload) VALUES($1,$2,$3,'review','technical_review','technical','review-task','reviewer',$4,$5,'2026-09-10T11:00:00Z',jsonb_build_object('freshness','current','candidate_revision',1,'scope_id',$6::text,'scope_revision',1))`, []any{ids["review_evidence"], workspaceID, issueID, ids["reviewer"], ids["candidate"], ids["scope"]}},
		{`INSERT INTO issue_workflow_budget(id,workspace_id,issue_id,scope_id,concern_type,state,limits,consumption) VALUES($1,$2,$3,$4,'execution','active','{"attempts":3}','{"attempts":1}')`, []any{ids["budget"], workspaceID, issueID, ids["scope"]}},
	}
	for _, statement := range statements {
		if _, err := conn.Exec(ctx, statement.sql, statement.args...); err != nil {
			t.Fatalf("seed loader fixture: %v", err)
		}
	}
	states := map[string]string{"outcome": "open", "scope": "accepted", "authority": "active", "execution": "working", "review": "passed", "release": "deployed", "attention": "resolved", "evidence": "current", "budget": "active"}
	for concern, state := range states {
		if _, err := conn.Exec(ctx, `INSERT INTO issue_workflow_concern_head(workspace_id,issue_id,concern_type,revision,source_record_id,source_record_revision,state) VALUES($1,$2,$3,1,$4,1,$5)`, workspaceID, issueID, concern, ids[concern], state); err != nil {
			t.Fatal(err)
		}
	}
	return ids
}
