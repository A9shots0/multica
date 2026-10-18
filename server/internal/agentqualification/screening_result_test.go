package agentqualification

import "testing"

func TestThirdScreeningResultIsSealedInvalidAndWithinCeilings(t *testing.T) {
	result, err := CurrentScreeningResult()
	if err != nil {
		t.Fatalf("CurrentScreeningResult: %v", err)
	}
	if result.Totals.Runs != 2 || result.Totals.TotalTokens != 27238 {
		t.Fatalf("unexpected sealed totals: %+v", result.Totals)
	}
	if result.Observations[0].Verdict != "INVALID" || result.Observations[1].Verdict != "INVALID" {
		t.Fatalf("unexpected sealed verdict order: %+v", result.Observations)
	}
}

func TestScreeningResultRejectsReceiptDrift(t *testing.T) {
	result, err := CurrentScreeningResult()
	if err != nil {
		t.Fatalf("CurrentScreeningResult: %v", err)
	}
	result.Observations[0].FreshInputTokens++
	batch, err := ThirdScreeningBatch()
	if err != nil {
		t.Fatalf("CurrentScreeningBatch: %v", err)
	}
	if err := ValidateScreeningResult(batch, result); err == nil {
		t.Fatal("expected drifted receipt to fail validation")
	}
}

func TestFourthScreeningResultStopsAfterInvalidOverBudgetObservation(t *testing.T) {
	result, err := FourthScreeningResult()
	if err != nil {
		t.Fatalf("FourthScreeningResult: %v", err)
	}
	if result.Status != "stopped_invalid_harness" || result.Totals.Runs != 1 || result.Totals.TotalTokens != 176383 {
		t.Fatalf("unexpected stopped result: %+v", result)
	}
	if result.Observations[0].Verdict != "INVALID" || result.Observations[0].ToolCalls != 7 {
		t.Fatalf("invalid execution evidence not sealed: %+v", result.Observations[0])
	}
}
