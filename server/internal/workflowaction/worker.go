// Package workflowaction executes durable workflow action receipts.
package workflowaction

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/internal/workflowcommand"
	workflowstatev2 "github.com/multica-ai/multica/server/internal/workflowstate/v2"
	"github.com/multica-ai/multica/server/internal/workflowstore"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

var (
	ErrEffectUnknown  = errors.New("workflow action effect is not safely observable")
	ErrEffectMismatch = errors.New("workflow action effect identity mismatch")
)

type ObservationKind string

const (
	ObservationFound   ObservationKind = "found"
	ObservationAbsent  ObservationKind = "absent"
	ObservationUnknown ObservationKind = "unknown"
)

type Observation struct {
	Kind            ObservationKind
	TaskID          pgtype.UUID
	ReceiptRevision int64
	Result          []byte
}

type Effect interface {
	Observe(context.Context, db.IssueWorkflowActionReceipt) (Observation, error)
	Apply(context.Context, db.IssueWorkflowActionReceipt) (Observation, error)
}

type EffectFuncs struct {
	ObserveFunc func(context.Context, db.IssueWorkflowActionReceipt) (Observation, error)
	ApplyFunc   func(context.Context, db.IssueWorkflowActionReceipt) (Observation, error)
}

func (effect EffectFuncs) Observe(ctx context.Context, action db.IssueWorkflowActionReceipt) (Observation, error) {
	return effect.ObserveFunc(ctx, action)
}

func (effect EffectFuncs) Apply(ctx context.Context, action db.IssueWorkflowActionReceipt) (Observation, error) {
	return effect.ApplyFunc(ctx, action)
}

type Capacity interface {
	Available(context.Context, db.IssueWorkflowActionReceipt) (bool, error)
}

type CapacityFunc func(context.Context, db.IssueWorkflowActionReceipt) (bool, error)

func (fn CapacityFunc) Available(ctx context.Context, action db.IssueWorkflowActionReceipt) (bool, error) {
	return fn(ctx, action)
}

type WakeFunc func(context.Context, pgtype.UUID) error

type Worker struct {
	Starter        workflowstore.TxStarter
	Queries        *db.Queries
	Effect         Effect
	ReleaseEffect  Effect
	VerifierEffect Effect
	Capacity       Capacity
	CommandService workflowcommand.Service
	ControllerID   pgtype.UUID
	Now            func() time.Time
	LeaseDuration  time.Duration
	RetryDelay     time.Duration
	PollInterval   time.Duration
	BatchSize      int32
	wakeMu         sync.Mutex
	wake           chan struct{}
}

func (worker *Worker) Wake() {
	worker.wakeMu.Lock()
	defer worker.wakeMu.Unlock()
	if worker.wake == nil {
		worker.wake = make(chan struct{}, 1)
	}
	select {
	case worker.wake <- struct{}{}:
	default:
	}
}

func (worker *Worker) Run(ctx context.Context) error {
	worker.defaults()
	if err := worker.ReconcileOnce(ctx); err != nil {
		return err
	}
	ticker := time.NewTicker(worker.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		case <-worker.wake:
		}
		if err := worker.ReconcileOnce(ctx); err != nil {
			return err
		}
	}
}

func (worker *Worker) ReconcileOnce(ctx context.Context) error {
	worker.defaults()
	if worker.Starter == nil || worker.Queries == nil || (worker.Effect == nil && worker.ReleaseEffect == nil && worker.VerifierEffect == nil) || worker.Capacity == nil {
		return fmt.Errorf("workflow action worker is not configured")
	}
	now := worker.Now().UTC()
	due, err := worker.Queries.ListDueIssueWorkflowActions(ctx, db.ListDueIssueWorkflowActionsParams{Now: pgTime(now), BatchSize: worker.BatchSize})
	if err != nil {
		return err
	}
	for _, candidate := range due {
		if err := worker.reconcileCandidate(ctx, candidate); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
	}
	return nil
}

func (worker *Worker) reconcileCandidate(ctx context.Context, candidate db.IssueWorkflowActionReceipt) error {
	// Release effects are an explicitly configured execution lane. A worker that
	// has no release adapter must leave the durable receipt admitted for a
	// capable worker instead of claiming it and spending reconciliation budget.
	if isReleaseProviderAction(candidate.ActionType) && worker.ReleaseEffect == nil {
		return nil
	}
	if isReleaseVerificationAction(candidate.ActionType) && worker.VerifierEffect == nil {
		return nil
	}
	token := pgUUID(uuid.New())
	var action db.IssueWorkflowActionReceipt
	err := workflowstore.InIssueWriteTransaction(ctx, worker.Starter, worker.Queries, candidate.WorkspaceID, candidate.IssueID, func(write *workflowstore.IssueWrite) error {
		claimNow := worker.Now().UTC()
		var err error
		action, err = write.ClaimAction(ctx, db.ClaimIssueWorkflowActionExactParams{
			LeaseToken: token, LeaseExpiresAt: pgTime(claimNow.Add(worker.LeaseDuration)), WorkspaceID: candidate.WorkspaceID,
			IssueID: candidate.IssueID, ID: candidate.ID, ExpectedRevision: candidate.Revision, Now: pgTime(claimNow),
		})
		return err
	})
	if err != nil {
		return err
	}
	if action.ActionType == "finalize_non_live" {
		return worker.finalizeNonLive(ctx, action)
	}
	if action.ActionType == "resume_exact_operation" || action.ActionType == "replan_exact_operation" {
		return worker.settleOwnerDecision(ctx, action)
	}
	if isReleaseProviderAction(action.ActionType) {
		return worker.reconcileRelease(ctx, action)
	}

	effect := worker.Effect
	if isReleaseVerificationAction(action.ActionType) {
		effect = worker.VerifierEffect
	}
	observed, observeErr := effect.Observe(ctx, action)
	if observeErr != nil || observed.Kind == ObservationUnknown {
		return worker.markUnknown(ctx, action, errorText(observeErr, "effect observation unknown"))
	}
	if observed.Kind == ObservationFound {
		return worker.settle(ctx, action, observed)
	}
	if action.EffectAttemptCount >= action.MaxAttempts {
		return worker.deadLetter(ctx, action, "effect attempt limit reached without an observed effect")
	}

	available, err := worker.Capacity.Available(ctx, action)
	if err != nil {
		return worker.markUnknown(ctx, action, err.Error())
	}
	if !available {
		return worker.deferAction(ctx, action, "capacity unavailable")
	}

	var begun db.IssueWorkflowActionReceipt
	err = workflowstore.InIssueWriteTransaction(ctx, worker.Starter, worker.Queries, action.WorkspaceID, action.IssueID, func(write *workflowstore.IssueWrite) error {
		beginNow := worker.Now().UTC()
		var err error
		begun, err = write.BeginActionAttempt(ctx, db.BeginIssueWorkflowActionAttemptParams{
			Now: pgTime(beginNow), WorkspaceID: action.WorkspaceID, IssueID: action.IssueID, ID: action.ID,
			ExpectedRevision: action.Revision, LeaseToken: action.LeaseToken,
		})
		return err
	})
	if err != nil {
		return err
	}

	applied, applyErr := effect.Apply(ctx, begun)
	if applyErr == nil && applied.Kind == ObservationFound {
		return worker.settle(ctx, begun, applied)
	}
	if applied.Kind == ObservationUnknown || errors.Is(applyErr, ErrEffectUnknown) {
		return worker.markUnknown(ctx, begun, errorText(applyErr, "effect application uncertain"))
	}
	if begun.EffectAttemptCount >= begun.MaxAttempts {
		return worker.deadLetter(ctx, begun, errorText(applyErr, "effect attempt failed"))
	}
	return worker.deferAction(ctx, begun, errorText(applyErr, "effect attempt failed"))
}

func isReleaseProviderAction(actionType string) bool {
	switch actionType {
	case "deploy_exact_release", "expose_exact_release", "rollback_exact_release":
		return true
	default:
		return false
	}
}

func isReleaseVerificationAction(actionType string) bool {
	return actionType == "verify_runtime_exact_release" || actionType == "verify_business_exact_release" || actionType == "monitor_exact_release" || actionType == "verify_restoration_exact_release"
}

func (worker *Worker) reconcileRelease(ctx context.Context, action db.IssueWorkflowActionReceipt) error {
	effect := worker.ReleaseEffect
	if effect == nil {
		return nil
	}
	observed, observeErr := effect.Observe(ctx, action)
	if observeErr != nil || observed.Kind == ObservationUnknown {
		return worker.markUnknown(ctx, action, errorText(observeErr, "release effect observation unknown"))
	}
	if observed.Kind == ObservationFound {
		return worker.settleRelease(ctx, action, observed)
	}
	if action.EffectAttemptCount >= action.MaxAttempts {
		return worker.deadLetter(ctx, action, "release effect attempt limit reached without an observed effect")
	}
	available, err := worker.Capacity.Available(ctx, action)
	if err != nil {
		return worker.markUnknown(ctx, action, err.Error())
	}
	if !available {
		return worker.deferAction(ctx, action, "release capacity unavailable")
	}
	begun, err := worker.beginReleaseAttempt(ctx, action)
	if err != nil {
		return err
	}
	applied, applyErr := effect.Apply(ctx, begun)
	if applyErr == nil && applied.Kind == ObservationFound {
		return worker.settleRelease(ctx, begun, applied)
	}
	if applied.Kind == ObservationUnknown || errors.Is(applyErr, ErrEffectUnknown) || errors.Is(applyErr, ErrEffectMismatch) {
		return worker.markUnknown(ctx, begun, errorText(applyErr, "release effect application uncertain"))
	}
	if begun.EffectAttemptCount >= begun.MaxAttempts {
		return worker.deadLetter(ctx, begun, errorText(applyErr, "release effect attempt failed"))
	}
	return worker.deferAction(ctx, begun, errorText(applyErr, "release effect attempt failed"))
}

func (worker *Worker) beginReleaseAttempt(ctx context.Context, action db.IssueWorkflowActionReceipt) (db.IssueWorkflowActionReceipt, error) {
	snapshot, input, actorIDs, err := releaseActionInput(action)
	if err != nil {
		return db.IssueWorkflowActionReceipt{}, err
	}
	var begun db.IssueWorkflowActionReceipt
	err = workflowstore.InReleaseAdmissionTransaction(ctx, worker.Starter, worker.Queries, action.WorkspaceID, action.IssueID, actorIDs, func(write *workflowstore.IssueWrite) error {
		beginNow := worker.Now().UTC()
		begun, err = write.BeginActionAttempt(ctx, db.BeginIssueWorkflowActionAttemptParams{Now: pgTime(beginNow), WorkspaceID: action.WorkspaceID, IssueID: action.IssueID, ID: action.ID, ExpectedRevision: action.Revision, LeaseToken: action.LeaseToken})
		if err != nil {
			return err
		}
		var kind workflowstatev2.CommandKind
		switch action.ActionType {
		case "deploy_exact_release":
			kind = workflowstatev2.CommandStartDeployment
		case "verify_runtime_exact_release":
			kind = workflowstatev2.CommandBeginVerification
		default:
			return nil
		}
		_, err = worker.commandService().ExecuteInWrite(ctx, write, releasePrincipal(begun, snapshot, kind), releaseRequest(begun, snapshot, input, kind, snapshot.Expected))
		return err
	})
	return begun, err
}

func (worker *Worker) settleRelease(ctx context.Context, action db.IssueWorkflowActionReceipt, observed Observation) error {
	snapshot, input, actorIDs, err := releaseActionInput(action)
	if err != nil || json.Unmarshal(observed.Result, &input) != nil {
		return worker.markUnknown(ctx, action, "invalid release effect observation")
	}
	var projectionReceipt workflowcommand.Receipt
	err = workflowstore.InReleaseAdmissionTransaction(ctx, worker.Starter, worker.Queries, action.WorkspaceID, action.IssueID, actorIDs, func(write *workflowstore.IssueWrite) error {
		service := worker.commandService()
		expected := snapshot.Expected
		if action.ActionType == "deploy_exact_release" {
			start, executeErr := service.ExecuteInWrite(ctx, write, releasePrincipal(action, snapshot, workflowstatev2.CommandStartDeployment), releaseRequest(action, snapshot, input, workflowstatev2.CommandStartDeployment, expected))
			if executeErr != nil {
				return executeErr
			}
			expected = allReceiptRevisions(start)
			projectionReceipt = start
		}
		if action.ActionType == "verify_runtime_exact_release" {
			started, executeErr := service.ExecuteInWrite(ctx, write, releasePrincipal(action, snapshot, workflowstatev2.CommandBeginVerification), releaseRequest(action, snapshot, input, workflowstatev2.CommandBeginVerification, expected))
			if executeErr != nil {
				return executeErr
			}
			expected = allReceiptRevisions(started)
			projectionReceipt = started
		}
		kind := map[string]workflowstatev2.CommandKind{
			"deploy_exact_release": workflowstatev2.CommandRecordDeployment, "expose_exact_release": workflowstatev2.CommandRecordExposure,
			"verify_runtime_exact_release": workflowstatev2.CommandVerifyRuntime, "verify_business_exact_release": workflowstatev2.CommandVerifyBusiness,
			"rollback_exact_release": workflowstatev2.CommandRollbackSucceeded,
		}[action.ActionType]
		receipt, executeErr := service.ExecuteInWrite(ctx, write, releasePrincipal(action, snapshot, kind), releaseRequest(action, snapshot, input, kind, expected))
		if executeErr != nil {
			return executeErr
		}
		projectionReceipt = receipt
		result, _ := json.Marshal(receipt)
		_, executeErr = write.SettleAction(ctx, db.SettleIssueWorkflowActionParams{Now: pgTime(worker.Now().UTC()), EffectResult: result, WorkspaceID: action.WorkspaceID, IssueID: action.IssueID, ID: action.ID, ExpectedRevision: action.Revision, LeaseToken: action.LeaseToken})
		return executeErr
	})
	if err == nil {
		worker.commandService().NotifyProjection(ctx, action.WorkspaceID, action.IssueID, projectionReceipt)
		worker.Wake()
		return nil
	}
	if action.EffectAttemptCount >= action.MaxAttempts {
		return worker.deadLetter(ctx, action, err.Error())
	}
	return worker.deferAction(ctx, action, err.Error())
}

type releaseWorkerSnapshot struct {
	OperationID       string                              `json:"operation_id"`
	BranchKey         string                              `json:"branch_key"`
	ReleaseID         string                              `json:"release_id"`
	CandidateID       string                              `json:"candidate_id"`
	CandidateRevision int64                               `json:"candidate_revision"`
	ScopeID           string                              `json:"scope_id"`
	ScopeRevision     int64                               `json:"scope_revision"`
	AcceptanceVersion int64                               `json:"acceptance_version"`
	TargetKey         string                              `json:"target_key"`
	DesiredGeneration int64                               `json:"desired_target_generation"`
	OperatorID        string                              `json:"release_operator_id"`
	VerifierID        string                              `json:"planned_verifier_id"`
	ArtifactDigest    string                              `json:"artifact_digest"`
	RuntimeDigest     string                              `json:"runtime_contract_digest"`
	Expected          map[string]workflowstatev2.Revision `json:"expected"`
}

func releaseActionInput(action db.IssueWorkflowActionReceipt) (releaseWorkerSnapshot, workflowcommand.ReleaseEffectResultInput, []pgtype.UUID, error) {
	var snapshot releaseWorkerSnapshot
	if json.Unmarshal(action.Payload, &snapshot) != nil || snapshot.ReleaseID == "" || snapshot.OperatorID == "" || snapshot.VerifierID == "" || snapshot.OperatorID == snapshot.VerifierID || len(snapshot.Expected) != 9 {
		return snapshot, workflowcommand.ReleaseEffectResultInput{}, nil, ErrEffectMismatch
	}
	typ, evidence := releaseObservationShape(action.ActionType)
	input := workflowcommand.ReleaseEffectResultInput{Type: typ, EffectKey: action.EffectKey.String, ProviderReceiptID: uuid.NewSHA1(uuid.NameSpaceOID, []byte(action.EffectKey.String)).String(), ReleaseID: snapshot.ReleaseID, CandidateID: snapshot.CandidateID, CandidateRevision: snapshot.CandidateRevision, ScopeID: snapshot.ScopeID, ScopeRevision: snapshot.ScopeRevision, AcceptanceVersion: snapshot.AcceptanceVersion, OperationID: snapshot.OperationID, BranchKey: snapshot.BranchKey, TargetKey: snapshot.TargetKey, TargetGeneration: snapshot.DesiredGeneration, ArtifactDigest: snapshot.ArtifactDigest, RuntimeContractDigest: snapshot.RuntimeDigest, Evidence: evidence}
	return snapshot, input, []pgtype.UUID{mustPGUUID(snapshot.OperatorID), mustPGUUID(snapshot.VerifierID)}, nil
}

func releasePrincipal(action db.IssueWorkflowActionReceipt, snapshot releaseWorkerSnapshot, kind workflowstatev2.CommandKind) workflowcommand.Principal {
	actor := snapshot.OperatorID
	if kind == workflowstatev2.CommandBeginVerification || kind == workflowstatev2.CommandVerifyRuntime || kind == workflowstatev2.CommandVerifyBusiness {
		actor = snapshot.VerifierID
	}
	return workflowcommand.Principal{ActorID: mustPGUUID(actor), ActorType: "workflow_action", ActionID: action.ID, ActionKey: action.ActionKey, LeaseToken: action.LeaseToken}
}

func releaseRequest(action db.IssueWorkflowActionReceipt, snapshot releaseWorkerSnapshot, input workflowcommand.ReleaseEffectResultInput, kind workflowstatev2.CommandKind, expected map[string]workflowstatev2.Revision) workflowcommand.Request {
	concerns := []workflowstatev2.Concern{workflowstatev2.ConcernEvidence, workflowstatev2.ConcernRelease}
	if kind == workflowstatev2.CommandStartDeployment {
		concerns = []workflowstatev2.Concern{workflowstatev2.ConcernEvidence, workflowstatev2.ConcernExecution, workflowstatev2.ConcernRelease}
	} else if kind == workflowstatev2.CommandVerifyRuntime {
		concerns = []workflowstatev2.Concern{workflowstatev2.ConcernEvidence}
	} else if kind == workflowstatev2.CommandVerifyBusiness {
		concerns = []workflowstatev2.Concern{workflowstatev2.ConcernEvidence, workflowstatev2.ConcernExecution, workflowstatev2.ConcernOutcome, workflowstatev2.ConcernRelease}
	} else if kind == workflowstatev2.CommandRollbackSucceeded || kind == workflowstatev2.CommandRestoreVerified {
		concerns = []workflowstatev2.Concern{workflowstatev2.ConcernEvidence, workflowstatev2.ConcernExecution, workflowstatev2.ConcernOutcome, workflowstatev2.ConcernRelease}
	}
	revisions := make([]workflowstatev2.ExpectedRevision, 0, len(concerns))
	for _, concern := range concerns {
		revisions = append(revisions, workflowstatev2.ExpectedRevision{Concern: concern, Revision: expected[string(concern)]})
	}
	return workflowcommand.Request{WorkspaceID: action.WorkspaceID, IssueID: action.IssueID, CommandKey: "action/" + util.UUIDToString(action.ID) + "/" + string(kind), Kind: kind, Expected: revisions, OperationID: snapshot.OperationID, BranchID: snapshot.BranchKey, CandidateID: snapshot.CandidateID, TargetID: snapshot.TargetKey, ReleaseEffect: &input}
}

func allReceiptRevisions(receipt workflowcommand.Receipt) map[string]workflowstatev2.Revision {
	return map[string]workflowstatev2.Revision{"outcome": receipt.Revisions.Outcome, "scope": receipt.Revisions.Scope, "authority": receipt.Revisions.Authority, "execution": receipt.Revisions.Execution, "review": receipt.Revisions.Review, "release": receipt.Revisions.Release, "attention": receipt.Revisions.Attention, "evidence": receipt.Revisions.Evidence, "budget": receipt.Revisions.Budget}
}

func mustPGUUID(value string) pgtype.UUID {
	id, err := uuid.Parse(value)
	if err != nil {
		return pgtype.UUID{}
	}
	return pgUUID(id)
}

func (worker *Worker) settleOwnerDecision(ctx context.Context, action db.IssueWorkflowActionReceipt) error {
	var payload struct {
		DecisionID       string                              `json:"decision_id"`
		DecisionRevision workflowstatev2.Revision            `json:"decision_revision"`
		Outcome          workflowstatev2.DecisionOutcome     `json:"outcome"`
		OperationID      string                              `json:"operation_id"`
		Expected         map[string]workflowstatev2.Revision `json:"expected"`
	}
	if json.Unmarshal(action.Payload, &payload) != nil || payload.DecisionID == "" || payload.DecisionRevision == 0 || payload.OperationID == "" || (payload.Outcome != workflowstatev2.DecisionApprove && payload.Outcome != workflowstatev2.DecisionReject) {
		return worker.markUnknown(ctx, action, "invalid owner decision continuation payload")
	}
	if action.EffectAttemptCount >= action.MaxAttempts {
		return worker.deadLetter(ctx, action, "owner decision continuation attempt limit reached")
	}
	var begun db.IssueWorkflowActionReceipt
	err := workflowstore.InIssueWriteTransaction(ctx, worker.Starter, worker.Queries, action.WorkspaceID, action.IssueID, func(write *workflowstore.IssueWrite) error {
		var err error
		begun, err = write.BeginActionAttempt(ctx, db.BeginIssueWorkflowActionAttemptParams{Now: pgTime(worker.Now().UTC()), WorkspaceID: action.WorkspaceID, IssueID: action.IssueID, ID: action.ID, ExpectedRevision: action.Revision, LeaseToken: action.LeaseToken})
		return err
	})
	if err != nil {
		return err
	}
	expected := []workflowstatev2.ExpectedRevision{
		{Concern: workflowstatev2.ConcernExecution, Revision: payload.Expected[string(workflowstatev2.ConcernExecution)]},
		{Concern: workflowstatev2.ConcernAttention, Revision: payload.Expected[string(workflowstatev2.ConcernAttention)]},
	}
	service := worker.commandService()
	var projectionReceipt workflowcommand.Receipt
	err = workflowstore.InIssueWriteTransaction(ctx, worker.Starter, worker.Queries, begun.WorkspaceID, begun.IssueID, func(write *workflowstore.IssueWrite) error {
		receipt, err := service.ExecuteInWrite(ctx, write, workflowcommand.Principal{ActorID: worker.ControllerID, ActorType: "internal_controller", ActionID: begun.ID, ActionKey: begun.ActionKey, LeaseToken: begun.LeaseToken}, workflowcommand.Request{
			WorkspaceID: begun.WorkspaceID, IssueID: begun.IssueID, CommandKey: "action/" + util.UUIDToString(begun.ID) + "/settle-owner-decision",
			Kind: workflowstatev2.CommandSettleDecisionEffect, DecisionID: payload.DecisionID, ExpectedDecisionRevision: payload.DecisionRevision,
			DecisionOutcome: payload.Outcome, OperationID: payload.OperationID, Expected: expected,
		})
		if err != nil {
			return err
		}
		projectionReceipt = receipt
		result, _ := json.Marshal(receipt)
		_, err = write.SettleAction(ctx, db.SettleIssueWorkflowActionParams{Now: pgTime(worker.Now().UTC()), EffectResult: result, WorkspaceID: begun.WorkspaceID, IssueID: begun.IssueID, ID: begun.ID, ExpectedRevision: begun.Revision, LeaseToken: begun.LeaseToken})
		return err
	})
	if err == nil {
		service.NotifyProjection(ctx, begun.WorkspaceID, begun.IssueID, projectionReceipt)
		return nil
	}
	if begun.EffectAttemptCount >= begun.MaxAttempts {
		return worker.deadLetter(ctx, begun, err.Error())
	}
	return worker.deferAction(ctx, begun, err.Error())
}

func (worker *Worker) finalizeNonLive(ctx context.Context, action db.IssueWorkflowActionReceipt) error {
	var payload struct {
		OperationID        string                              `json:"operation_id"`
		Expected           map[string]workflowstatev2.Revision `json:"expected"`
		RequiresDeployment bool                                `json:"requires_deployment"`
		SourcePlanRevision workflowstatev2.Revision            `json:"source_plan_revision"`
	}
	if json.Unmarshal(action.Payload, &payload) != nil || payload.OperationID == "" || payload.RequiresDeployment || payload.SourcePlanRevision == 0 {
		return worker.markUnknown(ctx, action, "invalid non-live finalizer payload")
	}
	var begun db.IssueWorkflowActionReceipt
	err := workflowstore.InIssueWriteTransaction(ctx, worker.Starter, worker.Queries, action.WorkspaceID, action.IssueID, func(write *workflowstore.IssueWrite) error {
		var err error
		begun, err = write.BeginActionAttempt(ctx, db.BeginIssueWorkflowActionAttemptParams{Now: pgTime(worker.Now().UTC()), WorkspaceID: action.WorkspaceID, IssueID: action.IssueID, ID: action.ID, ExpectedRevision: action.Revision, LeaseToken: action.LeaseToken})
		return err
	})
	if err != nil {
		return err
	}
	concerns := []workflowstatev2.Concern{workflowstatev2.ConcernScope, workflowstatev2.ConcernReview, workflowstatev2.ConcernEvidence, workflowstatev2.ConcernOutcome, workflowstatev2.ConcernExecution, workflowstatev2.ConcernRelease}
	expected := make([]workflowstatev2.ExpectedRevision, 0, len(concerns))
	for _, concern := range concerns {
		revision, ok := payload.Expected[string(concern)]
		if !ok {
			return worker.markUnknown(ctx, begun, "non-live finalizer omitted a causal revision")
		}
		expected = append(expected, workflowstatev2.ExpectedRevision{Concern: concern, Revision: revision})
	}
	receipt, err := worker.commandService().Execute(ctx, workflowcommand.Principal{ActorID: worker.ControllerID, ActorType: "internal_controller", ActionID: begun.ID, ActionKey: begun.ActionKey, LeaseToken: begun.LeaseToken}, workflowcommand.Request{WorkspaceID: begun.WorkspaceID, IssueID: begun.IssueID, CommandKey: "action/" + util.UUIDToString(begun.ID) + "/complete-non-live", Kind: workflowstatev2.CommandCompleteNonLive, OperationID: payload.OperationID, Expected: expected})
	if err != nil {
		if begun.EffectAttemptCount >= begun.MaxAttempts {
			return worker.deadLetter(ctx, begun, err.Error())
		}
		return worker.deferAction(ctx, begun, err.Error())
	}
	result, _ := json.Marshal(receipt)
	return worker.settle(ctx, begun, Observation{Kind: ObservationFound, ReceiptRevision: begun.Revision, Result: result})
}

func (worker *Worker) settle(ctx context.Context, action db.IssueWorkflowActionReceipt, observed Observation) error {
	revision := action.Revision
	if observed.ReceiptRevision != 0 {
		revision = observed.ReceiptRevision
	}
	result := observed.Result
	if len(result) == 0 {
		result, _ = json.Marshal(map[string]string{"task_id": util.UUIDToString(observed.TaskID)})
	}
	return workflowstore.InIssueWriteTransaction(ctx, worker.Starter, worker.Queries, action.WorkspaceID, action.IssueID, func(write *workflowstore.IssueWrite) error {
		now := worker.Now().UTC()
		_, err := write.SettleAction(ctx, db.SettleIssueWorkflowActionParams{Now: pgTime(now), EffectResult: result, WorkspaceID: action.WorkspaceID, IssueID: action.IssueID, ID: action.ID, ExpectedRevision: revision, LeaseToken: action.LeaseToken})
		return err
	})
}

func (worker *Worker) deferAction(ctx context.Context, action db.IssueWorkflowActionReceipt, reason string) error {
	return workflowstore.InIssueWriteTransaction(ctx, worker.Starter, worker.Queries, action.WorkspaceID, action.IssueID, func(write *workflowstore.IssueWrite) error {
		now := worker.Now().UTC()
		_, err := write.DeferAction(ctx, db.DeferIssueWorkflowActionExactParams{NotBefore: pgTime(now.Add(worker.RetryDelay)), LastError: reason, WorkspaceID: action.WorkspaceID, IssueID: action.IssueID, ID: action.ID, ExpectedRevision: action.Revision, LeaseToken: action.LeaseToken, Now: pgTime(now)})
		return err
	})
}

func (worker *Worker) markUnknown(ctx context.Context, action db.IssueWorkflowActionReceipt, reason string) error {
	service := worker.commandService()
	return workflowstore.InIssueWriteTransaction(ctx, worker.Starter, worker.Queries, action.WorkspaceID, action.IssueID, func(write *workflowstore.IssueWrite) error {
		now := worker.Now().UTC()
		failed, err := write.RecordActionObservationFailure(ctx, db.RecordIssueWorkflowActionObservationFailureParams{Now: pgTime(now), NotBefore: pgTime(now.Add(worker.RetryDelay)), LastError: reason, WorkspaceID: action.WorkspaceID, IssueID: action.IssueID, ID: action.ID, ExpectedRevision: action.Revision, LeaseToken: action.LeaseToken})
		if err != nil || failed.State != "dead_letter" {
			return err
		}
		return worker.technicalStopInWrite(ctx, write, service, action, reason)
	})
}

func (worker *Worker) deadLetter(ctx context.Context, action db.IssueWorkflowActionReceipt, reason string) error {
	service := worker.commandService()
	return workflowstore.InIssueWriteTransaction(ctx, worker.Starter, worker.Queries, action.WorkspaceID, action.IssueID, func(write *workflowstore.IssueWrite) error {
		now := worker.Now().UTC()
		if _, err := write.DeadLetterAction(ctx, db.DeadLetterIssueWorkflowActionExactParams{Now: pgTime(now), LastError: reason, WorkspaceID: action.WorkspaceID, IssueID: action.IssueID, ID: action.ID, ExpectedRevision: action.Revision, LeaseToken: action.LeaseToken}); err != nil {
			return err
		}
		return worker.technicalStopInWrite(ctx, write, service, action, reason)
	})
}

func (worker *Worker) technicalStopInWrite(ctx context.Context, write *workflowstore.IssueWrite, service workflowcommand.Service, action db.IssueWorkflowActionReceipt, reason string) error {
	head, err := write.GetConcernHead(ctx, db.GetIssueWorkflowConcernHeadParams{WorkspaceID: action.WorkspaceID, IssueID: action.IssueID, ConcernType: string(workflowstatev2.ConcernExecution)})
	if err != nil {
		return err
	}
	_, err = service.ExecuteInWrite(ctx, write, workflowcommand.Principal{ActorID: worker.ControllerID, ActorType: "internal_controller"}, workflowcommand.Request{
		WorkspaceID: action.WorkspaceID, IssueID: action.IssueID, CommandKey: "action/" + util.UUIDToString(action.ID) + "/dead-letter",
		Kind: workflowstatev2.CommandTechnicalStop, OperationID: util.UUIDToString(action.OperationID), Detail: reason,
		Expected: []workflowstatev2.ExpectedRevision{{Concern: workflowstatev2.ConcernExecution, Revision: workflowstatev2.Revision(head.Revision)}},
	})
	return err
}

func (worker *Worker) commandService() workflowcommand.Service {
	service := worker.CommandService
	service.ControllerID = worker.ControllerID
	service.Now = worker.Now
	return service
}

func (worker *Worker) defaults() {
	worker.wakeMu.Lock()
	defer worker.wakeMu.Unlock()
	if worker.Now == nil {
		worker.Now = time.Now
	}
	if worker.LeaseDuration <= 0 {
		worker.LeaseDuration = time.Minute
	}
	if worker.RetryDelay <= 0 {
		worker.RetryDelay = 5 * time.Second
	}
	if worker.PollInterval <= 0 {
		worker.PollInterval = time.Second
	}
	if worker.BatchSize <= 0 {
		worker.BatchSize = 25
	}
	if worker.wake == nil {
		worker.wake = make(chan struct{}, 1)
	}
}

func pgTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}
func pgUUID(value uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: value, Valid: true} }
func errorText(err error, fallback string) string {
	if err != nil {
		return err.Error()
	}
	return fallback
}
