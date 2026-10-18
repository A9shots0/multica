package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/issuestatus"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Captured with the owner's message, not chosen later by an agent. It binds a
// discussion to the exact issue/decision the owner saw at send time.
var ErrChatWorkTargetChanged = errors.New("selected work changed; reopen Work and select the current issue before sending")

type ChatWorkTarget struct {
	// A sole linked issue provides conversational context for corrections only.
	// It never authorizes approval, cancellation, acceptance, or answering an Inbox request.
	CorrectionOnly        bool   `json:"correction_only,omitempty"`
	IssueID               string `json:"issue_id"`
	IssueRevision         int64  `json:"issue_revision"`
	OwnerID               string `json:"owner_id"`
	DecisionID            string `json:"decision_id,omitempty"`
	DecisionRevision      int64  `json:"decision_revision,omitempty"`
	RequestEventID        string `json:"request_event_id,omitempty"`
	IntakeID              string `json:"intake_id,omitempty"`
	RequestID             string `json:"request_id,omitempty"`
	NeedsRequestSelection bool   `json:"needs_request_selection,omitempty"`
	ResultTaskID          string `json:"result_task_id,omitempty"`
	ResultDigest          string `json:"result_digest,omitempty"`
}

func CaptureConversationCorrection(ctx context.Context, q *db.Queries, session db.ChatSession, owner pgtype.UUID) (ChatWorkTarget, error) {
	links, err := q.ListIssuesForChat(ctx, db.ListIssuesForChatParams{WorkspaceID: session.WorkspaceID, ChatSessionID: session.ID})
	if err != nil || len(links) != 1 {
		return ChatWorkTarget{}, err
	}
	issue, err := q.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{WorkspaceID: session.WorkspaceID, ID: links[0].ID})
	if err != nil {
		return ChatWorkTarget{}, err
	}
	category := issuestatus.Effective(ctx, q, issue.WorkspaceID, issue.Status)
	if category == "done" || category == "cancelled" {
		return ChatWorkTarget{}, nil
	}
	target, err := CaptureChatWorkTarget(ctx, q, session, owner, issue.ID)
	if err != nil {
		return ChatWorkTarget{}, err
	}
	if target.RequestID != "" || target.NeedsRequestSelection {
		return ChatWorkTarget{}, nil
	}
	target.CorrectionOnly = true
	target.ResultTaskID, target.ResultDigest = "", ""
	return target, nil
}

// ChatWorkResponseInput is one answer the owner intentionally composed for one
// guarded work request. Several inputs may travel in one conversational turn;
// they remain independent targets and are never collapsed into one decision.
type ChatWorkResponseInput struct {
	Target  ChatWorkTarget `json:"target"`
	Content string         `json:"content"`
}

// ChatWorkResponse is the frozen server-side form exposed to Mika. ResponseID
// is deterministic for the selected target so the runtime can submit each
// answer exactly once without being allowed to choose a different target.
type ChatWorkResponse struct {
	ResponseID string         `json:"response_id"`
	Target     ChatWorkTarget `json:"target"`
	Content    string         `json:"content"`
}

func ChatWorkResponseID(target ChatWorkTarget) string {
	raw, _ := json.Marshal(target)
	digest := sha256.Sum256(raw)
	return fmt.Sprintf("response-%x", digest[:8])
}

func CaptureChatWorkTarget(ctx context.Context, q *db.Queries, session db.ChatSession, owner, issueID pgtype.UUID, selectedRequest ...string) (ChatWorkTarget, error) {
	target := ChatWorkTarget{}
	if owner != session.CreatorID {
		return target, errors.New("work target belongs to another owner")
	}
	links, err := q.ListIssuesForChat(ctx, db.ListIssuesForChatParams{WorkspaceID: session.WorkspaceID, ChatSessionID: session.ID})
	if err != nil {
		return target, err
	}
	linked := false
	for _, link := range links {
		if link.ID == issueID {
			linked = true
			break
		}
	}
	if !linked {
		return target, errors.New("work target is not linked to this conversation")
	}
	issue, err := q.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{WorkspaceID: session.WorkspaceID, ID: issueID})
	if err != nil {
		return target, err
	}
	target = ChatWorkTarget{IssueID: util.UUIDToString(issue.ID), IssueRevision: issue.Revision, OwnerID: util.UUIDToString(owner)}
	items, err := q.ListInboxItems(ctx, db.ListInboxItemsParams{WorkspaceID: session.WorkspaceID, RecipientType: "member", RecipientID: owner})
	if err != nil {
		return target, err
	}
	requestID := ""
	if len(selectedRequest) > 0 {
		requestID = selectedRequest[0]
	}
	candidates := []db.ListInboxItemsRow{}
	for _, item := range items {
		if item.IssueID != issueID {
			continue
		}
		var details map[string]string
		if err := json.Unmarshal(item.Details, &details); err != nil {
			continue
		}
		if details["intake_state"] == "continued" {
			continue
		}
		if item.Type != "workflow_owner_decision" && !(item.Type == "chat_intake" && details["question_scope"] == "ordinary_task") {
			continue
		}
		if requestID == "" || util.UUIDToString(item.ID) == requestID {
			candidates = append(candidates, item)
		}
	}
	if requestID != "" && len(candidates) != 1 {
		return target, ErrChatWorkTargetChanged
	}
	if len(candidates) > 1 {
		target.NeedsRequestSelection = true
		return target, nil
	}
	for _, item := range candidates {
		var details map[string]string
		_ = json.Unmarshal(item.Details, &details)
		target.RequestID = util.UUIDToString(item.ID)
		if details["question_scope"] == "ordinary_task" {
			target.IntakeID = util.UUIDToString(item.ID)
			break
		}
		if item.Type == "workflow_owner_decision" {
			target.DecisionID = details["decision_id"]
			target.RequestEventID = details["workflow_event_id"]
			if target.DecisionID != "" {
				rows, err := q.ListIssueWorkflowProjectionInputs(ctx, db.ListIssueWorkflowProjectionInputsParams{WorkspaceID: session.WorkspaceID, IssueID: issueID})
				if err != nil {
					return target, err
				}
				for _, row := range rows {
					if row.RecordType != "decision" {
						continue
					}
					var decision struct {
						ID       string `json:"id"`
						Revision int64  `json:"revision"`
					}
					if json.Unmarshal(row.Payload, &decision) == nil && decision.ID == target.DecisionID {
						target.DecisionRevision = decision.Revision
						break
					}
				}
			}
			break
		}
	}
	if target.RequestID == "" {
		result, err := q.GetChatAcceptableResult(ctx, db.GetChatAcceptableResultParams{WorkspaceID: session.WorkspaceID, IssueID: issueID, OwnerID: owner})
		if err == nil {
			target.ResultTaskID = util.UUIDToString(result.ID)
			target.ResultDigest = ChatResultDigest(result.Result)
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return target, err
		}
	}
	return target, nil
}

func ChatResultDigest(result []byte) string { return fmt.Sprintf("%x", sha256.Sum256(result)) }
