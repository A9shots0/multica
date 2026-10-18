package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type chatOwnerDelegationKey struct{}
type chatOwnerDelegation struct {
	OwnerID, IssueID, TaskID string
	SessionID                pgtype.UUID
	MessageIDs               []pgtype.UUID
}

// Only set by SubmitChatWorkResponse after checking the immutable user input,
// target snapshot and current access. Headers cannot opt into this capability.
func hasChatOwnerDelegation(r *http.Request) bool {
	grant, ok := r.Context().Value(chatOwnerDelegationKey{}).(chatOwnerDelegation)
	return ok && grant.OwnerID == requestUserID(r) && grant.IssueID == chi.URLParam(r, "id") && grant.TaskID == r.Header.Get("X-Task-ID")
}

// Submit the owner's exact current message against its frozen target.
// Mika selects the operation, but cannot substitute another issue, revision,
// question, owner, or invented answer. Canonical Inbox handlers do the mutation.
// A sole linked conversational target permits corrections only.
func (h *Handler) SubmitChatWorkResponse(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Actor-Source") != "task_token" {
		writeError(w, 403, "requires a Chat task")
		return
	}
	task, ok := h.taskFromRequestHeader(r)
	if !ok || task.Status != "running" || !task.ChatInputTaskID.Valid || !task.ChatSessionID.Valid || uuidToString(task.AgentID) != r.Header.Get("X-Agent-ID") {
		writeError(w, 403, "requires the running owner Chat turn")
		return
	}
	ownerRequest, session, ok := h.chatOwnerRequest(w, r, task)
	if !ok {
		return
	}
	r = ownerRequest
	// The owner may have sent this correction while the opening turn was
	// still creating its issue. Resolve only the provenance frozen at send.
	var resolveErr error
	task, resolveErr = service.ResolveOpeningChatCorrection(r.Context(), h.Queries, session, task)
	if resolveErr != nil {
		writeError(w, 500, "conversation correction context is unavailable")
		return
	}
	var contextData struct {
		Target    service.ChatWorkTarget     `json:"chat_work_target"`
		Responses []service.ChatWorkResponse `json:"chat_work_responses"`
	}
	if json.Unmarshal(task.Context, &contextData) != nil {
		writeError(w, 409, "select the issue in Chat before submitting a decision; no target was authorized for this turn")
		return
	}
	var input struct {
		Action     string `json:"action"`
		ResponseID string `json:"response_id,omitempty"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		writeError(w, 400, "action is required")
		return
	}
	target := contextData.Target
	answer := ""
	var messageIDs []pgtype.UUID
	if len(contextData.Responses) > 0 {
		if input.ResponseID == "" {
			writeError(w, 409, "this turn contains multiple work responses; response_id is required")
			return
		}
		for _, response := range contextData.Responses {
			if response.ResponseID == input.ResponseID {
				target = response.Target
				answer = strings.TrimSpace(response.Content)
				break
			}
		}
		if answer == "" {
			writeError(w, 409, "work response is not available in this turn")
			return
		}
	} else {
		messages, err := h.Queries.ListChatInputMessages(r.Context(), task.ChatInputTaskID)
		if err != nil {
			writeError(w, 500, "owner input is unavailable")
			return
		}
		var text []string
		for _, message := range messages {
			if message.Role == "user" && message.MessageKind == "message" {
				text = append(text, message.Content)
				messageIDs = append(messageIDs, message.ID)
			}
		}
		answer = strings.TrimSpace(strings.Join(text, "\n\n"))
	}
	if target.IssueID == "" || target.OwnerID != requestUserID(r) {
		writeError(w, 409, "select the issue in Chat before submitting a decision; no target was authorized for this turn")
		return
	}
	if target.CorrectionOnly && input.Action != "correct" {
		writeError(w, 409, "this conversational target permits corrections only; select the specific work before submitting a decision")
		return
	}
	if target.NeedsRequestSelection {
		writeError(w, 409, "this issue has multiple pending requests; select the specific question in Work before responding")
		return
	}
	links, err := h.Queries.ListIssuesForChat(r.Context(), db.ListIssuesForChatParams{WorkspaceID: session.WorkspaceID, ChatSessionID: session.ID})
	if err != nil {
		writeError(w, 500, "cannot verify work target")
		return
	}
	linked := false
	for _, link := range links {
		if uuidToString(link.ID) == target.IssueID {
			linked = true
		}
	}
	if !linked {
		writeError(w, 409, "issue is no longer linked here")
		return
	}
	if target.CorrectionOnly {
		current, err := service.CaptureConversationCorrection(r.Context(), h.Queries, session, parseUUID(requestUserID(r)))
		if err != nil || current.IssueID != target.IssueID {
			writeError(w, 409, "conversation work changed; select the specific work before correcting it")
			return
		}
	}
	answerLimit := 2000 // Existing workflow question/decision contracts.
	if target.DecisionID == "" && target.RequestEventID == "" {
		answerLimit = maxOwnerDirectiveContentBytes
	}
	if answer == "" || len(answer) > answerLimit {
		writeError(w, 400, fmt.Sprintf("owner answer must be 1 to %d bytes", answerLimit))
		return
	}
	// Multi-response content has no per-message attachment provenance. Do not
	// guess which selected target owns an image from the shared input batch.
	if len(contextData.Responses) > 0 {
		messages, err := h.Queries.ListChatInputMessages(r.Context(), task.ChatInputTaskID)
		if err != nil {
			writeError(w, 500, "owner input is unavailable")
			return
		}
		var ids []pgtype.UUID
		for _, message := range messages {
			if message.MessageKind == "message" {
				ids = append(ids, message.ID)
			}
		}
		attachments, err := h.Queries.ListAttachmentsByChatMessageIDs(r.Context(), db.ListAttachmentsByChatMessageIDsParams{Column1: ids, WorkspaceID: session.WorkspaceID})
		if err != nil {
			writeError(w, 500, "owner attachments are unavailable")
			return
		}
		if len(attachments) > 0 {
			writeError(w, 409, "send attached evidence in a separate correction for one issue; multiple responses have no image-to-target mapping")
			return
		}
	}
	grant := chatOwnerDelegation{OwnerID: requestUserID(r), IssueID: target.IssueID, TaskID: uuidToString(task.ID), SessionID: session.ID, MessageIDs: messageIDs}
	ctx := context.WithValue(r.Context(), chatOwnerDelegationKey{}, grant)
	route := chi.NewRouteContext()
	route.URLParams.Add("id", target.IssueID)
	ctx = context.WithValue(ctx, chi.RouteCtxKey, route)
	forward := r.Clone(ctx)
	responseKey := input.ResponseID
	if responseKey == "" {
		responseKey = "single"
	}
	forward.Header.Set("X-Chat-Response-ID", responseKey)
	forward.Header.Set("Idempotency-Key", "chat-response:"+grant.TaskID+":"+responseKey)
	var body any
	var handler http.HandlerFunc
	switch {
	case target.DecisionID != "":
		if input.Action != "approve" && input.Action != "reject" {
			writeError(w, 400, "this decision requires approve or reject")
			return
		}
		route.URLParams.Add("decisionID", target.DecisionID)
		body = map[string]any{"expected_revision": target.DecisionRevision, "outcome": input.Action, "comment": answer}
		handler = h.RespondIssueWorkflowOwnerDecisionV2
	case target.RequestEventID != "":
		if input.Action != "answer" {
			writeError(w, 400, "this question requires answer")
			return
		}
		body = map[string]any{"request_event_id": target.RequestEventID, "answer": answer}
		handler = h.PostIssueWorkflowOwnerDecision
	case target.IntakeID != "":
		if input.Action != "answer" {
			writeError(w, 400, "this worker question requires answer")
			return
		}
		body = ownerDirectiveRequest{ExpectedIssueRevision: target.IssueRevision, Directive: "correct", Content: answer, IntakeID: target.IntakeID}
		handler = h.PostIssueOwnerDirective
	default:
		if !validOwnerDirectives[input.Action] {
			writeError(w, 400, "unsupported work direction")
			return
		}
		body = ownerDirectiveRequest{ExpectedIssueRevision: target.IssueRevision, Directive: input.Action, Content: answer, IntakeID: target.IntakeID, ResultTaskID: target.ResultTaskID, ResultDigest: target.ResultDigest}
		handler = h.PostIssueOwnerDirective
	}
	raw, _ := json.Marshal(body)
	forward.Body = io.NopCloser(bytes.NewReader(raw))
	handler(w, forward)
}

func chatDelegatedDirectiveID(r *http.Request) string {
	if !hasChatOwnerDelegation(r) {
		return ""
	}
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("chat-directive:"+r.Header.Get("X-Task-ID")+":"+r.Header.Get("X-Chat-Response-ID"))).String()
}

// Runtime tokens identify the runtime owner, who need not be the human who
// sent this chat message. Derive the acting owner from the immutable input
// task, then recheck their membership and invocation access. Never accept a
// caller-supplied owner identity or change the global token's permissions.
func (h *Handler) chatOwnerRequest(w http.ResponseWriter, r *http.Request, task db.AgentTaskQueue) (*http.Request, db.ChatSession, bool) {
	empty := db.ChatSession{}
	input, err := h.Queries.GetAgentTask(r.Context(), task.ChatInputTaskID)
	if err != nil || !input.InitiatorUserID.Valid || input.ChatSessionID != task.ChatSessionID || input.AgentID != task.AgentID {
		writeError(w, 403, "owner input is unavailable")
		return r, empty, false
	}
	owner := uuidToString(input.InitiatorUserID)
	workspace := ctxWorkspaceID(r.Context())
	member, err := h.Queries.GetMemberByUserAndWorkspace(r.Context(), db.GetMemberByUserAndWorkspaceParams{WorkspaceID: parseUUID(workspace), UserID: input.InitiatorUserID})
	if err != nil {
		writeError(w, 403, "Chat owner is no longer a member")
		return r, empty, false
	}
	user, err := h.Queries.GetUser(r.Context(), input.InitiatorUserID)
	if err != nil || auth.IsTemporarilyDisabledUser(owner, user.Email) {
		writeError(w, 403, "Chat owner is unavailable")
		return r, empty, false
	}
	agent, err := h.Queries.GetAgent(r.Context(), task.AgentID)
	if err != nil || !h.canAccessPrivateAgent(r.Context(), agent, "member", owner, workspace) || !h.canInvokeAgent(r.Context(), agent, "member", owner, owner, workspace) {
		writeError(w, 403, "Chat owner no longer has access to this agent")
		return r, empty, false
	}
	delegated := r.Clone(middleware.SetMemberContext(r.Context(), workspace, member))
	delegated.Header.Set("X-User-ID", owner)
	session, ok := h.gatePublicChatSessionForUser(w, delegated, owner, workspace, uuidToString(task.ChatSessionID))
	if !ok {
		return r, empty, false
	}
	if session.AgentID != task.AgentID || session.Status != "active" {
		writeError(w, 409, "Chat is no longer active for this agent")
		return r, empty, false
	}
	return delegated, session, true
}
