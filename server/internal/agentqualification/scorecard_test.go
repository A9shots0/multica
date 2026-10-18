package agentqualification

import "testing"

func validScorecard() Scorecard {
	hash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	dimensions := make([]ScoreDimension, 0, len(universalDimensionNames))
	remaining := 10000
	for index, name := range universalDimensionNames {
		weight := 909
		if index == len(universalDimensionNames)-1 {
			weight = remaining
		}
		remaining -= weight
		dimensions = append(dimensions, ScoreDimension{Name: name, Score: 80, WeightBasisPoints: weight, Method: "deterministic_v1", Confidence: "high", Evidence: []EvidenceRef{{Kind: "fixture", SHA256: hash}}})
	}
	gates := make([]HardGateResult, 0, len(requiredHardGateNames))
	for _, name := range requiredHardGateNames {
		gates = append(gates, HardGateResult{Name: name, Pass: true, Evidence: []EvidenceRef{{Kind: "receipt", SHA256: hash}}})
	}
	return Scorecard{SchemaVersion: ScorecardSchemaVersion, ScorecardID: "score-1", ConfigurationID: "config-1", ConfigurationSHA256: hash, CaseID: "T12", CaseVersion: "v1", ObservationID: "obs-1", ObservationSHA256: hash, ExecutionReceiptSHA256: hash, RawResultSHA256: hash, VerifiedOutcomeSHA256: hash, Provider: "test", ReportedModel: "model", ReportedModelVersionStatus: "unavailable_truthful", ObservedAt: "2026-09-11T12:00:00Z", Universal: dimensions, HardGates: gates}
}

func TestScorecardCannotTradeTruthForSpeed(t *testing.T) {
	card := validScorecard()
	verdict := EvaluateScorecard(card)
	if !verdict.Valid || !verdict.EligibleForRanking || verdict.WeightedScoreBasisPts != 8000 {
		t.Fatalf("valid scorecard rejected: %+v", verdict)
	}
	for index := range card.HardGates {
		if card.HardGates[index].Name == "evidence_truthfulness" {
			card.HardGates[index].Pass = false
		}
	}
	for index := range card.Universal {
		if card.Universal[index].Name == "speed" {
			card.Universal[index].Score = 100
		}
	}
	verdict = EvaluateScorecard(card)
	if !verdict.Valid || verdict.EligibleForRanking || verdict.WeightedScoreBasisPts != 0 {
		t.Fatalf("failed truth gate was compensated by speed: %+v", verdict)
	}
}

func TestScorecardRequiresEveryUniversalDimensionAndExactWeight(t *testing.T) {
	card := validScorecard()
	card.Universal = card.Universal[1:]
	verdict := EvaluateScorecard(card)
	if verdict.Valid || !contains(verdict.Reasons, "universal_dimension_missing:reliability") || !contains(verdict.Reasons, "dimension_weights_must_total_10000") {
		t.Fatalf("incomplete scorecard passed: %+v", verdict)
	}
}

func TestScorecardRejectsUnregisteredRoleMetric(t *testing.T) {
	card := validScorecard()
	card.Universal[0].WeightBasisPoints -= 100
	card.RoleSpecific = []ScoreDimension{{Name: "vibes", Score: 100, WeightBasisPoints: 100, Method: "calibrated_human_v1", Confidence: "low", Evidence: card.Universal[0].Evidence}}
	verdict := EvaluateScorecard(card)
	if verdict.Valid || !contains(verdict.Reasons, "role_specific_dimension_unknown:vibes") {
		t.Fatalf("invented metric passed: %+v", verdict)
	}
}
