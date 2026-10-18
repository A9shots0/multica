package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	googleuuid "github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/workflowprojection"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// projectOwnerInboxItem turns an authoritative Current Picture transition into
// one durable owner-control record. It intentionally ignores routine progress:
// only a decision, material incident, meaningful change, or verified terminal
// outcome is admitted to Inbox.
func projectOwnerInboxItem(ctx context.Context, q *db.Queries, workspaceID, issueID pgtype.UUID, revision int64) (*db.InboxItem, error) {
	issue, err := q.GetIssue(ctx, issueID)
	if err != nil || issue.WorkspaceID != workspaceID {
		return nil, err
	}
	picture, err := q.GetIssueCurrentPicture(ctx, db.GetIssueCurrentPictureParams{WorkspaceID: workspaceID, IssueID: issueID})
	if err != nil || picture.Revision != revision {
		return nil, err
	}
	if external, isExternal := externalWorkInboxProjection(picture.LatestMeaningfulChange); isExternal {
		if external == nil {
			return nil, nil
		}
		return upsertExternalWorkInboxItem(ctx, q, issue, workspaceID, issueID, external)
	}
	summary := workflowprojection.OwnerIssueSummaryFromDocument(picture.Document, picture.Revision, picture.GeneratedAt.Time)
	category, itemType, severity, referenceType, referenceID := "", "", "info", "", ""
	for _, signal := range summary.Signals {
		if len(signal.References) == 0 {
			continue
		}
		ref := signal.References[0]
		switch signal.Code {
		case "owner_attention_required":
			category, itemType, severity, referenceType, referenceID = "needs_you", "workflow_owner_decision", "action_required", ref.Type, ref.ID
		case "current_incident", "technical_stop", "current_regression", "rollback_in_progress", "service_restored_outcome_open":
			if category != "needs_you" {
				category, itemType, severity, referenceType, referenceID = "incident", "workflow_incident", "attention", ref.Type, ref.ID
			}
		case "outcome_complete":
			if category == "" {
				category, itemType, severity, referenceType, referenceID = "completion", "task_completed", "info", ref.Type, ref.ID
			}
		}
	}
	if category == "" && summary.LatestMeaningfulChange != nil {
		change := summary.LatestMeaningfulChange
		category, itemType, referenceType, referenceID = "update", "workflow_meaningful_update", change.Reference.Type, change.Reference.ID
	}
	if category == "" {
		return nil, nil
	}
	if referenceID == "" {
		referenceID = fmt.Sprintf("picture-%d", revision)
	}
	details := map[string]string{
		"owner_relevant": "true", "owner_category": category,
		"current_picture_revision": fmt.Sprintf("%d", revision),
		"reference_type":           referenceType, "reference_id": referenceID,
	}
	body := "The authoritative workflow picture changed."
	if summary.LatestMeaningfulChange != nil {
		details["change_code"] = summary.LatestMeaningfulChange.Code
		details["result_code"] = summary.LatestMeaningfulChange.ResultCode
		body = summary.LatestMeaningfulChange.Code
	}
	if category == "needs_you" {
		details["decision_id"] = referenceID
		details["required_action"] = "respond"
		body = "A bounded owner decision is required before work can continue."
	}
	if category == "completion" {
		details["independently_verified"] = "true"
		body = "The accepted outcome is independently verified and complete."
		var evidence map[string]any
		if json.Unmarshal(picture.EvidenceSummary, &evidence) == nil && evidence["completion_mode"] == "operator_assisted" {
			details["completion_mode"] = "operator_assisted"
			body = "The requesting member accepted the published outcome after independent review, with operator assistance."
		}
	}
	encoded, _ := json.Marshal(details)
	stable := googleuuid.NewSHA1(googleuuid.NameSpaceURL, []byte(fmt.Sprintf("multica-owner-inbox:%s:%s:%s:%s", uuidToString(workspaceID), uuidToString(issueID), category, referenceID)))
	recipientID := issue.CreatorID
	if plan, err := q.GetIssueWorkflowPlan(ctx, db.GetIssueWorkflowPlanParams{IssueID: issueID, WorkspaceID: workspaceID}); err == nil && len(plan.OwnerRuntimeRoute) > 0 {
		var authority struct {
			OwnerID string `json:"owner_id"`
		}
		if json.Unmarshal(plan.OwnerRuntimeRoute, &authority) != nil {
			return nil, errors.New("owner route recipient unavailable")
		}
		id, err := googleuuid.Parse(authority.OwnerID)
		if err != nil {
			return nil, err
		}
		recipientID = parseUUID(id.String())
		created, err := q.CreateOwnerRuntimeInboxNotice(ctx, db.CreateOwnerRuntimeInboxNoticeParams{ID: parseUUID(stable.String()), WorkspaceID: workspaceID, RecipientID: recipientID, Type: itemType, Severity: severity, IssueID: issueID, Title: issue.Title, Body: pgtype.Text{String: body, Valid: true}, Details: encoded})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return &created, nil
	}
	if issue.CreatorType != "member" || !issue.CreatorID.Valid {
		return nil, nil
	}
	created, err := q.CreateInboxItem(ctx, db.CreateInboxItemParams{
		ID: parseUUID(stable.String()), WorkspaceID: workspaceID, RecipientType: "member", RecipientID: recipientID,
		Type: itemType, Severity: severity, IssueID: issueID, Title: issue.Title,
		Body: pgtype.Text{String: body, Valid: true}, ActorType: pgtype.Text{String: "system", Valid: true}, Details: encoded,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, nil
		}
		return nil, err
	}
	return &created, nil
}

type externalWorkInboxUpdate struct {
	Provider        string
	WorkID          string
	ReportedState   string
	ReceiptRevision int64
	Milestone       string
	Change          string
	Dependencies    []string
	NextAction      string
	Question        string
	Options         []string
}

func externalWorkInboxProjection(raw []byte) (*externalWorkInboxUpdate, bool) {
	var latest struct {
		ExternalWork json.RawMessage `json:"external_work"`
	}
	if json.Unmarshal(raw, &latest) != nil || len(latest.ExternalWork) == 0 || string(latest.ExternalWork) == "null" {
		return nil, false
	}
	var external struct {
		Provider        string   `json:"provider"`
		WorkID          string   `json:"work_id"`
		ReportedState   string   `json:"reported_state"`
		ReceiptRevision int64    `json:"receipt_revision"`
		Milestone       string   `json:"current_milestone"`
		Change          string   `json:"change_since_last_receipt"`
		Dependencies    []string `json:"dependencies"`
		NextAction      string   `json:"next_action"`
		OwnerDecision   struct {
			Required bool     `json:"required"`
			Question *string  `json:"question"`
			Options  []string `json:"options"`
		} `json:"owner_decision_proposal"`
	}
	if json.Unmarshal(latest.ExternalWork, &external) != nil {
		return nil, true
	}
	state := strings.TrimSpace(external.ReportedState)
	if strings.TrimSpace(external.Provider) == "" || strings.TrimSpace(external.WorkID) == "" || external.ReceiptRevision < 1 || state == "working" {
		return nil, true
	}
	if state != "complete" && state != "blocked" && state != "stopped" && state != "waiting_owner" && state != "waiting_external" {
		return nil, true
	}
	question := ""
	if external.OwnerDecision.Required && external.OwnerDecision.Question != nil {
		question = strings.TrimSpace(*external.OwnerDecision.Question)
	}
	return &externalWorkInboxUpdate{
		Provider: strings.TrimSpace(external.Provider), WorkID: strings.TrimSpace(external.WorkID),
		ReportedState: state, ReceiptRevision: external.ReceiptRevision,
		Milestone: strings.TrimSpace(external.Milestone), Change: strings.TrimSpace(external.Change),
		Dependencies: external.Dependencies, NextAction: strings.TrimSpace(external.NextAction),
		Question: question, Options: external.OwnerDecision.Options,
	}, true
}

func upsertExternalWorkInboxItem(ctx context.Context, q *db.Queries, issue db.Issue, workspaceID, issueID pgtype.UUID, external *externalWorkInboxUpdate) (*db.InboxItem, error) {
	recipientID := issue.CreatorID
	if plan, err := q.GetIssueWorkflowPlan(ctx, db.GetIssueWorkflowPlanParams{IssueID: issueID, WorkspaceID: workspaceID}); err == nil && len(plan.OwnerRuntimeRoute) > 0 {
		var authority struct {
			OwnerID string `json:"owner_id"`
		}
		if json.Unmarshal(plan.OwnerRuntimeRoute, &authority) != nil {
			return nil, errors.New("owner route recipient unavailable")
		}
		id, err := googleuuid.Parse(authority.OwnerID)
		if err != nil {
			return nil, err
		}
		recipientID = parseUUID(id.String())
	} else if issue.CreatorType != "member" || !issue.CreatorID.Valid {
		return nil, nil
	}

	category, severity := "update", "info"
	if external.ReportedState == "blocked" || external.ReportedState == "stopped" ||
		(external.ReportedState == "waiting_external" && len(external.Dependencies) > 0) {
		category, severity = "incident", "attention"
	} else if external.ReportedState == "waiting_owner" {
		category = "needs_you"
	}
	details := map[string]string{
		"owner_relevant": "true", "owner_category": category,
		"external_work": "true", "provider_report_only": "true",
		"provider": external.Provider, "work_id": external.WorkID,
		"reported_state":    external.ReportedState,
		"receipt_revision":  fmt.Sprintf("%d", external.ReceiptRevision),
		"current_milestone": external.Milestone, "next_action": external.NextAction,
	}
	if len(external.Dependencies) > 0 {
		details["blocker"] = strings.TrimSpace(external.Dependencies[0])
		details["blocker_count"] = fmt.Sprintf("%d", len(external.Dependencies))
	}
	if external.Question != "" {
		details["question"] = external.Question
		details["external_decision_actionable"] = "false"
	}
	if len(external.Options) > 0 {
		details["options"] = strings.Join(external.Options, "\n")
	}
	body := external.Change
	if body == "" {
		body = external.Milestone
	}
	encoded, _ := json.Marshal(details)
	stable := googleuuid.NewSHA1(googleuuid.NameSpaceURL, []byte(fmt.Sprintf("multica-external-work-inbox:%s:%s:%s:%s", uuidToString(workspaceID), uuidToString(issueID), external.Provider, external.WorkID)))
	created, err := q.UpsertExternalWorkInboxItem(ctx, db.UpsertExternalWorkInboxItemParams{
		ID: parseUUID(stable.String()), WorkspaceID: workspaceID, RecipientID: recipientID,
		Severity: severity, IssueID: issueID, Title: issue.Title,
		Body: pgtype.Text{String: body, Valid: body != ""}, Details: encoded,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("external work inbox identity conflict")
	}
	if err != nil {
		return nil, err
	}
	return &created, nil
}
