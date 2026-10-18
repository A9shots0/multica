package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func ReviewPrimarySubscriptionReliabilityTrialEvidenceV2(root string) (PrimarySubscriptionTrialReviewV1, error) {
	definition, err := CurrentPrimarySubscriptionReliabilityTrialV2()
	if err != nil {
		return PrimarySubscriptionTrialReviewV1{}, err
	}
	ledgerPath := filepath.Join(root, "trial-ledger.json")
	ledgerRaw, err := os.ReadFile(ledgerPath)
	if err != nil {
		return PrimarySubscriptionTrialReviewV1{}, err
	}
	var ledger primarySubscriptionReviewLedger
	if err := json.Unmarshal(ledgerRaw, &ledger); err != nil {
		return PrimarySubscriptionTrialReviewV1{}, err
	}
	report := PrimarySubscriptionTrialReviewV1{
		SchemaVersion: PrimarySubscriptionReliabilityTrialReviewV2Schema,
		TrialID:       ledger.TrialID, EvidenceSnapshotAt: ledger.CompletedAt,
		SourceLedgerSHA256: reviewHash(ledgerRaw), SourceManifestSHA256: ledger.EvidenceManifestSHA256,
		Verdict: "PASS", SessionCount: len(ledger.Sessions), ObservationCount: len(ledger.Observations), ProviderCalls: ledger.ProviderCalls,
		Retries: ledger.Retries, Fallbacks: ledger.Fallbacks, NativeToolExecutions: ledger.NativeToolExecutions,
		RepositoryAccesses: ledger.RepositoryAccesses, ProductionAccesses: ledger.ProductionAccesses,
		BusinessActions: ledger.BusinessActions, PhoneFarmAccesses: ledger.PhoneFarmAccesses, PermanentSelections: ledger.PermanentSelections,
		ConfigurationSelection: "PROVISIONAL_LOCAL_R01_INLINE_SHARED_EXECUTOR_EXACT_SKILL_ONLY",
		PermanentSeatSelection: "HELD_NO_PERMANENT_ROLE_OR_MODEL_SELECTED",
		EvidenceBackedConclusions: []string{
			"The fresh reliability trial produced six provider-free PASS observations out of 12; all six R01 observations passed and all six R02 observations failed.",
			"Claude and Codex each passed all three R01 repetitions, so the evidence supports the inline shared executor plus exact architecture-assurance skill only and does not distinguish or select a provider lane.",
			"No Product/UI R02 observation passed; Product/UI execution remains unqualified and excluded from the provisional local scope.",
			"The prior 5 PASS / 2 INVALID / 1 FAILED trial remains immutable and is referenced only by its frozen result hash.",
		},
		Limitations: []string{
			"Codex CLI receipts pin the requested model but do not disclose trustworthy actual model identity.",
			"Claude Code is config-evaluated as Sonnet primary with allowed background Haiku; these six receipts reported Sonnet only, which does not erase the transport limitation.",
			"Three fresh repetitions per lane-case cell are useful local reliability evidence but not a permanent model or role selection.",
			"Five R02 observations failed during deterministic controller execution and one failed the hidden verifier after Browser capture; no R02 workflow is provisionally qualified.",
			"Claude cost is an API-equivalent estimate reported by the subscription CLI; Codex supplies no provider cash-cost receipt in this lane.",
		},
	}
	check := func(id string, ok bool, detail string) {
		status := "PASS"
		if !ok {
			status, report.Verdict = "FAIL", "REVISE"
		}
		report.Checks = append(report.Checks, PrimarySubscriptionTrialReviewCheckV1{ID: id, Status: status, Detail: detail})
	}
	authority := sha256.Sum256([]byte(definition.OwnerAuthorization))
	check("authority-and-definition", ledger.SchemaVersion == "silver-mediums.primary-subscription-reliability-trial-ledger.v2" && ledger.TrialID == definition.TrialID && ledger.DefinitionSHA256 == PrimarySubscriptionReliabilityTrialV2SHA256() && ledger.GateSHA256 == definition.GateSHA256 && ledger.OwnerAuthorizationHash == hex.EncodeToString(authority[:]) && definition.PriorTrialResultSHA256 == PrimarySubscriptionRoleTrialResultV1SHA256(), "ledger identity matches v2 authority and binds the immutable v1 result")
	check("terminal-counts", ledger.Status == "complete_verified" && len(ledger.Sessions) == 12 && len(ledger.Observations) == 12 && ledger.MaximumCLISessions == 12 && ledger.ProviderCalls == 12, "exactly 12 one-shot sessions, calls, and terminal observations")
	check("forbidden-effects", ledger.Retries == 0 && ledger.Fallbacks == 0 && ledger.NativeToolExecutions == 0 && ledger.RepositoryAccesses == 0 && ledger.ProductionAccesses == 0 && ledger.BusinessActions == 0 && ledger.PhoneFarmAccesses == 0 && ledger.PermanentSelections == 0 && len(ledger.Corrections) == 0, "all forbidden-effect counters and post-start corrections are zero")

	sessionsOK, rawOK, modelOK, cleanupOK, ceilingOK, callTotal := true, true, true, true, true, 0
	lanes := map[string]*PrimarySubscriptionTrialReviewLaneV1{}
	for index, session := range ledger.Sessions {
		if index >= len(definition.Invocations) {
			sessionsOK = false
			continue
		}
		invocation := definition.Invocations[index]
		sessionsOK = sessionsOK && session.Ordinal == invocation.Ordinal && session.ObservationID == invocation.ObservationID && session.Lane == invocation.Lane && session.Participant == invocation.Participant && session.ParticipantKind == invocation.ParticipantKind && session.Status == "completed" && session.ProviderCallCount == 1 && session.Retries == 0 && !session.Fallbacks && len(session.NativeToolEvents) == 0
		cleanupOK = cleanupOK && session.ProviderWorkdirRemoved
		callTotal += session.ProviderCallCount
		rawOK = rawOK && session.RawResponsePath != "" && session.RawResponseSHA256 != "" && reviewFileHash(filepath.Join(root, filepath.FromSlash(session.RawResponsePath))) == session.RawResponseSHA256
		ceilingOK = ceilingOK && session.InputTokens >= 0 && session.OutputTokens >= 0 && session.ReasoningTokens >= 0 && session.InputTokens+session.OutputTokens+session.ReasoningTokens <= definition.MaximumTotalReportedTokensPerSession && session.OutputTokens+session.ReasoningTokens <= definition.MaximumGeneratedTokensPerSession
		if session.Lane == "claude_code_subscription" {
			modelOK = modelOK && session.RequestedModel == "claude-sonnet-4-6" && session.RequestedEffort == "medium" && session.ObservedModelStatus == "reported_composite_allowed" && containsReviewString(session.ObservedModels, "claude-sonnet-4-6")
			for _, model := range session.ObservedModels {
				modelOK = modelOK && (model == "claude-sonnet-4-6" || model == "claude-haiku-4-5-20251001")
			}
		} else {
			modelOK = modelOK && session.RequestedModel == "gpt-5.6-luna" && session.RequestedEffort == "medium" && len(session.ObservedModels) == 0 && session.ObservedModelStatus == "unavailable_truthful_cli_request_pinned"
		}
		summary := lanes[session.Lane]
		if summary == nil {
			summary = &PrimarySubscriptionTrialReviewLaneV1{Lane: session.Lane}
			lanes[session.Lane] = summary
		}
		summary.Sessions++
		summary.Completed++
		summary.InputTokens += session.InputTokens
		summary.OutputTokens += session.OutputTokens
		summary.CostMicros += session.CostMicros
		summary.WallMillis += session.WallMillis
	}
	if lanes["claude_code_subscription"] != nil {
		lanes["claude_code_subscription"].ObservedModel = "claude-sonnet-4-6_in_six_receipts_composite_transport_limitation_retained"
	}
	if lanes["codex_subscription"] != nil {
		lanes["codex_subscription"].ObservedModel = "unavailable_truthful_requested_gpt-5.6-luna_pinned"
	}
	for _, name := range []string{"claude_code_subscription", "codex_subscription"} {
		if lanes[name] == nil {
			sessionsOK = false
			continue
		}
		report.LaneSummaries = append(report.LaneSummaries, *lanes[name])
	}
	check("session-identity", sessionsOK && callTotal == ledger.ProviderCalls, "all sessions match the frozen order and exactly one call each")
	check("raw-response-hashes", rawOK, "all retained raw-response files match their receipt hashes")
	check("model-attribution", modelOK, "Claude composite allowlist and Codex truthful-unavailable attribution remain explicit")
	check("resource-ceilings", ceilingOK, "all session token receipts remain under the frozen ceilings")
	check("provider-workdir-cleanup", cleanupOK, "all session receipts attest task-local provider-workdir removal")

	observationsOK, artifactsOK, browserOK, browserEvidenceCount := true, true, true, 0
	cells := map[string]*PrimarySubscriptionReliabilityTrialCellV2{}
	for index, observation := range ledger.Observations {
		if index >= len(definition.Invocations) {
			observationsOK = false
			continue
		}
		invocation := definition.Invocations[index]
		observationsOK = observationsOK && observation.ObservationID == invocation.ObservationID && observation.Lane == invocation.Lane && observation.CaseID == invocation.CaseID && observation.Arm == invocation.Arm && len(observation.SessionOrdinals) == 1 && observation.SessionOrdinals[0] == invocation.Ordinal && observation.FixtureRootRemoved && observation.InitialRootRemoved && pathIsAbsent(observation.Workspace.Root) && pathIsAbsent(observation.Workspace.InitialCopy)
		key := observation.Lane + ":" + observation.CaseID
		cell := cells[key]
		if cell == nil {
			cell = &PrimarySubscriptionReliabilityTrialCellV2{Lane: observation.Lane, CaseID: observation.CaseID}
			cells[key] = cell
		}
		cell.Observations++
		observationRoot := filepath.Join(root, "observations", reviewSafeName(observation.ObservationID))
		switch observation.Status {
		case "complete":
			report.PassObservations++
			cell.Passes++
			artifactsOK = artifactsOK && observation.CaseID == "R01" && observation.Grade != nil && observation.Grade.Verdict == "PASS" && observation.ToolReceiptSHA256 != "" && reviewFileHash(filepath.Join(observationRoot, "tool-receipt.json")) == observation.ToolReceiptSHA256
		case "invalid":
			report.InvalidObservations++
			cell.Invalid++
		case "failed":
			report.FailedObservations++
			cell.Failed++
			observationsOK = observationsOK && observation.CaseID == "R02" && observation.FailureClass == "controller_action_failure"
		case "candidate_fail":
			report.FailedObservations++
			cell.Failed++
			artifactsOK = artifactsOK && observation.CaseID == "R02" && observation.FailureClass == "hidden_verifier_failure" && observation.Grade != nil && observation.Grade.Verdict == "FAIL" && observation.ToolReceiptSHA256 != "" && reviewFileHash(filepath.Join(observationRoot, "tool-receipt.json")) == observation.ToolReceiptSHA256
			browserEvidenceCount++
			browserOK = browserOK && reviewBrowserEvidence(observation.BrowserEvidenceRoot)
		default:
			observationsOK = false
		}
	}
	for _, lane := range []string{"claude_code_subscription", "codex_subscription"} {
		for _, caseID := range []string{"R01", "R02"} {
			if cells[lane+":"+caseID] == nil {
				observationsOK = false
				continue
			}
			report.CellSummaries = append(report.CellSummaries, *cells[lane+":"+caseID])
		}
	}
	check("observation-terminal-cleanup", observationsOK && report.PassObservations == 6 && report.InvalidObservations == 0 && report.FailedObservations == 6, "all observations are terminal and cleaned with the observed 6/0/6 result")
	check("hidden-verdict-artifacts", artifactsOK, "six R01 PASS artifacts and the Browser-reached R02 FAIL artifact remain hash-bound")
	check("browser-evidence", browserOK && browserEvidenceCount == 1, "the sole Browser-reached R02 observation retains three captures and no capture token")
	manifest, manifestErr := reviewEvidenceManifest(root)
	check("evidence-manifest", manifestErr == nil && reviewHash(manifest) == ledger.EvidenceManifestSHA256, "sealed evidence manifest recomputes exactly")
	if report.Verdict != "PASS" {
		return report, fmt.Errorf("reliability trial v2 evidence review failed")
	}
	return report, nil
}
