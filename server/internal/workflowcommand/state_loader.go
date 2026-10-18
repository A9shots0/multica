// Package workflowcommand reconstructs the reducer's authoritative state from
// normalized workflow ledgers. The display projection is deliberately absent
// from this package: issue_current_picture is a rebuildable read model only.
package workflowcommand

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	workflowstatev2 "github.com/multica-ai/multica/server/internal/workflowstate/v2"
	"github.com/multica-ai/multica/server/internal/workflowstore"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

var ErrInvalidAuthoritativeState = errors.New("invalid authoritative workflow state")

type Loader struct{ Now func() time.Time }

type normalizedRecord struct {
	typ      string
	id       string
	revision int64
	fields   map[string]any
}

type fieldKind byte

const (
	stringKind fieldKind = iota
	stringAllowEmptyKind
	integerKind
	objectKind
	arrayKind
	boolKind
)

var requiredShape = map[string]map[string]fieldKind{
	"outcome":           {"state": stringKind, "acceptance_version": integerKind},
	"scope":             {"state": stringKind, "acceptance_version": integerKind, "owner_evidence_id": stringKind, "created_by": stringKind, "statement": objectKind, "impact": objectKind},
	"authority":         {"state": stringKind, "scope_id": stringKind, "scope_revision": integerKind, "owner_evidence_id": stringKind, "created_by": stringKind, "envelope": objectKind},
	"execution":         {"branch_key": stringKind, "operation_type": stringKind, "state": stringKind, "scope_id": stringKind, "scope_revision": integerKind, "checkpoint": objectKind, "expected_event": objectKind, "wait_reason": objectKind, "attempt_count": integerKind},
	"decision":          {"scope_id": stringKind, "scope_revision": integerKind, "decision_type": stringKind, "state": stringKind, "resolution_state": stringKind, "question": stringKind, "options": arrayKind, "response_schema": objectKind, "authority_gap": objectKind, "requester_id": stringKind, "accountable_owner_id": stringKind, "suspended_operation_id": stringKind, "suspended_execution_state": stringKind},
	"decision_response": {"decision_id": stringKind, "decision_revision": integerKind, "response_type": stringKind, "actor_id": stringKind, "idempotency_key": stringKind, "request_hash": stringKind, "selected_options": arrayKind, "content": objectKind, "scope_change": objectKind},
	"candidate":         {"scope_id": stringKind, "scope_revision": integerKind, "candidate_type": stringKind, "state": stringKind, "artifact": objectKind, "build_contract": objectKind, "created_by": stringKind},
	"review":            {"candidate_id": stringKind, "candidate_revision": integerKind, "scope_id": stringKind, "scope_revision": integerKind, "reviewer_id": stringKind, "outcome": stringKind, "criteria": arrayKind, "coverage": objectKind, "evidence_refs": arrayKind, "gaps": arrayKind, "correction_packet": objectKind, "correction_count": integerKind, "max_corrections": integerKind},
	"release":           {"candidate_id": stringKind, "candidate_revision": integerKind, "scope_id": stringKind, "scope_revision": integerKind, "authority_id": stringKind, "authority_revision": integerKind, "target_key": stringAllowEmptyKind, "target_generation": integerKind, "state": stringKind, "facets": objectKind, "recovery_policy": objectKind, "exposed_population": objectKind},
	"evidence":          {"concern_type": stringKind, "evidence_type": stringKind, "level": stringKind, "source": stringKind, "producer_type": stringKind, "observed_at": stringKind, "coverage": objectKind, "limitations": arrayKind, "contradictions": arrayKind, "payload": objectKind},
	"incident":          {"incident_type": stringKind, "severity": stringKind, "state": stringKind, "consequence": objectKind, "recovery": objectKind},
	"command":           {"command_key": stringKind, "request_hash": stringKind, "command_type": stringKind, "actor_type": stringKind, "expected_revisions": objectKind, "payload": objectKind, "state": stringKind, "result": objectKind},
	"action_receipt":    {"action_key": stringKind, "request_hash": stringKind, "command_id": stringKind, "action_type": stringKind, "state": stringKind, "payload": objectKind, "attempt_count": integerKind, "max_attempts": integerKind},
	"attention":         {"attention_type": stringKind, "read_state": stringKind, "acknowledged": boolKind, "resolution_state": stringKind, "context": objectKind, "next_action": objectKind},
	"budget":            {"scope_id": stringKind, "concern_type": stringKind, "state": stringKind, "limits": objectKind, "consumption": objectKind},
	"target_head":       {"target_key": stringKind, "generation": integerKind, "release_id": stringKind, "issue_id": stringKind, "candidate_id": stringKind, "state": stringKind},
}

var optionalShape = map[string]map[string]fieldKind{
	"scope":     {"requires_deployment": boolKind, "source_plan_revision": integerKind},
	"authority": {"valid_until": stringKind, "revoked_at": stringKind, "superseded_at": stringKind},
	"execution": {"authority_id": stringKind, "authority_revision": integerKind, "candidate_id": stringKind, "target_key": stringKind, "actor_id": stringKind, "lease_token": stringKind, "lease_expires_at": stringKind, "next_action_at": stringKind, "superseded_at": stringKind},
	"decision":  {"authority_id": stringKind, "authority_revision": integerKind, "candidate_id": stringKind, "target_key": stringKind, "expires_at": stringKind, "superseded_at": stringKind},
	"candidate": {"superseded_at": stringKind}, "review": {"superseded_at": stringKind},
	"release": {
		"attempt_contract_version": integerKind, "acceptance_version": integerKind, "operation_id": stringKind,
		"branch_key": stringAllowEmptyKind, "expected_target_release_id": stringKind,
		"expected_target_generation": integerKind, "release_kind": stringKind,
		"recovery_of_release_id": stringKind, "head_observed_at": stringKind,
		"release_operator_id": stringKind, "planned_verifier_id": stringKind,
		"build_manifest_digest": stringKind, "artifact_digest": stringKind,
		"runtime_contract_digest": stringKind, "authority_capability": stringKind,
		"authority_evidence_id": stringKind, "recovery_policy_revision": integerKind,
		"authority_envelope_digest": stringKind, "owner_evidence_revision": integerKind,
		"exposure_policy_digest": stringKind, "rollback_target_release_id": stringKind,
		"irreversible_effects_digest": stringKind, "reviewed_review_id": stringKind,
		"reviewed_review_revision": integerKind,
		"superseded_at":            stringKind,
	},
	"incident": {
		"release_revision": integerKind, "candidate_id": stringKind,
		"candidate_revision": integerKind, "target_generation": integerKind,
		"detection_evidence_id": stringKind, "recovery_action_id": stringKind,
		"compatibility_state": stringKind, "regression_fingerprint": stringKind,
		"recovery_release_id": stringKind,
	},
	"evidence":  {"producer_id": stringKind, "candidate_id": stringKind, "target_key": stringKind, "valid_until": stringKind, "superseded_at": stringKind},
	"attention": {"superseded_at": stringKind},
}

var concernRecordType = map[workflowstatev2.Concern]string{
	workflowstatev2.ConcernOutcome: "outcome", workflowstatev2.ConcernScope: "scope",
	workflowstatev2.ConcernAuthority: "authority", workflowstatev2.ConcernExecution: "execution",
	workflowstatev2.ConcernReview: "review", workflowstatev2.ConcernRelease: "release",
	workflowstatev2.ConcernAttention: "attention", workflowstatev2.ConcernEvidence: "evidence",
	workflowstatev2.ConcernBudget: "budget",
}

func (l Loader) Load(ctx context.Context, write *workflowstore.IssueWrite, workspaceID, issueID pgtype.UUID) (workflowstatev2.CurrentPicture, error) {
	if write == nil || !workspaceID.Valid || !issueID.Valid {
		return workflowstatev2.CurrentPicture{}, invalid("loader requires an admitted write and valid tenant identifiers")
	}
	heads, err := write.ListConcernHeads(ctx, db.ListIssueWorkflowConcernHeadsParams{WorkspaceID: workspaceID, IssueID: issueID})
	if err != nil {
		return workflowstatev2.CurrentPicture{}, fmt.Errorf("list concern heads: %w", err)
	}
	rows, err := write.ListProjectionInputs(ctx, db.ListIssueWorkflowProjectionInputsParams{WorkspaceID: workspaceID, IssueID: issueID})
	if err != nil {
		return workflowstatev2.CurrentPicture{}, fmt.Errorf("list normalized ledgers: %w", err)
	}
	return l.loadRows(util.UUIDToString(workspaceID), util.UUIDToString(issueID), heads, rows)
}

func (l Loader) loadRows(workspaceID, issueID string, heads []db.IssueWorkflowConcernHead, rows []db.ListIssueWorkflowProjectionInputsRow) (workflowstatev2.CurrentPicture, error) {
	if len(heads) != len(concernRecordType) {
		return workflowstatev2.CurrentPicture{}, invalid("expected exactly nine concern heads, got %d", len(heads))
	}
	headByConcern := make(map[workflowstatev2.Concern]db.IssueWorkflowConcernHead, len(heads))
	for _, head := range heads {
		concern := workflowstatev2.Concern(head.ConcernType)
		if _, ok := concernRecordType[concern]; !ok || head.Revision < 1 || head.SourceRecordRevision < 1 || head.State == "" {
			return workflowstatev2.CurrentPicture{}, invalid("malformed concern head %q", head.ConcernType)
		}
		if util.UUIDToString(head.WorkspaceID) != workspaceID || util.UUIDToString(head.IssueID) != issueID {
			return workflowstatev2.CurrentPicture{}, invalid("cross-tenant concern head %q", head.ConcernType)
		}
		if _, duplicate := headByConcern[concern]; duplicate {
			return workflowstatev2.CurrentPicture{}, invalid("duplicate concern head %q", concern)
		}
		headByConcern[concern] = head
	}

	records := make(map[string]normalizedRecord, len(rows))
	byType := make(map[string][]normalizedRecord)
	for _, row := range rows {
		record, err := decodeRecord(workspaceID, issueID, row)
		if err != nil {
			return workflowstatev2.CurrentPicture{}, err
		}
		key := recordKey(record.typ, record.id, record.revision)
		if _, duplicate := records[key]; duplicate {
			return workflowstatev2.CurrentPicture{}, invalid("duplicate normalized record %s", key)
		}
		records[key] = record
		byType[record.typ] = append(byType[record.typ], record)
	}

	selected := make(map[workflowstatev2.Concern]normalizedRecord, len(headByConcern))
	for concern, typ := range concernRecordType {
		head, ok := headByConcern[concern]
		if !ok {
			return workflowstatev2.CurrentPicture{}, invalid("missing concern head %q", concern)
		}
		record, ok := records[recordKey(typ, util.UUIDToString(head.SourceRecordID), head.SourceRecordRevision)]
		if !ok {
			return workflowstatev2.CurrentPicture{}, invalid("concern head %q references a missing source", concern)
		}
		state, err := stringField(record.fields, stateField(typ))
		if err != nil || (state != head.State && concern != workflowstatev2.ConcernEvidence) {
			return workflowstatev2.CurrentPicture{}, invalid("concern head %q state disagrees with source", concern)
		}
		if concern == workflowstatev2.ConcernEvidence && state != head.State && head.State != string(workflowstatev2.EvidenceStale) {
			return workflowstatev2.CurrentPicture{}, invalid("concern head %q state disagrees with source", concern)
		}
		if nonNull(record.fields["superseded_at"]) {
			return workflowstatev2.CurrentPicture{}, invalid("concern head %q references superseded source", concern)
		}
		selected[concern] = record
	}

	p := workflowstatev2.CurrentPicture{IssueID: issueID, Decisions: map[string]workflowstatev2.OwnerDecision{}}
	p.Revisions = workflowstatev2.ConcernRevisions{
		Outcome: rev(headByConcern[workflowstatev2.ConcernOutcome].Revision), Scope: rev(headByConcern[workflowstatev2.ConcernScope].Revision),
		Authority: rev(headByConcern[workflowstatev2.ConcernAuthority].Revision), Execution: rev(headByConcern[workflowstatev2.ConcernExecution].Revision),
		Review: rev(headByConcern[workflowstatev2.ConcernReview].Revision), Release: rev(headByConcern[workflowstatev2.ConcernRelease].Revision),
		Attention: rev(headByConcern[workflowstatev2.ConcernAttention].Revision), Evidence: rev(headByConcern[workflowstatev2.ConcernEvidence].Revision),
		Budget: rev(headByConcern[workflowstatev2.ConcernBudget].Revision),
	}
	if err := l.populate(&p, selected, byType, headByConcern); err != nil {
		return workflowstatev2.CurrentPicture{}, err
	}
	return p, nil
}

func validateReleaseAttempt(release normalizedRecord, byType map[string][]normalizedRecord, p *workflowstatev2.CurrentPicture, generation uint64, state string) error {
	target := p.Release.TargetID
	if target == "" {
		if generation != 0 || nonNull(release.fields["expected_target_generation"]) || nonNull(release.fields["expected_target_release_id"]) || nonNull(release.fields["head_observed_at"]) || nonNull(release.fields["recovery_of_release_id"]) {
			return invalid("untargeted release carries target-attempt metadata")
		}
		return nil
	}
	if generation == 0 {
		return invalid("targeted release must propose a positive generation")
	}
	head := findTargetHead(byType["target_head"], target)
	kind := optionalString(release.fields, "release_kind")
	if kind == "" {
		kind = "forward"
	}
	// A recovery release in rolling_back is only a proposal. Until exact
	// provider readback advances the target head, the observed head remains its
	// failed predecessor.
	observed := oneOf(state, "deployed", "exposed", "production_verifying", "live", "degraded", "rolled_back", "restored") || state == "rolling_back" && kind != "recovery"
	hasObservedAt := nonNull(release.fields["head_observed_at"])
	if hasObservedAt {
		if _, ok, err := timeField(release.fields, "head_observed_at"); err != nil || !ok {
			return invalid("release has malformed head observation")
		}
	}
	if observed {
		// The exact target head is the durable observation authority. The optional
		// timestamp is additive audit metadata; legacy writers remain loadable only
		// when the full head identity agrees.
		if head == nil || mustString(head.fields, "issue_id") != p.IssueID || mustString(head.fields, "release_id") != release.id || mustString(head.fields, "candidate_id") != p.Release.CandidateID || mustUint(head.fields, "generation") != generation || mustString(head.fields, "state") != state {
			return invalid("observed release does not match authoritative target head")
		}
	} else if hasObservedAt {
		return invalid("release claims a target-head observation before deployment")
	}

	hasExpected := nonNull(release.fields["expected_target_generation"])
	if !hasExpected {
		if observed {
			return nil // populated pre-513 row, already proven by its backfilled head marker
		}
		return invalid("release attempt is missing its predecessor target generation")
	}
	expected := mustUint(release.fields, "expected_target_generation")
	if generation <= expected {
		return invalid("release generation must advance its predecessor")
	}
	expectedReleaseID := optionalString(release.fields, "expected_target_release_id")
	if (expected == 0) != (expectedReleaseID == "") {
		return invalid("release predecessor identity is incoherent")
	}
	if !observed {
		if expected == 0 {
			if head != nil {
				return invalid("release attempt expected an absent target head")
			}
		} else if head == nil || mustString(head.fields, "release_id") != expectedReleaseID || mustUint(head.fields, "generation") != expected {
			return invalid("release attempt predecessor no longer matches target head")
		}
	}
	recoveryOf := optionalString(release.fields, "recovery_of_release_id")
	switch kind {
	case "forward":
		if recoveryOf != "" {
			return invalid("forward release cannot carry recovery identity")
		}
	case "recovery":
		if recoveryOf == "" || expectedReleaseID == "" || recoveryOf != expectedReleaseID {
			return invalid("recovery release must bind the exact predecessor release")
		}
	default:
		return invalid("invalid release kind %q", kind)
	}
	return nil
}

func (l Loader) populate(p *workflowstatev2.CurrentPicture, selected map[workflowstatev2.Concern]normalizedRecord, byType map[string][]normalizedRecord, heads map[workflowstatev2.Concern]db.IssueWorkflowConcernHead) error {
	outcome := selected[workflowstatev2.ConcernOutcome]
	state, _ := stringField(outcome.fields, "state")
	p.Outcome = workflowstatev2.OutcomeState(state)
	if !oneOf(state, "open", "satisfied", "cancelled", "superseded") {
		return invalid("invalid outcome state %q", state)
	}

	scope := selected[workflowstatev2.ConcernScope]
	acceptance, err := uintField(scope.fields, "acceptance_version")
	if err != nil {
		return err
	}
	ownerEvidence, err := stringField(scope.fields, "owner_evidence_id")
	if err != nil {
		return err
	}
	policy := workflowstatev2.DeploymentUnknown
	planRevision := workflowstatev2.Revision(0)
	requiresDeployment := scope.fields["requires_deployment"]
	sourcePlanRevision := scope.fields["source_plan_revision"]
	if nonNull(requiresDeployment) != nonNull(sourcePlanRevision) {
		return invalid("scope release policy and source plan revision must be present together")
	}
	if nonNull(requiresDeployment) {
		planValue, err := uintField(scope.fields, "source_plan_revision")
		if err != nil || planValue == 0 {
			return invalid("scope source plan revision must be positive")
		}
		planRevision = rev(planValue)
		if boolValue(requiresDeployment) {
			policy = workflowstatev2.DeploymentRequired
		} else {
			policy = workflowstatev2.DeploymentNotRequired
		}
	}
	p.Scope = workflowstatev2.ScopeFacet{Revision: p.Revisions.Scope, AcceptanceVersion: rev(acceptance), OwnerEvidenceRef: ownerEvidence, DeploymentPolicy: policy, SourcePlanRevision: planRevision}
	if mustUint(outcome.fields, "acceptance_version") != acceptance {
		return invalid("outcome acceptance version does not match authoritative scope")
	}
	for _, candidateRecord := range byType["candidate"] {
		candidateScope := findRevision(byType["scope"], mustString(candidateRecord.fields, "scope_id"), int64(mustUint(candidateRecord.fields, "scope_revision")))
		if candidateScope == nil {
			return invalid("candidate %s references a missing scope revision", candidateRecord.id)
		}
	}

	authority := selected[workflowstatev2.ConcernAuthority]
	authorityState, _ := stringField(authority.fields, "state")
	if !oneOf(authorityState, "missing", "active", "paused", "revoked", "expired", "superseded") {
		return invalid("invalid authority state %q", authorityState)
	}
	if authorityState == "revoked" || authorityState == "expired" || nonNull(authority.fields["revoked_at"]) {
		return invalid("authoritative authority is revoked or expired")
	}
	if deadline, ok, err := timeField(authority.fields, "valid_until"); err != nil {
		return err
	} else if ok && !deadline.After(l.now()) {
		return invalid("authoritative authority expired at %s", deadline.UTC().Format(time.RFC3339Nano))
	}
	scopeRevision, err := uintField(authority.fields, "scope_revision")
	if err != nil {
		return err
	}
	envelope, err := objectField(authority.fields, "envelope")
	if err != nil {
		return err
	}
	p.Authority = workflowstatev2.AuthorityFacet{State: workflowstatev2.AuthorityState(authorityState), Revision: p.Revisions.Authority, ScopeRevision: rev(scopeRevision), TargetID: optionalString(envelope, "target_id"), CandidateID: optionalString(envelope, "candidate_id")}
	if mustString(authority.fields, "scope_id") != scope.id || scopeRevision != uint64(scope.revision) || p.Authority.ScopeRevision != p.Scope.Revision {
		return invalid("authority scope revision does not match authoritative scope")
	}

	execution := selected[workflowstatev2.ConcernExecution]
	executionState, _ := stringField(execution.fields, "state")
	if !oneOf(executionState, "received", "queued", "working", "waiting_automatic", "in_review", "correction", "blocked_external", "technically_stopped", "paused", "completed", "failed", "cancelled", "superseded") {
		return invalid("invalid execution state %q", executionState)
	}
	branch, err := stringField(execution.fields, "branch_key")
	if err != nil || branch == "" {
		return invalid("execution branch is missing")
	}
	checkpoint, _ := optionalObject(execution.fields, "checkpoint")
	expected, _ := optionalObject(execution.fields, "expected_event")
	wait, _ := optionalObject(execution.fields, "wait_reason")
	suspendedState, suspendedOperation := optionalString(checkpoint, "suspended_state"), optionalString(checkpoint, "suspended_operation_id")
	if (suspendedState == "") != (suspendedOperation == "") || (suspendedState != "" && !oneOf(suspendedState, "received", "queued", "working", "waiting_automatic", "in_review", "correction", "blocked_external", "technically_stopped", "paused", "completed", "failed", "cancelled", "superseded")) {
		return invalid("execution has malformed suspended checkpoint")
	}
	p.Execution = workflowstatev2.ExecutionFacet{State: workflowstatev2.ExecutionState(executionState), OperationID: execution.id, ActorID: optionalString(execution.fields, "actor_id"), RunnableTargetID: optionalString(execution.fields, "target_key"), LeaseID: optionalString(execution.fields, "lease_token"), CheckpointID: optionalString(checkpoint, "id"), ExpectedSignal: optionalString(expected, "type"), ResumeCondition: optionalString(wait, "resume_condition"), NextAutomaticAction: optionalString(expected, "next_action"), SuspendedState: workflowstatev2.ExecutionState(suspendedState), SuspendedOperationID: suspendedOperation}
	if mustString(execution.fields, "scope_id") != scope.id || mustUint(execution.fields, "scope_revision") != uint64(scope.revision) {
		return invalid("execution scope binding does not match authoritative scope")
	}
	if authorityID := optionalString(execution.fields, "authority_id"); authorityID != "" && (authorityID != authority.id || optionalUint(execution.fields, "authority_revision") != uint64(authority.revision)) {
		return invalid("execution authority binding does not match authoritative authority")
	}

	review := selected[workflowstatev2.ConcernReview]
	reviewState, _ := stringField(review.fields, "outcome")
	if !oneOf(reviewState, "not_required", "pending", "in_progress", "passed", "failed", "inconclusive", "blocked_by_evidence") {
		return invalid("invalid review state %q", reviewState)
	}
	candidateID, err := stringField(review.fields, "candidate_id")
	if err != nil {
		return err
	}
	candidate := findLatest(byType["candidate"], candidateID)
	if candidate == nil {
		return invalid("review references missing candidate")
	}
	implementer, err := stringField(candidate.fields, "created_by")
	if err != nil {
		return err
	}
	reviewer, err := stringField(review.fields, "reviewer_id")
	if err != nil {
		return err
	}
	if reviewer == implementer {
		return invalid("reviewer must differ from implementer")
	}
	if mustUint(review.fields, "candidate_revision") != uint64(candidate.revision) || mustString(review.fields, "scope_id") != scope.id || mustUint(review.fields, "scope_revision") != uint64(scope.revision) {
		return invalid("review binding does not match authoritative candidate and scope")
	}
	criteria, err := strictStringSlice(review.fields["criteria"], "review criteria")
	if err != nil {
		return err
	}
	gaps, err := strictStringSlice(review.fields["gaps"], "review gaps")
	if err != nil {
		return err
	}
	correction, err := objectField(review.fields, "correction_packet")
	if err != nil {
		return err
	}
	correctionCount, maxCorrections := mustUint(review.fields, "correction_count"), mustUint(review.fields, "max_corrections")
	if correctionCount > math.MaxUint32 || maxCorrections > math.MaxUint32 || (maxCorrections != 0 && correctionCount > maxCorrections) {
		return invalid("review correction budget is invalid")
	}
	p.Review = workflowstatev2.ReviewFacet{State: workflowstatev2.ReviewState(reviewState), CandidateID: candidateID, AcceptanceVersion: p.Scope.AcceptanceVersion, ImplementerID: implementer, ReviewerID: reviewer, CriteriaRefs: criteria, CoverageRefs: objectKeys(review.fields["coverage"]), GapRefs: gaps, RequiredCorrection: optionalString(correction, "required_correction"), CorrectionCount: uint32(correctionCount), MaxCorrections: uint32(maxCorrections)}
	if err := loadTechnicalReviews(p, byType); err != nil {
		return err
	}

	release := selected[workflowstatev2.ConcernRelease]
	releaseState, _ := stringField(release.fields, "state")
	if !oneOf(releaseState, "not_required", "pending", "built", "reviewed", "authorized", "deploying", "deployed", "exposed", "production_verifying", "live", "degraded", "rolling_back", "rolled_back", "restored", "failed") {
		return invalid("invalid release state %q", releaseState)
	}
	generation, err := uintField(release.fields, "target_generation")
	if err != nil {
		return err
	}
	facets, _ := optionalObject(release.fields, "facets")
	recoveryPolicy, _ := optionalObject(release.fields, "recovery_policy")
	operatorID, verifierID := optionalString(release.fields, "release_operator_id"), optionalString(release.fields, "planned_verifier_id")
	legacyOperatorID, legacyVerifierID := optionalString(facets, "release_operator_id"), optionalString(facets, "planned_verifier_id")
	if operatorID == "" {
		operatorID = legacyOperatorID
	} else if legacyOperatorID != "" && legacyOperatorID != operatorID {
		return invalid("release operator binding disagrees with legacy facet")
	}
	if verifierID == "" {
		verifierID = legacyVerifierID
	} else if legacyVerifierID != "" && legacyVerifierID != verifierID {
		return invalid("release verifier binding disagrees with legacy facet")
	}
	releaseID, releaseKind, recoveryOfReleaseID := "", "", ""
	if optionalUint(release.fields, "attempt_contract_version") >= 2 {
		releaseID, releaseKind, recoveryOfReleaseID = release.id, optionalString(release.fields, "release_kind"), optionalString(release.fields, "recovery_of_release_id")
	}
	p.Release = workflowstatev2.ReleaseFacet{State: workflowstatev2.ReleaseState(releaseState), ReleaseID: releaseID, ReleaseKind: releaseKind, RecoveryOfReleaseID: recoveryOfReleaseID, CandidateID: mustString(release.fields, "candidate_id"), CandidateRevision: rev(mustUint(release.fields, "candidate_revision")), TargetID: mustString(release.fields, "target_key"), Generation: rev(generation), ExpectedTargetReleaseID: optionalString(release.fields, "expected_target_release_id"), ExpectedTargetGeneration: rev(optionalUint(release.fields, "expected_target_generation")), OperationID: optionalString(release.fields, "operation_id"), BranchID: optionalString(release.fields, "branch_key"), AcceptanceVersion: rev(optionalUint(release.fields, "acceptance_version")), ReleaseOperatorID: operatorID, PlannedVerifierID: verifierID, BuildManifestDigest: optionalString(release.fields, "build_manifest_digest"), ArtifactDigest: optionalString(release.fields, "artifact_digest"), RuntimeContractDigest: optionalString(release.fields, "runtime_contract_digest"), AuthorityCapability: optionalString(release.fields, "authority_capability"), AuthorityEvidenceID: optionalString(release.fields, "authority_evidence_id"), OwnerEvidenceRevision: rev(optionalUint(release.fields, "owner_evidence_revision")), RecoveryPolicyRevision: rev(optionalUint(release.fields, "recovery_policy_revision")), ExposurePolicyDigest: optionalString(release.fields, "exposure_policy_digest"), RollbackTargetReleaseID: optionalString(release.fields, "rollback_target_release_id")}
	if effects, ok := recoveryPolicy["irreversible_effects"]; ok {
		p.Release.IrreversibleEffects, err = strictStringSlice(effects, "release irreversible effects")
		if err != nil {
			return err
		}
	}
	effectiveReleaseKind := p.Release.ReleaseKind
	if effectiveReleaseKind == "" {
		effectiveReleaseKind = "forward"
	}
	releaseCandidate := findRevision(byType["candidate"], p.Release.CandidateID, int64(p.Release.CandidateRevision))
	if releaseCandidate == nil || (effectiveReleaseKind != "recovery" && p.Release.CandidateID != candidate.id) || mustString(release.fields, "scope_id") != scope.id || mustUint(release.fields, "scope_revision") != uint64(scope.revision) || mustString(release.fields, "authority_id") != authority.id || mustUint(release.fields, "authority_revision") != uint64(authority.revision) {
		return invalid("release binding does not match authoritative candidate, scope, and authority")
	}
	if acceptance := optionalUint(release.fields, "acceptance_version"); acceptance != 0 && acceptance != uint64(p.Scope.AcceptanceVersion) {
		return invalid("release acceptance version does not match authoritative scope")
	}
	if operationID := optionalString(release.fields, "operation_id"); operationID != "" && operationID != p.Execution.OperationID {
		return invalid("release operation binding does not match authoritative execution")
	}
	if branchKey := optionalString(release.fields, "branch_key"); branchKey != "" && branchKey != branch {
		return invalid("release branch binding does not match authoritative execution")
	}
	if version := optionalUint(release.fields, "attempt_contract_version"); version > 1 {
		if (version != 2 && version != 3) || optionalUint(release.fields, "acceptance_version") == 0 || optionalString(release.fields, "operation_id") == "" || optionalString(release.fields, "branch_key") == "" ||
			!nonNull(release.fields["expected_target_generation"]) || optionalString(release.fields, "release_operator_id") == "" || optionalString(release.fields, "planned_verifier_id") == "" ||
			optionalString(release.fields, "build_manifest_digest") == "" || optionalString(release.fields, "artifact_digest") == "" || optionalString(release.fields, "runtime_contract_digest") == "" ||
			optionalString(release.fields, "authority_capability") == "" || optionalString(release.fields, "authority_evidence_id") == "" || optionalUint(release.fields, "recovery_policy_revision") == 0 {
			return invalid("release attempt v2 has an incomplete immutable snapshot")
		}
		if version == 3 && (optionalString(release.fields, "authority_envelope_digest") == "" || optionalUint(release.fields, "owner_evidence_revision") == 0 || optionalString(release.fields, "exposure_policy_digest") == "" || optionalString(release.fields, "irreversible_effects_digest") == "" || optionalString(release.fields, "reviewed_review_id") == "" || optionalUint(release.fields, "reviewed_review_revision") == 0) {
			return invalid("release attempt v3 has an incomplete authority snapshot")
		}
	}
	if p.Release.ReleaseOperatorID != "" && p.Release.ReleaseOperatorID == p.Release.PlannedVerifierID {
		return invalid("planned verifier must differ from release operator")
	}
	if oneOf(releaseState, "deployed", "exposed", "production_verifying", "live", "degraded", "rolling_back", "rolled_back", "restored") && (p.Release.ReleaseOperatorID == "" || p.Release.PlannedVerifierID == "") {
		return invalid("deployed or verifying release requires operator and planned verifier identities")
	}
	legacyUnboundPending := releaseState == "pending" && optionalUint(release.fields, "attempt_contract_version") <= 1 && p.Release.TargetID == "" && p.Release.Generation == 0
	if (effectiveReleaseKind != "recovery" && p.Authority.CandidateID != "" && p.Authority.CandidateID != p.Release.CandidateID) || (p.Authority.TargetID != "" && p.Authority.TargetID != p.Release.TargetID && !legacyUnboundPending) {
		return invalid("release falls outside authoritative candidate or target")
	}
	if candidateID := optionalString(execution.fields, "candidate_id"); effectiveReleaseKind != "recovery" && candidateID != "" && candidateID != p.Release.CandidateID {
		return invalid("execution candidate does not match authoritative release")
	}
	if targetID := optionalString(execution.fields, "target_key"); targetID != "" && targetID != p.Release.TargetID {
		return invalid("execution target does not match authoritative release")
	}
	if err := validateReleaseAttempt(release, byType, p, generation, releaseState); err != nil {
		return err
	}

	attention := selected[workflowstatev2.ConcernAttention]
	read, _ := stringField(attention.fields, "read_state")
	resolution, _ := stringField(attention.fields, "resolution_state")
	if !oneOf(read, "unread", "read") || !oneOf(resolution, "open", "resuming", "resolved", "snoozed") {
		return invalid("invalid attention state")
	}
	p.Attention = workflowstatev2.AttentionFacet{DecisionID: optionalString(attention.fields, "decision_id"), Read: workflowstatev2.ReadState(read), Acknowledged: boolValue(attention.fields["acknowledged"]), Resolution: workflowstatev2.ResolutionState(resolution)}

	evidenceHead := selected[workflowstatev2.ConcernEvidence]
	evidenceState := heads[workflowstatev2.ConcernEvidence].State
	if !oneOf(evidenceState, "unknown", "current", "stale", "contradicted") {
		return invalid("invalid evidence state %q", evidenceState)
	}
	p.Evidence.State = workflowstatev2.EvidenceState(evidenceState)
	for _, evidence := range byType["evidence"] {
		if nonNull(evidence.fields["superseded_at"]) {
			// Superseded evidence remains reconstructable history, but cannot be
			// current authority for a reducer gate.
		}
		expired := false
		if deadline, ok, err := timeField(evidence.fields, "valid_until"); err != nil {
			return err
		} else if ok && !deadline.After(l.now()) {
			expired = true
			if evidence.id == evidenceHead.id && evidence.revision == evidenceHead.revision {
				return invalid("authoritative evidence %s is expired", evidence.id)
			}
		}
		bound, err := boundEvidence(evidence, p, byType)
		if err != nil {
			return err
		}
		if expired || nonNull(evidence.fields["superseded_at"]) {
			bound.Freshness = workflowstatev2.EvidenceStale
		}
		switch mustString(evidence.fields, "evidence_type") {
		case "runtime":
			p.Evidence.Runtime = append(p.Evidence.Runtime, bound)
		case "business", "regression":
			p.Evidence.Business = append(p.Evidence.Business, bound)
		case "restoration":
			p.Evidence.Restoration = append(p.Evidence.Restoration, bound)
		case "technical_review":
			// Reconstructed once, structurally, by loadTechnicalReviews.
		default:
			p.Evidence.Other = append(p.Evidence.Other, bound)
		}
	}
	if err := l.validateReleaseIncidents(p, byType); err != nil {
		return err
	}

	budget := selected[workflowstatev2.ConcernBudget]
	limits, err := objectField(budget.fields, "limits")
	if err != nil {
		return err
	}
	consumption, err := objectField(budget.fields, "consumption")
	if err != nil {
		return err
	}
	p.Budget = workflowstatev2.BudgetFacet{AttemptsUsed: uint32(optionalUint(consumption, "attempts")), AttemptLimit: uint32(optionalUint(limits, "attempts")), RuntimeUsed: optionalUint(consumption, "runtime"), RuntimeLimit: optionalUint(limits, "runtime"), TokensUsed: optionalUint(consumption, "tokens"), TokenLimit: optionalUint(limits, "tokens"), CostUsed: optionalUint(consumption, "cost"), CostLimit: optionalUint(limits, "cost")}
	if uint64(p.Budget.AttemptsUsed) != optionalUint(consumption, "attempts") || uint64(p.Budget.AttemptLimit) != optionalUint(limits, "attempts") {
		return invalid("attempt budget overflows uint32")
	}

	if err := l.loadDecisions(p, byType, scope, authority); err != nil {
		return err
	}
	var suspended *workflowstatev2.OwnerDecision
	for _, decision := range p.Decisions {
		pendingResume := decision.Resolution == workflowstatev2.ResolutionResuming && p.Attention.Resolution == workflowstatev2.ResolutionResuming
		if decision.SuspendedOperationID == p.Execution.OperationID {
			if suspended == nil || decision.Revision > suspended.Revision || (decision.Revision == suspended.Revision && decision.ID > suspended.ID) {
				copy := decision
				suspended = &copy
			}
		}
		if decision.State == workflowstatev2.DecisionUnanswered || pendingResume {
			p.Evidence.State = workflowstatev2.EvidenceStale
		}
	}
	if suspended != nil {
		p.Execution.SuspendedState, p.Execution.SuspendedOperationID = suspended.SuspendedState, suspended.SuspendedOperationID
	}
	if p.Evidence.State == workflowstatev2.EvidenceStale {
		for i := range p.Evidence.TechnicalReviews {
			p.Evidence.TechnicalReviews[i].Freshness = workflowstatev2.EvidenceStale
		}
		for _, list := range [][]workflowstatev2.BoundEvidence{p.Evidence.Runtime, p.Evidence.Business, p.Evidence.Restoration, p.Evidence.Other} {
			for i := range list {
				list[i].Freshness = workflowstatev2.EvidenceStale
			}
		}
	}
	if err := l.validateReleaseTerminalEvidence(p, byType); err != nil {
		return err
	}
	return nil
}

func (l Loader) validateReleaseTerminalEvidence(p *workflowstatev2.CurrentPicture, byType map[string][]normalizedRecord) error {
	required := []string{}
	switch p.Release.State {
	case workflowstatev2.ReleaseLive:
		required = []string{"runtime", "business", "regression"}
	case workflowstatev2.ReleaseRestored:
		required = []string{"runtime", "business", "regression", "restoration"}
	default:
		return nil
	}
	if p.Evidence.State != workflowstatev2.EvidenceCurrent {
		return invalid("release state %s requires a current authoritative evidence concern", p.Release.State)
	}
	for _, typ := range required {
		found := false
		for _, evidence := range byType["evidence"] {
			if mustString(evidence.fields, "evidence_type") != typ || nonNull(evidence.fields["superseded_at"]) {
				continue
			}
			if deadline, ok, err := timeField(evidence.fields, "valid_until"); err != nil || (ok && !deadline.After(l.now())) {
				continue
			}
			bound, err := boundEvidence(evidence, p, byType)
			if err != nil {
				return err
			}
			if bound.CandidateID == p.Release.CandidateID && bound.TargetID == p.Release.TargetID && bound.TargetGeneration == p.Release.Generation && bound.AcceptanceVersion == p.Scope.AcceptanceVersion && bound.ScopeRevision == p.Scope.Revision && bound.Freshness == workflowstatev2.EvidenceCurrent {
				found = true
				break
			}
		}
		if !found {
			return invalid("release state %s requires exact current %s evidence", p.Release.State, typ)
		}
	}
	return nil
}

func (l Loader) validateReleaseIncidents(p *workflowstatev2.CurrentPicture, byType map[string][]normalizedRecord) error {
	for _, incident := range byType["incident"] {
		values := []bool{nonNull(incident.fields["release_revision"]), nonNull(incident.fields["candidate_id"]), nonNull(incident.fields["candidate_revision"]), nonNull(incident.fields["target_generation"]), nonNull(incident.fields["detection_evidence_id"])}
		exact := true
		for _, present := range values {
			exact = exact && present
		}
		any := optionalString(incident.fields, "release_id") != "" || optionalString(incident.fields, "target_key") != ""
		for _, present := range values {
			any = any || present
		}
		if !exact {
			if any && (values[0] || values[1] || values[2] || values[3] || values[4]) {
				return invalid("incident %s has a partial exact release binding", incident.id)
			}
			continue
		}
		releaseID := optionalString(incident.fields, "release_id")
		releaseRevision := optionalUint(incident.fields, "release_revision")
		release := findRevision(byType["release"], releaseID, int64(releaseRevision))
		if release == nil || optionalString(incident.fields, "candidate_id") != mustString(release.fields, "candidate_id") || optionalUint(incident.fields, "candidate_revision") != mustUint(release.fields, "candidate_revision") || optionalString(incident.fields, "target_key") != mustString(release.fields, "target_key") || optionalUint(incident.fields, "target_generation") != mustUint(release.fields, "target_generation") {
			return invalid("incident %s release binding is inconsistent", incident.id)
		}
		if findLatest(byType["evidence"], optionalString(incident.fields, "detection_evidence_id")) == nil {
			return invalid("incident %s detection evidence is missing", incident.id)
		}
		fingerprint := optionalString(incident.fields, "regression_fingerprint")
		recoveryID := optionalString(incident.fields, "recovery_release_id")
		compatibility := optionalString(incident.fields, "compatibility_state")
		if fingerprint != "" {
			if compatibility == "rollback_safe" {
				recovery := findLatest(byType["release"], recoveryID)
				if recovery == nil || optionalString(recovery.fields, "release_kind") != "recovery" || optionalString(recovery.fields, "recovery_of_release_id") != releaseID || optionalString(recovery.fields, "expected_target_release_id") != releaseID || mustUint(recovery.fields, "expected_target_generation") != optionalUint(incident.fields, "target_generation") || mustUint(recovery.fields, "target_generation") != optionalUint(incident.fields, "target_generation")+1 {
					return invalid("incident %s recovery release binding is inconsistent", incident.id)
				}
			} else if recoveryID != "" {
				return invalid("incident %s containment must not name a recovery release", incident.id)
			}
		}
		state := mustString(incident.fields, "state")
		if !oneOf(state, "resolved", "closed", "cancelled", "superseded") && releaseID == selectedReleaseID(byType, p) && oneOf(string(p.Release.State), "live", "restored") {
			return invalid("terminal release has an unresolved exact incident")
		}
	}
	return nil
}

func selectedReleaseID(byType map[string][]normalizedRecord, p *workflowstatev2.CurrentPicture) string {
	if p.Release.ReleaseID != "" {
		return p.Release.ReleaseID
	}
	for _, release := range byType["release"] {
		if mustString(release.fields, "candidate_id") == p.Release.CandidateID && mustString(release.fields, "target_key") == p.Release.TargetID && mustUint(release.fields, "target_generation") == uint64(p.Release.Generation) {
			return release.id
		}
	}
	return ""
}

func (l Loader) loadDecisions(p *workflowstatev2.CurrentPicture, byType map[string][]normalizedRecord, scope, authority normalizedRecord) error {
	executions := map[string]normalizedRecord{}
	for _, e := range byType["execution"] {
		executions[e.id] = e
	}
	responses := byType["decision_response"]
	decisionRows := map[string]normalizedRecord{}
	for _, decision := range byType["decision"] {
		decisionRows[decision.id] = decision
	}
	for _, response := range responses {
		decision, ok := decisionRows[mustString(response.fields, "decision_id")]
		responseRevision := mustUint(response.fields, "decision_revision")
		if !ok || responseRevision == 0 || responseRevision >= uint64(decision.revision) {
			return invalid("decision response is not bound to a prior decision revision")
		}
		if mustString(response.fields, "actor_id") != mustString(decision.fields, "accountable_owner_id") {
			return invalid("decision response actor is not the accountable owner")
		}
		if !decisionOutcomeAllowed(workflowstatev2.DecisionType(mustString(decision.fields, "decision_type")), workflowstatev2.DecisionOutcome(mustString(response.fields, "response_type"))) {
			return invalid("response outcome is not allowed for decision type")
		}
	}
	for _, decision := range byType["decision"] {
		if nonNull(decision.fields["superseded_at"]) {
			continue
		}
		state := mustString(decision.fields, "state")
		resolution := mustString(decision.fields, "resolution_state")
		if !oneOf(state, "unanswered", "answered", "superseded", "expired") || !oneOf(resolution, "open", "resuming", "resolved", "snoozed") {
			return invalid("invalid decision state")
		}
		if state == "answered" && resolution == "resolved" {
			continue
		}
		if state == "expired" {
			return invalid("active decision %s is expired", decision.id)
		}
		if deadline, ok, err := timeField(decision.fields, "expires_at"); err != nil {
			return err
		} else if ok && !deadline.After(l.now()) && state == "unanswered" {
			return invalid("unanswered decision %s is expired", decision.id)
		}
		if mustString(decision.fields, "scope_id") != scope.id || mustUint(decision.fields, "scope_revision") != uint64(scope.revision) {
			return invalid("decision scope binding does not match authoritative scope")
		}
		if authorityID := optionalString(decision.fields, "authority_id"); authorityID != "" && (authorityID != authority.id || optionalUint(decision.fields, "authority_revision") != uint64(authority.revision)) {
			return invalid("decision authority binding does not match authoritative authority")
		}
		decisionCandidate := optionalString(decision.fields, "candidate_id")
		decisionTarget := optionalString(decision.fields, "target_key")
		var boundCandidate *normalizedRecord
		if decisionCandidate != "" {
			candidate := findLatest(byType["candidate"], decisionCandidate)
			if candidate == nil || mustString(candidate.fields, "scope_id") != mustString(decision.fields, "scope_id") || mustUint(candidate.fields, "scope_revision") != mustUint(decision.fields, "scope_revision") {
				return invalid("decision candidate is missing or outside the decision scope revision")
			}
			boundCandidate = candidate
			if p.Authority.CandidateID != "" && decisionCandidate != p.Authority.CandidateID {
				return invalid("decision candidate falls outside the authority envelope")
			}
		}
		if decisionTarget != "" {
			if p.Authority.TargetID != "" && decisionTarget != p.Authority.TargetID {
				return invalid("decision target falls outside the authority envelope")
			}
			if decisionCandidate == p.Release.CandidateID && p.Release.TargetID != "" && decisionTarget != p.Release.TargetID {
				return invalid("decision target disagrees with its current candidate release")
			}
			if decisionTarget == p.Release.TargetID && decisionCandidate != "" && decisionCandidate != p.Release.CandidateID {
				return invalid("decision candidate disagrees with the current target release")
			}
		}
		if decisionCandidate != "" && decisionTarget != "" {
			applicable := make([]normalizedRecord, 0, 1)
			for _, release := range byType["release"] {
				if mustString(release.fields, "candidate_id") == decisionCandidate &&
					mustUint(release.fields, "candidate_revision") == uint64(boundCandidate.revision) &&
					mustString(release.fields, "target_key") == decisionTarget &&
					mustString(release.fields, "scope_id") == mustString(decision.fields, "scope_id") &&
					mustUint(release.fields, "scope_revision") == mustUint(decision.fields, "scope_revision") &&
					!nonNull(release.fields["superseded_at"]) {
					applicable = append(applicable, release)
				}
			}
			if len(applicable) != 1 {
				return invalid("decision candidate and target require exactly one applicable release, got %d", len(applicable))
			}
			release := applicable[0]
			head := findTargetHead(byType["target_head"], decisionTarget)
			state := mustString(release.fields, "state")
			observed := oneOf(state, "deployed", "exposed", "production_verifying", "live", "degraded", "rolling_back", "rolled_back", "restored")
			if observed {
				if head == nil || mustString(head.fields, "issue_id") != p.IssueID || mustString(head.fields, "release_id") != release.id || mustString(head.fields, "candidate_id") != decisionCandidate || mustUint(head.fields, "generation") != mustUint(release.fields, "target_generation") || mustString(head.fields, "state") != state {
					return invalid("decision observed release disagrees with the target head")
				}
			} else {
				if !nonNull(release.fields["expected_target_generation"]) {
					return invalid("decision release is missing its target-head predecessor")
				}
				expected := mustUint(release.fields, "expected_target_generation")
				expectedID := optionalString(release.fields, "expected_target_release_id")
				if (expected == 0 && head != nil) || (expected > 0 && (head == nil || mustUint(head.fields, "generation") != expected || mustString(head.fields, "release_id") != expectedID)) {
					return invalid("decision release predecessor disagrees with the target head")
				}
			}
		}
		suspended := mustString(decision.fields, "suspended_operation_id")
		e, ok := executions[suspended]
		if !ok {
			return invalid("decision %s references missing suspended operation", decision.id)
		}
		suspendedState := mustString(decision.fields, "suspended_execution_state")
		if !oneOf(suspendedState, "received", "queued", "working", "waiting_automatic", "in_review", "correction", "blocked_external", "technically_stopped", "paused", "completed", "failed", "cancelled", "superseded") {
			return invalid("decision %s has invalid suspended execution state", decision.id)
		}
		gap, err := objectField(decision.fields, "authority_gap")
		if err != nil {
			return err
		}
		item := workflowstatev2.OwnerDecision{ID: decision.id, Revision: rev(decision.revision), Type: workflowstatev2.DecisionType(mustString(decision.fields, "decision_type")), State: workflowstatev2.DecisionState(state), Resolution: workflowstatev2.ResolutionState(resolution), ScopeRevision: rev(mustUint(decision.fields, "scope_revision")), AuthorityRevision: rev(optionalUint(decision.fields, "authority_revision")), CandidateID: decisionCandidate, TargetID: decisionTarget, SuspendedState: workflowstatev2.ExecutionState(suspendedState), SuspendedOperationID: suspended, BranchID: mustString(e.fields, "branch_key"), AuthorityGap: workflowstatev2.AuthorityGap{Kind: workflowstatev2.AuthorityGapKind(optionalString(gap, "kind")), RequestedCapability: optionalString(gap, "requested_capability"), Reason: optionalString(gap, "reason")}}
		if item.Type == workflowstatev2.DecisionAuthorization && (item.AuthorityGap.Kind != workflowstatev2.AuthorityGapOperationAuthorization || item.AuthorityGap.RequestedCapability == "" || item.AuthorityGap.Reason == "") {
			return invalid("authorization decision %s has no typed authority gap", decision.id)
		}
		suspendedRevision := optionalUint(gap, "suspended_execution_revision")
		expectedExecutionRevision := suspendedRevision
		if state == "answered" && resolution == "resuming" {
			expectedExecutionRevision++
			if p.Execution.State == workflowstatev2.ExecutionTechnicallyStopped {
				expectedExecutionRevision++
			}
		}
		if item.Type == workflowstatev2.DecisionAuthorization && (boundCandidate == nil || optionalUint(gap, "candidate_revision") != uint64(boundCandidate.revision) || optionalString(gap, "branch_key") != item.BranchID || suspendedRevision == 0 || expectedExecutionRevision != uint64(e.revision)) {
			return invalid("authorization decision %s lost its exact candidate, branch, or execution revision", decision.id)
		}
		matched := 0
		latestResponseRevision := uint64(0)
		for _, response := range responses {
			if mustString(response.fields, "decision_id") == decision.id {
				revision := mustUint(response.fields, "decision_revision")
				if revision < uint64(decision.revision) && revision > latestResponseRevision {
					latestResponseRevision = revision
				}
			}
		}
		for _, response := range responses {
			if mustString(response.fields, "decision_id") == decision.id && mustUint(response.fields, "decision_revision") == latestResponseRevision && latestResponseRevision != 0 {
				matched++
				item.Response = workflowstatev2.DecisionOutcome(mustString(response.fields, "response_type"))
				content, err := objectField(response.fields, "content")
				if err != nil {
					return err
				}
				item.SupportingText = optionalString(content, "text")
			}
		}
		if (decision.revision == 1 && matched != 0) || (decision.revision > 1 && matched != 1) {
			return invalid("decision %s has %d responses for its latest revision", decision.id, matched)
		}
		if matched == 1 {
			expectedState, expectedResolution := "answered", "resolved"
			switch item.Response {
			case workflowstatev2.DecisionAskQuestion:
				expectedState, expectedResolution = "unanswered", "open"
			case workflowstatev2.DecisionHold:
				expectedState, expectedResolution = "unanswered", "snoozed"
			case workflowstatev2.DecisionApprove, workflowstatev2.DecisionReject, workflowstatev2.DecisionProvideFact, workflowstatev2.DecisionExternalStepComplete, workflowstatev2.DecisionAuthenticationComplete:
				expectedResolution = "resuming"
			}
			if state != expectedState || resolution != expectedResolution {
				return invalid("decision state does not match its latest typed response")
			}
		}
		p.Decisions[item.ID] = item
	}
	return nil
}

func decisionOutcomeAllowed(t workflowstatev2.DecisionType, outcome workflowstatev2.DecisionOutcome) bool {
	if outcome == workflowstatev2.DecisionAskQuestion || outcome == workflowstatev2.DecisionHold {
		return true
	}
	switch t {
	case workflowstatev2.DecisionAuthorization:
		return oneOf(string(outcome), "approve", "reject")
	case workflowstatev2.DecisionApproval:
		return oneOf(string(outcome), "approve", "reject", "request_changes", "partial_acceptance", "cancel")
	case workflowstatev2.DecisionDirection:
		return oneOf(string(outcome), "approve", "reject", "request_changes", "partial_acceptance", "change_scope", "cancel")
	case workflowstatev2.DecisionFactualInput:
		return oneOf(string(outcome), "provide_fact", "cancel")
	case workflowstatev2.DecisionBudget:
		return oneOf(string(outcome), "approve", "reject", "change_budget", "cancel")
	case workflowstatev2.DecisionCancellation:
		return oneOf(string(outcome), "cancel", "reject")
	case workflowstatev2.DecisionExternalStep:
		return oneOf(string(outcome), "external_step_complete", "cancel")
	case workflowstatev2.DecisionAuthentication:
		return oneOf(string(outcome), "authentication_complete", "cancel")
	default:
		return false
	}
}

func loadTechnicalReviews(p *workflowstatev2.CurrentPicture, byType map[string][]normalizedRecord) error {
	evidenceByID := make(map[string]normalizedRecord, len(byType["evidence"]))
	for _, evidence := range byType["evidence"] {
		evidenceByID[evidence.id] = evidence
	}
	for _, review := range byType["review"] {
		candidateID := mustString(review.fields, "candidate_id")
		candidateRevision := mustUint(review.fields, "candidate_revision")
		candidate := findRevision(byType["candidate"], candidateID, int64(candidateRevision))
		reviewScopeID, reviewScopeRevision := mustString(review.fields, "scope_id"), mustUint(review.fields, "scope_revision")
		reviewScope := findRevision(byType["scope"], reviewScopeID, int64(reviewScopeRevision))
		if candidate == nil || reviewScope == nil || mustString(candidate.fields, "scope_id") != reviewScopeID || mustUint(candidate.fields, "scope_revision") != reviewScopeRevision {
			return invalid("review references a missing or inconsistently scoped candidate")
		}
		reviewer := mustString(review.fields, "reviewer_id")
		if reviewer == mustString(candidate.fields, "created_by") {
			return invalid("reviewer must differ from implementer")
		}
		result := workflowstatev2.ReviewState(mustString(review.fields, "outcome"))
		if !oneOf(string(result), "not_required", "pending", "in_progress", "passed", "failed", "inconclusive", "blocked_by_evidence") {
			return invalid("invalid review state %q", result)
		}
		criteria, err := strictStringSlice(review.fields["criteria"], "review criteria")
		if err != nil {
			return err
		}
		gaps, err := strictStringSlice(review.fields["gaps"], "review gaps")
		if err != nil {
			return err
		}
		refs, err := strictStringSlice(review.fields["evidence_refs"], "review evidence refs")
		if err != nil {
			return err
		}
		if oneOf(string(result), "passed", "failed", "inconclusive", "blocked_by_evidence") && len(refs) == 0 {
			return invalid("completed review %s has no evidence references", review.id)
		}
		freshness := workflowstatev2.EvidenceStale
		if !nonNull(review.fields["superseded_at"]) && candidateID == p.Review.CandidateID && reviewScopeRevision == uint64(p.Scope.Revision) {
			freshness = workflowstatev2.EvidenceCurrent
		}
		correction, err := objectField(review.fields, "correction_packet")
		if err != nil {
			return err
		}
		for _, ref := range refs {
			evidence, ok := evidenceByID[ref]
			if !ok || mustString(evidence.fields, "concern_type") != "review" || mustString(evidence.fields, "evidence_type") != "technical_review" || mustString(evidence.fields, "producer_type") != "reviewer" || optionalString(evidence.fields, "producer_id") != reviewer || optionalString(evidence.fields, "candidate_id") != candidateID {
				return invalid("review %s references unresolved or incorrectly bound evidence %s", review.id, ref)
			}
			payload, err := objectField(evidence.fields, "payload")
			if err != nil || optionalUint(payload, "candidate_revision") != candidateRevision || optionalString(payload, "scope_id") != reviewScopeID || optionalUint(payload, "scope_revision") != reviewScopeRevision || optionalString(payload, "freshness") != "current" {
				return invalid("review %s evidence %s binding disagrees with review", review.id, ref)
			}
			if mustString(evidence.fields, "source") == "workflow_review_task" {
				payloadCriteria, criteriaErr := strictStringSlice(payload["criteria_refs"], "review evidence criteria refs")
				payloadCoverage, coverageErr := strictStringSlice(payload["coverage_refs"], "review evidence coverage refs")
				expectedCoverage := objectKeys(review.fields["coverage"])
				sort.Strings(payloadCriteria)
				sort.Strings(payloadCoverage)
				sort.Strings(criteria)
				sort.Strings(expectedCoverage)
				if criteriaErr != nil || coverageErr != nil || optionalString(payload, "verdict") != string(result) || optionalString(payload, "review_id") != review.id || optionalUint(payload, "review_revision") != 1 || !reflect.DeepEqual(payloadCriteria, criteria) || !reflect.DeepEqual(payloadCoverage, expectedCoverage) {
					return invalid("review %s task evidence %s verdict binding disagrees with review", review.id, ref)
				}
			}
			p.Evidence.TechnicalReviews = append(p.Evidence.TechnicalReviews, workflowstatev2.ReviewEvidence{
				BoundEvidence: workflowstatev2.BoundEvidence{Ref: ref, CandidateID: candidateID, AcceptanceVersion: rev(mustUint(reviewScope.fields, "acceptance_version")), ScopeRevision: rev(reviewScopeRevision), Freshness: freshness},
				Result:        result, ReviewerID: reviewer, CriteriaRefs: criteria, CoverageRefs: objectKeys(review.fields["coverage"]), GapRefs: gaps, RequiredCorrection: optionalString(correction, "required_correction"),
			})
		}
	}
	return nil
}

func boundEvidence(r normalizedRecord, p *workflowstatev2.CurrentPicture, byType map[string][]normalizedRecord) (workflowstatev2.BoundEvidence, error) {
	payload, err := objectField(r.fields, "payload")
	if err != nil {
		return workflowstatev2.BoundEvidence{}, err
	}
	b := workflowstatev2.BoundEvidence{Ref: r.id, CandidateID: optionalString(r.fields, "candidate_id"), TargetID: optionalString(r.fields, "target_key"), TargetGeneration: rev(optionalUint(payload, "target_generation")), AcceptanceVersion: rev(optionalUint(payload, "acceptance_version")), ScopeRevision: rev(optionalUint(payload, "scope_revision")), Freshness: workflowstatev2.EvidenceState(optionalString(payload, "freshness"))}
	if b.Freshness == "" {
		b.Freshness = workflowstatev2.EvidenceUnknown
	}
	if b.Freshness != workflowstatev2.EvidenceUnknown && b.Freshness != workflowstatev2.EvidenceCurrent && b.Freshness != workflowstatev2.EvidenceStale && b.Freshness != workflowstatev2.EvidenceContradicted {
		return b, invalid("invalid evidence freshness")
	}
	typ := mustString(r.fields, "evidence_type")
	if typ == "owner_request" {
		if mustString(r.fields, "producer_type") != "owner" || optionalString(r.fields, "producer_id") == "" {
			return b, invalid("owner request evidence requires an attributable owner producer")
		}
		return b, nil
	}
	if typ == "implementation" {
		if mustString(r.fields, "producer_type") != "worker" || optionalString(r.fields, "producer_id") == "" || b.CandidateID == "" {
			return b, invalid("implementation evidence requires worker and candidate binding")
		}
		taskID, actionID := optionalString(payload, "task_id"), optionalString(payload, "workflow_action_id")
		operationID, branchKey := optionalString(payload, "operation_id"), optionalString(payload, "branch_key")
		if taskID == "" || actionID == "" || operationID == "" || branchKey == "" || optionalString(payload, "source_revision") == "" {
			return b, invalid("implementation evidence requires exact task, action, operation, branch, and source revision")
		}
		action := findLatest(byType["action_receipt"], actionID)
		if action == nil || mustString(action.fields, "action_type") != "review_correction" || mustString(action.fields, "state") != "settled" || optionalString(action.fields, "bound_task_id") != taskID || optionalString(action.fields, "operation_id") != operationID {
			return b, invalid("implementation evidence action binding is missing or inconsistent")
		}
		actionPayload, err := objectField(action.fields, "payload")
		if err != nil || optionalString(actionPayload, "branch_key") != branchKey || optionalString(actionPayload, "correction_worker_id") != optionalString(r.fields, "producer_id") {
			return b, invalid("implementation evidence disagrees with correction action")
		}
	}
	if oneOf(typ, "runtime", "business", "regression", "restoration") && b.CandidateID == "" {
		return b, invalid("release evidence requires candidate binding")
	}
	if b.CandidateID != "" {
		candidateRevision, err := uintField(payload, "candidate_revision")
		if err != nil {
			return b, err
		}
		candidate := findRevision(byType["candidate"], b.CandidateID, int64(candidateRevision))
		if candidate == nil {
			return b, invalid("evidence references missing candidate revision")
		}
		releaseID := optionalString(payload, "release_id")
		if oneOf(typ, "runtime", "business", "regression", "restoration") && releaseID == "" {
			return b, invalid("release evidence requires release binding")
		}
		if releaseID != "" {
			release := findLatest(byType["release"], releaseID)
			if release == nil || mustString(release.fields, "candidate_id") != b.CandidateID || mustUint(release.fields, "candidate_revision") != candidateRevision || mustString(release.fields, "target_key") != b.TargetID || mustUint(release.fields, "target_generation") != uint64(b.TargetGeneration) || mustUint(release.fields, "scope_revision") != uint64(b.ScopeRevision) {
				return b, invalid("evidence binding disagrees with its candidate or release")
			}
			scopeID := mustString(release.fields, "scope_id")
			scope := findRevision(byType["scope"], scopeID, int64(b.ScopeRevision))
			if scope == nil || optionalString(payload, "scope_id") != scopeID || mustUint(scope.fields, "acceptance_version") != uint64(b.AcceptanceVersion) {
				return b, invalid("evidence binding disagrees with its scope acceptance")
			}
			if oneOf(typ, "runtime", "business", "regression", "restoration") {
				producerID := optionalString(r.fields, "producer_id")
				facets, _ := optionalObject(release.fields, "facets")
				operator, verifier := optionalString(release.fields, "release_operator_id"), optionalString(release.fields, "planned_verifier_id")
				legacyOperator, legacyVerifier := optionalString(facets, "release_operator_id"), optionalString(facets, "planned_verifier_id")
				if operator == "" {
					operator = legacyOperator
				} else if legacyOperator != "" && legacyOperator != operator {
					return b, invalid("release evidence operator binding disagrees with legacy facet")
				}
				if verifier == "" {
					verifier = legacyVerifier
				} else if legacyVerifier != "" && legacyVerifier != verifier {
					return b, invalid("release evidence verifier binding disagrees with legacy facet")
				}
				if mustString(r.fields, "producer_type") != "verifier" || producerID == "" || verifier == "" || producerID != verifier || producerID == operator {
					return b, invalid("release evidence requires the independent planned verifier")
				}
			}
		}
	}
	return b, nil
}

func decodeRecord(workspaceID, issueID string, row db.ListIssueWorkflowProjectionInputsRow) (normalizedRecord, error) {
	if row.Revision < 1 || !row.ID.Valid {
		return normalizedRecord{}, invalid("malformed %s envelope", row.RecordType)
	}
	decoder := json.NewDecoder(bytes.NewReader(row.Payload))
	decoder.UseNumber()
	var fields map[string]any
	if err := decoder.Decode(&fields); err != nil || fields == nil {
		return normalizedRecord{}, invalid("decode %s payload", row.RecordType)
	}
	id := util.UUIDToString(row.ID)
	if row.RecordType == "target_head" {
		if mustString(fields, "workspace_id") != workspaceID {
			return normalizedRecord{}, invalid("cross-tenant target head")
		}
		if err := validateShape(row.RecordType, fields); err != nil {
			return normalizedRecord{}, err
		}
		if mustString(fields, "release_id") != id || mustUint(fields, "revision") != uint64(row.Revision) {
			return normalizedRecord{}, invalid("target head payload disagrees with authoritative envelope")
		}
		return normalizedRecord{typ: row.RecordType, id: id, revision: row.Revision, fields: fields}, nil
	}
	if mustString(fields, "id") != id || mustString(fields, "workspace_id") != workspaceID || mustString(fields, "issue_id") != issueID {
		return normalizedRecord{}, invalid("%s payload disagrees with authoritative envelope", row.RecordType)
	}
	if embedded, ok := fields["revision"]; ok && embedded != nil && mustUint(fields, "revision") != uint64(row.Revision) {
		return normalizedRecord{}, invalid("%s revision disagrees with authoritative envelope", row.RecordType)
	}
	if err := validateShape(row.RecordType, fields); err != nil {
		return normalizedRecord{}, err
	}
	return normalizedRecord{typ: row.RecordType, id: id, revision: row.Revision, fields: fields}, nil
}

func validateShape(typ string, fields map[string]any) error {
	required, known := requiredShape[typ]
	if !known {
		return invalid("unknown normalized record type %q", typ)
	}
	for name, kind := range required {
		value, ok := fields[name]
		if !ok || value == nil || !matchesKind(value, kind) {
			return invalid("%s required field %s has wrong type", typ, name)
		}
	}
	for name, kind := range optionalShape[typ] {
		if value, ok := fields[name]; ok && value != nil && !matchesKind(value, kind) {
			return invalid("%s optional field %s has wrong type", typ, name)
		}
	}
	if err := validateNestedShapes(typ, fields); err != nil {
		return err
	}
	return nil
}

func validateNestedShapes(typ string, fields map[string]any) error {
	strictOptional := func(object map[string]any, key string, kind fieldKind) error {
		if value, ok := object[key]; ok && value != nil && !matchesKind(value, kind) {
			return invalid("%s nested field %s has wrong type", typ, key)
		}
		return nil
	}
	switch typ {
	case "authority":
		envelope, _ := optionalObject(fields, "envelope")
		for _, key := range []string{"target_id", "candidate_id"} {
			if err := strictOptional(envelope, key, stringKind); err != nil {
				return err
			}
		}
	case "execution":
		for _, objectKey := range []string{"checkpoint", "expected_event", "wait_reason"} {
			object, _ := optionalObject(fields, objectKey)
			for _, key := range []string{"id", "type", "next_action", "resume_condition"} {
				if err := strictOptional(object, key, stringKind); err != nil {
					return err
				}
			}
		}
	case "release":
		facets, _ := optionalObject(fields, "facets")
		for _, key := range []string{"release_operator_id", "planned_verifier_id"} {
			if err := strictOptional(facets, key, stringKind); err != nil {
				return err
			}
		}
	case "evidence":
		payload, _ := optionalObject(fields, "payload")
		for _, key := range []string{"candidate_revision", "target_generation", "acceptance_version", "scope_revision"} {
			if err := strictOptional(payload, key, integerKind); err != nil {
				return err
			}
		}
		for _, key := range []string{"release_id", "scope_id", "freshness", "task_id", "workflow_action_id", "operation_id", "branch_key", "source_revision"} {
			if err := strictOptional(payload, key, stringKind); err != nil {
				return err
			}
		}
		for _, key := range []string{"limitations", "contradictions"} {
			if _, err := strictStringSlice(fields[key], "evidence "+key); err != nil {
				return err
			}
		}
	case "budget":
		for _, objectKey := range []string{"limits", "consumption"} {
			object, _ := optionalObject(fields, objectKey)
			for _, key := range []string{"attempts", "runtime", "tokens", "cost"} {
				if err := strictOptional(object, key, integerKind); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func matchesKind(value any, kind fieldKind) bool {
	switch kind {
	case stringKind:
		text, ok := value.(string)
		return ok && text != ""
	case stringAllowEmptyKind:
		_, ok := value.(string)
		return ok
	case integerKind:
		_, ok := exactUint(value)
		return ok
	case objectKind:
		_, ok := value.(map[string]any)
		return ok
	case arrayKind:
		_, ok := value.([]any)
		return ok
	case boolKind:
		_, ok := value.(bool)
		return ok
	default:
		return false
	}
}

func (l Loader) now() time.Time {
	if l.Now != nil {
		return l.Now().UTC()
	}
	return time.Now().UTC()
}
func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidAuthoritativeState, fmt.Sprintf(format, args...))
}
func recordKey(typ, id string, revision int64) string {
	return typ + "/" + id + "/" + strconv.FormatInt(revision, 10)
}
func rev(v any) workflowstatev2.Revision {
	switch n := v.(type) {
	case int64:
		return workflowstatev2.Revision(n)
	case uint64:
		return workflowstatev2.Revision(n)
	default:
		return 0
	}
}
func stateField(typ string) string {
	switch typ {
	case "review":
		return "outcome"
	case "evidence":
		return "payload.freshness"
	case "attention":
		return "resolution_state"
	default:
		return "state"
	}
}
func headState(r normalizedRecord) string {
	payload, _ := optionalObject(r.fields, "payload")
	return optionalString(payload, "freshness")
}
func nonNull(v any) bool { return v != nil }
func oneOf(v string, allowed ...string) bool {
	for _, a := range allowed {
		if v == a {
			return true
		}
	}
	return false
}
func stringField(m map[string]any, key string) (string, error) {
	if key == "payload.freshness" {
		p, err := objectField(m, "payload")
		if err != nil {
			return "", err
		}
		return stringField(p, "freshness")
	}
	v, ok := m[key].(string)
	if !ok || v == "" {
		return "", invalid("required string %s missing", key)
	}
	return v, nil
}
func mustString(m map[string]any, key string) string { v, _ := m[key].(string); return v }
func optionalString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	switch v := m[key].(type) {
	case string:
		return v
	case map[string]any:
		b, _ := json.Marshal(v)
		return string(b)
	default:
		return ""
	}
}
func objectField(m map[string]any, key string) (map[string]any, error) {
	v, ok := m[key].(map[string]any)
	if !ok {
		return nil, invalid("required object %s missing", key)
	}
	return v, nil
}
func optionalObject(m map[string]any, key string) (map[string]any, bool) {
	v, ok := m[key].(map[string]any)
	return v, ok
}
func uintField(m map[string]any, key string) (uint64, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return 0, invalid("required integer %s missing", key)
	}
	n, ok := exactUint(v)
	if !ok {
		return 0, invalid("%s must be a nonnegative integer", key)
	}
	return n, nil
}
func mustUint(m map[string]any, key string) uint64     { n, _ := exactUint(m[key]); return n }
func optionalUint(m map[string]any, key string) uint64 { n, _ := exactUint(m[key]); return n }
func exactUint(v any) (uint64, bool) {
	switch n := v.(type) {
	case json.Number:
		u, err := strconv.ParseUint(string(n), 10, 64)
		return u, err == nil
	case float64:
		if n < 0 || n > math.MaxUint64 || math.Trunc(n) != n {
			return 0, false
		}
		return uint64(n), true
	case int64:
		return uint64(n), n >= 0
	case uint64:
		return n, true
	default:
		return 0, false
	}
}
func boolValue(v any) bool { b, _ := v.(bool); return b }
func timeField(m map[string]any, key string) (time.Time, bool, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return time.Time{}, false, nil
	}
	s, ok := v.(string)
	if !ok {
		return time.Time{}, false, invalid("%s must be a timestamp", key)
	}
	parsed, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}, false, invalid("%s is malformed", key)
	}
	return parsed.UTC(), true, nil
}
func strictStringSlice(v any, name string) ([]string, error) {
	raw, ok := v.([]any)
	if !ok {
		return nil, invalid("%s must be an array", name)
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		s, ok := item.(string)
		if !ok || s == "" {
			return nil, invalid("%s must contain nonempty strings", name)
		}
		out = append(out, s)
	}
	return out, nil
}
func objectKeys(v any) []string {
	m, _ := v.(map[string]any)
	out := make([]string, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
func findLatest(rows []normalizedRecord, id string) *normalizedRecord {
	var found *normalizedRecord
	for i := range rows {
		if rows[i].id == id && (found == nil || rows[i].revision > found.revision) {
			found = &rows[i]
		}
	}
	return found
}
func findRevision(rows []normalizedRecord, id string, revision int64) *normalizedRecord {
	for i := range rows {
		if rows[i].id == id && rows[i].revision == revision {
			return &rows[i]
		}
	}
	return nil
}
func findTargetHead(rows []normalizedRecord, target string) *normalizedRecord {
	for i := range rows {
		if mustString(rows[i].fields, "target_key") == target {
			return &rows[i]
		}
	}
	return nil
}
