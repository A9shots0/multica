package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/pkg/redact"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (h *Handler) loadBrainReference(ctx context.Context, resp AgentTaskResponse, task db.AgentTaskQueue) json.RawMessage {
	if h.SharedBrain == nil {
		return h.SharedBrain.Claim(ctx, resp.WorkspaceID, resp.ProjectID, "", task)
	}
	project := resp.ProjectID
	targetRevision := ""
	unavailable := json.RawMessage(`{"state":"unavailable","lifecycle":"not_recorded","reason":"Selected target could not be revalidated; shared context was withheld."}`)
	if task.IssueID.Valid {
		issue, err := h.Queries.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{WorkspaceID: parseUUID(resp.WorkspaceID), ID: task.IssueID})
		if err != nil {
			return unavailable
		}
		project = uuidToString(issue.ProjectID)
		targetRevision = fmt.Sprintf("issue:%s:%d", uuidToString(issue.ID), issue.Revision)
	} else if task.ChatSessionID.Valid {
		var source struct {
			Target *service.ChatWorkTarget `json:"chat_work_target"`
		}
		if len(task.Context) > 0 && json.Unmarshal(task.Context, &source) != nil {
			return unavailable
		}
		if source.Target != nil {
			session, err := h.Queries.GetChatSession(ctx, task.ChatSessionID)
			if err != nil || uuidToString(session.WorkspaceID) != resp.WorkspaceID {
				return unavailable
			}
			issueID, err := util.ParseUUID(source.Target.IssueID)
			if err != nil {
				return unavailable
			}
			current, err := service.CaptureChatWorkTarget(ctx, h.Queries, session, session.CreatorID, issueID, source.Target.RequestID)
			if err != nil || current != *source.Target {
				return unavailable
			}
			issue, err := h.Queries.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{WorkspaceID: session.WorkspaceID, ID: issueID})
			if err != nil {
				return unavailable
			}
			project = uuidToString(issue.ProjectID)
			raw, _ := json.Marshal(current)
			hash := sha256.Sum256(raw)
			targetRevision = "chat-work-target:sha256:" + hex.EncodeToString(hash[:])
		}
	}
	return h.SharedBrain.Claim(ctx, resp.WorkspaceID, project, targetRevision, task)
}

func brainReferenceBound(raw json.RawMessage) bool {
	var ref struct {
		Lifecycle string `json:"lifecycle"`
	}
	return json.Unmarshal(raw, &ref) == nil && ref.Lifecycle == "durable_source_bound"
}

func (h *Handler) InvokeAgentBrain(w http.ResponseWriter, r *http.Request) {
	task, workspace, ok := h.requireDaemonTaskAccessWithWorkspace(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	if h.SharedBrain == nil || h.SharedBrain.Client == nil {
		writeError(w, 503, "shared brain unconfigured")
		return
	}
	if task.Status != "running" && task.Status != "dispatched" || task.WorkflowActionID.Valid {
		writeError(w, 403, "brain tools require an active ordinary task")
		return
	}
	runtime, err := h.Queries.GetAgentRuntime(r.Context(), task.RuntimeID)
	if err != nil || !runtime.DaemonID.Valid || runtime.DaemonID.String != middleware.DaemonIDFromContext(r.Context()) {
		writeError(w, 403, "task belongs to another daemon")
		return
	}
	link, err := h.Queries.GetBrainTaskLink(r.Context(), db.GetBrainTaskLinkParams{WorkspaceID: parseUUID(workspace), TaskID: task.ID, Attempt: task.Attempt})
	if err != nil || !h.SharedBrain.Client.Allows(workspace, link.ProjectID) {
		writeError(w, 403, "task has no authorized brain project binding")
		return
	}
	var request struct {
		Operation string          `json:"operation"`
		Input     json.RawMessage `json:"input"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil {
		writeError(w, 400, "invalid brain request")
		return
	}
	var input struct {
		Query        string   `json:"q"`
		ID           string   `json:"id"`
		Revision     string   `json:"revision"`
		EventID      string   `json:"eventId"`
		Summary      string   `json:"summary"`
		EvidenceRefs []string `json:"evidenceRefs"`
		TaskID       string   `json:"taskId"`
		Cursor       string   `json:"cursor"`
		Limit        *int     `json:"limit"`
	}
	decoder = json.NewDecoder(bytes.NewReader(request.Input))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil {
		writeError(w, 400, "invalid brain input")
		return
	}
	if len(input.Query) > 1000 {
		writeError(w, 400, "brain query too long")
		return
	}
	var result json.RawMessage
	switch request.Operation {
	case "context":
		result, err = h.SharedBrain.Client.Context(r.Context(), link.ProjectID, redact.Text(input.Query))
	case "read":
		result, err = h.SharedBrain.Client.Read(r.Context(), link.ProjectID, input.ID, input.Revision)
	case "events":
		limit := 20
		if input.Limit != nil {
			limit = *input.Limit
		}
		result, err = h.SharedBrain.Client.Events(r.Context(), link.ProjectID, input.TaskID, input.Cursor, limit)
	case "evidence":
		result, err = h.SharedBrain.Client.Evidence(r.Context(), link.ProjectID, input.ID, input.Revision)
	case "handoff":
		if _, err = uuid.Parse(input.EventID); err != nil || strings.TrimSpace(input.Summary) == "" || len(input.Summary) > 4000 || len(input.EvidenceRefs) > 20 {
			writeError(w, 400, "bounded summary and stable event UUID required")
			return
		}
		refs := []string{"multica://workspace/" + workspace + "/task/" + uuidToString(task.ID)}
		for _, ref := range input.EvidenceRefs {
			if len(ref) > 500 {
				writeError(w, 400, "evidence reference too long")
				return
			}
			refs = append(refs, redact.Text(ref))
		}
		id := fmt.Sprintf("multica:%s:%s:%d:handoff:%s", workspace, uuidToString(task.ID), task.Attempt, input.EventID)
		payload := map[string]any{"project": link.ProjectID, "taskId": uuidToString(task.ID), "kind": "handoff", "summary": redact.Text(input.Summary), "evidenceRefs": refs, "targetRevision": link.TargetRevision, "sourceEventId": id, "idempotencyKey": id, "ownerSystem": "multica", "observedAt": time.Now().UTC().Format(time.RFC3339Nano)}
		raw, _ := json.Marshal(payload)
		row, queueErr := h.Queries.EnqueueBrainHandoff(r.Context(), db.EnqueueBrainHandoffParams{EventID: id, WorkspaceID: link.WorkspaceID, ProjectID: link.ProjectID, Payload: raw})
		if queueErr != nil {
			writeError(w, 500, "handoff could not be saved")
			return
		}
		var saved map[string]any
		if json.Unmarshal(row.Payload, &saved) != nil {
			writeError(w, 500, "saved handoff invalid")
			return
		}
		delete(saved, "observedAt")
		delete(payload, "observedAt")
		a, _ := json.Marshal(saved)
		b, _ := json.Marshal(payload)
		if !bytes.Equal(a, b) {
			writeError(w, 409, "event identity already has a different handoff")
			return
		}
		writeJSON(w, 200, map[string]any{"queued": !row.DeliveredAt.Valid, "delivered": row.DeliveredAt.Valid, "accepted": false, "idempotencyKey": id, "warning": "Saved in Multica; only delivered records are visible in the shared brain. Task acceptance is separate."})
		return
	default:
		writeError(w, 400, "unsupported brain operation")
		return
	}
	if err != nil {
		writeError(w, 502, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.Copy(w, bytes.NewReader(result))
}
