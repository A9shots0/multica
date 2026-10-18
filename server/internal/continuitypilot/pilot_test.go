package continuitypilot

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestGenerateIsDeterministicAndCounterbalanced(t *testing.T) {
	config := testConfig("codex")
	first, err := Generate(config)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Generate(config)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("same input produced different plans")
	}
	provider := first.Providers[0]
	if len(provider.Pairs) != MaximumPairs {
		t.Fatalf("pairs = %d", len(provider.Pairs))
	}
	controlFirst, treatmentFirst := 0, 0
	for _, pair := range provider.Pairs {
		if pair.Runs[0].Condition == Control {
			controlFirst++
		} else {
			treatmentFirst++
		}
		if pair.Runs[0].Condition == pair.Runs[1].Condition {
			t.Fatalf("pair %s is not counterbalanced", pair.ID)
		}
		for _, run := range pair.Runs {
			if run.SubagentsEnabled {
				t.Fatal("subagents enabled")
			}
			if run.Condition == Treatment && (run.WorkPackages != 3 || run.RequiredFreshRotations != 2 || run.CheckpointByteLimit != 2048 || run.CarryPriorTranscript) {
				t.Fatalf("treatment policy = %+v", run)
			}
		}
	}
	if controlFirst != MaximumPairs/2 || treatmentFirst != MaximumPairs/2 {
		t.Fatalf("orders were not counterbalanced: control=%d treatment=%d", controlFirst, treatmentFirst)
	}
	if provider.Budget.TokenCeiling != 25_000 || provider.Budget.TurnCeiling != 22 || provider.Budget.WallMilliseconds != 30_000 {
		t.Fatalf("budget = %+v", provider.Budget)
	}
}

func TestGenerateRejectsInsufficientOrUntrustedShadow(t *testing.T) {
	config := testConfig("claude")
	config.Providers[0].ShadowRuns = config.Providers[0].ShadowRuns[:ShadowMinimum-1]
	if _, err := Generate(config); err == nil || !strings.Contains(err.Error(), "at least 20") {
		t.Fatalf("error = %v", err)
	}
	config = testConfig("claude")
	config.Providers[0].ShadowRuns[0].TerminalAuthoritative = false
	if _, err := Generate(config); err == nil || !strings.Contains(err.Error(), "non-authoritative") {
		t.Fatalf("error = %v", err)
	}
}

func TestEvaluateAdoptsOnlyFromAuthoritativeMatchedReceipts(t *testing.T) {
	plan := mustPlan(t, "codex")
	input := EvaluationInput{Plan: plan}
	for _, pair := range plan.Providers[0].Pairs[:NormalPairs] {
		for _, run := range pair.Runs {
			result := validResult(plan.Providers[0].Provider, run)
			if run.Condition == Control {
				result.TerminalReceipt.UncachedInput = 1_000
			} else {
				result.TerminalReceipt.UncachedInput = 650
			}
			input.Runs = append(input.Runs, result)
		}
	}
	evaluation, err := Evaluate(input)
	if err != nil {
		t.Fatal(err)
	}
	got := evaluation.Providers[0]
	if got.Decision != "adopt" || got.CompletePairs != 8 || got.SavingsCount != 8 || got.MedianTokenRatio != .65 {
		t.Fatalf("evaluation = %+v", got)
	}
}

func TestEvaluateStopsFutileWaveOne(t *testing.T) {
	plan := mustPlan(t, "claude")
	input := EvaluationInput{Plan: plan}
	for _, pair := range plan.Providers[0].Pairs[:WaveOnePairs] {
		for _, run := range pair.Runs {
			result := validResult("claude", run)
			result.TerminalReceipt.UncachedInput = 1_000
			input.Runs = append(input.Runs, result)
		}
	}
	evaluation, err := Evaluate(input)
	if err != nil {
		t.Fatal(err)
	}
	if got := evaluation.Providers[0]; got.Decision != "stop_futility" {
		t.Fatalf("evaluation = %+v", got)
	}
}

func TestEvaluateRejectsProxyContextEnforcementAndOversizeCheckpoint(t *testing.T) {
	plan := mustPlan(t, "codex")
	treatment := plan.Providers[0].Pairs[0].Runs[0]
	if treatment.Condition != Treatment {
		treatment = plan.Providers[0].Pairs[0].Runs[1]
	}
	result := validResult("codex", treatment)
	result.ContextActions = []ContextAction{{Action: "rotate_at_70_percent", Enforced: true, NumeratorExact: false, DenominatorExact: true}}
	_, err := Evaluate(EvaluationInput{Plan: plan, Runs: []RunResult{result}})
	if err == nil || !strings.Contains(err.Error(), "exact numerator") {
		t.Fatalf("error = %v", err)
	}
	result.ContextActions = nil
	result.MaximumCheckpointBytes = CheckpointByteLimit + 1
	_, err = Evaluate(EvaluationInput{Plan: plan, Runs: []RunResult{result}})
	if err == nil || !strings.Contains(err.Error(), "checkpoint isolation") {
		t.Fatalf("error = %v", err)
	}
}

func TestProcessedTokensIncludesAllReceiptClasses(t *testing.T) {
	receipt := TerminalReceipt{UncachedInput: 11, CacheRead: 13, CacheWrite: 17, Output: 19}
	if got := receipt.ProcessedTokens(); got != 60 {
		t.Fatalf("processed tokens = %d", got)
	}
}

func TestEvaluateRejectsCherryPickedLaterPair(t *testing.T) {
	plan := mustPlan(t, "codex")
	pair := plan.Providers[0].Pairs[1]
	input := EvaluationInput{Plan: plan, Runs: []RunResult{
		validResult("codex", pair.Runs[0]), validResult("codex", pair.Runs[1]),
	}}
	if _, err := Evaluate(input); err == nil || !strings.Contains(err.Error(), "incomplete earlier pair") {
		t.Fatalf("error = %v", err)
	}
}

func TestEvaluateRejectsIndividualBlindedQualityLoss(t *testing.T) {
	plan := mustPlan(t, "codex")
	pair := plan.Providers[0].Pairs[0]
	first, second := validResult("codex", pair.Runs[0]), validResult("codex", pair.Runs[1])
	if pair.Runs[0].Condition == Treatment {
		*first.BlindedQualityScore = 79
		*second.BlindedQualityScore = 90
	} else {
		*first.BlindedQualityScore = 90
		*second.BlindedQualityScore = 79
	}
	evaluation, err := Evaluate(EvaluationInput{Plan: plan, Runs: []RunResult{first, second}})
	if err != nil {
		t.Fatal(err)
	}
	if got := evaluation.Providers[0]; got.Decision != "reject" || got.WorstTreatmentQualityLoss != 11 {
		t.Fatalf("evaluation = %+v", got)
	}
}

func TestEvaluateDoesNotAdoptBelowMedianQualityLimit(t *testing.T) {
	plan := mustPlan(t, "codex")
	input := EvaluationInput{Plan: plan}
	for _, pair := range plan.Providers[0].Pairs[:NormalPairs] {
		for _, run := range pair.Runs {
			result := validResult("codex", run)
			if run.Condition == Control {
				result.TerminalReceipt.UncachedInput = 1_000
				*result.BlindedQualityScore = 90
			} else {
				result.TerminalReceipt.UncachedInput = 650
				*result.BlindedQualityScore = 84
			}
			input.Runs = append(input.Runs, result)
		}
	}
	evaluation, err := Evaluate(input)
	if err != nil {
		t.Fatal(err)
	}
	if got := evaluation.Providers[0]; got.Decision != "extend" || got.MedianQualityDifference != -6 {
		t.Fatalf("evaluation = %+v", got)
	}
}

func TestEvaluateFailsClosedOnCeilingBreach(t *testing.T) {
	plan := mustPlan(t, "claude")
	run := plan.Providers[0].Pairs[0].Runs[0]
	for name, breach := range map[string]func(*RunResult){
		"tokens": func(result *RunResult) {
			result.TerminalReceipt.UncachedInput = plan.Providers[0].Budget.TokenCeiling + 1
		},
		"turns": func(result *RunResult) { result.TerminalReceipt.Turns = plan.Providers[0].Budget.TurnCeiling + 1 },
		"wall": func(result *RunResult) {
			result.TerminalReceipt.WallMilliseconds = plan.Providers[0].Budget.WallMilliseconds + 1
		},
	} {
		t.Run(name, func(t *testing.T) {
			result := validResult("claude", run)
			breach(&result)
			if _, err := Evaluate(EvaluationInput{Plan: plan, Runs: []RunResult{result}}); err == nil || !strings.Contains(err.Error(), "ceiling") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func testConfig(provider string) Config {
	config := Config{StudyID: "continuous-work-v1", Seed: "held-secret-random-seed", QualityGates: []string{"acceptance_tests", "artifact_review"}}
	p := ProviderConfig{Provider: provider, Model: provider + "-fixed-model"}
	for i := 0; i < MaximumPairs; i++ {
		body, _ := json.Marshal(struct{ N int }{i})
		p.WorkItems = append(p.WorkItems, WorkItem{ID: string(rune('a' + i)), InputSHA256: digest(string(body))})
	}
	for i := 0; i < ShadowMinimum; i++ {
		p.ShadowRuns = append(p.ShadowRuns, ShadowRun{TerminalAuthoritative: true, ProcessedTokens: 20_000, Turns: 20, WallMilliseconds: 20_000})
	}
	config.Providers = []ProviderConfig{p}
	return config
}

func mustPlan(t *testing.T, provider string) Plan {
	t.Helper()
	plan, err := Generate(testConfig(provider))
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func validResult(provider string, run RunPlan) RunResult {
	score := 90.0
	result := RunResult{
		RunID: run.ID, Provider: provider,
		TerminalReceipt:        TerminalReceipt{Terminal: true, Authoritative: true, UncachedInput: 1_000, Turns: 10, WallMilliseconds: 1_000},
		QualityGates:           []QualityGateResult{{Name: "acceptance_tests", Passed: true}, {Name: "artifact_review", Passed: true}},
		ObservedNativeSessions: run.NativeSessions,
		BlindedQualityScore:    &score, QualityScoreBlinded: true,
	}
	if run.Condition == Treatment {
		result.ObservedWorkPackages = TreatmentWorkPackages
		result.ObservedFreshRotations = TreatmentRotations
		result.MaximumCheckpointBytes = CheckpointByteLimit
	}
	return result
}
