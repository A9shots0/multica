package main

import (
	"context"
	"github.com/multica-ai/multica/server/internal/agentqualification"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func offlineTrial(t *testing.T) trialProtocol {
	t.Helper()
	q, err := agentqualification.HistoricalQualificationContracts()
	if err != nil {
		t.Fatal(err)
	}
	return trialProtocol{contracts: q}
}
func TestHistoricalTrialRejectsRealInvokerBeforeClaims(t *testing.T) {
	if _, err := offlineTrial(t).executeNextObservation(context.Background(), t.TempDir(), realTrialInvoker{}); err == nil {
		t.Fatal("historical trial admitted real invoker")
	}
}

func TestHistoricalLivePreparationRejectsBeforeRuntimeResolution(t *testing.T) {
	called := false
	_, err := offlineTrial(t).prepareTrialWithRuntimes(t.TempDir(), false, func() ([]trialRuntimeIdentity, error) {
		called = true
		return nil, nil
	})
	if err == nil || !strings.Contains(err.Error(), "cannot prepare live runtimes") || called {
		t.Fatalf("historical live preparation err=%v resolverCalled=%v", err, called)
	}
}

func TestOfflineFakeLedgerCannotBecomeLiveExecution(t *testing.T) {
	for _, mutation := range []string{"unchanged", "marker removed", "identity changed"} {
		t.Run(mutation, func(t *testing.T) {
			root := t.TempDir()
			ledger := trialLedger{OfflineFake: true, Runtimes: []trialRuntimeIdentity{fakeTrialRuntimeIdentity("claude_code_subscription"), fakeTrialRuntimeIdentity("codex_subscription")}}
			expected := "execution mode differs"
			if mutation == "marker removed" {
				ledger.OfflineFake = false
				expected = "requires offline fake ledger marker"
			}
			if mutation == "identity changed" {
				ledger.Runtimes[0].Version = "claimed-live"
				expected = "non-fake runtime identity"
			}
			if err := saveTrialLedger(root, ledger); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "trial-ledger.json")
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			// Zero/current contracts exercise the execution fence before any
			// current-source custody failure could mask a broken mode check.
			_, err = (trialProtocol{}).executeNextObservation(context.Background(), root, realTrialInvoker{})
			if err == nil || !strings.Contains(err.Error(), expected) {
				t.Fatalf("expected %q rejection, got %v", expected, err)
			}
			after, err := os.ReadFile(path)
			if err != nil || string(before) != string(after) {
				t.Fatal("rejected execution changed ledger")
			}
		})
	}
}

func TestFakeTrialCannotExtendExistingLiveLedger(t *testing.T) {
	protocol := offlineTrial(t)
	definition, err := protocol.activeTrialDefinition()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	ledger := testPreparedLedger(definition)
	if err := validateTrialExecutionMode(ledger, false); err != nil {
		t.Fatalf("legacy unmarked live ledger rejected: %v", err)
	}
	if err := saveTrialLedger(root, ledger); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "trial-ledger.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := protocol.runFakeTrial(context.Background(), root); err == nil || !strings.Contains(err.Error(), "execution mode differs") {
		t.Fatalf("fake trial admitted live ledger: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(before) != string(after) {
		t.Fatal("rejected fake execution changed live ledger")
	}
}
