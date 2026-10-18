package main

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/multica-ai/multica/server/internal/testutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func TestR02RepeatBrowserInvalidationAcceptsOnlyExactPaidBlockedState(t *testing.T) {
	admission, hash, err := agentqualification.CurrentR02RepeatAdmission()
	if err != nil {
		t.Fatal(err)
	}
	candidate := agentqualification.RealisticCandidate{ConfigurationID: admission.ConfigurationID, Model: admission.Model, Effort: admission.Effort, ServingProvider: admission.ServingProvider}
	campaign := r02RepeatCampaignState{AdmissionID: admission.AdmissionID, AdmissionSHA256: hash}
	state := realisticRunState{
		ClaimID: "exact-run", Candidate: candidate,
		Workspace: agentqualification.RealisticCaseWorkspace{CaseID: "R02"},
		Controller: agentqualification.RealisticControllerResult{
			CaseID: "R02", RunFamilyID: "exact-run", Attempt: 1, TerminalState: "awaiting_browser_evidence",
			ProviderCalls: 1, ProviderReceipts: [][]byte{[]byte(`{"receipt":true}`)}, ProviderReceiptHashes: []string{strings.Repeat("a", 64)},
			InputTokens: 10, OutputTokens: 2, ReasoningTokens: 1, CostMicros: 20,
			ProviderAccountingStatus: "exact", ProviderTokenAccountingStatus: "exact", ProviderCostAccountingStatus: "exact",
		},
	}
	if err := validateR02RepeatBrowserInvalidation(admission, hash, campaign, state); err != nil {
		t.Fatal(err)
	}
	mutations := []struct {
		name string
		edit func(*r02RepeatCampaignState, *realisticRunState)
	}{
		{"completed campaign", func(c *r02RepeatCampaignState, _ *realisticRunState) {
			c.Result = &agentqualification.R02RepeatRunRecord{}
		}},
		{"wrong terminal", func(_ *r02RepeatCampaignState, s *realisticRunState) { s.Controller.TerminalState = "completed" }},
		{"existing proof", func(_ *r02RepeatCampaignState, s *realisticRunState) {
			s.Controller.BrowserProofs = []agentqualification.RealisticBrowserProof{{Viewport: "narrow"}}
		}},
		{"ambiguous accounting", func(_ *r02RepeatCampaignState, s *realisticRunState) { s.Controller.AmbiguousProviderCalls = 1 }},
		{"pending provider", func(_ *r02RepeatCampaignState, s *realisticRunState) {
			s.Controller.PendingProviderCall = &agentqualification.RealisticProviderCallCheckpoint{}
		}},
		{"zero paid calls", func(_ *r02RepeatCampaignState, s *realisticRunState) {
			s.Controller.ProviderCalls = 0
			s.Controller.ProviderReceipts = nil
			s.Controller.ProviderReceiptHashes = nil
		}},
		{"wrong candidate", func(_ *r02RepeatCampaignState, s *realisticRunState) { s.Candidate.ConfigurationID = "other" }},
		{"over cap", func(_ *r02RepeatCampaignState, s *realisticRunState) {
			s.Controller.CostMicros = admission.MaxSpendMicros + 1
		}},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			changedCampaign, changedState := campaign, state
			mutation.edit(&changedCampaign, &changedState)
			if err := validateR02RepeatBrowserInvalidation(admission, hash, changedCampaign, changedState); err == nil {
				t.Fatal("invalid administrative target was accepted")
			}
		})
	}
}

func TestR02RepeatBrowserInvalidationSealedEvidence(t *testing.T) {
	repoRoot := testutil.QualificationArchiveRoot(t, "r02-browser-seal-v1")
	root := filepath.Join(repoRoot, ".qualification-evidence", "realistic-r02-repeat-v1")
	var campaign r02RepeatCampaignState
	if err := readJSONFile(filepath.Join(root, "r02-repeat-campaign.json"), &campaign); err != nil {
		t.Fatal(err)
	}
	if campaign.Result == nil || campaign.Result.Outcome != "invalid" || campaign.Result.FailureClass != "non_safety_harness_imperfection" || campaign.Result.Observation.ProviderCalls != 8 || campaign.Result.Observation.TotalTokens != 25_479 || campaign.Result.Observation.CostMicros != 68_849 {
		t.Fatalf("sealed campaign result drifted: %+v", campaign.Result)
	}
	evidenceRoot := filepath.Join(root, "observations", safeArtifactName(campaign.Result.Observation.ConfigurationID), "r02", "evidence")
	failurePath := filepath.Join(evidenceRoot, "failure-seal", "observation.json")
	if err := agentqualification.VerifyRealisticPaidFailureSeal(failurePath, campaign.Result.Observation); err != nil {
		t.Fatal(err)
	}
	var correction r02RepeatPostBatchCorrection
	if err := readJSONFile(filepath.Join(evidenceRoot, "post-batch-browser-capability-correction.json"), &correction); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(root, "observations", safeArtifactName(campaign.Result.Observation.ConfigurationID), "r02", "run-state.json")
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(raw)
	if correction.OriginalRunStateSHA256 != hex.EncodeToString(hash[:]) || !correction.NoProviderCall || !correction.NoRetry || len(correction.AccidentalExecutions) != 2 || correction.TotalAccidentalCostUSD != "0.1193255" {
		t.Fatalf("post-batch correction receipt drifted: %+v", correction)
	}
	if correction.AccidentalExecutions[0].CompletedCalls+correction.AccidentalExecutions[1].CompletedCalls != 14 || correction.AccidentalExecutions[1].PendingZeroTokenCall != 1 {
		t.Fatalf("accidental executions were merged or incompletely recorded: %+v", correction.AccidentalExecutions)
	}
}
