package taskcontinuity

import (
	"testing"
	"time"
)

var observedAt = time.Date(2026, time.October, 18, 12, 0, 0, 0, time.UTC)

func TestEvaluateRecoveryAndBoundaryMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   Observation
		want Decision
	}{
		{
			name: "browser unavailable keeps UI recovery active",
			in: Observation{
				Kind:                    KindBrowserUnavailable,
				Scope:                   ScopeLane,
				RecoveryExhausted:       true,
				ConsecutiveObservations: 3,
				ObservedAt:              observedAt,
				EvidenceRefs:            []string{"browser:all-surfaces-failed"},
				AccountableParty:        "browser runtime",
				ResumeCondition:         "a supported browser surface works",
			},
			want: Decision{GoalState: StateActive, LaneState: StateDegraded, Continue: true},
		},
		{
			name: "browser unavailable cannot externally block UI goal",
			in: Observation{
				Kind:                    KindBrowserUnavailable,
				Scope:                   ScopeGoal,
				RecoveryExhausted:       true,
				ConsecutiveObservations: 3,
				ObservedAt:              observedAt,
				EvidenceRefs:            []string{"browser:all-surfaces-failed"},
				AccountableParty:        "browser runtime",
				ResumeCondition:         "a supported browser surface works",
			},
			want: Decision{GoalState: StateDegraded, LaneState: StateDegraded, Continue: true},
		},
		{
			name: "stale Brain evidence starts recovery rather than asking owner",
			in: Observation{
				Kind:                    KindEvidenceUnavailable,
				Scope:                   ScopeGoal,
				ConsecutiveObservations: 1,
			},
			want: Decision{GoalState: StateDegraded, LaneState: StateDegraded, Continue: true},
		},
		{
			name: "dirty checkout with isolated worktree option stays active",
			in: Observation{
				Kind:               KindWorkspaceConflict,
				Scope:              ScopeLane,
				SafeInScopeActions: 1,
			},
			want: Decision{GoalState: StateActive, LaneState: StateDegraded, Continue: true},
		},
		{
			name: "occupied lane waits without stopping its parent",
			in: Observation{
				Kind:                    KindResourceOccupied,
				Scope:                   ScopeLane,
				ConsecutiveObservations: 3,
				EvidenceRefs:            []string{"lock:repo/path"},
				AccountableParty:        "release queue",
				ResumeCondition:         "the current lease is released",
			},
			want: Decision{GoalState: StateActive, LaneState: StateWaitingLane, Continue: true},
		},
		{
			name: "transient provider failure remains recoverable",
			in: Observation{
				Kind:                    KindTransientProvider,
				Scope:                   ScopeGoal,
				RecoveryAttempts:        2,
				ConsecutiveObservations: 2,
			},
			want: Decision{GoalState: StateDegraded, LaneState: StateDegraded, Continue: true},
		},
		{
			name: "context recovery remains agent-owned",
			in: Observation{
				Kind:             KindContextRecovery,
				Scope:            ScopeGoal,
				RecoveryAttempts: 1,
			},
			want: Decision{GoalState: StateDegraded, LaneState: StateDegraded, Continue: true},
		},
		{
			name: "whole-goal product decision needs owner",
			in: Observation{
				Kind:  KindProductDecision,
				Scope: ScopeGoal,
			},
			want: Decision{GoalState: StateNeedsOwner, LaneState: StateNeedsOwner, OwnerDecisionRequired: true},
		},
		{
			name: "destructive action without authority needs owner",
			in: Observation{
				Kind:  KindDestructiveAction,
				Scope: ScopeGoal,
			},
			want: Decision{GoalState: StateNeedsOwner, LaneState: StateNeedsOwner, OwnerDecisionRequired: true},
		},
		{
			name: "owner decision in one lane does not stop other work",
			in: Observation{
				Kind:               KindProductDecision,
				Scope:              ScopeLane,
				SafeInScopeActions: 2,
			},
			want: Decision{GoalState: StateActive, LaneState: StateNeedsOwner, Continue: true, OwnerDecisionRequired: true},
		},
		{
			name: "evidenced production lock can externally block whole goal",
			in: Observation{
				Kind:                    KindProductionLock,
				Scope:                   ScopeGoal,
				RecoveryExhausted:       true,
				ConsecutiveObservations: 3,
				ObservedAt:              observedAt,
				EvidenceRefs:            []string{"release-queue:lease-42"},
				AccountableParty:        "production release queue",
				ResumeCondition:         "lease 42 is released",
			},
			want: Decision{GoalState: StateExternalBlocked, LaneState: StateExternalBlocked},
		},
		{
			name: "provider safety lock is mechanical user action not owner decision",
			in: Observation{
				Kind:                    KindProviderSafetyLock,
				Scope:                   ScopeGoal,
				RecoveryExhausted:       true,
				ConsecutiveObservations: 3,
				ObservedAt:              observedAt,
				EvidenceRefs:            []string{"provider:goal-blocked"},
				AccountableParty:        "provider safety system",
				ResumeCondition:         "the user resumes or the provider clears the goal",
				RequiresUserAction:      true,
			},
			want: Decision{GoalState: StateExternalBlocked, LaneState: StateExternalBlocked, UserActionRequired: true},
		},
		{
			name: "first production-lock observation cannot block",
			in: Observation{
				Kind:                    KindProductionLock,
				Scope:                   ScopeGoal,
				RecoveryExhausted:       true,
				ConsecutiveObservations: 1,
				ObservedAt:              observedAt,
				EvidenceRefs:            []string{"release-queue:lease-42"},
				AccountableParty:        "production release queue",
				ResumeCondition:         "lease 42 is released",
			},
			want: Decision{GoalState: StateDegraded, LaneState: StateDegraded, Continue: true},
		},
		{
			name: "safe action defeats external-block classification",
			in: Observation{
				Kind:                    KindProductionLock,
				Scope:                   ScopeGoal,
				SafeInScopeActions:      1,
				RecoveryExhausted:       true,
				ConsecutiveObservations: 3,
				ObservedAt:              observedAt,
				EvidenceRefs:            []string{"release-queue:lease-42"},
				AccountableParty:        "production release queue",
				ResumeCondition:         "lease 42 is released",
			},
			want: Decision{GoalState: StateActive, LaneState: StateWaitingLane, Continue: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Evaluate(tt.in)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("Evaluate() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestExternalBlockRequiresEveryProof(t *testing.T) {
	t.Parallel()

	base := Observation{
		Kind:                    KindExternalDependency,
		Scope:                   ScopeGoal,
		RecoveryAttempts:        3,
		RecoveryExhausted:       true,
		ConsecutiveObservations: 3,
		ObservedAt:              observedAt,
		EvidenceRefs:            []string{"provider:event-42"},
		AccountableParty:        "provider",
		ResumeCondition:         "provider event 42 clears",
	}
	mutations := map[string]func(*Observation){
		"recovery not exhausted": func(in *Observation) { in.RecoveryExhausted = false },
		"undated evidence":       func(in *Observation) { in.ObservedAt = time.Time{} },
		"too few observations":   func(in *Observation) { in.ConsecutiveObservations = 2 },
		"no evidence":            func(in *Observation) { in.EvidenceRefs = nil },
		"no accountable party":   func(in *Observation) { in.AccountableParty = "" },
		"no resume condition":    func(in *Observation) { in.ResumeCondition = "" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			in := base
			mutate(&in)
			got, err := Evaluate(in)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if got.GoalState == StateExternalBlocked || !got.Continue {
				t.Fatalf("Evaluate() = %+v, want recoverable degraded state", got)
			}
		})
	}
}

func TestEvaluateRejectsMalformedObservation(t *testing.T) {
	t.Parallel()

	tests := []Observation{
		{Kind: KindToolUnavailable, Scope: "unknown"},
		{Kind: "unknown", Scope: ScopeGoal},
		{Kind: KindToolUnavailable, Scope: ScopeGoal, SafeInScopeActions: -1},
		{Kind: KindToolUnavailable, Scope: ScopeGoal, RecoveryAttempts: -1},
		{Kind: KindToolUnavailable, Scope: ScopeGoal, ConsecutiveObservations: -1},
	}

	for _, in := range tests {
		if _, err := Evaluate(in); err == nil {
			t.Fatalf("Evaluate(%+v) succeeded, want validation error", in)
		}
	}
}

func TestRuntimeInstructionCarriesTheBehavioralContract(t *testing.T) {
	t.Parallel()

	wants := []string{
		"preferred tool",
		"other safe in-scope work",
		"whole goal",
		"three unchanged checks",
		"recovery is exhausted",
		"Do not ask the owner",
		"Browser access is mandatory for UI work",
		"do not replace browser-based UI verification",
		"Browser recovery is agent-owned",
	}
	for _, want := range wants {
		if !contains(RuntimeInstruction, want) {
			t.Errorf("RuntimeInstruction missing %q", want)
		}
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
