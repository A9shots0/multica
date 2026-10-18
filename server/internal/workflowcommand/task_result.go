package workflowcommand

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"time"

	googleuuid "github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/internal/workflowprojection"
	workflowstatev2 "github.com/multica-ai/multica/server/internal/workflowstate/v2"
	"github.com/multica-ai/multica/server/internal/workflowstore"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// ReconcileWorkflowTaskResult selects the command from the exact settled
// action type. Free-form output and terminal status alone are never authority.
func (s Service) ReconcileWorkflowTaskResult(ctx context.Context, taskID pgtype.UUID) (Receipt, error) {
	if s.Queries == nil || s.Starter == nil || !taskID.Valid {
		return Receipt{}, fmt.Errorf("workflow task result reconciler is not configured")
	}
	route, err := s.Queries.GetWorkflowTaskResultRoute(ctx, taskID)
	if err != nil {
		return Receipt{}, err
	}
	if replay, found, err := s.processedTaskResultReplay(ctx, route); err != nil {
		return Receipt{}, err
	} else if found {
		return replay, nil
	}
	if route.ReconciliationState == "failed" {
		return rejectedTaskResultReceipt(route), nil
	}
	receipt, err := s.reconcileWorkflowTaskResult(ctx, route)
	if err != nil && isPermanentTaskResultError(err) {
		if markErr := s.markTaskResultFailed(ctx, taskID, err); markErr != nil {
			return Receipt{}, markErr
		}
		return rejectedTaskResultReceipt(route), nil
	}
	return receipt, err
}

func (s Service) processedTaskResultReplay(ctx context.Context, route db.GetWorkflowTaskResultRouteRow) (Receipt, bool, error) {
	suffix, kind := "/correction-result", workflowstatev2.CommandSubmitCorrection
	if route.ActionType == "review_candidate" {
		suffix = "/review-result"
	} else if route.ActionType == "verify_runtime_exact_release" {
		suffix, kind = "/runtime-verification-result", workflowstatev2.CommandVerifyRuntime
	} else if route.ActionType == "verify_business_exact_release" {
		suffix, kind = "/business-verification-result", workflowstatev2.CommandVerifyBusiness
	} else if route.ActionType == "monitor_exact_release" {
		suffix, kind = "/production-regression-result", workflowstatev2.CommandProductionFailed
	} else if route.ActionType == "verify_restoration_exact_release" {
		suffix, kind = "/restoration-verification-result", workflowstatev2.CommandRestoreVerified
	}
	commandKey := "task/" + util.UUIDToString(route.TaskID) + suffix
	command, err := s.Queries.GetIssueWorkflowCommandByKey(ctx, db.GetIssueWorkflowCommandByKeyParams{WorkspaceID: route.WorkspaceID, IssueID: route.IssueID, CommandKey: commandKey})
	if errors.Is(err, pgx.ErrNoRows) {
		return Receipt{}, false, nil
	}
	if err != nil {
		return Receipt{}, false, err
	}
	if command.State != "processed" {
		return Receipt{}, false, ErrRecoveryRequired
	}
	var semantic commandSemantic
	workflowResult, resultErr := decodeWorkflowTaskResult(route.ActionType, route.Result)
	if resultErr != nil || json.Unmarshal(command.Payload, &semantic) != nil {
		return Receipt{}, false, ErrIdempotencyConflict
	}
	if route.ActionType == "review_candidate" {
		kind = reviewKind(workflowResult.Verdict)
	}
	exactResult := reflect.DeepEqual(semantic.Request.Correction, workflowResult)
	if route.ActionType == "verify_runtime_exact_release" || route.ActionType == "verify_business_exact_release" || route.ActionType == "monitor_exact_release" || route.ActionType == "verify_restoration_exact_release" {
		request, requestErr := releaseVerificationTaskRequest(route, workflowResult)
		exactResult = requestErr == nil && reflect.DeepEqual(semantic.Request.ReleaseEffect, request.ReleaseEffect)
	}
	if kind == "" || semantic.Principal != util.UUIDToString(route.AgentID) || semantic.ActorType != "agent_task" || semantic.TaskID != util.UUIDToString(route.TaskID) || semantic.ActionID != util.UUIDToString(route.ActionID) || semantic.ActionKey != route.ActionKey || semantic.Request.CommandKey != commandKey || semantic.Request.Kind != kind || !exactResult {
		return Receipt{}, false, ErrIdempotencyConflict
	}
	var receipt Receipt
	if json.Unmarshal(command.Result, &receipt) != nil {
		return Receipt{}, false, ErrInvalidAuthoritativeState
	}
	return receipt, true, nil
}

func (s Service) reconcileWorkflowTaskResult(ctx context.Context, route db.GetWorkflowTaskResultRouteRow) (Receipt, error) {
	taskID := route.TaskID
	if _, ok := parseTaskResultSnapshot(route); !ok {
		return Receipt{}, ErrInvalidAuthoritativeState
	}
	workflowResult, decodeErr := decodeWorkflowTaskResult(route.ActionType, route.Result)
	if decodeErr != nil {
		return Receipt{}, ErrInvalidAuthoritativeState
	}
	request := Request{WorkspaceID: route.WorkspaceID, IssueID: route.IssueID, Correction: workflowResult}
	switch route.ActionType {
	case "review_correction":
		if workflowResult.Type != "correction" {
			return Receipt{}, ErrInvalidAuthoritativeState
		}
		request.CommandKey = "task/" + util.UUIDToString(taskID) + "/correction-result"
		request.Kind = workflowstatev2.CommandSubmitCorrection
	case "review_candidate":
		if workflowResult.Type != "review" {
			return Receipt{}, ErrInvalidAuthoritativeState
		}
		request.CommandKey = "task/" + util.UUIDToString(taskID) + "/review-result"
		request.Kind = reviewKind(workflowResult.Verdict)
		if request.Kind == "" {
			return Receipt{}, ErrInvalidAuthoritativeState
		}
		request.CriteriaRefs = append([]string{}, workflowResult.CriteriaRefs...)
		request.CoverageRefs = append([]string{}, workflowResult.CoverageRefs...)
		request.GapRefs = append([]string{}, workflowResult.GapRefs...)
		request.RequiredCorrection = workflowResult.RequiredCorrection
	case "verify_runtime_exact_release", "verify_business_exact_release", "monitor_exact_release", "verify_restoration_exact_release":
		var requestErr error
		request, requestErr = releaseVerificationTaskRequest(route, workflowResult)
		if requestErr != nil {
			return Receipt{}, requestErr
		}
	default:
		return Receipt{}, ErrUnsupportedCommand
	}
	principal := Principal{ActorID: route.AgentID, ActorType: "agent_task", TaskID: taskID, ActionID: route.ActionID, ActionKey: route.ActionKey}
	state := applyState{}
	applyResult := func(write *workflowstore.IssueWrite) error {
		picture, err := s.Loader.Load(ctx, write, route.WorkspaceID, route.IssueID)
		if err != nil {
			return err
		}
		concerns := []workflowstatev2.Concern{workflowstatev2.ConcernExecution, workflowstatev2.ConcernReview, workflowstatev2.ConcernEvidence}
		if request.Kind == workflowstatev2.CommandSubmitCorrection {
			concerns = append(concerns, workflowstatev2.ConcernRelease)
		} else if request.Kind == workflowstatev2.CommandVerifyRuntime {
			concerns = []workflowstatev2.Concern{workflowstatev2.ConcernRelease, workflowstatev2.ConcernEvidence}
		} else if request.Kind == workflowstatev2.CommandVerifyBusiness {
			concerns = []workflowstatev2.Concern{workflowstatev2.ConcernEvidence, workflowstatev2.ConcernExecution, workflowstatev2.ConcernOutcome, workflowstatev2.ConcernRelease}
		} else if request.Kind == workflowstatev2.CommandProductionFailed {
			concerns = []workflowstatev2.Concern{workflowstatev2.ConcernEvidence, workflowstatev2.ConcernExecution, workflowstatev2.ConcernOutcome, workflowstatev2.ConcernRelease}
		} else if request.Kind == workflowstatev2.CommandRestoreVerified {
			concerns = []workflowstatev2.Concern{workflowstatev2.ConcernEvidence, workflowstatev2.ConcernExecution, workflowstatev2.ConcernOutcome, workflowstatev2.ConcernRelease}
		}
		for _, concern := range concerns {
			request.Expected = append(request.Expected, workflowstatev2.ExpectedRevision{Concern: concern, Revision: picture.Revisions.Get(concern)})
		}
		if request.Kind == workflowstatev2.CommandVerifyRuntime {
			begin := request
			begin.CommandKey = "task/" + util.UUIDToString(taskID) + "/begin-runtime-verification"
			begin.Kind = workflowstatev2.CommandBeginVerification
			begin.Expected = []workflowstatev2.ExpectedRevision{
				{Concern: workflowstatev2.ConcernRelease, Revision: picture.Revisions.Release},
				{Concern: workflowstatev2.ConcernEvidence, Revision: picture.Revisions.Evidence},
			}
			beginState := applyState{}
			if err := s.executeInWrite(ctx, write, principal, begin, &beginState); err != nil {
				return err
			}
			request.Expected = []workflowstatev2.ExpectedRevision{
				{Concern: workflowstatev2.ConcernEvidence, Revision: beginState.receipt.Revisions.Evidence},
			}
		}
		return s.executeInWrite(ctx, write, principal, request, &state)
	}
	var err error
	if route.ActionType == "verify_runtime_exact_release" || route.ActionType == "verify_business_exact_release" || route.ActionType == "monitor_exact_release" || route.ActionType == "verify_restoration_exact_release" {
		var snapshot releaseEffectSnapshot
		if json.Unmarshal(route.Payload, &snapshot) != nil {
			return Receipt{}, ErrInvalidAuthoritativeState
		}
		err = workflowstore.InReleaseAdmissionTransaction(ctx, s.Starter, s.Queries, route.WorkspaceID, route.IssueID, []pgtype.UUID{mustUUID(snapshot.ReleaseOperatorID), mustUUID(snapshot.PlannedVerifierID)}, applyResult)
	} else {
		err = workflowstore.InIssueWriteTransaction(ctx, s.Starter, s.Queries, route.WorkspaceID, route.IssueID, applyResult)
	}
	if err != nil {
		return Receipt{}, err
	}
	if !state.replay && s.Wake != nil && len(state.receipt.ActionIDs) > 0 {
		if err := s.Wake(ctx, append([]string(nil), state.receipt.ActionIDs...)); err != nil && s.OnWakeError != nil {
			s.OnWakeError(err)
		}
	}
	if !state.replay {
		s.NotifyProjection(ctx, route.WorkspaceID, route.IssueID, state.receipt)
	}
	return state.receipt, nil
}

func decodeWorkflowTaskResult(actionType string, result []byte) (*CorrectionResultInput, error) {
	var envelope struct {
		WorkflowResult json.RawMessage `json:"workflow_result"`
	}
	if json.Unmarshal(result, &envelope) != nil || len(envelope.WorkflowResult) == 0 || bytes.Equal(envelope.WorkflowResult, []byte("null")) {
		return nil, ErrInvalidAuthoritativeState
	}
	decodeStrict := func(destination any) error {
		decoder := json.NewDecoder(bytes.NewReader(envelope.WorkflowResult))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(destination); err != nil {
			return err
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			return ErrInvalidAuthoritativeState
		}
		return nil
	}
	switch actionType {
	case "verify_runtime_exact_release":
		var typed struct {
			Type         string             `json:"type"`
			RuntimeProof *VerificationProof `json:"runtime_proof"`
		}
		if decodeStrict(&typed) != nil {
			return nil, ErrInvalidAuthoritativeState
		}
		return &CorrectionResultInput{Type: typed.Type, RuntimeProof: typed.RuntimeProof}, nil
	case "verify_business_exact_release":
		var typed struct {
			Type            string             `json:"type"`
			BusinessProof   *VerificationProof `json:"business_proof"`
			RegressionProof *VerificationProof `json:"regression_proof"`
		}
		if decodeStrict(&typed) != nil {
			return nil, ErrInvalidAuthoritativeState
		}
		return &CorrectionResultInput{Type: typed.Type, BusinessProof: typed.BusinessProof, RegressionProof: typed.RegressionProof}, nil
	case "monitor_exact_release":
		var typed struct {
			Type            string             `json:"type"`
			RegressionProof *VerificationProof `json:"regression_proof"`
		}
		if decodeStrict(&typed) != nil {
			return nil, ErrInvalidAuthoritativeState
		}
		return &CorrectionResultInput{Type: typed.Type, RegressionProof: typed.RegressionProof}, nil
	case "verify_restoration_exact_release":
		var typed struct {
			Type             string             `json:"type"`
			RuntimeProof     *VerificationProof `json:"runtime_proof"`
			BusinessProof    *VerificationProof `json:"business_proof"`
			RegressionProof  *VerificationProof `json:"regression_proof"`
			RestorationProof *VerificationProof `json:"restoration_proof"`
		}
		if decodeStrict(&typed) != nil {
			return nil, ErrInvalidAuthoritativeState
		}
		return &CorrectionResultInput{Type: typed.Type, RuntimeProof: typed.RuntimeProof, BusinessProof: typed.BusinessProof, RegressionProof: typed.RegressionProof, RestorationProof: typed.RestorationProof}, nil
	default:
		var typed CorrectionResultInput
		if json.Unmarshal(envelope.WorkflowResult, &typed) != nil {
			return nil, ErrInvalidAuthoritativeState
		}
		return &typed, nil
	}
}

func releaseVerificationTaskRequest(route db.GetWorkflowTaskResultRouteRow, result *CorrectionResultInput) (Request, error) {
	var snapshot releaseEffectSnapshot
	decoder := json.NewDecoder(strings.NewReader(string(route.Payload)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&snapshot) != nil || decoder.Decode(&struct{}{}) != io.EOF || result == nil {
		return Request{}, ErrInvalidAuthoritativeState
	}
	request := Request{WorkspaceID: route.WorkspaceID, IssueID: route.IssueID, OperationID: snapshot.OperationID, BranchID: snapshot.BranchKey, CandidateID: snapshot.CandidateID, TargetID: snapshot.TargetKey}
	evidence := map[string]any{}
	switch route.ActionType {
	case "verify_runtime_exact_release":
		if result.Type != "runtime_verification" || !emptyNonVerificationResultFields(result) || result.RuntimeProof == nil || result.BusinessProof != nil || result.RegressionProof != nil || !validVerificationProof(*result.RuntimeProof) {
			return Request{}, ErrInvalidAuthoritativeState
		}
		request.CommandKey = "task/" + util.UUIDToString(route.TaskID) + "/runtime-verification-result"
		request.Kind = workflowstatev2.CommandVerifyRuntime
		evidence["runtime_proof"] = result.RuntimeProof
	case "verify_business_exact_release":
		if result.Type != "business_verification" || !emptyNonVerificationResultFields(result) || result.RuntimeProof != nil || result.BusinessProof == nil || result.RegressionProof == nil || !validVerificationProof(*result.BusinessProof) || !validVerificationProof(*result.RegressionProof) || reflect.DeepEqual(result.BusinessProof, result.RegressionProof) {
			return Request{}, ErrInvalidAuthoritativeState
		}
		request.CommandKey = "task/" + util.UUIDToString(route.TaskID) + "/business-verification-result"
		request.Kind = workflowstatev2.CommandVerifyBusiness
		evidence["business_proof"], evidence["regression_proof"] = result.BusinessProof, result.RegressionProof
	case "monitor_exact_release":
		if result.Type != "production_regression" || !emptyNonVerificationResultFields(result) || result.RuntimeProof != nil || result.BusinessProof != nil || result.RegressionProof == nil || result.RestorationProof != nil || !validVerificationProof(*result.RegressionProof) {
			return Request{}, ErrInvalidAuthoritativeState
		}
		request.CommandKey = "task/" + util.UUIDToString(route.TaskID) + "/production-regression-result"
		request.Kind = workflowstatev2.CommandProductionFailed
		evidence["regression_proof"] = result.RegressionProof
	case "verify_restoration_exact_release":
		if result.Type != "restoration_verification" || !emptyNonVerificationResultFields(result) || result.RuntimeProof == nil || result.BusinessProof == nil || result.RegressionProof == nil || result.RestorationProof == nil || !validVerificationProof(*result.RuntimeProof) || !validVerificationProof(*result.BusinessProof) || !validVerificationProof(*result.RegressionProof) || !validVerificationProof(*result.RestorationProof) || !distinctVerificationProofs(*result.RuntimeProof, *result.BusinessProof, *result.RegressionProof, *result.RestorationProof) {
			return Request{}, ErrInvalidAuthoritativeState
		}
		request.CommandKey = "task/" + util.UUIDToString(route.TaskID) + "/restoration-verification-result"
		request.Kind = workflowstatev2.CommandRestoreVerified
		evidence["runtime_proof"], evidence["business_proof"], evidence["regression_proof"], evidence["restoration_proof"] = result.RuntimeProof, result.BusinessProof, result.RegressionProof, result.RestorationProof
	default:
		return Request{}, ErrUnsupportedCommand
	}
	request.ReleaseEffect = &ReleaseEffectResultInput{
		Type:      map[workflowstatev2.CommandKind]string{workflowstatev2.CommandVerifyRuntime: "runtime", workflowstatev2.CommandVerifyBusiness: "business", workflowstatev2.CommandProductionFailed: "regression", workflowstatev2.CommandRestoreVerified: "restoration"}[request.Kind],
		EffectKey: route.EffectKey.String, ProviderReceiptID: util.UUIDToString(route.TaskID), ReleaseID: snapshot.ReleaseID,
		CandidateID: snapshot.CandidateID, CandidateRevision: snapshot.CandidateRevision, ScopeID: snapshot.ScopeID,
		ScopeRevision: snapshot.ScopeRevision, AcceptanceVersion: snapshot.AcceptanceVersion, OperationID: snapshot.OperationID,
		BranchKey: snapshot.BranchKey, TargetKey: snapshot.TargetKey, TargetGeneration: snapshot.DesiredTargetGeneration,
		ArtifactDigest: snapshot.ArtifactDigest, RuntimeContractDigest: snapshot.RuntimeContractDigest, Evidence: evidence,
	}
	return request, nil
}

func emptyNonVerificationResultFields(result *CorrectionResultInput) bool {
	return strings.TrimSpace(result.SourceRevision) == "" && len(result.Artifact) == 0 && len(result.BuildContract) == 0 && len(result.Evidence) == 0 &&
		strings.TrimSpace(result.Verdict) == "" && len(result.CriteriaRefs) == 0 && len(result.CoverageRefs) == 0 && len(result.GapRefs) == 0 && strings.TrimSpace(result.RequiredCorrection) == ""
}

func rejectedTaskResultReceipt(route db.GetWorkflowTaskResultRouteRow) Receipt {
	return Receipt{State: "rejected", ActionIDs: []string{util.UUIDToString(route.ActionID)}, ActionState: "failed"}
}

func (s Service) ReconcileCorrectionTaskResult(ctx context.Context, taskID pgtype.UUID) (Receipt, error) {
	return s.ReconcileWorkflowTaskResult(ctx, taskID)
}

func (s Service) ReconcilePendingCorrectionResults(ctx context.Context, batchSize int32) error {
	return s.ReconcilePendingTaskResults(ctx, batchSize)
}

// ReconcilePendingTaskResults recovers the commit-before-command window for
// every normalized workflow task. A permanently invalid typed result is
// recorded on its exact action receipt and projected, while later rows in the
// same batch continue; one poison result can never starve valid work.
func (s Service) ReconcilePendingTaskResults(ctx context.Context, batchSize int32) error {
	if batchSize <= 0 {
		batchSize = 25
	}
	rows, err := s.Queries.ListPendingWorkflowTaskResults(ctx, batchSize)
	if err != nil {
		return err
	}
	var failures []error
	for _, taskID := range rows {
		if _, reconcileErr := s.ReconcileWorkflowTaskResult(ctx, taskID); reconcileErr != nil {
			if isPermanentTaskResultError(reconcileErr) {
				if markErr := s.markTaskResultFailed(ctx, taskID, reconcileErr); markErr != nil {
					failures = append(failures, fmt.Errorf("mark workflow task %s result failed: %w", util.UUIDToString(taskID), markErr))
				}
				continue
			}
			failures = append(failures, fmt.Errorf("reconcile workflow task %s result: %w", util.UUIDToString(taskID), reconcileErr))
		}
	}
	return errors.Join(failures...)
}

func isPermanentTaskResultError(err error) bool {
	return errors.Is(err, ErrInvalidAuthoritativeState) || errors.Is(err, ErrUnsupportedCommand) || errors.Is(err, ErrForbidden)
}

func (s Service) markTaskResultFailed(ctx context.Context, taskID pgtype.UUID, cause error) error {
	if s.Now == nil {
		s.Now = time.Now
	}
	route, err := s.Queries.GetWorkflowTaskResultRoute(ctx, taskID)
	if err != nil {
		return err
	}
	detail := "typed workflow result rejected: " + cause.Error()
	if len(detail) > 2048 {
		detail = detail[:2048]
	}
	var projectionRevision int64
	err = workflowstore.InIssueWriteTransaction(ctx, s.Starter, s.Queries, route.WorkspaceID, route.IssueID, func(write *workflowstore.IssueWrite) error {
		picture, err := s.Loader.Load(ctx, write, route.WorkspaceID, route.IssueID)
		if err != nil {
			return err
		}
		if _, err := write.MarkTaskResultReconciliationFailed(ctx, db.MarkWorkflowTaskResultReconciliationFailedParams{LastError: detail, TaskID: taskID, WorkspaceID: route.WorkspaceID, IssueID: route.IssueID}); err != nil {
			return err
		}
		currentAction, err := taskResultActionStillCurrent(ctx, write, route, picture)
		if err != nil {
			return err
		}
		if !currentAction {
			// The failed receipt is itself a durable, terminal execution fence.
			// A stale action must not stop newer work; it is quarantined once and
			// excluded from future polling.
			var projectErr error
			projectionRevision, projectErr = s.projectTaskResultQuarantine(ctx, write, route)
			return projectErr
		}
		if !s.ControllerID.Valid {
			return fmt.Errorf("workflow result fail-closed controller is not configured")
		}
		state := applyState{}
		executeErr := s.executeInWrite(ctx, write, Principal{ActorID: s.ControllerID, ActorType: "internal_controller"}, Request{
			WorkspaceID: route.WorkspaceID, IssueID: route.IssueID,
			CommandKey: "task/" + util.UUIDToString(taskID) + "/invalid-result/technical-stop",
			Kind:       workflowstatev2.CommandTechnicalStop, OperationID: picture.Execution.OperationID,
			Detail:   "typed workflow result is missing or invalid",
			Expected: []workflowstatev2.ExpectedRevision{{Concern: workflowstatev2.ConcernExecution, Revision: picture.Revisions.Execution}},
		}, &state)
		projectionRevision = state.receipt.CurrentPictureRevision
		return executeErr
	})
	if err == nil && projectionRevision > 0 {
		s.NotifyProjection(ctx, route.WorkspaceID, route.IssueID, Receipt{CurrentPictureRevision: projectionRevision})
	}
	return err
}

func (s Service) projectTaskResultQuarantine(ctx context.Context, write *workflowstore.IssueWrite, route db.GetWorkflowTaskResultRouteRow) (int64, error) {
	expected, revision := int64(0), int64(1)
	current, err := write.GetCurrentPicture(ctx, db.GetIssueCurrentPictureParams{WorkspaceID: route.WorkspaceID, IssueID: route.IssueID})
	if err == nil {
		expected, revision = current.Revision, current.Revision+1
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	storedPicture, err := workflowprojection.ProjectInWrite(ctx, write, route.WorkspaceID, route.IssueID, expected, revision, s.Now().UTC())
	if err == nil && s.OnProjectionStored != nil {
		err = s.OnProjectionStored(ctx, write, route.WorkspaceID, route.IssueID, storedPicture)
	}
	return revision, err
}

type taskResultSnapshot struct {
	OperationID       string   `json:"operation_id"`
	BranchKey         string   `json:"branch_key"`
	CandidateID       string   `json:"candidate_id"`
	CandidateRevision int64    `json:"candidate_revision"`
	ScopeID           string   `json:"scope_id"`
	ScopeRevision     int64    `json:"scope_revision"`
	AcceptanceVersion int64    `json:"acceptance_version"`
	ReviewID          string   `json:"review_id"`
	ReviewRevision    int64    `json:"review_revision"`
	ReviewerID        string   `json:"reviewer_id"`
	ReviewerRuntimeID string   `json:"reviewer_runtime_id"`
	EvidenceRefs      []string `json:"implementation_evidence_refs"`
	ReleaseID         string   `json:"release_id"`
	ReleaseRevision   int64    `json:"release_revision"`
	TargetKey         string   `json:"target_key"`
	TargetGeneration  int64    `json:"desired_target_generation"`
}

func parseTaskResultSnapshot(route db.GetWorkflowTaskResultRouteRow) (taskResultSnapshot, bool) {
	var snapshot taskResultSnapshot
	if json.Unmarshal(route.Payload, &snapshot) != nil || strings.TrimSpace(snapshot.BranchKey) == "" || snapshot.CandidateRevision <= 0 || snapshot.ScopeRevision <= 0 || snapshot.AcceptanceVersion <= 0 || snapshot.ReviewRevision <= 0 {
		return snapshot, false
	}
	for _, value := range []string{snapshot.OperationID, snapshot.CandidateID, snapshot.ScopeID, snapshot.ReviewID} {
		if _, err := googleuuid.Parse(value); err != nil {
			return snapshot, false
		}
	}
	if snapshot.OperationID != util.UUIDToString(route.OperationID) {
		return snapshot, false
	}
	if route.ActionType == "review_candidate" {
		for _, value := range []string{snapshot.ReviewerID, snapshot.ReviewerRuntimeID} {
			if _, err := googleuuid.Parse(value); err != nil {
				return snapshot, false
			}
		}
		if !meaningfulRefs(snapshot.EvidenceRefs) {
			return snapshot, false
		}
	} else if route.ActionType == "verify_runtime_exact_release" || route.ActionType == "verify_business_exact_release" {
		if _, err := googleuuid.Parse(snapshot.ReleaseID); err != nil || snapshot.ReleaseRevision <= 0 || strings.TrimSpace(snapshot.TargetKey) == "" || snapshot.TargetGeneration <= 0 {
			return snapshot, false
		}
	}
	return snapshot, true
}

func taskResultActionStillCurrent(ctx context.Context, write *workflowstore.IssueWrite, route db.GetWorkflowTaskResultRouteRow, picture workflowstatev2.CurrentPicture) (bool, error) {
	snapshot, ok := parseTaskResultSnapshot(route)
	if !ok {
		return false, nil
	}
	currentCandidateID := picture.Review.CandidateID
	if route.ActionType == "verify_runtime_exact_release" || route.ActionType == "verify_business_exact_release" {
		currentCandidateID = picture.Release.CandidateID
	}
	if snapshot.OperationID != util.UUIDToString(route.OperationID) || snapshot.OperationID != picture.Execution.OperationID || snapshot.CandidateID != currentCandidateID || uint64(snapshot.ScopeRevision) != uint64(picture.Scope.Revision) || uint64(snapshot.AcceptanceVersion) != uint64(picture.Scope.AcceptanceVersion) {
		return false, nil
	}
	execution, err := selectedExecution(ctx, write, route.WorkspaceID, route.IssueID)
	if err != nil {
		return false, err
	}
	if execution.ID != route.OperationID || execution.BranchKey != snapshot.BranchKey || !execution.CandidateID.Valid || util.UUIDToString(execution.CandidateID) != snapshot.CandidateID {
		return false, nil
	}
	candidateID := mustUUID(snapshot.CandidateID)
	if !candidateID.Valid {
		return false, nil
	}
	candidate, err := write.GetCandidate(ctx, db.GetIssueWorkflowCandidateParams{WorkspaceID: route.WorkspaceID, IssueID: route.IssueID, ID: candidateID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	if candidate.Revision != snapshot.CandidateRevision || util.UUIDToString(candidate.ScopeID) != snapshot.ScopeID || candidate.ScopeRevision != snapshot.ScopeRevision || candidate.SupersededAt.Valid {
		return false, nil
	}
	switch route.ActionType {
	case "review_correction":
		if picture.Execution.State != workflowstatev2.ExecutionCorrection || picture.Review.State != workflowstatev2.ReviewFailed || snapshot.ReviewID == "" || snapshot.ReviewRevision <= 0 {
			return false, nil
		}
		reviewHead, err := write.GetConcernHead(ctx, db.GetIssueWorkflowConcernHeadParams{WorkspaceID: route.WorkspaceID, IssueID: route.IssueID, ConcernType: string(workflowstatev2.ConcernReview)})
		if err != nil {
			return false, err
		}
		return util.UUIDToString(reviewHead.SourceRecordID) == snapshot.ReviewID && reviewHead.SourceRecordRevision == snapshot.ReviewRevision, nil
	case "review_candidate":
		if picture.Execution.State != workflowstatev2.ExecutionInReview || picture.Review.State != workflowstatev2.ReviewInProgress || snapshot.ReviewID == "" || snapshot.ReviewRevision <= 0 || !meaningfulRefs(snapshot.EvidenceRefs) {
			return false, nil
		}
		reviewHead, err := write.GetConcernHead(ctx, db.GetIssueWorkflowConcernHeadParams{WorkspaceID: route.WorkspaceID, IssueID: route.IssueID, ConcernType: string(workflowstatev2.ConcernReview)})
		if err != nil {
			return false, err
		}
		if util.UUIDToString(reviewHead.SourceRecordID) != snapshot.ReviewID || reviewHead.SourceRecordRevision != snapshot.ReviewRevision {
			return false, nil
		}
		evidenceHead, err := write.GetConcernHead(ctx, db.GetIssueWorkflowConcernHeadParams{WorkspaceID: route.WorkspaceID, IssueID: route.IssueID, ConcernType: string(workflowstatev2.ConcernEvidence)})
		if err != nil {
			return false, err
		}
		for _, ref := range snapshot.EvidenceRefs {
			if ref == util.UUIDToString(evidenceHead.SourceRecordID) {
				return true, nil
			}
		}
		return false, nil
	case "verify_runtime_exact_release", "verify_business_exact_release":
		wantState := workflowstatev2.ReleaseExposed
		if route.ActionType == "verify_business_exact_release" {
			wantState = workflowstatev2.ReleaseProductionVerifying
		}
		releaseHead, err := write.GetConcernHead(ctx, db.GetIssueWorkflowConcernHeadParams{WorkspaceID: route.WorkspaceID, IssueID: route.IssueID, ConcernType: string(workflowstatev2.ConcernRelease)})
		if err != nil {
			return false, err
		}
		return util.UUIDToString(releaseHead.SourceRecordID) == snapshot.ReleaseID && releaseHead.SourceRecordRevision == snapshot.ReleaseRevision &&
			picture.Release.State == wantState && picture.Release.CandidateID == snapshot.CandidateID &&
			picture.Release.CandidateRevision == workflowstatev2.Revision(snapshot.CandidateRevision) && picture.Release.TargetID == snapshot.TargetKey &&
			picture.Release.Generation == workflowstatev2.Revision(snapshot.TargetGeneration), nil
	default:
		return false, nil
	}
}

type TaskResultReconciler struct {
	Service      Service
	PollInterval time.Duration
	BatchSize    int32
	OnError      func(error)
}

// Run scans immediately at startup and then on a poll fallback. Result errors
// are durable data conditions, not worker-fatal conditions; database/systemic
// errors are reported and retried on the next tick.
func (worker TaskResultReconciler) Run(ctx context.Context) error {
	if worker.PollInterval <= 0 {
		worker.PollInterval = time.Second
	}
	if worker.BatchSize <= 0 {
		worker.BatchSize = 25
	}
	reconcile := func() {
		if err := worker.Service.ReconcilePendingTaskResults(ctx, worker.BatchSize); err != nil {
			if worker.OnError != nil {
				worker.OnError(err)
			} else {
				slog.Error("workflow task-result reconciliation failed", "error", err)
			}
		}
	}
	reconcile()
	ticker := time.NewTicker(worker.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			reconcile()
		}
	}
}

func reviewKind(verdict string) workflowstatev2.CommandKind {
	switch verdict {
	case "passed":
		return workflowstatev2.CommandReviewPassed
	case "failed":
		return workflowstatev2.CommandReviewFailed
	case "inconclusive":
		return workflowstatev2.CommandReviewInconclusive
	case "blocked_evidence":
		return workflowstatev2.CommandReviewBlockedEvidence
	default:
		return ""
	}
}
