package workflowprojection

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// LegacyNonLiveSuccessParams builds the one compatibility Current Picture
// needed when the legacy controller accepts an independently reviewed,
// non-deployment workflow as Done. The caller must write it in the same
// transaction as the accepted event and owner-Inbox admission.
//
// ExpectedRevision is deliberately zero. This adapter may create missing
// compatibility truth, but it must never replace a normalized v2/v3 picture or
// a previously recorded legacy technical stop.
func LegacyNonLiveSuccessParams(
	workspaceID, issueID, eventID, taskID, actorID pgtype.UUID,
	reason string,
	generatedAt time.Time,
) (db.UpsertIssueCurrentPictureParams, error) {
	zeroUUID := [16]byte{}
	if !workspaceID.Valid || workspaceID.Bytes == zeroUUID ||
		!issueID.Valid || issueID.Bytes == zeroUUID ||
		!eventID.Valid || eventID.Bytes == zeroUUID ||
		!taskID.Valid || taskID.Bytes == zeroUUID ||
		!actorID.Valid || actorID.Bytes == zeroUUID || generatedAt.IsZero() {
		return db.UpsertIssueCurrentPictureParams{}, errors.New("legacy non-live success requires exact workspace, issue, event, task, actor, and time")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return db.UpsertIssueCurrentPictureParams{}, errors.New("legacy non-live success requires review evidence")
	}

	const revision int64 = 1
	eventIDText := uuidString(eventID)
	taskIDText := uuidString(taskID)
	actorIDText := uuidString(actorID)
	occurredAt := generatedAt.UTC().Format(time.RFC3339Nano)
	outcome := map[string]any{"id": eventIDText, "revision": revision, "state": "satisfied"}
	scope := map[string]any{"id": eventIDText, "revision": revision, "requires_deployment": false}
	execution := map[string]any{
		"id": taskIDText, "revision": revision, "state": "completed",
		"task_id": taskIDText, "actor_type": "agent", "actor_id": actorIDText,
	}
	latest := map[string]any{
		"id": eventIDText, "revision": revision, "state": "passed", "record_type": "review",
		"change_code": "review_recorded", "result_code": "succeeded", "occurred_at": occurredAt,
	}
	document := Document{
		ContractVersion: ContractVersion, OwnerSummarySchemaVersion: OwnerIssueSummarySchemaVersion,
		OriginalBusinessRequest: map[string]any{}, Outcome: outcome, HistoricalCompletion: map[string]any{},
		Scope: scope, Authority: map[string]any{}, Execution: execution,
		ActiveBranches: []map[string]any{}, PrimaryBranch: map[string]any{},
		Release: map[string]any{"state": "not_required"}, ObservedRelease: map[string]any{},
		Health: "unknown", Attention: map[string]any{}, CurrentMilestone: "complete",
		LatestMeaningfulChange: latest, NextAutomaticAction: map[string]any{},
		ActiveActor: map[string]any{}, WaitReason: map[string]any{}, Candidate: map[string]any{},
		Target: map[string]any{}, EvidenceSummary: map[string]any{"independent_review": reason},
		AuthoritySummary: map[string]any{}, UnresolvedDecisions: []map[string]any{},
		UnresolvedIncidents: []map[string]any{}, UnresolvedDependencies: []map[string]any{},
		Budgets: []map[string]any{}, Supersession: map[string]any{}, TerminalReason: "",
		NextActor: map[string]any{},
	}
	documentJSON, err := json.Marshal(document)
	if err != nil {
		return db.UpsertIssueCurrentPictureParams{}, err
	}
	latestJSON, err := json.Marshal(latest)
	if err != nil {
		return db.UpsertIssueCurrentPictureParams{}, err
	}
	sourcesJSON, err := json.Marshal([]SourceRevision{{Type: "legacy_workflow_event", ID: eventIDText, Revision: revision}})
	if err != nil {
		return db.UpsertIssueCurrentPictureParams{}, err
	}
	scopeRevision := int64(revision)
	projection := Projection{
		ContractVersion: ContractVersion, Revision: revision, ScopeRevision: &scopeRevision,
		OutcomeState: "satisfied", OutcomeOpen: false, ExecutionState: "completed",
		ReleaseState: "not_required", HealthState: "unknown", AttentionState: "resolved",
		CurrentMilestone: "complete", LatestMeaningfulChange: latestJSON,
		NextAutomaticAction: []byte(`{}`), ActiveActor: []byte(`{}`), WaitReason: []byte(`{}`),
		Candidate: []byte(`{}`), Target: []byte(`{}`),
		EvidenceSummary:  mustJSON(map[string]any{"independent_review": reason}),
		AuthoritySummary: []byte(`{}`), UnresolvedDecisions: []byte(`[]`),
		UnresolvedIncidents: []byte(`[]`), UnresolvedDependencies: []byte(`[]`),
		BudgetSummary: []byte(`[]`), SupersessionSummary: []byte(`{}`), TerminalReason: "",
		NextActor: []byte(`{}`), SourceRevisions: sourcesJSON, Document: documentJSON,
		GeneratedAt: generatedAt.UTC(),
	}
	return projection.params(workspaceID, issueID, 0), nil
}
