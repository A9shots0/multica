// Package taskcontinuity decides whether an obstacle affects one work lane or
// the whole accepted goal. It deliberately does not classify provider/runtime
// errors; package taskfailure owns that technical taxonomy.
package taskcontinuity

import (
	_ "embed"
	"fmt"
	"strings"
	"time"
)

// State is the provider-neutral execution state used by supervisors and
// provider adapters.
type State string

const (
	StateActive          State = "active"
	StateDegraded        State = "degraded"
	StateWaitingLane     State = "waiting_lane"
	StateNeedsOwner      State = "needs_owner"
	StateExternalBlocked State = "external_blocked"
	StateComplete        State = "complete"
	StateCancelled       State = "cancelled"
)

// Scope identifies whether an observation affects one lane or every remaining
// path to the accepted result.
type Scope string

const (
	ScopeLane Scope = "lane"
	ScopeGoal Scope = "goal"
)

// Kind identifies the decision boundary without coupling it to one provider's
// error wording.
type Kind string

const (
	KindToolUnavailable       Kind = "tool_unavailable"
	KindEvidenceUnavailable   Kind = "evidence_unavailable"
	KindWorkspaceConflict     Kind = "workspace_conflict"
	KindResourceOccupied      Kind = "resource_occupied"
	KindTransientProvider     Kind = "transient_provider"
	KindContextRecovery       Kind = "context_recovery"
	KindExternalDependency    Kind = "external_dependency"
	KindProductionLock        Kind = "production_lock"
	KindProviderSafetyLock    Kind = "provider_safety_lock"
	KindProductDecision       Kind = "product_decision"
	KindSecurityTrustChange   Kind = "security_trust_change"
	KindAccessChange          Kind = "access_change"
	KindDestructiveAction     Kind = "destructive_action"
	KindProductionDataAction  Kind = "production_data_action"
	KindBudgetChange          Kind = "budget_change"
	KindOutboundCommunication Kind = "outbound_communication"
	KindScopeExpansion        Kind = "scope_expansion"
	KindInterruptOtherWork    Kind = "interrupt_other_work"
)

var validKinds = map[Kind]struct{}{
	KindToolUnavailable: {}, KindEvidenceUnavailable: {}, KindWorkspaceConflict: {},
	KindResourceOccupied: {}, KindTransientProvider: {}, KindContextRecovery: {},
	KindExternalDependency: {}, KindProductionLock: {}, KindProviderSafetyLock: {},
	KindProductDecision: {}, KindSecurityTrustChange: {}, KindAccessChange: {},
	KindDestructiveAction: {}, KindProductionDataAction: {}, KindBudgetChange: {},
	KindOutboundCommunication: {}, KindScopeExpansion: {}, KindInterruptOtherWork: {},
}

// Observation is the evidence available when an agent or supervisor proposes
// waiting, asking the owner, or stopping the whole goal.
type Observation struct {
	Kind                    Kind
	Scope                   Scope
	SafeInScopeActions      int
	RecoveryAttempts        int
	RecoveryExhausted       bool
	ConsecutiveObservations int
	ObservedAt              time.Time
	EvidenceRefs            []string
	AccountableParty        string
	ResumeCondition         string
	RequiresUserAction      bool
}

// Decision separates execution continuity from owner judgment. A mechanical
// user-only provider action is not mislabeled as an owner decision.
type Decision struct {
	GoalState             State
	LaneState             State
	Continue              bool
	OwnerDecisionRequired bool
	UserActionRequired    bool
}

// Evaluate applies the shared continuity contract. It is intentionally strict
// about whole-goal external blocking and permissive about continued recovery:
// failing to prove a stop means keep the goal alive, not invent permission.
func Evaluate(in Observation) (Decision, error) {
	if in.Scope != ScopeLane && in.Scope != ScopeGoal {
		return Decision{}, fmt.Errorf("unknown continuity scope %q", in.Scope)
	}
	if _, ok := validKinds[in.Kind]; !ok {
		return Decision{}, fmt.Errorf("unknown continuity kind %q", in.Kind)
	}
	if in.SafeInScopeActions < 0 || in.RecoveryAttempts < 0 || in.ConsecutiveObservations < 0 {
		return Decision{}, fmt.Errorf("continuity counts must be non-negative")
	}

	if in.SafeInScopeActions > 0 {
		return decisionWithSafeAlternative(in), nil
	}

	if ownerBoundary(in.Kind) {
		if in.Scope == ScopeLane {
			return Decision{GoalState: StateActive, LaneState: StateNeedsOwner, Continue: true, OwnerDecisionRequired: true}, nil
		}
		return Decision{
			GoalState:             StateNeedsOwner,
			LaneState:             StateNeedsOwner,
			OwnerDecisionRequired: true,
		}, nil
	}

	if in.Scope == ScopeLane {
		laneState := StateDegraded
		if laneWaitingEvidence(in) {
			laneState = StateWaitingLane
		}
		return Decision{GoalState: StateActive, LaneState: laneState, Continue: true}, nil
	}

	if externallyBlockedEvidence(in) {
		return Decision{
			GoalState:          StateExternalBlocked,
			LaneState:          StateExternalBlocked,
			UserActionRequired: in.RequiresUserAction,
		}, nil
	}

	return Decision{GoalState: StateDegraded, LaneState: StateDegraded, Continue: true}, nil
}

func laneWaitingEvidence(in Observation) bool {
	switch in.Kind {
	case KindResourceOccupied, KindExternalDependency, KindProductionLock, KindProviderSafetyLock:
		return len(in.EvidenceRefs) > 0 && in.AccountableParty != "" && in.ResumeCondition != ""
	default:
		return false
	}
}

func decisionWithSafeAlternative(in Observation) Decision {
	laneState := StateDegraded
	ownerDecisionRequired := false
	switch {
	case ownerBoundary(in.Kind):
		laneState = StateNeedsOwner
		ownerDecisionRequired = true
	case in.Kind == KindResourceOccupied,
		in.Kind == KindProductionLock,
		in.Kind == KindProviderSafetyLock,
		in.Kind == KindExternalDependency:
		laneState = StateWaitingLane
	}
	return Decision{
		GoalState:             StateActive,
		LaneState:             laneState,
		Continue:              true,
		OwnerDecisionRequired: ownerDecisionRequired,
	}
}

func ownerBoundary(kind Kind) bool {
	switch kind {
	case KindProductDecision,
		KindSecurityTrustChange,
		KindAccessChange,
		KindDestructiveAction,
		KindProductionDataAction,
		KindBudgetChange,
		KindOutboundCommunication,
		KindScopeExpansion,
		KindInterruptOtherWork:
		return true
	default:
		return false
	}
}

func externallyBlockedEvidence(in Observation) bool {
	return in.ConsecutiveObservations >= 3 &&
		in.RecoveryExhausted &&
		!in.ObservedAt.IsZero() &&
		len(in.EvidenceRefs) > 0 &&
		in.AccountableParty != "" &&
		in.ResumeCondition != ""
}

// RuntimeInstruction is injected into every Multica-managed provider. The
// embedded file is also the source for standalone provider adapters, keeping
// Codex, Claude, and Copilot on byte-identical policy prose.
//
//go:embed runtime_instruction.txt
var runtimeInstruction string

var RuntimeInstruction = strings.TrimSpace(runtimeInstruction)
