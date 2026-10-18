package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/internal/workflowprojection"
	"github.com/multica-ai/multica/server/internal/workflowstate"
	"github.com/multica-ai/multica/server/internal/workflowstore"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type issueWorkflowPlanRequest struct {
	SquadID            string `json:"squad_id"`
	ImplementerID      string `json:"implementer_id"`
	ReviewerID         string `json:"reviewer_id"`
	ReleaseAgentID     string `json:"release_agent_id,omitempty"`
	VerifierID         string `json:"verifier_id,omitempty"`
	RequiresOwner      bool   `json:"requires_owner"`
	RequiresDeployment bool   `json:"requires_deployment"`
}

type squadWorkflowTemplateRequest struct {
	ImplementerID      string `json:"implementer_id"`
	ReviewerID         string `json:"reviewer_id"`
	ReleaseAgentID     string `json:"release_agent_id,omitempty"`
	VerifierID         string `json:"verifier_id,omitempty"`
	RequiresOwner      bool   `json:"requires_owner"`
	RequiresDeployment bool   `json:"requires_deployment"`
	MaxCorrections     int32  `json:"max_corrections"`
}

type issueWorkflowEventRequest struct {
	Candidate         string                             `json:"candidate,omitempty"`
	StageEvidence     *service.OwnerRuntimeStageEvidence `json:"stage_evidence,omitempty"`
	Event             workflowstate.Event                `json:"event"`
	Reason            string                             `json:"reason"`
	Correction        *workflowCorrectionPacket          `json:"correction,omitempty"`
	OwnerDecision     *workflowOwnerDecisionPacket       `json:"owner_decision,omitempty"`
	ArtifactEvidence  *workflowArtifactEvidencePacketV1  `json:"artifact_evidence,omitempty"`
	TestEvidence      *workflowTestEvidencePacketV1      `json:"test_evidence,omitempty"`
	BlastRadiusReview *workflowBlastRadiusReviewPacketV1 `json:"blast_radius_review,omitempty"`
}

type workflowArtifactEvidencePacketV1 struct {
	CandidateSHA256        string                        `json:"candidate_sha256,omitempty"`
	SchemaVersion          int32                         `json:"schema_version"`
	Contract               string                        `json:"contract"`
	Artifacts              []workflowArtifactDigestV1    `json:"artifacts"`
	Inventory              workflowInventoryEvidenceV1   `json:"inventory"`
	Roots                  []workflowInventoryRootV1     `json:"roots"`
	Worktrees              []workflowInventoryWorktreeV1 `json:"worktrees"`
	InventoryRecordsSHA256 string                        `json:"inventory_records_sha256"`
}

type workflowArtifactDigestV1 struct {
	Path           string `json:"path"`
	DeclaredSHA256 string `json:"declared_sha256"`
	ObservedSHA256 string `json:"observed_sha256"`
}

type workflowInventoryEvidenceV1 struct {
	RootCount               int64 `json:"root_count"`
	DirtyRootCount          int64 `json:"dirty_root_count"`
	CleanRootCount          int64 `json:"clean_root_count"`
	WorktreeCount           int64 `json:"worktree_count"`
	PerWorktreeRecordCount  int64 `json:"per_worktree_record_count"`
	RequiredPathCheckCount  int64 `json:"required_path_check_count"`
	CompletedPathCheckCount int64 `json:"completed_path_check_count"`
	PresentPathCount        int64 `json:"present_path_count"`
	AbsentPathCount         int64 `json:"absent_path_count"`
}

type workflowInventoryRootV1 struct {
	Path               string `json:"path"`
	Branch             string `json:"branch"`
	HeadSHA            string `json:"head_sha"`
	TrackedChangeCount int64  `json:"tracked_change_count"`
	UntrackedFileCount int64  `json:"untracked_file_count"`
	Dirty              bool   `json:"dirty"`
}

type workflowInventoryWorktreeV1 struct {
	RootPath    string `json:"root_path"`
	Path        string `json:"path"`
	Branch      string `json:"branch"`
	HeadSHA     string `json:"head_sha"`
	PathChecked bool   `json:"path_checked"`
	PathPresent bool   `json:"path_present"`
}

var workflowSHA256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var workflowGitObjectPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

const maxWorkflowInventoryWorktrees = 4096

// workflowTestEvidencePacketV1 is the reviewer's proof that a build and test
// run actually happened for the candidate under review, not just an opinion
// that the diff looked right. It closes the gap where review_passed advanced
// an issue to done on nothing but the reviewer's "reason" string: no build,
// no test suite, no exit code, ever checked anywhere server-side.
//
// This is still self-reported evidence — the server has no safe way to
// execute an agent-controlled command itself (see stageHandoffNote's review
// instructions and ADR-equivalent discussion in the handoff commit). What it
// closes is the gap between "the reviewer said it passed" and "the reviewer
// is on record, in a schema the server actually checks, naming the exact
// commands it ran, their exit codes, and a digest of their real output" — and
// the server refuses the transition outright if that record is missing,
// incomplete, names the wrong commands, or admits a non-zero exit code.
type workflowTestEvidencePacketV1 struct {
	SchemaVersion int32                  `json:"schema_version"`
	Contract      string                 `json:"contract"`
	Runs          []workflowCommandRunV1 `json:"runs"`
}

type workflowCommandRunV1 struct {
	Command      string `json:"command"`
	ExitCode     int32  `json:"exit_code"`
	ObservedAt   string `json:"observed_at"`
	OutputSHA256 string `json:"output_sha256"`
	OutputTail   string `json:"output_tail"`
}

const workflowTestEvidenceContract = "build_test_run.v1"
const maxWorkflowCommandRuns = 8
const maxWorkflowOutputTailBytes = 8192

// workflowRequiredEvidenceCommands is an allow-list, not free text: a reviewer
// cannot satisfy the gate by reporting a passing exit code for some command
// that never actually compiled or exercised the code under review. Multiple
// spellings are accepted per gate because this worktree's git backing store
// can be unavailable, which forces `-buildvcs=false`.
var workflowRequiredEvidenceCommands = []struct {
	label   string
	allowed map[string]struct{}
}{
	{label: "build", allowed: map[string]struct{}{
		"go build ./...":                 {},
		"go build -buildvcs=false ./...": {},
	}},
	{label: "test", allowed: map[string]struct{}{
		"go test ./...":                 {},
		"go test -buildvcs=false ./...": {},
	}},
}

// validateWorkflowTestEvidenceRequest requires review_passed to carry a
// structurally genuine build_test_run.v1 packet: one run per required gate,
// using a recognized command, every one of them exiting zero, each with a
// real output digest and a bounded, non-empty capture of what it printed.
// Anything else — missing, wrong contract, wrong command, a declared failure,
// a malformed digest — is refused with a specific reason rather than silently
// accepted the way a bare "reason" string is today.
func validateWorkflowTestEvidenceRequest(req issueWorkflowEventRequest) error {
	if req.Event != workflowstate.ReviewPassed {
		if req.TestEvidence != nil {
			return errors.New("test_evidence is only valid for review_passed")
		}
		return nil
	}
	packet := req.TestEvidence
	if packet == nil {
		return errors.New("review_passed requires test_evidence proving a real build and test run actually happened and passed")
	}
	if packet.SchemaVersion != 1 || packet.Contract != workflowTestEvidenceContract {
		return errors.New("test_evidence must use build_test_run.v1 schema version 1")
	}
	if len(packet.Runs) < len(workflowRequiredEvidenceCommands) || len(packet.Runs) > maxWorkflowCommandRuns {
		return fmt.Errorf("test_evidence must contain between %d and %d command runs", len(workflowRequiredEvidenceCommands), maxWorkflowCommandRuns)
	}
	satisfied := make([]bool, len(workflowRequiredEvidenceCommands))
	now := time.Now()
	for i := range packet.Runs {
		run := &packet.Runs[i]
		run.Command = strings.TrimSpace(run.Command)
		run.OutputSHA256 = strings.TrimSpace(run.OutputSHA256)
		if run.Command == "" {
			return errors.New("test_evidence command run requires a command")
		}
		if !workflowSHA256Pattern.MatchString(run.OutputSHA256) {
			return errors.New("test_evidence command run requires a lowercase SHA-256 output digest")
		}
		if len(run.OutputTail) == 0 || len(run.OutputTail) > maxWorkflowOutputTailBytes {
			return fmt.Errorf("test_evidence command run requires captured output text up to %d bytes", maxWorkflowOutputTailBytes)
		}
		observed, err := time.Parse(time.RFC3339Nano, run.ObservedAt)
		if err != nil {
			return errors.New("test_evidence command run requires an RFC3339Nano observed_at timestamp")
		}
		if observed.After(now.Add(time.Minute)) {
			return errors.New("test_evidence command run cannot be observed in the future")
		}
		matched := false
		for idx, spec := range workflowRequiredEvidenceCommands {
			if _, ok := spec.allowed[run.Command]; !ok {
				continue
			}
			matched = true
			if run.ExitCode != 0 {
				return fmt.Errorf("test_evidence %s run did not exit zero (exit code %d): review_passed requires a genuinely passing run", spec.label, run.ExitCode)
			}
			satisfied[idx] = true
		}
		if !matched {
			return errors.New("test_evidence command run does not match a recognized build or test command")
		}
	}
	for idx, spec := range workflowRequiredEvidenceCommands {
		if !satisfied[idx] {
			return fmt.Errorf("test_evidence must include a passing %s run using one of the recognized commands", spec.label)
		}
	}
	return nil
}

// workflowBlastRadiusReviewPacketV1 is the reviewer's recorded answer to the
// question the test-evidence gate above does not ask: not "does this build
// and pass its own tests" but "does this design decision actually make sense
// against the rest of the system, and could it break, duplicate, or conflict
// with anything outside this one ticket's narrow scope."
//
// This closes a real production gap: an implementer once represented "one
// client, two subscription-tier metrics (free/paid)" by duplicating the
// CLIENT record per metric instead of keeping one client with two metrics —
// a data-modeling mistake that would build clean and pass whatever unit
// tests the ticket asked for, and that a review scoped to "does this satisfy
// the acceptance criteria" has no way to catch. A senior engineer catches it
// by asking "does this match how clients are already represented everywhere
// else" — which requires actually looking at the rest of the codebase, not
// just reading the diff.
//
// Like workflowTestEvidencePacketV1, this is still self-reported: the server
// cannot mechanically verify that a reviewer's reasoning is correct, only
// that a reviewer went through the motion of recording real, specific,
// non-trivial reasoning naming real places it looked. That is a materially
// higher bar than an unstructured "reason" string — it cannot be satisfied
// by silence, and it cannot be satisfied by boilerplate — but it is not, and
// cannot be, a guarantee that the judgment itself was correct.
type workflowBlastRadiusReviewPacketV1 struct {
	SchemaVersion       int32    `json:"schema_version"`
	Contract            string   `json:"contract"`
	RelatedAreasChecked []string `json:"related_areas_checked"`
	Reasoning           string   `json:"reasoning"`
}

const workflowBlastRadiusReviewContract = "blast_radius_review.v1"
const minWorkflowBlastRadiusReasoningBytes = 120
const maxWorkflowBlastRadiusReasoningBytes = 8192
const maxWorkflowBlastRadiusRelatedAreas = 32

// workflowBlastRadiusTrivialReasoning is a small set of known rubber-stamp
// phrases, checked before the length gate so a reviewer typing exactly one
// of them gets an honest "that's a rubber stamp" rejection rather than a
// generic "too short" one. It is deliberately not a quality classifier.
var workflowBlastRadiusTrivialReasoning = map[string]struct{}{
	"looks fine":            {},
	"lgtm":                  {},
	"looks good":            {},
	"looks good to me":      {},
	"no issues found":       {},
	"no issues":             {},
	"approved":              {},
	"everything looks good": {},
	"nothing to flag":       {},
	"seems fine":            {},
}

const minWorkflowBlastRadiusUniqueWords = 12

// validateWorkflowBlastRadiusReviewRequest requires review_passed to carry a
// structurally genuine blast_radius_review.v1 packet alongside test_evidence:
// at least one real area of the codebase the reviewer names as having
// actually checked, and reasoning text that is neither a known rubber-stamp
// phrase, nor under the minimum length, nor just one phrase repeated to pad
// out the length. It is additive to validateWorkflowTestEvidenceRequest, not
// a replacement — a review_passed missing either packet is refused.
//
// None of these checks can verify the reasoning is actually CORRECT — only a
// human or a more careful review of that reasoning can do that. What this
// closes is the gap where review_passed carried no design/blast-radius
// reasoning at all, the same way test_evidence closed the gap where it
// carried no build/test proof at all.
func validateWorkflowBlastRadiusReviewRequest(req issueWorkflowEventRequest) error {
	if req.Event != workflowstate.ReviewPassed {
		if req.BlastRadiusReview != nil {
			return errors.New("blast_radius_review is only valid for review_passed")
		}
		return nil
	}
	packet := req.BlastRadiusReview
	if packet == nil {
		return errors.New("review_passed requires blast_radius_review recording what design/blast-radius impact the reviewer actually checked, not just build/test evidence")
	}
	if packet.SchemaVersion != 1 || packet.Contract != workflowBlastRadiusReviewContract {
		return errors.New("blast_radius_review must use blast_radius_review.v1 schema version 1")
	}
	if len(packet.RelatedAreasChecked) == 0 || len(packet.RelatedAreasChecked) > maxWorkflowBlastRadiusRelatedAreas {
		return fmt.Errorf("blast_radius_review must name between 1 and %d real related areas actually checked", maxWorkflowBlastRadiusRelatedAreas)
	}
	for i := range packet.RelatedAreasChecked {
		packet.RelatedAreasChecked[i] = strings.TrimSpace(packet.RelatedAreasChecked[i])
		if len(packet.RelatedAreasChecked[i]) < 4 {
			return errors.New("blast_radius_review related_areas_checked entries must name a real file or area, not a placeholder")
		}
	}
	reasoning := strings.TrimSpace(packet.Reasoning)
	packet.Reasoning = reasoning
	if _, trivial := workflowBlastRadiusTrivialReasoning[strings.ToLower(reasoning)]; trivial {
		return errors.New("blast_radius_review reasoning cannot be a rubber-stamp phrase (\"looks fine\", \"LGTM\", etc.)")
	}
	if len(reasoning) < minWorkflowBlastRadiusReasoningBytes {
		return fmt.Errorf("blast_radius_review reasoning must be at least %d characters of real design/blast-radius reasoning, not a placeholder", minWorkflowBlastRadiusReasoningBytes)
	}
	if len(reasoning) > maxWorkflowBlastRadiusReasoningBytes {
		return fmt.Errorf("blast_radius_review reasoning exceeds %d characters", maxWorkflowBlastRadiusReasoningBytes)
	}
	if uniqueWords := workflowUniqueWordCount(reasoning); uniqueWords < minWorkflowBlastRadiusUniqueWords {
		return fmt.Errorf("blast_radius_review reasoning must contain at least %d distinct words of real reasoning, not one phrase padded or repeated to length", minWorkflowBlastRadiusUniqueWords)
	}
	return nil
}

// workflowUniqueWordCount is a cheap, honest heuristic, not a quality
// classifier: it counts distinct lowercase word tokens so that "looks fine
// looks fine looks fine ..." padded past the length minimum is still caught,
// without pretending to judge whether the reasoning is actually good.
func workflowUniqueWordCount(s string) int {
	seen := make(map[string]struct{})
	for _, word := range strings.Fields(strings.ToLower(s)) {
		word = strings.Trim(word, ".,;:!?()\"'`")
		if word == "" {
			continue
		}
		seen[word] = struct{}{}
	}
	return len(seen)
}

type workflowOwnerDecisionPacket struct {
	Question       string   `json:"question"`
	Recommendation string   `json:"recommendation"`
	Consequence    string   `json:"consequence"`
	Options        []string `json:"options,omitempty"`
}

type workflowOwnerDecisionRequest struct {
	RequestEventID string `json:"request_event_id"`
	Answer         string `json:"answer"`
}

type workflowCorrectionPacket struct {
	FailedCriteria     []string `json:"failed_criteria"`
	Evidence           string   `json:"evidence"`
	RequiredCorrection string   `json:"required_correction"`
	Candidate          string   `json:"candidate,omitempty"`
}

type issueWorkflowEventResponse struct {
	EventID  string                 `json:"event_id"`
	Replay   bool                   `json:"replay"`
	Plan     db.IssueWorkflowPlan   `json:"plan"`
	Decision workflowstate.Decision `json:"decision"`
	Dispatch string                 `json:"dispatch,omitempty"`
}

func (h *Handler) PutSquadWorkflowTemplate(w http.ResponseWriter, r *http.Request) {
	squad, workspaceID, ok := h.loadSquadInWorkspace(w, r)
	if !ok {
		return
	}
	member, ok := h.requireWorkspaceMember(w, r, workspaceID, "workspace not found")
	if !ok {
		return
	}
	if !canManageSquad(member, squad) {
		writeError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	var req squadWorkflowTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	implementerID, ok := parseUUIDOrBadRequest(w, req.ImplementerID, "implementer_id")
	if !ok {
		return
	}
	reviewerID, ok := parseUUIDOrBadRequest(w, req.ReviewerID, "reviewer_id")
	if !ok {
		return
	}
	if implementerID == reviewerID {
		writeError(w, http.StatusBadRequest, "implementer and reviewer must be different agents")
		return
	}
	releaseID, ok := optionalWorkflowUUID(w, req.ReleaseAgentID, "release_agent_id")
	if !ok {
		return
	}
	verifierID, ok := optionalWorkflowUUID(w, req.VerifierID, "verifier_id")
	if !ok {
		return
	}
	if req.RequiresDeployment && (!releaseID.Valid || !verifierID.Valid) {
		writeError(w, http.StatusBadRequest, "deployment workflows require release_agent_id and verifier_id")
		return
	}
	if releaseID.Valid && verifierID.Valid && releaseID == verifierID {
		writeError(w, http.StatusBadRequest, "release agent and production verifier must be different agents")
		return
	}
	if req.MaxCorrections == 0 {
		req.MaxCorrections = 1
	}
	if req.MaxCorrections < 1 || req.MaxCorrections > 3 {
		writeError(w, http.StatusBadRequest, "max_corrections must be between 1 and 3")
		return
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)
	if _, err := qtx.LockWorkflowOwnerWorkspace(r.Context(), squad.WorkspaceID); err != nil {
		writeError(w, http.StatusConflict, "workspace is no longer available")
		return
	}
	if _, err := qtx.LockSquadWorkflowTemplateSquad(r.Context(), db.LockSquadWorkflowTemplateSquadParams{
		SquadID: squad.ID, WorkspaceID: squad.WorkspaceID,
	}); err != nil {
		writeError(w, http.StatusConflict, "squad is no longer available")
		return
	}

	requestedAgents := make([]pgtype.UUID, 0, 4)
	seenAgents := make(map[pgtype.UUID]struct{}, 4)
	for _, id := range []pgtype.UUID{implementerID, reviewerID, releaseID, verifierID} {
		if id.Valid {
			if _, seen := seenAgents[id]; !seen {
				seenAgents[id] = struct{}{}
				requestedAgents = append(requestedAgents, id)
			}
		}
	}
	lockedAgents, err := qtx.LockSquadWorkflowTemplateAgents(r.Context(), db.LockSquadWorkflowTemplateAgentsParams{
		WorkspaceID: squad.WorkspaceID, SquadID: squad.ID, AgentIds: requestedAgents,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to lock squad members")
		return
	}
	if len(lockedAgents) != len(requestedAgents) {
		writeError(w, http.StatusBadRequest, "every workflow role must be an active agent in the squad")
		return
	}

	template, err := qtx.UpsertSquadWorkflowTemplate(r.Context(), db.UpsertSquadWorkflowTemplateParams{
		SquadID: squad.ID, WorkspaceID: squad.WorkspaceID,
		ImplementerID: implementerID, ReviewerID: reviewerID,
		ReleaseAgentID: releaseID, VerifierID: verifierID,
		RequiresOwner: req.RequiresOwner, RequiresDeployment: req.RequiresDeployment,
		MaxCorrections: req.MaxCorrections,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save squad workflow template")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit squad workflow template")
		return
	}
	writeJSON(w, http.StatusOK, template)
}

func (h *Handler) GetSquadWorkflowTemplate(w http.ResponseWriter, r *http.Request) {
	squad, _, ok := h.loadSquadInWorkspace(w, r)
	if !ok {
		return
	}
	template, err := h.Queries.GetSquadWorkflowTemplate(r.Context(), db.GetSquadWorkflowTemplateParams{
		SquadID: squad.ID, WorkspaceID: squad.WorkspaceID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "squad workflow template not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read squad workflow template")
		return
	}
	writeJSON(w, http.StatusOK, template)
}

func buildIssueWorkflowBriefing(plan db.IssueWorkflowPlan, agentID, issueIdentifier string, packets ...*workflowCorrectionPacket) string {
	if len(plan.OwnerRuntimeRoute) > 0 {
		return ownerRuntimeWorkflowBriefing(plan, agentID, issueIdentifier)
	}
	role := ""
	events := ""
	switch agentID {
	case uuidToString(plan.ImplementerID):
		role = "implementer"
		events = "implementation_ready after the implementation is complete and evidenced. If one real owner decision is required, use owner_decision_required with --question, --recommendation, --consequence, and optional repeatable --option"
	case uuidToString(plan.ReviewerID):
		role = "independent reviewer"
		events = "review_passed when the evidence passes. For review_failed, also supply --failed-criterion \"<criterion>\" --evidence \"<observed failure>\" --required-correction \"<bounded correction>\" and optionally --candidate \"<revision>\""
	case uuidToString(plan.ReleaseAgentID):
		role = "release operator"
		events = "deployment_succeeded after the deployment receipt is verified"
	case uuidToString(plan.VerifierID):
		role = "independent production verifier"
		events = "production_verified when the live business flow passes, or production_failed when it does not"
	default:
		return ""
	}
	if issueIdentifier == "" {
		return ""
	}
	briefing := "## Deterministic Workflow Handoff\n\n" +
		"You are the " + role + " for this squad-owned issue. Do the assigned work and record exactly one structured result before your final issue comment. " +
		"Run `multica issue workflow-event " + issueIdentifier + " <event> --reason \"<short evidence>\"`. Use " + events + ". " +
		"The controller routes the next role. Do not mention Mika or another agent merely to hand work forward. Your final comment should report evidence, not perform routing."
	if role == "implementer" && len(packets) > 0 && packets[0] != nil {
		packet := packets[0]
		briefing += "\n\n## Bounded Correction Packet\n\nFailed criteria:\n- " + strings.Join(packet.FailedCriteria, "\n- ") +
			"\nEvidence: " + packet.Evidence +
			"\nRequired correction: " + packet.RequiredCorrection
		if packet.Candidate != "" {
			briefing += "\nCandidate: " + packet.Candidate
		}
		briefing += "\nCorrect only this failure and preserve unrelated work."
	}
	return briefing
}

func (h *Handler) PutIssueWorkflowPlan(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}

	var req issueWorkflowPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	squadID, ok := parseUUIDOrBadRequest(w, req.SquadID, "squad_id")
	if !ok {
		return
	}
	implementerID, ok := parseUUIDOrBadRequest(w, req.ImplementerID, "implementer_id")
	if !ok {
		return
	}
	reviewerID, ok := parseUUIDOrBadRequest(w, req.ReviewerID, "reviewer_id")
	if !ok {
		return
	}
	if implementerID == reviewerID {
		writeError(w, http.StatusBadRequest, "implementer and reviewer must be different agents")
		return
	}
	releaseID, ok := optionalWorkflowUUID(w, req.ReleaseAgentID, "release_agent_id")
	if !ok {
		return
	}
	verifierID, ok := optionalWorkflowUUID(w, req.VerifierID, "verifier_id")
	if !ok {
		return
	}
	if req.RequiresDeployment && (!releaseID.Valid || !verifierID.Valid) {
		writeError(w, http.StatusBadRequest, "deployment workflows require release_agent_id and verifier_id")
		return
	}
	if releaseID.Valid && verifierID.Valid && releaseID == verifierID {
		writeError(w, http.StatusBadRequest, "release agent and production verifier must be different agents")
		return
	}

	squad, err := h.Queries.GetSquadInWorkspace(r.Context(), db.GetSquadInWorkspaceParams{ID: squadID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		writeError(w, http.StatusBadRequest, "squad not found in issue workspace")
		return
	}
	if !issue.AssigneeType.Valid || issue.AssigneeType.String != "squad" || issue.AssigneeID != squad.ID {
		writeError(w, http.StatusConflict, "issue must be assigned to the workflow squad")
		return
	}
	members, err := h.Queries.ListSquadMembers(r.Context(), squad.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read squad members")
		return
	}
	memberAgents := make(map[string]bool, len(members))
	for _, member := range members {
		if member.MemberType == "agent" {
			memberAgents[uuidToString(member.MemberID)] = true
		}
	}
	for label, id := range map[string]pgtype.UUID{
		"implementer":   implementerID,
		"reviewer":      reviewerID,
		"release agent": releaseID,
		"verifier":      verifierID,
	} {
		if id.Valid && !memberAgents[uuidToString(id)] {
			writeError(w, http.StatusBadRequest, label+" must be an agent in the assigned squad")
			return
		}
	}

	actorType, actorID := h.resolveActor(r, requestUserID(r), uuidToString(issue.WorkspaceID))
	if actorType != "member" && !(actorType == "agent" && h.isCurrentWorkflowLeaderTask(r, issue, squad, actorID)) {
		writeError(w, http.StatusForbidden, "only a member or the current squad leader task can set the workflow plan")
		return
	}

	plan, err := h.Queries.UpsertIssueWorkflowPlan(r.Context(), db.UpsertIssueWorkflowPlanParams{
		IssueID: issue.ID, WorkspaceID: issue.WorkspaceID, SquadID: squad.ID,
		ImplementerID: implementerID, ReviewerID: reviewerID,
		ReleaseAgentID: releaseID, VerifierID: verifierID,
		RequiresOwner: req.RequiresOwner, RequiresDeployment: req.RequiresDeployment,
		CurrentState: string(workflowstate.Working),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save workflow plan")
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (h *Handler) GetIssueWorkflowPlan(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	plan, err := h.Queries.GetIssueWorkflowPlan(r.Context(), db.GetIssueWorkflowPlanParams{IssueID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "workflow plan not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read workflow plan")
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (h *Handler) PostIssueWorkflowEvent(w http.ResponseWriter, r *http.Request) {
	if h.rejectChatTaskOperationalMutation(w, r) {
		return
	}
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	var req issueWorkflowEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" {
		writeError(w, http.StatusBadRequest, "reason is required")
		return
	}
	if err := normalizeWorkflowCorrectionRequest(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := normalizeWorkflowOwnerDecisionRequest(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ownerRoute := false
	if plan, planErr := h.Queries.GetIssueWorkflowPlan(r.Context(), db.GetIssueWorkflowPlanParams{IssueID: issue.ID, WorkspaceID: issue.WorkspaceID}); planErr == nil {
		ownerRoute = len(plan.OwnerRuntimeRoute) > 0
	}
	if err := validateOwnerOrLegacyArtifactRequest(req, ownerRoute); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	taskID, ok := parseUUIDOrBadRequest(w, r.Header.Get("X-Task-ID"), "task id")
	if !ok {
		return
	}
	sourceTask, err := h.Queries.GetAgentTaskInWorkspace(r.Context(), db.GetAgentTaskInWorkspaceParams{ID: taskID, WorkspaceID: issue.WorkspaceID})
	if err != nil || !sourceTask.IssueID.Valid || sourceTask.IssueID != issue.ID {
		writeError(w, http.StatusBadRequest, "task does not belong to issue")
		return
	}
	if sourceTask.Status != "running" {
		writeError(w, http.StatusConflict, "workflow events require the currently running task")
		return
	}
	// A task bound to a normalized v2 action must finish only through its typed
	// terminal result. Letting it also write the legacy workflow plan would
	// create a second, independently routed handoff beside the authoritative v2
	// command path.
	if sourceTask.WorkflowActionID.Valid {
		writeError(w, http.StatusConflict, "normalized workflow tasks must return a typed terminal result")
		return
	}
	actorType, actorID := h.resolveActor(r, requestUserID(r), uuidToString(issue.WorkspaceID))
	if actorType != "agent" || actorID != uuidToString(sourceTask.AgentID) {
		writeError(w, http.StatusForbidden, "workflow events must come from the running task agent")
		return
	}

	if req.Event == workflowstate.OwnerDecisionRequired {
		if _, planErr := h.Queries.GetIssueWorkflowPlan(r.Context(), db.GetIssueWorkflowPlanParams{IssueID: issue.ID, WorkspaceID: issue.WorkspaceID}); errors.Is(planErr, pgx.ErrNoRows) {
			h.postOrdinaryTaskQuestion(w, r, issue, sourceTask, req)
			return
		}
	}
	response, targetID, err := h.applyIssueWorkflowEvent(r, issue, sourceTask, req)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	// A QA rejection is a declared system failure, not permission to blindly
	// repeat the same implementation loop. Preserve the workflow correction
	// state, but let the investigation establish causes and safer next steps
	// before another implementation task is admitted.
	if targetID.Valid && req.Event != workflowstate.ReviewFailed {
		_, enqueueErr := h.TaskService.EnqueueTaskForWorkflowTransition(r.Context(), issue, targetID, response.Plan.SquadID, sourceTask, parseUUID(response.EventID))
		if enqueueErr != nil && !errors.Is(enqueueErr, service.ErrDuplicatePendingTask) {
			slog.Error("accepted workflow successor dispatch failed; completion recovery required",
				"issue_id", uuidToString(issue.ID), "source_task_id", uuidToString(sourceTask.ID),
				"event_id", response.EventID, "target_agent_id", uuidToString(targetID), "error", enqueueErr)
			response.Dispatch = "pending_retry"
			writeJSON(w, http.StatusAccepted, response)
			return
		}
		response.Dispatch = "queued"
	}
	if req.Event == workflowstate.ReviewFailed && h.TaskService != nil {
		summary := req.Reason
		if req.Correction != nil {
			if correction, marshalErr := json.Marshal(req.Correction); marshalErr == nil {
				summary += "\n\nStructured correction evidence: " + string(correction)
			}
		}
		if _, launchErr := h.TaskService.LaunchAIFailureInvestigation(r.Context(), issue, "qa_rejection", parseUUID(response.EventID), sourceTask.ID, sourceTask.AgentID, sourceTask.AccountableUserID, summary); launchErr != nil {
			slog.Warn("launch QA rejection investigation failed", "issue_id", uuidToString(issue.ID), "event_id", response.EventID, "error", launchErr)
			response.Dispatch = "investigation_recording_failed"
		} else {
			response.Dispatch = "investigation_queued"
		}
	}
	writeJSON(w, http.StatusCreated, response)
}

func (h *Handler) applyIssueWorkflowEvent(r *http.Request, issue db.Issue, sourceTask db.AgentTaskQueue, req issueWorkflowEventRequest) (issueWorkflowEventResponse, pgtype.UUID, error) {
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		return issueWorkflowEventResponse{}, pgtype.UUID{}, err
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)
	if _, err := qtx.LockIssueTaskAdmission(r.Context(), db.LockIssueTaskAdmissionParams{ID: issue.ID, WorkspaceID: issue.WorkspaceID}); err != nil {
		return issueWorkflowEventResponse{}, pgtype.UUID{}, err
	}
	// An ordinary task can have the same agent as a workflow role, but it has
	// neither that workflow's execution authority nor its completion recovery.
	// Refuse the mixed route before committing an event or changing the stage.
	boundPlan, err := qtx.GetIssueWorkflowPlanForUpdate(r.Context(), db.GetIssueWorkflowPlanForUpdateParams{IssueID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		return issueWorkflowEventResponse{}, pgtype.UUID{}, errors.New("workflow plan not found")
	}
	if len(boundPlan.OwnerRuntimeRoute) == 0 && (!sourceTask.SquadID.Valid || sourceTask.SquadID != boundPlan.SquadID) {
		return issueWorkflowEventResponse{}, pgtype.UUID{}, errors.New("ordinary tasks cannot advance a legacy squad workflow; use ordinary delegation instead")
	}
	var successPicture db.IssueCurrentPicture
	var successOwnerItem *db.InboxItem
	storedSuccessProjection := false

	if existing, err := qtx.GetIssueWorkflowEventBySource(r.Context(), db.GetIssueWorkflowEventBySourceParams{SourceTaskID: sourceTask.ID, EventType: string(req.Event)}); err == nil {
		var decision workflowstate.Decision
		if json.Unmarshal(existing.Decision, &decision) != nil {
			return issueWorkflowEventResponse{}, pgtype.UUID{}, errors.New("stored workflow decision is invalid")
		}
		plan, planErr := qtx.GetIssueWorkflowPlanForUpdate(r.Context(), db.GetIssueWorkflowPlanForUpdateParams{IssueID: issue.ID, WorkspaceID: issue.WorkspaceID})
		if planErr != nil {
			return issueWorkflowEventResponse{}, pgtype.UUID{}, planErr
		}
		if len(plan.OwnerRuntimeRoute) > 0 {
			var supplied any = map[string]any{}
			switch {
			case req.ArtifactEvidence != nil:
				supplied = req.ArtifactEvidence
			case req.Correction != nil:
				supplied = req.Correction
			case req.OwnerDecision != nil:
				supplied = req.OwnerDecision
			case req.StageEvidence != nil:
				supplied = req.StageEvidence
			}
			raw, _ := json.Marshal(supplied)
			var incoming, stored any
			if json.Unmarshal(raw, &incoming) != nil || json.Unmarshal(existing.Payload, &stored) != nil || !reflect.DeepEqual(incoming, stored) || req.Reason != existing.Reason || (req.StageEvidence != nil && req.Candidate != req.StageEvidence.CandidateSHA256) {
				return issueWorkflowEventResponse{}, pgtype.UUID{}, errors.New("event already accepted with different evidence")
			}
		}
		if err := tx.Commit(r.Context()); err != nil {
			return issueWorkflowEventResponse{}, pgtype.UUID{}, err
		}
		return issueWorkflowEventResponse{EventID: uuidToString(existing.ID), Replay: true, Plan: plan, Decision: decision}, optionalUUIDFromString(decision.EnqueueAgentID), nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return issueWorkflowEventResponse{}, pgtype.UUID{}, err
	}

	plan, err := qtx.GetIssueWorkflowPlanForUpdate(r.Context(), db.GetIssueWorkflowPlanForUpdateParams{IssueID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		return issueWorkflowEventResponse{}, pgtype.UUID{}, errors.New("workflow plan not found")
	}
	squad, err := qtx.GetSquadInWorkspace(r.Context(), db.GetSquadInWorkspaceParams{ID: plan.SquadID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		return issueWorkflowEventResponse{}, pgtype.UUID{}, errors.New("workflow squad not found")
	}
	input := workflowInput(plan, sourceTask, req.Event, squad.LeaderID)
	if len(plan.OwnerRuntimeRoute) > 0 {
		currentTask, taskErr := qtx.LockOwnerRuntimeSourceTask(r.Context(), db.LockOwnerRuntimeSourceTaskParams{TaskID: sourceTask.ID, IssueID: issue.ID})
		if taskErr != nil {
			return issueWorkflowEventResponse{}, pgtype.UUID{}, errors.New("reporting task is no longer running")
		}
		sourceTask = currentTask
		if _, err := h.TaskService.ValidateOwnerRuntimeTask(r.Context(), qtx, sourceTask, false); err != nil {
			return issueWorkflowEventResponse{}, pgtype.UUID{}, err
		}
		if err := service.ValidateOwnerRuntimeStageEvidence(r.Context(), qtx, plan, sourceTask, string(req.Event), req.Candidate, req.StageEvidence); err != nil {
			return issueWorkflowEventResponse{}, pgtype.UUID{}, err
		}
		input.LeaderID = ""
	}
	decision := workflowstate.Decide(input)
	if decision.Ignored {
		return issueWorkflowEventResponse{}, pgtype.UUID{}, errors.New("workflow event refused: " + decision.Reason)
	}
	if len(plan.OwnerRuntimeRoute) == 0 && decision.EnqueueAgentID != "" && req.Event != workflowstate.ReviewFailed {
		target, targetErr := qtx.GetAgent(r.Context(), parseUUID(decision.EnqueueAgentID))
		if targetErr != nil || target.WorkspaceID != issue.WorkspaceID {
			return issueWorkflowEventResponse{}, pgtype.UUID{}, errors.New("workflow successor is unavailable in this workspace")
		}
		if err := service.ValidateLegacyWorkflowTargetConfiguration(target); err != nil {
			return issueWorkflowEventResponse{}, pgtype.UUID{}, err
		}
	}

	eventID := dbid.NewV7()
	suspendedState := plan.SuspendedState
	suspendedAgentID := plan.SuspendedAgentID
	if decision.Next == workflowstate.NeedsOwner || decision.Next == workflowstate.Blocked {
		if req.Event == workflowstate.ReviewPassed && decision.Next == workflowstate.NeedsOwner {
			if plan.RequiresDeployment {
				suspendedState = pgtype.Text{String: string(workflowstate.Deploying), Valid: true}
				suspendedAgentID = plan.ReleaseAgentID
			} else {
				suspendedState = pgtype.Text{String: string(workflowstate.Done), Valid: true}
				suspendedAgentID = pgtype.UUID{}
			}
		} else {
			suspendedState = pgtype.Text{String: plan.CurrentState, Valid: true}
			suspendedAgentID = sourceTask.AgentID
		}
	} else if workflowstate.State(plan.CurrentState) == workflowstate.NeedsOwner || workflowstate.State(plan.CurrentState) == workflowstate.Blocked {
		suspendedState = pgtype.Text{}
		suspendedAgentID = pgtype.UUID{}
	}
	correctionCount := plan.CorrectionCount
	if req.Event == workflowstate.ReviewFailed && decision.Next == workflowstate.Correction {
		correctionCount++
	}
	updated, err := qtx.UpdateIssueWorkflowPlanState(r.Context(), db.UpdateIssueWorkflowPlanStateParams{
		CurrentState: string(decision.Next), SuspendedState: suspendedState, SuspendedAgentID: suspendedAgentID,
		CorrectionCount: correctionCount,
		IssueID:         issue.ID, WorkspaceID: issue.WorkspaceID, ExpectedRevision: plan.Revision,
	})
	if err != nil {
		return issueWorkflowEventResponse{}, pgtype.UUID{}, errors.New("workflow changed concurrently; retry event")
	}
	projectedStatus := workflowBoardStatus(r.Context(), qtx, issue.WorkspaceID, decision.Next)
	if projectedStatus != "" && issue.Status != projectedStatus {
		if _, err := qtx.UpdateIssueStatus(r.Context(), db.UpdateIssueStatusParams{
			ID: issue.ID, Status: projectedStatus, WorkspaceID: issue.WorkspaceID,
		}); err != nil {
			return issueWorkflowEventResponse{}, pgtype.UUID{}, errors.New("failed to project workflow state to issue status")
		}
	}
	decisionJSON, _ := json.Marshal(decision)
	payloadJSON := []byte("{}")
	if req.ArtifactEvidence != nil {
		payloadJSON, _ = json.Marshal(req.ArtifactEvidence)
	} else if req.TestEvidence != nil || req.BlastRadiusReview != nil {
		// review_passed carries two required, independently validated packets —
		// technical build/test proof and the reviewer's recorded design/blast-
		// radius reasoning (see workflowTestEvidencePacketV1 and
		// workflowBlastRadiusReviewPacketV1). Both are stored together here so
		// the reasoning is on the same durable, queryable event record the
		// build/test evidence already is — not buried in a side channel — and
		// so the owner can pull it later to spot-check reviewer quality.
		payloadJSON, _ = json.Marshal(map[string]any{
			"test_evidence":       req.TestEvidence,
			"blast_radius_review": req.BlastRadiusReview,
		})
	} else if req.Correction != nil {
		payloadJSON, _ = json.Marshal(req.Correction)
	} else if req.OwnerDecision != nil {
		payloadJSON, _ = json.Marshal(req.OwnerDecision)
	} else if req.StageEvidence != nil {
		payloadJSON, _ = json.Marshal(req.StageEvidence)
	}
	event, err := qtx.CreateIssueWorkflowEvent(r.Context(), db.CreateIssueWorkflowEventParams{
		ID: eventID, WorkspaceID: issue.WorkspaceID, IssueID: issue.ID,
		SourceTaskID: sourceTask.ID, ActorID: sourceTask.AgentID,
		EventType: string(req.Event), Reason: req.Reason, Payload: payloadJSON, Decision: decisionJSON,
	})
	if err != nil {
		return issueWorkflowEventResponse{}, pgtype.UUID{}, err
	}
	if req.Event == workflowstate.ReviewPassed && decision.Next == workflowstate.Done && !plan.RequiresOwner && !plan.RequiresDeployment {
		if h.TaskService == nil || h.TaskService.AdmitLegacyWorkflowProjection == nil || h.TaskService.OnLegacyWorkflowProjection == nil {
			return issueWorkflowEventResponse{}, pgtype.UUID{}, errors.New("legacy success projection admission is unavailable")
		}
		params, paramsErr := workflowprojection.LegacyNonLiveSuccessParams(
			issue.WorkspaceID, issue.ID, event.ID, sourceTask.ID, sourceTask.AgentID, req.Reason, event.CreatedAt.Time,
		)
		if paramsErr != nil {
			return issueWorkflowEventResponse{}, pgtype.UUID{}, paramsErr
		}
		successPicture, err = workflowstore.UpsertLegacyCurrentPicture(r.Context(), tx, params)
		if errors.Is(err, pgx.ErrNoRows) {
			return issueWorkflowEventResponse{}, pgtype.UUID{}, errors.New("legacy success cannot replace existing Current Picture truth")
		}
		if err != nil {
			return issueWorkflowEventResponse{}, pgtype.UUID{}, err
		}
		successOwnerItem, err = h.TaskService.AdmitLegacyWorkflowProjection(
			r.Context(), qtx, issue.WorkspaceID, issue.ID, successPicture.Revision,
		)
		if err != nil {
			return issueWorkflowEventResponse{}, pgtype.UUID{}, err
		}
		storedSuccessProjection = true
	}
	if len(plan.OwnerRuntimeRoute) > 0 {
		successPicture, successOwnerItem, err = h.TaskService.StoreOwnerRuntimeProjection(r.Context(), tx, updated, sourceTask, decision.Reason)
		if err != nil {
			return issueWorkflowEventResponse{}, pgtype.UUID{}, err
		}
		storedSuccessProjection = true
	}
	var ownerAlert *db.InboxItem
	if decision.NotifyOwner {
		if !sourceTask.AccountableUserID.Valid {
			return issueWorkflowEventResponse{}, pgtype.UUID{}, errors.New("owner decision cannot be requested without an accountable member")
		}
		packet := req.OwnerDecision
		if packet == nil {
			packet = &workflowOwnerDecisionPacket{
				Question: "Approve the reviewed result?", Recommendation: req.Reason,
				Consequence: "Approval continues the configured workflow; rejection leaves the issue waiting for your direction.",
				Options:     []string{"Approve", "Reject"},
			}
		}
		details, _ := json.Marshal(map[string]string{"workflow_event_id": uuidToString(event.ID), "question": packet.Question, "recommendation": packet.Recommendation, "consequence": packet.Consequence, "options": strings.Join(packet.Options, "\n")})
		item, createErr := qtx.CreateInboxItem(r.Context(), db.CreateInboxItemParams{
			ID: dbid.NewV7(), WorkspaceID: issue.WorkspaceID, RecipientType: "member", RecipientID: sourceTask.AccountableUserID,
			Type: "workflow_owner_decision", Severity: "action_required", IssueID: issue.ID,
			Title: issue.Title, Body: pgtype.Text{String: packet.Question, Valid: true},
			ActorType: pgtype.Text{String: "agent", Valid: true}, ActorID: sourceTask.AgentID, Details: details,
		})
		if createErr != nil {
			return issueWorkflowEventResponse{}, pgtype.UUID{}, errors.New("failed to create owner decision alert")
		}
		ownerAlert = &item
	}
	if err := tx.Commit(r.Context()); err != nil {
		return issueWorkflowEventResponse{}, pgtype.UUID{}, err
	}
	if ownerAlert != nil {
		h.publish(protocol.EventInboxNew, uuidToString(issue.WorkspaceID), "agent", uuidToString(sourceTask.AgentID), map[string]any{"item": inboxToResponse(*ownerAlert)})
	}
	if storedSuccessProjection {
		h.TaskService.OnLegacyWorkflowProjection(r.Context(), issue.WorkspaceID, issue.ID, successPicture.Revision, successOwnerItem)
	}
	return issueWorkflowEventResponse{EventID: uuidToString(event.ID), Plan: updated, Decision: decision}, optionalUUIDFromString(decision.EnqueueAgentID), nil
}

func validateWorkflowArtifactEvidenceRequest(req issueWorkflowEventRequest) error {
	if req.Event != workflowstate.ImplementationReady {
		if req.ArtifactEvidence != nil {
			return errors.New("artifact_evidence is only valid for implementation_ready")
		}
		return nil
	}
	packet := req.ArtifactEvidence
	if packet == nil {
		return errors.New("implementation_ready requires artifact_evidence")
	}
	if packet.SchemaVersion != 1 || packet.Contract != "repository_worktree_inventory.v1" {
		return errors.New("artifact_evidence must use repository_worktree_inventory.v1 schema version 1")
	}
	if len(packet.Artifacts) == 0 || len(packet.Artifacts) > 16 {
		return errors.New("artifact_evidence must contain between 1 and 16 artifacts")
	}
	paths := make(map[string]struct{}, len(packet.Artifacts))
	for i := range packet.Artifacts {
		artifact := &packet.Artifacts[i]
		artifact.Path = strings.TrimSpace(artifact.Path)
		artifact.DeclaredSHA256 = strings.TrimSpace(artifact.DeclaredSHA256)
		artifact.ObservedSHA256 = strings.TrimSpace(artifact.ObservedSHA256)
		if artifact.Path == "" {
			return errors.New("artifact_evidence artifact path is required")
		}
		if _, duplicate := paths[artifact.Path]; duplicate {
			return errors.New("artifact_evidence artifact paths must be unique")
		}
		paths[artifact.Path] = struct{}{}
		if !workflowSHA256Pattern.MatchString(artifact.DeclaredSHA256) || artifact.DeclaredSHA256 != artifact.ObservedSHA256 {
			return errors.New("artifact_evidence artifact hashes must be matching lowercase SHA-256 values")
		}
	}
	inventory := packet.Inventory
	if len(packet.Roots) == 0 || int64(len(packet.Roots)) != inventory.RootCount {
		return errors.New("artifact_evidence requires one explicit record for every repository root")
	}
	rootPaths := make(map[string]struct{}, len(packet.Roots))
	worktreesByRoot := make(map[string]int, len(packet.Roots))
	var dirtyRoots int64
	for i := range packet.Roots {
		root := &packet.Roots[i]
		root.Path = strings.TrimSpace(root.Path)
		root.Branch = strings.TrimSpace(root.Branch)
		root.HeadSHA = strings.TrimSpace(root.HeadSHA)
		if root.Path == "" || root.Branch == "" || !workflowGitObjectPattern.MatchString(root.HeadSHA) || root.TrackedChangeCount < 0 || root.UntrackedFileCount < 0 {
			return errors.New("artifact_evidence contains an invalid repository root record")
		}
		if _, duplicate := rootPaths[root.Path]; duplicate {
			return errors.New("artifact_evidence repository root paths must be unique")
		}
		rootPaths[root.Path] = struct{}{}
		derivedDirty := root.TrackedChangeCount+root.UntrackedFileCount > 0
		if root.Dirty != derivedDirty {
			return errors.New("artifact_evidence repository dirty state must match recorded changes")
		}
		if root.Dirty {
			dirtyRoots++
		}
	}
	cleanRoots := int64(len(packet.Roots)) - dirtyRoots
	if inventory.RootCount <= 0 || inventory.DirtyRootCount != dirtyRoots || inventory.CleanRootCount != cleanRoots || inventory.DirtyRootCount+inventory.CleanRootCount != inventory.RootCount {
		return errors.New("artifact_evidence root inventory is incomplete or inconsistent")
	}
	if len(packet.Worktrees) == 0 || len(packet.Worktrees) > maxWorkflowInventoryWorktrees ||
		int64(len(packet.Worktrees)) != inventory.WorktreeCount || inventory.PerWorktreeRecordCount != inventory.WorktreeCount {
		return errors.New("artifact_evidence requires one record for every worktree")
	}
	// The same Git common directory may be inspected through more than one
	// explicitly authorized root (for example the main checkout and one linked
	// worktree). Each root's porcelain output is independent evidence, so only
	// a duplicate (root_path, path) record is invalid. Global path uniqueness
	// incorrectly rejected complete evidence for that legitimate topology.
	worktreePaths := make(map[string]struct{}, len(packet.Worktrees))
	var presentPaths, completedChecks int64
	for i := range packet.Worktrees {
		worktree := &packet.Worktrees[i]
		worktree.RootPath = strings.TrimSpace(worktree.RootPath)
		worktree.Path = strings.TrimSpace(worktree.Path)
		worktree.Branch = strings.TrimSpace(worktree.Branch)
		worktree.HeadSHA = strings.TrimSpace(worktree.HeadSHA)
		if _, knownRoot := rootPaths[worktree.RootPath]; !knownRoot || worktree.Path == "" || worktree.Branch == "" || !workflowGitObjectPattern.MatchString(worktree.HeadSHA) {
			return errors.New("artifact_evidence contains an invalid or orphaned worktree record")
		}
		worktreeKey := worktree.RootPath + "\x00" + worktree.Path
		if _, duplicate := worktreePaths[worktreeKey]; duplicate {
			return errors.New("artifact_evidence worktree paths must be unique")
		}
		worktreePaths[worktreeKey] = struct{}{}
		worktreesByRoot[worktree.RootPath]++
		if !worktree.PathChecked {
			return errors.New("artifact_evidence requires an explicit path check for every worktree")
		}
		completedChecks++
		if worktree.PathPresent {
			presentPaths++
		}
	}
	for rootPath := range rootPaths {
		if worktreesByRoot[rootPath] == 0 {
			return errors.New("artifact_evidence requires at least one worktree record for every root")
		}
	}
	absentPaths := int64(len(packet.Worktrees)) - presentPaths
	if inventory.RequiredPathCheckCount != inventory.WorktreeCount ||
		inventory.CompletedPathCheckCount != completedChecks ||
		inventory.PresentPathCount != presentPaths || inventory.AbsentPathCount != absentPaths ||
		inventory.PresentPathCount+inventory.AbsentPathCount != inventory.CompletedPathCheckCount {
		return errors.New("artifact_evidence requires a complete per-worktree path check")
	}
	packet.InventoryRecordsSHA256 = strings.TrimSpace(packet.InventoryRecordsSHA256)
	if !workflowSHA256Pattern.MatchString(packet.InventoryRecordsSHA256) {
		return errors.New("artifact_evidence inventory_records_sha256 must be a lowercase SHA-256 value")
	}
	recordsSHA256, err := workflowInventoryRecordsSHA256(packet.Roots, packet.Worktrees)
	if err != nil || recordsSHA256 != packet.InventoryRecordsSHA256 {
		return errors.New("artifact_evidence inventory record digest does not match the admitted records")
	}
	return nil
}

func workflowInventoryRecordsSHA256(roots []workflowInventoryRootV1, worktrees []workflowInventoryWorktreeV1) (string, error) {
	records, err := json.Marshal(struct {
		Roots     []workflowInventoryRootV1     `json:"roots"`
		Worktrees []workflowInventoryWorktreeV1 `json:"worktrees"`
	}{Roots: roots, Worktrees: worktrees})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(records)
	return hex.EncodeToString(sum[:]), nil
}

// PostIssueWorkflowOwnerDecision records a member's answer to the one current
// structured question and resumes the exact suspended stage without waking Mika.
func (h *Handler) PostIssueWorkflowOwnerDecision(w http.ResponseWriter, r *http.Request) {
	if isMachineCredentialActor(r) && !hasChatOwnerDelegation(r) {
		writeError(w, http.StatusForbidden, "owner decisions require a human actor")
		return
	}
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	var req workflowOwnerDecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Answer = strings.TrimSpace(req.Answer)
	requestEventID, ok := parseUUIDOrBadRequest(w, req.RequestEventID, "request_event_id")
	if !ok {
		return
	}
	if req.Answer == "" || len(req.Answer) > 2000 {
		writeError(w, http.StatusBadRequest, "answer must be 1 to 2000 characters")
		return
	}
	memberID := parseUUID(requestUserID(r))

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start owner decision")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)
	if _, err := qtx.LockIssueTaskAdmission(r.Context(), db.LockIssueTaskAdmissionParams{ID: issue.ID, WorkspaceID: issue.WorkspaceID}); err != nil {
		writeError(w, 409, "issue admission unavailable")
		return
	}
	plan, err := qtx.GetIssueWorkflowPlanForUpdate(r.Context(), db.GetIssueWorkflowPlanForUpdateParams{IssueID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if err != nil || workflowstate.State(plan.CurrentState) != workflowstate.NeedsOwner {
		writeError(w, http.StatusConflict, "issue is not waiting for an owner decision")
		return
	}
	requestEvent, err := qtx.GetIssueWorkflowEvent(r.Context(), db.GetIssueWorkflowEventParams{ID: requestEventID, IssueID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		writeError(w, http.StatusConflict, "owner decision request is not current for this issue")
		return
	}
	if requestEvent.EventType != string(workflowstate.OwnerDecisionRequired) && requestEvent.EventType != string(workflowstate.ReviewPassed) {
		writeError(w, http.StatusConflict, "event did not request an owner decision")
		return
	}
	sourceTask, err := qtx.GetAgentTaskInWorkspace(r.Context(), db.GetAgentTaskInWorkspaceParams{ID: requestEvent.SourceTaskID, WorkspaceID: issue.WorkspaceID})
	if err != nil || !sourceTask.AccountableUserID.Valid || sourceTask.AccountableUserID != memberID {
		writeError(w, http.StatusForbidden, "decision belongs to the accountable issue owner")
		return
	}
	if requestEvent.EventType == string(workflowstate.OwnerDecisionRequired) && (sourceTask.AgentID != plan.SuspendedAgentID) {
		writeError(w, http.StatusConflict, "owner decision request has been superseded")
		return
	}
	if len(plan.OwnerRuntimeRoute) > 0 {
		entries, err := qtx.ListIssueWorkflowEvents(r.Context(), db.ListIssueWorkflowEventsParams{IssueID: issue.ID, WorkspaceID: issue.WorkspaceID})
		if err != nil || len(entries) == 0 || entries[len(entries)-1].ID != requestEventID {
			writeError(w, 409, "owner question has been superseded")
			return
		}
	}

	decision := workflowstate.Decide(workflowInput(plan, sourceTask, workflowstate.OwnerDecisionRecorded, pgtype.UUID{}))
	if decision.Ignored {
		writeError(w, http.StatusConflict, "owner decision cannot resume workflow: "+decision.Reason)
		return
	}
	updated, err := qtx.UpdateIssueWorkflowPlanState(r.Context(), db.UpdateIssueWorkflowPlanStateParams{
		CurrentState: string(decision.Next), SuspendedState: pgtype.Text{}, SuspendedAgentID: pgtype.UUID{}, CorrectionCount: plan.CorrectionCount,
		IssueID: issue.ID, WorkspaceID: issue.WorkspaceID, ExpectedRevision: plan.Revision,
	})
	if err != nil {
		writeError(w, http.StatusConflict, "workflow changed concurrently; retry decision")
		return
	}
	if status := workflowBoardStatus(r.Context(), qtx, issue.WorkspaceID, decision.Next); status != "" && status != issue.Status {
		if _, err := qtx.UpdateIssueStatus(r.Context(), db.UpdateIssueStatusParams{ID: issue.ID, Status: status, WorkspaceID: issue.WorkspaceID}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to resume issue status")
			return
		}
	}
	decisionJSON, _ := json.Marshal(decision)
	payload, _ := json.Marshal(map[string]any{"request_event_id": req.RequestEventID, "answer": req.Answer})
	recorded, err := qtx.CreateIssueWorkflowEvent(r.Context(), db.CreateIssueWorkflowEventParams{
		ID: dbid.NewV7(), WorkspaceID: issue.WorkspaceID, IssueID: issue.ID, SourceTaskID: sourceTask.ID,
		ActorID: memberID, EventType: string(workflowstate.OwnerDecisionRecorded), Reason: req.Answer, Payload: payload, Decision: decisionJSON,
	})
	if err != nil {
		writeError(w, http.StatusConflict, "owner decision was already recorded")
		return
	}
	var routedTask db.AgentTaskQueue
	var routedPicture db.IssueCurrentPicture
	var routedItem *db.InboxItem
	if len(plan.OwnerRuntimeRoute) > 0 {
		routedTask, err = h.TaskService.EnqueueOwnerRuntimeTask(r.Context(), tx, issue, updated, sourceTask, recorded.ID, pgtype.UUID{})
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		routedPicture, routedItem, err = h.TaskService.StoreOwnerRuntimeProjection(r.Context(), tx, updated, routedTask, "")
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		if _, err = tx.Exec(r.Context(), `UPDATE inbox_item SET read=true,archived=true WHERE issue_id=$1 AND recipient_id=$2 AND details->>'workflow_event_id'=$3`, issue.ID, memberID, req.RequestEventID); err != nil {
			writeError(w, 500, "question answer receipt unavailable")
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit owner decision")
		return
	}
	response := issueWorkflowEventResponse{EventID: uuidToString(recorded.ID), Plan: updated, Decision: decision}
	if routedTask.ID.Valid {
		h.TaskService.PublishOwnerIntakeTask(r.Context(), routedTask)
		if h.TaskService.OnLegacyWorkflowProjection != nil {
			h.TaskService.OnLegacyWorkflowProjection(r.Context(), issue.WorkspaceID, issue.ID, routedPicture.Revision, routedItem)
		}
		response.Dispatch = "queued"
		writeJSON(w, http.StatusCreated, response)
		return
	}
	if target := optionalUUIDFromString(decision.EnqueueAgentID); target.Valid {
		if _, err := h.TaskService.EnqueueTaskForWorkflowTransition(r.Context(), issue, target, plan.SquadID, sourceTask, recorded.ID); err != nil && !errors.Is(err, service.ErrDuplicatePendingTask) {
			response.Dispatch = "pending_retry"
			writeJSON(w, http.StatusAccepted, response)
			return
		}
		response.Dispatch = "queued"
	}
	writeJSON(w, http.StatusCreated, response)
}

func workflowBoardStatus(ctx context.Context, q *db.Queries, workspaceID pgtype.UUID, state workflowstate.State) string {
	fallback := ""
	preferred := ""
	switch state {
	case workflowstate.Received:
		fallback = "backlog"
	case workflowstate.Queued:
		fallback = "todo"
	case workflowstate.Working, workflowstate.Correction, workflowstate.RolledBack:
		fallback = "in_progress"
	case workflowstate.TechnicalReview:
		fallback = "in_review"
	case workflowstate.Deploying:
		fallback, preferred = "in_review", "deploying"
	case workflowstate.ProductionVerifying, workflowstate.RollingBack:
		fallback, preferred = "in_review", "verifying"
	case workflowstate.NeedsOwner:
		fallback, preferred = "in_review", "needs_you"
	case workflowstate.Blocked, workflowstate.Failed:
		fallback = "blocked"
	case workflowstate.Done:
		fallback = "done"
	case workflowstate.Live:
		fallback, preferred = "done", "live"
	}
	if preferred != "" {
		entry, err := q.GetIssueStatusEntryByKey(ctx, db.GetIssueStatusEntryByKeyParams{WorkspaceID: workspaceID, Key: preferred})
		if err == nil && !entry.ArchivedAt.Valid {
			return preferred
		}
	}
	return fallback
}

func workflowInput(plan db.IssueWorkflowPlan, task db.AgentTaskQueue, event workflowstate.Event, leaderID pgtype.UUID) workflowstate.Input {
	return workflowstate.Input{
		IssueID: uuidToString(plan.IssueID), Current: workflowstate.State(plan.CurrentState), Event: event,
		ActorID: uuidToString(task.AgentID), ImplementerID: uuidToString(plan.ImplementerID), ReviewerID: uuidToString(plan.ReviewerID),
		LeaderID: uuidToString(leaderID), CorrectionCount: int(plan.CorrectionCount), MaxCorrections: int(plan.MaxCorrections),
		ReleaseAgentID: uuidToString(plan.ReleaseAgentID), VerifierID: uuidToString(plan.VerifierID),
		RequiresOwner: plan.RequiresOwner, RequiresDeployment: plan.RequiresDeployment,
		ResumeState: workflowstate.State(plan.SuspendedState.String), ResumeAgentID: uuidToString(plan.SuspendedAgentID),
	}
}

func normalizeWorkflowCorrectionRequest(req *issueWorkflowEventRequest) error {
	if req.Event != workflowstate.ReviewFailed {
		if req.Correction != nil {
			return errors.New("correction packet is only valid for review_failed")
		}
		return nil
	}
	if req.Correction == nil {
		return errors.New("review_failed requires a structured correction packet")
	}
	packet := req.Correction
	if len(packet.FailedCriteria) == 0 || len(packet.FailedCriteria) > 5 {
		return errors.New("review_failed requires 1 to 5 failed criteria")
	}
	for i := range packet.FailedCriteria {
		packet.FailedCriteria[i] = strings.TrimSpace(packet.FailedCriteria[i])
		if packet.FailedCriteria[i] == "" || len(packet.FailedCriteria[i]) > 500 {
			return errors.New("each failed criterion must be 1 to 500 characters")
		}
	}
	packet.Evidence = strings.TrimSpace(packet.Evidence)
	packet.RequiredCorrection = strings.TrimSpace(packet.RequiredCorrection)
	packet.Candidate = strings.TrimSpace(packet.Candidate)
	if packet.Evidence == "" || len(packet.Evidence) > 1000 {
		return errors.New("correction evidence must be 1 to 1000 characters")
	}
	if packet.RequiredCorrection == "" || len(packet.RequiredCorrection) > 1000 {
		return errors.New("required correction must be 1 to 1000 characters")
	}
	if len(packet.Candidate) > 500 {
		return errors.New("candidate must be at most 500 characters")
	}
	return nil
}

func normalizeWorkflowOwnerDecisionRequest(req *issueWorkflowEventRequest) error {
	if req.Event != workflowstate.OwnerDecisionRequired {
		if req.OwnerDecision != nil {
			return errors.New("owner decision packet is only valid for owner_decision_required")
		}
		return nil
	}
	if req.OwnerDecision == nil {
		return errors.New("owner_decision_required requires a structured owner decision packet")
	}
	p := req.OwnerDecision
	p.Question, p.Recommendation, p.Consequence = strings.TrimSpace(p.Question), strings.TrimSpace(p.Recommendation), strings.TrimSpace(p.Consequence)
	if p.Question == "" || len(p.Question) > 500 {
		return errors.New("owner question must be 1 to 500 characters")
	}
	if p.Recommendation == "" || len(p.Recommendation) > 1000 {
		return errors.New("owner recommendation must be 1 to 1000 characters")
	}
	if p.Consequence == "" || len(p.Consequence) > 1000 {
		return errors.New("owner consequence must be 1 to 1000 characters")
	}
	if len(p.Options) > 5 {
		return errors.New("owner decision supports at most 5 options")
	}
	for i := range p.Options {
		p.Options[i] = strings.TrimSpace(p.Options[i])
		if p.Options[i] == "" || len(p.Options[i]) > 200 {
			return errors.New("each owner option must be 1 to 200 characters")
		}
	}
	return nil
}

func loadWorkflowCorrectionPacket(ctx context.Context, q *db.Queries, issue db.Issue, task *db.AgentTaskQueue) *workflowCorrectionPacket {
	if !task.TriggerEvidenceKind.Valid || task.TriggerEvidenceKind.String != "workflow_event" || !task.TriggerEvidenceRefID.Valid {
		return nil
	}
	event, err := q.GetIssueWorkflowEvent(ctx, db.GetIssueWorkflowEventParams{
		ID: task.TriggerEvidenceRefID, IssueID: issue.ID, WorkspaceID: issue.WorkspaceID,
	})
	if err != nil || event.EventType != string(workflowstate.ReviewFailed) {
		return nil
	}
	var packet workflowCorrectionPacket
	if json.Unmarshal(event.Payload, &packet) != nil || len(packet.FailedCriteria) == 0 {
		return nil
	}
	return &packet
}

func loadWorkflowOwnerAnswer(ctx context.Context, q *db.Queries, issue db.Issue, task *db.AgentTaskQueue) string {
	if !task.TriggerEvidenceKind.Valid || task.TriggerEvidenceKind.String != "workflow_event" || !task.TriggerEvidenceRefID.Valid {
		return ""
	}
	event, err := q.GetIssueWorkflowEvent(ctx, db.GetIssueWorkflowEventParams{ID: task.TriggerEvidenceRefID, IssueID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if err != nil || event.EventType != string(workflowstate.OwnerDecisionRecorded) {
		return ""
	}
	var payload struct {
		Answer string `json:"answer"`
	}
	if json.Unmarshal(event.Payload, &payload) != nil {
		return ""
	}
	return strings.TrimSpace(payload.Answer)
}

func optionalWorkflowUUID(w http.ResponseWriter, raw, label string) (pgtype.UUID, bool) {
	if strings.TrimSpace(raw) == "" {
		return pgtype.UUID{}, true
	}
	return parseUUIDOrBadRequest(w, raw, label)
}

func optionalUUIDFromString(raw string) pgtype.UUID {
	if raw == "" {
		return pgtype.UUID{}
	}
	return parseUUID(raw)
}

func (h *Handler) isCurrentWorkflowLeaderTask(r *http.Request, issue db.Issue, squad db.Squad, actorID string) bool {
	// The header is request input: an absent or malformed value means "not the
	// leader's task", never a panic.
	taskID, err := util.ParseUUID(r.Header.Get("X-Task-ID"))
	if err != nil || actorID != uuidToString(squad.LeaderID) {
		return false
	}
	task, err := h.Queries.GetAgentTaskInWorkspace(r.Context(), db.GetAgentTaskInWorkspaceParams{ID: taskID, WorkspaceID: issue.WorkspaceID})
	return err == nil && task.IssueID == issue.ID && task.AgentID == squad.LeaderID && task.IsLeaderTask && task.SquadID == squad.ID
}
