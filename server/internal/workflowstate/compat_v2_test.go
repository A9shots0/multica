package workflowstate

import (
	"errors"
	"testing"

	workflowstatev2 "github.com/multica-ai/multica/server/internal/workflowstate/v2"
)

func legacyPicture() workflowstatev2.CurrentPicture {
	p := workflowstatev2.NewCurrentPicture("issue-1", 1, 1, "owner-acceptance")
	p.Scope.DeploymentPolicy = workflowstatev2.DeploymentNotRequired
	p.Scope.SourcePlanRevision = 1
	return p
}

func applyLegacyV2(t *testing.T, picture workflowstatev2.CurrentPicture, in LegacyV2Input) workflowstatev2.CurrentPicture {
	t.Helper()
	if in.ReleasePreparation != nil {
		in.ReleasePreparation.IssueID = picture.IssueID
		in.ReleasePreparation.ScopeRevision = picture.Scope.Revision
		in.ReleasePreparation.AuthorityRevision = picture.Authority.Revision
		in.ReleasePreparation.Expected = allConcernRevisions(picture)
	}
	commands, err := AdaptLegacyEvent(picture, in)
	if err != nil {
		t.Fatalf("adapt %s: %v", in.Legacy.Event, err)
	}
	for _, command := range commands {
		result, err := workflowstatev2.Apply(workflowstatev2.ApplyInput{
			Picture: picture, Command: command, AuthoritativeTargetGeneration: in.AuthoritativeTargetGeneration,
		})
		if err != nil {
			t.Fatalf("apply adapted %s command %s: %v", in.Legacy.Event, command.Kind, err)
		}
		picture = result.Picture
	}
	return picture
}

func legacyFixture(event Event) LegacyV2Input {
	generation := workflowstatev2.Revision(1)
	in := LegacyV2Input{
		Legacy: Input{
			IssueID: "issue-1", Current: Failed, Event: event,
			ActorID:       "worker-a",
			ImplementerID: "worker-a", ReviewerID: "reviewer-b",
			ReleaseAgentID: "release-c", VerifierID: "verify-d",
			MaxCorrections: 1,
		},
		ReleasePreparation: &workflowstatev2.CommandEnvelope{
			Kind:    workflowstatev2.CommandPrepareRelease,
			ActorID: "controller-1", ActorRole: workflowstatev2.RoleController,
			OperationID: "operation-1", BranchID: "branch-1",
			CandidateID: "candidate-1", TargetID: "production", TargetGeneration: 1,
			AcceptanceVersion: 1, ReleaseOperatorID: "release-c", PlannedVerifierID: "verify-d",
			AuthorityCapability: "deploy_exact_release", AuthorityEvidenceID: "authority-1", OwnerEvidenceRevision: 1,
			RecoveryPolicyRevision: 1, ExposurePolicyDigest: "exposure-policy",
			BuildManifestDigest: "manifest", ArtifactDigest: "artifact", RuntimeContractDigest: "runtime",
		},
		EventID: "event-" + string(event), ControllerID: "controller-1",
		OperationID: "operation-1", CandidateID: "candidate-1",
		TargetID: "production", TargetGeneration: 1, AuthoritativeTargetGeneration: &generation,
		RunnableTargetID: "worker-a", CapacityEvidenceRef: "capacity-1",
		LeaseID: "lease-1", CheckpointID: "checkpoint-1", NextAutomaticAction: "dispatch",
		CriteriaRefs: []string{"acceptance-1"}, CoverageRefs: []string{"test-1"},
		Evidence: LegacyEvidenceRefs{
			Implementation: "implementation-1", Review: "review-1", Build: "build-1",
			Authorization: "authority-1", DeploymentStart: "deploy-start-1",
			Deployment: "deployment-1", Exposure: "exposure-1",
			VerificationStart: "verify-start-1", Runtime: "runtime-1", Business: "business-1",
		},
	}
	switch event {
	case ReviewPassed, ReviewFailed:
		in.Legacy.ActorID = "reviewer-b"
	case DeploymentSucceeded:
		in.Legacy.ActorID = "release-c"
	case ProductionVerified:
		in.Legacy.ActorID = "verify-d"
	}
	return in
}

func TestLegacyAdapterPreservesPilotLiveSuccessWithoutTrustingLegacyState(t *testing.T) {
	picture := legacyPicture()
	picture.Scope.DeploymentPolicy = workflowstatev2.DeploymentRequired
	grant := workflowstatev2.CommandEnvelope{
		CommandID: "grant-1", IssueID: picture.IssueID, Kind: workflowstatev2.CommandGrantAuthority,
		ActorID: "owner-1", ActorRole: workflowstatev2.RoleOwner,
		Expected: allConcernRevisions(picture), ScopeRevision: picture.Scope.Revision,
		AuthorityRevision: picture.Authority.Revision, CandidateID: "candidate-1", TargetID: "production",
	}
	granted, err := workflowstatev2.Apply(workflowstatev2.ApplyInput{Picture: picture, Command: grant})
	if err != nil {
		t.Fatal(err)
	}
	picture = granted.Picture

	in := legacyFixture(IssueReceived)
	in.Legacy.RequiresDeployment = true
	picture = applyLegacyV2(t, picture, in)
	if picture.Execution.State != workflowstatev2.ExecutionQueued {
		t.Fatalf("received adapter state = %s", picture.Execution.State)
	}

	in = legacyFixture(TaskStarted)
	in.Legacy.RequiresDeployment = true
	picture = applyLegacyV2(t, picture, in)
	in = legacyFixture(ImplementationReady)
	in.Legacy.RequiresDeployment = true
	picture = applyLegacyV2(t, picture, in)
	in = legacyFixture(ReviewPassed)
	in.Legacy.RequiresDeployment = true
	picture = applyLegacyV2(t, picture, in)
	if picture.Release.State != workflowstatev2.ReleaseDeploying {
		t.Fatalf("review pass adapter release = %s", picture.Release.State)
	}

	in = legacyFixture(DeploymentSucceeded)
	in.Legacy.ActorID = "release-c"
	in.Legacy.RequiresDeployment = true
	picture = applyLegacyV2(t, picture, in)
	if picture.Release.State != workflowstatev2.ReleaseProductionVerifying {
		t.Fatalf("deployment adapter release = %s", picture.Release.State)
	}

	in = legacyFixture(ProductionVerified)
	in.Legacy.ActorID = "verify-d"
	in.Legacy.RequiresDeployment = true
	picture = applyLegacyV2(t, picture, in)
	if picture.Release.State != workflowstatev2.ReleaseLive || picture.Outcome != workflowstatev2.OutcomeSatisfied {
		t.Fatalf("verified adapter picture = release %s outcome %s", picture.Release.State, picture.Outcome)
	}
}

func TestLegacyAdapterPreservesPilotCorrectionLoop(t *testing.T) {
	picture := legacyPicture()
	picture = applyLegacyV2(t, picture, legacyFixture(IssueReceived))
	picture = applyLegacyV2(t, picture, legacyFixture(TaskStarted))
	picture = applyLegacyV2(t, picture, legacyFixture(ImplementationReady))

	failed := legacyFixture(ReviewFailed)
	failed.Legacy.ActorID = "reviewer-b"
	failed.GapRefs = []string{"gap-1"}
	failed.RequiredCorrection = "repair the failing boundary"
	picture = applyLegacyV2(t, picture, failed)
	if picture.Execution.State != workflowstatev2.ExecutionCorrection {
		t.Fatalf("failed review state = %s", picture.Execution.State)
	}
	if picture.Review.CorrectionCount != 1 || picture.Review.MaxCorrections != 1 {
		t.Fatalf("correction budget = %d/%d, want 1/1", picture.Review.CorrectionCount, picture.Review.MaxCorrections)
	}

	restart := legacyFixture(TaskStarted)
	restart.EventID = "event-correction-start"
	restart.OperationID = "operation-2"
	restart.LeaseID = "lease-2"
	picture = applyLegacyV2(t, picture, restart)

	ready := legacyFixture(ImplementationReady)
	ready.EventID = "event-correction-ready"
	ready.CandidateID = "candidate-2"
	ready.Evidence.Implementation = "implementation-2"
	picture = applyLegacyV2(t, picture, ready)

	passed := legacyFixture(ReviewPassed)
	passed.EventID = "event-correction-passed"
	passed.Legacy.ActorID = "reviewer-b"
	passed.CandidateID = "candidate-2"
	passed.Evidence.Review = "review-2"
	picture = applyLegacyV2(t, picture, passed)
	if picture.Outcome != workflowstatev2.OutcomeSatisfied || picture.Execution.State != workflowstatev2.ExecutionCompleted || picture.Release.State != workflowstatev2.ReleaseNotRequired {
		t.Fatalf("corrected adapter picture = outcome %s execution %s release %s", picture.Outcome, picture.Execution.State, picture.Release.State)
	}
}

func TestLegacyAdapterTaskStartedRequiresQueuedRunnableTarget(t *testing.T) {
	picture := legacyPicture()
	picture = applyLegacyV2(t, picture, legacyFixture(IssueReceived))

	valid := legacyFixture(TaskStarted)
	commands, err := AdaptLegacyEvent(picture, valid)
	if err != nil || len(commands) != 1 || commands[0].ActorID != picture.Execution.RunnableTargetID {
		t.Fatalf("valid queued actor rejected: commands=%+v err=%v", commands, err)
	}

	wrong := legacyFixture(TaskStarted)
	wrong.Legacy.ActorID = "different-worker"
	requireLegacyAdapterReject(t, picture, wrong)
}

func TestLegacyAdapterImplementationReadyRequiresIndependentReviewer(t *testing.T) {
	picture := legacyWorkingPicture(t)

	valid := legacyFixture(ImplementationReady)
	commands, err := AdaptLegacyEvent(picture, valid)
	if err != nil || len(commands) != 1 || commands[0].ReviewerID != "reviewer-b" {
		t.Fatalf("valid reviewer binding rejected: commands=%+v err=%v", commands, err)
	}
	result, err := workflowstatev2.Apply(workflowstatev2.ApplyInput{Picture: picture, Command: commands[0]})
	if err != nil || result.Picture.Review.ReviewerID != "reviewer-b" {
		t.Fatalf("planned reviewer was not preserved in v2: picture=%+v err=%v", result.Picture.Review, err)
	}
	reviewCommands, err := AdaptLegacyEvent(result.Picture, legacyFixture(ReviewPassed))
	if err != nil || len(reviewCommands) == 0 {
		t.Fatalf("build planned review command: commands=%+v err=%v", reviewCommands, err)
	}
	reviewCommands[0].ActorID = "unplanned-reviewer"
	_, err = workflowstatev2.Apply(workflowstatev2.ApplyInput{Picture: result.Picture, Command: reviewCommands[0]})
	var gateErr *workflowstatev2.GateError
	if !errors.As(err, &gateErr) || gateErr.Code != workflowstatev2.GateInvalidActorRole {
		t.Fatalf("v2 accepted unplanned reviewer: %v", err)
	}

	for _, mutate := range []func(*LegacyV2Input){
		func(in *LegacyV2Input) { in.Legacy.ReviewerID = "" },
		func(in *LegacyV2Input) { in.Legacy.ReviewerID = in.Legacy.ImplementerID },
	} {
		invalid := legacyFixture(ImplementationReady)
		mutate(&invalid)
		requireLegacyAdapterReject(t, picture, invalid)
	}
}

func TestLegacyAdapterReviewFailedRequiresCorrectionCapacity(t *testing.T) {
	picture := legacyReviewingPicture(t)
	valid := legacyFixture(ReviewFailed)
	valid.Legacy.MaxCorrections = 2
	valid.GapRefs, valid.RequiredCorrection = []string{"gap-1"}, "repair it"
	commands, err := AdaptLegacyEvent(picture, valid)
	if err != nil || len(commands) != 1 || commands[0].CorrectionCount != 1 || commands[0].MaxCorrections != 2 {
		t.Fatalf("valid correction rejected or budget lost: commands=%+v err=%v", commands, err)
	}

	tests := []struct {
		name   string
		mutate func(*LegacyV2Input)
	}{
		{"zero limit", func(in *LegacyV2Input) { in.Legacy.MaxCorrections = 0 }},
		{"exhausted", func(in *LegacyV2Input) { in.Legacy.CorrectionCount = 2 }},
		{"negative count", func(in *LegacyV2Input) { in.Legacy.CorrectionCount = -1 }},
		{"missing implementer", func(in *LegacyV2Input) { in.Legacy.ImplementerID = "" }},
		{"unavailable implementer", func(in *LegacyV2Input) { in.RunnableTargetID = "other-worker" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			invalid := valid
			test.mutate(&invalid)
			requireLegacyAdapterReject(t, picture, invalid)
		})
	}

	correcting, err := workflowstatev2.Apply(workflowstatev2.ApplyInput{Picture: picture, Command: commands[0]})
	if err != nil {
		t.Fatal(err)
	}
	nextReview := applyLegacyV2(t, correcting.Picture, func() LegacyV2Input {
		started := legacyFixture(TaskStarted)
		started.EventID, started.OperationID = "event-next-start", "operation-2"
		return started
	}())
	nextReady := legacyFixture(ImplementationReady)
	nextReady.EventID, nextReady.CandidateID = "event-next-ready", "candidate-2"
	nextReady.Evidence.Implementation = "implementation-2"
	nextReview = applyLegacyV2(t, nextReview, nextReady)
	staleBudget := valid
	staleBudget.EventID, staleBudget.CandidateID = "event-stale-budget", "candidate-2"
	staleBudget.Legacy.CorrectionCount = 0
	requireLegacyAdapterReject(t, nextReview, staleBudget)
}

func TestLegacyAdapterDeploymentSucceededRequiresIndependentVerifier(t *testing.T) {
	picture := legacyDeployingPicture(t)
	valid := legacyFixture(DeploymentSucceeded)
	commands, err := AdaptLegacyEvent(picture, valid)
	if err != nil || len(commands) != 3 || commands[2].ActorID != "verify-d" {
		t.Fatalf("valid deployment handoff rejected: commands=%+v err=%v", commands, err)
	}

	for _, verifier := range []string{"", "release-c"} {
		invalid := legacyFixture(DeploymentSucceeded)
		invalid.Legacy.VerifierID = verifier
		requireLegacyAdapterReject(t, picture, invalid)
	}
}

func TestLegacyAdapterProductionVerificationUsesPersistedReleaseIdentities(t *testing.T) {
	picture := legacyDeployingPicture(t)
	picture = applyLegacyV2(t, picture, legacyFixture(DeploymentSucceeded))
	if picture.Release.ReleaseOperatorID != "release-c" || picture.Release.PlannedVerifierID != "verify-d" {
		t.Fatalf("release identities were not persisted: %+v", picture.Release)
	}

	drifted := legacyFixture(ProductionVerified)
	drifted.Legacy.ActorID = "release-c"
	drifted.Legacy.VerifierID = "release-c"
	drifted.Legacy.ReleaseAgentID = "other-release-operator"
	requireLegacyAdapterReject(t, picture, drifted)

	valid := legacyFixture(ProductionVerified)
	commands, err := AdaptLegacyEvent(picture, valid)
	if err != nil || len(commands) != 2 || commands[0].ActorID != "verify-d" || commands[1].ActorID != "verify-d" {
		t.Fatalf("persisted verifier path rejected: commands=%+v err=%v", commands, err)
	}
	verified := applyLegacyV2(t, picture, valid)
	if verified.Release.State != workflowstatev2.ReleaseLive || verified.Outcome != workflowstatev2.OutcomeSatisfied {
		t.Fatalf("valid persisted verifier did not complete release: release=%s outcome=%s", verified.Release.State, verified.Outcome)
	}
}

func legacyWorkingPicture(t *testing.T) workflowstatev2.CurrentPicture {
	t.Helper()
	picture := legacyPicture()
	picture = applyLegacyV2(t, picture, legacyFixture(IssueReceived))
	return applyLegacyV2(t, picture, legacyFixture(TaskStarted))
}

func legacyReviewingPicture(t *testing.T) workflowstatev2.CurrentPicture {
	t.Helper()
	return applyLegacyV2(t, legacyWorkingPicture(t), legacyFixture(ImplementationReady))
}

func legacyDeployingPicture(t *testing.T) workflowstatev2.CurrentPicture {
	t.Helper()
	picture := legacyReviewingPicture(t)
	picture.Scope.DeploymentPolicy = workflowstatev2.DeploymentRequired
	picture.Authority = workflowstatev2.AuthorityFacet{
		State: workflowstatev2.AuthorityActive, Revision: picture.Authority.Revision,
		ScopeRevision: picture.Scope.Revision, CandidateID: "candidate-1", TargetID: "production",
	}
	in := legacyFixture(ReviewPassed)
	in.Legacy.RequiresDeployment = true
	return applyLegacyV2(t, picture, in)
}

func requireLegacyAdapterReject(t *testing.T, picture workflowstatev2.CurrentPicture, in LegacyV2Input) {
	t.Helper()
	commands, err := AdaptLegacyEvent(picture, in)
	if len(commands) != 0 {
		t.Fatalf("invalid event returned commands: %+v", commands)
	}
	var adapterErr *LegacyAdapterError
	if !errors.As(err, &adapterErr) || adapterErr.Code != LegacyAdapterInvalidInput {
		t.Fatalf("invalid event error = %v, want %s", err, LegacyAdapterInvalidInput)
	}
}

func TestLegacyAdapterUnsupportedEventsFailClosed(t *testing.T) {
	picture := legacyPicture()
	unsupported := []Event{
		OwnerDecisionRequired, OwnerDecisionRecorded,
		DependencyBlocked, DependencyResolved, DeploymentStarted,
		ProductionFailed, RollbackSucceeded, TaskInterrupted,
	}
	for _, event := range unsupported {
		t.Run(string(event), func(t *testing.T) {
			commands, err := AdaptLegacyEvent(picture, legacyFixture(event))
			if len(commands) != 0 {
				t.Fatalf("unsupported event returned commands: %+v", commands)
			}
			var adapterErr *LegacyAdapterError
			if !errors.As(err, &adapterErr) || adapterErr.Code != LegacyAdapterUnsupportedEvent {
				t.Fatalf("unsupported event error = %v", err)
			}
		})
	}
}

func TestLegacyAdapterRequiresSeparatedProductionEvidence(t *testing.T) {
	picture := legacyPicture()
	picture.Release = workflowstatev2.ReleaseFacet{State: workflowstatev2.ReleaseProductionVerifying, CandidateID: "candidate-1", TargetID: "production", Generation: 1}
	in := legacyFixture(ProductionVerified)
	in.Legacy.ActorID = "verify-d"
	in.Evidence.Business = ""
	commands, err := AdaptLegacyEvent(picture, in)
	if err == nil || len(commands) != 0 {
		t.Fatalf("unqualified production verification was accepted: commands=%+v err=%v", commands, err)
	}
	if picture.Release.State != workflowstatev2.ReleaseProductionVerifying || picture.Outcome != workflowstatev2.OutcomeOpen {
		t.Fatalf("failed adaptation mutated authoritative picture: %+v", picture)
	}
}

func TestLegacyAdapterRequiresExactReleasePreparation(t *testing.T) {
	p := legacyReviewingPicture(t)
	p.Scope.DeploymentPolicy = workflowstatev2.DeploymentRequired
	p.Authority = workflowstatev2.AuthorityFacet{State: workflowstatev2.AuthorityActive, Revision: p.Authority.Revision, ScopeRevision: p.Scope.Revision, CandidateID: "candidate-1", TargetID: "production"}
	for _, mutate := range []func(*LegacyV2Input){
		func(in *LegacyV2Input) { in.ReleasePreparation = nil },
		func(in *LegacyV2Input) { in.ReleasePreparation.ActorID = "other-controller" },
		func(in *LegacyV2Input) { in.ReleasePreparation.ReleaseOperatorID = "other-operator" },
		func(in *LegacyV2Input) { in.ReleasePreparation.PlannedVerifierID = "other-verifier" },
		func(in *LegacyV2Input) { in.ReleasePreparation.TargetGeneration = 2 },
		func(in *LegacyV2Input) { in.ReleasePreparation.IssueID = "other-issue" },
		func(in *LegacyV2Input) { in.ReleasePreparation.ScopeRevision++ },
		func(in *LegacyV2Input) { in.ReleasePreparation.AuthorityRevision++ },
		func(in *LegacyV2Input) { in.ReleasePreparation.Expected[0].Revision++ },
		func(in *LegacyV2Input) { in.ReleasePreparation.OperationID = "other-operation" },
	} {
		in := legacyFixture(ReviewPassed)
		in.Legacy.RequiresDeployment = true
		in.ReleasePreparation.IssueID = p.IssueID
		in.ReleasePreparation.ScopeRevision = p.Scope.Revision
		in.ReleasePreparation.AuthorityRevision = p.Authority.Revision
		in.ReleasePreparation.Expected = allConcernRevisions(p)
		mutate(&in)
		requireLegacyAdapterReject(t, p, in)
	}
}
