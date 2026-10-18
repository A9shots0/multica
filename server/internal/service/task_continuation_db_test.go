package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	googleuuid "github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/continuitycheckpoint"
	"github.com/multica-ai/multica/server/internal/events"
	internalflags "github.com/multica-ai/multica/server/internal/featureflags"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/internal/workflowstore"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/featureflag"
)

type continuationFixture struct {
	pool        *pgxpool.Pool
	queries     *db.Queries
	service     *TaskService
	workspaceID pgtype.UUID
	issueID     pgtype.UUID
	taskID      pgtype.UUID
	executionID pgtype.UUID
	daemonID    string
}

func newContinuationFixture(t *testing.T) continuationFixture {
	return newContinuationFixtureWithEnrollment(t, true)
}

func newContinuationFixtureWithEnrollment(t *testing.T, enroll bool) continuationFixture {
	t.Helper()
	pool := newResolveOriginatorPool(t)
	ctx := context.Background()
	bootstrap := testutil.New(pool, "", "")
	suffix := fmt.Sprintf("continuation-%d", time.Now().UnixNano())
	userID := bootstrap.User(t, suffix, suffix+"@example.invalid")
	workspaceID := bootstrap.Workspace(t, suffix, suffix)
	fx := testutil.New(pool, workspaceID, userID)
	fx.Member(t, workspaceID, userID, "owner")
	daemonID := "daemon-" + suffix
	runtimeID := fx.Runtime(t, suffix, testutil.Cols{"daemon_id": daemonID, "runtime_mode": "local"})
	agentID := fx.Agent(t, suffix, runtimeID)
	projectID := fx.Project(t, suffix)
	workDir := "/tmp/multica-continuity-durable"
	fx.Insert(t, "project_resource", testutil.Cols{
		"project_id": projectID, "workspace_id": workspaceID, "resource_type": "local_directory",
		"resource_ref": testutil.Raw(fmt.Sprintf(`'{"local_path":%q,"daemon_id":%q,"execution_mode":"in_place"}'::jsonb`, workDir, daemonID)),
		"position":     0, "created_by": userID,
	})
	issueID := fx.Issue(t, suffix, testutil.Cols{"project_id": projectID})
	taskID := googleuuid.NewString()
	fx.Task(t, agentID, testutil.Cols{
		"id": taskID, "runtime_id": runtimeID, "issue_id": issueID,
		"status": "queued",
	})

	queries := db.New(pool)
	workspaceUUID, issueUUID := util.MustParseUUID(workspaceID), util.MustParseUUID(issueID)
	executionID := util.MustParseUUID(googleuuid.NewString())
	create := db.CreateIssueWorkflowExecutionParams{
		ID: executionID, WorkspaceID: workspaceUUID, IssueID: issueUUID,
		BranchKey: "continuous-work", OperationType: "implementation", State: "running",
		ScopeID: util.MustParseUUID(googleuuid.NewString()), ScopeRevision: 1,
		ActorType: pgtype.Text{String: "agent", Valid: true}, ActorID: util.MustParseUUID(agentID),
		TaskID: util.MustParseUUID(taskID), Checkpoint: []byte(`{"id":"workflow-checkpoint","suspended_state":"paused","suspended_operation_id":"operation-1"}`), ExpectedEvent: []byte(`{}`),
		WaitReason: []byte(`{}`), AttemptCount: 1,
	}
	if err := workflowstore.InIssueWriteTransaction(ctx, pool, queries, workspaceUUID, issueUUID, func(write *workflowstore.IssueWrite) error {
		emptyObject, emptyArray := []byte(`{}`), []byte(`[]`)
		if _, err := write.UpsertCurrentPictureCAS(ctx, db.UpsertIssueCurrentPictureParams{
			WorkspaceID: workspaceUUID, IssueID: issueUUID, ContractVersion: 2, Revision: 1,
			ScopeRevision: pgtype.Int8{Int64: 1, Valid: true}, OutcomeState: "open", OutcomeOpen: true,
			ExecutionState: "running", ReleaseState: "pending", HealthState: "unknown", AttentionState: "resolved",
			CurrentMilestone: "implementation", LatestMeaningfulChange: emptyObject, NextAutomaticAction: emptyObject,
			ActiveActor: emptyObject, WaitReason: emptyObject, Candidate: emptyObject, Target: emptyObject,
			EvidenceSummary: emptyObject, AuthoritySummary: emptyObject, UnresolvedDecisions: emptyArray,
			UnresolvedIncidents: emptyArray, UnresolvedDependencies: emptyArray, BudgetSummary: emptyObject,
			SupersessionSummary: emptyObject, NextActor: emptyObject, SourceRevisions: []byte(`{"scope":1}`),
			Document: []byte(`{"outcome":"open"}`), GeneratedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			ExpectedRevision: 0,
		}); err != nil {
			return err
		}
		_, err := write.CreateExecution(ctx, create)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	service := NewTaskService(queries, pool, nil, events.New())
	if enroll {
		enrolled, err := service.EnableTaskContinuation(ctx, util.MustParseUUID(taskID), 3)
		if err != nil {
			t.Fatalf("explicitly enroll continuation task: %v", err)
		}
		if enrolled.ContinuationRootTaskID != enrolled.ID || enrolled.ContinuationStep.Int32 != 1 || enrolled.ContinuationMaxPackages.Int32 != 3 {
			t.Fatalf("unexpected enrolled lineage: %+v", enrolled)
		}
		if _, err := pool.Exec(ctx, `UPDATE agent_task_queue SET status='running', started_at=now() WHERE id=$1 AND status='queued'`, enrolled.ID); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM agent_task_continuation_receipt WHERE workspace_id=$1`, workspaceUUID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM issue_workflow_execution_checkpoint_snapshot WHERE workspace_id=$1`, workspaceUUID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM issue_workflow_execution WHERE workspace_id=$1`, workspaceUUID)
	})
	return continuationFixture{
		pool: pool, queries: queries, service: service,
		workspaceID: workspaceUUID, issueID: issueUUID, taskID: util.MustParseUUID(taskID), executionID: executionID, daemonID: daemonID,
	}
}

func TestContinuationFeatureFlagEnrollsExactEligibleClaim(t *testing.T) {
	fx := newContinuationFixtureWithEnrollment(t, false)
	provider := featureflag.NewStaticProvider()
	provider.Set(internalflags.ContinuousWorkCheckpointV1, featureflag.Rule{Default: true})
	fx.service.FeatureFlags = featureflag.NewService(provider)

	claimed, err := fx.service.ClaimTask(context.Background(), mustTaskAgentID(t, fx))
	if err != nil {
		t.Fatalf("claim eligible continuous task: %v", err)
	}
	if claimed == nil || claimed.ContinuationRootTaskID != claimed.ID || claimed.ContinuationStep.Int32 != 1 || claimed.ContinuationMaxPackages.Int32 != 3 {
		t.Fatalf("feature-flag enrollment = %+v", claimed)
	}
}

func TestContinuationFeatureFlagCanTargetOneExactTask(t *testing.T) {
	fx := newContinuationFixtureWithEnrollment(t, false)
	provider := featureflag.NewStaticProvider()
	exactTaskRule := featureflag.Rule{
		Allow:   []string{util.UUIDToString(fx.taskID)},
		AllowBy: "task_id",
	}
	provider.Set(internalflags.ContinuousWorkCheckpointV1, exactTaskRule)
	provider.Set(internalflags.ContinuousWorkObservationV1, exactTaskRule)
	fx.service.FeatureFlags = featureflag.NewService(provider)

	task, err := fx.queries.GetAgentTask(context.Background(), fx.taskID)
	if err != nil {
		t.Fatal(err)
	}
	enabled, err := fx.service.continuityRolloutEnabled(context.Background(), fx.queries, task, internalflags.ContinuousWorkCheckpointV1)
	if err != nil || !enabled {
		t.Fatalf("exact task rollout = %v, %v; want enabled", enabled, err)
	}
	if enabled, err = fx.service.continuityRolloutEnabled(context.Background(), fx.queries, task, internalflags.ContinuousWorkObservationV1); err != nil || !enabled {
		t.Fatalf("exact task observation rollout = %v, %v; want enabled", enabled, err)
	}

	sibling := task
	sibling.ID = util.MustParseUUID(googleuuid.NewString())
	enabled, err = fx.service.continuityRolloutEnabled(context.Background(), fx.queries, sibling, internalflags.ContinuousWorkCheckpointV1)
	if err != nil || enabled {
		t.Fatalf("sibling task rollout = %v, %v; want disabled", enabled, err)
	}
	if enabled, err = fx.service.continuityRolloutEnabled(context.Background(), fx.queries, sibling, internalflags.ContinuousWorkObservationV1); err != nil || enabled {
		t.Fatalf("sibling task observation rollout = %v, %v; want disabled", enabled, err)
	}

	claimed, err := fx.service.ClaimTask(context.Background(), task.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	if claimed == nil || claimed.ID != task.ID || !claimed.ContinuationRootTaskID.Valid || !claimed.ContinuityObservationEnabled {
		t.Fatalf("exact targeted claim did not enroll both controls: %+v", claimed)
	}
}

func TestContinuationFeatureFlagRejectsAmbiguousExecutionBeforeProviderClaim(t *testing.T) {
	fx := newContinuationFixtureWithEnrollment(t, false)
	ctx := context.Background()
	original, err := fx.queries.GetIssueWorkflowExecution(ctx, db.GetIssueWorkflowExecutionParams{WorkspaceID: fx.workspaceID, IssueID: fx.issueID, ID: fx.executionID})
	if err != nil {
		t.Fatal(err)
	}
	if err := workflowstore.InIssueWriteTransaction(ctx, fx.pool, fx.queries, fx.workspaceID, fx.issueID, func(write *workflowstore.IssueWrite) error {
		_, err := write.CreateExecution(ctx, db.CreateIssueWorkflowExecutionParams{
			ID: util.MustParseUUID(googleuuid.NewString()), WorkspaceID: fx.workspaceID, IssueID: fx.issueID,
			BranchKey: "ambiguous-enrollment", OperationType: "implementation", State: "running",
			ScopeID: util.MustParseUUID(googleuuid.NewString()), ScopeRevision: original.ScopeRevision,
			ActorType: original.ActorType, ActorID: original.ActorID, TaskID: fx.taskID,
			Checkpoint: []byte(`{}`), ExpectedEvent: []byte(`{}`), WaitReason: []byte(`{}`), AttemptCount: 1,
		})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	provider := featureflag.NewStaticProvider()
	provider.Set(internalflags.ContinuousWorkCheckpointV1, featureflag.Rule{Default: true})
	fx.service.FeatureFlags = featureflag.NewService(provider)

	if claimed, err := fx.service.ClaimTask(ctx, mustTaskAgentID(t, fx)); !errors.Is(err, ErrContinuationNotAdmitted) || claimed != nil {
		t.Fatalf("ambiguous feature-flag claim = (%+v, %v)", claimed, err)
	}
	task, err := fx.queries.GetAgentTask(ctx, fx.taskID)
	if err != nil || task.Status != "queued" || task.ContinuationRootTaskID.Valid {
		t.Fatalf("ambiguous task crossed provider boundary: %+v err=%v", task, err)
	}
}

func mustTaskAgentID(t *testing.T, fx continuationFixture) pgtype.UUID {
	t.Helper()
	task, err := fx.queries.GetAgentTask(context.Background(), fx.taskID)
	if err != nil {
		t.Fatal(err)
	}
	return task.AgentID
}

func validatedContinuation(t *testing.T, step int32, summary string) continuitycheckpoint.Validated {
	t.Helper()
	raw, err := json.Marshal(continuitycheckpoint.Receipt{
		SchemaVersion: 1, PackageStep: step, Summary: summary,
		Completed: []string{"bounded package completed"}, NextAction: "continue with a fresh worker",
		Verification: []string{"database contract checked"}, OpenRisks: []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	validated, err := continuitycheckpoint.DecodeAndValidate(raw, step)
	if err != nil {
		t.Fatal(err)
	}
	return validated
}

func TestCompleteTaskWithContinuationCheckpointIsAtomicAndIdempotent(t *testing.T) {
	fx := newContinuationFixture(t)
	ctx := context.Background()
	checkpoint := validatedContinuation(t, 1, "package one complete")

	const workers = 2
	start := make(chan struct{})
	results := make(chan *ContinuationCompletion, workers)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			result, err := fx.service.CompleteTaskWithContinuationCheckpoint(ctx, fx.taskID, []byte(`{"output":"internal checkpoint"}`), checkpoint, "/tmp/multica-continuity-durable", "", "", "", fx.daemonID)
			if err != nil {
				errs <- err
				return
			}
			results <- result
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent completion: %v", err)
	}
	var successorID pgtype.UUID
	resultCount := 0
	for result := range results {
		resultCount++
		if !successorID.Valid {
			successorID = result.Successor.ID
		} else if successorID != result.Successor.ID {
			t.Fatalf("forked successors: %s != %s", util.UUIDToString(successorID), util.UUIDToString(result.Successor.ID))
		}
		if result.Source.Status != "completed" || result.Successor.Status != "queued" || !result.Successor.ForceFreshSession {
			t.Fatalf("unexpected result: %+v", result)
		}
	}
	if resultCount != workers {
		t.Fatalf("results=%d want=%d", resultCount, workers)
	}

	successor, err := fx.queries.GetAgentTask(ctx, successorID)
	if err != nil {
		t.Fatal(err)
	}
	if successor.SessionID.Valid || successor.ContinuationOfTaskID != fx.taskID || successor.ContinuationStep.Int32 != 2 || successor.Attempt != 1 {
		t.Fatalf("successor continuity identity = %+v", successor)
	}
	if !successor.WorkDir.Valid || successor.WorkDir.String != "/tmp/multica-continuity-durable" {
		t.Fatalf("successor prior workdir = %+v", successor.WorkDir)
	}
	receipt, err := fx.queries.GetAgentTaskContinuationReceiptBySource(ctx, fx.taskID)
	var storedReceiptCheckpoint, expectedCheckpoint any
	_ = json.Unmarshal(receipt.Checkpoint, &storedReceiptCheckpoint)
	_ = json.Unmarshal(checkpoint.Canonical, &expectedCheckpoint)
	if err != nil || receipt.CheckpointSha256 != checkpoint.SHA256 || receipt.SuccessorTaskID != successor.ID || !reflect.DeepEqual(storedReceiptCheckpoint, expectedCheckpoint) {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	execution, err := fx.queries.GetIssueWorkflowExecution(ctx, db.GetIssueWorkflowExecutionParams{WorkspaceID: fx.workspaceID, IssueID: fx.issueID, ID: fx.executionID})
	var storedCheckpoint, originalCheckpoint any
	_ = json.Unmarshal(execution.Checkpoint, &storedCheckpoint)
	_ = json.Unmarshal([]byte(`{"id":"workflow-checkpoint","suspended_state":"paused","suspended_operation_id":"operation-1"}`), &originalCheckpoint)
	if err != nil || execution.Revision != 2 || !reflect.DeepEqual(storedCheckpoint, originalCheckpoint) {
		t.Fatalf("execution=%+v err=%v", execution, err)
	}
	if _, err := fx.pool.Exec(ctx, `UPDATE agent_task_queue SET status='running', started_at=now() WHERE id=$1 AND status='queued'`, successor.ID); err != nil {
		t.Fatal(err)
	}
	stepTwo, err := fx.service.CompleteTaskWithContinuationCheckpoint(ctx, successor.ID, []byte(`{}`), validatedContinuation(t, 2, "package two complete"), "/tmp/multica-continuity-durable", "", "", "", fx.daemonID)
	if err != nil {
		t.Fatalf("step two checkpoint: %v", err)
	}
	if stepTwo.Successor.ContinuationStep.Int32 != 3 || stepTwo.Successor.ContinuationOfTaskID != successor.ID || !stepTwo.Successor.ForceFreshSession {
		t.Fatalf("step three identity=%+v", stepTwo.Successor)
	}
	if plan, err := ContinuityCheckpointPlan(stepTwo.Successor); err != nil || plan != nil {
		t.Fatalf("terminal package offered another checkpoint: plan=%+v err=%v", plan, err)
	}
	execution, err = fx.queries.GetIssueWorkflowExecution(ctx, db.GetIssueWorkflowExecutionParams{WorkspaceID: fx.workspaceID, IssueID: fx.issueID, ID: fx.executionID})
	if err != nil || execution.Revision != 3 || execution.TaskID != successor.ID {
		t.Fatalf("second handoff execution=%+v err=%v", execution, err)
	}

	mismatch := validatedContinuation(t, 1, "different replay")
	if _, err := fx.service.CompleteTaskWithContinuationCheckpoint(ctx, fx.taskID, nil, mismatch, "/tmp/multica-continuity-durable", "", "", "", fx.daemonID); !errors.Is(err, ErrContinuationReceiptMismatch) {
		t.Fatalf("mismatched replay error=%v", err)
	}
}

func TestCompleteTaskWithContinuationCheckpointRollsBackOnAmbiguousExecution(t *testing.T) {
	fx := newContinuationFixture(t)
	ctx := context.Background()
	original, err := fx.queries.GetIssueWorkflowExecution(ctx, db.GetIssueWorkflowExecutionParams{WorkspaceID: fx.workspaceID, IssueID: fx.issueID, ID: fx.executionID})
	if err != nil {
		t.Fatal(err)
	}
	duplicate := db.CreateIssueWorkflowExecutionParams{
		ID: util.MustParseUUID(googleuuid.NewString()), WorkspaceID: fx.workspaceID, IssueID: fx.issueID,
		BranchKey: "ambiguous", OperationType: "implementation", State: "running",
		ScopeID: util.MustParseUUID(googleuuid.NewString()), ScopeRevision: 1,
		ActorType: original.ActorType, ActorID: original.ActorID, TaskID: fx.taskID,
		Checkpoint: []byte(`{}`), ExpectedEvent: []byte(`{}`), WaitReason: []byte(`{}`), AttemptCount: 1,
	}
	if err := workflowstore.InIssueWriteTransaction(ctx, fx.pool, fx.queries, fx.workspaceID, fx.issueID, func(write *workflowstore.IssueWrite) error {
		_, err := write.CreateExecution(ctx, duplicate)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.pool.Exec(ctx, `UPDATE agent_task_queue SET status='dispatched' WHERE id=$1`, fx.taskID); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.service.StartTask(ctx, fx.taskID); !errors.Is(err, ErrContinuationNotAdmitted) {
		t.Fatalf("ambiguous start error=%v", err)
	}
	afterRejectedStart, err := fx.queries.GetAgentTask(ctx, fx.taskID)
	if err != nil || afterRejectedStart.Status != "dispatched" {
		t.Fatalf("ambiguous start was not rolled back: %+v err=%v", afterRejectedStart, err)
	}
	if _, err := fx.pool.Exec(ctx, `UPDATE agent_task_queue SET status='running' WHERE id=$1`, fx.taskID); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.service.CompleteTaskWithContinuationCheckpoint(ctx, fx.taskID, nil, validatedContinuation(t, 1, "ambiguous"), "/tmp/multica-continuity-durable", "", "", "", fx.daemonID); !errors.Is(err, ErrContinuationNotAdmitted) {
		t.Fatalf("ambiguous execution error=%v", err)
	}
	source, err := fx.queries.GetAgentTask(ctx, fx.taskID)
	if err != nil || source.Status != "running" {
		t.Fatalf("source changed after rollback: %+v err=%v", source, err)
	}
}

func TestContinuationRetryKeepsFreshStepAndVerifiedCapsule(t *testing.T) {
	for _, reason := range []string{"provider_network", "codex_semantic_inactivity"} {
		t.Run(reason, func(t *testing.T) {
			fx := newContinuationFixture(t)
			ctx := context.Background()
			first, err := fx.service.CompleteTaskWithContinuationCheckpoint(ctx, fx.taskID, []byte(`{}`), validatedContinuation(t, 1, "package one complete"), "/tmp/multica-continuity-durable", "", "", "", fx.daemonID)
			if err != nil {
				t.Fatal(err)
			}
			stepTwo := first.Successor
			if _, err := fx.pool.Exec(ctx, `UPDATE agent_task_queue SET status='failed', completed_at=now(), failure_reason=$2, session_id=NULL WHERE id=$1`, stepTwo.ID, reason); err != nil {
				t.Fatal(err)
			}
			retry, err := fx.queries.CreateRetryTask(ctx, db.CreateRetryTaskParams{ID: stepTwo.ID, NewTaskID: util.MustParseUUID(googleuuid.NewString())})
			if err != nil {
				t.Fatal(err)
			}
			if retry.ContinuationRootTaskID != fx.taskID || retry.ContinuationStep.Int32 != 2 || retry.ContinuationMaxPackages.Int32 != 3 {
				t.Fatalf("retry lost package identity: %+v", retry)
			}
			if !retry.ForceFreshSession || retry.SessionID.Valid || !retry.WorkDir.Valid || retry.WorkDir.String != "/tmp/multica-continuity-durable" {
				t.Fatalf("retry lost fresh-session workspace contract: %+v", retry)
			}
			if err := fx.service.ValidateContinuationClaim(ctx, fx.queries, retry); err != nil {
				t.Fatalf("retry failed continuation admission: %v", err)
			}
			candidates, err := fx.queries.ListFreshWorkflowResumeSnapshotCandidates(ctx, retry.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(candidates) != 1 || candidates[0].TaskID != fx.taskID || candidates[0].ContinuationSourceStep != 1 {
				t.Fatalf("continuation retry lost exact-lineage checkpoint: %+v", candidates)
			}
		})
	}
}

func TestContinuationNearLimitReceiptRoundTripsThroughJSONB(t *testing.T) {
	fx := newContinuationFixture(t)
	receipt := continuitycheckpoint.Receipt{
		SchemaVersion: 1, PackageStep: 1,
		Summary:      strings.Repeat("s", continuitycheckpoint.MaxSummaryRunes),
		Completed:    []string{strings.Repeat("c", continuitycheckpoint.MaxItemRunes)},
		NextAction:   strings.Repeat("n", continuitycheckpoint.MaxNextActionRunes),
		Verification: []string{strings.Repeat("v", continuitycheckpoint.MaxItemRunes)},
		OpenRisks:    []string{strings.Repeat("r", continuitycheckpoint.MaxItemRunes)},
	}
	var checkpoint continuitycheckpoint.Validated
	for padding := continuitycheckpoint.MaxItemRunes; padding > 0; padding-- {
		receipt.OpenRisks = []string{strings.Repeat("r", continuitycheckpoint.MaxItemRunes), strings.Repeat("p", padding)}
		raw, err := json.Marshal(receipt)
		if err != nil {
			t.Fatal(err)
		}
		candidate, err := continuitycheckpoint.DecodeAndValidate(raw, 1)
		if err == nil && len(candidate.Canonical) >= 2000 {
			checkpoint = candidate
			break
		}
	}
	if len(checkpoint.Canonical) < 2000 {
		t.Fatalf("failed to construct near-limit receipt: %d bytes", len(checkpoint.Canonical))
	}
	if _, err := fx.service.CompleteTaskWithContinuationCheckpoint(context.Background(), fx.taskID, []byte(`{}`), checkpoint, "/tmp/multica-continuity-durable", "", "", "", fx.daemonID); err != nil {
		t.Fatalf("near-limit receipt did not round-trip: %v", err)
	}
	stored, err := fx.queries.GetAgentTaskContinuationReceiptBySource(context.Background(), fx.taskID)
	if err != nil {
		t.Fatal(err)
	}
	validated, err := continuitycheckpoint.DecodeAndValidate(stored.Checkpoint, 1)
	if err != nil || validated.SHA256 != checkpoint.SHA256 {
		t.Fatalf("stored near-limit receipt changed: digest=%q err=%v", validated.SHA256, err)
	}
}

func TestContinuationRejectsWrongDaemonAndUnprovenWorkdir(t *testing.T) {
	fx := newContinuationFixture(t)
	ctx := context.Background()
	checkpoint := validatedContinuation(t, 1, "safe boundary")
	if _, err := fx.service.CompleteTaskWithContinuationCheckpoint(ctx, fx.taskID, nil, checkpoint, "/tmp/multica-continuity-durable", "", "", "", "wrong-daemon"); !errors.Is(err, ErrWorkflowTaskRuntimeForbidden) {
		t.Fatalf("wrong daemon error=%v", err)
	}
	if _, err := fx.pool.Exec(ctx, `UPDATE project_resource SET resource_ref=jsonb_set(resource_ref, '{execution_mode}', '"worktree"') WHERE project_id=(SELECT project_id FROM issue WHERE id=$1)`, fx.issueID); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.service.CompleteTaskWithContinuationCheckpoint(ctx, fx.taskID, nil, checkpoint, "/tmp/multica-continuity-durable", "", "", "", fx.daemonID); !errors.Is(err, ErrContinuationUnsafeWorkdir) {
		t.Fatalf("worktree mode error=%v", err)
	}
	source, err := fx.queries.GetAgentTask(ctx, fx.taskID)
	if err != nil || source.Status != "running" {
		t.Fatalf("rejected task changed: %+v err=%v", source, err)
	}
}
