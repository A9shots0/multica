package workflowcommand

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	workflowstatev2 "github.com/multica-ai/multica/server/internal/workflowstate/v2"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	testWorkspace = "00000000-0000-0000-0000-000000000101"
	testIssue     = "00000000-0000-0000-0000-000000000102"
)

func fixedNow() time.Time { return time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC) }

func expected(p workflowstatev2.CurrentPicture, concerns ...workflowstatev2.Concern) []workflowstatev2.ExpectedRevision {
	out := make([]workflowstatev2.ExpectedRevision, 0, len(concerns))
	for _, concern := range concerns {
		out = append(out, workflowstatev2.ExpectedRevision{Concern: concern, Revision: p.Revisions.Get(concern)})
	}
	return out
}

func TestLoaderReconstructsAuthoritativeState(t *testing.T) {
	heads, rows := validLoaderFixture(t)
	picture, err := (Loader{Now: func() time.Time { return time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC) }}).loadRows(testWorkspace, testIssue, heads, rows)
	if err != nil {
		t.Fatal(err)
	}
	if picture.Outcome != workflowstatev2.OutcomeOpen || picture.Execution.OperationID != id(4) || picture.Execution.State != workflowstatev2.ExecutionWorking {
		t.Fatalf("unexpected core state: %+v", picture)
	}
	if picture.Scope.OwnerEvidenceRef != id(10) || picture.Authority.ScopeRevision != picture.Scope.Revision {
		t.Fatalf("scope/authority not reconstructed: %+v %+v", picture.Scope, picture.Authority)
	}
	if picture.Review.ImplementerID != id(11) || picture.Review.ReviewerID != id(12) {
		t.Fatalf("review identities = %+v", picture.Review)
	}
	if picture.Release.ReleaseOperatorID != id(13) || picture.Release.PlannedVerifierID != id(14) {
		t.Fatalf("release identities = %+v", picture.Release)
	}
	if decision := picture.Decisions[id(20)]; decision.BranchID != "secondary" || decision.Response != workflowstatev2.DecisionApprove {
		t.Fatalf("decision = %+v", decision)
	}
	if len(picture.Evidence.Runtime) != 1 || picture.Budget.AttemptLimit != 3 || picture.Budget.AttemptsUsed != 1 {
		t.Fatalf("evidence/budget = %+v %+v", picture.Evidence, picture.Budget)
	}
	if len(picture.Evidence.TechnicalReviews) != 1 || picture.Evidence.TechnicalReviews[0].Result != workflowstatev2.ReviewPassed {
		t.Fatalf("technical review evidence = %+v", picture.Evidence.TechnicalReviews)
	}
	if decision := picture.Decisions[id(20)]; decision.SuspendedState != workflowstatev2.ExecutionWorking || decision.SuspendedOperationID != id(16) {
		t.Fatalf("decision suspension = %+v", decision)
	}
}

func TestReloadedReviewEvidenceSatisfiesReducerGates(t *testing.T) {
	heads, rows := validLoaderFixture(t)
	picture, err := (Loader{Now: fixedNow}).loadRows(testWorkspace, testIssue, heads, rows)
	if err != nil {
		t.Fatal(err)
	}
	picture.Scope.DeploymentPolicy = workflowstatev2.DeploymentNotRequired
	picture.Scope.SourcePlanRevision = 1
	complete := workflowstatev2.CommandEnvelope{CommandID: "complete", IssueID: testIssue, Kind: workflowstatev2.CommandCompleteNonLive, ActorID: id(30), ActorRole: workflowstatev2.RoleController, Expected: expected(picture, workflowstatev2.ConcernOutcome, workflowstatev2.ConcernExecution, workflowstatev2.ConcernRelease)}
	if _, err := workflowstatev2.Apply(workflowstatev2.ApplyInput{Picture: picture, Command: complete}); err != nil {
		t.Fatalf("complete after reload: %v", err)
	}
	picture.Scope.DeploymentPolicy = workflowstatev2.DeploymentRequired
	picture.Release = workflowstatev2.ReleaseFacet{State: workflowstatev2.ReleasePending, CandidateID: id(15)}
	prepare := workflowstatev2.CommandEnvelope{CommandID: "prepare", IssueID: testIssue, Kind: workflowstatev2.CommandPrepareRelease, ActorID: id(30), ActorRole: workflowstatev2.RoleController, OperationID: picture.Execution.OperationID, BranchID: "primary", CandidateID: id(15), TargetID: "production", TargetGeneration: 1, AcceptanceVersion: picture.Scope.AcceptanceVersion, ReleaseOperatorID: id(13), PlannedVerifierID: id(14), AuthorityCapability: "deploy_exact_release", AuthorityEvidenceID: id(10), OwnerEvidenceRevision: 1, RecoveryPolicyRevision: 1, ExposurePolicyDigest: "sha256:exposure", BuildManifestDigest: "manifest", ArtifactDigest: "artifact", RuntimeContractDigest: "runtime", ScopeRevision: picture.Scope.Revision, AuthorityRevision: picture.Authority.Revision, Expected: expected(picture, workflowstatev2.ConcernRelease)}
	generation := workflowstatev2.Revision(1)
	prepared, err := workflowstatev2.Apply(workflowstatev2.ApplyInput{Picture: picture, Command: prepare, AuthoritativeTargetGeneration: &generation})
	if err != nil {
		t.Fatalf("prepare after reload: %v", err)
	}
	picture = prepared.Picture
	build := workflowstatev2.CommandEnvelope{CommandID: "build", IssueID: testIssue, Kind: workflowstatev2.CommandBuildRelease, ActorID: id(13), ActorRole: workflowstatev2.RoleRelease, CandidateID: id(15), TargetID: "production", TargetGeneration: 1, EvidenceRefs: []string{"build-receipt"}, BuildManifestDigest: "manifest", ArtifactDigest: "artifact", RuntimeContractDigest: "runtime", Expected: expected(picture, workflowstatev2.ConcernRelease, workflowstatev2.ConcernEvidence)}
	if _, err := workflowstatev2.Apply(workflowstatev2.ApplyInput{Picture: picture, Command: build, AuthoritativeTargetGeneration: &generation}); err != nil {
		t.Fatalf("build after reload: %v", err)
	}
}

func TestReloadedAuthorizationDecisionWaitsForDurableEffect(t *testing.T) {
	for _, tc := range []struct {
		name        string
		outcome     workflowstatev2.DecisionOutcome
		wantSettled workflowstatev2.ExecutionState
	}{
		{"approve", workflowstatev2.DecisionApprove, workflowstatev2.ExecutionWorking},
		{"reject", workflowstatev2.DecisionReject, workflowstatev2.ExecutionQueued},
	} {
		t.Run(tc.name, func(t *testing.T) {
			heads, rows := validLoaderFixture(t)
			setPayloadField(t, &rows[3], "state", "paused")
			heads[3].State = "paused"
			setPayloadField(t, &rows[6], "decision_id", id(20))
			rows[11] = row(t, "decision", 20, map[string]any{"scope_id": id(2), "scope_revision": 1, "authority_id": id(3), "authority_revision": 1, "candidate_id": id(15), "target_key": "production", "decision_type": "authorization", "state": "unanswered", "resolution_state": "open", "question": "Proceed?", "requester_id": id(11), "accountable_owner_id": id(10), "suspended_operation_id": id(16), "suspended_execution_state": "working", "options": []any{"approve", "reject"}, "response_schema": map[string]any{"type": "authorization"}, "authority_gap": map[string]any{"kind": "operation_authorization", "requested_capability": "deploy", "reason": "owner authorization required", "candidate_revision": 1, "branch_key": "secondary", "suspended_execution_revision": 1}})
			rows = append(rows[:12], rows[13:]...)
			picture, err := (Loader{Now: fixedNow}).loadRows(testWorkspace, testIssue, heads, rows)
			if err != nil {
				t.Fatal(err)
			}
			decision := picture.Decisions[id(20)]
			cmd := workflowstatev2.CommandEnvelope{CommandID: "respond-" + tc.name, IssueID: testIssue, Kind: workflowstatev2.CommandRespondDecision, ActorID: id(10), ActorRole: workflowstatev2.RoleOwner, DecisionID: decision.ID, ExpectedDecisionRevision: decision.Revision, DecisionType: decision.Type, DecisionOutcome: tc.outcome, ScopeRevision: decision.ScopeRevision, AuthorityRevision: decision.AuthorityRevision, CandidateID: decision.CandidateID, TargetID: decision.TargetID, BranchID: decision.BranchID, OperationID: decision.SuspendedOperationID, Expected: expected(picture, workflowstatev2.ConcernAttention, workflowstatev2.ConcernExecution)}
			result, err := workflowstatev2.Apply(workflowstatev2.ApplyInput{Picture: picture, Command: cmd})
			if err != nil {
				t.Fatal(err)
			}
			if result.Picture.Execution.State != workflowstatev2.ExecutionPaused || result.Picture.Attention.Resolution != workflowstatev2.ResolutionResuming {
				t.Fatalf("result = %+v", result.Picture)
			}
			settle := workflowstatev2.CommandEnvelope{CommandID: "settle-" + tc.name, IssueID: testIssue, Kind: workflowstatev2.CommandSettleDecisionEffect, ActorID: id(30), ActorRole: workflowstatev2.RoleController, DecisionID: decision.ID, ExpectedDecisionRevision: 2, DecisionType: decision.Type, DecisionOutcome: tc.outcome, ScopeRevision: decision.ScopeRevision, AuthorityRevision: decision.AuthorityRevision, CandidateID: decision.CandidateID, TargetID: decision.TargetID, BranchID: decision.BranchID, OperationID: decision.SuspendedOperationID, SuspendedState: decision.SuspendedState, Expected: expected(result.Picture, workflowstatev2.ConcernAttention, workflowstatev2.ConcernExecution)}
			settled, err := workflowstatev2.Apply(workflowstatev2.ApplyInput{Picture: result.Picture, Command: settle})
			if err != nil || settled.Picture.Execution.State != tc.wantSettled || settled.Picture.Outcome != workflowstatev2.OutcomeOpen {
				t.Fatalf("settle result=%+v err=%v", settled.Picture, err)
			}
		})
	}
}

func TestLoaderRetainsParallelEvidenceWithoutGrantingCurrentAuthority(t *testing.T) {
	heads, rows := validLoaderFixture(t)
	rows = append(rows,
		row(t, "candidate", 40, map[string]any{"scope_id": id(2), "scope_revision": 1, "candidate_type": "source", "state": "ready", "artifact": map[string]any{}, "build_contract": map[string]any{}, "created_by": id(41)}),
		row(t, "release", 42, map[string]any{"candidate_id": id(40), "candidate_revision": 1, "scope_id": id(2), "scope_revision": 1, "authority_id": id(3), "authority_revision": 1, "target_key": "staging", "target_generation": 4, "state": "deployed", "facets": map[string]any{"release_operator_id": id(43), "planned_verifier_id": id(44)}, "recovery_policy": map[string]any{}, "exposed_population": map[string]any{}}),
		row(t, "evidence", 45, map[string]any{"concern_type": "release", "evidence_type": "runtime", "level": "runtime", "source": "parallel", "producer_type": "verifier", "producer_id": id(44), "candidate_id": id(40), "target_key": "staging", "observed_at": "2026-09-10T11:00:00Z", "coverage": map[string]any{}, "limitations": []any{}, "contradictions": []any{}, "payload": map[string]any{"freshness": "current", "candidate_revision": 1, "release_id": id(42), "target_generation": 4, "acceptance_version": 1, "scope_id": id(2), "scope_revision": 1}}),
	)
	picture, err := (Loader{Now: fixedNow}).loadRows(testWorkspace, testIssue, heads, rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(picture.Evidence.Runtime) != 2 || picture.Evidence.Runtime[1].CandidateID != id(40) {
		t.Fatalf("parallel evidence = %+v", picture.Evidence.Runtime)
	}
}

func TestDecisionAcceptsOneExactParallelReleaseAttemptWithAbsentHead(t *testing.T) {
	heads, rows := validLoaderFixture(t)
	setPayloadField(t, &rows[2], "envelope", map[string]any{})
	setPayloadField(t, &rows[11], "candidate_id", id(40))
	setPayloadField(t, &rows[11], "target_key", "staging")
	rows = append(rows,
		row(t, "candidate", 40, map[string]any{"scope_id": id(2), "scope_revision": 1, "candidate_type": "source", "state": "ready", "artifact": map[string]any{}, "build_contract": map[string]any{}, "created_by": id(41)}),
		row(t, "release", 42, map[string]any{"candidate_id": id(40), "candidate_revision": 1, "scope_id": id(2), "scope_revision": 1, "authority_id": id(3), "authority_revision": 1, "target_key": "staging", "target_generation": 1, "expected_target_generation": 0, "state": "pending", "facets": map[string]any{}, "recovery_policy": map[string]any{}, "exposed_population": map[string]any{}}),
	)
	picture, err := (Loader{Now: fixedNow}).loadRows(testWorkspace, testIssue, heads, rows)
	if err != nil {
		t.Fatal(err)
	}
	decision := picture.Decisions[id(20)]
	if decision.CandidateID != id(40) || decision.TargetID != "staging" || decision.BranchID != "secondary" {
		t.Fatalf("decision = %+v", decision)
	}
}

func TestReleaseProofAlwaysRequiresExactPlannedVerifier(t *testing.T) {
	for _, releaseState := range []string{"pending", "failed"} {
		for _, evidenceType := range []string{"runtime", "business", "restoration"} {
			t.Run(releaseState+"/"+evidenceType, func(t *testing.T) {
				heads, rows := validLoaderFixture(t)
				heads[5].State = releaseState
				setPayloadField(t, &rows[5], "state", releaseState)
				setPayloadField(t, &rows[5], "facets", map[string]any{"release_operator_id": id(13)})
				setPayloadField(t, &rows[7], "evidence_type", evidenceType)
				setPayloadField(t, &rows[7], "level", evidenceType)
				setPayloadField(t, &rows[13], "state", releaseState)
				_, err := (Loader{Now: fixedNow}).loadRows(testWorkspace, testIssue, heads, rows)
				if !errors.Is(err, ErrInvalidAuthoritativeState) {
					t.Fatalf("error = %v", err)
				}
			})
		}
	}
}

func TestLoaderFailsClosedOnInvalidLedgers(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		mutate func([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow)
	}{
		{"missing head", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			return h[:8], r
		}},
		{"duplicate head", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			return append(h, h[0]), r
		}},
		{"cross tenant head", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			h[0].WorkspaceID = uuid(id(99))
			return h, r
		}},
		{"missing source", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			h[0].SourceRecordID = uuid(id(99))
			return h, r
		}},
		{"head state mismatch", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			h[0].State = "satisfied"
			return h, r
		}},
		{"malformed envelope", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			r[0].Payload = []byte(`null`)
			return h, r
		}},
		{"expired authority", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			setPayloadField(t, &r[2], "valid_until", now.Add(-time.Second).Format(time.RFC3339Nano))
			return h, r
		}},
		{"revoked authority", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			h[2].State = "revoked"
			setPayloadField(t, &r[2], "state", "revoked")
			return h, r
		}},
		{"expired evidence", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			setPayloadField(t, &r[7], "valid_until", now.Format(time.RFC3339Nano))
			return h, r
		}},
		{"execution scope revision drift", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			setPayloadField(t, &r[3], "scope_revision", 2)
			return h, r
		}},
		{"review candidate revision drift", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			setPayloadField(t, &r[4], "candidate_revision", 2)
			return h, r
		}},
		{"review unresolved later evidence ref", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			setPayloadField(t, &r[4], "evidence_refs", []any{id(18), id(99)})
			return h, r
		}},
		{"review later evidence wrong producer", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			r = append(r, row(t, "evidence", 19, map[string]any{"concern_type": "review", "evidence_type": "technical_review", "level": "technical", "source": "review-task", "producer_type": "reviewer", "producer_id": id(99), "candidate_id": id(15), "observed_at": "2026-09-10T11:00:00Z", "coverage": map[string]any{}, "limitations": []any{}, "contradictions": []any{}, "payload": map[string]any{"freshness": "current", "candidate_revision": 1, "scope_id": id(2), "scope_revision": 1}}))
			setPayloadField(t, &r[4], "evidence_refs", []any{id(18), id(19)})
			return h, r
		}},
		{"release authority revision drift", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			setPayloadField(t, &r[5], "authority_revision", 2)
			return h, r
		}},
		{"release verifier is operator", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			setPayloadField(t, &r[5], "facets", map[string]any{"release_operator_id": id(13), "planned_verifier_id": id(13)})
			return h, r
		}},
		{"evidence release drift", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			var fields map[string]any
			if err := json.Unmarshal(r[7].Payload, &fields); err != nil {
				t.Fatal(err)
			}
			fields["payload"].(map[string]any)["release_id"] = id(99)
			r[7].Payload, _ = json.Marshal(fields)
			return h, r
		}},
		{"expired unanswered decision", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			setPayloadField(t, &r[11], "state", "unanswered")
			setPayloadField(t, &r[11], "resolution_state", "open")
			setPayloadField(t, &r[11], "expires_at", now.Format(time.RFC3339Nano))
			return h, r
		}},
		{"candidate scope revision drift", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			setPayloadField(t, &r[9], "scope_revision", 2)
			return h, r
		}},
		{"decision response wrong owner", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			setPayloadField(t, &r[12], "actor_id", id(99))
			return h, r
		}},
		{"decision response disallowed outcome", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			setPayloadField(t, &r[11], "decision_type", "factual_input")
			return h, r
		}},
		{"decision unknown candidate", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			setPayloadField(t, &r[11], "candidate_id", id(99))
			return h, r
		}},
		{"decision candidate scope mismatch", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			r = append(r,
				row(t, "scope", 50, map[string]any{"state": "accepted", "acceptance_version": 1, "owner_evidence_id": id(10), "created_by": id(10), "statement": map[string]any{}, "impact": map[string]any{}}),
				row(t, "candidate", 51, map[string]any{"scope_id": id(50), "scope_revision": 1, "candidate_type": "source", "state": "ready", "artifact": map[string]any{}, "build_contract": map[string]any{}, "created_by": id(11)}),
			)
			setPayloadField(t, &r[11], "candidate_id", id(51))
			return h, r
		}},
		{"decision target mismatch", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			setPayloadField(t, &r[11], "candidate_id", id(15))
			setPayloadField(t, &r[11], "target_key", "staging")
			return h, r
		}},
		{"decision missing applicable release", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			setPayloadField(t, &r[2], "envelope", map[string]any{})
			setPayloadField(t, &r[11], "candidate_id", id(40))
			setPayloadField(t, &r[11], "target_key", "staging")
			r = append(r, row(t, "candidate", 40, map[string]any{"scope_id": id(2), "scope_revision": 1, "candidate_type": "source", "state": "ready", "artifact": map[string]any{}, "build_contract": map[string]any{}, "created_by": id(41)}))
			return h, r
		}},
		{"decision duplicate applicable releases", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			r = append(r, row(t, "release", 60, map[string]any{"candidate_id": id(15), "candidate_revision": 1, "scope_id": id(2), "scope_revision": 1, "authority_id": id(3), "authority_revision": 1, "target_key": "production", "target_generation": 2, "state": "pending", "facets": map[string]any{}, "recovery_policy": map[string]any{}, "exposed_population": map[string]any{}}))
			return h, r
		}},
		{"decision missing target head", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			setPayloadField(t, &r[2], "envelope", map[string]any{})
			setPayloadField(t, &r[11], "candidate_id", id(40))
			setPayloadField(t, &r[11], "target_key", "staging")
			r = append(r,
				row(t, "candidate", 40, map[string]any{"scope_id": id(2), "scope_revision": 1, "candidate_type": "source", "state": "ready", "artifact": map[string]any{}, "build_contract": map[string]any{}, "created_by": id(41)}),
				row(t, "release", 42, map[string]any{"candidate_id": id(40), "candidate_revision": 1, "scope_id": id(2), "scope_revision": 1, "authority_id": id(3), "authority_revision": 1, "target_key": "staging", "target_generation": 4, "state": "pending", "facets": map[string]any{}, "recovery_policy": map[string]any{}, "exposed_population": map[string]any{}}),
			)
			return h, r
		}},
		{"release missing planned verifier", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			setPayloadField(t, &r[5], "facets", map[string]any{"release_operator_id": id(13)})
			return h, r
		}},
		{"evidence wrong producer", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			setPayloadField(t, &r[7], "producer_type", "worker")
			return h, r
		}},
		{"evidence wrong verifier identity", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			setPayloadField(t, &r[7], "producer_id", id(99))
			return h, r
		}},
		{"target head state drift", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			setPayloadField(t, &r[13], "state", "live")
			return h, r
		}},
		{"wrong object shape", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			setPayloadField(t, &r[1], "statement", []any{})
			return h, r
		}},
		{"wrong array shape", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			setPayloadField(t, &r[4], "criteria", map[string]any{})
			return h, r
		}},
		{"wrong boolean shape", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			setPayloadField(t, &r[6], "acknowledged", "false")
			return h, r
		}},
		{"fractional integer", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			setPayloadField(t, &r[8], "limits", map[string]any{"attempts": 1.5})
			return h, r
		}},
		{"wrong nested shape", func(h []db.IssueWorkflowConcernHead, r []db.ListIssueWorkflowProjectionInputsRow) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
			setPayloadField(t, &r[5], "facets", map[string]any{"release_operator_id": map[string]any{}, "planned_verifier_id": id(14)})
			return h, r
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, r := validLoaderFixture(t)
			h, r = tt.mutate(h, r)
			_, err := (Loader{Now: func() time.Time { return now }}).loadRows(testWorkspace, testIssue, h, r)
			if !errors.Is(err, ErrInvalidAuthoritativeState) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestLoaderSeparatesReleaseAttemptFromObservedTargetHead(t *testing.T) {
	baseHeads, baseRows := validLoaderFixture(t)
	setPayloadField(t, &baseRows[5], "state", "authorized")
	setPayloadField(t, &baseRows[5], "head_observed_at", nil)
	setPayloadField(t, &baseRows[5], "target_generation", 2)
	setPayloadField(t, &baseRows[5], "expected_target_generation", 1)
	setPayloadField(t, &baseRows[5], "expected_target_release_id", id(60))
	setPayloadField(t, &baseRows[5], "release_kind", "forward")
	setNestedPayloadField(t, &baseRows[7], "payload", "target_generation", 2)
	baseHeads[5].State = "authorized"
	baseRows[13] = targetRow(t, 60, map[string]any{"workspace_id": testWorkspace, "target_key": "production", "revision": 1, "generation": 1, "release_id": id(60), "issue_id": id(61), "candidate_id": id(62), "state": "live"})

	if _, err := (Loader{Now: fixedNow}).loadRows(testWorkspace, testIssue, baseHeads, baseRows); err != nil {
		t.Fatalf("valid proposed attempt: %v", err)
	}

	tests := []struct {
		name   string
		mutate func([]db.ListIssueWorkflowProjectionInputsRow)
	}{
		{"missing predecessor generation", func(rows []db.ListIssueWorkflowProjectionInputsRow) {
			setPayloadField(t, &rows[5], "expected_target_generation", nil)
		}},
		{"predecessor generation drift", func(rows []db.ListIssueWorkflowProjectionInputsRow) {
			setPayloadField(t, &rows[5], "expected_target_generation", 9)
		}},
		{"predecessor release drift", func(rows []db.ListIssueWorkflowProjectionInputsRow) {
			setPayloadField(t, &rows[5], "expected_target_release_id", id(63))
		}},
		{"premature head observation", func(rows []db.ListIssueWorkflowProjectionInputsRow) {
			setPayloadField(t, &rows[5], "head_observed_at", "2026-09-10T11:00:00Z")
		}},
		{"nonadvancing generation", func(rows []db.ListIssueWorkflowProjectionInputsRow) {
			setPayloadField(t, &rows[5], "target_generation", 1)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rows := cloneRows(baseRows)
			tc.mutate(rows)
			if _, err := (Loader{Now: fixedNow}).loadRows(testWorkspace, testIssue, baseHeads, rows); !errors.Is(err, ErrInvalidAuthoritativeState) {
				t.Fatalf("error = %v, want invalid authoritative state", err)
			}
		})
	}
}

func TestLoaderRequiresRecoveryAsNewGenerationFromExactHead(t *testing.T) {
	heads, rows := validLoaderFixture(t)
	setPayloadField(t, &rows[5], "state", "authorized")
	setPayloadField(t, &rows[5], "head_observed_at", nil)
	setPayloadField(t, &rows[5], "target_generation", 2)
	setPayloadField(t, &rows[5], "expected_target_generation", 1)
	setPayloadField(t, &rows[5], "expected_target_release_id", id(60))
	setPayloadField(t, &rows[5], "release_kind", "recovery")
	setPayloadField(t, &rows[5], "recovery_of_release_id", id(60))
	setNestedPayloadField(t, &rows[7], "payload", "target_generation", 2)
	heads[5].State = "authorized"
	rows[13] = targetRow(t, 60, map[string]any{"workspace_id": testWorkspace, "target_key": "production", "revision": 1, "generation": 1, "release_id": id(60), "issue_id": id(61), "candidate_id": id(62), "state": "degraded"})
	if _, err := (Loader{Now: fixedNow}).loadRows(testWorkspace, testIssue, heads, rows); err != nil {
		t.Fatalf("valid recovery generation: %v", err)
	}
	for _, tc := range []struct{ name, recoveryID string }{{"missing recovery release", ""}, {"wrong recovery release", id(63)}} {
		t.Run(tc.name, func(t *testing.T) {
			mutated := cloneRows(rows)
			setPayloadField(t, &mutated[5], "recovery_of_release_id", tc.recoveryID)
			if _, err := (Loader{Now: fixedNow}).loadRows(testWorkspace, testIssue, heads, mutated); !errors.Is(err, ErrInvalidAuthoritativeState) {
				t.Fatalf("error = %v, want invalid authoritative state", err)
			}
		})
	}
}

func TestLoaderTerminalReleaseRequiresExactCurrentProofSet(t *testing.T) {
	proof := func(t *testing.T, typ string, n int) db.ListIssueWorkflowProjectionInputsRow {
		t.Helper()
		return row(t, "evidence", n, map[string]any{"concern_type": "release", "evidence_type": typ, "level": typ, "source": "normalized-receipt", "producer_type": "verifier", "producer_id": id(14), "candidate_id": id(15), "target_key": "production", "observed_at": "2026-09-10T11:00:00Z", "coverage": map[string]any{}, "limitations": []any{}, "contradictions": []any{}, "payload": map[string]any{"freshness": "current", "candidate_revision": 1, "release_id": id(6), "target_generation": 1, "acceptance_version": 1, "scope_id": id(2), "scope_revision": 1}})
	}
	for _, terminal := range []string{"live", "restored"} {
		t.Run(terminal, func(t *testing.T) {
			heads, rows := validLoaderFixture(t)
			heads[5].State = terminal
			setPayloadField(t, &rows[5], "state", terminal)
			for key, value := range map[string]any{
				"attempt_contract_version": 2, "acceptance_version": 1, "operation_id": id(4), "branch_key": "main", "expected_target_generation": 0,
				"release_operator_id": id(13), "planned_verifier_id": id(14), "build_manifest_digest": "build", "artifact_digest": "artifact",
				"runtime_contract_digest": "runtime", "authority_capability": "deploy", "authority_evidence_id": id(10), "recovery_policy_revision": 1,
			} {
				setPayloadField(t, &rows[5], key, value)
			}
			setPayloadField(t, &rows[5], "facets", map[string]any{})
			setPayloadField(t, &rows[13], "state", terminal)
			rows = append(rows, proof(t, "business", 70), proof(t, "regression", 71))
			if terminal == "restored" {
				rows = append(rows, proof(t, "restoration", 72))
			}
			if _, err := (Loader{Now: fixedNow}).loadRows(testWorkspace, testIssue, heads, rows); err != nil {
				t.Fatalf("complete proof set rejected: %v", err)
			}
			wrongProducer := cloneRows(rows)
			setPayloadField(t, &wrongProducer[7], "producer_id", id(99))
			if _, err := (Loader{Now: fixedNow}).loadRows(testWorkspace, testIssue, heads, wrongProducer); !errors.Is(err, ErrInvalidAuthoritativeState) {
				t.Fatalf("wrong frozen verifier error = %v", err)
			}
			for _, omitted := range []string{"runtime", "business", "regression"} {
				t.Run("missing_"+omitted, func(t *testing.T) {
					filtered := make([]db.ListIssueWorkflowProjectionInputsRow, 0, len(rows))
					for _, evidence := range rows {
						var fields map[string]any
						_ = json.Unmarshal(evidence.Payload, &fields)
						if evidence.RecordType != "evidence" || fields["evidence_type"] != omitted {
							filtered = append(filtered, evidence)
						}
					}
					_, err := (Loader{Now: fixedNow}).loadRows(testWorkspace, testIssue, heads, filtered)
					if !errors.Is(err, ErrInvalidAuthoritativeState) {
						t.Fatalf("missing %s error = %v", omitted, err)
					}
				})
			}
		})
	}
}

func TestLoaderValidatesExactIncidentRelations(t *testing.T) {
	heads, rows := validLoaderFixture(t)
	exact := row(t, "incident", 80, map[string]any{"release_id": id(6), "release_revision": 1, "candidate_id": id(15), "candidate_revision": 1, "target_key": "production", "target_generation": 1, "detection_evidence_id": id(8), "incident_type": "regression", "severity": "warning", "state": "open", "consequence": map[string]any{}, "recovery": map[string]any{}})
	if _, err := (Loader{Now: fixedNow}).loadRows(testWorkspace, testIssue, heads, append(rows, exact)); err != nil {
		t.Fatalf("exact incident rejected: %v", err)
	}
	for _, tc := range []struct {
		name  string
		field string
		value any
	}{{"partial", "candidate_revision", nil}, {"wrong generation", "target_generation", 2}, {"wrong candidate", "candidate_id", id(40)}, {"missing evidence", "detection_evidence_id", id(99)}} {
		t.Run(tc.name, func(t *testing.T) {
			bad := exact
			setPayloadField(t, &bad, tc.field, tc.value)
			_, err := (Loader{Now: fixedNow}).loadRows(testWorkspace, testIssue, heads, append(rows, bad))
			if !errors.Is(err, ErrInvalidAuthoritativeState) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestLoaderRejectsIncompleteV2ReleaseAttemptSnapshot(t *testing.T) {
	heads, rows := validLoaderFixture(t)
	complete := map[string]any{
		"attempt_contract_version": 2, "acceptance_version": 1, "operation_id": id(4), "branch_key": "main",
		"expected_target_generation": 0, "release_operator_id": id(13), "planned_verifier_id": id(14),
		"build_manifest_digest": "build", "artifact_digest": "artifact", "runtime_contract_digest": "runtime",
		"authority_capability": "deploy", "authority_evidence_id": id(10), "recovery_policy_revision": 1,
	}
	for key, value := range complete {
		setPayloadField(t, &rows[5], key, value)
	}
	if _, err := (Loader{Now: fixedNow}).loadRows(testWorkspace, testIssue, heads, rows); err != nil {
		t.Fatalf("complete v2 attempt rejected: %v", err)
	}
	for _, missing := range []string{"acceptance_version", "operation_id", "branch_key", "expected_target_generation", "release_operator_id", "planned_verifier_id", "build_manifest_digest", "artifact_digest", "runtime_contract_digest", "authority_capability", "authority_evidence_id", "recovery_policy_revision"} {
		t.Run(missing, func(t *testing.T) {
			_, bad := validLoaderFixture(t)
			for key, value := range complete {
				setPayloadField(t, &bad[5], key, value)
			}
			setPayloadField(t, &bad[5], missing, nil)
			_, err := (Loader{Now: fixedNow}).loadRows(testWorkspace, testIssue, heads, bad)
			if !errors.Is(err, ErrInvalidAuthoritativeState) {
				t.Fatalf("missing %s error = %v", missing, err)
			}
		})
	}
}

func cloneRows(rows []db.ListIssueWorkflowProjectionInputsRow) []db.ListIssueWorkflowProjectionInputsRow {
	cloned := append([]db.ListIssueWorkflowProjectionInputsRow(nil), rows...)
	for i := range cloned {
		cloned[i].Payload = append([]byte(nil), rows[i].Payload...)
	}
	return cloned
}

func validLoaderFixture(t *testing.T) ([]db.IssueWorkflowConcernHead, []db.ListIssueWorkflowProjectionInputsRow) {
	t.Helper()
	concerns := []struct {
		concern, typ, state string
		n                   int
	}{
		{"outcome", "outcome", "open", 1}, {"scope", "scope", "accepted", 2}, {"authority", "authority", "active", 3},
		{"execution", "execution", "working", 4}, {"review", "review", "passed", 5}, {"release", "release", "deployed", 6},
		{"attention", "attention", "open", 7}, {"evidence", "evidence", "current", 8}, {"budget", "budget", "active", 9},
	}
	heads := make([]db.IssueWorkflowConcernHead, 0, 9)
	rows := make([]db.ListIssueWorkflowProjectionInputsRow, 0, 14)
	for _, c := range concerns {
		heads = append(heads, db.IssueWorkflowConcernHead{WorkspaceID: uuid(testWorkspace), IssueID: uuid(testIssue), ConcernType: c.concern, Revision: 1, SourceRecordID: uuid(id(c.n)), SourceRecordRevision: 1, State: c.state})
	}
	rows = append(rows,
		row(t, "outcome", 1, map[string]any{"state": "open", "acceptance_version": 1}),
		row(t, "scope", 2, map[string]any{"state": "accepted", "acceptance_version": 1, "owner_evidence_id": id(10), "created_by": id(10), "statement": map[string]any{}, "impact": map[string]any{}}),
		row(t, "authority", 3, map[string]any{"state": "active", "scope_id": id(2), "scope_revision": 1, "owner_evidence_id": id(10), "created_by": id(10), "envelope": map[string]any{"target_id": "production", "candidate_id": id(15)}}),
		row(t, "execution", 4, map[string]any{"branch_key": "main", "operation_type": "implementation", "state": "working", "scope_id": id(2), "scope_revision": 1, "authority_id": id(3), "authority_revision": 1, "actor_id": id(11), "lease_token": id(30), "checkpoint": map[string]any{"id": "cp-1"}, "expected_event": map[string]any{"type": "task.completed", "next_action": "review"}, "wait_reason": map[string]any{}, "attempt_count": 1}),
		row(t, "review", 5, map[string]any{"candidate_id": id(15), "candidate_revision": 1, "scope_id": id(2), "scope_revision": 1, "reviewer_id": id(12), "outcome": "passed", "criteria": []any{"acceptance"}, "coverage": map[string]any{"tests": true}, "evidence_refs": []any{id(18)}, "gaps": []any{}, "correction_packet": map[string]any{}, "correction_count": 0, "max_corrections": 0}),
		row(t, "release", 6, map[string]any{"candidate_id": id(15), "candidate_revision": 1, "scope_id": id(2), "scope_revision": 1, "authority_id": id(3), "authority_revision": 1, "target_key": "production", "target_generation": 1, "state": "deployed", "head_observed_at": "2026-09-10T11:00:00Z", "facets": map[string]any{"release_operator_id": id(13), "planned_verifier_id": id(14)}, "recovery_policy": map[string]any{}, "exposed_population": map[string]any{}}),
		row(t, "attention", 7, map[string]any{"attention_type": "decision", "read_state": "unread", "acknowledged": false, "resolution_state": "open", "context": map[string]any{}, "next_action": map[string]any{}}),
		row(t, "evidence", 8, map[string]any{"concern_type": "release", "evidence_type": "runtime", "level": "runtime", "source": "test", "producer_type": "verifier", "producer_id": id(14), "candidate_id": id(15), "target_key": "production", "observed_at": "2026-09-10T11:00:00Z", "coverage": map[string]any{}, "limitations": []any{}, "contradictions": []any{}, "payload": map[string]any{"freshness": "current", "candidate_revision": 1, "release_id": id(6), "target_generation": 1, "acceptance_version": 1, "scope_id": id(2), "scope_revision": 1}}),
		row(t, "budget", 9, map[string]any{"scope_id": id(2), "concern_type": "execution", "state": "active", "limits": map[string]any{"attempts": 3, "runtime": 100, "tokens": 200, "cost": 300}, "consumption": map[string]any{"attempts": 1, "runtime": 10, "tokens": 20, "cost": 30}}),
		row(t, "candidate", 15, map[string]any{"scope_id": id(2), "scope_revision": 1, "candidate_type": "source", "state": "ready", "artifact": map[string]any{}, "build_contract": map[string]any{}, "created_by": id(11)}),
		row(t, "execution", 16, map[string]any{"branch_key": "secondary", "operation_type": "audit", "state": "paused", "scope_id": id(2), "scope_revision": 1, "checkpoint": map[string]any{}, "expected_event": map[string]any{}, "wait_reason": map[string]any{}, "attempt_count": 0}),
		rowRevision(t, "decision", 20, 2, map[string]any{"scope_id": id(2), "scope_revision": 1, "authority_id": id(3), "authority_revision": 1, "candidate_id": id(15), "target_key": "production", "decision_type": "approval", "state": "answered", "resolution_state": "resuming", "question": "Proceed?", "requester_id": id(11), "accountable_owner_id": id(10), "suspended_operation_id": id(16), "suspended_execution_state": "working", "options": []any{}, "response_schema": map[string]any{}, "authority_gap": map[string]any{}}),
		row(t, "decision_response", 21, map[string]any{"decision_id": id(20), "decision_revision": 1, "response_type": "approve", "actor_id": id(10), "idempotency_key": "response-1", "request_hash": "hash", "selected_options": []any{}, "content": map[string]any{"text": "yes"}, "scope_change": map[string]any{}}),
		targetRow(t, 6, map[string]any{"workspace_id": testWorkspace, "target_key": "production", "revision": 1, "generation": 1, "release_id": id(6), "issue_id": testIssue, "candidate_id": id(15), "state": "deployed"}),
		row(t, "evidence", 18, map[string]any{"concern_type": "review", "evidence_type": "technical_review", "level": "technical", "source": "review-task", "producer_type": "reviewer", "producer_id": id(12), "candidate_id": id(15), "observed_at": "2026-09-10T11:00:00Z", "coverage": map[string]any{}, "limitations": []any{}, "contradictions": []any{}, "payload": map[string]any{"freshness": "current", "candidate_revision": 1, "scope_id": id(2), "scope_revision": 1}}),
	)
	return heads, rows
}

func row(t *testing.T, typ string, n int, fields map[string]any) db.ListIssueWorkflowProjectionInputsRow {
	return rowRevision(t, typ, n, 1, fields)
}
func rowRevision(t *testing.T, typ string, n int, revision int64, fields map[string]any) db.ListIssueWorkflowProjectionInputsRow {
	t.Helper()
	fields["id"] = id(n)
	fields["workspace_id"] = testWorkspace
	fields["issue_id"] = testIssue
	fields["revision"] = revision
	payload, _ := json.Marshal(fields)
	return db.ListIssueWorkflowProjectionInputsRow{RecordType: typ, ID: uuid(id(n)), Revision: revision, Payload: payload}
}
func targetRow(t *testing.T, n int, fields map[string]any) db.ListIssueWorkflowProjectionInputsRow {
	t.Helper()
	payload, _ := json.Marshal(fields)
	return db.ListIssueWorkflowProjectionInputsRow{RecordType: "target_head", ID: uuid(id(n)), Revision: 1, Payload: payload}
}
func setPayloadField(t *testing.T, row *db.ListIssueWorkflowProjectionInputsRow, key string, value any) {
	t.Helper()
	var fields map[string]any
	if err := json.Unmarshal(row.Payload, &fields); err != nil {
		t.Fatal(err)
	}
	fields[key] = value
	row.Payload, _ = json.Marshal(fields)
}

func setNestedPayloadField(t *testing.T, row *db.ListIssueWorkflowProjectionInputsRow, objectKey, key string, value any) {
	t.Helper()
	var fields map[string]any
	if err := json.Unmarshal(row.Payload, &fields); err != nil {
		t.Fatal(err)
	}
	object, ok := fields[objectKey].(map[string]any)
	if !ok {
		t.Fatalf("%s is not an object", objectKey)
	}
	object[key] = value
	row.Payload, _ = json.Marshal(fields)
}
func id(n int) string { return "00000000-0000-0000-0000-" + fmt12(n) }
func fmt12(n int) string {
	return "000000000" + string(rune('0'+n/100)) + string(rune('0'+(n/10)%10)) + string(rune('0'+n%10))
}
func uuid(s string) pgtype.UUID {
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		panic(err)
	}
	return u
}
