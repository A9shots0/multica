package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/workflowprojection"
	"github.com/multica-ai/multica/server/internal/workflowstore"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

type operatorCompletionRequest struct {
	ExpectedPlanRevision    int64  `json:"expected_plan_revision"`
	ExpectedPictureRevision int64  `json:"expected_picture_revision"`
	ReviewTaskID            string `json:"review_task_id"`
	EvidenceCommentID       string `json:"evidence_comment_id"`
	EvidenceCommentRevision int64  `json:"evidence_comment_revision"`
	CandidateSHA256         string `json:"candidate_sha256"`
	Reason                  string `json:"reason"`
}

type operatorCompletionRecord struct {
	Contract        string                    `json:"contract"`
	ActorType       string                    `json:"actor_type"`
	Request         operatorCompletionRequest `json:"request"`
	PreviousPicture *db.IssueCurrentPicture   `json:"previous_picture"`
	ReviewResult    json.RawMessage           `json:"review_result"`
	EvidenceComment string                    `json:"evidence_comment"`
}

type operatorCompletionResponse struct {
	CompletionID    string `json:"completion_id"`
	PictureRevision int64  `json:"picture_revision"`
	CompletionMode  string `json:"completion_mode"`
	Replay          bool   `json:"replay"`
}

// PostIssueOperatorCompletion is the explicit human acceptance of a published,
// independently reviewed legacy deliverable. It does not impersonate an agent
// result, grant execution authority, or settle normalized/release workflows.
func (h *Handler) PostIssueOperatorCompletion(w http.ResponseWriter, r *http.Request) {
	if isMachineCredentialActor(r) {
		writeError(w, http.StatusForbidden, "operator completion requires a human actor")
		return
	}
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	actorType, actorID := h.resolveActor(r, requestUserID(r), uuidToString(issue.WorkspaceID))
	if actorType != "member" || issue.CreatorType != "member" || actorID != uuidToString(issue.CreatorID) {
		writeError(w, http.StatusForbidden, "only the original requesting member can accept this outcome")
		return
	}
	var req operatorCompletionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&req) != nil || decoder.Decode(new(any)) != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid operator completion request")
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.ExpectedPlanRevision < 1 || req.ExpectedPictureRevision < 0 || req.ExpectedPictureRevision > 1 || req.EvidenceCommentRevision < 1 ||
		!workflowSHA256Pattern.MatchString(req.CandidateSHA256) || req.Reason == "" || len(req.Reason) > 4000 {
		writeError(w, http.StatusBadRequest, "current revisions, candidate SHA256 and an evidence-backed reason are required")
		return
	}
	reviewID, ok := parseUUIDOrBadRequest(w, req.ReviewTaskID, "review task id")
	if !ok {
		return
	}
	commentID, ok := parseUUIDOrBadRequest(w, req.EvidenceCommentID, "evidence comment id")
	if !ok {
		return
	}
	if h.TaskService == nil || h.TaskService.AdmitLegacyWorkflowProjection == nil || h.TaskService.OnLegacyWorkflowProjection == nil {
		writeError(w, http.StatusServiceUnavailable, "operator completion projection admission is unavailable")
		return
	}
	var response operatorCompletionResponse
	var stored db.IssueCurrentPicture
	var inbox *db.InboxItem
	err := workflowstore.InOperatorCompletionTransaction(r.Context(), h.TxStarter, h.Queries, issue.WorkspaceID, issue.ID, func(write *workflowstore.IssueWrite) error {
		s, err := write.ReadOperatorCompletion(r.Context(), issue.WorkspaceID, issue.ID, reviewID, commentID)
		if err != nil {
			if errors.Is(err, workflowstore.ErrOperatorCompletionBusy) {
				return err
			}
			return errors.New("completion evidence is unavailable or changed")
		}
		if s.Existing != nil {
			var prior operatorCompletionRecord
			if s.Existing.WorkspaceID != issue.WorkspaceID || s.Existing.IssueID != issue.ID || s.Existing.ActorID != issue.CreatorID ||
				json.Unmarshal(s.Existing.Payload, &prior) != nil || prior.Contract != "operator_assisted_completion.v1" || !reflect.DeepEqual(prior.Request, req) {
				return errors.New("review task was already accepted by a different completion request")
			}
			if s.Picture == nil || s.Picture.Revision != req.ExpectedPictureRevision+1 || s.Plan.CurrentState != "done" || s.Issue.Status != "done" || !operatorCompletionPictureMatches(*s.Picture, uuidToString(s.Existing.ID)) {
				return errors.New("workflow changed after the recorded operator completion")
			}
			response = operatorCompletionResponse{uuidToString(s.Existing.ID), s.Picture.Revision, "operator_assisted", true}
			return nil
		}
		if s.Issue.CreatorID != issue.CreatorID || s.Issue.CreatorType != "member" || s.Plan.Revision != req.ExpectedPlanRevision ||
			s.Plan.RequiresDeployment || s.Plan.RequiresOwner || s.Normalized || s.ActiveTasks || s.PendingOwnerDecision ||
			s.Plan.SuspendedState.Valid || s.Plan.SuspendedAgentID.Valid || !operatorCompletionStateEligible(s.Plan.CurrentState) {
			return errors.New("completion requires the unchanged idle non-deployment workflow with no pending owner decision")
		}
		actualRevision := int64(0)
		if s.Picture != nil {
			actualRevision = s.Picture.Revision
		}
		if actualRevision != req.ExpectedPictureRevision {
			return errors.New("Current Picture changed; read it before accepting completion")
		}
		if s.Review.IssueID != issue.ID || s.Review.AgentID != s.Plan.ReviewerID || s.Review.AgentID == s.Plan.ImplementerID ||
			!operatorReviewScopeMatches(s) || s.Review.Status != "completed" || !s.Review.CompletedAt.Valid || s.Review.WorkflowActionID.Valid ||
			s.Review.AccountableUserID != issue.CreatorID || !operatorReviewPassed(s.Review.Result) {
			return errors.New("completion requires a completed independent passing review in the current workflow")
		}
		if s.Comment.IssueID != issue.ID || s.Comment.Revision != req.EvidenceCommentRevision ||
			!strings.Contains(s.Comment.Content, req.CandidateSHA256) || s.Comment.CreatedAt.Time.Before(s.Review.CompletedAt.Time) {
			return errors.New("published evidence must bind the reviewed candidate after review and match the current comment revision")
		}
		eventID := dbid.NewV7()
		now := time.Now().UTC()
		params, err := workflowprojection.OperatorAssistedCompletionParams(s.Picture, issue.WorkspaceID, issue.ID, eventID, reviewID, s.Review.AgentID, issue.CreatorID, req.Reason, req.CandidateSHA256, now)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(operatorCompletionRecord{"operator_assisted_completion.v1", "member", req, s.Picture, s.Review.Result, s.Comment.Content})
		if err != nil {
			return err
		}
		stored, inbox, err = write.StoreOperatorCompletion(r.Context(), db.CreateIssueWorkflowEventParams{
			ID: eventID, WorkspaceID: issue.WorkspaceID, IssueID: issue.ID, SourceTaskID: reviewID, ActorID: issue.CreatorID,
			EventType: "operator_completed", Reason: req.Reason, Payload: payload, Decision: []byte(`{"next":"done","completion_mode":"operator_assisted"}`),
		}, db.UpdateIssueWorkflowPlanStateParams{WorkspaceID: issue.WorkspaceID, IssueID: issue.ID, ExpectedRevision: s.Plan.Revision, CurrentState: "done", CorrectionCount: s.Plan.CorrectionCount}, params, h.TaskService.AdmitLegacyWorkflowProjection)
		if err != nil {
			return err
		}
		response = operatorCompletionResponse{uuidToString(eventID), stored.Revision, "operator_assisted", false}
		return nil
	})
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if !response.Replay {
		h.TaskService.OnLegacyWorkflowProjection(r.Context(), issue.WorkspaceID, issue.ID, stored.Revision, inbox)
	}
	writeJSON(w, http.StatusOK, response)
}

func operatorCompletionStateEligible(state string) bool {
	return state == "working" || state == "technical_review" || state == "correction"
}

func operatorReviewScopeMatches(s workflowstore.OperatorCompletionSnapshot) bool {
	if s.Review.SquadID.Valid {
		return s.Review.SquadID == s.Plan.SquadID
	}
	// A native direct-owner mention has no task squad stamp. Keep that history
	// intact and require its original same-issue human comment as provenance.
	trigger := s.ReviewTrigger
	return trigger != nil && s.Review.OriginatorUserID == s.Issue.CreatorID &&
		s.Review.TriggerCommentID.Valid && s.Review.TriggerEvidenceKind.Valid && s.Review.TriggerEvidenceKind.String == "comment" &&
		s.Review.TriggerEvidenceRefID == s.Review.TriggerCommentID && trigger.ID == s.Review.TriggerCommentID &&
		trigger.WorkspaceID == s.Issue.WorkspaceID && trigger.IssueID == s.Issue.ID &&
		trigger.AuthorType == "member" && trigger.AuthorID == s.Issue.CreatorID &&
		s.Review.CompletedAt.Valid && !trigger.CreatedAt.Time.After(s.Review.CompletedAt.Time)
}

func operatorCompletionPictureMatches(picture db.IssueCurrentPicture, eventID string) bool {
	var sources []workflowprojection.SourceRevision
	var evidence struct {
		CompletionMode string `json:"completion_mode"`
	}
	return picture.CurrentMilestone == "complete" && picture.OutcomeState == "satisfied" && !picture.OutcomeOpen &&
		json.Unmarshal(picture.SourceRevisions, &sources) == nil && len(sources) == 1 && sources[0].Type == "legacy_workflow_event" && sources[0].ID == eventID && sources[0].Revision == 1 &&
		json.Unmarshal(picture.EvidenceSummary, &evidence) == nil && evidence.CompletionMode == "operator_assisted"
}

func operatorReviewPassed(raw []byte) bool {
	var result struct {
		Output string `json:"output"`
	}
	if json.Unmarshal(raw, &result) != nil {
		return false
	}
	text := strings.TrimSpace(result.Output)
	if strings.HasPrefix(text, "```json\n") && strings.HasSuffix(text, "```") {
		text = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(text, "```json\n"), "```"))
	}
	var review struct {
		Summary  string   `json:"summary"`
		Verdict  string   `json:"verdict"`
		Findings []string `json:"findings"`
	}
	decoder := json.NewDecoder(bytes.NewBufferString(text))
	decoder.DisallowUnknownFields()
	return decoder.Decode(&review) == nil && decoder.Decode(new(any)) == io.EOF && strings.TrimSpace(review.Summary) != "" && review.Verdict == "pass" && review.Findings != nil && len(review.Findings) == 0
}
