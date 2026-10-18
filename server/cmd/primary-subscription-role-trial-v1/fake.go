package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

type fakeTrialInvoker struct {
	contracts agentqualification.QualificationContracts
}

func (fake fakeTrialInvoker) Invoke(_ context.Context, _ string, invocation agentqualification.PrimarySubscriptionTrialInvocationV1, prompt string, schema []byte) trialSessionReceipt {
	receipt := (trialProtocol{contracts: fake.contracts}).baseSessionReceipt(invocation, prompt, schema)
	started := time.Date(2026, 9, 14, 20, 0, invocation.Ordinal, 0, time.UTC)
	receipt.StartedAt, receipt.CompletedAt, receipt.WallMillis = started.Format(time.RFC3339Nano), started.Add(time.Second).Format(time.RFC3339Nano), 1000
	receipt.ProviderCallCount, receipt.ProviderWorkdirRemoved, receipt.Status = 1, true, "completed"
	receipt.InputTokens, receipt.OutputTokens, receipt.CostMicros = 100+int64(invocation.Ordinal), 20, 0
	if invocation.Lane == "claude_code_subscription" {
		receipt.ObservedModels = []string{"claude-haiku-4-5-20251001", "claude-sonnet-4-6"}
		receipt.ObservedModelStatus = "reported_composite_allowed"
		receipt.ModelUsage = []trialModelUsage{{Model: "claude-haiku-4-5-20251001", InputTokens: 1}, {Model: "claude-sonnet-4-6", InputTokens: receipt.InputTokens - 1, OutputTokens: receipt.OutputTokens}}
		receipt.CostBoundary = "fake_provider"
	} else {
		receipt.ObservedModelStatus = "unavailable_truthful_cli_request_pinned"
		receipt.CostBoundary = "fake_provider"
	}
	var output agentqualification.PrimarySubscriptionTrialParticipantOutputV1
	if invocation.ParticipantKind == "specialist" {
		output.Evidence = json.RawMessage(`{"boundary":"synthetic fixture only","constraints":["preserve authority","use controller actions","verify local result"],"decision":"PROCEED_WITH_CONSTRAINTS"}`)
	} else if invocation.CaseID == "R01" {
		output.Actions = fakeR01Actions()
	} else {
		output.Actions = fakeR02Actions()
	}
	candidate, _ := json.Marshal(output)
	receipt.Candidate = candidate
	receipt.CandidateOutputSHA256 = hashBytes(candidate)
	receipt.RawResponse = []byte(fmt.Sprintf(`{"fake_provider":true,"ordinal":%d,"candidate_sha256":%q}`, invocation.Ordinal, receipt.CandidateOutputSHA256))
	return receipt
}

func fakeR01Actions() []agentqualification.RealisticCandidateAction {
	return []agentqualification.RealisticCandidateAction{
		{Kind: "run_check", Check: "python -m unittest discover -s tests -v"},
		{Kind: "edit", Path: "consumer_faulty/normalize.py", Contents: "def normalize_role(role: str) -> str:\n    return role\n"},
		{Kind: "edit", Path: "EVIDENCE.md", Contents: "Local-only proof: failing test before and passing test after. CRM authority, correct consumer, Phone Farm contract, repositories, production, and external systems were untouched.\n"},
		{Kind: "run_check", Check: "python -m unittest discover -s tests -v"},
		{Kind: "finish", Summary: "repaired only the synthetic faulty consumer and verified the healthy control"},
	}
}

func fakeR02Actions() []agentqualification.RealisticCandidateAction {
	return []agentqualification.RealisticCandidateAction{
		{Kind: "edit", Path: "report/report.js", Contents: "export function canonicalTotal(data) {\n  return data.published_posts.reduce((sum, post) => sum + post.views, 0);\n}\n\nexport function renderReport(data) {\n  document.querySelector(\"#total\").textContent = canonicalTotal(data);\n  document.querySelector(\"#cards\").innerHTML = data.published_posts\n    .map((post) => `<article class=\"card\"><h2>${post.id}</h2><p>${post.views} views</p></article>`)\n    .join(\"\");\n}\n\nif (typeof document !== \"undefined\") {\n  fetch(\"./data.json\").then((response) => response.json()).then(renderReport);\n}\n"},
		{Kind: "edit", Path: "report/styles.css", Contents: "body { background: #111318; color: #f5f7fb; font-family: system-ui, sans-serif; margin: 0; }\nmain { margin: 40px auto; width: min(980px, calc(100% - 32px)); }\n#cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(220px, 100%), 1fr)); gap: 20px; }\n.card { background: #20242c; border: 1px solid #343a46; border-radius: 14px; padding: 20px; min-width: 0; }\n"},
		{Kind: "run_check", Check: "node tests/report.test.mjs"},
		{Kind: "run_check", Check: "local-static-preview report"},
		{Kind: "browser"},
		{Kind: "finish", Summary: "truthful four-card report verified locally at all three required viewports"},
	}
}

type fakeTrialBrowser struct{ runFamily string }

func (collector fakeTrialBrowser) Capture(_ context.Context, _ string, _ string) ([]agentqualification.RealisticBrowserProof, error) {
	proofs := []agentqualification.RealisticBrowserProof{}
	for _, viewport := range []string{"narrow", "medium", "wide"} {
		proofs = append(proofs, agentqualification.RealisticBrowserProof{Viewport: viewport, CaseID: "R02", RunFamilyID: collector.runFamily, WorkspaceTreeSHA256: strings.Repeat("b", 64), RequestSHA256: strings.Repeat("c", 64), ScreenshotArtifactPath: viewport + ".jpg", ScreenshotSHA256: strings.Repeat("d", 64), DOMSnapshotArtifactPath: viewport + ".json", DOMSnapshotSHA256: strings.Repeat("e", 64), RenderedTotal: 47, RenderedCardCount: 4, HorizontalOverflow: false})
	}
	return proofs, nil
}

const offlineFakeRuntimeVersion = "offline_fake_no_executable"

func fakeTrialRuntimeIdentity(lane string) trialRuntimeIdentity {
	identity := "offline-fake:" + lane
	return trialRuntimeIdentity{Lane: lane, ExecutablePath: identity, ExecutableSHA256: hashBytes([]byte(identity)), Version: offlineFakeRuntimeVersion}
}

func validateTrialRuntimeMode(ledger trialLedger) error {
	for _, runtime := range ledger.Runtimes {
		fake := fakeTrialRuntimeIdentity(runtime.Lane)
		if ledger.OfflineFake {
			if runtime != fake {
				return fmt.Errorf("offline fake ledger has non-fake runtime identity")
			}
		} else if runtime.Version == fake.Version || runtime.ExecutablePath == fake.ExecutablePath || runtime.ExecutableSHA256 == fake.ExecutableSHA256 {
			return fmt.Errorf("fake runtime identity requires offline fake ledger marker")
		}
	}
	return nil
}

func validateTrialExecutionMode(ledger trialLedger, fake bool) error {
	if err := validateTrialRuntimeMode(ledger); err != nil {
		return err
	}
	if ledger.OfflineFake != fake {
		return fmt.Errorf("trial execution mode differs from prepared ledger")
	}
	return nil
}

func (protocol trialProtocol) runFakeTrial(ctx context.Context, root string) (trialLedger, error) {
	if _, err := protocol.prepareTrialWithRuntimes(root, true, func() ([]trialRuntimeIdentity, error) {
		return []trialRuntimeIdentity{fakeTrialRuntimeIdentity("claude_code_subscription"), fakeTrialRuntimeIdentity("codex_subscription")}, nil
	}); err != nil {
		return trialLedger{}, err
	}
	definition, err := protocol.activeTrialDefinition()
	if err != nil {
		return trialLedger{}, err
	}
	for {
		ledger, err := loadTrialLedger(root)
		if err != nil {
			return ledger, err
		}
		if err := protocol.validateTrialLedger(definition, ledger, false); err != nil {
			return ledger, err
		}
		if pending := pendingBrowserObservation(ledger); pending != "" {
			index := -1
			for i := range ledger.Observations {
				if ledger.Observations[i].ObservationID == pending {
					index = i
				}
			}
			observation := &ledger.Observations[index]
			state, executeErr := agentqualification.ExecutePrimarySubscriptionTrialActionsV1(observation.Workspace, observation.RunFamilyID, observation.Actions, observation.ExecutionState, fakeTrialBrowser{runFamily: observation.RunFamilyID})
			if executeErr != nil {
				return ledger, executeErr
			}
			observation.ExecutionState, observation.Status = state, state.Controller.TerminalState
			if err := closeTrialObservation(root, &ledger, index); err != nil {
				return ledger, err
			}
			if err := saveTrialLedger(root, ledger); err != nil {
				return ledger, err
			}
			continue
		}
		if len(ledger.Sessions) == definition.MaximumCLISessions {
			ledger.Status, ledger.CompletedAt = "complete", time.Now().UTC().Format(time.RFC3339Nano)
			if err := saveTrialLedger(root, ledger); err != nil {
				return ledger, err
			}
			return protocol.verifyTrial(root)
		}
		if _, err := protocol.executeNextObservation(ctx, root, fakeTrialInvoker{contracts: protocol.contracts}); err != nil {
			return ledger, err
		}
	}
}
