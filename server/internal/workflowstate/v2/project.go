package workflowstatev2

import "slices"

func Apply(in ApplyInput) (ApplyResult, error) {
	if in.PreviouslyAppliedCommandID != "" || in.PreviouslyAppliedFingerprint != "" {
		if in.PreviouslyAppliedCommandID != in.Command.CommandID || in.PreviouslyAppliedFingerprint != in.Command.Fingerprint() {
			return ApplyResult{}, gate(GateIdempotencyConflict, "stored command identity or fingerprint differs from replay")
		}
		return ApplyResult{Picture: in.Picture, Duplicate: true}, nil
	}
	p := clonePicture(in.Picture)
	c := in.Command
	if c.CommandID == "" || c.IssueID == "" || c.Kind == "" || c.ActorID == "" || c.ActorRole == "" {
		return ApplyResult{}, gate(GateInvalidCommand, "command id, issue id, kind, actor id, and actor role are required")
	}
	if len(c.Expected) == 0 {
		return ApplyResult{}, gate(GateInvalidCommand, "at least one causal concern revision is required")
	}
	if p.IssueID == "" || c.IssueID != p.IssueID {
		return ApplyResult{}, gate(GateInvalidCommand, "command issue does not match the current picture")
	}
	if err := validateExpectedRevisions(p, c.Expected); err != nil {
		return ApplyResult{}, err
	}
	if err := requireMutationRevisions(p, c); err != nil {
		return ApplyResult{}, err
	}
	if commandUsesTargetGeneration(c.Kind) {
		if in.AuthoritativeTargetGeneration == nil || c.TargetGeneration != *in.AuthoritativeTargetGeneration {
			actual := Revision(0)
			if in.AuthoritativeTargetGeneration != nil {
				actual = *in.AuthoritativeTargetGeneration
			}
			return ApplyResult{}, &GateError{Code: GateTargetGenerationConflict, Concern: ConcernRelease, Expected: c.TargetGeneration, Actual: actual, Reason: "command generation differs from authoritative target generation"}
		}
	}
	if p.Outcome == OutcomeCancelled || p.Outcome == OutcomeSuperseded {
		return ApplyResult{}, gate(GateOutcomeTerminal, "cancelled or superseded outcomes refuse future work")
	}
	if p.Outcome == OutcomeSatisfied && !allowedAfterSatisfied(c.Kind) {
		return ApplyResult{}, gate(GateOutcomeTerminal, "satisfied outcome accepts only regression, cancellation, or recovery commands")
	}

	var err error
	switch c.Kind {
	case CommandQueue:
		err = queue(&p, c)
	case CommandStart:
		err = start(&p, c)
	case CommandWaitAutomatically:
		err = waitAutomatically(&p, c)
	case CommandBlockExternally:
		err = setExecution(&p, c, RoleWorker, ExecutionBlockedExternal)
	case CommandTechnicalStop:
		err = setExecution(&p, c, RoleController, ExecutionTechnicallyStopped)
	case CommandSubmitCorrection:
		err = submitCorrection(&p, c)
	case CommandBeginReview:
		err = beginReview(&p, c)
	case CommandReviewPassed, CommandReviewFailed, CommandReviewInconclusive, CommandReviewBlockedEvidence:
		err = recordReview(&p, c)
	case CommandRequestDecision:
		err = requestDecision(&p, c)
	case CommandRespondDecision:
		err = respondDecision(&p, c)
	case CommandReadDecision:
		err = readDecision(&p, c, false)
	case CommandAcknowledgeDecision:
		err = readDecision(&p, c, true)
	case CommandSettleDecisionEffect:
		err = settleDecisionEffect(&p, c)
	case CommandCompleteNonLive:
		err = completeNonLive(&p, c)
	case CommandPrepareRelease:
		err = prepareRelease(&p, c)
	case CommandBuildRelease:
		err = buildRelease(&p, c)
	case CommandAuthorizeRelease:
		err = authorizeRelease(&p, c)
	case CommandStartDeployment:
		err = releaseStep(&p, c, RoleRelease, ReleaseAuthorized, ReleaseDeploying, true)
	case CommandRecordDeployment:
		err = releaseStep(&p, c, RoleRelease, ReleaseDeploying, ReleaseDeployed, true)
	case CommandRecordExposure:
		err = releaseStep(&p, c, RoleRelease, ReleaseDeployed, ReleaseExposed, true)
	case CommandBeginVerification:
		err = releaseStep(&p, c, RoleVerifier, ReleaseExposed, ReleaseProductionVerifying, false)
	case CommandVerifyRuntime:
		err = verifyRuntime(&p, c)
	case CommandVerifyBusiness:
		err = verifyBusiness(&p, c)
	case CommandProductionFailed:
		err = productionFailed(&p, c)
	case CommandBeginRollback:
		err = releaseStep(&p, c, RoleRelease, ReleaseDegraded, ReleaseRollingBack, true)
	case CommandRollbackSucceeded:
		err = rollbackSucceeded(&p, c)
	case CommandRestoreVerified:
		err = restoreVerified(&p, c)
	case CommandPause:
		err = pause(&p, c)
	case CommandResume:
		err = resume(&p, c)
	case CommandCancel:
		err = cancel(&p, c)
	case CommandSupersede:
		err = supersede(&p, c)
	case CommandReviseScope:
		err = reviseScope(&p, c)
	case CommandGrantAuthority:
		err = grantAuthority(&p, c)
	case CommandRevokeAuthority:
		err = revokeAuthority(&p, c)
	case CommandRecordEvidence:
		err = recordEvidence(&p, c)
	case CommandContradictEvidence:
		err = contradictEvidence(&p, c)
	default:
		err = gate(GateInvalidCommand, "unknown command kind")
	}
	if err != nil {
		return ApplyResult{}, err
	}
	p.LastCommandID = c.CommandID
	return ApplyResult{Picture: p}, nil
}

func clonePicture(p CurrentPicture) CurrentPicture {
	decisions := p.Decisions
	p.Decisions = make(map[string]OwnerDecision, len(p.Decisions))
	for k, v := range decisions {
		p.Decisions[k] = v
	}
	p.Review.CriteriaRefs = slices.Clone(p.Review.CriteriaRefs)
	p.Review.CoverageRefs = slices.Clone(p.Review.CoverageRefs)
	p.Review.GapRefs = slices.Clone(p.Review.GapRefs)
	p.Evidence.TechnicalReviews = slices.Clone(p.Evidence.TechnicalReviews)
	for i := range p.Evidence.TechnicalReviews {
		p.Evidence.TechnicalReviews[i].CriteriaRefs = slices.Clone(p.Evidence.TechnicalReviews[i].CriteriaRefs)
		p.Evidence.TechnicalReviews[i].CoverageRefs = slices.Clone(p.Evidence.TechnicalReviews[i].CoverageRefs)
		p.Evidence.TechnicalReviews[i].GapRefs = slices.Clone(p.Evidence.TechnicalReviews[i].GapRefs)
	}
	p.Evidence.Runtime = slices.Clone(p.Evidence.Runtime)
	p.Evidence.Business = slices.Clone(p.Evidence.Business)
	p.Evidence.Restoration = slices.Clone(p.Evidence.Restoration)
	p.Evidence.Other = slices.Clone(p.Evidence.Other)
	return p
}

func gate(code GateErrorCode, reason string) error { return &GateError{Code: code, Reason: reason} }

func validateExpectedRevisions(p CurrentPicture, expected []ExpectedRevision) error {
	seen := map[Concern]bool{}
	for _, item := range expected {
		if !knownConcern(item.Concern) || seen[item.Concern] {
			return gate(GateInvalidCommand, "expected concern revisions must be unique and named")
		}
		seen[item.Concern] = true
		actual := p.Revisions.Get(item.Concern)
		if actual != item.Revision {
			return &GateError{Code: GateStaleRevision, Concern: item.Concern, Expected: item.Revision, Actual: actual, Reason: "causal revision no longer matches"}
		}
	}
	return nil
}

func knownConcern(c Concern) bool {
	return slices.Contains([]Concern{ConcernOutcome, ConcernScope, ConcernAuthority, ConcernExecution, ConcernReview, ConcernRelease, ConcernAttention, ConcernEvidence, ConcernBudget}, c)
}

func mutationConcerns(kind CommandKind) []Concern {
	switch kind {
	case CommandQueue, CommandStart, CommandWaitAutomatically, CommandBlockExternally, CommandTechnicalStop, CommandPause, CommandResume:
		return []Concern{ConcernExecution}
	case CommandSubmitCorrection:
		return []Concern{ConcernExecution, ConcernReview, ConcernRelease, ConcernEvidence}
	case CommandBeginReview:
		return []Concern{ConcernExecution, ConcernReview, ConcernEvidence}
	case CommandReviewPassed, CommandReviewFailed, CommandReviewInconclusive, CommandReviewBlockedEvidence:
		return []Concern{ConcernReview, ConcernExecution, ConcernEvidence}
	case CommandRequestDecision:
		return []Concern{ConcernExecution, ConcernAttention, ConcernEvidence}
	case CommandRespondDecision:
		return []Concern{ConcernAttention, ConcernExecution}
	case CommandReadDecision, CommandAcknowledgeDecision:
		return []Concern{ConcernAttention}
	case CommandSettleDecisionEffect:
		return []Concern{ConcernAttention, ConcernExecution}
	case CommandCompleteNonLive:
		return []Concern{ConcernOutcome, ConcernExecution, ConcernRelease}
	case CommandPrepareRelease:
		return []Concern{ConcernRelease}
	case CommandBuildRelease:
		return []Concern{ConcernRelease, ConcernEvidence}
	case CommandAuthorizeRelease, CommandRecordDeployment, CommandRecordExposure, CommandBeginVerification, CommandBeginRollback:
		return []Concern{ConcernRelease, ConcernEvidence}
	case CommandStartDeployment:
		return []Concern{ConcernExecution, ConcernRelease, ConcernEvidence}
	case CommandVerifyRuntime:
		return []Concern{ConcernEvidence}
	case CommandVerifyBusiness:
		return []Concern{ConcernEvidence, ConcernRelease, ConcernOutcome, ConcernExecution}
	case CommandProductionFailed:
		return []Concern{ConcernRelease, ConcernEvidence, ConcernExecution, ConcernOutcome}
	case CommandRollbackSucceeded:
		return []Concern{ConcernRelease, ConcernEvidence, ConcernOutcome, ConcernExecution}
	case CommandRestoreVerified:
		return []Concern{ConcernRelease, ConcernExecution, ConcernEvidence, ConcernOutcome}
	case CommandCancel:
		return []Concern{ConcernOutcome, ConcernExecution}
	case CommandSupersede:
		return []Concern{ConcernOutcome, ConcernExecution, ConcernAuthority, ConcernAttention}
	case CommandReviseScope:
		return []Concern{ConcernScope, ConcernAuthority, ConcernReview, ConcernExecution, ConcernAttention, ConcernEvidence}
	case CommandGrantAuthority:
		return []Concern{ConcernAuthority, ConcernAttention}
	case CommandRevokeAuthority:
		return []Concern{ConcernAuthority, ConcernExecution, ConcernAttention}
	case CommandRecordEvidence:
		return []Concern{ConcernEvidence}
	case CommandContradictEvidence:
		return []Concern{ConcernEvidence, ConcernRelease}
	default:
		return nil
	}
}

func requireMutationRevisions(p CurrentPicture, c CommandEnvelope) error {
	provided := make(map[Concern]bool, len(c.Expected))
	for _, item := range c.Expected {
		provided[item.Concern] = true
	}
	for _, concern := range mutationConcerns(c.Kind) {
		if !provided[concern] {
			return &GateError{Code: GateStaleRevision, Concern: concern, Expected: p.Revisions.Get(concern), Actual: 0, Reason: "command omitted a concern revision it can mutate"}
		}
	}
	return nil
}

func commandUsesTargetGeneration(kind CommandKind) bool {
	return slices.Contains([]CommandKind{CommandPrepareRelease, CommandBuildRelease, CommandAuthorizeRelease, CommandStartDeployment, CommandRecordDeployment, CommandRecordExposure, CommandBeginVerification, CommandVerifyRuntime, CommandVerifyBusiness, CommandProductionFailed, CommandBeginRollback, CommandRollbackSucceeded, CommandRestoreVerified, CommandContradictEvidence}, kind)
}

func allowedAfterSatisfied(kind CommandKind) bool {
	return slices.Contains([]CommandKind{CommandProductionFailed, CommandContradictEvidence, CommandBeginRollback, CommandRollbackSucceeded, CommandRestoreVerified, CommandCancel, CommandSupersede}, kind)
}

func requireRole(c CommandEnvelope, roles ...ActorRole) error {
	if slices.Contains(roles, c.ActorRole) {
		return nil
	}
	return gate(GateInvalidActorRole, "actor role is not authorized for this transition")
}

func requireScope(p *CurrentPicture, c CommandEnvelope) error {
	if c.ScopeRevision != p.Scope.Revision {
		return &GateError{Code: GateStaleScope, Concern: ConcernScope, Expected: c.ScopeRevision, Actual: p.Scope.Revision, Reason: "command is not bound to the current scope"}
	}
	return nil
}

func requireAuthority(p *CurrentPicture, c CommandEnvelope) error {
	if p.Authority.State != AuthorityActive {
		return gate(GateMissingAuthority, "consequential action requires active bounded authority")
	}
	if c.AuthorityRevision != p.Authority.Revision {
		return &GateError{Code: GateStaleAuthority, Concern: ConcernAuthority, Expected: c.AuthorityRevision, Actual: p.Authority.Revision, Reason: "authority revision no longer matches"}
	}
	if err := requireScope(p, c); err != nil {
		return err
	}
	if p.Authority.ScopeRevision != p.Scope.Revision {
		return gate(GateStaleAuthority, "authority is bound to an older scope")
	}
	if p.Authority.TargetID != "" && c.TargetID != p.Authority.TargetID {
		return gate(GateMissingAuthority, "target is outside the authority envelope")
	}
	if p.Authority.CandidateID != "" && c.CandidateID != p.Authority.CandidateID {
		return gate(GateMissingAuthority, "candidate is outside the authority envelope")
	}
	return nil
}

func requireCandidateTarget(p *CurrentPicture, c CommandEnvelope) error {
	if c.CandidateID == "" {
		return gate(GateMissingCandidate, "candidate binding is required")
	}
	if c.TargetID == "" {
		return gate(GateMissingTarget, "target binding is required")
	}
	if p.Release.CandidateID != "" && p.Release.CandidateID != c.CandidateID {
		return gate(GateContradictoryTransition, "command candidate does not match the active release")
	}
	if p.Release.TargetID != "" && p.Release.TargetID != c.TargetID {
		return gate(GateContradictoryTransition, "command target does not match the active release")
	}
	if c.TargetGeneration != p.Release.Generation {
		return &GateError{Code: GateTargetGenerationConflict, Concern: ConcernRelease, Expected: c.TargetGeneration, Actual: p.Release.Generation, Reason: "release target generation changed"}
	}
	return nil
}

func queue(p *CurrentPicture, c CommandEnvelope) error {
	if err := requireRole(c, RoleController); err != nil {
		return err
	}
	if c.OperationID == "" || c.RunnableTargetID == "" || c.CapacityEvidenceRef == "" || c.NextAutomaticAction == "" {
		return gate(GateContradictoryTransition, "queued requires an operation, runnable target, usable-capacity evidence, and automatic next action")
	}
	if p.Execution.State != ExecutionReceived && p.Execution.State != ExecutionWaitingAutomatic && p.Execution.State != ExecutionCorrection && p.Execution.State != ExecutionFailed {
		return gate(GateContradictoryTransition, "execution cannot queue from its current state")
	}
	p.Execution.State, p.Execution.OperationID = ExecutionQueued, c.OperationID
	p.Execution.RunnableTargetID, p.Execution.NextAutomaticAction = c.RunnableTargetID, c.NextAutomaticAction
	p.Revisions.Increment(ConcernExecution)
	return nil
}

func start(p *CurrentPicture, c CommandEnvelope) error {
	if err := requireRole(c, RoleWorker); err != nil {
		return err
	}
	if p.Execution.State != ExecutionQueued || c.OperationID == "" || c.OperationID != p.Execution.OperationID || c.LeaseID == "" {
		return gate(GateContradictoryTransition, "working requires the queued operation and a live lease")
	}
	p.Execution.State, p.Execution.ActorID, p.Execution.LeaseID = ExecutionWorking, c.ActorID, c.LeaseID
	p.Execution.CheckpointID = c.CheckpointID
	p.Revisions.Increment(ConcernExecution)
	return nil
}

func waitAutomatically(p *CurrentPicture, c CommandEnvelope) error {
	if err := requireRole(c, RoleController, RoleWorker, RoleReviewer, RoleRelease, RoleVerifier); err != nil {
		return err
	}
	if c.ExpectedSignal == "" || c.ResumeCondition == "" || c.NextAutomaticAction == "" {
		return gate(GateContradictoryTransition, "automatic wait requires an expected signal, resume condition, and next action")
	}
	p.Execution.SuspendedState, p.Execution.SuspendedOperationID = p.Execution.State, p.Execution.OperationID
	p.Execution.State, p.Execution.ExpectedSignal, p.Execution.ResumeCondition = ExecutionWaitingAutomatic, c.ExpectedSignal, c.ResumeCondition
	p.Execution.NextAutomaticAction = c.NextAutomaticAction
	p.Revisions.Increment(ConcernExecution)
	return nil
}

func setExecution(p *CurrentPicture, c CommandEnvelope, role ActorRole, state ExecutionState) error {
	if err := requireRole(c, role); err != nil {
		return err
	}
	// A technical stop while an owner decision continuation is paused must not
	// replace the exact pre-decision resume state with the incidental "paused"
	// state. That frozen state remains part of the visible unresolved decision.
	if !(state == ExecutionTechnicallyStopped && p.Execution.State == ExecutionPaused && p.Execution.SuspendedOperationID == p.Execution.OperationID) {
		p.Execution.SuspendedState, p.Execution.SuspendedOperationID = p.Execution.State, p.Execution.OperationID
	}
	p.Execution.State, p.Execution.ExpectedSignal, p.Execution.ResumeCondition = state, c.ExpectedSignal, c.ResumeCondition
	p.Revisions.Increment(ConcernExecution)
	return nil
}

func beginReview(p *CurrentPicture, c CommandEnvelope) error {
	if err := requireRole(c, RoleController); err != nil {
		return err
	}
	if p.Execution.State != ExecutionWorking && p.Execution.State != ExecutionCorrection {
		return gate(GateContradictoryTransition, "review requires completed implementation or correction work")
	}
	if c.CandidateID == "" || len(c.EvidenceRefs) == 0 {
		return gate(GateMissingEvidence, "review requires an exact candidate and implementation evidence")
	}
	if c.ReviewerID == "" {
		return gate(GateInvalidActorRole, "review requires an exact independent reviewer")
	}
	if c.ReviewerID == p.Execution.ActorID {
		return gate(GateInvalidActorRole, "reviewer must be independent from the implementer")
	}
	implementerID := p.Execution.ActorID
	p.Execution.State, p.Review.State, p.Review.CandidateID = ExecutionInReview, ReviewInProgress, c.CandidateID
	p.Review.AcceptanceVersion = p.Scope.AcceptanceVersion
	p.Review.ImplementerID = implementerID
	p.Review.ReviewerID, p.Review.CriteriaRefs, p.Review.CoverageRefs, p.Review.GapRefs, p.Review.RequiredCorrection = c.ReviewerID, nil, nil, nil, ""
	p.Execution.ActorID = c.ReviewerID
	for _, ref := range c.EvidenceRefs {
		p.Evidence.Other = append(p.Evidence.Other, boundEvidence(p, c, ref))
	}
	p.Revisions.Increment(ConcernExecution)
	p.Revisions.Increment(ConcernReview)
	p.Revisions.Increment(ConcernEvidence)
	return nil
}

// submitCorrection is deliberately one transition. Persistors must create a
// new immutable candidate and implementation evidence, supersede the rejected
// candidate, and advance all three concern heads in the same transaction. A
// half-state where execution names the replacement while review still names
// the rejected candidate is not authoritative and must never be observable.
func submitCorrection(p *CurrentPicture, c CommandEnvelope) error {
	if err := requireRole(c, RoleWorker); err != nil {
		return err
	}
	if p.Execution.State != ExecutionCorrection || p.Review.State != ReviewFailed {
		return gate(GateContradictoryTransition, "correction submission requires a failed review awaiting correction")
	}
	if c.OperationID == "" || c.OperationID != p.Execution.OperationID || c.CandidateID == "" || c.CandidateID == p.Review.CandidateID {
		return gate(GateContradictoryTransition, "correction submission requires the active operation and a new candidate")
	}
	if c.ActorID == "" || c.ActorID != p.Review.ImplementerID {
		return gate(GateInvalidActorRole, "correction result must come from the failed candidate implementer")
	}
	if c.ReviewerID == "" || c.ReviewerID == c.ActorID {
		return gate(GateInvalidActorRole, "correction requires an exact independent reviewer")
	}
	if len(c.EvidenceRefs) == 0 {
		return gate(GateMissingEvidence, "correction submission requires normalized implementation evidence")
	}
	if p.Review.MaxCorrections == 0 || p.Review.CorrectionCount == 0 || p.Review.CorrectionCount > p.Review.MaxCorrections {
		return gate(GateContradictoryTransition, "correction budget is missing or exhausted")
	}
	if p.Authority.CandidateID != "" && p.Authority.CandidateID != c.CandidateID {
		return gate(GateStaleAuthority, "candidate-bound authority does not transfer to a replacement candidate")
	}
	p.Execution.State = ExecutionInReview
	p.Execution.ActorID = c.ReviewerID
	p.Review.State = ReviewInProgress
	p.Review.CandidateID = c.CandidateID
	p.Review.AcceptanceVersion = p.Scope.AcceptanceVersion
	p.Review.ImplementerID = c.ActorID
	p.Review.ReviewerID = c.ReviewerID
	p.Review.CriteriaRefs, p.Review.CoverageRefs, p.Review.GapRefs, p.Review.RequiredCorrection = []string{}, []string{}, []string{}, ""
	for i := range p.Evidence.TechnicalReviews {
		if p.Evidence.TechnicalReviews[i].CandidateID != c.CandidateID {
			p.Evidence.TechnicalReviews[i].Freshness = EvidenceStale
		}
	}
	for _, ref := range c.EvidenceRefs {
		p.Evidence.Other = append(p.Evidence.Other, boundEvidence(p, c, ref))
	}
	p.Evidence.State = EvidenceCurrent
	p.Release = ReleaseFacet{State: ReleasePending, CandidateID: c.CandidateID, CandidateRevision: 1}
	p.Revisions.Increment(ConcernExecution)
	p.Revisions.Increment(ConcernReview)
	p.Revisions.Increment(ConcernRelease)
	p.Revisions.Increment(ConcernEvidence)
	return nil
}

func recordReview(p *CurrentPicture, c CommandEnvelope) error {
	if err := requireRole(c, RoleReviewer); err != nil {
		return err
	}
	if p.Execution.State != ExecutionInReview || p.Review.State != ReviewInProgress || c.CandidateID == "" || c.CandidateID != p.Review.CandidateID || p.Review.AcceptanceVersion != p.Scope.AcceptanceVersion {
		return gate(GateContradictoryTransition, "review result is not bound to the active candidate and acceptance version")
	}
	if len(c.EvidenceRefs) == 0 {
		return gate(GateMissingEvidence, "review result requires evidence")
	}
	if len(c.CriteriaRefs) == 0 || len(c.CoverageRefs) == 0 {
		return gate(GateMissingEvidence, "review result requires structured criteria and coverage")
	}
	if c.Kind != CommandReviewPassed && (len(c.GapRefs) == 0 || c.RequiredCorrection == "") {
		return gate(GateMissingEvidence, "non-passing review requires gaps and required correction")
	}
	if c.ActorID == p.Review.ImplementerID || (p.Review.ReviewerID != "" && c.ActorID != p.Review.ReviewerID) {
		return gate(GateInvalidActorRole, "reviewer must be independent from the implementer")
	}
	correctionExhausted := c.Kind == CommandReviewFailed && p.Review.MaxCorrections > 0 && p.Review.CorrectionCount >= p.Review.MaxCorrections
	if c.Kind == CommandReviewFailed && (c.MaxCorrections > 0 || c.CorrectionCount > 0) {
		if c.MaxCorrections == 0 || c.CorrectionCount == 0 || c.CorrectionCount > c.MaxCorrections {
			return gate(GateContradictoryTransition, "correction budget is missing or exhausted")
		}
		p.Review.CorrectionCount, p.Review.MaxCorrections = c.CorrectionCount, c.MaxCorrections
	}
	p.Review.ReviewerID = c.ActorID
	p.Review.CriteriaRefs, p.Review.CoverageRefs, p.Review.GapRefs, p.Review.RequiredCorrection = slices.Clone(c.CriteriaRefs), slices.Clone(c.CoverageRefs), slices.Clone(c.GapRefs), c.RequiredCorrection
	switch c.Kind {
	case CommandReviewPassed:
		p.Review.State, p.Execution.State = ReviewPassed, ExecutionCompleted
	case CommandReviewFailed:
		p.Review.State = ReviewFailed
		if correctionExhausted {
			p.Execution.State = ExecutionTechnicallyStopped
		} else {
			p.Execution.State = ExecutionCorrection
		}
	case CommandReviewInconclusive:
		p.Review.State, p.Execution.State = ReviewInconclusive, ExecutionTechnicallyStopped
	case CommandReviewBlockedEvidence:
		p.Review.State, p.Execution.State = ReviewBlockedByEvidence, ExecutionTechnicallyStopped
	}
	for _, ref := range c.EvidenceRefs {
		p.Evidence.TechnicalReviews = append(p.Evidence.TechnicalReviews, ReviewEvidence{BoundEvidence: BoundEvidence{Ref: ref, CandidateID: c.CandidateID, AcceptanceVersion: p.Scope.AcceptanceVersion, ScopeRevision: p.Scope.Revision, Freshness: EvidenceCurrent}, Result: p.Review.State, ReviewerID: c.ActorID, CriteriaRefs: slices.Clone(c.CriteriaRefs), CoverageRefs: slices.Clone(c.CoverageRefs), GapRefs: slices.Clone(c.GapRefs), RequiredCorrection: c.RequiredCorrection})
	}
	p.Evidence.State = EvidenceCurrent
	p.Revisions.Increment(ConcernReview)
	p.Revisions.Increment(ConcernExecution)
	p.Revisions.Increment(ConcernEvidence)
	return nil
}

func requestDecision(p *CurrentPicture, c CommandEnvelope) error {
	if err := requireRole(c, RoleController); err != nil {
		return err
	}
	if c.DecisionID == "" || c.DecisionType != DecisionAuthorization || c.OperationID == "" || c.OperationID != p.Execution.OperationID || c.BranchID == "" || c.CandidateID == "" || c.CandidateID != p.Review.CandidateID || c.AuthorityGap.Kind != AuthorityGapOperationAuthorization || c.AuthorityGap.RequestedCapability == "" || c.AuthorityGap.Reason == "" {
		return gate(GateInvalidDecision, "decision id, type, branch, and active suspended operation are required")
	}
	if _, exists := p.Decisions[c.DecisionID]; exists {
		return gate(GateInvalidDecision, "decision id already exists")
	}
	for _, existing := range p.Decisions {
		if existing.State == DecisionUnanswered || existing.Resolution == ResolutionResuming {
			return gate(GateInvalidDecision, "one issue-wide owner decision is already active")
		}
	}
	suspendedState := p.Execution.State
	if p.Execution.State == ExecutionPaused && p.Execution.SuspendedOperationID == c.OperationID {
		suspendedState = p.Execution.SuspendedState
	} else {
		p.Execution.SuspendedState, p.Execution.SuspendedOperationID = p.Execution.State, c.OperationID
	}
	p.Decisions[c.DecisionID] = OwnerDecision{ID: c.DecisionID, Revision: 1, Type: c.DecisionType, State: DecisionUnanswered, Resolution: ResolutionOpen, ScopeRevision: p.Scope.Revision, AuthorityRevision: p.Authority.Revision, CandidateID: c.CandidateID, TargetID: c.TargetID, SuspendedState: suspendedState, SuspendedOperationID: c.OperationID, BranchID: c.BranchID, AuthorityGap: c.AuthorityGap}
	p.Execution.State = ExecutionPaused
	p.Attention = AttentionFacet{DecisionID: c.DecisionID, Read: ReadUnread, Resolution: ResolutionOpen}
	p.Revisions.Increment(ConcernExecution)
	p.Revisions.Increment(ConcernAttention)
	invalidateEvidence(p)
	p.Revisions.Increment(ConcernEvidence)
	return nil
}

func respondDecision(p *CurrentPicture, c CommandEnvelope) error {
	if err := requireRole(c, RoleOwner); err != nil {
		return err
	}
	d, ok := p.Decisions[c.DecisionID]
	if !ok {
		return gate(GateInvalidDecision, "decision does not exist")
	}
	if d.State != DecisionUnanswered || d.Revision != c.ExpectedDecisionRevision {
		return &GateError{Code: GateStaleDecision, Concern: ConcernAttention, Expected: c.ExpectedDecisionRevision, Actual: d.Revision, Reason: "decision is answered, expired, or superseded"}
	}
	if c.ScopeRevision != d.ScopeRevision || p.Scope.Revision != d.ScopeRevision || p.Authority.Revision != d.AuthorityRevision || c.AuthorityRevision != d.AuthorityRevision || c.CandidateID != d.CandidateID || c.TargetID != d.TargetID || c.BranchID != d.BranchID || c.OperationID != d.SuspendedOperationID {
		return gate(GateStaleDecision, "response is not bound to the current decision scope, authority, branch, operation, candidate, and target")
	}
	if !decisionOutcomeAllowed(d.Type, c.DecisionOutcome) {
		return gate(GateInvalidDecision, "response type cannot satisfy this decision")
	}
	d.Response, d.SupportingText, d.Revision = c.DecisionOutcome, c.Detail, d.Revision+1
	switch c.DecisionOutcome {
	case DecisionAskQuestion:
		d.Resolution = ResolutionOpen
		p.Attention.Read, p.Attention.Resolution = ReadRead, ResolutionOpen
	case DecisionHold:
		d.Resolution, p.Attention.Resolution, p.Execution.State = ResolutionSnoozed, ResolutionSnoozed, ExecutionPaused
	case DecisionReject:
		d.State, d.Resolution = DecisionAnswered, ResolutionResuming
		p.Execution.State, p.Attention.Resolution = ExecutionPaused, ResolutionResuming
	case DecisionChangeScope, DecisionChangeBudget:
		d.State, d.Resolution = DecisionAnswered, ResolutionResolved
		p.Execution.State, p.Attention.Resolution = ExecutionPaused, ResolutionResolved
	case DecisionApprove:
		d.State, d.Resolution = DecisionAnswered, ResolutionResuming
		p.Execution.State, p.Attention.Resolution = ExecutionPaused, ResolutionResuming
	default:
		return gate(GateInvalidDecision, "unknown decision response")
	}
	p.Decisions[c.DecisionID] = d
	p.Revisions.Increment(ConcernAttention)
	p.Revisions.Increment(ConcernExecution)
	return nil
}

func readDecision(p *CurrentPicture, c CommandEnvelope, acknowledge bool) error {
	if err := requireRole(c, RoleOwner); err != nil {
		return err
	}
	d, ok := p.Decisions[c.DecisionID]
	if !ok || d.State != DecisionUnanswered || d.Revision != c.ExpectedDecisionRevision || p.Attention.DecisionID != d.ID {
		return gate(GateStaleDecision, "decision is not the current unanswered attention")
	}
	p.Attention.Read = ReadRead
	if acknowledge {
		p.Attention.Acknowledged = true
	}
	p.Revisions.Increment(ConcernAttention)
	return nil
}

func settleDecisionEffect(p *CurrentPicture, c CommandEnvelope) error {
	if err := requireRole(c, RoleController); err != nil {
		return err
	}
	d, ok := p.Decisions[c.DecisionID]
	if !ok || d.State != DecisionAnswered || d.Resolution != ResolutionResuming || d.Revision != c.ExpectedDecisionRevision || p.Attention.DecisionID != d.ID || p.Attention.Resolution != ResolutionResuming || p.Execution.State != ExecutionPaused {
		return gate(GateStaleDecision, "decision effect is not the current resuming obligation")
	}
	if c.DecisionType != d.Type || c.ScopeRevision != d.ScopeRevision || c.AuthorityRevision != d.AuthorityRevision || c.CandidateID != d.CandidateID || c.TargetID != d.TargetID || c.OperationID != d.SuspendedOperationID || c.BranchID != d.BranchID || c.SuspendedState != d.SuspendedState || c.DecisionOutcome != d.Response {
		return gate(GateStaleDecision, "decision effect lost its exact authority binding")
	}
	switch d.Response {
	case DecisionApprove:
		p.Execution.State, p.Execution.OperationID = d.SuspendedState, d.SuspendedOperationID
	case DecisionReject:
		p.Execution.State, p.Execution.OperationID = ExecutionQueued, d.SuspendedOperationID
	default:
		return gate(GateInvalidDecision, "unsupported authorization decision effect")
	}
	p.Execution.SuspendedState, p.Execution.SuspendedOperationID = "", ""
	d.Resolution = ResolutionResolved
	delete(p.Decisions, d.ID)
	p.Attention.Resolution = ResolutionResolved
	p.Revisions.Increment(ConcernExecution)
	p.Revisions.Increment(ConcernAttention)
	return nil
}

func hasUnansweredDecision(p *CurrentPicture, answered OwnerDecision) bool {
	for id, d := range p.Decisions {
		if id != answered.ID && d.State == DecisionUnanswered && d.BranchID == answered.BranchID && d.SuspendedOperationID == answered.SuspendedOperationID {
			return true
		}
	}
	return false
}

func decisionOutcomeAllowed(t DecisionType, outcome DecisionOutcome) bool {
	if outcome == DecisionAskQuestion || outcome == DecisionHold {
		return true
	}
	switch t {
	case DecisionApproval, DecisionAuthorization:
		return slices.Contains([]DecisionOutcome{DecisionApprove, DecisionReject}, outcome)
	case DecisionDirection:
		return slices.Contains([]DecisionOutcome{DecisionApprove, DecisionReject, DecisionRequestChanges, DecisionPartialAcceptance, DecisionChangeScope, DecisionCancel}, outcome)
	case DecisionFactualInput:
		return outcome == DecisionProvideFact || outcome == DecisionCancel
	case DecisionBudget:
		return slices.Contains([]DecisionOutcome{DecisionApprove, DecisionReject, DecisionChangeBudget, DecisionCancel}, outcome)
	case DecisionCancellation:
		return outcome == DecisionCancel || outcome == DecisionReject
	case DecisionExternalStep:
		return outcome == DecisionExternalStepComplete || outcome == DecisionCancel
	case DecisionAuthentication:
		return outcome == DecisionAuthenticationComplete || outcome == DecisionCancel
	default:
		return false
	}
}

func completeNonLive(p *CurrentPicture, c CommandEnvelope) error {
	if err := requireRole(c, RoleController); err != nil {
		return err
	}
	if p.Review.State != ReviewPassed || !hasCurrentReviewEvidence(p, p.Review.CandidateID) || p.Review.AcceptanceVersion != p.Scope.AcceptanceVersion {
		return gate(GateMissingEvidence, "Done requires current independent review evidence")
	}
	if p.Scope.DeploymentPolicy != DeploymentNotRequired || p.Scope.SourcePlanRevision == 0 {
		return gate(GateContradictoryTransition, "non-live completion requires an explicit normalized no-deployment policy")
	}
	p.Outcome, p.Execution.State, p.Release.State = OutcomeSatisfied, ExecutionCompleted, ReleaseNotRequired
	p.Revisions.Increment(ConcernOutcome)
	p.Revisions.Increment(ConcernExecution)
	p.Revisions.Increment(ConcernRelease)
	return nil
}

func hasCurrentReviewEvidence(p *CurrentPicture, candidateID string) bool {
	for _, e := range p.Evidence.TechnicalReviews {
		if e.CandidateID == candidateID && e.AcceptanceVersion == p.Scope.AcceptanceVersion && e.ScopeRevision == p.Scope.Revision && e.Freshness == EvidenceCurrent && e.Result == ReviewPassed {
			return true
		}
	}
	return false
}

func prepareRelease(p *CurrentPicture, c CommandEnvelope) error {
	if err := requireRole(c, RoleController); err != nil {
		return err
	}
	if p.Scope.DeploymentPolicy != DeploymentRequired || p.Scope.SourcePlanRevision == 0 {
		return gate(GateContradictoryTransition, "release preparation requires an explicit normalized deployment requirement")
	}
	if p.Review.State != ReviewPassed || !hasCurrentReviewEvidence(p, c.CandidateID) || c.CandidateID == "" || c.CandidateID != p.Review.CandidateID {
		return gate(GateMissingEvidence, "release preparation requires the current independently reviewed candidate")
	}
	if err := requireAuthority(p, c); err != nil {
		return err
	}
	if p.Release.State != ReleasePending || c.TargetID == "" || c.TargetGeneration == 0 || c.TargetGeneration != c.ExpectedTargetGeneration+1 || c.ReleaseOperatorID == "" || c.PlannedVerifierID == "" || c.ReleaseOperatorID == c.PlannedVerifierID || c.OperationID == "" || c.BranchID == "" || c.AcceptanceVersion != p.Scope.AcceptanceVersion || c.AuthorityCapability != "deploy_exact_release" || c.AuthorityEvidenceID == "" || c.OwnerEvidenceRevision == 0 || c.RecoveryPolicyRevision == 0 || c.ExposurePolicyDigest == "" || c.BuildManifestDigest == "" || c.ArtifactDigest == "" || c.RuntimeContractDigest == "" {
		return gate(GateInvalidCommand, "release preparation requires a complete exact authority and actor snapshot")
	}
	if c.ExpectedTargetGeneration == 0 && c.ExpectedTargetReleaseID != "" || c.ExpectedTargetGeneration > 0 && c.ExpectedTargetReleaseID == "" {
		return gate(GateTargetGenerationConflict, "release predecessor identity and generation are inconsistent")
	}
	p.Release = ReleaseFacet{
		State: ReleasePending, ReleaseID: c.ReleaseID, ReleaseKind: "forward", CandidateID: c.CandidateID, CandidateRevision: 1,
		TargetID: c.TargetID, Generation: c.TargetGeneration,
		ExpectedTargetReleaseID: c.ExpectedTargetReleaseID, ExpectedTargetGeneration: c.ExpectedTargetGeneration,
		OperationID: c.OperationID, BranchID: c.BranchID, AcceptanceVersion: c.AcceptanceVersion,
		ReleaseOperatorID: c.ReleaseOperatorID, PlannedVerifierID: c.PlannedVerifierID,
		AuthorityCapability: c.AuthorityCapability, AuthorityEvidenceID: c.AuthorityEvidenceID,
		OwnerEvidenceRevision: c.OwnerEvidenceRevision, RecoveryPolicyRevision: c.RecoveryPolicyRevision,
		ExposurePolicyDigest: c.ExposurePolicyDigest, RollbackTargetReleaseID: c.RollbackTargetReleaseID,
		IrreversibleEffects: append([]string{}, c.IrreversibleEffects...),
		BuildManifestDigest: c.BuildManifestDigest, ArtifactDigest: c.ArtifactDigest, RuntimeContractDigest: c.RuntimeContractDigest,
	}
	p.Revisions.Increment(ConcernRelease)
	return nil
}

func buildRelease(p *CurrentPicture, c CommandEnvelope) error {
	if err := requireRole(c, RoleRelease); err != nil {
		return err
	}
	if p.Review.State != ReviewPassed || !hasCurrentReviewEvidence(p, c.CandidateID) || c.CandidateID == "" || c.CandidateID != p.Review.CandidateID || c.TargetID == "" || len(c.EvidenceRefs) == 0 || p.Release.State != ReleasePending {
		return gate(GateMissingEvidence, "release build requires reviewed candidate, target, and reproducibility evidence")
	}
	if c.ActorID != p.Release.ReleaseOperatorID || c.TargetID != p.Release.TargetID || c.TargetGeneration != p.Release.Generation || c.BuildManifestDigest != p.Release.BuildManifestDigest || c.ArtifactDigest != p.Release.ArtifactDigest || c.RuntimeContractDigest != p.Release.RuntimeContractDigest {
		return gate(GateInvalidCommand, "release build does not match the frozen operator or reproducible build snapshot")
	}
	p.Release.State = ReleaseBuilt
	p.Evidence.Runtime, p.Evidence.Business = nil, nil
	for _, ref := range c.EvidenceRefs {
		p.Evidence.Other = append(p.Evidence.Other, boundEvidence(p, c, ref))
	}
	p.Revisions.Increment(ConcernRelease)
	p.Revisions.Increment(ConcernEvidence)
	return nil
}

func authorizeRelease(p *CurrentPicture, c CommandEnvelope) error {
	if err := requireRole(c, RoleController); err != nil {
		return err
	}
	if err := requireCandidateTarget(p, c); err != nil {
		return err
	}
	if p.Release.State != ReleaseBuilt {
		return gate(GateContradictoryTransition, "only a built release can be authorized")
	}
	if err := requireAuthority(p, c); err != nil {
		return err
	}
	if len(c.EvidenceRefs) == 0 {
		return gate(GateMissingEvidence, "release authorization requires a durable decision record")
	}
	if c.AuthorityCapability != p.Release.AuthorityCapability || c.AuthorityEvidenceID != p.Release.AuthorityEvidenceID || c.AuthorityRevision != p.Authority.Revision || c.CandidateID != p.Release.CandidateID || c.TargetID != p.Release.TargetID || c.TargetGeneration != p.Release.Generation {
		return gate(GateMissingAuthority, "release authorization does not match the frozen authority snapshot")
	}
	p.Release.State = ReleaseAuthorized
	for _, ref := range c.EvidenceRefs {
		p.Evidence.Other = append(p.Evidence.Other, boundEvidence(p, c, ref))
	}
	p.Revisions.Increment(ConcernRelease)
	p.Revisions.Increment(ConcernEvidence)
	return nil
}

func releaseStep(p *CurrentPicture, c CommandEnvelope, role ActorRole, from, to ReleaseState, authority bool) error {
	if err := requireRole(c, role); err != nil {
		return err
	}
	if err := requireCandidateTarget(p, c); err != nil {
		return err
	}
	if p.Release.State != from {
		return gate(GateContradictoryTransition, "release is not in the required prior state")
	}
	if authority {
		if err := requireAuthority(p, c); err != nil {
			return err
		}
	}
	if len(c.EvidenceRefs) == 0 {
		return gate(GateMissingEvidence, "release transition requires a durable receipt or verification evidence")
	}
	if p.Release.ReleaseOperatorID != "" && role == RoleRelease && c.ActorID != p.Release.ReleaseOperatorID {
		return gate(GateInvalidActorRole, "release actor does not match the built release operator")
	}
	if c.Kind == CommandRecordDeployment && c.PlannedVerifierID != "" {
		if c.PlannedVerifierID == p.Release.ReleaseOperatorID {
			return gate(GateInvalidActorRole, "production verifier must be independent from the release operator")
		}
		if p.Release.PlannedVerifierID != "" && p.Release.PlannedVerifierID != c.PlannedVerifierID {
			return gate(GateContradictoryTransition, "planned production verifier changed")
		}
		p.Release.PlannedVerifierID = c.PlannedVerifierID
	}
	if c.Kind == CommandBeginVerification && p.Release.PlannedVerifierID != "" && c.ActorID != p.Release.PlannedVerifierID {
		return gate(GateInvalidActorRole, "verification actor does not match the planned verifier")
	}
	p.Release.State = to
	if c.Kind == CommandStartDeployment {
		p.Execution.ActorID = c.ActorID
		p.Execution.RunnableTargetID = c.TargetID
		p.Revisions.Increment(ConcernExecution)
	}
	for _, ref := range c.EvidenceRefs {
		p.Evidence.Other = append(p.Evidence.Other, boundEvidence(p, c, ref))
	}
	p.Revisions.Increment(ConcernRelease)
	p.Revisions.Increment(ConcernEvidence)
	return nil
}

func verifyRuntime(p *CurrentPicture, c CommandEnvelope) error {
	if err := requireRole(c, RoleVerifier); err != nil {
		return err
	}
	if err := requireCandidateTarget(p, c); err != nil {
		return err
	}
	if p.Release.State != ReleaseProductionVerifying || len(c.EvidenceRefs) == 0 {
		return gate(GateMissingEvidence, "runtime verification requires current target evidence")
	}
	if p.Release.PlannedVerifierID != "" && (c.ActorID != p.Release.PlannedVerifierID || c.ActorID == p.Release.ReleaseOperatorID) {
		return gate(GateInvalidActorRole, "runtime verification actor does not match the independent planned verifier")
	}
	for _, ref := range c.EvidenceRefs {
		p.Evidence.Runtime = append(p.Evidence.Runtime, boundEvidence(p, c, ref))
	}
	p.Evidence.State = EvidenceCurrent
	p.Revisions.Increment(ConcernEvidence)
	return nil
}

func verifyBusiness(p *CurrentPicture, c CommandEnvelope) error {
	if err := requireRole(c, RoleVerifier); err != nil {
		return err
	}
	if err := requireCandidateTarget(p, c); err != nil {
		return err
	}
	if p.Release.State != ReleaseProductionVerifying || !hasBoundCurrent(p.Evidence.Runtime, p, c) || len(c.EvidenceRefs) == 0 || p.Evidence.State != EvidenceCurrent {
		return gate(GateMissingEvidence, "Live requires current runtime and business verification")
	}
	if p.Release.PlannedVerifierID != "" && (c.ActorID != p.Release.PlannedVerifierID || c.ActorID == p.Release.ReleaseOperatorID) {
		return gate(GateInvalidActorRole, "business verification actor does not match the independent planned verifier")
	}
	for _, ref := range c.EvidenceRefs {
		p.Evidence.Business = append(p.Evidence.Business, boundEvidence(p, c, ref))
	}
	p.Release.State, p.Outcome, p.Execution.State = ReleaseLive, OutcomeSatisfied, ExecutionCompleted
	p.Revisions.Increment(ConcernEvidence)
	p.Revisions.Increment(ConcernRelease)
	p.Revisions.Increment(ConcernOutcome)
	p.Revisions.Increment(ConcernExecution)
	return nil
}

func productionFailed(p *CurrentPicture, c CommandEnvelope) error {
	if err := requireRole(c, RoleVerifier, RoleObserver); err != nil {
		return err
	}
	if err := requireCandidateTarget(p, c); err != nil {
		return err
	}
	if len(c.EvidenceRefs) == 0 {
		return gate(GateMissingEvidence, "production failure requires evidence")
	}
	if c.ReleaseID == "" || c.ReleaseID != p.Release.ReleaseID || c.CandidateRevision == 0 || c.CandidateRevision != p.Release.CandidateRevision {
		return gate(GateContradictoryTransition, "production failure must bind the exact current release and candidate revision")
	}
	if p.Release.State != ReleaseLive {
		return gate(GateContradictoryTransition, "only the exact current live release can regress")
	}
	if c.RecoveryMode != RecoveryRollback && c.RecoveryMode != RecoveryContainment {
		return gate(GateInvalidCommand, "production failure requires a frozen rollback or containment decision")
	}
	failedRelease := p.Release
	p.Release.State, p.Evidence.State = ReleaseDegraded, EvidenceContradicted
	for _, ref := range c.EvidenceRefs {
		e := boundEvidence(p, c, ref)
		e.Freshness = EvidenceContradicted
		p.Evidence.Business = append(p.Evidence.Business, e)
	}
	if c.RecoveryMode == RecoveryRollback {
		if c.RecoveryReleaseID == "" || c.RecoveryReleaseID == failedRelease.ReleaseID || c.RecoveryOfReleaseID != failedRelease.ReleaseID || c.RecoveryTargetGeneration != failedRelease.Generation+1 || failedRelease.RollbackTargetReleaseID == "" {
			return gate(GateInvalidCommand, "rollback requires a new recovery release at the next target generation")
		}
		p.Release.ReleaseID = c.RecoveryReleaseID
		p.Release.ReleaseKind = "recovery"
		p.Release.RecoveryOfReleaseID = failedRelease.ReleaseID
		if c.RecoveryCandidateID == "" || c.RecoveryCandidateRevision == 0 || c.RecoverySourceCandidateID == "" || c.RecoverySourceRevision == 0 || c.RecoveryBuildManifestDigest == "" || c.RecoveryArtifactDigest == "" || c.RecoveryRuntimeDigest == "" || c.RecoveryReviewedReviewID == "" || c.RecoveryReviewedReviewRevision == 0 {
			return gate(GateInvalidCommand, "rollback requires an exact known-good candidate and artifact snapshot")
		}
		p.Release.CandidateID = c.RecoveryCandidateID
		p.Release.CandidateRevision = c.RecoveryCandidateRevision
		p.Release.BuildManifestDigest = c.RecoveryBuildManifestDigest
		p.Release.ArtifactDigest = c.RecoveryArtifactDigest
		p.Release.RuntimeContractDigest = c.RecoveryRuntimeDigest
		p.Release.State = ReleaseRollingBack
		p.Release.Generation = c.RecoveryTargetGeneration
		p.Release.ExpectedTargetReleaseID = failedRelease.ReleaseID
		p.Release.ExpectedTargetGeneration = failedRelease.Generation
	}
	if p.Outcome == OutcomeSatisfied {
		p.Outcome = OutcomeOpen
	}
	p.Execution.State = ExecutionTechnicallyStopped
	p.Revisions.Increment(ConcernRelease)
	p.Revisions.Increment(ConcernEvidence)
	p.Revisions.Increment(ConcernExecution)
	p.Revisions.Increment(ConcernOutcome)
	return nil
}

func rollbackSucceeded(p *CurrentPicture, c CommandEnvelope) error {
	if err := requireRole(c, RoleRelease); err != nil {
		return err
	}
	if err := requireCandidateTarget(p, c); err != nil {
		return err
	}
	if p.Release.State != ReleaseRollingBack || p.Release.ReleaseKind != "recovery" || p.Release.RecoveryOfReleaseID == "" || len(c.EvidenceRefs) == 0 || c.ActorID != p.Release.ReleaseOperatorID {
		return gate(GateContradictoryTransition, "rollback receipt must bind the exact recovery release and operator")
	}
	p.Release.State = ReleaseRolledBack
	for _, ref := range c.EvidenceRefs {
		e := boundEvidence(p, c, ref)
		e.Freshness = EvidenceContradicted
		p.Evidence.Other = append(p.Evidence.Other, e)
	}
	p.Revisions.Increment(ConcernRelease)
	p.Revisions.Increment(ConcernEvidence)
	// Recovery of the prior service state does not satisfy the requested change.
	if p.Outcome == OutcomeSatisfied {
		p.Outcome = OutcomeOpen
		p.Revisions.Increment(ConcernOutcome)
	}
	p.Execution.State = ExecutionTechnicallyStopped
	p.Revisions.Increment(ConcernExecution)
	return nil
}

func restoreVerified(p *CurrentPicture, c CommandEnvelope) error {
	if err := requireRole(c, RoleVerifier); err != nil {
		return err
	}
	if err := requireCandidateTarget(p, c); err != nil {
		return err
	}
	if p.Release.State != ReleaseRolledBack || p.Release.ReleaseKind != "recovery" || len(c.EvidenceRefs) != 4 {
		return gate(GateMissingEvidence, "restoration requires rolled-back state and independent evidence")
	}
	if p.Release.PlannedVerifierID == "" || c.ActorID != p.Release.PlannedVerifierID || c.ActorID == p.Release.ReleaseOperatorID {
		return gate(GateInvalidActorRole, "restoration requires the exact independent planned verifier")
	}
	p.Release.State, p.Execution.State = ReleaseRestored, ExecutionCorrection
	p.Evidence.Runtime = append(p.Evidence.Runtime, boundEvidence(p, c, c.EvidenceRefs[0]))
	p.Evidence.Business = append(p.Evidence.Business, boundEvidence(p, c, c.EvidenceRefs[1]), boundEvidence(p, c, c.EvidenceRefs[2]))
	p.Evidence.Restoration = append(p.Evidence.Restoration, boundEvidence(p, c, c.EvidenceRefs[3]))
	p.Evidence.State = EvidenceCurrent
	// Restored service does not satisfy the requested new outcome.
	wasOpen := p.Outcome == OutcomeOpen
	p.Outcome = OutcomeOpen
	p.Revisions.Increment(ConcernRelease)
	p.Revisions.Increment(ConcernExecution)
	p.Revisions.Increment(ConcernEvidence)
	if !wasOpen {
		p.Revisions.Increment(ConcernOutcome)
	}
	return nil
}

func pause(p *CurrentPicture, c CommandEnvelope) error {
	if err := requireRole(c, RoleOwner, RoleController); err != nil {
		return err
	}
	p.Execution.SuspendedState, p.Execution.SuspendedOperationID = p.Execution.State, p.Execution.OperationID
	p.Execution.State = ExecutionPaused
	p.Revisions.Increment(ConcernExecution)
	return nil
}

func resume(p *CurrentPicture, c CommandEnvelope) error {
	if err := requireRole(c, RoleController); err != nil {
		return err
	}
	if p.Execution.State != ExecutionPaused && p.Execution.State != ExecutionWaitingAutomatic && p.Execution.State != ExecutionBlockedExternal {
		return gate(GateContradictoryTransition, "only suspended execution can resume")
	}
	if c.OperationID == "" || c.OperationID != p.Execution.SuspendedOperationID {
		return gate(GateContradictoryTransition, "resume must name the exact suspended operation")
	}
	p.Execution.State, p.Execution.OperationID = p.Execution.SuspendedState, p.Execution.SuspendedOperationID
	p.Execution.SuspendedState, p.Execution.SuspendedOperationID = "", ""
	p.Revisions.Increment(ConcernExecution)
	return nil
}

func cancel(p *CurrentPicture, c CommandEnvelope) error {
	if err := requireRole(c, RoleOwner, RoleController); err != nil {
		return err
	}
	p.Outcome, p.Execution.State = OutcomeCancelled, ExecutionCancelled
	p.Revisions.Increment(ConcernOutcome)
	p.Revisions.Increment(ConcernExecution)
	return nil
}

func supersede(p *CurrentPicture, c CommandEnvelope) error {
	if err := requireRole(c, RoleOwner, RoleController); err != nil {
		return err
	}
	p.Outcome, p.Execution.State, p.Authority.State = OutcomeSuperseded, ExecutionSuperseded, AuthoritySuperseded
	for id, d := range p.Decisions {
		if d.State == DecisionUnanswered {
			d.State, d.Resolution, d.Revision = DecisionSuperseded, ResolutionResolved, d.Revision+1
			p.Decisions[id] = d
		}
	}
	p.Revisions.Increment(ConcernOutcome)
	p.Revisions.Increment(ConcernExecution)
	p.Revisions.Increment(ConcernAuthority)
	p.Revisions.Increment(ConcernAttention)
	return nil
}

func reviseScope(p *CurrentPicture, c CommandEnvelope) error {
	if err := requireRole(c, RoleOwner); err != nil {
		return err
	}
	if c.ScopeRevision != p.Scope.Revision {
		return gate(GateStaleScope, "scope revision command is stale")
	}
	p.Scope.Revision++
	p.Scope.AcceptanceVersion++
	p.Revisions.Scope = p.Scope.Revision
	p.Authority.State = AuthoritySuperseded
	p.Authority.Revision++
	p.Revisions.Increment(ConcernAuthority)
	p.Review.State = ReviewPending
	p.Revisions.Increment(ConcernReview)
	p.Execution.State = ExecutionPaused
	p.Revisions.Increment(ConcernExecution)
	for id, d := range p.Decisions {
		if d.State == DecisionUnanswered {
			d.State, d.Resolution, d.Revision = DecisionSuperseded, ResolutionResolved, d.Revision+1
			p.Decisions[id] = d
		}
	}
	p.Revisions.Increment(ConcernAttention)
	return nil
}

func grantAuthority(p *CurrentPicture, c CommandEnvelope) error {
	if err := requireRole(c, RoleOwner); err != nil {
		return err
	}
	if err := requireScope(p, c); err != nil {
		return err
	}
	p.Authority = AuthorityFacet{State: AuthorityActive, Revision: p.Authority.Revision + 1, ScopeRevision: p.Scope.Revision, TargetID: c.TargetID, CandidateID: c.CandidateID}
	p.Revisions.Authority = p.Authority.Revision
	invalidateOpenDecisions(p)
	return nil
}

func revokeAuthority(p *CurrentPicture, c CommandEnvelope) error {
	if err := requireRole(c, RoleOwner); err != nil {
		return err
	}
	if c.AuthorityRevision != p.Authority.Revision {
		return gate(GateStaleAuthority, "authority revocation is stale")
	}
	p.Authority.State, p.Execution.State = AuthorityRevoked, ExecutionPaused
	p.Authority.Revision++
	p.Revisions.Authority = p.Authority.Revision
	p.Revisions.Increment(ConcernExecution)
	invalidateOpenDecisions(p)
	return nil
}

func recordEvidence(p *CurrentPicture, c CommandEnvelope) error {
	if err := requireRole(c, RoleReviewer, RoleVerifier, RoleObserver, RoleController); err != nil {
		return err
	}
	if len(c.EvidenceRefs) == 0 {
		return gate(GateMissingEvidence, "evidence references are required")
	}
	p.Evidence.State = EvidenceCurrent
	for _, ref := range c.EvidenceRefs {
		p.Evidence.Other = append(p.Evidence.Other, boundEvidence(p, c, ref))
	}
	p.Revisions.Increment(ConcernEvidence)
	return nil
}

func contradictEvidence(p *CurrentPicture, c CommandEnvelope) error {
	if err := requireRole(c, RoleReviewer, RoleVerifier, RoleObserver); err != nil {
		return err
	}
	if len(c.EvidenceRefs) == 0 {
		return gate(GateMissingEvidence, "contradiction requires evidence")
	}
	if err := requireCandidateTarget(p, c); err != nil {
		return err
	}
	p.Evidence.State = EvidenceContradicted
	for _, ref := range c.EvidenceRefs {
		e := boundEvidence(p, c, ref)
		e.Freshness = EvidenceContradicted
		p.Evidence.Other = append(p.Evidence.Other, e)
	}
	if p.Release.State == ReleaseLive {
		p.Release.State = ReleaseDegraded
		p.Revisions.Increment(ConcernRelease)
	}
	p.Revisions.Increment(ConcernEvidence)
	return nil
}

func boundEvidence(p *CurrentPicture, c CommandEnvelope, ref string) BoundEvidence {
	return BoundEvidence{Ref: ref, CandidateID: c.CandidateID, TargetID: c.TargetID, TargetGeneration: c.TargetGeneration, AcceptanceVersion: p.Scope.AcceptanceVersion, ScopeRevision: p.Scope.Revision, Freshness: EvidenceCurrent}
}

func hasBoundCurrent(records []BoundEvidence, p *CurrentPicture, c CommandEnvelope) bool {
	for _, e := range records {
		if e.CandidateID == c.CandidateID && e.TargetID == c.TargetID && e.TargetGeneration == c.TargetGeneration && e.AcceptanceVersion == p.Scope.AcceptanceVersion && e.ScopeRevision == p.Scope.Revision && e.Freshness == EvidenceCurrent {
			return true
		}
	}
	return false
}

func invalidateEvidence(p *CurrentPicture) {
	for i := range p.Evidence.TechnicalReviews {
		p.Evidence.TechnicalReviews[i].Freshness = EvidenceStale
	}
	for i := range p.Evidence.Runtime {
		p.Evidence.Runtime[i].Freshness = EvidenceStale
	}
	for i := range p.Evidence.Business {
		p.Evidence.Business[i].Freshness = EvidenceStale
	}
	for i := range p.Evidence.Restoration {
		p.Evidence.Restoration[i].Freshness = EvidenceStale
	}
	for i := range p.Evidence.Other {
		p.Evidence.Other[i].Freshness = EvidenceStale
	}
	p.Evidence.State = EvidenceStale
}

func invalidateOpenDecisions(p *CurrentPicture) {
	changed := false
	for id, d := range p.Decisions {
		if d.State == DecisionUnanswered {
			d.State, d.Resolution, d.Revision = DecisionSuperseded, ResolutionResolved, d.Revision+1
			p.Decisions[id] = d
			changed = true
		}
	}
	if changed {
		p.Revisions.Increment(ConcernAttention)
	}
}
