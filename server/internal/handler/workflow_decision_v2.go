package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/workflowcommand"
	workflowstatev2 "github.com/multica-ai/multica/server/internal/workflowstate/v2"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type workflowDecisionPacketV1 struct {
	ContractVersion int            `json:"contract_version"`
	IssueID         string         `json:"issue_id"`
	Decision        map[string]any `json:"decision"`
	Attention       map[string]any `json:"attention"`
	Continuation    map[string]any `json:"continuation"`
	Replacement     any            `json:"replacement"`
}

type workflowDecisionResponseRequest struct {
	ExpectedRevision workflowstatev2.Revision        `json:"expected_revision"`
	Outcome          workflowstatev2.DecisionOutcome `json:"outcome"`
	Comment          string                          `json:"comment,omitempty"`
}

func (h *Handler) GetIssueWorkflowOwnerDecisionV2(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	packet, err := h.loadWorkflowDecisionPacket(r.Context(), issue.WorkspaceID, issue.ID, chi.URLParam(r, "decisionID"), "", requestUserID(r), false)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "owner decision not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "owner decision is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, packet)
}

func (h *Handler) RespondIssueWorkflowOwnerDecisionV2(w http.ResponseWriter, r *http.Request) {
	if isMachineCredentialActor(r) && !hasChatOwnerDelegation(r) {
		writeError(w, http.StatusForbidden, "owner decisions require a human actor")
		return
	}
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	var input workflowDecisionResponseRequest
	if decodeWorkflowDecisionResponse(r.Body, &input) != nil || input.ExpectedRevision == 0 || !validOwnerDecisionOutcome(input.Outcome) {
		writeError(w, http.StatusBadRequest, "expected_revision and a supported owner response are required")
		return
	}
	h.executeOwnerDecisionMutation(w, r, issue.WorkspaceID, issue.ID, workflowstatev2.CommandRespondDecision, chi.URLParam(r, "decisionID"), input.ExpectedRevision, input.Outcome, input.Comment)
}

func validOwnerDecisionOutcome(outcome workflowstatev2.DecisionOutcome) bool {
	switch outcome {
	case workflowstatev2.DecisionApprove, workflowstatev2.DecisionReject:
		return true
	default:
		return false
	}
}

func (h *Handler) ReadIssueWorkflowOwnerDecisionV2(w http.ResponseWriter, r *http.Request) {
	h.executeOwnerDecisionAttentionMutation(w, r, workflowstatev2.CommandReadDecision)
}
func (h *Handler) AcknowledgeIssueWorkflowOwnerDecisionV2(w http.ResponseWriter, r *http.Request) {
	h.executeOwnerDecisionAttentionMutation(w, r, workflowstatev2.CommandAcknowledgeDecision)
}

func (h *Handler) executeOwnerDecisionAttentionMutation(w http.ResponseWriter, r *http.Request, kind workflowstatev2.CommandKind) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	packet, err := h.loadWorkflowDecisionPacket(r.Context(), issue.WorkspaceID, issue.ID, "", chi.URLParam(r, "attentionID"), requestUserID(r), false)
	if err != nil {
		writeError(w, http.StatusNotFound, "owner decision not found")
		return
	}
	decisionID, _ := packet.Decision["id"].(string)
	revision, ok := wholeRevision(packet.Decision["revision"])
	if !ok {
		writeError(w, http.StatusInternalServerError, "owner decision is unavailable")
		return
	}
	h.executeOwnerDecisionMutation(w, r, issue.WorkspaceID, issue.ID, kind, decisionID, revision, "", "")
}

func (h *Handler) executeOwnerDecisionMutation(w http.ResponseWriter, r *http.Request, workspaceID, issueID pgtype.UUID, kind workflowstatev2.CommandKind, decisionID string, revision workflowstatev2.Revision, outcome workflowstatev2.DecisionOutcome, detail string) {
	commandKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if commandKey == "" {
		writeError(w, http.StatusBadRequest, "Idempotency-Key is required")
		return
	}
	if h.WorkflowCommandService == nil {
		writeError(w, http.StatusServiceUnavailable, "authoritative workflow decisions are unavailable")
		return
	}
	expected, err := h.ownerDecisionRequestExpected(r.Context(), workspaceID, issueID, kind, commandKey)
	if err != nil {
		writeError(w, http.StatusConflict, "workflow decision was replaced")
		return
	}
	receipt, err := h.WorkflowCommandService.Execute(r.Context(), workflowcommand.Principal{ActorID: parseUUID(requestUserID(r)), ActorType: "member"}, workflowcommand.Request{WorkspaceID: workspaceID, IssueID: issueID, CommandKey: commandKey, Kind: kind, Expected: expected, DecisionID: decisionID, ExpectedDecisionRevision: revision, DecisionOutcome: outcome, Detail: strings.TrimSpace(detail)})
	if err != nil {
		switch ownerDecisionMutationStatus(err) {
		case http.StatusForbidden:
			writeError(w, http.StatusForbidden, "decision belongs to the accountable owner")
			return
		case http.StatusConflict:
			if !errors.Is(err, workflowcommand.ErrDecisionStale) {
				writeError(w, http.StatusConflict, "owner decision response conflicted")
				return
			}
			h.writeOwnerDecisionStale(w, r, workspaceID, issueID, decisionID, revision)
			return
		default:
			writeError(w, http.StatusInternalServerError, "owner decision response failed")
			return
		}
	}
	if kind != workflowstatev2.CommandRespondDecision {
		packet, packetErr := h.loadWorkflowDecisionPacket(r.Context(), workspaceID, issueID, decisionID, "", requestUserID(r), false)
		if packetErr != nil {
			writeError(w, http.StatusInternalServerError, "owner decision changed but could not be reloaded")
			return
		}
		writeJSON(w, http.StatusOK, packet)
		return
	}
	responseID := ""
	if rows, queryErr := h.Queries.ListIssueWorkflowProjectionInputs(r.Context(), db.ListIssueWorkflowProjectionInputsParams{WorkspaceID: workspaceID, IssueID: issueID}); queryErr == nil {
		for _, row := range rows {
			if row.RecordType != "decision_response" {
				continue
			}
			var f map[string]any
			if json.Unmarshal(row.Payload, &f) == nil && f["decision_id"] == decisionID && f["idempotency_key"] == commandKey {
				responseID, _ = f["id"].(string)
			}
		}
	}
	actionID := ""
	if len(receipt.ActionIDs) != 0 {
		actionID = receipt.ActionIDs[0]
	}
	projectionRevision := int64(0)
	if row, e := h.Queries.GetIssueCurrentPicture(r.Context(), db.GetIssueCurrentPictureParams{WorkspaceID: workspaceID, IssueID: issueID}); e == nil {
		projectionRevision = row.Revision
	}
	writeJSON(w, http.StatusOK, map[string]any{"decision_id": decisionID, "decision_revision": revision + 1, "response_id": responseID, "response_type": outcome, "resolution_state": "resuming", "action_receipt_id": actionID, "action_state": receipt.ActionState, "command_id": receipt.CommandID, "current_picture_revision": projectionRevision})
}

// Concern revisions are captured by this endpoint, not supplied by the caller.
// An exact retry must reuse that first snapshot after the command advances it.
// Execute still verifies the actor, decision, answer and complete request hash.
func (h *Handler) ownerDecisionRequestExpected(ctx context.Context, workspaceID, issueID pgtype.UUID, kind workflowstatev2.CommandKind, key string) ([]workflowstatev2.ExpectedRevision, error) {
	command, err := h.Queries.GetIssueWorkflowCommandByKey(ctx, db.GetIssueWorkflowCommandByKeyParams{WorkspaceID: workspaceID, IssueID: issueID, CommandKey: key})
	if errors.Is(err, pgx.ErrNoRows) {
		return h.ownerDecisionExpected(ctx, workspaceID, issueID, kind)
	}
	if err != nil {
		return nil, err
	}
	var expected []workflowstatev2.ExpectedRevision
	if err := json.Unmarshal(command.ExpectedRevisions, &expected); err != nil {
		return nil, err
	}
	return expected, nil
}

func ownerDecisionMutationStatus(err error) int {
	if errors.Is(err, workflowcommand.ErrForbidden) {
		return http.StatusForbidden
	}
	return http.StatusConflict
}

func decodeWorkflowDecisionResponse(body io.Reader, out *workflowDecisionResponseRequest) error {
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func (h *Handler) writeOwnerDecisionStale(w http.ResponseWriter, r *http.Request, workspaceID, issueID pgtype.UUID, decisionID string, revision workflowstatev2.Revision) {
	current, currentErr := h.loadWorkflowDecisionPacket(r.Context(), workspaceID, issueID, "", "", requestUserID(r), true)
	var replacement any
	terminal := "replaced"
	if currentErr == nil {
		replacement, terminal = current, ""
	} else if submitted, submittedErr := h.loadWorkflowDecisionPacket(r.Context(), workspaceID, issueID, decisionID, "", requestUserID(r), false); submittedErr == nil {
		if state, _ := submitted.Decision["state"].(string); state != "" {
			terminal = state
		}
	}
	writeJSON(w, http.StatusConflict, map[string]any{"code": "decision_stale", "error": "workflow decision was replaced; reload the current decision", "submitted_decision_id": decisionID, "submitted_revision": revision, "current_decision": replacement, "terminal_state": terminal})
}

func (h *Handler) ownerDecisionExpected(ctx context.Context, workspaceID, issueID pgtype.UUID, kind workflowstatev2.CommandKind) ([]workflowstatev2.ExpectedRevision, error) {
	heads, err := h.Queries.ListIssueWorkflowConcernHeads(ctx, db.ListIssueWorkflowConcernHeadsParams{WorkspaceID: workspaceID, IssueID: issueID})
	if err != nil {
		return nil, err
	}
	wanted := map[workflowstatev2.Concern]bool{workflowstatev2.ConcernAttention: true}
	if kind == workflowstatev2.CommandRespondDecision {
		wanted[workflowstatev2.ConcernExecution] = true
	}
	expected := make([]workflowstatev2.ExpectedRevision, 0, len(wanted))
	for _, head := range heads {
		concern := workflowstatev2.Concern(head.ConcernType)
		if wanted[concern] {
			expected = append(expected, workflowstatev2.ExpectedRevision{Concern: concern, Revision: workflowstatev2.Revision(head.Revision)})
			delete(wanted, concern)
		}
	}
	if len(wanted) != 0 {
		return nil, pgx.ErrNoRows
	}
	sort.Slice(expected, func(i, j int) bool { return expected[i].Concern < expected[j].Concern })
	return expected, nil
}

func (h *Handler) loadWorkflowDecisionPacket(ctx context.Context, workspaceID, issueID pgtype.UUID, decisionID, attentionID, ownerID string, current bool) (workflowDecisionPacketV1, error) {
	rows, err := h.Queries.ListIssueWorkflowProjectionInputs(ctx, db.ListIssueWorkflowProjectionInputsParams{WorkspaceID: workspaceID, IssueID: issueID})
	if err != nil {
		return workflowDecisionPacketV1{}, err
	}
	var decision, attention map[string]any
	actions := map[string]map[string]any{}
	executions := map[string]map[string]any{}
	for _, row := range rows {
		var f map[string]any
		if json.Unmarshal(row.Payload, &f) != nil {
			return workflowDecisionPacketV1{}, errors.New("malformed authoritative workflow packet")
		}
		switch row.RecordType {
		case "execution":
			id, _ := f["id"].(string)
			executions[id] = f
		case "decision":
			id, _ := f["id"].(string)
			active := f["state"] == "unanswered" || f["resolution_state"] == "resuming"
			if f["accountable_owner_id"] == ownerID && ((current && active) || (!current && id == decisionID)) {
				decision = f
			}
		case "attention":
			id, _ := f["id"].(string)
			if attentionID != "" && id == attentionID {
				attention = f
			}
		case "action_receipt":
			id, _ := f["id"].(string)
			actions[id] = f
		}
	}
	if decision == nil && attentionID != "" && attention != nil {
		bound, _ := attention["decision_id"].(string)
		return h.loadWorkflowDecisionPacket(ctx, workspaceID, issueID, bound, attentionID, ownerID, false)
	}
	if decision == nil {
		return workflowDecisionPacketV1{}, pgx.ErrNoRows
	}
	if attention == nil {
		for _, row := range rows {
			if row.RecordType != "attention" {
				continue
			}
			var f map[string]any
			if json.Unmarshal(row.Payload, &f) == nil && f["decision_id"] == decision["id"] {
				attention = f
			}
		}
	}
	if attention == nil {
		return workflowDecisionPacketV1{}, pgx.ErrNoRows
	}
	var continuation map[string]any
	// Resolve the branch from the exact suspended execution even when there is
	// no continuation receipt. A receipt may supply its immutable snapshot below.
	if operationID, ok := decision["suspended_operation_id"].(string); ok {
		if execution := executions[operationID]; execution != nil {
			decision["branch_id"] = execution["branch_key"]
		}
	}
	if actionID, _ := attention["action_receipt_id"].(string); actionID != "" {
		if action := actions[actionID]; action != nil {
			continuation = map[string]any{"action_receipt_id": actionID, "state": action["state"]}
			if payload, ok := action["payload"].(map[string]any); ok {
				decision["branch_id"] = payload["branch_key"]
			}
		}
	}
	decision["type"] = decision["decision_type"]
	decision["operation_id"] = decision["suspended_operation_id"]
	return workflowDecisionPacketV1{ContractVersion: 1, IssueID: uuidToString(issueID), Decision: decision, Attention: attention, Continuation: continuation, Replacement: nil}, nil
}

func wholeRevision(value any) (workflowstatev2.Revision, bool) {
	number, ok := value.(float64)
	if !ok || number < 1 || number != float64(uint64(number)) {
		return 0, false
	}
	return workflowstatev2.Revision(number), true
}
