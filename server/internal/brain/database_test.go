package brain

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestDatabaseLifecycleSurvivesRetriesAndTransactions(t *testing.T) {
	url := os.Getenv("MULTICA_BRAIN_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("requires explicitly isolated brain test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	f := testutil.New(pool, "", "")
	suffix := uuid.NewString()
	f.UserID = f.User(t, "Brain proof", "brain-"+suffix+"@example.invalid")
	f.WorkspaceID = f.Workspace(t, "Brain proof", "brain-"+suffix)
	runtime := f.Runtime(t, "Brain isolated runtime")
	agent := f.Agent(t, "Brain test worker", runtime, testutil.Cols{"owner_id": f.UserID})
	task := f.Task(t, agent, testutil.Cols{"attempt": 1, "runtime_id": runtime})
	f.Cleanup(t, "DELETE FROM brain_task_link WHERE workspace_id=$1", f.WorkspaceID)
	f.Cleanup(t, "DELETE FROM brain_event_outbox WHERE workspace_id=$1", f.WorkspaceID)
	ws, _ := util.ParseUUID(f.WorkspaceID)
	taskUUID, _ := util.ParseUUID(task)
	q := db.New(pool)
	link, err := q.BindBrainTask(ctx, db.BindBrainTaskParams{WorkspaceID: ws, TaskID: taskUUID, Attempt: 1, ProjectID: "crm"})
	if err != nil || link.ProjectID != "crm" {
		t.Fatalf("bind: %v", err)
	}
	changed, err := q.BindBrainTask(ctx, db.BindBrainTaskParams{WorkspaceID: ws, TaskID: taskUUID, Attempt: 1, ProjectID: "other"})
	if err != nil || changed.ProjectID != "crm" {
		t.Fatal("replay changed immutable project binding")
	}
	other, _ := util.ParseUUID(uuid.NewString())
	if _, err = q.BindBrainTask(ctx, db.BindBrainTaskParams{WorkspaceID: other, TaskID: taskUUID, Attempt: 1, ProjectID: "crm"}); err == nil {
		t.Fatal("cross-workspace task bound")
	}
	f.Exec(t, "UPDATE agent_task_queue SET status='running',started_at=now() WHERE id=$1", task)
	if got := f.Count(t, "SELECT count(*) FROM brain_event_outbox WHERE workspace_id=$1", f.WorkspaceID); got != 1 {
		t.Fatalf("start not captured atomically: %d", got)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1", task); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if got := f.Count(t, "SELECT count(*) FROM brain_event_outbox WHERE workspace_id=$1", f.WorkspaceID); got != 1 {
		t.Fatal("rolled-back completion escaped")
	}
	f.Exec(t, "UPDATE agent_task_queue SET status='completed',completed_at=now(),result='{}'::jsonb WHERE id=$1", task)
	// A fast retry overwrites mutable task timestamps before the polling loop.
	// The committed first-attempt completion must already be in the outbox.
	f.Exec(t, "UPDATE agent_task_queue SET status='queued',attempt=2,started_at=NULL,completed_at=NULL WHERE id=$1", task)
	for i := 0; i < 2; i++ {
		if _, err = q.CaptureBrainEvents(ctx); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := q.ListPendingBrainEvents(ctx)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, row := range rows {
		if row.WorkspaceID != ws {
			continue
		}
		seen++
		var payload map[string]any
		if err = json.Unmarshal(row.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		if payload["project"] != "crm" || payload["idempotencyKey"] != row.EventID {
			t.Fatal("outbox provenance changed")
		}
		if _, ok := payload["output"]; ok {
			t.Fatal("raw worker output was exported")
		}
	}
	if seen != 2 {
		t.Fatalf("expected one start and one completion after replay; got %d", seen)
	}
}
