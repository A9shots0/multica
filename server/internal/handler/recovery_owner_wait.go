package handler

import (
	"context"
	"encoding/json"
	"strings"

	googleuuid "github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/issuestatus"
	"github.com/multica-ai/multica/server/internal/workflowprojection"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const recoveryOwnerWaitFallbackConsequence = "This issue stays paused until the owner answers; the existing work remains attached to the same issue."

type recoveryOwnerWait struct {
	Kind        string
	Question    string
	Consequence string
	DecisionID  string
	Revision    int64
	OccurredAt  string
	OwnerID     string
}

func recoveryOwnerWaitFromMetadata(metadata map[string]any, issueID, ownerID string, revision int64, occurredAt string) (recoveryOwnerWait, bool) {
	kind, _ := metadata["recovery.wait_kind"].(string)
	kind = strings.TrimSpace(kind)
	if kind != "owner" && !strings.HasPrefix(kind, "owner_") {
		return recoveryOwnerWait{}, false
	}
	question, _ := metadata["recovery.wait_reason"].(string)
	question = strings.TrimSpace(question)
	if question == "" || issueID == "" || revision < 1 {
		return recoveryOwnerWait{}, false
	}
	consequence, _ := metadata["recovery.next_action"].(string)
	consequence = strings.TrimSpace(consequence)
	if consequence == "" {
		consequence = recoveryOwnerWaitFallbackConsequence
	}
	decisionID := googleuuid.NewSHA1(
		googleuuid.NameSpaceURL,
		[]byte("multica-recovery-owner-wait:"+issueID+":"+kind+":"+question),
	).String()
	return recoveryOwnerWait{
		Kind: kind, Question: question, Consequence: consequence,
		DecisionID: decisionID, Revision: revision, OccurredAt: occurredAt,
		OwnerID: ownerID,
	}, true
}

func recoveryOwnerWaitJSON(value any) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return encoded
}

// applyRecoveryOwnerWaitFallback projects legacy recovery bookkeeping only
// when no authoritative normalized Current Picture exists. The raw issue
// status stays untouched: this is a read model derived from the durable wait
// kind, question and consequence, not a cosmetic label rewrite.
func applyRecoveryOwnerWaitFallback(issue *IssueResponse) {
	if issue == nil || issue.CurrentPicture != nil {
		return
	}
	category := issue.StatusCategory
	if category == "" {
		category = issue.Status
	}
	if category != "blocked" {
		return
	}
	ownerID := ""
	if issue.CreatorType == "member" {
		ownerID = issue.CreatorID
	}
	wait, ok := recoveryOwnerWaitFromMetadata(issue.Metadata, issue.ID, ownerID, issue.Revision, issue.UpdatedAt)
	if !ok {
		return
	}
	reference := workflowprojection.OwnerIssueReference{
		Type: "recovery_wait", ID: wait.DecisionID, Revision: wait.Revision,
	}
	change := &workflowprojection.OwnerMeaningfulChange{
		Code: "decision_requested", ResultCode: "recorded",
		OccurredAt: wait.OccurredAt, Reference: reference,
	}
	decision := map[string]any{
		"id": wait.DecisionID, "revision": wait.Revision,
		"state": "unanswered", "resolution_state": "open",
		"decision_type": "authorization", "question": wait.Question,
		"consequence": wait.Consequence,
		"source":      "recovery_metadata", "wait_kind": wait.Kind,
	}
	if wait.OwnerID != "" {
		decision["accountable_owner_id"] = wait.OwnerID
	}
	waitReason := map[string]any{
		"kind": "owner", "wait_kind": wait.Kind, "question": wait.Question,
		"consequence": wait.Consequence, "decision_id": wait.DecisionID,
	}
	issue.CurrentPicture = &issueCurrentPictureCompactResponse{
		ContractVersion: 2,
		OwnerSummary: workflowprojection.OwnerIssueSummary{
			SchemaVersion:          workflowprojection.OwnerIssueSummarySchemaVersion,
			CurrentPictureRevision: wait.Revision,
			Stage:                  workflowprojection.OwnerStageNeedsOwner,
			Signals: []workflowprojection.OwnerIssueSignal{{
				Code: "owner_attention_required", References: []workflowprojection.OwnerIssueReference{reference},
			}},
			NextAction: &workflowprojection.OwnerIssueNextAction{
				Kind: "answer_decision", Automatic: false, ActorType: "owner", ActorID: wait.OwnerID,
			},
			LatestMeaningfulChange: change,
		},
		Revision:     wait.Revision,
		OutcomeState: "open", OutcomeOpen: true,
		ExecutionState: "paused", ReleaseState: "not_required",
		HealthState: "unknown", AttentionState: "open",
		CurrentMilestone: "needs_attention",
		LatestMeaningfulChange: recoveryOwnerWaitJSON(map[string]any{
			"id": wait.DecisionID, "revision": wait.Revision,
			"record_type": "recovery_wait", "change_code": "decision_requested",
			"result_code": "recorded", "occurred_at": wait.OccurredAt,
		}),
		NextAutomaticAction: recoveryOwnerWaitJSON(map[string]any{}),
		ActiveActor:         recoveryOwnerWaitJSON(map[string]any{}),
		WaitReason:          recoveryOwnerWaitJSON(waitReason),
		Candidate:           recoveryOwnerWaitJSON(map[string]any{}),
		Target:              recoveryOwnerWaitJSON(map[string]any{}),
		EvidenceSummary: recoveryOwnerWaitJSON(map[string]any{
			"recovery_wait_kind": wait.Kind, "question": wait.Question,
			"consequence": wait.Consequence,
		}),
		UnresolvedDecisions:    recoveryOwnerWaitJSON([]map[string]any{decision}),
		UnresolvedIncidents:    recoveryOwnerWaitJSON([]map[string]any{}),
		UnresolvedDependencies: recoveryOwnerWaitJSON([]map[string]any{}),
		NextActor:              recoveryOwnerWaitJSON(map[string]any{"type": "owner", "id": wait.OwnerID}),
		GeneratedAt:            wait.OccurredAt,
	}
	if wait.OwnerID == "" {
		issue.CurrentPicture.OwnerSummary.NextAction.ActorID = ""
		issue.CurrentPicture.NextActor = recoveryOwnerWaitJSON(map[string]any{"type": "owner"})
	}
}

func (h *Handler) recoveryOwnerWaitRecipient(ctx context.Context, issue db.Issue) (pgtype.UUID, error) {
	recipient := pgtype.UUID{}
	switch issue.CreatorType {
	case "member":
		recipient = issue.CreatorID
	case "agent":
		agent, err := h.Queries.GetAgent(ctx, issue.CreatorID)
		if err != nil {
			return pgtype.UUID{}, err
		}
		if uuidToString(agent.WorkspaceID) != uuidToString(issue.WorkspaceID) {
			return pgtype.UUID{}, nil
		}
		recipient = agent.OwnerID
	default:
		return pgtype.UUID{}, nil
	}
	if !recipient.Valid {
		return pgtype.UUID{}, nil
	}
	if _, err := h.Queries.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{
		UserID: recipient, WorkspaceID: issue.WorkspaceID,
	}); err != nil {
		return pgtype.UUID{}, err
	}
	return recipient, nil
}

// projectRecoveryOwnerWaitInbox admits the same compatibility fact to Inbox.
// It deliberately creates no executable decision id: legacy recovery metadata
// is enough to classify and explain the wait, but not enough to invent typed
// approval authority. The existing typed workflow path remains the only place
// an Approve/Reject control can execute work.
func (h *Handler) projectRecoveryOwnerWaitInbox(ctx context.Context, issue db.Issue) error {
	if !issue.CreatorID.Valid || issuestatus.Effective(ctx, h.Queries, issue.WorkspaceID, issue.Status) != "blocked" {
		return nil
	}
	recipient, err := h.recoveryOwnerWaitRecipient(ctx, issue)
	if err != nil || !recipient.Valid {
		return err
	}
	wait, ok := recoveryOwnerWaitFromMetadata(
		parseIssueMetadata(issue.Metadata), uuidToString(issue.ID), uuidToString(recipient),
		issue.Revision, timestampToString(issue.UpdatedAt),
	)
	if !ok {
		return nil
	}
	details := map[string]string{
		"owner_relevant": "true", "owner_category": "needs_you",
		"required_action": "respond", "recovery_owner_wait": "true",
		"recovery_wait_kind": wait.Kind, "question": wait.Question,
		"consequence": wait.Consequence,
	}
	encoded, err := json.Marshal(details)
	if err != nil {
		return err
	}
	_, err = h.Queries.UpsertRecoveryOwnerWaitInboxItem(ctx, db.UpsertRecoveryOwnerWaitInboxItemParams{
		WorkspaceID: issue.WorkspaceID, RecipientID: recipient, IssueID: issue.ID,
		Title: issue.Title, Body: pgtype.Text{String: wait.Question, Valid: true}, Details: encoded,
	})
	return err
}
