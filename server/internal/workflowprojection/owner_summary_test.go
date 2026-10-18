package workflowprojection

import (
	"encoding/json"
	"testing"
	"time"
)

func TestOwnerIssueSummaryFixtureCatalog(t *testing.T) {
	tests := []struct {
		name  string
		apply func(*ownerSummaryInput)
		stage OwnerIssueStage
	}{
		{name: "minimum incoming", apply: func(*ownerSummaryInput) {}, stage: OwnerStageIncoming},
		{name: "typical owner decision", apply: applyOwnerStage(OwnerStageNeedsOwner), stage: OwnerStageNeedsOwner},
		{name: "maximum recovery with owner and dependency signals", apply: func(input *ownerSummaryInput) {
			applyOwnerStage(OwnerStageRecovering)(input)
			applyOwnerStage(OwnerStageNeedsOwner)(input)
			input.UnresolvedDependencies = []map[string]any{{"id": "execution-dependency", "revision": float64(2)}}
		}, stage: OwnerStageRecovering},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := ownerSummaryBase()
			test.apply(&input)
			summary := buildOwnerIssueSummary(input)
			if summary.Stage != test.stage || summary.SchemaVersion != 1 || summary.CurrentPictureRevision != input.Revision {
				t.Fatalf("summary = %#v, want stage %q", summary, test.stage)
			}
		})
	}
}

func TestOwnerIssueSummarySafetyPrecedencePairwise(t *testing.T) {
	priority := []OwnerIssueStage{
		OwnerStageTechnicalStop, OwnerStageRecovering, OwnerStageNeedsOwner, OwnerStageBlocked,
		OwnerStageVerifying, OwnerStageReleasing, OwnerStageInReview, OwnerStageActive,
		OwnerStageIncoming, OwnerStageComplete,
	}
	for higherIndex, higher := range priority {
		for _, lower := range priority[higherIndex+1:] {
			t.Run(string(higher)+"_over_"+string(lower), func(t *testing.T) {
				input := ownerSummaryBase()
				applyOwnerStage(lower)(&input)
				applyOwnerStage(higher)(&input)
				if got := buildOwnerIssueSummary(input).Stage; got != higher {
					t.Fatalf("stage = %q, want %q over %q", got, higher, lower)
				}
			})
		}
	}
}

func TestOwnerIssueSummaryFailsClosedForMalformedOrUnprovenProgress(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ownerSummaryInput)
	}{
		{name: "future execution", mutate: func(input *ownerSummaryInput) { input.Execution["state"] = "future" }},
		{name: "legacy deployed release", mutate: func(input *ownerSummaryInput) { input.Release["state"] = "deployed" }},
		{name: "stale finite task lease", mutate: func(input *ownerSummaryInput) {
			input.Execution = map[string]any{"id": "execution-1", "revision": float64(1), "state": "working", "task_id": "task-1", "actor_type": "worker", "actor_id": "agent-1", "lease_expires_at": "2026-09-10T12:00:00Z"}
		}},
		{name: "deployed is not live", mutate: applyOwnerStage(OwnerStageVerifying)},
		{name: "rollback does not satisfy requested outcome", mutate: func(input *ownerSummaryInput) {
			applyOwnerStage(OwnerStageRecovering)(input)
			input.Outcome["state"] = "satisfied"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := ownerSummaryBase()
			test.mutate(&input)
			summary := buildOwnerIssueSummary(input)
			switch test.name {
			case "deployed is not live":
				if summary.Stage != OwnerStageVerifying {
					t.Fatalf("stage = %q, want verifying", summary.Stage)
				}
			case "rollback does not satisfy requested outcome":
				if summary.Stage != OwnerStageRecovering {
					t.Fatalf("stage = %q, want recovering", summary.Stage)
				}
			default:
				if summary.Stage != OwnerStageStatusUnavailable {
					t.Fatalf("stage = %q, want unavailable", summary.Stage)
				}
			}
		})
	}
}

func TestOwnerIssueSummaryDocumentMarkerAndRevision(t *testing.T) {
	input := ownerSummaryBase()
	document := map[string]any{
		"contract_version": 2, "owner_summary_schema_version": 1,
		"outcome": input.Outcome, "scope": input.Scope, "execution": input.Execution,
		"release": input.Release, "health": input.Health, "attention": input.Attention,
		"unresolved_decisions": input.UnresolvedDecisions, "unresolved_incidents": input.UnresolvedIncidents,
		"unresolved_dependencies": input.UnresolvedDependencies, "next_automatic_action": input.NextAutomaticAction,
		"active_actor": input.ActiveActor, "latest_meaningful_change": input.LatestMeaningfulChange,
		"terminal_reason": input.TerminalReason,
	}
	raw, _ := json.Marshal(document)
	generatedAt := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	if got := OwnerIssueSummaryFromDocument(raw, 17, generatedAt); got.Stage != OwnerStageIncoming || got.CurrentPictureRevision != 17 {
		t.Fatalf("summary = %#v", got)
	}
	delete(document, "owner_summary_schema_version")
	raw, _ = json.Marshal(document)
	if got := OwnerIssueSummaryFromDocument(raw, 18, generatedAt); got.Stage != OwnerStageStatusUnavailable || got.CurrentPictureRevision != 18 {
		t.Fatalf("unmarked summary = %#v", got)
	}
}

func TestOwnerIssueSummaryDistinguishesDoneLiveAndRecoveryTruth(t *testing.T) {
	nonLive := ownerSummaryBase()
	applyOwnerStage(OwnerStageComplete)(&nonLive)
	nonLiveSummary := buildOwnerIssueSummary(nonLive)
	if nonLiveSummary.Stage != OwnerStageComplete || !hasOwnerSignal(nonLiveSummary, "outcome_done") || hasOwnerSignal(nonLiveSummary, "verified_live") {
		t.Fatalf("non-live completion = %#v", nonLiveSummary)
	}

	live := ownerSummaryBase()
	setReleasePhase(&live, "verified_live", "live")
	live.Outcome["state"] = "satisfied"
	live.Execution["state"] = "completed"
	live.Scope["requires_deployment"] = true
	liveSummary := buildOwnerIssueSummary(live)
	if liveSummary.Stage != OwnerStageComplete || !hasOwnerSignal(liveSummary, "verified_live") || hasOwnerSignal(liveSummary, "outcome_done") {
		t.Fatalf("live completion = %#v", liveSummary)
	}

	for _, test := range []struct {
		phase, state, signal string
	}{
		{phase: "regressed", state: "degraded", signal: "current_regression"},
		{phase: "rolling_back", state: "rolling_back", signal: "rollback_in_progress"},
		{phase: "rolled_back", state: "restored", signal: "service_restored_outcome_open"},
	} {
		input := ownerSummaryBase()
		setReleasePhase(&input, test.phase, test.state)
		input.UnresolvedIncidents = []map[string]any{{"id": "incident-1", "revision": float64(1)}}
		summary := buildOwnerIssueSummary(input)
		if summary.Stage != OwnerStageRecovering || !hasOwnerSignal(summary, test.signal) {
			t.Fatalf("%s summary = %#v", test.phase, summary)
		}
	}
}

func TestOwnerIssueSummaryRejectsMalformedAutomaticContinuation(t *testing.T) {
	for name, mutate := range map[string]func(*ownerSummaryInput){
		"invalid due time": func(input *ownerSummaryInput) { input.Execution["next_action_at"] = "not-a-time" },
		"past due time":    func(input *ownerSummaryInput) { input.Execution["next_action_at"] = "2026-09-09T00:00:00Z" },
		"empty event": func(input *ownerSummaryInput) {
			input.Execution["expected_event"] = map[string]any{}
			input.NextAutomaticAction = map[string]any{}
		},
		"unknown event": func(input *ownerSummaryInput) {
			input.Execution["expected_event"] = map[string]any{"type": "future_event"}
			input.NextAutomaticAction = map[string]any{"type": "future_event"}
		},
	} {
		t.Run(name, func(t *testing.T) {
			input := ownerSummaryBase()
			applyOwnerStage(OwnerStageActive)(&input)
			mutate(&input)
			if got := buildOwnerIssueSummary(input); got.Stage != OwnerStageStatusUnavailable || got.NextAction != nil {
				t.Fatalf("summary = %#v, want unavailable with no promise", got)
			}
		})
	}
}

func TestNormalizeOwnerIssueSummaryRejectsContradictorySignalsAndActions(t *testing.T) {
	base := OwnerIssueSummary{SchemaVersion: 1, CurrentPictureRevision: 7, Stage: OwnerStageComplete,
		Signals: []OwnerIssueSignal{{Code: "outcome_complete", References: []OwnerIssueReference{}}, {Code: "outcome_done", References: []OwnerIssueReference{}}}}
	for name, mutate := range map[string]func(*OwnerIssueSummary){
		"higher priority stop": func(summary *OwnerIssueSummary) {
			summary.Signals = append(summary.Signals, OwnerIssueSignal{Code: "technical_stop", References: []OwnerIssueReference{}})
		},
		"duplicate": func(summary *OwnerIssueSummary) { summary.Signals = append(summary.Signals, summary.Signals[0]) },
		"arbitrary action": func(summary *OwnerIssueSummary) {
			summary.Stage = OwnerStageActive
			summary.Signals = []OwnerIssueSignal{{Code: "work_active", References: []OwnerIssueReference{}}}
			summary.NextAction = &OwnerIssueNextAction{Kind: "run_prose", Automatic: true}
		},
	} {
		t.Run(name, func(t *testing.T) {
			summary := base
			summary.Signals = append([]OwnerIssueSignal(nil), base.Signals...)
			mutate(&summary)
			if got := NormalizeOwnerIssueSummary(summary, 7); got.Stage != OwnerStageStatusUnavailable {
				t.Fatalf("normalized = %#v", got)
			}
		})
	}
}

func TestOwnerMeaningfulChangeIsTypedAndNoiseCannotReplaceIt(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	records := []Record{
		fixture("outcome", "outcome-1", 1, now, map[string]any{"state": "open"}),
		fixture("attention", "attention-1", 99, now.Add(time.Hour), map[string]any{"attention_type": "awareness", "read_state": "read", "resolution_state": "resolved", "context": map[string]any{}, "next_action": map[string]any{}}),
	}
	projection, err := Project(records, 1, now.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	var document Document
	if err := json.Unmarshal(projection.Document, &document); err != nil {
		t.Fatal(err)
	}
	change := meaningfulChange(document.LatestMeaningfulChange)
	if change == nil || change.Code != "outcome_changed" || change.ResultCode != "changed" || change.Reference.ID != "outcome-1" ||
		change.OccurredAt != now.Format(time.RFC3339Nano) {
		t.Fatalf("meaningful change = %#v", change)
	}
}

func hasOwnerSignal(summary OwnerIssueSummary, code string) bool {
	for _, signal := range summary.Signals {
		if signal.Code == code {
			return true
		}
	}
	return false
}

func ownerSummaryBase() ownerSummaryInput {
	return ownerSummaryInput{
		ContractVersion: 2, Revision: 7,
		GeneratedAt: time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC),
		Outcome:     map[string]any{"id": "outcome-1", "revision": float64(1), "state": "open"},
		Scope:       map[string]any{"id": "scope-1", "revision": float64(1), "requires_deployment": false},
		Execution:   map[string]any{"id": "execution-1", "revision": float64(1), "state": "received"},
		Release:     map[string]any{"id": "release-1", "revision": float64(1), "state": "pending"},
		Health:      "unknown", Attention: map[string]any{}, UnresolvedDecisions: []map[string]any{},
		UnresolvedIncidents: []map[string]any{}, UnresolvedDependencies: []map[string]any{},
		NextAutomaticAction: map[string]any{}, ActiveActor: map[string]any{},
		LatestMeaningfulChange: map[string]any{},
	}
}

func applyOwnerStage(stage OwnerIssueStage) func(*ownerSummaryInput) {
	return func(input *ownerSummaryInput) {
		switch stage {
		case OwnerStageTechnicalStop:
			input.TerminalReason = "attempt ceiling reached"
		case OwnerStageRecovering:
			setReleasePhase(input, "regressed", "degraded")
			input.UnresolvedIncidents = []map[string]any{{"id": "incident-1", "revision": float64(1)}}
		case OwnerStageNeedsOwner:
			input.UnresolvedDecisions = []map[string]any{{"id": "decision-1", "revision": float64(1)}}
		case OwnerStageBlocked:
			input.Execution["state"] = "blocked_external"
			input.UnresolvedDependencies = []map[string]any{{"id": "execution-1", "revision": float64(1)}}
		case OwnerStageVerifying:
			setReleasePhase(input, "deployed_checking", "deployed")
		case OwnerStageReleasing:
			setReleasePhase(input, "releasing", "authorized")
		case OwnerStageInReview:
			input.Execution["state"] = "in_review"
		case OwnerStageActive:
			input.Execution = map[string]any{"id": "execution-1", "revision": float64(1), "state": "waiting_automatic", "next_action_at": "2026-09-11T00:00:00Z", "expected_event": map[string]any{"type": "provider_readback"}}
			input.NextAutomaticAction = map[string]any{"type": "provider_readback"}
		case OwnerStageIncoming:
			input.Execution["state"] = "received"
		case OwnerStageComplete:
			input.Outcome["state"] = "satisfied"
			input.Execution["state"] = "completed"
			input.Release["state"] = "not_required"
		}
	}
}

func setReleasePhase(input *ownerSummaryInput, phase, state string) {
	input.ContractVersion = 3
	input.Release = map[string]any{
		"id": "release-1", "revision": float64(2), "state": state,
		"candidate_id": "candidate-1", "candidate_revision": float64(2),
		"scope_revision": float64(1), "acceptance_version": float64(1), "authority_revision": float64(1),
		"target_key": "production", "target_generation": float64(1),
	}
	missing := ReleaseEvidenceGate{State: "missing", Refs: []string{}, Limitations: []string{}}
	passedAt := "2026-09-10T00:00:00Z"
	passed := ReleaseEvidenceGate{State: "passed", Refs: []string{"proof-1"}, ObservedAt: &passedAt, Limitations: []string{}}
	deployment := missing
	regression := missing
	rollback := missing
	restoration := missing
	review, build, exposure, runtime, business := missing, missing, missing, missing, missing
	deployedRevision := (*string)(nil)
	if phase == "deployed_checking" {
		deployment = passed
		value := "provider-revision-1"
		deployedRevision = &value
	}
	if phase == "regressed" {
		regression = ReleaseEvidenceGate{State: "contradicted", Refs: []string{"regression-1"}, Limitations: []string{}}
	}
	if phase == "verified_live" {
		review, build, deployment, exposure, runtime, business, regression = passed, passed, passed, passed, passed, passed, passed
		value := "provider-revision-1"
		deployedRevision = &value
	}
	if phase == "rolling_back" {
		input.Health = "degraded"
	}
	if phase == "rolled_back" {
		rollback, restoration = passed, passed
		value := "provider-revision-2"
		deployedRevision = &value
	}
	input.ReleaseStatus = &ReleaseStatus{
		SchemaVersion: 1, Phase: phase,
		Binding:    ReleaseBinding{ReleaseID: "release-1", ReleaseRevision: 2, CandidateID: "candidate-1", CandidateRevision: 2, SourceRevision: "source-1", ArtifactDigest: "artifact-1", BuildManifestDigest: "build-1", ScopeRevision: 1, AcceptanceVersion: 1, AuthorityRevision: 1, TargetKey: "production", TargetGeneration: 1, DeployedRevision: deployedRevision},
		Gates:      ReleaseGates{Review: review, Build: build, Deployment: deployment, Exposure: exposure, Runtime: runtime, Business: business, Regression: regression, Rollback: rollback, Restoration: restoration},
		Historical: ReleaseHistory{},
	}
	input.Health = map[string]string{"deployed_checking": "verifying", "releasing": "unknown", "regressed": "degraded", "rolling_back": "degraded", "rolled_back": "restored", "verified_live": "healthy"}[phase]
	if phase == "verified_live" || phase == "rolling_back" || phase == "rolled_back" {
		input.ReleaseStatus.Historical = ReleaseHistory{WasVerifiedLive: true, LastVerifiedLiveAt: &passedAt}
	}
}
