package workflowaction

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/agentqualification"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// ReviewTaskEffect is the local deterministic effect for a review_candidate
// receipt. It shares the correction effect's established owner lock order and
// task insert/bind primitives, but has its own exact normalized specification.
type ReviewTaskEffect LocalTaskEffect

func (effect ReviewTaskEffect) Observe(ctx context.Context, action db.IssueWorkflowActionReceipt) (Observation, error) {
	spec, err := effect.Queries.GetWorkflowReviewTaskSpec(ctx, db.GetWorkflowReviewTaskSpecParams{ID: action.ID, WorkspaceID: action.WorkspaceID, IssueID: action.IssueID})
	if err != nil {
		return Observation{Kind: ObservationUnknown}, fmt.Errorf("%w: load review task specification: %v", ErrEffectUnknown, err)
	}
	taskID := reviewTaskID(action.ID)
	task, err := effect.Queries.GetWorkflowCorrectionTaskEffect(ctx, db.GetWorkflowCorrectionTaskEffectParams{ID: taskID, WorkflowActionID: action.ID})
	if err != nil {
		if err == pgx.ErrNoRows {
			return Observation{Kind: ObservationAbsent, TaskID: taskID}, nil
		}
		return Observation{Kind: ObservationUnknown}, fmt.Errorf("%w: observe review task: %v", ErrEffectUnknown, err)
	}
	expected := expectedReviewTaskContext(action, spec)
	if len(expected) == 0 {
		return Observation{Kind: ObservationUnknown}, ErrEffectMismatch
	}
	if _, err := agentqualification.ValidateGuardedTaskContextForExecutionV1(expected, nil, guardedExpectedIdentity(action, spec.AgentID, spec.RuntimeID), time.Now().UTC()); err != nil {
		return Observation{Kind: ObservationUnknown}, fmt.Errorf("guarded review task observation: %w", err)
	}
	if task.ID != taskID || task.WorkflowActionID != action.ID || task.WorkflowOperationID != action.OperationID || task.AgentID != spec.AgentID || task.RuntimeID != spec.RuntimeID || task.IssueID != action.IssueID || !task.WorkflowBranchKey.Valid || task.WorkflowBranchKey.String != spec.BranchKey || !reviewObservedStatus(task.Status) || !jsonSemanticEqual(task.Context, expected) {
		return Observation{Kind: ObservationUnknown}, ErrEffectMismatch
	}
	result, _ := json.Marshal(map[string]string{"task_id": util.UUIDToString(task.ID)})
	return Observation{Kind: ObservationFound, TaskID: task.ID, ReceiptRevision: action.Revision, Result: result}, nil
}

func (effect ReviewTaskEffect) Apply(ctx context.Context, action db.IssueWorkflowActionReceipt) (Observation, error) {
	if effect.Now == nil {
		effect.Now = time.Now
	}
	spec, err := effect.Queries.GetWorkflowReviewTaskSpec(ctx, db.GetWorkflowReviewTaskSpecParams{ID: action.ID, WorkspaceID: action.WorkspaceID, IssueID: action.IssueID})
	if err != nil {
		return Observation{Kind: ObservationUnknown}, fmt.Errorf("%w: load review task specification: %v", ErrEffectUnknown, err)
	}
	contextJSON := expectedReviewTaskContext(action, spec)
	if len(contextJSON) == 0 {
		return Observation{Kind: ObservationUnknown}, ErrEffectMismatch
	}
	if _, err := agentqualification.ValidateGuardedTaskContextForExecutionV1(contextJSON, nil, guardedExpectedIdentity(action, spec.AgentID, spec.RuntimeID), effect.Now().UTC()); err != nil {
		return Observation{Kind: ObservationUnknown}, fmt.Errorf("guarded review task admission: %w", err)
	}
	tx, err := effect.Starter.Begin(ctx)
	if err != nil {
		return Observation{Kind: ObservationUnknown}, err
	}
	defer tx.Rollback(ctx)
	q := effect.Queries.WithTx(tx)
	if _, err = q.LockWorkflowCorrectionTaskWorkspace(ctx, action.WorkspaceID); err != nil {
		return Observation{Kind: ObservationUnknown}, err
	}
	if _, err = q.LockWorkflowCorrectionTaskAgent(ctx, db.LockWorkflowCorrectionTaskAgentParams{AgentID: spec.AgentID, WorkspaceID: action.WorkspaceID, RuntimeID: spec.RuntimeID}); err != nil {
		return Observation{Kind: ObservationUnknown}, err
	}
	if _, err = q.LockWorkflowCorrectionTaskIssue(ctx, db.LockWorkflowCorrectionTaskIssueParams{IssueID: action.IssueID, WorkspaceID: action.WorkspaceID}); err != nil {
		return Observation{Kind: ObservationUnknown}, err
	}
	if _, err = q.LockWorkflowCorrectionTaskRuntime(ctx, db.LockWorkflowCorrectionTaskRuntimeParams{RuntimeID: spec.RuntimeID, WorkspaceID: action.WorkspaceID}); err != nil {
		return Observation{Kind: ObservationUnknown}, err
	}
	locked, err := q.LockWorkflowReviewTaskEffect(ctx, db.LockWorkflowReviewTaskEffectParams{ID: action.ID, WorkspaceID: action.WorkspaceID, IssueID: action.IssueID, ExpectedRevision: action.Revision, LeaseToken: action.LeaseToken, Now: pgTime(effect.Now())})
	if err != nil || locked.AgentID != spec.AgentID || locked.RuntimeID != spec.RuntimeID || locked.OperationID != action.OperationID || locked.BranchKey != spec.BranchKey {
		return Observation{Kind: ObservationUnknown}, ErrEffectMismatch
	}
	taskID := reviewTaskID(action.ID)
	task, err := q.CreateWorkflowCorrectionTask(ctx, db.CreateWorkflowCorrectionTaskParams{ID: taskID, AgentID: spec.AgentID, RuntimeID: spec.RuntimeID, IssueID: action.IssueID, Priority: 0, Context: contextJSON, WorkflowActionID: action.ID, WorkflowOperationID: action.OperationID, WorkflowBranchKey: pgtype.Text{String: spec.BranchKey, Valid: true}})
	if err != nil || task.Status != "queued" || !jsonSemanticEqual(task.Context, contextJSON) {
		return Observation{Kind: ObservationUnknown}, ErrEffectMismatch
	}
	bound, err := q.BindWorkflowActionTaskExact(ctx, db.BindWorkflowActionTaskExactParams{TaskID: task.ID, Now: pgTime(effect.Now()), Context: contextJSON, WorkspaceID: action.WorkspaceID, IssueID: action.IssueID, ID: action.ID, ExpectedRevision: action.Revision, LeaseToken: action.LeaseToken, WorkflowBranchKey: pgtype.Text{String: spec.BranchKey, Valid: true}, AgentID: spec.AgentID, RuntimeID: spec.RuntimeID})
	if err != nil {
		return Observation{Kind: ObservationUnknown}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Observation{Kind: ObservationUnknown}, err
	}
	if effect.Wake != nil {
		_ = effect.Wake(ctx, task.ID)
	}
	result, _ := json.Marshal(map[string]string{"task_id": util.UUIDToString(task.ID)})
	return Observation{Kind: ObservationFound, TaskID: task.ID, ReceiptRevision: bound.Revision, Result: result}, nil
}

func expectedReviewTaskContext(action db.IssueWorkflowActionReceipt, spec db.GetWorkflowReviewTaskSpecRow) []byte {
	var payload map[string]any
	if json.Unmarshal(action.Payload, &payload) != nil {
		return nil
	}
	payload["workflow_action_id"] = util.UUIDToString(action.ID)
	payload["effect_key"] = action.EffectKey.String
	payload["task_kind"] = "independent_review"
	delete(payload, agentqualification.GuardedDispatchTaskContextKey)
	value, err := normalizeGuardedTaskContextV1(action.Payload, payload, guardedExpectedIdentity(action, spec.AgentID, spec.RuntimeID))
	if err != nil {
		return nil
	}
	return value
}

func reviewObservedStatus(status string) bool {
	switch status {
	case "queued", "dispatched", "waiting_local_directory", "running", "deferred", "completed", "failed", "cancelled":
		return true
	default:
		return false
	}
}

func jsonSemanticEqual(left, right []byte) bool {
	var a, b any
	return json.Unmarshal(left, &a) == nil && json.Unmarshal(right, &b) == nil && reflect.DeepEqual(a, b)
}

func reviewTaskID(actionID pgtype.UUID) pgtype.UUID {
	value := uuid.NewSHA1(uuid.UUID(actionID.Bytes), []byte("review_candidate/task"))
	return pgtype.UUID{Bytes: value, Valid: actionID.Valid}
}
