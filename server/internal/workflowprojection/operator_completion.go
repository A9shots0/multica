package workflowprojection

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// OperatorAssistedCompletionParams settles only the known legacy stop shape.
// Normalized workflow concerns remain owned by their typed command path.
// The caller retains the complete previous picture in the operator event.
func OperatorAssistedCompletionParams(previous *db.IssueCurrentPicture, workspaceID, issueID, eventID, reviewTaskID, reviewerID, operatorID pgtype.UUID, reason, candidateSHA256 string, generatedAt time.Time) (db.UpsertIssueCurrentPictureParams, error) {
	expected := int64(0)
	if previous != nil {
		var sources []SourceRevision
		if previous.WorkspaceID != workspaceID || previous.IssueID != issueID || previous.Revision != 1 ||
			previous.ContractVersion != ContractVersion || previous.CurrentMilestone != "technical_stop" ||
			previous.ExecutionState != "failed" || previous.OutcomeState != "open" || !previous.OutcomeOpen ||
			previous.ScopeRevision.Valid || previous.AuthorityRevision.Valid || previous.ReleaseState != "not_required" ||
			json.Unmarshal(previous.SourceRevisions, &sources) != nil || len(sources) != 1 || sources[0].Type != "execution" || sources[0].Revision != 1 {
			return db.UpsertIssueCurrentPictureParams{}, errors.New("operator completion cannot replace normalized or unrelated Current Picture truth")
		}
		expected = previous.Revision
	}
	if !operatorID.Valid || operatorID.Bytes == [16]byte{} {
		return db.UpsertIssueCurrentPictureParams{}, errors.New("operator completion requires its human actor")
	}
	params, err := LegacyNonLiveSuccessParams(workspaceID, issueID, eventID, reviewTaskID, reviewerID, reason, generatedAt)
	if err != nil {
		return params, err
	}
	var document Document
	if err := json.Unmarshal(params.Document, &document); err != nil {
		return params, err
	}
	revision := expected + 1
	document.Outcome["revision"] = revision
	document.Scope["revision"] = revision
	document.LatestMeaningfulChange = map[string]any{
		"id": uuidString(eventID), "revision": int64(1), "record_type": "legacy_workflow_event",
		"change_code": "outcome_changed", "result_code": "succeeded", "state": "satisfied",
		"occurred_at": generatedAt.UTC().Format(time.RFC3339Nano),
	}
	document.EvidenceSummary = map[string]any{
		"completion_mode": "operator_assisted", "operator_id": uuidString(operatorID),
		"independent_review": reason, "review_task_id": uuidString(reviewTaskID),
		"candidate_sha256": candidateSHA256, "review_candidate_binding": "operator_attested",
	}
	params.ExpectedRevision, params.Revision = expected, revision
	params.ScopeRevision = pgtype.Int8{Int64: revision, Valid: true}
	params.LatestMeaningfulChange = mustJSON(document.LatestMeaningfulChange)
	params.EvidenceSummary = mustJSON(document.EvidenceSummary)
	params.SourceRevisions = mustJSON([]SourceRevision{{Type: "legacy_workflow_event", ID: uuidString(eventID), Revision: 1}})
	params.Document = mustJSON(document)
	return params, nil
}
