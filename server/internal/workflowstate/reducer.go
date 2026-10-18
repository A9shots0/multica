// Package workflowstate defines the deterministic workflow transitions used by
// the Silver Mediums local pilot. It deliberately contains no model calls and
// no database access: callers must prove actor identity, persist event IDs, and
// apply the returned action transactionally.
package workflowstate

type State string

const (
	Received            State = "received"
	Queued              State = "queued"
	Working             State = "working"
	TechnicalReview     State = "technical_review"
	Correction          State = "correction"
	Deploying           State = "deploying"
	ProductionVerifying State = "production_verifying"
	NeedsOwner          State = "needs_owner"
	Blocked             State = "blocked"
	Done                State = "done"
	Live                State = "live"
	Failed              State = "failed"
	RollingBack         State = "rolling_back"
	RolledBack          State = "rolled_back"
)

type Event string

const (
	IssueReceived         Event = "issue_received"
	TaskStarted           Event = "task_started"
	ImplementationReady   Event = "implementation_ready"
	ReviewPassed          Event = "review_passed"
	ReviewFailed          Event = "review_failed"
	OwnerDecisionRequired Event = "owner_decision_required"
	OwnerDecisionRecorded Event = "owner_decision_recorded"
	DependencyBlocked     Event = "dependency_blocked"
	DependencyResolved    Event = "dependency_resolved"
	DeploymentStarted     Event = "deployment_started"
	DeploymentSucceeded   Event = "deployment_succeeded"
	ProductionVerified    Event = "production_verified"
	ProductionFailed      Event = "production_failed"
	RollbackSucceeded     Event = "rollback_succeeded"
	TaskInterrupted       Event = "task_interrupted"
)

const (
	ReasonIndependentReviewerMissing = "independent_reviewer_missing"
	ReasonReleaseAgentMissing        = "release_agent_missing"
	ReasonVerifierMissing            = "verifier_missing"
	ReasonRecoveryUnavailable        = "recovery_unavailable"
	ReasonInvalidTransition          = "invalid_transition"
	ReasonWrongActor                 = "wrong_actor"
	ReasonDuplicateEvent             = "duplicate_event"
	ReasonCorrectionLimitReached     = "correction_limit_reached"
)

// Input is a fully resolved event. Identity and declared gates come from
// durable task/issue records, never from parsing a comment.
type Input struct {
	IssueID             string
	Current             State
	Event               Event
	EventAlreadyApplied bool
	ActorID             string
	ImplementerID       string
	ReviewerID          string
	LeaderID            string
	ReleaseAgentID      string
	VerifierID          string
	RequiresOwner       bool
	RequiresDeployment  bool
	CanAutoRecover      bool
	ResumeState         State
	ResumeAgentID       string
	CorrectionCount     int
	MaxCorrections      int
}

// Decision is an instruction for the transactional controller. Empty action
// fields mean no such side effect is authorized.
type Decision struct {
	Next            State  `json:"next"`
	EnqueueAgentID  string `json:"enqueue_agent_id,omitempty"`
	RecoveryIssueID string `json:"recovery_issue_id,omitempty"`
	NotifyOwner     bool   `json:"notify_owner,omitempty"`
	WakeLeader      bool   `json:"wake_leader,omitempty"`
	Urgent          bool   `json:"urgent,omitempty"`
	Ignored         bool   `json:"ignored,omitempty"`
	Reason          string `json:"reason,omitempty"`
}

func ignored(in Input, reason string) Decision {
	return Decision{Next: in.Current, Ignored: true, Reason: reason}
}

func blockedForLeader(in Input, reason string) Decision {
	return Decision{Next: Blocked, EnqueueAgentID: in.LeaderID, WakeLeader: true, Reason: reason}
}

// Decide reduces one structured event into one deterministic transition.
// Persisted event-ID dedup must be checked by the caller and supplied through
// EventAlreadyApplied; this makes a replay a strict no-op.
func Decide(in Input) Decision {
	if in.EventAlreadyApplied {
		return ignored(in, ReasonDuplicateEvent)
	}

	switch in.Event {
	case IssueReceived:
		if in.Current != Received || in.ImplementerID == "" {
			return ignored(in, ReasonInvalidTransition)
		}
		return Decision{Next: Queued, EnqueueAgentID: in.ImplementerID}

	case TaskStarted:
		if in.Current != Queued && in.Current != Correction {
			return ignored(in, ReasonInvalidTransition)
		}
		return Decision{Next: Working}

	case ImplementationReady:
		if in.Current != Working && in.Current != Correction {
			return ignored(in, ReasonInvalidTransition)
		}
		if in.ActorID == "" || in.ActorID != in.ImplementerID {
			return ignored(in, ReasonWrongActor)
		}
		if in.ReviewerID == "" || in.ReviewerID == in.ImplementerID {
			return blockedForLeader(in, ReasonIndependentReviewerMissing)
		}
		return Decision{Next: TechnicalReview, EnqueueAgentID: in.ReviewerID}

	case ReviewFailed:
		if in.Current != TechnicalReview {
			return ignored(in, ReasonInvalidTransition)
		}
		if in.ActorID == "" || in.ActorID != in.ReviewerID || in.ReviewerID == in.ImplementerID {
			return ignored(in, ReasonWrongActor)
		}
		if in.ImplementerID == "" {
			return blockedForLeader(in, ReasonInvalidTransition)
		}
		if in.MaxCorrections < 1 || in.CorrectionCount >= in.MaxCorrections {
			return blockedForLeader(in, ReasonCorrectionLimitReached)
		}
		return Decision{Next: Correction, EnqueueAgentID: in.ImplementerID}

	case ReviewPassed:
		if in.Current != TechnicalReview {
			return ignored(in, ReasonInvalidTransition)
		}
		if in.ActorID == "" || in.ActorID != in.ReviewerID || in.ReviewerID == in.ImplementerID {
			return ignored(in, ReasonWrongActor)
		}
		if in.RequiresOwner {
			return Decision{Next: NeedsOwner, NotifyOwner: true}
		}
		if in.RequiresDeployment {
			if in.ReleaseAgentID == "" {
				return blockedForLeader(in, ReasonReleaseAgentMissing)
			}
			return Decision{Next: Deploying, EnqueueAgentID: in.ReleaseAgentID}
		}
		return Decision{Next: Done}

	case OwnerDecisionRequired:
		if !isExecutable(in.Current) {
			return ignored(in, ReasonInvalidTransition)
		}
		return Decision{Next: NeedsOwner, NotifyOwner: true}

	case OwnerDecisionRecorded:
		if in.Current == NeedsOwner && in.ResumeState == Done {
			return Decision{Next: Done}
		}
		if in.Current != NeedsOwner || !isExecutable(in.ResumeState) || in.ResumeAgentID == "" {
			return ignored(in, ReasonInvalidTransition)
		}
		return Decision{Next: in.ResumeState, EnqueueAgentID: in.ResumeAgentID}

	case DependencyBlocked:
		if !isExecutable(in.Current) {
			return ignored(in, ReasonInvalidTransition)
		}
		return Decision{Next: Blocked, WakeLeader: true}

	case DependencyResolved:
		if in.Current != Blocked || !isExecutable(in.ResumeState) || in.ResumeAgentID == "" {
			return ignored(in, ReasonInvalidTransition)
		}
		return Decision{Next: in.ResumeState, EnqueueAgentID: in.ResumeAgentID}

	case DeploymentStarted:
		if in.Current != Deploying || in.ActorID != in.ReleaseAgentID || in.ReleaseAgentID == "" {
			return ignored(in, ReasonInvalidTransition)
		}
		return Decision{Next: Deploying}

	case DeploymentSucceeded:
		if in.Current != Deploying || in.ActorID != in.ReleaseAgentID || in.ReleaseAgentID == "" {
			return ignored(in, ReasonWrongActor)
		}
		if in.VerifierID == "" || in.VerifierID == in.ReleaseAgentID {
			return blockedForLeader(in, ReasonVerifierMissing)
		}
		return Decision{Next: ProductionVerifying, EnqueueAgentID: in.VerifierID}

	case ProductionVerified:
		if in.Current != ProductionVerifying || in.ActorID != in.VerifierID || in.VerifierID == "" {
			return ignored(in, ReasonWrongActor)
		}
		return Decision{Next: Live}

	case ProductionFailed:
		if in.Current != ProductionVerifying || in.ActorID != in.VerifierID || in.VerifierID == "" {
			return ignored(in, ReasonWrongActor)
		}
		if in.ReleaseAgentID == "" {
			return Decision{Next: Failed, NotifyOwner: true, WakeLeader: true, Urgent: true, Reason: ReasonReleaseAgentMissing}
		}
		return Decision{Next: RollingBack, EnqueueAgentID: in.ReleaseAgentID, NotifyOwner: true, Urgent: true}

	case RollbackSucceeded:
		if in.Current != RollingBack || in.ActorID != in.ReleaseAgentID || in.ReleaseAgentID == "" {
			return ignored(in, ReasonWrongActor)
		}
		return Decision{Next: RolledBack}

	case TaskInterrupted:
		if !isExecutable(in.Current) {
			return ignored(in, ReasonInvalidTransition)
		}
		if !in.CanAutoRecover || in.ImplementerID == "" {
			return Decision{Next: Failed, WakeLeader: true, Reason: ReasonRecoveryUnavailable}
		}
		return Decision{Next: Queued, EnqueueAgentID: in.ImplementerID, RecoveryIssueID: in.IssueID}
	}

	return ignored(in, ReasonInvalidTransition)
}

func isExecutable(state State) bool {
	switch state {
	case Queued, Working, TechnicalReview, Correction, Deploying, ProductionVerifying, RollingBack:
		return true
	default:
		return false
	}
}
