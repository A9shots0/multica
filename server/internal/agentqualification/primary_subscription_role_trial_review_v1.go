package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const PrimarySubscriptionRoleTrialReviewV1Schema = "silver-mediums.primary-subscription-role-trial-review.v1"
const PrimarySubscriptionReliabilityTrialReviewV2Schema = "silver-mediums.primary-subscription-reliability-trial-review.v2"

type PrimarySubscriptionTrialReviewCheckV1 struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type PrimarySubscriptionTrialReviewLaneV1 struct {
	Lane          string `json:"lane"`
	Sessions      int    `json:"sessions"`
	Completed     int    `json:"completed"`
	Invalid       int    `json:"invalid"`
	InputTokens   int64  `json:"input_tokens"`
	OutputTokens  int64  `json:"output_tokens"`
	CostMicros    int64  `json:"cost_micros"`
	WallMillis    int64  `json:"wall_millis"`
	ObservedModel string `json:"observed_model"`
}

type PrimarySubscriptionTrialReviewArmV1 struct {
	Arm          string `json:"arm"`
	Observations int    `json:"observations"`
	Passes       int    `json:"passes"`
	Invalid      int    `json:"invalid"`
	Failed       int    `json:"failed"`
}

type PrimarySubscriptionReliabilityTrialCellV2 struct {
	Lane         string `json:"lane"`
	CaseID       string `json:"case_id"`
	Observations int    `json:"observations"`
	Passes       int    `json:"passes"`
	Invalid      int    `json:"invalid"`
	Failed       int    `json:"failed"`
}

type PrimarySubscriptionTrialReviewV1 struct {
	SchemaVersion             string                                      `json:"schema_version"`
	TrialID                   string                                      `json:"trial_id"`
	EvidenceSnapshotAt        string                                      `json:"evidence_snapshot_at"`
	SourceLedgerSHA256        string                                      `json:"source_ledger_sha256"`
	SourceManifestSHA256      string                                      `json:"source_manifest_sha256"`
	Verdict                   string                                      `json:"verdict"`
	Checks                    []PrimarySubscriptionTrialReviewCheckV1     `json:"checks"`
	SessionCount              int                                         `json:"session_count"`
	ObservationCount          int                                         `json:"observation_count"`
	ProviderCalls             int                                         `json:"provider_calls"`
	PassObservations          int                                         `json:"pass_observations"`
	InvalidObservations       int                                         `json:"invalid_observations"`
	FailedObservations        int                                         `json:"failed_observations"`
	Retries                   int                                         `json:"retries"`
	Fallbacks                 int                                         `json:"fallbacks"`
	NativeToolExecutions      int                                         `json:"native_tool_executions"`
	RepositoryAccesses        int                                         `json:"repository_accesses"`
	ProductionAccesses        int                                         `json:"production_accesses"`
	BusinessActions           int                                         `json:"business_actions"`
	PhoneFarmAccesses         int                                         `json:"phone_farm_accesses"`
	PermanentSelections       int                                         `json:"permanent_selections"`
	LaneSummaries             []PrimarySubscriptionTrialReviewLaneV1      `json:"lane_summaries"`
	ArmSummaries              []PrimarySubscriptionTrialReviewArmV1       `json:"arm_summaries"`
	CellSummaries             []PrimarySubscriptionReliabilityTrialCellV2 `json:"cell_summaries,omitempty"`
	ConfigurationSelection    string                                      `json:"configuration_selection"`
	PermanentSeatSelection    string                                      `json:"permanent_seat_selection"`
	EvidenceBackedConclusions []string                                    `json:"evidence_backed_conclusions"`
	Limitations               []string                                    `json:"limitations"`
}

type primarySubscriptionReviewLedger struct {
	SchemaVersion          string                                 `json:"schema_version"`
	TrialID                string                                 `json:"trial_id"`
	DefinitionSHA256       string                                 `json:"definition_sha256"`
	GateSHA256             string                                 `json:"gate_sha256"`
	OwnerAuthorizationHash string                                 `json:"owner_authorization_sha256"`
	Status                 string                                 `json:"status"`
	CompletedAt            string                                 `json:"completed_at"`
	Sessions               []primarySubscriptionReviewSession     `json:"sessions"`
	Observations           []primarySubscriptionReviewObservation `json:"observations"`
	Corrections            []primarySubscriptionReviewCorrection  `json:"corrections"`
	MaximumCLISessions     int                                    `json:"maximum_cli_sessions"`
	ProviderCalls          int                                    `json:"provider_calls"`
	Retries                int                                    `json:"retries"`
	Fallbacks              int                                    `json:"fallbacks"`
	NativeToolExecutions   int                                    `json:"native_tool_executions"`
	RepositoryAccesses     int                                    `json:"repository_accesses"`
	ProductionAccesses     int                                    `json:"production_accesses"`
	BusinessActions        int                                    `json:"business_actions"`
	PhoneFarmAccesses      int                                    `json:"phone_farm_accesses"`
	PermanentSelections    int                                    `json:"permanent_selections"`
	EvidenceManifestSHA256 string                                 `json:"evidence_manifest_sha256"`
}

type primarySubscriptionReviewSession struct {
	Ordinal                int      `json:"ordinal"`
	ObservationID          string   `json:"observation_id"`
	Lane                   string   `json:"lane"`
	Participant            string   `json:"participant"`
	ParticipantKind        string   `json:"participant_kind"`
	Status                 string   `json:"status"`
	RequestedModel         string   `json:"requested_model"`
	RequestedEffort        string   `json:"requested_effort"`
	ObservedModels         []string `json:"observed_models"`
	ObservedModelStatus    string   `json:"observed_model_status"`
	InputTokens            int64    `json:"input_tokens"`
	OutputTokens           int64    `json:"output_tokens"`
	ReasoningTokens        int64    `json:"reasoning_tokens"`
	CostMicros             int64    `json:"cost_micros"`
	WallMillis             int64    `json:"wall_millis"`
	RawResponsePath        string   `json:"raw_response_path"`
	RawResponseSHA256      string   `json:"raw_response_sha256"`
	CandidateOutputSHA256  string   `json:"candidate_output_sha256"`
	NativeToolEvents       []string `json:"native_tool_events"`
	ProviderWorkdirRemoved bool     `json:"provider_workdir_removed"`
	ProviderCallCount      int      `json:"provider_call_count"`
	Retries                int      `json:"retries"`
	Fallbacks              bool     `json:"fallbacks"`
	FailureClass           string   `json:"failure_class"`
}

type primarySubscriptionReviewObservation struct {
	ObservationID   string `json:"observation_id"`
	Lane            string `json:"lane"`
	CaseID          string `json:"case_id"`
	Arm             string `json:"arm"`
	Status          string `json:"status"`
	SessionOrdinals []int  `json:"session_ordinals"`
	Workspace       struct {
		Root        string `json:"Root"`
		InitialCopy string `json:"InitialCopy"`
	} `json:"workspace"`
	Grade *struct {
		Verdict string `json:"verdict"`
	} `json:"grade"`
	ToolReceiptSHA256   string `json:"tool_receipt_sha256"`
	FixtureRootRemoved  bool   `json:"fixture_root_removed"`
	InitialRootRemoved  bool   `json:"initial_root_removed"`
	BrowserEvidenceRoot string `json:"browser_evidence_root"`
	FailureClass        string `json:"failure_class"`
}

type primarySubscriptionReviewCorrection struct {
	CorrectionID string `json:"correction_id"`
	RecordPath   string `json:"record_path"`
	RecordSHA256 string `json:"record_sha256"`
}

type primarySubscriptionReviewBrowserEvidence struct {
	SchemaVersion string `json:"schema_version"`
	Captures      []struct {
		Viewport                string `json:"viewport"`
		ScreenshotArtifactPath  string `json:"screenshot_artifact_path"`
		ScreenshotSHA256        string `json:"screenshot_sha256"`
		DOMSnapshotArtifactPath string `json:"dom_snapshot_artifact_path"`
		DOMSnapshotSHA256       string `json:"dom_snapshot_sha256"`
	} `json:"captures"`
}

func ReviewPrimarySubscriptionRoleTrialEvidenceV1(root string) (PrimarySubscriptionTrialReviewV1, error) {
	definition, err := CurrentPrimarySubscriptionRoleTrialV1()
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
		SchemaVersion: PrimarySubscriptionRoleTrialReviewV1Schema, TrialID: ledger.TrialID,
		EvidenceSnapshotAt: ledger.CompletedAt, SourceLedgerSHA256: reviewHash(ledgerRaw), SourceManifestSHA256: ledger.EvidenceManifestSHA256,
		Verdict: "PASS", SessionCount: len(ledger.Sessions), ObservationCount: len(ledger.Observations), ProviderCalls: ledger.ProviderCalls,
		Retries: ledger.Retries, Fallbacks: ledger.Fallbacks, NativeToolExecutions: ledger.NativeToolExecutions,
		RepositoryAccesses: ledger.RepositoryAccesses, ProductionAccesses: ledger.ProductionAccesses, BusinessActions: ledger.BusinessActions,
		PhoneFarmAccesses: ledger.PhoneFarmAccesses, PermanentSelections: ledger.PermanentSelections,
		ConfigurationSelection: "HELD_NO_CONFIGURATION_QUALIFIED", PermanentSeatSelection: "HELD_NO_PERMANENT_SEAT_SELECTED",
		EvidenceBackedConclusions: []string{
			"The sealed trial produced five provider-free PASS observations out of eight; three terminal failures remain part of the result.",
			"Inline shared-executor plus exact-skill observations passed three of four cells; separate-specialist observations passed two of four cells. This single non-repeated sample does not select either topology.",
			"Both separate Product/UI observations failed at the downstream executor contract for different reasons; the evidence does not justify a persistent Product/UI seat.",
			"The lowest-complexity shared executor plus selected skill remains a candidate for further provider-free contract validation, not a qualified configuration.",
		},
		Limitations: []string{
			"Codex CLI receipts pin the requested model but do not expose trustworthy actual model identity.",
			"Claude Code may use disclosed background Haiku; all six receipts in this trial happened to report Sonnet only.",
			"Each lane-case-arm cell ran once, so the result does not estimate reliability.",
			"The reviewer can re-stat disclosed fixture paths but relies on the runner's boolean receipt for provider-profile cleanup because credential-bearing profile paths are intentionally not retained.",
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
	check("authority-and-definition", ledger.SchemaVersion == "silver-mediums.primary-subscription-role-trial-ledger.v1" && ledger.TrialID == definition.TrialID && ledger.DefinitionSHA256 == PrimarySubscriptionRoleTrialV1SHA256() && ledger.GateSHA256 == definition.GateSHA256 && ledger.OwnerAuthorizationHash == hex.EncodeToString(authority[:]), "ledger identity matches the frozen owner-authorized definition")
	check("terminal-counts", ledger.Status == "complete_verified" && len(ledger.Sessions) == 12 && len(ledger.Observations) == 8 && ledger.MaximumCLISessions == 12 && ledger.ProviderCalls == 12, "exactly 12 terminal sessions, 12 calls, and eight observations")
	check("forbidden-effects", ledger.Retries == 0 && ledger.Fallbacks == 0 && ledger.NativeToolExecutions == 0 && ledger.RepositoryAccesses == 0 && ledger.ProductionAccesses == 0 && ledger.BusinessActions == 0 && ledger.PhoneFarmAccesses == 0 && ledger.PermanentSelections == 0, "all forbidden-effect counters are zero")

	sessionsOK, rawOK, modelOK, cleanupOK, ceilingOK, callTotal := true, true, true, true, true, 0
	lanes := map[string]*PrimarySubscriptionTrialReviewLaneV1{}
	for index, session := range ledger.Sessions {
		invocation := definition.Invocations[index]
		sessionsOK = sessionsOK && session.Ordinal == invocation.Ordinal && session.ObservationID == invocation.ObservationID && session.Lane == invocation.Lane && session.Participant == invocation.Participant && session.Retries == 0 && !session.Fallbacks && len(session.NativeToolEvents) == 0 && session.Status != "reserved_no_replay"
		cleanupOK = cleanupOK && session.ProviderWorkdirRemoved
		callTotal += session.ProviderCallCount
		if session.RawResponsePath == "" || session.RawResponseSHA256 == "" || reviewFileHash(filepath.Join(root, filepath.FromSlash(session.RawResponsePath))) != session.RawResponseSHA256 {
			rawOK = false
		}
		ceilingOK = ceilingOK && session.InputTokens >= 0 && session.OutputTokens >= 0 && session.ReasoningTokens >= 0 && session.InputTokens+session.OutputTokens+session.ReasoningTokens <= definition.MaximumTotalReportedTokensPerSession && session.OutputTokens+session.ReasoningTokens <= definition.MaximumGeneratedTokensPerSession
		if session.Lane == "claude_code_subscription" {
			modelOK = modelOK && session.RequestedModel == "claude-sonnet-4-6" && session.RequestedEffort == "medium" && containsReviewString(session.ObservedModels, "claude-sonnet-4-6")
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
		if session.Status == "completed" {
			summary.Completed++
		} else if session.Status == "invalid" {
			summary.Invalid++
		}
		summary.InputTokens += session.InputTokens
		summary.OutputTokens += session.OutputTokens
		summary.CostMicros += session.CostMicros
		summary.WallMillis += session.WallMillis
	}
	lanes["claude_code_subscription"].ObservedModel = "claude-sonnet-4-6_only_in_six_observed_receipts"
	lanes["codex_subscription"].ObservedModel = "unavailable_truthful_requested_gpt-5.6-luna_pinned"
	for _, name := range []string{"claude_code_subscription", "codex_subscription"} {
		report.LaneSummaries = append(report.LaneSummaries, *lanes[name])
	}
	check("session-identity", sessionsOK && callTotal == ledger.ProviderCalls, "every session matches its frozen ordinal, lane, participant, and one-shot call receipt")
	check("raw-response-hashes", rawOK, "all 12 retained raw-response paths match their ledger SHA-256 values")
	check("model-attribution", modelOK, "Claude composite allowlist and Codex truthful-unavailable attribution remain bounded")
	check("resource-ceilings", ceilingOK, "all reported session token totals remain under frozen ceilings")
	check("provider-workdir-cleanup", cleanupOK, "all 12 runner receipts attest disposable provider-workdir removal")

	observationsOK, artifactsOK, browserOK := true, true, true
	arms := map[string]*PrimarySubscriptionTrialReviewArmV1{}
	for index, observation := range ledger.Observations {
		if index >= len(definition.Invocations) {
			observationsOK = false
			continue
		}
		observationsOK = observationsOK && observation.ObservationID != "" && observation.Lane != "" && observation.CaseID != "" && observation.Arm != "" && containsReviewString([]string{"complete", "invalid", "failed", "candidate_fail"}, observation.Status) && observation.FixtureRootRemoved && observation.InitialRootRemoved && pathIsAbsent(observation.Workspace.Root) && pathIsAbsent(observation.Workspace.InitialCopy)
		arm := arms[observation.Arm]
		if arm == nil {
			arm = &PrimarySubscriptionTrialReviewArmV1{Arm: observation.Arm}
			arms[observation.Arm] = arm
		}
		arm.Observations++
		switch observation.Status {
		case "complete":
			report.PassObservations++
			arm.Passes++
			observationRoot := filepath.Join(root, "observations", reviewSafeName(observation.ObservationID))
			artifactsOK = artifactsOK && observation.Grade != nil && observation.Grade.Verdict == "PASS" && observation.ToolReceiptSHA256 != "" && reviewFileHash(filepath.Join(observationRoot, "tool-receipt.json")) == observation.ToolReceiptSHA256
			if observation.CaseID == "R02" {
				browserOK = browserOK && reviewBrowserEvidence(observation.BrowserEvidenceRoot)
			}
		case "invalid":
			report.InvalidObservations++
			arm.Invalid++
		case "failed", "candidate_fail":
			report.FailedObservations++
			arm.Failed++
		}
	}
	for _, name := range []string{"shared_executor_with_exact_skill_packet", "conditional_specialist_then_shared_executor"} {
		report.ArmSummaries = append(report.ArmSummaries, *arms[name])
	}
	check("observation-terminal-cleanup", observationsOK && report.PassObservations == 5 && report.InvalidObservations == 2 && report.FailedObservations == 1, "all eight observations are terminal, absent on disk where required, and retain the observed 5/2/1 outcomes")
	check("hidden-verdict-artifacts", artifactsOK, "all five completed observations have PASS grades and matching tool-receipt hashes")
	check("browser-evidence", browserOK, "both completed R02 observations retain three hash-valid viewport captures and no capture token")

	correctionOK := len(ledger.Corrections) == 1
	if correctionOK {
		correction := ledger.Corrections[0]
		raw, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(correction.RecordPath)))
		var record struct {
			CorrectionID              string `json:"correction_id"`
			PreservedFailedOrdinal    int    `json:"preserved_failed_session_ordinal"`
			AfterNativeToolEventCount int    `json:"after_native_tool_event_count"`
			AuthContentRecorded       bool   `json:"auth_content_recorded"`
			ProviderCalls             int    `json:"provider_calls"`
			Retries                   int    `json:"retries"`
			Replays                   int    `json:"replays"`
		}
		decodeErr := json.Unmarshal(raw, &record)
		correctionOK = readErr == nil && decodeErr == nil && reviewHash(raw) == correction.RecordSHA256 && record.CorrectionID == correction.CorrectionID && record.PreservedFailedOrdinal == 7 && record.AfterNativeToolEventCount == 0 && !record.AuthContentRecorded && record.ProviderCalls == 0 && record.Retries == 0 && record.Replays == 0
	}
	check("bounded-correction", correctionOK, "one hash-valid correction preserves failed session 7 and records zero calls, retries, replays, tools, or credential content")
	manifest, manifestErr := reviewEvidenceManifest(root)
	check("evidence-manifest", manifestErr == nil && reviewHash(manifest) == ledger.EvidenceManifestSHA256, "sealed evidence manifest recomputes exactly")
	return report, nil
}

func reviewBrowserEvidence(root string) bool {
	if root == "" || !pathIsAbsent(filepath.Join(root, "browser-capture-token.txt")) {
		return false
	}
	raw, err := os.ReadFile(filepath.Join(root, "browser-evidence.json"))
	if err != nil {
		return false
	}
	var evidence primarySubscriptionReviewBrowserEvidence
	if json.Unmarshal(raw, &evidence) != nil || evidence.SchemaVersion != "silver-mediums.realistic-browser-evidence.v1" || len(evidence.Captures) != 3 {
		return false
	}
	want := []string{"narrow", "medium", "wide"}
	for index, capture := range evidence.Captures {
		if capture.Viewport != want[index] || reviewFileHash(filepath.Join(root, capture.ScreenshotArtifactPath)) != capture.ScreenshotSHA256 || reviewFileHash(filepath.Join(root, capture.DOMSnapshotArtifactPath)) != capture.DOMSnapshotSHA256 {
			return false
		}
	}
	return true
}

func reviewEvidenceManifest(root string) ([]byte, error) {
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
		rows = append(rows, filepath.ToSlash(relative)+"\x00"+reviewHash(raw)+"\n")
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(rows)
	return []byte(strings.Join(rows, "")), nil
}

func reviewFileHash(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return reviewHash(raw)
}

func reviewHash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func pathIsAbsent(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}

func containsReviewString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func reviewSafeName(value string) string {
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

func WritePrimarySubscriptionRoleTrialReviewV1(path string, report PrimarySubscriptionTrialReviewV1) error {
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if existing, err := os.ReadFile(path); err == nil {
		if reviewHash(existing) != reviewHash(raw) {
			return fmt.Errorf("review report already exists with different content")
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}
