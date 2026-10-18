package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

const trialLedgerSchema = "silver-mediums.primary-subscription-role-trial-ledger.v1"
const reliabilityTrialLedgerSchema = "silver-mediums.primary-subscription-reliability-trial-ledger.v2"
const codexTransportCorrectionID = "item14-codex-transport-correction-20260914"

type trialRuntimeIdentity struct {
	Lane             string `json:"lane"`
	ExecutablePath   string `json:"executable_path"`
	ExecutableSHA256 string `json:"executable_sha256"`
	Version          string `json:"version"`
}

type trialModelUsage struct {
	Model        string `json:"model"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	CostMicros   int64  `json:"cost_micros"`
}

type trialSessionReceipt struct {
	Ordinal                int               `json:"ordinal"`
	ObservationID          string            `json:"observation_id"`
	Lane                   string            `json:"lane"`
	Participant            string            `json:"participant"`
	ParticipantKind        string            `json:"participant_kind"`
	Status                 string            `json:"status"`
	RequestedModel         string            `json:"requested_model"`
	RequestedEffort        string            `json:"requested_effort"`
	ObservedModels         []string          `json:"observed_models"`
	ObservedModelStatus    string            `json:"observed_model_status"`
	ModelUsage             []trialModelUsage `json:"model_usage,omitempty"`
	InputTokens            int64             `json:"input_tokens"`
	CachedInputTokens      int64             `json:"cached_input_tokens"`
	OutputTokens           int64             `json:"output_tokens"`
	ReasoningTokens        int64             `json:"reasoning_tokens"`
	CostMicros             int64             `json:"cost_micros"`
	CostBoundary           string            `json:"cost_boundary"`
	StartedAt              string            `json:"started_at"`
	CompletedAt            string            `json:"completed_at"`
	WallMillis             int64             `json:"wall_millis"`
	PromptSHA256           string            `json:"prompt_sha256"`
	OutputSchemaSHA256     string            `json:"output_schema_sha256"`
	RawResponsePath        string            `json:"raw_response_path,omitempty"`
	RawResponseSHA256      string            `json:"raw_response_sha256,omitempty"`
	CandidateOutputSHA256  string            `json:"candidate_output_sha256,omitempty"`
	NativeToolEvents       []string          `json:"native_tool_events"`
	ProviderWorkdirRemoved bool              `json:"provider_workdir_removed"`
	ProviderCallCount      int               `json:"provider_call_count"`
	Retries                int               `json:"retries"`
	Fallbacks              bool              `json:"fallbacks"`
	FailureClass           string            `json:"failure_class,omitempty"`
	FailureDetail          string            `json:"failure_detail,omitempty"`
	Candidate              json.RawMessage   `json:"-"`
	RawResponse            []byte            `json:"-"`
}

type trialObservation struct {
	ObservationID       string                                                      `json:"observation_id"`
	Lane                string                                                      `json:"lane"`
	CaseID              string                                                      `json:"case_id"`
	Arm                 string                                                      `json:"arm"`
	RunFamilyID         string                                                      `json:"run_family_id"`
	Status              string                                                      `json:"status"`
	SessionOrdinals     []int                                                       `json:"session_ordinals"`
	SpecialistEvidence  json.RawMessage                                             `json:"specialist_evidence,omitempty"`
	Actions             []agentqualification.RealisticCandidateAction               `json:"actions,omitempty"`
	ExecutionState      agentqualification.PrimarySubscriptionTrialExecutionStateV1 `json:"execution_state"`
	Workspace           agentqualification.RealisticCaseWorkspace                   `json:"workspace"`
	StartingTreeSHA256  string                                                      `json:"starting_tree_sha256"`
	FinalTreeSHA256     string                                                      `json:"final_tree_sha256,omitempty"`
	Grade               *agentqualification.RealisticWorkspaceGrade                 `json:"grade,omitempty"`
	ToolReceiptSHA256   string                                                      `json:"tool_receipt_sha256,omitempty"`
	ChangedPathSHA256   map[string]string                                           `json:"changed_path_sha256,omitempty"`
	FixtureRootRemoved  bool                                                        `json:"fixture_root_removed"`
	InitialRootRemoved  bool                                                        `json:"initial_root_removed"`
	BrowserEvidenceRoot string                                                      `json:"browser_evidence_root,omitempty"`
	FailureClass        string                                                      `json:"failure_class,omitempty"`
	FailureDetail       string                                                      `json:"failure_detail,omitempty"`
}

type trialCorrectionReference struct {
	CorrectionID string `json:"correction_id"`
	RecordPath   string `json:"record_path"`
	RecordSHA256 string `json:"record_sha256"`
}

type codexTransportCorrectionRecord struct {
	SchemaVersion                 string   `json:"schema_version"`
	CorrectionID                  string   `json:"correction_id"`
	TrialID                       string   `json:"trial_id"`
	DefinitionSHA256              string   `json:"definition_sha256"`
	AppliedAt                     string   `json:"applied_at"`
	PreservedFailedSessionOrdinal int      `json:"preserved_failed_session_ordinal"`
	PreservedRawResponseSHA256    string   `json:"preserved_raw_response_sha256"`
	PreservedFailureClass         string   `json:"preserved_failure_class"`
	BeforeNativeDiagnosticCount   int      `json:"before_native_diagnostic_count"`
	AfterNativeToolEventCount     int      `json:"after_native_tool_event_count"`
	AppliesToSessionOrdinals      []int    `json:"applies_to_session_ordinals"`
	OutputSchemaSHA256ByOrdinal   []string `json:"output_schema_sha256_by_ordinal"`
	TaskLocalProfile              bool     `json:"task_local_profile"`
	OpaqueAuthCopy                bool     `json:"opaque_auth_copy"`
	AuthContentRecorded           bool     `json:"auth_content_recorded"`
	ProfileCleanupRequired        bool     `json:"profile_cleanup_required"`
	ProviderCalls                 int      `json:"provider_calls"`
	Retries                       int      `json:"retries"`
	Replays                       int      `json:"replays"`
	Reason                        string   `json:"reason"`
}

type trialLedger struct {
	OfflineFake              bool                       `json:"offline_fake,omitempty"`
	SchemaVersion            string                     `json:"schema_version"`
	TrialID                  string                     `json:"trial_id"`
	DefinitionSHA256         string                     `json:"definition_sha256"`
	GateSHA256               string                     `json:"gate_sha256"`
	OwnerAuthorizationSHA256 string                     `json:"owner_authorization_sha256"`
	Status                   string                     `json:"status"`
	PreparedAt               string                     `json:"prepared_at"`
	CompletedAt              string                     `json:"completed_at,omitempty"`
	Runtimes                 []trialRuntimeIdentity     `json:"runtimes"`
	Sessions                 []trialSessionReceipt      `json:"sessions"`
	Observations             []trialObservation         `json:"observations"`
	Corrections              []trialCorrectionReference `json:"corrections,omitempty"`
	MaximumCLISessions       int                        `json:"maximum_cli_sessions"`
	ProviderCalls            int                        `json:"provider_calls"`
	Retries                  int                        `json:"retries"`
	Fallbacks                int                        `json:"fallbacks"`
	NativeToolExecutions     int                        `json:"native_tool_executions"`
	RepositoryAccesses       int                        `json:"repository_accesses"`
	ProductionAccesses       int                        `json:"production_accesses"`
	BusinessActions          int                        `json:"business_actions"`
	PhoneFarmAccesses        int                        `json:"phone_farm_accesses"`
	PermanentSelections      int                        `json:"permanent_selections"`
	EvidenceManifestSHA256   string                     `json:"evidence_manifest_sha256,omitempty"`
}

type trialInvoker interface {
	Invoke(context.Context, string, agentqualification.PrimarySubscriptionTrialInvocationV1, string, []byte) trialSessionReceipt
}

func (protocol trialProtocol) prepareTrial(root string) (trialLedger, error) {
	if protocol.contracts.Historical() {
		return trialLedger{}, fmt.Errorf("historical contract cannot prepare live runtimes")
	}
	return protocol.prepareTrialWithRuntimes(root, false, resolveLiveTrialRuntimes)
}

func (protocol trialProtocol) prepareTrialWithRuntimes(root string, fake bool, resolve func() ([]trialRuntimeIdentity, error)) (trialLedger, error) {
	if protocol.contracts.Historical() && !fake {
		return trialLedger{}, fmt.Errorf("historical contract cannot prepare live runtimes")
	}
	definition, err := protocol.activeTrialDefinition()
	if err != nil {
		return trialLedger{}, err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return trialLedger{}, err
	}
	path := filepath.Join(root, "trial-ledger.json")
	if _, err := os.Stat(path); err == nil {
		ledger, loadErr := loadTrialLedger(root)
		if loadErr != nil {
			return ledger, loadErr
		}
		if modeErr := validateTrialExecutionMode(ledger, fake); modeErr != nil {
			return ledger, modeErr
		}
		if validateErr := protocol.validateTrialLedger(definition, ledger, false); validateErr != nil {
			return ledger, validateErr
		}
		return ledger, nil
	} else if !os.IsNotExist(err) {
		return trialLedger{}, err
	}
	runtimes, err := resolve()
	if err != nil {
		return trialLedger{}, err
	}
	authorityHash := sha256.Sum256([]byte(definition.OwnerAuthorization))
	ledger := trialLedger{OfflineFake: fake, SchemaVersion: activeTrialLedgerSchema(), TrialID: definition.TrialID, DefinitionSHA256: activeTrialDefinitionSHA256(), GateSHA256: definition.GateSHA256, OwnerAuthorizationSHA256: hex.EncodeToString(authorityHash[:]), Status: "prepared_authorized", PreparedAt: time.Now().UTC().Format(time.RFC3339Nano), Runtimes: runtimes, MaximumCLISessions: definition.MaximumCLISessions}
	if err := validateTrialRuntimeMode(ledger); err != nil {
		return trialLedger{}, err
	}
	if err := writeJSONExclusive(path, ledger); err != nil {
		return trialLedger{}, err
	}
	return ledger, nil
}

func resolveLiveTrialRuntimes() ([]trialRuntimeIdentity, error) {
	runtimes := []trialRuntimeIdentity{}
	for _, lane := range []struct{ id, executable string }{{"claude_code_subscription", windowsExecutable("claude")}, {"codex_subscription", windowsExecutable("codex")}} {
		path, lookErr := exec.LookPath(lane.executable)
		if lookErr != nil {
			return nil, fmt.Errorf("%s runtime unavailable: %w", lane.id, lookErr)
		}
		absolute, _ := filepath.Abs(path)
		hash, hashErr := fileHash(absolute)
		if hashErr != nil {
			return nil, hashErr
		}
		versionCommand := exec.Command(absolute, "--version")
		versionCommand.Env = providerChildEnvironment(lane.id)
		versionRaw, versionErr := versionCommand.Output()
		if versionErr != nil {
			return nil, fmt.Errorf("%s version check failed: %w", lane.id, versionErr)
		}
		runtimes = append(runtimes, trialRuntimeIdentity{Lane: lane.id, ExecutablePath: absolute, ExecutableSHA256: hash, Version: strings.TrimSpace(string(versionRaw))})
	}
	return runtimes, nil
}

func (protocol trialProtocol) executeNextObservation(ctx context.Context, root string, invoker trialInvoker) (trialLedger, error) {
	if protocol.contracts.Historical() {
		fake, ok := invoker.(fakeTrialInvoker)
		if !ok {
			return trialLedger{}, fmt.Errorf("historical contract requires the in-process fake invoker")
		}
		fake.contracts = protocol.contracts
		invoker = fake
	}

	ledger, err := loadTrialLedger(root)
	if err != nil {
		return ledger, err
	}
	_, fakeInvoker := invoker.(fakeTrialInvoker)
	if err := validateTrialExecutionMode(ledger, fakeInvoker); err != nil {
		return ledger, err
	}
	definition, err := protocol.activeTrialDefinition()
	if err != nil {
		return trialLedger{}, err
	}
	if err := protocol.validateTrialLedger(definition, ledger, false); err != nil {
		return ledger, err
	}
	if len(ledger.Sessions) >= definition.MaximumCLISessions {
		return ledger, fmt.Errorf("all authorized CLI sessions are already consumed")
	}
	observationID := definition.Invocations[len(ledger.Sessions)].ObservationID
	if pending := pendingBrowserObservation(ledger); pending != "" && pending != observationID {
		return ledger, fmt.Errorf("R02 Browser evidence pending for %s", pending)
	}
	observation, index, err := ensureTrialObservation(root, &ledger, definition, observationID)
	if err != nil {
		return ledger, err
	}
	invocations := trialObservationInvocations(definition, observationID)
	for _, invocation := range invocations {
		if invocation.Ordinal <= len(ledger.Sessions) {
			continue
		}
		if invocation.Ordinal != len(ledger.Sessions)+1 {
			return ledger, fmt.Errorf("trial session ordinal gap")
		}
		prompt, schema, buildErr := protocol.buildActiveTrialPrompt(invocation, observation.Workspace, observation.SpecialistEvidence)
		if buildErr != nil {
			return ledger, buildErr
		}
		sessionRoot := filepath.Join(root, "sessions", fmt.Sprintf("%02d-%s-%s", invocation.Ordinal, safeName(observationID), safeName(invocation.Participant)))
		if err := os.MkdirAll(sessionRoot, 0o700); err != nil {
			return ledger, err
		}
		reserved := trialSessionReceipt{Ordinal: invocation.Ordinal, ObservationID: observationID, Lane: invocation.Lane, Participant: invocation.Participant, ParticipantKind: invocation.ParticipantKind, Status: "reserved_no_replay", RequestedModel: laneFor(definition, invocation.Lane).RequestedModel, RequestedEffort: laneFor(definition, invocation.Lane).Effort, ObservedModels: []string{}, NativeToolEvents: []string{}, PromptSHA256: hashBytes([]byte(prompt)), OutputSchemaSHA256: hashBytes(schema), Retries: 0, Fallbacks: false}
		ledger.Sessions = append(ledger.Sessions, reserved)
		if err := saveTrialLedger(root, ledger); err != nil {
			return ledger, err
		}
		receipt := invoker.Invoke(ctx, sessionRoot, invocation, prompt, schema)
		if receipt.Ordinal != invocation.Ordinal || receipt.ObservationID != observationID || receipt.Lane != invocation.Lane || receipt.Participant != invocation.Participant || receipt.PromptSHA256 != reserved.PromptSHA256 || receipt.OutputSchemaSHA256 != reserved.OutputSchemaSHA256 {
			return ledger, fmt.Errorf("provider receipt identity drifted")
		}
		if len(receipt.RawResponse) != 0 {
			name := "raw-response.json"
			if invocation.Lane == "codex_subscription" {
				name = "raw-events.jsonl"
			}
			path := filepath.Join(sessionRoot, name)
			if err := writeBytesExclusive(path, receipt.RawResponse); err != nil {
				return ledger, err
			}
			receipt.RawResponsePath = filepath.ToSlash(filepath.Join("sessions", filepath.Base(sessionRoot), name))
			receipt.RawResponseSHA256 = hashBytes(receipt.RawResponse)
		}
		receipt.RawResponse = nil
		if ceilingErr := validateTrialSessionCeilings(definition, receipt); ceilingErr != nil {
			receipt.Status, receipt.FailureClass, receipt.FailureDetail = "invalid", "resource_boundary_exceeded", ceilingErr.Error()
			receipt.Candidate = nil
		}
		ledger.Sessions[len(ledger.Sessions)-1] = receipt
		ledger.ProviderCalls += receipt.ProviderCallCount
		ledger.NativeToolExecutions += len(receipt.NativeToolEvents)
		if receipt.Status == "completed" {
			parsed, parseErr := protocol.parseActiveTrialOutput(receipt.Candidate, invocation, observation.Workspace)
			if parseErr != nil {
				receipt.Status, receipt.FailureClass, receipt.FailureDetail = "invalid", "response_protocol_failure", parseErr.Error()
				ledger.Sessions[len(ledger.Sessions)-1] = receipt
			} else if invocation.ParticipantKind == "specialist" {
				observation.SpecialistEvidence = append(json.RawMessage(nil), parsed.Evidence...)
			} else {
				observation.Actions = append([]agentqualification.RealisticCandidateAction(nil), parsed.Actions...)
				observation.ExecutionState.Controller.ProviderCalls = len(invocations)
			}
		}
		observation.SessionOrdinals = append(observation.SessionOrdinals, invocation.Ordinal)
		ledger.Observations[index] = observation
		if err := saveTrialLedger(root, ledger); err != nil {
			return ledger, err
		}
	}
	invalid := false
	for _, ordinal := range observation.SessionOrdinals {
		if ledger.Sessions[ordinal-1].Status != "completed" {
			invalid = true
		}
	}
	if !invalid && len(observation.Actions) != 0 {
		state, executionErr := agentqualification.ExecutePrimarySubscriptionTrialActionsV1(observation.Workspace, observation.RunFamilyID, observation.Actions, observation.ExecutionState, nil)
		observation.ExecutionState = state
		if executionErr != nil {
			observation.Status, observation.FailureClass, observation.FailureDetail = "failed", "controller_action_failure", executionErr.Error()
		} else {
			observation.Status = state.Controller.TerminalState
		}
	} else {
		observation.Status, observation.FailureClass = "invalid", "participant_session_invalid"
		observation.FailureDetail = "one or more frozen participant sessions failed or produced invalid output"
	}
	ledger.Observations[index] = observation
	if observation.Status == "completed" {
		if err := closeTrialObservation(root, &ledger, index); err != nil {
			return ledger, err
		}
	} else if observation.Status == "invalid" || observation.Status == "failed" {
		if err := cleanupTrialWorkspace(&ledger.Observations[index]); err != nil {
			return ledger, err
		}
	}
	if len(ledger.Sessions) == definition.MaximumCLISessions && pendingBrowserObservation(ledger) == "" {
		ledger.Status, ledger.CompletedAt = "complete", time.Now().UTC().Format(time.RFC3339Nano)
	}
	if err := saveTrialLedger(root, ledger); err != nil {
		return ledger, err
	}
	return ledger, nil
}

func validateTrialSessionCeilings(definition agentqualification.PrimarySubscriptionRoleTrialV1, receipt trialSessionReceipt) error {
	values := []int64{receipt.InputTokens, receipt.CachedInputTokens, receipt.OutputTokens, receipt.ReasoningTokens}
	for _, value := range values {
		if value < 0 {
			return fmt.Errorf("negative provider token receipt")
		}
	}
	total := receipt.InputTokens + receipt.OutputTokens + receipt.ReasoningTokens
	generated := receipt.OutputTokens + receipt.ReasoningTokens
	if total > definition.MaximumTotalReportedTokensPerSession {
		return fmt.Errorf("reported token ceiling exceeded")
	}
	if generated > definition.MaximumGeneratedTokensPerSession {
		return fmt.Errorf("generated token ceiling exceeded")
	}
	return nil
}

func (protocol trialProtocol) applyCodexTransportCorrection(root string) (trialLedger, error) {
	definition, err := protocol.activeTrialDefinition()
	if err != nil {
		return trialLedger{}, err
	}
	ledger, err := loadTrialLedger(root)
	if err != nil {
		return ledger, err
	}
	for _, correction := range ledger.Corrections {
		if correction.CorrectionID == codexTransportCorrectionID {
			if err := protocol.validateTrialLedger(definition, ledger, false); err != nil {
				return ledger, err
			}
			return ledger, nil
		}
	}
	if ledger.SchemaVersion != trialLedgerSchema || ledger.TrialID != definition.TrialID || ledger.DefinitionSHA256 != agentqualification.PrimarySubscriptionRoleTrialV1SHA256() || ledger.GateSHA256 != definition.GateSHA256 || len(ledger.Sessions) != 7 || len(ledger.Observations) != 5 || ledger.ProviderCalls != 7 || ledger.NativeToolExecutions != 2 || ledger.Retries != 0 || ledger.Fallbacks != 0 || ledger.RepositoryAccesses != 0 || ledger.ProductionAccesses != 0 || ledger.BusinessActions != 0 || ledger.PhoneFarmAccesses != 0 || ledger.PermanentSelections != 0 {
		return ledger, fmt.Errorf("Codex correction precondition failed")
	}
	failed := &ledger.Sessions[6]
	wantDiagnostics := []string{"item.completed:error", "item.completed:error"}
	if failed.Ordinal != 7 || failed.Status != "invalid" || failed.Lane != "codex_subscription" || failed.FailureClass != "provider_or_transport_failure" || failed.ProviderCallCount != 1 || failed.Retries != 0 || failed.Fallbacks || !failed.ProviderWorkdirRemoved || !slices.Equal(failed.NativeToolEvents, wantDiagnostics) || failed.RawResponsePath == "" || failed.RawResponseSHA256 == "" {
		return ledger, fmt.Errorf("preserved session 7 does not match the exact correction boundary")
	}
	rawPath := filepath.Join(root, filepath.FromSlash(failed.RawResponsePath))
	raw, err := os.ReadFile(rawPath)
	if err != nil || hashBytes(raw) != failed.RawResponseSHA256 {
		return ledger, fmt.Errorf("preserved session 7 raw evidence mismatch")
	}
	reparsed := parseCodexTrialEvents(raw)
	if len(reparsed.NativeToolEvents) != 0 || reparsed.ProviderCallCount != 1 || reparsed.Failure == "" {
		return ledger, fmt.Errorf("corrected Codex diagnostic classification did not fail closed")
	}
	schemaHashes := make([]string, 0, 5)
	for _, invocation := range definition.Invocations[7:] {
		schema, schemaErr := protocol.activeTrialResponseSchema(invocation)
		if schemaErr != nil {
			return ledger, schemaErr
		}
		schemaHashes = append(schemaHashes, fmt.Sprintf("%02d:%s", invocation.Ordinal, hashBytes(schema)))
	}
	failed.NativeToolEvents = []string{}
	ledger.NativeToolExecutions = 0
	record := codexTransportCorrectionRecord{
		SchemaVersion: "silver-mediums.primary-subscription-role-trial-correction.v1", CorrectionID: codexTransportCorrectionID,
		TrialID: definition.TrialID, DefinitionSHA256: ledger.DefinitionSHA256, AppliedAt: time.Now().UTC().Format(time.RFC3339Nano),
		PreservedFailedSessionOrdinal: 7, PreservedRawResponseSHA256: failed.RawResponseSHA256, PreservedFailureClass: failed.FailureClass,
		BeforeNativeDiagnosticCount: 2, AfterNativeToolEventCount: 0, AppliesToSessionOrdinals: []int{8, 9, 10, 11, 12}, OutputSchemaSHA256ByOrdinal: schemaHashes,
		TaskLocalProfile: true, OpaqueAuthCopy: true, AuthContentRecorded: false, ProfileCleanupRequired: true,
		ProviderCalls: 0, Retries: 0, Replays: 0,
		Reason: "Session 7 remains terminally failed. Codex error diagnostics are not tool executions; subsequent authorized sessions use strict provider schemas and a disposable task-local subscription profile.",
	}
	recordRaw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return ledger, err
	}
	relative := filepath.ToSlash(filepath.Join("corrections", codexTransportCorrectionID+".json"))
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := writeOrEqual(path, recordRaw); err != nil {
		return ledger, err
	}
	ledger.Corrections = append(ledger.Corrections, trialCorrectionReference{CorrectionID: codexTransportCorrectionID, RecordPath: relative, RecordSHA256: hashBytes(recordRaw)})
	if err := protocol.validateTrialLedger(definition, ledger, false); err != nil {
		return ledger, err
	}
	if err := saveTrialLedger(root, ledger); err != nil {
		return ledger, err
	}
	return ledger, nil
}

func ensureTrialObservation(root string, ledger *trialLedger, definition agentqualification.PrimarySubscriptionRoleTrialV1, observationID string) (trialObservation, int, error) {
	for index, observation := range ledger.Observations {
		if observation.ObservationID == observationID {
			return observation, index, nil
		}
	}
	invocations := trialObservationInvocations(definition, observationID)
	if len(invocations) == 0 {
		return trialObservation{}, -1, fmt.Errorf("unknown trial observation")
	}
	runFamily := definition.TrialID + ":" + observationID
	parent, err := os.MkdirTemp("", "sm-item14-role-trial-observation-")
	if err != nil {
		return trialObservation{}, -1, err
	}
	workspace, err := agentqualification.PrepareRealisticCaseWorkspaceForRun(parent, invocations[0].CaseID, runFamily)
	if err != nil {
		_ = os.RemoveAll(parent)
		return trialObservation{}, -1, err
	}
	observation := trialObservation{ObservationID: observationID, Lane: invocations[0].Lane, CaseID: invocations[0].CaseID, Arm: invocations[0].Arm, RunFamilyID: runFamily, Status: "prepared", SessionOrdinals: []int{}, Workspace: workspace, StartingTreeSHA256: workspace.StartingTreeSHA256, ChangedPathSHA256: map[string]string{}}
	if observation.CaseID == "R02" {
		observation.BrowserEvidenceRoot = filepath.Join(root, "observations", safeName(observationID), "browser")
	}
	ledger.Observations = append(ledger.Observations, observation)
	if err := saveTrialLedger(root, *ledger); err != nil {
		return trialObservation{}, -1, err
	}
	return observation, len(ledger.Observations) - 1, nil
}

func closeTrialObservation(root string, ledger *trialLedger, index int) error {
	observation := &ledger.Observations[index]
	if observation.ExecutionState.Controller.TerminalState != "completed" {
		return fmt.Errorf("trial observation is not complete")
	}
	grade, err := agentqualification.GradePrimarySubscriptionTrialWorkspaceV1(observation.Workspace, observation.Lane+":"+observation.Arm, observation.ExecutionState.Controller)
	if err != nil {
		return err
	}
	observation.Grade = &grade
	finalHash, err := agentqualification.RealisticFixtureTreeSHA256(os.DirFS(observation.Workspace.Root), ".")
	if err != nil {
		return err
	}
	observation.FinalTreeSHA256 = finalHash
	toolReceipt, err := agentqualification.PrimarySubscriptionTrialToolReceiptV1(observation.Workspace, observation.Lane+":"+observation.Arm, observation.ExecutionState.Controller)
	if err != nil {
		return err
	}
	toolRaw, _ := json.MarshalIndent(toolReceipt, "", "  ")
	observation.ToolReceiptSHA256 = hashBytes(toolRaw)
	observationRoot := filepath.Join(root, "observations", safeName(observation.ObservationID))
	if err := os.MkdirAll(observationRoot, 0o700); err != nil {
		return err
	}
	if err := writeOrEqual(filepath.Join(observationRoot, "tool-receipt.json"), toolRaw); err != nil {
		return err
	}
	gradeRaw, _ := json.MarshalIndent(grade, "", "  ")
	if err := writeOrEqual(filepath.Join(observationRoot, "grade.json"), gradeRaw); err != nil {
		return err
	}
	changed, err := changedPathHashes(observation.Workspace.InitialCopy, observation.Workspace.Root)
	if err != nil {
		return err
	}
	observation.ChangedPathSHA256 = changed
	observation.Status = "complete"
	if grade.Verdict != "PASS" {
		observation.Status, observation.FailureClass = "candidate_fail", "hidden_verifier_failure"
	}
	if err := cleanupTrialWorkspace(observation); err != nil {
		return err
	}
	observationRaw, _ := json.MarshalIndent(observation, "", "  ")
	return writeOrEqual(filepath.Join(observationRoot, "observation.json"), observationRaw)
}

func (protocol trialProtocol) finalizeTrialObservation(root, observationID string) (trialLedger, error) {
	definition, err := protocol.activeTrialDefinition()
	if err != nil {
		return trialLedger{}, err
	}
	ledger, err := loadTrialLedger(root)
	if err != nil {
		return ledger, err
	}
	if err := protocol.validateTrialLedger(definition, ledger, false); err != nil {
		return ledger, err
	}
	index := -1
	for i := range ledger.Observations {
		if ledger.Observations[i].ObservationID == observationID {
			index = i
		}
	}
	if index < 0 || ledger.Observations[index].Status != "awaiting_browser_evidence" {
		return ledger, fmt.Errorf("observation is not awaiting Browser evidence")
	}
	observation := &ledger.Observations[index]
	collector := agentqualification.FileRealisticBrowserCollector{RunFamilyID: observation.RunFamilyID, ArtifactRoot: observation.BrowserEvidenceRoot}
	state, err := agentqualification.ExecutePrimarySubscriptionTrialActionsV1(observation.Workspace, observation.RunFamilyID, observation.Actions, observation.ExecutionState, collector)
	if err != nil {
		return ledger, err
	}
	observation.ExecutionState = state
	observation.Status = state.Controller.TerminalState
	if observation.Status != "completed" {
		return ledger, fmt.Errorf("Browser continuation did not complete")
	}
	if err := os.Remove(filepath.Join(observation.BrowserEvidenceRoot, "browser-capture-token.txt")); err != nil && !os.IsNotExist(err) {
		return ledger, err
	}
	if err := closeTrialObservation(root, &ledger, index); err != nil {
		return ledger, err
	}
	if len(ledger.Sessions) == definition.MaximumCLISessions && pendingBrowserObservation(ledger) == "" {
		ledger.Status, ledger.CompletedAt = "complete", time.Now().UTC().Format(time.RFC3339Nano)
	}
	if err := saveTrialLedger(root, ledger); err != nil {
		return ledger, err
	}
	return ledger, nil
}

func cleanupTrialWorkspace(observation *trialObservation) error {
	rootParent := filepath.Dir(observation.Workspace.Root)
	initial := observation.Workspace.InitialCopy
	if err := os.RemoveAll(rootParent); err != nil {
		return err
	}
	_, rootErr := os.Stat(observation.Workspace.Root)
	_, initialErr := os.Stat(initial)
	observation.FixtureRootRemoved = os.IsNotExist(rootErr)
	observation.InitialRootRemoved = os.IsNotExist(initialErr)
	if !observation.FixtureRootRemoved || !observation.InitialRootRemoved {
		return fmt.Errorf("disposable trial fixture cleanup not verified")
	}
	return nil
}

func (protocol trialProtocol) cleanupAbandonedTrial(root string) (trialLedger, error) {
	definition, err := protocol.activeTrialDefinition()
	if err != nil {
		return trialLedger{}, err
	}
	ledger, err := loadTrialLedger(root)
	if err != nil {
		return ledger, err
	}
	if err := protocol.validateTrialLedger(definition, ledger, false); err != nil {
		return ledger, err
	}
	if len(ledger.Sessions) != 0 || len(ledger.Observations) != 1 || ledger.Observations[0].Status != "prepared" {
		return ledger, fmt.Errorf("only a zero-session prepared campaign may use abandoned cleanup")
	}
	parent := filepath.Dir(ledger.Observations[0].Workspace.Root)
	temporary := filepath.Clean(os.TempDir())
	if !strings.HasPrefix(strings.ToLower(filepath.Clean(parent)), strings.ToLower(temporary)+string(os.PathSeparator)) || !strings.HasPrefix(filepath.Base(parent), "sm-item14-role-trial-observation-") {
		return ledger, fmt.Errorf("abandoned workspace is outside the exact trial temp boundary")
	}
	if err := cleanupTrialWorkspace(&ledger.Observations[0]); err != nil {
		return ledger, err
	}
	ledger.Observations[0].Status = "aborted_pre_provider_cleaned"
	ledger.Status, ledger.CompletedAt = "aborted_pre_provider_cleaned", time.Now().UTC().Format(time.RFC3339Nano)
	if err := saveTrialLedger(root, ledger); err != nil {
		return ledger, err
	}
	return ledger, nil
}

func (protocol trialProtocol) validateTrialLedger(definition agentqualification.PrimarySubscriptionRoleTrialV1, ledger trialLedger, requireComplete bool) error {
	if err := validateTrialRuntimeMode(ledger); err != nil {
		return err
	}
	authorityHash := sha256.Sum256([]byte(definition.OwnerAuthorization))
	if ledger.SchemaVersion != activeTrialLedgerSchema() || ledger.TrialID != definition.TrialID || ledger.DefinitionSHA256 != activeTrialDefinitionSHA256() || ledger.GateSHA256 != definition.GateSHA256 || ledger.OwnerAuthorizationSHA256 != hex.EncodeToString(authorityHash[:]) || ledger.MaximumCLISessions != definition.MaximumCLISessions || ledger.Retries != 0 || ledger.Fallbacks != 0 || ledger.RepositoryAccesses != 0 || ledger.ProductionAccesses != 0 || ledger.BusinessActions != 0 || ledger.PhoneFarmAccesses != 0 || ledger.PermanentSelections != 0 {
		return fmt.Errorf("trial ledger identity or safety counters drifted")
	}
	if len(ledger.Runtimes) != len(definition.Lanes) {
		return fmt.Errorf("trial runtime identity count drifted")
	}
	runtimeLanes := map[string]bool{}
	for _, runtime := range ledger.Runtimes {
		if runtimeLanes[runtime.Lane] || laneFor(definition, runtime.Lane).Lane == "" || strings.TrimSpace(runtime.ExecutablePath) == "" || len(runtime.ExecutableSHA256) != 64 || strings.Trim(runtime.ExecutableSHA256, "0123456789abcdef") != "" || strings.TrimSpace(runtime.Version) == "" {
			return fmt.Errorf("trial runtime identity invalid")
		}
		runtimeLanes[runtime.Lane] = true
	}
	if len(ledger.Sessions) > definition.MaximumCLISessions || ledger.ProviderCalls > definition.MaximumCLISessions {
		return fmt.Errorf("trial session ceiling exceeded")
	}
	providerCalls, nativeToolExecutions := 0, 0
	for index, session := range ledger.Sessions {
		invocation := definition.Invocations[index]
		lane := laneFor(definition, invocation.Lane)
		schema, schemaErr := protocol.activeTrialResponseSchema(invocation)
		if schemaErr != nil {
			return schemaErr
		}
		schemaHashValid := session.OutputSchemaSHA256 == hashBytes(schema)
		if !schemaHashValid && trialDefinitionVariant == "v1" && session.Ordinal == 7 && session.Status == "invalid" && session.FailureClass == "provider_or_transport_failure" && containsCorrection(ledger.Corrections, codexTransportCorrectionID) {
			legacyInvocation := invocation
			legacyInvocation.Lane = "claude_code_subscription"
			legacySchema, legacyErr := protocol.contracts.PrimarySubscriptionTrialResponseSchemaV1(legacyInvocation)
			schemaHashValid = legacyErr == nil && session.OutputSchemaSHA256 == hashBytes(legacySchema)
		}
		if session.Ordinal != index+1 || session.Ordinal != invocation.Ordinal || session.ObservationID != invocation.ObservationID || session.Lane != invocation.Lane || session.Participant != invocation.Participant || session.ParticipantKind != invocation.ParticipantKind || session.RequestedModel != lane.RequestedModel || session.RequestedEffort != lane.Effort || !schemaHashValid || session.Retries != 0 || session.Fallbacks || len(session.NativeToolEvents) != 0 || !session.ProviderWorkdirRemoved && session.Status != "reserved_no_replay" {
			return fmt.Errorf("trial session %d identity or safety evidence invalid", index+1)
		}
		if !containsString([]string{"reserved_no_replay", "completed", "invalid"}, session.Status) || session.ProviderCallCount < 0 || session.ProviderCallCount > 1 || session.Status == "reserved_no_replay" && session.ProviderCallCount != 0 {
			return fmt.Errorf("trial session %d lifecycle or call count invalid", index+1)
		}
		if session.Status == "completed" {
			if session.ProviderCallCount != 1 || session.RawResponsePath == "" || session.RawResponseSHA256 == "" || session.CandidateOutputSHA256 == "" {
				return fmt.Errorf("trial session %d completed receipt incomplete", index+1)
			}
			if session.Lane == "claude_code_subscription" {
				if session.ObservedModelStatus != observedModelStatusForLane(session.Lane) || !containsString(session.ObservedModels, lane.RequestedModel) {
					return fmt.Errorf("trial session %d Claude model attribution invalid", index+1)
				}
				for _, model := range session.ObservedModels {
					if !containsString(lane.AllowedObservedModels, model) {
						return fmt.Errorf("trial session %d Claude model attribution drifted", index+1)
					}
				}
			} else if session.ObservedModelStatus != observedModelStatusForLane(session.Lane) || len(session.ObservedModels) != 0 {
				return fmt.Errorf("trial session %d Codex model attribution invalid", index+1)
			}
		}
		providerCalls += session.ProviderCallCount
		nativeToolExecutions += len(session.NativeToolEvents)
	}
	if ledger.ProviderCalls != providerCalls || ledger.NativeToolExecutions != nativeToolExecutions {
		return fmt.Errorf("trial aggregate session accounting drifted")
	}
	if requireComplete {
		if !containsString([]string{"complete", "complete_verified"}, ledger.Status) || len(ledger.Sessions) != definition.MaximumCLISessions || len(ledger.Observations) != trialObservationCount(definition) || ledger.NativeToolExecutions != 0 || pendingBrowserObservation(ledger) != "" {
			return fmt.Errorf("trial ledger is not complete")
		}
		for _, observation := range ledger.Observations {
			if !observation.FixtureRootRemoved || !observation.InitialRootRemoved || observation.Status == "awaiting_browser_evidence" || observation.Status == "prepared" {
				return fmt.Errorf("trial observation %s is not terminal and cleaned", observation.ObservationID)
			}
		}
	}
	return nil
}

func (protocol trialProtocol) verifyTrial(root string) (trialLedger, error) {
	definition, err := protocol.activeTrialDefinition()
	if err != nil {
		return trialLedger{}, err
	}
	ledger, err := loadTrialLedger(root)
	if err != nil {
		return ledger, err
	}
	if err := protocol.validateTrialLedger(definition, ledger, true); err != nil {
		return ledger, err
	}
	manifest, err := evidenceManifest(root)
	if err != nil {
		return ledger, err
	}
	manifestSHA256 := hashBytes(manifest)
	if ledger.EvidenceManifestSHA256 != "" && ledger.EvidenceManifestSHA256 != manifestSHA256 {
		return ledger, fmt.Errorf("trial evidence changed after manifest seal")
	}
	ledger.EvidenceManifestSHA256 = manifestSHA256
	ledger.Status = "complete_verified"
	if err := saveTrialLedger(root, ledger); err != nil {
		return ledger, err
	}
	return ledger, nil
}

func loadTrialLedger(root string) (trialLedger, error) {
	var ledger trialLedger
	raw, err := os.ReadFile(filepath.Join(root, "trial-ledger.json"))
	if err != nil {
		return ledger, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&ledger); err != nil {
		return ledger, err
	}
	return ledger, nil
}

func saveTrialLedger(root string, ledger trialLedger) error {
	raw, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(root, "trial-ledger.json"), raw)
}

func trialObservationInvocations(definition agentqualification.PrimarySubscriptionRoleTrialV1, observationID string) []agentqualification.PrimarySubscriptionTrialInvocationV1 {
	result := []agentqualification.PrimarySubscriptionTrialInvocationV1{}
	for _, invocation := range definition.Invocations {
		if invocation.ObservationID == observationID {
			result = append(result, invocation)
		}
	}
	return result
}

func trialObservationCount(definition agentqualification.PrimarySubscriptionRoleTrialV1) int {
	seen := map[string]struct{}{}
	for _, invocation := range definition.Invocations {
		seen[invocation.ObservationID] = struct{}{}
	}
	return len(seen)
}

func laneFor(definition agentqualification.PrimarySubscriptionRoleTrialV1, lane string) agentqualification.PrimarySubscriptionTrialLaneV1 {
	for _, candidate := range definition.Lanes {
		if candidate.Lane == lane {
			return candidate
		}
	}
	return agentqualification.PrimarySubscriptionTrialLaneV1{}
}

func observedModelStatusForLane(lane string) string {
	if lane == "claude_code_subscription" {
		return "reported_composite_allowed"
	}
	return "unavailable_truthful_cli_request_pinned"
}

func containsCorrection(corrections []trialCorrectionReference, correctionID string) bool {
	for _, correction := range corrections {
		if correction.CorrectionID == correctionID {
			return true
		}
	}
	return false
}

func pendingBrowserObservation(ledger trialLedger) string {
	for _, observation := range ledger.Observations {
		if observation.Status == "awaiting_browser_evidence" {
			return observation.ObservationID
		}
	}
	return ""
}

func changedPathHashes(initial, final string) (map[string]string, error) {
	left, err := fileHashes(initial)
	if err != nil {
		return nil, err
	}
	right, err := fileHashes(final)
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	for path, hash := range right {
		if left[path] != hash {
			result[path] = hash
		}
	}
	for path := range left {
		if _, ok := right[path]; !ok {
			result[path] = "removed"
		}
	}
	return result, nil
}

func fileHashes(root string) (map[string]string, error) {
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || strings.HasSuffix(entry.Name(), ".pyc") || strings.Contains(filepath.ToSlash(path), "/__pycache__/") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(root, path)
		result[filepath.ToSlash(relative)] = hashBytes(raw)
		return nil
	})
	return result, err
}

func evidenceManifest(root string) ([]byte, error) {
	rows := []string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Base(path) == "trial-ledger.json" || filepath.Base(path) == "implement-progress.md" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(root, path)
		rows = append(rows, filepath.ToSlash(relative)+"\x00"+hashBytes(raw)+"\n")
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(rows)
	return []byte(strings.Join(rows, "")), nil
}

func writeJSONExclusive(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeBytesExclusive(path, raw)
}

func writeBytesExclusive(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(raw); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func writeAtomic(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return err
	}
	if _, err := temporary.Write(raw); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := replaceExistingFile(temporaryPath, path); err != nil {
		return err
	}
	committed = true
	return nil
}

func writeOrEqual(path string, raw []byte) error {
	existing, err := os.ReadFile(path)
	if err == nil {
		if !strings.EqualFold(hashBytes(existing), hashBytes(raw)) {
			return fmt.Errorf("write-once evidence drifted: %s", path)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	return writeBytesExclusive(path, raw)
}

func safeName(value string) string {
	value = strings.ToLower(value)
	var builder strings.Builder
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '-' {
			builder.WriteRune(character)
		} else {
			builder.WriteByte('-')
		}
	}
	return strings.Trim(builder.String(), "-")
}

func hashBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func fileHash(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return hashBytes(raw), nil
}

func windowsExecutable(name string) string {
	if os.PathSeparator == '\\' {
		return name + ".cmd"
	}
	return name
}

func cleanupPath(path string) bool {
	_ = os.RemoveAll(path)
	_, err := os.Stat(path)
	return errors.Is(err, os.ErrNotExist)
}
