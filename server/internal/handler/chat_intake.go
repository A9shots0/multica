package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	googleuuid "github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/issuestatus"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const chatIntakeType = "chat_intake"

// CreateChatIntake binds an owner request from Chat directly to an existing issue.
// The original Chat request is already the owner's instruction, so it immediately
// admits the request, records the owner direction and comment, and queues the
// assigned worker task exactly once without creating an action-required Inbox item.
func (h *Handler) CreateChatIntake(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Actor-Source") != "task_token" {
		writeError(w, http.StatusForbidden, "intake requires the current Chat task")
		return
	}
	task, ok := h.taskFromRequestHeader(r)
	if !ok || !task.ChatSessionID.Valid || task.Status != "running" || uuidToString(task.AgentID) != r.Header.Get("X-Agent-ID") {
		writeError(w, http.StatusForbidden, "intake requires the running Chat task")
		return
	}
	var session db.ChatSession
	if task.ChatInputTaskID.Valid {
		var ownerRequest *http.Request
		ownerRequest, session, ok = h.chatOwnerRequest(w, r, task)
		if ok {
			r = ownerRequest
		}
	} else {
		// Legacy turns have no immutable input owner; retain their existing
		// same-owner behavior without broadening runtime authority.
		session, ok = h.gatePublicChatSessionForUser(w, r, requestUserID(r), ctxWorkspaceID(r.Context()), uuidToString(task.ChatSessionID))
	}
	if !ok {
		return
	}
	if session.AgentID != task.AgentID {
		writeError(w, http.StatusForbidden, "task does not belong to this Chat agent")
		return
	}
	var input struct {
		IssueID string `json:"issue_id"`
		Request string `json:"request"`
		AgentID string `json:"agent_id,omitempty"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil {
		writeError(w, http.StatusBadRequest, "issue_id and request are required")
		return
	}
	input.Request = strings.TrimSpace(sanitizeNullBytes(input.Request))
	if input.Request == "" || len(input.Request) > 4000 {
		writeError(w, http.StatusBadRequest, "a bounded request is required")
		return
	}
	issueID, ok := parseUUIDOrBadRequest(w, input.IssueID, "issue id")
	if !ok {
		return
	}
	var selectedAgent pgtype.UUID
	if input.AgentID != "" {
		selectedAgent, ok = parseUUIDOrBadRequest(w, input.AgentID, "agent id")
		if !ok {
			return
		}
	}
	issue, err := h.Queries.GetIssueInWorkspace(r.Context(), db.GetIssueInWorkspaceParams{ID: issueID, WorkspaceID: session.WorkspaceID})
	if err != nil {
		writeError(w, http.StatusNotFound, "issue not found in this workspace")
		return
	}
	// One receipt per issue per turn: retries replay, while a single owner
	// message can dispatch several independent issues.
	id := parseUUID(googleuuid.NewSHA1(googleuuid.NameSpaceURL, []byte("multica-chat-intake:"+uuidToString(task.ID)+":"+uuidToString(issueID))).String())
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "intake unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	// Serializes concurrent binds and closes a completion race before writing.
	var status string
	if tx.QueryRow(r.Context(), `SELECT status FROM agent_task_queue WHERE id=$1 FOR UPDATE`, task.ID).Scan(&status) != nil || status != "running" {
		writeError(w, http.StatusConflict, "Chat task is no longer running")
		return
	}
	q := h.Queries.WithTx(tx)
	if task.ChatInputTaskID.Valid {
		// A search result is not the owner's selected work. New issues are
		// linked by their creation transaction; existing work must already be
		// selected in this conversation before an assistant can dispatch it.
		links, linkErr := q.ListIssuesForChat(r.Context(), db.ListIssuesForChatParams{WorkspaceID: session.WorkspaceID, ChatSessionID: session.ID})
		if linkErr != nil {
			writeError(w, http.StatusInternalServerError, "conversation issue selection unavailable")
			return
		}
		selected := false
		for _, link := range links {
			if link.ID == issue.ID {
				selected = true
				break
			}
		}
		if !selected {
			writeError(w, http.StatusConflict, "issue is not linked to this conversation; create new work for a fresh request, or ask the owner to select existing work in Chat before binding it")
			return
		}
	}
	item, lookupErr := q.GetInboxItemInWorkspace(r.Context(), db.GetInboxItemInWorkspaceParams{ID: id, WorkspaceID: session.WorkspaceID})
	if errors.Is(lookupErr, pgx.ErrNoRows) {
		legacyID := parseUUID(googleuuid.NewSHA1(googleuuid.NameSpaceURL, []byte("multica-chat-intake:"+uuidToString(task.ID))).String())
		legacy, legacyErr := q.GetInboxItemInWorkspace(r.Context(), db.GetInboxItemInWorkspaceParams{ID: legacyID, WorkspaceID: session.WorkspaceID})
		if legacyErr == nil && legacy.IssueID == issue.ID {
			item, lookupErr = legacy, nil
		}
		if legacyErr != nil && !errors.Is(legacyErr, pgx.ErrNoRows) {
			writeError(w, 500, "prior receipt unavailable")
			return
		}
	}
	if lookupErr == nil {
		var details map[string]string
		_ = json.Unmarshal(item.Details, &details)
		if item.IssueID != issue.ID || details["request"] != input.Request {
			writeError(w, http.StatusConflict, "this Chat task already bound a different request")
			return
		}
		if selectedAgent.Valid {
			prior, loadErr := q.GetAgentTask(r.Context(), parseUUID(details["task_id"]))
			if loadErr != nil || prior.IssueID != issue.ID || prior.AgentID != selectedAgent {
				writeError(w, http.StatusConflict, "this request already dispatched a different worker")
				return
			}
		}
		writeJSON(w, http.StatusOK, chatIntakeReceipt(item, details, true))
		return
	}
	if !errors.Is(lookupErr, pgx.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "intake lookup failed")
		return
	}

	locked, err := q.LockIssueForDescriptionUpdate(r.Context(), db.LockIssueForDescriptionUpdateParams{ID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		writeError(w, http.StatusConflict, "issue changed; reload before continuing")
		return
	}

	category := issuestatus.Effective(r.Context(), q, issue.WorkspaceID, locked.Status)
	resumeFailure := false
	if category == "blocked" {
		resumeFailure, err = q.CanResumeIssueFailureForChatIntake(r.Context(), db.CanResumeIssueFailureForChatIntakeParams{ID: issue.ID, WorkspaceID: issue.WorkspaceID})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failure recovery state unavailable")
			return
		}
	}
	if category == "blocked" && !resumeFailure {
		writeError(w, http.StatusConflict, "this work is waiting on a decision or dependency; inspect it with chat work and answer its current request")
		return
	}
	if category == "done" || category == "cancelled" {
		writeError(w, http.StatusConflict, "this issue is closed; reopen it before starting new work")
		return
	}
	var active bool
	if tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM agent_task_queue WHERE issue_id=$1 AND status IN ('queued','dispatched','running','waiting_local_directory'))`, issue.ID).Scan(&active) != nil || active {
		// The one remaining refusal that still points somewhere real: an agent
		// is already working, and a second run would duplicate it.
		writeError(w, http.StatusConflict, "this issue already has pending work; follow the existing run")
		return
	}
	assignSelected := !locked.AssigneeID.Valid && (!locked.AssigneeType.Valid || locked.AssigneeType.String == "") && selectedAgent.Valid
	if assignSelected {
		locked.AssigneeType = pgtype.Text{String: "agent", Valid: true}
		locked.AssigneeID = selectedAgent
	} else if selectedAgent.Valid && (locked.AssigneeType.String != "agent" || locked.AssigneeID != selectedAgent) {
		writeError(w, http.StatusConflict, "this issue already has a different assignee; binding cannot replace an existing route")
		return
	}
	if !locked.AssigneeType.Valid || (locked.AssigneeType.String != "agent" && locked.AssigneeType.String != "squad") || !locked.AssigneeID.Valid {
		// A generic squad template is not a project routing decision. Mika
		// must choose an eligible worker before binding; never ask the owner
		// to repair this or silently install a legacy controller to pick one.
		writeError(w, http.StatusConflict, "Chat must select a suitable project worker; inspect project context and agent runtimes, then retry chat bind on this same issue with --agent-id. Do not ask the owner to repair routing")
		return
	}

	targetID := locked.AssigneeID
	if locked.AssigneeType.String == "squad" {
		squad, squadErr := q.LockSquadForAutopilotAssignment(r.Context(), db.LockSquadForAutopilotAssignmentParams{ID: locked.AssigneeID, WorkspaceID: issue.WorkspaceID})
		if squadErr != nil || squad.ArchivedAt.Valid {
			writeError(w, http.StatusForbidden, "invocation not allowed")
			return
		}
		targetID = squad.LeaderID
	}
	agent, agentErr := q.LockAgentForAutopilotAssignment(r.Context(), db.LockAgentForAutopilotAssignmentParams{ID: targetID, WorkspaceID: issue.WorkspaceID})
	invocationGate := &Handler{Queries: q}
	userID := uuidToString(session.CreatorID)
	if agentErr != nil || agent.ArchivedAt.Valid || !agent.RuntimeID.Valid || !invocationGate.canInvokeAgent(r.Context(), agent, "member", userID, userID, uuidToString(issue.WorkspaceID)) {
		writeError(w, http.StatusForbidden, "invocation not allowed")
		return
	}
	if assignSelected {
		// Permission, assignment, receipt and enqueue share this transaction.
		// A refused invocation or enqueue leaves the issue unassigned.
		if tx.QueryRow(r.Context(), `UPDATE issue SET assignee_type='agent',assignee_id=$2,revision=revision+1,updated_at=now() WHERE id=$1 AND assignee_id IS NULL RETURNING revision`, issue.ID, selectedAgent).Scan(&locked.Revision) != nil {
			writeError(w, http.StatusConflict, "issue routing changed; refresh before binding")
			return
		}
	}

	if _, err = q.LinkChatIssue(r.Context(), db.LinkChatIssueParams{WorkspaceID: session.WorkspaceID, ChatSessionID: session.ID, IssueID: issue.ID, Source: "binding_receipt"}); err != nil {
		writeError(w, http.StatusInternalServerError, "issue binding failed")
		return
	}

	intakeIDStr := uuidToString(id)
	commentContent := fmt.Sprintf("Bound Chat request [%s]:\n\n%s", intakeIDStr, input.Request)
	created, err := q.CreateComment(r.Context(), db.CreateCommentParams{
		ID: dbid.NewV7(), IssueID: issue.ID, WorkspaceID: issue.WorkspaceID,
		AuthorType: "member", AuthorID: session.CreatorID, Content: commentContent, Type: "comment",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "owner direction could not be recorded")
		return
	}

	directiveID := dbid.NewV7()
	directive, err := q.CreateIssueOwnerDirective(r.Context(), db.CreateIssueOwnerDirectiveParams{
		ID: directiveID, WorkspaceID: issue.WorkspaceID, IssueID: issue.ID, DirectiveType: "correct",
		ExpectedIssueRevision: locked.Revision, Content: input.Request, State: "applied",
		ActorID: session.CreatorID, CommentID: created.ID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "owner receipt could not be recorded")
		return
	}

	if resumeFailure {
		if _, err := q.ResumeIssueFromTerminalTaskFailureWait(r.Context(), db.ResumeIssueFromTerminalTaskFailureWaitParams{
			ID: issue.ID, WorkspaceID: issue.WorkspaceID,
		}); err != nil {
			writeError(w, http.StatusConflict, "failure recovery could not be applied")
			return
		}
	}

	queuedTask, err := h.TaskService.EnqueueOwnerIntakeInTransaction(r.Context(), tx, locked, created.ID)
	if err != nil {
		writeError(w, http.StatusConflict, "work could not be admitted: "+err.Error())
		return
	}

	issueRevision := created.IssueRevision
	if resumeFailure || assignSelected {
		locked, err = q.GetIssue(r.Context(), issue.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "continued issue state unavailable")
			return
		}
		issueRevision = locked.Revision
	}

	directiveReceipt := map[string]any{
		"id": uuidToString(directive.ID), "issue_id": uuidToString(issue.ID),
		"revision": directive.Revision, "directive": directive.DirectiveType, "state": directive.State,
		"comment_id": uuidToString(created.ID), "issue_revision": issueRevision,
		"task_id": uuidToString(queuedTask.ID), "task_state": queuedTask.Status, "intake_id": intakeIDStr,
	}
	encodedDirectiveReceipt, _ := json.Marshal(directiveReceipt)

	details := map[string]string{
		"owner_relevant": "false", "owner_category": "update", "required_action": "follow_work",
		"chat_session_id": uuidToString(session.ID), "source_task_id": uuidToString(task.ID),
		"request": input.Request, "intake_state": "continued",
		"task_id": uuidToString(queuedTask.ID), "owner_content": input.Request,
		"directive_receipt": string(encodedDirectiveReceipt),
	}
	encoded, _ := json.Marshal(details)
	item, err = q.CreateInboxItem(r.Context(), db.CreateInboxItemParams{
		ID: id, WorkspaceID: session.WorkspaceID, RecipientType: "member", RecipientID: session.CreatorID,
		Type: chatIntakeType, Severity: "info", IssueID: issue.ID, Title: issue.Title,
		Body:      pgtype.Text{String: "Request from Chat:\n\n" + input.Request + "\n\nA worker task was queued. Progress and review evidence remain available in the linked conversation.", Valid: true},
		ActorType: pgtype.Text{String: "agent", Valid: true}, ActorID: task.AgentID, Details: encoded,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "intake could not be recorded")
		return
	}
	if _, err = tx.Exec(r.Context(), `UPDATE inbox_item SET read=true WHERE id=$1`, id); err != nil || tx.Commit(r.Context()) != nil {
		writeError(w, http.StatusInternalServerError, "intake could not be committed")
		return
	}

	h.TaskService.PublishOwnerIntakeTask(r.Context(), queuedTask)
	commentResp := commentToResponse(created.Comment(), nil, nil)
	commentResp.IssueRevision = created.IssueRevision
	h.publish(protocol.EventCommentCreated, uuidToString(issue.WorkspaceID), "member", userID, map[string]any{
		"comment": commentResp, "issue_title": issue.Title, "issue_revision": created.IssueRevision,
	})
	if resumeFailure || assignSelected {
		response := issueToResponse(locked, h.getIssuePrefix(r.Context(), locked.WorkspaceID))
		h.fillStatusCategory(r.Context(), locked.WorkspaceID, &response)
		h.publish(protocol.EventIssueUpdated, uuidToString(issue.WorkspaceID), "member", userID, map[string]any{
			"issue": response, "status_changed": resumeFailure, "prev_status": issue.Status,
		})
	}

	writeJSON(w, http.StatusCreated, chatIntakeReceipt(item, details, false))
}

func chatIntakeReceipt(item db.InboxItem, details map[string]string, replay bool) map[string]any {
	receipt := map[string]any{
		"intake_id":       uuidToString(item.ID),
		"inbox_item_id":   uuidToString(item.ID),
		"issue_id":        uuidToString(item.IssueID),
		"chat_session_id": details["chat_session_id"],
		"state":           details["intake_state"],
		"task_id":         details["task_id"],
		"replay":          replay,
	}
	if encoded, ok := details["directive_receipt"]; ok && encoded != "" {
		var dr map[string]any
		if err := json.Unmarshal([]byte(encoded), &dr); err == nil {
			if ts, ok := dr["task_state"]; ok {
				receipt["task_state"] = ts
			}
			if ir, ok := dr["issue_revision"]; ok {
				receipt["issue_revision"] = ir
			}
		}
	}
	return receipt
}

// continueChatIntake is reached only after the existing human-only directive
// gate. The Inbox receipt, revision-checked owner directive, comment and task
// commit together. Retries cannot cancel or duplicate an already admitted run.
func (h *Handler) continueChatIntake(w http.ResponseWriter, r *http.Request, issue db.Issue, req ownerDirectiveRequest) {
	if req.Directive != "correct" || h.TaskService == nil {
		writeError(w, http.StatusBadRequest, "intake continuation requires a bounded owner direction")
		return
	}
	id, ok := parseUUIDOrBadRequest(w, req.IntakeID, "intake id")
	if !ok {
		return
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "continuation unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	var raw []byte
	err = tx.QueryRow(r.Context(), `SELECT details FROM inbox_item WHERE id=$1 AND workspace_id=$2 AND issue_id=$3 AND recipient_type='member' AND recipient_id=$4 AND (type=$5 OR (type='workflow_owner_decision' AND details->>'question_scope'='ordinary_task')) FOR UPDATE`, id, issue.WorkspaceID, issue.ID, parseUUID(requestUserID(r)), chatIntakeType).Scan(&raw)
	if err != nil {
		writeError(w, http.StatusNotFound, "intake not found")
		return
	}
	details := map[string]string{}
	if json.Unmarshal(raw, &details) != nil {
		writeError(w, http.StatusConflict, "intake is invalid")
		return
	}
	if details["directive_receipt"] != "" {
		routeJSON, _ := json.Marshal(req.OwnerRuntimeRoute)
		if details["owner_content"] != req.Content || (details["owner_runtime_route"] != "" && details["owner_runtime_route"] != string(routeJSON)) || (details["owner_runtime_route"] == "" && req.OwnerRuntimeRoute != nil) {
			writeError(w, http.StatusConflict, "intake was already continued with a different direction")
			return
		}
		writeJSON(w, http.StatusOK, json.RawMessage(details["directive_receipt"]))
		return
	}
	q := h.Queries.WithTx(tx)
	locked, err := q.LockIssueForDescriptionUpdate(r.Context(), db.LockIssueForDescriptionUpdateParams{ID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if err != nil || locked.Revision != req.ExpectedIssueRevision {
		writeError(w, http.StatusConflict, "issue changed; reload before continuing")
		return
	}
	isTaskQuestion := details["question_scope"] == "ordinary_task"
	if !isTaskQuestion && strings.TrimSpace(details["request"]) == "" {
		writeError(w, http.StatusConflict, "bound Chat request is unavailable; continuation was not admitted")
		return
	}
	if isTaskQuestion {
		sourceID, parseErr := googleuuid.Parse(details["source_task_id"])
		if parseErr != nil {
			writeError(w, http.StatusConflict, "question source is invalid")
			return
		}
		source, sourceErr := q.GetAgentTaskInWorkspace(r.Context(), db.GetAgentTaskInWorkspaceParams{ID: parseUUID(sourceID.String()), WorkspaceID: issue.WorkspaceID})
		if sourceErr != nil || source.IssueID != issue.ID || source.AccountableUserID != parseUUID(requestUserID(r)) || source.AgentID != locked.AssigneeID || locked.AssigneeType.String != "agent" || source.Status != "completed" {
			writeError(w, http.StatusConflict, "wait for the reporting worker to finish; changed or failed work must be inspected first")
			return
		}
		var newer bool
		if tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM agent_task_queue WHERE issue_id=$1 AND (created_at,id)>($2,$3))`, issue.ID, source.CreatedAt, source.ID).Scan(&newer) != nil || newer {
			writeError(w, http.StatusConflict, "a newer task superseded this question")
			return
		}
	}
	// Do not turn a new Chat request into authority over an existing decision,
	// running operation, finished issue or parked dependency.
	category := issuestatus.Effective(r.Context(), q, issue.WorkspaceID, locked.Status)
	resumeFailure := false
	if category == "blocked" {
		resumeFailure, err = q.CanResumeIssueFailureForChatIntake(r.Context(), db.CanResumeIssueFailureForChatIntakeParams{ID: issue.ID, WorkspaceID: issue.WorkspaceID})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failure recovery state unavailable")
			return
		}
	}
	// Same rule as CreateChatIntake: a closed issue is a different request, and
	// everything else is the owner continuing their own work. The Current
	// Picture and status gates that used to stand here redirected to
	// "authoritative workflow controls" that were never written.
	if category == "done" || category == "cancelled" {
		writeError(w, http.StatusConflict, "this issue is closed; reopen it before continuing work on it")
		return
	}
	var active bool
	if tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM agent_task_queue WHERE issue_id=$1 AND status IN ('queued','dispatched','running','waiting_local_directory'))`, issue.ID).Scan(&active) != nil || active {
		writeError(w, http.StatusConflict, "this issue already has pending work; follow the existing run")
		return
	}
	if req.OwnerRuntimeRoute != nil {
		if isTaskQuestion {
			writeError(w, http.StatusConflict, "answer the current question before declaring a new route")
			return
		}
		h.admitOwnerRuntimeRoute(w, r, tx, locked, id, details, req)
		return
	}
	if !locked.AssigneeType.Valid || (locked.AssigneeType.String != "agent" && locked.AssigneeType.String != "squad") {
		writeError(w, http.StatusConflict, "Chat must select and assign a suitable project worker before continuing this request; inspect project context and agent runtimes, assign with --no-start, then retry on this same issue")
		return
	}
	// Reading an issue does not authorize invoking its assignee. Resolve the
	// exact target under the same transaction, including the assigned squad's
	// leader, and apply the existing human invocation gate before any writes.
	targetID := locked.AssigneeID
	if locked.AssigneeType.String == "squad" {
		squad, squadErr := q.LockSquadForAutopilotAssignment(r.Context(), db.LockSquadForAutopilotAssignmentParams{ID: locked.AssigneeID, WorkspaceID: issue.WorkspaceID})
		if squadErr != nil || squad.ArchivedAt.Valid {
			writeError(w, http.StatusForbidden, "invocation not allowed")
			return
		}
		targetID = squad.LeaderID
	}
	agent, agentErr := q.LockAgentForAutopilotAssignment(r.Context(), db.LockAgentForAutopilotAssignmentParams{ID: targetID, WorkspaceID: issue.WorkspaceID})
	invocationGate := &Handler{Queries: q}
	userID := requestUserID(r)
	if agentErr != nil || agent.ArchivedAt.Valid || !agent.RuntimeID.Valid || !invocationGate.canInvokeAgent(r.Context(), agent, "member", userID, userID, uuidToString(issue.WorkspaceID)) {
		writeError(w, http.StatusForbidden, "invocation not allowed")
		return
	}
	directiveID := dbid.NewV7()
	content := chatIntakeContinuationContent(req.IntakeID, details["request"], req.Content)
	if isTaskQuestion {
		content = fmt.Sprintf("Recorded owner answer to worker question [%s]\nOwner directive receipt: %s\n\nThe platform has already saved this answer and closed this question. This task is the admitted continuation. Apply the owner's choice to the original issue work; do not submit the answer again through Chat, Inbox, or owner directives. Submission instructions in the quoted owner message were addressed to the assistant that recorded this receipt, not to this continuation worker. If the substantive choice is unclear, ask a new specific question. Do not claim the issue accepted or complete merely because this answer was recorded.\n\nQuestion: %s\n\nExact owner message:\n%s", req.IntakeID, uuidToString(directiveID), details["question"], req.Content)
	}
	created, err := q.CreateComment(r.Context(), db.CreateCommentParams{ID: dbid.NewV7(), IssueID: issue.ID, WorkspaceID: issue.WorkspaceID, AuthorType: "member", AuthorID: parseUUID(requestUserID(r)), Content: content, Type: "comment"})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "owner direction could not be recorded")
		return
	}
	directive, err := q.CreateIssueOwnerDirective(r.Context(), db.CreateIssueOwnerDirectiveParams{ID: directiveID, WorkspaceID: issue.WorkspaceID, IssueID: issue.ID, DirectiveType: "correct", ExpectedIssueRevision: req.ExpectedIssueRevision, Content: req.Content, State: "applied", ActorID: parseUUID(requestUserID(r)), CommentID: created.ID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "owner receipt could not be recorded")
		return
	}
	if isTaskQuestion {
		// Release only this question inside the same transaction as its answer
		// and admitted successor. Any admission failure restores the open question.
		if _, err := tx.Exec(r.Context(), `UPDATE inbox_item SET details=jsonb_set(details,'{intake_state}','"answering"') WHERE id=$1`, id); err != nil {
			writeError(w, http.StatusInternalServerError, "answer unavailable")
			return
		}
	}
	task, err := h.TaskService.EnqueueOwnerIntakeInTransaction(r.Context(), tx, locked, created.ID)
	if err != nil {
		writeError(w, http.StatusConflict, "work could not be admitted: "+err.Error())
		return
	}
	issueRevision := created.IssueRevision
	if resumeFailure || isTaskQuestion {
		locked, err = q.GetIssue(r.Context(), issue.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "continued issue state unavailable")
			return
		}
		issueRevision = locked.Revision
	}
	receipt := map[string]any{"id": uuidToString(directive.ID), "issue_id": uuidToString(issue.ID), "revision": directive.Revision, "directive": directive.DirectiveType, "state": directive.State, "comment_id": uuidToString(created.ID), "issue_revision": issueRevision, "task_id": uuidToString(task.ID), "task_state": task.Status, "intake_id": req.IntakeID}
	encodedReceipt, _ := json.Marshal(receipt)
	details["intake_state"], details["task_id"], details["owner_content"], details["directive_receipt"] = "continued", uuidToString(task.ID), req.Content, string(encodedReceipt)
	details["owner_category"], details["required_action"] = "update", "follow_work"
	encoded, _ := json.Marshal(details)
	if _, err = tx.Exec(r.Context(), `UPDATE inbox_item SET details=$2, severity='info', body=$3, read=true, archived=true WHERE id=$1`, id, encoded, "Your direction was recorded and a worker task was queued. Progress and review evidence remain available in the linked conversation."); err != nil || tx.Commit(r.Context()) != nil {
		writeError(w, http.StatusInternalServerError, "continuation could not be committed")
		return
	}
	h.publish(protocol.EventInboxArchived, uuidToString(issue.WorkspaceID), "member", requestUserID(r), map[string]any{"item_id": uuidToString(id), "recipient_id": requestUserID(r)})
	h.TaskService.PublishOwnerIntakeTask(r.Context(), task)
	comment := commentToResponse(created.Comment(), nil, nil)
	comment.IssueRevision = created.IssueRevision
	h.publish(protocol.EventCommentCreated, uuidToString(issue.WorkspaceID), "member", requestUserID(r), map[string]any{"comment": comment, "issue_title": issue.Title, "issue_revision": created.IssueRevision})
	if resumeFailure || isTaskQuestion {
		response := issueToResponse(locked, h.getIssuePrefix(r.Context(), locked.WorkspaceID))
		h.fillStatusCategory(r.Context(), locked.WorkspaceID, &response)
		h.publish(protocol.EventIssueUpdated, uuidToString(issue.WorkspaceID), "member", requestUserID(r), map[string]any{"issue": response, "status_changed": true, "prev_status": issue.Status})
	}
	writeJSON(w, http.StatusCreated, receipt)
}

// The bound Chat request is durable task input. An owner's continuation note
// supplements it; it must not silently replace attachment references or scope.
func chatIntakeContinuationContent(intakeID, request, ownerNote string) string {
	return fmt.Sprintf("Owner direction from Chat intake [%s]:\n\nBound Chat request:\n%s\n\nOwner continuation note:\n%s", intakeID, request, ownerNote)
}
