package agentqualification

import "testing"

func TestRealisticNextRunMatrixIsCappedAndLocked(t *testing.T) {
	matrix, err := CurrentRealisticNextRunMatrix()
	if err != nil {
		t.Fatal(err)
	}
	if matrix.Status != "frozen_reviewed" || len(matrix.Candidates) != 2 || matrix.Retries != 0 || matrix.ProviderFallbacks {
		t.Fatalf("unsafe or unexpected next matrix: %+v", matrix)
	}
	if matrix.MaxIncrementalSpendMicros > 3000000 || matrix.MaxTotalTokens > 300000 {
		t.Fatal("next matrix exceeds its reviewed cap")
	}
}
