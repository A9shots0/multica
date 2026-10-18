package migrations

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	taskservice "github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/internal/workflowaction"
	"github.com/multica-ai/multica/server/internal/workflowcommand"
	"github.com/multica-ai/multica/server/internal/workflowprojection"
	workflowstatev2 "github.com/multica-ai/multica/server/internal/workflowstate/v2"
	"github.com/multica-ai/multica/server/internal/workflowstore"
	generated "github.com/multica-ai/multica/server/pkg/db/generated"
)

const workflowV2TestDatabaseEnv = "WORKFLOW_V2_TEST_DATABASE_URL"

func TestWorkflowV2DatabaseContracts(t *testing.T) {
	ctx, conn, queries := setupWorkflowV2Database(t)

	t.Run("concern heads initialize only at exact baseline", func(t *testing.T) {
		workspaceID, issueID := workflowV2UUID(101), workflowV2UUID(102)
		concerns := []struct{ concern, state string }{{"outcome", "open"}, {"scope", "accepted"}, {"authority", "active"}, {"execution", "received"}, {"review", "pending"}, {"release", "pending"}, {"attention", "open"}, {"evidence", "unknown"}, {"budget", "active"}}
		for i, item := range concerns {
			got, err := queries.InitializeIssueWorkflowConcernHead(ctx, generated.InitializeIssueWorkflowConcernHeadParams{
				WorkspaceID: workspaceID, IssueID: issueID, ConcernType: item.concern,
				SourceRecordID: workflowV2UUID(byte(110 + i)), SourceRecordRevision: 1,
				State: item.state, ExpectedRevision: 0,
			})
			if err != nil || got.Revision != 1 {
				t.Fatalf("initialize %s = (%+v, %v), want revision 1", item.concern, got, err)
			}
		}
		heads, err := queries.ListIssueWorkflowConcernHeads(ctx, generated.ListIssueWorkflowConcernHeadsParams{WorkspaceID: workspaceID, IssueID: issueID})
		if err != nil || len(heads) != 9 {
			t.Fatalf("heads = %d, %v, want 9", len(heads), err)
		}
		bad := generated.InitializeIssueWorkflowConcernHeadParams{WorkspaceID: workspaceID, IssueID: workflowV2UUID(103), ConcernType: "outcome", SourceRecordID: workflowV2UUID(104), SourceRecordRevision: 2, State: "open", ExpectedRevision: 0}
		if _, err := queries.InitializeIssueWorkflowConcernHead(ctx, bad); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("nonbaseline source revision error = %v, want pgx.ErrNoRows", err)
		}
		bad.SourceRecordRevision, bad.ExpectedRevision = 1, 1
		if _, err := queries.InitializeIssueWorkflowConcernHead(ctx, bad); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("nonzero expected revision error = %v, want pgx.ErrNoRows", err)
		}
		bad.ExpectedRevision, bad.State = 0, "active"
		if _, err := queries.InitializeIssueWorkflowConcernHead(ctx, bad); err == nil {
			t.Fatal("outcome concern accepted an authority-only state")
		}
	})

	t.Run("authorization rejection replans only the proposed operation", func(t *testing.T) {
		workspaceID, issueID := seedLoadableWorkflowV2(t, ctx, conn, 49)
		controllerID, ownerID := workflowV2UUID(67), workflowV2UUID(61)
		if _, err := conn.Exec(ctx, `INSERT INTO member(id,workspace_id,user_id,role) VALUES($1,$2,$3,'owner')`, workflowV2UUID(68), workspaceID, ownerID); err != nil {
			t.Fatal(err)
		}
		now := func() time.Time { return time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC) }
		service := workflowcommand.Service{Starter: conn, Queries: queries, Loader: workflowcommand.Loader{Now: now}, Now: now, ControllerID: controllerID}
		request := workflowcommand.Request{WorkspaceID: workspaceID, IssueID: issueID, CommandKey: "request-authorization-reject", Kind: workflowstatev2.CommandRequestDecision, OperationID: util.UUIDToString(workflowV2UUID(54)), CandidateID: util.UUIDToString(workflowV2UUID(60)), Expected: []workflowstatev2.ExpectedRevision{{Concern: workflowstatev2.ConcernExecution, Revision: 1}, {Concern: workflowstatev2.ConcernAttention, Revision: 1}, {Concern: workflowstatev2.ConcernEvidence, Revision: 1}}, Decision: &workflowcommand.DecisionInput{Type: workflowstatev2.DecisionAuthorization, Question: "Run this operation?", Recommendation: "Reject and replan if unsafe", Consequence: "Only this operation is denied", AccountableOwnerID: ownerID, AuthorityGap: workflowstatev2.AuthorityGap{Kind: workflowstatev2.AuthorityGapOperationAuthorization, RequestedCapability: "execute proposed operation", Reason: "owner authorization required"}}}
		if _, err := service.Execute(ctx, workflowcommand.Principal{ActorID: controllerID, ActorType: "internal_controller"}, request); err != nil {
			t.Fatal(err)
		}
		decisionID := stableWorkflowCommandUUID(workspaceID, issueID, request.CommandKey, "decision")
		response := workflowcommand.Request{WorkspaceID: workspaceID, IssueID: issueID, CommandKey: "reject-authorization", Kind: workflowstatev2.CommandRespondDecision, DecisionID: decisionID, DecisionOutcome: workflowstatev2.DecisionReject, ExpectedDecisionRevision: 1, Expected: []workflowstatev2.ExpectedRevision{{Concern: workflowstatev2.ConcernExecution, Revision: 2}, {Concern: workflowstatev2.ConcernAttention, Revision: 2}}}
		if _, err := service.Execute(ctx, workflowcommand.Principal{ActorID: ownerID, ActorType: "member"}, response); err != nil {
			t.Fatal(err)
		}
		worker := workflowaction.Worker{Starter: conn, Queries: queries, ControllerID: controllerID, CommandService: service, Now: now, LeaseDuration: time.Minute, RetryDelay: time.Second, Effect: workflowaction.EffectFuncs{ObserveFunc: func(context.Context, generated.IssueWorkflowActionReceipt) (workflowaction.Observation, error) {
			return workflowaction.Observation{}, errors.New("unexpected external effect")
		}, ApplyFunc: func(context.Context, generated.IssueWorkflowActionReceipt) (workflowaction.Observation, error) {
			return workflowaction.Observation{}, errors.New("unexpected external effect")
		}}, Capacity: workflowaction.CapacityFunc(func(context.Context, generated.IssueWorkflowActionReceipt) (bool, error) { return true, nil })}
		if err := worker.ReconcileOnce(ctx); err != nil {
			t.Fatal(err)
		}
		var executionState, outcomeState string
		if err := conn.QueryRow(ctx, `SELECT state FROM issue_workflow_execution WHERE workspace_id=$1 AND issue_id=$2 AND id=$3`, workspaceID, issueID, workflowV2UUID(54)).Scan(&executionState); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT state FROM issue_workflow_outcome WHERE workspace_id=$1 AND issue_id=$2`, workspaceID, issueID).Scan(&outcomeState); err != nil {
			t.Fatal(err)
		}
		var forbiddenActions int
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_action_receipt WHERE workspace_id=$1 AND issue_id=$2 AND action_type IN ('review_correction','cancel_exact_operation')`, workspaceID, issueID).Scan(&forbiddenActions); err != nil {
			t.Fatal(err)
		}
		if executionState != "queued" || outcomeState != "open" || forbiddenActions != 0 {
			t.Fatalf("reject execution=%s outcome=%s forbidden_actions=%d", executionState, outcomeState, forbiddenActions)
		}
	})

	t.Run("owner decision continuation failure reaches bounded technical stop", func(t *testing.T) {
		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		qtx := generated.New(tx)
		workspaceID, issueID := seedLoadableWorkflowV2(t, ctx, tx, 90)
		controllerID, ownerID := workflowV2UUID(118), workflowV2UUID(102)
		if _, err := tx.Exec(ctx, `INSERT INTO member(id,workspace_id,user_id,role) VALUES($1,$2,$3,'owner')`, workflowV2UUID(119), workspaceID, ownerID); err != nil {
			t.Fatal(err)
		}
		now := func() time.Time { return time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC) }
		service := workflowcommand.Service{Starter: tx, Queries: qtx, Loader: workflowcommand.Loader{Now: now}, Now: now, ControllerID: controllerID}
		request := authorizationDecisionRequest(workspaceID, issueID, "bounded-owner-continuation", workflowV2UUID(95), workflowV2UUID(101), ownerID)
		if _, err := service.Execute(ctx, workflowcommand.Principal{ActorID: controllerID, ActorType: "internal_controller"}, request); err != nil {
			t.Fatal(err)
		}
		decisionID := stableWorkflowCommandUUID(workspaceID, issueID, request.CommandKey, "decision")
		response := workflowcommand.Request{WorkspaceID: workspaceID, IssueID: issueID, CommandKey: "bounded-owner-continuation-reject", Kind: workflowstatev2.CommandRespondDecision, DecisionID: decisionID, DecisionOutcome: workflowstatev2.DecisionReject, ExpectedDecisionRevision: 1, Expected: []workflowstatev2.ExpectedRevision{{Concern: workflowstatev2.ConcernExecution, Revision: 2}, {Concern: workflowstatev2.ConcernAttention, Revision: 2}}}
		if _, err := service.Execute(ctx, workflowcommand.Principal{ActorID: ownerID, ActorType: "member"}, response); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `UPDATE issue_workflow_action_receipt SET max_attempts=1, payload=jsonb_set(payload,'{expected,attention}','999'::jsonb) WHERE workspace_id=$1 AND issue_id=$2 AND action_type='replan_exact_operation'`, workspaceID, issueID); err != nil {
			t.Fatal(err)
		}
		worker := workflowaction.Worker{Starter: tx, Queries: qtx, ControllerID: controllerID, CommandService: service, Now: now, LeaseDuration: time.Minute, RetryDelay: time.Second, Effect: workflowaction.EffectFuncs{}, Capacity: workflowaction.CapacityFunc(func(context.Context, generated.IssueWorkflowActionReceipt) (bool, error) { return true, nil })}
		if err := worker.ReconcileOnce(ctx); err != nil {
			t.Fatal(err)
		}
		var actionState, executionState string
		if err := tx.QueryRow(ctx, `SELECT state FROM issue_workflow_action_receipt WHERE workspace_id=$1 AND issue_id=$2 AND action_type='replan_exact_operation'`, workspaceID, issueID).Scan(&actionState); err != nil {
			t.Fatal(err)
		}
		if err := tx.QueryRow(ctx, `SELECT state FROM issue_workflow_execution WHERE workspace_id=$1 AND issue_id=$2`, workspaceID, issueID).Scan(&executionState); err != nil {
			t.Fatal(err)
		}
		if actionState != "dead_letter" || executionState != "technically_stopped" {
			t.Fatalf("bounded continuation action=%s execution=%s", actionState, executionState)
		}
		if err := worker.ReconcileOnce(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("owner decision continuation rejects altered immutable bindings", func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			path  string
			value string
		}{
			{name: "decision type", path: "decision_type", value: `"scope_change"`},
			{name: "target", path: "target_key", value: `"other-target"`},
			{name: "suspended state", path: "suspended_state", value: `"queued"`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				tx, err := conn.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback(ctx)
				qtx := generated.New(tx)
				workspaceID, issueID := seedLoadableWorkflowV2(t, ctx, tx, 90)
				controllerID, ownerID := workflowV2UUID(118), workflowV2UUID(102)
				if _, err := tx.Exec(ctx, `INSERT INTO member(id,workspace_id,user_id,role) VALUES($1,$2,$3,'owner')`, workflowV2UUID(119), workspaceID, ownerID); err != nil {
					t.Fatal(err)
				}
				now := func() time.Time { return time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC) }
				service := workflowcommand.Service{Starter: tx, Queries: qtx, Loader: workflowcommand.Loader{Now: now}, Now: now, ControllerID: controllerID}
				request := authorizationDecisionRequest(workspaceID, issueID, "altered-owner-continuation", workflowV2UUID(95), workflowV2UUID(101), ownerID)
				if _, err := service.Execute(ctx, workflowcommand.Principal{ActorID: controllerID, ActorType: "internal_controller"}, request); err != nil {
					t.Fatal(err)
				}
				decisionID := stableWorkflowCommandUUID(workspaceID, issueID, request.CommandKey, "decision")
				response := workflowcommand.Request{WorkspaceID: workspaceID, IssueID: issueID, CommandKey: "altered-owner-continuation-reject", Kind: workflowstatev2.CommandRespondDecision, DecisionID: decisionID, DecisionOutcome: workflowstatev2.DecisionReject, ExpectedDecisionRevision: 1, Expected: []workflowstatev2.ExpectedRevision{{Concern: workflowstatev2.ConcernExecution, Revision: 2}, {Concern: workflowstatev2.ConcernAttention, Revision: 2}}}
				receipt, err := service.Execute(ctx, workflowcommand.Principal{ActorID: ownerID, ActorType: "member"}, response)
				if err != nil || len(receipt.ActionIDs) != 1 {
					t.Fatalf("respond receipt=%+v err=%v", receipt, err)
				}
				actionID := workflowV2UUIDString(receipt.ActionIDs[0])
				if _, err := tx.Exec(ctx, `UPDATE issue_workflow_action_receipt SET payload=jsonb_set(payload,ARRAY[$3],$4::jsonb) WHERE workspace_id=$1 AND issue_id=$2 AND id=$5`, workspaceID, issueID, tc.path, tc.value, actionID); err != nil {
					t.Fatal(err)
				}
				lease := pgtype.UUID{Bytes: uuid.New(), Valid: true}
				claimed, err := qtx.ClaimIssueWorkflowActionExact(ctx, generated.ClaimIssueWorkflowActionExactParams{LeaseToken: lease, LeaseExpiresAt: workflowV2Time(now().Add(time.Minute)), WorkspaceID: workspaceID, IssueID: issueID, ID: actionID, ExpectedRevision: 1, Now: workflowV2Time(now())})
				if err != nil {
					t.Fatal(err)
				}
				settle := workflowcommand.Request{WorkspaceID: workspaceID, IssueID: issueID, CommandKey: "action/" + util.UUIDToString(actionID) + "/settle-owner-decision", Kind: workflowstatev2.CommandSettleDecisionEffect, DecisionID: decisionID, ExpectedDecisionRevision: 2, DecisionOutcome: workflowstatev2.DecisionReject, OperationID: util.UUIDToString(workflowV2UUID(95)), Expected: []workflowstatev2.ExpectedRevision{{Concern: workflowstatev2.ConcernExecution, Revision: 3}, {Concern: workflowstatev2.ConcernAttention, Revision: 3}}}
				if _, err := service.Execute(ctx, workflowcommand.Principal{ActorID: controllerID, ActorType: "internal_controller", ActionID: actionID, ActionKey: claimed.ActionKey, LeaseToken: lease}, settle); !errors.Is(err, workflowcommand.ErrForbidden) {
					t.Fatalf("altered binding error=%v, want ErrForbidden", err)
				}
				var commands int
				if err := tx.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_command WHERE workspace_id=$1 AND issue_id=$2 AND command_key=$3`, workspaceID, issueID, settle.CommandKey).Scan(&commands); err != nil || commands != 0 {
					t.Fatalf("settlement residue=%d err=%v", commands, err)
				}
			})
		}
	})

	t.Run("concern head CAS fences same concern and permits independent concerns", func(t *testing.T) {
		workspaceID, issueID := workflowV2UUID(120), workflowV2UUID(121)
		for i, concern := range []string{"execution", "review", "budget"} {
			if _, err := queries.InitializeIssueWorkflowConcernHead(ctx, generated.InitializeIssueWorkflowConcernHeadParams{WorkspaceID: workspaceID, IssueID: issueID, ConcernType: concern, SourceRecordID: workflowV2UUID(byte(122 + i)), SourceRecordRevision: 1, State: map[string]string{"execution": "received", "review": "pending", "budget": "active"}[concern], ExpectedRevision: 0}); err != nil {
				t.Fatal(err)
			}
		}
		run := func(concern string, source byte) error {
			other, err := workflowV2TestConnection(ctx)
			if err != nil {
				return err
			}
			defer other.Close(ctx)
			_, err = generated.New(other).AdvanceIssueWorkflowConcernHeadCAS(ctx, generated.AdvanceIssueWorkflowConcernHeadCASParams{WorkspaceID: workspaceID, IssueID: issueID, ConcernType: concern, ExpectedRevision: 1, SourceRecordID: workflowV2UUID(source), SourceRecordRevision: 2, State: map[string]string{"execution": "working", "review": "passed", "budget": "exhausted"}[concern]})
			return err
		}
		var wg sync.WaitGroup
		errs := make(chan error, 2)
		for _, source := range []byte{130, 131} {
			wg.Add(1)
			go func(source byte) { defer wg.Done(); errs <- run("execution", source) }(source)
		}
		wg.Wait()
		close(errs)
		succeeded, stale := 0, 0
		for err := range errs {
			if err == nil {
				succeeded++
			} else if errors.Is(err, pgx.ErrNoRows) {
				stale++
			} else {
				t.Fatal(err)
			}
		}
		if succeeded != 1 || stale != 1 {
			t.Fatalf("same concern results = success %d stale %d, want 1/1", succeeded, stale)
		}

		errs = make(chan error, 2)
		for _, item := range []struct {
			concern string
			source  byte
		}{{"review", 132}, {"budget", 133}} {
			wg.Add(1)
			go func(concern string, source byte) { defer wg.Done(); errs <- run(concern, source) }(item.concern, item.source)
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("independent concern CAS: %v", err)
			}
		}
	})

	t.Run("head and source record roll back atomically", func(t *testing.T) {
		workspaceID, issueID, outcomeID := workflowV2UUID(160), workflowV2UUID(161), workflowV2UUID(162)
		if _, err := conn.Exec(ctx, `INSERT INTO workspace(id) VALUES($1)`, workspaceID); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, `INSERT INTO issue(id, workspace_id) VALUES($1,$2)`, issueID, workspaceID); err != nil {
			t.Fatal(err)
		}
		rollback := errors.New("force rollback")
		err := workflowstore.InIssueWriteTransaction(ctx, conn, queries, workspaceID, issueID, func(write *workflowstore.IssueWrite) error {
			if _, err := write.CreateOutcome(ctx, generated.CreateIssueWorkflowOutcomeParams{ID: outcomeID, WorkspaceID: workspaceID, IssueID: issueID, State: "open", AcceptanceVersion: 1}); err != nil {
				return err
			}
			if _, err := write.InitializeConcernHead(ctx, generated.InitializeIssueWorkflowConcernHeadParams{WorkspaceID: workspaceID, IssueID: issueID, ConcernType: "outcome", SourceRecordID: outcomeID, SourceRecordRevision: 1, State: "open", ExpectedRevision: 0}); err != nil {
				return err
			}
			return rollback
		})
		if !errors.Is(err, rollback) {
			t.Fatalf("transaction error = %v", err)
		}
		for _, table := range []string{"issue_workflow_outcome", "issue_workflow_concern_head"} {
			var count int
			if err := conn.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE workspace_id=$1 AND issue_id=$2", workspaceID, issueID).Scan(&count); err != nil || count != 0 {
				t.Fatalf("%s count after rollback = %d, %v", table, count, err)
			}
		}
	})

	t.Run("decision persists exact suspended execution state", func(t *testing.T) {
		workspaceID, issueID := workflowV2UUID(163), workflowV2UUID(164)
		params := generated.CreateIssueWorkflowDecisionParams{
			ID: workflowV2UUID(165), WorkspaceID: workspaceID, IssueID: issueID,
			ScopeID: workflowV2UUID(166), ScopeRevision: 1, DecisionType: "approval",
			State: "unanswered", ResolutionState: "open", Question: "Proceed?",
			Options: []byte(`[]`), ResponseSchema: []byte(`{}`), AuthorityGap: []byte(`{}`),
			RequesterID: workflowV2UUID(167), AccountableOwnerID: workflowV2UUID(168),
			SuspendedOperationID: workflowV2UUID(169), SuspendedExecutionState: "working",
		}
		got, err := queries.CreateIssueWorkflowDecision(ctx, params)
		if err != nil || got.SuspendedExecutionState != "working" {
			t.Fatalf("decision = (%+v, %v)", got, err)
		}
		params.ID, params.SuspendedExecutionState = workflowV2UUID(170), "invented"
		if _, err := queries.CreateIssueWorkflowDecision(ctx, params); err == nil {
			t.Fatal("invalid suspended execution state was accepted")
		}
	})

	t.Run("loader uses normalized authority and ignores display projection", func(t *testing.T) {
		workspaceID, issueID := seedLoadableWorkflowV2(t, ctx, conn, 168)
		load := func() (any, error) {
			var picture any
			err := workflowstore.InIssueWriteTransaction(ctx, conn, queries, workspaceID, issueID, func(write *workflowstore.IssueWrite) error {
				loaded, err := (workflowcommand.Loader{Now: func() time.Time { return time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC) }}).Load(ctx, write, workspaceID, issueID)
				picture = loaded
				return err
			})
			return picture, err
		}
		before, err := load()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, `INSERT INTO issue_current_picture (workspace_id,issue_id,contract_version,revision,outcome_state,outcome_open,execution_state,release_state,health_state,attention_state,source_revisions,document,generated_at) VALUES ($1,$2,2,1,'satisfied',false,'failed','failed','degraded','open','{}','{"fabricated":true}',now())`, workspaceID, issueID); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, `DELETE FROM issue_current_picture WHERE workspace_id=$1 AND issue_id=$2`, workspaceID, issueID); err != nil {
			t.Fatal(err)
		}
		after, err := load()
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("loader changed after display projection rebuild: before=%+v after=%+v", before, after)
		}

		for _, tc := range []struct{ name, mutate string }{
			{"missing ledger", `DELETE FROM issue_workflow_budget WHERE workspace_id=$1 AND issue_id=$2`},
			{"malformed ledger", `UPDATE issue_workflow_authority SET envelope='null'::jsonb WHERE workspace_id=$1 AND issue_id=$2`},
			{"duplicate concern head", `INSERT INTO issue_workflow_concern_head SELECT * FROM issue_workflow_concern_head WHERE workspace_id=$1 AND issue_id=$2 AND concern_type='outcome'`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				tx, err := conn.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback(ctx)
				_, err = tx.Exec(ctx, tc.mutate, workspaceID, issueID)
				if tc.name == "duplicate concern head" {
					if err == nil {
						t.Fatal("duplicate concern head was accepted")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				var ignored any
				err = workflowstore.InIssueWriteTransaction(ctx, tx, generated.New(tx), workspaceID, issueID, func(write *workflowstore.IssueWrite) error {
					loaded, loadErr := (workflowcommand.Loader{Now: func() time.Time { return time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC) }}).Load(ctx, write, workspaceID, issueID)
					ignored = loaded
					return loadErr
				})
				_ = ignored
				if !errors.Is(err, workflowcommand.ErrInvalidAuthoritativeState) {
					t.Fatalf("error = %v", err)
				}
			})
		}
	})

	t.Run("command service atomically applies decision request and exact replay", func(t *testing.T) {
		workspaceID, issueID := seedLoadableWorkflowV2(t, ctx, conn, 200)
		controllerID, ownerID, operationID := workflowV2UUID(213), workflowV2UUID(212), workflowV2UUID(205)
		if _, err := conn.Exec(ctx, `INSERT INTO member(id,workspace_id,user_id,role) VALUES($1,$2,$3,'owner')`, workflowV2UUID(220), workspaceID, ownerID); err != nil {
			t.Fatal(err)
		}
		now := func() time.Time { return time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC) }
		wakeErr := errors.New("wake unavailable")
		var observedWake error
		service := workflowcommand.Service{Starter: conn, Queries: queries, Loader: workflowcommand.Loader{Now: now}, Now: now, ControllerID: controllerID, Wake: func(context.Context, []string) error { return wakeErr }, OnWakeError: func(err error) { observedWake = err }}
		request := workflowcommand.Request{WorkspaceID: workspaceID, IssueID: issueID, CommandKey: "request-decision-1", Kind: workflowstatev2.CommandRequestDecision, OperationID: util.UUIDToString(operationID), CandidateID: util.UUIDToString(workflowV2UUID(211)), Expected: []workflowstatev2.ExpectedRevision{{Concern: workflowstatev2.ConcernExecution, Revision: 1}, {Concern: workflowstatev2.ConcernAttention, Revision: 1}, {Concern: workflowstatev2.ConcernEvidence, Revision: 1}}, Decision: &workflowcommand.DecisionInput{Type: workflowstatev2.DecisionAuthorization, Question: "Proceed?", Recommendation: "Approve this exact operation", Consequence: "Work remains paused until answered", AccountableOwnerID: ownerID, AuthorityGap: workflowstatev2.AuthorityGap{Kind: workflowstatev2.AuthorityGapOperationAuthorization, RequestedCapability: "execute proposed operation", Reason: "owner authorization is required"}}}
		principal := workflowcommand.Principal{ActorID: controllerID, ActorType: "internal_controller"}
		wrongController := principal
		wrongController.ActorType = "member"
		if _, err := service.Execute(ctx, wrongController, request); !errors.Is(err, workflowcommand.ErrForbidden) {
			t.Fatalf("wrong initial controller principal kind error=%v", err)
		}
		var wrongControllerResidue int
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_command WHERE workspace_id=$1 AND issue_id=$2`, workspaceID, issueID).Scan(&wrongControllerResidue); err != nil || wrongControllerResidue != 0 {
			t.Fatalf("wrong controller residue=%d err=%v", wrongControllerResidue, err)
		}
		for table, want := range map[string]int{"issue_workflow_decision": 0, "issue_workflow_action_receipt": 0, "issue_current_picture": 0, "issue_workflow_attention": 1} {
			var count int
			if err := conn.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE workspace_id=$1 AND issue_id=$2", workspaceID, issueID).Scan(&count); err != nil || count != want {
				t.Fatalf("wrong controller %s residue=%d want=%d err=%v", table, count, want, err)
			}
		}
		first, err := service.Execute(ctx, principal, request)
		if err != nil {
			t.Fatal(err)
		}
		if first.State != "applied" || first.ActionState != "admitted" || len(first.ActionIDs) != 1 || !errors.Is(observedWake, wakeErr) {
			t.Fatalf("receipt/wake = %+v, %v", first, observedWake)
		}
		replayed, err := service.Execute(ctx, principal, request)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(first, replayed) {
			t.Fatalf("replay changed logical receipt: first=%+v replay=%+v", first, replayed)
		}
		if _, err := service.Execute(ctx, wrongController, request); !errors.Is(err, workflowcommand.ErrForbidden) {
			t.Fatalf("wrong replay controller principal kind error=%v", err)
		}
		request.Detail = "mismatch"
		if _, err := service.Execute(ctx, principal, request); !errors.Is(err, workflowcommand.ErrIdempotencyConflict) {
			t.Fatalf("mismatch error=%v", err)
		}
		decisionID := stableWorkflowCommandUUID(workspaceID, issueID, "request-decision-1", "decision")
		response := workflowcommand.Request{WorkspaceID: workspaceID, IssueID: issueID, CommandKey: "respond-decision-1", Kind: workflowstatev2.CommandRespondDecision, DecisionID: decisionID, DecisionOutcome: workflowstatev2.DecisionApprove, ExpectedDecisionRevision: 1, Expected: []workflowstatev2.ExpectedRevision{{Concern: workflowstatev2.ConcernExecution, Revision: 2}, {Concern: workflowstatev2.ConcernAttention, Revision: 2}}}
		ownerPrincipal := workflowcommand.Principal{ActorID: ownerID, ActorType: "member"}
		wrongOwner := ownerPrincipal
		wrongOwner.ActorType = "agent_task"
		wrongOwnerRequest := response
		wrongOwnerRequest.CommandKey = "respond-wrong-principal-kind"
		if _, err := service.Execute(ctx, wrongOwner, wrongOwnerRequest); !errors.Is(err, workflowcommand.ErrForbidden) {
			t.Fatalf("wrong initial owner principal kind error=%v", err)
		}
		var responseResidue int
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_decision_response WHERE workspace_id=$1 AND issue_id=$2`, workspaceID, issueID).Scan(&responseResidue); err != nil || responseResidue != 0 {
			t.Fatalf("wrong owner response residue=%d err=%v", responseResidue, err)
		}
		for table, want := range map[string]int{"issue_workflow_command": 1, "issue_workflow_decision": 1, "issue_workflow_action_receipt": 1, "issue_current_picture": 1, "issue_workflow_attention": 2} {
			var count int
			if err := conn.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE workspace_id=$1 AND issue_id=$2", workspaceID, issueID).Scan(&count); err != nil || count != want {
				t.Fatalf("wrong owner %s residue=%d want=%d err=%v", table, count, want, err)
			}
		}
		entered, release := make(chan struct{}), make(chan struct{})
		injectedRace := errors.New("stop after owner authority lock")
		racingService := service
		racingService.Failpoint = func(stage string) error {
			if stage == "after_authority" {
				close(entered)
				<-release
				return injectedRace
			}
			return nil
		}
		racingResponse := response
		racingResponse.CommandKey = "respond-owner-race"
		serviceDone := make(chan error, 1)
		go func() { _, err := racingService.Execute(ctx, ownerPrincipal, racingResponse); serviceDone <- err }()
		<-entered
		other, err := workflowV2TestConnection(ctx)
		if err != nil {
			t.Fatal(err)
		}
		updateDone := make(chan error, 1)
		go func() {
			_, err := other.Exec(ctx, `UPDATE member SET role='admin' WHERE workspace_id=$1 AND user_id=$2`, workspaceID, ownerID)
			updateDone <- err
		}()
		select {
		case err := <-updateDone:
			t.Fatalf("member downgrade bypassed owner authority lock: %v", err)
		case <-time.After(100 * time.Millisecond):
		}
		close(release)
		if err := <-serviceDone; !errors.Is(err, injectedRace) {
			t.Fatalf("paused owner service error=%v", err)
		}
		if err := <-updateDone; err != nil {
			t.Fatal(err)
		}
		_ = other.Close(ctx)
		if _, err := conn.Exec(ctx, `UPDATE member SET role='owner' WHERE workspace_id=$1 AND user_id=$2`, workspaceID, ownerID); err != nil {
			t.Fatal(err)
		}
		approved, err := service.Execute(ctx, ownerPrincipal, response)
		if err != nil {
			t.Fatal(err)
		}
		if approved.ActionState != "admitted" || len(approved.ActionIDs) != 1 {
			t.Fatalf("approve receipt=%+v", approved)
		}
		approvedReplay, err := service.Execute(ctx, ownerPrincipal, response)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(approved, approvedReplay) {
			t.Fatalf("approve replay changed: %+v %+v", approved, approvedReplay)
		}
		if _, err := service.Execute(ctx, wrongOwner, response); !errors.Is(err, workflowcommand.ErrForbidden) {
			t.Fatalf("wrong replay owner principal kind error=%v", err)
		}
		for table, want := range map[string]int{"issue_workflow_command": 2, "issue_workflow_decision": 1, "issue_workflow_decision_response": 1, "issue_workflow_action_receipt": 2, "issue_current_picture": 1} {
			var count int
			if err := conn.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE workspace_id=$1 AND issue_id=$2", workspaceID, issueID).Scan(&count); err != nil || count != want {
				t.Fatalf("%s count=%d err=%v", table, count, err)
			}
		}
		worker := workflowaction.Worker{Starter: conn, Queries: queries, ControllerID: controllerID, CommandService: service, Now: now, LeaseDuration: time.Minute, RetryDelay: time.Second, Effect: workflowaction.EffectFuncs{ObserveFunc: func(context.Context, generated.IssueWorkflowActionReceipt) (workflowaction.Observation, error) {
			return workflowaction.Observation{}, errors.New("decision continuation must not use an external effect")
		}, ApplyFunc: func(context.Context, generated.IssueWorkflowActionReceipt) (workflowaction.Observation, error) {
			return workflowaction.Observation{}, errors.New("decision continuation must not use an external effect")
		}}, Capacity: workflowaction.CapacityFunc(func(context.Context, generated.IssueWorkflowActionReceipt) (bool, error) { return true, nil })}
		if err := worker.ReconcileOnce(ctx); err != nil {
			t.Fatalf("settle durable owner decision continuation: %v", err)
		}
		var decisionState, resolutionState, executionState, attentionState string
		if err := conn.QueryRow(ctx, `SELECT state,resolution_state FROM issue_workflow_decision WHERE workspace_id=$1 AND issue_id=$2 AND id=$3`, workspaceID, issueID, workflowV2UUIDString(decisionID)).Scan(&decisionState, &resolutionState); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT state FROM issue_workflow_execution WHERE workspace_id=$1 AND issue_id=$2 AND id=$3`, workspaceID, issueID, operationID).Scan(&executionState); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT resolution_state FROM issue_workflow_attention WHERE workspace_id=$1 AND issue_id=$2 AND decision_id=$3 ORDER BY revision DESC LIMIT 1`, workspaceID, issueID, workflowV2UUIDString(decisionID)).Scan(&attentionState); err != nil {
			t.Fatal(err)
		}
		if decisionState != "answered" || resolutionState != "resolved" || executionState != "working" || attentionState != "resolved" {
			t.Fatalf("settled decision=%s/%s execution=%s attention=%s", decisionState, resolutionState, executionState, attentionState)
		}
		rows, err := conn.Query(ctx, `SELECT action_type,state FROM issue_workflow_action_receipt WHERE workspace_id=$1 AND issue_id=$2 ORDER BY action_type`, workspaceID, issueID)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var actionType, state string
			if err := rows.Scan(&actionType, &state); err != nil || state != "settled" {
				t.Fatalf("action %s state=%s err=%v", actionType, state, err)
			}
		}
	})

	t.Run("command service applies review failure with exact reviewer lease and every evidence reference", func(t *testing.T) {
		workspaceID, issueID := seedLoadableWorkflowV2(t, ctx, conn, 221)
		executionID, candidateID := workflowV2UUID(226), workflowV2UUID(232)
		implementerID, reviewerID, runtimeID := workflowV2UUID(234), workflowV2UUID(235), workflowV2UUID(239)
		taskID, leaseToken := workflowV2UUID(242), workflowV2UUID(240)
		actionID, sourceCommandID := workflowV2UUID(241), workflowV2UUID(243)
		evidenceOne, evidenceTwo := workflowV2UUID(244), workflowV2UUID(245)
		nowValue := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
		for _, statement := range []struct {
			sql  string
			args []any
		}{
			{`UPDATE issue_workflow_execution SET state='in_review', actor_id=$3, task_id=$4 WHERE workspace_id=$1 AND issue_id=$2`, []any{workspaceID, issueID, reviewerID, taskID}},
			{`UPDATE issue_workflow_review SET outcome='in_progress', criteria='[]', coverage='{}', evidence_refs='[]' WHERE workspace_id=$1 AND issue_id=$2`, []any{workspaceID, issueID}},
			{`UPDATE issue_workflow_concern_head SET state='in_review' WHERE workspace_id=$1 AND issue_id=$2 AND concern_type='execution'`, []any{workspaceID, issueID}},
			{`UPDATE issue_workflow_concern_head SET state='in_progress' WHERE workspace_id=$1 AND issue_id=$2 AND concern_type='review'`, []any{workspaceID, issueID}},
			{`INSERT INTO agent_runtime(id,workspace_id) VALUES($1,$2)`, []any{runtimeID, workspaceID}},
			{`INSERT INTO agent(id,workspace_id,runtime_id) VALUES($1,$2,$3)`, []any{implementerID, workspaceID, runtimeID}},
			{`INSERT INTO agent(id,workspace_id) VALUES($1,$2)`, []any{reviewerID, workspaceID}},
			{`INSERT INTO agent_task_queue(id,agent_id,issue_id,status,workflow_action_id,workflow_operation_id,workflow_branch_key) VALUES($1,$2,$3,'running',$4,$5,'main')`, []any{taskID, reviewerID, issueID, actionID, executionID}},
			{`INSERT INTO issue_workflow_action_receipt(id,workspace_id,issue_id,action_key,request_hash,command_id,action_type,state,operation_id,payload,bound_task_id,lease_token,lease_expires_at) VALUES($1,$2,$3,'review/leased','seed',$4,'review_candidate','settled',$5,jsonb_build_object('branch_key','main','reviewer_id',$7::text,'candidate_id',$8::text,'scope_revision',1),$6,$9,$10)`, []any{actionID, workspaceID, issueID, sourceCommandID, executionID, taskID, reviewerID, candidateID, leaseToken, nowValue.Add(time.Hour)}},
		} {
			if _, err := conn.Exec(ctx, statement.sql, statement.args...); err != nil {
				t.Fatal(err)
			}
		}
		for _, evidenceID := range []pgtype.UUID{evidenceOne, evidenceTwo} {
			if _, err := conn.Exec(ctx, `INSERT INTO issue_workflow_evidence(id,workspace_id,issue_id,concern_type,evidence_type,level,source,producer_type,producer_id,candidate_id,observed_at,payload) VALUES($1,$2,$3,'review','technical_review','technical','review-task','reviewer',$4,$5,$6,jsonb_build_object('candidate_revision',1,'scope_id',$7::text,'scope_revision',1,'freshness','current'))`, evidenceID, workspaceID, issueID, reviewerID, candidateID, nowValue, workflowV2UUID(224)); err != nil {
				t.Fatal(err)
			}
		}
		now := func() time.Time { return nowValue }
		service := workflowcommand.Service{Starter: conn, Queries: queries, Loader: workflowcommand.Loader{Now: now}, Now: now}
		principal := workflowcommand.Principal{ActorID: reviewerID, ActorType: "agent_task", TaskID: taskID, ActionKey: "review/leased", LeaseToken: leaseToken}
		request := workflowcommand.Request{WorkspaceID: workspaceID, IssueID: issueID, CommandKey: "review-failed", Kind: workflowstatev2.CommandReviewFailed, Expected: []workflowstatev2.ExpectedRevision{{Concern: workflowstatev2.ConcernExecution, Revision: 1}, {Concern: workflowstatev2.ConcernReview, Revision: 1}, {Concern: workflowstatev2.ConcernEvidence, Revision: 1}}, EvidenceRefs: []string{util.UUIDToString(evidenceOne), util.UUIDToString(workflowV2UUID(246))}, CriteriaRefs: []string{"acceptance"}, CoverageRefs: []string{"tests"}, GapRefs: []string{"failure"}, RequiredCorrection: "repair boundary", CorrectionCount: 1, MaxCorrections: 2}
		request.EvidenceRefs[1] = util.UUIDToString(evidenceTwo)
		wrongType := principal
		wrongType.ActorType = "member"
		wrongTypeRequest := request
		wrongTypeRequest.CommandKey = "review-wrong-principal-kind"
		if _, err := service.Execute(ctx, wrongType, wrongTypeRequest); !errors.Is(err, workflowcommand.ErrForbidden) {
			t.Fatalf("wrong initial reviewer principal kind error=%v", err)
		}
		for table, want := range map[string]int{"issue_workflow_command": 0, "issue_workflow_action_receipt": 1, "issue_workflow_review": 1, "issue_current_picture": 0} {
			var count int
			if err := conn.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE workspace_id=$1 AND issue_id=$2", workspaceID, issueID).Scan(&count); err != nil || count != want {
				t.Fatalf("wrong-type %s residue=%d want=%d err=%v", table, count, want, err)
			}
		}
		var advancedHeads int
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_concern_head WHERE workspace_id=$1 AND issue_id=$2 AND revision<>1`, workspaceID, issueID).Scan(&advancedHeads); err != nil || advancedHeads != 0 {
			t.Fatalf("wrong-type advanced heads=%d err=%v", advancedHeads, err)
		}
		assertLockedTransition := func(name, mutate, restore string, id pgtype.UUID) {
			t.Helper()
			entered, release := make(chan struct{}), make(chan struct{})
			injectedRace := errors.New("stop after authority lock")
			racingService := service
			racingService.Failpoint = func(stage string) error {
				if stage == "after_authority" {
					close(entered)
					<-release
					return injectedRace
				}
				return nil
			}
			racingRequest := request
			racingRequest.CommandKey = "review-authority-race-" + name
			serviceDone := make(chan error, 1)
			go func() { _, err := racingService.Execute(ctx, principal, racingRequest); serviceDone <- err }()
			<-entered
			other, err := workflowV2TestConnection(ctx)
			if err != nil {
				t.Fatal(err)
			}
			updateDone := make(chan error, 1)
			go func() { _, err := other.Exec(ctx, mutate, id); updateDone <- err }()
			select {
			case err := <-updateDone:
				t.Fatalf("%s transition bypassed reviewer authority lock: %v", name, err)
			case <-time.After(100 * time.Millisecond):
			}
			close(release)
			if err := <-serviceDone; !errors.Is(err, injectedRace) {
				t.Fatalf("paused service error=%v", err)
			}
			if err := <-updateDone; err != nil {
				t.Fatal(err)
			}
			_ = other.Close(ctx)
			if _, err := conn.Exec(ctx, restore, id); err != nil {
				t.Fatal(err)
			}
		}
		assertLockedTransition("task", `UPDATE agent_task_queue SET status='finished' WHERE id=$1`, `UPDATE agent_task_queue SET status='running' WHERE id=$1`, taskID)
		assertLockedTransition("agent", `UPDATE agent SET archived_at=now() WHERE id=$1`, `UPDATE agent SET archived_at=NULL WHERE id=$1`, reviewerID)
		assertLockedTransition("action", `UPDATE issue_workflow_action_receipt SET state='dead_letter' WHERE id=$1`, `UPDATE issue_workflow_action_receipt SET state='settled' WHERE id=$1`, actionID)
		for _, tc := range []struct {
			name, mutate, restore string
			principal             workflowcommand.Principal
		}{
			{"finished task", `UPDATE agent_task_queue SET status='finished' WHERE id=$1`, `UPDATE agent_task_queue SET status='running' WHERE id=$1`, principal},
			{"archived agent", `UPDATE agent SET archived_at=now() WHERE id=$1`, `UPDATE agent SET archived_at=NULL WHERE id=$1`, principal},
			{"dead action", `UPDATE issue_workflow_action_receipt SET state='dead_letter' WHERE id=$1`, `UPDATE issue_workflow_action_receipt SET state='settled' WHERE id=$1`, principal},
			{"wrong task binding", `UPDATE issue_workflow_action_receipt SET bound_task_id=NULL WHERE id=$1`, `UPDATE issue_workflow_action_receipt SET bound_task_id=$2 WHERE id=$1`, principal},
		} {
			id := actionID
			if tc.name == "finished task" {
				id = taskID
			} else if tc.name == "archived agent" {
				id = reviewerID
			}
			if _, err := conn.Exec(ctx, tc.mutate, id); err != nil {
				t.Fatal(err)
			}
			attempt := request
			attempt.CommandKey = "reject-stale-" + tc.name
			if _, err := service.Execute(ctx, tc.principal, attempt); !errors.Is(err, workflowcommand.ErrForbidden) {
				t.Fatalf("%s error=%v", tc.name, err)
			}
			var restoreErr error
			if tc.name == "wrong task binding" {
				_, restoreErr = conn.Exec(ctx, tc.restore, id, taskID)
			} else {
				_, restoreErr = conn.Exec(ctx, tc.restore, id)
			}
			if restoreErr != nil {
				t.Fatal(restoreErr)
			}
		}
		if _, err := conn.Exec(ctx, `UPDATE issue_workflow_evidence SET producer_id=$1 WHERE id=$2`, workflowV2UUID(234), evidenceTwo); err != nil {
			t.Fatal(err)
		}
		attempt := request
		attempt.CommandKey = "review-wrong-second-producer"
		if _, err := service.Execute(ctx, principal, attempt); err == nil {
			t.Fatalf("valid first and wrong-producer second evidence error=%v", err)
		}
		if _, err := conn.Exec(ctx, `UPDATE issue_workflow_evidence SET producer_id=$1 WHERE id=$2`, reviewerID, evidenceTwo); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, `UPDATE issue_workflow_evidence SET valid_until=$1 WHERE id=$2`, nowValue, evidenceTwo); err != nil {
			t.Fatal(err)
		}
		expiredAttempt := request
		expiredAttempt.CommandKey = "review-expired-second-evidence"
		if _, err := service.Execute(ctx, principal, expiredAttempt); err == nil {
			t.Fatalf("expired non-head evidence error=%v", err)
		}
		if _, err := conn.Exec(ctx, `UPDATE issue_workflow_evidence SET valid_until=NULL WHERE id=$1`, evidenceTwo); err != nil {
			t.Fatal(err)
		}
		request.EvidenceRefs[1] = util.UUIDToString(workflowV2UUID(246))
		if _, err := service.Execute(ctx, principal, request); err == nil {
			t.Fatal("valid first and missing second evidence reference was accepted")
		}
		var rejectedRows int
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_command WHERE workspace_id=$1 AND issue_id=$2 AND command_key='review-failed'`, workspaceID, issueID).Scan(&rejectedRows); err != nil || rejectedRows != 0 {
			t.Fatalf("rejected command residue=%d err=%v", rejectedRows, err)
		}
		request.EvidenceRefs[1] = util.UUIDToString(evidenceTwo)
		first, err := service.Execute(ctx, principal, request)
		if err != nil {
			t.Fatal(err)
		}
		replay, err := service.Execute(ctx, principal, request)
		if err != nil || !reflect.DeepEqual(first, replay) {
			t.Fatalf("stored replay=(%+v,%v), first=%+v", replay, err, first)
		}
		if first.ActionState != "admitted" || len(first.ActionIDs) != 1 {
			t.Fatalf("receipt=%+v", first)
		}
		if _, err := conn.Exec(ctx, `UPDATE issue_workflow_action_receipt SET state='committed',lease_expires_at=$1 WHERE id=$2`, nowValue.Add(-time.Second), actionID); err != nil {
			t.Fatal(err)
		}
		settledReplay, err := service.Execute(ctx, principal, request)
		if err != nil || !reflect.DeepEqual(first, settledReplay) {
			t.Fatalf("settled source action replay=(%+v,%v), first=%+v", settledReplay, err, first)
		}
		if _, err := service.Execute(ctx, wrongType, request); !errors.Is(err, workflowcommand.ErrForbidden) {
			t.Fatalf("wrong replay reviewer principal kind error=%v", err)
		}
		var actionType, actionKey, operation, payloadOperation, payloadBranch, payloadCandidate, payloadScope, payloadReview, payloadWorker string
		var payloadCandidateRevision, payloadScopeRevision, payloadAcceptanceVersion, payloadReviewRevision int64
		correctionActionID := workflowV2UUIDString(first.ActionIDs[0])
		if err := conn.QueryRow(ctx, `SELECT action_type,action_key,operation_id::text,payload->>'operation_id',payload->>'branch_key',payload->>'candidate_id',(payload->>'candidate_revision')::bigint,payload->>'scope_id',(payload->>'scope_revision')::bigint,(payload->>'acceptance_version')::bigint,payload->>'review_id',(payload->>'review_revision')::bigint,payload->>'correction_worker_id' FROM issue_workflow_action_receipt WHERE id=$1`, correctionActionID).Scan(&actionType, &actionKey, &operation, &payloadOperation, &payloadBranch, &payloadCandidate, &payloadCandidateRevision, &payloadScope, &payloadScopeRevision, &payloadAcceptanceVersion, &payloadReview, &payloadReviewRevision, &payloadWorker); err != nil {
			t.Fatal(err)
		}
		expectedReviewID := stableWorkflowCommandUUID(workspaceID, issueID, request.CommandKey, "review")
		if actionType != "review_correction" || !strings.Contains(actionKey, first.CommandID) || operation != util.UUIDToString(executionID) || payloadOperation != operation || payloadBranch != "main" || payloadCandidate != util.UUIDToString(candidateID) || payloadCandidateRevision != 1 || payloadScope != util.UUIDToString(workflowV2UUID(224)) || payloadScopeRevision != 1 || payloadAcceptanceVersion != 1 || payloadReview != expectedReviewID || payloadReviewRevision != 1 || payloadWorker != util.UUIDToString(implementerID) {
			t.Fatalf("frozen correction action payload mismatch: candidate=%s/%d scope=%s/%d acceptance=%d review=%s/%d worker=%s", payloadCandidate, payloadCandidateRevision, payloadScope, payloadScopeRevision, payloadAcceptanceVersion, payloadReview, payloadReviewRevision, payloadWorker)
		}
		for concern, wantRevision := range map[string]int64{"execution": 2, "review": 2, "evidence": 2} {
			var got int64
			if err := conn.QueryRow(ctx, `SELECT revision FROM issue_workflow_concern_head WHERE workspace_id=$1 AND issue_id=$2 AND concern_type=$3`, workspaceID, issueID, concern).Scan(&got); err != nil || got != wantRevision {
				t.Fatalf("%s head revision=%d err=%v", concern, got, err)
			}
		}
		worker := workflowaction.Worker{
			Starter: conn,
			Queries: queries,
			Effect:  workflowaction.LocalTaskEffect{Starter: conn, Queries: queries, Now: now},
			Capacity: workflowaction.CapacityFunc(func(context.Context, generated.IssueWorkflowActionReceipt) (bool, error) {
				return true, nil
			}),
			Now: now,
		}
		if err := worker.ReconcileOnce(ctx); err != nil {
			t.Fatal(err)
		}
		var correctionAgent pgtype.UUID
		var correctionTaskCount int
		if err := conn.QueryRow(ctx, `SELECT count(*), min(agent_id::text)::uuid FROM agent_task_queue WHERE workflow_action_id=$1 AND workflow_operation_id=$2 AND workflow_branch_key='main'`, correctionActionID, executionID).Scan(&correctionTaskCount, &correctionAgent); err != nil {
			t.Fatal(err)
		}
		if correctionTaskCount != 1 || correctionAgent != implementerID || correctionAgent == reviewerID {
			t.Fatalf("correction task count=%d agent=%s implementer=%s reviewer=%s", correctionTaskCount, util.UUIDToString(correctionAgent), util.UUIDToString(implementerID), util.UUIDToString(reviewerID))
		}
		var taskCandidate, taskScope, taskReview string
		var taskCandidateRevision, taskScopeRevision, taskAcceptanceVersion, taskReviewRevision int64
		if err := conn.QueryRow(ctx, `SELECT context->>'candidate_id',(context->>'candidate_revision')::bigint,context->>'scope_id',(context->>'scope_revision')::bigint,(context->>'acceptance_version')::bigint,context->>'review_id',(context->>'review_revision')::bigint FROM agent_task_queue WHERE workflow_action_id=$1`, correctionActionID).Scan(&taskCandidate, &taskCandidateRevision, &taskScope, &taskScopeRevision, &taskAcceptanceVersion, &taskReview, &taskReviewRevision); err != nil {
			t.Fatal(err)
		}
		if taskCandidate != payloadCandidate || taskCandidateRevision != payloadCandidateRevision || taskScope != payloadScope || taskScopeRevision != payloadScopeRevision || taskAcceptanceVersion != payloadAcceptanceVersion || taskReview != payloadReview || taskReviewRevision != payloadReviewRevision {
			t.Fatal("correction task did not preserve the admitted failed-review snapshot")
		}
		request.CommandKey = "review-replay-other-action"
		principal.ActionKey = "review/other"
		if _, err := service.Execute(ctx, principal, request); !errors.Is(err, workflowcommand.ErrForbidden) {
			t.Fatalf("wrong action binding error=%v", err)
		}
	})

	t.Run("command service persists authorization rejection as resuming replan", func(t *testing.T) {
		for _, tc := range []struct {
			name, actionType, executionState, outcomeState string
			outcome                                        workflowstatev2.DecisionOutcome
		}{
			{name: "reject", outcome: workflowstatev2.DecisionReject, actionType: "replan_exact_operation", executionState: "paused", outcomeState: "open"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				tx, err := conn.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback(ctx)
				q := generated.New(tx)
				workspaceID, issueID := seedLoadableWorkflowV2(t, ctx, tx, 60)
				controllerID, ownerID, operationID := workflowV2UUID(76), workflowV2UUID(72), workflowV2UUID(65)
				if _, err := tx.Exec(ctx, `INSERT INTO member(id,workspace_id,user_id,role) VALUES($1,$2,$3,'owner')`, workflowV2UUID(77), workspaceID, ownerID); err != nil {
					t.Fatal(err)
				}
				now := func() time.Time { return time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC) }
				service := workflowcommand.Service{Starter: tx, Queries: q, Loader: workflowcommand.Loader{Now: now}, Now: now, ControllerID: controllerID}
				request := authorizationDecisionRequest(workspaceID, issueID, "request-"+tc.name, operationID, workflowV2UUID(71), ownerID)
				if _, err := service.Execute(ctx, workflowcommand.Principal{ActorID: controllerID, ActorType: "internal_controller"}, request); err != nil {
					t.Fatal(err)
				}
				decisionID := stableWorkflowCommandUUID(workspaceID, issueID, request.CommandKey, "decision")
				response := workflowcommand.Request{WorkspaceID: workspaceID, IssueID: issueID, CommandKey: "respond-" + tc.name, Kind: workflowstatev2.CommandRespondDecision, DecisionID: decisionID, DecisionOutcome: tc.outcome, Detail: tc.name + " reason", ExpectedDecisionRevision: 1, Expected: []workflowstatev2.ExpectedRevision{{Concern: workflowstatev2.ConcernExecution, Revision: 2}, {Concern: workflowstatev2.ConcernAttention, Revision: 2}}}
				principal := workflowcommand.Principal{ActorID: ownerID, ActorType: "member"}
				if _, err := tx.Exec(ctx, `UPDATE member SET role='admin' WHERE workspace_id=$1 AND user_id=$2`, workspaceID, ownerID); err != nil {
					t.Fatal(err)
				}
				unauthorized := response
				unauthorized.CommandKey += "-admin"
				if _, err := service.Execute(ctx, principal, unauthorized); !errors.Is(err, workflowcommand.ErrForbidden) {
					t.Fatalf("admin-not-owner error=%v", err)
				}
				var unauthorizedCommands int
				if err := tx.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_command WHERE workspace_id=$1 AND issue_id=$2 AND command_key=$3`, workspaceID, issueID, unauthorized.CommandKey).Scan(&unauthorizedCommands); err != nil || unauthorizedCommands != 0 {
					t.Fatalf("downgraded owner command residue=%d err=%v", unauthorizedCommands, err)
				}
				if _, err := tx.Exec(ctx, `UPDATE member SET role='owner' WHERE workspace_id=$1 AND user_id=$2`, workspaceID, ownerID); err != nil {
					t.Fatal(err)
				}
				first, err := service.Execute(ctx, principal, response)
				if err != nil {
					t.Fatal(err)
				}
				replay, err := service.Execute(ctx, principal, response)
				if err != nil || !reflect.DeepEqual(first, replay) {
					t.Fatalf("stored replay=(%+v,%v), first=%+v", replay, err, first)
				}
				stale := response
				stale.CommandKey += "-stale"
				if _, err := service.Execute(ctx, principal, stale); !errors.Is(err, workflowcommand.ErrDecisionStale) {
					t.Fatalf("answered decision error=%v, want ErrDecisionStale", err)
				}
				var staleCommands int
				if err := tx.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_command WHERE workspace_id=$1 AND issue_id=$2 AND command_key=$3`, workspaceID, issueID, stale.CommandKey).Scan(&staleCommands); err != nil || staleCommands != 0 {
					t.Fatalf("stale command residue=%d err=%v", staleCommands, err)
				}
				if len(first.ActionIDs) != 1 {
					t.Fatalf("receipt=%+v", first)
				}
				var actionType, actionDecision, payloadDecision, responseType, responseText, executionState, outcomeState string
				if err := tx.QueryRow(ctx, `SELECT action_type,decision_id::text,payload->>'decision_id' FROM issue_workflow_action_receipt WHERE id=$1`, workflowV2UUIDString(first.ActionIDs[0])).Scan(&actionType, &actionDecision, &payloadDecision); err != nil {
					t.Fatal(err)
				}
				if err := tx.QueryRow(ctx, `SELECT response_type,content->>'text' FROM issue_workflow_decision_response WHERE workspace_id=$1 AND issue_id=$2 AND decision_id=$3`, workspaceID, issueID, workflowV2UUIDString(decisionID)).Scan(&responseType, &responseText); err != nil {
					t.Fatal(err)
				}
				if err := tx.QueryRow(ctx, `SELECT state FROM issue_workflow_execution WHERE id=$1`, operationID).Scan(&executionState); err != nil {
					t.Fatal(err)
				}
				if err := tx.QueryRow(ctx, `SELECT state FROM issue_workflow_outcome WHERE workspace_id=$1 AND issue_id=$2`, workspaceID, issueID).Scan(&outcomeState); err != nil {
					t.Fatal(err)
				}
				if actionType != tc.actionType || actionDecision != decisionID || payloadDecision != decisionID || responseType != string(tc.outcome) || responseText != response.Detail || executionState != tc.executionState || outcomeState != tc.outcomeState {
					t.Fatalf("persisted action=(%s,%s,%s) response=(%s,%s) state=(%s,%s)", actionType, actionDecision, payloadDecision, responseType, responseText, executionState, outcomeState)
				}
			})
		}
	})

	t.Run("command service rolls back every transaction stage and retries cleanly", func(t *testing.T) {
		for index, stage := range []string{"after_actions", "after_domain", "after_heads", "after_projection", "after_receipt"} {
			t.Run(stage, func(t *testing.T) {
				tx, err := conn.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback(ctx)
				base := byte(90 + index*20)
				workspaceID, issueID := seedLoadableWorkflowV2(t, ctx, tx, base)
				controllerID, ownerID, operationID := workflowV2UUID(base+17), workflowV2UUID(base+12), workflowV2UUID(base+5)
				if _, err := tx.Exec(ctx, `INSERT INTO member(id,workspace_id,user_id,role) VALUES($1,$2,$3,'owner')`, workflowV2UUID(base+18), workspaceID, ownerID); err != nil {
					t.Fatal(err)
				}
				now := func() time.Time { return time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC) }
				injected := errors.New("injected " + stage)
				service := workflowcommand.Service{Starter: tx, Queries: generated.New(tx), Loader: workflowcommand.Loader{Now: now}, Now: now, ControllerID: controllerID, Failpoint: func(got string) error {
					if got == stage {
						return injected
					}
					return nil
				}}
				request := authorizationDecisionRequest(workspaceID, issueID, "failure-"+stage, operationID, workflowV2UUID(base+11), ownerID)
				principal := workflowcommand.Principal{ActorID: controllerID, ActorType: "internal_controller"}
				if _, err := service.Execute(ctx, principal, request); !errors.Is(err, injected) {
					t.Fatalf("error=%v", err)
				}
				for _, table := range []string{"issue_workflow_command", "issue_workflow_decision", "issue_workflow_action_receipt", "issue_current_picture"} {
					var count int
					if err := tx.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE workspace_id=$1 AND issue_id=$2", workspaceID, issueID).Scan(&count); err != nil || count != 0 {
						t.Fatalf("%s residue=%d err=%v", table, count, err)
					}
				}
				service.Failpoint = nil
				if _, err := service.Execute(ctx, principal, request); err != nil {
					t.Fatalf("clean retry: %v", err)
				}
			})
		}
	})

	t.Run("command batch later failure rolls back earlier command and does not wake", func(t *testing.T) {
		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		workspaceID, issueID := seedLoadableWorkflowV2(t, ctx, tx, 30)
		controllerID, ownerID, operationID := workflowV2UUID(47), workflowV2UUID(42), workflowV2UUID(35)
		if _, err := tx.Exec(ctx, `INSERT INTO member(id,workspace_id,user_id,role) VALUES($1,$2,$3,'owner')`, workflowV2UUID(48), workspaceID, ownerID); err != nil {
			t.Fatal(err)
		}
		now := func() time.Time { return time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC) }
		wakes := 0
		service := workflowcommand.Service{Starter: tx, Queries: generated.New(tx), Loader: workflowcommand.Loader{Now: now}, Now: now, ControllerID: controllerID, Wake: func(context.Context, []string) error { wakes++; return nil }}
		first := authorizationDecisionRequest(workspaceID, issueID, "batch-first", operationID, workflowV2UUID(41), ownerID)
		second := first
		second.CommandKey, second.Kind = "batch-second", workflowstatev2.CommandQueue
		if _, err := service.ExecuteBatch(ctx, workflowcommand.Principal{ActorID: controllerID, ActorType: "internal_controller"}, []workflowcommand.Request{first, second}); !errors.Is(err, workflowcommand.ErrUnsupportedCommand) {
			t.Fatalf("batch error=%v", err)
		}
		var commands int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_command WHERE workspace_id=$1 AND issue_id=$2`, workspaceID, issueID).Scan(&commands); err != nil || commands != 0 || wakes != 0 {
			t.Fatalf("commands=%d wakes=%d err=%v", commands, wakes, err)
		}
	})

	t.Run("technical stop executes only through exact admitted capability", func(t *testing.T) {
		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		q := generated.New(tx)
		workspaceID, issueID := seedLoadableWorkflowV2(t, ctx, tx, 80)
		controllerID, operationID := workflowV2UUID(98), workflowV2UUID(85)
		now := func() time.Time { return time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC) }
		wakes := 0
		service := workflowcommand.Service{Loader: workflowcommand.Loader{Now: now}, Now: now, ControllerID: controllerID, Wake: func(context.Context, []string) error { wakes++; return nil }}
		principal := workflowcommand.Principal{ActorID: controllerID, ActorType: "internal_controller"}
		request := workflowcommand.Request{WorkspaceID: workspaceID, IssueID: issueID, CommandKey: "action/00000000-0000-0000-0000-000000000099/dead-letter", Kind: workflowstatev2.CommandTechnicalStop, OperationID: util.UUIDToString(operationID), Detail: "review correction retry ceiling exhausted", Expected: []workflowstatev2.ExpectedRevision{{Concern: workflowstatev2.ConcernExecution, Revision: 1}}}
		run := func(p workflowcommand.Principal, r workflowcommand.Request, callbackErr error) (workflowcommand.Receipt, error) {
			var receipt workflowcommand.Receipt
			err := workflowstore.InIssueWriteTransaction(ctx, tx, q, r.WorkspaceID, r.IssueID, func(write *workflowstore.IssueWrite) error {
				var err error
				receipt, err = service.ExecuteInWrite(ctx, write, p, r)
				if err != nil {
					return err
				}
				return callbackErr
			})
			return receipt, err
		}
		assertZeroResidue := func(label string) {
			t.Helper()
			for table, want := range map[string]int{"issue_workflow_command": 0, "issue_workflow_action_receipt": 0, "issue_current_picture": 0} {
				var count int
				if err := tx.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE workspace_id=$1 AND issue_id=$2", workspaceID, issueID).Scan(&count); err != nil || count != want {
					t.Fatalf("%s %s count=%d err=%v", label, table, count, err)
				}
			}
			var executionRevision, headRevision int64
			var executionState, terminalReason string
			if err := tx.QueryRow(ctx, `SELECT revision,state,terminal_reason FROM issue_workflow_execution WHERE id=$1`, operationID).Scan(&executionRevision, &executionState, &terminalReason); err != nil || executionRevision != 1 || executionState != "working" || terminalReason != "" {
				t.Fatalf("%s execution=(%d,%s,%q) err=%v", label, executionRevision, executionState, terminalReason, err)
			}
			if err := tx.QueryRow(ctx, `SELECT revision FROM issue_workflow_concern_head WHERE workspace_id=$1 AND issue_id=$2 AND concern_type='execution'`, workspaceID, issueID).Scan(&headRevision); err != nil || headRevision != 1 {
				t.Fatalf("%s execution head=%d err=%v", label, headRevision, err)
			}
		}
		wrongID := principal
		wrongID.ActorID = workflowV2UUID(99)
		wrongType := principal
		wrongType.ActorType = "member"
		missingOperation := request
		missingOperation.OperationID = ""
		wrongOperation := request
		wrongOperation.OperationID = util.UUIDToString(workflowV2UUID(97))
		omittedRevision := request
		omittedRevision.Expected = nil
		staleRevision := request
		staleRevision.Expected = append([]workflowstatev2.ExpectedRevision(nil), request.Expected...)
		staleRevision.Expected[0].Revision = 2
		for _, tc := range []struct {
			name      string
			principal workflowcommand.Principal
			request   workflowcommand.Request
		}{{"wrong controller", wrongID, request}, {"wrong type", wrongType, request}, {"missing operation", principal, missingOperation}, {"wrong operation", principal, wrongOperation}, {"omitted revision", principal, omittedRevision}, {"stale revision", principal, staleRevision}} {
			if _, err := run(tc.principal, tc.request, nil); !errors.Is(err, workflowcommand.ErrForbidden) {
				t.Fatalf("%s error=%v", tc.name, err)
			}
			assertZeroResidue(tc.name)
		}

		foreignWorkspace, foreignIssue := workflowV2UUID(100), workflowV2UUID(101)
		if _, err := tx.Exec(ctx, `INSERT INTO workspace(id) VALUES($1)`, foreignWorkspace); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO issue(id,workspace_id) VALUES($1,$2)`, foreignIssue, foreignWorkspace); err != nil {
			t.Fatal(err)
		}
		for _, foreign := range []workflowcommand.Request{
			func() workflowcommand.Request { v := request; v.IssueID = foreignIssue; return v }(),
			func() workflowcommand.Request {
				v := request
				v.WorkspaceID, v.IssueID = foreignWorkspace, foreignIssue
				return v
			}(),
		} {
			err := workflowstore.InIssueWriteTransaction(ctx, tx, q, workspaceID, issueID, func(write *workflowstore.IssueWrite) error {
				_, err := service.ExecuteInWrite(ctx, write, principal, foreign)
				return err
			})
			if !errors.Is(err, workflowstore.ErrInvalidIssueWriteCapability) {
				t.Fatalf("foreign admitted capability error=%v", err)
			}
			assertZeroResidue("foreign capability")
		}
		var escaped *workflowstore.IssueWrite
		if err := workflowstore.InIssueWriteTransaction(ctx, tx, q, workspaceID, issueID, func(write *workflowstore.IssueWrite) error {
			escaped = write
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := service.ExecuteInWrite(ctx, escaped, principal, request); !errors.Is(err, workflowstore.ErrInvalidIssueWriteCapability) {
			t.Fatalf("expired callback capability error=%v", err)
		}
		assertZeroResidue("expired capability")

		forcedRollback := errors.New("outer callback rollback")
		if _, err := run(principal, request, forcedRollback); !errors.Is(err, forcedRollback) {
			t.Fatalf("outer rollback error=%v", err)
		}
		assertZeroResidue("outer rollback")

		first, err := run(principal, request, nil)
		if err != nil {
			t.Fatal(err)
		}
		if first.State != "applied" || first.ActionState != "none" || len(first.ActionIDs) != 0 || wakes != 0 {
			t.Fatalf("receipt/wakes=(%+v,%d)", first, wakes)
		}
		var actionCount int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_action_receipt WHERE workspace_id=$1 AND issue_id=$2`, workspaceID, issueID).Scan(&actionCount); err != nil || actionCount != 0 {
			t.Fatalf("technical stop action count=%d err=%v", actionCount, err)
		}
		var state, terminalReason, projectedReason string
		var revision, headRevision int64
		if err := tx.QueryRow(ctx, `SELECT state,terminal_reason,revision FROM issue_workflow_execution WHERE id=$1`, operationID).Scan(&state, &terminalReason, &revision); err != nil {
			t.Fatal(err)
		}
		if err := tx.QueryRow(ctx, `SELECT revision FROM issue_workflow_concern_head WHERE workspace_id=$1 AND issue_id=$2 AND concern_type='execution'`, workspaceID, issueID).Scan(&headRevision); err != nil {
			t.Fatal(err)
		}
		if err := tx.QueryRow(ctx, `SELECT terminal_reason FROM issue_current_picture WHERE workspace_id=$1 AND issue_id=$2`, workspaceID, issueID).Scan(&projectedReason); err != nil {
			t.Fatal(err)
		}
		if state != "technically_stopped" || terminalReason != request.Detail || projectedReason != request.Detail || revision != 2 || headRevision != 2 {
			t.Fatalf("technical stop state=(%s,%q,%q,%d,%d)", state, terminalReason, projectedReason, revision, headRevision)
		}
		replay, err := run(principal, request, nil)
		if err != nil || !reflect.DeepEqual(first, replay) || wakes != 0 {
			t.Fatalf("replay=(%+v,%v) first=%+v wakes=%d", replay, err, first, wakes)
		}
		mismatch := request
		mismatch.Detail = "different terminal reason"
		if _, err := run(principal, mismatch, nil); !errors.Is(err, workflowcommand.ErrIdempotencyConflict) {
			t.Fatalf("mismatch error=%v", err)
		}
	})

	t.Run("issue write capability fences every exposed operation family", func(t *testing.T) {
		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		q := generated.New(tx)
		workspaceID, issueID := seedLoadableWorkflowV2(t, ctx, tx, 105)
		generatedAt := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
		sameWorkspaceIssue, foreignWorkspace, foreignIssue := workflowV2UUID(123), workflowV2UUID(124), workflowV2UUID(125)
		if _, err := tx.Exec(ctx, `INSERT INTO issue(id,workspace_id) VALUES($1,$2)`, sameWorkspaceIssue, workspaceID); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO workspace(id) VALUES($1)`, foreignWorkspace); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO issue(id,workspace_id) VALUES($1,$2)`, foreignIssue, foreignWorkspace); err != nil {
			t.Fatal(err)
		}
		var retained *workflowstore.IssueWrite
		asyncStart, asyncResult := make(chan struct{}), make(chan error, 1)
		if err := workflowstore.InIssueWriteTransaction(ctx, tx, q, workspaceID, issueID, func(write *workflowstore.IssueWrite) error {
			retained = write
			go func() {
				<-asyncStart
				_, err := write.ListProjectionInputs(context.Background(), generated.ListIssueWorkflowProjectionInputsParams{WorkspaceID: workspaceID, IssueID: issueID})
				asyncResult <- err
			}()
			for _, scope := range []struct{ workspaceID, issueID pgtype.UUID }{{workspaceID, sameWorkspaceIssue}, {foreignWorkspace, foreignIssue}} {
				if _, err := write.ListConcernHeads(ctx, generated.ListIssueWorkflowConcernHeadsParams{WorkspaceID: scope.workspaceID, IssueID: scope.issueID}); !errors.Is(err, workflowstore.ErrInvalidIssueWriteCapability) {
					t.Fatalf("foreign read error=%v", err)
				}
				if _, err := write.CreateOutcome(ctx, generated.CreateIssueWorkflowOutcomeParams{ID: workflowV2UUID(126), WorkspaceID: scope.workspaceID, IssueID: scope.issueID, State: "open", AcceptanceVersion: 1}); !errors.Is(err, workflowstore.ErrInvalidIssueWriteCapability) {
					t.Fatalf("foreign mutation error=%v", err)
				}
				if _, err := workflowprojection.ProjectInWrite(ctx, write, scope.workspaceID, scope.issueID, 0, 1, generatedAt); !errors.Is(err, workflowstore.ErrInvalidIssueWriteCapability) {
					t.Fatalf("foreign projection capability error=%v", err)
				}
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := retained.ListConcernHeads(ctx, generated.ListIssueWorkflowConcernHeadsParams{WorkspaceID: workspaceID, IssueID: issueID}); !errors.Is(err, workflowstore.ErrInvalidIssueWriteCapability) {
			t.Fatalf("post-callback read error=%v", err)
		}
		if _, err := retained.CreateOutcome(ctx, generated.CreateIssueWorkflowOutcomeParams{ID: workflowV2UUID(127), WorkspaceID: workspaceID, IssueID: issueID, State: "open", AcceptanceVersion: 1}); !errors.Is(err, workflowstore.ErrInvalidIssueWriteCapability) {
			t.Fatalf("post-callback mutation error=%v", err)
		}
		if _, err := workflowprojection.ProjectInWrite(ctx, retained, workspaceID, issueID, 0, 1, generatedAt); !errors.Is(err, workflowstore.ErrInvalidIssueWriteCapability) {
			t.Fatalf("post-callback projection capability error=%v", err)
		}
		close(asyncStart)
		if err := <-asyncResult; !errors.Is(err, workflowstore.ErrInvalidIssueWriteCapability) {
			t.Fatalf("async retained capability error=%v", err)
		}
		for _, scope := range []struct{ workspaceID, issueID pgtype.UUID }{{workspaceID, issueID}, {workspaceID, sameWorkspaceIssue}, {foreignWorkspace, foreignIssue}} {
			var pictures int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM issue_current_picture WHERE workspace_id=$1 AND issue_id=$2`, scope.workspaceID, scope.issueID).Scan(&pictures); err != nil || pictures != 0 {
				t.Fatalf("scope %s/%s projection residue=%d err=%v", util.UUIDToString(scope.workspaceID), util.UUIDToString(scope.issueID), pictures, err)
			}
		}
		var foreignOutcomes int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_outcome WHERE id IN ($1,$2)`, workflowV2UUID(126), workflowV2UUID(127)).Scan(&foreignOutcomes); err != nil || foreignOutcomes != 0 {
			t.Fatalf("capability mutation residue=%d err=%v", foreignOutcomes, err)
		}
	})

	t.Run("issue write capability expires before commit begins", func(t *testing.T) {
		workspaceID, issueID := workflowV2UUID(128), workflowV2UUID(129)
		if _, err := conn.Exec(ctx, `INSERT INTO workspace(id) VALUES($1)`, workspaceID); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, `INSERT INTO issue(id,workspace_id) VALUES($1,$2)`, issueID, workspaceID); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, _ = conn.Exec(context.Background(), `DELETE FROM issue WHERE id=$1`, issueID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM workspace WHERE id=$1`, workspaceID)
		})

		starter := newBlockingCommitStarter(conn)
		var retained, clone *workflowstore.IssueWrite
		done := make(chan error, 1)
		go func() {
			done <- workflowstore.InIssueWriteTransaction(ctx, starter, generated.New(conn), workspaceID, issueID, func(write *workflowstore.IssueWrite) error {
				retained = write
				copied := *write
				clone = &copied
				return nil
			})
		}()
		select {
		case <-starter.commitEntered:
		case <-time.After(5 * time.Second):
			starter.release()
			t.Fatal("timed out waiting for blocked commit")
		}
		t.Cleanup(starter.release)

		touches := starter.tx.touches.Load()
		for name, candidate := range map[string]*workflowstore.IssueWrite{"original": retained, "clone": clone} {
			if candidate == nil || candidate.MatchesIssue(workspaceID, issueID) {
				t.Fatalf("%s capability remained active after callback returned and commit began", name)
			}
			if _, err := candidate.ListConcernHeads(ctx, generated.ListIssueWorkflowConcernHeadsParams{WorkspaceID: workspaceID, IssueID: issueID}); !errors.Is(err, workflowstore.ErrInvalidIssueWriteCapability) {
				t.Fatalf("%s commit-window read error=%v", name, err)
			}
			if _, err := candidate.CreateOutcome(ctx, generated.CreateIssueWorkflowOutcomeParams{ID: workflowV2UUID(130), WorkspaceID: workspaceID, IssueID: issueID, State: "open", AcceptanceVersion: 1}); !errors.Is(err, workflowstore.ErrInvalidIssueWriteCapability) {
				t.Fatalf("%s commit-window mutation error=%v", name, err)
			}
			if _, err := workflowprojection.ProjectInWrite(ctx, candidate, workspaceID, issueID, 0, 1, time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)); !errors.Is(err, workflowstore.ErrInvalidIssueWriteCapability) {
				t.Fatalf("%s commit-window projection error=%v", name, err)
			}
			service := workflowcommand.Service{}
			if _, err := service.ExecuteInWrite(ctx, candidate, workflowcommand.Principal{}, workflowcommand.Request{WorkspaceID: workspaceID, IssueID: issueID}); !errors.Is(err, workflowstore.ErrInvalidIssueWriteCapability) {
				t.Fatalf("%s commit-window command error=%v", name, err)
			}
		}
		if got := starter.tx.touches.Load(); got != touches {
			t.Fatalf("expired capability touched transaction %d additional times", got-touches)
		}
		starter.release()
		if err := <-done; err != nil {
			t.Fatalf("commit returned %v", err)
		}
	})

	t.Run("issue write operation lease drains before invalidation and commit", func(t *testing.T) {
		workspaceID, issueID := seedLoadableWorkflowV2(t, ctx, conn, 131)
		t.Cleanup(func() {
			_ = queries.DeleteIssueWorkflowV2RecordsByIssue(context.Background(), generated.DeleteIssueWorkflowV2RecordsByIssueParams{WorkspaceID: workspaceID, IssueID: issueID})
			_, _ = conn.Exec(context.Background(), `DELETE FROM issue WHERE id=$1`, issueID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM workspace WHERE id=$1`, workspaceID)
		})

		starter := newBlockingCommitStarter(conn)
		starter.blockQuery = true
		starter.queryEntered = make(chan struct{})
		starter.releaseQuery = make(chan struct{})
		callbackReturned := make(chan struct{})
		projectionDone := make(chan error, 1)
		txDone := make(chan error, 1)
		go func() {
			txDone <- workflowstore.InIssueWriteTransaction(ctx, starter, generated.New(conn), workspaceID, issueID, func(write *workflowstore.IssueWrite) error {
				starter.tx.capability = write
				copied := *write
				starter.tx.clone = &copied
				go func() {
					_, err := workflowprojection.ProjectInWrite(context.Background(), starter.tx.clone, workspaceID, issueID, 0, 1, time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC))
					projectionDone <- err
				}()
				<-starter.queryEntered
				close(callbackReturned)
				return nil
			})
		}()
		<-callbackReturned
		t.Cleanup(starter.release)
		t.Cleanup(starter.releaseQueryCall)

		// The projection is blocked inside its first query while holding the
		// composite operation lease. Callback return must therefore be unable
		// to invalidate the capability or begin Commit.
		select {
		case <-starter.commitEntered:
			t.Fatal("commit overlapped a blocked composite projection query")
		default:
		}
		starter.releaseQueryCall()
		if err := <-projectionDone; err != nil {
			t.Fatalf("blocked projection returned %v", err)
		}
		select {
		case <-starter.commitEntered:
		case <-time.After(5 * time.Second):
			t.Fatal("commit did not begin after the operation lease drained")
		}
		if starter.tx.operationInFlight.Load() != 0 {
			t.Fatal("commit began while a database operation was still in flight")
		}
		touches := starter.tx.touches.Load()
		for name, candidate := range map[string]*workflowstore.IssueWrite{"original": starter.tx.capability, "clone": starter.tx.clone} {
			if candidate.MatchesIssue(workspaceID, issueID) {
				t.Fatalf("%s capability remained active once commit began", name)
			}
			if _, err := workflowprojection.ProjectInWrite(ctx, candidate, workspaceID, issueID, 1, 2, time.Now()); !errors.Is(err, workflowstore.ErrInvalidIssueWriteCapability) {
				t.Fatalf("%s new composite access during commit error=%v", name, err)
			}
		}
		if got := starter.tx.touches.Load(); got != touches {
			t.Fatalf("rejected composite access touched transaction %d additional times", got-touches)
		}
		starter.release()
		if err := <-txDone; err != nil {
			t.Fatalf("transaction returned %v", err)
		}
	})

	t.Run("command service same head concurrency leaves loser with zero residue", func(t *testing.T) {
		workspaceID, issueID := seedLoadableWorkflowV2(t, ctx, conn, 140)
		t.Cleanup(func() {
			_ = queries.DeleteIssueWorkflowV2RecordsByIssue(context.Background(), generated.DeleteIssueWorkflowV2RecordsByIssueParams{WorkspaceID: workspaceID, IssueID: issueID})
			_, _ = conn.Exec(context.Background(), `DELETE FROM member WHERE workspace_id=$1`, workspaceID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM issue WHERE id=$1`, issueID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM workspace WHERE id=$1`, workspaceID)
		})
		controllerID, ownerID, operationID := workflowV2UUID(157), workflowV2UUID(152), workflowV2UUID(145)
		if _, err := conn.Exec(ctx, `INSERT INTO member(id,workspace_id,user_id,role) VALUES($1,$2,$3,'owner')`, workflowV2UUID(158), workspaceID, ownerID); err != nil {
			t.Fatal(err)
		}
		now := func() time.Time { return time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC) }
		run := func(key string) error {
			other, err := workflowV2TestConnection(ctx)
			if err != nil {
				return err
			}
			defer other.Close(ctx)
			request := authorizationDecisionRequest(workspaceID, issueID, key, operationID, workflowV2UUID(151), ownerID)
			_, err = (workflowcommand.Service{Starter: other, Queries: generated.New(other), Loader: workflowcommand.Loader{Now: now}, Now: now, ControllerID: controllerID}).Execute(ctx, workflowcommand.Principal{ActorID: controllerID, ActorType: "internal_controller"}, request)
			return err
		}
		start := make(chan struct{})
		errs := make(chan error, 2)
		var wg sync.WaitGroup
		for _, key := range []string{"concurrent-a", "concurrent-b"} {
			wg.Add(1)
			go func(key string) { defer wg.Done(); <-start; errs <- run(key) }(key)
		}
		close(start)
		wg.Wait()
		close(errs)
		successes, failures := 0, 0
		for err := range errs {
			if err == nil {
				successes++
			} else {
				failures++
			}
		}
		if successes != 1 || failures != 1 {
			t.Fatalf("concurrent results success=%d failure=%d", successes, failures)
		}
		for table, want := range map[string]int{"issue_workflow_command": 1, "issue_workflow_decision": 1, "issue_workflow_action_receipt": 1, "issue_workflow_attention": 2, "issue_current_picture": 1} {
			var count int
			if err := conn.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE workspace_id=$1 AND issue_id=$2", workspaceID, issueID).Scan(&count); err != nil || count != want {
				t.Fatalf("%s count=%d err=%v", table, count, err)
			}
		}
		for concern, want := range map[string]int64{"execution": 2, "attention": 2, "evidence": 2, "outcome": 1, "scope": 1, "authority": 1, "review": 1, "release": 1, "budget": 1} {
			var revision int64
			if err := conn.QueryRow(ctx, `SELECT revision FROM issue_workflow_concern_head WHERE workspace_id=$1 AND issue_id=$2 AND concern_type=$3`, workspaceID, issueID, concern).Scan(&revision); err != nil || revision != want {
				t.Fatalf("%s head revision=%d want=%d err=%v", concern, revision, want, err)
			}
		}
	})

	t.Run("migration repairs 465 without dropping grandfathered primary key", func(t *testing.T) {
		var foreignKeys, primaryKeys int
		if err := conn.QueryRow(ctx, `
			SELECT
				count(*) FILTER (WHERE contype = 'f'),
				count(*) FILTER (WHERE contype = 'p')
			FROM pg_constraint
			WHERE conrelid = 'squad_workflow_template'::regclass
		`).Scan(&foreignKeys, &primaryKeys); err != nil {
			t.Fatal(err)
		}
		if foreignKeys != 0 || primaryKeys != 1 {
			t.Fatalf("squad workflow constraints = (foreign=%d, primary=%d), want (0, 1)", foreignKeys, primaryKeys)
		}
	})

	t.Run("idempotent command replays exact request and refuses mismatch", func(t *testing.T) {
		workspaceID, issueID, otherIssueID := workflowV2UUID(1), workflowV2UUID(2), workflowV2UUID(31)
		firstID, retryID := workflowV2UUID(3), workflowV2UUID(4)
		base := generated.CreateIssueWorkflowCommandParams{
			ID: firstID, WorkspaceID: workspaceID, IssueID: issueID,
			CommandKey: "command-1", RequestHash: "hash-1",
			CommandType: "queue", ActorType: "controller",
			ExpectedRevisions: []byte(`{"execution":1}`), Payload: []byte(`{"task":"one"}`),
		}
		first, err := queries.CreateIssueWorkflowCommand(ctx, base)
		if err != nil {
			t.Fatal(err)
		}
		base.ID = retryID
		replayed, err := queries.CreateIssueWorkflowCommand(ctx, base)
		if err != nil {
			t.Fatal(err)
		}
		if replayed.ID != first.ID {
			t.Fatalf("retry returned id %v, want original %v", replayed.ID, first.ID)
		}
		base.RequestHash = "different-payload-hash"
		if _, err := queries.CreateIssueWorkflowCommand(ctx, base); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("mismatched retry error = %v, want pgx.ErrNoRows", err)
		}
		base.RequestHash = "hash-1"
		base.Payload = []byte(`{"task":"different"}`)
		if _, err := queries.CreateIssueWorkflowCommand(ctx, base); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("same-hash payload mismatch error = %v, want pgx.ErrNoRows", err)
		}
		base.ID = workflowV2UUID(32)
		base.IssueID = otherIssueID
		base.RequestHash = "other-issue-hash"
		base.Payload = []byte(`{"task":"other-issue"}`)
		other, err := queries.CreateIssueWorkflowCommand(ctx, base)
		if err != nil {
			t.Fatalf("same key on another issue: %v", err)
		}
		if other.ID != base.ID || other.IssueID != otherIssueID {
			t.Fatalf("other issue command = %+v, want independent command %+v", other, base.ID)
		}
		var oldIndex, newIndex *string
		if err := conn.QueryRow(ctx, `SELECT to_regclass('uq_issue_workflow_command_idempotency')::text, to_regclass('uq_issue_workflow_command_issue_idempotency')::text`).Scan(&oldIndex, &newIndex); err != nil {
			t.Fatal(err)
		}
		if oldIndex != nil || newIndex == nil {
			t.Fatalf("idempotency indexes old=%v new=%v, want old absent/new present", oldIndex, newIndex)
		}
		downSQL := readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "502_drop_workspace_command_idempotency_idx.down.sql"))
		if _, err := conn.Exec(ctx, downSQL); err == nil {
			t.Fatal("502 down unexpectedly recreated workspace-only uniqueness across cross-issue duplicate keys")
		}
		// PostgreSQL leaves a failed concurrent build as an invalid relation.
		// Remove only that disposable-test artifact so later contract cases see
		// the same fully-upgraded schema.
		if _, err := conn.Exec(ctx, `DROP INDEX CONCURRENTLY IF EXISTS uq_issue_workflow_command_idempotency`); err != nil {
			t.Fatalf("remove failed down-migration index: %v", err)
		}
		var issueScopedIndexRemains bool
		if err := conn.QueryRow(ctx, `SELECT to_regclass('uq_issue_workflow_command_issue_idempotency') IS NOT NULL`).Scan(&issueScopedIndexRemains); err != nil {
			t.Fatalf("verify issue-scoped index after failed down: %v", err)
		}
		if !issueScopedIndexRemains {
			t.Fatal("failed 502 down removed the issue-scoped idempotency index")
		}
	})

	t.Run("idempotent action and decision response replay or refuse mismatch", func(t *testing.T) {
		workspaceID, issueID := workflowV2UUID(5), workflowV2UUID(6)
		action := generated.CreateIssueWorkflowActionReceiptParams{
			ID: workflowV2UUID(7), WorkspaceID: workspaceID, IssueID: issueID,
			ActionKey: "action-replay", RequestHash: "action-hash",
			CommandID: workflowV2UUID(8), ActionType: "dispatch",
			Payload: []byte(`{"dispatch":true}`), NotBefore: workflowV2Time(time.Now().UTC()), MaxAttempts: 2,
		}
		firstAction, err := queries.CreateIssueWorkflowActionReceipt(ctx, action)
		if err != nil {
			t.Fatal(err)
		}
		action.ID = workflowV2UUID(9)
		replayedAction, err := queries.CreateIssueWorkflowActionReceipt(ctx, action)
		if err != nil {
			t.Fatal(err)
		}
		if replayedAction.ID != firstAction.ID {
			t.Fatalf("action retry returned id %v, want %v", replayedAction.ID, firstAction.ID)
		}
		action.RequestHash = "mismatched-action-hash"
		if _, err := queries.CreateIssueWorkflowActionReceipt(ctx, action); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("mismatched action retry error = %v, want pgx.ErrNoRows", err)
		}
		action.RequestHash = "action-hash"
		action.Payload = []byte(`{"dispatch":false}`)
		if _, err := queries.CreateIssueWorkflowActionReceipt(ctx, action); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("same-hash action payload mismatch error = %v, want pgx.ErrNoRows", err)
		}

		response := generated.CreateIssueWorkflowDecisionResponseParams{
			ID: workflowV2UUID(16), WorkspaceID: workspaceID, IssueID: issueID,
			DecisionID: workflowV2UUID(17), DecisionRevision: 1,
			ResponseType: "approve", ActorID: workflowV2UUID(18),
			IdempotencyKey: "response-replay", RequestHash: "response-hash",
			SelectedOptions: []byte(`[]`), Content: []byte(`{"note":"yes"}`), ScopeChange: []byte(`{}`),
		}
		firstResponse, err := queries.CreateIssueWorkflowDecisionResponse(ctx, response)
		if err != nil {
			t.Fatal(err)
		}
		response.ID = workflowV2UUID(19)
		replayedResponse, err := queries.CreateIssueWorkflowDecisionResponse(ctx, response)
		if err != nil {
			t.Fatal(err)
		}
		if replayedResponse.ID != firstResponse.ID {
			t.Fatalf("response retry returned id %v, want %v", replayedResponse.ID, firstResponse.ID)
		}
		response.RequestHash = "mismatched-response-hash"
		if _, err := queries.CreateIssueWorkflowDecisionResponse(ctx, response); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("mismatched response retry error = %v, want pgx.ErrNoRows", err)
		}
		response.RequestHash = "response-hash"
		response.Content = []byte(`{"note":"different"}`)
		if _, err := queries.CreateIssueWorkflowDecisionResponse(ctx, response); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("same-hash response payload mismatch error = %v, want pgx.ErrNoRows", err)
		}
	})

	t.Run("expired action lease is reclaimed and old fence is rejected", func(t *testing.T) {
		if _, err := conn.Exec(ctx, `DELETE FROM issue_workflow_action_receipt`); err != nil {
			t.Fatal(err)
		}
		workspaceID, issueID := workflowV2UUID(10), workflowV2UUID(11)
		if _, err := conn.Exec(ctx, `INSERT INTO workspace(id) VALUES($1)`, workspaceID); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, `INSERT INTO issue(id,workspace_id) VALUES($1,$2)`, issueID, workspaceID); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, _ = conn.Exec(context.Background(), `DELETE FROM issue WHERE id=$1`, issueID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM workspace WHERE id=$1`, workspaceID)
		})
		actionID, commandID := workflowV2UUID(12), workflowV2UUID(13)
		now := time.Now().UTC().Truncate(time.Microsecond)
		created, err := queries.CreateIssueWorkflowActionReceipt(ctx, generated.CreateIssueWorkflowActionReceiptParams{
			ID: actionID, WorkspaceID: workspaceID, IssueID: issueID,
			ActionKey: "action-lease", RequestHash: "action-hash", CommandID: commandID,
			ActionType: "review_correction", Payload: []byte(`{"task":"lease"}`),
			NotBefore: workflowV2Time(now), MaxAttempts: 3,
		})
		if err != nil {
			t.Fatal(err)
		}
		if created.Revision != 1 {
			t.Fatalf("created action revision = %d, want 1", created.Revision)
		}

		oldLease, newLease := workflowV2UUID(14), workflowV2UUID(15)
		claim := func(params generated.ClaimIssueWorkflowActionExactParams) (generated.IssueWorkflowActionReceipt, error) {
			var row generated.IssueWorkflowActionReceipt
			err := workflowstore.InIssueWriteTransaction(ctx, conn, queries, workspaceID, issueID, func(write *workflowstore.IssueWrite) error {
				var err error
				row, err = write.ClaimAction(ctx, params)
				return err
			})
			return row, err
		}
		for name, expiry := range map[string]time.Time{"past": now.Add(-time.Second), "equal": now} {
			if _, err := claim(generated.ClaimIssueWorkflowActionExactParams{LeaseToken: oldLease, LeaseExpiresAt: workflowV2Time(expiry), Now: workflowV2Time(now), WorkspaceID: workspaceID, IssueID: issueID, ID: actionID, ExpectedRevision: 1}); !errors.Is(err, pgx.ErrNoRows) {
				t.Fatalf("%s claim expiry error=%v", name, err)
			}
		}
		first, err := claim(generated.ClaimIssueWorkflowActionExactParams{
			LeaseToken: oldLease, LeaseExpiresAt: workflowV2Time(now.Add(time.Minute)), Now: workflowV2Time(now), WorkspaceID: workspaceID, IssueID: issueID, ID: actionID, ExpectedRevision: 1,
		})
		if err != nil {
			t.Fatal(err)
		}
		if first.Revision != 2 || first.LeaseCount != 1 || first.EffectAttemptCount != 0 {
			t.Fatalf("first claim = (revision %d, leases %d, effects %d), want (2, 1, 0)", first.Revision, first.LeaseCount, first.EffectAttemptCount)
		}
		if _, err := claim(generated.ClaimIssueWorkflowActionExactParams{
			LeaseToken: newLease, LeaseExpiresAt: workflowV2Time(now.Add(2 * time.Minute)), Now: workflowV2Time(now.Add(30 * time.Second)), WorkspaceID: workspaceID, IssueID: issueID, ID: actionID, ExpectedRevision: 2,
		}); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("unexpired reclaim error = %v, want pgx.ErrNoRows", err)
		}

		reclaimed, err := claim(generated.ClaimIssueWorkflowActionExactParams{
			LeaseToken: newLease, LeaseExpiresAt: workflowV2Time(now.Add(3 * time.Minute)), Now: workflowV2Time(now.Add(2 * time.Minute)), WorkspaceID: workspaceID, IssueID: issueID, ID: actionID, ExpectedRevision: 2,
		})
		if err != nil {
			t.Fatal(err)
		}
		if reclaimed.Revision != 3 || reclaimed.LeaseCount != 2 || reclaimed.EffectAttemptCount != 0 || reclaimed.LeaseToken != newLease {
			t.Fatalf("reclaim = (revision %d, leases %d, effects %d, lease %v)", reclaimed.Revision, reclaimed.LeaseCount, reclaimed.EffectAttemptCount, reclaimed.LeaseToken)
		}
		renew := func(params generated.RenewIssueWorkflowActionLeaseParams) (generated.IssueWorkflowActionReceipt, error) {
			var row generated.IssueWorkflowActionReceipt
			err := workflowstore.InIssueWriteTransaction(ctx, conn, queries, workspaceID, issueID, func(write *workflowstore.IssueWrite) error {
				var err error
				row, err = write.RenewAction(ctx, params)
				return err
			})
			return row, err
		}
		baseRenew := generated.RenewIssueWorkflowActionLeaseParams{LeaseExpiresAt: workflowV2Time(now.Add(4 * time.Minute)), WorkspaceID: workspaceID, IssueID: issueID, ID: actionID, ExpectedRevision: 3, LeaseToken: newLease, Now: workflowV2Time(now.Add(2 * time.Minute))}
		wrongToken := baseRenew
		wrongToken.LeaseToken = oldLease
		if _, err := renew(wrongToken); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("wrong-token renew error=%v", err)
		}
		staleRevision := baseRenew
		staleRevision.ExpectedRevision = 2
		if _, err := renew(staleRevision); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("stale-revision renew error=%v", err)
		}
		expired := baseRenew
		expired.Now = workflowV2Time(now.Add(4 * time.Minute))
		if _, err := renew(expired); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("expired renew error=%v", err)
		}
		for name, expiry := range map[string]time.Time{"past": now.Add(-time.Second), "equal-now": now.Add(2 * time.Minute), "shorter": now.Add(150 * time.Second), "equal-current": now.Add(3 * time.Minute)} {
			invalid := baseRenew
			invalid.LeaseExpiresAt = workflowV2Time(expiry)
			if _, err := renew(invalid); !errors.Is(err, pgx.ErrNoRows) {
				t.Fatalf("%s renew expiry error=%v", name, err)
			}
		}
		renewed, err := renew(baseRenew)
		if err != nil || renewed.Revision != 4 {
			t.Fatalf("renewed=(%d,%v), want revision 4", renewed.Revision, err)
		}
		for name, params := range map[string]generated.BeginIssueWorkflowActionAttemptParams{
			"wrong token":    {Now: workflowV2Time(now.Add(2 * time.Minute)), WorkspaceID: workspaceID, IssueID: issueID, ID: actionID, ExpectedRevision: 4, LeaseToken: oldLease},
			"stale revision": {Now: workflowV2Time(now.Add(2 * time.Minute)), WorkspaceID: workspaceID, IssueID: issueID, ID: actionID, ExpectedRevision: 3, LeaseToken: newLease},
			"expired lease":  {Now: workflowV2Time(now.Add(4 * time.Minute)), WorkspaceID: workspaceID, IssueID: issueID, ID: actionID, ExpectedRevision: 4, LeaseToken: newLease},
		} {
			if _, err := queries.BeginIssueWorkflowActionAttempt(ctx, params); !errors.Is(err, pgx.ErrNoRows) {
				t.Fatalf("%s begin attempt error=%v", name, err)
			}
		}
		for name, params := range map[string]generated.DeferIssueWorkflowActionExactParams{
			"wrong token":    {NotBefore: workflowV2Time(now.Add(5 * time.Minute)), LastError: "defer", WorkspaceID: workspaceID, IssueID: issueID, ID: actionID, ExpectedRevision: 4, LeaseToken: oldLease, Now: workflowV2Time(now.Add(2 * time.Minute))},
			"stale revision": {NotBefore: workflowV2Time(now.Add(5 * time.Minute)), LastError: "defer", WorkspaceID: workspaceID, IssueID: issueID, ID: actionID, ExpectedRevision: 3, LeaseToken: newLease, Now: workflowV2Time(now.Add(2 * time.Minute))},
			"expired lease":  {NotBefore: workflowV2Time(now.Add(5 * time.Minute)), LastError: "defer", WorkspaceID: workspaceID, IssueID: issueID, ID: actionID, ExpectedRevision: 4, LeaseToken: newLease, Now: workflowV2Time(now.Add(4 * time.Minute))},
		} {
			if _, err := queries.DeferIssueWorkflowActionExact(ctx, params); !errors.Is(err, pgx.ErrNoRows) {
				t.Fatalf("%s defer error=%v", name, err)
			}
		}
		begun, err := queries.BeginIssueWorkflowActionAttempt(ctx, generated.BeginIssueWorkflowActionAttemptParams{Now: workflowV2Time(now.Add(2 * time.Minute)), WorkspaceID: workspaceID, IssueID: issueID, ID: actionID, ExpectedRevision: 4, LeaseToken: newLease})
		if err != nil || begun.Revision != 5 || begun.EffectAttemptCount != 1 {
			t.Fatalf("begun=(revision %d, effects %d, err %v), want (5,1,nil)", begun.Revision, begun.EffectAttemptCount, err)
		}

		settle := generated.SettleIssueWorkflowActionParams{
			Now: workflowV2Time(now.Add(2 * time.Minute)), EffectResult: []byte(`{"ok":true}`),
			WorkspaceID: workspaceID, IssueID: issueID, ID: actionID, ExpectedRevision: 5, LeaseToken: oldLease,
		}
		if _, err := queries.SettleIssueWorkflowAction(ctx, settle); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("old lease settlement error = %v, want pgx.ErrNoRows", err)
		}
		settle.LeaseToken = newLease
		settled, err := queries.SettleIssueWorkflowAction(ctx, settle)
		if err != nil {
			t.Fatal(err)
		}
		if settled.Revision != 6 || settled.State != "settled" {
			t.Fatalf("settled action = (revision %d, state %s), want (6, settled)", settled.Revision, settled.State)
		}
	})

	t.Run("technical stop before correction claim blocks the frozen failed review", func(t *testing.T) {
		workspaceID, issueID := seedLoadableWorkflowV2(t, ctx, conn, 180)
		operationID, candidateID := workflowV2UUID(185), workflowV2UUID(191)
		implementerID, reviewerID := workflowV2UUID(193), workflowV2UUID(194)
		runtimeID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
		taskID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
		reviewActionID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
		reviewCommandID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
		leaseToken := pgtype.UUID{Bytes: uuid.New(), Valid: true}
		controllerID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
		nowValue := time.Date(2026, 9, 10, 15, 30, 0, 0, time.UTC)
		for _, statement := range []struct {
			sql  string
			args []any
		}{
			{`UPDATE issue_workflow_execution SET state='in_review',actor_id=$3,task_id=$4 WHERE workspace_id=$1 AND issue_id=$2`, []any{workspaceID, issueID, reviewerID, taskID}},
			{`UPDATE issue_workflow_review SET outcome='in_progress' WHERE workspace_id=$1 AND issue_id=$2`, []any{workspaceID, issueID}},
			{`UPDATE issue_workflow_concern_head SET state='in_review' WHERE workspace_id=$1 AND issue_id=$2 AND concern_type='execution'`, []any{workspaceID, issueID}},
			{`UPDATE issue_workflow_concern_head SET state='in_progress' WHERE workspace_id=$1 AND issue_id=$2 AND concern_type='review'`, []any{workspaceID, issueID}},
			{`INSERT INTO agent_runtime(id,workspace_id) VALUES($1,$2)`, []any{runtimeID, workspaceID}},
			{`INSERT INTO agent(id,workspace_id,runtime_id) VALUES($1,$2,$3)`, []any{implementerID, workspaceID, runtimeID}},
			{`INSERT INTO agent(id,workspace_id) VALUES($1,$2)`, []any{reviewerID, workspaceID}},
			{`INSERT INTO agent_task_queue(id,agent_id,issue_id,status,workflow_action_id,workflow_operation_id,workflow_branch_key) VALUES($1,$2,$3,'running',$4,$5,'main')`, []any{taskID, reviewerID, issueID, reviewActionID, operationID}},
			{`INSERT INTO issue_workflow_action_receipt(id,workspace_id,issue_id,action_key,request_hash,command_id,action_type,state,operation_id,payload,bound_task_id,lease_token,lease_expires_at) VALUES($1,$2,$3,'review/stale-snapshot','seed',$4,'review_candidate','settled',$5,jsonb_build_object('branch_key','main','reviewer_id',$7::text,'candidate_id',$8::text,'scope_revision',1),$6,$9,$10)`, []any{reviewActionID, workspaceID, issueID, reviewCommandID, operationID, taskID, reviewerID, candidateID, leaseToken, nowValue.Add(time.Hour)}},
		} {
			if _, err := conn.Exec(ctx, statement.sql, statement.args...); err != nil {
				t.Fatal(err)
			}
		}
		t.Cleanup(func() {
			_, _ = conn.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE issue_id=$1`, issueID)
			_ = queries.DeleteIssueWorkflowV2RecordsByIssue(context.Background(), generated.DeleteIssueWorkflowV2RecordsByIssueParams{WorkspaceID: workspaceID, IssueID: issueID})
			_, _ = conn.Exec(context.Background(), `DELETE FROM agent WHERE id IN ($1,$2)`, implementerID, reviewerID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM agent_runtime WHERE id=$1`, runtimeID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM issue WHERE id=$1`, issueID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM workspace WHERE id=$1`, workspaceID)
		})
		now := func() time.Time { return nowValue }
		service := workflowcommand.Service{Starter: conn, Queries: queries, Loader: workflowcommand.Loader{Now: now}, Now: now, ControllerID: controllerID}
		reviewRequest := workflowcommand.Request{
			WorkspaceID: workspaceID, IssueID: issueID, CommandKey: "review-failed-stale-snapshot", Kind: workflowstatev2.CommandReviewFailed,
			Expected:     []workflowstatev2.ExpectedRevision{{Concern: workflowstatev2.ConcernExecution, Revision: 1}, {Concern: workflowstatev2.ConcernReview, Revision: 1}, {Concern: workflowstatev2.ConcernEvidence, Revision: 1}},
			EvidenceRefs: []string{util.UUIDToString(workflowV2UUID(197))}, CriteriaRefs: []string{"acceptance"}, CoverageRefs: []string{"tests"}, GapRefs: []string{"failure"}, RequiredCorrection: "repair exact snapshot", CorrectionCount: 1, MaxCorrections: 2,
		}
		receipt, err := service.Execute(ctx, workflowcommand.Principal{ActorID: reviewerID, ActorType: "agent_task", TaskID: taskID, ActionKey: "review/stale-snapshot", LeaseToken: leaseToken}, reviewRequest)
		if err != nil || len(receipt.ActionIDs) != 1 {
			t.Fatalf("review failed receipt=(%+v,%v)", receipt, err)
		}
		correctionActionID := workflowV2UUIDString(receipt.ActionIDs[0])
		if _, err := service.Execute(ctx, workflowcommand.Principal{ActorID: controllerID, ActorType: "internal_controller"}, workflowcommand.Request{
			WorkspaceID: workspaceID, IssueID: issueID, CommandKey: "technical-stop-before-correction", Kind: workflowstatev2.CommandTechnicalStop,
			OperationID: util.UUIDToString(operationID), Detail: "superseded before correction claim",
			Expected: []workflowstatev2.ExpectedRevision{{Concern: workflowstatev2.ConcernExecution, Revision: 2}},
		}); err != nil {
			t.Fatal(err)
		}
		worker := workflowaction.Worker{Starter: conn, Queries: queries, Effect: workflowaction.LocalTaskEffect{Starter: conn, Queries: queries, Now: now}, Capacity: workflowaction.CapacityFunc(func(context.Context, generated.IssueWorkflowActionReceipt) (bool, error) { return true, nil }), Now: now}
		if err := worker.ReconcileOnce(ctx); err != nil {
			t.Fatal(err)
		}
		var tasks int
		var state string
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE workflow_action_id=$1`, correctionActionID).Scan(&tasks); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT state FROM issue_workflow_action_receipt WHERE id=$1`, correctionActionID).Scan(&state); err != nil {
			t.Fatal(err)
		}
		if tasks != 0 || state != "reconciling" {
			t.Fatalf("stale correction tasks=%d action_state=%s", tasks, state)
		}
	})

	t.Run("superseded candidate before correction claim blocks the frozen failed review", func(t *testing.T) {
		workspaceID, issueID := seedLoadableWorkflowV2(t, ctx, conn, 180)
		agentID, runtimeID, operationID := workflowV2UUID(193), workflowV2UUID(194), workflowV2UUID(185)
		prepareWorkflowCorrectionFixture(t, ctx, conn, workspaceID, issueID, 180)
		if _, err := conn.Exec(ctx, `INSERT INTO agent_runtime(id,workspace_id) VALUES($1,$2)`, runtimeID, workspaceID); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, `INSERT INTO agent(id,workspace_id,runtime_id) VALUES($1,$2,$3)`, agentID, workspaceID, runtimeID); err != nil {
			t.Fatal(err)
		}
		nowValue := time.Date(2026, 9, 10, 15, 45, 0, 0, time.UTC)
		actionID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
		if _, err := queries.CreateIssueWorkflowActionReceipt(ctx, generated.CreateIssueWorkflowActionReceiptParams{ID: actionID, WorkspaceID: workspaceID, IssueID: issueID, ActionKey: "superseded-candidate", RequestHash: "superseded-candidate-hash", CommandID: pgtype.UUID{Bytes: uuid.New(), Valid: true}, ActionType: "review_correction", OperationID: operationID, Payload: workflowCorrectionActionPayload(t, 180, operationID, agentID), NotBefore: workflowV2Time(nowValue), MaxAttempts: 3}); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, `UPDATE issue_workflow_candidate SET superseded_at=$1 WHERE workspace_id=$2 AND issue_id=$3 AND id=$4`, nowValue, workspaceID, issueID, workflowV2UUID(191)); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, _ = conn.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE issue_id=$1`, issueID)
			_ = queries.DeleteIssueWorkflowV2RecordsByIssue(context.Background(), generated.DeleteIssueWorkflowV2RecordsByIssueParams{WorkspaceID: workspaceID, IssueID: issueID})
			_, _ = conn.Exec(context.Background(), `DELETE FROM agent WHERE id=$1`, agentID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM agent_runtime WHERE id=$1`, runtimeID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM issue WHERE id=$1`, issueID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM workspace WHERE id=$1`, workspaceID)
		})
		now := func() time.Time { return nowValue }
		worker := workflowaction.Worker{Starter: conn, Queries: queries, Effect: workflowaction.LocalTaskEffect{Starter: conn, Queries: queries, Now: now}, Capacity: workflowaction.CapacityFunc(func(context.Context, generated.IssueWorkflowActionReceipt) (bool, error) { return true, nil }), Now: now}
		if err := worker.ReconcileOnce(ctx); err != nil {
			t.Fatal(err)
		}
		var tasks int
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE workflow_action_id=$1`, actionID).Scan(&tasks); err != nil || tasks != 0 {
			t.Fatalf("superseded candidate task residue=%d err=%v", tasks, err)
		}
	})

	t.Run("review correction worker creates one deterministic task and capacity costs no attempt", func(t *testing.T) {
		workspaceID, issueID := seedLoadableWorkflowV2(t, ctx, conn, 180)
		agentID, runtimeID, operationID := workflowV2UUID(193), workflowV2UUID(250), workflowV2UUID(185)
		prepareWorkflowCorrectionFixture(t, ctx, conn, workspaceID, issueID, 180)
		if _, err := conn.Exec(ctx, `INSERT INTO agent_runtime(id,workspace_id) VALUES($1,$2)`, runtimeID, workspaceID); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, `INSERT INTO agent(id,workspace_id,runtime_id) VALUES($1,$2,$3)`, agentID, workspaceID, runtimeID); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, _ = conn.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE issue_id=$1`, issueID)
			_ = queries.DeleteIssueWorkflowV2RecordsByIssue(context.Background(), generated.DeleteIssueWorkflowV2RecordsByIssueParams{WorkspaceID: workspaceID, IssueID: issueID})
			_, _ = conn.Exec(context.Background(), `DELETE FROM agent WHERE id=$1`, agentID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM agent_runtime WHERE id=$1`, runtimeID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM issue WHERE id=$1`, issueID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM workspace WHERE id=$1`, workspaceID)
		})
		now := time.Date(2026, 9, 10, 16, 0, 0, 0, time.UTC)
		createAction := func(id, commandID pgtype.UUID, key string) generated.IssueWorkflowActionReceipt {
			payload := workflowCorrectionActionPayload(t, 180, operationID, agentID)
			row, err := queries.CreateIssueWorkflowActionReceipt(ctx, generated.CreateIssueWorkflowActionReceiptParams{ID: id, WorkspaceID: workspaceID, IssueID: issueID, ActionKey: key, RequestHash: key + "-hash", CommandID: commandID, ActionType: "review_correction", OperationID: operationID, Payload: payload, NotBefore: workflowV2Time(now), MaxAttempts: 3})
			if err != nil {
				t.Fatal(err)
			}
			return row
		}
		action := createAction(workflowV2UUID(240), workflowV2UUID(241), "worker-success")
		wakeCount := 0
		effect := workflowaction.LocalTaskEffect{Starter: conn, Queries: queries, Now: func() time.Time { return now }, Wake: func(context.Context, pgtype.UUID) error { wakeCount++; return errors.New("dropped wake") }}
		worker := workflowaction.Worker{Starter: conn, Queries: queries, Effect: effect, Capacity: workflowaction.CapacityFunc(func(context.Context, generated.IssueWorkflowActionReceipt) (bool, error) { return true, nil }), Now: func() time.Time { return now }}
		if err := worker.ReconcileOnce(ctx); err != nil {
			t.Fatal(err)
		}
		var state string
		var revision int64
		var leases, attempts, taskCount int
		if err := conn.QueryRow(ctx, `SELECT state,revision,lease_count,effect_attempt_count FROM issue_workflow_action_receipt WHERE id=$1`, action.ID).Scan(&state, &revision, &leases, &attempts); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE workflow_action_id=$1 AND workflow_operation_id=$2 AND agent_id=$3 AND runtime_id=$4`, action.ID, operationID, agentID, runtimeID).Scan(&taskCount); err != nil {
			t.Fatal(err)
		}
		if state != "settled" || revision != 5 || leases != 1 || attempts != 1 || taskCount != 1 || wakeCount != 1 {
			t.Fatalf("settled action state=%s rev=%d leases=%d attempts=%d tasks=%d wakes=%d", state, revision, leases, attempts, taskCount, wakeCount)
		}
		worker.Wake()
		worker.Wake()
		if err := worker.ReconcileOnce(ctx); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE workflow_action_id=$1`, action.ID).Scan(&taskCount); err != nil || taskCount != 1 {
			t.Fatalf("duplicate wake tasks=%d err=%v", taskCount, err)
		}

		deferred := createAction(workflowV2UUID(242), workflowV2UUID(243), "worker-capacity")
		worker.Capacity = workflowaction.CapacityFunc(func(context.Context, generated.IssueWorkflowActionReceipt) (bool, error) { return false, nil })
		if err := worker.ReconcileOnce(ctx); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT state,lease_count,effect_attempt_count FROM issue_workflow_action_receipt WHERE id=$1`, deferred.ID).Scan(&state, &leases, &attempts); err != nil {
			t.Fatal(err)
		}
		if state != "retry_wait" || leases != 1 || attempts != 0 {
			t.Fatalf("capacity deferral state=%s leases=%d attempts=%d", state, leases, attempts)
		}

		crashed := createAction(workflowV2UUID(244), workflowV2UUID(245), "worker-crash-before-settlement")
		var begun generated.IssueWorkflowActionReceipt
		if err := workflowstore.InIssueWriteTransaction(ctx, conn, queries, workspaceID, issueID, func(write *workflowstore.IssueWrite) error {
			claimed, err := write.ClaimAction(ctx, generated.ClaimIssueWorkflowActionExactParams{LeaseToken: workflowV2UUID(246), LeaseExpiresAt: workflowV2Time(now.Add(time.Minute)), WorkspaceID: workspaceID, IssueID: issueID, ID: crashed.ID, ExpectedRevision: crashed.Revision, Now: workflowV2Time(now)})
			if err != nil {
				return err
			}
			begun, err = write.BeginActionAttempt(ctx, generated.BeginIssueWorkflowActionAttemptParams{Now: workflowV2Time(now), WorkspaceID: workspaceID, IssueID: issueID, ID: crashed.ID, ExpectedRevision: claimed.Revision, LeaseToken: claimed.LeaseToken})
			return err
		}); err != nil {
			t.Fatal(err)
		}
		applied, err := effect.Apply(ctx, begun)
		if err != nil || applied.Kind != workflowaction.ObservationFound {
			t.Fatalf("crash fixture apply=(%+v,%v)", applied, err)
		}
		if _, err := conn.Exec(ctx, `UPDATE agent_task_queue SET status='dispatched' WHERE workflow_action_id=$1`, crashed.ID); err != nil {
			t.Fatal(err)
		}
		later := now.Add(2 * time.Minute)
		recoveryEffect := workflowaction.LocalTaskEffect{Starter: conn, Queries: queries, Now: func() time.Time { return later }}
		recovery := workflowaction.Worker{Starter: conn, Queries: queries, Effect: recoveryEffect, Capacity: workflowaction.CapacityFunc(func(context.Context, generated.IssueWorkflowActionReceipt) (bool, error) { return true, nil }), Now: func() time.Time { return later }}
		if err := recovery.ReconcileOnce(ctx); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT state,lease_count,effect_attempt_count FROM issue_workflow_action_receipt WHERE id=$1`, crashed.ID).Scan(&state, &leases, &attempts); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE workflow_action_id=$1`, crashed.ID).Scan(&taskCount); err != nil {
			t.Fatal(err)
		}
		if state != "settled" || leases != 2 || attempts != 1 || taskCount != 1 {
			t.Fatalf("crash recovery state=%s leases=%d attempts=%d tasks=%d", state, leases, attempts, taskCount)
		}
		for _, lifecycleState := range []string{"running", "completed"} {
			if _, err := conn.Exec(ctx, `UPDATE agent_task_queue SET status=$2 WHERE workflow_action_id=$1`, crashed.ID, lifecycleState); err != nil {
				t.Fatal(err)
			}
			observed, err := recoveryEffect.Observe(ctx, crashed)
			if err != nil || observed.Kind != workflowaction.ObservationFound {
				t.Fatalf("observe exact %s task=(%+v,%v)", lifecycleState, observed, err)
			}
		}

		mismatched := createAction(workflowV2UUID(247), workflowV2UUID(248), "worker-semantic-mismatch")
		mismatchedTaskID := pgtype.UUID{Bytes: uuid.NewSHA1(uuid.UUID(mismatched.ID.Bytes), []byte("review_correction/task")), Valid: true}
		if _, err := conn.Exec(ctx, `INSERT INTO agent_task_queue(id,agent_id,runtime_id,issue_id,status,context,originator_source,trigger_evidence_kind,trigger_evidence_ref_id,workflow_action_id,workflow_operation_id,workflow_branch_key) VALUES($1,$2,$3,$4,'queued','{"wrong":true}','workflow_action','workflow_action',$5,$5,$6,'main')`, mismatchedTaskID, agentID, runtimeID, issueID, mismatched.ID, operationID); err != nil {
			t.Fatal(err)
		}
		worker.Capacity = workflowaction.CapacityFunc(func(context.Context, generated.IssueWorkflowActionReceipt) (bool, error) { return true, nil })
		if err := worker.ReconcileOnce(ctx); err != nil {
			t.Fatal(err)
		}
		var lastError string
		var observationFailures int
		if err := conn.QueryRow(ctx, `SELECT state,effect_attempt_count,observation_failure_count,last_error FROM issue_workflow_action_receipt WHERE id=$1`, mismatched.ID).Scan(&state, &attempts, &observationFailures, &lastError); err != nil {
			t.Fatal(err)
		}
		if state != "reconciling" || attempts != 0 || observationFailures != 1 || !strings.Contains(lastError, workflowaction.ErrEffectMismatch.Error()) {
			t.Fatalf("semantic mismatch state=%s attempts=%d observation_failures=%d error=%q", state, attempts, observationFailures, lastError)
		}
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE workflow_action_id=$1`, mismatched.ID).Scan(&taskCount); err != nil || taskCount != 1 {
			t.Fatalf("semantic mismatch task count=%d err=%v", taskCount, err)
		}
		successTaskID := pgtype.UUID{Bytes: uuid.NewSHA1(uuid.UUID(action.ID.Bytes), []byte("review_correction/task")), Valid: true}
		if _, err := queries.GetWorkflowCorrectionTaskEffect(ctx, generated.GetWorkflowCorrectionTaskEffectParams{ID: successTaskID, WorkflowActionID: mismatched.ID}); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("task/action identity collision lookup error=%v", err)
		}

		startup := createAction(workflowV2UUID(249), workflowV2UUID(251), "worker-startup-scan")
		startupConn, err := workflowV2TestConnection(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer startupConn.Close(ctx)
		startupQueries := generated.New(startupConn)
		runCtx, cancelRun := context.WithCancel(ctx)
		runDone := make(chan error, 1)
		startupWorker := workflowaction.Worker{Starter: startupConn, Queries: startupQueries, Effect: workflowaction.LocalTaskEffect{Starter: startupConn, Queries: startupQueries, Now: func() time.Time { return now }}, Capacity: workflowaction.CapacityFunc(func(context.Context, generated.IssueWorkflowActionReceipt) (bool, error) { return true, nil }), Now: func() time.Time { return now }, PollInterval: time.Hour}
		go func() { runDone <- startupWorker.Run(runCtx) }()
		deadline := time.Now().Add(5 * time.Second)
		for {
			if err := conn.QueryRow(ctx, `SELECT state FROM issue_workflow_action_receipt WHERE id=$1`, startup.ID).Scan(&state); err != nil {
				cancelRun()
				t.Fatal(err)
			}
			if state == "settled" {
				break
			}
			if time.Now().After(deadline) {
				cancelRun()
				t.Fatalf("startup scan did not settle action; state=%s", state)
			}
			time.Sleep(10 * time.Millisecond)
		}
		cancelRun()
		if err := <-runDone; !errors.Is(err, context.Canceled) {
			t.Fatalf("startup worker exit error=%v", err)
		}
	})

	t.Run("two workers converge on one correction task", func(t *testing.T) {
		workspaceID, issueID := seedLoadableWorkflowV2(t, ctx, conn, 180)
		agentID, runtimeID, operationID := workflowV2UUID(193), workflowV2UUID(194), workflowV2UUID(185)
		prepareWorkflowCorrectionFixture(t, ctx, conn, workspaceID, issueID, 180)
		if _, err := conn.Exec(ctx, `INSERT INTO agent_runtime(id,workspace_id) VALUES($1,$2)`, runtimeID, workspaceID); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, `INSERT INTO agent(id,workspace_id,runtime_id) VALUES($1,$2,$3)`, agentID, workspaceID, runtimeID); err != nil {
			t.Fatal(err)
		}
		actionID := workflowV2UUID(198)
		now := time.Date(2026, 9, 10, 17, 0, 0, 0, time.UTC)
		payload := workflowCorrectionActionPayload(t, 180, operationID, agentID)
		if _, err := queries.CreateIssueWorkflowActionReceipt(ctx, generated.CreateIssueWorkflowActionReceiptParams{ID: actionID, WorkspaceID: workspaceID, IssueID: issueID, ActionKey: "two-workers", RequestHash: "two-workers-hash", CommandID: workflowV2UUID(199), ActionType: "review_correction", OperationID: operationID, Payload: payload, NotBefore: workflowV2Time(now), MaxAttempts: 3}); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, _ = conn.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE issue_id=$1`, issueID)
			_ = queries.DeleteIssueWorkflowV2RecordsByIssue(context.Background(), generated.DeleteIssueWorkflowV2RecordsByIssueParams{WorkspaceID: workspaceID, IssueID: issueID})
			_, _ = conn.Exec(context.Background(), `DELETE FROM agent WHERE id=$1`, agentID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM agent_runtime WHERE id=$1`, runtimeID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM issue WHERE id=$1`, issueID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM workspace WHERE id=$1`, workspaceID)
		})
		other, err := workflowV2TestConnection(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer other.Close(ctx)
		makeWorker := func(connection *pgx.Conn) workflowaction.Worker {
			q := generated.New(connection)
			return workflowaction.Worker{Starter: connection, Queries: q, Effect: workflowaction.LocalTaskEffect{Starter: connection, Queries: q, Now: func() time.Time { return now }}, Capacity: workflowaction.CapacityFunc(func(context.Context, generated.IssueWorkflowActionReceipt) (bool, error) { return true, nil }), Now: func() time.Time { return now }}
		}
		workers := []workflowaction.Worker{makeWorker(conn), makeWorker(other)}
		start := make(chan struct{})
		errs := make(chan error, len(workers))
		var wg sync.WaitGroup
		for index := range workers {
			wg.Add(1)
			go func(worker *workflowaction.Worker) {
				defer wg.Done()
				<-start
				errs <- worker.ReconcileOnce(ctx)
			}(&workers[index])
		}
		close(start)
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("concurrent worker error=%v", err)
			}
		}
		var state string
		var leases, attempts, tasks int
		if err := conn.QueryRow(ctx, `SELECT state,lease_count,effect_attempt_count FROM issue_workflow_action_receipt WHERE id=$1`, actionID).Scan(&state, &leases, &attempts); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE workflow_action_id=$1`, actionID).Scan(&tasks); err != nil {
			t.Fatal(err)
		}
		if state != "settled" || leases != 1 || attempts != 1 || tasks != 1 {
			t.Fatalf("concurrent result state=%s leases=%d attempts=%d tasks=%d", state, leases, attempts, tasks)
		}
	})

	t.Run("correction task creation serializes with implementer archive", func(t *testing.T) {
		for _, archiveWins := range []bool{true, false} {
			name := "create wins then archive cleanup cancels"
			if archiveWins {
				name = "archive wins with zero task"
			}
			t.Run(name, func(t *testing.T) {
				workspaceID, issueID := seedLoadableWorkflowV2(t, ctx, conn, 180)
				agentID, runtimeID, operationID := workflowV2UUID(193), workflowV2UUID(194), workflowV2UUID(185)
				prepareWorkflowCorrectionFixture(t, ctx, conn, workspaceID, issueID, 180)
				if _, err := conn.Exec(ctx, `INSERT INTO agent_runtime(id,workspace_id) VALUES($1,$2)`, runtimeID, workspaceID); err != nil {
					t.Fatal(err)
				}
				if _, err := conn.Exec(ctx, `INSERT INTO agent(id,workspace_id,runtime_id) VALUES($1,$2,$3)`, agentID, workspaceID, runtimeID); err != nil {
					t.Fatal(err)
				}
				actionID, commandID := pgtype.UUID{Bytes: uuid.New(), Valid: true}, pgtype.UUID{Bytes: uuid.New(), Valid: true}
				nowValue := time.Date(2026, 9, 10, 17, 30, 0, 0, time.UTC)
				action, err := queries.CreateIssueWorkflowActionReceipt(ctx, generated.CreateIssueWorkflowActionReceiptParams{ID: actionID, WorkspaceID: workspaceID, IssueID: issueID, ActionKey: "archive-race", RequestHash: "archive-race-hash", CommandID: commandID, ActionType: "review_correction", OperationID: operationID, Payload: workflowCorrectionActionPayload(t, 180, operationID, agentID), NotBefore: workflowV2Time(nowValue), MaxAttempts: 3})
				if err != nil {
					t.Fatal(err)
				}
				leaseToken := pgtype.UUID{Bytes: uuid.New(), Valid: true}
				claimed, err := queries.ClaimIssueWorkflowActionExact(ctx, generated.ClaimIssueWorkflowActionExactParams{LeaseToken: leaseToken, LeaseExpiresAt: workflowV2Time(nowValue.Add(time.Hour)), WorkspaceID: workspaceID, IssueID: issueID, ID: action.ID, ExpectedRevision: action.Revision, Now: workflowV2Time(nowValue)})
				if err != nil {
					t.Fatal(err)
				}
				begun, err := queries.BeginIssueWorkflowActionAttempt(ctx, generated.BeginIssueWorkflowActionAttemptParams{Now: workflowV2Time(nowValue), WorkspaceID: workspaceID, IssueID: issueID, ID: action.ID, ExpectedRevision: claimed.Revision, LeaseToken: leaseToken})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					_, _ = conn.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE issue_id=$1`, issueID)
					_ = queries.DeleteIssueWorkflowV2RecordsByIssue(context.Background(), generated.DeleteIssueWorkflowV2RecordsByIssueParams{WorkspaceID: workspaceID, IssueID: issueID})
					_, _ = conn.Exec(context.Background(), `DELETE FROM agent WHERE id=$1`, agentID)
					_, _ = conn.Exec(context.Background(), `DELETE FROM agent_runtime WHERE id=$1`, runtimeID)
					_, _ = conn.Exec(context.Background(), `DELETE FROM issue WHERE id=$1`, issueID)
					_, _ = conn.Exec(context.Background(), `DELETE FROM workspace WHERE id=$1`, workspaceID)
				})
				effectConn, err := workflowV2TestConnection(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer effectConn.Close(context.Background())
				effectQueries := generated.New(effectConn)
				effect := workflowaction.LocalTaskEffect{Starter: effectConn, Queries: effectQueries, Now: func() time.Time { return nowValue }}
				if archiveWins {
					archiveTx, err := conn.Begin(ctx)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := archiveTx.Exec(ctx, `UPDATE agent SET archived_at=now() WHERE id=$1`, agentID); err != nil {
						t.Fatal(err)
					}
					result := make(chan error, 1)
					go func() { _, err := effect.Apply(context.Background(), begun); result <- err }()
					select {
					case err := <-result:
						_ = archiveTx.Rollback(ctx)
						t.Fatalf("task creation bypassed uncommitted archive: %v", err)
					case <-time.After(100 * time.Millisecond):
					}
					if err := archiveTx.Commit(ctx); err != nil {
						t.Fatal(err)
					}
					if err := <-result; err == nil {
						t.Fatal("archived implementer admitted a correction task")
					}
					var tasks int
					if err := conn.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE workflow_action_id=$1`, actionID).Scan(&tasks); err != nil || tasks != 0 {
						t.Fatalf("archive-wins task residue=%d err=%v", tasks, err)
					}
					return
				}

				entered, release := make(chan struct{}), make(chan struct{})
				effect.Failpoint = func(stage string) error {
					if stage == "after_owner_locks" {
						close(entered)
						<-release
					}
					return nil
				}
				applyResult := make(chan error, 1)
				go func() { _, err := effect.Apply(context.Background(), begun); applyResult <- err }()
				<-entered
				archiveTx, err := conn.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				archiveResult := make(chan error, 1)
				go func() {
					_, err := archiveTx.Exec(context.Background(), `UPDATE agent SET archived_at=now() WHERE id=$1`, agentID)
					archiveResult <- err
				}()
				select {
				case err := <-archiveResult:
					_ = archiveTx.Rollback(ctx)
					t.Fatalf("archive bypassed correction agent lock: %v", err)
				case <-time.After(100 * time.Millisecond):
				}
				close(release)
				if err := <-applyResult; err != nil {
					_ = archiveTx.Rollback(ctx)
					t.Fatal(err)
				}
				if err := <-archiveResult; err != nil {
					t.Fatal(err)
				}
				if err := archiveTx.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				cancelled, err := conn.Exec(ctx, `UPDATE agent_task_queue SET status='cancelled' WHERE agent_id=$1 AND status IN ('queued','dispatched','running','waiting_local_directory','deferred')`, agentID)
				if err != nil || cancelled.RowsAffected() != 1 {
					t.Fatalf("archive cleanup cancelled=%d err=%v", cancelled.RowsAffected(), err)
				}
				var queued int
				if err := conn.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE workflow_action_id=$1 AND status='queued'`, actionID).Scan(&queued); err != nil || queued != 0 {
					t.Fatalf("create-wins orphan queued tasks=%d err=%v", queued, err)
				}
			})
		}
	})

	t.Run("slow effect cannot settle an expired lease and is reconciled", func(t *testing.T) {
		workspaceID, issueID := seedLoadableWorkflowV2(t, ctx, conn, 180)
		actionID, operationID := workflowV2UUID(198), workflowV2UUID(185)
		clock := time.Date(2026, 9, 10, 17, 30, 0, 0, time.UTC)
		if _, err := queries.CreateIssueWorkflowActionReceipt(ctx, generated.CreateIssueWorkflowActionReceiptParams{ID: actionID, WorkspaceID: workspaceID, IssueID: issueID, ActionKey: "slow-effect", RequestHash: "slow-effect-hash", CommandID: workflowV2UUID(199), ActionType: "review_correction", OperationID: operationID, Payload: []byte(`{}`), NotBefore: workflowV2Time(clock), MaxAttempts: 3}); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = queries.DeleteIssueWorkflowV2RecordsByIssue(context.Background(), generated.DeleteIssueWorkflowV2RecordsByIssueParams{WorkspaceID: workspaceID, IssueID: issueID})
			_, _ = conn.Exec(context.Background(), `DELETE FROM issue WHERE id=$1`, issueID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM workspace WHERE id=$1`, workspaceID)
		})
		applied := false
		effect := workflowaction.EffectFuncs{
			ObserveFunc: func(_ context.Context, action generated.IssueWorkflowActionReceipt) (workflowaction.Observation, error) {
				if applied {
					return workflowaction.Observation{Kind: workflowaction.ObservationFound, TaskID: workflowV2UUID(197), ReceiptRevision: action.Revision}, nil
				}
				return workflowaction.Observation{Kind: workflowaction.ObservationAbsent}, nil
			},
			ApplyFunc: func(context.Context, generated.IssueWorkflowActionReceipt) (workflowaction.Observation, error) {
				applied = true
				clock = clock.Add(2 * time.Minute)
				return workflowaction.Observation{Kind: workflowaction.ObservationFound, TaskID: workflowV2UUID(197)}, nil
			},
		}
		worker := workflowaction.Worker{Starter: conn, Queries: queries, Effect: effect, Capacity: workflowaction.CapacityFunc(func(context.Context, generated.IssueWorkflowActionReceipt) (bool, error) { return true, nil }), Now: func() time.Time { return clock }, LeaseDuration: time.Minute}
		if err := worker.ReconcileOnce(ctx); err != nil {
			t.Fatal(err)
		}
		var state string
		var revision, leases, attempts int
		if err := conn.QueryRow(ctx, `SELECT state,revision,lease_count,effect_attempt_count FROM issue_workflow_action_receipt WHERE id=$1`, actionID).Scan(&state, &revision, &leases, &attempts); err != nil {
			t.Fatal(err)
		}
		if state != "leased" || revision != 3 || leases != 1 || attempts != 1 {
			t.Fatalf("expired settlement mutated action state=%s revision=%d leases=%d attempts=%d", state, revision, leases, attempts)
		}
		if err := worker.ReconcileOnce(ctx); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT state,revision,lease_count,effect_attempt_count FROM issue_workflow_action_receipt WHERE id=$1`, actionID).Scan(&state, &revision, &leases, &attempts); err != nil {
			t.Fatal(err)
		}
		if state != "settled" || revision != 5 || leases != 2 || attempts != 1 {
			t.Fatalf("expired settlement recovery state=%s revision=%d leases=%d attempts=%d", state, revision, leases, attempts)
		}
	})

	t.Run("semantic observation mismatch reaches bounded technical stop", func(t *testing.T) {
		workspaceID, issueID := seedLoadableWorkflowV2(t, ctx, conn, 180)
		agentID, runtimeID, operationID, actionID, controllerID := workflowV2UUID(193), workflowV2UUID(194), workflowV2UUID(185), workflowV2UUID(198), workflowV2UUID(197)
		prepareWorkflowCorrectionFixture(t, ctx, conn, workspaceID, issueID, 180)
		if _, err := conn.Exec(ctx, `INSERT INTO agent_runtime(id,workspace_id) VALUES($1,$2)`, runtimeID, workspaceID); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, `INSERT INTO agent(id,workspace_id,runtime_id) VALUES($1,$2,$3)`, agentID, workspaceID, runtimeID); err != nil {
			t.Fatal(err)
		}
		start := time.Date(2026, 9, 10, 17, 45, 0, 0, time.UTC)
		payload := workflowCorrectionActionPayload(t, 180, operationID, agentID)
		if _, err := queries.CreateIssueWorkflowActionReceipt(ctx, generated.CreateIssueWorkflowActionReceiptParams{ID: actionID, WorkspaceID: workspaceID, IssueID: issueID, ActionKey: "bounded-mismatch", RequestHash: "bounded-mismatch-hash", CommandID: workflowV2UUID(199), ActionType: "review_correction", OperationID: operationID, Payload: payload, NotBefore: workflowV2Time(start), MaxAttempts: 3}); err != nil {
			t.Fatal(err)
		}
		taskID := pgtype.UUID{Bytes: uuid.NewSHA1(uuid.UUID(actionID.Bytes), []byte("review_correction/task")), Valid: true}
		if _, err := conn.Exec(ctx, `INSERT INTO agent_task_queue(id,agent_id,runtime_id,issue_id,status,context,originator_source,trigger_evidence_kind,trigger_evidence_ref_id,workflow_action_id,workflow_operation_id,workflow_branch_key) VALUES($1,$2,$3,$4,'queued','{"wrong":true}','workflow_action','workflow_action',$5,$5,$6,'main')`, taskID, agentID, runtimeID, issueID, actionID, operationID); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, _ = conn.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE workflow_action_id=$1`, actionID)
			_ = queries.DeleteIssueWorkflowV2RecordsByIssue(context.Background(), generated.DeleteIssueWorkflowV2RecordsByIssueParams{WorkspaceID: workspaceID, IssueID: issueID})
			_, _ = conn.Exec(context.Background(), `DELETE FROM agent WHERE id=$1`, agentID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM agent_runtime WHERE id=$1`, runtimeID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM issue WHERE id=$1`, issueID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM workspace WHERE id=$1`, workspaceID)
		})
		clock := start
		service := workflowcommand.Service{Loader: workflowcommand.Loader{Now: func() time.Time { return clock }}, Now: func() time.Time { return clock }, ControllerID: controllerID}
		worker := workflowaction.Worker{Starter: conn, Queries: queries, Effect: workflowaction.LocalTaskEffect{Starter: conn, Queries: queries, Now: func() time.Time { return clock }}, Capacity: workflowaction.CapacityFunc(func(context.Context, generated.IssueWorkflowActionReceipt) (bool, error) { return true, nil }), CommandService: service, ControllerID: controllerID, Now: func() time.Time { return clock }, RetryDelay: time.Second}
		for attempt := 1; attempt <= 3; attempt++ {
			if err := worker.ReconcileOnce(ctx); err != nil {
				t.Fatal(err)
			}
			var state string
			var observations, effects int
			if err := conn.QueryRow(ctx, `SELECT state,observation_failure_count,effect_attempt_count FROM issue_workflow_action_receipt WHERE id=$1`, actionID).Scan(&state, &observations, &effects); err != nil {
				t.Fatal(err)
			}
			wantState := "reconciling"
			if attempt == 3 {
				wantState = "dead_letter"
			}
			if state != wantState || observations != attempt || effects != 0 {
				t.Fatalf("observation failure %d state=%s observations=%d effects=%d", attempt, state, observations, effects)
			}
			clock = clock.Add(2 * time.Second)
		}
		var executionState string
		var commands int
		if err := conn.QueryRow(ctx, `SELECT state FROM issue_workflow_execution WHERE id=$1`, operationID).Scan(&executionState); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_command WHERE workspace_id=$1 AND issue_id=$2`, workspaceID, issueID).Scan(&commands); err != nil {
			t.Fatal(err)
		}
		if executionState != "technically_stopped" || commands != 1 {
			t.Fatalf("bounded mismatch execution=%s commands=%d", executionState, commands)
		}
	})

	t.Run("review correction retry ceiling technical-stops atomically", func(t *testing.T) {
		workspaceID, issueID := seedLoadableWorkflowV2(t, ctx, conn, 220)
		agentID, runtimeID, operationID, controllerID := workflowV2UUID(233), workflowV2UUID(234), workflowV2UUID(225), workflowV2UUID(239)
		if _, err := conn.Exec(ctx, `INSERT INTO agent_runtime(id,workspace_id) VALUES($1,$2)`, runtimeID, workspaceID); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, `INSERT INTO agent(id,workspace_id,runtime_id) VALUES($1,$2,$3)`, agentID, workspaceID, runtimeID); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, _ = conn.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE issue_id=$1`, issueID)
			_ = queries.DeleteIssueWorkflowV2RecordsByIssue(context.Background(), generated.DeleteIssueWorkflowV2RecordsByIssueParams{WorkspaceID: workspaceID, IssueID: issueID})
			_, _ = conn.Exec(context.Background(), `DELETE FROM agent WHERE id=$1`, agentID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM agent_runtime WHERE id=$1`, runtimeID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM issue WHERE id=$1`, issueID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM workspace WHERE id=$1`, workspaceID)
		})
		now := time.Date(2026, 9, 10, 18, 0, 0, 0, time.UTC)
		create := func(id, commandID pgtype.UUID, key string) generated.IssueWorkflowActionReceipt {
			row, err := queries.CreateIssueWorkflowActionReceipt(ctx, generated.CreateIssueWorkflowActionReceiptParams{ID: id, WorkspaceID: workspaceID, IssueID: issueID, ActionKey: key, RequestHash: key + "-hash", CommandID: commandID, ActionType: "review_correction", OperationID: operationID, Payload: []byte(`{"operation_id":"` + util.UUIDToString(operationID) + `"}`), NotBefore: workflowV2Time(now), MaxAttempts: 1})
			if err != nil {
				t.Fatal(err)
			}
			return row
		}
		absentFailure := workflowaction.EffectFuncs{
			ObserveFunc: func(context.Context, generated.IssueWorkflowActionReceipt) (workflowaction.Observation, error) {
				return workflowaction.Observation{Kind: workflowaction.ObservationAbsent}, nil
			},
			ApplyFunc: func(context.Context, generated.IssueWorkflowActionReceipt) (workflowaction.Observation, error) {
				return workflowaction.Observation{Kind: workflowaction.ObservationAbsent}, errors.New("correction task creation failed")
			},
		}
		newWorker := func(failpoint func(string) error) workflowaction.Worker {
			service := workflowcommand.Service{Loader: workflowcommand.Loader{Now: func() time.Time { return now }}, Now: func() time.Time { return now }, ControllerID: controllerID, Failpoint: failpoint}
			return workflowaction.Worker{Starter: conn, Queries: queries, Effect: absentFailure, Capacity: workflowaction.CapacityFunc(func(context.Context, generated.IssueWorkflowActionReceipt) (bool, error) { return true, nil }), CommandService: service, ControllerID: controllerID, Now: func() time.Time { return now }}
		}

		failed := create(workflowV2UUID(240), workflowV2UUID(241), "worker-deadletter-failpoint")
		worker := newWorker(func(stage string) error {
			if stage == "after_domain" {
				return errors.New("forced technical-stop rollback")
			}
			return nil
		})
		if err := worker.ReconcileOnce(ctx); err == nil || !strings.Contains(err.Error(), "forced technical-stop rollback") {
			t.Fatalf("deadletter failpoint error=%v", err)
		}
		var state, executionState string
		var revision, commandCount, pictureCount int
		if err := conn.QueryRow(ctx, `SELECT state,revision FROM issue_workflow_action_receipt WHERE id=$1`, failed.ID).Scan(&state, &revision); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT state FROM issue_workflow_execution WHERE id=$1`, operationID).Scan(&executionState); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_command WHERE workspace_id=$1 AND issue_id=$2`, workspaceID, issueID).Scan(&commandCount); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM issue_current_picture WHERE workspace_id=$1 AND issue_id=$2`, workspaceID, issueID).Scan(&pictureCount); err != nil {
			t.Fatal(err)
		}
		if state != "leased" || revision != 3 || executionState != "working" || commandCount != 0 || pictureCount != 0 {
			t.Fatalf("rolled-back deadletter action=(%s,%d) execution=%s commands=%d pictures=%d", state, revision, executionState, commandCount, pictureCount)
		}

		if _, err := conn.Exec(ctx, `UPDATE issue_workflow_action_receipt SET lease_expires_at=$2 WHERE id=$1`, failed.ID, now.Add(-time.Second)); err != nil {
			t.Fatal(err)
		}
		worker = newWorker(nil)
		if err := worker.ReconcileOnce(ctx); err != nil {
			t.Fatal(err)
		}
		var terminalReason string
		if err := conn.QueryRow(ctx, `SELECT state,revision FROM issue_workflow_action_receipt WHERE id=$1`, failed.ID).Scan(&state, &revision); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT state,terminal_reason FROM issue_workflow_execution WHERE id=$1`, operationID).Scan(&executionState, &terminalReason); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_command WHERE workspace_id=$1 AND issue_id=$2`, workspaceID, issueID).Scan(&commandCount); err != nil {
			t.Fatal(err)
		}
		if state != "dead_letter" || executionState != "technically_stopped" || terminalReason == "" || commandCount != 1 {
			t.Fatalf("deadletter action=(%s,%d) execution=(%s,%q) commands=%d", state, revision, executionState, terminalReason, commandCount)
		}
	})

	t.Run("typed correction and independent review complete an explicit non-live issue", func(t *testing.T) {
		const base byte = 52
		workspaceID, issueID := seedLoadableWorkflowV2(t, ctx, conn, base)
		operationID, candidateID := workflowV2UUID(base+5), workflowV2UUID(base+11)
		implementerID, reviewerID := workflowV2UUID(base+13), workflowV2UUID(base+14)
		implementerRuntime, reviewerRuntime := workflowV2UUID(91), workflowV2UUID(92)
		initialTask, initialAction, initialCommand := workflowV2UUID(93), workflowV2UUID(94), workflowV2UUID(95)
		nowValue := time.Date(2026, 9, 10, 19, 0, 0, 0, time.UTC)
		for _, statement := range []struct {
			sql  string
			args []any
		}{
			{`UPDATE issue_workflow_scope SET requires_deployment=false,source_plan_revision=1 WHERE workspace_id=$1 AND issue_id=$2`, []any{workspaceID, issueID}},
			{`UPDATE issue_workflow_authority SET envelope='{}' WHERE workspace_id=$1 AND issue_id=$2`, []any{workspaceID, issueID}},
			{`UPDATE issue_workflow_execution SET state='in_review',actor_type='reviewer',actor_id=$3,task_id=$4,candidate_id=$5 WHERE workspace_id=$1 AND issue_id=$2`, []any{workspaceID, issueID, reviewerID, initialTask, candidateID}},
			{`UPDATE issue_workflow_review SET outcome='in_progress',criteria='[]',coverage='{}',evidence_refs='[]',correction_count=1,max_corrections=2 WHERE workspace_id=$1 AND issue_id=$2`, []any{workspaceID, issueID}},
			{`UPDATE issue_workflow_release SET state='pending',target_key='',target_generation=0,facets='{}',started_at=NULL,deployed_at=NULL WHERE workspace_id=$1 AND issue_id=$2`, []any{workspaceID, issueID}},
			{`UPDATE issue_workflow_concern_head SET state='in_review' WHERE workspace_id=$1 AND issue_id=$2 AND concern_type='execution'`, []any{workspaceID, issueID}},
			{`UPDATE issue_workflow_concern_head SET state='in_progress' WHERE workspace_id=$1 AND issue_id=$2 AND concern_type='review'`, []any{workspaceID, issueID}},
			{`UPDATE issue_workflow_concern_head SET state='pending' WHERE workspace_id=$1 AND issue_id=$2 AND concern_type='release'`, []any{workspaceID, issueID}},
			{`UPDATE issue_workflow_concern_head SET source_record_id=$3,source_record_revision=1,state='current' WHERE workspace_id=$1 AND issue_id=$2 AND concern_type='evidence'`, []any{workspaceID, issueID, workflowV2UUID(base + 17)}},
			{`DELETE FROM issue_workflow_evidence WHERE workspace_id=$1 AND issue_id=$2 AND id=$3`, []any{workspaceID, issueID, workflowV2UUID(base + 9)}},
			{`INSERT INTO agent_runtime(id,workspace_id) VALUES($1,$3),($2,$3)`, []any{implementerRuntime, reviewerRuntime, workspaceID}},
			{`INSERT INTO agent(id,workspace_id,runtime_id) VALUES($1,$3,$4),($2,$3,$5)`, []any{implementerID, reviewerID, workspaceID, implementerRuntime, reviewerRuntime}},
			{`INSERT INTO agent_task_queue(id,agent_id,runtime_id,issue_id,status,workflow_action_id,workflow_operation_id,workflow_branch_key) VALUES($1,$2,$3,$4,'running',$5,$6,'main')`, []any{initialTask, reviewerID, reviewerRuntime, issueID, initialAction, operationID}},
			{`INSERT INTO issue_workflow_action_receipt(id,workspace_id,issue_id,action_key,request_hash,command_id,action_type,state,operation_id,payload,bound_task_id) VALUES($1,$2,$3,'review/vertical','seed',$4,'review_candidate','settled',$5,jsonb_build_object('branch_key','main','reviewer_id',$6::text,'candidate_id',$7::text,'scope_revision',1),$8)`, []any{initialAction, workspaceID, issueID, initialCommand, operationID, reviewerID, candidateID, initialTask}},
		} {
			if _, err := conn.Exec(ctx, statement.sql, statement.args...); err != nil {
				t.Fatal(err)
			}
		}
		var seededTarget string
		if err := conn.QueryRow(ctx, `SELECT target_key FROM issue_workflow_release WHERE workspace_id=$1 AND issue_id=$2`, workspaceID, issueID).Scan(&seededTarget); err != nil || seededTarget != "" {
			t.Fatalf("seeded pending release target=%q err=%v", seededTarget, err)
		}
		t.Cleanup(func() {
			_, _ = conn.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE issue_id=$1`, issueID)
			_ = queries.DeleteIssueWorkflowV2RecordsByIssue(context.Background(), generated.DeleteIssueWorkflowV2RecordsByIssueParams{WorkspaceID: workspaceID, IssueID: issueID})
			_, _ = conn.Exec(context.Background(), `DELETE FROM agent WHERE id IN ($1,$2)`, implementerID, reviewerID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM agent_runtime WHERE id IN ($1,$2)`, implementerRuntime, reviewerRuntime)
			_, _ = conn.Exec(context.Background(), `DELETE FROM issue WHERE id=$1`, issueID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM workspace WHERE id=$1`, workspaceID)
		})
		now := func() time.Time { return nowValue }
		controllerID := workflowV2UUID(96)
		service := workflowcommand.Service{Starter: conn, Queries: queries, Loader: workflowcommand.Loader{Now: now}, Now: now, ControllerID: controllerID}
		failed, err := service.Execute(ctx, workflowcommand.Principal{ActorID: reviewerID, ActorType: "agent_task", TaskID: initialTask, ActionKey: "review/vertical"}, workflowcommand.Request{
			WorkspaceID: workspaceID, IssueID: issueID, CommandKey: "vertical-review-failed", Kind: workflowstatev2.CommandReviewFailed,
			Expected:     []workflowstatev2.ExpectedRevision{{Concern: workflowstatev2.ConcernExecution, Revision: 1}, {Concern: workflowstatev2.ConcernReview, Revision: 1}, {Concern: workflowstatev2.ConcernEvidence, Revision: 1}},
			EvidenceRefs: []string{util.UUIDToString(workflowV2UUID(base + 17))}, CriteriaRefs: []string{"acceptance"}, CoverageRefs: []string{"tests"}, GapRefs: []string{"defect"}, RequiredCorrection: "repair defect", CorrectionCount: 1, MaxCorrections: 2,
		})
		if err != nil || len(failed.ActionIDs) != 1 {
			t.Fatalf("review failure=(%+v,%v)", failed, err)
		}
		worker := workflowaction.Worker{Starter: conn, Queries: queries, Effect: workflowaction.LocalTaskEffect{Starter: conn, Queries: queries, Now: now}, Capacity: workflowaction.CapacityFunc(func(context.Context, generated.IssueWorkflowActionReceipt) (bool, error) { return true, nil }), CommandService: service, ControllerID: controllerID, Now: now}
		if err := worker.ReconcileOnce(ctx); err != nil {
			t.Fatal(err)
		}
		correctionAction := workflowV2UUIDString(failed.ActionIDs[0])
		var correctionTask pgtype.UUID
		if err := conn.QueryRow(ctx, `SELECT bound_task_id FROM issue_workflow_action_receipt WHERE id=$1`, correctionAction).Scan(&correctionTask); err != nil || !correctionTask.Valid {
			t.Fatalf("correction task=%v err=%v", correctionTask, err)
		}
		correctionResult, _ := json.Marshal(map[string]any{"workflow_result": map[string]any{"type": "correction", "source_revision": "git:fixed", "artifact": map[string]any{"commit": "fixed"}, "build_contract": map[string]any{"tests": "passed"}, "evidence": map[string]any{"summary": "corrected"}}})
		if _, err := conn.Exec(ctx, `UPDATE agent_task_queue SET status='completed',result=$2,completed_at=$3 WHERE id=$1`, correctionTask, correctionResult, nowValue); err != nil {
			t.Fatal(err)
		}
		submitted, err := service.ReconcileWorkflowTaskResult(ctx, correctionTask)
		if err != nil || len(submitted.ActionIDs) != 1 {
			t.Fatalf("submit correction=(%+v,%v)", submitted, err)
		}
		if err := worker.ReconcileOnce(ctx); err != nil {
			t.Fatal(err)
		}
		reviewAction := workflowV2UUIDString(submitted.ActionIDs[0])
		var reviewTask pgtype.UUID
		if err := conn.QueryRow(ctx, `SELECT bound_task_id FROM issue_workflow_action_receipt WHERE id=$1`, reviewAction).Scan(&reviewTask); err != nil || !reviewTask.Valid {
			t.Fatalf("review task=%v err=%v", reviewTask, err)
		}
		var reviewAgent pgtype.UUID
		var packet []byte
		if err := conn.QueryRow(ctx, `SELECT agent_id,context FROM agent_task_queue WHERE id=$1`, reviewTask).Scan(&reviewAgent, &packet); err != nil || reviewAgent != reviewerID || reviewAgent == implementerID || !json.Valid(packet) {
			t.Fatalf("review assignment agent=%v packet=%s err=%v", reviewAgent, packet, err)
		}
		reviewResult, _ := json.Marshal(map[string]any{"workflow_result": map[string]any{"type": "review", "verdict": "passed", "evidence": map[string]any{"summary": "independent pass"}, "criteria_refs": []string{"acceptance"}, "coverage_refs": []string{"tests"}}})
		if _, err := conn.Exec(ctx, `UPDATE agent_task_queue SET status='completed',result=$2,completed_at=$3 WHERE id=$1`, reviewTask, reviewResult, nowValue); err != nil {
			t.Fatal(err)
		}
		// A completed task from a superseded review attempt on the same
		// operation and candidate must be quarantined durably without stopping
		// the current review or starving the valid
		// review result later in the same ordered scan. This also models a
		// process crash after terminal task commit but before reconciliation.
		poisonID := func(name string) pgtype.UUID {
			return pgtype.UUID{Bytes: uuid.NewSHA1(uuid.NameSpaceOID, []byte(util.UUIDToString(workspaceID)+"/"+name)), Valid: true}
		}
		poisonTask, poisonAction := poisonID("stale-poison-task"), poisonID("stale-poison-action")
		poisonOperation, poisonCommand := operationID, poisonID("stale-poison-command")
		priorReview := poisonID("prior-review")
		var correctedCandidateID, implementationEvidenceID pgtype.UUID
		if err := conn.QueryRow(ctx, `SELECT candidate_id FROM issue_workflow_execution WHERE workspace_id=$1 AND issue_id=$2 AND id=$3`, workspaceID, issueID, operationID).Scan(&correctedCandidateID); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT source_record_id FROM issue_workflow_concern_head WHERE workspace_id=$1 AND issue_id=$2 AND concern_type='evidence'`, workspaceID, issueID).Scan(&implementationEvidenceID); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, `INSERT INTO issue_workflow_review(id,workspace_id,issue_id,candidate_id,candidate_revision,scope_id,scope_revision,reviewer_id,outcome,criteria,coverage,evidence_refs,superseded_at) VALUES($1,$2,$3,$4,1,$5,1,$6,'in_progress','[]','{}','[]',$7)`, priorReview, workspaceID, issueID, correctedCandidateID, workflowV2UUID(base+3), reviewerID, nowValue); err != nil {
			t.Fatalf("insert prior review attempt: %v", err)
		}
		malformedResult := []byte(`{"workflow_result":{}}`)
		poisonPayload, _ := json.Marshal(map[string]any{"operation_id": util.UUIDToString(poisonOperation), "branch_key": "main", "candidate_id": util.UUIDToString(correctedCandidateID), "candidate_revision": 1, "scope_id": util.UUIDToString(workflowV2UUID(base + 3)), "scope_revision": 1, "acceptance_version": 1, "review_id": util.UUIDToString(priorReview), "review_revision": 1, "reviewer_id": util.UUIDToString(reviewerID), "reviewer_runtime_id": util.UUIDToString(reviewerRuntime), "implementation_evidence_refs": []string{util.UUIDToString(implementationEvidenceID)}})
		if _, err := conn.Exec(ctx, `
			INSERT INTO agent_task_queue(id,agent_id,runtime_id,issue_id,status,result,completed_at,workflow_action_id,workflow_operation_id,workflow_branch_key)
			VALUES($1,$2,$3,$4,'completed',$5,$6,$7,$8,'main')
		`, poisonTask, reviewerID, reviewerRuntime, issueID, malformedResult, nowValue.Add(-time.Minute), poisonAction, poisonOperation); err != nil {
			t.Fatalf("insert stale poison task: %v", err)
		}
		if _, err := conn.Exec(ctx, `
			INSERT INTO issue_workflow_action_receipt(id,workspace_id,issue_id,action_key,request_hash,command_id,action_type,state,operation_id,payload,bound_task_id)
			VALUES($1,$2,$3,'stale/poison','seed',$4,'review_candidate','settled',$5,$6,$7)
		`, poisonAction, workspaceID, issueID, poisonCommand, poisonOperation, poisonPayload, poisonTask); err != nil {
			t.Fatalf("insert stale poison receipt: %v", err)
		}
		staleService := service
		staleService.ControllerID = pgtype.UUID{}
		if err := staleService.ReconcilePendingTaskResults(ctx, 25); err != nil {
			t.Fatalf("poison-isolated recovery scan: %v", err)
		}
		var poisonState, operationAfterPoison string
		var poisonRevision, projectionAfterPoison int64
		if err := conn.QueryRow(ctx, `SELECT reconciliation_state,revision FROM issue_workflow_action_receipt WHERE id=$1`, poisonAction).Scan(&poisonState, &poisonRevision); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT state FROM issue_workflow_execution WHERE workspace_id=$1 AND issue_id=$2 AND id=$3`, workspaceID, issueID, operationID).Scan(&operationAfterPoison); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT revision FROM issue_current_picture WHERE workspace_id=$1 AND issue_id=$2`, workspaceID, issueID).Scan(&projectionAfterPoison); err != nil {
			t.Fatal(err)
		}
		if poisonState != "failed" || poisonRevision != 2 || operationAfterPoison != "completed" {
			t.Fatalf("stale poison state=%s revision=%d current operation=%s", poisonState, poisonRevision, operationAfterPoison)
		}
		firstRejected, err := staleService.ReconcileWorkflowTaskResult(ctx, poisonTask)
		if err != nil || firstRejected.State != "rejected" {
			t.Fatalf("first quarantined callback=(%+v,%v)", firstRejected, err)
		}
		secondRejected, err := staleService.ReconcileWorkflowTaskResult(ctx, poisonTask)
		if err != nil || !reflect.DeepEqual(firstRejected, secondRejected) {
			t.Fatalf("repeated quarantined callback=(%+v,%v), want %+v", secondRejected, err, firstRejected)
		}
		if err := staleService.ReconcilePendingTaskResults(ctx, 25); err != nil {
			t.Fatalf("repeat poison-isolated recovery scan: %v", err)
		}
		var poisonRevisionAfter, projectionAfterRepeat int64
		if err := conn.QueryRow(ctx, `SELECT revision FROM issue_workflow_action_receipt WHERE id=$1`, poisonAction).Scan(&poisonRevisionAfter); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT revision FROM issue_current_picture WHERE workspace_id=$1 AND issue_id=$2`, workspaceID, issueID).Scan(&projectionAfterRepeat); err != nil {
			t.Fatal(err)
		}
		if poisonRevisionAfter != poisonRevision || projectionAfterRepeat != projectionAfterPoison {
			t.Fatalf("quarantined result hot-looped: action %d->%d projection %d->%d", poisonRevision, poisonRevisionAfter, projectionAfterPoison, projectionAfterRepeat)
		}
		passed, err := service.ReconcileWorkflowTaskResult(ctx, reviewTask)
		if err != nil || len(passed.ActionIDs) != 1 {
			t.Fatalf("review pass=(%+v,%v)", passed, err)
		}
		if err := worker.ReconcileOnce(ctx); err != nil {
			t.Fatal(err)
		}
		var outcome, execution, release string
		if err := conn.QueryRow(ctx, `SELECT o.state,e.state,r.state FROM issue_workflow_outcome o JOIN issue_workflow_execution e USING(workspace_id,issue_id) JOIN issue_workflow_release r ON r.workspace_id=o.workspace_id AND r.issue_id=o.issue_id JOIN issue_workflow_concern_head rh ON rh.workspace_id=r.workspace_id AND rh.issue_id=r.issue_id AND rh.concern_type='release' AND rh.source_record_id=r.id WHERE o.workspace_id=$1 AND o.issue_id=$2`, workspaceID, issueID).Scan(&outcome, &execution, &release); err != nil {
			t.Fatal(err)
		}
		if outcome != "satisfied" || execution != "completed" || release != "not_required" {
			t.Fatalf("terminal state outcome=%s execution=%s release=%s", outcome, execution, release)
		}
		var ordinaryTasks int
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND workflow_action_id IS NULL`, issueID).Scan(&ordinaryTasks); err != nil || ordinaryTasks != 0 {
			t.Fatalf("ordinary/Mika tasks=%d err=%v", ordinaryTasks, err)
		}
	})

	t.Run("missing typed result is terminally quarantined and technically stops its current operation", func(t *testing.T) {
		const base byte = 130
		workspaceID, issueID := seedLoadableWorkflowV2(t, ctx, conn, base)
		prepareWorkflowCorrectionFixture(t, ctx, conn, workspaceID, issueID, base)
		operationID := workflowV2UUID(base + 5)
		fixtureID := func(name string) pgtype.UUID {
			return pgtype.UUID{Bytes: uuid.NewSHA1(uuid.NameSpaceOID, []byte(util.UUIDToString(workspaceID)+"/"+name)), Valid: true}
		}
		agentID, runtimeID := fixtureID("poison-agent"), fixtureID("poison-runtime")
		taskID, actionID, sourceCommand := fixtureID("poison-task"), fixtureID("poison-action"), fixtureID("poison-command")
		controllerID := fixtureID("poison-controller")
		nowValue := time.Date(2026, 9, 10, 20, 0, 0, 0, time.UTC)
		for _, statement := range []struct {
			sql  string
			args []any
		}{
			{`UPDATE issue_workflow_execution SET candidate_id=$3 WHERE workspace_id=$1 AND issue_id=$2`, []any{workspaceID, issueID, workflowV2UUID(base + 11)}},
			{`INSERT INTO agent_runtime(id,workspace_id) VALUES($1,$2)`, []any{runtimeID, workspaceID}},
			{`INSERT INTO agent(id,workspace_id,runtime_id) VALUES($1,$2,$3)`, []any{agentID, workspaceID, runtimeID}},
			{`INSERT INTO agent_task_queue(id,agent_id,runtime_id,issue_id,status,result,completed_at,workflow_action_id,workflow_operation_id,workflow_branch_key) VALUES($1,$2,$3,$4,'completed','{}',$5,$6,$7,'main')`, []any{taskID, agentID, runtimeID, issueID, nowValue, actionID, operationID}},
			{`INSERT INTO issue_workflow_action_receipt(id,workspace_id,issue_id,action_key,request_hash,command_id,action_type,state,operation_id,payload,bound_task_id) VALUES($1,$2,$3,'current/poison','seed',$4,'review_correction','settled',$5::uuid,jsonb_build_object('operation_id',$5::text,'branch_key','main','candidate_id',$6::text,'candidate_revision',1,'scope_id',$7::text,'scope_revision',1,'acceptance_version',1,'review_id',$8::text,'review_revision',1),$9)`, []any{actionID, workspaceID, issueID, sourceCommand, operationID, workflowV2UUID(base + 11), workflowV2UUID(base + 3), workflowV2UUID(base + 6), taskID}},
		} {
			if _, err := conn.Exec(ctx, statement.sql, statement.args...); err != nil {
				t.Fatal(err)
			}
		}
		t.Cleanup(func() {
			_, _ = conn.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE id=$1`, taskID)
			_ = queries.DeleteIssueWorkflowV2RecordsByIssue(context.Background(), generated.DeleteIssueWorkflowV2RecordsByIssueParams{WorkspaceID: workspaceID, IssueID: issueID})
			_, _ = conn.Exec(context.Background(), `DELETE FROM agent WHERE id=$1`, agentID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM agent_runtime WHERE id=$1`, runtimeID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM issue WHERE id=$1`, issueID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM workspace WHERE id=$1`, workspaceID)
		})
		now := func() time.Time { return nowValue }
		service := workflowcommand.Service{Starter: conn, Queries: queries, Loader: workflowcommand.Loader{Now: now}, Now: now, ControllerID: controllerID}
		first, err := service.ReconcileWorkflowTaskResult(ctx, taskID)
		if err != nil || first.State != "rejected" || first.ActionState != "failed" {
			t.Fatalf("missing result callback=(%+v,%v)", first, err)
		}
		var executionState, terminalReason, reconciliationState string
		var actionRevision, commandCount int64
		if err := conn.QueryRow(ctx, `SELECT state,terminal_reason FROM issue_workflow_execution WHERE id=$1`, operationID).Scan(&executionState, &terminalReason); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT reconciliation_state,revision FROM issue_workflow_action_receipt WHERE id=$1`, actionID).Scan(&reconciliationState, &actionRevision); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_command WHERE workspace_id=$1 AND issue_id=$2 AND command_key=$3`, workspaceID, issueID, "task/"+util.UUIDToString(taskID)+"/invalid-result/technical-stop").Scan(&commandCount); err != nil {
			t.Fatal(err)
		}
		if executionState != "technically_stopped" || terminalReason != "typed workflow result is missing or invalid" || reconciliationState != "failed" || actionRevision != 2 || commandCount != 1 {
			t.Fatalf("quarantine execution=(%s,%q) action=(%s,%d) commands=%d", executionState, terminalReason, reconciliationState, actionRevision, commandCount)
		}
		second, err := service.ReconcileWorkflowTaskResult(ctx, taskID)
		if err != nil || !reflect.DeepEqual(first, second) {
			t.Fatalf("repeated missing result callback=(%+v,%v), want %+v", second, err, first)
		}
		var revisionAfter, commandsAfter int64
		if err := conn.QueryRow(ctx, `SELECT revision FROM issue_workflow_action_receipt WHERE id=$1`, actionID).Scan(&revisionAfter); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_command WHERE workspace_id=$1 AND issue_id=$2`, workspaceID, issueID).Scan(&commandsAfter); err != nil {
			t.Fatal(err)
		}
		if revisionAfter != actionRevision || commandsAfter != commandCount {
			t.Fatalf("terminal callback replay mutated state: action %d->%d commands %d->%d", actionRevision, revisionAfter, commandCount, commandsAfter)
		}
	})

	t.Run("current picture rejects stale and lower revisions", func(t *testing.T) {
		workspaceID, issueID := workflowV2UUID(20), workflowV2UUID(21)
		base := workflowV2CurrentPictureParams(workspaceID, issueID, 5, 0)
		if _, err := queries.UpsertIssueCurrentPicture(ctx, base); err != nil {
			t.Fatal(err)
		}
		lower := workflowV2CurrentPictureParams(workspaceID, issueID, 4, 5)
		if _, err := queries.UpsertIssueCurrentPicture(ctx, lower); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("lower revision error = %v, want pgx.ErrNoRows", err)
		}
		stale := workflowV2CurrentPictureParams(workspaceID, issueID, 6, 4)
		if _, err := queries.UpsertIssueCurrentPicture(ctx, stale); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("stale expected revision error = %v, want pgx.ErrNoRows", err)
		}
		higher := workflowV2CurrentPictureParams(workspaceID, issueID, 6, 5)
		got, err := queries.UpsertIssueCurrentPicture(ctx, higher)
		if err != nil {
			t.Fatal(err)
		}
		if got.Revision != 6 {
			t.Fatalf("current picture revision = %d, want 6", got.Revision)
		}
	})

	t.Run("target head requires zero for create and advances by generation CAS", func(t *testing.T) {
		workspaceID, issueID := workflowV2UUID(70), workflowV2UUID(71)
		params := generated.AdvanceIssueWorkflowTargetHeadCASParams{
			WorkspaceID: workspaceID, TargetKey: "production", Generation: 1,
			ReleaseID: workflowV2UUID(72), IssueID: issueID,
			CandidateID: workflowV2UUID(73), State: "released", ExpectedGeneration: 9,
		}
		if _, err := queries.AdvanceIssueWorkflowTargetHeadCAS(ctx, params); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("absent target with nonzero expectation error = %v, want pgx.ErrNoRows", err)
		}
		params.ExpectedGeneration = 0
		created, err := queries.AdvanceIssueWorkflowTargetHeadCAS(ctx, params)
		if err != nil {
			t.Fatal(err)
		}
		if created.Generation != 1 || created.Revision != 1 {
			t.Fatalf("created target head = (generation %d, revision %d), want (1, 1)", created.Generation, created.Revision)
		}
		params.Generation = 2
		params.ExpectedGeneration = 0
		if _, err := queries.AdvanceIssueWorkflowTargetHeadCAS(ctx, params); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("stale target expectation error = %v, want pgx.ErrNoRows", err)
		}
		params.ExpectedGeneration = 1
		advanced, err := queries.AdvanceIssueWorkflowTargetHeadCAS(ctx, params)
		if err != nil {
			t.Fatal(err)
		}
		if advanced.Generation != 2 || advanced.Revision != 2 {
			t.Fatalf("advanced target head = (generation %d, revision %d), want (2, 2)", advanced.Generation, advanced.Revision)
		}
	})

	t.Run("release attempts preserve proposed and observed target generations", func(t *testing.T) {
		workspaceID, issueID := workflowV2UUID(170), workflowV2UUID(171)
		attempt := generated.CreateIssueWorkflowReleaseAttemptParams{
			ID: workflowV2UUID(172), WorkspaceID: workspaceID, IssueID: issueID,
			CandidateID: workflowV2UUID(173), CandidateRevision: 1,
			ScopeID: workflowV2UUID(174), ScopeRevision: 1,
			AuthorityID: workflowV2UUID(175), AuthorityRevision: 1,
			TargetKey: "production", TargetGeneration: 2, State: "authorized",
			Facets: []byte(`{}`), RecoveryPolicy: []byte(`{"mode":"rollback"}`), ExposedPopulation: []byte(`{}`),
			AcceptanceVersion: pgtype.Int8{Int64: 1, Valid: true}, OperationID: workflowV2UUID(176), BranchKey: "main",
			ExpectedTargetReleaseID: workflowV2UUID(177), ExpectedTargetGeneration: pgtype.Int8{Int64: 1, Valid: true},
			ReleaseKind: "forward", ReleaseOperatorID: workflowV2UUID(178), PlannedVerifierID: workflowV2UUID(179),
			BuildManifestDigest: pgtype.Text{String: "build-1", Valid: true}, ArtifactDigest: pgtype.Text{String: "artifact-1", Valid: true},
			RuntimeContractDigest: pgtype.Text{String: "runtime-1", Valid: true}, AuthorityCapability: pgtype.Text{String: "deploy", Valid: true},
			AuthorityEvidenceID: workflowV2UUID(180), RecoveryPolicyRevision: pgtype.Int8{Int64: 1, Valid: true},
		}
		created, err := queries.CreateIssueWorkflowReleaseAttempt(ctx, attempt)
		if err != nil {
			t.Fatal(err)
		}
		if created.TargetGeneration != 2 || created.ExpectedTargetGeneration.Int64 != 1 || created.HeadObservedAt.Valid {
			t.Fatalf("created attempt = %+v", created)
		}
		omissions := []struct {
			name   string
			mutate func(*generated.CreateIssueWorkflowReleaseAttemptParams)
		}{
			{"acceptance", func(p *generated.CreateIssueWorkflowReleaseAttemptParams) { p.AcceptanceVersion = pgtype.Int8{} }},
			{"operation", func(p *generated.CreateIssueWorkflowReleaseAttemptParams) { p.OperationID = pgtype.UUID{} }},
			{"branch", func(p *generated.CreateIssueWorkflowReleaseAttemptParams) { p.BranchKey = "" }},
			{"predecessor generation", func(p *generated.CreateIssueWorkflowReleaseAttemptParams) { p.ExpectedTargetGeneration = pgtype.Int8{} }},
			{"operator", func(p *generated.CreateIssueWorkflowReleaseAttemptParams) { p.ReleaseOperatorID = pgtype.UUID{} }},
			{"verifier", func(p *generated.CreateIssueWorkflowReleaseAttemptParams) { p.PlannedVerifierID = pgtype.UUID{} }},
			{"build digest", func(p *generated.CreateIssueWorkflowReleaseAttemptParams) { p.BuildManifestDigest = pgtype.Text{} }},
			{"artifact digest", func(p *generated.CreateIssueWorkflowReleaseAttemptParams) { p.ArtifactDigest = pgtype.Text{} }},
			{"runtime digest", func(p *generated.CreateIssueWorkflowReleaseAttemptParams) { p.RuntimeContractDigest = pgtype.Text{} }},
			{"authority capability", func(p *generated.CreateIssueWorkflowReleaseAttemptParams) { p.AuthorityCapability = pgtype.Text{} }},
			{"authority evidence", func(p *generated.CreateIssueWorkflowReleaseAttemptParams) { p.AuthorityEvidenceID = pgtype.UUID{} }},
			{"recovery policy revision", func(p *generated.CreateIssueWorkflowReleaseAttemptParams) { p.RecoveryPolicyRevision = pgtype.Int8{} }},
		}
		for i, omission := range omissions {
			bad := attempt
			bad.ID = workflowV2UUID(byte(200 + i))
			bad.TargetKey = fmt.Sprintf("incomplete-%d", i)
			omission.mutate(&bad)
			if _, err := queries.CreateIssueWorkflowReleaseAttempt(ctx, bad); err == nil {
				t.Fatalf("v2 attempt missing %s was accepted", omission.name)
			}
		}

		duplicate := attempt
		duplicate.ID = workflowV2UUID(181)
		duplicate.IssueID = workflowV2UUID(182)
		if _, err := queries.CreateIssueWorkflowReleaseAttempt(ctx, duplicate); err == nil {
			t.Fatal("duplicate proposed workspace/target generation was accepted")
		}

		badRecovery := attempt
		badRecovery.ID = workflowV2UUID(183)
		badRecovery.TargetKey = "recovery-target"
		badRecovery.TargetGeneration = 1
		badRecovery.ExpectedTargetGeneration = pgtype.Int8{Int64: 1, Valid: true}
		badRecovery.ReleaseKind = "recovery"
		badRecovery.RecoveryOfReleaseID = attempt.ExpectedTargetReleaseID
		if _, err := queries.CreateIssueWorkflowReleaseAttempt(ctx, badRecovery); err == nil {
			t.Fatal("nonadvancing recovery generation was accepted")
		}

		recovery := badRecovery
		recovery.ID = workflowV2UUID(184)
		recovery.TargetGeneration = 2
		if _, err := queries.CreateIssueWorkflowReleaseAttempt(ctx, recovery); err != nil {
			t.Fatalf("new monotonic recovery generation: %v", err)
		}
	})

	t.Run("workspace target serialization blocks a concurrent issue", func(t *testing.T) {
		databaseURL := os.Getenv(workflowV2TestDatabaseEnv)
		firstConn, err := pgx.Connect(ctx, databaseURL)
		if err != nil {
			t.Fatal(err)
		}
		defer firstConn.Close(ctx)
		secondConn, err := pgx.Connect(ctx, databaseURL)
		if err != nil {
			t.Fatal(err)
		}
		defer secondConn.Close(ctx)
		firstTx, err := firstConn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer firstTx.Rollback(ctx)
		secondTx, err := secondConn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer secondTx.Rollback(ctx)
		params := generated.LockIssueWorkflowTargetParams{WorkspaceID: workflowV2UUID(190), TargetKey: "production"}
		if err := generated.New(firstTx).LockIssueWorkflowTarget(ctx, params); err != nil {
			t.Fatal(err)
		}
		entered := make(chan error, 1)
		go func() { entered <- generated.New(secondTx).LockIssueWorkflowTarget(context.Background(), params) }()
		select {
		case err := <-entered:
			t.Fatalf("second target admission did not serialize: %v", err)
		case <-time.After(100 * time.Millisecond):
		}
		if err := firstTx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-entered:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("second target admission remained blocked after first commit")
		}
	})

	t.Run("exact release incidents reject partial bindings", func(t *testing.T) {
		params := generated.CreateIssueWorkflowReleaseIncidentParams{
			ID: workflowV2UUID(220), WorkspaceID: workflowV2UUID(221), IssueID: workflowV2UUID(222),
			ReleaseID: workflowV2UUID(223), ReleaseRevision: pgtype.Int8{Int64: 2, Valid: true},
			CandidateID: workflowV2UUID(224), CandidateRevision: pgtype.Int8{Int64: 3, Valid: true},
			TargetKey: pgtype.Text{String: "production", Valid: true}, TargetGeneration: pgtype.Int8{Int64: 7, Valid: true},
			DetectionEvidenceID: workflowV2UUID(225), CompatibilityState: pgtype.Text{String: "rollback_safe", Valid: true},
			RegressionFingerprint: pgtype.Text{String: "exact-regression-220", Valid: true}, RecoveryReleaseID: workflowV2UUID(227),
			IncidentType: "regression", Severity: "urgent", State: "open", Consequence: []byte(`{}`), Recovery: []byte(`{}`),
		}
		if _, err := queries.CreateIssueWorkflowReleaseIncident(ctx, params); err != nil {
			t.Fatalf("exact incident insert: %v", err)
		}
		duplicate := params
		duplicate.ID = workflowV2UUID(228)
		duplicate.RecoveryReleaseID = workflowV2UUID(229)
		if _, err := queries.CreateIssueWorkflowReleaseIncident(ctx, duplicate); err == nil {
			t.Fatal("duplicate active regression fingerprint was accepted")
		}
		partial := params
		partial.ID = workflowV2UUID(226)
		partial.CandidateRevision = pgtype.Int8{}
		if _, err := queries.CreateIssueWorkflowReleaseIncident(ctx, partial); err == nil {
			t.Fatal("partial exact incident binding was accepted")
		}
	})

	t.Run("projection inputs and rebuild are deterministic", func(t *testing.T) {
		workspaceID, issueID := workflowV2UUID(30), workflowV2UUID(31)
		if _, err := conn.Exec(ctx, `INSERT INTO issue_workflow_outcome (id, workspace_id, issue_id, state, acceptance_version) VALUES ($1, $2, $3, 'open', 1)`, workflowV2UUID(32), workspaceID, issueID); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, `INSERT INTO issue_workflow_attention (id, workspace_id, issue_id, attention_type, read_state, resolution_state) VALUES ($1, $2, $3, 'decision', 'unread', 'open')`, workflowV2UUID(33), workspaceID, issueID); err != nil {
			t.Fatal(err)
		}
		params := generated.ListIssueWorkflowProjectionInputsParams{WorkspaceID: workspaceID, IssueID: issueID}
		first, err := queries.ListIssueWorkflowProjectionInputs(ctx, params)
		if err != nil {
			t.Fatal(err)
		}
		second, err := queries.ListIssueWorkflowProjectionInputs(ctx, params)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("identical authoritative records produced different ordered inputs\nfirst=%#v\nsecond=%#v", first, second)
		}

		picture := workflowV2CurrentPictureParams(workspaceID, issueID, 1, 0)
		created, err := queries.UpsertIssueCurrentPicture(ctx, picture)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := queries.DeleteIssueCurrentPictureForRebuild(ctx, generated.DeleteIssueCurrentPictureForRebuildParams{WorkspaceID: workspaceID, IssueID: issueID}); err != nil {
			t.Fatal(err)
		}
		rebuilt, err := queries.UpsertIssueCurrentPicture(ctx, picture)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(created.Document, rebuilt.Document) || !reflect.DeepEqual(created.SourceRevisions, rebuilt.SourceRevisions) {
			t.Fatalf("rebuilt projection differs: created=%s/%s rebuilt=%s/%s", created.Document, created.SourceRevisions, rebuilt.Document, rebuilt.SourceRevisions)
		}
	})

	t.Run("scoped cleanup removes every record without crossing tenants and rolls back atomically", func(t *testing.T) {
		workspaceA, workspaceB, issueID := workflowV2UUID(40), workflowV2UUID(41), workflowV2UUID(42)
		seedAllWorkflowV2Records(t, ctx, conn, workspaceA, issueID, 50)
		seedAllWorkflowV2Records(t, ctx, conn, workspaceB, issueID, 80)

		if err := queries.DeleteIssueWorkflowV2RecordsByIssue(ctx, generated.DeleteIssueWorkflowV2RecordsByIssueParams{WorkspaceID: workspaceA, IssueID: issueID}); err != nil {
			t.Fatal(err)
		}
		assertWorkflowV2RecordCounts(t, ctx, conn, workspaceA, issueID, 0)
		assertWorkflowV2RecordCounts(t, ctx, conn, workspaceB, issueID, 1)

		squadA, squadB := workflowV2UUID(110), workflowV2UUID(111)
		if _, err := conn.Exec(ctx, `INSERT INTO squad_workflow_template (squad_id, workspace_id, implementer_id, reviewer_id) VALUES ($1,$2,$3,$4),($5,$6,$7,$8)`,
			squadA, workspaceA, workflowV2UUID(112), workflowV2UUID(113), squadB, workspaceB, workflowV2UUID(114), workflowV2UUID(115)); err != nil {
			t.Fatal(err)
		}

		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		qtx := generated.New(tx)
		if err := qtx.DeleteIssueWorkflowV2RecordsByWorkspace(ctx, workspaceB); err != nil {
			t.Fatal(err)
		}
		if _, err := qtx.DeleteSquadWorkflowTemplateScoped(ctx, generated.DeleteSquadWorkflowTemplateScopedParams{WorkspaceID: workspaceB, SquadID: squadB}); err != nil {
			t.Fatal(err)
		}
		if err := tx.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
		assertWorkflowV2RecordCounts(t, ctx, conn, workspaceB, issueID, 1)

		tx, err = conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		qtx = generated.New(tx)
		if err := qtx.DeleteIssueWorkflowV2RecordsByWorkspace(ctx, workspaceB); err != nil {
			t.Fatal(err)
		}
		if _, err := qtx.DeleteSquadWorkflowTemplateScoped(ctx, generated.DeleteSquadWorkflowTemplateScopedParams{WorkspaceID: workspaceB, SquadID: squadB}); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		assertWorkflowV2RecordCounts(t, ctx, conn, workspaceB, issueID, 0)
	})

	t.Run("populated down migration refuses destructive rollback", func(t *testing.T) {
		newID := func() pgtype.UUID { return pgtype.UUID{Bytes: uuid.New(), Valid: true} }
		workspaceID, issueID, runtimeID, agentID, actionID, taskID := newID(), newID(), newID(), newID(), newID(), newID()
		for _, statement := range []struct {
			sql  string
			args []any
		}{
			{`INSERT INTO workspace(id) VALUES($1) ON CONFLICT DO NOTHING`, []any{workspaceID}},
			{`INSERT INTO issue(id,workspace_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, []any{issueID, workspaceID}},
			{`INSERT INTO agent_runtime(id,workspace_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, []any{runtimeID, workspaceID}},
			{`INSERT INTO agent(id,workspace_id,runtime_id) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, []any{agentID, workspaceID, runtimeID}},
			{`INSERT INTO issue_workflow_action_receipt(id,workspace_id,issue_id,action_key,request_hash,command_id,action_type,payload,lease_count) VALUES($1,$2,$3,'down-guard','down-guard-hash',$4,'review_correction','{}',1)`, []any{actionID, workspaceID, issueID, newID()}},
			{`INSERT INTO agent_task_queue(id,agent_id,runtime_id,issue_id,status,context,workflow_action_id,workflow_operation_id,workflow_branch_key) VALUES($1,$2,$3,$4,'queued','{}',$5,$6,'main')`, []any{taskID, agentID, runtimeID, issueID, actionID, newID()}},
		} {
			if _, err := conn.Exec(ctx, statement.sql, statement.args...); err != nil {
				t.Fatal(err)
			}
		}
		t.Cleanup(func() {
			_, _ = conn.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE id=$1`, taskID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM issue_workflow_action_receipt WHERE id=$1`, actionID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM agent WHERE id=$1`, agentID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM agent_runtime WHERE id=$1`, runtimeID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM issue WHERE id=$1`, issueID)
			_, _ = conn.Exec(context.Background(), `DELETE FROM workspace WHERE id=$1`, workspaceID)
		})
		if _, err := conn.Exec(ctx, readMigrationFile(t, "503_issue_workflow_action_execution_metadata.down.sql")); err == nil || !strings.Contains(err.Error(), "workflow action execution metadata contains durable records") {
			t.Fatalf("503 down migration error=%v", err)
		}
		if _, err := conn.Exec(ctx, readMigrationFile(t, "504_agent_task_workflow_action_binding.down.sql")); err == nil || !strings.Contains(err.Error(), "agent tasks contain durable workflow action bindings") {
			t.Fatalf("504 down migration error=%v", err)
		}
		for _, name := range []string{"uq_agent_task_workflow_action", "uq_issue_workflow_action_effect_key", "idx_issue_workflow_action_reconciliation_due"} {
			var exists bool
			if err := conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE schemaname=current_schema() AND indexname=$1)`, name).Scan(&exists); err != nil || !exists {
				t.Fatalf("required workflow action index %s exists=%v err=%v", name, exists, err)
			}
		}
		if _, err := conn.Exec(ctx, readMigrationFile(t, "466_issue_workflow_v2_records.down.sql")); err == nil || !strings.Contains(err.Error(), "workflow v2 contains durable records") {
			t.Fatalf("populated rollback error = %v, want preservation refusal", err)
		}
		var tableExists bool
		if err := conn.QueryRow(ctx, `SELECT to_regclass('issue_workflow_outcome') IS NOT NULL`).Scan(&tableExists); err != nil {
			t.Fatal(err)
		}
		if !tableExists {
			t.Fatal("rollback refusal did not preserve workflow v2 tables")
		}
	})

	t.Run("admission lock cannot insert behind issue deletion", func(t *testing.T) {
		workspaceID, issueID := workflowV2UUID(120), workflowV2UUID(121)
		if _, err := conn.Exec(ctx, `INSERT INTO workspace (id) VALUES ($1)`, workspaceID); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, `INSERT INTO issue (id, workspace_id) VALUES ($1, $2)`, issueID, workspaceID); err != nil {
			t.Fatal(err)
		}
		deleteTx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		deleteQueries := generated.New(deleteTx)
		if _, err := deleteQueries.LockIssueForDelete(ctx, generated.LockIssueForDeleteParams{ID: issueID, WorkspaceID: workspaceID}); err != nil {
			_ = deleteTx.Rollback(ctx)
			t.Fatal(err)
		}
		if err := deleteQueries.DeleteIssue(ctx, generated.DeleteIssueParams{ID: issueID, WorkspaceID: workspaceID}); err != nil {
			_ = deleteTx.Rollback(ctx)
			t.Fatal(err)
		}

		writerConn, err := pgx.Connect(ctx, os.Getenv(workflowV2TestDatabaseEnv))
		if err != nil {
			_ = deleteTx.Rollback(ctx)
			t.Fatal(err)
		}
		defer writerConn.Close(context.Background())
		if _, err := writerConn.Exec(ctx, `SELECT set_config('search_path', $1, false)`, "workflow_v2_contract_test"); err != nil {
			_ = deleteTx.Rollback(ctx)
			t.Fatal(err)
		}
		lockResult := make(chan error, 1)
		go func() {
			lockResult <- workflowstore.InIssueWriteTransaction(context.Background(), writerConn, generated.New(writerConn), workspaceID, issueID, func(write *workflowstore.IssueWrite) error {
				_, err := write.CreateOutcome(context.Background(), generated.CreateIssueWorkflowOutcomeParams{
					ID: workflowV2UUID(122), WorkspaceID: workspaceID, IssueID: issueID,
					State: "open", AcceptanceVersion: 1, TerminalReason: "",
				})
				return err
			})
		}()
		select {
		case lockErr := <-lockResult:
			_ = deleteTx.Rollback(ctx)
			t.Fatalf("admission lock returned before deleting transaction settled: %v", lockErr)
		case <-time.After(100 * time.Millisecond):
		}
		if err := deleteTx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		select {
		case lockErr := <-lockResult:
			if !errors.Is(lockErr, pgx.ErrNoRows) {
				t.Fatalf("admission after delete error = %v, want pgx.ErrNoRows", lockErr)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("admission lock did not wake after delete committed")
		}
	})

	t.Run("correction task owner fence loses issue deletion race", func(t *testing.T) {
		newID := func() pgtype.UUID { return pgtype.UUID{Bytes: uuid.New(), Valid: true} }
		workspaceID, issueID, runtimeID, agentID := newID(), newID(), newID(), newID()
		for _, statement := range []struct {
			sql  string
			args []any
		}{
			{`INSERT INTO workspace(id) VALUES($1)`, []any{workspaceID}},
			{`INSERT INTO issue(id,workspace_id) VALUES($1,$2)`, []any{issueID, workspaceID}},
			{`INSERT INTO agent_runtime(id,workspace_id) VALUES($1,$2)`, []any{runtimeID, workspaceID}},
			{`INSERT INTO agent(id,workspace_id,runtime_id) VALUES($1,$2,$3)`, []any{agentID, workspaceID, runtimeID}},
		} {
			if _, err := conn.Exec(ctx, statement.sql, statement.args...); err != nil {
				t.Fatal(err)
			}
		}
		deleteTx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := deleteTx.Exec(ctx, `SELECT id FROM issue WHERE id=$1 FOR UPDATE`, issueID); err != nil {
			t.Fatal(err)
		}
		writerConn, err := workflowV2TestConnection(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer writerConn.Close(context.Background())
		result := make(chan error, 1)
		go func() {
			result <- lockWorkflowCorrectionTaskOwnersInOrder(context.Background(), writerConn, workspaceID, agentID, issueID, runtimeID)
		}()
		select {
		case got := <-result:
			_ = deleteTx.Rollback(ctx)
			t.Fatalf("task fence returned before issue deletion settled: %v", got)
		case <-time.After(100 * time.Millisecond):
		}
		if _, err := deleteTx.Exec(ctx, `DELETE FROM issue WHERE id=$1`, issueID); err != nil {
			t.Fatal(err)
		}
		if err := deleteTx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		got := <-result
		if !errors.Is(got, pgx.ErrNoRows) {
			t.Fatalf("task fence after committed issue deletion=%v", got)
		}
		var tasks int
		if err := writerConn.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, issueID).Scan(&tasks); err != nil || tasks != 0 {
			t.Fatalf("task residue after issue deletion=%d err=%v", tasks, err)
		}
		_, _ = conn.Exec(context.Background(), `DELETE FROM agent WHERE id=$1`, agentID)
		_, _ = conn.Exec(context.Background(), `DELETE FROM agent_runtime WHERE id=$1`, runtimeID)
		_, _ = conn.Exec(context.Background(), `DELETE FROM workspace WHERE id=$1`, workspaceID)
	})

	t.Run("correction task owner fence loses workspace deletion race", func(t *testing.T) {
		newID := func() pgtype.UUID { return pgtype.UUID{Bytes: uuid.New(), Valid: true} }
		workspaceID, issueID, runtimeID, agentID := newID(), newID(), newID(), newID()
		for _, statement := range []struct {
			sql  string
			args []any
		}{
			{`INSERT INTO workspace(id) VALUES($1)`, []any{workspaceID}},
			{`INSERT INTO issue(id,workspace_id) VALUES($1,$2)`, []any{issueID, workspaceID}},
			{`INSERT INTO agent_runtime(id,workspace_id) VALUES($1,$2)`, []any{runtimeID, workspaceID}},
			{`INSERT INTO agent(id,workspace_id,runtime_id) VALUES($1,$2,$3)`, []any{agentID, workspaceID, runtimeID}},
		} {
			if _, err := conn.Exec(ctx, statement.sql, statement.args...); err != nil {
				t.Fatal(err)
			}
		}
		deleteTx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := deleteTx.Exec(ctx, `SELECT id FROM workspace WHERE id=$1 FOR UPDATE`, workspaceID); err != nil {
			t.Fatal(err)
		}
		writerConn, err := workflowV2TestConnection(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer writerConn.Close(context.Background())
		result := make(chan error, 1)
		go func() {
			result <- lockWorkflowCorrectionTaskOwnersInOrder(context.Background(), writerConn, workspaceID, agentID, issueID, runtimeID)
		}()
		select {
		case got := <-result:
			_ = deleteTx.Rollback(ctx)
			t.Fatalf("task fence returned before workspace deletion settled: %v", got)
		case <-time.After(100 * time.Millisecond):
		}
		if _, err := deleteTx.Exec(ctx, `DELETE FROM workspace WHERE id=$1`, workspaceID); err != nil {
			t.Fatal(err)
		}
		if err := deleteTx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		got := <-result
		if !errors.Is(got, pgx.ErrNoRows) {
			t.Fatalf("task fence after committed workspace deletion=%v", got)
		}
		var tasks int
		if err := writerConn.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 OR agent_id=$2 OR runtime_id=$3`, issueID, agentID, runtimeID).Scan(&tasks); err != nil || tasks != 0 {
			t.Fatalf("task residue after workspace deletion=%d err=%v", tasks, err)
		}
	})

	t.Run("template writer loses reverse races to workspace squad and agent deletion", func(t *testing.T) {
		for _, owner := range []string{"workspace", "squad", "agent"} {
			t.Run(owner, func(t *testing.T) {
				base := byte(130)
				if owner == "squad" {
					base = 140
				}
				if owner == "agent" {
					base = 150
				}
				workspaceID, squadID := workflowV2UUID(base), workflowV2UUID(base+1)
				implementerID, reviewerID := workflowV2UUID(base+2), workflowV2UUID(base+3)
				for _, seed := range []struct {
					sql  string
					args []any
				}{
					{`INSERT INTO workspace(id) VALUES ($1)`, []any{workspaceID}},
					{`INSERT INTO squad(id, workspace_id) VALUES ($1, $2)`, []any{squadID, workspaceID}},
					{`INSERT INTO agent(id, workspace_id) VALUES ($1, $2), ($3, $2)`, []any{implementerID, workspaceID, reviewerID}},
					{`INSERT INTO squad_member(squad_id, member_type, member_id) VALUES ($1, 'agent', $2), ($1, 'agent', $3)`, []any{squadID, implementerID, reviewerID}},
				} {
					if _, err := conn.Exec(ctx, seed.sql, seed.args...); err != nil {
						t.Fatal(err)
					}
				}
				deleteTx, err := conn.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				deleteQueries := generated.New(deleteTx)
				switch owner {
				case "workspace":
					if _, err := deleteTx.Exec(ctx, `SELECT id FROM workspace WHERE id=$1 FOR UPDATE`, workspaceID); err != nil {
						t.Fatal(err)
					}
					if _, err := deleteTx.Exec(ctx, `DELETE FROM workspace WHERE id=$1`, workspaceID); err != nil {
						t.Fatal(err)
					}
				case "squad":
					if _, err := deleteQueries.LockWorkflowOwnerWorkspace(ctx, workspaceID); err != nil {
						t.Fatal(err)
					}
					if _, err := deleteTx.Exec(ctx, `SELECT id FROM squad WHERE id=$1 FOR UPDATE`, squadID); err != nil {
						t.Fatal(err)
					}
					if _, err := deleteTx.Exec(ctx, `UPDATE squad SET archived_at=now() WHERE id=$1`, squadID); err != nil {
						t.Fatal(err)
					}
				case "agent":
					if _, err := deleteQueries.LockWorkflowOwnerWorkspace(ctx, workspaceID); err != nil {
						t.Fatal(err)
					}
					if _, err := deleteTx.Exec(ctx, `SELECT id FROM agent WHERE id=$1 FOR UPDATE`, implementerID); err != nil {
						t.Fatal(err)
					}
					if _, err := deleteTx.Exec(ctx, `DELETE FROM agent WHERE id=$1`, implementerID); err != nil {
						t.Fatal(err)
					}
				}

				writerConn, err := workflowV2TestConnection(ctx)
				if err != nil {
					_ = deleteTx.Rollback(ctx)
					t.Fatal(err)
				}
				defer writerConn.Close(context.Background())
				result := make(chan error, 1)
				go func() {
					result <- writeWorkflowTemplateWithLocks(context.Background(), writerConn, workspaceID, squadID, implementerID, reviewerID)
				}()
				select {
				case err := <-result:
					_ = deleteTx.Rollback(ctx)
					t.Fatalf("writer returned before %s delete committed: %v", owner, err)
				case <-time.After(100 * time.Millisecond):
				}
				if err := deleteTx.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				select {
				case err := <-result:
					if !errors.Is(err, pgx.ErrNoRows) {
						t.Fatalf("writer after %s delete = %v, want pgx.ErrNoRows", owner, err)
					}
				case <-time.After(2 * time.Second):
					t.Fatalf("writer did not wake after %s delete", owner)
				}
			})
		}
	})
}

func stableWorkflowCommandUUID(workspaceID, issueID pgtype.UUID, key, suffix string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(util.UUIDToString(workspaceID)+"/"+util.UUIDToString(issueID)+"/"+key+"/"+suffix)).String()
}

type nonLiveFinalizerFixture struct {
	WorkspaceID, IssueID, ActionID, ControllerID pgtype.UUID
	PrincipalActionID, PrincipalLease            pgtype.UUID
	PrincipalActionKey                           string
	Request                                      workflowcommand.Request
	Service                                      workflowcommand.Service
	Now                                          time.Time
}

func seedNonLiveFinalizerFixture(t *testing.T, ctx context.Context, conn *pgx.Conn, queries *generated.Queries, base byte) nonLiveFinalizerFixture {
	t.Helper()
	workspaceID, issueID := seedLoadableWorkflowV2(t, ctx, conn, base)
	t.Cleanup(func() {
		_ = queries.DeleteIssueWorkflowV2RecordsByIssue(context.Background(), generated.DeleteIssueWorkflowV2RecordsByIssueParams{WorkspaceID: workspaceID, IssueID: issueID})
		_, _ = conn.Exec(context.Background(), `DELETE FROM issue WHERE id=$1`, issueID)
		_, _ = conn.Exec(context.Background(), `DELETE FROM workspace WHERE id=$1`, workspaceID)
	})
	ids := map[string]pgtype.UUID{
		"outcome": workflowV2UUID(base + 2), "scope": workflowV2UUID(base + 3), "execution": workflowV2UUID(base + 5),
		"review": workflowV2UUID(base + 6), "release": workflowV2UUID(base + 7), "candidate": workflowV2UUID(base + 11),
		"reviewer": workflowV2UUID(base + 14), "review_evidence": workflowV2UUID(base + 17),
	}
	now := time.Date(2026, 9, 10, 21, 0, 0, 0, time.UTC)
	for _, statement := range []struct {
		sql  string
		args []any
	}{
		{`UPDATE issue_workflow_scope SET requires_deployment=false,source_plan_revision=1 WHERE id=$1`, []any{ids["scope"]}},
		{`UPDATE issue_workflow_authority SET envelope='{}' WHERE workspace_id=$1 AND issue_id=$2`, []any{workspaceID, issueID}},
		{`UPDATE issue_workflow_execution SET state='completed' WHERE id=$1`, []any{ids["execution"]}},
		{`UPDATE issue_workflow_release SET state='pending',target_key='',target_generation=0,started_at=NULL,deployed_at=NULL,facets='{}' WHERE id=$1`, []any{ids["release"]}},
		{`UPDATE issue_workflow_evidence SET concern_id=$2::uuid,concern_revision=1,payload=jsonb_build_object('verdict','passed','review_id',($2::uuid)::text,'review_revision',1,'criteria_refs',jsonb_build_array('acceptance'),'coverage_refs',jsonb_build_array('tests'),'candidate_revision',1,'scope_id',($3::uuid)::text,'scope_revision',1,'freshness','current') WHERE id=$1`, []any{ids["review_evidence"], ids["review"], ids["scope"]}},
		{`DELETE FROM issue_workflow_evidence WHERE id=$1`, []any{workflowV2UUID(base + 9)}},
		{`UPDATE issue_workflow_concern_head SET source_record_id=$3,source_record_revision=1,state='current' WHERE workspace_id=$1 AND issue_id=$2 AND concern_type='evidence'`, []any{workspaceID, issueID, ids["review_evidence"]}},
		{`UPDATE issue_workflow_concern_head SET state=CASE concern_type WHEN 'execution' THEN 'completed' WHEN 'review' THEN 'passed' WHEN 'release' THEN 'pending' ELSE state END WHERE workspace_id=$1 AND issue_id=$2`, []any{workspaceID, issueID}},
		{`DELETE FROM issue_workflow_target_head WHERE workspace_id=$1 AND issue_id=$2`, []any{workspaceID, issueID}},
	} {
		if _, err := conn.Exec(ctx, statement.sql, statement.args...); err != nil {
			t.Fatalf("prepare non-live finalizer fixture: %v", err)
		}
	}
	expectedMap := map[string]workflowstatev2.Revision{"scope": 1, "review": 1, "evidence": 1, "outcome": 1, "execution": 1, "release": 1}
	actionPayload, _ := json.Marshal(map[string]any{
		"operation_id": util.UUIDToString(ids["execution"]), "candidate_id": util.UUIDToString(ids["candidate"]),
		"scope_revision": 1, "acceptance_version": 1, "source_plan_revision": 1, "requires_deployment": false, "expected": expectedMap,
	})
	actionID, sourceCommandID, controllerID := workflowV2UUID(base+18), workflowV2UUID(base+19), workflowV2UUID(base+20)
	actionKey := "workflow/" + util.UUIDToString(sourceCommandID) + "/finalize_non_live"
	if _, err := queries.CreateIssueWorkflowActionReceipt(ctx, generated.CreateIssueWorkflowActionReceiptParams{ID: actionID, WorkspaceID: workspaceID, IssueID: issueID, ActionKey: actionKey, RequestHash: "finalizer-source", CommandID: sourceCommandID, ActionType: "finalize_non_live", OperationID: ids["execution"], Payload: actionPayload, NotBefore: workflowV2Time(now), MaxAttempts: 3}); err != nil {
		t.Fatal(err)
	}
	expected := make([]workflowstatev2.ExpectedRevision, 0, 6)
	for _, concern := range []workflowstatev2.Concern{workflowstatev2.ConcernScope, workflowstatev2.ConcernReview, workflowstatev2.ConcernEvidence, workflowstatev2.ConcernOutcome, workflowstatev2.ConcernExecution, workflowstatev2.ConcernRelease} {
		expected = append(expected, workflowstatev2.ExpectedRevision{Concern: concern, Revision: 1})
	}
	fx := nonLiveFinalizerFixture{
		WorkspaceID: workspaceID, IssueID: issueID, ActionID: actionID, ControllerID: controllerID, Now: now,
		Request: workflowcommand.Request{WorkspaceID: workspaceID, IssueID: issueID, CommandKey: "action/" + util.UUIDToString(actionID) + "/complete-non-live", Kind: workflowstatev2.CommandCompleteNonLive, OperationID: util.UUIDToString(ids["execution"]), Expected: expected},
	}
	fx.Service = workflowcommand.Service{Starter: conn, Queries: queries, Loader: workflowcommand.Loader{Now: func() time.Time { return fx.Now }}, Now: func() time.Time { return fx.Now }, ControllerID: controllerID}
	return fx
}

func claimAndBeginNonLiveFinalizer(t *testing.T, ctx context.Context, queries *generated.Queries, fx nonLiveFinalizerFixture, now time.Time) generated.IssueWorkflowActionReceipt {
	t.Helper()
	claimed, err := queries.ClaimIssueWorkflowActionExact(ctx, generated.ClaimIssueWorkflowActionExactParams{LeaseToken: workflowV2UUID(95), LeaseExpiresAt: workflowV2Time(now.Add(time.Hour)), WorkspaceID: fx.WorkspaceID, IssueID: fx.IssueID, ID: fx.ActionID, ExpectedRevision: 1, Now: workflowV2Time(now)})
	if err != nil {
		t.Fatal(err)
	}
	begun, err := queries.BeginIssueWorkflowActionAttempt(ctx, generated.BeginIssueWorkflowActionAttemptParams{Now: workflowV2Time(now), WorkspaceID: fx.WorkspaceID, IssueID: fx.IssueID, ID: fx.ActionID, ExpectedRevision: claimed.Revision, LeaseToken: claimed.LeaseToken})
	if err != nil {
		t.Fatal(err)
	}
	return begun
}

func finalizerPrincipal(fx nonLiveFinalizerFixture, begun generated.IssueWorkflowActionReceipt) workflowcommand.Principal {
	actionID, actionKey, lease := begun.ID, begun.ActionKey, begun.LeaseToken
	if fx.PrincipalActionID.Valid {
		actionID = fx.PrincipalActionID
	}
	if fx.PrincipalActionKey != "" {
		actionKey = fx.PrincipalActionKey
	}
	if fx.PrincipalLease.Valid {
		lease = fx.PrincipalLease
	}
	return workflowcommand.Principal{ActorID: fx.ControllerID, ActorType: "internal_controller", ActionID: actionID, ActionKey: actionKey, LeaseToken: lease}
}

func mutateFinalizerPayload(t *testing.T, ctx context.Context, conn *pgx.Conn, actionID pgtype.UUID, field string, value any) {
	t.Helper()
	encoded, _ := json.Marshal(value)
	if _, err := conn.Exec(ctx, `UPDATE issue_workflow_action_receipt SET payload=jsonb_set(payload,ARRAY[$2],$3::jsonb) WHERE id=$1`, actionID, field, string(encoded)); err != nil {
		t.Fatal(err)
	}
}

func mutateFinalizerExpected(t *testing.T, ctx context.Context, conn *pgx.Conn, actionID pgtype.UUID, concern string, revision int) {
	t.Helper()
	encoded, _ := json.Marshal(revision)
	if _, err := conn.Exec(ctx, `UPDATE issue_workflow_action_receipt SET payload=jsonb_set(payload,ARRAY['expected',$2],$3::jsonb) WHERE id=$1`, actionID, concern, string(encoded)); err != nil {
		t.Fatal(err)
	}
}

func assertNonLiveFinalizerState(t *testing.T, ctx context.Context, conn *pgx.Conn, fx nonLiveFinalizerFixture, actionState string, commandCount int) {
	t.Helper()
	var gotAction, outcome, execution, release string
	var outcomeRevision, executionRevision, releaseRevision, commands, pictures int
	if err := conn.QueryRow(ctx, `SELECT state FROM issue_workflow_action_receipt WHERE id=$1`, fx.ActionID).Scan(&gotAction); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(ctx, `SELECT state,revision FROM issue_workflow_outcome WHERE workspace_id=$1 AND issue_id=$2`, fx.WorkspaceID, fx.IssueID).Scan(&outcome, &outcomeRevision); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(ctx, `SELECT state,revision FROM issue_workflow_execution WHERE workspace_id=$1 AND issue_id=$2`, fx.WorkspaceID, fx.IssueID).Scan(&execution, &executionRevision); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(ctx, `SELECT state,revision FROM issue_workflow_release WHERE workspace_id=$1 AND issue_id=$2`, fx.WorkspaceID, fx.IssueID).Scan(&release, &releaseRevision); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_command WHERE workspace_id=$1 AND issue_id=$2`, fx.WorkspaceID, fx.IssueID).Scan(&commands); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM issue_current_picture WHERE workspace_id=$1 AND issue_id=$2`, fx.WorkspaceID, fx.IssueID).Scan(&pictures); err != nil {
		t.Fatal(err)
	}
	if gotAction != actionState || commands != commandCount {
		t.Fatalf("action=%s commands=%d, want %s/%d", gotAction, commands, actionState, commandCount)
	}
	if commandCount == 0 {
		if outcome != "open" || execution != "completed" || release != "pending" || outcomeRevision != 1 || executionRevision != 1 || releaseRevision != 1 || pictures != 0 {
			t.Fatalf("rollback residue outcome=%s/%d execution=%s/%d release=%s/%d pictures=%d", outcome, outcomeRevision, execution, executionRevision, release, releaseRevision, pictures)
		}
		return
	}
	if outcome != "satisfied" || execution != "completed" || release != "not_required" || outcomeRevision != 2 || executionRevision != 2 || releaseRevision != 2 || pictures != 1 {
		t.Fatalf("completed state outcome=%s/%d execution=%s/%d release=%s/%d pictures=%d", outcome, outcomeRevision, execution, executionRevision, release, releaseRevision, pictures)
	}
	for _, concern := range []string{"outcome", "execution", "release"} {
		var revision int
		if err := conn.QueryRow(ctx, `SELECT revision FROM issue_workflow_concern_head WHERE workspace_id=$1 AND issue_id=$2 AND concern_type=$3`, fx.WorkspaceID, fx.IssueID, concern).Scan(&revision); err != nil || revision != 2 {
			t.Fatalf("%s head revision=%d err=%v", concern, revision, err)
		}
	}
}

func workflowV2UUIDString(value string) pgtype.UUID {
	id, err := uuid.Parse(value)
	if err != nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: id, Valid: true}
}

func TestWorkflowV2ReleaseAdmissionDatabase(t *testing.T) {
	ctx, conn, queries := setupWorkflowV2Database(t)
	const base byte = 5
	workspaceID, issueID := seedLoadableWorkflowV2(t, ctx, conn, base)
	scopeID, authorityID, executionID := workflowV2UUID(base+3), workflowV2UUID(base+4), workflowV2UUID(base+5)
	reviewID, releaseID, candidateID := workflowV2UUID(base+6), workflowV2UUID(base+7), workflowV2UUID(base+11)
	knownGoodReleaseID, knownGoodCandidateID, knownGoodReviewID, knownGoodReviewEvidenceID := workflowV2UUID(base+18), workflowV2UUID(base+19), workflowV2UUID(base+20), workflowV2UUID(base+21)
	knownGoodIssueID := workflowV2UUID(240)
	ownerID, operatorID, verifierID := workflowV2UUID(base+12), workflowV2UUID(base+15), workflowV2UUID(base+16)
	operatorRuntime, verifierRuntime := workflowV2UUID(230), workflowV2UUID(231)
	controllerID := workflowV2UUID(232)
	nowValue := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	envelope := map[string]any{
		"version": 1, "capability": "deploy_exact_release", "candidate_id": util.UUIDToString(candidateID), "candidate_revision": 1,
		"scope_id": util.UUIDToString(scopeID), "scope_revision": 1, "acceptance_version": 1,
		"operation_id": util.UUIDToString(executionID), "branch_key": "main", "target_key": "production", "target_id": "production",
		"expected_target_release_id": util.UUIDToString(knownGoodReleaseID), "expected_target_generation": 1, "desired_target_generation": 2,
		"exposure_policy": map[string]any{"mode": "canary", "percent": 10}, "audience_policy": map[string]any{"segment": "internal"},
		"risk_envelope": map[string]any{"level": "bounded"}, "exclusions": []string{}, "budget_envelope": map[string]any{"attempts": 1},
		"recovery_policy": map[string]any{"mode": "rollback"}, "recovery_policy_revision": 1, "rollback_target_release_id": util.UUIDToString(knownGoodReleaseID), "irreversible_effects": []string{},
		"owner_evidence_id": util.UUIDToString(ownerID), "owner_evidence_revision": 1,
		"release_operator_id": util.UUIDToString(operatorID), "planned_verifier_id": util.UUIDToString(verifierID),
		"build_manifest_digest": "sha256:manifest", "artifact_digest": "sha256:artifact", "runtime_contract_digest": "sha256:runtime",
		"reviewed_review_id": util.UUIDToString(reviewID), "reviewed_review_revision": 1,
	}
	envelopeJSON, _ := json.Marshal(envelope)
	envelopeSum := sha256.Sum256(envelopeJSON)
	ownerEvidencePayload, _ := json.Marshal(map[string]any{
		"revision": 1, "freshness": "current", "authority_id": util.UUIDToString(authorityID), "authority_revision": 1,
		"envelope_digest": fmt.Sprintf("%x", envelopeSum[:]), "capability": "deploy_exact_release",
		"candidate_id": util.UUIDToString(candidateID), "candidate_revision": 1,
		"scope_id": util.UUIDToString(scopeID), "scope_revision": 1, "acceptance_version": 1,
		"operation_id": util.UUIDToString(executionID), "branch_key": "main", "target_key": "production", "target_generation": 2,
	})
	for _, statement := range []struct {
		sql  string
		args []any
	}{
		{`DELETE FROM issue_workflow_target_head WHERE workspace_id=$1 AND target_key='production'`, []any{workspaceID}},
		{`INSERT INTO issue(id,workspace_id) VALUES($1,$2)`, []any{knownGoodIssueID, workspaceID}},
		{`UPDATE issue_workflow_scope SET requires_deployment=true,source_plan_revision=1 WHERE id=$1`, []any{scopeID}},
		{`UPDATE issue_workflow_authority SET envelope=$2 WHERE id=$1`, []any{authorityID, envelopeJSON}},
		{`UPDATE issue_workflow_execution SET candidate_id=$2 WHERE id=$1`, []any{executionID, candidateID}},
		{`UPDATE issue_workflow_release SET target_key='',target_generation=0,state='pending',facets='{}',head_observed_at=NULL WHERE id=$1`, []any{releaseID}},
		{`UPDATE issue_workflow_concern_head SET state='pending' WHERE workspace_id=$1 AND issue_id=$2 AND concern_type='release'`, []any{workspaceID, issueID}},
		{`UPDATE issue_workflow_concern_head SET source_record_id=$3,state='current' WHERE workspace_id=$1 AND issue_id=$2 AND concern_type='evidence'`, []any{workspaceID, issueID, workflowV2UUID(base + 17)}},
		{`DELETE FROM issue_workflow_evidence WHERE id=$1`, []any{workflowV2UUID(base + 9)}},
		{`UPDATE issue_workflow_evidence SET concern_id=$2::uuid,concern_revision=1,payload=jsonb_build_object('freshness','current','candidate_revision',1,'scope_id',$3::uuid::text,'scope_revision',1,'verdict','passed','review_id',$2::uuid::text,'review_revision',1,'criteria_refs',jsonb_build_array('acceptance'),'coverage_refs',jsonb_build_array('tests')) WHERE id=$1`, []any{workflowV2UUID(base + 17), reviewID, scopeID}},
		{`INSERT INTO issue_workflow_candidate(id,workspace_id,issue_id,scope_id,scope_revision,candidate_type,state,source_revision,artifact,build_contract,created_by) SELECT $1,workspace_id,$3,scope_id,scope_revision,candidate_type,'ready','known-good',artifact,build_contract,created_by FROM issue_workflow_candidate WHERE id=$2`, []any{knownGoodCandidateID, candidateID, knownGoodIssueID}},
		{`INSERT INTO issue_workflow_review(id,workspace_id,issue_id,candidate_id,candidate_revision,scope_id,scope_revision,reviewer_id,outcome,criteria,coverage,evidence_refs,gaps,correction_packet) VALUES($1,$2,$3,$4,1,$5,1,$6,'passed','["acceptance"]','{"regression":true}',jsonb_build_array($7::uuid::text),'[]','{}')`, []any{knownGoodReviewID, workspaceID, knownGoodIssueID, knownGoodCandidateID, scopeID, verifierID, knownGoodReviewEvidenceID}},
		{`INSERT INTO issue_workflow_evidence(id,workspace_id,issue_id,concern_type,concern_id,concern_revision,evidence_type,level,source,producer_type,producer_id,candidate_id,observed_at,coverage,payload) VALUES($1,$2,$3,'review',$4,1,'technical_review','review','known-good-review','reviewer',$5,$6,$7,'{"regression":true}',jsonb_build_object('freshness','current','candidate_revision',1,'scope_id',$8::uuid::text,'scope_revision',1,'acceptance_version',1,'verdict','passed','review_id',$4::uuid::text,'review_revision',1,'criteria_refs',jsonb_build_array('acceptance'),'coverage_refs',jsonb_build_array('regression')))`, []any{knownGoodReviewEvidenceID, workspaceID, knownGoodIssueID, knownGoodReviewID, verifierID, knownGoodCandidateID, nowValue, scopeID}},
		{`INSERT INTO issue_workflow_release(id,workspace_id,issue_id,candidate_id,candidate_revision,scope_id,scope_revision,authority_id,authority_revision,target_key,target_generation,state,revision,facets,recovery_policy,exposed_population,attempt_contract_version,acceptance_version,operation_id,branch_key,expected_target_generation,release_kind,head_observed_at,release_operator_id,planned_verifier_id,build_manifest_digest,artifact_digest,runtime_contract_digest,authority_capability,authority_evidence_id,recovery_policy_revision,reviewed_review_id,reviewed_review_revision) VALUES($1,$2,$3,$4,1,$5,1,$6,1,'production',1,'live',1,jsonb_build_object('release_operator_id',$7::uuid::text,'planned_verifier_id',$8::uuid::text),'{}','{}',1,1,$9,'main',0,'forward',$10,$7,$8,'sha256:known-good-manifest','sha256:known-good-artifact','sha256:known-good-runtime','deploy_exact_release',$11,1,$12,1)`, []any{knownGoodReleaseID, workspaceID, knownGoodIssueID, knownGoodCandidateID, scopeID, authorityID, operatorID, verifierID, executionID, nowValue, ownerID, knownGoodReviewID}},
		{`INSERT INTO issue_workflow_target_head(workspace_id,target_key,generation,release_id,issue_id,candidate_id,state,revision) VALUES($1,'production',1,$2,$3,$4,'live',1)`, []any{workspaceID, knownGoodReleaseID, knownGoodIssueID, knownGoodCandidateID}},
		{`INSERT INTO agent_runtime(id,workspace_id,daemon_id) VALUES($1,$3,'release-operator-daemon'),($2,$3,'release-verifier-daemon')`, []any{operatorRuntime, verifierRuntime, workspaceID}},
		{`INSERT INTO agent(id,workspace_id,runtime_id) VALUES($1,$3,$4),($2,$3,$5)`, []any{operatorID, verifierID, workspaceID, operatorRuntime, verifierRuntime}},
		{`INSERT INTO member(id,workspace_id,user_id,role) VALUES($1,$2,$3,'owner')`, []any{workflowV2UUID(234), workspaceID, ownerID}},
		{`INSERT INTO issue_workflow_evidence(id,workspace_id,issue_id,concern_type,concern_id,concern_revision,evidence_type,level,source,producer_type,producer_id,candidate_id,target_key,observed_at,payload) VALUES($1,$2,$3,'authority',$4,1,'deployment_authorization','owner','owner-grant','owner',$1,$5,'production',$6,$7)`, []any{ownerID, workspaceID, issueID, authorityID, candidateID, nowValue, ownerEvidencePayload}},
	} {
		if _, err := conn.Exec(ctx, statement.sql, statement.args...); err != nil {
			t.Fatal(err)
		}
	}
	clock := nowValue
	now := func() time.Time { return clock }
	service := workflowcommand.Service{Starter: conn, Queries: queries, Loader: workflowcommand.Loader{Now: now}, Now: now, ControllerID: controllerID}
	admission := &workflowcommand.ReleaseAdmissionInput{TargetID: "production", ExpectedTargetReleaseID: util.UUIDToString(knownGoodReleaseID), ExpectedTargetGeneration: 1, DesiredTargetGeneration: 2, ReleaseOperatorID: operatorID, PlannedVerifierID: verifierID, BuildManifestDigest: "sha256:manifest", ArtifactDigest: "sha256:artifact", RuntimeContractDigest: "sha256:runtime", ExposurePolicy: map[string]any{"mode": "canary", "percent": float64(10)}, AudiencePolicy: map[string]any{"segment": "internal"}, RiskEnvelope: map[string]any{"level": "bounded"}, Exclusions: []string{}, BudgetEnvelope: map[string]any{"attempts": float64(1)}, RecoveryPolicy: map[string]any{"mode": "rollback"}, RecoveryPolicyRevision: 1, RollbackTargetReleaseID: util.UUIDToString(knownGoodReleaseID), IrreversibleEffects: []string{}}
	prepare := workflowcommand.Request{WorkspaceID: workspaceID, IssueID: issueID, CommandKey: "release/prepare", Kind: workflowstatev2.CommandPrepareRelease, OperationID: util.UUIDToString(executionID), Expected: releaseSnapshotExpected(1, 1), ReleaseAdmission: admission}
	badEvidencePayload := append([]byte(nil), ownerEvidencePayload...)
	var badPayload map[string]any
	_ = json.Unmarshal(badEvidencePayload, &badPayload)
	badPayload["envelope_digest"] = "sha256:unrelated"
	badEvidencePayload, _ = json.Marshal(badPayload)
	if _, err := conn.Exec(ctx, `UPDATE issue_workflow_evidence SET payload=$2 WHERE id=$1`, ownerID, badEvidencePayload); err != nil {
		t.Fatal(err)
	}
	badPrepare := prepare
	badPrepare.CommandKey = "release/bad-owner-evidence"
	if _, err := service.Execute(ctx, workflowcommand.Principal{ActorID: controllerID, ActorType: "internal_controller"}, badPrepare); !errors.Is(err, workflowcommand.ErrForbidden) {
		t.Fatalf("unrelated owner evidence = %v, want forbidden", err)
	}
	if _, err := conn.Exec(ctx, `UPDATE issue_workflow_evidence SET payload=$2 WHERE id=$1`, ownerID, ownerEvidencePayload); err != nil {
		t.Fatal(err)
	}
	var residue int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_command WHERE workspace_id=$1 AND issue_id=$2`, workspaceID, issueID).Scan(&residue); err != nil || residue != 0 {
		t.Fatalf("failed authority residue=%d err=%v", residue, err)
	}
	if _, err := conn.Exec(ctx, `UPDATE issue_workflow_authority SET valid_until=$2 WHERE id=$1`, authorityID, nowValue); err != nil {
		t.Fatal(err)
	}
	expiredPrepare := prepare
	expiredPrepare.CommandKey = "release/expired-authority"
	if _, err := service.Execute(ctx, workflowcommand.Principal{ActorID: controllerID, ActorType: "internal_controller"}, expiredPrepare); err == nil {
		t.Fatal("expired deployment authority unexpectedly admitted")
	}
	if _, err := conn.Exec(ctx, `UPDATE issue_workflow_authority SET valid_until=NULL WHERE id=$1`, authorityID); err != nil {
		t.Fatal(err)
	}
	for _, blocked := range []struct {
		name    string
		block   string
		restore string
		args    []any
	}{
		{name: "archived operator", block: `UPDATE agent SET archived_at=$2 WHERE id=$1`, restore: `UPDATE agent SET archived_at=NULL WHERE id=$1`, args: []any{operatorID, nowValue}},
		{name: "revoked authority", block: `UPDATE issue_workflow_authority SET revoked_at=$2,revoked_by=$3 WHERE id=$1`, restore: `UPDATE issue_workflow_authority SET revoked_at=NULL,revoked_by=NULL WHERE id=$1`, args: []any{authorityID, nowValue, ownerID}},
		{name: "superseded review", block: `UPDATE issue_workflow_review SET superseded_at=$2,superseded_by=$3 WHERE id=$1`, restore: `UPDATE issue_workflow_review SET superseded_at=NULL,superseded_by=NULL WHERE id=$1`, args: []any{reviewID, nowValue, workflowV2UUID(248)}},
	} {
		if _, err := conn.Exec(ctx, blocked.block, blocked.args...); err != nil {
			t.Fatal(err)
		}
		request := prepare
		request.CommandKey = "release/blocked/" + strings.ReplaceAll(blocked.name, " ", "-")
		if _, err := service.Execute(ctx, workflowcommand.Principal{ActorID: controllerID, ActorType: "internal_controller"}, request); err == nil {
			t.Fatalf("%s unexpectedly admitted release", blocked.name)
		}
		if _, err := conn.Exec(ctx, blocked.restore, blocked.args[0]); err != nil {
			t.Fatal(err)
		}
	}
	runAdmissionRace := func(name, update, restore string, args []any) {
		t.Helper()
		raceConn, err := workflowV2TestConnection(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer raceConn.Close(ctx)
		tx, err := raceConn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, update, args...); err != nil {
			t.Fatal(err)
		}
		request := prepare
		request.CommandKey = "release/race/" + name
		result := make(chan error, 1)
		go func() {
			_, executeErr := service.Execute(ctx, workflowcommand.Principal{ActorID: controllerID, ActorType: "internal_controller"}, request)
			result <- executeErr
		}()
		select {
		case err := <-result:
			t.Fatalf("%s admission bypassed competing lock: %v", name, err)
		case <-time.After(50 * time.Millisecond):
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		if err := <-result; err == nil {
			t.Fatalf("%s race unexpectedly admitted release", name)
		}
		if _, err := conn.Exec(ctx, restore, args[0]); err != nil {
			t.Fatal(err)
		}
		var commands int
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_command WHERE workspace_id=$1 AND issue_id=$2 AND command_key=$3`, workspaceID, issueID, request.CommandKey).Scan(&commands); err != nil || commands != 0 {
			t.Fatalf("%s race command residue=%d err=%v", name, commands, err)
		}
	}
	runAdmissionRace("archive-wins", `UPDATE agent SET archived_at=$2 WHERE id=$1`, `UPDATE agent SET archived_at=NULL WHERE id=$1`, []any{operatorID, nowValue})
	runAdmissionRace("revoke-wins", `UPDATE issue_workflow_authority SET revoked_at=$2,revoked_by=$3 WHERE id=$1`, `UPDATE issue_workflow_authority SET revoked_at=NULL,revoked_by=NULL WHERE id=$1`, []any{authorityID, nowValue, ownerID})
	runAdmissionRace("supersede-wins", `UPDATE issue_workflow_review SET superseded_at=$2,superseded_by=$3 WHERE id=$1`, `UPDATE issue_workflow_review SET superseded_at=NULL,superseded_by=NULL WHERE id=$1`, []any{reviewID, nowValue, workflowV2UUID(248)})
	selfAdmission := *admission
	selfAdmission.PlannedVerifierID = operatorID
	selfPrepare := prepare
	selfPrepare.CommandKey, selfPrepare.ReleaseAdmission = "release/self-verification", &selfAdmission
	if _, err := service.Execute(ctx, workflowcommand.Principal{ActorID: controllerID, ActorType: "internal_controller"}, selfPrepare); err == nil {
		t.Fatal("self-verification admission unexpectedly succeeded")
	}
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_command WHERE workspace_id=$1 AND issue_id=$2`, workspaceID, issueID).Scan(&residue); err != nil || residue != 0 {
		t.Fatalf("pre-admission failures left command residue=%d err=%v", residue, err)
	}
	for _, stage := range []string{"after_authority", "after_actions", "after_domain", "after_heads", "after_projection", "after_receipt"} {
		failpointErr := errors.New("release admission failpoint")
		failing := service
		failing.Failpoint = func(got string) error {
			if got == stage {
				return failpointErr
			}
			return nil
		}
		request := prepare
		request.CommandKey = "release/failpoint/" + stage
		if _, err := failing.Execute(ctx, workflowcommand.Principal{ActorID: controllerID, ActorType: "internal_controller"}, request); !errors.Is(err, failpointErr) {
			t.Fatalf("%s error=%v, want failpoint", stage, err)
		}
		var commands, actions int
		var releaseRevision, releaseHeadRevision int64
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_command WHERE workspace_id=$1 AND issue_id=$2`, workspaceID, issueID).Scan(&commands); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_action_receipt WHERE workspace_id=$1 AND issue_id=$2`, workspaceID, issueID).Scan(&actions); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT revision FROM issue_workflow_release WHERE id=$1`, releaseID).Scan(&releaseRevision); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT revision FROM issue_workflow_concern_head WHERE workspace_id=$1 AND issue_id=$2 AND concern_type='release'`, workspaceID, issueID).Scan(&releaseHeadRevision); err != nil {
			t.Fatal(err)
		}
		if commands != 0 || actions != 0 || releaseRevision != 1 || releaseHeadRevision != 1 {
			t.Fatalf("%s residue commands=%d actions=%d release=%d head=%d", stage, commands, actions, releaseRevision, releaseHeadRevision)
		}
	}

	secondConn, err := workflowV2TestConnection(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer secondConn.Close(ctx)
	secondService := workflowcommand.Service{Starter: secondConn, Queries: generated.New(secondConn), Loader: workflowcommand.Loader{Now: now}, Now: now, ControllerID: controllerID}
	competing := prepare
	competing.CommandKey = "release/prepare-competing"
	type prepareResult struct {
		request workflowcommand.Request
		receipt workflowcommand.Receipt
		err     error
	}
	results := make(chan prepareResult, 2)
	for _, item := range []struct {
		svc workflowcommand.Service
		req workflowcommand.Request
	}{{service, prepare}, {secondService, competing}} {
		go func(item struct {
			svc workflowcommand.Service
			req workflowcommand.Request
		}) {
			receipt, executeErr := item.svc.Execute(ctx, workflowcommand.Principal{ActorID: controllerID, ActorType: "internal_controller"}, item.req)
			results <- prepareResult{request: item.req, receipt: receipt, err: executeErr}
		}(item)
	}
	var winner prepareResult
	successes := 0
	for range 2 {
		result := <-results
		if result.err == nil {
			winner, successes = result, successes+1
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent prepare successes=%d, want one", successes)
	}
	prepare, prepareReceipt := winner.request, winner.receipt
	if len(prepareReceipt.ActionIDs) != 1 {
		t.Fatalf("prepare actions = %#v", prepareReceipt.ActionIDs)
	}
	var prepareActionID pgtype.UUID
	prepareActionID = assertReleaseActionPayload(t, ctx, conn, workspaceID, issueID, prepare.CommandKey, "prepare_release", candidateID, scopeID, authorityID, reviewID, executionID, operatorID, verifierID, knownGoodReleaseID, 1, 2, 2, 2, 1)
	taskID := workflowV2UUID(233)
	build := &workflowcommand.ReleaseBuildResultInput{Type: "release_build", BuildManifestDigest: "sha256:manifest", ArtifactDigest: "sha256:artifact", RuntimeContractDigest: "sha256:runtime", Reproducible: true, Coverage: map[string]any{"source_tree": "sha", "assets": "sha", "migrations": "sha", "config": "sha", "runtime_contract": "sha", "artifact": "sha", "build_manifest": "sha", "regression_review_evidence_id": util.UUIDToString(workflowV2UUID(base + 17))}}
	badBuild := *build
	badBuild.Coverage = maps.Clone(build.Coverage)
	badBuild.Coverage["regression_review_evidence_id"] = util.UUIDToString(workflowV2UUID(249))
	resultJSON, _ := json.Marshal(map[string]any{"workflow_result": &badBuild})
	if _, err := conn.Exec(ctx, `INSERT INTO agent_task_queue(id,agent_id,runtime_id,issue_id,status,result,completed_at,workflow_action_id,workflow_operation_id,workflow_branch_key) VALUES($1,$2,$3,$4,'completed',$5,$6,$7,$8,'main')`, taskID, operatorID, operatorRuntime, issueID, resultJSON, nowValue, prepareActionID, executionID); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `UPDATE issue_workflow_action_receipt SET state='settled',bound_task_id=$2,settled_at=$3 WHERE id=$1`, prepareActionID, taskID, nowValue); err != nil {
		t.Fatal(err)
	}
	buildRequest := workflowcommand.Request{WorkspaceID: workspaceID, IssueID: issueID, CommandKey: "task/" + util.UUIDToString(taskID) + "/release-build", Kind: workflowstatev2.CommandBuildRelease, Expected: releaseSnapshotExpected(2, 1), ReleaseAdmission: admission, ReleaseBuild: build}
	badBuildRequest := buildRequest
	badBuildRequest.ReleaseBuild = &badBuild
	if _, err := service.Execute(ctx, workflowcommand.Principal{ActorID: operatorID, ActorType: "agent_task", TaskID: taskID}, badBuildRequest); err == nil {
		t.Fatal("nonexistent regression review evidence unexpectedly authorized a build")
	}
	resultJSON, _ = json.Marshal(map[string]any{"workflow_result": build})
	if _, err := conn.Exec(ctx, `UPDATE agent_task_queue SET result=$2 WHERE id=$1`, taskID, resultJSON); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Execute(ctx, workflowcommand.Principal{ActorID: operatorID, ActorType: "agent_task", TaskID: taskID}, buildRequest); err != nil {
		t.Fatalf("build release: %v", err)
	}
	authorize := workflowcommand.Request{WorkspaceID: workspaceID, IssueID: issueID, CommandKey: "release/authorize", Kind: workflowstatev2.CommandAuthorizeRelease, OperationID: util.UUIDToString(executionID), Expected: releaseSnapshotExpected(3, 2), ReleaseAdmission: admission}
	var projectionRevision int64
	var projectionDocument []byte
	if err := conn.QueryRow(ctx, `SELECT revision,document FROM issue_current_picture WHERE workspace_id=$1 AND issue_id=$2`, workspaceID, issueID).Scan(&projectionRevision, &projectionDocument); err != nil {
		t.Fatal(err)
	}
	for _, stage := range []string{"after_authority", "after_actions", "after_domain", "after_heads", "after_projection", "after_receipt"} {
		failpointErr := errors.New("release authorization failpoint")
		failing := service
		failing.Failpoint = func(got string) error {
			if got == stage {
				return failpointErr
			}
			return nil
		}
		request := authorize
		request.CommandKey = "release/authorize/failpoint/" + stage
		if _, err := failing.Execute(ctx, workflowcommand.Principal{ActorID: controllerID, ActorType: "internal_controller"}, request); !errors.Is(err, failpointErr) {
			t.Fatalf("authorize %s error=%v, want failpoint", stage, err)
		}
		var releaseState string
		var releaseRevision, evidenceHeadRevision, deployActions, commandRows, authorizationEvidence int64
		var gotProjectionRevision int64
		var gotProjectionDocument []byte
		if err := conn.QueryRow(ctx, `SELECT state,revision FROM issue_workflow_release WHERE id=$1`, releaseID).Scan(&releaseState, &releaseRevision); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT revision FROM issue_workflow_concern_head WHERE workspace_id=$1 AND issue_id=$2 AND concern_type='evidence'`, workspaceID, issueID).Scan(&evidenceHeadRevision); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_action_receipt WHERE workspace_id=$1 AND issue_id=$2 AND action_type='deploy_exact_release'`, workspaceID, issueID).Scan(&deployActions); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_command WHERE workspace_id=$1 AND issue_id=$2 AND command_key=$3`, workspaceID, issueID, request.CommandKey).Scan(&commandRows); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_evidence WHERE workspace_id=$1 AND issue_id=$2 AND evidence_type='deployment_authorization_snapshot'`, workspaceID, issueID).Scan(&authorizationEvidence); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT revision,document FROM issue_current_picture WHERE workspace_id=$1 AND issue_id=$2`, workspaceID, issueID).Scan(&gotProjectionRevision, &gotProjectionDocument); err != nil {
			t.Fatal(err)
		}
		if releaseState != "built" || releaseRevision != 3 || evidenceHeadRevision != 2 || deployActions != 0 || commandRows != 0 || authorizationEvidence != 0 || gotProjectionRevision != projectionRevision || !bytes.Equal(gotProjectionDocument, projectionDocument) {
			t.Fatalf("authorize %s residue state=%s release=%d evidence=%d deploy=%d command=%d authorization_evidence=%d projection=%d/%d", stage, releaseState, releaseRevision, evidenceHeadRevision, deployActions, commandRows, authorizationEvidence, gotProjectionRevision, projectionRevision)
		}
	}
	authorized, err := service.Execute(ctx, workflowcommand.Principal{ActorID: controllerID, ActorType: "internal_controller"}, authorize)
	if err != nil {
		t.Fatalf("authorize release: %v", err)
	}
	if len(authorized.ActionIDs) != 1 {
		t.Fatalf("deploy action = %#v", authorized.ActionIDs)
	}
	var state string
	var attempts int16
	var deploys int
	if err := conn.QueryRow(ctx, `SELECT state,attempt_contract_version FROM issue_workflow_release WHERE id=$1`, releaseID).Scan(&state, &attempts); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_action_receipt WHERE workspace_id=$1 AND issue_id=$2 AND action_type='deploy_exact_release'`, workspaceID, issueID).Scan(&deploys); err != nil {
		t.Fatal(err)
	}
	_ = assertReleaseActionPayload(t, ctx, conn, workspaceID, issueID, authorize.CommandKey, "deploy_exact_release", candidateID, scopeID, authorityID, reviewID, executionID, operatorID, verifierID, knownGoodReleaseID, 1, 2, 4, 4, 3)
	if state != "authorized" || attempts != 3 || deploys != 1 {
		t.Fatalf("release state=%s contract=%d deploys=%d", state, attempts, deploys)
	}
	replay, err := service.Execute(ctx, workflowcommand.Principal{ActorID: controllerID, ActorType: "internal_controller"}, authorize)
	if err != nil || !reflect.DeepEqual(authorized, replay) {
		t.Fatalf("authorize replay=(%+v,%v), want %+v", replay, err, authorized)
	}
	if _, err := conn.Exec(ctx, `UPDATE agent SET archived_at=$2 WHERE id IN ($1,$3)`, operatorID, nowValue, verifierID); err != nil {
		t.Fatal(err)
	}
	replayAfterArchive, err := service.Execute(ctx, workflowcommand.Principal{ActorID: controllerID, ActorType: "internal_controller"}, authorize)
	if err != nil || !reflect.DeepEqual(authorized, replayAfterArchive) {
		t.Fatalf("authorize replay after actor archive=(%+v,%v), want %+v", replayAfterArchive, err, authorized)
	}
	mismatch := authorize
	mismatch.ReleaseAdmission = &workflowcommand.ReleaseAdmissionInput{}
	if _, err := service.Execute(ctx, workflowcommand.Principal{ActorID: controllerID, ActorType: "internal_controller"}, mismatch); !errors.Is(err, workflowcommand.ErrIdempotencyConflict) {
		t.Fatalf("semantic mismatch after archive = %v, want idempotency conflict", err)
	}
	if _, err := conn.Exec(ctx, `UPDATE agent SET archived_at=NULL WHERE id IN ($1,$2)`, operatorID, verifierID); err != nil {
		t.Fatal(err)
	}
	provider := workflowaction.NewFakeReleaseProvider()
	provider.SeedTarget(workspaceID, "production", util.UUIDToString(knownGoodReleaseID), 1)
	var deployEffectKey string
	if err := conn.QueryRow(ctx, `SELECT effect_key FROM issue_workflow_action_receipt WHERE workspace_id=$1 AND issue_id=$2 AND action_type='deploy_exact_release'`, workspaceID, issueID).Scan(&deployEffectKey); err != nil {
		t.Fatal(err)
	}
	// A worker without the explicitly configured release lane must leave the
	// durable action untouched for a capable worker. It must not claim it or
	// spend either lease or reconciliation budget.
	withoutRelease := workflowaction.Worker{Starter: conn, Queries: queries, Effect: provider, Capacity: workflowaction.CapacityFunc(func(context.Context, generated.IssueWorkflowActionReceipt) (bool, error) { return true, nil }), CommandService: service, ControllerID: controllerID, Now: now, LeaseDuration: time.Hour, RetryDelay: time.Second}
	if err := withoutRelease.ReconcileOnce(ctx); err != nil {
		t.Fatalf("worker without release adapter: %v", err)
	}
	var untouchedState string
	var untouchedLeaseCount, untouchedObservationFailures int32
	if err := conn.QueryRow(ctx, `SELECT state,lease_count,observation_failure_count FROM issue_workflow_action_receipt WHERE effect_key=$1`, deployEffectKey).Scan(&untouchedState, &untouchedLeaseCount, &untouchedObservationFailures); err != nil || untouchedState != "admitted" || untouchedLeaseCount != 0 || untouchedObservationFailures != 0 {
		t.Fatalf("unconfigured release lane mutated action state=%s leases=%d failures=%d err=%v", untouchedState, untouchedLeaseCount, untouchedObservationFailures, err)
	}
	capacity := false
	verifierEffect := workflowaction.ReleaseVerificationTaskEffect(workflowaction.LocalTaskEffect{Starter: conn, Queries: queries, Now: now})
	worker := workflowaction.Worker{Starter: conn, Queries: queries, Effect: provider, ReleaseEffect: provider, VerifierEffect: verifierEffect, Capacity: workflowaction.CapacityFunc(func(context.Context, generated.IssueWorkflowActionReceipt) (bool, error) { return capacity, nil }), CommandService: service, ControllerID: controllerID, Now: now, LeaseDuration: time.Hour, RetryDelay: time.Second}
	provider.SetUnknown(deployEffectKey, true)
	if err := worker.ReconcileOnce(ctx); err != nil {
		t.Fatalf("unknown release observation: %v", err)
	}
	var effectAttempts, observationFailures int32
	if err := conn.QueryRow(ctx, `SELECT effect_attempt_count,observation_failure_count FROM issue_workflow_action_receipt WHERE effect_key=$1`, deployEffectKey).Scan(&effectAttempts, &observationFailures); err != nil || effectAttempts != 0 || observationFailures != 1 {
		t.Fatalf("unknown observation attempts=%d failures=%d err=%v", effectAttempts, observationFailures, err)
	}
	provider.SetUnknown(deployEffectKey, false)
	clock = clock.Add(2 * time.Second)
	if err := worker.ReconcileOnce(ctx); err != nil {
		t.Fatalf("capacity deferral: %v", err)
	}
	if err := conn.QueryRow(ctx, `SELECT effect_attempt_count FROM issue_workflow_action_receipt WHERE effect_key=$1`, deployEffectKey).Scan(&effectAttempts); err != nil || effectAttempts != 0 {
		t.Fatalf("capacity deferral spent attempts=%d err=%v", effectAttempts, err)
	}
	capacity = true
	clock = clock.Add(2 * time.Second)
	// A command-side failure after domain persistence is still inside the
	// admitted command transaction. It may consume the already-committed effect
	// attempt, but it cannot leave a command, release/head transition, provider
	// effect, or projection residue. Expiry then permits exact reclamation.
	transitionStages := []string{"after_authority", "after_actions", "after_domain", "after_heads", "after_projection", "after_receipt"}
	for _, failStage := range transitionStages {
		worker.CommandService.Failpoint = func(stage string) error {
			if stage == failStage {
				return fmt.Errorf("injected StartDeployment %s rollback", failStage)
			}
			return nil
		}
		beforeStartDeployment := workflowV2ReleaseSemanticSnapshot(t, ctx, conn, workspaceID, issueID, releaseID)
		if err := worker.ReconcileOnce(ctx); err == nil || !strings.Contains(err.Error(), "injected StartDeployment") {
			t.Fatalf("StartDeployment %s failpoint=%v", failStage, err)
		}
		if after := workflowV2ReleaseSemanticSnapshot(t, ctx, conn, workspaceID, issueID, releaseID); !bytes.Equal(beforeStartDeployment, after) {
			t.Fatalf("StartDeployment %s failpoint left semantic residue\nbefore=%s\nafter=%s", failStage, beforeStartDeployment, after)
		}
		clock = clock.Add(2 * time.Hour)
		// Each matrix row starts from the same durable admitted obligation so
		// exercising every rollback boundary does not consume the production
		// retry ceiling merely as a test artifact.
		if _, err := conn.Exec(ctx, `UPDATE issue_workflow_action_receipt SET state='admitted',lease_token=NULL,lease_expires_at=NULL,effect_attempt_count=0,attempt_count=0,reconciliation_state='none',not_before=$2 WHERE effect_key=$1`, deployEffectKey, clock); err != nil {
			t.Fatal(err)
		}
	}
	var stateAfterFailpoint string
	if err := conn.QueryRow(ctx, `SELECT state FROM issue_workflow_release WHERE id=$1`, releaseID).Scan(&stateAfterFailpoint); err != nil || stateAfterFailpoint != "authorized" || provider.ApplyCount(deployEffectKey) != 0 {
		t.Fatalf("8c failpoint residue release=%s applies=%d err=%v", stateAfterFailpoint, provider.ApplyCount(deployEffectKey), err)
	}
	var failedTransitionCommands int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_command WHERE workspace_id=$1 AND issue_id=$2 AND command_key LIKE 'action/%/start_deployment'`, workspaceID, issueID).Scan(&failedTransitionCommands); err != nil || failedTransitionCommands != 0 {
		t.Fatalf("8c failpoint command residue=%d err=%v", failedTransitionCommands, err)
	}
	worker.CommandService.Failpoint = nil
	clock = clock.Add(2 * time.Hour)
	worker.ReleaseEffect = workflowaction.EffectFuncs{ObserveFunc: provider.Observe, ApplyFunc: func(ctx context.Context, action generated.IssueWorkflowActionReceipt) (workflowaction.Observation, error) {
		if _, err := provider.Apply(ctx, action); err != nil {
			return workflowaction.Observation{Kind: workflowaction.ObservationUnknown}, err
		}
		// Simulate an external call that completed after this worker's lease
		// expired. The stale token cannot record or settle anything; the next
		// poll must reclaim and reconcile the exact provider effect.
		clock = clock.Add(2 * time.Hour)
		return workflowaction.Observation{Kind: workflowaction.ObservationUnknown}, workflowaction.ErrEffectUnknown
	}}
	if err := worker.ReconcileOnce(ctx); err != nil {
		t.Fatalf("ambiguous post-apply crash: %v", err)
	}
	var releaseAfterAmbiguity string
	if err := conn.QueryRow(ctx, `SELECT state FROM issue_workflow_release WHERE id=$1`, releaseID).Scan(&releaseAfterAmbiguity); err != nil || releaseAfterAmbiguity != "deploying" || provider.ApplyCount(deployEffectKey) != 1 {
		t.Fatalf("ambiguous effect state=%s applies=%d err=%v", releaseAfterAmbiguity, provider.ApplyCount(deployEffectKey), err)
	}
	clock = clock.Add(2 * time.Second)
	worker.ReleaseEffect = provider
	for _, failStage := range transitionStages {
		worker.CommandService.Failpoint = func(stage string) error {
			if stage == failStage {
				return fmt.Errorf("injected RecordDeployment %s rollback", failStage)
			}
			return nil
		}
		beforeRecordDeployment := workflowV2ReleaseSemanticSnapshot(t, ctx, conn, workspaceID, issueID, releaseID)
		if err := worker.ReconcileOnce(ctx); err != nil {
			t.Fatalf("RecordDeployment %s failpoint recovery scheduling: %v", failStage, err)
		}
		if after := workflowV2ReleaseSemanticSnapshot(t, ctx, conn, workspaceID, issueID, releaseID); !bytes.Equal(beforeRecordDeployment, after) {
			t.Fatalf("RecordDeployment %s failpoint left semantic residue\nbefore=%s\nafter=%s", failStage, beforeRecordDeployment, after)
		}
		clock = clock.Add(2 * time.Second)
	}
	if err := conn.QueryRow(ctx, `SELECT state FROM issue_workflow_release WHERE id=$1`, releaseID).Scan(&releaseAfterAmbiguity); err != nil || releaseAfterAmbiguity != "deploying" {
		t.Fatalf("record deployment failpoint residue=%s err=%v", releaseAfterAmbiguity, err)
	}
	worker.CommandService.Failpoint = nil
	clock = clock.Add(2 * time.Second)
	assertReleaseState := func(step, want string, liveAllowed bool) {
		t.Helper()
		var got string
		if err := conn.QueryRow(ctx, `SELECT state FROM issue_workflow_release WHERE id=$1`, releaseID).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("release worker step %s state=%s want=%s", step, got, want)
		}
		var projectedRelease string
		if err := conn.QueryRow(ctx, `SELECT release_state FROM issue_current_picture WHERE workspace_id=$1 AND issue_id=$2`, workspaceID, issueID).Scan(&projectedRelease); err != nil {
			t.Fatal(err)
		}
		if projectedRelease != want || !liveAllowed && projectedRelease == "live" {
			t.Fatalf("projection release=%s want=%s at step %s", projectedRelease, want, step)
		}
		var outcome string
		if err := conn.QueryRow(ctx, `SELECT state FROM issue_workflow_outcome WHERE workspace_id=$1 AND issue_id=$2 ORDER BY revision DESC LIMIT 1`, workspaceID, issueID).Scan(&outcome); err != nil {
			t.Fatal(err)
		}
		if !liveAllowed && outcome == "satisfied" {
			t.Fatalf("release became satisfied before independent business proof at step %s", step)
		}
	}
	if err := worker.ReconcileOnce(ctx); err != nil {
		t.Fatalf("release worker deployment readback: %v", err)
	}
	assertReleaseState("deployment-readback", "deployed", false)

	// Provider exposure can succeed while its authoritative RecordExposure
	// transaction rolls back. The durable receipt retries by observation, never
	// by a second provider Apply.
	clock = clock.Add(2 * time.Second)
	for _, failStage := range transitionStages {
		worker.CommandService.Failpoint = func(stage string) error {
			if stage == failStage {
				return fmt.Errorf("injected RecordExposure %s rollback", failStage)
			}
			return nil
		}
		beforeRecordExposure := workflowV2ReleaseSemanticSnapshot(t, ctx, conn, workspaceID, issueID, releaseID)
		if err := worker.ReconcileOnce(ctx); err != nil {
			t.Fatalf("RecordExposure %s failpoint recovery scheduling: %v", failStage, err)
		}
		if after := workflowV2ReleaseSemanticSnapshot(t, ctx, conn, workspaceID, issueID, releaseID); !bytes.Equal(beforeRecordExposure, after) {
			t.Fatalf("RecordExposure %s failpoint left semantic residue\nbefore=%s\nafter=%s", failStage, beforeRecordExposure, after)
		}
		clock = clock.Add(2 * time.Second)
	}
	var exposureRollbackState string
	if err := conn.QueryRow(ctx, `SELECT state FROM issue_workflow_release WHERE id=$1`, releaseID).Scan(&exposureRollbackState); err != nil || exposureRollbackState != "deployed" {
		t.Fatalf("record exposure failpoint residue=%s err=%v", exposureRollbackState, err)
	}
	worker.CommandService.Failpoint = nil

	// Both workers see the same due exposure obligation. Exact claim fencing and
	// the immutable provider effect key permit only one provider application.
	clock = clock.Add(2 * time.Second)
	secondVerifierEffect := workflowaction.ReleaseVerificationTaskEffect(workflowaction.LocalTaskEffect{Starter: secondConn, Queries: generated.New(secondConn), Now: now})
	secondWorker := workflowaction.Worker{
		Starter: secondConn, Queries: generated.New(secondConn), Effect: provider, ReleaseEffect: provider,
		VerifierEffect: secondVerifierEffect, Capacity: worker.Capacity, CommandService: secondService, ControllerID: controllerID, Now: now,
		LeaseDuration: time.Hour, RetryDelay: time.Second,
	}
	workerResults := make(chan error, 2)
	go func() { workerResults <- worker.ReconcileOnce(ctx) }()
	go func() { workerResults <- secondWorker.ReconcileOnce(ctx) }()
	for range 2 {
		if err := <-workerResults; err != nil {
			t.Fatalf("same-target worker race: %v", err)
		}
	}
	assertReleaseState("two-worker-exposure", "exposed", false)
	var exposeEffectKey string
	if err := conn.QueryRow(ctx, `SELECT effect_key FROM issue_workflow_action_receipt WHERE workspace_id=$1 AND issue_id=$2 AND action_type='expose_exact_release'`, workspaceID, issueID).Scan(&exposeEffectKey); err != nil || provider.ApplyCount(exposeEffectKey) != 1 {
		t.Fatalf("two workers exposure applies=%d err=%v", provider.ApplyCount(exposeEffectKey), err)
	}
	semanticReleaseSnapshot := func() []byte {
		return workflowV2ReleaseSemanticSnapshot(t, ctx, conn, workspaceID, issueID, releaseID)
	}
	var monitorTaskID pgtype.UUID

	completeVerificationTask := func(actionType string, result workflowcommand.CorrectionResultInput) pgtype.UUID {
		t.Helper()
		clock = clock.Add(2 * time.Second)
		if err := worker.ReconcileOnce(ctx); err != nil {
			t.Fatalf("create %s task: %v", actionType, err)
		}
		var taskID pgtype.UUID
		if err := conn.QueryRow(ctx, `SELECT bound_task_id FROM issue_workflow_action_receipt WHERE workspace_id=$1 AND issue_id=$2 AND action_type=$3`, workspaceID, issueID, actionType).Scan(&taskID); err != nil || !taskID.Valid {
			t.Fatalf("%s task binding=%s err=%v", actionType, util.UUIDToString(taskID), err)
		}
		encoded, _ := json.Marshal(map[string]any{"workflow_result": result})
		taskService := &taskservice.TaskService{Queries: queries, TxStarter: conn}
		if taskID != monitorTaskID {
			if _, err := conn.Exec(ctx, `UPDATE agent_task_queue SET status='running' WHERE id=$1 AND status='queued'`, taskID); err != nil {
				t.Fatal(err)
			}
			if _, err := taskService.CompleteWorkflowTaskForDaemon(ctx, taskID, encoded, "", "", "", false, "", "", "wrong-same-workspace-daemon"); !errors.Is(err, taskservice.ErrWorkflowTaskRuntimeForbidden) {
				t.Fatalf("wrong runtime completed %s: %v", actionType, err)
			}
			var stillRunning string
			if err := conn.QueryRow(ctx, `SELECT status FROM agent_task_queue WHERE id=$1`, taskID).Scan(&stillRunning); err != nil || stillRunning != "running" {
				t.Fatalf("wrong runtime residue status=%s err=%v", stillRunning, err)
			}
		}
		if taskID != monitorTaskID && actionType == "verify_runtime_exact_release" {
			if _, err := conn.Exec(ctx, `UPDATE agent SET archived_at=$2 WHERE id=$1`, verifierID, clock); err != nil {
				t.Fatal(err)
			}
			if _, err := taskService.CompleteWorkflowTaskForDaemon(ctx, taskID, encoded, "", "", "", false, "", "", "release-verifier-daemon"); !errors.Is(err, taskservice.ErrWorkflowTaskRuntimeForbidden) {
				t.Fatalf("archived verifier completed runtime task: %v", err)
			}
			if _, err := conn.Exec(ctx, `UPDATE agent SET archived_at=NULL WHERE id=$1`, verifierID); err != nil {
				t.Fatal(err)
			}
		} else if taskID != monitorTaskID {
			if _, err := conn.Exec(ctx, `UPDATE agent SET runtime_id=$2 WHERE id=$1`, verifierID, operatorRuntime); err != nil {
				t.Fatal(err)
			}
			if _, err := taskService.CompleteWorkflowTaskForDaemon(ctx, taskID, encoded, "", "", "", false, "", "", "release-verifier-daemon"); !errors.Is(err, taskservice.ErrWorkflowTaskRuntimeForbidden) {
				t.Fatalf("rebound verifier completed business task: %v", err)
			}
			if _, err := conn.Exec(ctx, `UPDATE agent SET runtime_id=$2 WHERE id=$1`, verifierID, verifierRuntime); err != nil {
				t.Fatal(err)
			}
		}
		if taskID != monitorTaskID {
			if _, err := taskService.CompleteWorkflowTaskForDaemon(ctx, taskID, encoded, "", "", "", false, "", "", "release-verifier-daemon"); err != nil {
				t.Fatalf("correct verifier runtime complete %s: %v", actionType, err)
			}
			if replayed, err := taskService.CompleteWorkflowTaskForDaemon(ctx, taskID, encoded, "", "", "", false, "", "", "release-verifier-daemon"); err != nil || replayed.Status != "completed" {
				t.Fatalf("verifier completion replay %s=(%v,%v)", actionType, replayed, err)
			}
		}
		if actionType == "verify_runtime_exact_release" || actionType == "verify_business_exact_release" || actionType == "monitor_exact_release" || actionType == "verify_restoration_exact_release" {
			failAt := []int{1}
			failStages := []string{"after_domain"}
			if actionType == "monitor_exact_release" || actionType == "verify_restoration_exact_release" {
				failStages = transitionStages
			}
			// Each structured verifier result is one admitted command transaction.
			// BeginVerification and its verifier result share one admitted
			// transaction, so a failure of its shared consequential domain
			// boundary proves both state changes roll back together. The
			// surrounding generic command boundaries are exhaustively exercised
			// above by the single-command release transitions.
			for _, failStage := range failStages {
				for _, targetCall := range failAt {
					before := semanticReleaseSnapshot()
					calls := 0
					service.Failpoint = func(stage string) error {
						if stage == failStage {
							calls++
							if calls == targetCall {
								return fmt.Errorf("injected %s %s transition rollback", actionType, failStage)
							}
						}
						return nil
					}
					receipt, reconcileErr := service.ReconcileWorkflowTaskResult(ctx, taskID)
					if reconcileErr == nil || !strings.Contains(reconcileErr.Error(), "transition rollback") {
						var actionError, actionState string
						_ = conn.QueryRow(ctx, `SELECT last_error,state FROM issue_workflow_action_receipt WHERE bound_task_id=$1`, taskID).Scan(&actionError, &actionState)
						t.Fatalf("%s %s transition failpoint call=%d receipt=%+v err=%v action=%s last_error=%s", actionType, failStage, targetCall, receipt, reconcileErr, actionState, actionError)
					}
					after := semanticReleaseSnapshot()
					if !bytes.Equal(before, after) {
						t.Fatalf("%s %s transition failpoint call=%d left semantic residue\nbefore=%s\nafter=%s", actionType, failStage, targetCall, before, after)
					}
				}
			}
			service.Failpoint = nil
		}
		reconciled, err := service.ReconcileWorkflowTaskResult(ctx, taskID)
		if err != nil {
			t.Fatalf("reconcile %s task: %v", actionType, err)
		}
		if reconciled.State != "applied" {
			var lastError string
			var debugJSON []byte
			var predicatesJSON []byte
			_ = conn.QueryRow(ctx, `SELECT last_error FROM issue_workflow_action_receipt WHERE bound_task_id=$1`, taskID).Scan(&lastError)
			_ = conn.QueryRow(ctx, `SELECT jsonb_build_object('action_state',a.state,'action_revision',a.revision,'action_operation',a.operation_id,'payload',a.payload,'release_state',r.state,'release_revision',r.revision,'release_operation',r.operation_id,'release_branch',r.branch_key,'execution_state',x.state,'execution_revision',x.revision,'task_status',t.status,'task_agent',t.agent_id,'task_runtime',t.runtime_id,'task_operation',t.workflow_operation_id,'task_branch',t.workflow_branch_key,'planned_verifier',r.planned_verifier_id,'target_state',h.state,'target_issue',h.issue_id,'target_release',h.release_id,'target_candidate',h.candidate_id,'target_generation',h.generation) FROM issue_workflow_action_receipt a JOIN agent_task_queue t ON t.id=a.bound_task_id JOIN issue_workflow_release r ON r.id=a.release_id JOIN issue_workflow_execution x ON x.id=r.operation_id JOIN issue_workflow_target_head h ON h.workspace_id=r.workspace_id AND h.target_key=r.target_key WHERE a.bound_task_id=$1`, taskID).Scan(&debugJSON)
			_ = conn.QueryRow(ctx, `SELECT jsonb_build_object('release_head',EXISTS(SELECT 1 FROM issue_workflow_concern_head h WHERE h.workspace_id=r.workspace_id AND h.issue_id=r.issue_id AND h.concern_type='release' AND h.source_record_id=r.id AND h.source_record_revision=r.revision AND h.state=r.state),'execution_head',EXISTS(SELECT 1 FROM issue_workflow_concern_head h WHERE h.workspace_id=x.workspace_id AND h.issue_id=x.issue_id AND h.concern_type='execution' AND h.source_record_id=x.id AND h.source_record_revision=x.revision AND h.state=x.state),'agent',EXISTS(SELECT 1 FROM agent g WHERE g.id=t.agent_id AND g.id=r.planned_verifier_id AND g.workspace_id=r.workspace_id AND g.archived_at IS NULL AND g.id<>r.release_operator_id),'runtime',EXISTS(SELECT 1 FROM agent g JOIN agent_runtime ar ON ar.id=t.runtime_id AND ar.id=g.runtime_id AND ar.workspace_id=r.workspace_id WHERE g.id=t.agent_id),'payload_match',(a.payload->>'release_revision')::bigint=r.revision AND a.payload->>'planned_verifier_id'=t.agent_id::text) FROM issue_workflow_action_receipt a JOIN agent_task_queue t ON t.id=a.bound_task_id JOIN issue_workflow_release r ON r.id=a.release_id JOIN issue_workflow_execution x ON x.id=r.operation_id WHERE a.bound_task_id=$1`, taskID).Scan(&predicatesJSON)
			t.Fatalf("reconcile %s task state=%s last_error=%s debug=%s predicates=%s", actionType, reconciled.State, lastError, debugJSON, predicatesJSON)
		}
		return taskID
	}
	runtimeProof := workflowcommand.VerificationProof{CriteriaRefs: []string{"runtime-contract"}, CoverageRefs: []string{"availability", "contract"}, Evidence: map[string]any{"healthy": true, "runtime_digest": "sha256:runtime"}}
	completeVerificationTask("verify_runtime_exact_release", workflowcommand.CorrectionResultInput{Type: "runtime_verification", RuntimeProof: &runtimeProof})
	assertReleaseState("task-authenticated-runtime", "production_verifying", false)
	businessProof := workflowcommand.VerificationProof{CriteriaRefs: []string{"owner-outcome"}, CoverageRefs: []string{"canary-business-check"}, Evidence: map[string]any{"accepted": true, "sample": "canary"}}
	regressionProof := workflowcommand.VerificationProof{CriteriaRefs: []string{"no-regression"}, CoverageRefs: []string{"regression-suite"}, Evidence: map[string]any{"passed": true, "suite": "release"}}
	completeVerificationTask("verify_business_exact_release", workflowcommand.CorrectionResultInput{Type: "business_verification", BusinessProof: &businessProof, RegressionProof: &regressionProof})
	assertReleaseState("task-authenticated-business", "live", true)
	var targetGeneration int64
	var targetRelease pgtype.UUID
	if err := conn.QueryRow(ctx, `SELECT generation,release_id FROM issue_workflow_target_head WHERE workspace_id=$1 AND target_key='production'`, workspaceID).Scan(&targetGeneration, &targetRelease); err != nil {
		t.Fatal(err)
	}
	if targetGeneration != 2 || targetRelease != releaseID {
		t.Fatalf("observed target head=(%d,%s), want exact release generation 2", targetGeneration, util.UUIDToString(targetRelease))
	}
	for _, actionType := range []string{"deploy_exact_release", "expose_exact_release"} {
		var actionState, effectKey string
		var attempts int32
		if err := conn.QueryRow(ctx, `SELECT state,effect_key,effect_attempt_count FROM issue_workflow_action_receipt WHERE workspace_id=$1 AND issue_id=$2 AND action_type=$3`, workspaceID, issueID, actionType).Scan(&actionState, &effectKey, &attempts); err != nil {
			t.Fatal(err)
		}
		if actionState != "settled" || attempts != 1 || provider.ApplyCount(effectKey) != 1 {
			t.Fatalf("%s state=%s attempts=%d applies=%d", actionType, actionState, attempts, provider.ApplyCount(effectKey))
		}
	}
	var final workflowstatev2.CurrentPicture
	if err := workflowstore.InIssueWriteTransaction(ctx, conn, queries, workspaceID, issueID, func(write *workflowstore.IssueWrite) error {
		var loadErr error
		final, loadErr = (workflowcommand.Loader{Now: now}).Load(ctx, write, workspaceID, issueID)
		return loadErr
	}); err != nil {
		t.Fatalf("reload live release: %v", err)
	}
	if final.Release.State != workflowstatev2.ReleaseLive || final.Outcome != workflowstatev2.OutcomeSatisfied || len(final.Evidence.Runtime) != 1 || len(final.Evidence.Business) < 2 {
		t.Fatalf("final authoritative release=%s outcome=%s runtime=%d business=%d", final.Release.State, final.Outcome, len(final.Evidence.Runtime), len(final.Evidence.Business))
	}
	// The predecessor's independent proof remains immutable history. Seed it only
	// after the forward release has completed so earlier forward transitions do
	// not accidentally treat historical proof as current transition output.
	if _, err := conn.Exec(ctx, `INSERT INTO issue_workflow_evidence(id,workspace_id,issue_id,concern_type,concern_id,concern_revision,evidence_type,level,source,producer_type,producer_id,candidate_id,target_key,observed_at,payload) SELECT gen_random_uuid(),$1,$2,'release',$3,1,kind,'production','known-good-proof','verifier',$4,$5,'production',$6,jsonb_build_object('freshness','current','release_id',$3::uuid::text,'candidate_revision',1,'scope_id',$7::uuid::text,'scope_revision',1,'acceptance_version',1,'target_generation',1) FROM unnest(ARRAY['runtime','business','regression']) AS kind`, workspaceID, knownGoodIssueID, knownGoodReleaseID, verifierID, knownGoodCandidateID, nowValue, scopeID); err != nil {
		t.Fatal(err)
	}
	productionRegression := workflowcommand.VerificationProof{CriteriaRefs: []string{"availability"}, CoverageRefs: []string{"current-production"}, Evidence: map[string]any{"healthy": false, "error_rate": "critical"}}
	// Complete the exact monitor task without reconciling it yet. Each mutation
	// below executes the real command service inside an outer savepoint, proving
	// invalid predecessor proof cannot leave a command or semantic residue.
	clock = clock.Add(2 * time.Second)
	if err := worker.ReconcileOnce(ctx); err != nil {
		t.Fatalf("create monitor task for predecessor proof gates: %v", err)
	}
	var monitorActionID pgtype.UUID
	var monitorActionKey, monitorEffectKey string
	if err := conn.QueryRow(ctx, `SELECT id,action_key,effect_key,bound_task_id FROM issue_workflow_action_receipt WHERE workspace_id=$1 AND issue_id=$2 AND action_type='monitor_exact_release'`, workspaceID, issueID).Scan(&monitorActionID, &monitorActionKey, &monitorEffectKey, &monitorTaskID); err != nil {
		t.Fatal(err)
	}
	monitorResult, _ := json.Marshal(map[string]any{"workflow_result": workflowcommand.CorrectionResultInput{Type: "production_regression", RegressionProof: &productionRegression}})
	if _, err := conn.Exec(ctx, `UPDATE agent_task_queue SET status='running' WHERE id=$1 AND status='queued'`, monitorTaskID); err != nil {
		t.Fatal(err)
	}
	if _, err := (&taskservice.TaskService{Queries: queries, TxStarter: conn}).CompleteWorkflowTaskForDaemon(ctx, monitorTaskID, monitorResult, "", "", "", false, "", "", "release-verifier-daemon"); err != nil {
		t.Fatalf("complete monitor task for predecessor proof gates: %v", err)
	}
	expectedRegression := make([]workflowstatev2.ExpectedRevision, 0, 4)
	rows, err := conn.Query(ctx, `SELECT concern_type,revision FROM issue_workflow_concern_head WHERE workspace_id=$1 AND issue_id=$2 AND concern_type = ANY($3::text[]) ORDER BY concern_type`, workspaceID, issueID, []string{"evidence", "execution", "outcome", "release"})
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var concern string
		var revision int64
		if err := rows.Scan(&concern, &revision); err != nil {
			t.Fatal(err)
		}
		expectedRegression = append(expectedRegression, workflowstatev2.ExpectedRevision{Concern: workflowstatev2.Concern(concern), Revision: workflowstatev2.Revision(revision)})
	}
	rows.Close()
	regressionRequest := workflowcommand.Request{WorkspaceID: workspaceID, IssueID: issueID, CommandKey: "task/" + util.UUIDToString(monitorTaskID) + "/production-regression-result", Kind: workflowstatev2.CommandProductionFailed, Expected: expectedRegression, OperationID: util.UUIDToString(executionID), BranchID: "main", CandidateID: util.UUIDToString(candidateID), TargetID: "production", ReleaseEffect: &workflowcommand.ReleaseEffectResultInput{Type: "regression", EffectKey: monitorEffectKey, ProviderReceiptID: util.UUIDToString(monitorTaskID), ReleaseID: util.UUIDToString(releaseID), CandidateID: util.UUIDToString(candidateID), CandidateRevision: 1, ScopeID: util.UUIDToString(scopeID), ScopeRevision: 1, AcceptanceVersion: 1, OperationID: util.UUIDToString(executionID), BranchKey: "main", TargetKey: "production", TargetGeneration: 2, ArtifactDigest: "sha256:artifact", RuntimeContractDigest: "sha256:runtime", Evidence: map[string]any{"regression_proof": productionRegression}}}
	regressionPrincipal := workflowcommand.Principal{ActorID: verifierID, ActorType: "agent_task", TaskID: monitorTaskID, ActionID: monitorActionID, ActionKey: monitorActionKey}
	proofMutations := []struct {
		name string
		sql  string
		args []any
	}{
		{"expired", `UPDATE issue_workflow_evidence SET valid_until=$2 WHERE concern_id=$1 AND evidence_type='runtime'`, []any{knownGoodReleaseID, clock}},
		{"wrong producer", `UPDATE issue_workflow_evidence SET producer_id=$2 WHERE concern_id=$1 AND evidence_type='business'`, []any{knownGoodReleaseID, operatorID}},
		{"wrong candidate", `UPDATE issue_workflow_evidence SET candidate_id=$2 WHERE concern_id=$1 AND evidence_type='regression'`, []any{knownGoodReleaseID, candidateID}},
		{"wrong generation", `UPDATE issue_workflow_evidence SET payload=jsonb_set(payload,'{target_generation}','999'::jsonb) WHERE concern_id=$1 AND evidence_type='runtime'`, []any{knownGoodReleaseID}},
	}
	for _, mutation := range proofMutations {
		t.Run("known-good predecessor "+mutation.name, func(t *testing.T) {
			before := workflowV2ReleaseSemanticSnapshot(t, ctx, conn, workspaceID, issueID, releaseID)
			tx, err := conn.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if _, err := tx.Exec(ctx, mutation.sql, mutation.args...); err != nil {
				t.Fatal(err)
			}
			isolated := service
			isolated.Starter, isolated.Queries = tx, generated.New(tx)
			var admissionErr error
			admissionErr = workflowstore.InReleaseAdmissionTransaction(ctx, tx, generated.New(tx), workspaceID, issueID, []pgtype.UUID{operatorID, verifierID}, func(write *workflowstore.IssueWrite) error {
				_, admissionErr = isolated.ExecuteInWrite(ctx, write, regressionPrincipal, regressionRequest)
				return admissionErr
			})
			if !errors.Is(admissionErr, workflowcommand.ErrInvalidAuthoritativeState) {
				t.Fatalf("invalid predecessor proof error=%v", admissionErr)
			}
			var commandResidue int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_command WHERE workspace_id=$1 AND issue_id=$2 AND command_key=$3`, workspaceID, issueID, regressionRequest.CommandKey).Scan(&commandResidue); err != nil || commandResidue != 0 {
				t.Fatalf("invalid predecessor command residue=%d err=%v", commandResidue, err)
			}
			if err := tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			if after := workflowV2ReleaseSemanticSnapshot(t, ctx, conn, workspaceID, issueID, releaseID); !bytes.Equal(before, after) {
				t.Fatalf("invalid predecessor proof left semantic residue\nbefore=%s\nafter=%s", before, after)
			}
		})
	}
	completeVerificationTask("monitor_exact_release", workflowcommand.CorrectionResultInput{Type: "production_regression", RegressionProof: &productionRegression})
	var forwardHistoryState, reopenedOutcome, targetBeforeRollback string
	var incidentCount, recoveryReleaseCount int
	if err := conn.QueryRow(ctx, `SELECT state FROM issue_workflow_release WHERE id=$1`, releaseID).Scan(&forwardHistoryState); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(ctx, `SELECT state FROM issue_workflow_outcome WHERE workspace_id=$1 AND issue_id=$2 ORDER BY revision DESC LIMIT 1`, workspaceID, issueID).Scan(&reopenedOutcome); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(ctx, `SELECT state FROM issue_workflow_target_head WHERE workspace_id=$1 AND target_key='production'`, workspaceID).Scan(&targetBeforeRollback); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_incident WHERE workspace_id=$1 AND issue_id=$2 AND regression_fingerprint IS NOT NULL AND state='open'`, workspaceID, issueID).Scan(&incidentCount); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_release WHERE workspace_id=$1 AND issue_id=$2 AND release_kind='recovery'`, workspaceID, issueID).Scan(&recoveryReleaseCount); err != nil {
		t.Fatal(err)
	}
	var recoveryReleaseID pgtype.UUID
	if err := conn.QueryRow(ctx, `SELECT recovery_release_id FROM issue_workflow_incident WHERE workspace_id=$1 AND issue_id=$2 AND state='open'`, workspaceID, issueID).Scan(&recoveryReleaseID); err != nil {
		t.Fatal(err)
	}
	var recoveryState string
	var recoveryGeneration int64
	var recoveryIssueID, recoveryCandidateID, recoveryReviewID pgtype.UUID
	var recoveryBuild, recoveryArtifact, recoveryRuntime string
	if err := conn.QueryRow(ctx, `SELECT issue_id,candidate_id,state,target_generation,build_manifest_digest,artifact_digest,runtime_contract_digest,reviewed_review_id FROM issue_workflow_release WHERE id=$1`, recoveryReleaseID).Scan(&recoveryIssueID, &recoveryCandidateID, &recoveryState, &recoveryGeneration, &recoveryBuild, &recoveryArtifact, &recoveryRuntime, &recoveryReviewID); err != nil {
		t.Fatal(err)
	}
	var snapshottedSource string
	if err := conn.QueryRow(ctx, `SELECT source_revision FROM issue_workflow_candidate WHERE workspace_id=$1 AND issue_id=$2 AND id=$3`, workspaceID, issueID, recoveryCandidateID).Scan(&snapshottedSource); err != nil {
		t.Fatal(err)
	}
	if recoveryIssueID != issueID || recoveryCandidateID == knownGoodCandidateID || recoveryReviewID != knownGoodReviewID || recoveryBuild != "sha256:known-good-manifest" || recoveryArtifact != "sha256:known-good-artifact" || recoveryRuntime != "sha256:known-good-runtime" || !strings.Contains(snapshottedSource, util.UUIDToString(knownGoodReleaseID)) {
		t.Fatalf("recovery lineage issue=%s candidate=%s source=%s review=%s build=%s artifact=%s runtime=%s", util.UUIDToString(recoveryIssueID), util.UUIDToString(recoveryCandidateID), snapshottedSource, util.UUIDToString(recoveryReviewID), recoveryBuild, recoveryArtifact, recoveryRuntime)
	}
	if forwardHistoryState != "live" || targetBeforeRollback != "live" || reopenedOutcome != "open" || incidentCount != 1 || recoveryReleaseCount != 1 || recoveryState != "rolling_back" || recoveryGeneration != 3 {
		t.Fatalf("rollback admission forward=%s head=%s outcome=%s incidents=%d recovery_releases=%d recovery=%s generation=%d", forwardHistoryState, targetBeforeRollback, reopenedOutcome, incidentCount, recoveryReleaseCount, recoveryState, recoveryGeneration)
	}
	var rollbackEffectKey string
	if err := conn.QueryRow(ctx, `SELECT effect_key FROM issue_workflow_action_receipt WHERE workspace_id=$1 AND issue_id=$2 AND action_type='rollback_exact_release'`, workspaceID, issueID).Scan(&rollbackEffectKey); err != nil {
		t.Fatal(err)
	}
	// The provider may commit while the command transaction rolls back. The
	// durable action must observe that same immutable effect on the next poll,
	// never apply a second rollback or advance the target before the command CAS.
	worker.CommandService.Failpoint = func(stage string) error {
		if stage == "after_domain" {
			return errors.New("injected rollback settlement crash")
		}
		return nil
	}
	if err := worker.ReconcileOnce(ctx); err != nil {
		t.Fatalf("rollback provider crash recovery scheduling: %v", err)
	}
	if err := conn.QueryRow(ctx, `SELECT state FROM issue_workflow_release WHERE id=$1`, recoveryReleaseID).Scan(&recoveryState); err != nil || recoveryState != "rolling_back" || provider.ApplyCount(rollbackEffectKey) != 1 {
		var actionState, reconciliationState, lastError string
		var actionPayload []byte
		_ = conn.QueryRow(ctx, `SELECT state,reconciliation_state,last_error,payload FROM issue_workflow_action_receipt WHERE workspace_id=$1 AND issue_id=$2 AND action_type='rollback_exact_release'`, workspaceID, issueID).Scan(&actionState, &reconciliationState, &lastError, &actionPayload)
		t.Fatalf("rollback failpoint residue recovery=%s applies=%d err=%v action=%s reconciliation=%s last_error=%s payload=%s", recoveryState, provider.ApplyCount(rollbackEffectKey), err, actionState, reconciliationState, lastError, actionPayload)
	}
	worker.CommandService.Failpoint = nil
	clock = clock.Add(2 * time.Second)
	if err := worker.ReconcileOnce(ctx); err != nil {
		t.Fatalf("rollback observed-effect reconciliation: %v", err)
	}
	var rollbackHeadRelease pgtype.UUID
	var rollbackHeadState string
	if err := conn.QueryRow(ctx, `SELECT release_id,generation,state FROM issue_workflow_target_head WHERE workspace_id=$1 AND target_key='production'`, workspaceID).Scan(&rollbackHeadRelease, &targetGeneration, &rollbackHeadState); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(ctx, `SELECT state FROM issue_workflow_release WHERE id=$1`, recoveryReleaseID).Scan(&recoveryState); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(ctx, `SELECT state FROM issue_workflow_release WHERE id=$1`, releaseID).Scan(&forwardHistoryState); err != nil {
		t.Fatal(err)
	}
	if rollbackHeadRelease != recoveryReleaseID || targetGeneration != 3 || rollbackHeadState != "rolled_back" || recoveryState != "rolled_back" {
		var actionState, lastError string
		var notBefore time.Time
		var actionPayload, headDebug []byte
		_ = conn.QueryRow(ctx, `SELECT state,last_error,not_before FROM issue_workflow_action_receipt WHERE workspace_id=$1 AND issue_id=$2 AND action_type='rollback_exact_release'`, workspaceID, issueID).Scan(&actionState, &lastError, &notBefore)
		_ = conn.QueryRow(ctx, `SELECT payload FROM issue_workflow_action_receipt WHERE workspace_id=$1 AND issue_id=$2 AND action_type='rollback_exact_release'`, workspaceID, issueID).Scan(&actionPayload)
		_ = conn.QueryRow(ctx, `SELECT jsonb_build_object('source_id',h.source_record_id,'source_revision',h.source_record_revision,'state',h.state,'release_id',r.id,'release_revision',r.revision,'release_state',r.state) FROM issue_workflow_concern_head h JOIN issue_workflow_release r ON r.id=h.source_record_id WHERE h.workspace_id=$1 AND h.issue_id=$2 AND h.concern_type='release'`, workspaceID, issueID).Scan(&headDebug)
		t.Fatalf("rollback observation head=(%s,%d,%s) recovery=%s action=%s not_before=%s last_error=%s concern=%s payload=%s", util.UUIDToString(rollbackHeadRelease), targetGeneration, rollbackHeadState, recoveryState, actionState, notBefore, lastError, headDebug, actionPayload)
	}
	restorationProof := workflowcommand.VerificationProof{CriteriaRefs: []string{"restoration"}, CoverageRefs: []string{"known-good-generation"}, Evidence: map[string]any{"restored": true, "generation": 3}}
	completeVerificationTask("verify_restoration_exact_release", workflowcommand.CorrectionResultInput{Type: "restoration_verification", RuntimeProof: &runtimeProof, BusinessProof: &businessProof, RegressionProof: &regressionProof, RestorationProof: &restorationProof})
	var incidentState, restoredProjection string
	if err := conn.QueryRow(ctx, `SELECT state FROM issue_workflow_release WHERE id=$1`, recoveryReleaseID).Scan(&recoveryState); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(ctx, `SELECT state FROM issue_workflow_incident WHERE workspace_id=$1 AND issue_id=$2 AND regression_fingerprint IS NOT NULL`, workspaceID, issueID).Scan(&incidentState); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(ctx, `SELECT release_state FROM issue_current_picture WHERE workspace_id=$1 AND issue_id=$2`, workspaceID, issueID).Scan(&restoredProjection); err != nil {
		t.Fatal(err)
	}
	if recoveryState != "restored" || forwardHistoryState != "live" || incidentState != "resolved" || restoredProjection != "restored" {
		var projectionDocument []byte
		_ = conn.QueryRow(ctx, `SELECT document FROM issue_current_picture WHERE workspace_id=$1 AND issue_id=$2`, workspaceID, issueID).Scan(&projectionDocument)
		t.Fatalf("restoration recovery=%s forward_history=%s incident=%s projection=%s document=%s", recoveryState, forwardHistoryState, incidentState, restoredProjection, projectionDocument)
	}
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_incident WHERE workspace_id=$1 AND issue_id=$2 AND regression_fingerprint IS NOT NULL`, workspaceID, issueID).Scan(&incidentCount); err != nil || incidentCount != 1 {
		t.Fatalf("rollback regression dedupe incidents=%d err=%v", incidentCount, err)
	}
	applyCounts := map[string]int{}
	for _, actionType := range []string{"deploy_exact_release", "expose_exact_release", "verify_runtime_exact_release", "verify_business_exact_release", "rollback_exact_release"} {
		var effectKey string
		if err := conn.QueryRow(ctx, `SELECT effect_key FROM issue_workflow_action_receipt WHERE workspace_id=$1 AND issue_id=$2 AND action_type=$3`, workspaceID, issueID, actionType).Scan(&effectKey); err != nil {
			t.Fatal(err)
		}
		applyCounts[effectKey] = provider.ApplyCount(effectKey)
	}
	worker.Wake()
	worker.Wake()
	if err := worker.ReconcileOnce(ctx); err != nil {
		t.Fatalf("duplicate wake reconciliation: %v", err)
	}
	if err := worker.ReconcileOnce(ctx); err != nil {
		t.Fatalf("dropped wake poll reconciliation: %v", err)
	}
	for effectKey, count := range applyCounts {
		if provider.ApplyCount(effectKey) != count {
			t.Fatalf("recovery re-applied %s: got %d want %d", effectKey, provider.ApplyCount(effectKey), count)
		}
	}
	var settledDeployID pgtype.UUID
	var settledDeployRevision int64
	if err := conn.QueryRow(ctx, `SELECT id,revision FROM issue_workflow_action_receipt WHERE effect_key=$1`, deployEffectKey).Scan(&settledDeployID, &settledDeployRevision); err == nil {
		staleErr := workflowstore.InIssueWriteTransaction(ctx, conn, queries, workspaceID, issueID, func(write *workflowstore.IssueWrite) error {
			_, settleErr := write.SettleAction(ctx, generated.SettleIssueWorkflowActionParams{Now: workflowV2Time(clock), EffectResult: []byte(`{"late":true}`), WorkspaceID: workspaceID, IssueID: issueID, ID: settledDeployID, ExpectedRevision: settledDeployRevision, LeaseToken: workflowV2UUID(249)})
			return settleErr
		})
		if !errors.Is(staleErr, pgx.ErrNoRows) {
			t.Fatalf("stale token settled release action: %v", staleErr)
		}
	}

	// The provider target fence is exercised with two independently persisted
	// issue/action records, not merely two workers racing one receipt. Both
	// immutable snapshots propose generation two from the same exact predecessor;
	// exactly one provider CAS may win.
	var firstDeployKey string
	if err := conn.QueryRow(ctx, `SELECT action_key FROM issue_workflow_action_receipt WHERE id=$1`, settledDeployID).Scan(&firstDeployKey); err != nil {
		t.Fatal(err)
	}
	firstDeploy, err := queries.GetIssueWorkflowActionReceiptByKey(ctx, generated.GetIssueWorkflowActionReceiptByKeyParams{WorkspaceID: workspaceID, ActionKey: firstDeployKey})
	if err != nil {
		t.Fatal(err)
	}
	secondIssueID := workflowV2UUIDString("90000000-0000-0000-0000-000000000001")
	secondActionID := workflowV2UUIDString("90000000-0000-0000-0000-000000000002")
	secondCommandID := workflowV2UUIDString("90000000-0000-0000-0000-000000000003")
	secondReleaseID := workflowV2UUIDString("90000000-0000-0000-0000-000000000004")
	secondCandidateID := workflowV2UUIDString("90000000-0000-0000-0000-000000000005")
	if _, err := conn.Exec(ctx, `INSERT INTO issue(id,workspace_id) VALUES($1,$2)`, secondIssueID, workspaceID); err != nil {
		t.Fatal(err)
	}
	var secondPayload map[string]any
	if err := json.Unmarshal(firstDeploy.Payload, &secondPayload); err != nil {
		t.Fatal(err)
	}
	secondRequestHash := "second-complete-db-issue-release-request"
	secondPayload["command_hash"] = secondRequestHash
	secondPayload["release_id"] = util.UUIDToString(secondReleaseID)
	secondPayload["candidate_id"] = util.UUIDToString(secondCandidateID)
	encodedSecondPayload, err := json.Marshal(secondPayload)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO issue_workflow_command(id,workspace_id,issue_id,command_key,request_hash,command_type,actor_type,actor_id,scope_id,scope_revision,authority_id,authority_revision,candidate_id,target_key,expected_revisions,payload,state,result,processed_at) VALUES($1,$2,$3,'release/db-target-race',$4,'authorize_release','internal_controller',$5,$6,1,$7,1,$8,'production',$9,$10,'processed','{}',$11)`, secondCommandID, workspaceID, secondIssueID, secondRequestHash, controllerID, scopeID, authorityID, secondCandidateID, firstDeploy.Payload, encodedSecondPayload, clock); err != nil {
		t.Fatal(err)
	}
	secondActionKey := "workflow/second-complete-db-issue/deploy_exact_release"
	secondAction, err := queries.CreateIssueWorkflowActionReceipt(ctx, generated.CreateIssueWorkflowActionReceiptParams{
		ID: secondActionID, WorkspaceID: workspaceID, IssueID: secondIssueID, ActionKey: secondActionKey,
		RequestHash: secondRequestHash, CommandID: secondCommandID, ActionType: "deploy_exact_release",
		OperationID: firstDeploy.OperationID, ReleaseID: secondReleaseID,
		TargetKey: pgtype.Text{String: "production", Valid: true}, Payload: encodedSecondPayload,
		NotBefore: workflowV2Time(clock), MaxAttempts: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	secondEffectKey := secondAction.EffectKey.String
	firstDeployProviderKey := firstDeploy.EffectKey.String
	providerRace := workflowaction.NewFakeReleaseProvider()
	providerRace.SeedTarget(workspaceID, "production", util.UUIDToString(knownGoodReleaseID), 1)
	raced := make(chan error, 2)
	for _, action := range []generated.IssueWorkflowActionReceipt{firstDeploy, secondAction} {
		go func(action generated.IssueWorkflowActionReceipt) {
			_, applyErr := providerRace.Apply(ctx, action)
			raced <- applyErr
		}(action)
	}
	wins, conflicts := 0, 0
	for range 2 {
		switch err := <-raced; {
		case err == nil:
			wins++
		case errors.Is(err, workflowaction.ErrEffectMismatch):
			conflicts++
		default:
			t.Fatalf("two DB issue target race unexpected error: %v", err)
		}
	}
	if wins != 1 || conflicts != 1 || providerRace.ApplyCount(firstDeployProviderKey)+providerRace.ApplyCount(secondEffectKey) != 1 {
		t.Fatalf("two DB issue target race wins=%d conflicts=%d applies=%d", wins, conflicts, providerRace.ApplyCount(firstDeployProviderKey)+providerRace.ApplyCount(secondEffectKey))
	}
}

func assertReleaseActionPayload(t *testing.T, ctx context.Context, conn *pgx.Conn, workspaceID, issueID pgtype.UUID, commandKey, actionType string, candidateID, scopeID, authorityID, reviewID, operationID, operatorID, verifierID, predecessorReleaseID pgtype.UUID, predecessorGeneration, desiredGeneration, releaseRevision, expectedReleaseRevision, expectedEvidenceRevision int64) pgtype.UUID {
	t.Helper()
	var actionID, commandID pgtype.UUID
	var actionKey, actionHash, commandHash string
	var payload []byte
	if err := conn.QueryRow(ctx, `SELECT action.id,action.action_key,action.request_hash,action.payload,command.id,command.request_hash FROM issue_workflow_action_receipt action JOIN issue_workflow_command command ON command.id=action.command_id AND command.workspace_id=action.workspace_id AND command.issue_id=action.issue_id WHERE action.workspace_id=$1 AND action.issue_id=$2 AND action.action_type=$3 AND command.command_key=$4`, workspaceID, issueID, actionType, commandKey).Scan(&actionID, &actionKey, &actionHash, &payload, &commandID, &commandHash); err != nil {
		t.Fatal(err)
	}
	var frozen struct {
		ContractVersion          int              `json:"contract_version"`
		EffectContract           string           `json:"effect_contract"`
		CommandHash              string           `json:"command_hash"`
		CandidateID              string           `json:"candidate_id"`
		CandidateRevision        int64            `json:"candidate_revision"`
		ScopeID                  string           `json:"scope_id"`
		ScopeRevision            int64            `json:"scope_revision"`
		AcceptanceVersion        int64            `json:"acceptance_version"`
		ReviewID                 string           `json:"review_id"`
		ReviewRevision           int64            `json:"review_revision"`
		AuthorityID              string           `json:"authority_id"`
		AuthorityRevision        int64            `json:"authority_revision"`
		AuthorityCapability      string           `json:"authority_capability"`
		AuthorityEnvelopeDigest  string           `json:"authority_envelope_digest"`
		AuthorityEvidenceID      string           `json:"authority_evidence_id"`
		OwnerEvidenceRevision    int64            `json:"owner_evidence_revision"`
		OperationID              string           `json:"operation_id"`
		BranchKey                string           `json:"branch_key"`
		TargetKey                string           `json:"target_key"`
		ExpectedTargetReleaseID  string           `json:"expected_target_release_id"`
		ExpectedTargetGeneration int64            `json:"expected_target_generation"`
		DesiredTargetGeneration  int64            `json:"desired_target_generation"`
		ReleaseOperatorID        string           `json:"release_operator_id"`
		PlannedVerifierID        string           `json:"planned_verifier_id"`
		BuildManifestDigest      string           `json:"build_manifest_digest"`
		ArtifactDigest           string           `json:"artifact_digest"`
		RuntimeContractDigest    string           `json:"runtime_contract_digest"`
		ExposurePolicy           map[string]any   `json:"exposure_policy"`
		AudiencePolicy           map[string]any   `json:"audience_policy"`
		RiskEnvelope             map[string]any   `json:"risk_envelope"`
		Exclusions               []string         `json:"exclusions"`
		BudgetEnvelope           map[string]any   `json:"budget_envelope"`
		RecoveryPolicy           map[string]any   `json:"recovery_policy"`
		RecoveryPolicyRevision   int64            `json:"recovery_policy_revision"`
		RollbackTargetReleaseID  string           `json:"rollback_target_release_id"`
		IrreversibleEffects      []string         `json:"irreversible_effects"`
		ReleaseRevision          int64            `json:"release_revision"`
		Expected                 map[string]int64 `json:"expected"`
	}
	if err := json.Unmarshal(payload, &frozen); err != nil {
		t.Fatal(err)
	}
	expected := map[string]int64{"outcome": 1, "scope": 1, "authority": 1, "execution": 1, "review": 1, "release": expectedReleaseRevision, "attention": 1, "evidence": expectedEvidenceRevision, "budget": 1}
	wantActionID := uuid.NewSHA1(uuid.UUID(commandID.Bytes), []byte(actionType+"/"+util.UUIDToString(operationID)+"/"))
	wantActionKey := "workflow/" + util.UUIDToString(commandID) + "/" + actionType
	if actionID.Bytes != [16]byte(wantActionID) || actionKey != wantActionKey || actionHash == "" || actionHash != commandHash || frozen.ContractVersion != 1 || frozen.EffectContract != "observe_before_apply_v1" || frozen.CommandHash != commandHash || frozen.CandidateID != util.UUIDToString(candidateID) || frozen.CandidateRevision != 1 || frozen.ScopeID != util.UUIDToString(scopeID) || frozen.ScopeRevision != 1 || frozen.AcceptanceVersion != 1 || frozen.ReviewID != util.UUIDToString(reviewID) || frozen.ReviewRevision != 1 || frozen.AuthorityID != util.UUIDToString(authorityID) || frozen.AuthorityRevision != 1 || frozen.AuthorityCapability != "deploy_exact_release" || frozen.AuthorityEnvelopeDigest == "" || frozen.AuthorityEvidenceID == "" || frozen.OwnerEvidenceRevision != 1 || frozen.OperationID != util.UUIDToString(operationID) || frozen.BranchKey != "main" || frozen.TargetKey != "production" || frozen.ExpectedTargetReleaseID != util.UUIDToString(predecessorReleaseID) || frozen.ExpectedTargetGeneration != predecessorGeneration || frozen.DesiredTargetGeneration != desiredGeneration || frozen.ReleaseOperatorID != util.UUIDToString(operatorID) || frozen.PlannedVerifierID != util.UUIDToString(verifierID) || frozen.BuildManifestDigest != "sha256:manifest" || frozen.ArtifactDigest != "sha256:artifact" || frozen.RuntimeContractDigest != "sha256:runtime" || !reflect.DeepEqual(frozen.ExposurePolicy, map[string]any{"mode": "canary", "percent": float64(10)}) || !reflect.DeepEqual(frozen.AudiencePolicy, map[string]any{"segment": "internal"}) || !reflect.DeepEqual(frozen.RiskEnvelope, map[string]any{"level": "bounded"}) || !reflect.DeepEqual(frozen.Exclusions, []string{}) || !reflect.DeepEqual(frozen.BudgetEnvelope, map[string]any{"attempts": float64(1)}) || !reflect.DeepEqual(frozen.RecoveryPolicy, map[string]any{"mode": "rollback"}) || frozen.RecoveryPolicyRevision != 1 || frozen.RollbackTargetReleaseID != util.UUIDToString(predecessorReleaseID) || !reflect.DeepEqual(frozen.IrreversibleEffects, []string{}) || frozen.ReleaseRevision != releaseRevision || !reflect.DeepEqual(frozen.Expected, expected) {
		t.Fatalf("incomplete frozen release action payload: %s", payload)
	}
	return actionID
}

func releaseSnapshotExpected(release, evidence workflowstatev2.Revision) []workflowstatev2.ExpectedRevision {
	return []workflowstatev2.ExpectedRevision{
		{Concern: workflowstatev2.ConcernScope, Revision: 1},
		{Concern: workflowstatev2.ConcernAuthority, Revision: 1},
		{Concern: workflowstatev2.ConcernExecution, Revision: 1},
		{Concern: workflowstatev2.ConcernReview, Revision: 1},
		{Concern: workflowstatev2.ConcernRelease, Revision: release},
		{Concern: workflowstatev2.ConcernEvidence, Revision: evidence},
	}
}

func setupWorkflowV2Database(t *testing.T) (context.Context, *pgx.Conn, *generated.Queries) {
	t.Helper()
	databaseURL := os.Getenv(workflowV2TestDatabaseEnv)
	if databaseURL == "" {
		t.Skipf("integration test requires an isolated database at %s", workflowV2TestDatabaseEnv)
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to isolated workflow v2 test database: %v", err)
	}
	schema := "workflow_v2_contract_test"
	identifier := pgx.Identifier{schema}.Sanitize()
	_, _ = conn.Exec(ctx, "DROP SCHEMA IF EXISTS "+identifier+" CASCADE")
	if _, err := conn.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `SELECT set_config('search_path', $1, false)`, schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = conn.Exec(context.Background(), "SET search_path TO public")
		_, _ = conn.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+identifier+" CASCADE")
		_ = conn.Close(context.Background())
	})

	if _, err := conn.Exec(ctx, `
		CREATE TABLE workspace (id UUID PRIMARY KEY);
		CREATE TABLE issue (id UUID PRIMARY KEY, workspace_id UUID NOT NULL);
		CREATE TABLE member (id UUID PRIMARY KEY, workspace_id UUID NOT NULL, user_id UUID NOT NULL, role TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now());
		CREATE TABLE issue_vcs_pull_request (issue_id UUID NOT NULL);
		CREATE TABLE issue_workflow_event (workspace_id UUID NOT NULL, issue_id UUID NOT NULL);
		CREATE TABLE issue_workflow_plan (workspace_id UUID NOT NULL, issue_id UUID NOT NULL);
		CREATE TABLE squad (id UUID PRIMARY KEY, workspace_id UUID, archived_at TIMESTAMPTZ);
		CREATE TABLE agent_runtime (id UUID PRIMARY KEY, workspace_id UUID NOT NULL, daemon_id TEXT);
		CREATE TABLE agent (id UUID PRIMARY KEY, workspace_id UUID, runtime_id UUID, archived_at TIMESTAMPTZ);
		CREATE TABLE agent_task_queue (
			id UUID PRIMARY KEY, agent_id UUID NOT NULL, runtime_id UUID, issue_id UUID NOT NULL,
			status TEXT NOT NULL, priority INTEGER NOT NULL DEFAULT 0, context JSONB,
			result JSONB, completed_at TIMESTAMPTZ,
			originator_source TEXT, trigger_evidence_kind TEXT, trigger_evidence_ref_id UUID
		);
		CREATE TABLE squad_member (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(), squad_id UUID NOT NULL,
			member_type TEXT NOT NULL, member_id UUID NOT NULL,
			UNIQUE(squad_id, member_type, member_id)
		);
		CREATE TABLE squad_workflow_template (
			squad_id UUID PRIMARY KEY CONSTRAINT squad_workflow_template_squad_id_fkey REFERENCES squad(id) ON DELETE CASCADE,
			workspace_id UUID NOT NULL CONSTRAINT squad_workflow_template_workspace_id_fkey REFERENCES workspace(id) ON DELETE CASCADE,
			implementer_id UUID NOT NULL CONSTRAINT squad_workflow_template_implementer_id_fkey REFERENCES agent(id),
			reviewer_id UUID NOT NULL CONSTRAINT squad_workflow_template_reviewer_id_fkey REFERENCES agent(id),
			release_agent_id UUID CONSTRAINT squad_workflow_template_release_agent_id_fkey REFERENCES agent(id),
			verifier_id UUID CONSTRAINT squad_workflow_template_verifier_id_fkey REFERENCES agent(id),
			requires_owner BOOLEAN NOT NULL DEFAULT FALSE,
			requires_deployment BOOLEAN NOT NULL DEFAULT FALSE,
			max_corrections INTEGER NOT NULL DEFAULT 1,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE INDEX squad_workflow_template_workspace_idx ON squad_workflow_template(workspace_id);
	`); err != nil {
		t.Fatalf("create migration 465 fixture: %v", err)
	}
	applyMigrationFile(t, ctx, conn, "284_task_owner_row_fence.up.sql")
	applyMigrationFile(t, ctx, conn, "466_issue_workflow_v2_records.up.sql")
	paths, err := filepath.Glob(filepath.Join(realMigrationsDir(t), "*.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	for _, path := range paths {
		base := filepath.Base(path)
		if base < "467_" || base > "521_zzzz" {
			continue
		}
		applyMigrationFile(t, ctx, conn, base)
	}
	// The checkpoint ledger extends the original workflow-v2 schema. Apply it
	// explicitly without dragging unrelated later product migrations into this
	// deliberately small contract fixture.
	applyMigrationFile(t, ctx, conn, "636_issue_workflow_execution_checkpoint_snapshot.up.sql")
	applyMigrationFile(t, ctx, conn, "637_issue_workflow_execution_checkpoint_snapshot_identity_index.up.sql")
	applyMigrationFile(t, ctx, conn, "638_issue_workflow_execution_checkpoint_snapshot_task_index.up.sql")
	// Continuation receipts participate in the generated issue/workspace cleanup
	// queries exercised by this fixture, so the deliberately small schema must
	// include their table and task lineage columns as well.
	applyMigrationFile(t, ctx, conn, "641_agent_task_continuation.up.sql")
	applyMigrationFile(t, ctx, conn, "642_agent_task_continuation_identity_index.up.sql")
	return ctx, conn, generated.New(conn)
}

func workflowV2TestConnection(ctx context.Context) (*pgx.Conn, error) {
	conn, err := pgx.Connect(ctx, os.Getenv(workflowV2TestDatabaseEnv))
	if err != nil {
		return nil, err
	}
	if _, err := conn.Exec(ctx, `SELECT set_config('search_path', $1, false)`, "workflow_v2_contract_test"); err != nil {
		_ = conn.Close(ctx)
		return nil, err
	}
	return conn, nil
}

type blockingCommitStarter struct {
	conn          *pgx.Conn
	commitEntered chan struct{}
	releaseCommit chan struct{}
	releaseOnce   sync.Once
	blockQuery    bool
	queryEntered  chan struct{}
	releaseQuery  chan struct{}
	queryRelease  sync.Once
	tx            *blockingCommitTx
}

func newBlockingCommitStarter(conn *pgx.Conn) *blockingCommitStarter {
	return &blockingCommitStarter{
		conn:          conn,
		commitEntered: make(chan struct{}),
		releaseCommit: make(chan struct{}),
	}
}

func (starter *blockingCommitStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := starter.conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	starter.tx = &blockingCommitTx{
		Tx:            tx,
		commitEntered: starter.commitEntered,
		releaseCommit: starter.releaseCommit,
		blockQuery:    starter.blockQuery,
		queryEntered:  starter.queryEntered,
		releaseQuery:  starter.releaseQuery,
	}
	return starter.tx, nil
}

func (starter *blockingCommitStarter) release() {
	starter.releaseOnce.Do(func() { close(starter.releaseCommit) })
}

func (starter *blockingCommitStarter) releaseQueryCall() {
	starter.queryRelease.Do(func() {
		if starter.releaseQuery != nil {
			close(starter.releaseQuery)
		}
	})
}

type blockingCommitTx struct {
	pgx.Tx
	commitEntered     chan struct{}
	releaseCommit     chan struct{}
	touches           atomic.Int64
	operationInFlight atomic.Int64
	blockQuery        bool
	queryEntered      chan struct{}
	releaseQuery      chan struct{}
	queryOnce         sync.Once
	capability        *workflowstore.IssueWrite
	clone             *workflowstore.IssueWrite
}

func (tx *blockingCommitTx) Commit(ctx context.Context) error {
	close(tx.commitEntered)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-tx.releaseCommit:
		return tx.Tx.Commit(ctx)
	}
}

func (tx *blockingCommitTx) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	tx.touches.Add(1)
	tx.operationInFlight.Add(1)
	defer tx.operationInFlight.Add(-1)
	return tx.Tx.Exec(ctx, sql, arguments...)
}

func (tx *blockingCommitTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	tx.touches.Add(1)
	tx.operationInFlight.Add(1)
	defer tx.operationInFlight.Add(-1)
	if tx.blockQuery {
		tx.queryOnce.Do(func() {
			close(tx.queryEntered)
			<-tx.releaseQuery
		})
	}
	return tx.Tx.Query(ctx, sql, args...)
}

func TestNonLiveFinalizerDatabaseContracts(t *testing.T) {
	ctx, conn, queries := setupWorkflowV2Database(t)
	t.Run("non-live finalizer is exactly fenced and crash recoverable", func(t *testing.T) {
		t.Run("exact leased action completes and settles", func(t *testing.T) {
			fx := seedNonLiveFinalizerFixture(t, ctx, conn, queries, 40)
			begun := claimAndBeginNonLiveFinalizer(t, ctx, queries, fx, fx.Now)
			receipt, err := fx.Service.Execute(ctx, finalizerPrincipal(fx, begun), fx.Request)
			if err != nil {
				t.Fatal(err)
			}
			if len(receipt.ActionIDs) != 0 {
				t.Fatalf("completion admitted unexpected actions: %v", receipt.ActionIDs)
			}
			result, _ := json.Marshal(receipt)
			if _, err := queries.SettleIssueWorkflowAction(ctx, generated.SettleIssueWorkflowActionParams{Now: workflowV2Time(fx.Now), EffectResult: result, WorkspaceID: fx.WorkspaceID, IssueID: fx.IssueID, ID: fx.ActionID, ExpectedRevision: begun.Revision, LeaseToken: begun.LeaseToken}); err != nil {
				t.Fatal(err)
			}
			assertNonLiveFinalizerState(t, ctx, conn, fx, "settled", 1)
		})

		mutations := []struct {
			name   string
			mutate func(*nonLiveFinalizerFixture, *generated.IssueWorkflowActionReceipt)
		}{
			{"action key", func(fx *nonLiveFinalizerFixture, _ *generated.IssueWorkflowActionReceipt) {
				fx.PrincipalActionKey += "/wrong"
			}},
			{"action id", func(fx *nonLiveFinalizerFixture, _ *generated.IssueWorkflowActionReceipt) {
				fx.PrincipalActionID = workflowV2UUID(99)
			}},
			{"operation", func(fx *nonLiveFinalizerFixture, _ *generated.IssueWorkflowActionReceipt) {
				fx.Request.OperationID = util.UUIDToString(workflowV2UUID(98))
			}},
			{"candidate", func(fx *nonLiveFinalizerFixture, begun *generated.IssueWorkflowActionReceipt) {
				mutateFinalizerPayload(t, ctx, conn, begun.ID, "candidate_id", util.UUIDToString(workflowV2UUID(97)))
			}},
			{"scope revision", func(fx *nonLiveFinalizerFixture, begun *generated.IssueWorkflowActionReceipt) {
				mutateFinalizerPayload(t, ctx, conn, begun.ID, "scope_revision", 2)
			}},
			{"acceptance", func(fx *nonLiveFinalizerFixture, begun *generated.IssueWorkflowActionReceipt) {
				mutateFinalizerPayload(t, ctx, conn, begun.ID, "acceptance_version", 2)
			}},
			{"source plan revision", func(fx *nonLiveFinalizerFixture, begun *generated.IssueWorkflowActionReceipt) {
				mutateFinalizerPayload(t, ctx, conn, begun.ID, "source_plan_revision", 2)
			}},
			{"deployment policy", func(fx *nonLiveFinalizerFixture, begun *generated.IssueWorkflowActionReceipt) {
				mutateFinalizerPayload(t, ctx, conn, begun.ID, "requires_deployment", true)
			}},
			{"wrong lease", func(fx *nonLiveFinalizerFixture, _ *generated.IssueWorkflowActionReceipt) {
				fx.PrincipalLease = workflowV2UUID(96)
			}},
			{"expired lease", func(fx *nonLiveFinalizerFixture, begun *generated.IssueWorkflowActionReceipt) {
				if _, err := conn.Exec(ctx, `UPDATE issue_workflow_action_receipt SET lease_expires_at=$2 WHERE id=$1`, begun.ID, fx.Now.Add(-time.Second)); err != nil {
					t.Fatal(err)
				}
			}},
		}
		for _, concern := range []string{"scope", "review", "evidence", "outcome", "execution", "release"} {
			concern := concern
			mutations = append(mutations, struct {
				name   string
				mutate func(*nonLiveFinalizerFixture, *generated.IssueWorkflowActionReceipt)
			}{"expected " + concern, func(fx *nonLiveFinalizerFixture, begun *generated.IssueWorkflowActionReceipt) {
				mutateFinalizerExpected(t, ctx, conn, begun.ID, concern, 2)
			}})
		}
		for _, tc := range mutations {
			t.Run(tc.name+" fails with zero residue", func(t *testing.T) {
				fx := seedNonLiveFinalizerFixture(t, ctx, conn, queries, 40)
				begun := claimAndBeginNonLiveFinalizer(t, ctx, queries, fx, fx.Now)
				fx.PrincipalActionID, fx.PrincipalActionKey, fx.PrincipalLease = begun.ID, begun.ActionKey, begun.LeaseToken
				tc.mutate(&fx, &begun)
				if _, err := fx.Service.Execute(ctx, finalizerPrincipal(fx, begun), fx.Request); err == nil {
					t.Fatal("mutated finalizer authority was accepted")
				}
				assertNonLiveFinalizerState(t, ctx, conn, fx, "leased", 0)
			})
		}

		t.Run("post-commit crash reclaims and replays once", func(t *testing.T) {
			fx := seedNonLiveFinalizerFixture(t, ctx, conn, queries, 40)
			begun := claimAndBeginNonLiveFinalizer(t, ctx, queries, fx, fx.Now)
			first, err := fx.Service.Execute(ctx, finalizerPrincipal(fx, begun), fx.Request)
			if err != nil {
				t.Fatal(err)
			}
			later := fx.Now.Add(2 * time.Hour)
			if _, err := conn.Exec(ctx, `UPDATE issue_workflow_action_receipt SET lease_expires_at=$2 WHERE id=$1`, fx.ActionID, later.Add(-time.Second)); err != nil {
				t.Fatal(err)
			}
			worker := workflowaction.Worker{
				Starter: conn, Queries: queries,
				Effect: workflowaction.EffectFuncs{
					ObserveFunc: func(context.Context, generated.IssueWorkflowActionReceipt) (workflowaction.Observation, error) {
						return workflowaction.Observation{}, nil
					},
					ApplyFunc: func(context.Context, generated.IssueWorkflowActionReceipt) (workflowaction.Observation, error) {
						return workflowaction.Observation{}, nil
					},
				},
				Capacity:       workflowaction.CapacityFunc(func(context.Context, generated.IssueWorkflowActionReceipt) (bool, error) { return true, nil }),
				CommandService: workflowcommand.Service{Starter: conn, Queries: queries, Loader: workflowcommand.Loader{Now: func() time.Time { return later }}, Now: func() time.Time { return later }, ControllerID: fx.ControllerID},
				ControllerID:   fx.ControllerID, Now: func() time.Time { return later }, LeaseDuration: time.Minute,
			}
			if err := worker.ReconcileOnce(ctx); err != nil {
				t.Fatal(err)
			}
			var stored []byte
			if err := conn.QueryRow(ctx, `SELECT effect_result FROM issue_workflow_action_receipt WHERE id=$1`, fx.ActionID).Scan(&stored); err != nil {
				t.Fatal(err)
			}
			var replayed workflowcommand.Receipt
			if json.Unmarshal(stored, &replayed) != nil || !reflect.DeepEqual(first, replayed) {
				t.Fatalf("replay receipt differs: first=%+v replay=%+v raw=%s", first, replayed, stored)
			}
			assertNonLiveFinalizerState(t, ctx, conn, fx, "settled", 1)
		})

		for _, stage := range []string{"after_authority", "after_actions", "after_domain", "after_heads", "after_projection", "after_receipt"} {
			stage := stage
			t.Run("rollback at "+stage, func(t *testing.T) {
				fx := seedNonLiveFinalizerFixture(t, ctx, conn, queries, 40)
				begun := claimAndBeginNonLiveFinalizer(t, ctx, queries, fx, fx.Now)
				fx.Service.Failpoint = func(got string) error {
					if got == stage {
						return errors.New("forced finalizer rollback")
					}
					return nil
				}
				if _, err := fx.Service.Execute(ctx, finalizerPrincipal(fx, begun), fx.Request); err == nil || !strings.Contains(err.Error(), "forced finalizer rollback") {
					t.Fatalf("failpoint error=%v", err)
				}
				assertNonLiveFinalizerState(t, ctx, conn, fx, "leased", 0)
			})
		}
	})
}

func (tx *blockingCommitTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	tx.touches.Add(1)
	return tx.Tx.QueryRow(ctx, sql, args...)
}

func writeWorkflowTemplateWithLocks(ctx context.Context, conn *pgx.Conn, workspaceID, squadID, implementerID, reviewerID pgtype.UUID) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := generated.New(tx)
	if _, err := q.LockWorkflowOwnerWorkspace(ctx, workspaceID); err != nil {
		return err
	}
	if _, err := q.LockSquadWorkflowTemplateSquad(ctx, generated.LockSquadWorkflowTemplateSquadParams{SquadID: squadID, WorkspaceID: workspaceID}); err != nil {
		return err
	}
	locked, err := q.LockSquadWorkflowTemplateAgents(ctx, generated.LockSquadWorkflowTemplateAgentsParams{WorkspaceID: workspaceID, SquadID: squadID, AgentIds: []pgtype.UUID{implementerID, reviewerID}})
	if err != nil {
		return err
	}
	if len(locked) != 2 {
		return pgx.ErrNoRows
	}
	if _, err := q.UpsertSquadWorkflowTemplate(ctx, generated.UpsertSquadWorkflowTemplateParams{
		SquadID: squadID, WorkspaceID: workspaceID, ImplementerID: implementerID,
		ReviewerID: reviewerID, RequiresOwner: false, RequiresDeployment: false, MaxCorrections: 1,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func seedAllWorkflowV2Records(t *testing.T, ctx context.Context, conn *pgx.Conn, workspaceID, issueID pgtype.UUID, idBase byte) {
	t.Helper()
	id := workflowV2UUID(idBase)
	statements := []string{
		`INSERT INTO agent_task_context_manifest (id,workspace_id,issue_id,task_id,task_attempt,provider_attempt,schema_version,measurement_state,aggregate_limit_bytes,aggregate_bytes,blocks,manifest_sha256) VALUES ($1,$2,$3,$1,1,1,1,'exact',1,0,'[]',repeat('a',64))`,
		`INSERT INTO agent_task_usage_sample (id,workspace_id,issue_id,task_id,task_attempt,provider_attempt,backend_attempt,sequence,provider,model,input_tokens,output_tokens,cache_read_tokens,cache_write_tokens,provider_calls,source_event,model_context_window_kind,context_tokens_measurement_kind,shadow_decision,sample_sha256) VALUES ($1,$2,$3,$1,1,1,1,1,'test','test',0,0,0,0,1,'test','unknown','unknown','unknown',repeat('a',64))`,
		`INSERT INTO agent_task_loop_observation (id,workspace_id,issue_id,task_id,task_attempt,provider_attempt,schema_version,provider,tool,input_sha256,output_sha256,result_status,repeated_cycles,shadow_decision,fingerprint_sha256,observation_sha256) VALUES ($1,$2,$3,$1,1,1,1,'test','test',repeat('a',64),repeat('b',64),'ok',3,'would_checkpoint',repeat('c',64),repeat('d',64))`,
		`INSERT INTO agent_task_continuation_receipt (id,workspace_id,issue_id,root_task_id,source_task_id,successor_task_id,source_step,next_step,max_packages,checkpoint,checkpoint_sha256,execution_id,execution_revision) VALUES ($1,$2,$3,$1,$1,$1,1,2,3,'{}'::jsonb,repeat('a',64),$1,1)`,
		`INSERT INTO issue_workflow_outcome (id,workspace_id,issue_id,state,acceptance_version) VALUES ($1,$2,$3,'open',1)`,
		`INSERT INTO issue_workflow_scope (id,workspace_id,issue_id,revision,acceptance_version,state,owner_evidence_id,created_by,statement) VALUES ($1,$2,$3,1,1,'accepted',$1,$1,'{}')`,
		`INSERT INTO issue_workflow_authority (id,workspace_id,issue_id,revision,scope_id,scope_revision,owner_evidence_id,state,envelope,created_by) VALUES ($1,$2,$3,1,$1,1,$1,'active','{}',$1)`,
		`INSERT INTO issue_workflow_execution (id,workspace_id,issue_id,branch_key,operation_type,state,scope_id,scope_revision) VALUES ($1,$2,$3,'main','implementation','received',$1,1)`,
		`INSERT INTO issue_workflow_execution_checkpoint_snapshot (id,workspace_id,issue_id,execution_id,execution_revision,scope_id,scope_revision,branch_key,execution_state,checkpoint,snapshot_document,chain_anchor,snapshot_sha256) VALUES ($1,$2,$3,$1,1,$1,1,'main','received','{}','{}',true,repeat('a',64))`,
		`INSERT INTO issue_workflow_decision (id,workspace_id,issue_id,scope_id,scope_revision,decision_type,state,resolution_state,question,requester_id,accountable_owner_id,suspended_operation_id,suspended_execution_state) VALUES ($1,$2,$3,$1,1,'approval','unanswered','open','Approve?',$1,$1,$1,'working')`,
		`INSERT INTO issue_workflow_decision_response (id,workspace_id,issue_id,decision_id,decision_revision,response_type,actor_id,idempotency_key,request_hash) VALUES ($1,$2,$3,$1,1,'approve',$1,'response-key','response-hash')`,
		`INSERT INTO issue_workflow_candidate (id,workspace_id,issue_id,scope_id,scope_revision,candidate_type,state,artifact,created_by) VALUES ($1,$2,$3,$1,1,'source','ready','{}',$1)`,
		`INSERT INTO issue_workflow_review (id,workspace_id,issue_id,candidate_id,candidate_revision,scope_id,scope_revision,reviewer_id,outcome) VALUES ($1,$2,$3,$1,1,$1,1,$1,'passed')`,
		`INSERT INTO issue_workflow_release (id,workspace_id,issue_id,candidate_id,candidate_revision,scope_id,scope_revision,authority_id,authority_revision,target_key,target_generation,state) VALUES ($1,$2,$3,$1,1,$1,1,$1,1,'production',1,'pending')`,
		`INSERT INTO issue_workflow_evidence (id,workspace_id,issue_id,concern_type,evidence_type,level,source,producer_type,observed_at,payload) VALUES ($1,$2,$3,'outcome','business','runtime','test','verifier',now(),'{}')`,
		`INSERT INTO issue_workflow_incident (id,workspace_id,issue_id,incident_type,severity,state) VALUES ($1,$2,$3,'regression','urgent','open')`,
		`INSERT INTO issue_workflow_command (id,workspace_id,issue_id,command_key,request_hash,command_type,actor_type) VALUES ($1,$2,$3,'seed-command','seed-command-hash','queue','controller')`,
		`INSERT INTO issue_workflow_action_receipt (id,workspace_id,issue_id,action_key,request_hash,command_id,action_type) VALUES ($1,$2,$3,'seed-action','seed-action-hash',$1,'dispatch')`,
		`INSERT INTO issue_workflow_budget (id,workspace_id,issue_id,scope_id,concern_type,limits) VALUES ($1,$2,$3,$1,'execution','{}')`,
		`INSERT INTO issue_workflow_attention (id,workspace_id,issue_id,attention_type,read_state,resolution_state) VALUES ($1,$2,$3,'decision','unread','open')`,
		`INSERT INTO issue_current_picture (workspace_id,issue_id,contract_version,revision,outcome_state,outcome_open,execution_state,release_state,health_state,attention_state,source_revisions,document,generated_at) VALUES ($2,$3,2,1,'open',true,'received','pending','unknown','open',jsonb_build_object('seed',$1::text),'{}',now())`,
		`INSERT INTO issue_workflow_target_head (workspace_id,target_key,generation,release_id,issue_id,candidate_id,state) VALUES ($2,'production',1,$1,$3,$1,'pending')`,
		`INSERT INTO issue_workflow_concern_head (workspace_id,issue_id,concern_type,revision,source_record_id,source_record_revision,state) VALUES ($2,$3,'outcome',1,$1,1,'open')`,
	}
	for _, statement := range statements {
		if _, err := conn.Exec(ctx, statement, id, workspaceID, issueID); err != nil {
			t.Fatalf("seed workflow v2 record with %q: %v", statement, err)
		}
	}
}

type workflowV2Execer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func workflowV2ReleaseSemanticSnapshot(t *testing.T, ctx context.Context, conn *pgx.Conn, workspaceID, issueID, releaseID pgtype.UUID) []byte {
	t.Helper()
	var snapshot []byte
	if err := conn.QueryRow(ctx, `
		SELECT jsonb_build_object(
			'release', (SELECT jsonb_build_object('state',state,'revision',revision) FROM issue_workflow_release WHERE id=$3),
			'releases', (SELECT jsonb_agg(jsonb_build_object('id',id,'candidate_id',candidate_id,'state',state,'revision',revision,'generation',target_generation,'kind',release_kind,'recovery_of',recovery_of_release_id) ORDER BY id) FROM issue_workflow_release WHERE workspace_id=$1 AND issue_id=$2),
			'candidates', (SELECT jsonb_agg(jsonb_build_object('id',id,'revision',revision,'state',state,'source_revision',source_revision,'superseded_at',superseded_at) ORDER BY id) FROM issue_workflow_candidate WHERE workspace_id=$1 AND issue_id=$2),
			'incidents', (SELECT jsonb_agg(jsonb_build_object('id',id,'state',state,'revision',revision,'release_id',release_id,'recovery_release_id',recovery_release_id,'fingerprint',regression_fingerprint) ORDER BY id) FROM issue_workflow_incident WHERE workspace_id=$1 AND issue_id=$2),
			'recovery_actions', (SELECT jsonb_agg(jsonb_build_object('id',id,'type',action_type,'state',state,'revision',revision,'bound_task_id',bound_task_id) ORDER BY id) FROM issue_workflow_action_receipt WHERE workspace_id=$1 AND issue_id=$2 AND action_type IN ('rollback_exact_release','verify_restoration_exact_release')),
			'outcome', (SELECT jsonb_build_object('state',state,'revision',revision) FROM issue_workflow_outcome WHERE workspace_id=$1 AND issue_id=$2 ORDER BY revision DESC LIMIT 1),
			'execution', (SELECT jsonb_build_object('state',state,'revision',revision) FROM issue_workflow_execution WHERE workspace_id=$1 AND issue_id=$2 ORDER BY revision DESC LIMIT 1),
			'heads', (SELECT jsonb_object_agg(concern_type,jsonb_build_object('revision',revision,'source_record_id',source_record_id,'source_record_revision',source_record_revision,'state',state) ORDER BY concern_type) FROM issue_workflow_concern_head WHERE workspace_id=$1 AND issue_id=$2),
			'evidence', (SELECT jsonb_agg(jsonb_build_object('id',id,'type',evidence_type,'concern_id',concern_id,'concern_revision',concern_revision,'superseded_at',superseded_at) ORDER BY id) FROM issue_workflow_evidence WHERE workspace_id=$1 AND issue_id=$2),
			'commands', (SELECT jsonb_agg(jsonb_build_object('id',id,'key',command_key,'state',state) ORDER BY id) FROM issue_workflow_command WHERE workspace_id=$1 AND issue_id=$2),
			'picture', (SELECT jsonb_build_object('revision',revision,'document',document) FROM issue_current_picture WHERE workspace_id=$1 AND issue_id=$2),
			'target', (SELECT jsonb_build_object('generation',generation,'release_id',release_id,'issue_id',issue_id,'candidate_id',candidate_id,'state',state) FROM issue_workflow_target_head WHERE workspace_id=$1 AND target_key='production')
		)`, workspaceID, issueID, releaseID).Scan(&snapshot); err != nil {
		t.Fatalf("capture semantic release snapshot: %v", err)
	}
	return snapshot
}

func seedLoadableWorkflowV2(t *testing.T, ctx context.Context, conn workflowV2Execer, base byte) (pgtype.UUID, pgtype.UUID) {
	t.Helper()
	workspaceID, issueID := workflowV2UUID(base), workflowV2UUID(base+1)
	ids := map[string]pgtype.UUID{
		"outcome": workflowV2UUID(base + 2), "scope": workflowV2UUID(base + 3), "authority": workflowV2UUID(base + 4),
		"execution": workflowV2UUID(base + 5), "review": workflowV2UUID(base + 6), "release": workflowV2UUID(base + 7),
		"attention": workflowV2UUID(base + 8), "evidence": workflowV2UUID(base + 9), "budget": workflowV2UUID(base + 10),
		"candidate": workflowV2UUID(base + 11), "owner": workflowV2UUID(base + 12), "implementer": workflowV2UUID(base + 13),
		"reviewer": workflowV2UUID(base + 14), "operator": workflowV2UUID(base + 15), "verifier": workflowV2UUID(base + 16),
		"review_evidence": workflowV2UUID(base + 17),
	}
	statements := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO workspace(id) VALUES($1)`, []any{workspaceID}},
		{`INSERT INTO issue(id,workspace_id) VALUES($1,$2)`, []any{issueID, workspaceID}},
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
	return workspaceID, issueID
}

func authorizationDecisionRequest(workspaceID, issueID pgtype.UUID, key string, operationID, candidateID, ownerID pgtype.UUID) workflowcommand.Request {
	return workflowcommand.Request{
		WorkspaceID: workspaceID, IssueID: issueID, CommandKey: key, Kind: workflowstatev2.CommandRequestDecision,
		OperationID: util.UUIDToString(operationID), CandidateID: util.UUIDToString(candidateID),
		Expected: []workflowstatev2.ExpectedRevision{{Concern: workflowstatev2.ConcernExecution, Revision: 1}, {Concern: workflowstatev2.ConcernAttention, Revision: 1}, {Concern: workflowstatev2.ConcernEvidence, Revision: 1}},
		Decision: &workflowcommand.DecisionInput{Type: workflowstatev2.DecisionAuthorization, Question: "Proceed with this exact operation?", Recommendation: "Approve only if the proposed operation is authorized", Consequence: "Work remains paused until the continuation receipt settles", AccountableOwnerID: ownerID, AuthorityGap: workflowstatev2.AuthorityGap{Kind: workflowstatev2.AuthorityGapOperationAuthorization, RequestedCapability: "execute proposed operation", Reason: "owner authorization required"}},
	}
}

func prepareWorkflowCorrectionFixture(t *testing.T, ctx context.Context, conn workflowV2Execer, workspaceID, issueID pgtype.UUID, base byte) {
	t.Helper()
	for _, statement := range []struct {
		sql  string
		args []any
	}{
		{`UPDATE issue_workflow_execution SET state='correction' WHERE workspace_id=$1 AND issue_id=$2 AND id=$3`, []any{workspaceID, issueID, workflowV2UUID(base + 5)}},
		{`UPDATE issue_workflow_review SET outcome='failed' WHERE workspace_id=$1 AND issue_id=$2 AND id=$3`, []any{workspaceID, issueID, workflowV2UUID(base + 6)}},
		{`UPDATE issue_workflow_concern_head SET state='correction' WHERE workspace_id=$1 AND issue_id=$2 AND concern_type='execution'`, []any{workspaceID, issueID}},
		{`UPDATE issue_workflow_concern_head SET state='failed' WHERE workspace_id=$1 AND issue_id=$2 AND concern_type='review'`, []any{workspaceID, issueID}},
	} {
		if _, err := conn.Exec(ctx, statement.sql, statement.args...); err != nil {
			t.Fatalf("prepare correction fixture: %v", err)
		}
	}
}

func workflowCorrectionActionPayload(t *testing.T, base byte, operationID, workerID pgtype.UUID) []byte {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"operation_id":         util.UUIDToString(operationID),
		"branch_key":           "main",
		"candidate_id":         util.UUIDToString(workflowV2UUID(base + 11)),
		"candidate_revision":   int64(1),
		"scope_id":             util.UUIDToString(workflowV2UUID(base + 3)),
		"scope_revision":       int64(1),
		"acceptance_version":   int64(1),
		"review_id":            util.UUIDToString(workflowV2UUID(base + 6)),
		"review_revision":      int64(1),
		"correction_worker_id": util.UUIDToString(workerID),
	})
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func lockWorkflowCorrectionTaskOwnersInOrder(ctx context.Context, conn *pgx.Conn, workspaceID, agentID, issueID, runtimeID pgtype.UUID) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := generated.New(tx)
	if _, err := q.LockWorkflowCorrectionTaskWorkspace(ctx, workspaceID); err != nil {
		return err
	}
	if _, err := q.LockWorkflowCorrectionTaskAgent(ctx, generated.LockWorkflowCorrectionTaskAgentParams{AgentID: agentID, WorkspaceID: workspaceID, RuntimeID: runtimeID}); err != nil {
		return err
	}
	if _, err := q.LockWorkflowCorrectionTaskIssue(ctx, generated.LockWorkflowCorrectionTaskIssueParams{IssueID: issueID, WorkspaceID: workspaceID}); err != nil {
		return err
	}
	if _, err := q.LockWorkflowCorrectionTaskRuntime(ctx, generated.LockWorkflowCorrectionTaskRuntimeParams{RuntimeID: runtimeID, WorkspaceID: workspaceID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func assertWorkflowV2RecordCounts(t *testing.T, ctx context.Context, conn *pgx.Conn, workspaceID, issueID pgtype.UUID, want int) {
	t.Helper()
	for _, table := range []string{
		"agent_task_context_manifest", "agent_task_usage_sample", "agent_task_loop_observation", "agent_task_continuation_receipt",
		"issue_workflow_outcome", "issue_workflow_scope", "issue_workflow_authority",
		"issue_workflow_execution", "issue_workflow_execution_checkpoint_snapshot", "issue_workflow_decision", "issue_workflow_decision_response",
		"issue_workflow_candidate", "issue_workflow_review", "issue_workflow_release",
		"issue_workflow_evidence", "issue_workflow_incident", "issue_workflow_command",
		"issue_workflow_action_receipt", "issue_workflow_budget", "issue_workflow_attention",
		"issue_current_picture", "issue_workflow_target_head", "issue_workflow_concern_head",
	} {
		var count int
		query := fmt.Sprintf("SELECT count(*) FROM %s WHERE workspace_id = $1 AND issue_id = $2", table)
		if err := conn.QueryRow(ctx, query, workspaceID, issueID).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != want {
			t.Errorf("%s count = %d, want %d", table, count, want)
		}
	}
}

func workflowV2CurrentPictureParams(workspaceID, issueID pgtype.UUID, revision, expected int64) generated.UpsertIssueCurrentPictureParams {
	emptyObject := []byte(`{}`)
	emptyArray := []byte(`[]`)
	return generated.UpsertIssueCurrentPictureParams{
		WorkspaceID: workspaceID, IssueID: issueID, ContractVersion: 2,
		Revision: revision, OutcomeState: "open", OutcomeOpen: true,
		ExecutionState: "received", ReleaseState: "pending", HealthState: "unknown",
		AttentionState: "resolved", CurrentMilestone: "received",
		LatestMeaningfulChange: emptyObject, NextAutomaticAction: emptyObject,
		ActiveActor: emptyObject, WaitReason: emptyObject, Candidate: emptyObject,
		Target: emptyObject, EvidenceSummary: emptyObject, AuthoritySummary: emptyObject,
		UnresolvedDecisions: emptyArray, UnresolvedIncidents: emptyArray,
		UnresolvedDependencies: emptyArray, BudgetSummary: emptyObject,
		SupersessionSummary: emptyObject, NextActor: emptyObject,
		SourceRevisions: []byte(`{"outcome":1}`), Document: []byte(`{"outcome":"open"}`),
		GeneratedAt: workflowV2Time(time.Unix(1_000, 0).UTC()), ExpectedRevision: expected,
	}
}

func workflowV2UUID(last byte) pgtype.UUID {
	var bytes [16]byte
	bytes[15] = last
	return pgtype.UUID{Bytes: bytes, Valid: true}
}

func workflowV2Time(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

func TestWorkflowV2StateVocabularyMatchesPureContract(t *testing.T) {
	body := readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "..", "pkg", "db", "queries", "workflow_v2.sql"))
	for _, forbidden := range []string{"'done'", "state IN ('open', 'answered', 'resuming')"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("workflow v2 persistence uses legacy state vocabulary %s", forbidden)
		}
	}
	for _, required := range []string{"'completed'", "'unanswered'", "resolution_state = 'resuming'"} {
		if !strings.Contains(body, required) {
			t.Errorf("workflow v2 persistence is missing contract state %s", required)
		}
	}
}
