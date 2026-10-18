package workflowstatev2

import (
	"errors"
	"testing"
)

func command(p CurrentPicture, id string, kind CommandKind, role ActorRole) CommandEnvelope {
	c := CommandEnvelope{
		CommandID: id, IssueID: p.IssueID, Kind: kind, ActorID: string(role) + "-1", ActorRole: role,
		ScopeRevision: p.Scope.Revision, AuthorityRevision: p.Authority.Revision,
	}
	for _, concern := range mutationConcerns(kind) {
		c.Expected = append(c.Expected, ExpectedRevision{Concern: concern, Revision: p.Revisions.Get(concern)})
	}
	if kind == CommandBeginReview {
		c.ReviewerID = "reviewer-1"
	}
	return c
}

func applyOK(t *testing.T, p CurrentPicture, c CommandEnvelope) CurrentPicture {
	t.Helper()
	in := ApplyInput{Picture: p, Command: c}
	if commandUsesTargetGeneration(c.Kind) {
		generation := c.TargetGeneration
		in.AuthoritativeTargetGeneration = &generation
	}
	r, err := Apply(in)
	if err != nil {
		t.Fatalf("apply %s: %v", c.Kind, err)
	}
	return r.Picture
}

func requireGate(t *testing.T, err error, code GateErrorCode) {
	t.Helper()
	var g *GateError
	if !errors.As(err, &g) || g.Code != code {
		t.Fatalf("gate = %v, want %s", err, code)
	}
}

func workingPicture(t *testing.T) CurrentPicture {
	t.Helper()
	p := NewCurrentPicture("issue-1", 1, 1, "owner-acceptance")
	q := command(p, "queue-1", CommandQueue, RoleController)
	q.OperationID, q.RunnableTargetID, q.CapacityEvidenceRef, q.NextAutomaticAction = "op-1", "worker-pool", "capacity-1", "dispatch"
	p = applyOK(t, p, q)
	s := command(p, "start-1", CommandStart, RoleWorker)
	s.OperationID, s.LeaseID, s.CheckpointID = "op-1", "lease-1", "checkpoint-1"
	return applyOK(t, p, s)
}

func reviewedPicture(t *testing.T) CurrentPicture {
	t.Helper()
	p := workingPicture(t)
	p.Scope.DeploymentPolicy, p.Scope.SourcePlanRevision = DeploymentNotRequired, 1
	b := command(p, "review-1", CommandBeginReview, RoleController)
	b.CandidateID, b.EvidenceRefs = "candidate-1", []string{"implementation-1"}
	p = applyOK(t, p, b)
	r := command(p, "review-pass-1", CommandReviewPassed, RoleReviewer)
	r.CandidateID, r.EvidenceRefs = "candidate-1", []string{"review-1"}
	r.CriteriaRefs, r.CoverageRefs = []string{"acceptance-1"}, []string{"test-1"}
	return applyOK(t, p, r)
}

func preparedReleasePicture(t *testing.T, target string, generation Revision) CurrentPicture {
	t.Helper()
	p := reviewedPicture(t)
	p.Scope.DeploymentPolicy = DeploymentRequired
	grant := command(p, "release-authority", CommandGrantAuthority, RoleOwner)
	grant.CandidateID, grant.TargetID = p.Review.CandidateID, target
	p = applyOK(t, p, grant)
	prepare := command(p, "prepare-release", CommandPrepareRelease, RoleController)
	prepare.OperationID, prepare.BranchID = p.Execution.OperationID, "branch-1"
	prepare.CandidateID, prepare.TargetID, prepare.TargetGeneration = p.Review.CandidateID, target, generation
	prepare.ExpectedTargetGeneration = generation - 1
	if generation > 1 {
		prepare.ExpectedTargetReleaseID = "prior-release"
	}
	prepare.AcceptanceVersion = p.Scope.AcceptanceVersion
	prepare.ReleaseOperatorID, prepare.PlannedVerifierID = "release_operator-1", "verifier-1"
	prepare.AuthorityCapability, prepare.AuthorityEvidenceID, prepare.OwnerEvidenceRevision = "deploy_exact_release", "owner-release-authority", 1
	prepare.RecoveryPolicyRevision, prepare.ExposurePolicyDigest = 1, "sha256:exposure"
	prepare.BuildManifestDigest, prepare.ArtifactDigest, prepare.RuntimeContractDigest = "manifest", "artifact", "runtime"
	p = applyOK(t, p, prepare)
	p.Release.ReleaseID = "release-current"
	p.Release.ReleaseKind = "forward"
	p.Release.CandidateRevision = 1
	p.Release.RollbackTargetReleaseID = "release-known-good"
	return p
}

func TestQueueAndWaitTruthGates(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*CommandEnvelope)
		code   GateErrorCode
	}{
		{"missing operation", func(c *CommandEnvelope) { c.OperationID = "" }, GateContradictoryTransition},
		{"missing runnable target", func(c *CommandEnvelope) { c.RunnableTargetID = "" }, GateContradictoryTransition},
		{"missing capacity evidence", func(c *CommandEnvelope) { c.CapacityEvidenceRef = "" }, GateContradictoryTransition},
		{"wrong role", func(c *CommandEnvelope) { c.ActorRole = RoleWorker }, GateInvalidActorRole},
		{"no causal revision", func(c *CommandEnvelope) { c.Expected = nil }, GateInvalidCommand},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewCurrentPicture("issue-1", 1, 1, "owner")
			c := command(p, "queue", CommandQueue, RoleController)
			c.OperationID, c.RunnableTargetID, c.CapacityEvidenceRef, c.NextAutomaticAction = "op", "worker", "capacity", "dispatch"
			tt.mutate(&c)
			_, err := Apply(ApplyInput{Picture: p, Command: c})
			requireGate(t, err, tt.code)
		})
	}

	p := workingPicture(t)
	w := command(p, "wait", CommandWaitAutomatically, RoleWorker)
	w.ExpectedSignal, w.ResumeCondition, w.NextAutomaticAction = "ci.completed", "candidate matches", "begin review"
	p = applyOK(t, p, w)
	if p.Execution.State != ExecutionWaitingAutomatic || p.Execution.SuspendedOperationID != "op-1" {
		t.Fatalf("automatic wait lost continuation: %+v", p.Execution)
	}
	r := command(p, "resume", CommandResume, RoleController)
	r.OperationID = "op-1"
	p = applyOK(t, p, r)
	if p.Execution.State != ExecutionWorking {
		t.Fatalf("resume state = %s", p.Execution.State)
	}
}

func TestCausalityIdempotencyAndIsolation(t *testing.T) {
	p := NewCurrentPicture("issue-1", 1, 1, "owner")
	c := command(p, "pause", CommandPause, RoleController)
	result, err := Apply(ApplyInput{Picture: p, Command: c, PreviouslyAppliedCommandID: c.CommandID, PreviouslyAppliedFingerprint: c.Fingerprint()})
	if err != nil || !result.Duplicate || result.Picture.Revisions != p.Revisions {
		t.Fatalf("duplicate replay mutated picture: %+v %v", result, err)
	}
	p = applyOK(t, p, c)
	_, err = Apply(ApplyInput{Picture: p, Command: c})
	requireGate(t, err, GateStaleRevision)

	other := NewCurrentPicture("issue-2", 1, 1, "owner")
	cancel := command(p, "cancel", CommandCancel, RoleOwner)
	p = applyOK(t, p, cancel)
	if other.Outcome != OutcomeOpen || other.Execution.State != ExecutionReceived {
		t.Fatal("one issue changed another issue")
	}
}

func TestTypedOwnerDecisionMatrix(t *testing.T) {
	tests := []struct {
		name        string
		outcome     DecisionOutcome
		wantSettled ExecutionState
	}{
		{"approve resumes exact operation", DecisionApprove, ExecutionWorking},
		{"reject queues controller replan", DecisionReject, ExecutionQueued},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := workingPicture(t)
			p.Review.CandidateID = "candidate-1"
			req := command(p, "request", CommandRequestDecision, RoleController)
			req.DecisionID, req.DecisionType, req.OperationID, req.BranchID, req.CandidateID = "decision-1", DecisionAuthorization, "op-1", "branch-1", "candidate-1"
			req.AuthorityGap = AuthorityGap{Kind: AuthorityGapOperationAuthorization, RequestedCapability: "run proposed operation", Reason: "owner authorization required"}
			p = applyOK(t, p, req)
			resp := command(p, "respond", CommandRespondDecision, RoleOwner)
			resp.DecisionID, resp.ExpectedDecisionRevision, resp.DecisionOutcome, resp.OperationID, resp.BranchID = "decision-1", 1, tt.outcome, "op-1", "branch-1"
			resp.CandidateID, resp.AuthorityRevision = "candidate-1", p.Authority.Revision
			p = applyOK(t, p, resp)
			if p.Execution.State != ExecutionPaused || p.Attention.Resolution != ResolutionResuming || p.Decisions["decision-1"].Resolution != ResolutionResuming {
				t.Fatalf("response was presented as settled before its durable effect: %+v", p)
			}
			settle := command(p, "settle", CommandSettleDecisionEffect, RoleController)
			settle.DecisionID, settle.ExpectedDecisionRevision, settle.DecisionOutcome = "decision-1", 2, tt.outcome
			settle.DecisionType, settle.SuspendedState = DecisionAuthorization, ExecutionWorking
			settle.OperationID, settle.BranchID, settle.CandidateID, settle.TargetID = "op-1", "branch-1", "candidate-1", ""
			settle.AuthorityRevision = p.Authority.Revision
			for name, mutate := range map[string]func(*CommandEnvelope){
				"decision type":   func(c *CommandEnvelope) { c.DecisionType = DecisionDirection },
				"target":          func(c *CommandEnvelope) { c.TargetID = "other-target" },
				"suspended state": func(c *CommandEnvelope) { c.SuspendedState = ExecutionQueued },
			} {
				t.Run("reject altered "+name, func(t *testing.T) {
					altered := settle
					mutate(&altered)
					if _, err := Apply(ApplyInput{Picture: p, Command: altered}); err == nil {
						t.Fatalf("altered %s was accepted", name)
					}
				})
			}
			p = applyOK(t, p, settle)
			if _, active := p.Decisions["decision-1"]; p.Execution.State != tt.wantSettled || p.Attention.Resolution != ResolutionResolved || active || p.Outcome != OutcomeOpen {
				t.Fatalf("settled picture = execution %s outcome %s attention %s", p.Execution.State, p.Outcome, p.Attention.Resolution)
			}
		})
	}
}

func TestScopeRevisionInvalidatesAuthorityReviewAndDecision(t *testing.T) {
	p := workingPicture(t)
	p.Review.CandidateID = "candidate-1"
	req := command(p, "request", CommandRequestDecision, RoleController)
	req.DecisionID, req.DecisionType, req.OperationID, req.BranchID, req.CandidateID = "decision-1", DecisionAuthorization, "op-1", "branch-1", "candidate-1"
	req.AuthorityGap = AuthorityGap{Kind: AuthorityGapOperationAuthorization, RequestedCapability: "deploy", Reason: "owner authorization required"}
	p = applyOK(t, p, req)
	grant := command(p, "grant", CommandGrantAuthority, RoleOwner)
	grant.TargetID = "production"
	p = applyOK(t, p, grant)
	revise := command(p, "revise", CommandReviseScope, RoleOwner)
	p = applyOK(t, p, revise)
	if p.Scope.Revision != 2 || p.Authority.State != AuthoritySuperseded || p.Review.State != ReviewPending || p.Decisions["decision-1"].State != DecisionSuperseded {
		t.Fatalf("scope invalidation incomplete: %+v", p)
	}
	resp := command(p, "late-response", CommandRespondDecision, RoleOwner)
	resp.DecisionID, resp.ExpectedDecisionRevision, resp.DecisionOutcome, resp.ScopeRevision, resp.OperationID, resp.BranchID = "decision-1", 1, DecisionApprove, 1, "op-1", "branch-1"
	_, err := Apply(ApplyInput{Picture: p, Command: resp})
	requireGate(t, err, GateStaleDecision)
}

func TestNonLiveDoneRequiresIndependentEvidence(t *testing.T) {
	p := reviewedPicture(t)
	c := command(p, "done", CommandCompleteNonLive, RoleController)
	p = applyOK(t, p, c)
	if p.Outcome != OutcomeSatisfied || p.Release.State != ReleaseNotRequired || p.Execution.State != ExecutionCompleted {
		t.Fatalf("not done: %+v", p)
	}

	bad := NewCurrentPicture("bad", 1, 1, "owner")
	c = command(bad, "done-bad", CommandCompleteNonLive, RoleController)
	_, err := Apply(ApplyInput{Picture: bad, Command: c})
	requireGate(t, err, GateMissingEvidence)
}

func TestReleaseScenarioDeployedIsNotLiveAndRollbackReopensOutcome(t *testing.T) {
	p := preparedReleasePicture(t, "production", 7)
	build := command(p, "build", CommandBuildRelease, RoleRelease)
	build.CandidateID, build.TargetID, build.TargetGeneration, build.EvidenceRefs = "candidate-1", "production", 7, []string{"artifact-digest"}
	build.BuildManifestDigest, build.ArtifactDigest, build.RuntimeContractDigest = "manifest", "artifact", "runtime"
	p = applyOK(t, p, build)

	step := func(id string, kind CommandKind, role ActorRole, evidence string) {
		c := command(p, id, kind, role)
		c.CandidateID, c.TargetID, c.TargetGeneration, c.AuthorityRevision = "candidate-1", "production", 7, p.Authority.Revision
		c.EvidenceRefs = []string{evidence}
		if kind == CommandAuthorizeRelease {
			c.AuthorityCapability, c.AuthorityEvidenceID = p.Release.AuthorityCapability, p.Release.AuthorityEvidenceID
		}
		p = applyOK(t, p, c)
	}
	step("authorize", CommandAuthorizeRelease, RoleController, "approval")
	step("deploying", CommandStartDeployment, RoleRelease, "deployment-start")
	step("deployed", CommandRecordDeployment, RoleRelease, "provider-receipt")
	if p.Release.State != ReleaseDeployed || p.Outcome == OutcomeSatisfied {
		t.Fatal("deployed was falsely projected as live/done")
	}
	step("exposed", CommandRecordExposure, RoleRelease, "traffic-receipt")
	step("verifying", CommandBeginVerification, RoleVerifier, "probe-start")
	step("runtime", CommandVerifyRuntime, RoleVerifier, "runtime-proof")
	step("business", CommandVerifyBusiness, RoleVerifier, "business-proof")
	if p.Release.State != ReleaseLive || p.Outcome != OutcomeSatisfied {
		t.Fatalf("live projection incomplete: %+v", p)
	}

	failed := command(p, "failed", CommandProductionFailed, RoleVerifier)
	failed.CandidateID, failed.CandidateRevision, failed.TargetID, failed.TargetGeneration = "candidate-1", 1, "production", 7
	failed.ReleaseID, failed.RecoveryReleaseID, failed.RecoveryOfReleaseID = "release-current", "release-recovery", "release-current"
	failed.RecoveryMode, failed.RecoveryTargetGeneration, failed.EvidenceRefs = RecoveryRollback, 8, []string{"failed-probe"}
	failed.RecoveryCandidateID, failed.RecoveryCandidateRevision = "candidate-1", 1
	failed.RecoverySourceCandidateID, failed.RecoverySourceRevision = "known-good-candidate", 1
	failed.RecoveryBuildManifestDigest = "build-known-good"
	failed.RecoveryArtifactDigest, failed.RecoveryRuntimeDigest = "artifact-known-good", "runtime-known-good"
	failed.RecoveryReviewedReviewID, failed.RecoveryReviewedReviewRevision = "known-good-review", 1
	p = applyOK(t, p, failed)
	if p.Release.State != ReleaseRollingBack || p.Release.ReleaseID != "release-recovery" || p.Release.RecoveryOfReleaseID != "release-current" || p.Release.Generation != 8 || p.Outcome != OutcomeOpen || p.Evidence.State != EvidenceContradicted {
		t.Fatal("regression did not reopen outcome and establish a new recovery generation")
	}
	rolledBack := command(p, "rolled-back", CommandRollbackSucceeded, RoleRelease)
	rolledBack.CandidateID, rolledBack.TargetID, rolledBack.TargetGeneration = "candidate-1", "production", 8
	rolledBack.AuthorityRevision, rolledBack.EvidenceRefs = p.Authority.Revision, []string{"rollback-receipt"}
	p = applyOK(t, p, rolledBack)
	if p.Outcome != OutcomeOpen || p.Release.State != ReleaseRolledBack {
		t.Fatal("rollback falsely satisfied requested change")
	}
	restored := command(p, "restored", CommandRestoreVerified, RoleVerifier)
	restored.CandidateID, restored.TargetID, restored.TargetGeneration = "candidate-1", "production", 8
	restored.EvidenceRefs = []string{"recovery-runtime", "recovery-business", "recovery-regression", "restoration-proof"}
	p = applyOK(t, p, restored)
	if p.Release.State != ReleaseRestored || p.Execution.State != ExecutionCorrection || p.Outcome != OutcomeOpen {
		t.Fatalf("restore projection = %+v", p)
	}
}

func TestProductionRegressionRecoveryGenerationAndContainment(t *testing.T) {
	live := preparedReleasePicture(t, "production", 4)
	live.Release.State = ReleaseLive
	live.Outcome = OutcomeSatisfied
	live.Execution.State = ExecutionCompleted

	regression := command(live, "regression", CommandProductionFailed, RoleVerifier)
	regression.ReleaseID, regression.CandidateID, regression.CandidateRevision = live.Release.ReleaseID, live.Release.CandidateID, live.Release.CandidateRevision
	regression.TargetID, regression.TargetGeneration, regression.EvidenceRefs = live.Release.TargetID, live.Release.Generation, []string{"current-regression"}
	regression.RecoveryMode, regression.RecoveryReleaseID, regression.RecoveryOfReleaseID = RecoveryRollback, "release-recovery", live.Release.ReleaseID
	regression.RecoveryTargetGeneration = live.Release.Generation + 1
	regression.RecoveryCandidateID, regression.RecoveryCandidateRevision = live.Release.CandidateID, live.Release.CandidateRevision
	regression.RecoverySourceCandidateID, regression.RecoverySourceRevision = "known-good-candidate", 1
	regression.RecoveryBuildManifestDigest = "build-known-good"
	regression.RecoveryArtifactDigest, regression.RecoveryRuntimeDigest = "artifact-known-good", "runtime-known-good"
	regression.RecoveryReviewedReviewID, regression.RecoveryReviewedReviewRevision = "known-good-review", 1
	recovering := applyOK(t, live, regression)
	if recovering.Release.ReleaseID == live.Release.ReleaseID || recovering.Release.Generation != live.Release.Generation+1 || recovering.Outcome != OutcomeOpen {
		t.Fatalf("recovery did not use a new generation and reopen outcome: %+v", recovering)
	}

	bad := regression
	bad.CommandID = "bad-generation"
	bad.RecoveryTargetGeneration = live.Release.Generation
	_, err := Apply(ApplyInput{Picture: live, Command: bad, AuthoritativeTargetGeneration: &bad.TargetGeneration})
	requireGate(t, err, GateInvalidCommand)

	contain := regression
	contain.CommandID, contain.RecoveryMode = "contain", RecoveryContainment
	contain.RecoveryReleaseID, contain.RecoveryOfReleaseID, contain.RecoveryTargetGeneration = "", "", 0
	contained := applyOK(t, live, contain)
	if contained.Release.ReleaseID != live.Release.ReleaseID || contained.Release.Generation != live.Release.Generation || contained.Release.State != ReleaseDegraded || contained.Outcome != OutcomeOpen || contained.Execution.State != ExecutionTechnicallyStopped {
		t.Fatalf("containment mutated target generation or retained satisfied outcome: %+v", contained)
	}
}

func TestTargetGenerationAndAuthorityGates(t *testing.T) {
	p := preparedReleasePicture(t, "production", 3)
	b := command(p, "build", CommandBuildRelease, RoleRelease)
	b.CandidateID, b.TargetID, b.TargetGeneration, b.EvidenceRefs = "candidate-1", "production", 3, []string{"digest"}
	b.BuildManifestDigest, b.ArtifactDigest, b.RuntimeContractDigest = "manifest", "artifact", "runtime"
	p = applyOK(t, p, b)
	c := command(p, "authorize", CommandAuthorizeRelease, RoleController)
	c.CandidateID, c.TargetID, c.TargetGeneration, c.EvidenceRefs = "candidate-1", "production", 2, []string{"owner-authority"}
	c.AuthorityCapability, c.AuthorityEvidenceID = p.Release.AuthorityCapability, p.Release.AuthorityEvidenceID
	authoritative := Revision(3)
	_, err := Apply(ApplyInput{Picture: p, Command: c, AuthoritativeTargetGeneration: &authoritative})
	requireGate(t, err, GateTargetGenerationConflict)
	c.TargetGeneration = 3
	p.Authority.State = AuthorityMissing
	_, err = Apply(ApplyInput{Picture: p, Command: c, AuthoritativeTargetGeneration: &authoritative})
	requireGate(t, err, GateMissingAuthority)
}

func TestPrepareReleaseRequiresCompleteIndependentAuthoritySnapshot(t *testing.T) {
	base := reviewedPicture(t)
	base.Scope.DeploymentPolicy = DeploymentRequired
	grant := command(base, "grant-release", CommandGrantAuthority, RoleOwner)
	grant.CandidateID, grant.TargetID = base.Review.CandidateID, "production"
	base = applyOK(t, base, grant)
	valid := command(base, "prepare", CommandPrepareRelease, RoleController)
	valid.OperationID, valid.BranchID = base.Execution.OperationID, "main"
	valid.CandidateID, valid.TargetID, valid.TargetGeneration = base.Review.CandidateID, "production", 1
	valid.AcceptanceVersion = base.Scope.AcceptanceVersion
	valid.ReleaseOperatorID, valid.PlannedVerifierID = "operator", "verifier"
	valid.BuildManifestDigest, valid.ArtifactDigest, valid.RuntimeContractDigest = "manifest", "artifact", "runtime"
	valid.AuthorityCapability, valid.AuthorityEvidenceID, valid.OwnerEvidenceRevision = "deploy_exact_release", "owner-proof", 1
	valid.RecoveryPolicyRevision, valid.ExposurePolicyDigest = 1, "exposure"
	for name, mutate := range map[string]func(*CommandEnvelope){
		"deployment policy is not authority": func(c *CommandEnvelope) { c.AuthorityCapability = "" },
		"operator cannot verify itself":      func(c *CommandEnvelope) { c.PlannedVerifierID = c.ReleaseOperatorID },
		"generation must advance one":        func(c *CommandEnvelope) { c.TargetGeneration = 2 },
		"missing reproducible manifest":      func(c *CommandEnvelope) { c.BuildManifestDigest = "" },
		"missing recovery policy revision":   func(c *CommandEnvelope) { c.RecoveryPolicyRevision = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			mutate(&candidate)
			_, err := Apply(ApplyInput{Picture: base, Command: candidate, AuthoritativeTargetGeneration: &candidate.TargetGeneration})
			if err == nil {
				t.Fatal("incomplete release authority snapshot was accepted")
			}
		})
	}
	prepared := applyOK(t, base, valid)
	if prepared.Release.State != ReleasePending || prepared.Release.ReleaseOperatorID != "operator" || prepared.Release.PlannedVerifierID != "verifier" || prepared.Release.Generation != 1 {
		t.Fatalf("prepared release snapshot = %+v", prepared.Release)
	}
}

func TestEvidenceBindingRejectsOldCandidateProof(t *testing.T) {
	p := NewCurrentPicture("issue-1", 2, 4, "owner")
	p.Release = ReleaseFacet{State: ReleaseProductionVerifying, CandidateID: "candidate-2", TargetID: "production", Generation: 9}
	p.Evidence.State = EvidenceCurrent
	p.Evidence.Runtime = []BoundEvidence{{Ref: "old-runtime", CandidateID: "candidate-1", TargetID: "production", TargetGeneration: 8, AcceptanceVersion: 3, ScopeRevision: 1, Freshness: EvidenceCurrent}}
	c := command(p, "business", CommandVerifyBusiness, RoleVerifier)
	c.CandidateID, c.TargetID, c.TargetGeneration, c.EvidenceRefs = "candidate-2", "production", 9, []string{"business-current"}
	authoritative := Revision(9)
	_, err := Apply(ApplyInput{Picture: p, Command: c, AuthoritativeTargetGeneration: &authoritative})
	requireGate(t, err, GateMissingEvidence)
}

func TestAuthoritativeGenerationIsIndependentInput(t *testing.T) {
	p := reviewedPicture(t)
	c := command(p, "build", CommandBuildRelease, RoleRelease)
	c.CandidateID, c.TargetID, c.TargetGeneration, c.EvidenceRefs = "candidate-1", "production", 5, []string{"digest"}
	actual := Revision(6)
	_, err := Apply(ApplyInput{Picture: p, Command: c, AuthoritativeTargetGeneration: &actual})
	requireGate(t, err, GateTargetGenerationConflict)
}

func TestEveryMutableConcernRevisionAndKnownConcernRequired(t *testing.T) {
	p := reviewedPicture(t)
	c := command(p, "done", CommandCompleteNonLive, RoleController)
	c.Expected = c.Expected[:len(c.Expected)-1]
	_, err := Apply(ApplyInput{Picture: p, Command: c})
	requireGate(t, err, GateStaleRevision)

	c = command(p, "done-unknown", CommandCompleteNonLive, RoleController)
	c.Expected = append(c.Expected, ExpectedRevision{Concern: Concern("mystery"), Revision: 0})
	_, err = Apply(ApplyInput{Picture: p, Command: c})
	requireGate(t, err, GateInvalidCommand)
}

func TestRequestDecisionRequiresCurrentEvidenceRevision(t *testing.T) {
	p := workingPicture(t)

	makeRequest := func(id string) CommandEnvelope {
		c := command(p, id, CommandRequestDecision, RoleController)
		c.DecisionID = "decision-" + id
		c.DecisionType = DecisionAuthorization
		c.OperationID = p.Execution.OperationID
		c.BranchID = "branch-1"
		p.Review.CandidateID = "candidate-1"
		c.CandidateID = "candidate-1"
		c.AuthorityGap = AuthorityGap{Kind: AuthorityGapOperationAuthorization, RequestedCapability: "deploy", Reason: "owner authorization required"}
		return c
	}

	t.Run("omitted evidence revision", func(t *testing.T) {
		c := makeRequest("omitted")
		expected := make([]ExpectedRevision, 0, len(c.Expected)-1)
		for _, item := range c.Expected {
			if item.Concern != ConcernEvidence {
				expected = append(expected, item)
			}
		}
		c.Expected = expected

		_, err := Apply(ApplyInput{Picture: p, Command: c})
		requireGateConcern(t, err, GateStaleRevision, ConcernEvidence)
	})

	t.Run("stale evidence revision", func(t *testing.T) {
		c := makeRequest("stale")
		for i := range c.Expected {
			if c.Expected[i].Concern == ConcernEvidence {
				c.Expected[i].Revision--
			}
		}

		_, err := Apply(ApplyInput{Picture: p, Command: c})
		requireGateConcern(t, err, GateStaleRevision, ConcernEvidence)
	})
}

func requireGateConcern(t *testing.T, err error, code GateErrorCode, concern Concern) {
	t.Helper()
	var g *GateError
	if !errors.As(err, &g) || g.Code != code || g.Concern != concern {
		t.Fatalf("gate = %v, want %s for %s", err, code, concern)
	}
}

func TestEveryCommandDeclaresOnlyKnownMutationConcerns(t *testing.T) {
	kinds := []CommandKind{CommandQueue, CommandStart, CommandWaitAutomatically, CommandBlockExternally, CommandTechnicalStop, CommandSubmitCorrection, CommandBeginReview, CommandReviewPassed, CommandReviewFailed, CommandReviewInconclusive, CommandReviewBlockedEvidence, CommandRequestDecision, CommandReadDecision, CommandAcknowledgeDecision, CommandRespondDecision, CommandSettleDecisionEffect, CommandCompleteNonLive, CommandPrepareRelease, CommandBuildRelease, CommandAuthorizeRelease, CommandStartDeployment, CommandRecordDeployment, CommandRecordExposure, CommandBeginVerification, CommandVerifyRuntime, CommandVerifyBusiness, CommandProductionFailed, CommandBeginRollback, CommandRollbackSucceeded, CommandRestoreVerified, CommandPause, CommandResume, CommandCancel, CommandSupersede, CommandReviseScope, CommandGrantAuthority, CommandRevokeAuthority, CommandRecordEvidence, CommandContradictEvidence}
	for _, kind := range kinds {
		concerns := mutationConcerns(kind)
		if len(concerns) == 0 {
			t.Fatalf("%s has no mutation concern declaration", kind)
		}
		seen := map[Concern]bool{}
		for _, concern := range concerns {
			if !knownConcern(concern) || seen[concern] {
				t.Fatalf("%s has invalid mutation concern %q", kind, concern)
			}
			seen[concern] = true
		}
	}
}

func TestIndependentReviewAndStructuredFailureEvidence(t *testing.T) {
	p := workingPicture(t)
	b := command(p, "begin", CommandBeginReview, RoleController)
	b.CandidateID, b.EvidenceRefs = "candidate-1", []string{"implementation"}
	p = applyOK(t, p, b)
	fail := command(p, "fail", CommandReviewFailed, RoleReviewer)
	fail.ActorID, fail.CandidateID, fail.EvidenceRefs = p.Review.ImplementerID, "candidate-1", []string{"review-log"}
	fail.CriteriaRefs, fail.CoverageRefs, fail.GapRefs, fail.RequiredCorrection = []string{"criterion-1"}, []string{"test-1"}, []string{"gap-1"}, "repair the failing boundary"
	_, err := Apply(ApplyInput{Picture: p, Command: fail})
	requireGate(t, err, GateInvalidActorRole)

	fail.ActorID = "reviewer-1"
	p = applyOK(t, p, fail)
	if len(p.Evidence.TechnicalReviews) != 1 || p.Evidence.TechnicalReviews[0].Result != ReviewFailed || p.Evidence.TechnicalReviews[0].RequiredCorrection == "" || len(p.Evidence.TechnicalReviews[0].GapRefs) != 1 {
		t.Fatalf("structured failed-review evidence was not retained: %+v", p.Evidence.TechnicalReviews)
	}
}

func TestSubmitCorrectionAtomicallyEstablishesIndependentReview(t *testing.T) {
	p := workingPicture(t)
	begin := command(p, "begin", CommandBeginReview, RoleController)
	begin.CandidateID, begin.ReviewerID, begin.EvidenceRefs = "candidate-1", "reviewer-1", []string{"implementation-1"}
	p = applyOK(t, p, begin)
	fail := command(p, "fail", CommandReviewFailed, RoleReviewer)
	fail.ActorID, fail.CandidateID = "reviewer-1", "candidate-1"
	fail.EvidenceRefs, fail.CriteriaRefs, fail.CoverageRefs = []string{"review-1"}, []string{"criterion"}, []string{"coverage"}
	fail.GapRefs, fail.RequiredCorrection, fail.CorrectionCount, fail.MaxCorrections = []string{"gap"}, "repair it", 1, 2
	p = applyOK(t, p, fail)

	submit := command(p, "submit", CommandSubmitCorrection, RoleWorker)
	submit.ActorID, submit.OperationID, submit.CandidateID, submit.ReviewerID = p.Review.ImplementerID, p.Execution.OperationID, "candidate-2", "reviewer-1"
	submit.EvidenceRefs = []string{"implementation-2"}
	p = applyOK(t, p, submit)
	if p.Execution.State != ExecutionInReview || p.Review.State != ReviewInProgress || p.Review.CandidateID != "candidate-2" || p.Review.ReviewerID != "reviewer-1" || p.Review.CorrectionCount != 1 || p.Review.MaxCorrections != 2 {
		t.Fatalf("correction did not establish coherent review state: execution=%+v review=%+v", p.Execution, p.Review)
	}
	if got := p.Evidence.Other[len(p.Evidence.Other)-1]; got.Ref != "implementation-2" || got.CandidateID != "candidate-2" || got.Freshness != EvidenceCurrent {
		t.Fatalf("replacement implementation evidence not bound: %+v", got)
	}
}

func TestSubmitCorrectionRejectsSelfReviewAndExhaustedBudget(t *testing.T) {
	p := workingPicture(t)
	p.Execution.State = ExecutionCorrection
	p.Review = ReviewFacet{State: ReviewFailed, CandidateID: "candidate-1", ImplementerID: p.Execution.ActorID, CorrectionCount: 1, MaxCorrections: 1}
	submit := command(p, "submit", CommandSubmitCorrection, RoleWorker)
	submit.OperationID, submit.CandidateID, submit.ActorID, submit.ReviewerID, submit.EvidenceRefs = p.Execution.OperationID, "candidate-2", p.Execution.ActorID, p.Execution.ActorID, []string{"implementation-2"}
	_, err := Apply(ApplyInput{Picture: p, Command: submit})
	requireGate(t, err, GateInvalidActorRole)

	submit.ReviewerID = "reviewer-1"
	p.Review.CorrectionCount = 2
	_, err = Apply(ApplyInput{Picture: p, Command: submit})
	requireGate(t, err, GateContradictoryTransition)
}

func TestNonPassingReviewEvidenceMatrix(t *testing.T) {
	for _, tc := range []struct {
		kind  CommandKind
		state ReviewState
	}{{CommandReviewFailed, ReviewFailed}, {CommandReviewInconclusive, ReviewInconclusive}, {CommandReviewBlockedEvidence, ReviewBlockedByEvidence}} {
		t.Run(string(tc.state), func(t *testing.T) {
			p := workingPicture(t)
			b := command(p, "begin", CommandBeginReview, RoleController)
			b.CandidateID, b.EvidenceRefs = "candidate-1", []string{"implementation"}
			p = applyOK(t, p, b)
			r := command(p, "result", tc.kind, RoleReviewer)
			r.CandidateID, r.EvidenceRefs = "candidate-1", []string{"review-log"}
			r.CriteriaRefs, r.CoverageRefs, r.GapRefs, r.RequiredCorrection = []string{"criterion"}, []string{"coverage"}, []string{"gap"}, "required correction"
			p = applyOK(t, p, r)
			got := p.Evidence.TechnicalReviews
			if len(got) != 1 || got[0].Result != tc.state || len(got[0].CriteriaRefs) == 0 || len(got[0].CoverageRefs) == 0 || len(got[0].GapRefs) == 0 || got[0].RequiredCorrection == "" {
				t.Fatalf("structured %s evidence missing: %+v", tc.state, got)
			}
		})
	}
}

func TestNewCandidateBuildReplacesRuntimeEvidence(t *testing.T) {
	p := preparedReleasePicture(t, "production", 2)
	p.Evidence.Runtime = []BoundEvidence{{Ref: "old", CandidateID: "old-candidate", TargetID: "production", TargetGeneration: 1, AcceptanceVersion: p.Scope.AcceptanceVersion, ScopeRevision: p.Scope.Revision, Freshness: EvidenceCurrent}}
	b := command(p, "build-new", CommandBuildRelease, RoleRelease)
	b.CandidateID, b.TargetID, b.TargetGeneration, b.EvidenceRefs = "candidate-1", "production", 2, []string{"new-digest"}
	b.BuildManifestDigest, b.ArtifactDigest, b.RuntimeContractDigest = "manifest", "artifact", "runtime"
	p = applyOK(t, p, b)
	if len(p.Evidence.Runtime) != 0 || len(p.Evidence.Business) != 0 {
		t.Fatal("new candidate retained old runtime/business proof")
	}
}

func TestOneIssueWideDecisionAndAuthorityInvalidation(t *testing.T) {
	p := workingPicture(t)
	p.Review.CandidateID = "candidate-1"
	req := command(p, "request-1", CommandRequestDecision, RoleController)
	req.DecisionID, req.DecisionType, req.OperationID, req.BranchID, req.CandidateID = "decision-1", DecisionAuthorization, "op-1", "branch-1", "candidate-1"
	req.AuthorityGap = AuthorityGap{Kind: AuthorityGapOperationAuthorization, RequestedCapability: "deploy", Reason: "owner authorization required"}
	p = applyOK(t, p, req)

	second := command(p, "request-2", CommandRequestDecision, RoleController)
	second.DecisionID, second.DecisionType, second.OperationID, second.BranchID, second.CandidateID = "decision-2", DecisionAuthorization, "op-1", "branch-2", "candidate-1"
	second.AuthorityGap = req.AuthorityGap
	_, err := Apply(ApplyInput{Picture: p, Command: second})
	requireGate(t, err, GateInvalidDecision)

	grant := command(p, "grant", CommandGrantAuthority, RoleOwner)
	p = applyOK(t, p, grant)
	if p.Decisions["decision-1"].State != DecisionSuperseded {
		t.Fatal("authority revision did not invalidate open decision")
	}
}

func TestReadAndAcknowledgeNeverAnswerDecision(t *testing.T) {
	p := workingPicture(t)
	p.Review.CandidateID = "candidate-1"
	req := command(p, "request", CommandRequestDecision, RoleController)
	req.DecisionID, req.DecisionType, req.OperationID, req.BranchID, req.CandidateID = "decision-1", DecisionAuthorization, "op-1", "branch-1", "candidate-1"
	req.AuthorityGap = AuthorityGap{Kind: AuthorityGapOperationAuthorization, RequestedCapability: "deploy", Reason: "owner authorization required"}
	p = applyOK(t, p, req)

	read := command(p, "read", CommandReadDecision, RoleOwner)
	read.DecisionID, read.ExpectedDecisionRevision = "decision-1", 1
	p = applyOK(t, p, read)
	ack := command(p, "ack", CommandAcknowledgeDecision, RoleOwner)
	ack.DecisionID, ack.ExpectedDecisionRevision = "decision-1", 1
	p = applyOK(t, p, ack)
	if d := p.Decisions["decision-1"]; d.State != DecisionUnanswered || d.Response != "" || p.Attention.Read != ReadRead || !p.Attention.Acknowledged {
		t.Fatalf("read/ack changed decision authority: decision=%+v attention=%+v", d, p.Attention)
	}
}

func TestExactReplayAndSatisfiedTerminalGate(t *testing.T) {
	p := NewCurrentPicture("issue-1", 1, 1, "owner")
	c := command(p, "pause", CommandPause, RoleController)
	changed := c
	changed.Detail = "different payload"
	_, err := Apply(ApplyInput{Picture: p, Command: changed, PreviouslyAppliedCommandID: c.CommandID, PreviouslyAppliedFingerprint: c.Fingerprint()})
	requireGate(t, err, GateIdempotencyConflict)

	done := reviewedPicture(t)
	done = applyOK(t, done, command(done, "done", CommandCompleteNonLive, RoleController))
	unrelated := command(done, "pause-after-done", CommandPause, RoleController)
	_, err = Apply(ApplyInput{Picture: done, Command: unrelated})
	requireGate(t, err, GateOutcomeTerminal)
}
