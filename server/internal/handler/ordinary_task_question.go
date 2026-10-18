package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	googleuuid "github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/issuestatus"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Ordinary questions are explicit task reports, never inferred from comments.
// They reuse Inbox's durable owner-response receipt without creating a plan or
// declaring a business outcome complete.
type ordinaryTaskQuestionResponse struct {
	ID          string `json:"id"`
	TaskID      string `json:"task_id"`
	Question    string `json:"question"`
	Consequence string `json:"consequence"`
}

func (h *Handler) ordinaryTaskQuestionsByIssue(ctx context.Context, workspace pgtype.UUID, ids []pgtype.UUID) map[string]*ordinaryTaskQuestionResponse {
	result := map[string]*ordinaryTaskQuestionResponse{}
	rows, err := h.Queries.ListOpenOrdinaryTaskQuestions(ctx, db.ListOpenOrdinaryTaskQuestionsParams{WorkspaceID: workspace, IssueIds: ids})
	if err != nil {
		return result
	}
	for _, row := range rows {
		var details map[string]string
		if json.Unmarshal(row.Details, &details) != nil || details["question"] == "" {
			continue
		}
		id := uuidToString(row.IssueID)
		if result[id] == nil {
			result[id] = &ordinaryTaskQuestionResponse{ID: uuidToString(row.ID), TaskID: details["source_task_id"], Question: details["question"], Consequence: details["consequence"]}
		}
	}
	return result
}

func (h *Handler) postOrdinaryTaskQuestion(w http.ResponseWriter, r *http.Request, issue db.Issue, source db.AgentTaskQueue, req issueWorkflowEventRequest) {
	if r.Header.Get("X-Actor-Source") != "task_token" || req.OwnerDecision == nil || !source.AccountableUserID.Valid || source.ChatSessionID.Valid || source.AutopilotRunID.Valid || source.WorkflowActionID.Valid || source.WorkflowOperationID.Valid || source.SquadID.Valid {
		writeError(w, http.StatusForbidden, "an ordinary issue task with an accountable member is required")
		return
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "question unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	locked, err := q.LockIssueTaskAdmission(r.Context(), db.LockIssueTaskAdmissionParams{ID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if err != nil || locked.AssigneeType.String != "agent" || locked.AssigneeID != source.AgentID {
		writeError(w, http.StatusConflict, "task is no longer the assigned issue worker")
		return
	}
	// Match issue admission's lock order. A concurrent task completion/cancel
	// retains its lock; decline this optional report instead of waiting on it.
	var running bool
	if tx.QueryRow(r.Context(), `SELECT status='running' FROM agent_task_queue WHERE id=$1 FOR UPDATE SKIP LOCKED`, source.ID).Scan(&running) != nil || !running {
		writeError(w, http.StatusConflict, "question requires the currently running task")
		return
	}
	category := issuestatus.Effective(r.Context(), q, issue.WorkspaceID, locked.Status)
	if category != "todo" && category != "backlog" && category != "in_progress" {
		writeError(w, http.StatusConflict, "the issue is not open for ordinary worker questions")
		return
	}
	var guarded map[string]json.RawMessage
	if len(source.Context) > 0 && json.Unmarshal(source.Context, &guarded) != nil {
		writeError(w, http.StatusConflict, "task context is invalid")
		return
	}
	if guarded["guarded_dispatch"] != nil || guarded["workflow_assignment_v1"] != nil {
		writeError(w, http.StatusConflict, "use the task's authoritative workflow result")
		return
	}
	if _, err := q.GetIssueWorkflowPlan(r.Context(), db.GetIssueWorkflowPlanParams{IssueID: issue.ID, WorkspaceID: issue.WorkspaceID}); !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusConflict, "use the existing workflow plan")
		return
	}
	if _, err := q.GetIssueCurrentPicture(r.Context(), db.GetIssueCurrentPictureParams{IssueID: issue.ID, WorkspaceID: issue.WorkspaceID}); !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusConflict, "use the authoritative workflow controls")
		return
	}
	if _, err := q.GetMemberByUserAndWorkspace(r.Context(), db.GetMemberByUserAndWorkspaceParams{UserID: source.AccountableUserID, WorkspaceID: issue.WorkspaceID}); err != nil {
		writeError(w, http.StatusForbidden, "accountable member is unavailable")
		return
	}
	var superseded bool
	if tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM agent_task_queue WHERE issue_id=$1 AND (created_at,id)>($2,$3))`, issue.ID, source.CreatedAt, source.ID).Scan(&superseded) != nil || superseded {
		writeError(w, http.StatusConflict, "a newer task superseded this report")
		return
	}
	id := parseUUID(googleuuid.NewSHA1(googleuuid.NameSpaceURL, []byte("multica-task-owner-question:"+uuidToString(source.ID))).String())
	if prior, err := q.GetInboxItemInWorkspace(r.Context(), db.GetInboxItemInWorkspaceParams{ID: id, WorkspaceID: issue.WorkspaceID}); err == nil {
		var details map[string]string
		_ = json.Unmarshal(prior.Details, &details)
		if details["question"] != req.OwnerDecision.Question || details["recommendation"] != req.OwnerDecision.Recommendation || details["consequence"] != req.OwnerDecision.Consequence || details["options"] != strings.Join(req.OwnerDecision.Options, "\n") {
			writeError(w, http.StatusConflict, "this task already reported a different question")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"question_id": uuidToString(id), "inbox_item_id": uuidToString(id), "dispatch": "waiting_owner", "replay": true})
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "question lookup failed")
		return
	}
	open, err := q.ListOpenOrdinaryTaskQuestions(r.Context(), db.ListOpenOrdinaryTaskQuestionsParams{WorkspaceID: issue.WorkspaceID, IssueIds: []pgtype.UUID{issue.ID}})
	if err != nil || len(open) > 0 {
		writeError(w, http.StatusConflict, "answer the issue's existing owner question first")
		return
	}
	if tx.QueryRow(r.Context(), `UPDATE issue SET revision=revision+1,updated_at=now() WHERE id=$1 AND workspace_id=$2 RETURNING revision,updated_at`, issue.ID, issue.WorkspaceID).Scan(&locked.Revision, &locked.UpdatedAt) != nil {
		writeError(w, http.StatusInternalServerError, "question revision could not be recorded")
		return
	}
	details, _ := json.Marshal(map[string]string{"question_scope": "ordinary_task", "source_task_id": uuidToString(source.ID), "question": req.OwnerDecision.Question, "recommendation": req.OwnerDecision.Recommendation, "consequence": req.OwnerDecision.Consequence, "options": strings.Join(req.OwnerDecision.Options, "\n"), "intake_state": "pending", "owner_relevant": "true", "owner_category": "needs_you", "required_action": "answer_worker_question"})
	item, err := q.CreateInboxItem(r.Context(), db.CreateInboxItemParams{ID: id, WorkspaceID: issue.WorkspaceID, RecipientType: "member", RecipientID: source.AccountableUserID, Type: "workflow_owner_decision", Severity: "action_required", IssueID: issue.ID, Title: issue.Title, Body: pgtype.Text{String: req.OwnerDecision.Question, Valid: true}, ActorType: pgtype.Text{String: "agent", Valid: true}, ActorID: source.AgentID, Details: details})
	if err != nil || tx.Commit(r.Context()) != nil {
		writeError(w, http.StatusInternalServerError, "question could not be recorded")
		return
	}
	h.publish(protocol.EventInboxNew, uuidToString(issue.WorkspaceID), "agent", uuidToString(source.AgentID), map[string]any{"item": inboxToResponse(item)})
	response := issueToResponse(locked, h.getIssuePrefix(r.Context(), issue.WorkspaceID))
	response.OwnerQuestion = &ordinaryTaskQuestionResponse{ID: uuidToString(id), TaskID: uuidToString(source.ID), Question: req.OwnerDecision.Question, Consequence: req.OwnerDecision.Consequence}
	h.publish(protocol.EventIssueUpdated, uuidToString(issue.WorkspaceID), "agent", uuidToString(source.AgentID), map[string]any{"issue": response})
	writeJSON(w, http.StatusCreated, map[string]any{"question_id": uuidToString(id), "inbox_item_id": uuidToString(id), "dispatch": "waiting_owner", "replay": false})
}

// Delivered by the backend in ordinary task claims, including older daemons.
func ordinaryTaskQuestionBriefing(issueID string) string {
	return "## Owner Questions\n\nIf a real owner answer is required before work can continue, while this task is running execute `multica issue workflow-event " + issueID + " owner_decision_required --reason \"<why the answer is needed>\" --question \"<exact question>\" --recommendation \"<recommended choice and why>\" --consequence \"<what each choice changes>\" --option \"<choice>\"` (repeat --option for choices). Only a successful response containing inbox_item_id proves the question reached Inbox. After that receipt, stop work requiring the answer and finish this turn; the accountable member's Inbox answer queues the continuation after this turn ends. A member mention, issue comment or free-text final answer does not create an actionable owner question. Never claim Inbox delivery if the command failed, and never mark the issue Done merely because you asked a question. Existing authoritative workflow instructions take precedence."
}

// Invalidated questions stay in durable history. Derive their present action
// from the same source facts as Board/admission, without silently answering or
// editing the original receipt on a read request.
func (h *Handler) projectOrdinaryQuestionLifecycle(ctx context.Context, workspace pgtype.UUID, responses []InboxItemResponse) {
	ids := []pgtype.UUID{}
	for _, response := range responses {
		var details map[string]string
		if response.IssueID != nil && json.Unmarshal(response.Details, &details) == nil && details["question_scope"] == "ordinary_task" && details["intake_state"] == "pending" {
			ids = append(ids, parseUUID(*response.IssueID))
		}
	}
	if len(ids) == 0 {
		return
	}
	current, err := h.Queries.ListOpenOrdinaryTaskQuestions(ctx, db.ListOpenOrdinaryTaskQuestionsParams{WorkspaceID: workspace, IssueIds: ids})
	if err != nil {
		return
	}
	eligible := map[string]bool{}
	for _, question := range current {
		eligible[uuidToString(question.ID)] = true
	}
	for index := range responses {
		response := &responses[index]
		var details map[string]string
		if json.Unmarshal(response.Details, &details) != nil || details["question_scope"] != "ordinary_task" || details["intake_state"] != "pending" || eligible[response.ID] {
			continue
		}
		details["intake_state"], details["owner_category"], details["required_action"] = "invalidated", "history", "inspect_issue"
		response.Details, _ = json.Marshal(details)
		response.Severity = "info"
		body := "This worker question is no longer current because its execution or issue context changed. Inspect the issue before requesting a new owner-authorized continuation. Original question: " + details["question"]
		response.Body = &body
	}
}
