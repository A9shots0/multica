package continuitypilot

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
)

type EvaluationInput struct {
	Plan Plan        `json:"plan"`
	Runs []RunResult `json:"runs"`
}

type RunResult struct {
	RunID                  string              `json:"run_id"`
	Provider               string              `json:"provider"`
	TerminalReceipt        TerminalReceipt     `json:"terminal_receipt"`
	QualityGates           []QualityGateResult `json:"quality_gates"`
	ObservedNativeSessions int                 `json:"observed_native_sessions"`
	ObservedWorkPackages   int                 `json:"observed_work_packages"`
	ObservedFreshRotations int                 `json:"observed_fresh_rotations"`
	MaximumCheckpointBytes int                 `json:"maximum_checkpoint_bytes"`
	CarriedPriorTranscript bool                `json:"carried_prior_transcript"`
	SubagentsEnabled       bool                `json:"subagents_enabled"`
	ContextActions         []ContextAction     `json:"context_actions,omitempty"`
	BlindedQualityScore    *float64            `json:"blinded_quality_score"`
	QualityScoreBlinded    bool                `json:"quality_score_blinded"`
}

type TerminalReceipt struct {
	Terminal         bool  `json:"terminal"`
	Authoritative    bool  `json:"authoritative"`
	UncachedInput    int64 `json:"uncached_input_tokens"`
	CacheRead        int64 `json:"cache_read_tokens"`
	CacheWrite       int64 `json:"cache_write_tokens"`
	Output           int64 `json:"output_tokens"`
	Turns            int64 `json:"turns"`
	WallMilliseconds int64 `json:"wall_milliseconds"`
}

func (r TerminalReceipt) ProcessedTokens() int64 {
	return r.UncachedInput + r.CacheRead + r.CacheWrite + r.Output
}

type QualityGateResult struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
}

type ContextAction struct {
	Action           string `json:"action"`
	Enforced         bool   `json:"enforced"`
	NumeratorExact   bool   `json:"numerator_exact"`
	DenominatorExact bool   `json:"denominator_exact"`
}

type Evaluation struct {
	ContractVersion string               `json:"contract_version"`
	StudyID         string               `json:"study_id"`
	Providers       []ProviderEvaluation `json:"providers"`
}

type ProviderEvaluation struct {
	Provider                  string  `json:"provider"`
	CompletePairs             int     `json:"complete_pairs"`
	MedianTokenRatio          float64 `json:"median_token_ratio,omitempty"`
	GeomeanTokenRatio         float64 `json:"geomean_token_ratio,omitempty"`
	SavingsCount              int     `json:"savings_count"`
	P90WallRatio              float64 `json:"p90_wall_ratio,omitempty"`
	WaveOneMedianTokenRatio   float64 `json:"wave_one_median_token_ratio,omitempty"`
	WaveOneSavingsCount       int     `json:"wave_one_savings_count"`
	MedianQualityDifference   float64 `json:"median_quality_difference,omitempty"`
	WorstTreatmentQualityLoss float64 `json:"worst_treatment_quality_loss,omitempty"`
	AllQualityGatesPass       bool    `json:"all_quality_gates_pass"`
	Decision                  string  `json:"decision"`
	Reason                    string  `json:"reason"`
}

func Evaluate(input EvaluationInput) (Evaluation, error) {
	if input.Plan.ContractVersion != ContractVersion {
		return Evaluation{}, errors.New("unsupported plan contract")
	}
	if err := validatePlanShape(input.Plan); err != nil {
		return Evaluation{}, err
	}
	type plannedRun struct {
		provider string
		plan     RunPlan
		budget   Budget
	}
	plannedRuns := make(map[string]plannedRun)
	for _, provider := range input.Plan.Providers {
		for _, pair := range provider.Pairs {
			for _, run := range pair.Runs {
				if _, exists := plannedRuns[run.ID]; exists {
					return Evaluation{}, fmt.Errorf("duplicate planned run %q", run.ID)
				}
				plannedRuns[run.ID] = plannedRun{provider: provider.Provider, plan: run, budget: provider.Budget}
			}
		}
	}
	results := make(map[string]RunResult, len(input.Runs))
	for _, result := range input.Runs {
		if _, exists := results[result.RunID]; exists {
			return Evaluation{}, fmt.Errorf("duplicate run result %q", result.RunID)
		}
		planned, exists := plannedRuns[result.RunID]
		if !exists {
			return Evaluation{}, fmt.Errorf("unplanned run result %q", result.RunID)
		}
		if err := validateResult(planned.provider, input.Plan.QualityGates, planned.plan, planned.budget, result); err != nil {
			return Evaluation{}, err
		}
		results[result.RunID] = result
	}
	evaluation := Evaluation{ContractVersion: ContractVersion, StudyID: input.Plan.StudyID}
	for _, provider := range input.Plan.Providers {
		pe := ProviderEvaluation{Provider: provider.Provider, AllQualityGatesPass: true}
		var tokenRatios, wallRatios, qualityDifferences []float64
		seenIncompletePair := false
		for _, pair := range provider.Pairs {
			first, okFirst := results[pair.Runs[0].ID]
			second, okSecond := results[pair.Runs[1].ID]
			if seenIncompletePair && (okFirst || okSecond) {
				return Evaluation{}, fmt.Errorf("provider %q has run data after an incomplete earlier pair", provider.Provider)
			}
			for _, result := range []struct {
				value RunResult
				ok    bool
			}{{first, okFirst}, {second, okSecond}} {
				if result.ok && !allGatesPass(result.value.QualityGates) {
					pe.AllQualityGatesPass = false
				}
			}
			if !okFirst || !okSecond {
				seenIncompletePair = true
				continue
			}
			planned := map[string]RunPlan{pair.Runs[0].ID: pair.Runs[0], pair.Runs[1].ID: pair.Runs[1]}
			control, treatment := first, second
			if planned[first.RunID].Condition == Treatment {
				control, treatment = second, first
			}
			controlTokens, treatmentTokens := control.TerminalReceipt.ProcessedTokens(), treatment.TerminalReceipt.ProcessedTokens()
			ratio := float64(treatmentTokens) / float64(controlTokens)
			tokenRatios = append(tokenRatios, ratio)
			wallRatios = append(wallRatios, float64(treatment.TerminalReceipt.WallMilliseconds)/float64(control.TerminalReceipt.WallMilliseconds))
			qualityDifference := *treatment.BlindedQualityScore - *control.BlindedQualityScore
			qualityDifferences = append(qualityDifferences, qualityDifference)
			if loss := -qualityDifference; loss > pe.WorstTreatmentQualityLoss {
				pe.WorstTreatmentQualityLoss = loss
			}
			if treatmentTokens < controlTokens {
				pe.SavingsCount++
			}
			pe.CompletePairs++
		}
		if pe.CompletePairs > 0 {
			pe.MedianTokenRatio = median(tokenRatios)
			pe.GeomeanTokenRatio = geomean(tokenRatios)
			pe.P90WallRatio = percentileFloat(wallRatios, .90)
			pe.MedianQualityDifference = median(qualityDifferences)
		}
		if pe.CompletePairs >= WaveOnePairs {
			pe.WaveOneMedianTokenRatio = median(tokenRatios[:WaveOnePairs])
			for _, ratio := range tokenRatios[:WaveOnePairs] {
				if ratio < 1 {
					pe.WaveOneSavingsCount++
				}
			}
		}
		decide(&pe)
		evaluation.Providers = append(evaluation.Providers, pe)
	}
	return evaluation, nil
}

func validatePlanShape(plan Plan) error {
	if plan.StudyID == "" || len(plan.Providers) == 0 || len(plan.QualityGates) == 0 {
		return errors.New("plan is incomplete")
	}
	if !validSHA256(plan.SeedSHA256) {
		return errors.New("plan seed digest is invalid")
	}
	if err := uniqueNonempty("quality gate", plan.QualityGates); err != nil {
		return err
	}
	if plan.Design.ProviderAnalysis != "separate" || plan.Design.WaveOnePairs != WaveOnePairs || plan.Design.NormalPairs != NormalPairs || plan.Design.MaximumPairs != MaximumPairs || plan.Design.ShadowMinimum != ShadowMinimum {
		return errors.New("plan design does not match the audited protocol")
	}
	if plan.Design.MedianTokenRatioMax != .80 || plan.Design.GeomeanTokenRatioMax != .85 || plan.Design.SavingsRequiredOfEight != 7 || plan.Design.P90WallRatioMax != 1.25 || plan.Design.FutilityMedianMin != .95 || plan.Design.FutilitySavingsMin != 3 || plan.Design.MedianQualityDeltaMin != -5 || plan.Design.MaxIndividualQualityLoss != 10 {
		return errors.New("plan thresholds do not match the audited protocol")
	}
	providerNames := make([]string, 0, len(plan.Providers))
	for _, provider := range plan.Providers {
		providerNames = append(providerNames, provider.Provider)
		if provider.Provider == "" || provider.Model == "" {
			return errors.New("planned provider and model are required")
		}
		if len(provider.Pairs) != MaximumPairs {
			return fmt.Errorf("provider %q plan must contain %d pairs", provider.Provider, MaximumPairs)
		}
		if provider.Budget.TokenCeiling <= 0 || provider.Budget.TurnCeiling <= 0 || provider.Budget.WallMilliseconds <= 0 || provider.Budget.WrapFraction != .80 || provider.Budget.InterruptFraction != 1 {
			return fmt.Errorf("provider %q has invalid budget controls", provider.Provider)
		}
		controlFirst := 0
		pairIDs := make([]string, 0, len(provider.Pairs))
		for pairIndex, pair := range provider.Pairs {
			pairIDs = append(pairIDs, pair.ID)
			if pair.ID == "" || pair.WorkItem.ID == "" || !validSHA256(pair.WorkItem.InputSHA256) {
				return fmt.Errorf("provider %q has an invalid pair identity", provider.Provider)
			}
			if pair.Ordinal != pairIndex+1 || pair.Stage != stage(pairIndex+1) {
				return fmt.Errorf("pair %q has invalid ordinal or stage", pair.ID)
			}
			conditions := []Condition{pair.Runs[0].Condition, pair.Runs[1].Condition}
			slices.Sort(conditions)
			if conditions[0] != Control || conditions[1] != Treatment {
				return fmt.Errorf("pair %q must contain control and treatment", pair.ID)
			}
			if pair.Runs[0].Condition == Control {
				controlFirst++
			}
			for order, run := range pair.Runs {
				if run.Order != order+1 || !run.ProviderNative || run.SubagentsEnabled || run.ContextPercentPolicy != "enforce only with exact numerator and exact denominator" {
					return fmt.Errorf("run %q violates the provider-native protocol", run.ID)
				}
				if run.Condition == Control && (run.NativeSessions != 1 || run.WorkPackages != 1 || run.RequiredFreshRotations != 0 || run.CheckpointByteLimit != 0 || run.CarryPriorTranscript) {
					return fmt.Errorf("control run %q violates the single-session protocol", run.ID)
				}
				if run.Condition == Treatment && (run.NativeSessions != 3 || run.WorkPackages != TreatmentWorkPackages || run.RequiredFreshRotations != TreatmentRotations || run.CheckpointByteLimit != CheckpointByteLimit || run.CarryPriorTranscript) {
					return fmt.Errorf("treatment run %q violates the rotation protocol", run.ID)
				}
			}
		}
		if err := uniqueNonempty("pair", pairIDs); err != nil {
			return fmt.Errorf("provider %q: %w", provider.Provider, err)
		}
		if controlFirst != MaximumPairs/2 {
			return fmt.Errorf("provider %q run order is not evenly counterbalanced", provider.Provider)
		}
	}
	return uniqueNonempty("provider", providerNames)
}

func validateResult(provider string, gates []string, plan RunPlan, budget Budget, result RunResult) error {
	if result.Provider != provider {
		return fmt.Errorf("run %q provider mismatch", result.RunID)
	}
	r := result.TerminalReceipt
	if !r.Terminal || !r.Authoritative {
		return fmt.Errorf("run %q lacks an authoritative terminal receipt", result.RunID)
	}
	if r.UncachedInput < 0 || r.CacheRead < 0 || r.CacheWrite < 0 || r.Output < 0 || r.ProcessedTokens() <= 0 || r.Turns <= 0 || r.WallMilliseconds <= 0 {
		return fmt.Errorf("run %q has invalid usage", result.RunID)
	}
	if r.ProcessedTokens() > budget.TokenCeiling || r.Turns > budget.TurnCeiling || r.WallMilliseconds > budget.WallMilliseconds {
		return fmt.Errorf("run %q breached its token, turn, or wall-time ceiling", result.RunID)
	}
	if result.BlindedQualityScore == nil || !result.QualityScoreBlinded || *result.BlindedQualityScore < 0 || *result.BlindedQualityScore > 100 || math.IsNaN(*result.BlindedQualityScore) {
		return fmt.Errorf("run %q lacks a valid blinded quality score from 0 to 100", result.RunID)
	}
	if result.SubagentsEnabled {
		return fmt.Errorf("run %q enabled subagents", result.RunID)
	}
	for _, action := range result.ContextActions {
		if action.Enforced && (!action.NumeratorExact || !action.DenominatorExact) {
			return fmt.Errorf("run %q enforced context percentage without exact numerator and denominator", result.RunID)
		}
	}
	if err := validateGateResults(gates, result.QualityGates); err != nil {
		return fmt.Errorf("run %q: %w", result.RunID, err)
	}
	if plan.Condition == Control {
		if result.ObservedNativeSessions != 1 {
			return fmt.Errorf("control run %q did not use one native session", result.RunID)
		}
	} else {
		if result.ObservedNativeSessions != TreatmentRotations+1 || result.ObservedWorkPackages != TreatmentWorkPackages || result.ObservedFreshRotations < TreatmentRotations || result.MaximumCheckpointBytes < 0 || result.MaximumCheckpointBytes > CheckpointByteLimit || result.CarriedPriorTranscript {
			return fmt.Errorf("treatment run %q violated rotation/checkpoint isolation", result.RunID)
		}
	}
	return nil
}

func validateGateResults(expected []string, got []QualityGateResult) error {
	if len(got) != len(expected) {
		return errors.New("quality gate set mismatch")
	}
	seen := map[string]struct{}{}
	for _, gate := range got {
		seen[gate.Name] = struct{}{}
	}
	for _, name := range expected {
		if _, ok := seen[name]; !ok {
			return fmt.Errorf("missing quality gate %q", name)
		}
	}
	if len(seen) != len(got) {
		return errors.New("duplicate quality gate result")
	}
	return nil
}

func allGatesPass(gates []QualityGateResult) bool {
	for _, gate := range gates {
		if !gate.Passed {
			return false
		}
	}
	return true
}

func decide(pe *ProviderEvaluation) {
	if !pe.AllQualityGatesPass {
		pe.Decision, pe.Reason = "reject", "a hard quality gate failed"
		return
	}
	if pe.WorstTreatmentQualityLoss > 10 {
		pe.Decision, pe.Reason = "reject", "an individual treatment lost more than 10 quality points"
		return
	}
	if pe.CompletePairs < WaveOnePairs {
		pe.Decision, pe.Reason = "collect", "fewer than four complete matched pairs"
		return
	}
	if pe.CompletePairs < NormalPairs {
		if pe.WaveOneMedianTokenRatio >= .95 || pe.WaveOneSavingsCount < 3 {
			pe.Decision, pe.Reason = "stop_futility", "wave 1 crossed a predeclared futility boundary"
		} else {
			pe.Decision, pe.Reason = "continue_wave_2", "wave 1 is promising"
		}
		return
	}
	requiredSavings := int(math.Ceil(.875 * float64(pe.CompletePairs)))
	if pe.MedianTokenRatio <= .80 && pe.GeomeanTokenRatio <= .85 && pe.SavingsCount >= requiredSavings && pe.P90WallRatio <= 1.25 && pe.MedianQualityDifference >= -5 {
		pe.Decision, pe.Reason = "adopt", "all quality and efficiency thresholds passed"
		return
	}
	if pe.CompletePairs < MaximumPairs {
		pe.Decision, pe.Reason = "extend", "normal sample was inconclusive; extension remains within the cap"
		return
	}
	pe.Decision, pe.Reason = "reject", "efficiency thresholds did not pass by the sample cap"
}

func median(values []float64) float64 {
	copyValues := slices.Clone(values)
	sort.Float64s(copyValues)
	n := len(copyValues)
	if n%2 == 1 {
		return copyValues[n/2]
	}
	return (copyValues[n/2-1] + copyValues[n/2]) / 2
}
func geomean(values []float64) float64 {
	sum := 0.0
	for _, value := range values {
		if value == 0 {
			return 0
		}
		sum += math.Log(value)
	}
	return math.Exp(sum / float64(len(values)))
}
func percentileFloat(values []float64, q float64) float64 {
	copyValues := slices.Clone(values)
	sort.Float64s(copyValues)
	return copyValues[int(math.Ceil(q*float64(len(copyValues))))-1]
}
