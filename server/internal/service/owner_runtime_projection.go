package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/internal/workflowprojection"
	"github.com/multica-ai/multica/server/internal/workflowstore"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// StoreOwnerRuntimeProjection adapts accepted plan/events only. It does not
// create normalized workflow records or turn task completion into acceptance.
func (s *TaskService) StoreOwnerRuntimeProjection(ctx context.Context, tx pgx.Tx, plan db.IssueWorkflowPlan, task db.AgentTaskQueue, stopReason string) (db.IssueCurrentPicture, *db.InboxItem, error) {
	if tx == nil {
		return db.IssueCurrentPicture{}, nil, errors.New("owner projection requires transaction support")
	}
	q := db.New(tx)
	route, err := ParseOwnerRuntimeRoute(plan)
	if err != nil || route == nil {
		return db.IssueCurrentPicture{}, nil, errors.New("owner projection requires its persisted route")
	}
	expected := int64(0)
	if prior, err := q.GetIssueCurrentPicture(ctx, db.GetIssueCurrentPictureParams{IssueID: plan.IssueID, WorkspaceID: plan.WorkspaceID}); err == nil {
		var authority map[string]any
		if json.Unmarshal(prior.AuthoritySummary, &authority) != nil || authority["owner_runtime_directive"] != route.DirectiveID {
			return db.IssueCurrentPicture{}, nil, errors.New("owner route cannot overwrite another authoritative picture")
		}
		expected = prior.Revision
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return db.IssueCurrentPicture{}, nil, err
	}
	events, err := q.ListIssueWorkflowEvents(ctx, db.ListIssueWorkflowEventsParams{IssueID: plan.IssueID, WorkspaceID: plan.WorkspaceID})
	if err != nil {
		return db.IssueCurrentPicture{}, nil, err
	}
	params, err := OwnerRuntimeProjectionParams(plan, *route, task, events, expected, stopReason, time.Now().UTC())
	if err != nil {
		return db.IssueCurrentPicture{}, nil, err
	}
	picture, err := workflowstore.UpsertLegacyCurrentPicture(ctx, tx, params)
	if err != nil {
		return picture, nil, err
	}
	// The event writes the actionable question with its real workflow_event_id.
	// Do not create a second, non-existent normalized decision.
	if plan.CurrentState == "needs_owner" {
		return picture, nil, nil
	}
	if s.AdmitLegacyWorkflowProjection == nil {
		return picture, nil, errors.New("owner route Inbox projection is unavailable")
	}
	item, err := s.AdmitLegacyWorkflowProjection(ctx, q, plan.WorkspaceID, plan.IssueID, picture.Revision)
	return picture, item, err
}

func OwnerRuntimeProjectionParams(plan db.IssueWorkflowPlan, route OwnerRuntimeRoute, task db.AgentTaskQueue, events []db.IssueWorkflowEvent, expected int64, stopReason string, now time.Time) (db.UpsertIssueCurrentPictureParams, error) {
	revision := expected + 1
	object := func(v any) []byte { raw, _ := json.Marshal(v); return raw }
	ref := func(id string) map[string]any { return map[string]any{"id": id, "revision": int64(1)} }
	outcome := ref(route.DirectiveID)
	outcome["state"] = "open"
	scope := ref(route.DirectiveID)
	scope["requires_deployment"] = true
	execution := ref(util.UUIDToString(task.ID))
	execution["state"] = "received"
	authority := map[string]any{"owner_runtime_directive": route.DirectiveID, "owner_id": route.OwnerID}
	release := map[string]any{"state": "pending"}
	candidate := map[string]any{}
	target := map[string]any{}
	evidence := map[string]any{}
	next := map[string]any{"type": "task.completed"}
	actor := map[string]any{}
	decisions := []map[string]any{}
	incidents := []map[string]any{}
	dependencies := []map[string]any{}
	latest := map[string]any{}
	sources := []map[string]any{{"type": "owner_runtime_directive", "id": route.DirectiveID, "revision": int64(1)}}
	var manifestEvent db.IssueWorkflowEvent
	var digest string
	packets := map[string]OwnerRuntimeStageEvidence{}
	for _, event := range events {
		if event.EventType == "implementation_ready" {
			if d, err := ValidateFileManifest(event.Payload); err == nil {
				manifestEvent = event
				digest = d
				packets = map[string]OwnerRuntimeStageEvidence{}
			}
		}
		var packet OwnerRuntimeStageEvidence
		if json.Unmarshal(event.Payload, &packet) == nil && digest != "" && packet.CandidateSHA256 == digest {
			packets[event.EventType] = packet
		}
		latest = map[string]any{"id": util.UUIDToString(event.ID), "revision": int64(1), "record_type": "evidence", "change_code": "evidence_recorded", "result_code": "succeeded", "occurred_at": event.CreatedAt.Time.UTC().Format(time.RFC3339Nano)}
		sources = append(sources, map[string]any{"type": "owner_workflow_event", "id": util.UUIDToString(event.ID), "revision": int64(1)})
	}
	if digest != "" {
		candidate = ref(util.UUIDToString(manifestEvent.ID))
		candidate["source_revision"] = digest
		candidate["artifact_digest"] = digest
	}
	health := "unknown"
	outcomeState := "open"
	attention := "none"
	contract := int32(2)
	var releaseStatus *workflowprojection.ReleaseStatus
	if review, ok := packets["review_passed"]; ok {
		contract = 3
		releaseID := util.UUIDToString(manifestEvent.ID)
		release = map[string]any{"id": releaseID, "revision": plan.Revision, "candidate_id": releaseID, "candidate_revision": int64(1), "scope_revision": int64(1), "acceptance_version": int64(1), "authority_revision": int64(1), "target_key": review.TargetKey, "target_generation": int64(1), "artifact_digest": digest, "build_manifest_digest": review.BuildManifestSHA256, "state": "authorized"}
		missing := workflowprojection.ReleaseEvidenceGate{State: "missing", Refs: []string{}, Limitations: []string{}}
		releaseStatus = &workflowprojection.ReleaseStatus{SchemaVersion: 1, Phase: "releasing", Binding: workflowprojection.ReleaseBinding{ReleaseID: releaseID, ReleaseRevision: plan.Revision, CandidateID: releaseID, CandidateRevision: 1, SourceRevision: digest, ArtifactDigest: digest, BuildManifestDigest: review.BuildManifestSHA256, ScopeRevision: 1, AcceptanceVersion: 1, AuthorityRevision: 1, TargetKey: review.TargetKey, TargetGeneration: 1}, Gates: workflowprojection.ReleaseGates{Review: review.Gates["review"], Build: review.Gates["build"], Deployment: missing, Exposure: missing, Runtime: missing, Business: missing, Regression: missing, Rollback: missing, Restoration: missing}}
		if deployment, ok := packets["deployment_succeeded"]; ok {
			receipt := deployment.DeployedRevision
			releaseStatus.Binding.DeployedRevision = &receipt
			releaseStatus.Phase = "deployed_checking"
			release["state"] = "production_verifying"
			releaseStatus.Gates.Deployment = deployment.Gates["deployment"]
			releaseStatus.Gates.Exposure = deployment.Gates["exposure"]
			evidence["items"] = []map[string]any{{"id": releaseID, "evidence_type": "deployment", "applies": true, "provider_receipt_id": receipt}}
			target = map[string]any{"release_id": releaseID, "candidate_id": releaseID, "target_key": review.TargetKey, "generation": int64(1)}
		}
		if verification, ok := packets["production_verified"]; ok && plan.CurrentState == "live" {
			releaseStatus.Gates.Runtime = verification.Gates["runtime"]
			releaseStatus.Gates.Business = verification.Gates["business"]
			releaseStatus.Gates.Regression = verification.Gates["regression"]
			releaseStatus.Phase = "verified_live"
			release["state"] = "live"
			health = "healthy"
			outcomeState = "satisfied"
			execution["state"] = "completed"
			next = map[string]any{}
		}
		if plan.CurrentState == "rolling_back" {
			releaseStatus.Phase = "rolling_back"
			release["state"] = "rolling_back"
		}
		if rollback, ok := packets["rollback_succeeded"]; ok && plan.CurrentState == "rolled_back" {
			receipt := rollback.DeployedRevision
			releaseStatus.Binding.DeployedRevision = &receipt
			releaseStatus.Gates.Rollback = rollback.Gates["rollback"]
			releaseStatus.Gates.Restoration = rollback.Gates["restoration"]
			releaseStatus.Phase = "rolled_back"
			release["state"] = "restored"
			health = "restored"
			next = map[string]any{}
			evidence["items"] = []map[string]any{{"id": releaseID, "evidence_type": "deployment", "applies": true, "provider_receipt_id": receipt}}
		}
	}
	switch plan.CurrentState {
	case "queued", "working", "correction":
		if task.Status == "running" {
			// Completion is an event awaited from the active worker, not a
			// pending automatic action. Otherwise Board classifies this real
			// execution as waiting despite its authoritative active actor.
			next = map[string]any{}
			execution["state"] = "working"
			execution["task_id"] = util.UUIDToString(task.ID)
			execution["actor_type"] = "agent"
			execution["actor_id"] = util.UUIDToString(task.AgentID)
			actor = map[string]any{"actor_type": "agent", "actor_id": util.UUIDToString(task.AgentID)}
		}
	case "technical_review":
		execution["state"] = "in_review"
	case "needs_owner":
		execution["state"] = "paused"
		attention = "needs_owner"
		next = map[string]any{}
		for i := len(events) - 1; i >= 0; i-- {
			if events[i].EventType == "owner_decision_required" {
				decision := ref(util.UUIDToString(events[i].ID))
				var question map[string]any
				_ = json.Unmarshal(events[i].Payload, &question)
				decision["question"] = question["question"]
				decisions = append(decisions, decision)
				break
			}
		}
	case "blocked", "failed":
		if stopReason == "" {
			stopReason = "The declared workflow stopped; inspect its latest evidence before resuming."
		}
	}
	if stopReason != "" {
		execution["state"] = "technically_stopped"
		attention = "incident"
		next = map[string]any{}
		if releaseStatus != nil {
			releaseStatus.Phase = "technical_stop"
			releaseStatus.Stop = &workflowprojection.ReleaseStop{Code: "owner_workflow_stopped", Reason: stopReason}
		}
	}
	outcome["state"] = outcomeState
	document := workflowprojection.Document{ContractVersion: contract, OwnerSummarySchemaVersion: 1, ReleaseStatus: releaseStatus, OriginalBusinessRequest: map[string]any{}, Outcome: outcome, HistoricalCompletion: map[string]any{}, Scope: scope, Authority: authority, Execution: execution, ActiveBranches: []map[string]any{}, PrimaryBranch: map[string]any{}, Release: release, ObservedRelease: map[string]any{}, Health: health, Attention: map[string]any{}, CurrentMilestone: plan.CurrentState, LatestMeaningfulChange: latest, NextAutomaticAction: next, ActiveActor: actor, WaitReason: map[string]any{"reason": stopReason}, Candidate: candidate, Target: target, EvidenceSummary: evidence, AuthoritySummary: authority, UnresolvedDecisions: decisions, UnresolvedIncidents: incidents, UnresolvedDependencies: dependencies, Budgets: []map[string]any{}, Supersession: map[string]any{}, TerminalReason: stopReason, NextActor: map[string]any{}}
	raw := object(document)
	if contract == 3 && !workflowprojection.ValidReleaseStatusDocument(raw) {
		return db.UpsertIssueCurrentPictureParams{}, errors.New("owner workflow evidence cannot produce valid release truth")
	}
	summary := workflowprojection.OwnerIssueSummaryFromDocument(raw, revision, now)
	if summary.Stage == workflowprojection.OwnerStageStatusUnavailable {
		return db.UpsertIssueCurrentPictureParams{}, errors.New("owner workflow evidence cannot produce an authoritative stage")
	}
	return db.UpsertIssueCurrentPictureParams{WorkspaceID: plan.WorkspaceID, IssueID: plan.IssueID, ContractVersion: contract, Revision: revision, ScopeRevision: pgtype.Int8{Int64: 1, Valid: true}, AuthorityRevision: pgtype.Int8{Int64: 1, Valid: true}, OutcomeState: outcomeState, OutcomeOpen: outcomeState != "satisfied", ExecutionState: execution["state"].(string), ReleaseState: release["state"].(string), HealthState: health, AttentionState: attention, CurrentMilestone: plan.CurrentState, LatestMeaningfulChange: object(latest), NextAutomaticAction: object(next), ActiveActor: object(actor), WaitReason: object(document.WaitReason), Candidate: object(candidate), Target: object(target), EvidenceSummary: object(evidence), AuthoritySummary: object(authority), UnresolvedDecisions: object(decisions), UnresolvedIncidents: object(incidents), UnresolvedDependencies: object(dependencies), BudgetSummary: []byte(`[]`), SupersessionSummary: []byte(`{}`), TerminalReason: stopReason, NextActor: []byte(`{}`), SourceRevisions: object(sources), Document: raw, GeneratedAt: pgtype.Timestamptz{Time: now, Valid: true}, ExpectedRevision: expected}, nil
}
