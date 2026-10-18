package workflowcommand

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	googleuuid "github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	workflowstatev2 "github.com/multica-ai/multica/server/internal/workflowstate/v2"
	"github.com/multica-ai/multica/server/internal/workflowstore"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestPreviewSourceRevisionIsStableAndCandidateBound(t *testing.T) {
	candidateID := googleuuid.MustParse("55555555-5555-4555-8555-555555555555")
	if got, want := previewSourceRevision(candidateID), "candidate:55555555-5555-4555-8555-555555555555:1"; got != want {
		t.Fatalf("preview source revision = %q, want %q", got, want)
	}
}

func TestSemanticFingerprintIncludesDerivedPrincipalAndCanonicalExpectedRevisions(t *testing.T) {
	p := Principal{ActorID: uuid("00000000-0000-0000-0000-000000000901"), ActorType: "reviewer", TaskID: uuid("00000000-0000-0000-0000-000000000903"), ActionKey: "review/action", LeaseToken: uuid("00000000-0000-0000-0000-000000000904")}
	baseline := p
	r := Request{WorkspaceID: uuid(testWorkspace), IssueID: uuid(testIssue), CommandKey: "review-1", Kind: workflowstatev2.CommandReviewFailed, Expected: []workflowstatev2.ExpectedRevision{{Concern: workflowstatev2.ConcernReview, Revision: 2}, {Concern: workflowstatev2.ConcernEvidence, Revision: 3}}}
	c := workflowstatev2.CommandEnvelope{ActorRole: workflowstatev2.RoleReviewer}
	a, _, _, err := semanticFingerprint(p, r, c)
	if err != nil {
		t.Fatal(err)
	}
	r.Expected[0], r.Expected[1] = r.Expected[1], r.Expected[0]
	b, _, _, err := semanticFingerprint(p, r, c)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("expected-revision order changed semantic hash: %s != %s", a, b)
	}
	differentActor := baseline
	differentActor.ActorID = uuid("00000000-0000-0000-0000-000000000902")
	different, _, _, _ := semanticFingerprint(differentActor, r, c)
	if a == different {
		t.Fatal("derived actor identity was absent from semantic hash")
	}
	for name, mutate := range map[string]func(*Principal){
		"task":   func(v *Principal) { v.TaskID = uuid("00000000-0000-0000-0000-000000000905") },
		"action": func(v *Principal) { v.ActionKey = "review/other" },
		"lease":  func(v *Principal) { v.LeaseToken = uuid("00000000-0000-0000-0000-000000000906") },
	} {
		changed := baseline
		mutate(&changed)
		got, _, _, _ := semanticFingerprint(changed, r, c)
		if got == a {
			t.Fatalf("%s binding was absent from semantic hash", name)
		}
	}
}

func TestDeriveActionsKeepsExecutionPausedUntilExactEffectSettles(t *testing.T) {
	w, i := uuid(testWorkspace), uuid(testIssue)
	request := Request{WorkspaceID: w, IssueID: i, CommandKey: "respond", Kind: workflowstatev2.CommandRespondDecision, DecisionID: "decision", DecisionOutcome: workflowstatev2.DecisionApprove}
	commandID := stableUUID(w, i, request.CommandKey, "command")
	p := workflowstatev2.CurrentPicture{Execution: workflowstatev2.ExecutionFacet{State: workflowstatev2.ExecutionPaused}, Decisions: map[string]workflowstatev2.OwnerDecision{"decision": {ID: "decision", Resolution: workflowstatev2.ResolutionResuming, SuspendedOperationID: "00000000-0000-0000-0000-000000000777"}}}
	actions, err := deriveActions(request, commandID, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 || actions[0].Type != "resume_exact_operation" || actions[0].OperationID != "00000000-0000-0000-0000-000000000777" {
		t.Fatalf("resume action not exact: %#v", actions)
	}
	again, _ := deriveActions(request, commandID, p)
	if !reflect.DeepEqual(actions, again) {
		t.Fatalf("action identity is unstable: %#v %#v", actions, again)
	}
}

func TestDeriveActionsSeparatesApproveAndReject(t *testing.T) {
	w, i := uuid(testWorkspace), uuid(testIssue)
	request := Request{WorkspaceID: w, IssueID: i, CommandKey: "respond", Kind: workflowstatev2.CommandRespondDecision, DecisionID: "decision"}
	commandID := stableUUID(w, i, request.CommandKey, "command")
	for _, tc := range []struct {
		name       string
		outcome    workflowstatev2.OutcomeState
		resolution workflowstatev2.ResolutionState
		execution  workflowstatev2.ExecutionState
		want       string
	}{
		{"approve", workflowstatev2.OutcomeOpen, workflowstatev2.ResolutionResuming, workflowstatev2.ExecutionPaused, "resume_exact_operation"},
		{"reject", workflowstatev2.OutcomeOpen, workflowstatev2.ResolutionResuming, workflowstatev2.ExecutionPaused, "replan_exact_operation"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request.DecisionOutcome = map[string]workflowstatev2.DecisionOutcome{"approve": workflowstatev2.DecisionApprove, "reject": workflowstatev2.DecisionReject}[tc.name]
			p := workflowstatev2.CurrentPicture{Outcome: tc.outcome, Execution: workflowstatev2.ExecutionFacet{State: tc.execution}, Decisions: map[string]workflowstatev2.OwnerDecision{"decision": {ID: "decision", Resolution: tc.resolution, SuspendedOperationID: "00000000-0000-0000-0000-000000000777"}}}
			actions, err := deriveActions(request, commandID, p)
			if err != nil {
				t.Fatal(err)
			}
			if len(actions) != 1 || actions[0].Type != tc.want {
				t.Fatalf("actions=%#v want %s", actions, tc.want)
			}
		})
	}
}

func TestDeriveActionsRejectsUnsupportedDecisionOutcomes(t *testing.T) {
	r := Request{WorkspaceID: uuid(testWorkspace), IssueID: uuid(testIssue), CommandKey: "respond", Kind: workflowstatev2.CommandRespondDecision, DecisionID: "decision"}
	p := workflowstatev2.CurrentPicture{Decisions: map[string]workflowstatev2.OwnerDecision{"decision": {ID: "decision"}}}
	for _, outcome := range []workflowstatev2.DecisionOutcome{workflowstatev2.DecisionAskQuestion, workflowstatev2.DecisionChangeScope, workflowstatev2.DecisionChangeBudget, "invented"} {
		r.DecisionOutcome = outcome
		if actions, err := deriveActions(r, stableUUID(r.WorkspaceID, r.IssueID, r.CommandKey, "command"), p); !errors.Is(err, ErrUnsupportedCommand) || len(actions) != 0 {
			t.Fatalf("outcome %q actions=%#v error=%v", outcome, actions, err)
		}
	}
}

func TestChangedConcernsAreFixedAndSorted(t *testing.T) {
	a := workflowstatev2.ConcernRevisions{Outcome: 1, Execution: 1, Evidence: 1, Review: 1}
	b := a
	b.Execution++
	b.Evidence++
	b.Review++
	got := changedConcerns(a, b)
	want := []workflowstatev2.Concern{workflowstatev2.ConcernEvidence, workflowstatev2.ConcernExecution, workflowstatev2.ConcernReview}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("changed concerns = %v, want %v", got, want)
	}
}

func TestServiceRejectsInvalidBoundaryWithoutDatabase(t *testing.T) {
	_, err := (Service{}).Execute(t.Context(), Principal{}, Request{})
	if err == nil || errors.Is(err, ErrUnsupportedCommand) {
		t.Fatalf("invalid service boundary error = %v", err)
	}
}

func TestTypedWorkflowResultContentMustBeMeaningful(t *testing.T) {
	for _, value := range []map[string]any{nil, {}, {"": "value"}, {"summary": nil}, {"summary": "  "}, {"summary": map[string]any{}}, {"items": []any{}}, {"nested": []any{map[string]any{"value": " "}}}} {
		if meaningfulObject(value) {
			t.Fatalf("meaningfulObject(%#v) = true", value)
		}
	}
	if !meaningfulObject(map[string]any{"summary": "verified"}) {
		t.Fatal("meaningful structured result was rejected")
	}
	for _, refs := range [][]string{nil, {}, {""}, {"  "}, {"same", "same"}} {
		if meaningfulRefs(refs) {
			t.Fatalf("meaningfulRefs(%#v) = true", refs)
		}
	}
	if !meaningfulRefs([]string{"criterion", "coverage"}) {
		t.Fatal("meaningful refs were rejected")
	}
	pass := &CorrectionResultInput{Evidence: map[string]any{"summary": "pass"}, CriteriaRefs: []string{"criterion"}, CoverageRefs: []string{"coverage"}}
	if !validReviewResultSemantics(pass, workflowstatev2.CommandReviewPassed) {
		t.Fatal("coherent pass was rejected")
	}
	pass.GapRefs, pass.RequiredCorrection = []string{"gap"}, "fix it"
	if validReviewResultSemantics(pass, workflowstatev2.CommandReviewPassed) {
		t.Fatal("pass with unresolved gaps/correction was accepted")
	}
}

func TestReleaseEffectEvidenceRequiresDistinctTypedProofs(t *testing.T) {
	runtime := VerificationProof{CriteriaRefs: []string{"runtime-contract"}, CoverageRefs: []string{"availability"}, Evidence: map[string]any{"healthy": true}}
	business := VerificationProof{CriteriaRefs: []string{"owner-outcome"}, CoverageRefs: []string{"canary"}, Evidence: map[string]any{"accepted": true}}
	regression := VerificationProof{CriteriaRefs: []string{"no-regression"}, CoverageRefs: []string{"suite"}, Evidence: map[string]any{"passed": true}}
	restoration := VerificationProof{CriteriaRefs: []string{"restoration"}, CoverageRefs: []string{"known-good"}, Evidence: map[string]any{"restored": true}}

	cases := []struct {
		name     string
		kind     workflowstatev2.CommandKind
		evidence map[string]any
		want     bool
	}{
		{"deployment", workflowstatev2.CommandRecordDeployment, map[string]any{"provider_state": "deployed", "receipt_verified": true}, true},
		{"deployment unknown field", workflowstatev2.CommandRecordDeployment, map[string]any{"provider_state": "deployed", "receipt_verified": true, "claim": true}, false},
		{"runtime", workflowstatev2.CommandVerifyRuntime, map[string]any{"runtime_proof": runtime}, true},
		{"runtime empty nested evidence", workflowstatev2.CommandVerifyRuntime, map[string]any{"runtime_proof": VerificationProof{CriteriaRefs: []string{"runtime"}, CoverageRefs: []string{"all"}, Evidence: map[string]any{"summary": map[string]any{}}}}, false},
		{"business distinct", workflowstatev2.CommandVerifyBusiness, map[string]any{"business_proof": business, "regression_proof": regression}, true},
		{"business duplicates proof", workflowstatev2.CommandVerifyBusiness, map[string]any{"business_proof": business, "regression_proof": business}, false},
		{"business missing regression", workflowstatev2.CommandVerifyBusiness, map[string]any{"business_proof": business}, false},
		{"restoration distinct", workflowstatev2.CommandRestoreVerified, map[string]any{"runtime_proof": runtime, "business_proof": business, "regression_proof": regression, "restoration_proof": restoration}, true},
		{"restoration duplicate identities", workflowstatev2.CommandRestoreVerified, map[string]any{"runtime_proof": runtime, "business_proof": business, "regression_proof": regression, "restoration_proof": regression}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := validReleaseEffectEvidence(tc.kind, tc.evidence); got != tc.want {
				t.Fatalf("validReleaseEffectEvidence()=%v want %v", got, tc.want)
			}
		})
	}
}

func TestReleaseVerificationWireShapeRejectsIrrelevantKnownFields(t *testing.T) {
	payload, _ := json.Marshal(releaseEffectSnapshot{OperationID: "00000000-0000-0000-0000-000000000901", BranchKey: "main", ReleaseID: "00000000-0000-0000-0000-000000000902", CandidateID: "00000000-0000-0000-0000-000000000903", CandidateRevision: 1, ScopeID: "00000000-0000-0000-0000-000000000904", ScopeRevision: 1, AcceptanceVersion: 1, TargetKey: "production", DesiredTargetGeneration: 1, ArtifactDigest: "sha256:artifact", RuntimeContractDigest: "sha256:runtime"})
	route := db.GetWorkflowTaskResultRouteRow{ActionType: "verify_runtime_exact_release", Payload: payload, EffectKey: pgtype.Text{String: "effect", Valid: true}}
	proof := VerificationProof{CriteriaRefs: []string{"runtime"}, CoverageRefs: []string{"all"}, Evidence: map[string]any{"healthy": true}}
	valid := &CorrectionResultInput{Type: "runtime_verification", RuntimeProof: &proof}
	if _, err := releaseVerificationTaskRequest(route, valid); err != nil {
		t.Fatalf("valid runtime result rejected: %v", err)
	}
	mutations := []struct {
		name string
		set  func(*CorrectionResultInput)
	}{
		{"source revision", func(v *CorrectionResultInput) { v.SourceRevision = "unexpected" }},
		{"artifact", func(v *CorrectionResultInput) { v.Artifact = map[string]any{"unexpected": true} }},
		{"review verdict", func(v *CorrectionResultInput) { v.Verdict = "passed" }},
		{"review criteria", func(v *CorrectionResultInput) { v.CriteriaRefs = []string{"unexpected"} }},
		{"business proof", func(v *CorrectionResultInput) { v.BusinessProof = &proof }},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			candidate := *valid
			mutation.set(&candidate)
			if _, err := releaseVerificationTaskRequest(route, &candidate); !errors.Is(err, ErrInvalidAuthoritativeState) {
				t.Fatalf("irrelevant field accepted: %v", err)
			}
		})
	}
}

func TestRestorationTaskResultRequiresFourDistinctProofs(t *testing.T) {
	payload, _ := json.Marshal(releaseEffectSnapshot{OperationID: "00000000-0000-0000-0000-000000000901", BranchKey: "main", ReleaseID: "00000000-0000-0000-0000-000000000902", CandidateID: "00000000-0000-0000-0000-000000000903", CandidateRevision: 1, ScopeID: "00000000-0000-0000-0000-000000000904", ScopeRevision: 1, AcceptanceVersion: 1, TargetKey: "production", DesiredTargetGeneration: 2, ArtifactDigest: "sha256:artifact", RuntimeContractDigest: "sha256:runtime"})
	route := db.GetWorkflowTaskResultRouteRow{ActionType: "verify_restoration_exact_release", Payload: payload, EffectKey: pgtype.Text{String: "effect", Valid: true}}
	proof := func(kind string) *VerificationProof {
		return &VerificationProof{CriteriaRefs: []string{kind}, CoverageRefs: []string{kind + "-coverage"}, Evidence: map[string]any{"proof_type": kind}}
	}
	valid := &CorrectionResultInput{Type: "restoration_verification", RuntimeProof: proof("runtime"), BusinessProof: proof("business"), RegressionProof: proof("regression"), RestorationProof: proof("restoration")}
	if _, err := releaseVerificationTaskRequest(route, valid); err != nil {
		t.Fatalf("distinct restoration proofs rejected: %v", err)
	}
	duplicate := *valid
	duplicate.RestorationProof = duplicate.RegressionProof
	if _, err := releaseVerificationTaskRequest(route, &duplicate); !errors.Is(err, ErrInvalidAuthoritativeState) {
		t.Fatalf("duplicate restoration proof accepted: %v", err)
	}
}

func TestKnownGoodRecoveryRequiresExactIndependentCurrentProofSet(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	releaseID, issueID, candidateID, verifierID := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}, pgtype.UUID{Bytes: [16]byte{2}, Valid: true}, pgtype.UUID{Bytes: [16]byte{3}, Valid: true}, pgtype.UUID{Bytes: [16]byte{4}, Valid: true}
	release := db.IssueWorkflowRelease{ID: releaseID, IssueID: issueID, Revision: 7, CandidateID: candidateID, CandidateRevision: 3, ScopeID: pgtype.UUID{Bytes: [16]byte{5}, Valid: true}, ScopeRevision: 2, AcceptanceVersion: pgtype.Int8{Int64: 4, Valid: true}, TargetKey: "production", TargetGeneration: 9, PlannedVerifierID: verifierID}
	proof := func(kind string) db.IssueWorkflowEvidence {
		payload, _ := json.Marshal(map[string]any{"freshness": "current", "release_id": util.UUIDToString(releaseID), "candidate_revision": 3, "scope_id": util.UUIDToString(release.ScopeID), "scope_revision": 2, "acceptance_version": 4, "target_generation": 9})
		return db.IssueWorkflowEvidence{IssueID: issueID, ConcernID: releaseID, ConcernRevision: pgtype.Int8{Int64: 7, Valid: true}, EvidenceType: kind, ProducerType: "verifier", ProducerID: verifierID, CandidateID: candidateID, TargetKey: pgtype.Text{String: "production", Valid: true}, Payload: payload}
	}
	valid := []db.IssueWorkflowEvidence{proof("runtime"), proof("business"), proof("regression")}
	if !knownGoodEvidenceComplete(release, valid, now) {
		t.Fatal("exact known-good proof set rejected")
	}
	cases := []struct {
		name string
		edit func([]db.IssueWorkflowEvidence) []db.IssueWorkflowEvidence
	}{
		{"missing regression", func(rows []db.IssueWorkflowEvidence) []db.IssueWorkflowEvidence { return rows[:2] }},
		{"wrong producer", func(rows []db.IssueWorkflowEvidence) []db.IssueWorkflowEvidence {
			rows[1].ProducerID = pgtype.UUID{Bytes: [16]byte{9}, Valid: true}
			return rows
		}},
		{"wrong candidate", func(rows []db.IssueWorkflowEvidence) []db.IssueWorkflowEvidence {
			rows[1].CandidateID = pgtype.UUID{Bytes: [16]byte{9}, Valid: true}
			return rows
		}},
		{"wrong generation", func(rows []db.IssueWorkflowEvidence) []db.IssueWorkflowEvidence {
			var payload map[string]any
			_ = json.Unmarshal(rows[1].Payload, &payload)
			payload["target_generation"] = float64(8)
			rows[1].Payload, _ = json.Marshal(payload)
			return rows
		}},
		{"expired", func(rows []db.IssueWorkflowEvidence) []db.IssueWorkflowEvidence {
			rows[1].ValidUntil = pgtype.Timestamptz{Time: now, Valid: true}
			return rows
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows := append([]db.IssueWorkflowEvidence(nil), valid...)
			if knownGoodEvidenceComplete(release, tc.edit(rows), now) {
				t.Fatal("invalid known-good proof set accepted")
			}
		})
	}
}

func TestReleaseVerificationTaskResultStrictActionDecoders(t *testing.T) {
	tests := []struct {
		name       string
		actionType string
		result     string
	}{
		{"runtime unknown field", "verify_runtime_exact_release", `{"workflow_result":{"type":"runtime_verification","runtime_proof":{"criteria_refs":["runtime"],"coverage_refs":["all"],"evidence":{"healthy":true}},"unknown":true}}`},
		{"runtime malformed nested proof", "verify_runtime_exact_release", `{"workflow_result":{"type":"runtime_verification","runtime_proof":{"criteria_refs":{"not":"an array"},"coverage_refs":["all"],"evidence":{"healthy":true}}}}`},
		{"runtime trailing document", "verify_runtime_exact_release", `{"workflow_result":{"type":"runtime_verification","runtime_proof":{"criteria_refs":["runtime"],"coverage_refs":["all"],"evidence":{"healthy":true}}}} {}`},
		{"business unknown field", "verify_business_exact_release", `{"workflow_result":{"type":"business_verification","business_proof":{"criteria_refs":["outcome"],"coverage_refs":["canary"],"evidence":{"accepted":true}},"regression_proof":{"criteria_refs":["regression"],"coverage_refs":["suite"],"evidence":{"passed":true}},"unknown":true}}`},
		{"business malformed nested proof", "verify_business_exact_release", `{"workflow_result":{"type":"business_verification","business_proof":{"criteria_refs":["outcome"],"coverage_refs":false,"evidence":{"accepted":true}},"regression_proof":{"criteria_refs":["regression"],"coverage_refs":["suite"],"evidence":{"passed":true}}}}`},
		{"business trailing document", "verify_business_exact_release", `{"workflow_result":{"type":"business_verification","business_proof":{"criteria_refs":["outcome"],"coverage_refs":["canary"],"evidence":{"accepted":true}},"regression_proof":{"criteria_refs":["regression"],"coverage_refs":["suite"],"evidence":{"passed":true}}}} {}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := decodeWorkflowTaskResult(test.actionType, []byte(test.result)); !errors.Is(err, ErrInvalidAuthoritativeState) {
				t.Fatalf("strict decoder error=%v, want invalid authoritative state", err)
			}
		})
	}
}

func TestBatchRejectsEmptyBoundaryWithoutDatabase(t *testing.T) {
	if _, err := (Service{}).ExecuteBatch(t.Context(), Principal{}, nil); err == nil {
		t.Fatal("empty batch accepted")
	}
}

func TestExecuteInWriteRejectsNilAndUnadmittedCapabilities(t *testing.T) {
	request := Request{WorkspaceID: uuid(testWorkspace), IssueID: uuid(testIssue), CommandKey: "technical-stop", Kind: workflowstatev2.CommandTechnicalStop}
	principal := Principal{ActorID: uuid("00000000-0000-0000-0000-000000000901"), ActorType: "internal_controller"}
	service := Service{ControllerID: principal.ActorID}
	for _, write := range []*workflowstore.IssueWrite{nil, new(workflowstore.IssueWrite)} {
		if _, err := service.ExecuteInWrite(t.Context(), write, principal, request); !errors.Is(err, workflowstore.ErrInvalidIssueWriteCapability) {
			t.Fatalf("nil or unadmitted write capability error=%v", err)
		}
	}
}

func TestRequestHasNoCallerControlledActorRole(t *testing.T) {
	typ := reflect.TypeOf(Request{})
	for _, name := range []string{"ActorID", "ActorRole", "Role"} {
		if _, ok := typ.FieldByName(name); ok {
			t.Fatalf("Request exposes caller-controlled %s", name)
		}
	}
}

func TestReplayRoleRequiresExactAuthenticatedPrincipalKind(t *testing.T) {
	controller := uuid("00000000-0000-0000-0000-000000000901")
	service := Service{ControllerID: controller}
	for _, tc := range []struct {
		request   Request
		principal Principal
	}{
		{Request{Kind: workflowstatev2.CommandRequestDecision}, Principal{ActorID: controller, ActorType: "member"}},
		{Request{Kind: workflowstatev2.CommandRespondDecision}, Principal{ActorID: controller, ActorType: "agent_task"}},
		{Request{Kind: workflowstatev2.CommandReviewFailed}, Principal{ActorID: controller, ActorType: "member"}},
	} {
		if _, err := service.replayRole(tc.principal, tc.request); !errors.Is(err, ErrForbidden) {
			t.Fatalf("kind=%s type=%s error=%v", tc.request.Kind, tc.principal.ActorType, err)
		}
	}
}

func TestValidateReviewEvidenceRejectsExactExpiryBoundary(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	reviewer, candidate, scope := uuid("00000000-0000-0000-0000-000000000901"), uuid("00000000-0000-0000-0000-000000000902"), uuid("00000000-0000-0000-0000-000000000903")
	payload, _ := json.Marshal(map[string]any{"candidate_revision": 1, "scope_id": "00000000-0000-0000-0000-000000000903", "scope_revision": 1, "freshness": "current"})
	row := db.IssueWorkflowEvidence{EvidenceType: "technical_review", ConcernType: "review", ProducerType: "reviewer", ProducerID: reviewer, CandidateID: candidate, Payload: payload}
	row.ValidUntil = pgtype.Timestamptz{Time: now, Valid: true}
	if err := validateReviewEvidence(row, reviewer, candidate, 1, scope, 1, now); !errors.Is(err, ErrForbidden) {
		t.Fatalf("boundary expiry error=%v", err)
	}
	row.ValidUntil.Time = now.Add(time.Nanosecond)
	if err := validateReviewEvidence(row, reviewer, candidate, 1, scope, 1, now); err != nil {
		t.Fatalf("future evidence rejected: %v", err)
	}
}

func invalidUUID() pgtype.UUID { return pgtype.UUID{} }
