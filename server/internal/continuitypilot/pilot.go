// Package continuitypilot defines an offline plan and evaluation contract for
// the continuous-work experiment. It deliberately contains no provider client:
// generating a plan or evaluating receipts cannot start paid work.
package continuitypilot

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
)

const (
	ContractVersion       = "multica.continuity-pilot.v1"
	CheckpointByteLimit   = 2 * 1024
	ShadowMinimum         = 20
	WaveOnePairs          = 4
	NormalPairs           = 8
	MaximumPairs          = 12
	TreatmentWorkPackages = 3
	TreatmentRotations    = 2
)

type Condition string

const (
	Control   Condition = "control"
	Treatment Condition = "treatment"
)

type Config struct {
	StudyID      string           `json:"study_id"`
	Seed         string           `json:"seed"`
	QualityGates []string         `json:"quality_gates"`
	Providers    []ProviderConfig `json:"providers"`
}

type ProviderConfig struct {
	Provider   string      `json:"provider"`
	Model      string      `json:"model"`
	WorkItems  []WorkItem  `json:"work_items"`
	ShadowRuns []ShadowRun `json:"shadow_runs"`
}

type WorkItem struct {
	ID          string `json:"id"`
	InputSHA256 string `json:"input_sha256"`
}

// ShadowRun is an already-completed historical run. Only authoritative
// terminal measurements may be used to set experimental ceilings.
type ShadowRun struct {
	TerminalAuthoritative bool  `json:"terminal_authoritative"`
	ProcessedTokens       int64 `json:"processed_tokens"`
	Turns                 int64 `json:"turns"`
	WallMilliseconds      int64 `json:"wall_milliseconds"`
}

type Plan struct {
	ContractVersion string         `json:"contract_version"`
	StudyID         string         `json:"study_id"`
	SeedSHA256      string         `json:"seed_sha256"`
	QualityGates    []string       `json:"quality_gates"`
	Design          Design         `json:"design"`
	Providers       []ProviderPlan `json:"providers"`
}

type Design struct {
	ProviderAnalysis         string  `json:"provider_analysis"`
	Pairing                  string  `json:"pairing"`
	Preflight                string  `json:"preflight"`
	WaveOnePairs             int     `json:"wave_one_pairs"`
	NormalPairs              int     `json:"normal_pairs"`
	MaximumPairs             int     `json:"maximum_pairs"`
	ShadowMinimum            int     `json:"shadow_minimum_per_provider"`
	MedianTokenRatioMax      float64 `json:"median_token_ratio_max"`
	GeomeanTokenRatioMax     float64 `json:"geomean_token_ratio_max"`
	SavingsRequiredOfEight   int     `json:"savings_required_of_eight"`
	P90WallRatioMax          float64 `json:"p90_wall_ratio_max"`
	FutilityMedianMin        float64 `json:"futility_median_ratio_min"`
	FutilitySavingsMin       int     `json:"futility_savings_min_of_four"`
	MedianQualityDeltaMin    float64 `json:"median_quality_delta_min"`
	MaxIndividualQualityLoss float64 `json:"max_individual_quality_loss"`
}

type ProviderPlan struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Budget   Budget `json:"budget"`
	Pairs    []Pair `json:"pairs"`
}

type Budget struct {
	TokenCeiling      int64   `json:"token_ceiling"`
	TurnCeiling       int64   `json:"turn_ceiling"`
	WallMilliseconds  int64   `json:"wall_milliseconds_ceiling"`
	WrapFraction      float64 `json:"wrap_fraction"`
	InterruptFraction float64 `json:"interrupt_fraction"`
}

type Pair struct {
	ID       string     `json:"id"`
	Ordinal  int        `json:"ordinal"`
	Stage    string     `json:"stage"`
	WorkItem WorkItem   `json:"work_item"`
	Runs     [2]RunPlan `json:"runs"`
}

type RunPlan struct {
	ID                     string    `json:"id"`
	Condition              Condition `json:"condition"`
	Order                  int       `json:"order"`
	ProviderNative         bool      `json:"provider_native"`
	SubagentsEnabled       bool      `json:"subagents_enabled"`
	NativeSessions         int       `json:"native_sessions"`
	WorkPackages           int       `json:"work_packages"`
	RequiredFreshRotations int       `json:"required_fresh_rotations"`
	CheckpointByteLimit    int       `json:"checkpoint_byte_limit"`
	CarryPriorTranscript   bool      `json:"carry_prior_transcript"`
	ContextPercentPolicy   string    `json:"context_percent_policy"`
}

func Generate(config Config) (Plan, error) {
	if err := validateConfig(config); err != nil {
		return Plan{}, err
	}
	plan := Plan{
		ContractVersion: ContractVersion,
		StudyID:         config.StudyID,
		SeedSHA256:      digest(config.Seed),
		QualityGates:    slices.Clone(config.QualityGates),
		Design: Design{
			ProviderAnalysis: "separate",
			Pairing:          "matched and deterministically counterbalanced",
			Preflight:        "validate contracts and one representative work item without adding an experimental pair",
			WaveOnePairs:     WaveOnePairs, NormalPairs: NormalPairs, MaximumPairs: MaximumPairs,
			ShadowMinimum: ShadowMinimum, MedianTokenRatioMax: .80, GeomeanTokenRatioMax: .85,
			SavingsRequiredOfEight: 7, P90WallRatioMax: 1.25, FutilityMedianMin: .95, FutilitySavingsMin: 3,
			MedianQualityDeltaMin: -5, MaxIndividualQualityLoss: 10,
		},
	}
	providers := slices.Clone(config.Providers)
	sort.Slice(providers, func(i, j int) bool { return providers[i].Provider < providers[j].Provider })
	for _, provider := range providers {
		pp := ProviderPlan{Provider: provider.Provider, Model: provider.Model, Budget: budgetFromShadow(provider.ShadowRuns)}
		controlFirstByItem := balancedOrder(config.Seed, provider.Provider, provider.WorkItems[:MaximumPairs])
		for i, item := range provider.WorkItems[:MaximumPairs] {
			pairID := shortID(config.StudyID, provider.Provider, item.ID, "pair")
			controlFirst := controlFirstByItem[item.ID]
			conditions := [2]Condition{Treatment, Control}
			if controlFirst {
				conditions = [2]Condition{Control, Treatment}
			}
			pair := Pair{ID: pairID, Ordinal: i + 1, Stage: stage(i + 1), WorkItem: item}
			for order, condition := range conditions {
				pair.Runs[order] = makeRun(pairID, condition, order+1)
			}
			pp.Pairs = append(pp.Pairs, pair)
		}
		plan.Providers = append(plan.Providers, pp)
	}
	return plan, nil
}

func validateConfig(config Config) error {
	if strings.TrimSpace(config.StudyID) == "" || strings.TrimSpace(config.Seed) == "" {
		return errors.New("study_id and seed are required")
	}
	if len(config.QualityGates) == 0 {
		return errors.New("at least one hard quality gate is required")
	}
	if err := uniqueNonempty("quality gate", config.QualityGates); err != nil {
		return err
	}
	if len(config.Providers) == 0 {
		return errors.New("at least one provider is required")
	}
	providerNames := make([]string, 0, len(config.Providers))
	for _, p := range config.Providers {
		providerNames = append(providerNames, p.Provider)
		if strings.TrimSpace(p.Provider) == "" || strings.TrimSpace(p.Model) == "" {
			return errors.New("provider and model are required")
		}
		if len(p.WorkItems) < MaximumPairs {
			return fmt.Errorf("provider %q needs at least %d matched work items", p.Provider, MaximumPairs)
		}
		if len(p.ShadowRuns) < ShadowMinimum {
			return fmt.Errorf("provider %q needs at least %d completed shadow runs", p.Provider, ShadowMinimum)
		}
		ids := make([]string, 0, len(p.WorkItems))
		for _, item := range p.WorkItems {
			ids = append(ids, item.ID)
			if strings.TrimSpace(item.ID) == "" || !validSHA256(item.InputSHA256) {
				return fmt.Errorf("provider %q has invalid work item", p.Provider)
			}
		}
		if err := uniqueNonempty("work item", ids); err != nil {
			return fmt.Errorf("provider %q: %w", p.Provider, err)
		}
		for _, shadow := range p.ShadowRuns {
			if !shadow.TerminalAuthoritative || shadow.ProcessedTokens <= 0 || shadow.Turns <= 0 || shadow.WallMilliseconds <= 0 {
				return fmt.Errorf("provider %q has a non-authoritative or non-positive shadow run", p.Provider)
			}
		}
	}
	return uniqueNonempty("provider", providerNames)
}

func makeRun(pairID string, condition Condition, order int) RunPlan {
	run := RunPlan{ID: shortID(pairID, string(condition), "run"), Condition: condition, Order: order,
		ProviderNative: true, SubagentsEnabled: false, ContextPercentPolicy: "enforce only with exact numerator and exact denominator"}
	if condition == Control {
		run.NativeSessions = 1
		run.WorkPackages = 1
		return run
	}
	run.NativeSessions = TreatmentRotations + 1
	run.WorkPackages = TreatmentWorkPackages
	run.RequiredFreshRotations = TreatmentRotations
	run.CheckpointByteLimit = CheckpointByteLimit
	run.CarryPriorTranscript = false
	return run
}

func budgetFromShadow(runs []ShadowRun) Budget {
	tokens, turns, wall := make([]int64, len(runs)), make([]int64, len(runs)), make([]int64, len(runs))
	for i, run := range runs {
		tokens[i], turns[i], wall[i] = run.ProcessedTokens, run.Turns, run.WallMilliseconds
	}
	return Budget{
		TokenCeiling:     int64(math.Ceil(1.25 * float64(nearestRank(tokens, .95)))),
		TurnCeiling:      nearestRank(turns, .95) + 2,
		WallMilliseconds: int64(math.Ceil(1.5 * float64(nearestRank(wall, .95)))),
		WrapFraction:     .80, InterruptFraction: 1.0,
	}
}

func stage(ordinal int) string {
	switch {
	case ordinal <= WaveOnePairs:
		return "wave_1"
	case ordinal <= NormalPairs:
		return "wave_2"
	default:
		return "extension"
	}
}

// balancedOrder assigns exactly half of the pairs to each run order. Hash
// ranking keeps that assignment reproducible without letting list order bias it.
func balancedOrder(seed, provider string, items []WorkItem) map[string]bool {
	ranked := slices.Clone(items)
	sort.Slice(ranked, func(i, j int) bool {
		return digest(seed, provider, ranked[i].ID) < digest(seed, provider, ranked[j].ID)
	})
	result := make(map[string]bool, len(items))
	for _, item := range ranked[:len(ranked)/2] {
		result[item.ID] = true
	}
	return result
}

func nearestRank(values []int64, quantile float64) int64 {
	copyValues := slices.Clone(values)
	slices.Sort(copyValues)
	index := int(math.Ceil(quantile*float64(len(copyValues)))) - 1
	if index < 0 {
		index = 0
	}
	return copyValues[index]
}

func uniqueNonempty(label string, values []string) error {
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return fmt.Errorf("%s must not be empty", label)
		}
		if _, exists := seen[value]; exists {
			return fmt.Errorf("duplicate %s %q", label, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func digest(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		h.Write([]byte{0})
		h.Write([]byte(part))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func shortID(parts ...string) string { return digest(parts...)[:24] }
