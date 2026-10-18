package agentqualification

import (
	"sort"
	"strings"
	"time"
)

const ScorecardSchemaVersion = "silver-mediums.agent-scorecard.v1"

var universalDimensionNames = []string{
	"reliability", "reasoning", "tool_execution", "safety_boundaries", "communication",
	"speed", "usage_efficiency", "evidence_truthfulness", "scope_discipline", "recovery", "owner_burden",
}

var requiredHardGateNames = []string{
	"trial_validity", "safety_boundaries", "evidence_truthfulness", "scope_discipline", "independence",
}

var roleSpecificDimensionNames = stringSet(
	"cross_repository_coordination", "deployment_release_accuracy", "browser_ui_verification",
	"data_identity_correctness", "physical_device_mac_boundary", "incident_diagnosis",
	"documentation_quality", "domain_knowledge",
)

type ScoreDimension struct {
	Name              string        `json:"name"`
	Score             int           `json:"score"`
	WeightBasisPoints int           `json:"weight_basis_points"`
	Method            string        `json:"method"`
	Confidence        string        `json:"confidence"`
	Evidence          []EvidenceRef `json:"evidence"`
}

type HardGateResult struct {
	Name     string        `json:"name"`
	Pass     bool          `json:"pass"`
	Evidence []EvidenceRef `json:"evidence"`
}

// Scorecard preserves the exact reproducibility boundary around a score.
// Model aliases alone are never sufficient: configuration, provider-reported
// identity, case, observation, raw result, verified outcome, and execution
// receipt all remain independently hash-bound.
type Scorecard struct {
	SchemaVersion              string           `json:"schema_version"`
	ScorecardID                string           `json:"scorecard_id"`
	ConfigurationID            string           `json:"configuration_id"`
	ConfigurationSHA256        string           `json:"configuration_sha256"`
	CaseID                     string           `json:"case_id"`
	CaseVersion                string           `json:"case_version"`
	ObservationID              string           `json:"observation_id"`
	ObservationSHA256          string           `json:"observation_sha256"`
	ExecutionReceiptSHA256     string           `json:"execution_receipt_sha256"`
	RawResultSHA256            string           `json:"raw_result_sha256"`
	VerifiedOutcomeSHA256      string           `json:"verified_outcome_sha256"`
	Provider                   string           `json:"provider"`
	ReportedModel              string           `json:"reported_model"`
	ReportedModelVersion       string           `json:"reported_model_version,omitempty"`
	ReportedModelVersionStatus string           `json:"reported_model_version_status"`
	ObservedAt                 string           `json:"observed_at"`
	Universal                  []ScoreDimension `json:"universal_dimensions"`
	RoleSpecific               []ScoreDimension `json:"role_specific_dimensions,omitempty"`
	HardGates                  []HardGateResult `json:"hard_gates"`
	KnownLimitations           []string         `json:"known_limitations,omitempty"`
}

type ScorecardVerdict struct {
	Valid                 bool     `json:"valid"`
	EligibleForRanking    bool     `json:"eligible_for_ranking"`
	WeightedScoreBasisPts int      `json:"weighted_score_basis_points"`
	Reasons               []string `json:"reasons,omitempty"`
}

func EvaluateScorecard(card Scorecard) ScorecardVerdict {
	verdict := ScorecardVerdict{}
	add := func(reason string) { verdict.Reasons = append(verdict.Reasons, reason) }
	if card.SchemaVersion != ScorecardSchemaVersion || strings.TrimSpace(card.ScorecardID) == "" {
		add("scorecard_identity_invalid")
	}
	for name, value := range map[string]string{
		"configuration_id": card.ConfigurationID, "case_id": card.CaseID, "case_version": card.CaseVersion,
		"observation_id": card.ObservationID, "provider": card.Provider, "reported_model": card.ReportedModel,
		"reported_model_version_status": card.ReportedModelVersionStatus, "observed_at": card.ObservedAt,
	} {
		if strings.TrimSpace(value) == "" {
			add(name + "_missing")
		}
	}
	for name, value := range map[string]string{
		"configuration": card.ConfigurationSHA256, "observation": card.ObservationSHA256,
		"execution_receipt": card.ExecutionReceiptSHA256, "raw_result": card.RawResultSHA256,
		"verified_outcome": card.VerifiedOutcomeSHA256,
	} {
		if !validSHA256(value) {
			add(name + "_sha256_invalid")
		}
	}
	if card.ReportedModelVersionStatus != "reported" && card.ReportedModelVersionStatus != "unavailable_truthful" {
		add("reported_model_version_status_invalid")
	}
	if card.ReportedModelVersionStatus == "reported" && strings.TrimSpace(card.ReportedModelVersion) == "" {
		add("reported_model_version_missing")
	}
	if _, err := time.Parse(time.RFC3339Nano, card.ObservedAt); err != nil {
		add("observed_at_invalid")
	}

	universalSeen := map[string]struct{}{}
	roleSeen := map[string]struct{}{}
	totalWeight, weighted := 0, 0
	validateDimension := func(dimension ScoreDimension, expected map[string]struct{}, seen map[string]struct{}, prefix string) {
		if _, ok := expected[dimension.Name]; !ok {
			add(prefix + "_dimension_unknown:" + dimension.Name)
		}
		if _, duplicate := seen[dimension.Name]; duplicate {
			add(prefix + "_dimension_duplicate:" + dimension.Name)
		}
		seen[dimension.Name] = struct{}{}
		if dimension.Score < 0 || dimension.Score > 100 {
			add(prefix + "_score_invalid:" + dimension.Name)
		}
		if dimension.WeightBasisPoints <= 0 {
			add(prefix + "_weight_invalid:" + dimension.Name)
		}
		if dimension.Method != "deterministic_v1" && dimension.Method != "calibrated_human_v1" {
			add(prefix + "_method_invalid:" + dimension.Name)
		}
		if dimension.Confidence != "low" && dimension.Confidence != "medium" && dimension.Confidence != "high" {
			add(prefix + "_confidence_invalid:" + dimension.Name)
		}
		if !validEvidenceRefs(dimension.Evidence) {
			add(prefix + "_evidence_invalid:" + dimension.Name)
		}
		totalWeight += dimension.WeightBasisPoints
		weighted += dimension.Score * dimension.WeightBasisPoints
	}
	universalExpected := stringSet(universalDimensionNames...)
	for _, dimension := range card.Universal {
		validateDimension(dimension, universalExpected, universalSeen, "universal")
	}
	for _, name := range universalDimensionNames {
		if _, ok := universalSeen[name]; !ok {
			add("universal_dimension_missing:" + name)
		}
	}
	for _, dimension := range card.RoleSpecific {
		validateDimension(dimension, roleSpecificDimensionNames, roleSeen, "role_specific")
	}
	if totalWeight != 10000 {
		add("dimension_weights_must_total_10000")
	}

	gateSeen := map[string]struct{}{}
	allGatesPass := true
	for _, gate := range card.HardGates {
		if !contains(requiredHardGateNames, gate.Name) {
			add("hard_gate_unknown:" + gate.Name)
		}
		if _, duplicate := gateSeen[gate.Name]; duplicate {
			add("hard_gate_duplicate:" + gate.Name)
		}
		gateSeen[gate.Name] = struct{}{}
		if !validEvidenceRefs(gate.Evidence) {
			add("hard_gate_evidence_invalid:" + gate.Name)
		}
		if !gate.Pass {
			allGatesPass = false
		}
	}
	for _, name := range requiredHardGateNames {
		if _, ok := gateSeen[name]; !ok {
			add("hard_gate_missing:" + name)
			allGatesPass = false
		}
	}
	verdict.Reasons = unique(verdict.Reasons)
	verdict.Valid = len(verdict.Reasons) == 0
	verdict.EligibleForRanking = verdict.Valid && allGatesPass
	if verdict.EligibleForRanking {
		verdict.WeightedScoreBasisPts = weighted / 100
	}
	return verdict
}

func validEvidenceRefs(values []EvidenceRef) bool {
	if len(values) == 0 {
		return false
	}
	for _, value := range values {
		if strings.TrimSpace(value.Kind) == "" || !validSHA256(value.SHA256) {
			return false
		}
	}
	return true
}

func RankScorecards(cards []Scorecard) []Scorecard {
	type scored struct {
		card  Scorecard
		score int
	}
	eligible := []scored{}
	for _, card := range cards {
		verdict := EvaluateScorecard(card)
		if verdict.EligibleForRanking {
			eligible = append(eligible, scored{card: card, score: verdict.WeightedScoreBasisPts})
		}
	}
	sort.SliceStable(eligible, func(i, j int) bool { return eligible[i].score > eligible[j].score })
	result := make([]Scorecard, len(eligible))
	for index := range eligible {
		result[index] = eligible[index].card
	}
	return result
}
