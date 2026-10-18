package workflowcommand

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"sort"
	"strings"
	"time"

	googleuuid "github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/internal/workflowprojection"
	workflowstatev2 "github.com/multica-ai/multica/server/internal/workflowstate/v2"
	"github.com/multica-ai/multica/server/internal/workflowstore"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

var (
	ErrForbidden             = errors.New("workflow command principal is not authorized")
	ErrDecisionStale         = errors.New("workflow owner decision is stale, replaced, or terminal")
	ErrIdempotencyConflict   = errors.New("workflow command idempotency conflict")
	ErrRecoveryRequired      = errors.New("workflow command was received but not processed")
	ErrUnsupportedCommand    = errors.New("workflow command persistence is not implemented")
	ErrPersistenceDivergence = errors.New("persisted workflow state diverges from reducer result")
)

// Principal is supplied by trusted server authentication/lifecycle code. A
// request deliberately has no actor or role fields.
type Principal struct {
	ActorID    pgtype.UUID
	ActorType  string
	TaskID     pgtype.UUID
	ActionID   pgtype.UUID
	ActionKey  string
	LeaseToken pgtype.UUID
}

// ReleaseEffectResultInput is a normalized provider observation. It is
// produced by the durable action worker, never by an HTTP caller. Every field
// is compared with the immutable action snapshot before it can advance state.
type ReleaseEffectResultInput struct {
	Type                  string         `json:"type"`
	EffectKey             string         `json:"effect_key"`
	ProviderReceiptID     string         `json:"provider_receipt_id"`
	ReleaseID             string         `json:"release_id"`
	CandidateID           string         `json:"candidate_id"`
	CandidateRevision     int64          `json:"candidate_revision"`
	ScopeID               string         `json:"scope_id"`
	ScopeRevision         int64          `json:"scope_revision"`
	AcceptanceVersion     int64          `json:"acceptance_version"`
	OperationID           string         `json:"operation_id"`
	BranchKey             string         `json:"branch_key"`
	TargetKey             string         `json:"target_key"`
	TargetGeneration      int64          `json:"target_generation"`
	ArtifactDigest        string         `json:"artifact_digest"`
	RuntimeContractDigest string         `json:"runtime_contract_digest"`
	Evidence              map[string]any `json:"evidence"`
}

type DecisionInput struct {
	Type               workflowstatev2.DecisionType `json:"type"`
	Question           string                       `json:"question"`
	Recommendation     string                       `json:"recommendation"`
	Consequence        string                       `json:"consequence"`
	Options            []string                     `json:"options"`
	AccountableOwnerID pgtype.UUID                  `json:"accountable_owner_id"`
	AuthorityGap       workflowstatev2.AuthorityGap `json:"authority_gap"`
}

// CorrectionResultInput is the only task result shape that may create a new
// workflow candidate. Free-form task output and PR URLs are deliberately not
// represented here and remain progress-only data.
type CorrectionResultInput struct {
	Type               string             `json:"type"`
	SourceRevision     string             `json:"source_revision,omitempty"`
	Artifact           map[string]any     `json:"artifact,omitempty"`
	BuildContract      map[string]any     `json:"build_contract,omitempty"`
	Evidence           map[string]any     `json:"evidence,omitempty"`
	Verdict            string             `json:"verdict,omitempty"`
	CriteriaRefs       []string           `json:"criteria_refs,omitempty"`
	CoverageRefs       []string           `json:"coverage_refs,omitempty"`
	GapRefs            []string           `json:"gap_refs,omitempty"`
	RequiredCorrection string             `json:"required_correction,omitempty"`
	RuntimeProof       *VerificationProof `json:"runtime_proof,omitempty"`
	BusinessProof      *VerificationProof `json:"business_proof,omitempty"`
	RegressionProof    *VerificationProof `json:"regression_proof,omitempty"`
	RestorationProof   *VerificationProof `json:"restoration_proof,omitempty"`
}

// VerificationProof is an independently attributable, structured verifier
// result. Criteria and coverage are references into the immutable release
// packet; Evidence contains the verifier's observed facts, never provider or
// controller-authored conclusions.
type VerificationProof struct {
	CriteriaRefs []string       `json:"criteria_refs"`
	CoverageRefs []string       `json:"coverage_refs"`
	Evidence     map[string]any `json:"evidence"`
}

type ReleaseAdmissionInput struct {
	TargetID                 string         `json:"target_id"`
	ExpectedTargetReleaseID  string         `json:"expected_target_release_id,omitempty"`
	ExpectedTargetGeneration uint64         `json:"expected_target_generation"`
	DesiredTargetGeneration  uint64         `json:"desired_target_generation"`
	ReleaseOperatorID        pgtype.UUID    `json:"release_operator_id"`
	PlannedVerifierID        pgtype.UUID    `json:"planned_verifier_id"`
	BuildManifestDigest      string         `json:"build_manifest_digest"`
	ArtifactDigest           string         `json:"artifact_digest"`
	RuntimeContractDigest    string         `json:"runtime_contract_digest"`
	ExposurePolicy           map[string]any `json:"exposure_policy"`
	AudiencePolicy           map[string]any `json:"audience_policy"`
	RiskEnvelope             map[string]any `json:"risk_envelope"`
	Exclusions               []string       `json:"exclusions"`
	BudgetEnvelope           map[string]any `json:"budget_envelope"`
	RecoveryPolicy           map[string]any `json:"recovery_policy"`
	RecoveryPolicyRevision   uint64         `json:"recovery_policy_revision"`
	RollbackTargetReleaseID  string         `json:"rollback_target_release_id,omitempty"`
	IrreversibleEffects      []string       `json:"irreversible_effects"`
}

type ReleaseBuildResultInput struct {
	Type                  string         `json:"type"`
	BuildManifestDigest   string         `json:"build_manifest_digest"`
	ArtifactDigest        string         `json:"artifact_digest"`
	RuntimeContractDigest string         `json:"runtime_contract_digest"`
	Reproducible          bool           `json:"reproducible"`
	Coverage              map[string]any `json:"coverage"`
}

type deploymentAuthorityEnvelope struct {
	Version                  int            `json:"version"`
	Capability               string         `json:"capability"`
	CandidateID              string         `json:"candidate_id"`
	CandidateRevision        int64          `json:"candidate_revision"`
	ScopeID                  string         `json:"scope_id"`
	ScopeRevision            int64          `json:"scope_revision"`
	AcceptanceVersion        int64          `json:"acceptance_version"`
	OperationID              string         `json:"operation_id"`
	BranchKey                string         `json:"branch_key"`
	TargetKey                string         `json:"target_key"`
	TargetID                 string         `json:"target_id"`
	ExpectedTargetReleaseID  string         `json:"expected_target_release_id,omitempty"`
	ExpectedTargetGeneration int64          `json:"expected_target_generation"`
	DesiredTargetGeneration  int64          `json:"desired_target_generation"`
	ExposurePolicy           map[string]any `json:"exposure_policy"`
	AudiencePolicy           map[string]any `json:"audience_policy"`
	RiskEnvelope             map[string]any `json:"risk_envelope"`
	Exclusions               []string       `json:"exclusions"`
	BudgetEnvelope           map[string]any `json:"budget_envelope"`
	RecoveryPolicy           map[string]any `json:"recovery_policy"`
	RecoveryPolicyRevision   int64          `json:"recovery_policy_revision"`
	RollbackTargetReleaseID  string         `json:"rollback_target_release_id,omitempty"`
	IrreversibleEffects      []string       `json:"irreversible_effects"`
	OwnerEvidenceID          string         `json:"owner_evidence_id"`
	OwnerEvidenceRevision    int64          `json:"owner_evidence_revision"`
	ReleaseOperatorID        string         `json:"release_operator_id"`
	PlannedVerifierID        string         `json:"planned_verifier_id"`
	BuildManifestDigest      string         `json:"build_manifest_digest"`
	ArtifactDigest           string         `json:"artifact_digest"`
	RuntimeContractDigest    string         `json:"runtime_contract_digest"`
	ReviewedReviewID         string         `json:"reviewed_review_id"`
	ReviewedReviewRevision   int64          `json:"reviewed_review_revision"`
}

type releaseEffectSnapshot struct {
	ContractVersion            int                                 `json:"contract_version"`
	EffectContract             string                              `json:"effect_contract"`
	CommandHash                string                              `json:"command_hash"`
	OperationID                string                              `json:"operation_id"`
	BranchKey                  string                              `json:"branch_key"`
	ReleaseID                  string                              `json:"release_id"`
	ReleaseKind                string                              `json:"release_kind,omitempty"`
	RecoveryOfReleaseID        string                              `json:"recovery_of_release_id,omitempty"`
	ReleaseRevision            int64                               `json:"release_revision"`
	CandidateID                string                              `json:"candidate_id"`
	CandidateRevision          int64                               `json:"candidate_revision"`
	ScopeID                    string                              `json:"scope_id"`
	ScopeRevision              int64                               `json:"scope_revision"`
	AcceptanceVersion          int64                               `json:"acceptance_version"`
	ReviewID                   string                              `json:"review_id"`
	ReviewRevision             int64                               `json:"review_revision"`
	AuthorityID                string                              `json:"authority_id"`
	AuthorityRevision          int64                               `json:"authority_revision"`
	AuthorityCapability        string                              `json:"authority_capability"`
	AuthorityEnvelopeDigest    string                              `json:"authority_envelope_digest"`
	AuthorityEvidenceID        string                              `json:"authority_evidence_id"`
	OwnerEvidenceRevision      int64                               `json:"owner_evidence_revision"`
	TargetKey                  string                              `json:"target_key"`
	ExpectedTargetReleaseID    string                              `json:"expected_target_release_id"`
	ExpectedTargetGeneration   int64                               `json:"expected_target_generation"`
	DesiredTargetGeneration    int64                               `json:"desired_target_generation"`
	ReleaseOperatorID          string                              `json:"release_operator_id"`
	PlannedVerifierID          string                              `json:"planned_verifier_id"`
	BuildManifestDigest        string                              `json:"build_manifest_digest"`
	ArtifactDigest             string                              `json:"artifact_digest"`
	RuntimeContractDigest      string                              `json:"runtime_contract_digest"`
	ExposurePolicy             map[string]any                      `json:"exposure_policy"`
	AudiencePolicy             map[string]any                      `json:"audience_policy"`
	RiskEnvelope               map[string]any                      `json:"risk_envelope"`
	Exclusions                 []string                            `json:"exclusions"`
	BudgetEnvelope             map[string]any                      `json:"budget_envelope"`
	RecoveryPolicy             map[string]any                      `json:"recovery_policy"`
	RecoveryPolicyRevision     int64                               `json:"recovery_policy_revision"`
	RollbackTargetReleaseID    string                              `json:"rollback_target_release_id"`
	KnownGoodCandidateID       string                              `json:"known_good_candidate_id,omitempty"`
	KnownGoodCandidateRevision int64                               `json:"known_good_candidate_revision,omitempty"`
	IrreversibleEffects        []string                            `json:"irreversible_effects"`
	Expected                   map[string]workflowstatev2.Revision `json:"expected"`
}

type knownGoodRecoverySnapshot struct {
	release   db.IssueWorkflowRelease
	candidate db.IssueWorkflowCandidate
	review    db.IssueWorkflowReview
}

func (s Service) loadKnownGoodRecoverySnapshot(ctx context.Context, write *workflowstore.IssueWrite, workspaceID, admittedIssueID pgtype.UUID, targetKey string, generation int64, releaseID pgtype.UUID) (knownGoodRecoverySnapshot, error) {
	release, err := write.GetWorkspaceReleaseByTargetGeneration(ctx, workspaceID, admittedIssueID, targetKey, generation)
	if err != nil || release.ID != releaseID || (release.State != "live" && release.State != "restored") || release.ReleaseKind == "recovery" && release.State != "restored" || !release.CandidateID.Valid || release.CandidateRevision < 1 || !release.BuildManifestDigest.Valid || !release.ArtifactDigest.Valid || !release.RuntimeContractDigest.Valid || !release.ReviewedReviewID.Valid || release.ReviewedReviewRevision.Int64 != 1 || !release.PlannedVerifierID.Valid || !release.ReleaseOperatorID.Valid || release.PlannedVerifierID == release.ReleaseOperatorID {
		return knownGoodRecoverySnapshot{}, ErrInvalidAuthoritativeState
	}
	candidate, err := write.GetWorkspaceCandidate(ctx, workspaceID, admittedIssueID, release.CandidateID)
	if err != nil || candidate.IssueID != release.IssueID || candidate.Revision != release.CandidateRevision || candidate.ScopeID != release.ScopeID || candidate.ScopeRevision != release.ScopeRevision || candidate.SupersededAt.Valid {
		return knownGoodRecoverySnapshot{}, ErrInvalidAuthoritativeState
	}
	review, err := write.GetWorkspaceReview(ctx, workspaceID, admittedIssueID, release.ReviewedReviewID)
	if err != nil || review.IssueID != release.IssueID || review.CandidateID != release.CandidateID || review.CandidateRevision != release.CandidateRevision || review.ScopeID != release.ScopeID || review.ScopeRevision != release.ScopeRevision || review.ReviewerID != release.PlannedVerifierID || review.Outcome != "passed" || review.SupersededAt.Valid {
		return knownGoodRecoverySnapshot{}, ErrInvalidAuthoritativeState
	}
	evidence, err := write.ListWorkspaceReleaseEvidence(ctx, workspaceID, admittedIssueID, release.ID)
	if err != nil || !knownGoodEvidenceComplete(release, evidence, s.Now().UTC()) {
		return knownGoodRecoverySnapshot{}, ErrInvalidAuthoritativeState
	}
	return knownGoodRecoverySnapshot{release: release, candidate: candidate, review: review}, nil
}

func knownGoodEvidenceComplete(release db.IssueWorkflowRelease, evidence []db.IssueWorkflowEvidence, now time.Time) bool {
	proofs := map[string]bool{"runtime": false, "business": false, "regression": false}
	for _, row := range evidence {
		if _, wanted := proofs[row.EvidenceType]; !wanted || proofs[row.EvidenceType] || row.IssueID != release.IssueID || row.ConcernID != release.ID || !row.ConcernRevision.Valid || row.ConcernRevision.Int64 != release.Revision || row.ProducerType != "verifier" || row.ProducerID != release.PlannedVerifierID || row.CandidateID != release.CandidateID || !row.TargetKey.Valid || row.TargetKey.String != release.TargetKey || row.ValidUntil.Valid && !row.ValidUntil.Time.After(now) {
			continue
		}
		var payload struct {
			Freshness         string `json:"freshness"`
			ReleaseID         string `json:"release_id"`
			CandidateRevision int64  `json:"candidate_revision"`
			ScopeID           string `json:"scope_id"`
			ScopeRevision     int64  `json:"scope_revision"`
			AcceptanceVersion int64  `json:"acceptance_version"`
			TargetGeneration  int64  `json:"target_generation"`
		}
		if json.Unmarshal(row.Payload, &payload) == nil && payload.Freshness == "current" && payload.ReleaseID == util.UUIDToString(release.ID) && payload.CandidateRevision == release.CandidateRevision && payload.ScopeID == util.UUIDToString(release.ScopeID) && payload.ScopeRevision == release.ScopeRevision && payload.AcceptanceVersion == release.AcceptanceVersion.Int64 && payload.TargetGeneration == release.TargetGeneration {
			proofs[row.EvidenceType] = true
		}
	}
	if !proofs["runtime"] || !proofs["business"] || !proofs["regression"] {
		return false
	}
	return true
}

// Request is the caller-controlled semantic input. Actor identity and role are
// derived after admission from Principal and normalized bindings.
type Request struct {
	WorkspaceID              pgtype.UUID                        `json:"workspace_id"`
	IssueID                  pgtype.UUID                        `json:"issue_id"`
	CommandKey               string                             `json:"command_key"`
	Kind                     workflowstatev2.CommandKind        `json:"kind"`
	Expected                 []workflowstatev2.ExpectedRevision `json:"expected"`
	OperationID              string                             `json:"operation_id,omitempty"`
	BranchID                 string                             `json:"branch_id,omitempty"`
	DecisionID               string                             `json:"decision_id,omitempty"`
	Decision                 *DecisionInput                     `json:"decision,omitempty"`
	Correction               *CorrectionResultInput             `json:"correction,omitempty"`
	ReleaseAdmission         *ReleaseAdmissionInput             `json:"release_admission,omitempty"`
	ReleaseBuild             *ReleaseBuildResultInput           `json:"release_build,omitempty"`
	ReleaseEffect            *ReleaseEffectResultInput          `json:"release_effect,omitempty"`
	DecisionOutcome          workflowstatev2.DecisionOutcome    `json:"decision_outcome,omitempty"`
	ExpectedDecisionRevision workflowstatev2.Revision           `json:"expected_decision_revision,omitempty"`
	CandidateID              string                             `json:"candidate_id,omitempty"`
	TargetID                 string                             `json:"target_id,omitempty"`
	EvidenceRefs             []string                           `json:"evidence_refs,omitempty"`
	CriteriaRefs             []string                           `json:"criteria_refs,omitempty"`
	CoverageRefs             []string                           `json:"coverage_refs,omitempty"`
	GapRefs                  []string                           `json:"gap_refs,omitempty"`
	RequiredCorrection       string                             `json:"required_correction,omitempty"`
	CorrectionCount          uint32                             `json:"correction_count,omitempty"`
	MaxCorrections           uint32                             `json:"max_corrections,omitempty"`
	Detail                   string                             `json:"detail,omitempty"`
}

type Receipt struct {
	State                  string                           `json:"state"`
	CommandID              string                           `json:"command_id"`
	Fingerprint            string                           `json:"fingerprint"`
	Revisions              workflowstatev2.ConcernRevisions `json:"revisions"`
	ActionIDs              []string                         `json:"action_ids"`
	ActionState            string                           `json:"action_state"`
	CurrentPictureRevision int64                            `json:"-"`
}

type Service struct {
	Starter            workflowstore.TxStarter
	Queries            *db.Queries
	Loader             Loader
	Now                func() time.Time
	Wake               func(context.Context, []string) error
	OnWakeError        func(error)
	OnProjection       func(context.Context, pgtype.UUID, pgtype.UUID, int64)
	OnProjectionStored func(context.Context, *workflowstore.IssueWrite, pgtype.UUID, pgtype.UUID, db.IssueCurrentPicture) error
	ControllerID       pgtype.UUID
	// Failpoint is test-only transaction instrumentation. Returning an error at
	// any stage aborts the enclosing issue write transaction.
	Failpoint func(stage string) error
}

type admittedAction struct {
	ID, Key, Type, OperationID, DecisionID, ReleaseID, TargetKey string
	Payload                                                      []byte
}

type applyState struct {
	receipt Receipt
	replay  bool
}

func (s Service) Execute(ctx context.Context, principal Principal, request Request) (Receipt, error) {
	if s.Starter == nil || s.Queries == nil || !principal.ActorID.Valid || !request.WorkspaceID.Valid || !request.IssueID.Valid || request.CommandKey == "" {
		return Receipt{}, fmt.Errorf("invalid workflow command service input")
	}
	if s.Now == nil {
		s.Now = time.Now
	}
	// Accepted release commands replay from their immutable command receipt
	// before consulting mutable agent/runtime authority. Archival or runtime
	// rotation after commit must not make an exact replay disappear or restart
	// the promised effect.
	if isReleaseCommand(request.Kind) {
		if receipt, found, err := s.releaseReplay(ctx, principal, request); found || err != nil {
			return receipt, err
		}
	}
	state := applyState{}
	apply := func(write *workflowstore.IssueWrite) error {
		return s.executeInWrite(ctx, write, principal, request, &state)
	}
	var err error
	if request.Kind == workflowstatev2.CommandPrepareRelease || request.Kind == workflowstatev2.CommandBuildRelease || request.Kind == workflowstatev2.CommandAuthorizeRelease {
		if request.ReleaseAdmission == nil {
			return Receipt{}, fmt.Errorf("release command requires exact actor admission snapshot")
		}
		err = workflowstore.InReleaseAdmissionTransaction(ctx, s.Starter, s.Queries, request.WorkspaceID, request.IssueID, []pgtype.UUID{request.ReleaseAdmission.ReleaseOperatorID, request.ReleaseAdmission.PlannedVerifierID}, apply)
	} else {
		err = workflowstore.InIssueWriteTransaction(ctx, s.Starter, s.Queries, request.WorkspaceID, request.IssueID, apply)
	}
	if err != nil {
		return Receipt{}, err
	}
	if !state.replay && s.Wake != nil && len(state.receipt.ActionIDs) != 0 {
		if err := s.Wake(ctx, append([]string(nil), state.receipt.ActionIDs...)); err != nil {
			if s.OnWakeError != nil {
				s.OnWakeError(err)
			}
		}
	}
	if !state.replay && s.OnProjection != nil {
		s.OnProjection(ctx, request.WorkspaceID, request.IssueID, state.receipt.CurrentPictureRevision)
	}
	return state.receipt, nil
}

func isReleaseCommand(kind workflowstatev2.CommandKind) bool {
	switch kind {
	case workflowstatev2.CommandPrepareRelease, workflowstatev2.CommandBuildRelease, workflowstatev2.CommandAuthorizeRelease,
		workflowstatev2.CommandStartDeployment, workflowstatev2.CommandRecordDeployment, workflowstatev2.CommandRecordExposure,
		workflowstatev2.CommandBeginVerification, workflowstatev2.CommandVerifyRuntime, workflowstatev2.CommandVerifyBusiness,
		workflowstatev2.CommandProductionFailed, workflowstatev2.CommandRollbackSucceeded, workflowstatev2.CommandRestoreVerified:
		return true
	default:
		return false
	}
}

func (s Service) releaseReplay(ctx context.Context, principal Principal, request Request) (Receipt, bool, error) {
	existing, err := s.Queries.GetIssueWorkflowCommandByKey(ctx, db.GetIssueWorkflowCommandByKeyParams{WorkspaceID: request.WorkspaceID, IssueID: request.IssueID, CommandKey: request.CommandKey})
	if errors.Is(err, pgx.ErrNoRows) {
		return Receipt{}, false, nil
	}
	if err != nil {
		return Receipt{}, true, err
	}
	role, err := s.replayRole(principal, request)
	if err != nil {
		return Receipt{}, true, err
	}
	if _, err = replayFingerprint(existing.Payload, principal, request, role); err != nil || existing.ActorID != principal.ActorID || existing.ActorType != principal.ActorType || existing.IssueID != request.IssueID {
		if err != nil {
			return Receipt{}, true, err
		}
		return Receipt{}, true, ErrIdempotencyConflict
	}
	if existing.State != "processed" {
		return Receipt{}, true, ErrRecoveryRequired
	}
	var receipt Receipt
	if err := json.Unmarshal(existing.Result, &receipt); err != nil {
		return Receipt{}, true, fmt.Errorf("decode stored workflow command receipt: %w", err)
	}
	return receipt, true, nil
}

// ExecuteInWrite applies one command through an already-admitted issue write.
// It never starts or commits a transaction and never emits wake hints; those
// responsibilities remain with the owner of the admitted callback.
func (s Service) ExecuteInWrite(ctx context.Context, write *workflowstore.IssueWrite, principal Principal, request Request) (Receipt, error) {
	if write == nil {
		return Receipt{}, fmt.Errorf("workflow command requires the exact active issue capability: %w", workflowstore.ErrInvalidIssueWriteCapability)
	}
	state := applyState{}
	err := write.WithIssueOperation(ctx, request.WorkspaceID, request.IssueID, func(leasedCtx context.Context) error {
		if !principal.ActorID.Valid || !request.WorkspaceID.Valid || !request.IssueID.Valid || request.CommandKey == "" {
			return fmt.Errorf("invalid admitted workflow command input")
		}
		if s.Now == nil {
			s.Now = time.Now
		}
		return s.executeInWrite(leasedCtx, write, principal, request, &state)
	})
	if err != nil {
		return Receipt{}, err
	}
	return state.receipt, nil
}

// NotifyProjection publishes a committed Current Picture revision. It exists
// for transaction owners that use ExecuteInWrite; they must call it only after
// their outer transaction commits.
func (s Service) NotifyProjection(ctx context.Context, workspaceID, issueID pgtype.UUID, receipt Receipt) {
	if s.OnProjection != nil && receipt.CurrentPictureRevision > 0 {
		s.OnProjection(ctx, workspaceID, issueID, receipt.CurrentPictureRevision)
	}
}

// ExecuteBatch applies a same-issue expansion under one admission lock and one
// database transaction. It is intentionally not wired to the legacy handler;
// an expansion that requires multiple logical principals must remain
// unsupported rather than weakening role derivation.
func (s Service) ExecuteBatch(ctx context.Context, principal Principal, requests []Request) ([]Receipt, error) {
	if len(requests) == 0 {
		return nil, fmt.Errorf("workflow command batch is empty")
	}
	first := requests[0]
	if s.Starter == nil || s.Queries == nil || !principal.ActorID.Valid || !first.WorkspaceID.Valid || !first.IssueID.Valid {
		return nil, fmt.Errorf("invalid workflow command batch input")
	}
	if s.Now == nil {
		s.Now = time.Now
	}
	states := make([]applyState, len(requests))
	err := workflowstore.InIssueWriteTransaction(ctx, s.Starter, s.Queries, first.WorkspaceID, first.IssueID, func(write *workflowstore.IssueWrite) error {
		for index, request := range requests {
			if request.WorkspaceID != first.WorkspaceID || request.IssueID != first.IssueID {
				return fmt.Errorf("workflow command batch crosses issue admission boundary")
			}
			if err := s.executeInWrite(ctx, write, principal, request, &states[index]); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	receipts := make([]Receipt, len(states))
	wakeIDs := []string{}
	for index, state := range states {
		receipts[index] = state.receipt
		if !state.replay {
			wakeIDs = append(wakeIDs, state.receipt.ActionIDs...)
		}
	}
	if s.OnProjection != nil {
		for index, state := range states {
			if !state.replay {
				s.OnProjection(ctx, requests[index].WorkspaceID, requests[index].IssueID, state.receipt.CurrentPictureRevision)
			}
		}
	}
	if s.Wake != nil && len(wakeIDs) != 0 {
		if err := s.Wake(ctx, wakeIDs); err != nil && s.OnWakeError != nil {
			s.OnWakeError(err)
		}
	}
	return receipts, nil
}

func (s Service) executeInWrite(ctx context.Context, write *workflowstore.IssueWrite, principal Principal, request Request, state *applyState) error {
	picture, err := s.Loader.Load(ctx, write, request.WorkspaceID, request.IssueID)
	if err != nil {
		return fmt.Errorf("load authoritative workflow state: %w", err)
	}
	existing, lookupErr := write.GetCommandByKey(ctx, request.WorkspaceID, request.IssueID, request.CommandKey)
	if lookupErr == nil {
		role, err := s.replayRole(principal, request)
		if err != nil {
			return err
		}
		_, err = replayFingerprint(existing.Payload, principal, request, role)
		if err != nil {
			return err
		}
		hash := existing.RequestHash
		if hash != existing.RequestHash || existing.IssueID != request.IssueID || existing.ActorID != principal.ActorID || existing.ActorType != principal.ActorType {
			return ErrIdempotencyConflict
		}
		if existing.State != "processed" {
			return ErrRecoveryRequired
		}
		if err := json.Unmarshal(existing.Result, &state.receipt); err != nil {
			return fmt.Errorf("decode stored workflow command receipt: %w", err)
		}
		state.replay = true
		return nil
	}
	if !errors.Is(lookupErr, pgx.ErrNoRows) {
		return lookupErr
	}
	role, command, err := s.deriveCommand(ctx, write, principal, request, picture)
	if err != nil {
		return fmt.Errorf("derive workflow command: %w", err)
	}
	if err := s.fail("after_authority"); err != nil {
		return err
	}
	command.ActorRole, command.ActorID = role, util.UUIDToString(principal.ActorID)
	commandID := stableUUID(request.WorkspaceID, request.IssueID, request.CommandKey, "command")
	command.CommandID = commandID.String()
	command.IssueID = util.UUIDToString(request.IssueID)
	fingerprint, payload, expectedJSON, err := semanticFingerprint(principal, request, command)
	if err != nil {
		return err
	}

	created, err := write.CreateCommand(ctx, db.CreateIssueWorkflowCommandParams{
		ID: pgUUID(commandID), WorkspaceID: request.WorkspaceID, IssueID: request.IssueID,
		CommandKey: request.CommandKey, RequestHash: fingerprint, CommandType: string(request.Kind),
		ActorType: principal.ActorType, ActorID: principal.ActorID, ScopeRevision: nullableInt64(int64(picture.Scope.Revision)),
		AuthorityRevision: nullableInt64(int64(picture.Authority.Revision)), ExpectedRevisions: expectedJSON, Payload: payload,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrIdempotencyConflict
		}
		return err
	}
	if created.State != "received" {
		return ErrRecoveryRequired
	}

	applyInput := workflowstatev2.ApplyInput{Picture: picture, Command: command}
	if isReleaseCommand(request.Kind) {
		generation := command.TargetGeneration
		applyInput.AuthoritativeTargetGeneration = &generation
	}
	result, err := workflowstatev2.Apply(applyInput)
	if err != nil {
		return err
	}
	mutated := changedConcerns(picture.Revisions, result.Picture.Revisions)
	actions, sources, err := s.persist(ctx, write, request, principal, commandID, picture, result.Picture, mutated, fingerprint)
	if err != nil {
		return fmt.Errorf("persist workflow command: %w", err)
	}
	if err := advanceHeads(ctx, write, request, picture, result.Picture, mutated, sources); err != nil {
		return err
	}
	if err := s.fail("after_heads"); err != nil {
		return err
	}
	reloaded, err := s.Loader.Load(ctx, write, request.WorkspaceID, request.IssueID)
	if err != nil {
		return fmt.Errorf("reload persisted workflow state: %w", err)
	}
	expectedPicture := result.Picture
	expectedPicture.LastCommandID, reloaded.LastCommandID = "", ""
	canonicalizePicture(&expectedPicture)
	canonicalizePicture(&reloaded)
	if !reflect.DeepEqual(expectedPicture, reloaded) {
		want, _ := json.Marshal(expectedPicture)
		got, _ := json.Marshal(reloaded)
		return fmt.Errorf("%w: reducer=%s reloaded=%s", ErrPersistenceDivergence, want, got)
	}

	projectionExpected, projectionRevision := int64(0), int64(1)
	current, err := write.GetCurrentPicture(ctx, db.GetIssueCurrentPictureParams{WorkspaceID: request.WorkspaceID, IssueID: request.IssueID})
	if err == nil {
		projectionExpected, projectionRevision = current.Revision, current.Revision+1
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	storedPicture, err := workflowprojection.ProjectInWrite(ctx, write, request.WorkspaceID, request.IssueID, projectionExpected, projectionRevision, s.Now())
	if err != nil {
		return err
	}
	if s.OnProjectionStored != nil {
		if err := s.OnProjectionStored(ctx, write, request.WorkspaceID, request.IssueID, storedPicture); err != nil {
			return fmt.Errorf("store owner notification intent: %w", err)
		}
	}
	if err := s.fail("after_projection"); err != nil {
		return err
	}
	actionIDs := make([]string, len(actions))
	for i := range actions {
		actionIDs[i] = actions[i].ID
	}
	actionState := "none"
	if len(actionIDs) != 0 {
		actionState = "admitted"
	}
	state.receipt = Receipt{State: "applied", CommandID: command.CommandID, Fingerprint: fingerprint, Revisions: result.Picture.Revisions, ActionIDs: actionIDs, ActionState: actionState, CurrentPictureRevision: projectionRevision}
	receiptJSON, _ := json.Marshal(state.receipt)
	processedAt := pgtype.Timestamptz{Time: s.Now().UTC(), Valid: true}
	if _, err := write.SetCommandResult(ctx, db.SetIssueWorkflowCommandResultParams{State: "processed", Result: receiptJSON, Error: "", ProcessedAt: processedAt, WorkspaceID: request.WorkspaceID, IssueID: request.IssueID, ID: pgUUID(commandID), ExpectedState: "received"}); err != nil {
		return err
	}
	return s.fail("after_receipt")
}

func (s Service) fail(stage string) error {
	if s.Failpoint == nil {
		return nil
	}
	return s.Failpoint(stage)
}

func hashCanonical(value any) string {
	payload, _ := json.Marshal(value)
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func validReleaseBuildCoverage(coverage map[string]any) bool {
	for _, key := range []string{"source_tree", "assets", "migrations", "config", "runtime_contract", "artifact", "build_manifest", "regression_review_evidence_id"} {
		if !meaningfulValue(coverage[key]) {
			return false
		}
	}
	return true
}

func (s Service) validateRegressionReviewEvidence(ctx context.Context, write *workflowstore.IssueWrite, r Request, review db.IssueWorkflowReview, candidateID pgtype.UUID, coverage map[string]any) error {
	ref, ok := coverage["regression_review_evidence_id"].(string)
	ref = strings.TrimSpace(ref)
	evidenceID := mustUUID(ref)
	if !ok || !evidenceID.Valid {
		return ErrInvalidAuthoritativeState
	}
	var refs []string
	if json.Unmarshal(review.EvidenceRefs, &refs) != nil || !slices.Contains(refs, ref) {
		return ErrForbidden
	}
	candidate, err := write.GetCandidate(ctx, db.GetIssueWorkflowCandidateParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: candidateID})
	if err != nil {
		return ErrInvalidAuthoritativeState
	}
	evidence, err := write.LockEvidenceRecord(ctx, db.LockIssueWorkflowEvidenceRecordParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: evidenceID})
	if err != nil || validateCompletionReviewEvidence(evidence, review, candidate, s.Now().UTC()) != nil {
		return ErrForbidden
	}
	return nil
}

func (s Service) validateReleaseBuildEvidence(ctx context.Context, write *workflowstore.IssueWrite, r Request, p workflowstatev2.CurrentPicture) error {
	head, err := concernHead(ctx, write, r, workflowstatev2.ConcernEvidence)
	if err != nil || head.State != "current" {
		return ErrInvalidAuthoritativeState
	}
	rows, err := write.ListEvidence(ctx, db.ListIssueWorkflowEvidenceParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID})
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.ID != head.SourceRecordID || row.EvidenceType != "reproducible_build" || row.ProducerType != "release_operator" || util.UUIDToString(row.ProducerID) != p.Release.ReleaseOperatorID || row.CandidateID != mustUUID(p.Release.CandidateID) || !row.TargetKey.Valid || row.TargetKey.String != p.Release.TargetID || row.SupersededAt.Valid || row.ValidUntil.Valid && !row.ValidUntil.Time.After(s.Now().UTC()) {
			continue
		}
		var payload struct {
			Freshness             string         `json:"freshness"`
			ReleaseID             string         `json:"release_id"`
			BuildManifestDigest   string         `json:"build_manifest_digest"`
			ArtifactDigest        string         `json:"artifact_digest"`
			RuntimeContractDigest string         `json:"runtime_contract_digest"`
			CandidateRevision     int64          `json:"candidate_revision"`
			TargetGeneration      int64          `json:"target_generation"`
			ScopeRevision         int64          `json:"scope_revision"`
			AcceptanceVersion     int64          `json:"acceptance_version"`
			Reproducible          bool           `json:"reproducible"`
			Coverage              map[string]any `json:"coverage"`
		}
		if json.Unmarshal(row.Payload, &payload) == nil && payload.Freshness == "current" && payload.Reproducible && row.ConcernID.Valid && payload.ReleaseID == util.UUIDToString(row.ConcernID) && payload.CandidateRevision == int64(p.Release.CandidateRevision) && payload.TargetGeneration == int64(p.Release.Generation) && payload.ScopeRevision == int64(p.Scope.Revision) && payload.AcceptanceVersion == int64(p.Scope.AcceptanceVersion) && payload.BuildManifestDigest == p.Release.BuildManifestDigest && payload.ArtifactDigest == p.Release.ArtifactDigest && payload.RuntimeContractDigest == p.Release.RuntimeContractDigest && validReleaseBuildCoverage(payload.Coverage) {
			return nil
		}
	}
	return ErrInvalidAuthoritativeState
}

func (s Service) releaseAuthority(ctx context.Context, write *workflowstore.IssueWrite, r Request, p workflowstatev2.CurrentPicture) (deploymentAuthorityEnvelope, db.IssueWorkflowAuthority, db.IssueWorkflowReview, string, error) {
	var empty deploymentAuthorityEnvelope
	concerns := []workflowstatev2.Concern{workflowstatev2.ConcernAuthority, workflowstatev2.ConcernEvidence, workflowstatev2.ConcernExecution, workflowstatev2.ConcernRelease, workflowstatev2.ConcernReview, workflowstatev2.ConcernScope}
	heads := make(map[workflowstatev2.Concern]db.IssueWorkflowConcernHead, len(concerns))
	for _, concern := range concerns {
		head, err := write.LockConcernHead(ctx, db.LockIssueWorkflowConcernHeadParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ConcernType: string(concern)})
		if err != nil || head.Revision != int64(p.Revisions.Get(concern)) {
			return empty, db.IssueWorkflowAuthority{}, db.IssueWorkflowReview{}, "", ErrInvalidAuthoritativeState
		}
		heads[concern] = head
	}
	authorityHead := heads[workflowstatev2.ConcernAuthority]
	if authorityHead.State != "active" {
		return empty, db.IssueWorkflowAuthority{}, db.IssueWorkflowReview{}, "", ErrForbidden
	}
	sources, err := write.LockReleaseSnapshotSources(ctx, r.WorkspaceID, r.IssueID,
		heads[workflowstatev2.ConcernScope].SourceRecordID,
		authorityHead.SourceRecordID,
		heads[workflowstatev2.ConcernExecution].SourceRecordID,
		heads[workflowstatev2.ConcernReview].SourceRecordID,
		mustUUID(p.Review.CandidateID),
	)
	if err != nil {
		return empty, db.IssueWorkflowAuthority{}, db.IssueWorkflowReview{}, "", ErrInvalidAuthoritativeState
	}
	authority := sources.Authority
	if err != nil || authority.Revision != authorityHead.SourceRecordRevision || authority.State != "active" || authority.RevokedAt.Valid || authority.SupersededAt.Valid || authority.ValidUntil.Valid && !authority.ValidUntil.Time.After(s.Now().UTC()) {
		return empty, db.IssueWorkflowAuthority{}, db.IssueWorkflowReview{}, "", ErrForbidden
	}
	if sources.Scope.Revision != heads[workflowstatev2.ConcernScope].SourceRecordRevision || sources.Execution.Revision != heads[workflowstatev2.ConcernExecution].SourceRecordRevision || sources.Candidate.Revision != int64(p.Release.CandidateRevision) || sources.Candidate.SupersededAt.Valid {
		return empty, db.IssueWorkflowAuthority{}, db.IssueWorkflowReview{}, "", ErrInvalidAuthoritativeState
	}
	decoder := json.NewDecoder(bytes.NewReader(authority.Envelope))
	decoder.DisallowUnknownFields()
	var envelope deploymentAuthorityEnvelope
	if decoder.Decode(&envelope) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return empty, db.IssueWorkflowAuthority{}, db.IssueWorkflowReview{}, "", ErrInvalidAuthoritativeState
	}
	reviewHead := heads[workflowstatev2.ConcernReview]
	if reviewHead.State != "passed" {
		return empty, db.IssueWorkflowAuthority{}, db.IssueWorkflowReview{}, "", ErrInvalidAuthoritativeState
	}
	review := sources.Review
	if err != nil || reviewHead.SourceRecordRevision != 1 || review.Outcome != "passed" {
		return empty, db.IssueWorkflowAuthority{}, db.IssueWorkflowReview{}, "", ErrInvalidAuthoritativeState
	}
	if envelope.Version != 1 || envelope.Capability != "deploy_exact_release" || envelope.CandidateID != p.Review.CandidateID || envelope.CandidateRevision != review.CandidateRevision || envelope.ScopeID != util.UUIDToString(review.ScopeID) || envelope.ScopeRevision != review.ScopeRevision || envelope.AcceptanceVersion != int64(p.Scope.AcceptanceVersion) || envelope.OperationID != p.Execution.OperationID || envelope.BranchKey == "" || envelope.TargetKey == "" || envelope.TargetID != envelope.TargetKey || envelope.DesiredTargetGeneration != envelope.ExpectedTargetGeneration+1 || envelope.DesiredTargetGeneration < 1 || envelope.ReleaseOperatorID == envelope.PlannedVerifierID || !mustUUID(envelope.ReleaseOperatorID).Valid || !mustUUID(envelope.PlannedVerifierID).Valid || envelope.OwnerEvidenceID != util.UUIDToString(authority.OwnerEvidenceID) || envelope.OwnerEvidenceRevision < 1 || envelope.RecoveryPolicyRevision < 1 || !meaningfulObject(envelope.ExposurePolicy) || !meaningfulObject(envelope.AudiencePolicy) || !meaningfulObject(envelope.RiskEnvelope) || envelope.Exclusions == nil || !meaningfulObject(envelope.BudgetEnvelope) || !meaningfulObject(envelope.RecoveryPolicy) || strings.TrimSpace(envelope.BuildManifestDigest) == "" || strings.TrimSpace(envelope.ArtifactDigest) == "" || strings.TrimSpace(envelope.RuntimeContractDigest) == "" || envelope.ReviewedReviewID != util.UUIDToString(review.ID) || envelope.ReviewedReviewRevision != 1 {
		return empty, db.IssueWorkflowAuthority{}, db.IssueWorkflowReview{}, "", ErrInvalidAuthoritativeState
	}
	if envelope.ExpectedTargetGeneration == 0 && envelope.ExpectedTargetReleaseID != "" || envelope.ExpectedTargetGeneration > 0 && !mustUUID(envelope.ExpectedTargetReleaseID).Valid {
		return empty, db.IssueWorkflowAuthority{}, db.IssueWorkflowReview{}, "", ErrInvalidAuthoritativeState
	}
	seen := map[string]bool{}
	for _, effect := range envelope.IrreversibleEffects {
		if strings.TrimSpace(effect) == "" || seen[effect] {
			return empty, db.IssueWorkflowAuthority{}, db.IssueWorkflowReview{}, "", ErrInvalidAuthoritativeState
		}
		seen[effect] = true
	}
	var canonicalEnvelope any
	if json.Unmarshal(authority.Envelope, &canonicalEnvelope) != nil {
		return empty, db.IssueWorkflowAuthority{}, db.IssueWorkflowReview{}, "", ErrInvalidAuthoritativeState
	}
	digest := hashCanonical(canonicalEnvelope)
	ownerEvidence, err := write.LockEvidenceRecord(ctx, db.LockIssueWorkflowEvidenceRecordParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: mustUUID(envelope.OwnerEvidenceID)})
	if err != nil || ownerEvidence.EvidenceType != "deployment_authorization" || ownerEvidence.ConcernType != "authority" || ownerEvidence.ConcernID != authority.ID || !ownerEvidence.ConcernRevision.Valid || ownerEvidence.ConcernRevision.Int64 != authority.Revision || ownerEvidence.ProducerType != "owner" || ownerEvidence.ProducerID != authority.CreatedBy || ownerEvidence.CandidateID != sources.Candidate.ID || !ownerEvidence.TargetKey.Valid || ownerEvidence.TargetKey.String != envelope.TargetKey || ownerEvidence.SupersededAt.Valid || ownerEvidence.ValidUntil.Valid && !ownerEvidence.ValidUntil.Time.After(s.Now().UTC()) {
		return empty, db.IssueWorkflowAuthority{}, db.IssueWorkflowReview{}, "", ErrForbidden
	}
	ownerOK, err := write.LockOwnerAuthority(ctx, r.WorkspaceID, r.IssueID, authority.CreatedBy)
	if err != nil || !ownerOK {
		return empty, db.IssueWorkflowAuthority{}, db.IssueWorkflowReview{}, "", ErrForbidden
	}
	var ownerPayload struct {
		Revision          int64  `json:"revision"`
		Freshness         string `json:"freshness"`
		AuthorityID       string `json:"authority_id"`
		AuthorityRevision int64  `json:"authority_revision"`
		EnvelopeDigest    string `json:"envelope_digest"`
		Capability        string `json:"capability"`
		CandidateID       string `json:"candidate_id"`
		CandidateRevision int64  `json:"candidate_revision"`
		ScopeID           string `json:"scope_id"`
		ScopeRevision     int64  `json:"scope_revision"`
		AcceptanceVersion int64  `json:"acceptance_version"`
		OperationID       string `json:"operation_id"`
		BranchKey         string `json:"branch_key"`
		TargetKey         string `json:"target_key"`
		TargetGeneration  int64  `json:"target_generation"`
	}
	if json.Unmarshal(ownerEvidence.Payload, &ownerPayload) != nil || ownerPayload.Revision != envelope.OwnerEvidenceRevision || ownerPayload.Freshness != "current" || ownerPayload.AuthorityID != util.UUIDToString(authority.ID) || ownerPayload.AuthorityRevision != authority.Revision || ownerPayload.EnvelopeDigest != digest || ownerPayload.Capability != envelope.Capability || ownerPayload.CandidateID != envelope.CandidateID || ownerPayload.CandidateRevision != envelope.CandidateRevision || ownerPayload.ScopeID != envelope.ScopeID || ownerPayload.ScopeRevision != envelope.ScopeRevision || ownerPayload.AcceptanceVersion != envelope.AcceptanceVersion || ownerPayload.OperationID != envelope.OperationID || ownerPayload.BranchKey != envelope.BranchKey || ownerPayload.TargetKey != envelope.TargetKey || ownerPayload.TargetGeneration != envelope.DesiredTargetGeneration {
		return empty, db.IssueWorkflowAuthority{}, db.IssueWorkflowReview{}, "", ErrForbidden
	}
	var reviewEvidenceRefs []string
	if json.Unmarshal(review.EvidenceRefs, &reviewEvidenceRefs) != nil || len(reviewEvidenceRefs) == 0 {
		return empty, db.IssueWorkflowAuthority{}, db.IssueWorkflowReview{}, "", ErrInvalidAuthoritativeState
	}
	for _, ref := range reviewEvidenceRefs {
		row, lockErr := write.LockEvidenceRecord(ctx, db.LockIssueWorkflowEvidenceRecordParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: mustUUID(ref)})
		if lockErr != nil || validateCompletionReviewEvidence(row, review, sources.Candidate, s.Now().UTC()) != nil {
			return empty, db.IssueWorkflowAuthority{}, db.IssueWorkflowReview{}, "", ErrForbidden
		}
	}
	evidenceHead := heads[workflowstatev2.ConcernEvidence]
	if _, err := write.LockEvidenceRecord(ctx, db.LockIssueWorkflowEvidenceRecordParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: evidenceHead.SourceRecordID}); err != nil {
		return empty, db.IssueWorkflowAuthority{}, db.IssueWorkflowReview{}, "", ErrInvalidAuthoritativeState
	}
	return envelope, authority, review, digest, nil
}

func isReleaseEffectCommand(kind workflowstatev2.CommandKind) bool {
	switch kind {
	case workflowstatev2.CommandStartDeployment, workflowstatev2.CommandRecordDeployment, workflowstatev2.CommandRecordExposure,
		workflowstatev2.CommandBeginVerification, workflowstatev2.CommandVerifyRuntime, workflowstatev2.CommandVerifyBusiness,
		workflowstatev2.CommandProductionFailed, workflowstatev2.CommandRollbackSucceeded, workflowstatev2.CommandRestoreVerified:
		return true
	default:
		return false
	}
}

func releaseEffectActionType(kind workflowstatev2.CommandKind) (string, workflowstatev2.ActorRole, string) {
	switch kind {
	case workflowstatev2.CommandStartDeployment, workflowstatev2.CommandRecordDeployment:
		return "deploy_exact_release", workflowstatev2.RoleRelease, "deployment"
	case workflowstatev2.CommandRecordExposure:
		return "expose_exact_release", workflowstatev2.RoleRelease, "exposure"
	case workflowstatev2.CommandBeginVerification, workflowstatev2.CommandVerifyRuntime:
		return "verify_runtime_exact_release", workflowstatev2.RoleVerifier, "runtime"
	case workflowstatev2.CommandVerifyBusiness:
		return "verify_business_exact_release", workflowstatev2.RoleVerifier, "business"
	case workflowstatev2.CommandProductionFailed:
		return "monitor_exact_release", workflowstatev2.RoleVerifier, "regression"
	case workflowstatev2.CommandRollbackSucceeded:
		return "rollback_exact_release", workflowstatev2.RoleRelease, "rollback"
	case workflowstatev2.CommandRestoreVerified:
		return "verify_restoration_exact_release", workflowstatev2.RoleVerifier, "restoration"
	default:
		return "", "", ""
	}
}

func (s Service) releaseEffectAuthority(ctx context.Context, write *workflowstore.IssueWrite, principal Principal, r Request, p workflowstatev2.CurrentPicture) (workflowstatev2.ActorRole, releaseEffectSnapshot, error) {
	var frozen releaseEffectSnapshot
	actionType, role, resultType := releaseEffectActionType(r.Kind)
	if actionType == "" || !principal.ActionID.Valid || principal.ActionKey == "" || r.ReleaseEffect == nil {
		return "", frozen, ErrForbidden
	}
	action, err := write.GetActionByKey(ctx, r.WorkspaceID, r.IssueID, principal.ActionKey)
	if err != nil || action.ID != principal.ActionID || action.ActionType != actionType || action.OperationID != mustUUID(p.Execution.OperationID) || action.ReleaseID != mustUUID(r.ReleaseEffect.ReleaseID) || !action.TargetKey.Valid || action.TargetKey.String != p.Release.TargetID || !action.EffectKey.Valid || action.EffectKey.String != r.ReleaseEffect.EffectKey {
		return "", frozen, fmt.Errorf("%w: release action identity", ErrForbidden)
	}
	if principal.ActorType == "workflow_action" {
		if !principal.LeaseToken.Valid || r.CommandKey != "action/"+util.UUIDToString(principal.ActionID)+"/"+string(r.Kind) || action.State != "leased" || action.LeaseToken != principal.LeaseToken || !action.LeaseExpiresAt.Valid || !action.LeaseExpiresAt.Time.After(s.Now().UTC()) {
			return "", frozen, ErrForbidden
		}
	} else if principal.ActorType == "agent_task" {
		locked, lockErr := write.LockReleaseVerificationResultAuthority(ctx, db.LockWorkflowReleaseVerificationResultAuthorityParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, TaskID: principal.TaskID})
		if lockErr != nil || locked.WorkflowActionID != action.ID || locked.ActionKey != action.ActionKey || locked.VerifierID != principal.ActorID || action.State != "settled" || action.BoundTaskID != principal.TaskID {
			return "", frozen, fmt.Errorf("%w: release verifier task authority (%v)", ErrForbidden, lockErr)
		}
		allowedKey := "task/" + util.UUIDToString(principal.TaskID) + "/runtime-verification-result"
		if r.Kind == workflowstatev2.CommandBeginVerification {
			allowedKey = "task/" + util.UUIDToString(principal.TaskID) + "/begin-runtime-verification"
		} else if r.Kind == workflowstatev2.CommandVerifyBusiness {
			allowedKey = "task/" + util.UUIDToString(principal.TaskID) + "/business-verification-result"
		} else if r.Kind == workflowstatev2.CommandProductionFailed {
			allowedKey = "task/" + util.UUIDToString(principal.TaskID) + "/production-regression-result"
		} else if r.Kind == workflowstatev2.CommandRestoreVerified {
			allowedKey = "task/" + util.UUIDToString(principal.TaskID) + "/restoration-verification-result"
		}
		if r.CommandKey != allowedKey {
			return "", frozen, fmt.Errorf("%w: release verifier command key", ErrForbidden)
		}
	} else {
		return "", frozen, ErrForbidden
	}
	decoder := json.NewDecoder(bytes.NewReader(action.Payload))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&frozen) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return "", frozen, ErrInvalidAuthoritativeState
	}
	if actionType == "rollback_exact_release" && (frozen.ReleaseKind != "recovery" || frozen.RecoveryOfReleaseID == "" || frozen.RecoveryOfReleaseID != frozen.ExpectedTargetReleaseID) {
		return "", frozen, ErrInvalidAuthoritativeState
	}
	result := r.ReleaseEffect
	expectedActor := frozen.ReleaseOperatorID
	if role == workflowstatev2.RoleVerifier {
		expectedActor = frozen.PlannedVerifierID
	}
	if principal.ActorID != mustUUID(expectedActor) || frozen.ContractVersion != 1 || frozen.EffectContract != "observe_before_apply_v1" || frozen.ReleaseOperatorID == frozen.PlannedVerifierID {
		return "", frozen, fmt.Errorf("%w: release effect actor or contract", ErrForbidden)
	}
	if frozen.ReleaseID != result.ReleaseID || frozen.CandidateID != result.CandidateID || frozen.CandidateRevision != result.CandidateRevision || frozen.ScopeID != result.ScopeID || frozen.ScopeRevision != result.ScopeRevision || frozen.AcceptanceVersion != result.AcceptanceVersion || frozen.OperationID != result.OperationID || frozen.BranchKey != result.BranchKey || frozen.TargetKey != result.TargetKey || frozen.DesiredTargetGeneration != result.TargetGeneration || frozen.ArtifactDigest != result.ArtifactDigest || frozen.RuntimeContractDigest != result.RuntimeContractDigest || result.Type != resultType || strings.TrimSpace(result.ProviderReceiptID) == "" {
		return "", frozen, fmt.Errorf("%w: release observation identity", ErrForbidden)
	}
	if !validReleaseEffectEvidence(r.Kind, result.Evidence) {
		return "", frozen, fmt.Errorf("%w: release observation evidence", ErrForbidden)
	}
	if !exactExpected(r.Expected, p, mutationConcernsForReleaseEffect(r.Kind)...) {
		return "", frozen, fmt.Errorf("%w: release observation causal revisions", ErrForbidden)
	}
	if !write.MatchesReleaseAdmission(mustUUID(frozen.ReleaseOperatorID), mustUUID(frozen.PlannedVerifierID)) {
		return "", frozen, fmt.Errorf("%w: release admission capability", ErrForbidden)
	}
	releaseHead, err := concernHead(ctx, write, r, workflowstatev2.ConcernRelease)
	if err != nil || releaseHead.SourceRecordID != mustUUID(frozen.ReleaseID) {
		return "", frozen, fmt.Errorf("%w: release concern head mismatch (err=%v head=%s frozen=%s)", ErrInvalidAuthoritativeState, err, util.UUIDToString(releaseHead.SourceRecordID), frozen.ReleaseID)
	}
	release, err := write.GetRelease(ctx, db.GetIssueWorkflowReleaseParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: releaseHead.SourceRecordID})
	expectedReleaseRevision := frozen.ReleaseRevision
	if r.Kind == workflowstatev2.CommandRecordDeployment || r.Kind == workflowstatev2.CommandVerifyRuntime {
		expectedReleaseRevision++
	}
	if err != nil {
		return "", frozen, fmt.Errorf("%w: load release snapshot: %v", ErrInvalidAuthoritativeState, err)
	}
	if frozen.CommandHash != action.RequestHash || !frozenExpectedMatchesCurrent(frozen.Expected, p, r.Kind) || expectedReleaseRevision != release.Revision || release.CandidateID != mustUUID(frozen.CandidateID) || release.CandidateRevision != frozen.CandidateRevision || release.ScopeID != mustUUID(frozen.ScopeID) || release.ScopeRevision != frozen.ScopeRevision || release.AcceptanceVersion.Int64 != frozen.AcceptanceVersion || release.OperationID != mustUUID(frozen.OperationID) || release.BranchKey != frozen.BranchKey || release.TargetKey != frozen.TargetKey || release.TargetGeneration != frozen.DesiredTargetGeneration || util.UUIDToString(release.ExpectedTargetReleaseID) != frozen.ExpectedTargetReleaseID || release.ExpectedTargetGeneration.Int64 != frozen.ExpectedTargetGeneration || util.UUIDToString(release.ReleaseOperatorID) != frozen.ReleaseOperatorID || util.UUIDToString(release.PlannedVerifierID) != frozen.PlannedVerifierID || release.BuildManifestDigest.String != frozen.BuildManifestDigest || release.ArtifactDigest.String != frozen.ArtifactDigest || release.RuntimeContractDigest.String != frozen.RuntimeContractDigest || util.UUIDToString(release.ReviewedReviewID) != frozen.ReviewID || release.ReviewedReviewRevision.Int64 != frozen.ReviewRevision {
		return "", frozen, fmt.Errorf("%w: release snapshot mismatch (kind=%s row_revision=%d frozen_revision=%d row_expected=%s/%d frozen_expected=%s/%d)", ErrInvalidAuthoritativeState, r.Kind, release.Revision, expectedReleaseRevision, util.UUIDToString(release.ExpectedTargetReleaseID), release.ExpectedTargetGeneration.Int64, frozen.ExpectedTargetReleaseID, frozen.ExpectedTargetGeneration)
	}
	if r.Kind == workflowstatev2.CommandProductionFailed || r.Kind == workflowstatev2.CommandRollbackSucceeded || r.Kind == workflowstatev2.CommandRestoreVerified {
		if r.Kind == workflowstatev2.CommandProductionFailed && release.State != "live" || r.Kind == workflowstatev2.CommandRollbackSucceeded && release.State != "rolling_back" || r.Kind == workflowstatev2.CommandRestoreVerified && release.State != "rolled_back" {
			return "", frozen, fmt.Errorf("%w: release state %s is not valid for %s", ErrInvalidAuthoritativeState, release.State, r.Kind)
		}
		if err := write.LockReleaseTarget(ctx, r.WorkspaceID, r.IssueID, frozen.TargetKey); err != nil {
			return "", frozen, err
		}
		head, headErr := write.LockReleaseTargetHead(ctx, r.WorkspaceID, r.IssueID, frozen.TargetKey)
		headMatches := headErr == nil && head.Generation == frozen.DesiredTargetGeneration && head.ReleaseID == release.ID && head.CandidateID == release.CandidateID
		if r.Kind == workflowstatev2.CommandRollbackSucceeded {
			headMatches = headErr == nil && head.Generation == frozen.ExpectedTargetGeneration && util.UUIDToString(head.ReleaseID) == frozen.ExpectedTargetReleaseID
		}
		if !headMatches {
			return "", frozen, ErrForbidden
		}
		return role, frozen, nil
	}
	envelope, authority, _, digest, err := s.releaseAuthority(ctx, write, r, p)
	if err != nil || util.UUIDToString(authority.ID) != frozen.AuthorityID || authority.Revision != frozen.AuthorityRevision || digest != frozen.AuthorityEnvelopeDigest || envelope.Capability != frozen.AuthorityCapability || envelope.OwnerEvidenceID != frozen.AuthorityEvidenceID ||
		envelope.OwnerEvidenceRevision != frozen.OwnerEvidenceRevision || envelope.CandidateID != frozen.CandidateID || envelope.CandidateRevision != frozen.CandidateRevision || envelope.ScopeID != frozen.ScopeID || envelope.ScopeRevision != frozen.ScopeRevision || envelope.AcceptanceVersion != frozen.AcceptanceVersion || envelope.OperationID != frozen.OperationID || envelope.BranchKey != frozen.BranchKey || envelope.TargetKey != frozen.TargetKey || envelope.ExpectedTargetReleaseID != frozen.ExpectedTargetReleaseID || envelope.ExpectedTargetGeneration != frozen.ExpectedTargetGeneration || envelope.DesiredTargetGeneration != frozen.DesiredTargetGeneration || envelope.ReleaseOperatorID != frozen.ReleaseOperatorID || envelope.PlannedVerifierID != frozen.PlannedVerifierID || envelope.BuildManifestDigest != frozen.BuildManifestDigest || envelope.ArtifactDigest != frozen.ArtifactDigest || envelope.RuntimeContractDigest != frozen.RuntimeContractDigest || envelope.ReviewedReviewID != frozen.ReviewID || envelope.ReviewedReviewRevision != frozen.ReviewRevision || envelope.RecoveryPolicyRevision != frozen.RecoveryPolicyRevision || envelope.RollbackTargetReleaseID != frozen.RollbackTargetReleaseID || !reflect.DeepEqual(envelope.ExposurePolicy, frozen.ExposurePolicy) || !reflect.DeepEqual(envelope.AudiencePolicy, frozen.AudiencePolicy) || !reflect.DeepEqual(envelope.RiskEnvelope, frozen.RiskEnvelope) || !reflect.DeepEqual(envelope.Exclusions, frozen.Exclusions) || !reflect.DeepEqual(envelope.BudgetEnvelope, frozen.BudgetEnvelope) || !reflect.DeepEqual(envelope.RecoveryPolicy, frozen.RecoveryPolicy) || !reflect.DeepEqual(envelope.IrreversibleEffects, frozen.IrreversibleEffects) {
		return "", frozen, fmt.Errorf("%w: release authority snapshot", ErrForbidden)
	}
	if err := write.LockReleaseTarget(ctx, r.WorkspaceID, r.IssueID, frozen.TargetKey); err != nil {
		return "", frozen, err
	}
	head, headErr := write.LockReleaseTargetHead(ctx, r.WorkspaceID, r.IssueID, frozen.TargetKey)
	preDeploy := r.Kind == workflowstatev2.CommandStartDeployment || r.Kind == workflowstatev2.CommandRecordDeployment
	if preDeploy {
		if frozen.ExpectedTargetGeneration == 0 && !errors.Is(headErr, pgx.ErrNoRows) || frozen.ExpectedTargetGeneration > 0 && (headErr != nil || head.Generation != frozen.ExpectedTargetGeneration || util.UUIDToString(head.ReleaseID) != frozen.ExpectedTargetReleaseID) {
			return "", frozen, ErrForbidden
		}
	} else if headErr != nil || head.Generation != frozen.DesiredTargetGeneration || head.ReleaseID != release.ID || head.CandidateID != release.CandidateID {
		return "", frozen, ErrForbidden
	}
	return role, frozen, nil
}

func frozenExpectedMatchesCurrent(expected map[string]workflowstatev2.Revision, picture workflowstatev2.CurrentPicture, kind workflowstatev2.CommandKind) bool {
	if len(expected) != 9 {
		return false
	}
	current := allRevisionMap(picture)
	deltas := map[string]workflowstatev2.Revision{}
	if kind == workflowstatev2.CommandRecordDeployment {
		deltas["execution"], deltas["release"], deltas["evidence"] = 1, 1, 1
	} else if kind == workflowstatev2.CommandVerifyRuntime {
		deltas["release"], deltas["evidence"] = 1, 1
	}
	for concern, revision := range current {
		if expected[concern]+deltas[concern] != revision {
			return false
		}
	}
	return true
}

func mutationConcernsForReleaseEffect(kind workflowstatev2.CommandKind) []workflowstatev2.Concern {
	switch kind {
	case workflowstatev2.CommandStartDeployment:
		return []workflowstatev2.Concern{workflowstatev2.ConcernEvidence, workflowstatev2.ConcernExecution, workflowstatev2.ConcernRelease}
	case workflowstatev2.CommandVerifyRuntime:
		return []workflowstatev2.Concern{workflowstatev2.ConcernEvidence}
	case workflowstatev2.CommandVerifyBusiness:
		return []workflowstatev2.Concern{workflowstatev2.ConcernEvidence, workflowstatev2.ConcernExecution, workflowstatev2.ConcernOutcome, workflowstatev2.ConcernRelease}
	case workflowstatev2.CommandProductionFailed, workflowstatev2.CommandRollbackSucceeded, workflowstatev2.CommandRestoreVerified:
		return []workflowstatev2.Concern{workflowstatev2.ConcernEvidence, workflowstatev2.ConcernExecution, workflowstatev2.ConcernOutcome, workflowstatev2.ConcernRelease}
	default:
		return []workflowstatev2.Concern{workflowstatev2.ConcernEvidence, workflowstatev2.ConcernRelease}
	}
}

func validReleaseEffectEvidence(kind workflowstatev2.CommandKind, evidence map[string]any) bool {
	if evidence == nil {
		return false
	}
	decode := func(target any) bool {
		value, err := json.Marshal(evidence)
		if err != nil {
			return false
		}
		decoder := json.NewDecoder(bytes.NewReader(value))
		decoder.DisallowUnknownFields()
		return decoder.Decode(target) == nil && decoder.Decode(&struct{}{}) == io.EOF
	}
	switch kind {
	case workflowstatev2.CommandStartDeployment, workflowstatev2.CommandRecordDeployment:
		var result struct {
			ProviderState   string `json:"provider_state"`
			ReceiptVerified bool   `json:"receipt_verified"`
		}
		return decode(&result) && result.ProviderState == "deployed" && result.ReceiptVerified
	case workflowstatev2.CommandRecordExposure:
		var result struct {
			ExposureState    string `json:"exposure_state"`
			AudienceVerified bool   `json:"audience_verified"`
		}
		return decode(&result) && result.ExposureState == "exposed" && result.AudienceVerified
	case workflowstatev2.CommandBeginVerification, workflowstatev2.CommandVerifyRuntime:
		var result struct {
			RuntimeProof VerificationProof `json:"runtime_proof"`
		}
		return decode(&result) && validVerificationProof(result.RuntimeProof)
	case workflowstatev2.CommandVerifyBusiness:
		var result struct {
			BusinessProof   VerificationProof `json:"business_proof"`
			RegressionProof VerificationProof `json:"regression_proof"`
		}
		return decode(&result) && validVerificationProof(result.BusinessProof) && validVerificationProof(result.RegressionProof) && !reflect.DeepEqual(result.BusinessProof, result.RegressionProof)
	case workflowstatev2.CommandProductionFailed:
		var result struct {
			RegressionProof VerificationProof `json:"regression_proof"`
		}
		return decode(&result) && validVerificationProof(result.RegressionProof)
	case workflowstatev2.CommandRollbackSucceeded:
		var result struct {
			ProviderState   string `json:"provider_state"`
			ReceiptVerified bool   `json:"receipt_verified"`
		}
		return decode(&result) && result.ProviderState == "rolled_back" && result.ReceiptVerified
	case workflowstatev2.CommandRestoreVerified:
		var result struct {
			RuntimeProof     VerificationProof `json:"runtime_proof"`
			BusinessProof    VerificationProof `json:"business_proof"`
			RegressionProof  VerificationProof `json:"regression_proof"`
			RestorationProof VerificationProof `json:"restoration_proof"`
		}
		return decode(&result) && validVerificationProof(result.RuntimeProof) && validVerificationProof(result.BusinessProof) && validVerificationProof(result.RegressionProof) && validVerificationProof(result.RestorationProof) && distinctVerificationProofs(result.RuntimeProof, result.BusinessProof, result.RegressionProof, result.RestorationProof)
	default:
		return false
	}
}

func distinctVerificationProofs(proofs ...VerificationProof) bool {
	seen := make(map[string]struct{}, len(proofs))
	for _, proof := range proofs {
		digest := hashCanonical(proof)
		if _, exists := seen[digest]; exists {
			return false
		}
		seen[digest] = struct{}{}
	}
	return true
}

func validVerificationProof(proof VerificationProof) bool {
	return nonBlankStrings(proof.CriteriaRefs) && nonBlankStrings(proof.CoverageRefs) && meaningfulObject(proof.Evidence)
}

func nonBlankStrings(values []string) bool {
	if len(values) == 0 {
		return false
	}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return false
		}
	}
	return true
}

func (s Service) deriveCommand(ctx context.Context, write *workflowstore.IssueWrite, principal Principal, r Request, p workflowstatev2.CurrentPicture) (workflowstatev2.ActorRole, workflowstatev2.CommandEnvelope, error) {
	c := workflowstatev2.CommandEnvelope{Kind: r.Kind, Expected: append([]workflowstatev2.ExpectedRevision(nil), r.Expected...), OperationID: r.OperationID, BranchID: r.BranchID, DecisionID: r.DecisionID, DecisionOutcome: r.DecisionOutcome, ExpectedDecisionRevision: r.ExpectedDecisionRevision, CandidateID: r.CandidateID, TargetID: r.TargetID, EvidenceRefs: append([]string(nil), r.EvidenceRefs...), CriteriaRefs: append([]string(nil), r.CriteriaRefs...), CoverageRefs: append([]string(nil), r.CoverageRefs...), GapRefs: append([]string(nil), r.GapRefs...), RequiredCorrection: r.RequiredCorrection, CorrectionCount: r.CorrectionCount, MaxCorrections: r.MaxCorrections, Detail: r.Detail, ScopeRevision: p.Scope.Revision, AuthorityRevision: p.Authority.Revision}
	actor := util.UUIDToString(principal.ActorID)
	switch r.Kind {
	case workflowstatev2.CommandPrepareRelease:
		if !s.ControllerID.Valid || principal.ActorType != "internal_controller" || principal.ActorID != s.ControllerID || r.ReleaseAdmission == nil || !write.MatchesReleaseAdmission(r.ReleaseAdmission.ReleaseOperatorID, r.ReleaseAdmission.PlannedVerifierID) || r.OperationID != p.Execution.OperationID || p.Scope.DeploymentPolicy != workflowstatev2.DeploymentRequired || !exactReleaseSnapshotExpected(r.Expected, p) {
			return "", c, ErrForbidden
		}
		envelope, authority, review, digest, err := s.releaseAuthority(ctx, write, r, p)
		if err != nil {
			return "", c, err
		}
		input := r.ReleaseAdmission
		if input.TargetID != envelope.TargetKey || input.ExpectedTargetReleaseID != envelope.ExpectedTargetReleaseID || int64(input.ExpectedTargetGeneration) != envelope.ExpectedTargetGeneration || int64(input.DesiredTargetGeneration) != envelope.DesiredTargetGeneration || input.ReleaseOperatorID != mustUUID(envelope.ReleaseOperatorID) || input.PlannedVerifierID != mustUUID(envelope.PlannedVerifierID) || input.BuildManifestDigest != envelope.BuildManifestDigest || input.ArtifactDigest != envelope.ArtifactDigest || input.RuntimeContractDigest != envelope.RuntimeContractDigest || !reflect.DeepEqual(input.ExposurePolicy, envelope.ExposurePolicy) || !reflect.DeepEqual(input.AudiencePolicy, envelope.AudiencePolicy) || !reflect.DeepEqual(input.RiskEnvelope, envelope.RiskEnvelope) || !reflect.DeepEqual(input.Exclusions, envelope.Exclusions) || !reflect.DeepEqual(input.BudgetEnvelope, envelope.BudgetEnvelope) || !reflect.DeepEqual(input.RecoveryPolicy, envelope.RecoveryPolicy) || int64(input.RecoveryPolicyRevision) != envelope.RecoveryPolicyRevision || input.RollbackTargetReleaseID != envelope.RollbackTargetReleaseID || !reflect.DeepEqual(input.IrreversibleEffects, envelope.IrreversibleEffects) {
			return "", c, ErrForbidden
		}
		if err := write.LockReleaseTarget(ctx, r.WorkspaceID, r.IssueID, envelope.TargetKey); err != nil {
			return "", c, err
		}
		head, headErr := write.LockReleaseTargetHead(ctx, r.WorkspaceID, r.IssueID, envelope.TargetKey)
		if envelope.ExpectedTargetGeneration == 0 {
			if headErr == nil || !errors.Is(headErr, pgx.ErrNoRows) {
				return "", c, ErrForbidden
			}
		} else if headErr != nil || head.Generation != envelope.ExpectedTargetGeneration || util.UUIDToString(head.ReleaseID) != envelope.ExpectedTargetReleaseID {
			return "", c, ErrForbidden
		}
		c.OperationID, c.BranchID, c.CandidateID, c.TargetID = envelope.OperationID, envelope.BranchKey, envelope.CandidateID, envelope.TargetKey
		releaseHead, releaseHeadErr := concernHead(ctx, write, r, workflowstatev2.ConcernRelease)
		if releaseHeadErr != nil {
			return "", c, releaseHeadErr
		}
		c.ReleaseID = util.UUIDToString(releaseHead.SourceRecordID)
		c.TargetGeneration, c.ExpectedTargetGeneration = workflowstatev2.Revision(envelope.DesiredTargetGeneration), workflowstatev2.Revision(envelope.ExpectedTargetGeneration)
		c.ExpectedTargetReleaseID, c.AcceptanceVersion = envelope.ExpectedTargetReleaseID, workflowstatev2.Revision(envelope.AcceptanceVersion)
		c.ReleaseOperatorID, c.PlannedVerifierID = envelope.ReleaseOperatorID, envelope.PlannedVerifierID
		c.BuildManifestDigest, c.ArtifactDigest, c.RuntimeContractDigest = envelope.BuildManifestDigest, envelope.ArtifactDigest, envelope.RuntimeContractDigest
		c.AuthorityCapability, c.AuthorityEvidenceID, c.OwnerEvidenceRevision = envelope.Capability, envelope.OwnerEvidenceID, workflowstatev2.Revision(envelope.OwnerEvidenceRevision)
		c.RecoveryPolicyRevision, c.RollbackTargetReleaseID, c.IrreversibleEffects = workflowstatev2.Revision(envelope.RecoveryPolicyRevision), envelope.RollbackTargetReleaseID, append([]string(nil), envelope.IrreversibleEffects...)
		c.ExposurePolicyDigest = hashCanonical(envelope.ExposurePolicy)
		c.AuthorityRevision = workflowstatev2.Revision(authority.Revision)
		_ = review
		_ = digest
		return workflowstatev2.RoleController, c, nil
	case workflowstatev2.CommandBuildRelease:
		if principal.ActorType != "agent_task" || !principal.TaskID.Valid || r.ReleaseBuild == nil || r.ReleaseAdmission == nil || !write.MatchesReleaseAdmission(r.ReleaseAdmission.ReleaseOperatorID, r.ReleaseAdmission.PlannedVerifierID) || r.CommandKey != "task/"+util.UUIDToString(principal.TaskID)+"/release-build" || !exactReleaseSnapshotExpected(r.Expected, p) {
			return "", c, ErrForbidden
		}
		buildRoute, err := write.LockReleaseBuildAuthority(ctx, db.LockWorkflowReleaseBuildAuthorityParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, TaskID: principal.TaskID})
		if err != nil || buildRoute.ReleaseOperatorID != principal.ActorID || r.ReleaseAdmission.ReleaseOperatorID != buildRoute.ReleaseOperatorID || r.ReleaseAdmission.PlannedVerifierID != buildRoute.PlannedVerifierID || r.ReleaseBuild.Type != "release_build" || !r.ReleaseBuild.Reproducible || !validReleaseBuildCoverage(r.ReleaseBuild.Coverage) {
			return "", c, ErrForbidden
		}
		envelope, _, review, _, err := s.releaseAuthority(ctx, write, r, p)
		if err != nil || envelope.TargetKey != buildRoute.TargetKey || envelope.DesiredTargetGeneration != buildRoute.TargetGeneration || envelope.CandidateID != util.UUIDToString(buildRoute.CandidateID) || envelope.OperationID != util.UUIDToString(buildRoute.OperationID) || envelope.BranchKey != buildRoute.BranchKey.String {
			return "", c, ErrForbidden
		}
		if err := s.validateRegressionReviewEvidence(ctx, write, r, review, buildRoute.CandidateID, r.ReleaseBuild.Coverage); err != nil {
			return "", c, err
		}
		if err := write.LockReleaseTarget(ctx, r.WorkspaceID, r.IssueID, envelope.TargetKey); err != nil {
			return "", c, err
		}
		targetHead, targetErr := write.LockReleaseTargetHead(ctx, r.WorkspaceID, r.IssueID, envelope.TargetKey)
		if envelope.ExpectedTargetGeneration == 0 && (targetErr == nil || !errors.Is(targetErr, pgx.ErrNoRows)) || envelope.ExpectedTargetGeneration > 0 && (targetErr != nil || targetHead.Generation != envelope.ExpectedTargetGeneration || util.UUIDToString(targetHead.ReleaseID) != envelope.ExpectedTargetReleaseID) {
			return "", c, ErrForbidden
		}
		var terminal struct {
			WorkflowResult *ReleaseBuildResultInput `json:"workflow_result"`
		}
		if json.Unmarshal(buildRoute.Result, &terminal) != nil || terminal.WorkflowResult == nil || !reflect.DeepEqual(terminal.WorkflowResult, r.ReleaseBuild) || r.ReleaseBuild.BuildManifestDigest != buildRoute.BuildManifestDigest.String || r.ReleaseBuild.ArtifactDigest != buildRoute.ArtifactDigest.String || r.ReleaseBuild.RuntimeContractDigest != buildRoute.RuntimeContractDigest.String {
			return "", c, ErrInvalidAuthoritativeState
		}
		c.OperationID, c.BranchID, c.CandidateID, c.TargetID = util.UUIDToString(buildRoute.OperationID), buildRoute.BranchKey.String, util.UUIDToString(buildRoute.CandidateID), buildRoute.TargetKey
		c.TargetGeneration = workflowstatev2.Revision(buildRoute.TargetGeneration)
		c.BuildManifestDigest, c.ArtifactDigest, c.RuntimeContractDigest = r.ReleaseBuild.BuildManifestDigest, r.ReleaseBuild.ArtifactDigest, r.ReleaseBuild.RuntimeContractDigest
		c.EvidenceRefs = []string{stableUUID(r.WorkspaceID, r.IssueID, r.CommandKey, "build-evidence").String()}
		return workflowstatev2.RoleRelease, c, nil
	case workflowstatev2.CommandAuthorizeRelease:
		if !s.ControllerID.Valid || principal.ActorType != "internal_controller" || principal.ActorID != s.ControllerID || r.ReleaseAdmission == nil || !write.MatchesReleaseAdmission(r.ReleaseAdmission.ReleaseOperatorID, r.ReleaseAdmission.PlannedVerifierID) || r.OperationID != p.Execution.OperationID || !exactReleaseSnapshotExpected(r.Expected, p) {
			return "", c, ErrForbidden
		}
		envelope, authority, _, _, err := s.releaseAuthority(ctx, write, r, p)
		if err != nil || p.Release.State != workflowstatev2.ReleaseBuilt || envelope.TargetKey != p.Release.TargetID || envelope.DesiredTargetGeneration != int64(p.Release.Generation) || envelope.ReleaseOperatorID != p.Release.ReleaseOperatorID || envelope.PlannedVerifierID != p.Release.PlannedVerifierID {
			return "", c, ErrForbidden
		}
		if err := s.validateReleaseBuildEvidence(ctx, write, r, p); err != nil {
			return "", c, err
		}
		if err := write.LockReleaseTarget(ctx, r.WorkspaceID, r.IssueID, envelope.TargetKey); err != nil {
			return "", c, err
		}
		head, headErr := write.LockReleaseTargetHead(ctx, r.WorkspaceID, r.IssueID, envelope.TargetKey)
		if envelope.ExpectedTargetGeneration == 0 && !errors.Is(headErr, pgx.ErrNoRows) || envelope.ExpectedTargetGeneration > 0 && (headErr != nil || head.Generation != envelope.ExpectedTargetGeneration || util.UUIDToString(head.ReleaseID) != envelope.ExpectedTargetReleaseID) {
			return "", c, ErrForbidden
		}
		c.OperationID, c.BranchID, c.CandidateID, c.TargetID, c.TargetGeneration = envelope.OperationID, envelope.BranchKey, envelope.CandidateID, envelope.TargetKey, workflowstatev2.Revision(envelope.DesiredTargetGeneration)
		c.AuthorityRevision, c.AuthorityCapability, c.AuthorityEvidenceID = workflowstatev2.Revision(authority.Revision), envelope.Capability, envelope.OwnerEvidenceID
		c.EvidenceRefs = []string{stableUUID(r.WorkspaceID, r.IssueID, r.CommandKey, "authorization-evidence").String()}
		return workflowstatev2.RoleController, c, nil
	case workflowstatev2.CommandStartDeployment, workflowstatev2.CommandRecordDeployment, workflowstatev2.CommandRecordExposure, workflowstatev2.CommandBeginVerification, workflowstatev2.CommandVerifyRuntime, workflowstatev2.CommandVerifyBusiness, workflowstatev2.CommandProductionFailed, workflowstatev2.CommandRollbackSucceeded, workflowstatev2.CommandRestoreVerified:
		role, frozen, err := s.releaseEffectAuthority(ctx, write, principal, r, p)
		if err != nil {
			return "", c, err
		}
		c.OperationID, c.BranchID, c.CandidateID, c.TargetID = frozen.OperationID, frozen.BranchKey, frozen.CandidateID, frozen.TargetKey
		c.TargetGeneration = workflowstatev2.Revision(frozen.DesiredTargetGeneration)
		c.ReleaseID, c.CandidateRevision = frozen.ReleaseID, workflowstatev2.Revision(frozen.CandidateRevision)
		c.PlannedVerifierID, c.ReleaseOperatorID = frozen.PlannedVerifierID, frozen.ReleaseOperatorID
		c.AuthorityRevision = workflowstatev2.Revision(frozen.AuthorityRevision)
		c.AuthorityCapability, c.AuthorityEvidenceID = frozen.AuthorityCapability, frozen.AuthorityEvidenceID
		c.EvidenceRefs = []string{stableUUID(r.WorkspaceID, r.IssueID, r.CommandKey, "release-effect-evidence").String()}
		if r.Kind == workflowstatev2.CommandVerifyBusiness {
			c.EvidenceRefs = append(c.EvidenceRefs, stableUUID(r.WorkspaceID, r.IssueID, r.CommandKey, "regression-evidence").String())
		}
		if r.Kind == workflowstatev2.CommandProductionFailed {
			release, releaseErr := write.GetRelease(ctx, db.GetIssueWorkflowReleaseParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: mustUUID(frozen.ReleaseID)})
			if releaseErr != nil || release.State != "live" {
				return "", c, ErrInvalidAuthoritativeState
			}
			var recovery struct {
				Policy              map[string]any `json:"policy"`
				Revision            int64          `json:"revision"`
				RollbackTargetID    string         `json:"rollback_target_release_id"`
				IrreversibleEffects []string       `json:"irreversible_effects"`
			}
			if json.Unmarshal(release.RecoveryPolicy, &recovery) != nil || recovery.Revision != frozen.RecoveryPolicyRevision || !reflect.DeepEqual(recovery.Policy, frozen.RecoveryPolicy) || !reflect.DeepEqual(recovery.IrreversibleEffects, frozen.IrreversibleEffects) {
				return "", c, ErrInvalidAuthoritativeState
			}
			mode, _ := recovery.Policy["mode"].(string)
			c.RecoveryMode = workflowstatev2.RecoveryContainment
			if mode == "rollback" && len(recovery.IrreversibleEffects) == 0 && recovery.RollbackTargetID != "" && recovery.RollbackTargetID == frozen.RollbackTargetReleaseID && recovery.RollbackTargetID == frozen.ExpectedTargetReleaseID {
				knownGood, knownGoodErr := s.loadKnownGoodRecoverySnapshot(ctx, write, r.WorkspaceID, r.IssueID, release.TargetKey, release.ExpectedTargetGeneration.Int64, mustUUID(recovery.RollbackTargetID))
				if knownGoodErr != nil {
					return "", c, ErrInvalidAuthoritativeState
				}
				c.RecoveryMode = workflowstatev2.RecoveryRollback
				c.RecoveryReleaseID = stableUUID(r.WorkspaceID, r.IssueID, r.CommandKey, "recovery-release").String()
				c.RecoveryOfReleaseID = frozen.ReleaseID
				c.RecoveryTargetGeneration = workflowstatev2.Revision(frozen.DesiredTargetGeneration + 1)
				c.RecoveryCandidateID, c.RecoveryCandidateRevision = stableUUID(r.WorkspaceID, r.IssueID, r.CommandKey, "known-good-candidate-snapshot").String(), 1
				c.RecoverySourceCandidateID, c.RecoverySourceRevision = util.UUIDToString(knownGood.candidate.ID), workflowstatev2.Revision(knownGood.candidate.Revision)
				c.RecoveryBuildManifestDigest, c.RecoveryArtifactDigest, c.RecoveryRuntimeDigest = knownGood.release.BuildManifestDigest.String, knownGood.release.ArtifactDigest.String, knownGood.release.RuntimeContractDigest.String
				c.RecoveryReviewedReviewID, c.RecoveryReviewedReviewRevision = util.UUIDToString(knownGood.review.ID), 1
			}
		}
		if r.Kind == workflowstatev2.CommandRestoreVerified {
			c.EvidenceRefs = []string{
				stableUUID(r.WorkspaceID, r.IssueID, r.CommandKey, "restored-runtime-evidence").String(),
				stableUUID(r.WorkspaceID, r.IssueID, r.CommandKey, "restored-business-evidence").String(),
				stableUUID(r.WorkspaceID, r.IssueID, r.CommandKey, "restored-regression-evidence").String(),
				stableUUID(r.WorkspaceID, r.IssueID, r.CommandKey, "restoration-evidence").String(),
			}
		}
		return role, c, nil
	case workflowstatev2.CommandSubmitCorrection:
		if principal.ActorType != "agent_task" || !principal.TaskID.Valid || r.Correction == nil || r.CommandKey != "task/"+util.UUIDToString(principal.TaskID)+"/correction-result" {
			return "", c, ErrForbidden
		}
		authority, err := write.LockCorrectionResultAuthority(ctx, db.LockWorkflowCorrectionResultAuthorityParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, TaskID: principal.TaskID})
		if err != nil || authority.ImplementerID != principal.ActorID || util.UUIDToString(authority.RejectedCandidateID) != p.Review.CandidateID || util.UUIDToString(authority.OperationID) != p.Execution.OperationID || authority.ScopeRevision != int64(p.Scope.Revision) || authority.AcceptanceVersion != int64(p.Scope.AcceptanceVersion) || authority.CorrectionCount != int32(p.Review.CorrectionCount) || authority.MaxCorrections != int32(p.Review.MaxCorrections) {
			return "", c, ErrForbidden
		}
		var terminal struct {
			WorkflowResult *CorrectionResultInput `json:"workflow_result"`
		}
		if json.Unmarshal(authority.Result, &terminal) != nil || terminal.WorkflowResult == nil || terminal.WorkflowResult.Type != "correction" || strings.TrimSpace(terminal.WorkflowResult.SourceRevision) == "" || !meaningfulObject(terminal.WorkflowResult.Artifact) || !meaningfulObject(terminal.WorkflowResult.BuildContract) || !meaningfulObject(terminal.WorkflowResult.Evidence) || !reflect.DeepEqual(terminal.WorkflowResult, r.Correction) {
			return "", c, ErrInvalidAuthoritativeState
		}
		c.OperationID = util.UUIDToString(authority.OperationID)
		c.BranchID = authority.BranchKey.String
		c.CandidateID = util.UUIDToString(authority.CorrectionCandidateID)
		c.ReviewerID = util.UUIDToString(authority.ReviewerID)
		c.EvidenceRefs = []string{stableUUID(r.WorkspaceID, r.IssueID, r.CommandKey, "implementation-evidence").String()}
		return workflowstatev2.RoleWorker, c, nil
	case workflowstatev2.CommandReviewPassed, workflowstatev2.CommandReviewFailed, workflowstatev2.CommandReviewInconclusive, workflowstatev2.CommandReviewBlockedEvidence:
		if principal.ActorType == "agent_task" && principal.TaskID.Valid && r.CommandKey == "task/"+util.UUIDToString(principal.TaskID)+"/review-result" {
			authority, err := write.LockReviewResultAuthority(ctx, db.LockWorkflowReviewResultAuthorityParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, TaskID: principal.TaskID})
			if err != nil || authority.ReviewerID != principal.ActorID || util.UUIDToString(authority.OperationID) != p.Execution.OperationID || util.UUIDToString(authority.CandidateID) != p.Review.CandidateID || authority.CandidateRevision != 1 || authority.ScopeRevision != int64(p.Scope.Revision) || authority.AcceptanceVersion != int64(p.Scope.AcceptanceVersion) {
				return "", c, ErrForbidden
			}
			var terminal struct {
				WorkflowResult *CorrectionResultInput `json:"workflow_result"`
			}
			if json.Unmarshal(authority.Result, &terminal) != nil || terminal.WorkflowResult == nil || terminal.WorkflowResult.Type != "review" || !meaningfulObject(terminal.WorkflowResult.Evidence) || !reflect.DeepEqual(terminal.WorkflowResult, r.Correction) || reviewKind(terminal.WorkflowResult.Verdict) != r.Kind {
				return "", c, ErrInvalidAuthoritativeState
			}
			if !validReviewResultSemantics(terminal.WorkflowResult, r.Kind) {
				return "", c, ErrInvalidAuthoritativeState
			}
			c.OperationID = util.UUIDToString(authority.OperationID)
			c.BranchID = authority.BranchKey.String
			c.CandidateID = util.UUIDToString(authority.CandidateID)
			c.ReviewerID = util.UUIDToString(authority.ReviewerID)
			c.EvidenceRefs = []string{stableUUID(r.WorkspaceID, r.IssueID, r.CommandKey, "review-evidence").String()}
			c.CriteriaRefs = append([]string{}, terminal.WorkflowResult.CriteriaRefs...)
			c.CoverageRefs = append([]string{}, terminal.WorkflowResult.CoverageRefs...)
			c.GapRefs = append([]string{}, terminal.WorkflowResult.GapRefs...)
			c.RequiredCorrection = terminal.WorkflowResult.RequiredCorrection
			c.CorrectionCount, c.MaxCorrections = uint32(authority.CorrectionCount), uint32(authority.MaxCorrections)
			return workflowstatev2.RoleReviewer, c, nil
		}
		execution, err := selectedExecution(ctx, write, r.WorkspaceID, r.IssueID)
		if err != nil || principal.ActorType != "agent_task" || actor == "" || actor != p.Review.ReviewerID || !principal.TaskID.Valid || execution.ActorID != principal.ActorID {
			return "", c, ErrForbidden
		}
		authorized, err := write.LockReviewerAuthority(ctx, db.LockWorkflowReviewerCommandAuthorityParams{ActorID: principal.ActorID, WorkspaceID: r.WorkspaceID, TaskID: principal.TaskID, IssueID: r.IssueID, OperationID: execution.ID, CandidateID: p.Review.CandidateID, BoundScopeRevision: int64(p.Scope.Revision)})
		if err != nil || !authorized {
			return "", c, ErrForbidden
		}
		c.CandidateID, c.ReviewerID = p.Review.CandidateID, actor
		return workflowstatev2.RoleReviewer, c, nil
	case workflowstatev2.CommandRequestDecision:
		if r.Decision == nil || r.Decision.Type != workflowstatev2.DecisionAuthorization || r.CandidateID == "" || r.CandidateID != p.Review.CandidateID || r.Decision.AuthorityGap.Kind != workflowstatev2.AuthorityGapOperationAuthorization || strings.TrimSpace(r.Decision.AuthorityGap.RequestedCapability) == "" || strings.TrimSpace(r.Decision.AuthorityGap.Reason) == "" || !s.ControllerID.Valid || principal.ActorID != s.ControllerID || principal.ActorType != "internal_controller" || r.OperationID != p.Execution.OperationID {
			return "", c, ErrForbidden
		}
		execution, err := selectedExecution(ctx, write, r.WorkspaceID, r.IssueID)
		if err != nil || util.UUIDToString(execution.ID) != p.Execution.OperationID {
			return "", c, ErrInvalidAuthoritativeState
		}
		for _, decision := range p.Decisions {
			if decision.State == workflowstatev2.DecisionUnanswered || decision.Resolution == workflowstatev2.ResolutionResuming {
				return "", c, ErrUnsupportedCommand
			}
		}
		ownerAuthorized, err := write.LockOwnerAuthority(ctx, r.WorkspaceID, r.IssueID, r.Decision.AccountableOwnerID)
		if err != nil || !ownerAuthorized {
			return "", c, ErrForbidden
		}
		c.DecisionID = stableUUID(r.WorkspaceID, r.IssueID, r.CommandKey, "decision").String()
		c.DecisionType, c.OperationID, c.BranchID, c.CandidateID, c.AuthorityGap = r.Decision.Type, p.Execution.OperationID, execution.BranchKey, r.CandidateID, r.Decision.AuthorityGap
		return workflowstatev2.RoleController, c, nil
	case workflowstatev2.CommandTechnicalStop:
		if !s.ControllerID.Valid || principal.ActorID != s.ControllerID || principal.ActorType != "internal_controller" || r.OperationID == "" || r.OperationID != p.Execution.OperationID || r.Detail == "" || len(r.Expected) != 1 || r.Expected[0].Concern != workflowstatev2.ConcernExecution || r.Expected[0].Revision != p.Revisions.Execution {
			return "", c, ErrForbidden
		}
		execution, err := selectedExecution(ctx, write, r.WorkspaceID, r.IssueID)
		if err != nil || util.UUIDToString(execution.ID) != r.OperationID {
			return "", c, ErrForbidden
		}
		c.OperationID = r.OperationID
		return workflowstatev2.RoleController, c, nil
	case workflowstatev2.CommandCompleteNonLive:
		if !s.ControllerID.Valid || principal.ActorID != s.ControllerID || principal.ActorType != "internal_controller" || !principal.ActionID.Valid || !principal.LeaseToken.Valid || principal.ActionKey == "" || r.CommandKey != "action/"+util.UUIDToString(principal.ActionID)+"/complete-non-live" || p.Scope.DeploymentPolicy != workflowstatev2.DeploymentNotRequired || p.Scope.SourcePlanRevision == 0 || r.OperationID != p.Execution.OperationID || !exactExpected(r.Expected, p, workflowstatev2.ConcernScope, workflowstatev2.ConcernReview, workflowstatev2.ConcernEvidence, workflowstatev2.ConcernOutcome, workflowstatev2.ConcernExecution, workflowstatev2.ConcernRelease) {
			return "", c, ErrForbidden
		}
		expectedPayload, _ := json.Marshal(expectedRevisionMap(r.Expected))
		authorized, err := write.LockFinalizerAuthority(ctx, db.LockWorkflowFinalizerCommandAuthorityParams{ActionID: principal.ActionID, WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ActionKey: principal.ActionKey, OperationID: mustUUID(r.OperationID), LeaseToken: principal.LeaseToken, Now: pgtype.Timestamptz{Time: s.Now().UTC(), Valid: true}, CandidateID: p.Review.CandidateID, ScopeRevision: int64(p.Scope.Revision), AcceptanceVersion: int64(p.Scope.AcceptanceVersion), SourcePlanRevision: int64(p.Scope.SourcePlanRevision), ExpectedRevisions: expectedPayload})
		if err != nil || !authorized {
			return "", c, ErrForbidden
		}
		reviewHead, err := concernHead(ctx, write, r, workflowstatev2.ConcernReview)
		if err != nil || reviewHead.State != "passed" {
			return "", c, ErrInvalidAuthoritativeState
		}
		review, err := write.GetReview(ctx, db.GetIssueWorkflowReviewParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: reviewHead.SourceRecordID})
		if err != nil || review.Outcome != "passed" || review.CandidateID != mustUUID(p.Review.CandidateID) || review.ScopeRevision != int64(p.Scope.Revision) || review.SupersededAt.Valid {
			return "", c, ErrInvalidAuthoritativeState
		}
		var refs []string
		if json.Unmarshal(review.EvidenceRefs, &refs) != nil || len(refs) == 0 {
			return "", c, ErrInvalidAuthoritativeState
		}
		sort.Strings(refs)
		rows, err := write.ListEvidence(ctx, db.ListIssueWorkflowEvidenceParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID})
		if err != nil {
			return "", c, err
		}
		byID := make(map[string]db.IssueWorkflowEvidence, len(rows))
		for _, row := range rows {
			byID[util.UUIDToString(row.ID)] = row
		}
		candidate, err := write.GetCandidate(ctx, db.GetIssueWorkflowCandidateParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: review.CandidateID})
		if err != nil {
			return "", c, err
		}
		for _, ref := range refs {
			row, ok := byID[ref]
			if !ok || validateCompletionReviewEvidence(row, review, candidate, s.Now().UTC()) != nil {
				return "", c, ErrInvalidAuthoritativeState
			}
		}
		c.OperationID, c.CandidateID, c.EvidenceRefs = p.Execution.OperationID, p.Review.CandidateID, refs
		c.DeploymentPolicy, c.SourcePlanRevision = p.Scope.DeploymentPolicy, p.Scope.SourcePlanRevision
		return workflowstatev2.RoleController, c, nil
	case workflowstatev2.CommandRespondDecision:
		if principal.ActorType != "member" {
			return "", c, ErrForbidden
		}
		if r.DecisionOutcome != workflowstatev2.DecisionApprove && r.DecisionOutcome != workflowstatev2.DecisionReject {
			return "", c, ErrUnsupportedCommand
		}
		row, err := write.GetDecision(ctx, db.GetIssueWorkflowDecisionParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: mustUUID(r.DecisionID)})
		if err != nil || row.AccountableOwnerID != principal.ActorID {
			return "", c, ErrForbidden
		}
		ownerAuthorized, err := write.LockOwnerAuthority(ctx, r.WorkspaceID, r.IssueID, principal.ActorID)
		if err != nil || !ownerAuthorized {
			return "", c, ErrForbidden
		}
		d, ok := p.Decisions[r.DecisionID]
		if !ok || row.State != "unanswered" || row.ResolutionState != "open" || row.Revision != int64(r.ExpectedDecisionRevision) || d.State != workflowstatev2.DecisionUnanswered || d.Revision != r.ExpectedDecisionRevision {
			return "", c, ErrDecisionStale
		}
		relation, err := exactDecisionRequestRelation(ctx, write, r.WorkspaceID, r.IssueID, mustUUID(r.DecisionID))
		if err != nil || relation.decisionRevision != int64(r.ExpectedDecisionRevision) || relation.ownerID != principal.ActorID {
			return "", c, ErrForbidden
		}
		c.DecisionType, c.ScopeRevision, c.AuthorityRevision, c.CandidateID, c.TargetID, c.BranchID, c.OperationID = d.Type, d.ScopeRevision, d.AuthorityRevision, d.CandidateID, d.TargetID, d.BranchID, d.SuspendedOperationID
		return workflowstatev2.RoleOwner, c, nil
	case workflowstatev2.CommandReadDecision, workflowstatev2.CommandAcknowledgeDecision:
		if principal.ActorType != "member" {
			return "", c, ErrForbidden
		}
		row, err := write.GetDecision(ctx, db.GetIssueWorkflowDecisionParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: mustUUID(r.DecisionID)})
		if err != nil || row.AccountableOwnerID != principal.ActorID {
			return "", c, ErrForbidden
		}
		ownerAuthorized, err := write.LockOwnerAuthority(ctx, r.WorkspaceID, r.IssueID, principal.ActorID)
		if err != nil || !ownerAuthorized {
			return "", c, ErrForbidden
		}
		d, ok := p.Decisions[r.DecisionID]
		if !ok || row.State != "unanswered" || row.ResolutionState != "open" || row.Revision != int64(r.ExpectedDecisionRevision) || d.State != workflowstatev2.DecisionUnanswered || d.Revision != r.ExpectedDecisionRevision {
			return "", c, ErrDecisionStale
		}
		return workflowstatev2.RoleOwner, c, nil
	case workflowstatev2.CommandSettleDecisionEffect:
		if !s.ControllerID.Valid || principal.ActorID != s.ControllerID || principal.ActorType != "internal_controller" || !principal.ActionID.Valid || !principal.LeaseToken.Valid || principal.ActionKey == "" || r.CommandKey != "action/"+util.UUIDToString(principal.ActionID)+"/settle-owner-decision" || !exactExpected(r.Expected, p, workflowstatev2.ConcernExecution, workflowstatev2.ConcernAttention) {
			return "", c, ErrForbidden
		}
		action, err := write.GetActionByKey(ctx, r.WorkspaceID, r.IssueID, principal.ActionKey)
		if err != nil || action.ID != principal.ActionID || action.LeaseToken != principal.LeaseToken || action.State != "leased" || !action.LeaseExpiresAt.Valid || !action.LeaseExpiresAt.Time.After(s.Now().UTC()) || (action.ActionType != "resume_exact_operation" && action.ActionType != "replan_exact_operation") {
			return "", c, ErrForbidden
		}
		var frozen struct {
			DecisionID        string                              `json:"decision_id"`
			DecisionRevision  workflowstatev2.Revision            `json:"decision_revision"`
			DecisionType      workflowstatev2.DecisionType        `json:"decision_type"`
			Outcome           workflowstatev2.DecisionOutcome     `json:"outcome"`
			ScopeRevision     workflowstatev2.Revision            `json:"scope_revision"`
			AuthorityRevision workflowstatev2.Revision            `json:"authority_revision"`
			CandidateID       string                              `json:"candidate_id"`
			TargetID          string                              `json:"target_key"`
			OperationID       string                              `json:"operation_id"`
			BranchID          string                              `json:"branch_key"`
			SuspendedState    workflowstatev2.ExecutionState      `json:"suspended_state"`
			CandidateRevision int64                               `json:"candidate_revision"`
			ExecutionRevision workflowstatev2.Revision            `json:"execution_revision"`
			AttentionID       string                              `json:"attention_id"`
			AttentionRevision workflowstatev2.Revision            `json:"attention_revision"`
			ResponseID        string                              `json:"response_id"`
			ResponseRevision  int64                               `json:"response_revision"`
			RequestActionID   string                              `json:"source_request_action_id"`
			RequestActionKey  string                              `json:"source_request_action_key"`
			RequestActionRev  int64                               `json:"source_request_action_revision"`
			Expected          map[string]workflowstatev2.Revision `json:"expected"`
		}
		if json.Unmarshal(action.Payload, &frozen) != nil {
			return "", c, ErrForbidden
		}
		candidate, candidateErr := write.GetCandidate(ctx, db.GetIssueWorkflowCandidateParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: mustUUID(p.Review.CandidateID)})
		attention, attentionErr := write.GetAttention(ctx, db.GetIssueWorkflowAttentionParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: mustUUID(frozen.AttentionID)})
		d, decisionOK := p.Decisions[r.DecisionID]
		if candidateErr != nil || attentionErr != nil || !decisionOK || frozen.DecisionID != r.DecisionID || action.DecisionID != mustUUID(r.DecisionID) || util.UUIDToString(action.OperationID) != frozen.OperationID || frozen.DecisionRevision != r.ExpectedDecisionRevision || frozen.DecisionType != d.Type || frozen.Outcome != r.DecisionOutcome || frozen.Outcome != d.Response || frozen.ScopeRevision != p.Scope.Revision || frozen.AuthorityRevision != p.Authority.Revision || frozen.CandidateID != p.Review.CandidateID || frozen.CandidateRevision != candidate.Revision || frozen.TargetID != d.TargetID || frozen.OperationID != d.SuspendedOperationID || frozen.BranchID != d.BranchID || frozen.SuspendedState != d.SuspendedState || frozen.ExecutionRevision != p.Revisions.Execution || frozen.AttentionRevision != p.Revisions.Attention || attention.DecisionID != mustUUID(r.DecisionID) || attention.ActionReceiptID != action.ID || frozen.ResponseID == "" || frozen.ResponseRevision == 0 || frozen.RequestActionID == "" || frozen.RequestActionKey == "" || frozen.RequestActionRev == 0 || frozen.Expected[string(workflowstatev2.ConcernExecution)] != p.Revisions.Execution || frozen.Expected[string(workflowstatev2.ConcernAttention)] != p.Revisions.Attention {
			return "", c, ErrForbidden
		}
		if err := validateDecisionContinuationSources(ctx, write, r.WorkspaceID, r.IssueID, frozen.ResponseID, frozen.ResponseRevision, frozen.RequestActionID, frozen.RequestActionKey, frozen.RequestActionRev, r.DecisionID, r.DecisionOutcome); err != nil {
			return "", c, ErrForbidden
		}
		c.DecisionID, c.ExpectedDecisionRevision, c.DecisionType, c.DecisionOutcome = frozen.DecisionID, frozen.DecisionRevision, frozen.DecisionType, frozen.Outcome
		c.ScopeRevision, c.AuthorityRevision, c.CandidateID, c.TargetID, c.OperationID, c.BranchID = frozen.ScopeRevision, frozen.AuthorityRevision, frozen.CandidateID, frozen.TargetID, frozen.OperationID, frozen.BranchID
		c.SuspendedState = frozen.SuspendedState
		return workflowstatev2.RoleController, c, nil
	default:
		return "", c, fmt.Errorf("%w: %s", ErrUnsupportedCommand, r.Kind)
	}
}

// replayRole authenticates the immutable principal kind without consulting a
// mutable task/action lease. Exact admitted identity is then proven by the
// stored semantic fingerprint and actor columns.
func (s Service) replayRole(principal Principal, r Request) (workflowstatev2.ActorRole, error) {
	switch r.Kind {
	case workflowstatev2.CommandPrepareRelease, workflowstatev2.CommandAuthorizeRelease:
		if s.ControllerID.Valid && principal.ActorID == s.ControllerID && principal.ActorType == "internal_controller" {
			return workflowstatev2.RoleController, nil
		}
	case workflowstatev2.CommandBuildRelease:
		if principal.ActorType == "agent_task" && principal.TaskID.Valid {
			return workflowstatev2.RoleRelease, nil
		}
	case workflowstatev2.CommandStartDeployment, workflowstatev2.CommandRecordDeployment, workflowstatev2.CommandRecordExposure:
		if principal.ActorType == "workflow_action" && principal.ActionID.Valid && principal.ActionKey != "" {
			return workflowstatev2.RoleRelease, nil
		}
	case workflowstatev2.CommandBeginVerification, workflowstatev2.CommandVerifyRuntime, workflowstatev2.CommandVerifyBusiness:
		if principal.ActorType == "workflow_action" && principal.ActionID.Valid && principal.ActionKey != "" {
			return workflowstatev2.RoleVerifier, nil
		}
	case workflowstatev2.CommandSubmitCorrection:
		if principal.ActorType == "agent_task" {
			return workflowstatev2.RoleWorker, nil
		}
	case workflowstatev2.CommandRequestDecision:
		if s.ControllerID.Valid && principal.ActorID == s.ControllerID && principal.ActorType == "internal_controller" {
			return workflowstatev2.RoleController, nil
		}
	case workflowstatev2.CommandSettleDecisionEffect:
		if s.ControllerID.Valid && principal.ActorID == s.ControllerID && principal.ActorType == "internal_controller" && principal.ActionID.Valid && principal.ActionKey != "" {
			return workflowstatev2.RoleController, nil
		}
	case workflowstatev2.CommandTechnicalStop:
		if s.ControllerID.Valid && principal.ActorID == s.ControllerID && principal.ActorType == "internal_controller" {
			return workflowstatev2.RoleController, nil
		}
	case workflowstatev2.CommandCompleteNonLive:
		if s.ControllerID.Valid && principal.ActorID == s.ControllerID && principal.ActorType == "internal_controller" && principal.ActionID.Valid && principal.ActionKey != "" {
			return workflowstatev2.RoleController, nil
		}
	case workflowstatev2.CommandRespondDecision, workflowstatev2.CommandReadDecision, workflowstatev2.CommandAcknowledgeDecision:
		if principal.ActorType == "member" {
			return workflowstatev2.RoleOwner, nil
		}
	case workflowstatev2.CommandReviewPassed, workflowstatev2.CommandReviewFailed, workflowstatev2.CommandReviewInconclusive, workflowstatev2.CommandReviewBlockedEvidence:
		if principal.ActorType == "agent_task" {
			return workflowstatev2.RoleReviewer, nil
		}
	}
	return "", ErrForbidden
}

func semanticFingerprint(principal Principal, request Request, command workflowstatev2.CommandEnvelope) (string, []byte, []byte, error) {
	request.Expected = append([]workflowstatev2.ExpectedRevision(nil), request.Expected...)
	sort.Slice(request.Expected, func(i, j int) bool { return request.Expected[i].Concern < request.Expected[j].Concern })
	semantic := commandSemantic{
		Principal: util.UUIDToString(principal.ActorID), ActorType: principal.ActorType,
		TaskID: util.UUIDToString(principal.TaskID), ActionKey: principal.ActionKey,
		ActionID: util.UUIDToString(principal.ActionID), LeaseToken: fingerprintLeaseToken(principal, request),
		Request: request, DerivedRole: command.ActorRole, DerivedCandidate: command.CandidateID,
		DerivedEvidence: append([]string(nil), command.EvidenceRefs...), DeploymentPolicy: command.DeploymentPolicy,
		SourcePlanRevision: command.SourcePlanRevision,
	}
	payload, err := json.Marshal(semantic)
	if err != nil {
		return "", nil, nil, err
	}
	expected := append([]workflowstatev2.ExpectedRevision(nil), request.Expected...)
	sort.Slice(expected, func(i, j int) bool { return expected[i].Concern < expected[j].Concern })
	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		return "", nil, nil, err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), payload, expectedJSON, nil
}

type commandSemantic struct {
	Principal          string                           `json:"principal"`
	ActorType          string                           `json:"actor_type"`
	TaskID             string                           `json:"task_id,omitempty"`
	ActionKey          string                           `json:"action_key,omitempty"`
	ActionID           string                           `json:"action_id,omitempty"`
	LeaseToken         string                           `json:"lease_token,omitempty"`
	Request            Request                          `json:"request"`
	DerivedRole        workflowstatev2.ActorRole        `json:"derived_role"`
	DerivedCandidate   string                           `json:"derived_candidate,omitempty"`
	DerivedEvidence    []string                         `json:"derived_evidence,omitempty"`
	DeploymentPolicy   workflowstatev2.DeploymentPolicy `json:"deployment_policy,omitempty"`
	SourcePlanRevision workflowstatev2.Revision         `json:"source_plan_revision,omitempty"`
}

func replayFingerprint(payload []byte, principal Principal, request Request, role workflowstatev2.ActorRole) (string, error) {
	request.Expected = append([]workflowstatev2.ExpectedRevision(nil), request.Expected...)
	sort.Slice(request.Expected, func(i, j int) bool { return request.Expected[i].Concern < request.Expected[j].Concern })
	var semantic commandSemantic
	if err := json.Unmarshal(payload, &semantic); err != nil {
		return "", ErrIdempotencyConflict
	}
	if semantic.Principal != util.UUIDToString(principal.ActorID) || semantic.ActorType != principal.ActorType || semantic.TaskID != util.UUIDToString(principal.TaskID) || semantic.ActionKey != principal.ActionKey || semantic.ActionID != util.UUIDToString(principal.ActionID) || semantic.LeaseToken != fingerprintLeaseToken(principal, request) || semantic.DerivedRole != role || !reflect.DeepEqual(semantic.Request, request) {
		return "", fmt.Errorf("%w: stored principal=%q type=%q task=%q action=%q/%q lease=%q role=%q request_equal=%t", ErrIdempotencyConflict, semantic.Principal, semantic.ActorType, semantic.TaskID, semantic.ActionID, semantic.ActionKey, semantic.LeaseToken, semantic.DerivedRole, reflect.DeepEqual(semantic.Request, request))
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func fingerprintLeaseToken(principal Principal, request Request) string {
	// Finalizer replay is keyed by immutable action identity. The lease proves
	// first admission only and may legitimately change after an uncertain
	// post-commit result before the stored command is replayed.
	if request.Kind == workflowstatev2.CommandCompleteNonLive || request.Kind == workflowstatev2.CommandSettleDecisionEffect || isReleaseEffectCommand(request.Kind) {
		return ""
	}
	return util.UUIDToString(principal.LeaseToken)
}

func expectedRevisionMap(expected []workflowstatev2.ExpectedRevision) map[string]workflowstatev2.Revision {
	result := make(map[string]workflowstatev2.Revision, len(expected))
	for _, item := range expected {
		result[string(item.Concern)] = item.Revision
	}
	return result
}

func allRevisionMap(p workflowstatev2.CurrentPicture) map[string]workflowstatev2.Revision {
	return map[string]workflowstatev2.Revision{
		"outcome": p.Revisions.Outcome, "scope": p.Revisions.Scope,
		"authority": p.Revisions.Authority, "execution": p.Revisions.Execution,
		"review": p.Revisions.Review, "release": p.Revisions.Release,
		"attention": p.Revisions.Attention, "evidence": p.Revisions.Evidence,
		"budget": p.Revisions.Budget,
	}
}

func stableUUID(workspaceID, issueID pgtype.UUID, key, suffix string) googleuuid.UUID {
	return googleuuid.NewSHA1(googleuuid.NameSpaceOID, []byte(util.UUIDToString(workspaceID)+"/"+util.UUIDToString(issueID)+"/"+key+"/"+suffix))
}

func previewSourceRevision(candidateID googleuuid.UUID) string {
	return "candidate:" + candidateID.String() + ":1"
}

func pgUUID(id googleuuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }
func mustUUID(value string) pgtype.UUID {
	id, err := googleuuid.Parse(value)
	if err != nil {
		return pgtype.UUID{}
	}
	return pgUUID(id)
}
func nullableInt64(v int64) pgtype.Int8 { return pgtype.Int8{Int64: v, Valid: true} }

func changedConcerns(a, b workflowstatev2.ConcernRevisions) []workflowstatev2.Concern {
	all := []workflowstatev2.Concern{workflowstatev2.ConcernAttention, workflowstatev2.ConcernAuthority, workflowstatev2.ConcernBudget, workflowstatev2.ConcernEvidence, workflowstatev2.ConcernExecution, workflowstatev2.ConcernOutcome, workflowstatev2.ConcernRelease, workflowstatev2.ConcernReview, workflowstatev2.ConcernScope}
	var out []workflowstatev2.Concern
	for _, c := range all {
		if a.Get(c) != b.Get(c) {
			out = append(out, c)
		}
	}
	return out
}

func isReviewResultKind(kind workflowstatev2.CommandKind) bool {
	switch kind {
	case workflowstatev2.CommandReviewPassed, workflowstatev2.CommandReviewFailed, workflowstatev2.CommandReviewInconclusive, workflowstatev2.CommandReviewBlockedEvidence:
		return true
	default:
		return false
	}
}

type sourceRef struct {
	id       pgtype.UUID
	revision int64
	state    string
}

func advanceHeads(ctx context.Context, write *workflowstore.IssueWrite, r Request, before, after workflowstatev2.CurrentPicture, concerns []workflowstatev2.Concern, sources map[workflowstatev2.Concern]sourceRef) error {
	for _, concern := range concerns {
		source, ok := sources[concern]
		if !ok {
			return fmt.Errorf("%w: no source for %s", ErrUnsupportedCommand, concern)
		}
		if _, err := write.AdvanceConcernHeadCAS(ctx, db.AdvanceIssueWorkflowConcernHeadCASParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ConcernType: string(concern), ExpectedRevision: int64(before.Revisions.Get(concern)), SourceRecordID: source.id, SourceRecordRevision: source.revision, State: source.state}); err != nil {
			return err
		}
	}
	return nil
}

func canonicalizePicture(p *workflowstatev2.CurrentPicture) {
	sort.Slice(p.Evidence.TechnicalReviews, func(i, j int) bool { return p.Evidence.TechnicalReviews[i].Ref < p.Evidence.TechnicalReviews[j].Ref })
	for _, list := range [][]workflowstatev2.BoundEvidence{p.Evidence.Runtime, p.Evidence.Business, p.Evidence.Restoration, p.Evidence.Other} {
		sort.Slice(list, func(i, j int) bool { return list[i].Ref < list[j].Ref })
	}
}

func (s Service) persist(ctx context.Context, write *workflowstore.IssueWrite, r Request, principal Principal, commandID googleuuid.UUID, before, after workflowstatev2.CurrentPicture, concerns []workflowstatev2.Concern, fingerprint string) ([]admittedAction, map[workflowstatev2.Concern]sourceRef, error) {
	sources := map[workflowstatev2.Concern]sourceRef{}
	actions, err := deriveActions(r, commandID, after)
	if err != nil {
		return nil, nil, err
	}
	var reviewResultAuthority *db.LockWorkflowReviewResultAuthorityRow
	if isReviewResultKind(r.Kind) && r.Correction != nil && r.CommandKey == "task/"+util.UUIDToString(principal.TaskID)+"/review-result" {
		authority, lockErr := write.LockReviewResultAuthority(ctx, db.LockWorkflowReviewResultAuthorityParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, TaskID: principal.TaskID})
		if lockErr != nil {
			return nil, nil, fmt.Errorf("lock review result authority: %w", lockErr)
		}
		reviewResultAuthority = &authority
	}
	for i := range actions {
		a := &actions[i]
		if a.Type == "rollback_exact_release" {
			sourceAction, sourceErr := write.GetActionByKey(ctx, r.WorkspaceID, r.IssueID, principal.ActionKey)
			var frozen releaseEffectSnapshot
			if sourceErr != nil || json.Unmarshal(sourceAction.Payload, &frozen) != nil || after.Release.ReleaseID == "" || after.Release.RecoveryOfReleaseID != frozen.ReleaseID {
				return nil, nil, ErrInvalidAuthoritativeState
			}
			a.ReleaseID, a.TargetKey = after.Release.ReleaseID, after.Release.TargetID
			payload := map[string]any{}
			if json.Unmarshal(sourceAction.Payload, &payload) != nil {
				return nil, nil, ErrInvalidAuthoritativeState
			}
			knownGood, knownGoodErr := s.loadKnownGoodRecoverySnapshot(ctx, write, r.WorkspaceID, r.IssueID, frozen.TargetKey, frozen.ExpectedTargetGeneration, mustUUID(frozen.ExpectedTargetReleaseID))
			if knownGoodErr != nil {
				return nil, nil, ErrInvalidAuthoritativeState
			}
			payload["command_hash"], payload["release_id"], payload["release_revision"] = fingerprint, after.Release.ReleaseID, int64(1)
			payload["release_kind"], payload["recovery_of_release_id"] = "recovery", frozen.ReleaseID
			payload["candidate_id"], payload["candidate_revision"] = after.Release.CandidateID, after.Release.CandidateRevision
			payload["expected_target_release_id"], payload["expected_target_generation"] = frozen.ReleaseID, frozen.DesiredTargetGeneration
			payload["desired_target_generation"] = after.Release.Generation
			payload["build_manifest_digest"], payload["artifact_digest"], payload["runtime_contract_digest"] = knownGood.release.BuildManifestDigest.String, after.Release.ArtifactDigest, after.Release.RuntimeContractDigest
			payload["review_id"], payload["review_revision"] = util.UUIDToString(knownGood.review.ID), int64(1)
			payload["known_good_candidate_id"], payload["known_good_candidate_revision"] = util.UUIDToString(knownGood.candidate.ID), knownGood.candidate.Revision
			payload["expected"] = allRevisionMap(after)
			a.Payload, _ = json.Marshal(payload)
		} else if a.Type == "verify_restoration_exact_release" && r.Kind == workflowstatev2.CommandRollbackSucceeded {
			sourceAction, sourceErr := write.GetActionByKey(ctx, r.WorkspaceID, r.IssueID, principal.ActionKey)
			var frozen releaseEffectSnapshot
			if sourceErr != nil || json.Unmarshal(sourceAction.Payload, &frozen) != nil || after.Release.ReleaseID != frozen.ReleaseID {
				return nil, nil, ErrInvalidAuthoritativeState
			}
			a.ReleaseID, a.TargetKey = after.Release.ReleaseID, after.Release.TargetID
			payload := map[string]any{}
			if json.Unmarshal(sourceAction.Payload, &payload) != nil {
				return nil, nil, ErrInvalidAuthoritativeState
			}
			payload["command_hash"], payload["release_revision"], payload["expected"] = fingerprint, frozen.ReleaseRevision+1, allRevisionMap(after)
			a.Payload, _ = json.Marshal(payload)
		} else if a.Type == "prepare_release" || a.Type == "deploy_exact_release" || a.Type == "expose_exact_release" || a.Type == "verify_runtime_exact_release" || a.Type == "verify_business_exact_release" || a.Type == "monitor_exact_release" || a.Type == "verify_restoration_exact_release" {
			envelope, authority, review, authorityDigest, err := s.releaseAuthority(ctx, write, r, before)
			if err != nil {
				return nil, nil, err
			}
			releaseHead, err := concernHead(ctx, write, r, workflowstatev2.ConcernRelease)
			if err != nil {
				return nil, nil, err
			}
			release, err := write.GetRelease(ctx, db.GetIssueWorkflowReleaseParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: releaseHead.SourceRecordID})
			if err != nil {
				return nil, nil, err
			}
			releaseRevision := release.Revision + 1
			// Runtime proof advances only the evidence concern. The durable business
			// verification action must therefore freeze the still-current release
			// revision rather than predict a release CAS that does not occur.
			if r.Kind == workflowstatev2.CommandVerifyRuntime {
				releaseRevision = release.Revision
			}
			a.ReleaseID, a.TargetKey = util.UUIDToString(release.ID), release.TargetKey
			a.Payload, _ = json.Marshal(map[string]any{
				"contract_version": 1, "effect_contract": "observe_before_apply_v1", "command_hash": fingerprint,
				"operation_id": envelope.OperationID, "branch_key": envelope.BranchKey,
				"release_id": util.UUIDToString(release.ID), "release_revision": releaseRevision,
				"candidate_id": envelope.CandidateID, "candidate_revision": envelope.CandidateRevision,
				"scope_id": envelope.ScopeID, "scope_revision": envelope.ScopeRevision, "acceptance_version": envelope.AcceptanceVersion,
				"review_id": util.UUIDToString(review.ID), "review_revision": int64(1),
				"authority_id": util.UUIDToString(authority.ID), "authority_revision": authority.Revision,
				"authority_capability": envelope.Capability, "authority_envelope_digest": authorityDigest,
				"authority_evidence_id": envelope.OwnerEvidenceID, "owner_evidence_revision": envelope.OwnerEvidenceRevision,
				"target_key": envelope.TargetKey, "expected_target_release_id": envelope.ExpectedTargetReleaseID,
				"expected_target_generation": envelope.ExpectedTargetGeneration, "desired_target_generation": envelope.DesiredTargetGeneration,
				"release_operator_id": envelope.ReleaseOperatorID, "planned_verifier_id": envelope.PlannedVerifierID,
				"build_manifest_digest": envelope.BuildManifestDigest, "artifact_digest": envelope.ArtifactDigest,
				"runtime_contract_digest": envelope.RuntimeContractDigest, "exposure_policy": envelope.ExposurePolicy,
				"audience_policy": envelope.AudiencePolicy, "risk_envelope": envelope.RiskEnvelope,
				"exclusions": envelope.Exclusions, "budget_envelope": envelope.BudgetEnvelope,
				"recovery_policy": envelope.RecoveryPolicy, "recovery_policy_revision": envelope.RecoveryPolicyRevision,
				"rollback_target_release_id": envelope.RollbackTargetReleaseID, "irreversible_effects": envelope.IrreversibleEffects,
				"expected": allRevisionMap(after),
			})
		} else if a.Type == "review_correction" {
			candidate, err := write.GetCandidate(ctx, db.GetIssueWorkflowCandidateParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: mustUUID(after.Review.CandidateID)})
			if err != nil || !candidate.CreatedBy.Valid || candidate.ScopeRevision != int64(after.Scope.Revision) {
				return nil, nil, fmt.Errorf("freeze correction candidate: %w", ErrInvalidAuthoritativeState)
			}
			scope, err := write.GetScope(ctx, db.GetIssueWorkflowScopeParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: candidate.ScopeID})
			if err != nil || scope.Revision != candidate.ScopeRevision || scope.AcceptanceVersion != int64(after.Scope.AcceptanceVersion) {
				return nil, nil, fmt.Errorf("freeze correction scope (err=%v scope_revision=%d candidate_scope_revision=%d acceptance=%d expected_acceptance=%d): %w", err, scope.Revision, candidate.ScopeRevision, scope.AcceptanceVersion, after.Scope.AcceptanceVersion, ErrInvalidAuthoritativeState)
			}
			execution, err := selectedExecution(ctx, write, r.WorkspaceID, r.IssueID)
			if err != nil || execution.ID != mustUUID(a.OperationID) || execution.BranchKey == "" {
				return nil, nil, fmt.Errorf("freeze correction execution: %w", ErrInvalidAuthoritativeState)
			}
			correctionCandidateID := stableUUID(r.WorkspaceID, r.IssueID, a.Key, "correction-candidate")
			a.Payload, _ = json.Marshal(map[string]any{
				"operation_id":                  a.OperationID,
				"branch_key":                    execution.BranchKey,
				"candidate_id":                  util.UUIDToString(candidate.ID),
				"candidate_revision":            candidate.Revision,
				"correction_candidate_id":       correctionCandidateID.String(),
				"correction_candidate_revision": int64(1),
				"preview_source_revision":       previewSourceRevision(correctionCandidateID),
				"scope_id":                      util.UUIDToString(scope.ID),
				"scope_revision":                scope.Revision,
				"acceptance_version":            scope.AcceptanceVersion,
				"review_id":                     stableUUID(r.WorkspaceID, r.IssueID, r.CommandKey, "review").String(),
				"review_revision":               int64(1),
				"reviewer_id":                   after.Review.ReviewerID,
				"correction_worker_id":          util.UUIDToString(candidate.CreatedBy),
				"criteria_refs":                 append([]string(nil), after.Review.CriteriaRefs...),
				"coverage_refs":                 append([]string(nil), after.Review.CoverageRefs...),
				"gap_refs":                      append([]string(nil), after.Review.GapRefs...),
				"required_correction":           after.Review.RequiredCorrection,
				"correction_count":              after.Review.CorrectionCount,
				"max_corrections":               after.Review.MaxCorrections,
			})
		} else if a.Type == "review_candidate" {
			authority, err := write.LockCorrectionResultAuthority(ctx, db.LockWorkflowCorrectionResultAuthorityParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, TaskID: principal.TaskID})
			if err != nil || !authority.BranchKey.Valid || !authority.ReviewerID.Valid || !authority.ReviewerRuntimeID.Valid {
				return nil, nil, ErrInvalidAuthoritativeState
			}

			scope, err := write.GetScope(ctx, db.GetIssueWorkflowScopeParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: authority.ScopeID})
			if err != nil || scope.Revision != authority.ScopeRevision || scope.AcceptanceVersion != authority.AcceptanceVersion || r.Correction == nil {
				return nil, nil, ErrInvalidAuthoritativeState
			}
			a.Payload, _ = json.Marshal(map[string]any{
				"operation_id":                 a.OperationID,
				"branch_key":                   authority.BranchKey.String,
				"candidate_id":                 after.Review.CandidateID,
				"candidate_revision":           int64(1),
				"scope_id":                     util.UUIDToString(authority.ScopeID),
				"scope_revision":               authority.ScopeRevision,
				"acceptance_version":           authority.AcceptanceVersion,
				"reviewer_id":                  util.UUIDToString(authority.ReviewerID),
				"reviewer_runtime_id":          util.UUIDToString(authority.ReviewerRuntimeID),
				"review_id":                    stableUUID(r.WorkspaceID, r.IssueID, r.CommandKey, "review").String(),
				"review_revision":              int64(1),
				"implementation_evidence_refs": append([]string(nil), afterEvidenceRefs(before, after)...),
				"candidate_source_revision":    r.Correction.SourceRevision,
				"candidate_artifact":           r.Correction.Artifact,
				"candidate_build_contract":     r.Correction.BuildContract,
				"scope_statement":              json.RawMessage(scope.Statement),
				"scope_impact":                 json.RawMessage(scope.Impact),
				"typed_result_contract": map[string]any{
					"type": "review", "verdicts": []string{"passed", "failed", "inconclusive", "blocked_evidence"},
					"required": []string{"evidence", "criteria_refs", "coverage_refs"},
				},
			})
		} else if a.Type == "finalize_non_live" {
			a.Payload, _ = json.Marshal(map[string]any{
				"operation_id":         after.Execution.OperationID,
				"candidate_id":         after.Review.CandidateID,
				"scope_revision":       after.Scope.Revision,
				"acceptance_version":   after.Scope.AcceptanceVersion,
				"source_plan_revision": after.Scope.SourcePlanRevision,
				"requires_deployment":  false,
				"expected": map[string]workflowstatev2.Revision{
					"scope": after.Revisions.Scope, "review": after.Revisions.Review,
					"evidence": after.Revisions.Evidence, "outcome": after.Revisions.Outcome,
					"execution": after.Revisions.Execution, "release": after.Revisions.Release,
				},
			})
		} else if a.Type == "owner_decision_requested" {
			decisionID := stableUUID(r.WorkspaceID, r.IssueID, r.CommandKey, "decision")
			attentionID := stableUUID(r.WorkspaceID, r.IssueID, r.CommandKey, "attention")
			candidate, err := write.GetCandidate(ctx, db.GetIssueWorkflowCandidateParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: mustUUID(r.CandidateID)})
			if err != nil {
				return nil, nil, ErrInvalidAuthoritativeState
			}
			a.Payload, _ = json.Marshal(map[string]any{
				"decision_id": a.DecisionID, "decision_revision": int64(1), "decision_type": string(workflowstatev2.DecisionAuthorization),
				"accountable_owner_id": util.UUIDToString(r.Decision.AccountableOwnerID), "attention_id": attentionID.String(), "attention_revision": int64(1),
				"scope_revision": after.Scope.Revision, "authority_revision": after.Authority.Revision, "candidate_id": r.CandidateID, "candidate_revision": candidate.Revision,
				"operation_id": a.OperationID, "branch_key": after.Decisions[decisionID.String()].BranchID,
				"suspended_state": string(after.Decisions[decisionID.String()].SuspendedState), "suspended_execution_revision": after.Revisions.Execution, "authority_gap": r.Decision.AuthorityGap,
				"response_schema": map[string]any{"type": "authorization", "outcomes": []string{"approve", "reject"}},
			})
		} else if a.Type == "resume_exact_operation" || a.Type == "replan_exact_operation" {
			d := after.Decisions[r.DecisionID]
			relation, err := exactDecisionRequestRelation(ctx, write, r.WorkspaceID, r.IssueID, mustUUID(r.DecisionID))
			if err != nil {
				return nil, nil, err
			}
			candidate, err := write.GetCandidate(ctx, db.GetIssueWorkflowCandidateParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: mustUUID(d.CandidateID)})
			if err != nil {
				return nil, nil, ErrInvalidAuthoritativeState
			}
			responseID := stableUUID(r.WorkspaceID, r.IssueID, r.CommandKey, "decision-response")
			a.Payload, _ = json.Marshal(map[string]any{
				"decision_id": r.DecisionID, "decision_revision": d.Revision, "decision_type": string(d.Type), "outcome": string(d.Response),
				"scope_revision": d.ScopeRevision, "authority_revision": d.AuthorityRevision, "candidate_id": d.CandidateID, "candidate_revision": candidate.Revision, "target_key": d.TargetID,
				"operation_id": d.SuspendedOperationID, "branch_key": d.BranchID, "suspended_state": string(d.SuspendedState),
				"execution_revision": after.Revisions.Execution, "attention_id": util.UUIDToString(relation.attention.ID), "attention_revision": after.Revisions.Attention,
				"response_id": responseID.String(), "response_revision": relation.decisionRevision,
				"source_request_action_id": util.UUIDToString(relation.actionID), "source_request_action_key": relation.actionKey, "source_request_action_revision": relation.actionRevision,
				"expected": map[string]workflowstatev2.Revision{"execution": after.Revisions.Execution, "attention": after.Revisions.Attention},
			})
		}
		created, err := write.CreateAction(ctx, db.CreateIssueWorkflowActionReceiptParams{
			ID: mustUUID(a.ID), WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ActionKey: a.Key,
			RequestHash: fingerprint, CommandID: pgUUID(commandID), ActionType: a.Type,
			OperationID: mustUUID(a.OperationID), DecisionID: mustUUID(a.DecisionID), ReleaseID: mustUUID(a.ReleaseID), TargetKey: textValue(a.TargetKey), Payload: a.Payload,
			NotBefore: pgtype.Timestamptz{Time: s.Now().UTC(), Valid: true}, MaxAttempts: 3,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("create %s action: %w", a.Type, err)
		}
		if created.State != "admitted" {
			return nil, nil, ErrIdempotencyConflict
		}
	}
	if err := s.fail("after_actions"); err != nil {
		return nil, nil, err
	}

	execution, err := selectedExecution(ctx, write, r.WorkspaceID, r.IssueID)
	if err != nil {
		return nil, nil, fmt.Errorf("select execution for %s persistence: %w", r.Kind, err)
	}
	if containsConcern(concerns, workflowstatev2.ConcernExecution) && r.Kind != workflowstatev2.CommandSubmitCorrection {
		terminalReason := execution.TerminalReason
		checkpoint := execution.Checkpoint
		candidateID, targetKey := execution.CandidateID, execution.TargetKey
		actorType, actorID := execution.ActorType, execution.ActorID
		if r.Kind == workflowstatev2.CommandStartDeployment {
			candidateID = mustUUID(after.Release.CandidateID)
			targetKey = pgtype.Text{String: after.Execution.RunnableTargetID, Valid: after.Execution.RunnableTargetID != ""}
			actorType = pgtype.Text{String: "release_operator", Valid: true}
			actorID = principal.ActorID
		}
		if r.Kind == workflowstatev2.CommandTechnicalStop {
			terminalReason = r.Detail
			var fields map[string]any
			if err := json.Unmarshal(checkpoint, &fields); err != nil || fields == nil {
				return nil, nil, ErrInvalidAuthoritativeState
			}
			fields["suspended_state"] = string(before.Execution.State)
			fields["suspended_operation_id"] = before.Execution.OperationID
			checkpoint, _ = json.Marshal(fields)
		}
		if r.Kind == workflowstatev2.CommandProductionFailed {
			terminalReason = "current production regression detected; recovery remains unsatisfied"
		}
		updated, err := write.UpdateExecution(ctx, db.UpdateIssueWorkflowExecutionCASParams{
			State: string(after.Execution.State), AuthorityID: execution.AuthorityID, AuthorityRevision: execution.AuthorityRevision,
			CandidateID: candidateID, TargetKey: targetKey, ActorType: actorType, ActorID: actorID, TaskID: execution.TaskID,
			Checkpoint: checkpoint, ExpectedEvent: execution.ExpectedEvent, WaitReason: execution.WaitReason,
			LeaseToken: execution.LeaseToken, LeaseExpiresAt: execution.LeaseExpiresAt, NextActionAt: execution.NextActionAt,
			AttemptCount: execution.AttemptCount, TerminalReason: terminalReason, WorkspaceID: r.WorkspaceID, IssueID: r.IssueID,
			ID: execution.ID, ExpectedRevision: execution.Revision,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("advance execution for %s: %w", r.Kind, err)
		}
		sources[workflowstatev2.ConcernExecution] = sourceRef{updated.ID, updated.Revision, updated.State}
	}

	switch r.Kind {
	case workflowstatev2.CommandPrepareRelease:
		envelope, authority, review, authorityDigest, err := s.releaseAuthority(ctx, write, r, before)
		if err != nil {
			return nil, nil, err
		}
		releaseHead, err := concernHead(ctx, write, r, workflowstatev2.ConcernRelease)
		if err != nil {
			return nil, nil, err
		}
		facets, _ := json.Marshal(map[string]any{"release_operator_id": envelope.ReleaseOperatorID, "planned_verifier_id": envelope.PlannedVerifierID, "exposure_policy": envelope.ExposurePolicy, "audience_policy": envelope.AudiencePolicy, "risk_envelope": envelope.RiskEnvelope, "exclusions": envelope.Exclusions, "budget_envelope": envelope.BudgetEnvelope})
		recovery, _ := json.Marshal(map[string]any{"policy": envelope.RecoveryPolicy, "revision": envelope.RecoveryPolicyRevision, "rollback_target_release_id": envelope.RollbackTargetReleaseID, "irreversible_effects": envelope.IrreversibleEffects})
		updated, err := write.AdmitPendingReleaseAttempt(ctx, db.AdmitPendingIssueWorkflowReleaseAttemptCASParams{CandidateID: mustUUID(envelope.CandidateID), CandidateRevision: envelope.CandidateRevision, ScopeID: mustUUID(envelope.ScopeID), ScopeRevision: envelope.ScopeRevision, AuthorityID: authority.ID, AuthorityRevision: authority.Revision, TargetKey: envelope.TargetKey, TargetGeneration: envelope.DesiredTargetGeneration, Facets: facets, RecoveryPolicy: recovery, ExposedPopulation: []byte(`{}`), AcceptanceVersion: nullableInt64(envelope.AcceptanceVersion), OperationID: mustUUID(envelope.OperationID), BranchKey: envelope.BranchKey, ExpectedTargetReleaseID: mustUUID(envelope.ExpectedTargetReleaseID), ExpectedTargetGeneration: nullableInt64(envelope.ExpectedTargetGeneration), ReleaseOperatorID: mustUUID(envelope.ReleaseOperatorID), PlannedVerifierID: mustUUID(envelope.PlannedVerifierID), BuildManifestDigest: pgtype.Text{String: envelope.BuildManifestDigest, Valid: true}, ArtifactDigest: pgtype.Text{String: envelope.ArtifactDigest, Valid: true}, RuntimeContractDigest: pgtype.Text{String: envelope.RuntimeContractDigest, Valid: true}, AuthorityCapability: pgtype.Text{String: envelope.Capability, Valid: true}, AuthorityEvidenceID: authority.OwnerEvidenceID, RecoveryPolicyRevision: nullableInt64(envelope.RecoveryPolicyRevision), AuthorityEnvelopeDigest: pgtype.Text{String: authorityDigest, Valid: true}, OwnerEvidenceRevision: nullableInt64(envelope.OwnerEvidenceRevision), ExposurePolicyDigest: pgtype.Text{String: hashCanonical(envelope.ExposurePolicy), Valid: true}, RollbackTargetReleaseID: mustUUID(envelope.RollbackTargetReleaseID), IrreversibleEffectsDigest: pgtype.Text{String: hashCanonical(envelope.IrreversibleEffects), Valid: true}, ReviewedReviewID: review.ID, ReviewedReviewRevision: nullableInt64(1), WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: releaseHead.SourceRecordID, ExpectedRevision: releaseHead.SourceRecordRevision})
		if err != nil {
			return nil, nil, err
		}
		sources[workflowstatev2.ConcernRelease] = sourceRef{updated.ID, updated.Revision, updated.State}
	case workflowstatev2.CommandBuildRelease:
		releaseHead, err := concernHead(ctx, write, r, workflowstatev2.ConcernRelease)
		if err != nil {
			return nil, nil, err
		}
		release, err := write.GetRelease(ctx, db.GetIssueWorkflowReleaseParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: releaseHead.SourceRecordID})
		if err != nil {
			return nil, nil, err
		}
		updated, err := write.UpdateRelease(ctx, db.UpdateIssueWorkflowReleaseCASParams{State: "built", Facets: release.Facets, ExposedPopulation: release.ExposedPopulation, WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: release.ID, ExpectedRevision: release.Revision})
		if err != nil {
			return nil, nil, err
		}
		newRefs := afterEvidenceRefs(before, after)
		if len(newRefs) != 1 || r.ReleaseBuild == nil {
			return nil, nil, ErrInvalidAuthoritativeState
		}
		payload, _ := json.Marshal(map[string]any{"freshness": "current", "candidate_revision": release.CandidateRevision, "release_id": util.UUIDToString(release.ID), "release_revision": updated.Revision, "scope_id": util.UUIDToString(release.ScopeID), "scope_revision": release.ScopeRevision, "acceptance_version": release.AcceptanceVersion.Int64, "target_generation": release.TargetGeneration, "build_manifest_digest": r.ReleaseBuild.BuildManifestDigest, "artifact_digest": r.ReleaseBuild.ArtifactDigest, "runtime_contract_digest": r.ReleaseBuild.RuntimeContractDigest, "reproducible": true, "coverage": r.ReleaseBuild.Coverage, "task_id": util.UUIDToString(principal.TaskID)})
		evidence, err := write.CreateEvidence(ctx, db.CreateIssueWorkflowEvidenceParams{ID: mustUUID(newRefs[0]), WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ConcernType: "release", ConcernID: release.ID, ConcernRevision: nullableInt64(updated.Revision), EvidenceType: "reproducible_build", Level: "build", Source: "release_operator_task", ProducerType: "release_operator", ProducerID: principal.ActorID, CandidateID: release.CandidateID, TargetKey: pgtype.Text{String: release.TargetKey, Valid: true}, ObservedAt: pgtype.Timestamptz{Time: s.Now().UTC(), Valid: true}, Coverage: []byte(`{}`), Limitations: []byte(`[]`), Contradictions: []byte(`[]`), Payload: payload})
		if err != nil {
			return nil, nil, err
		}
		sources[workflowstatev2.ConcernRelease], sources[workflowstatev2.ConcernEvidence] = sourceRef{updated.ID, updated.Revision, updated.State}, sourceRef{evidence.ID, 1, string(after.Evidence.State)}
	case workflowstatev2.CommandAuthorizeRelease:
		envelope, authority, _, authorityDigest, err := s.releaseAuthority(ctx, write, r, before)
		if err != nil {
			return nil, nil, err
		}
		releaseHead, err := concernHead(ctx, write, r, workflowstatev2.ConcernRelease)
		if err != nil {
			return nil, nil, err
		}
		release, err := write.GetRelease(ctx, db.GetIssueWorkflowReleaseParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: releaseHead.SourceRecordID})
		if err != nil {
			return nil, nil, err
		}
		updated, err := write.UpdateRelease(ctx, db.UpdateIssueWorkflowReleaseCASParams{State: "authorized", Facets: release.Facets, ExposedPopulation: release.ExposedPopulation, WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: release.ID, ExpectedRevision: release.Revision})
		if err != nil {
			return nil, nil, err
		}
		newRefs := afterEvidenceRefs(before, after)
		if len(newRefs) != 1 {
			return nil, nil, ErrInvalidAuthoritativeState
		}
		payload, _ := json.Marshal(map[string]any{"freshness": "current", "candidate_revision": release.CandidateRevision, "release_id": util.UUIDToString(release.ID), "release_revision": updated.Revision, "scope_id": util.UUIDToString(release.ScopeID), "scope_revision": release.ScopeRevision, "acceptance_version": release.AcceptanceVersion.Int64, "target_generation": release.TargetGeneration, "authority_id": util.UUIDToString(authority.ID), "authority_revision": authority.Revision, "authority_envelope_digest": authorityDigest, "owner_evidence_id": envelope.OwnerEvidenceID, "owner_evidence_revision": envelope.OwnerEvidenceRevision})
		evidence, err := write.CreateEvidence(ctx, db.CreateIssueWorkflowEvidenceParams{ID: mustUUID(newRefs[0]), WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ConcernType: "authority", ConcernID: authority.ID, ConcernRevision: nullableInt64(authority.Revision), EvidenceType: "deployment_authorization_snapshot", Level: "owner", Source: "normalized_authority", ProducerType: "controller", ProducerID: principal.ActorID, CandidateID: release.CandidateID, TargetKey: pgtype.Text{String: release.TargetKey, Valid: true}, ObservedAt: pgtype.Timestamptz{Time: s.Now().UTC(), Valid: true}, Coverage: []byte(`{}`), Limitations: []byte(`[]`), Contradictions: []byte(`[]`), Payload: payload})
		if err != nil {
			return nil, nil, err
		}
		sources[workflowstatev2.ConcernRelease], sources[workflowstatev2.ConcernEvidence] = sourceRef{updated.ID, updated.Revision, updated.State}, sourceRef{evidence.ID, 1, string(after.Evidence.State)}
	case workflowstatev2.CommandProductionFailed:
		if r.ReleaseEffect == nil {
			return nil, nil, ErrInvalidAuthoritativeState
		}
		forward, err := write.GetRelease(ctx, db.GetIssueWorkflowReleaseParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: mustUUID(r.ReleaseEffect.ReleaseID)})
		if err != nil || forward.State != "live" {
			return nil, nil, ErrInvalidAuthoritativeState
		}
		newRefs := afterEvidenceRefs(before, after)
		if len(newRefs) != 1 {
			return nil, nil, ErrInvalidAuthoritativeState
		}
		boundReleaseRevision := forward.Revision
		if after.Release.State != workflowstatev2.ReleaseRollingBack {
			boundReleaseRevision++
		}
		evidencePayload := cloneMap(r.ReleaseEffect.Evidence)
		evidencePayload["freshness"], evidencePayload["release_id"], evidencePayload["release_revision"] = "contradicted", util.UUIDToString(forward.ID), boundReleaseRevision
		evidencePayload["candidate_revision"], evidencePayload["scope_id"], evidencePayload["scope_revision"] = forward.CandidateRevision, util.UUIDToString(forward.ScopeID), forward.ScopeRevision
		evidencePayload["acceptance_version"], evidencePayload["target_generation"] = forward.AcceptanceVersion.Int64, forward.TargetGeneration
		evidencePayload["operation_id"], evidencePayload["branch_key"] = util.UUIDToString(forward.OperationID), forward.BranchKey
		evidenceJSON, _ := json.Marshal(evidencePayload)
		evidence, err := write.CreateEvidence(ctx, db.CreateIssueWorkflowEvidenceParams{ID: mustUUID(newRefs[0]), WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ConcernType: "release", ConcernID: forward.ID, ConcernRevision: nullableInt64(boundReleaseRevision), EvidenceType: "regression", Level: "production", Source: "workflow_release_monitor", ProducerType: "verifier", ProducerID: principal.ActorID, CandidateID: forward.CandidateID, TargetKey: pgtype.Text{String: forward.TargetKey, Valid: true}, ObservedAt: pgtype.Timestamptz{Time: s.Now().UTC(), Valid: true}, Coverage: mustJSON(r.ReleaseEffect.Evidence["regression_proof"]), Limitations: []byte(`[]`), Contradictions: []byte(`["current production regression"]`), Payload: evidenceJSON})
		if err != nil {
			return nil, nil, err
		}
		sources[workflowstatev2.ConcernEvidence] = sourceRef{evidence.ID, 1, string(after.Evidence.State)}
		recoveryActionID := pgtype.UUID{}
		if len(actions) == 1 && actions[0].Type == "rollback_exact_release" {
			recoveryActionID = mustUUID(actions[0].ID)
		}
		fingerprint := hashCanonical(map[string]any{"release_id": util.UUIDToString(forward.ID), "release_revision": forward.Revision, "candidate_id": util.UUIDToString(forward.CandidateID), "candidate_revision": forward.CandidateRevision, "target_key": forward.TargetKey, "target_generation": forward.TargetGeneration, "evidence_id": util.UUIDToString(evidence.ID)})
		incidentID := stableUUID(r.WorkspaceID, r.IssueID, fingerprint, "production-regression")
		compatibility := "containment_required"
		recoveryReleaseID := pgtype.UUID{}
		if after.Release.State == workflowstatev2.ReleaseRollingBack {
			compatibility = "rollback_safe"
			recoveryReleaseID = mustUUID(after.Release.ReleaseID)
		}
		incidentRecovery, _ := json.Marshal(map[string]any{"mode": string(cRecoveryMode(before, after)), "failed_release_id": util.UUIDToString(forward.ID), "recovery_release_id": after.Release.ReleaseID, "expected_target_generation": forward.TargetGeneration, "desired_target_generation": after.Release.Generation})
		if _, err := write.CreateReleaseIncident(ctx, db.CreateIssueWorkflowReleaseIncidentParams{ID: pgUUID(incidentID), WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ReleaseID: forward.ID, ReleaseRevision: nullableInt64(boundReleaseRevision), CandidateID: forward.CandidateID, CandidateRevision: nullableInt64(forward.CandidateRevision), TargetKey: pgtype.Text{String: forward.TargetKey, Valid: true}, TargetGeneration: nullableInt64(forward.TargetGeneration), DetectionEvidenceID: evidence.ID, RecoveryActionID: recoveryActionID, CompatibilityState: pgtype.Text{String: compatibility, Valid: true}, RegressionFingerprint: pgtype.Text{String: fingerprint, Valid: true}, RecoveryReleaseID: recoveryReleaseID, IncidentType: "production_regression", Severity: "critical", State: "open", Consequence: []byte(`{"requested_outcome":"unsatisfied"}`), Recovery: incidentRecovery}); err != nil {
			return nil, nil, fmt.Errorf("create production regression incident (compatibility=%s recovery_release=%q release_revision=%d): %w", compatibility, after.Release.ReleaseID, boundReleaseRevision, err)
		}
		if after.Release.State == workflowstatev2.ReleaseRollingBack {
			knownGood, knownGoodErr := s.loadKnownGoodRecoverySnapshot(ctx, write, r.WorkspaceID, r.IssueID, forward.TargetKey, forward.ExpectedTargetGeneration.Int64, forward.RollbackTargetReleaseID)
			if knownGoodErr != nil {
				return nil, nil, ErrInvalidAuthoritativeState
			}
			if _, err := write.CreateCandidate(ctx, db.CreateIssueWorkflowCandidateParams{ID: mustUUID(after.Release.CandidateID), WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ScopeID: forward.ScopeID, ScopeRevision: forward.ScopeRevision, CandidateType: knownGood.candidate.CandidateType, State: "ready", SourceRevision: "recovery-snapshot:" + util.UUIDToString(knownGood.release.ID) + ":" + knownGood.candidate.SourceRevision, Artifact: knownGood.candidate.Artifact, BuildContract: knownGood.candidate.BuildContract, CreatedBy: knownGood.candidate.CreatedBy}); err != nil {
				return nil, nil, err
			}
			facets := knownGood.release.Facets
			recovery, err := write.CreateRecoveryReleaseAttempt(ctx, db.CreateIssueWorkflowRecoveryReleaseAttemptParams{ID: mustUUID(after.Release.ReleaseID), WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, CandidateID: mustUUID(after.Release.CandidateID), CandidateRevision: int64(after.Release.CandidateRevision), ScopeID: forward.ScopeID, ScopeRevision: forward.ScopeRevision, AuthorityID: forward.AuthorityID, AuthorityRevision: forward.AuthorityRevision, TargetKey: forward.TargetKey, TargetGeneration: int64(after.Release.Generation), Facets: facets, RecoveryPolicy: forward.RecoveryPolicy, AcceptanceVersion: forward.AcceptanceVersion, OperationID: forward.OperationID, BranchKey: forward.BranchKey, ExpectedTargetReleaseID: forward.ID, ExpectedTargetGeneration: nullableInt64(forward.TargetGeneration), RecoveryOfReleaseID: forward.ID, ReleaseOperatorID: forward.ReleaseOperatorID, PlannedVerifierID: forward.PlannedVerifierID, BuildManifestDigest: knownGood.release.BuildManifestDigest, ArtifactDigest: knownGood.release.ArtifactDigest, RuntimeContractDigest: knownGood.release.RuntimeContractDigest, AuthorityCapability: forward.AuthorityCapability, AuthorityEvidenceID: forward.AuthorityEvidenceID, RecoveryPolicyRevision: forward.RecoveryPolicyRevision, AuthorityEnvelopeDigest: forward.AuthorityEnvelopeDigest, OwnerEvidenceRevision: forward.OwnerEvidenceRevision, ExposurePolicyDigest: forward.ExposurePolicyDigest, RollbackTargetReleaseID: forward.RollbackTargetReleaseID, IrreversibleEffectsDigest: forward.IrreversibleEffectsDigest, ReviewedReviewID: knownGood.review.ID, ReviewedReviewRevision: nullableInt64(1)})
			if err != nil {
				return nil, nil, err
			}
			sources[workflowstatev2.ConcernRelease] = sourceRef{recovery.ID, recovery.Revision, recovery.State}
		} else {
			updated, err := write.UpdateRelease(ctx, db.UpdateIssueWorkflowReleaseCASParams{State: "degraded", Facets: forward.Facets, ExposedPopulation: forward.ExposedPopulation, StartedAt: forward.StartedAt, DeployedAt: forward.DeployedAt, RestoredAt: forward.RestoredAt, WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: forward.ID, ExpectedRevision: forward.Revision})
			if err != nil {
				return nil, nil, err
			}
			sources[workflowstatev2.ConcernRelease] = sourceRef{updated.ID, updated.Revision, updated.State}
			head, headErr := write.LockReleaseTargetHead(ctx, r.WorkspaceID, r.IssueID, forward.TargetKey)
			if headErr != nil || head.ReleaseID != forward.ID || head.Generation != forward.TargetGeneration || head.CandidateID != forward.CandidateID {
				return nil, nil, ErrInvalidAuthoritativeState
			}
			if _, err := write.UpdateReleaseTargetHeadState(ctx, db.UpdateIssueWorkflowTargetHeadStateCASParams{State: "degraded", WorkspaceID: r.WorkspaceID, TargetKey: forward.TargetKey, ReleaseID: forward.ID, IssueID: r.IssueID, CandidateID: forward.CandidateID, Generation: forward.TargetGeneration, ExpectedRevision: head.Revision}); err != nil {
				return nil, nil, err
			}
		}
		outcome, err := write.GetLatestOutcome(ctx, db.GetLatestIssueWorkflowOutcomeParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID})
		if err != nil {
			return nil, nil, err
		}
		updatedOutcome, err := write.UpdateOutcome(ctx, db.UpdateIssueWorkflowOutcomeCASParams{State: string(after.Outcome), AcceptanceVersion: int64(after.Scope.AcceptanceVersion), TerminalReason: "production regression reopened the requested outcome", WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: outcome.ID, ExpectedRevision: outcome.Revision})
		if err != nil {
			return nil, nil, err
		}
		sources[workflowstatev2.ConcernOutcome] = sourceRef{updatedOutcome.ID, updatedOutcome.Revision, updatedOutcome.State}
	case workflowstatev2.CommandRestoreVerified:
		if r.ReleaseEffect == nil {
			return nil, nil, ErrInvalidAuthoritativeState
		}
		releaseHead, err := concernHead(ctx, write, r, workflowstatev2.ConcernRelease)
		if err != nil {
			return nil, nil, err
		}
		release, err := write.GetRelease(ctx, db.GetIssueWorkflowReleaseParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: releaseHead.SourceRecordID})
		if err != nil || release.ReleaseKind != "recovery" || release.State != "rolled_back" || !release.RecoveryOfReleaseID.Valid {
			return nil, nil, ErrInvalidAuthoritativeState
		}
		restoredAt := pgtype.Timestamptz{Time: s.Now().UTC(), Valid: true}
		updated, err := write.UpdateRelease(ctx, db.UpdateIssueWorkflowReleaseCASParams{State: "restored", Facets: release.Facets, ExposedPopulation: release.ExposedPopulation, StartedAt: release.StartedAt, DeployedAt: release.DeployedAt, RestoredAt: restoredAt, WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: release.ID, ExpectedRevision: release.Revision})
		if err != nil {
			return nil, nil, err
		}
		newRefs := afterEvidenceRefs(before, after)
		if len(newRefs) != 4 {
			return nil, nil, ErrInvalidAuthoritativeState
		}
		proofs := []struct {
			typ string
			key string
		}{{"runtime", "runtime_proof"}, {"business", "business_proof"}, {"regression", "regression_proof"}, {"restoration", "restoration_proof"}}
		var firstEvidence db.IssueWorkflowEvidence
		for i, proof := range proofs {
			value := r.ReleaseEffect.Evidence[proof.key]
			payload := map[string]any{"freshness": "current", "proof": value, "provider_receipt_id": r.ReleaseEffect.ProviderReceiptID, "effect_key": r.ReleaseEffect.EffectKey, "release_id": util.UUIDToString(release.ID), "release_revision": updated.Revision, "candidate_revision": release.CandidateRevision, "scope_id": util.UUIDToString(release.ScopeID), "scope_revision": release.ScopeRevision, "acceptance_version": release.AcceptanceVersion.Int64, "target_generation": release.TargetGeneration, "operation_id": util.UUIDToString(release.OperationID), "branch_key": release.BranchKey}
			payloadJSON, _ := json.Marshal(payload)
			created, createErr := write.CreateEvidence(ctx, db.CreateIssueWorkflowEvidenceParams{ID: mustUUID(newRefs[i]), WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ConcernType: "release", ConcernID: release.ID, ConcernRevision: nullableInt64(updated.Revision), EvidenceType: proof.typ, Level: "production", Source: "workflow_restoration_task", ProducerType: "verifier", ProducerID: principal.ActorID, CandidateID: release.CandidateID, TargetKey: pgtype.Text{String: release.TargetKey, Valid: true}, ObservedAt: restoredAt, Coverage: mustJSON(value), Limitations: []byte(`[]`), Contradictions: []byte(`[]`), Payload: payloadJSON})
			if createErr != nil {
				return nil, nil, createErr
			}
			if i == 0 {
				firstEvidence = created
			}
		}
		sources[workflowstatev2.ConcernEvidence] = sourceRef{firstEvidence.ID, 1, string(after.Evidence.State)}
		sources[workflowstatev2.ConcernRelease] = sourceRef{updated.ID, updated.Revision, updated.State}
		head, err := write.LockReleaseTargetHead(ctx, r.WorkspaceID, r.IssueID, release.TargetKey)
		if err != nil || head.ReleaseID != release.ID || head.Generation != release.TargetGeneration || head.CandidateID != release.CandidateID {
			return nil, nil, ErrInvalidAuthoritativeState
		}
		if _, err := write.UpdateReleaseTargetHeadState(ctx, db.UpdateIssueWorkflowTargetHeadStateCASParams{State: "restored", WorkspaceID: r.WorkspaceID, TargetKey: release.TargetKey, ReleaseID: release.ID, IssueID: r.IssueID, CandidateID: release.CandidateID, Generation: release.TargetGeneration, ExpectedRevision: head.Revision}); err != nil {
			return nil, nil, err
		}
		incident, err := write.LockRecoveryIncident(ctx, db.LockIssueWorkflowRecoveryIncidentParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, RecoveryReleaseID: release.ID})
		if err != nil || !incident.RecoveryActionID.Valid || incident.RecoveryReleaseID != release.ID || incident.ReleaseID != release.RecoveryOfReleaseID {
			return nil, nil, ErrInvalidAuthoritativeState
		}
		if _, err := write.UpdateIncident(ctx, db.UpdateIssueWorkflowIncidentCASParams{State: "resolved", Consequence: incident.Consequence, Recovery: incident.Recovery, ResolvedAt: restoredAt, WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: incident.ID, ExpectedRevision: incident.Revision}); err != nil {
			return nil, nil, err
		}
	case workflowstatev2.CommandStartDeployment, workflowstatev2.CommandRecordDeployment, workflowstatev2.CommandRecordExposure, workflowstatev2.CommandBeginVerification, workflowstatev2.CommandVerifyRuntime, workflowstatev2.CommandVerifyBusiness, workflowstatev2.CommandRollbackSucceeded:
		if r.ReleaseEffect == nil {
			return nil, nil, ErrInvalidAuthoritativeState
		}
		releaseHead, err := concernHead(ctx, write, r, workflowstatev2.ConcernRelease)
		if err != nil {
			return nil, nil, err
		}
		release, err := write.GetRelease(ctx, db.GetIssueWorkflowReleaseParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: releaseHead.SourceRecordID})
		if err != nil || util.UUIDToString(release.ID) != r.ReleaseEffect.ReleaseID {
			return nil, nil, ErrInvalidAuthoritativeState
		}
		startedAt, deployedAt := release.StartedAt, release.DeployedAt
		if r.Kind == workflowstatev2.CommandStartDeployment && !startedAt.Valid {
			startedAt = pgtype.Timestamptz{Time: s.Now().UTC(), Valid: true}
		}
		if r.Kind == workflowstatev2.CommandRecordDeployment && !deployedAt.Valid {
			deployedAt = pgtype.Timestamptz{Time: s.Now().UTC(), Valid: true}
		}
		exposedPopulation := release.ExposedPopulation
		if r.Kind == workflowstatev2.CommandRecordExposure {
			exposedPopulation, _ = json.Marshal(r.ReleaseEffect.Evidence)
		}
		updated := release
		if r.Kind != workflowstatev2.CommandVerifyRuntime {
			updated, err = write.UpdateRelease(ctx, db.UpdateIssueWorkflowReleaseCASParams{State: string(after.Release.State), Facets: release.Facets, ExposedPopulation: exposedPopulation, StartedAt: startedAt, DeployedAt: deployedAt, RestoredAt: release.RestoredAt, WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: release.ID, ExpectedRevision: release.Revision})
			if err != nil {
				return nil, nil, err
			}
		}
		newRefs := afterEvidenceRefs(before, after)
		if len(newRefs) != 1 && !(r.Kind == workflowstatev2.CommandVerifyBusiness && len(newRefs) == 2) {
			return nil, nil, ErrInvalidAuthoritativeState
		}
		evidenceType, level, producerType := "deployment", "provider", "release_operator"
		switch r.Kind {
		case workflowstatev2.CommandStartDeployment:
			evidenceType = "deployment_started"
		case workflowstatev2.CommandRecordExposure:
			evidenceType = "exposure"
		case workflowstatev2.CommandBeginVerification:
			evidenceType, level, producerType = "verification_started", "production", "verifier"
		case workflowstatev2.CommandVerifyRuntime:
			evidenceType, level, producerType = "runtime", "production", "verifier"
		case workflowstatev2.CommandVerifyBusiness:
			evidenceType, level, producerType = "business", "production", "verifier"
		case workflowstatev2.CommandRollbackSucceeded:
			evidenceType, level, producerType = "rollback", "provider", "release_operator"
		}
		payload := cloneMap(r.ReleaseEffect.Evidence)
		payload["freshness"], payload["provider_receipt_id"], payload["effect_key"] = string(after.Evidence.State), r.ReleaseEffect.ProviderReceiptID, r.ReleaseEffect.EffectKey
		payload["candidate_revision"], payload["release_id"], payload["release_revision"] = release.CandidateRevision, util.UUIDToString(release.ID), updated.Revision
		payload["scope_id"], payload["scope_revision"], payload["acceptance_version"] = util.UUIDToString(release.ScopeID), release.ScopeRevision, release.AcceptanceVersion.Int64
		payload["target_generation"], payload["operation_id"], payload["branch_key"] = release.TargetGeneration, util.UUIDToString(release.OperationID), release.BranchKey
		var evidence db.IssueWorkflowEvidence
		for index, ref := range newRefs {
			rowType := evidenceType
			rowPayload := cloneMap(payload)
			coverageJSON := []byte(`{}`)
			if r.Kind == workflowstatev2.CommandVerifyRuntime {
				proof := r.ReleaseEffect.Evidence["runtime_proof"]
				rowPayload["proof"] = proof
				coverageJSON, _ = json.Marshal(proof)
			}
			if r.Kind == workflowstatev2.CommandVerifyBusiness && index == 1 {
				rowType = "regression"
			}
			if r.Kind == workflowstatev2.CommandVerifyBusiness {
				proofKey := "business_proof"
				if rowType == "regression" {
					proofKey = "regression_proof"
				}
				proof := r.ReleaseEffect.Evidence[proofKey]
				rowPayload["proof"] = proof
				coverageJSON, _ = json.Marshal(proof)
			}
			rowPayloadJSON, _ := json.Marshal(rowPayload)
			created, createErr := write.CreateEvidence(ctx, db.CreateIssueWorkflowEvidenceParams{ID: mustUUID(ref), WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ConcernType: "release", ConcernID: release.ID, ConcernRevision: nullableInt64(updated.Revision), EvidenceType: rowType, Level: level, Source: "workflow_release_effect", ProducerType: producerType, ProducerID: principal.ActorID, CandidateID: release.CandidateID, TargetKey: pgtype.Text{String: release.TargetKey, Valid: true}, ObservedAt: pgtype.Timestamptz{Time: s.Now().UTC(), Valid: true}, Coverage: coverageJSON, Limitations: []byte(`[]`), Contradictions: []byte(`[]`), Payload: rowPayloadJSON})
			if createErr != nil {
				return nil, nil, createErr
			}
			if index == 0 {
				evidence = created
			}
		}
		sources[workflowstatev2.ConcernEvidence] = sourceRef{evidence.ID, 1, string(after.Evidence.State)}
		if r.Kind != workflowstatev2.CommandVerifyRuntime {
			sources[workflowstatev2.ConcernRelease] = sourceRef{updated.ID, updated.Revision, updated.State}
		}
		if r.Kind == workflowstatev2.CommandRecordDeployment || r.Kind == workflowstatev2.CommandRollbackSucceeded {
			if _, err := write.AdvanceReleaseTargetHead(ctx, db.AdvanceIssueWorkflowTargetHeadCASParams{WorkspaceID: r.WorkspaceID, TargetKey: release.TargetKey, Generation: release.TargetGeneration, ReleaseID: release.ID, IssueID: r.IssueID, CandidateID: release.CandidateID, State: updated.State, ExpectedGeneration: release.ExpectedTargetGeneration.Int64}); err != nil {
				return nil, nil, err
			}
		} else if r.Kind == workflowstatev2.CommandRecordExposure || r.Kind == workflowstatev2.CommandBeginVerification || r.Kind == workflowstatev2.CommandVerifyBusiness {
			head, err := write.LockReleaseTargetHead(ctx, r.WorkspaceID, r.IssueID, release.TargetKey)
			if err != nil {
				return nil, nil, err
			}
			if _, err := write.UpdateReleaseTargetHeadState(ctx, db.UpdateIssueWorkflowTargetHeadStateCASParams{State: updated.State, WorkspaceID: r.WorkspaceID, TargetKey: release.TargetKey, ReleaseID: release.ID, IssueID: r.IssueID, CandidateID: release.CandidateID, Generation: release.TargetGeneration, ExpectedRevision: head.Revision}); err != nil {
				return nil, nil, err
			}
		}
		if r.Kind == workflowstatev2.CommandVerifyBusiness {
			outcome, err := write.GetLatestOutcome(ctx, db.GetLatestIssueWorkflowOutcomeParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID})
			if err != nil {
				return nil, nil, err
			}
			updatedOutcome, err := write.UpdateOutcome(ctx, db.UpdateIssueWorkflowOutcomeCASParams{State: string(after.Outcome), AcceptanceVersion: int64(after.Scope.AcceptanceVersion), CompletionEvidenceID: evidence.ID, TerminalReason: "verified live by independent runtime and business evidence", WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: outcome.ID, ExpectedRevision: outcome.Revision})
			if err != nil {
				return nil, nil, err
			}
			sources[workflowstatev2.ConcernOutcome] = sourceRef{updatedOutcome.ID, updatedOutcome.Revision, updatedOutcome.State}
		}
	case workflowstatev2.CommandSubmitCorrection:
		authority, err := write.LockCorrectionResultAuthority(ctx, db.LockWorkflowCorrectionResultAuthorityParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, TaskID: principal.TaskID})
		if err != nil || r.Correction == nil {
			return nil, nil, ErrInvalidAuthoritativeState
		}
		candidateID := mustUUID(after.Review.CandidateID)
		artifact, _ := json.Marshal(r.Correction.Artifact)
		buildContract, _ := json.Marshal(r.Correction.BuildContract)
		candidate, err := write.CreateCandidate(ctx, db.CreateIssueWorkflowCandidateParams{ID: candidateID, WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ScopeID: authority.ScopeID, ScopeRevision: authority.ScopeRevision, CandidateType: "correction", State: "ready_for_review", SourceRevision: r.Correction.SourceRevision, Artifact: artifact, BuildContract: buildContract, CreatedBy: principal.ActorID})
		if err != nil {
			return nil, nil, err
		}
		if _, err := write.SupersedeCandidate(ctx, db.SupersedeIssueWorkflowCandidateCASParams{SupersededBy: candidate.ID, SupersededAt: pgtype.Timestamptz{Time: s.Now().UTC(), Valid: true}, WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: authority.RejectedCandidateID, ExpectedRevision: authority.RejectedCandidateRevision}); err != nil {
			return nil, nil, err
		}
		evidenceID := mustUUID(after.Evidence.Other[len(after.Evidence.Other)-1].Ref)
		evidencePayload := cloneMap(r.Correction.Evidence)
		evidencePayload["candidate_revision"] = candidate.Revision
		evidencePayload["scope_id"] = util.UUIDToString(authority.ScopeID)
		evidencePayload["scope_revision"] = authority.ScopeRevision
		evidencePayload["acceptance_version"] = authority.AcceptanceVersion
		evidencePayload["freshness"] = "current"
		evidencePayload["task_id"] = util.UUIDToString(authority.TaskID)
		evidencePayload["workflow_action_id"] = util.UUIDToString(authority.WorkflowActionID)
		evidencePayload["operation_id"] = util.UUIDToString(authority.OperationID)
		evidencePayload["branch_key"] = authority.BranchKey.String
		evidencePayload["source_revision"] = r.Correction.SourceRevision
		evidenceJSON, _ := json.Marshal(evidencePayload)
		evidence, err := write.CreateEvidence(ctx, db.CreateIssueWorkflowEvidenceParams{ID: evidenceID, WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ConcernType: "implementation", ConcernID: candidate.ID, ConcernRevision: nullableInt64(candidate.Revision), EvidenceType: "implementation", Level: "local", Source: "workflow_correction_task", ProducerType: "worker", ProducerID: principal.ActorID, CandidateID: candidate.ID, ObservedAt: pgtype.Timestamptz{Time: s.Now().UTC(), Valid: true}, Coverage: []byte(`{}`), Limitations: []byte(`[]`), Contradictions: []byte(`[]`), Payload: evidenceJSON})
		if err != nil {
			return nil, nil, err
		}
		reviewID := stableUUID(r.WorkspaceID, r.IssueID, r.CommandKey, "review")
		review, err := write.CreateReview(ctx, db.CreateIssueWorkflowReviewParams{ID: pgUUID(reviewID), WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, CandidateID: candidate.ID, CandidateRevision: candidate.Revision, ScopeID: authority.ScopeID, ScopeRevision: authority.ScopeRevision, ReviewerID: authority.ReviewerID, Outcome: "in_progress", Criteria: []byte(`[]`), Coverage: []byte(`{}`), EvidenceRefs: []byte(`[]`), Gaps: []byte(`[]`), CorrectionPacket: []byte(`{}`), CorrectionCount: authority.CorrectionCount, MaxCorrections: authority.MaxCorrections})
		if err != nil {
			return nil, nil, err
		}
		releaseHead, err := concernHead(ctx, write, r, workflowstatev2.ConcernRelease)
		if err != nil {
			return nil, nil, err
		}
		release, err := write.RebindPendingReleaseCandidate(ctx, db.RebindPendingIssueWorkflowReleaseCandidateCASParams{
			CandidateID: candidate.ID, CandidateRevision: candidate.Revision,
			WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: releaseHead.SourceRecordID,
			ExpectedRevision: releaseHead.SourceRecordRevision, ExpectedCandidateID: authority.RejectedCandidateID,
			ExpectedCandidateRevision: authority.RejectedCandidateRevision,
		})
		if err != nil {
			return nil, nil, err
		}
		updatedExecution, err := write.UpdateCorrectionCandidate(ctx, db.UpdateIssueWorkflowExecutionCorrectionCandidateCASParams{State: string(after.Execution.State), CandidateID: candidate.ID, ActorType: pgtype.Text{String: "reviewer", Valid: true}, ActorID: authority.ReviewerID, WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: authority.OperationID, ExpectedRevision: authority.ExecutionRevision})
		if err != nil {
			return nil, nil, err
		}
		sources[workflowstatev2.ConcernExecution] = sourceRef{updatedExecution.ID, updatedExecution.Revision, updatedExecution.State}
		sources[workflowstatev2.ConcernReview] = sourceRef{review.ID, 1, review.Outcome}
		sources[workflowstatev2.ConcernRelease] = sourceRef{release.ID, release.Revision, release.State}
		sources[workflowstatev2.ConcernEvidence] = sourceRef{evidence.ID, 1, string(after.Evidence.State)}
	case workflowstatev2.CommandTechnicalStop:
		// The execution CAS above is the complete authoritative source mutation.
	case workflowstatev2.CommandCompleteNonLive:
		reviewHead, err := concernHead(ctx, write, r, workflowstatev2.ConcernReview)
		if err != nil {
			return nil, nil, err
		}
		review, err := write.GetReview(ctx, db.GetIssueWorkflowReviewParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: reviewHead.SourceRecordID})
		if err != nil {
			return nil, nil, err
		}
		var evidenceRefs []string
		if json.Unmarshal(review.EvidenceRefs, &evidenceRefs) != nil || len(evidenceRefs) == 0 {
			return nil, nil, ErrInvalidAuthoritativeState
		}
		sort.Strings(evidenceRefs)
		outcome, err := selectedOutcome(ctx, write, r)
		if err != nil {
			return nil, nil, err
		}
		updatedOutcome, err := write.UpdateOutcome(ctx, db.UpdateIssueWorkflowOutcomeCASParams{State: string(after.Outcome), AcceptanceVersion: int64(after.Scope.AcceptanceVersion), CompletionEvidenceID: mustUUID(evidenceRefs[0]), TerminalReason: "completed after independent review; deployment explicitly not required", WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: outcome.ID, ExpectedRevision: outcome.Revision})
		if err != nil {
			return nil, nil, err
		}
		releaseHead, err := concernHead(ctx, write, r, workflowstatev2.ConcernRelease)
		if err != nil || releaseHead.State != "pending" {
			return nil, nil, ErrInvalidAuthoritativeState
		}
		release, err := write.GetRelease(ctx, db.GetIssueWorkflowReleaseParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: releaseHead.SourceRecordID})
		if err != nil || release.State != "pending" || release.CandidateID != review.CandidateID || release.CandidateRevision != review.CandidateRevision || release.TargetKey != "" || release.TargetGeneration != 0 || release.StartedAt.Valid || release.DeployedAt.Valid {
			return nil, nil, ErrInvalidAuthoritativeState
		}
		updatedRelease, err := write.UpdateRelease(ctx, db.UpdateIssueWorkflowReleaseCASParams{State: "not_required", Facets: release.Facets, ExposedPopulation: release.ExposedPopulation, WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: release.ID, ExpectedRevision: release.Revision})
		if err != nil {
			return nil, nil, err
		}
		sources[workflowstatev2.ConcernOutcome] = sourceRef{updatedOutcome.ID, updatedOutcome.Revision, updatedOutcome.State}
		sources[workflowstatev2.ConcernRelease] = sourceRef{updatedRelease.ID, updatedRelease.Revision, updatedRelease.State}
	case workflowstatev2.CommandReviewPassed, workflowstatev2.CommandReviewFailed, workflowstatev2.CommandReviewInconclusive, workflowstatev2.CommandReviewBlockedEvidence:
		evidenceRefs := append([]string(nil), r.EvidenceRefs...)
		if reviewResultAuthority != nil {
			evidenceRefs = []string{stableUUID(r.WorkspaceID, r.IssueID, r.CommandKey, "review-evidence").String()}
		}
		if len(evidenceRefs) == 0 {
			return nil, nil, fmt.Errorf("review result requires evidence source")
		}
		candidateID := mustUUID(after.Review.CandidateID)
		candidate, err := write.GetCandidate(ctx, db.GetIssueWorkflowCandidateParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: candidateID})
		if err != nil {
			return nil, nil, fmt.Errorf("load review candidate: %w", err)
		}
		scopeHead, err := concernHead(ctx, write, r, workflowstatev2.ConcernScope)
		if err != nil {
			return nil, nil, fmt.Errorf("load review scope head: %w", err)
		}
		criteria, _ := json.Marshal(r.CriteriaRefs)
		coverageMap := map[string]bool{}
		for _, v := range r.CoverageRefs {
			coverageMap[v] = true
		}
		coverage, _ := json.Marshal(coverageMap)
		if reviewResultAuthority != nil {
			authority := *reviewResultAuthority
			evidenceID := mustUUID(evidenceRefs[0])
			payload := cloneMap(r.Correction.Evidence)
			payload["verdict"] = r.Correction.Verdict
			resultReviewID := stableUUID(r.WorkspaceID, r.IssueID, r.CommandKey, "review")
			payload["review_id"] = resultReviewID.String()
			payload["review_revision"] = int64(1)
			payload["candidate_revision"] = candidate.Revision
			payload["scope_id"] = util.UUIDToString(authority.ScopeID)
			payload["scope_revision"] = authority.ScopeRevision
			payload["acceptance_version"] = authority.AcceptanceVersion
			payload["criteria_refs"] = append([]string(nil), r.CriteriaRefs...)
			payload["coverage_refs"] = append([]string(nil), r.CoverageRefs...)
			payload["gap_refs"] = append([]string(nil), r.GapRefs...)
			payload["task_id"] = util.UUIDToString(authority.TaskID)
			payload["workflow_action_id"] = util.UUIDToString(authority.WorkflowActionID)
			payload["operation_id"] = util.UUIDToString(authority.OperationID)
			payload["branch_key"] = authority.BranchKey.String
			payload["freshness"] = "current"
			payloadJSON, _ := json.Marshal(payload)
			createdEvidence, createErr := write.CreateEvidence(ctx, db.CreateIssueWorkflowEvidenceParams{ID: evidenceID, WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ConcernType: "review", ConcernID: pgUUID(resultReviewID), ConcernRevision: nullableInt64(1), EvidenceType: "technical_review", Level: "local", Source: "workflow_review_task", ProducerType: "reviewer", ProducerID: principal.ActorID, CandidateID: candidate.ID, ObservedAt: pgtype.Timestamptz{Time: s.Now().UTC(), Valid: true}, Coverage: coverage, Limitations: []byte(`[]`), Contradictions: []byte(`[]`), Payload: payloadJSON})
			if createErr != nil {
				return nil, nil, fmt.Errorf("create review result evidence: %w", createErr)
			}
			sources[workflowstatev2.ConcernEvidence] = sourceRef{createdEvidence.ID, 1, string(after.Evidence.State)}
		}
		refs, _ := json.Marshal(evidenceRefs)
		gaps, _ := json.Marshal(r.GapRefs)
		correction, _ := json.Marshal(map[string]any{"required_correction": after.Review.RequiredCorrection, "count": after.Review.CorrectionCount, "limit": after.Review.MaxCorrections})
		reviewID := stableUUID(r.WorkspaceID, r.IssueID, r.CommandKey, "review")
		review, err := write.CreateReview(ctx, db.CreateIssueWorkflowReviewParams{ID: pgUUID(reviewID), WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, CandidateID: candidate.ID, CandidateRevision: candidate.Revision, ScopeID: scopeHead.SourceRecordID, ScopeRevision: int64(after.Scope.Revision), ReviewerID: principal.ActorID, Outcome: string(after.Review.State), Criteria: criteria, Coverage: coverage, EvidenceRefs: refs, Gaps: gaps, CorrectionPacket: correction, CorrectionCount: int32(after.Review.CorrectionCount), MaxCorrections: int32(after.Review.MaxCorrections)})
		if err != nil {
			return nil, nil, fmt.Errorf("create review result: %w", err)
		}
		sources[workflowstatev2.ConcernReview] = sourceRef{review.ID, 1, review.Outcome}
		if _, exists := sources[workflowstatev2.ConcernEvidence]; exists {
			break
		}
		evidenceRows, err := write.ListEvidence(ctx, db.ListIssueWorkflowEvidenceParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID})
		if err != nil {
			return nil, nil, err
		}
		evidenceByID := map[string]db.IssueWorkflowEvidence{}
		for _, row := range evidenceRows {
			evidenceByID[util.UUIDToString(row.ID)] = row
		}
		var evidence db.IssueWorkflowEvidence
		for index, ref := range evidenceRefs {
			row, ok := evidenceByID[ref]
			if !ok {
				return nil, nil, fmt.Errorf("review evidence %s is not normalized", ref)
			}
			if err := validateReviewEvidence(row, principal.ActorID, candidate.ID, candidate.Revision, scopeHead.SourceRecordID, int64(after.Scope.Revision), s.Now().UTC()); err != nil {
				return nil, nil, err
			}
			if index == 0 {
				evidence = row
			}
		}
		sources[workflowstatev2.ConcernEvidence] = sourceRef{evidence.ID, 1, string(after.Evidence.State)}

	case workflowstatev2.CommandRequestDecision:
		scopeHead, err := concernHead(ctx, write, r, workflowstatev2.ConcernScope)
		if err != nil {
			return nil, nil, err
		}
		authorityHead, err := concernHead(ctx, write, r, workflowstatev2.ConcernAuthority)
		if err != nil {
			return nil, nil, err
		}
		optionsValue := r.Decision.Options
		if optionsValue == nil {
			optionsValue = []string{}
		}
		options, _ := json.Marshal(optionsValue)
		candidate, err := write.GetCandidate(ctx, db.GetIssueWorkflowCandidateParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: mustUUID(r.CandidateID)})
		if err != nil || candidate.ScopeID != scopeHead.SourceRecordID || candidate.ScopeRevision != int64(after.Scope.Revision) {
			return nil, nil, ErrInvalidAuthoritativeState
		}
		authorityGap, _ := json.Marshal(map[string]any{"kind": string(r.Decision.AuthorityGap.Kind), "requested_capability": r.Decision.AuthorityGap.RequestedCapability, "reason": r.Decision.AuthorityGap.Reason, "candidate_revision": candidate.Revision, "branch_key": execution.BranchKey, "suspended_execution_revision": after.Revisions.Execution})
		responseSchema := []byte(`{"type":"authorization","outcomes":["approve","reject"],"detail":{"type":"string"}}`)
		decisionID := mustUUID(after.Decisions[stableUUID(r.WorkspaceID, r.IssueID, r.CommandKey, "decision").String()].ID)
		decision, err := write.CreateDecision(ctx, db.CreateIssueWorkflowDecisionParams{ID: decisionID, WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ScopeID: scopeHead.SourceRecordID, ScopeRevision: int64(after.Scope.Revision), AuthorityID: authorityHead.SourceRecordID, AuthorityRevision: nullableInt64(int64(after.Authority.Revision)), CandidateID: mustUUID(r.CandidateID), TargetKey: textValue(r.TargetID), DecisionType: string(r.Decision.Type), State: "unanswered", ResolutionState: "open", Question: r.Decision.Question, Recommendation: r.Decision.Recommendation, Consequence: r.Decision.Consequence, Options: options, ResponseSchema: responseSchema, AuthorityGap: authorityGap, RequesterID: principal.ActorID, AccountableOwnerID: r.Decision.AccountableOwnerID, SuspendedOperationID: execution.ID, SuspendedExecutionState: string(before.Execution.State)})
		if err != nil {
			return nil, nil, err
		}
		_ = decision
		attentionID := stableUUID(r.WorkspaceID, r.IssueID, r.CommandKey, "attention")
		consequence, _ := json.Marshal(map[string]string{"summary": r.Decision.Consequence})
		recommendation, _ := json.Marshal(map[string]string{"summary": r.Decision.Recommendation})
		context, _ := json.Marshal(map[string]any{"question": r.Decision.Question, "operation_id": after.Execution.SuspendedOperationID, "branch_key": after.Decisions[util.UUIDToString(decisionID)].BranchID, "candidate_id": r.CandidateID, "scope_revision": after.Scope.Revision, "authority_revision": after.Authority.Revision, "authority_gap": r.Decision.AuthorityGap})
		nextAction := []byte(`{"type":"respond_authorization","outcomes":["approve","reject"]}`)
		updatedAttention, err := write.CreateAttention(ctx, db.CreateIssueWorkflowAttentionParams{ID: pgUUID(attentionID), WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, AttentionType: "decision", DecisionID: decisionID, Consequence: consequence, Recommendation: recommendation, Context: context, NextAction: nextAction, ReadState: string(after.Attention.Read), Acknowledged: after.Attention.Acknowledged, ResolutionState: string(after.Attention.Resolution), ActionReceiptID: actionID(actions)})
		if err != nil {
			return nil, nil, err
		}
		sources[workflowstatev2.ConcernAttention] = sourceRef{updatedAttention.ID, updatedAttention.Revision, updatedAttention.ResolutionState}
		evidenceHead, err := concernHead(ctx, write, r, workflowstatev2.ConcernEvidence)
		if err != nil {
			return nil, nil, err
		}
		sources[workflowstatev2.ConcernEvidence] = sourceRef{evidenceHead.SourceRecordID, evidenceHead.SourceRecordRevision, string(after.Evidence.State)}

	case workflowstatev2.CommandRespondDecision:
		decisionID := mustUUID(r.DecisionID)
		relation, err := exactDecisionRequestRelation(ctx, write, r.WorkspaceID, r.IssueID, decisionID)
		if err != nil {
			return nil, nil, err
		}
		current, err := write.GetDecision(ctx, db.GetIssueWorkflowDecisionParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: decisionID})
		if err != nil {
			return nil, nil, err
		}
		responseID := stableUUID(r.WorkspaceID, r.IssueID, r.CommandKey, "decision-response")
		content, _ := json.Marshal(map[string]string{"text": r.Detail})
		if _, err = write.CreateDecisionResponse(ctx, db.CreateIssueWorkflowDecisionResponseParams{ID: pgUUID(responseID), WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, DecisionID: decisionID, DecisionRevision: current.Revision, ResponseType: string(r.DecisionOutcome), ActorID: principal.ActorID, IdempotencyKey: r.CommandKey, RequestHash: fingerprint, SelectedOptions: []byte(`[]`), Content: content, ScopeChange: []byte(`{}`)}); err != nil {
			return nil, nil, err
		}
		updatedDecision, err := write.UpdateDecision(ctx, db.UpdateIssueWorkflowDecisionStateCASParams{State: string(after.Decisions[r.DecisionID].State), ResolutionState: string(after.Decisions[r.DecisionID].Resolution), ResolvedAt: resolvedAt(after.Decisions[r.DecisionID], s.Now()), WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: decisionID, ExpectedRevision: current.Revision})
		if err != nil {
			return nil, nil, err
		}
		_ = updatedDecision
		attention := relation.attention
		requestLease := pgUUID(googleuuid.NewSHA1(commandID, []byte("settle-owner-decision-request")))
		claimNow := s.Now().UTC()
		claimedRequest, err := write.ClaimAction(ctx, db.ClaimIssueWorkflowActionExactParams{LeaseToken: requestLease, LeaseExpiresAt: pgtype.Timestamptz{Time: claimNow.Add(time.Minute), Valid: true}, WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: relation.actionID, ExpectedRevision: relation.actionRevision, Now: pgtype.Timestamptz{Time: claimNow, Valid: true}})
		if err != nil || claimedRequest.ActionKey != relation.actionKey || claimedRequest.ActionType != "owner_decision_requested" || claimedRequest.DecisionID != decisionID {
			return nil, nil, ErrInvalidAuthoritativeState
		}
		requestResult, _ := json.Marshal(map[string]any{"decision_id": r.DecisionID, "response_id": responseID.String(), "outcome": r.DecisionOutcome})
		if _, err := write.SettleAction(ctx, db.SettleIssueWorkflowActionParams{Now: pgtype.Timestamptz{Time: s.Now().UTC(), Valid: true}, EffectResult: requestResult, WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: claimedRequest.ID, ExpectedRevision: claimedRequest.Revision, LeaseToken: requestLease}); err != nil {
			return nil, nil, err
		}
		updatedAttention, err := write.UpdateAttention(ctx, db.UpdateIssueWorkflowAttentionCASParams{ReadState: string(after.Attention.Read), Acknowledged: after.Attention.Acknowledged, ResolutionState: string(after.Attention.Resolution), ActionReceiptID: actionID(actions), ResolvedAt: resolvedTime(after.Attention, s.Now()), WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: attention.ID, ExpectedRevision: attention.Revision})
		if err != nil {
			return nil, nil, err
		}
		sources[workflowstatev2.ConcernAttention] = sourceRef{updatedAttention.ID, updatedAttention.Revision, updatedAttention.ResolutionState}
	case workflowstatev2.CommandReadDecision, workflowstatev2.CommandAcknowledgeDecision:
		attention, err := selectedAttention(ctx, write, r)
		if err != nil || util.UUIDToString(attention.DecisionID) != r.DecisionID {
			return nil, nil, ErrInvalidAuthoritativeState
		}
		updatedAttention, err := write.UpdateAttention(ctx, db.UpdateIssueWorkflowAttentionCASParams{ReadState: string(after.Attention.Read), Acknowledged: after.Attention.Acknowledged, ResolutionState: string(after.Attention.Resolution), ActionReceiptID: attention.ActionReceiptID, ResolvedAt: attention.ResolvedAt, WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: attention.ID, ExpectedRevision: attention.Revision})
		if err != nil {
			return nil, nil, err
		}
		sources[workflowstatev2.ConcernAttention] = sourceRef{updatedAttention.ID, updatedAttention.Revision, updatedAttention.ResolutionState}
	case workflowstatev2.CommandSettleDecisionEffect:
		decisionID := mustUUID(r.DecisionID)
		current, err := write.GetDecision(ctx, db.GetIssueWorkflowDecisionParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: decisionID})
		if err != nil || current.State != "answered" || current.ResolutionState != "resuming" || current.Revision != int64(r.ExpectedDecisionRevision) {
			return nil, nil, ErrInvalidAuthoritativeState
		}
		if _, err := write.UpdateDecision(ctx, db.UpdateIssueWorkflowDecisionStateCASParams{State: "answered", ResolutionState: "resolved", ResolvedAt: pgtype.Timestamptz{Time: s.Now().UTC(), Valid: true}, WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: decisionID, ExpectedRevision: current.Revision}); err != nil {
			return nil, nil, err
		}
		attention, err := selectedAttention(ctx, write, r)
		if err != nil || attention.DecisionID != decisionID {
			return nil, nil, ErrInvalidAuthoritativeState
		}
		updatedAttention, err := write.UpdateAttention(ctx, db.UpdateIssueWorkflowAttentionCASParams{ReadState: string(after.Attention.Read), Acknowledged: after.Attention.Acknowledged, ResolutionState: "resolved", ActionReceiptID: attention.ActionReceiptID, ResolvedAt: pgtype.Timestamptz{Time: s.Now().UTC(), Valid: true}, WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: attention.ID, ExpectedRevision: attention.Revision})
		if err != nil {
			return nil, nil, err
		}
		sources[workflowstatev2.ConcernAttention] = sourceRef{updatedAttention.ID, updatedAttention.Revision, updatedAttention.ResolutionState}
	default:
		return nil, nil, fmt.Errorf("%w: %s", ErrUnsupportedCommand, r.Kind)
	}
	if err := s.fail("after_domain"); err != nil {
		return nil, nil, err
	}
	return actions, sources, nil
}

func validateReviewEvidence(row db.IssueWorkflowEvidence, reviewerID, candidateID pgtype.UUID, candidateRevision int64, scopeID pgtype.UUID, scopeRevision int64, now time.Time) error {
	if row.EvidenceType != "technical_review" || row.ConcernType != "review" || row.ProducerType != "reviewer" || row.ProducerID != reviewerID || row.CandidateID != candidateID || row.SupersededAt.Valid {
		return ErrForbidden
	}
	if row.ValidUntil.Valid && !row.ValidUntil.Time.After(now) {
		return ErrForbidden
	}
	var payload struct {
		CandidateRevision int64  `json:"candidate_revision"`
		ScopeID           string `json:"scope_id"`
		ScopeRevision     int64  `json:"scope_revision"`
		Freshness         string `json:"freshness"`
	}
	if err := json.Unmarshal(row.Payload, &payload); err != nil || payload.CandidateRevision != candidateRevision || payload.ScopeID != util.UUIDToString(scopeID) || payload.ScopeRevision != scopeRevision || payload.Freshness != "current" {
		return ErrForbidden
	}
	return nil
}

func validateCompletionReviewEvidence(row db.IssueWorkflowEvidence, review db.IssueWorkflowReview, candidate db.IssueWorkflowCandidate, now time.Time) error {
	if validateReviewEvidence(row, review.ReviewerID, candidate.ID, candidate.Revision, review.ScopeID, review.ScopeRevision, now) != nil || row.ConcernID != review.ID || !row.ConcernRevision.Valid || row.ConcernRevision.Int64 != 1 {
		return ErrForbidden
	}
	var payload struct {
		Verdict        string   `json:"verdict"`
		ReviewID       string   `json:"review_id"`
		ReviewRevision int64    `json:"review_revision"`
		CriteriaRefs   []string `json:"criteria_refs"`
		CoverageRefs   []string `json:"coverage_refs"`
	}
	var criteria []string
	var coverage map[string]bool
	if json.Unmarshal(row.Payload, &payload) != nil || json.Unmarshal(review.Criteria, &criteria) != nil || json.Unmarshal(review.Coverage, &coverage) != nil || payload.Verdict != "passed" || payload.ReviewID != util.UUIDToString(review.ID) || payload.ReviewRevision != 1 {
		return ErrForbidden
	}
	expectedCoverage := make([]string, 0, len(coverage))
	for ref, covered := range coverage {
		if !covered {
			return ErrForbidden
		}
		expectedCoverage = append(expectedCoverage, ref)
	}
	sort.Strings(criteria)
	sort.Strings(expectedCoverage)
	sort.Strings(payload.CriteriaRefs)
	sort.Strings(payload.CoverageRefs)
	if !reflect.DeepEqual(criteria, payload.CriteriaRefs) || !reflect.DeepEqual(expectedCoverage, payload.CoverageRefs) {
		return ErrForbidden
	}
	return nil
}

func deriveActions(r Request, commandID googleuuid.UUID, after workflowstatev2.CurrentPicture) ([]admittedAction, error) {
	var typ, op, decision string
	switch r.Kind {
	case workflowstatev2.CommandPrepareRelease:
		typ, op = "prepare_release", after.Release.OperationID
	case workflowstatev2.CommandAuthorizeRelease:
		typ, op = "deploy_exact_release", after.Release.OperationID
	case workflowstatev2.CommandRecordDeployment:
		typ, op = "expose_exact_release", after.Release.OperationID
	case workflowstatev2.CommandRecordExposure:
		typ, op = "verify_runtime_exact_release", after.Release.OperationID
	case workflowstatev2.CommandVerifyRuntime:
		typ, op = "verify_business_exact_release", after.Release.OperationID
	case workflowstatev2.CommandVerifyBusiness:
		typ, op = "monitor_exact_release", after.Release.OperationID
	case workflowstatev2.CommandProductionFailed:
		if after.Release.State == workflowstatev2.ReleaseRollingBack {
			typ, op = "rollback_exact_release", after.Release.OperationID
		}
	case workflowstatev2.CommandRollbackSucceeded:
		typ, op = "verify_restoration_exact_release", after.Release.OperationID
	case workflowstatev2.CommandSubmitCorrection:
		typ = "review_candidate"
		op = after.Execution.OperationID
	case workflowstatev2.CommandReviewPassed:
		if after.Scope.DeploymentPolicy == workflowstatev2.DeploymentNotRequired && after.Scope.SourcePlanRevision > 0 {
			typ = "finalize_non_live"
			op = after.Execution.OperationID
		}
	case workflowstatev2.CommandReviewFailed:
		if after.Execution.State == workflowstatev2.ExecutionCorrection {
			typ = "review_correction"
			op = after.Execution.OperationID
		}
	case workflowstatev2.CommandRequestDecision:
		typ = "owner_decision_requested"
		op = after.Execution.SuspendedOperationID
		decision = stableUUID(r.WorkspaceID, r.IssueID, r.CommandKey, "decision").String()
	case workflowstatev2.CommandRespondDecision:
		decision = r.DecisionID
		op = after.Decisions[r.DecisionID].SuspendedOperationID
		switch r.DecisionOutcome {
		case workflowstatev2.DecisionApprove:
			typ = "resume_exact_operation"
		case workflowstatev2.DecisionReject:
			typ = "replan_exact_operation"
		default:
			return nil, ErrUnsupportedCommand
		}
	default:
		return nil, nil
	}
	if typ == "" {
		return nil, nil
	}
	id := googleuuid.NewSHA1(commandID, []byte(typ+"/"+op+"/"+decision))
	payload, _ := json.Marshal(map[string]string{"operation_id": op, "decision_id": decision})
	releaseID, targetKey := "", ""
	if strings.Contains(typ, "release") {
		targetKey = after.Release.TargetID
		if r.ReleaseEffect != nil {
			releaseID = r.ReleaseEffect.ReleaseID
		}
	}
	return []admittedAction{{ID: id.String(), Key: "workflow/" + commandID.String() + "/" + typ, Type: typ, OperationID: op, DecisionID: decision, ReleaseID: releaseID, TargetKey: targetKey, Payload: payload}}, nil
}

func afterEvidenceRefs(before, after workflowstatev2.CurrentPicture) []string {
	seen := make(map[string]struct{})
	for _, list := range [][]workflowstatev2.BoundEvidence{before.Evidence.Runtime, before.Evidence.Business, before.Evidence.Restoration, before.Evidence.Other} {
		for _, item := range list {
			seen[item.Ref] = struct{}{}
		}
	}
	var refs []string
	for _, list := range [][]workflowstatev2.BoundEvidence{after.Evidence.Runtime, after.Evidence.Business, after.Evidence.Restoration, after.Evidence.Other} {
		for _, item := range list {
			if _, ok := seen[item.Ref]; !ok {
				refs = append(refs, item.Ref)
			}
		}
	}
	return refs
}

func cloneMap(input map[string]any) map[string]any {
	result := make(map[string]any, len(input)+5)
	for key, value := range input {
		result[key] = value
	}
	return result
}

func mustJSON(value any) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		return []byte(`null`)
	}
	return encoded
}

func cRecoveryMode(before, after workflowstatev2.CurrentPicture) workflowstatev2.RecoveryMode {
	if after.Release.ReleaseKind == "recovery" && after.Release.RecoveryOfReleaseID == before.Release.ReleaseID {
		return workflowstatev2.RecoveryRollback
	}
	return workflowstatev2.RecoveryContainment
}

func meaningfulObject(input map[string]any) bool {
	if len(input) == 0 {
		return false
	}
	for key, value := range input {
		if strings.TrimSpace(key) != "" && meaningfulValue(value) {
			return true
		}
	}
	return false
}

func meaningfulValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(typed) != ""
	case map[string]any:
		return meaningfulObject(typed)
	case []any:
		for _, item := range typed {
			if meaningfulValue(item) {
				return true
			}
		}
		return false
	default:
		return true
	}
}

func meaningfulRefs(refs []string) bool {
	if len(refs) == 0 {
		return false
	}
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			return false
		}
		if _, duplicate := seen[ref]; duplicate {
			return false
		}
		seen[ref] = struct{}{}
	}
	return true
}

func validReviewResultSemantics(result *CorrectionResultInput, kind workflowstatev2.CommandKind) bool {
	if result == nil || !meaningfulObject(result.Evidence) || !meaningfulRefs(result.CriteriaRefs) || !meaningfulRefs(result.CoverageRefs) {
		return false
	}
	if kind == workflowstatev2.CommandReviewPassed {
		return len(result.GapRefs) == 0 && strings.TrimSpace(result.RequiredCorrection) == ""
	}
	if kind == workflowstatev2.CommandReviewFailed || kind == workflowstatev2.CommandReviewInconclusive || kind == workflowstatev2.CommandReviewBlockedEvidence {
		return meaningfulRefs(result.GapRefs) && strings.TrimSpace(result.RequiredCorrection) != ""
	}
	return false
}

func concernHead(ctx context.Context, write *workflowstore.IssueWrite, r Request, c workflowstatev2.Concern) (db.IssueWorkflowConcernHead, error) {
	return write.GetConcernHead(ctx, db.GetIssueWorkflowConcernHeadParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ConcernType: string(c)})
}
func selectedExecution(ctx context.Context, write *workflowstore.IssueWrite, w, i pgtype.UUID) (db.IssueWorkflowExecution, error) {
	h, e := write.GetConcernHead(ctx, db.GetIssueWorkflowConcernHeadParams{WorkspaceID: w, IssueID: i, ConcernType: string(workflowstatev2.ConcernExecution)})
	if e != nil {
		return db.IssueWorkflowExecution{}, e
	}
	return write.GetExecution(ctx, db.GetIssueWorkflowExecutionParams{WorkspaceID: w, IssueID: i, ID: h.SourceRecordID})
}
func selectedAttention(ctx context.Context, write *workflowstore.IssueWrite, r Request) (db.IssueWorkflowAttention, error) {
	h, e := concernHead(ctx, write, r, workflowstatev2.ConcernAttention)
	if e != nil {
		return db.IssueWorkflowAttention{}, e
	}
	return write.GetAttention(ctx, db.GetIssueWorkflowAttentionParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID, ID: h.SourceRecordID})
}

type decisionRequestRelation struct {
	decisionRevision int64
	ownerID          pgtype.UUID
	attention        db.IssueWorkflowAttention
	actionID         pgtype.UUID
	actionRevision   int64
	actionKey        string
}

// exactDecisionRequestRelation resolves the immutable decision -> active
// attention -> waiting request receipt chain while the issue admission lock is
// held. Requiring exactly one chain fails closed instead of choosing whichever
// unrelated attention happens to be the display concern head.
func exactDecisionRequestRelation(ctx context.Context, write *workflowstore.IssueWrite, workspaceID, issueID, decisionID pgtype.UUID) (decisionRequestRelation, error) {
	decision, err := write.GetDecision(ctx, db.GetIssueWorkflowDecisionParams{WorkspaceID: workspaceID, IssueID: issueID, ID: decisionID})
	if err != nil || decision.State != "unanswered" || decision.ResolutionState != "open" || decision.SupersededAt.Valid {
		return decisionRequestRelation{}, ErrInvalidAuthoritativeState
	}
	rows, err := write.ListProjectionInputs(ctx, db.ListIssueWorkflowProjectionInputsParams{WorkspaceID: workspaceID, IssueID: issueID})
	if err != nil {
		return decisionRequestRelation{}, err
	}
	var relation decisionRequestRelation
	relation.decisionRevision, relation.ownerID = decision.Revision, decision.AccountableOwnerID
	matchedAttention := 0
	for _, row := range rows {
		if row.RecordType != "attention" {
			continue
		}
		var item struct {
			ID              string `json:"id"`
			DecisionID      string `json:"decision_id"`
			ResolutionState string `json:"resolution_state"`
			SupersededAt    any    `json:"superseded_at"`
		}
		if json.Unmarshal(row.Payload, &item) != nil || item.DecisionID != util.UUIDToString(decisionID) || item.ResolutionState != "open" || item.SupersededAt != nil {
			continue
		}
		attention, getErr := write.GetAttention(ctx, db.GetIssueWorkflowAttentionParams{WorkspaceID: workspaceID, IssueID: issueID, ID: mustUUID(item.ID)})
		if getErr != nil || !attention.ActionReceiptID.Valid {
			return decisionRequestRelation{}, ErrInvalidAuthoritativeState
		}
		relation.attention = attention
		matchedAttention++
	}
	if matchedAttention != 1 {
		return decisionRequestRelation{}, ErrInvalidAuthoritativeState
	}
	matchedAction := 0
	for _, row := range rows {
		if row.RecordType != "action_receipt" || row.ID != relation.attention.ActionReceiptID {
			continue
		}
		var action struct {
			ID           string          `json:"id"`
			Revision     int64           `json:"revision"`
			ActionKey    string          `json:"action_key"`
			ActionType   string          `json:"action_type"`
			DecisionID   string          `json:"decision_id"`
			State        string          `json:"state"`
			Payload      json.RawMessage `json:"payload"`
			SupersededAt any             `json:"superseded_at"`
		}
		if json.Unmarshal(row.Payload, &action) != nil || action.ActionType != "owner_decision_requested" || action.DecisionID != util.UUIDToString(decisionID) || action.State != "admitted" || action.SupersededAt != nil {
			return decisionRequestRelation{}, ErrInvalidAuthoritativeState
		}
		var frozen struct {
			DecisionID        string `json:"decision_id"`
			DecisionRevision  int64  `json:"decision_revision"`
			AttentionID       string `json:"attention_id"`
			AttentionRevision int64  `json:"attention_revision"`
			OwnerID           string `json:"accountable_owner_id"`
		}
		if json.Unmarshal(action.Payload, &frozen) != nil || frozen.DecisionID != util.UUIDToString(decisionID) || frozen.DecisionRevision != decision.Revision || frozen.AttentionID != util.UUIDToString(relation.attention.ID) || frozen.AttentionRevision != relation.attention.Revision || frozen.OwnerID != util.UUIDToString(decision.AccountableOwnerID) {
			return decisionRequestRelation{}, ErrInvalidAuthoritativeState
		}
		relation.actionID, relation.actionRevision, relation.actionKey = row.ID, action.Revision, action.ActionKey
		matchedAction++
	}
	if matchedAction != 1 {
		return decisionRequestRelation{}, ErrInvalidAuthoritativeState
	}
	return relation, nil
}

func validateDecisionContinuationSources(ctx context.Context, write *workflowstore.IssueWrite, workspaceID, issueID pgtype.UUID, responseID string, responseRevision int64, requestActionID, requestActionKey string, requestActionRevision int64, decisionID string, outcome workflowstatev2.DecisionOutcome) error {
	rows, err := write.ListProjectionInputs(ctx, db.ListIssueWorkflowProjectionInputsParams{WorkspaceID: workspaceID, IssueID: issueID})
	if err != nil {
		return err
	}
	responseOK, requestOK := false, false
	for _, row := range rows {
		switch row.RecordType {
		case "decision_response":
			if util.UUIDToString(row.ID) != responseID {
				continue
			}
			var item struct {
				DecisionID       string `json:"decision_id"`
				DecisionRevision int64  `json:"decision_revision"`
				ResponseType     string `json:"response_type"`
			}
			if json.Unmarshal(row.Payload, &item) == nil && item.DecisionID == decisionID && item.DecisionRevision == responseRevision && item.ResponseType == string(outcome) {
				responseOK = true
			}
		case "action_receipt":
			if util.UUIDToString(row.ID) != requestActionID {
				continue
			}
			var item struct {
				Revision   int64  `json:"revision"`
				ActionKey  string `json:"action_key"`
				ActionType string `json:"action_type"`
				DecisionID string `json:"decision_id"`
				State      string `json:"state"`
			}
			if json.Unmarshal(row.Payload, &item) == nil && item.ActionKey == requestActionKey && item.ActionType == "owner_decision_requested" && item.DecisionID == decisionID && item.State == "settled" && requestActionRevision > 0 && item.Revision > requestActionRevision {
				requestOK = true
			}
		}
	}
	if !responseOK || !requestOK {
		return ErrInvalidAuthoritativeState
	}
	return nil
}
func selectedOutcome(ctx context.Context, write *workflowstore.IssueWrite, r Request) (db.IssueWorkflowOutcome, error) {
	h, e := concernHead(ctx, write, r, workflowstatev2.ConcernOutcome)
	if e != nil {
		return db.IssueWorkflowOutcome{}, e
	}
	outcome, err := write.GetLatestOutcome(ctx, db.GetLatestIssueWorkflowOutcomeParams{WorkspaceID: r.WorkspaceID, IssueID: r.IssueID})
	if err == nil && outcome.ID != h.SourceRecordID {
		return db.IssueWorkflowOutcome{}, ErrInvalidAuthoritativeState
	}
	return outcome, err
}
func containsConcern(cs []workflowstatev2.Concern, c workflowstatev2.Concern) bool {
	for _, v := range cs {
		if v == c {
			return true
		}
	}
	return false
}

func exactExpected(items []workflowstatev2.ExpectedRevision, picture workflowstatev2.CurrentPicture, concerns ...workflowstatev2.Concern) bool {
	if len(items) != len(concerns) {
		return false
	}
	want := make(map[workflowstatev2.Concern]workflowstatev2.Revision, len(concerns))
	for _, concern := range concerns {
		want[concern] = picture.Revisions.Get(concern)
	}
	for _, item := range items {
		revision, ok := want[item.Concern]
		if !ok || revision != item.Revision {
			return false
		}
		delete(want, item.Concern)
	}
	return len(want) == 0
}

func exactReleaseSnapshotExpected(items []workflowstatev2.ExpectedRevision, picture workflowstatev2.CurrentPicture) bool {
	return exactExpected(items, picture,
		workflowstatev2.ConcernScope,
		workflowstatev2.ConcernAuthority,
		workflowstatev2.ConcernExecution,
		workflowstatev2.ConcernReview,
		workflowstatev2.ConcernRelease,
		workflowstatev2.ConcernEvidence,
	)
}
func actionID(actions []admittedAction) pgtype.UUID {
	if len(actions) == 0 {
		return pgtype.UUID{}
	}
	return mustUUID(actions[0].ID)
}
func textValue(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }
func resolvedAt(d workflowstatev2.OwnerDecision, now time.Time) pgtype.Timestamptz {
	if d.State == workflowstatev2.DecisionAnswered {
		return pgtype.Timestamptz{Time: now.UTC(), Valid: true}
	}
	return pgtype.Timestamptz{}
}
func resolvedTime(a workflowstatev2.AttentionFacet, now time.Time) pgtype.Timestamptz {
	if a.Resolution == workflowstatev2.ResolutionResolved {
		return pgtype.Timestamptz{Time: now.UTC(), Valid: true}
	}
	return pgtype.Timestamptz{}
}
