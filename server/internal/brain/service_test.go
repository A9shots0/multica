package brain

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestClaimWithoutCompleteConfigurationIsUnavailable(t *testing.T) {
	tests := []struct {
		name    string
		service *Service
		project string
	}{
		{name: "missing service"},
		{name: "missing client", service: &Service{}},
		{
			name: "missing store",
			service: &Service{Client: &Client{config: Config{Workspaces: map[string]Workspace{
				workspaceID: {Projects: map[string]string{projectID: "crm"}},
			}}}},
			project: projectID,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := tt.service.Claim(context.Background(), workspaceID, tt.project, "", db.AgentTaskQueue{})
			var ref Reference
			if err := json.Unmarshal(raw, &ref); err != nil {
				t.Fatal(err)
			}
			if ref.State != "unconfigured" || ref.Lifecycle != "not_recorded" {
				t.Fatalf("incomplete configuration must fail closed: %#v", ref)
			}
		})
	}
}

type testStore struct {
	rows        []db.BrainEventOutbox
	failed      []db.MarkBrainEventFailureParams
	receipts    int
	failReceipt bool
}

func (s *testStore) BindBrainTask(context.Context, db.BindBrainTaskParams) (db.BrainTaskLink, error) {
	return db.BrainTaskLink{}, errors.New("test unavailable")
}
func (s *testStore) CaptureBrainEvents(context.Context) (int64, error) { return 0, nil }
func (s *testStore) ListPendingBrainEvents(context.Context) ([]db.BrainEventOutbox, error) {
	return s.rows, nil
}
func (s *testStore) MarkBrainEventDelivered(context.Context, string) error {
	if s.failReceipt {
		return errors.New("crash before receipt")
	}
	s.receipts++
	return nil
}
func (s *testStore) MarkBrainEventFailure(_ context.Context, p db.MarkBrainEventFailureParams) error {
	s.failed = append(s.failed, p)
	return nil
}

func TestCrashRetryPreservesBodyAndConflictBlocks(t *testing.T) {
	var bodies []string
	status := 200
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(raw))
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"schemaVersion":"silvermediums.brain.v2","servedAt":"now","recorded":{"id":"33333333-3333-4333-8333-333333333333","status":"task_observation","acceptance":"not_established"}}`))
	})
	var ws pgtype.UUID
	_ = ws.Scan(workspaceID)
	store := &testStore{failReceipt: true, rows: []db.BrainEventOutbox{{EventID: "stable-event", WorkspaceID: ws, ProjectID: "crm", Payload: []byte(`{"idempotencyKey":"stable-event","summary":"run finished; acceptance unknown"}`)}}}
	s := Service{Client: c, Store: store}
	if s.Reconcile(context.Background()) == nil {
		t.Fatal("receipt failure was hidden")
	}
	store.failReceipt = false
	if err := s.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 2 || bodies[0] != bodies[1] || store.receipts != 1 {
		t.Fatal("retry changed event or claimed false delivery")
	}
	status = 409
	if err := s.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.failed) != 1 || !store.failed[0].IsBlocked {
		t.Fatal("conflict must require review")
	}
	status = 503
	if err := s.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.failed[1].IsBlocked {
		t.Fatal("outage must remain retryable")
	}
}

func TestRemovedMappingNeverSends(t *testing.T) {
	c := testClient(t, func(http.ResponseWriter, *http.Request) { t.Error("unauthorized event was sent") })
	var ws pgtype.UUID
	_ = ws.Scan(workspaceID)
	store := &testStore{rows: []db.BrainEventOutbox{{EventID: "private", WorkspaceID: ws, ProjectID: "outside"}}}
	s := Service{Client: c, Store: store}
	if err := s.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.failed) != 1 || !store.failed[0].IsBlocked {
		t.Fatal("removed scope not blocked")
	}
}

func TestMissingDurableReceiptRemainsRetryable(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"schemaVersion":"silvermediums.brain.v2","servedAt":"now"}`))
	})
	var ws pgtype.UUID
	_ = ws.Scan(workspaceID)
	store := &testStore{rows: []db.BrainEventOutbox{{EventID: "pending", WorkspaceID: ws, ProjectID: "crm", Payload: []byte(`{"idempotencyKey":"pending"}`)}}}
	s := Service{Client: c, Store: store}
	if err := s.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.receipts != 0 || len(store.failed) != 1 || store.failed[0].IsBlocked {
		t.Fatal("unacknowledged event must stay pending for exact replay")
	}
}
