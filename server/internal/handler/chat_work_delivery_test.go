package handler

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func linkDeliveryIssue(t *testing.T, session, issue string) {
	t.Helper()
	dbfx.InsertNoID(t, "chat_issue_link", testutil.Cols{"workspace_id": testWorkspaceID, "chat_session_id": session, "issue_id": issue, "source": "origin"}, "chat_session_id=$1 AND issue_id=$2", session, issue)
}

func deliverWork(t *testing.T) {
	t.Helper()
	if _, err := service.DeliverChatWorkUpdates(context.Background(), testHandler.Queries, nil); err != nil {
		t.Fatal(err)
	}
}

func deliveryMessages(t *testing.T, session string) []db.ChatMessage {
	t.Helper()
	rows, err := testHandler.Queries.ListChatMessages(context.Background(), parseUUID(session))
	if err != nil {
		t.Fatal(err)
	}
	out := []db.ChatMessage{}
	for _, row := range rows {
		if row.MessageKind == "work_update" {
			out = append(out, row)
		}
	}
	return out
}

func TestChatWorkDeliveryDurableReplayAndUnlink(t *testing.T) {
	ctx := context.Background()
	agent, session, runtime, _ := setupDirectChatSession(t, ctx, "Automatic work updates")
	issue := dbfx.Issue(t, "Review result delivery")
	linkDeliveryIssue(t, session, issue)
	task := dbfx.Task(t, agent, testutil.Cols{"issue_id": issue, "runtime_id": runtime, "status": "running", "started_at": testutil.Raw("now()")})
	deliverWork(t)
	rows := deliveryMessages(t, session)
	if len(rows) != 1 {
		t.Fatalf("start not delivered: %d", len(rows))
	}
	if rows[0].TaskID.Valid {
		t.Fatal("worker event must not own a direct-chat turn")
	}
	dbfx.Exec(t, `UPDATE agent_task_queue SET status='completed',completed_at=now(),result='{"output":"The mobile preview is ready for review."}' WHERE id=$1`, task)
	// Simulate recovery after a closed browser/server restart, including two
	// concurrent reconcilers. Neither a UI request nor a model call is involved.
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := service.DeliverChatWorkUpdates(ctx, testHandler.Queries, nil)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	deliverWork(t)
	rows = deliveryMessages(t, session)
	if len(rows) != 2 || !strings.Contains(rows[1].Content, "mobile preview") {
		t.Fatalf("lost/duplicate result: %+v", rows)
	}
	var count int
	dbfx.QueryRow(t, `SELECT count(*) FROM chat_message WHERE chat_session_id=$1 AND role='assistant' AND created_at>(SELECT last_read_at FROM chat_session WHERE id=$1)`, session).Scan(&count)
	if count != 2 {
		t.Fatalf("unread updates = %d", count)
	}
	recent, err := testHandler.Queries.ListRecentChatWorkUpdates(ctx, db.ListRecentChatWorkUpdatesParams{WorkspaceID: parseUUID(testWorkspaceID), ChatSessionID: parseUUID(session)})
	if err != nil || len(recent) != 2 {
		t.Fatalf("Mika missing saved evidence: %v %d", err, len(recent))
	}
	work, err := service.LoadChatWork(ctx, testHandler.Queries, parseUUID(testWorkspaceID), parseUUID(session), pgtype.Timestamptz{}, pgtype.UUID{})
	if err != nil || len(work.Updates) != 2 || work.Updates[0].Kind != "result" || !strings.Contains(work.Updates[0].Body, "mobile preview") {
		t.Fatalf("Mika's shared context lost structured worker evidence: %v %+v", err, work.Updates)
	}
	dbfx.Exec(t, `DELETE FROM chat_issue_link WHERE chat_session_id=$1 AND issue_id=$2`, session, issue)
	if len(deliveryMessages(t, session)) != 0 {
		t.Fatal("unlinked evidence leaked into transcript")
	}
	page, err := testHandler.Queries.ListChatMessagesPage(ctx, db.ListChatMessagesPageParams{ChatSessionID: parseUUID(session), Limit: 50})
	if err != nil || len(page) != 0 {
		t.Fatalf("unlinked evidence leaked into paged history: %v %d", err, len(page))
	}
}

func TestChatWorkDeliveryHidesCheckpointWithoutChangingEvidence(t *testing.T) {
	ctx := context.Background()
	agent, session, runtime, _ := setupDirectChatSession(t, ctx, "Visible wireframes")
	issue := dbfx.Issue(t, "Wireframe checkpoint")
	linkDeliveryIssue(t, session, issue)
	task := dbfx.Task(t, agent, testutil.Cols{"issue_id": issue, "runtime_id": runtime, "status": "completed", "started_at": testutil.Raw("now()"), "completed_at": testutil.Raw("now()")})
	const report = "Two layouts are attached.\n<design-checkpoint>{\"stage\":\"wireframes\",\"status\":\"awaiting_feedback\"}</design-checkpoint>"
	comment := dbfx.Insert(t, "comment", testutil.Cols{"workspace_id": testWorkspaceID, "issue_id": issue, "author_type": "agent", "author_id": agent, "type": "comment", "content": report, "source_task_id": task})
	dbfx.Insert(t, "attachment", testutil.Cols{"workspace_id": testWorkspaceID, "issue_id": issue, "comment_id": comment, "uploader_type": "agent", "uploader_id": agent, "filename": "wireframes.png", "url": "https://cdn.example/wireframes.png", "content_type": "image/png", "size_bytes": 100})
	deliverWork(t)
	rows := deliveryMessages(t, session)
	if len(rows) != 1 || strings.Contains(string(rows[0].WorkUpdate), "design-checkpoint") || strings.Contains(rows[0].Content, "design-checkpoint") {
		t.Fatalf("checkpoint leaked through the normal delivery path: %+v", rows)
	}
	var update service.ChatWorkUpdate
	if err := json.Unmarshal(rows[0].WorkUpdate, &update); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(update.Body, "Two layouts are attached.") || update.Review == nil || len(update.Review.Artifacts) != 1 {
		t.Fatalf("filter lost review text or actual artifacts: %+v", update)
	}
	var stored string
	dbfx.QueryRow(t, `SELECT content FROM comment WHERE id=$1`, comment).Scan(&stored)
	if stored != report {
		t.Fatal("presentation filtering rewrote the completion evidence")
	}
}

func TestHistoricalChatWorkCheckpointReadbackPreservesIdentityAndArtifacts(t *testing.T) {
	ctx := context.Background()
	_, session, _, _ := setupDirectChatSession(t, ctx, "Historical checkpoint")
	issue := dbfx.Issue(t, "Historical wireframes")
	linkDeliveryIssue(t, session, issue)
	const body = "Two layouts are attached.\n<design-checkpoint>{\"stage\":\"wireframes\"}</design-checkpoint>"
	update := service.ChatWorkUpdate{Kind: "result", IssueID: issue, Body: body, Review: &service.ChatWorkReview{SchemaVersion: 1, SourceTaskID: issue, Artifacts: []service.ChatWorkArtifact{{ID: issue, Kind: "image", Label: "wireframes.png", URL: "/wireframes.png", Original: true}}}}
	raw, err := json.Marshal(update)
	if err != nil {
		t.Fatal(err)
	}
	id := dbfx.Insert(t, "chat_message", testutil.Cols{"chat_session_id": session, "role": "assistant", "content": body, "message_kind": "work_update", "work_issue_id": issue, "work_update": string(raw)})
	message, err := testHandler.Queries.GetChatMessage(ctx, parseUUID(id))
	if err != nil {
		t.Fatal(err)
	}
	read := withURLParam(withChatTestWorkspaceCtx(t, newRequest("GET", "/messages", nil)), "sessionId", session)
	var page ChatMessagesPageResponse
	testutil.Call(t, testHandler.ListChatMessagesPage, read).Want(200).JSON(&page)
	if len(page.Messages) != 1 {
		t.Fatalf("historical transcript count = %d", len(page.Messages))
	}
	response := page.Messages[0]
	if response.ID != id || response.Content != "Two layouts are attached." || strings.Contains(string(response.WorkUpdate), "design-checkpoint") {
		t.Fatalf("stored receipt reached transcript: %+v", response)
	}
	var visible service.ChatWorkUpdate
	if err := json.Unmarshal(response.WorkUpdate, &visible); err != nil || visible.Review == nil || len(visible.Review.Artifacts) != 1 || visible.Review.Artifacts[0].ID != issue {
		t.Fatalf("historical review identity was lost: %v %+v", err, visible)
	}
	work, err := service.LoadChatWork(ctx, testHandler.Queries, parseUUID(testWorkspaceID), parseUUID(session), pgtype.Timestamptz{}, pgtype.UUID{})
	if err != nil || len(work.Updates) != 1 || work.Updates[0].Body != "Two layouts are attached." || len(work.CatchUp) != 1 || work.CatchUp[0].Body != "Two layouts are attached." {
		t.Fatalf("shared work/catch-up exposed receipt: %v %+v", err, work)
	}
	stored, err := testHandler.Queries.GetChatMessage(ctx, parseUUID(id))
	if err != nil || string(stored.WorkUpdate) != string(message.WorkUpdate) || stored.Content != body {
		t.Fatal("readback rewrote historical evidence")
	}
}

func TestChatWorkReadbackHandlesMalformedPayloadWithoutFilteringHumanText(t *testing.T) {
	const body = "Literal <design-checkpoint>{}</design-checkpoint> text"
	human := chatMessageToResponse(db.ChatMessage{Role: "user", MessageKind: "message", Content: body}, nil)
	if human.Content != body {
		t.Fatal("human-authored text was filtered")
	}
	for _, raw := range []string{`{`, `null`, `{"body":123,"future_field":true}`} {
		response := chatMessageToResponse(db.ChatMessage{Role: "assistant", MessageKind: "work_update", Content: body, WorkUpdate: []byte(raw)}, nil)
		if strings.Contains(response.Content, "design-checkpoint") || strings.Contains(string(response.WorkUpdate), "design-checkpoint") {
			t.Fatalf("malformed worker payload exposed receipt: %+v", response)
		}
		if json.Valid([]byte(raw)) && raw != "null" && !strings.Contains(string(response.WorkUpdate), `"future_field":true`) {
			t.Fatal("readback dropped an unknown additive field")
		}
	}
}

func TestChatWorkDeliveryTreatsDurableCommentAsResultAfterCancellation(t *testing.T) {
	ctx := context.Background()
	agent, session, runtime, _ := setupDirectChatSession(t, ctx, "Delivered before cancellation")
	issue := dbfx.Issue(t, "Clients page mockup")
	linkDeliveryIssue(t, session, issue)
	task := dbfx.Task(t, agent, testutil.Cols{
		"issue_id":     issue,
		"runtime_id":   runtime,
		"status":       "cancelled",
		"started_at":   testutil.Raw("now()"),
		"completed_at": testutil.Raw("now()"),
	})
	comment := dbfx.Insert(t, "comment", testutil.Cols{
		"workspace_id":   testWorkspaceID,
		"issue_id":       issue,
		"author_type":    "agent",
		"author_id":      agent,
		"type":           "comment",
		"content":        "Built and attached the browser-openable Clients page mockup.",
		"source_task_id": task,
	})
	dbfx.Insert(t, "attachment", testutil.Cols{
		"workspace_id":  testWorkspaceID,
		"issue_id":      issue,
		"comment_id":    comment,
		"uploader_type": "agent",
		"uploader_id":   agent,
		"filename":      "clients-page-mockup.html",
		"url":           "https://cdn.example/clients-page-mockup.html",
		"content_type":  "text/html",
		"size_bytes":    54870,
	})

	deliverWork(t)
	rows := deliveryMessages(t, session)
	if len(rows) != 1 {
		t.Fatalf("delivered update count = %d, want 1", len(rows))
	}
	var update service.ChatWorkUpdate
	if err := json.Unmarshal(rows[0].WorkUpdate, &update); err != nil {
		t.Fatal(err)
	}
	if update.Kind != "result" {
		t.Fatalf("kind = %q, want result; body = %q", update.Kind, update.Body)
	}
	if !strings.Contains(update.Body, "Built and attached") || !strings.Contains(update.Body, "Open delivered preview") ||
		!strings.Contains(update.Body, "/attachments/") || !strings.Contains(update.Body, "/preview") {
		t.Fatalf("durable deliverable missing from body: %q", update.Body)
	}
	if strings.Contains(update.Body, "/api/attachments/") || strings.Contains(update.Body, "/download") {
		t.Fatalf("HTML deliverable used a download link instead of the browser preview route: %q", update.Body)
	}
	if strings.Contains(strings.ToLower(update.Body), "without producing a reviewable result") {
		t.Fatalf("delivered work was described as missing: %q", update.Body)
	}
	if update.Review == nil || update.Review.SchemaVersion != 1 || update.Review.SourceTaskID != task {
		t.Fatalf("versioned review contract missing or misbound: %#v", update.Review)
	}
	if len(update.Review.Artifacts) != 1 {
		t.Fatalf("review artifacts = %#v, want one", update.Review.Artifacts)
	}
	artifact := update.Review.Artifacts[0]
	if artifact.Kind != "page" || artifact.Label != "clients-page-mockup.html" || !artifact.Original ||
		!strings.Contains(artifact.URL, "/attachments/") || !strings.HasSuffix(artifact.URL, "/preview") {
		t.Fatalf("wrong structured review artifact: %#v", artifact)
	}
}

func TestChatWorkDeliveryCanonicalQuestion(t *testing.T) {
	ctx := context.Background()
	f := newOrdinaryQuestionFixture(t)
	_, session, _, _ := setupDirectChatSession(t, ctx, "Question comes back to chat")
	linkDeliveryIssue(t, session, f.issue)
	report := f.report(f.issue, ownerDecisionPacket())
	if report.Code != 201 {
		t.Fatalf("report: %s", report.Body)
	}
	var receipt map[string]any
	if err := json.Unmarshal(report.Body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	questionID := receipt["inbox_item_id"].(string)
	dbfx.Exec(t, `UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1`, f.task)
	deliverWork(t)
	rows := deliveryMessages(t, session)
	if len(rows) != 1 {
		t.Fatalf("question should replace generic completed-run notice: %d", len(rows))
	}
	var update service.ChatWorkUpdate
	if err := json.Unmarshal(rows[0].WorkUpdate, &update); err != nil {
		t.Fatal(err)
	}
	if update.Kind != "question" || update.RequestID != questionID {
		t.Fatalf("wrong canonical target: %+v", update)
	}
	// Answering elsewhere retains history but never creates another question.
	current, err := testHandler.Queries.GetIssue(ctx, parseUUID(f.issue))
	if err != nil {
		t.Fatal(err)
	}
	answer := f.answer(questionID, f.issue, testUserID, current.Revision)
	if answer.Code != 201 {
		t.Fatalf("answer elsewhere: %d %s", answer.Code, answer.Body)
	}
	deliverWork(t)
	if len(deliveryMessages(t, session)) != 1 {
		t.Fatal("answered question redelivered")
	}
}

func TestChatWorkDeliveryArchivedAndStaleQuestion(t *testing.T) {
	ctx := context.Background()
	f := newOrdinaryQuestionFixture(t)
	_, session, _, _ := setupDirectChatSession(t, ctx, "Archived work")
	linkDeliveryIssue(t, session, f.issue)
	report := f.report(f.issue, ownerDecisionPacket())
	if report.Code != 201 {
		t.Fatalf("report: %s", report.Body)
	}
	dbfx.Exec(t, `UPDATE chat_session SET status='archived' WHERE id=$1`, session)
	deliverWork(t)
	if len(deliveryMessages(t, session)) != 0 {
		t.Fatal("archived chat received an update")
	}
	// The owner changed the assignee before reopening the chat. Its old worker
	// question is no longer actionable and must not be newly delivered.
	dbfx.Exec(t, `UPDATE issue SET assignee_type=NULL,assignee_id=NULL WHERE id=$1`, f.issue)
	dbfx.Exec(t, `UPDATE chat_session SET status='active' WHERE id=$1`, session)
	deliverWork(t)
	if len(deliveryMessages(t, session)) != 0 {
		t.Fatal("stale question was delivered on return")
	}
}
