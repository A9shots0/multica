package agentqualification

import "testing"

func TestMinimumScreeningSuiteIsCalibratedPinnedAndPartiallyExecuted(t *testing.T) {
	suite, err := CurrentMinimumScreeningSuite()
	if err != nil {
		t.Fatal(err)
	}
	if suite.Status != "calibrated_partially_executed" || len(suite.Cases) != 4 {
		t.Fatalf("unexpected suite: %+v", suite)
	}
	changed := suite
	changed.Cases = append([]ScreeningCase(nil), suite.Cases...)
	changed.Cases[0].CandidatePacketSHA256 = "changed"
	if err := ValidateMinimumScreeningSuite(changed); err == nil {
		t.Fatal("changed packet hash was accepted")
	}
}
