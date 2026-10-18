package main

import (
	"context"
	"github.com/multica-ai/multica/server/internal/agentqualification"
	"testing"
)

func offlineContracts(t *testing.T) agentqualification.QualificationContracts {
	t.Helper()
	c, e := agentqualification.HistoricalQualificationContracts()
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func TestHistoricalShadowInputsRejectedBeforeRealCallbacks(t *testing.T) {
	for _, version := range []int{2, 3, 4} {
		deps := newFakeItem14OpenRouterDirectDependenciesV2()
		deps.Fake = false
		deps.OpenCredential = func() ([]byte, error) { t.Fatal("historical override read real credential"); return nil, nil }
		var err error
		switch version {
		case 2:
			_, err = executeItem14OpenRouterDirectV2(context.Background(), t.TempDir(), deps)
		case 3:
			_, err = executeItem14OpenRouterDirectV3(context.Background(), t.TempDir(), deps)
		case 4:
			_, err = executeItem14OpenRouterDirectV4(context.Background(), t.TempDir(), deps)
		}
		if err == nil {
			t.Fatal("historical override admitted real execution")
		}
	}
	deps := newFakeItem14OpenRouterDependencies()
	deps.Fake = false
	deps.Prepare = func(context.Context) (item14OpenRouterShadowPreflight, error) {
		t.Fatal("historical override reached preparation")
		return item14OpenRouterShadowPreflight{}, nil
	}
	if _, err := executeItem14OpenRouterShadowWithDependencies(context.Background(), t.TempDir(), deps); err == nil {
		t.Fatal("historical v1 override admitted real execution")
	}
}
