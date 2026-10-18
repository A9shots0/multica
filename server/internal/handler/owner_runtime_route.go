package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func ownerRuntimeWorkflowBriefing(plan db.IssueWorkflowPlan, agentID, issueID string) string {
	route, err := service.ParseOwnerRuntimeRoute(plan)
	if err != nil || route == nil {
		return ""
	}
	target, err := route.Target(plan.CurrentState)
	if err != nil || target.AgentID != agentID {
		return ""
	}
	result := ""
	switch plan.CurrentState {
	case "queued", "working", "correction":
		result = "If queued, first record task_started. Inspect the actual candidate files on this host. Record implementation_ready with --artifact-evidence-file containing {schema_version:1,contract:file_manifest.v1,artifacts:[{path,declared_sha256,observed_sha256}],candidate_sha256}. All hashes must be actual lowercase SHA256. Sort unique paths, then hash UTF-8 path + NUL + observed_sha256 + newline for each file to obtain candidate_sha256. Git is not required."
	case "technical_review":
		result = "Independently reread the exact candidate files. Record review_passed with review and build gates, or review_failed with the existing failed-criterion/evidence/required-correction fields."
	case "deploying":
		result = "Deploy only the independently reviewed candidate. Record deployment_succeeded with deployment and exposure gates and the actual deployed revision."
	case "production_verifying":
		result = "Independently verify the exact deployed candidate and actual business flow. Record production_verified with runtime, business and regression gates, or production_failed with the concrete failure."
	case "rolling_back":
		result = "Restore the actual captured previous release. Record rollback_succeeded only after verification, with rollback and restoration gates and the actual restored revision."
	}
	interruption := "No interruption is authorized. Stop with an actual owner decision if deployment cannot preserve availability."
	if route.AllowBriefDeploymentInterruption {
		interruption = "A brief deployment-only interruption is authorized; no offline maintenance, holds or device reservations."
	}
	return "## Owner-declared ordinary workflow\n\nYou have authority only for this exact issue, role, runtime and host. Do not dispatch or mention another agent to hand off work. Use `multica issue workflow-event " + issueID + " <event> --reason \"<actual evidence>\"`. " + result + "\n" + interruption + "\nFor passed review/release/verification/rollback, use --candidate <canonical candidate SHA256> and --stage-evidence-file <JSON file>. Packet: {candidate_sha256,build_manifest_sha256,target_key,deployed_revision,gates:{<assigned gate>:{state:passed,refs:[<actual evidence reference>],observed_at:<RFC3339 timestamp observed during this task>,limitations:[]}}}. Supply only your assigned gates. Keep the reviewed build-manifest digest and exact deployment target across stages. Read candidate manifest from task context; never infer passed gates from task completion. If one genuine owner decision is needed, report owner_decision_required with --question, --recommendation, --consequence and --option, then stop. Technical failures are dependency_blocked, not invented owner questions. After your accepted result, finish the task; the controller queues the next role only after your process finishes. No global leader or template substitution is authorized."
}

func validateOwnerOrLegacyArtifactRequest(req issueWorkflowEventRequest, ownerRoute bool) error {
	if !ownerRoute {
		if req.StageEvidence != nil {
			return errors.New("ordinary stage evidence requires an admitted owner route")
		}
		if err := validateWorkflowArtifactEvidenceRequest(req); err != nil {
			return err
		}
		if err := validateWorkflowTestEvidenceRequest(req); err != nil {
			return err
		}
		return validateWorkflowBlastRadiusReviewRequest(req)
	}
	if req.TestEvidence != nil {
		return errors.New("test_evidence requires a legacy (non owner-route) review; owner-declared workflows report build proof through their assigned stage_evidence gate instead")
	}
	if req.BlastRadiusReview != nil {
		return errors.New("blast_radius_review requires a legacy (non owner-route) review; owner-declared workflows report review evidence through their assigned stage_evidence gate instead")
	}
	if req.Event == "implementation_ready" {
		raw, _ := json.Marshal(req.ArtifactEvidence)
		_, err := service.ValidateFileManifest(raw)
		return err
	}
	if req.ArtifactEvidence != nil {
		return errors.New("file manifest is only valid for implementation_ready")
	}
	return nil
}

// Called only inside the existing human/recipient/revision/idle intake gate.
// Assignment is never visible without its exact plan, receipt and first task.
func (h *Handler) admitOwnerRuntimeRoute(w http.ResponseWriter, r *http.Request, tx pgx.Tx, issue db.Issue, intakeID pgtype.UUID, details map[string]string, req ownerDirectiveRequest) {
	q := h.Queries.WithTx(tx)
	if _, err := q.GetIssueWorkflowPlan(r.Context(), db.GetIssueWorkflowPlanParams{IssueID: issue.ID, WorkspaceID: issue.WorkspaceID}); !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 409, "an existing workflow cannot be replaced by this intake")
		return
	}
	directiveID := dbid.NewV7()
	route := service.OwnerRuntimeRoute{OwnerRuntimeRouteRequest: *req.OwnerRuntimeRoute, Version: 1, WorkspaceID: uuidToString(issue.WorkspaceID), IssueID: uuidToString(issue.ID), OwnerID: requestUserID(r), DirectiveID: uuidToString(directiveID), IssueRevision: issue.Revision}
	route.ScopeSHA256 = service.OwnerRuntimeScopeDigest(issue)
	if err := h.TaskService.ValidateOwnerRuntimeTargets(r.Context(), q, route); err != nil {
		writeError(w, 409, err.Error())
		return
	}
	comment, err := q.CreateComment(r.Context(), db.CreateCommentParams{ID: dbid.NewV7(), IssueID: issue.ID, WorkspaceID: issue.WorkspaceID, AuthorType: "member", AuthorID: parseUUID(route.OwnerID), Content: chatIntakeContinuationContent(req.IntakeID, details["request"], req.Content), Type: "comment"})
	if err != nil {
		writeError(w, 500, "owner direction could not be recorded")
		return
	}
	directive, err := q.CreateIssueOwnerDirective(r.Context(), db.CreateIssueOwnerDirectiveParams{ID: directiveID, WorkspaceID: issue.WorkspaceID, IssueID: issue.ID, DirectiveType: "correct", ExpectedIssueRevision: req.ExpectedIssueRevision, Content: req.Content, State: "applied", ActorID: parseUUID(route.OwnerID), CommentID: comment.ID})
	if err != nil {
		writeError(w, 500, "owner route receipt could not be recorded")
		return
	}
	raw, _ := json.Marshal(route)
	plan, err := q.InsertOwnerRuntimeWorkflowPlan(r.Context(), db.InsertOwnerRuntimeWorkflowPlanParams{IssueID: issue.ID, WorkspaceID: issue.WorkspaceID, SquadID: parseUUID(route.SquadID), ImplementerID: parseUUID(route.Implementation.AgentID), ReviewerID: parseUUID(route.Review.AgentID), ReleaseAgentID: parseUUID(route.Release.AgentID), VerifierID: parseUUID(route.Verification.AgentID), OwnerRuntimeRoute: raw})
	if err != nil {
		writeError(w, 409, "workflow already admitted or changed")
		return
	}
	if _, err = tx.Exec(r.Context(), `UPDATE issue SET assignee_type='squad',assignee_id=$2,status='todo',revision=revision+1,updated_at=now() WHERE id=$1`, issue.ID, plan.SquadID); err != nil {
		writeError(w, 409, "route assignment unavailable")
		return
	}
	issue, err = q.GetIssue(r.Context(), issue.ID)
	if err != nil {
		writeError(w, 500, "route issue unavailable")
		return
	}
	task, err := h.TaskService.EnqueueOwnerRuntimeTask(r.Context(), tx, issue, plan, db.AgentTaskQueue{}, pgtype.UUID{}, comment.ID)
	if err != nil {
		writeError(w, 409, "route could not be admitted: "+err.Error())
		return
	}
	picture, ownerItem, err := h.TaskService.StoreOwnerRuntimeProjection(r.Context(), tx, plan, task, "")
	if err != nil {
		writeError(w, 409, err.Error())
		return
	}
	receipt := map[string]any{"id": uuidToString(directive.ID), "issue_id": uuidToString(issue.ID), "revision": directive.Revision, "directive": "correct", "state": "applied", "comment_id": uuidToString(comment.ID), "issue_revision": issue.Revision, "task_id": uuidToString(task.ID), "task_state": task.Status, "intake_id": req.IntakeID}
	encodedReceipt, _ := json.Marshal(receipt)
	requestJSON, _ := json.Marshal(req.OwnerRuntimeRoute)
	details["intake_state"], details["task_id"], details["owner_content"], details["directive_receipt"] = "continued", uuidToString(task.ID), req.Content, string(encodedReceipt)
	details["owner_runtime_route"], details["owner_category"], details["required_action"] = string(requestJSON), "update", "follow_work"
	encoded, _ := json.Marshal(details)
	if _, err = tx.Exec(r.Context(), `UPDATE inbox_item SET details=$2,severity='info',body=$3,read=true WHERE id=$1`, intakeID, encoded, "Your declared implementation, independent review, release and live verification route was recorded. The first worker is queued."); err != nil {
		writeError(w, 500, "intake receipt unavailable")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "route could not be committed")
		return
	}
	h.publish(protocol.EventInboxRead, uuidToString(issue.WorkspaceID), "member", requestUserID(r), map[string]any{"item_id": uuidToString(intakeID), "recipient_id": requestUserID(r)})
	h.TaskService.PublishOwnerIntakeTask(r.Context(), task)
	if h.TaskService.OnLegacyWorkflowProjection != nil {
		h.TaskService.OnLegacyWorkflowProjection(r.Context(), issue.WorkspaceID, issue.ID, picture.Revision, ownerItem)
	}
	h.publish(protocol.EventIssueUpdated, uuidToString(issue.WorkspaceID), "member", requestUserID(r), map[string]any{"issue": issueToResponse(issue, h.getIssuePrefix(r.Context(), issue.WorkspaceID)), "status_changed": true})
	writeJSON(w, http.StatusCreated, receipt)
}
