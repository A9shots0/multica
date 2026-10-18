// Package workflowstatev2 defines the pure operating contract for an issue.
// It has no persistence, transport, UI, or runtime dependencies. Persistence
// layers are responsible for proving command idempotency and supplying the
// authoritative concern revisions consumed here.
package workflowstatev2

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

type Revision uint64

type Concern string

const (
	ConcernOutcome   Concern = "outcome"
	ConcernScope     Concern = "scope"
	ConcernAuthority Concern = "authority"
	ConcernExecution Concern = "execution"
	ConcernReview    Concern = "review"
	ConcernRelease   Concern = "release"
	ConcernAttention Concern = "attention"
	ConcernEvidence  Concern = "evidence"
	ConcernBudget    Concern = "budget"
)

type OutcomeState string

const (
	OutcomeOpen       OutcomeState = "open"
	OutcomeSatisfied  OutcomeState = "satisfied"
	OutcomeCancelled  OutcomeState = "cancelled"
	OutcomeSuperseded OutcomeState = "superseded"
)

type ExecutionState string

const (
	ExecutionReceived           ExecutionState = "received"
	ExecutionQueued             ExecutionState = "queued"
	ExecutionWorking            ExecutionState = "working"
	ExecutionWaitingAutomatic   ExecutionState = "waiting_automatic"
	ExecutionInReview           ExecutionState = "in_review"
	ExecutionCorrection         ExecutionState = "correction"
	ExecutionBlockedExternal    ExecutionState = "blocked_external"
	ExecutionTechnicallyStopped ExecutionState = "technically_stopped"
	ExecutionPaused             ExecutionState = "paused"
	ExecutionCompleted          ExecutionState = "completed"
	ExecutionFailed             ExecutionState = "failed"
	ExecutionCancelled          ExecutionState = "cancelled"
	ExecutionSuperseded         ExecutionState = "superseded"
)

type ReviewState string

const (
	ReviewNotRequired       ReviewState = "not_required"
	ReviewPending           ReviewState = "pending"
	ReviewInProgress        ReviewState = "in_progress"
	ReviewPassed            ReviewState = "passed"
	ReviewFailed            ReviewState = "failed"
	ReviewInconclusive      ReviewState = "inconclusive"
	ReviewBlockedByEvidence ReviewState = "blocked_by_evidence"
)

type ReleaseState string

const (
	ReleaseNotRequired         ReleaseState = "not_required"
	ReleasePending             ReleaseState = "pending"
	ReleaseBuilt               ReleaseState = "built"
	ReleaseReviewed            ReleaseState = "reviewed"
	ReleaseAuthorized          ReleaseState = "authorized"
	ReleaseDeploying           ReleaseState = "deploying"
	ReleaseDeployed            ReleaseState = "deployed"
	ReleaseExposed             ReleaseState = "exposed"
	ReleaseProductionVerifying ReleaseState = "production_verifying"
	ReleaseLive                ReleaseState = "live"
	ReleaseDegraded            ReleaseState = "degraded"
	ReleaseRollingBack         ReleaseState = "rolling_back"
	ReleaseRolledBack          ReleaseState = "rolled_back"
	ReleaseRestored            ReleaseState = "restored"
	ReleaseFailed              ReleaseState = "failed"
)

type AuthorityState string

const (
	AuthorityMissing    AuthorityState = "missing"
	AuthorityActive     AuthorityState = "active"
	AuthorityPaused     AuthorityState = "paused"
	AuthorityRevoked    AuthorityState = "revoked"
	AuthorityExpired    AuthorityState = "expired"
	AuthoritySuperseded AuthorityState = "superseded"
)

type EvidenceState string

const (
	EvidenceUnknown      EvidenceState = "unknown"
	EvidenceCurrent      EvidenceState = "current"
	EvidenceStale        EvidenceState = "stale"
	EvidenceContradicted EvidenceState = "contradicted"
)

type ReadState string

const (
	ReadUnread ReadState = "unread"
	ReadRead   ReadState = "read"
)

type DecisionState string

const (
	DecisionUnanswered DecisionState = "unanswered"
	DecisionAnswered   DecisionState = "answered"
	DecisionSuperseded DecisionState = "superseded"
	DecisionExpired    DecisionState = "expired"
)

type ResolutionState string

const (
	ResolutionOpen     ResolutionState = "open"
	ResolutionResuming ResolutionState = "resuming"
	ResolutionResolved ResolutionState = "resolved"
	ResolutionSnoozed  ResolutionState = "snoozed"
)

type DecisionType string

const (
	DecisionApproval       DecisionType = "approval"
	DecisionDirection      DecisionType = "direction"
	DecisionFactualInput   DecisionType = "factual_input"
	DecisionAuthorization  DecisionType = "authorization"
	DecisionBudget         DecisionType = "budget"
	DecisionCancellation   DecisionType = "cancellation"
	DecisionExternalStep   DecisionType = "external_step"
	DecisionAuthentication DecisionType = "authentication"
)

type DecisionOutcome string

const (
	DecisionApprove                DecisionOutcome = "approve"
	DecisionReject                 DecisionOutcome = "reject"
	DecisionRequestChanges         DecisionOutcome = "request_changes"
	DecisionPartialAcceptance      DecisionOutcome = "partial_acceptance"
	DecisionProvideFact            DecisionOutcome = "provide_fact"
	DecisionAskQuestion            DecisionOutcome = "ask_question"
	DecisionHold                   DecisionOutcome = "hold"
	DecisionCancel                 DecisionOutcome = "cancel"
	DecisionChangeScope            DecisionOutcome = "change_scope"
	DecisionChangeBudget           DecisionOutcome = "change_budget"
	DecisionExternalStepComplete   DecisionOutcome = "external_step_complete"
	DecisionAuthenticationComplete DecisionOutcome = "authentication_complete"
)

type ActorRole string

const (
	RoleOwner      ActorRole = "owner"
	RoleController ActorRole = "controller"
	RoleWorker     ActorRole = "worker"
	RoleReviewer   ActorRole = "reviewer"
	RoleRelease    ActorRole = "release_operator"
	RoleVerifier   ActorRole = "verifier"
	RoleObserver   ActorRole = "observer"
)

type CommandKind string

const (
	CommandQueue                 CommandKind = "queue"
	CommandStart                 CommandKind = "start"
	CommandWaitAutomatically     CommandKind = "wait_automatically"
	CommandBlockExternally       CommandKind = "block_externally"
	CommandTechnicalStop         CommandKind = "technical_stop"
	CommandSubmitCorrection      CommandKind = "submit_correction"
	CommandBeginReview           CommandKind = "begin_review"
	CommandReviewPassed          CommandKind = "review_passed"
	CommandReviewFailed          CommandKind = "review_failed"
	CommandReviewInconclusive    CommandKind = "review_inconclusive"
	CommandReviewBlockedEvidence CommandKind = "review_blocked_by_evidence"
	CommandRequestDecision       CommandKind = "request_owner_decision"
	CommandRespondDecision       CommandKind = "respond_owner_decision"
	CommandReadDecision          CommandKind = "read_owner_decision"
	CommandAcknowledgeDecision   CommandKind = "acknowledge_owner_decision"
	CommandSettleDecisionEffect  CommandKind = "settle_owner_decision_effect"
	CommandCompleteNonLive       CommandKind = "complete_non_live"
	CommandPrepareRelease        CommandKind = "prepare_release"
	CommandBuildRelease          CommandKind = "build_release"
	CommandAuthorizeRelease      CommandKind = "authorize_release"
	CommandStartDeployment       CommandKind = "start_deployment"
	CommandRecordDeployment      CommandKind = "record_deployment"
	CommandRecordExposure        CommandKind = "record_exposure"
	CommandBeginVerification     CommandKind = "begin_production_verification"
	CommandVerifyRuntime         CommandKind = "verify_runtime"
	CommandVerifyBusiness        CommandKind = "verify_business"
	CommandProductionFailed      CommandKind = "production_failed"
	CommandBeginRollback         CommandKind = "begin_rollback"
	CommandRollbackSucceeded     CommandKind = "rollback_succeeded"
	CommandRestoreVerified       CommandKind = "restore_verified"
	CommandPause                 CommandKind = "pause"
	CommandResume                CommandKind = "resume"
	CommandCancel                CommandKind = "cancel"
	CommandSupersede             CommandKind = "supersede"
	CommandReviseScope           CommandKind = "revise_scope"
	CommandGrantAuthority        CommandKind = "grant_authority"
	CommandRevokeAuthority       CommandKind = "revoke_authority"
	CommandRecordEvidence        CommandKind = "record_evidence"
	CommandContradictEvidence    CommandKind = "contradict_evidence"
)

type ExpectedRevision struct {
	Concern  Concern
	Revision Revision
}

// CommandEnvelope carries causality and binding. Detail is explanatory only;
// it never selects a transition or grants authority.
type CommandEnvelope struct {
	CommandID                      string
	IssueID                        string
	OperationID                    string
	BranchID                       string
	Kind                           CommandKind
	ActorID                        string
	ActorRole                      ActorRole
	Expected                       []ExpectedRevision
	ScopeRevision                  Revision
	DeploymentPolicy               DeploymentPolicy
	SourcePlanRevision             Revision
	AuthorityRevision              Revision
	DecisionID                     string
	ExpectedDecisionRevision       Revision
	DecisionType                   DecisionType
	DecisionOutcome                DecisionOutcome
	AuthorityGap                   AuthorityGap
	SuspendedState                 ExecutionState
	CandidateID                    string
	CandidateRevision              Revision
	ReleaseID                      string
	RecoveryReleaseID              string
	RecoveryOfReleaseID            string
	RecoveryCandidateID            string
	RecoveryCandidateRevision      Revision
	RecoverySourceCandidateID      string
	RecoverySourceRevision         Revision
	RecoveryBuildManifestDigest    string
	RecoveryArtifactDigest         string
	RecoveryRuntimeDigest          string
	RecoveryReviewedReviewID       string
	RecoveryReviewedReviewRevision Revision
	RecoveryMode                   RecoveryMode
	RecoveryTargetGeneration       Revision
	TargetID                       string
	TargetGeneration               Revision
	EvidenceRefs                   []string
	CriteriaRefs                   []string
	CoverageRefs                   []string
	GapRefs                        []string
	RequiredCorrection             string
	ReviewerID                     string
	CorrectionCount                uint32
	MaxCorrections                 uint32
	PlannedVerifierID              string
	ReleaseOperatorID              string
	ExpectedTargetReleaseID        string
	ExpectedTargetGeneration       Revision
	AcceptanceVersion              Revision
	BuildManifestDigest            string
	ArtifactDigest                 string
	RuntimeContractDigest          string
	AuthorityCapability            string
	AuthorityEvidenceID            string
	OwnerEvidenceRevision          Revision
	RecoveryPolicyRevision         Revision
	ExposurePolicyDigest           string
	RollbackTargetReleaseID        string
	IrreversibleEffects            []string
	RunnableTargetID               string
	CapacityEvidenceRef            string
	LeaseID                        string
	CheckpointID                   string
	ExpectedSignal                 string
	ResumeCondition                string
	NextAutomaticAction            string
	Detail                         string
}

type ConcernRevisions struct {
	Outcome, Scope, Authority, Execution, Review, Release, Attention, Evidence, Budget Revision
}

func (r ConcernRevisions) Get(concern Concern) Revision {
	switch concern {
	case ConcernOutcome:
		return r.Outcome
	case ConcernScope:
		return r.Scope
	case ConcernAuthority:
		return r.Authority
	case ConcernExecution:
		return r.Execution
	case ConcernReview:
		return r.Review
	case ConcernRelease:
		return r.Release
	case ConcernAttention:
		return r.Attention
	case ConcernEvidence:
		return r.Evidence
	case ConcernBudget:
		return r.Budget
	default:
		return 0
	}
}

func (r *ConcernRevisions) Increment(concern Concern) {
	switch concern {
	case ConcernOutcome:
		r.Outcome++
	case ConcernScope:
		r.Scope++
	case ConcernAuthority:
		r.Authority++
	case ConcernExecution:
		r.Execution++
	case ConcernReview:
		r.Review++
	case ConcernRelease:
		r.Release++
	case ConcernAttention:
		r.Attention++
	case ConcernEvidence:
		r.Evidence++
	case ConcernBudget:
		r.Budget++
	}
}

type ScopeFacet struct {
	Revision           Revision
	AcceptanceVersion  Revision
	OwnerEvidenceRef   string
	DeploymentPolicy   DeploymentPolicy
	SourcePlanRevision Revision
}

type DeploymentPolicy string

const (
	DeploymentUnknown     DeploymentPolicy = "unknown"
	DeploymentNotRequired DeploymentPolicy = "not_required"
	DeploymentRequired    DeploymentPolicy = "required"
)

type AuthorityFacet struct {
	State         AuthorityState
	Revision      Revision
	ScopeRevision Revision
	TargetID      string
	CandidateID   string
}

type ExecutionFacet struct {
	State                ExecutionState
	OperationID          string
	ActorID              string
	RunnableTargetID     string
	LeaseID              string
	CheckpointID         string
	ExpectedSignal       string
	ResumeCondition      string
	NextAutomaticAction  string
	SuspendedState       ExecutionState
	SuspendedOperationID string
}

type ReviewFacet struct {
	State              ReviewState
	CandidateID        string
	AcceptanceVersion  Revision
	ImplementerID      string
	ReviewerID         string
	CriteriaRefs       []string
	CoverageRefs       []string
	GapRefs            []string
	RequiredCorrection string
	CorrectionCount    uint32
	MaxCorrections     uint32
}

type ReleaseFacet struct {
	State                    ReleaseState
	ReleaseID                string
	ReleaseKind              string
	RecoveryOfReleaseID      string
	CandidateID              string
	CandidateRevision        Revision
	TargetID                 string
	Generation               Revision
	ExpectedTargetReleaseID  string
	ExpectedTargetGeneration Revision
	OperationID              string
	BranchID                 string
	AcceptanceVersion        Revision
	ReleaseOperatorID        string
	PlannedVerifierID        string
	BuildManifestDigest      string
	ArtifactDigest           string
	RuntimeContractDigest    string
	AuthorityCapability      string
	AuthorityEvidenceID      string
	OwnerEvidenceRevision    Revision
	RecoveryPolicyRevision   Revision
	ExposurePolicyDigest     string
	RollbackTargetReleaseID  string
	IrreversibleEffects      []string
}

type RecoveryMode string

const (
	RecoveryRollback    RecoveryMode = "rollback"
	RecoveryContainment RecoveryMode = "containment"
)

type EvidenceFacet struct {
	State            EvidenceState
	TechnicalReviews []ReviewEvidence
	Runtime          []BoundEvidence
	Business         []BoundEvidence
	Restoration      []BoundEvidence
	Other            []BoundEvidence
}

type BoundEvidence struct {
	Ref               string
	CandidateID       string
	TargetID          string
	TargetGeneration  Revision
	AcceptanceVersion Revision
	ScopeRevision     Revision
	Freshness         EvidenceState
}

type ReviewEvidence struct {
	BoundEvidence
	Result             ReviewState
	ReviewerID         string
	CriteriaRefs       []string
	CoverageRefs       []string
	GapRefs            []string
	RequiredCorrection string
}

type AttentionFacet struct {
	DecisionID   string
	Read         ReadState
	Acknowledged bool
	Resolution   ResolutionState
}

type AuthorityGapKind string

const AuthorityGapOperationAuthorization AuthorityGapKind = "operation_authorization"

type AuthorityGap struct {
	Kind                AuthorityGapKind
	RequestedCapability string
	Reason              string
}

type BudgetFacet struct {
	AttemptsUsed, AttemptLimit uint32
	RuntimeUsed, RuntimeLimit  uint64
	TokensUsed, TokenLimit     uint64
	CostUsed, CostLimit        uint64
}

type OwnerDecision struct {
	ID                   string
	Revision             Revision
	Type                 DecisionType
	State                DecisionState
	Resolution           ResolutionState
	ScopeRevision        Revision
	AuthorityRevision    Revision
	CandidateID          string
	TargetID             string
	SuspendedState       ExecutionState
	SuspendedOperationID string
	BranchID             string
	AuthorityGap         AuthorityGap
	Response             DecisionOutcome
	SupportingText       string
}

type CurrentPicture struct {
	IssueID       string
	Outcome       OutcomeState
	Scope         ScopeFacet
	Authority     AuthorityFacet
	Execution     ExecutionFacet
	Review        ReviewFacet
	Release       ReleaseFacet
	Attention     AttentionFacet
	Evidence      EvidenceFacet
	Budget        BudgetFacet
	Decisions     map[string]OwnerDecision
	Revisions     ConcernRevisions
	LastCommandID string
}

func NewCurrentPicture(issueID string, scopeRevision, acceptanceVersion Revision, ownerEvidenceRef string) CurrentPicture {
	return CurrentPicture{
		IssueID:   issueID,
		Outcome:   OutcomeOpen,
		Scope:     ScopeFacet{Revision: scopeRevision, AcceptanceVersion: acceptanceVersion, OwnerEvidenceRef: ownerEvidenceRef},
		Authority: AuthorityFacet{State: AuthorityMissing},
		Execution: ExecutionFacet{State: ExecutionReceived},
		Review:    ReviewFacet{State: ReviewPending},
		Release:   ReleaseFacet{State: ReleasePending},
		Attention: AttentionFacet{Read: ReadRead, Resolution: ResolutionResolved},
		Evidence:  EvidenceFacet{State: EvidenceUnknown},
		Decisions: make(map[string]OwnerDecision),
		Revisions: ConcernRevisions{Outcome: 1, Scope: scopeRevision, Authority: 1, Execution: 1, Review: 1, Release: 1, Attention: 1, Evidence: 1, Budget: 1},
	}
}

type GateErrorCode string

const (
	GateInvalidCommand           GateErrorCode = "invalid_command"
	GateInvalidActorRole         GateErrorCode = "invalid_actor_role"
	GateStaleRevision            GateErrorCode = "stale_revision"
	GateStaleScope               GateErrorCode = "stale_scope"
	GateStaleAuthority           GateErrorCode = "stale_authority"
	GateMissingAuthority         GateErrorCode = "missing_authority"
	GateMissingCandidate         GateErrorCode = "missing_candidate"
	GateMissingTarget            GateErrorCode = "missing_target"
	GateMissingEvidence          GateErrorCode = "missing_evidence"
	GateInvalidDecision          GateErrorCode = "invalid_decision"
	GateStaleDecision            GateErrorCode = "stale_decision"
	GateContradictoryTransition  GateErrorCode = "contradictory_transition"
	GateOutcomeTerminal          GateErrorCode = "outcome_terminal"
	GateTargetGenerationConflict GateErrorCode = "target_generation_conflict"
	GateIdempotencyConflict      GateErrorCode = "idempotency_conflict"
)

type GateError struct {
	Code     GateErrorCode
	Concern  Concern
	Expected Revision
	Actual   Revision
	Reason   string
}

func (e *GateError) Error() string {
	if e.Concern != "" {
		return fmt.Sprintf("workflow gate %s (%s): expected revision %d, actual %d: %s", e.Code, e.Concern, e.Expected, e.Actual, e.Reason)
	}
	return fmt.Sprintf("workflow gate %s: %s", e.Code, e.Reason)
}

type ApplyInput struct {
	Picture                       CurrentPicture
	Command                       CommandEnvelope
	PreviouslyAppliedCommandID    string
	PreviouslyAppliedFingerprint  string
	AuthoritativeTargetGeneration *Revision
}

func (c CommandEnvelope) Fingerprint() string {
	payload, _ := json.Marshal(c)
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

type ApplyResult struct {
	Picture   CurrentPicture
	Duplicate bool
}
