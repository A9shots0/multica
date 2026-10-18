// chat-gap-fixture creates disposable review scenarios in the isolated local DB.
// It does not dispatch workers, attach providers, or deploy anything.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/workflowcommand"
	"github.com/multica-ai/multica/server/internal/workflowprojection"
	v2 "github.com/multica-ai/multica/server/internal/workflowstate/v2"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"log"
	"net/url"
	"os"
	"strings"
	"time"
)

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
func parseUUID(s string) pgtype.UUID     { var id pgtype.UUID; must(id.Scan(s)); return id }
func uuidToString(id pgtype.UUID) string { return uuid.UUID(id.Bytes).String() }
func main() {
	u, err := url.Parse(os.Getenv("DATABASE_URL"))
	must(err)
	if u.Host != "127.0.0.1:55438" || u.Path != "/multica_review" {
		log.Fatal("Only the isolated local review database is allowed")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, u.String())
	must(err)
	defer pool.Close()
	workspaceID := parseUUID("f3a49167-bfb7-4f48-9a91-d2e09d3e90ef")
	ownerID := parseUUID("7a5af047-36c5-42ab-8b58-24059e42efcb")
	agentID := parseUUID("229f650c-c6e2-4d3c-8926-0d87d3d26048")
	if len(os.Args) == 3 && os.Args[1] == "revise" {
		id := parseUUID(os.Args[2])
		tag, err := pool.Exec(ctx, `UPDATE issue_workflow_decision SET revision=revision+1,question='Approve the revised local preview with larger labels?',updated_at=now() WHERE id=$1 AND workspace_id=$2 AND state='unanswered' AND question LIKE '%local preview%'`, id, workspaceID)
		must(err)
		fmt.Printf("Revised %d disposable decision\n", tag.RowsAffected())
		return
	}
	var sessionID pgtype.UUID
	must(pool.QueryRow(ctx, `INSERT INTO chat_session(workspace_id,agent_id,creator_id,title,explicitly_created_at) VALUES($1,$2,$3,'Multi-issue review checks',now()) RETURNING id`, workspaceID, agentID, ownerID).Scan(&sessionID))
	var issues, decisions []string
	for i, title := range []string{"Review the local preview", "Review the alternate local preview", "Wait for preview assets"} {
		var issueID pgtype.UUID
		must(pool.QueryRow(ctx, `WITH n AS (UPDATE workspace SET issue_counter=issue_counter+1 WHERE id=$1 RETURNING issue_counter) INSERT INTO issue(workspace_id,title,status,creator_type,creator_id,assignee_type,assignee_id,number) SELECT $1,$2,'in_progress','member',$3,'agent',$4,issue_counter FROM n RETURNING id`, workspaceID, title, ownerID, agentID).Scan(&issueID))
		ids := seedWorkflow(ctx, pool, issueID, workspaceID, ownerID, agentID)
		_, err = pool.Exec(ctx, `INSERT INTO chat_issue_link(workspace_id,chat_session_id,issue_id,source) VALUES($1,$2,$3,'manual')`, workspaceID, sessionID, issueID)
		must(err)
		issues = append(issues, uuidToString(issueID))
		if i == 2 {
			_, err = pool.Exec(ctx, `UPDATE issue_workflow_execution SET state='blocked_external',wait_reason='{"description":"Waiting for the approved image assets"}',expected_event='{"type":"provider_readback"}' WHERE issue_id=$1`, issueID)
			must(err)
			_, err = pool.Exec(ctx, `UPDATE issue_workflow_concern_head SET state='blocked_external' WHERE issue_id=$1 AND concern_type='execution'`, issueID)
			must(err)
			_, err = workflowprojection.ProjectAndStore(ctx, pool, db.New(pool), workspaceID, issueID, 0, 1, time.Now())
			must(err)
		} else {
			controller := parseUUID(uuid.NewString())
			svc := workflowcommand.Service{Starter: pool, Queries: db.New(pool), Loader: workflowcommand.Loader{Now: time.Now}, Now: time.Now, ControllerID: controller}
			_, err = svc.Execute(ctx, workflowcommand.Principal{ActorID: controller, ActorType: "internal_controller"}, workflowcommand.Request{WorkspaceID: workspaceID, IssueID: issueID, CommandKey: "local-review-" + uuidToString(issueID), Kind: v2.CommandRequestDecision, OperationID: uuidToString(ids["execution"]), CandidateID: uuidToString(ids["candidate"]), Expected: []v2.ExpectedRevision{{Concern: v2.ConcernExecution, Revision: 1}, {Concern: v2.ConcernAttention, Revision: 1}, {Concern: v2.ConcernEvidence, Revision: 1}}, Decision: &workflowcommand.DecisionInput{Type: v2.DecisionAuthorization, Question: "Approve this local preview?", Recommendation: "Review the labels before choosing.", Consequence: "Records your response for this disposable local check only.", AccountableOwnerID: ownerID, AuthorityGap: v2.AuthorityGap{Kind: v2.AuthorityGapOperationAuthorization, RequestedCapability: "execute proposed operation", Reason: "owner authorization required"}}})
			must(err)
			var decisionID string
			must(pool.QueryRow(ctx, `SELECT id FROM issue_workflow_decision WHERE issue_id=$1`, issueID).Scan(&decisionID))
			decisions = append(decisions, decisionID)
			details, _ := json.Marshal(map[string]string{"decision_id": decisionID, "question": "Approve this local preview?", "owner_relevant": "true"})
			_, err = pool.Exec(ctx, `INSERT INTO inbox_item(workspace_id,recipient_type,recipient_id,type,severity,title,body,issue_id,details) VALUES($1,'member',$2,'workflow_owner_decision','action_required',$3,'Approve this local preview?',$4,$5)`, workspaceID, ownerID, title, issueID, details)
			must(err)
		}
	}
	for i := 0; i < 60; i++ {
		_, err = pool.Exec(ctx, `INSERT INTO issue_workflow_event(id,workspace_id,issue_id,source_task_id,actor_id,event_type,reason,decision,created_at) VALUES($1,$2,$3,$4,$5,'fixture.review',$6,'{}',now()-$7::int*interval '1 minute')`, uuid.NewString(), workspaceID, parseUUID(issues[2]), uuid.NewString(), agentID, fmt.Sprintf("Local review checkpoint %02d", i+1), i+1)
		must(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO chat_message(chat_session_id,role,content,message_kind) VALUES($1,'assistant','This conversation contains disposable local test records: two independent review requests, one dependency, and older work history. These records do not represent deployed work.','message')`, sessionID)
	must(err)
	result, _ := json.Marshal(map[string]any{"session": uuidToString(sessionID), "issues": issues, "decisions": decisions})
	fmt.Println(string(result))
}

func seedWorkflow(ctx context.Context, conn *pgxpool.Pool, issueID, workspaceID, ownerID, agentID pgtype.UUID) map[string]pgtype.UUID {
	fresh := func(_ byte) pgtype.UUID { return parseUUID(uuid.NewString()) }
	base := byte(0)
	ids := map[string]pgtype.UUID{
		"outcome": fresh(base + 2), "scope": fresh(base + 3), "authority": fresh(base + 4),
		"execution": fresh(base + 5), "review": fresh(base + 6), "release": fresh(base + 7),
		"attention": fresh(base + 8), "evidence": fresh(base + 9), "budget": fresh(base + 10),
		"candidate": fresh(base + 11), "owner": ownerID, "implementer": agentID,
		"reviewer": fresh(base + 14), "operator": fresh(base + 15), "verifier": fresh(base + 16),
		"review_evidence": fresh(base + 17),
	}
	statements := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO issue_workflow_outcome(id,workspace_id,issue_id,state,acceptance_version) VALUES($1,$2,$3,'open',1)`, []any{ids["outcome"], workspaceID, issueID}},
		{`INSERT INTO issue_workflow_scope(id,workspace_id,issue_id,revision,acceptance_version,state,owner_evidence_id,created_by,statement) VALUES($1,$2,$3,1,1,'accepted',$4,$4,'{}')`, []any{ids["scope"], workspaceID, issueID, ids["owner"]}},
		{`INSERT INTO issue_workflow_authority(id,workspace_id,issue_id,revision,scope_id,scope_revision,owner_evidence_id,state,envelope,created_by) VALUES($1,$2,$3,1,$4,1,$5,'active',$6,$5)`, []any{ids["authority"], workspaceID, issueID, ids["scope"], ids["owner"], []byte(`{"target_id":"local-preview-` + uuidToString(issueID) + `"}`)}},
		{`INSERT INTO issue_workflow_execution(id,workspace_id,issue_id,branch_key,operation_type,state,scope_id,scope_revision,authority_id,authority_revision,actor_id,actor_type,checkpoint,expected_event,wait_reason) VALUES($1,$2,$3,'main','implementation','working',$4,1,$5,1,$6,'agent','{}','{}','{}')`, []any{ids["execution"], workspaceID, issueID, ids["scope"], ids["authority"], ids["implementer"]}},
		{`INSERT INTO issue_workflow_candidate(id,workspace_id,issue_id,scope_id,scope_revision,candidate_type,state,artifact,build_contract,created_by) VALUES($1,$2,$3,$4,1,'source','ready','{}','{}',$5)`, []any{ids["candidate"], workspaceID, issueID, ids["scope"], ids["implementer"]}},
		{`INSERT INTO issue_workflow_review(id,workspace_id,issue_id,candidate_id,candidate_revision,scope_id,scope_revision,reviewer_id,outcome,criteria,coverage,evidence_refs) VALUES($1,$2,$3,$4,1,$5,1,$6,'passed','["acceptance"]','{"tests":true}',jsonb_build_array($7::text))`, []any{ids["review"], workspaceID, issueID, ids["candidate"], ids["scope"], ids["reviewer"], ids["review_evidence"]}},
		{`INSERT INTO issue_workflow_release(id,workspace_id,issue_id,candidate_id,candidate_revision,scope_id,scope_revision,authority_id,authority_revision,target_key,target_generation,state,facets) VALUES($1,$2,$3,$4,1,$5,1,$6,1,'local-preview',1,'deployed',jsonb_build_object('release_operator_id',$7::text,'planned_verifier_id',$8::text))`, []any{ids["release"], workspaceID, issueID, ids["candidate"], ids["scope"], ids["authority"], ids["operator"], ids["verifier"]}},
		{`INSERT INTO issue_workflow_target_head(workspace_id,target_key,generation,release_id,issue_id,candidate_id,state) VALUES($1,'local-preview',1,$2,$3,$4,'deployed')`, []any{workspaceID, ids["release"], issueID, ids["candidate"]}},
		{`INSERT INTO issue_workflow_attention(id,workspace_id,issue_id,attention_type,read_state,resolution_state) VALUES($1,$2,$3,'workflow','read','resolved')`, []any{ids["attention"], workspaceID, issueID}},
		{`INSERT INTO issue_workflow_evidence(id,workspace_id,issue_id,concern_type,evidence_type,level,source,producer_type,producer_id,candidate_id,target_key,observed_at,payload) VALUES($1,$2,$3,'release','runtime','runtime','test','verifier',$4,$5,'local-preview',now(),jsonb_build_object('freshness','current','candidate_revision',1,'release_id',$6::text,'target_generation',1,'acceptance_version',1,'scope_id',$7::text,'scope_revision',1))`, []any{ids["evidence"], workspaceID, issueID, ids["verifier"], ids["candidate"], ids["release"], ids["scope"]}},
		{`INSERT INTO issue_workflow_evidence(id,workspace_id,issue_id,concern_type,evidence_type,level,source,producer_type,producer_id,candidate_id,observed_at,payload) VALUES($1,$2,$3,'review','technical_review','technical','review-task','reviewer',$4,$5,now(),jsonb_build_object('freshness','current','candidate_revision',1,'scope_id',$6::text,'scope_revision',1))`, []any{ids["review_evidence"], workspaceID, issueID, ids["reviewer"], ids["candidate"], ids["scope"]}},
		{`INSERT INTO issue_workflow_budget(id,workspace_id,issue_id,scope_id,concern_type,state,limits,consumption) VALUES($1,$2,$3,$4,'execution','active','{"attempts":3}','{"attempts":1}')`, []any{ids["budget"], workspaceID, issueID, ids["scope"]}},
	}
	for _, statement := range statements {
		if _, err := conn.Exec(ctx, strings.ReplaceAll(statement.sql, "local-preview", "local-preview-"+uuidToString(issueID)), statement.args...); err != nil {
			log.Fatal(err)
		}
	}
	states := map[string]string{"outcome": "open", "scope": "accepted", "authority": "active", "execution": "working", "review": "passed", "release": "deployed", "attention": "resolved", "evidence": "current", "budget": "active"}
	for concern, state := range states {
		if _, err := conn.Exec(ctx, `INSERT INTO issue_workflow_concern_head(workspace_id,issue_id,concern_type,revision,source_record_id,source_record_revision,state) VALUES($1,$2,$3,1,$4,1,$5)`, workspaceID, issueID, concern, ids[concern], state); err != nil {
			log.Fatal(err)
		}
	}
	return ids
}
