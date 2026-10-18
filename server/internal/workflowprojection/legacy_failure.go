package workflowprojection

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/workflowstore"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// StoreLegacyTechnicalStop publishes one compatibility Current Picture only
// when the issue has no authoritative normalized projection. A v2/v3 picture
// always wins and is never replaced by this legacy failure adapter.
func StoreLegacyTechnicalStop(ctx context.Context, starter workflowstore.TxStarter, queries *db.Queries, workspaceID, issueID, taskID pgtype.UUID, reason string, generatedAt time.Time) (db.IssueCurrentPicture, bool, error) {
	params, err := LegacyTechnicalStopParams(workspaceID, issueID, taskID, reason, generatedAt)
	if err != nil {
		return db.IssueCurrentPicture{}, false, err
	}
	var stored db.IssueCurrentPicture
	err = workflowstore.InIssueWriteTransaction(ctx, starter, queries, workspaceID, issueID, func(write *workflowstore.IssueWrite) error {
		var writeErr error
		stored, writeErr = write.UpsertCurrentPictureCAS(ctx, params)
		return writeErr
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.IssueCurrentPicture{}, false, nil
	}
	return stored, err == nil, err
}

// LegacyTechnicalStopParams builds the compatibility projection write so a
// caller that already owns a transaction can commit the plan, issue status,
// Current Picture, and owner Inbox admission as one durable unit.
func LegacyTechnicalStopParams(workspaceID, issueID, taskID pgtype.UUID, reason string, generatedAt time.Time) (db.UpsertIssueCurrentPictureParams, error) {
	projection, err := legacyTechnicalStopProjection(taskID, reason, generatedAt)
	if err != nil {
		return db.UpsertIssueCurrentPictureParams{}, err
	}
	return projection.params(workspaceID, issueID, 0), nil
}

func legacyTechnicalStopProjection(taskID pgtype.UUID, reason string, generatedAt time.Time) (Projection, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "workflow task stopped before recording its required event"
	}
	taskIDText := uuidString(taskID)
	occurredAt := generatedAt.UTC().Format(time.RFC3339Nano)
	execution := map[string]any{"id": taskIDText, "revision": int64(1), "state": "failed", "terminal_reason": reason}
	latest := map[string]any{
		"id": taskIDText, "revision": int64(1), "state": "failed", "record_type": "execution",
		"change_code": "execution_changed", "result_code": "failed", "occurred_at": occurredAt,
	}
	document := Document{
		ContractVersion: ContractVersion, OwnerSummarySchemaVersion: OwnerIssueSummarySchemaVersion,
		OriginalBusinessRequest: map[string]any{}, Outcome: map[string]any{"state": "open"},
		HistoricalCompletion: map[string]any{}, Scope: map[string]any{}, Authority: map[string]any{},
		Execution: execution, ActiveBranches: []map[string]any{}, PrimaryBranch: map[string]any{},
		Release: map[string]any{"state": "not_required"}, ObservedRelease: map[string]any{}, Health: "unknown",
		Attention: map[string]any{}, CurrentMilestone: "technical_stop", LatestMeaningfulChange: latest,
		NextAutomaticAction: map[string]any{}, ActiveActor: map[string]any{}, WaitReason: map[string]any{},
		Candidate: map[string]any{}, Target: map[string]any{}, EvidenceSummary: map[string]any{}, AuthoritySummary: map[string]any{},
		UnresolvedDecisions: []map[string]any{}, UnresolvedIncidents: []map[string]any{}, UnresolvedDependencies: []map[string]any{},
		Budgets: []map[string]any{}, Supersession: map[string]any{}, TerminalReason: reason, NextActor: map[string]any{},
	}
	documentJSON, err := json.Marshal(document)
	if err != nil {
		return Projection{}, err
	}
	latestJSON, _ := json.Marshal(latest)
	sourcesJSON, _ := json.Marshal([]SourceRevision{{Type: "execution", ID: taskIDText, Revision: 1}})
	return Projection{
		ContractVersion: ContractVersion, Revision: 1, OutcomeState: "open", OutcomeOpen: true,
		ExecutionState: "failed", ReleaseState: "not_required", HealthState: "unknown", AttentionState: "resolved",
		CurrentMilestone: "technical_stop", LatestMeaningfulChange: latestJSON,
		NextAutomaticAction: []byte(`{}`), ActiveActor: []byte(`{}`), WaitReason: []byte(`{}`),
		Candidate: []byte(`{}`), Target: []byte(`{}`), EvidenceSummary: []byte(`{}`), AuthoritySummary: []byte(`{}`),
		UnresolvedDecisions: []byte(`[]`), UnresolvedIncidents: []byte(`[]`), UnresolvedDependencies: []byte(`[]`),
		BudgetSummary: []byte(`[]`), SupersessionSummary: []byte(`{}`), TerminalReason: reason,
		NextActor: []byte(`{}`), SourceRevisions: sourcesJSON, Document: documentJSON, GeneratedAt: generatedAt.UTC(),
	}, nil
}
