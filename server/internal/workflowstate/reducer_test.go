package workflowstate

import "testing"

func TestDecideScenarioMatrix(t *testing.T) {
	tests := []struct {
		name string
		in   Input
		want Decision
	}{
		{
			name: "worker success routes directly to independent review",
			in:   Input{Current: Working, Event: ImplementationReady, ActorID: "worker-a", ImplementerID: "worker-a", ReviewerID: "reviewer-b"},
			want: Decision{Next: TechnicalReview, EnqueueAgentID: "reviewer-b"},
		},
		{
			name: "self review is refused and leader is asked to repair routing",
			in:   Input{Current: Working, Event: ImplementationReady, ActorID: "worker-a", ImplementerID: "worker-a", ReviewerID: "worker-a"},
			want: Decision{Next: Blocked, WakeLeader: true, Reason: ReasonIndependentReviewerMissing},
		},
		{
			name: "review failure routes to implementer without leader relay",
			in:   Input{Current: TechnicalReview, Event: ReviewFailed, ActorID: "reviewer-b", ImplementerID: "worker-a", ReviewerID: "reviewer-b", MaxCorrections: 1},
			want: Decision{Next: Correction, EnqueueAgentID: "worker-a"},
		},
		{
			name: "review pass begins required deployment",
			in:   Input{Current: TechnicalReview, Event: ReviewPassed, ActorID: "reviewer-b", ImplementerID: "worker-a", ReviewerID: "reviewer-b", RequiresDeployment: true, ReleaseAgentID: "release-c"},
			want: Decision{Next: Deploying, EnqueueAgentID: "release-c"},
		},
		{
			name: "review pass requests real owner gate only when declared",
			in:   Input{Current: TechnicalReview, Event: ReviewPassed, ActorID: "reviewer-b", ImplementerID: "worker-a", ReviewerID: "reviewer-b", RequiresOwner: true},
			want: Decision{Next: NeedsOwner, NotifyOwner: true},
		},
		{
			name: "review pass completes non live work",
			in:   Input{Current: TechnicalReview, Event: ReviewPassed, ActorID: "reviewer-b", ImplementerID: "worker-a", ReviewerID: "reviewer-b"},
			want: Decision{Next: Done},
		},
		{
			name: "deployment success requires production verification",
			in:   Input{Current: Deploying, Event: DeploymentSucceeded, ActorID: "release-c", ReleaseAgentID: "release-c", VerifierID: "verify-d"},
			want: Decision{Next: ProductionVerifying, EnqueueAgentID: "verify-d"},
		},
		{
			name: "production pass is live",
			in:   Input{Current: ProductionVerifying, Event: ProductionVerified, ActorID: "verify-d", VerifierID: "verify-d"},
			want: Decision{Next: Live},
		},
		{
			name: "production failure starts rollback and alerts owner once",
			in:   Input{Current: ProductionVerifying, Event: ProductionFailed, ActorID: "verify-d", VerifierID: "verify-d", ReleaseAgentID: "release-c"},
			want: Decision{Next: RollingBack, EnqueueAgentID: "release-c", NotifyOwner: true, Urgent: true},
		},
		{
			name: "rollback is not complete until restored health is proven",
			in:   Input{Current: RollingBack, Event: RollbackSucceeded, ActorID: "release-c", ReleaseAgentID: "release-c"},
			want: Decision{Next: RolledBack},
		},
		{
			name: "technical blocker wakes leader not owner",
			in:   Input{Current: Working, Event: DependencyBlocked, ActorID: "worker-a"},
			want: Decision{Next: Blocked, WakeLeader: true},
		},
		{
			name: "declared owner question notifies owner",
			in:   Input{Current: Working, Event: OwnerDecisionRequired, ActorID: "worker-a"},
			want: Decision{Next: NeedsOwner, NotifyOwner: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Decide(tt.in)
			if got != tt.want {
				t.Fatalf("Decide(%+v) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

func TestReviewFailureStopsAfterDeclaredCorrectionLimit(t *testing.T) {
	got := Decide(Input{
		Current: TechnicalReview, Event: ReviewFailed,
		ActorID: "reviewer-b", ImplementerID: "worker-a", ReviewerID: "reviewer-b",
		LeaderID: "leader-c", CorrectionCount: 1, MaxCorrections: 1,
	})
	want := Decision{
		Next: Blocked, EnqueueAgentID: "leader-c", WakeLeader: true,
		Reason: ReasonCorrectionLimitReached,
	}
	if got != want {
		t.Fatalf("correction ceiling = %+v, want %+v", got, want)
	}
}

func TestDecideRejectsInvalidAndDuplicateEvents(t *testing.T) {
	tests := []struct {
		name string
		in   Input
	}{
		{
			name: "duplicate is no op",
			in:   Input{Current: Working, Event: ImplementationReady, EventAlreadyApplied: true, ReviewerID: "reviewer-b"},
		},
		{
			name: "review pass from wrong state is refused",
			in:   Input{Current: Working, Event: ReviewPassed, ActorID: "reviewer-b", ReviewerID: "reviewer-b"},
		},
		{
			name: "review outcome from wrong actor is refused",
			in:   Input{Current: TechnicalReview, Event: ReviewPassed, ActorID: "worker-a", ImplementerID: "worker-a", ReviewerID: "reviewer-b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Decide(tt.in)
			if got.Next != tt.in.Current || got.EnqueueAgentID != "" || got.NotifyOwner || got.WakeLeader {
				t.Fatalf("invalid/duplicate event changed workflow: %+v", got)
			}
			if !got.Ignored {
				t.Fatalf("invalid/duplicate event was not marked ignored: %+v", got)
			}
		})
	}
}

func TestInterruptedWorkRecoveryIsScopedAndAutomatic(t *testing.T) {
	got := Decide(Input{
		Current:        Working,
		Event:          TaskInterrupted,
		ActorID:        "worker-a",
		ImplementerID:  "worker-a",
		CanAutoRecover: true,
		IssueID:        "issue-1",
	})
	want := Decision{Next: Queued, EnqueueAgentID: "worker-a", RecoveryIssueID: "issue-1"}
	if got != want {
		t.Fatalf("recovery = %+v, want %+v", got, want)
	}
}

func TestOwnerAndDependencyResumptionReturnsToExactSuspendedStep(t *testing.T) {
	tests := []struct {
		name string
		in   Input
		want Decision
	}{
		{
			name: "owner answer resumes deployment",
			in:   Input{Current: NeedsOwner, Event: OwnerDecisionRecorded, ResumeState: Deploying, ResumeAgentID: "release-c"},
			want: Decision{Next: Deploying, EnqueueAgentID: "release-c"},
		},
		{
			name: "owner approval completes reviewed non deployment work",
			in:   Input{Current: NeedsOwner, Event: OwnerDecisionRecorded, ResumeState: Done},
			want: Decision{Next: Done},
		},
		{
			name: "resolved dependency resumes review",
			in:   Input{Current: Blocked, Event: DependencyResolved, ResumeState: TechnicalReview, ResumeAgentID: "reviewer-b"},
			want: Decision{Next: TechnicalReview, EnqueueAgentID: "reviewer-b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Decide(tt.in); got != tt.want {
				t.Fatalf("resume = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParallelIssuesDoNotShareRecoveryScope(t *testing.T) {
	first := Decide(Input{Current: Working, Event: TaskInterrupted, IssueID: "issue-1", ImplementerID: "worker-a", CanAutoRecover: true})
	second := Decide(Input{Current: Working, Event: TaskInterrupted, IssueID: "issue-2", ImplementerID: "worker-a", CanAutoRecover: true})
	if first.RecoveryIssueID != "issue-1" || second.RecoveryIssueID != "issue-2" {
		t.Fatalf("recovery scopes crossed: first=%+v second=%+v", first, second)
	}
}
