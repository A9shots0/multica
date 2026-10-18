package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type archiveAtomicStarter struct {
	pool       *pgxpool.Pool
	failCancel error
	failCommit error
}

func (starter archiveAtomicStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := starter.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &archiveAtomicTx{Tx: tx, failCancel: starter.failCancel, failCommit: starter.failCommit}, nil
}

type archiveAtomicTx struct {
	pgx.Tx
	failCancel error
	failCommit error
}

func (tx *archiveAtomicTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if tx.failCancel != nil && strings.Contains(sql, "UPDATE agent_task_queue") && strings.Contains(sql, "status = 'cancelled'") {
		return nil, tx.failCancel
	}
	return tx.Tx.Query(ctx, sql, args...)
}

func (tx *archiveAtomicTx) Commit(ctx context.Context) error {
	if tx.failCommit == nil {
		return tx.Tx.Commit(ctx)
	}
	if err := tx.Tx.Rollback(ctx); err != nil {
		return err
	}
	return tx.failCommit
}

func archiveAtomicHandler(t *testing.T, starter txStarter) (*Handler, *atomic.Int32) {
	t.Helper()
	h := *testHandler
	h.TxStarter = starter
	bus := events.New()
	published := &atomic.Int32{}
	bus.Subscribe(protocol.EventAgentArchived, func(events.Event) { published.Add(1) })
	bus.Subscribe(protocol.EventTaskCancelled, func(events.Event) { published.Add(1) })
	h.Bus = bus
	h.TaskService = service.NewTaskService(h.Queries, starter, h.Hub, bus)
	return &h, published
}

func archiveAtomicFixture(t *testing.T, suffix string) (agentID, taskID string) {
	t.Helper()
	agentID = createHandlerTestAgent(t, "atomic-archive-"+suffix, nil)
	issueID := dbfx.Issue(t, "Atomic archive "+suffix)
	taskID = dbfx.Task(t, agentID, testutil.Cols{
		"issue_id":              issueID,
		"runtime_id":            handlerTestRuntimeID(t),
		"status":                "queued",
		"workflow_action_id":    uuid.NewString(),
		"workflow_operation_id": uuid.NewString(),
		"workflow_branch_key":   "main",
	})
	return agentID, taskID
}

func invokeArchiveAgent(t *testing.T, h *Handler, agentID string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	h.ArchiveAgent(w, withURLParam(newRequest(http.MethodPost, "/api/agents/"+agentID+"/archive", nil), "id", agentID))
	return w
}

func assertArchiveAtomicState(t *testing.T, agentID, taskID string, archived bool, taskStatus string) {
	t.Helper()
	var archivedAtValid bool
	if err := testPool.QueryRow(context.Background(), `SELECT archived_at IS NOT NULL FROM agent WHERE id=$1`, agentID).Scan(&archivedAtValid); err != nil {
		t.Fatal(err)
	}
	var gotStatus string
	if err := testPool.QueryRow(context.Background(), `SELECT status FROM agent_task_queue WHERE id=$1`, taskID).Scan(&gotStatus); err != nil {
		t.Fatal(err)
	}
	if archivedAtValid != archived || gotStatus != taskStatus {
		t.Fatalf("archive state=(archived %v, task %s), want (%v,%s)", archivedAtValid, gotStatus, archived, taskStatus)
	}
}

func TestArchiveAgentCancellationFailureRollsBackArchival(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	agentID, taskID := archiveAtomicFixture(t, "cancel-failure")
	injected := errors.New("injected archive task cancellation failure")
	h, published := archiveAtomicHandler(t, archiveAtomicStarter{pool: testPool, failCancel: injected})
	w := invokeArchiveAgent(t, h, agentID)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	assertArchiveAtomicState(t, agentID, taskID, false, "queued")
	if published.Load() != 0 {
		t.Fatalf("published %d events before failed transaction", published.Load())
	}
}

func TestArchiveAgentCommitFailurePublishesNothing(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	agentID, taskID := archiveAtomicFixture(t, "commit-failure")
	injected := errors.New("injected archive commit failure")
	h, published := archiveAtomicHandler(t, archiveAtomicStarter{pool: testPool, failCommit: injected})
	w := invokeArchiveAgent(t, h, agentID)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	assertArchiveAtomicState(t, agentID, taskID, false, "queued")
	if published.Load() != 0 {
		t.Fatalf("published %d events before failed commit", published.Load())
	}
}

func TestArchiveAgentCreateWinsRaceCancelsBeforePublishing(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	agentID := createHandlerTestAgent(t, "atomic-archive-create-wins", nil)
	issueID := dbfx.Issue(t, "Atomic archive create wins")
	runtimeID := handlerTestRuntimeID(t)
	taskID, actionID, operationID := uuid.NewString(), uuid.NewString(), uuid.NewString()

	creator, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer creator.Rollback(context.Background())
	if _, err := creator.Exec(context.Background(), `SELECT id FROM agent WHERE id=$1 FOR SHARE`, agentID); err != nil {
		t.Fatal(err)
	}
	if _, err := creator.Exec(context.Background(), `INSERT INTO agent_task_queue(id,agent_id,runtime_id,issue_id,status,priority,context,workflow_action_id,workflow_operation_id,workflow_branch_key) VALUES($1,$2,$3,$4,'queued',0,'{}',$5,$6,'main')`, taskID, agentID, runtimeID, issueID, actionID, operationID); err != nil {
		t.Fatal(err)
	}
	h, published := archiveAtomicHandler(t, testPool)
	type response struct {
		code int
		body string
	}
	result := make(chan response, 1)
	go func() {
		w := invokeArchiveAgent(t, h, agentID)
		result <- response{code: w.Code, body: w.Body.String()}
	}()
	select {
	case got := <-result:
		_ = creator.Rollback(context.Background())
		t.Fatalf("archive bypassed creator's agent lock: %+v", got)
	case <-time.After(100 * time.Millisecond):
	}
	if err := creator.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := <-result
	if got.code != http.StatusOK {
		t.Fatalf("status=%d body=%s", got.code, got.body)
	}
	assertArchiveAtomicState(t, agentID, taskID, true, "cancelled")
	if published.Load() != 1 {
		t.Fatalf("published events=%d, want only agent archived for issue task", published.Load())
	}
	claimService := service.NewTaskService(db.New(testPool), testPool, nil, events.New())
	claimed, err := claimService.ClaimTask(context.Background(), util.MustParseUUID(agentID))
	if err != nil || claimed != nil {
		t.Fatalf("archived agent claim=(%+v,%v)", claimed, err)
	}
}
