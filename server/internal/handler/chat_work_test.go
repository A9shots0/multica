package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestChatWorkThreeIssuesAndMikaAnswer(t *testing.T) {
	ctx := context.Background()
	question := newOrdinaryQuestionFixture(t)
	reported := question.report(question.issue, ownerDecisionPacket())
	if reported.Code != 201 {
		t.Fatalf("question: %s", reported.Body)
	}
	dbfx.Exec(t, `UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1`, question.task)
	agentID, sessionID, runtimeID, _ := setupDirectChatSession(t, ctx, "Three linked issues")
	running := dbfx.Issue(t, "Audit progressing")
	waiting := dbfx.Issue(t, "Waiting for local access")
	dbfx.Exec(t, `UPDATE agent_runtime SET status='online',last_seen_at=now() WHERE id=$1`, runtimeID)
	dbfx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "issue_id": running, "status": "running", "started_at": testutil.Raw("now()")})
	dbfx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "issue_id": waiting, "status": "waiting_local_directory"})
	for _, id := range []string{running, question.issue, waiting} {
		dbfx.InsertNoID(t, "chat_issue_link", testutil.Cols{"workspace_id": testWorkspaceID, "chat_session_id": sessionID, "issue_id": id, "source": "manual"}, "chat_session_id=$1 AND issue_id=$2", sessionID, id)
	}
	read := withURLParam(withChatTestWorkspaceCtx(t, newRequest("GET", "/work", nil)), "sessionId", sessionID)
	var snapshot service.ChatWork
	testutil.Call(t, testHandler.GetChatWork, read).Want(200).JSON(&snapshot)
	states := map[string]string{}
	for _, issue := range snapshot.Issues {
		states[issue.ID] = issue.State
	}
	if len(states) != 3 || states[running] != "working" || states[question.issue] != "needs_you" || states[waiting] != "waiting" {
		t.Fatalf("states: %v", states)
	}
	if len(snapshot.Requests) != 1 {
		t.Fatalf("Mika must receive canonical question: %+v", snapshot.Requests)
	}
	var requestDetails map[string]string
	if err := json.Unmarshal(snapshot.Requests[0].Details, &requestDetails); err != nil || requestDetails["options"] == "" {
		t.Fatalf("owner request options were not normalized as JSON: %s %v", snapshot.Requests[0].Details, err)
	}
	session, _ := testHandler.Queries.GetChatSession(ctx, parseUUID(sessionID))
	agent, _ := testHandler.Queries.GetAgent(ctx, parseUUID(agentID))
	target, err := service.CaptureChatWorkTarget(ctx, testHandler.Queries, session, parseUUID(testUserID), parseUUID(question.issue))
	if err != nil {
		t.Fatal(err)
	}
	sent, err := testHandler.TaskService.SendDirectChatMessage(ctx, session, agent, parseUUID(testUserID), "Use A; keep the preview compact.", nil, "member", parseUUID(testUserID), "", target)
	if err != nil {
		t.Fatal(err)
	}
	taskID := uuidToString(sent.Task.ID)
	markTaskRunning(t, ctx, taskID)
	request := func(method, path string, body any) *http.Request {
		r := withChatTestWorkspaceCtx(t, newRequest(method, path, body))
		r.Header.Set("X-Actor-Source", "task_token")
		r.Header.Set("X-Task-ID", taskID)
		r.Header.Set("X-Agent-ID", agentID)
		return r
	}
	var agentWork service.ChatWork
	testutil.Call(t, testHandler.GetCurrentChatWork, request("GET", "/api/chat/work", nil)).Want(200).JSON(&agentWork)
	if len(agentWork.Issues) != 3 || len(agentWork.Requests) != 1 {
		t.Fatalf("UI/Mika scope differs: %+v", agentWork)
	}
	for attempt := 0; attempt < 2; attempt++ {
		testutil.Call(t, testHandler.SubmitChatWorkResponse, request("POST", "/api/chat/work/respond", map[string]any{"action": "answer"})).WantOneOf(200, 201)
	}
	var count int
	dbfx.QueryRow(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND status='queued'`, question.issue).Scan(&count)
	if count != 1 {
		t.Fatalf("answer must enqueue exactly once: %d", count)
	}
	items, err := testHandler.Queries.ListInboxItems(ctx, db.ListInboxItemsParams{WorkspaceID: parseUUID(testWorkspaceID), RecipientType: "member", RecipientID: parseUUID(testUserID)})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.IssueID == parseUUID(question.issue) && item.Type == "workflow_owner_decision" {
			t.Fatal("answered question remains in Inbox")
		}
	}
	dbfx.Exec(t, `UPDATE agent_runtime SET last_seen_at=now()-interval '10 minutes' WHERE id=$1`, runtimeID)
	fresh, err := service.LoadChatWork(ctx, testHandler.Queries, parseUUID(testWorkspaceID), parseUUID(sessionID), pgtype.Timestamptz{}, pgtype.UUID{})
	if err != nil {
		t.Fatal(err)
	}
	for _, issue := range fresh.Issues {
		if issue.ID == running && issue.State != "unavailable" {
			t.Fatal("stale runtime still claims working")
		}
	}
	// A token cannot select a second issue or invent an answer via JSON.
	testutil.Call(t, testHandler.SubmitChatWorkResponse, request("POST", "/respond", map[string]any{"action": "cancel", "issue_id": running})).Want(400)
}

func TestChatWorkRequiresExplicitOwnerTarget(t *testing.T) {
	ctx := context.Background()
	agent, session, _, _ := setupDirectChatSession(t, ctx, "Brainstorm")
	task := sendDirectChat(t, ctx, agent, session, "That sounds interesting.")
	markTaskRunning(t, ctx, task)
	r := withChatTestWorkspaceCtx(t, newRequest("POST", "/respond", map[string]any{"action": "approve"}))
	r.Header.Set("X-Actor-Source", "task_token")
	r.Header.Set("X-Agent-ID", agent)
	r.Header.Set("X-Task-ID", task)
	testutil.Call(t, testHandler.SubmitChatWorkResponse, r).Want(409)
}

func TestChatWorkMultipleQuestionsRequireExplicitSelection(t *testing.T) {
	ctx := context.Background()
	agentID, sessionID, _, _ := setupDirectChatSession(t, ctx, "Two questions")
	issueID := dbfx.Issue(t, "Two independent decisions")
	dbfx.InsertNoID(t, "chat_issue_link", testutil.Cols{"workspace_id": testWorkspaceID, "chat_session_id": sessionID, "issue_id": issueID, "source": "manual"}, "chat_session_id=$1 AND issue_id=$2", sessionID, issueID)
	var requests []string
	for _, question := range []string{"Which layout?", "Which deadline?"} {
		requests = append(requests, dbfx.Insert(t, "inbox_item", testutil.Cols{"workspace_id": testWorkspaceID, "recipient_type": "member", "recipient_id": testUserID, "type": "workflow_owner_decision", "severity": "action_required", "title": question, "body": question, "issue_id": issueID, "details": `{"question_scope":"ordinary_task","intake_state":"pending"}`}))
	}
	session, _ := testHandler.Queries.GetChatSession(ctx, parseUUID(sessionID))
	agent, _ := testHandler.Queries.GetAgent(ctx, parseUUID(agentID))
	target, err := service.CaptureChatWorkTarget(ctx, testHandler.Queries, session, parseUUID(testUserID), parseUUID(issueID))
	if err != nil || !target.NeedsRequestSelection || target.IntakeID != "" {
		t.Fatalf("silently selected a question: %+v %v", target, err)
	}
	sent, err := testHandler.TaskService.SendDirectChatMessage(ctx, session, agent, parseUUID(testUserID), "Let's discuss both questions first.", nil, "member", parseUUID(testUserID), "", target)
	if err != nil {
		t.Fatal(err)
	}
	taskID := uuidToString(sent.Task.ID)
	markTaskRunning(t, ctx, taskID)
	r := withChatTestWorkspaceCtx(t, newRequest("POST", "/respond", map[string]any{"action": "answer"}))
	r.Header.Set("X-Actor-Source", "task_token")
	r.Header.Set("X-Agent-ID", agentID)
	r.Header.Set("X-Task-ID", taskID)
	testutil.Call(t, testHandler.SubmitChatWorkResponse, r).Want(409)
	work, err := service.LoadChatWork(ctx, testHandler.Queries, parseUUID(testWorkspaceID), parseUUID(sessionID), pgtype.Timestamptz{}, pgtype.UUID{})
	if err != nil {
		t.Fatal(err)
	}
	if len(work.Issues) != 1 || len(work.Issues[0].PendingRequests) != 2 {
		t.Fatalf("missing individual choices: %+v", work)
	}
	for _, request := range work.Issues[0].PendingRequests {
		if request.Target.NeedsRequestSelection || request.Target.IntakeID != request.ID || request.Target.RequestID != request.ID {
			t.Fatalf("wrong specific target: %+v", request)
		}
	}
	selected, err := service.CaptureChatWorkTarget(ctx, testHandler.Queries, session, parseUUID(testUserID), parseUUID(issueID), requests[1])
	if err != nil || selected.IntakeID != requests[1] {
		t.Fatalf("selection: %+v %v", selected, err)
	}
	dbfx.Exec(t, "UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1", taskID)
	_, err = testHandler.TaskService.SendDirectChatMessage(ctx, session, agent, parseUUID(testUserID), "Choose tomorrow for the deadline.", nil, "member", parseUUID(testUserID), "", selected)
	if err != nil {
		t.Fatalf("explicit question cannot be sent: %v", err)
	}
	dbfx.Exec(t, "UPDATE inbox_item SET archived=true WHERE id=$1", requests[1])
	_, err = service.CaptureChatWorkTarget(ctx, testHandler.Queries, session, parseUUID(testUserID), parseUUID(issueID), requests[1])
	if !errors.Is(err, service.ErrChatWorkTargetChanged) {
		t.Fatalf("resolved selection accepted: %v", err)
	}
	_, err = testHandler.TaskService.SendDirectChatMessage(ctx, session, agent, parseUUID(testUserID), "Choose tomorrow.", nil, "member", parseUUID(testUserID), "", selected)
	if !errors.Is(err, service.ErrChatWorkTargetChanged) {
		t.Fatalf("stale question send accepted: %v", err)
	}
}

func TestChatWorkCompoundResponsesPreserveIndependentTargets(t *testing.T) {
	ctx := context.Background()
	agentID, sessionID, _, _ := setupDirectChatSession(t, ctx, "Two answers in one turn")
	session, _ := testHandler.Queries.GetChatSession(ctx, parseUUID(sessionID))
	agent, _ := testHandler.Queries.GetAgent(ctx, parseUUID(agentID))

	type questionFixture struct {
		issueID   string
		requestID string
		target    service.ChatWorkTarget
		answer    string
	}
	questions := make([]questionFixture, 0, 2)
	for i, values := range []struct{ title, answer string }{
		{"Checkout spacing", "Do one more mobile-spacing pass"},
		{"Profile action", "Move the action below the metrics"},
	} {
		question := newOrdinaryQuestionFixture(t)
		issueID := question.issue
		dbfx.Exec(t, "UPDATE issue SET title=$2 WHERE id=$1", issueID, values.title)
		dbfx.InsertNoID(t, "chat_issue_link", testutil.Cols{"workspace_id": testWorkspaceID, "chat_session_id": sessionID, "issue_id": issueID, "source": "manual"}, "chat_session_id=$1 AND issue_id=$2", sessionID, issueID)
		packet := ownerDecisionPacket()
		packet["question"] = values.title
		reported := question.report(issueID, packet)
		if reported.Code != http.StatusCreated {
			t.Fatalf("question %d: %s", i, reported.Body)
		}
		var receipt struct {
			InboxItemID string `json:"inbox_item_id"`
		}
		if err := json.Unmarshal(reported.Body.Bytes(), &receipt); err != nil || receipt.InboxItemID == "" {
			t.Fatalf("question %d receipt: %s, %v", i, reported.Body, err)
		}
		requestID := receipt.InboxItemID
		dbfx.Exec(t, `UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1`, question.task)
		target, err := service.CaptureChatWorkTarget(ctx, testHandler.Queries, session, parseUUID(testUserID), parseUUID(issueID), requestID)
		if err != nil {
			t.Fatalf("capture target %d: %v", i, err)
		}
		questions = append(questions, questionFixture{issueID: issueID, requestID: requestID, target: target, answer: values.answer})
	}

	duplicate := []service.ChatWorkResponseInput{
		{Target: questions[0].target, Content: questions[0].answer},
		{Target: questions[0].target, Content: "A conflicting second answer"},
	}
	if _, err := testHandler.TaskService.SendDirectChatMessageWithOptions(ctx, session, agent, parseUUID(testUserID), "Duplicate", nil, "member", parseUUID(testUserID), "", service.DirectChatSendOptions{WorkResponses: duplicate}); err == nil {
		t.Fatal("duplicate target was accepted in one turn")
	}
	var rejectedMessageCount int
	dbfx.QueryRow(t, "SELECT count(*) FROM chat_message WHERE chat_session_id=$1", sessionID).Scan(&rejectedMessageCount)
	if rejectedMessageCount != 0 {
		t.Fatalf("rejected compound response saved %d messages", rejectedMessageCount)
	}

	inputs := []service.ChatWorkResponseInput{
		{Target: questions[0].target, Content: questions[0].answer},
		{Target: questions[1].target, Content: questions[1].answer},
	}
	request := withChatTestWorkspaceCtx(t, newRequest("POST", "/api/chat-sessions/"+sessionID+"/messages", map[string]any{
		"content":        "Checkout spacing: Do one more mobile-spacing pass\nProfile action: Move the action below the metrics",
		"work_responses": inputs,
	}))
	request = withURLParam(request, "sessionId", sessionID)
	var sent SendChatMessageResponse
	testutil.Call(t, testHandler.SendChatMessage, request).Want(201).JSON(&sent)
	stored, err := testHandler.Queries.GetAgentTask(ctx, parseUUID(sent.TaskID))
	if err != nil {
		t.Fatal(err)
	}
	var taskContext struct {
		Responses []service.ChatWorkResponse `json:"chat_work_responses"`
	}
	if err := json.Unmarshal(stored.Context, &taskContext); err != nil {
		t.Fatal(err)
	}
	if len(taskContext.Responses) != 2 {
		t.Fatalf("stored responses: %+v", taskContext.Responses)
	}
	for i, response := range taskContext.Responses {
		if response.Target != questions[i].target || response.Content != questions[i].answer || response.ResponseID != service.ChatWorkResponseID(questions[i].target) {
			t.Fatalf("response %d lost its target: %+v", i, response)
		}
	}

	taskID := sent.TaskID
	markTaskRunning(t, ctx, taskID)
	for i, response := range taskContext.Responses {
		r := withChatTestWorkspaceCtx(t, newRequest("POST", "/api/chat/work/respond", map[string]any{"action": "answer", "response_id": response.ResponseID}))
		r.Header.Set("X-Actor-Source", "task_token")
		r.Header.Set("X-Task-ID", taskID)
		r.Header.Set("X-Agent-ID", agentID)
		testutil.Call(t, testHandler.SubmitChatWorkResponse, r).WantOneOf(200, 201)

		var count int
		dbfx.QueryRow(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND status='queued'`, questions[i].issueID).Scan(&count)
		if count != 1 {
			t.Fatalf("response %d did not independently continue its issue: %d", i, count)
		}
	}
}

func TestChatWorkRejectsChangedTargetWithoutSavingMessage(t *testing.T) {
	ctx := context.Background()
	agentID, sessionID, _, _ := setupDirectChatSession(t, ctx, "Stale target")
	issueID := dbfx.Issue(t, "Target")
	dbfx.InsertNoID(t, "chat_issue_link", testutil.Cols{"workspace_id": testWorkspaceID, "chat_session_id": sessionID, "issue_id": issueID, "source": "manual"}, "chat_session_id=$1 AND issue_id=$2", sessionID, issueID)
	session, _ := testHandler.Queries.GetChatSession(ctx, parseUUID(sessionID))
	agent, _ := testHandler.Queries.GetAgent(ctx, parseUUID(agentID))
	target, err := service.CaptureChatWorkTarget(ctx, testHandler.Queries, session, parseUUID(testUserID), parseUUID(issueID))
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Exec(t, "UPDATE issue SET revision=revision+1 WHERE id=$1", issueID)
	_, err = testHandler.TaskService.SendDirectChatMessage(ctx, session, agent, parseUUID(testUserID), "Cancel this issue.", nil, "member", parseUUID(testUserID), "", target)
	if !errors.Is(err, service.ErrChatWorkTargetChanged) {
		t.Fatalf("stale target: %v", err)
	}
	var count int
	dbfx.QueryRow(t, "SELECT count(*) FROM chat_message WHERE chat_session_id=$1", sessionID).Scan(&count)
	if count != 0 {
		t.Fatalf("stale send saved %d messages", count)
	}
}

func TestChatWorkAgentCreateLinksOriginAndChild(t *testing.T) {
	restoreMemberOnlyIssueCreationDisabled(t)
	ctx := context.Background()
	agent, session, _, _ := setupDirectChatSession(t, ctx, "Origin links")
	task := sendDirectChat(t, ctx, agent, session, "Audit and split the authorized work.")
	markTaskRunning(t, ctx, task)
	create := func(parent string) string {
		body := map[string]any{"title": "Linked audit"}
		if parent != "" {
			body["parent_issue_id"] = parent
		}
		req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, body)
		req.Header.Set("X-Agent-ID", agent)
		req.Header.Set("X-Task-ID", task)
		var result IssueResponse
		testutil.Call(t, testHandler.CreateIssue, req).Want(201).JSON(&result)
		t.Cleanup(func() {
			testutil.Call(t, testHandler.DeleteIssue, withURLParam(newRequest("DELETE", "/api/issues/"+result.ID, nil), "id", result.ID)).Want(204)
		})
		return result.ID
	}
	parent := create("")
	child := create(parent)
	links, err := testHandler.Queries.ListIssuesForChat(ctx, db.ListIssuesForChatParams{WorkspaceID: parseUUID(testWorkspaceID), ChatSessionID: parseUUID(session)})
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, link := range links {
		found[uuidToString(link.ID)] = true
	}
	if !found[parent] || !found[child] || len(found) != 2 {
		t.Fatalf("origin links: %v", found)
	}
}

func TestChatWorkResponseGuards(t *testing.T) {
	for _, scenario := range []string{"wrong_agent", "unlinked", "stale_after_send", "changed_owner", "cancel_replay", "shared_runtime"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			agentID, sessionID, _, _ := setupDirectChatSession(t, ctx, scenario)
			issueID := dbfx.Issue(t, "Scoped target")
			dbfx.InsertNoID(t, "chat_issue_link", testutil.Cols{"workspace_id": testWorkspaceID, "chat_session_id": sessionID, "issue_id": issueID, "source": "manual"}, "chat_session_id=$1 AND issue_id=$2", sessionID, issueID)
			session, _ := testHandler.Queries.GetChatSession(ctx, parseUUID(sessionID))
			agent, _ := testHandler.Queries.GetAgent(ctx, parseUUID(agentID))
			target, err := service.CaptureChatWorkTarget(ctx, testHandler.Queries, session, parseUUID(testUserID), parseUUID(issueID))
			if err != nil {
				t.Fatal(err)
			}
			sent, err := testHandler.TaskService.SendDirectChatMessage(ctx, session, agent, parseUUID(testUserID), "Cancel this issue.", nil, "member", parseUUID(testUserID), "", target)
			if err != nil {
				t.Fatal(err)
			}
			taskID := uuidToString(sent.Task.ID)
			markTaskRunning(t, ctx, taskID)
			switch scenario {
			case "unlinked":
				dbfx.Exec(t, "DELETE FROM chat_issue_link WHERE chat_session_id=$1 AND issue_id=$2", sessionID, issueID)
			case "stale_after_send":
				dbfx.Exec(t, "UPDATE issue SET revision=revision+1 WHERE id=$1", issueID)
			case "changed_owner":
				other := dbfx.User(t, "Other chat owner", "other-chat-owner@example.test")
				dbfx.Exec(t, "UPDATE chat_session SET creator_id=$2 WHERE id=$1", sessionID, other)
			}
			submit := func(action string) *testutil.Response {
				r := withChatTestWorkspaceCtx(t, newRequest("POST", "/respond", map[string]any{"action": action}))
				r.Header.Set("X-Actor-Source", "task_token")
				r.Header.Set("X-Agent-ID", agentID)
				r.Header.Set("X-Task-ID", taskID)
				if scenario == "shared_runtime" {
					r.Header.Set("X-User-ID", agentID)
				}
				if scenario == "wrong_agent" {
					r.Header.Set("X-Agent-ID", testUserID)
				}
				return testutil.Call(t, testHandler.SubmitChatWorkResponse, r)
			}
			first := submit("cancel")
			if scenario == "cancel_replay" || scenario == "shared_runtime" {
				if first.Code != 201 {
					t.Fatalf("cancel: %d %s", first.Code, first.Body)
				}
				repeat := submit("cancel")
				if repeat.Code != 200 {
					t.Fatalf("replay: %d %s", repeat.Code, repeat.Body)
				}
				changed := submit("correct")
				if changed.Code != 409 {
					t.Fatalf("changed replay: %d", changed.Code)
				}
				var count int
				dbfx.QueryRow(t, "SELECT count(*) FROM issue_owner_directive WHERE issue_id=$1", issueID).Scan(&count)
				if count != 1 {
					t.Fatalf("duplicate directives: %d", count)
				}
			} else if first.Code < 400 || first.Code >= 500 {
				t.Fatalf("guard: %d %s", first.Code, first.Body)
			}
		})
	}
}
