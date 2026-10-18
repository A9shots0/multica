package workflowstate

import (
	"fmt"
	"slices"

	workflowstatev2 "github.com/multica-ai/multica/server/internal/workflowstate/v2"
)

// LegacyAdapterErrorCode identifies compatibility failures without falling
// back to the legacy state machine.
type LegacyAdapterErrorCode string

const (
	LegacyAdapterInvalidInput     LegacyAdapterErrorCode = "invalid_legacy_input"
	LegacyAdapterDuplicateEvent   LegacyAdapterErrorCode = "legacy_event_already_applied"
	LegacyAdapterUnsupportedEvent LegacyAdapterErrorCode = "unsupported_legacy_event"
	LegacyAdapterUnsupportedPlan  LegacyAdapterErrorCode = "unsupported_legacy_plan"
)

// LegacyAdapterError is fail-closed: callers must not apply any returned v2
// command when adaptation fails.
type LegacyAdapterError struct {
	Code   LegacyAdapterErrorCode
	Event  Event
	Reason string
}

func (e *LegacyAdapterError) Error() string {
	return fmt.Sprintf("legacy workflow adapter %s for %s: %s", e.Code, e.Event, e.Reason)
}

// LegacyEvidenceRefs supplies durable evidence identities that the v1 event
// vocabulary did not distinguish. The adapter never manufactures these refs.
type LegacyEvidenceRefs struct {
	Implementation    string
	Review            string
	Build             string
	Authorization     string
	DeploymentStart   string
	Deployment        string
	Exposure          string
	VerificationStart string
	Runtime           string
	Business          string
}

// LegacyV2Input wraps a resolved v1 event with the bindings required by v2.
// Legacy.Current is deliberately ignored: CurrentPicture is the only state
// authority. The remaining legacy plan fields are routing inputs, not state.
type LegacyV2Input struct {
	Legacy Input

	// Exact normalized plan snapshot supplied by the boundary's durable plan.
	// A legacy deployment boolean is not sufficient release authority.
	ReleasePreparation *workflowstatev2.CommandEnvelope

	EventID          string
	ControllerID     string
	OperationID      string
	CandidateID      string
	TargetID         string
	TargetGeneration workflowstatev2.Revision
	// AuthoritativeTargetGeneration is supplied independently of the legacy
	// event projection and is never inferred from TargetGeneration.
	AuthoritativeTargetGeneration *workflowstatev2.Revision
	RunnableTargetID              string
	CapacityEvidenceRef           string
	LeaseID                       string
	CheckpointID                  string
	NextAutomaticAction           string
	CriteriaRefs                  []string
	CoverageRefs                  []string
	GapRefs                       []string
	RequiredCorrection            string
	Evidence                      LegacyEvidenceRefs
}

// AdaptLegacyEvent converts a supported legacy event into causally complete v2
// commands. Some legacy events expand into multiple commands because v2 keeps
// review, deployment, exposure, runtime proof, and business proof separate.
// The returned commands have already been validated against successive v2
// pictures, but the input picture itself is never mutated.
func AdaptLegacyEvent(picture workflowstatev2.CurrentPicture, in LegacyV2Input) ([]workflowstatev2.CommandEnvelope, error) {
	if in.EventID == "" || in.Legacy.IssueID == "" || in.Legacy.IssueID != picture.IssueID {
		return nil, legacyAdapterError(LegacyAdapterInvalidInput, in.Legacy.Event, "event id and exact issue binding are required")
	}
	if in.Legacy.EventAlreadyApplied {
		return nil, legacyAdapterError(LegacyAdapterDuplicateEvent, in.Legacy.Event, "v2 idempotency records must resolve the replay")
	}

	current := picture
	commands := make([]workflowstatev2.CommandEnvelope, 0, 4)
	appendCommand := func(suffix string, command workflowstatev2.CommandEnvelope) error {
		command.CommandID = in.EventID + ":" + suffix
		command.IssueID = current.IssueID
		command.ScopeRevision = current.Scope.Revision
		command.AuthorityRevision = current.Authority.Revision
		command.Expected = allConcernRevisions(current)
		result, err := workflowstatev2.Apply(workflowstatev2.ApplyInput{
			Picture: current, Command: command, AuthoritativeTargetGeneration: in.AuthoritativeTargetGeneration,
		})
		if err != nil {
			return err
		}
		commands = append(commands, command)
		current = result.Picture
		return nil
	}

	switch in.Legacy.Event {
	case IssueReceived:
		if err := appendCommand("queue", workflowstatev2.CommandEnvelope{
			Kind: workflowstatev2.CommandQueue, ActorID: in.ControllerID, ActorRole: workflowstatev2.RoleController,
			OperationID: in.OperationID, RunnableTargetID: in.RunnableTargetID,
			CapacityEvidenceRef: in.CapacityEvidenceRef, NextAutomaticAction: in.NextAutomaticAction,
		}); err != nil {
			return nil, err
		}

	case TaskStarted:
		if current.Execution.State == workflowstatev2.ExecutionCorrection {
			if err := appendCommand("queue-correction", workflowstatev2.CommandEnvelope{
				Kind: workflowstatev2.CommandQueue, ActorID: in.ControllerID, ActorRole: workflowstatev2.RoleController,
				OperationID: in.OperationID, RunnableTargetID: in.RunnableTargetID,
				CapacityEvidenceRef: in.CapacityEvidenceRef, NextAutomaticAction: in.NextAutomaticAction,
			}); err != nil {
				return nil, err
			}
		}
		if in.Legacy.ActorID == "" || current.Execution.RunnableTargetID == "" || in.Legacy.ActorID != current.Execution.RunnableTargetID {
			return nil, legacyAdapterError(LegacyAdapterInvalidInput, in.Legacy.Event, "task actor must match the queued runnable target")
		}
		if err := appendCommand("start", workflowstatev2.CommandEnvelope{
			Kind: workflowstatev2.CommandStart, ActorID: in.Legacy.ActorID, ActorRole: workflowstatev2.RoleWorker,
			OperationID: in.OperationID, LeaseID: in.LeaseID, CheckpointID: in.CheckpointID,
		}); err != nil {
			return nil, err
		}

	case ImplementationReady:
		if in.Legacy.ActorID == "" || in.Legacy.ActorID != in.Legacy.ImplementerID || in.Legacy.ActorID != current.Execution.ActorID {
			return nil, legacyAdapterError(LegacyAdapterInvalidInput, in.Legacy.Event, "implementer must match the active v2 worker")
		}
		if in.Legacy.ReviewerID == "" || in.Legacy.ReviewerID == in.Legacy.ImplementerID {
			return nil, legacyAdapterError(LegacyAdapterInvalidInput, in.Legacy.Event, "a reviewer independent from the implementer is required")
		}
		if err := appendCommand("begin-review", workflowstatev2.CommandEnvelope{
			Kind: workflowstatev2.CommandBeginReview, ActorID: in.ControllerID, ActorRole: workflowstatev2.RoleController,
			CandidateID: in.CandidateID, ReviewerID: in.Legacy.ReviewerID, EvidenceRefs: refs(in.Evidence.Implementation),
		}); err != nil {
			return nil, err
		}

	case ReviewFailed:
		if in.Legacy.ActorID == "" || in.Legacy.ActorID != in.Legacy.ReviewerID || in.Legacy.ActorID != current.Review.ReviewerID {
			return nil, legacyAdapterError(LegacyAdapterInvalidInput, in.Legacy.Event, "reviewer must match the legacy plan")
		}
		if in.Legacy.ImplementerID == "" || current.Review.ImplementerID == "" || in.Legacy.ImplementerID != current.Review.ImplementerID || current.Execution.RunnableTargetID != in.Legacy.ImplementerID || in.RunnableTargetID != in.Legacy.ImplementerID {
			return nil, legacyAdapterError(LegacyAdapterInvalidInput, in.Legacy.Event, "the active implementer must remain available for correction")
		}
		if in.Legacy.MaxCorrections < 1 || in.Legacy.CorrectionCount < 0 || in.Legacy.CorrectionCount >= in.Legacy.MaxCorrections {
			return nil, legacyAdapterError(LegacyAdapterInvalidInput, in.Legacy.Event, "correction budget is missing or exhausted")
		}
		if uint32(in.Legacy.CorrectionCount) != current.Review.CorrectionCount ||
			(current.Review.MaxCorrections != 0 && uint32(in.Legacy.MaxCorrections) != current.Review.MaxCorrections) {
			return nil, legacyAdapterError(LegacyAdapterInvalidInput, in.Legacy.Event, "correction budget does not match the authoritative v2 review")
		}
		if err := appendCommand("review-failed", workflowstatev2.CommandEnvelope{
			Kind: workflowstatev2.CommandReviewFailed, ActorID: in.Legacy.ActorID, ActorRole: workflowstatev2.RoleReviewer,
			CandidateID: in.CandidateID, EvidenceRefs: refs(in.Evidence.Review),
			CriteriaRefs: in.CriteriaRefs, CoverageRefs: in.CoverageRefs, GapRefs: in.GapRefs,
			RequiredCorrection: in.RequiredCorrection, CorrectionCount: uint32(in.Legacy.CorrectionCount + 1), MaxCorrections: uint32(in.Legacy.MaxCorrections),
		}); err != nil {
			return nil, err
		}

	case ReviewPassed:
		if in.Legacy.ActorID == "" || in.Legacy.ActorID != in.Legacy.ReviewerID {
			return nil, legacyAdapterError(LegacyAdapterInvalidInput, in.Legacy.Event, "reviewer must match the legacy plan")
		}
		if in.Legacy.RequiresOwner {
			return nil, legacyAdapterError(LegacyAdapterUnsupportedPlan, in.Legacy.Event, "legacy owner approval is not a typed v2 decision")
		}
		if err := appendCommand("review-passed", workflowstatev2.CommandEnvelope{
			Kind: workflowstatev2.CommandReviewPassed, ActorID: in.Legacy.ActorID, ActorRole: workflowstatev2.RoleReviewer,
			CandidateID: in.CandidateID, EvidenceRefs: refs(in.Evidence.Review),
			CriteriaRefs: in.CriteriaRefs, CoverageRefs: in.CoverageRefs,
		}); err != nil {
			return nil, err
		}
		if !in.Legacy.RequiresDeployment {
			if err := appendCommand("complete-non-live", workflowstatev2.CommandEnvelope{
				Kind: workflowstatev2.CommandCompleteNonLive, ActorID: in.ControllerID, ActorRole: workflowstatev2.RoleController,
			}); err != nil {
				return nil, err
			}
			break
		}
		if in.Legacy.ReleaseAgentID == "" {
			return nil, legacyAdapterError(LegacyAdapterInvalidInput, in.Legacy.Event, "release agent is required")
		}
		prepare := in.ReleasePreparation
		if prepare == nil || prepare.Kind != workflowstatev2.CommandPrepareRelease ||
			prepare.IssueID != picture.IssueID || prepare.ScopeRevision != picture.Scope.Revision ||
			prepare.AuthorityRevision != picture.Authority.Revision || !slices.Equal(prepare.Expected, allConcernRevisions(picture)) ||
			prepare.OperationID != in.OperationID || prepare.OperationID != picture.Execution.OperationID ||
			prepare.ActorID != in.ControllerID || prepare.ActorRole != workflowstatev2.RoleController ||
			prepare.CandidateID != in.CandidateID || prepare.TargetID != in.TargetID ||
			prepare.TargetGeneration != in.TargetGeneration || prepare.ReleaseOperatorID != in.Legacy.ReleaseAgentID ||
			prepare.PlannedVerifierID != in.Legacy.VerifierID {
			return nil, legacyAdapterError(LegacyAdapterInvalidInput, in.Legacy.Event, "an exact normalized release preparation and independent actors are required")
		}
		if err := appendCommand("prepare-release", *prepare); err != nil {
			return nil, err
		}
		if err := appendCommand("build", workflowstatev2.CommandEnvelope{
			Kind: workflowstatev2.CommandBuildRelease, ActorID: in.Legacy.ReleaseAgentID, ActorRole: workflowstatev2.RoleRelease,
			CandidateID: in.CandidateID, TargetID: in.TargetID, TargetGeneration: in.TargetGeneration,
			EvidenceRefs:        refs(in.Evidence.Build),
			BuildManifestDigest: current.Release.BuildManifestDigest, ArtifactDigest: current.Release.ArtifactDigest,
			RuntimeContractDigest: current.Release.RuntimeContractDigest,
		}); err != nil {
			return nil, err
		}
		if err := appendCommand("authorize", workflowstatev2.CommandEnvelope{
			Kind: workflowstatev2.CommandAuthorizeRelease, ActorID: in.ControllerID, ActorRole: workflowstatev2.RoleController,
			CandidateID: in.CandidateID, TargetID: in.TargetID, TargetGeneration: in.TargetGeneration,
			EvidenceRefs:        refs(in.Evidence.Authorization),
			AuthorityCapability: current.Release.AuthorityCapability, AuthorityEvidenceID: current.Release.AuthorityEvidenceID,
		}); err != nil {
			return nil, err
		}
		if err := appendCommand("start-deployment", workflowstatev2.CommandEnvelope{
			Kind: workflowstatev2.CommandStartDeployment, ActorID: in.Legacy.ReleaseAgentID, ActorRole: workflowstatev2.RoleRelease,
			CandidateID: in.CandidateID, TargetID: in.TargetID, TargetGeneration: in.TargetGeneration,
			EvidenceRefs: refs(in.Evidence.DeploymentStart),
		}); err != nil {
			return nil, err
		}

	case DeploymentSucceeded:
		if in.Legacy.ActorID == "" || in.Legacy.ActorID != in.Legacy.ReleaseAgentID || in.Legacy.VerifierID == "" || in.Legacy.VerifierID == in.Legacy.ReleaseAgentID {
			return nil, legacyAdapterError(LegacyAdapterInvalidInput, in.Legacy.Event, "release actor and independent verifier are required")
		}
		if current.Release.ReleaseOperatorID == "" || in.Legacy.ReleaseAgentID != current.Release.ReleaseOperatorID {
			return nil, legacyAdapterError(LegacyAdapterInvalidInput, in.Legacy.Event, "release actor must match the operator that built the v2 release")
		}
		for _, step := range []struct {
			suffix   string
			kind     workflowstatev2.CommandKind
			actorID  string
			role     workflowstatev2.ActorRole
			evidence string
		}{
			{"deployed", workflowstatev2.CommandRecordDeployment, in.Legacy.ReleaseAgentID, workflowstatev2.RoleRelease, in.Evidence.Deployment},
			{"exposed", workflowstatev2.CommandRecordExposure, in.Legacy.ReleaseAgentID, workflowstatev2.RoleRelease, in.Evidence.Exposure},
			{"begin-verification", workflowstatev2.CommandBeginVerification, in.Legacy.VerifierID, workflowstatev2.RoleVerifier, in.Evidence.VerificationStart},
		} {
			if err := appendCommand(step.suffix, workflowstatev2.CommandEnvelope{
				Kind: step.kind, ActorID: step.actorID, ActorRole: step.role,
				CandidateID: in.CandidateID, TargetID: in.TargetID, TargetGeneration: in.TargetGeneration,
				EvidenceRefs: refs(step.evidence), PlannedVerifierID: in.Legacy.VerifierID,
			}); err != nil {
				return nil, err
			}
		}

	case ProductionVerified:
		if in.Legacy.ActorID == "" || in.Legacy.ActorID != in.Legacy.VerifierID ||
			current.Release.PlannedVerifierID == "" || in.Legacy.VerifierID != current.Release.PlannedVerifierID ||
			current.Release.ReleaseOperatorID == "" || in.Legacy.ReleaseAgentID != current.Release.ReleaseOperatorID ||
			current.Release.PlannedVerifierID == current.Release.ReleaseOperatorID {
			return nil, legacyAdapterError(LegacyAdapterInvalidInput, in.Legacy.Event, "verifier and release operator must match the independent identities persisted in v2")
		}
		for _, step := range []struct {
			suffix   string
			kind     workflowstatev2.CommandKind
			evidence string
		}{
			{"runtime-verified", workflowstatev2.CommandVerifyRuntime, in.Evidence.Runtime},
			{"business-verified", workflowstatev2.CommandVerifyBusiness, in.Evidence.Business},
		} {
			if err := appendCommand(step.suffix, workflowstatev2.CommandEnvelope{
				Kind: step.kind, ActorID: in.Legacy.VerifierID, ActorRole: workflowstatev2.RoleVerifier,
				CandidateID: in.CandidateID, TargetID: in.TargetID, TargetGeneration: in.TargetGeneration,
				EvidenceRefs: refs(step.evidence),
			}); err != nil {
				return nil, err
			}
		}

	default:
		return nil, legacyAdapterError(LegacyAdapterUnsupportedEvent, in.Legacy.Event, "event has no lossless v2 mapping")
	}

	return commands, nil
}

func allConcernRevisions(p workflowstatev2.CurrentPicture) []workflowstatev2.ExpectedRevision {
	concerns := []workflowstatev2.Concern{
		workflowstatev2.ConcernOutcome, workflowstatev2.ConcernScope, workflowstatev2.ConcernAuthority,
		workflowstatev2.ConcernExecution, workflowstatev2.ConcernReview, workflowstatev2.ConcernRelease,
		workflowstatev2.ConcernAttention, workflowstatev2.ConcernEvidence, workflowstatev2.ConcernBudget,
	}
	expected := make([]workflowstatev2.ExpectedRevision, 0, len(concerns))
	for _, concern := range concerns {
		expected = append(expected, workflowstatev2.ExpectedRevision{Concern: concern, Revision: p.Revisions.Get(concern)})
	}
	return expected
}

func refs(ref string) []string {
	if ref == "" {
		return nil
	}
	return []string{ref}
}

func legacyAdapterError(code LegacyAdapterErrorCode, event Event, reason string) error {
	return &LegacyAdapterError{Code: code, Event: event, Reason: reason}
}
