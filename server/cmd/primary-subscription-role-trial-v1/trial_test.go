package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func TestClaudeTrialCompositeReceiptIsExplicitAndBounded(t *testing.T) {
	raw := []byte(`{"type":"result","subtype":"success","is_error":false,"session_id":"s1","structured_output":{"evidence":{"ok":true}},"total_cost_usd":0.001,"usage":{"input_tokens":10,"cache_creation_input_tokens":2,"cache_read_input_tokens":3,"output_tokens":4},"modelUsage":{"claude-sonnet-4-6":{"inputTokens":9,"outputTokens":4,"costUSD":0.0009},"claude-haiku-4-5-20251001":{"inputTokens":6,"outputTokens":0,"costUSD":0.0001}}}`)
	receipt, output, err := parseClaudeTrialResult(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(receipt.ObservedModels) != 2 || receipt.ProviderCallCount != 1 || receipt.InputTokens != 15 || receipt.OutputTokens != 4 || receipt.CostMicros != 1000 || !json.Valid(output) {
		t.Fatalf("unexpected Claude receipt: %+v", receipt)
	}
	altered := []byte(`{"type":"result","subtype":"success","is_error":false,"session_id":"s1","structured_output":{"evidence":{"ok":true}},"total_cost_usd":0.001,"usage":{"input_tokens":10,"output_tokens":4},"modelUsage":{"claude-sonnet-4-6":{},"third-model":{}}}`)
	if _, _, err := parseClaudeTrialResult(altered); err == nil {
		t.Fatal("third Claude model passed")
	}
}

func TestCodexTrialToolEventFailsClosed(t *testing.T) {
	raw := []byte("{\"type\":\"thread.started\",\"thread_id\":\"t\"}\n{\"type\":\"turn.started\"}\n{\"type\":\"item.completed\",\"item\":{\"type\":\"command_execution\"}}\n{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":10,\"output_tokens\":2}}\n")
	parsed := parseCodexTrialEvents(raw)
	if !parsed.Completed || len(parsed.NativeToolEvents) != 1 || parsed.ProviderCallCount != 1 {
		t.Fatalf("Codex tool event not preserved: %+v", parsed)
	}
}

func TestCodexTrialErrorDiagnosticIsNotAToolEvent(t *testing.T) {
	raw := []byte("{\"type\":\"thread.started\",\"thread_id\":\"t\"}\n{\"type\":\"item.completed\",\"item\":{\"type\":\"error\"}}\n{\"type\":\"turn.started\"}\n{\"type\":\"turn.failed\"}\n")
	parsed := parseCodexTrialEvents(raw)
	if len(parsed.NativeToolEvents) != 0 || parsed.ProviderCallCount != 1 || parsed.Failure != "turn.failed" {
		t.Fatalf("Codex diagnostic classification drifted: %+v", parsed)
	}
}

func TestCodexStrictSchemasRequireEveryProperty(t *testing.T) {
	definition, err := offlineTrial(t).contracts.CurrentPrimarySubscriptionRoleTrialV1()
	if err != nil {
		t.Fatal(err)
	}
	for _, invocation := range definition.Invocations {
		if invocation.Lane != "codex_subscription" {
			continue
		}
		raw, err := offlineTrial(t).contracts.PrimarySubscriptionTrialResponseSchemaV1(invocation)
		if err != nil {
			t.Fatal(err)
		}
		var schema map[string]any
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatal(err)
		}
		assertStrictSchemaPropertiesRequired(t, schema)
	}
}

func assertStrictSchemaPropertiesRequired(t *testing.T, value any) {
	t.Helper()
	object, ok := value.(map[string]any)
	if !ok {
		return
	}
	if properties, ok := object["properties"].(map[string]any); ok {
		required, _ := object["required"].([]any)
		set := map[string]bool{}
		for _, entry := range required {
			set[entry.(string)] = true
		}
		for name := range properties {
			if !set[name] {
				t.Fatalf("schema property %s is not required", name)
			}
		}
	}
	for _, nested := range object {
		switch typed := nested.(type) {
		case map[string]any:
			assertStrictSchemaPropertiesRequired(t, typed)
		case []any:
			for _, entry := range typed {
				assertStrictSchemaPropertiesRequired(t, entry)
			}
		}
	}
}

func TestOpaqueCodexAuthCopyUsesDisposableDestination(t *testing.T) {
	sourceRoot := t.TempDir()
	secret := []byte("opaque-test-auth-material")
	if err := os.WriteFile(filepath.Join(sourceRoot, "auth.json"), secret, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", sourceRoot)
	destinationRoot := t.TempDir()
	destination := filepath.Join(destinationRoot, "auth.json")
	if err := copyOpaqueCodexAuth(destination); err != nil {
		t.Fatal(err)
	}
	copied, err := os.ReadFile(destination)
	if err != nil || string(copied) != string(secret) {
		t.Fatal("opaque authentication capsule copy mismatch")
	}
	if err := copyOpaqueCodexAuth(destination); err == nil {
		t.Fatal("opaque authentication capsule overwrite passed")
	}
}

func TestTrialSessionTokenCeilingsFailClosed(t *testing.T) {
	definition, err := offlineTrial(t).contracts.CurrentPrimarySubscriptionRoleTrialV1()
	if err != nil {
		t.Fatal(err)
	}
	if err := validateTrialSessionCeilings(definition, trialSessionReceipt{InputTokens: 1, OutputTokens: definition.MaximumGeneratedTokensPerSession}); err != nil {
		t.Fatal(err)
	}
	if err := validateTrialSessionCeilings(definition, trialSessionReceipt{InputTokens: definition.MaximumTotalReportedTokensPerSession, OutputTokens: 1}); err == nil {
		t.Fatal("total token ceiling overflow passed")
	}
	if err := validateTrialSessionCeilings(definition, trialSessionReceipt{OutputTokens: definition.MaximumGeneratedTokensPerSession + 1}); err == nil {
		t.Fatal("generated token ceiling overflow passed")
	}
}

func TestCodexTransportCorrectionPreservesFailedSessionAndRejectsReplay(t *testing.T) {
	definition, err := offlineTrial(t).contracts.CurrentPrimarySubscriptionRoleTrialV1()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	ledger := testPreparedLedger(definition)
	ledger.ProviderCalls, ledger.NativeToolExecutions = 7, 2
	ledger.Observations = make([]trialObservation, 5)
	for _, invocation := range definition.Invocations[:7] {
		session := testCompletedSession(t, definition, invocation)
		ledger.Sessions = append(ledger.Sessions, session)
	}
	raw := []byte("{\"type\":\"thread.started\",\"thread_id\":\"t\"}\n{\"type\":\"item.completed\",\"item\":{\"type\":\"error\"}}\n{\"type\":\"item.completed\",\"item\":{\"type\":\"error\"}}\n{\"type\":\"turn.started\"}\n{\"type\":\"turn.failed\"}\n")
	rawRelative := filepath.ToSlash(filepath.Join("sessions", "07", "raw-events.jsonl"))
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, filepath.FromSlash(rawRelative))), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rawRelative)), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	failed := &ledger.Sessions[6]
	failed.Status, failed.FailureClass, failed.FailureDetail = "invalid", "provider_or_transport_failure", "exit status 1"
	failed.RawResponsePath, failed.RawResponseSHA256 = rawRelative, hashBytes(raw)
	failed.NativeToolEvents = []string{"item.completed:error", "item.completed:error"}
	if err := saveTrialLedger(root, ledger); err != nil {
		t.Fatal(err)
	}
	corrected, err := offlineTrial(t).applyCodexTransportCorrection(root)
	if err != nil {
		t.Fatal(err)
	}
	if corrected.Sessions[6].Status != "invalid" || corrected.Sessions[6].FailureClass != "provider_or_transport_failure" || len(corrected.Sessions[6].NativeToolEvents) != 0 || corrected.NativeToolExecutions != 0 || len(corrected.Corrections) != 1 {
		t.Fatalf("unexpected corrected ledger: %+v", corrected)
	}
	replayed, err := offlineTrial(t).applyCodexTransportCorrection(root)
	if err != nil || len(replayed.Corrections) != 1 {
		t.Fatalf("provider-free correction verification replay failed: %v", err)
	}
	record, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(corrected.Corrections[0].RecordPath)))
	if err != nil || !strings.Contains(string(record), `"provider_calls": 0`) || strings.Contains(string(record), "opaque-test-auth-material") {
		t.Fatal("correction record boundary invalid")
	}
}

func TestFakeTrialRunsExactlyTwelveAndRejectsReplay(t *testing.T) {
	root := filepath.Join(t.TempDir(), "fake")
	ledger, err := offlineTrial(t).runFakeTrial(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger.Sessions) != 12 || len(ledger.Observations) != 8 || ledger.Status != "complete_verified" || ledger.NativeToolExecutions != 0 {
		t.Fatalf("unexpected fake ledger: sessions=%d observations=%d status=%s", len(ledger.Sessions), len(ledger.Observations), ledger.Status)
	}
	for _, observation := range ledger.Observations {
		if !observation.FixtureRootRemoved || !observation.InitialRootRemoved {
			t.Fatalf("fixture cleanup missing: %+v", observation)
		}
		if _, err := os.Stat(observation.Workspace.Root); !os.IsNotExist(err) {
			t.Fatalf("fixture still exists: %s", observation.Workspace.Root)
		}
	}
	if _, err := offlineTrial(t).executeNextObservation(context.Background(), root, fakeTrialInvoker{}); err == nil {
		t.Fatal("completed campaign replay passed")
	}
	if _, err := offlineTrial(t).verifyTrial(root); err != nil {
		t.Fatalf("provider-free verification replay failed: %v", err)
	}
}

func TestReliabilityV2FakeTrialRunsTwelveFreshObservationsAndRejectsCrossVersionReplay(t *testing.T) {
	previous := trialDefinitionVariant
	trialDefinitionVariant = "reliability-v2"
	t.Cleanup(func() { trialDefinitionVariant = previous })

	root := filepath.Join(t.TempDir(), "reliability-v2-fake")
	ledger, err := offlineTrial(t).runFakeTrial(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if ledger.SchemaVersion != reliabilityTrialLedgerSchema || len(ledger.Sessions) != 12 || len(ledger.Observations) != 12 || ledger.ProviderCalls != 12 || ledger.Status != "complete_verified" || len(ledger.Corrections) != 0 {
		t.Fatalf("unexpected reliability v2 ledger: %+v", ledger)
	}
	for _, observation := range ledger.Observations {
		if observation.Status != "complete" || !observation.FixtureRootRemoved || !observation.InitialRootRemoved {
			t.Fatalf("reliability v2 observation not terminal and clean: %+v", observation)
		}
	}
	if _, err := offlineTrial(t).executeNextObservation(context.Background(), root, fakeTrialInvoker{}); err == nil {
		t.Fatal("completed reliability v2 campaign replay passed")
	}
	trialDefinitionVariant = "v1"
	if _, err := offlineTrial(t).verifyTrial(root); err == nil {
		t.Fatal("reliability v2 ledger verified under v1 definition")
	}
	trialDefinitionVariant = "reliability-v2"
	if _, err := offlineTrial(t).verifyTrial(root); err != nil {
		t.Fatalf("reliability v2 provider-free verification failed: %v", err)
	}
}

func TestTrialLedgerRejectsAlteredAuthority(t *testing.T) {
	definition, err := offlineTrial(t).contracts.CurrentPrimarySubscriptionRoleTrialV1()
	if err != nil {
		t.Fatal(err)
	}
	ledger := testPreparedLedger(definition)
	if err := offlineTrial(t).validateTrialLedger(definition, ledger, false); err != nil {
		t.Fatal(err)
	}
	ledger.GateSHA256 = "altered"
	if err := offlineTrial(t).validateTrialLedger(definition, ledger, false); err == nil {
		t.Fatal("altered gate passed")
	}
}

func TestWriteAtomicReplacesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.json")
	if err := os.WriteFile(path, []byte("before"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeAtomic(path, []byte("after")); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != "after" {
		t.Fatalf("replace-existing mismatch: %q %v", raw, err)
	}
}

func TestReliabilityV2LedgerRejectsAlteredCallAccountingAndSelector(t *testing.T) {
	previous := trialDefinitionVariant
	trialDefinitionVariant = "reliability-v2"
	t.Cleanup(func() { trialDefinitionVariant = previous })
	definition, _ := agentqualification.CurrentPrimarySubscriptionReliabilityTrialV2()
	ledger := testPreparedLedger(definition)
	invocation := definition.Invocations[0]
	lane := laneFor(definition, invocation.Lane)
	schema, err := offlineTrial(t).activeTrialResponseSchema(invocation)
	if err != nil {
		t.Fatal(err)
	}
	ledger.Sessions = []trialSessionReceipt{{Ordinal: 1, ObservationID: invocation.ObservationID, Lane: invocation.Lane, Participant: invocation.Participant, ParticipantKind: invocation.ParticipantKind, Status: "reserved_no_replay", RequestedModel: lane.RequestedModel, RequestedEffort: lane.Effort, ObservedModels: []string{}, NativeToolEvents: []string{}, PromptSHA256: strings.Repeat("a", 64), OutputSchemaSHA256: hashBytes(schema)}}
	if err := offlineTrial(t).validateTrialLedger(definition, ledger, false); err != nil {
		t.Fatal(err)
	}
	ledger.ProviderCalls, ledger.Sessions[0].ProviderCallCount = 1, 1
	if err := offlineTrial(t).validateTrialLedger(definition, ledger, false); err == nil {
		t.Fatal("reserved session provider call passed")
	}
	ledger.ProviderCalls, ledger.Sessions[0].ProviderCallCount = 0, 0
	ledger.OwnerAuthorizationSHA256 = strings.Repeat("f", 64)
	if err := offlineTrial(t).validateTrialLedger(definition, ledger, false); err == nil {
		t.Fatal("altered owner authorization receipt passed")
	}
	ledger.OwnerAuthorizationSHA256 = hashBytes([]byte(definition.OwnerAuthorization))
	root := t.TempDir()
	if err := saveTrialLedger(root, ledger); err != nil {
		t.Fatal(err)
	}
	trialDefinitionVariant = "v1"
	if _, err := offlineTrial(t).prepareTrial(root); err == nil {
		t.Fatal("existing reliability v2 ledger prepared under v1 selector")
	}
}

func TestReliabilityV2ManifestCannotBeResealedAfterMutation(t *testing.T) {
	previous := trialDefinitionVariant
	trialDefinitionVariant = "reliability-v2"
	t.Cleanup(func() { trialDefinitionVariant = previous })
	root := filepath.Join(t.TempDir(), "reliability-v2-manifest")
	ledger, err := offlineTrial(t).runFakeTrial(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if ledger.EvidenceManifestSHA256 == "" {
		t.Fatal("fake trial did not seal an evidence manifest")
	}
	artifact := filepath.Join(root, "observations", safeName(ledger.Observations[0].ObservationID), "grade.json")
	if err := os.WriteFile(artifact, []byte("altered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := offlineTrial(t).verifyTrial(root); err == nil {
		t.Fatal("changed evidence acquired a replacement manifest seal")
	}
}

func testPreparedLedger(definition agentqualification.PrimarySubscriptionRoleTrialV1) trialLedger {
	runtimes := make([]trialRuntimeIdentity, 0, len(definition.Lanes))
	for _, lane := range definition.Lanes {
		runtimes = append(runtimes, trialRuntimeIdentity{Lane: lane.Lane, ExecutablePath: "C:/fixture/" + lane.Lane, ExecutableSHA256: strings.Repeat("a", 64), Version: "fixture-version"})
	}
	return trialLedger{SchemaVersion: activeTrialLedgerSchema(), TrialID: definition.TrialID, DefinitionSHA256: activeTrialDefinitionSHA256(), GateSHA256: definition.GateSHA256, OwnerAuthorizationSHA256: hashBytes([]byte(definition.OwnerAuthorization)), Status: "prepared_authorized", Runtimes: runtimes, Sessions: []trialSessionReceipt{}, Observations: []trialObservation{}, MaximumCLISessions: definition.MaximumCLISessions}
}

func testCompletedSession(t *testing.T, definition agentqualification.PrimarySubscriptionRoleTrialV1, invocation agentqualification.PrimarySubscriptionTrialInvocationV1) trialSessionReceipt {
	t.Helper()
	lane := laneFor(definition, invocation.Lane)
	schema, err := offlineTrial(t).activeTrialResponseSchema(invocation)
	if err != nil {
		t.Fatal(err)
	}
	session := trialSessionReceipt{Ordinal: invocation.Ordinal, ObservationID: invocation.ObservationID, Lane: invocation.Lane, Participant: invocation.Participant, ParticipantKind: invocation.ParticipantKind, Status: "completed", RequestedModel: lane.RequestedModel, RequestedEffort: lane.Effort, ObservedModels: []string{}, NativeToolEvents: []string{}, PromptSHA256: strings.Repeat("b", 64), OutputSchemaSHA256: hashBytes(schema), RawResponsePath: "fixture-response", RawResponseSHA256: strings.Repeat("c", 64), CandidateOutputSHA256: strings.Repeat("d", 64), ProviderCallCount: 1, ProviderWorkdirRemoved: true}
	if invocation.Lane == "claude_code_subscription" {
		session.ObservedModelStatus = observedModelStatusForLane(invocation.Lane)
		session.ObservedModels = []string{lane.RequestedModel}
	} else {
		session.ObservedModelStatus = observedModelStatusForLane(invocation.Lane)
	}
	return session
}
