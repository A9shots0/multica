package workflowstate

import "testing"

type scenarioStep struct {
	event Event
	actor string
	want  State
}

func runScenario(t *testing.T, initial State, base Input, steps []scenarioStep) []Decision {
	t.Helper()
	state := initial
	decisions := make([]Decision, 0, len(steps))
	for _, step := range steps {
		in := base
		in.Current = state
		in.Event = step.event
		in.ActorID = step.actor
		decision := Decide(in)
		if decision.Ignored {
			t.Fatalf("event %s from %s was ignored in %s: %+v", step.event, step.actor, state, decision)
		}
		if decision.Next != step.want {
			t.Fatalf("event %s from %s moved %s -> %s, want %s", step.event, step.actor, state, decision.Next, step.want)
		}
		decisions = append(decisions, decision)
		state = decision.Next
	}
	return decisions
}

func TestScenarioNormalLiveSuccessHasNoLeaderRelay(t *testing.T) {
	decisions := runScenario(t, Working, Input{
		IssueID:            "issue-1",
		ImplementerID:      "worker-a",
		ReviewerID:         "reviewer-b",
		ReleaseAgentID:     "release-c",
		VerifierID:         "verify-d",
		RequiresDeployment: true,
	}, []scenarioStep{
		{event: ImplementationReady, actor: "worker-a", want: TechnicalReview},
		{event: ReviewPassed, actor: "reviewer-b", want: Deploying},
		{event: DeploymentSucceeded, actor: "release-c", want: ProductionVerifying},
		{event: ProductionVerified, actor: "verify-d", want: Live},
	})

	for i, decision := range decisions {
		if decision.WakeLeader {
			t.Fatalf("normal transition %d woke the squad leader: %+v", i, decision)
		}
	}
}

func TestScenarioReviewCorrectionLoopRoutesDirectly(t *testing.T) {
	decisions := runScenario(t, Working, Input{
		IssueID:        "issue-1",
		ImplementerID:  "worker-a",
		ReviewerID:     "reviewer-b",
		MaxCorrections: 1,
	}, []scenarioStep{
		{event: ImplementationReady, actor: "worker-a", want: TechnicalReview},
		{event: ReviewFailed, actor: "reviewer-b", want: Correction},
		{event: TaskStarted, actor: "worker-a", want: Working},
		{event: ImplementationReady, actor: "worker-a", want: TechnicalReview},
		{event: ReviewPassed, actor: "reviewer-b", want: Done},
	})

	if decisions[1].EnqueueAgentID != "worker-a" {
		t.Fatalf("review failure did not return directly to implementer: %+v", decisions[1])
	}
	if decisions[3].EnqueueAgentID != "reviewer-b" {
		t.Fatalf("corrected implementation did not return directly to reviewer: %+v", decisions[3])
	}
}

func TestScenarioFailedReleaseAlertsAndRollsBackBeforeCorrection(t *testing.T) {
	decisions := runScenario(t, Deploying, Input{
		IssueID:        "issue-1",
		ReleaseAgentID: "release-c",
		VerifierID:     "verify-d",
	}, []scenarioStep{
		{event: DeploymentSucceeded, actor: "release-c", want: ProductionVerifying},
		{event: ProductionFailed, actor: "verify-d", want: RollingBack},
		{event: RollbackSucceeded, actor: "release-c", want: RolledBack},
	})

	failure := decisions[1]
	if !failure.NotifyOwner || !failure.Urgent || failure.EnqueueAgentID != "release-c" {
		t.Fatalf("failed production check did not alert and dispatch rollback: %+v", failure)
	}
}
