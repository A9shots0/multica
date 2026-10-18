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
	"github.com/multica-ai/multica/server/internal/workflowguard"
	"github.com/multica-ai/multica/server/internal/workflowstore"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type LocalTaskEffect struct {
	Starter   workflowstore.TxStarter
	Queries   *db.Queries
	Now       func() time.Time
	Wake      WakeFunc
	Failpoint func(string) error
}

func (effect LocalTaskEffect) Observe(ctx context.Context, action db.IssueWorkflowActionReceipt) (Observation, error) {
	if action.ActionType == "review_candidate" {
		return (ReviewTaskEffect(effect)).Observe(ctx, action)
	}
	if action.ActionType != "review_correction" {
		return Observation{Kind: ObservationUnknown}, ErrEffectMismatch
	}
	spec, err := effect.Queries.GetWorkflowCorrectionTaskSpec(ctx, db.GetWorkflowCorrectionTaskSpecParams{ID: action.ID, WorkspaceID: action.WorkspaceID, IssueID: action.IssueID})
	if err != nil {
		return Observation{Kind: ObservationUnknown}, fmt.Errorf("%w: load correction task specification: %v", ErrEffectUnknown, err)
	}
	taskID := correctionTaskID(action.ID)
	task, err := effect.Queries.GetWorkflowCorrectionTaskEffect(ctx, db.GetWorkflowCorrectionTaskEffectParams{ID: taskID, WorkflowActionID: action.ID})
	if err != nil {
		if err == pgx.ErrNoRows {
			return Observation{Kind: ObservationAbsent, TaskID: taskID}, nil
		}
		return Observation{Kind: ObservationUnknown}, fmt.Errorf("%w: observe correction task: %v", ErrEffectUnknown, err)
	}
	expectedContext := expectedTaskContext(action, spec)
	if len(expectedContext) == 0 {
		return Observation{Kind: ObservationUnknown}, ErrEffectMismatch
	}
	if _, err := agentqualification.ValidateGuardedTaskContextForExecutionV1(expectedContext, nil, guardedExpectedIdentity(action, spec.AgentID, spec.RuntimeID), time.Now().UTC()); err != nil {
		return Observation{Kind: ObservationUnknown}, fmt.Errorf("guarded correction task observation: %w", err)
	}
	if err := validateTask(task.ID, task.WorkflowActionID, task.WorkflowOperationID, task.AgentID, task.RuntimeID, task.IssueID, task.WorkflowBranchKey, task.Status, task.Context, expectedContext, spec, true); err != nil {
		return Observation{Kind: ObservationUnknown}, err
	}
	result, _ := json.Marshal(map[string]string{"task_id": util.UUIDToString(task.ID)})
	return Observation{Kind: ObservationFound, TaskID: task.ID, ReceiptRevision: action.Revision, Result: result}, nil
}

func (effect LocalTaskEffect) Apply(ctx context.Context, action db.IssueWorkflowActionReceipt) (Observation, error) {
	if action.ActionType == "review_candidate" {
		return (ReviewTaskEffect(effect)).Apply(ctx, action)
	}
	if action.ActionType != "review_correction" {
		return Observation{Kind: ObservationUnknown}, ErrEffectMismatch
	}
	if effect.Now == nil {
		effect.Now = time.Now
	}
	spec, err := effect.Queries.GetWorkflowCorrectionTaskSpec(ctx, db.GetWorkflowCorrectionTaskSpecParams{ID: action.ID, WorkspaceID: action.WorkspaceID, IssueID: action.IssueID})
	if err != nil {
		return Observation{Kind: ObservationUnknown}, fmt.Errorf("%w: load correction task specification: %v", ErrEffectUnknown, err)
	}
	contextJSON := expectedTaskContext(action, spec)
	if len(contextJSON) == 0 {
		return Observation{Kind: ObservationUnknown}, ErrEffectMismatch
	}
	if _, err := agentqualification.ValidateGuardedTaskContextForExecutionV1(contextJSON, nil, guardedExpectedIdentity(action, spec.AgentID, spec.RuntimeID), effect.Now().UTC()); err != nil {
		return Observation{Kind: ObservationUnknown}, fmt.Errorf("guarded correction task admission: %w", err)
	}
	tx, err := effect.Starter.Begin(ctx)
	if err != nil {
		return Observation{Kind: ObservationUnknown}, fmt.Errorf("%w: begin correction task: %v", ErrEffectUnknown, err)
	}
	defer tx.Rollback(ctx)
	q := effect.Queries.WithTx(tx)
	if _, err := q.LockWorkflowCorrectionTaskWorkspace(ctx, action.WorkspaceID); err != nil {
		return Observation{Kind: ObservationUnknown}, fmt.Errorf("%w: correction task workspace unavailable", ErrEffectUnknown)
	}
	if _, err := q.LockWorkflowCorrectionTaskAgent(ctx, db.LockWorkflowCorrectionTaskAgentParams{AgentID: spec.AgentID, WorkspaceID: action.WorkspaceID, RuntimeID: spec.RuntimeID}); err != nil {
		return Observation{Kind: ObservationUnknown}, fmt.Errorf("%w: correction task agent unavailable", ErrEffectUnknown)
	}
	if _, err := q.LockWorkflowCorrectionTaskIssue(ctx, db.LockWorkflowCorrectionTaskIssueParams{IssueID: action.IssueID, WorkspaceID: action.WorkspaceID}); err != nil {
		return Observation{Kind: ObservationUnknown}, fmt.Errorf("%w: correction task issue unavailable", ErrEffectUnknown)
	}
	if _, err := q.LockWorkflowCorrectionTaskRuntime(ctx, db.LockWorkflowCorrectionTaskRuntimeParams{RuntimeID: spec.RuntimeID, WorkspaceID: action.WorkspaceID}); err != nil {
		return Observation{Kind: ObservationUnknown}, fmt.Errorf("%w: correction task owners unavailable", ErrEffectUnknown)
	}
	if effect.Failpoint != nil {
		if err := effect.Failpoint("after_owner_locks"); err != nil {
			return Observation{Kind: ObservationUnknown}, err
		}
	}
	current, err := q.LockWorkflowCorrectionTaskEffect(ctx, db.LockWorkflowCorrectionTaskEffectParams{ID: action.ID, WorkspaceID: action.WorkspaceID, IssueID: action.IssueID, ExpectedRevision: action.Revision, LeaseToken: action.LeaseToken, Now: pgTime(effect.Now())})
	if err != nil {
		return Observation{Kind: ObservationUnknown}, fmt.Errorf("%w: stale correction action: %v", ErrEffectUnknown, err)
	}
	if current.AgentID != spec.AgentID || current.RuntimeID != spec.RuntimeID || current.OperationID != spec.OperationID || current.BranchKey != spec.BranchKey {
		return Observation{Kind: ObservationUnknown}, ErrEffectMismatch
	}
	taskID := correctionTaskID(action.ID)
	task, err := q.CreateWorkflowCorrectionTask(ctx, db.CreateWorkflowCorrectionTaskParams{ID: taskID, AgentID: spec.AgentID, RuntimeID: spec.RuntimeID, IssueID: spec.IssueID, Priority: 0, Context: contextJSON, WorkflowActionID: action.ID, WorkflowOperationID: action.OperationID, WorkflowBranchKey: pgtype.Text{String: spec.BranchKey, Valid: true}})
	if err != nil {
		return Observation{Kind: ObservationUnknown}, fmt.Errorf("%w: create correction task: %v", ErrEffectUnknown, err)
	}
	if err := validateTask(task.ID, task.WorkflowActionID, task.WorkflowOperationID, task.AgentID, task.RuntimeID, task.IssueID, task.WorkflowBranchKey, task.Status, task.Context, contextJSON, spec, false); err != nil {
		return Observation{Kind: ObservationUnknown}, err
	}
	bound, err := q.BindWorkflowActionTaskExact(ctx, db.BindWorkflowActionTaskExactParams{TaskID: task.ID, Now: pgTime(effect.Now()), Context: contextJSON, WorkspaceID: action.WorkspaceID, IssueID: action.IssueID, ID: action.ID, ExpectedRevision: action.Revision, LeaseToken: action.LeaseToken, WorkflowBranchKey: pgtype.Text{String: spec.BranchKey, Valid: true}, AgentID: spec.AgentID, RuntimeID: spec.RuntimeID})
	if err != nil {
		return Observation{Kind: ObservationUnknown}, fmt.Errorf("%w: bind correction task: %v", ErrEffectUnknown, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Observation{Kind: ObservationUnknown}, fmt.Errorf("%w: commit correction task: %v", ErrEffectUnknown, err)
	}
	if effect.Wake != nil {
		_ = effect.Wake(ctx, task.ID)
	}
	result, _ := json.Marshal(map[string]string{"task_id": util.UUIDToString(task.ID)})
	return Observation{Kind: ObservationFound, TaskID: task.ID, ReceiptRevision: bound.Revision, Result: result}, nil
}

func validateTask(id, actionID, operationID, agentID, runtimeID, issueID pgtype.UUID, branchKey pgtype.Text, status string, context, expectedContext []byte, spec db.GetWorkflowCorrectionTaskSpecRow, observe bool) error {
	validStatus := status == "queued"
	if observe {
		validStatus = status == "queued" || status == "dispatched" || status == "waiting_local_directory" || status == "running" || status == "completed"
	}
	if id != correctionTaskID(spec.ID) || actionID != spec.ID || operationID != spec.OperationID || agentID != spec.AgentID || runtimeID != spec.RuntimeID || issueID != spec.IssueID || !branchKey.Valid || branchKey.String != spec.BranchKey || !validStatus || !jsonEqual(context, expectedContext) {
		return ErrEffectMismatch
	}
	return nil
}

func jsonEqual(left, right []byte) bool {
	var leftValue, rightValue any
	if json.Unmarshal(left, &leftValue) != nil || json.Unmarshal(right, &rightValue) != nil {
		return false
	}
	return reflect.DeepEqual(leftValue, rightValue)
}

func expectedTaskContext(action db.IssueWorkflowActionReceipt, spec db.GetWorkflowCorrectionTaskSpecRow) []byte {
	var snapshot struct {
		CandidateID                 string   `json:"candidate_id"`
		CandidateRevision           int64    `json:"candidate_revision"`
		CorrectionCandidateID       string   `json:"correction_candidate_id"`
		CorrectionCandidateRevision int64    `json:"correction_candidate_revision"`
		PreviewSourceRevision       string   `json:"preview_source_revision"`
		ScopeID                     string   `json:"scope_id"`
		ScopeRevision               int64    `json:"scope_revision"`
		AcceptanceVersion           int64    `json:"acceptance_version"`
		ReviewID                    string   `json:"review_id"`
		ReviewRevision              int64    `json:"review_revision"`
		ReviewerID                  string   `json:"reviewer_id"`
		CriteriaRefs                []string `json:"criteria_refs"`
		CoverageRefs                []string `json:"coverage_refs"`
		GapRefs                     []string `json:"gap_refs"`
		RequiredCorrection          string   `json:"required_correction"`
		CorrectionCount             uint32   `json:"correction_count"`
		MaxCorrections              uint32   `json:"max_corrections"`
	}
	if json.Unmarshal(action.Payload, &snapshot) != nil {
		return nil
	}
	contextValue := map[string]any{
		"task_kind":           "correction",
		"workflow_action_id":  util.UUIDToString(action.ID),
		"effect_key":          action.EffectKey.String,
		"operation_id":        util.UUIDToString(action.OperationID),
		"branch_key":          spec.BranchKey,
		"candidate_id":        snapshot.CandidateID,
		"candidate_revision":  snapshot.CandidateRevision,
		"scope_id":            snapshot.ScopeID,
		"scope_revision":      snapshot.ScopeRevision,
		"acceptance_version":  snapshot.AcceptanceVersion,
		"review_id":           snapshot.ReviewID,
		"review_revision":     snapshot.ReviewRevision,
		"reviewer_id":         snapshot.ReviewerID,
		"criteria_refs":       snapshot.CriteriaRefs,
		"coverage_refs":       snapshot.CoverageRefs,
		"gap_refs":            snapshot.GapRefs,
		"required_correction": snapshot.RequiredCorrection,
		"correction_count":    snapshot.CorrectionCount,
		"max_corrections":     snapshot.MaxCorrections,
	}
	if snapshot.CorrectionCandidateID != "" || snapshot.CorrectionCandidateRevision != 0 || snapshot.PreviewSourceRevision != "" {
		contextValue["preview"] = map[string]any{
			"kind":               workflowguard.StaticPreviewKind,
			"artifact_root":      workflowguard.StaticPreviewArtifactRoot,
			"candidate_id":       snapshot.CorrectionCandidateID,
			"candidate_revision": snapshot.CorrectionCandidateRevision,
			"source_revision":    snapshot.PreviewSourceRevision,
			"policy_id":          workflowguard.StaticPreviewPolicyID,
		}
	}
	value, err := normalizeGuardedTaskContextV1(action.Payload, contextValue, guardedExpectedIdentity(action, spec.AgentID, spec.RuntimeID))
	if err != nil {
		return nil
	}
	return value
}

func guardedExpectedIdentity(action db.IssueWorkflowActionReceipt, agentID, runtimeID pgtype.UUID) agentqualification.GuardedDispatchExpectedIdentityV1 {
	return agentqualification.GuardedDispatchExpectedIdentityV1{
		WorkspaceID:     util.UUIDToString(action.WorkspaceID),
		IssueID:         util.UUIDToString(action.IssueID),
		ExecutorAgentID: util.UUIDToString(agentID),
		RuntimeID:       util.UUIDToString(runtimeID),
	}
}

func correctionTaskID(actionID pgtype.UUID) pgtype.UUID {
	value := uuid.NewSHA1(uuid.UUID(actionID.Bytes), []byte("review_correction/task"))
	return pgtype.UUID{Bytes: value, Valid: actionID.Valid}
}
