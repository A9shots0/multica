package agentqualification

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	RoleQualificationExecutionSchemaV6 = "silver-mediums.role-qualification-execution.v6"
	preflightSchemaV6                  = "silver-mediums.role-qualification-preflight.v6"
	attemptSchemaV6                    = "silver-mediums.role-qualification-attempt.v6"
	transportReceiptSchemaV6           = "silver-mediums.role-qualification-transport-receipt.v6"
	observationSchemaV6                = "silver-mediums.role-qualification-observation.v6"
	legacyRecoverySchemaV6             = "silver-mediums.role-qualification-legacy-recovery.v6"
	LegacyV4StrandedCellIDV6           = "rqv3-25-nex-product_ui-rq-pui-03"
	LegacyV4StrandedClaimSHA256V6      = "e5395343d22c0dd67e255d0296a16308f5341205d52ef68382082761469c418b"
	LegacyV4StrandedPreflightSHA256V6  = "eade462c805eddac64db83e09906326a715ebb6b2578d13ebc0e961e9ce1a73d"
)

type RoleQualificationExecutionDefinitionV6 struct {
	SchemaVersion         string `json:"schema_version"`
	ExecutionID           string `json:"execution_id"`
	Status                string `json:"status"`
	V5FreezeSHA256        string `json:"v5_freeze_sha256"`
	MaxCellsPerInvocation int    `json:"max_cells_per_invocation"`
	FreshPreflightSeconds int64  `json:"fresh_preflight_seconds"`
	Retries               int    `json:"retries"`
	Fallbacks             bool   `json:"fallbacks"`
	TransportErrorScope   string `json:"transport_error_scope"`
	ContinueUnaffected    bool   `json:"continue_unaffected"`
	ProviderGate          string `json:"provider_gate"`
}
type RoleQualificationPreflightV6 RoleQualificationPreflightV4

type RoleQualificationAttemptV6 struct {
	SchemaVersion       string                   `json:"schema_version"`
	CellID              string                   `json:"cell_id"`
	FrozenRequestSHA256 string                   `json:"frozen_request_sha256"`
	ClaimSHA256         string                   `json:"claim_sha256"`
	PreflightSHA256     string                   `json:"preflight_sha256"`
	Reserved            RoleQualificationUsageV3 `json:"reserved"`
	AttemptOrdinal      int                      `json:"attempt_ordinal"`
	Retries             int                      `json:"retries"`
	Fallbacks           bool                     `json:"fallbacks"`
	PreparedAt          string                   `json:"prepared_at"`
	LegacyV4Recovery    bool                     `json:"legacy_v4_recovery"`
}

type RoleQualificationTransportStageV6 string
type RoleQualificationTransportCauseV6 string

const (
	RoleQualificationStagePreProviderV6 RoleQualificationTransportStageV6 = "pre_provider"
	RoleQualificationStageProviderV6    RoleQualificationTransportStageV6 = "provider_request"
	RoleQualificationStageResponseV6    RoleQualificationTransportStageV6 = "response_read"
	RoleQualificationStageParseV6       RoleQualificationTransportStageV6 = "response_parse"
	RoleQualificationCauseTimeoutV6     RoleQualificationTransportCauseV6 = "timeout"
	RoleQualificationCauseConnectionV6  RoleQualificationTransportCauseV6 = "connection_error"
	RoleQualificationCauseReadV6        RoleQualificationTransportCauseV6 = "response_read_error"
	RoleQualificationCauseRouteV6       RoleQualificationTransportCauseV6 = "route_drift"
	RoleQualificationCauseModelV6       RoleQualificationTransportCauseV6 = "model_drift"
	RoleQualificationCauseProviderV6    RoleQualificationTransportCauseV6 = "provider_drift"
	RoleQualificationCauseTurnV6        RoleQualificationTransportCauseV6 = "turn_drift"
	RoleQualificationCauseMalformedV6   RoleQualificationTransportCauseV6 = "malformed_evidence"
	RoleQualificationCauseCredentialV6  RoleQualificationTransportCauseV6 = "credential_failure"
	RoleQualificationCauseAuthV6        RoleQualificationTransportCauseV6 = "auth_failure"
	RoleQualificationCauseRuntimeV6     RoleQualificationTransportCauseV6 = "runtime_failure"
	RoleQualificationCauseConfigV6      RoleQualificationTransportCauseV6 = "config_failure"
	RoleQualificationCauseInterruptedV6 RoleQualificationTransportCauseV6 = "interrupted"
)

type RoleQualificationTransportFailureV6 struct {
	Stage               RoleQualificationTransportStageV6 `json:"stage"`
	Cause               RoleQualificationTransportCauseV6 `json:"cause"`
	ProviderCallStarted bool                              `json:"provider_call_started"`
	UsageKnown          bool                              `json:"usage_known"`
	StartedAt           string                            `json:"started_at"`
	CompletedAt         string                            `json:"completed_at"`
	ErrorDetail         string                            `json:"error_detail"`
	ReportedUsage       RoleQualificationUsageV3          `json:"reported_usage"`
	TimingKnown         bool                              `json:"timing_known"`
	HTTPStatus          int                               `json:"http_status"`
	ContentType         string                            `json:"content_type"`
	PartialBody         []byte                            `json:"partial_body,omitempty"`
	ReadError           string                            `json:"read_error,omitempty"`
}
type RoleQualificationProviderResultV6 struct {
	RawResponse      json.RawMessage                      `json:"raw_response,omitempty"`
	OpenCodeEvidence *RoleQualificationOpenCodeEvidenceV6 `json:"opencode_evidence,omitempty"`
	TransportFailure *RoleQualificationTransportFailureV6 `json:"transport_failure,omitempty"`
}
type RoleQualificationProviderV6 interface {
	Call(context.Context, RoleQualificationExecutionCellV3) RoleQualificationProviderResultV6
}

type RoleQualificationTransportReceiptV6 struct {
	SchemaVersion       string                            `json:"schema_version"`
	CellID              string                            `json:"cell_id"`
	FrozenRequestSHA256 string                            `json:"frozen_request_sha256"`
	AttemptSHA256       string                            `json:"attempt_sha256"`
	Stage               RoleQualificationTransportStageV6 `json:"stage"`
	Cause               RoleQualificationTransportCauseV6 `json:"cause"`
	ProviderCallStarted bool                              `json:"provider_call_started"`
	UsageKnown          bool                              `json:"usage_known"`
	StartedAt           string                            `json:"started_at"`
	CompletedAt         string                            `json:"completed_at"`
	ErrorDetail         string                            `json:"error_detail"`
	ErrorDetailSHA256   string                            `json:"error_detail_sha256"`
	ReportedUsage       RoleQualificationUsageV3          `json:"reported_usage"`
	AccountedUsage      RoleQualificationUsageV3          `json:"accounted_usage"`
	SealedAt            string                            `json:"sealed_at"`
	LegacyV4Recovery    bool                              `json:"legacy_v4_recovery"`
	FailureScope        string                            `json:"failure_scope"`
	TimingKnown         bool                              `json:"timing_known"`
	HTTPStatus          int                               `json:"http_status"`
	ContentType         string                            `json:"content_type"`
	PartialBody         []byte                            `json:"partial_body,omitempty"`
	PartialBodySHA256   string                            `json:"partial_body_sha256,omitempty"`
	ReadError           string                            `json:"read_error,omitempty"`
	ReadErrorSHA256     string                            `json:"read_error_sha256,omitempty"`
}
type RoleQualificationObservationV6 struct {
	RoleQualificationObservationV3
	AttemptSHA256         string `json:"attempt_sha256"`
	TerminalReceiptSHA256 string `json:"terminal_receipt_sha256"`
	UsageKnown            bool   `json:"usage_known"`
	ProviderCallStarted   bool   `json:"provider_call_started"`
}
type RoleQualificationReplayV6 struct {
	Claims             int                      `json:"claims"`
	Attempts           int                      `json:"attempts"`
	SuccessReceipts    int                      `json:"success_receipts"`
	TransportReceipts  int                      `json:"transport_receipts"`
	Observations       int                      `json:"observations"`
	Totals             RoleQualificationUsageV3 `json:"totals"`
	GlobalHalt         bool                     `json:"global_halt"`
	GlobalHaltClass    string                   `json:"global_halt_class,omitempty"`
	EliminatedBindings []string                 `json:"eliminated_configuration_role_bindings,omitempty"`
	LocalUnscored      []string                 `json:"observation_local_unscored,omitempty"`
}
type RoleQualificationLegacyRecoveryV6 struct {
	SchemaVersion         string `json:"schema_version"`
	LegacyClaimSHA256     string `json:"legacy_claim_sha256"`
	LegacyPreflightSHA256 string `json:"legacy_preflight_sha256"`
	CellID                string `json:"cell_id"`
	Disposition           string `json:"disposition"`
	RecoveredAt           string `json:"recovered_at"`
}

const openCodeEvidenceSchemaV6 = "silver-mediums.role-qualification-opencode-http-evidence.v6"

// RoleQualificationOpenCodeEvidenceV6 preserves the exact request capsule and
// HTTP response bytes from which every OpenCode success/protocol-failure
// observation is derived. It is sealed before the normalized provider receipt.
type RoleQualificationOpenCodeEvidenceV6 struct {
	SchemaVersion       string                       `json:"schema_version"`
	CellID              string                       `json:"cell_id"`
	FrozenRequestSHA256 string                       `json:"frozen_request_sha256"`
	AttemptSHA256       string                       `json:"attempt_sha256"`
	Capsule             OpenCodeStructuredV33Capsule `json:"capsule"`
	CapsuleSHA256       string                       `json:"capsule_sha256"`
	HTTPStatus          int                          `json:"http_status"`
	ContentType         string                       `json:"content_type"`
	RawHTTPResponse     json.RawMessage              `json:"raw_http_response"`
	RawHTTPResponseSHA  string                       `json:"raw_http_response_sha256"`
	StartedAt           string                       `json:"started_at"`
	CompletedAt         string                       `json:"completed_at"`
	SealedAt            string                       `json:"sealed_at"`
}

//go:embed role_qualification_execution_v6.json
var roleQualificationExecutionV6JSON []byte

func CurrentRoleQualificationExecutionDefinitionV6() (RoleQualificationExecutionDefinitionV6, error) {
	var d RoleQualificationExecutionDefinitionV6
	if err := strictJSON(roleQualificationExecutionV6JSON, &d); err != nil {
		return d, err
	}
	if d.SchemaVersion != RoleQualificationExecutionSchemaV6 || d.ExecutionID != "role-qualification-structured-outcome-v6" || d.Status != "frozen_preflight_required_not_executed" || d.V5FreezeSHA256 != "cbe3d5125cb28b716c0db5423b223db1b192aaea21ec73a7925f9c46e4c90c8b" || d.V5FreezeSHA256 != RoleQualificationExecutionFreezeV5SHA256() || d.MaxCellsPerInvocation != 12 || d.FreshPreflightSeconds != 600 || d.Retries != 0 || d.Fallbacks || d.TransportErrorScope != "observation_local_unscored" || !d.ContinueUnaffected || d.ProviderGate != "exact_v6_freeze_plus_fresh_v6_preflight" {
		return d, fmt.Errorf("v6 definition drift")
	}
	return d, nil
}

func ValidateRoleQualificationPreflightV6(p RoleQualificationPreflightV6, classID string, now time.Time) error {
	x := RoleQualificationPreflightV4(p)
	x.SchemaVersion = "silver-mediums.role-qualification-preflight.v4"
	x.ExecutionFreezeSHA256 = RoleQualificationExecutionFreezeV4SHA256()
	if err := ValidateRoleQualificationPreflightV4(x, classID, now); err != nil {
		return err
	}
	if p.SchemaVersion != preflightSchemaV6 || p.ExecutionFreezeSHA256 != RoleQualificationExecutionFreezeV6SHA256() {
		return fmt.Errorf("v6 preflight identity")
	}
	return nil
}
func WriteRoleQualificationPreflightV6(root string, p RoleQualificationPreflightV6) error {
	if err := ValidateRoleQualificationPreflightV6(p, p.ConfigurationClassID, time.Now().UTC()); err != nil {
		return err
	}
	return writeOnceRoleQualificationJSONV3(filepath.Join(root, "preflight", p.ConfigurationClassID+".json"), p)
}

func roleQualificationOpenCodeProviderRawV6(cell RoleQualificationExecutionCellV3, evidence RoleQualificationOpenCodeEvidenceV6) (json.RawMessage, string, error) {
	receipt, err := ParseOpenCodeStructuredV33HTTPResponse(evidence.Capsule, evidence.HTTPStatus, evidence.ContentType, evidence.RawHTTPResponse)
	if err != nil {
		return nil, "", err
	}
	model := strings.TrimPrefix(cell.ConfigurationClass.Model, "openrouter/")
	if receipt.ProviderID != "openrouter" || receipt.ModelID != model || receipt.Variant != cell.ConfigurationClass.Effort || receipt.ProviderTurns != 1 {
		return nil, "", fmt.Errorf("v6 OpenCode route/turn drift")
	}
	started, e1 := time.Parse(time.RFC3339Nano, evidence.StartedAt)
	completed, e2 := time.Parse(time.RFC3339Nano, evidence.CompletedAt)
	if e1 != nil || e2 != nil || completed.Before(started) {
		return nil, "", fmt.Errorf("v6 OpenCode timing invalid")
	}
	total, ok := safeAddV33(receipt.Usage.InputTokens, receipt.Usage.OutputTokens, receipt.Usage.ReasoningTokens)
	if !ok {
		return nil, "", fmt.Errorf("v6 OpenCode token overflow")
	}
	failure := ""
	structured := append(json.RawMessage(nil), receipt.Structured...)
	switch receipt.Outcome {
	case "structured":
		if len(structured) == 0 {
			return nil, "", fmt.Errorf("v6 structured outcome lacks output")
		}
	case "structured_output_error":
		failure = "response_or_tool_protocol_failure"
		structured = nil
	default:
		return nil, "", fmt.Errorf("v6 unsupported OpenCode outcome %q", receipt.Outcome)
	}
	u := RoleQualificationUsageV3{Calls: 1, InputTokens: receipt.Usage.InputTokens, OutputTokens: receipt.Usage.OutputTokens, ReasoningTokens: receipt.Usage.ReasoningTokens, TotalTokens: total, OpenRouterCashMicros: receipt.Usage.CostMicros, WallMillis: completed.Sub(started).Milliseconds()}
	p := RoleQualificationRawProviderResponseV3{SchemaVersion: "silver-mediums.role-qualification-raw-provider-response.v3", ProviderCallID: receipt.AssistantMessageID, ActualModel: cell.ConfigurationClass.Model, ActualProvider: cell.ConfigurationClass.ServingProvider, ActualEffort: cell.ConfigurationClass.Effort, ActualTransport: RoleQualificationOpenCodeTransportV3, FallbackUsed: false, RetryCount: 0, StartedAt: evidence.StartedAt, CompletedAt: evidence.CompletedAt, StructuredOutput: structured, FailureClass: failure, Usage: u}
	raw, err := json.Marshal(p)
	return raw, receipt.Outcome, err
}

func validateRoleQualificationOpenCodeEvidenceV6(cell RoleQualificationExecutionCellV3, attempt RoleQualificationAttemptV6, evidence RoleQualificationOpenCodeEvidenceV6) (json.RawMessage, string, error) {
	attemptRaw, _ := json.Marshal(attempt)
	capsuleRaw, err := json.Marshal(evidence.Capsule)
	if err != nil {
		return nil, "", err
	}
	if evidence.SchemaVersion != openCodeEvidenceSchemaV6 || evidence.CellID != cell.CellID || evidence.FrozenRequestSHA256 != cell.RequestSHA256 || evidence.AttemptSHA256 != sha256Hex(attemptRaw) || evidence.CapsuleSHA256 != sha256Hex(capsuleRaw) || evidence.RawHTTPResponseSHA != sha256Hex(evidence.RawHTTPResponse) || len(evidence.RawHTTPResponse) == 0 || !json.Valid(evidence.RawHTTPResponse) {
		return nil, "", fmt.Errorf("v6 OpenCode immutable evidence drift")
	}
	c := evidence.Capsule
	if c.BindingID != cell.CellID || c.OpenCodeVersion != OpenCodeStructuredV33Version || c.Transport != OpenCodeStructuredV33Transport || c.Method != "POST" || c.SessionID == "" || c.RequestMessageID == "" || c.Workspace == "" || c.RoutePolicy != "enforced_single_route_no_fallback" || c.RetryCount != 0 || c.HTTPTimeoutSeconds != OpenCodeStructuredV33HTTPTimeoutSeconds || c.GeneratedTokenCeiling != cell.Request.MaxOutputTokens || !c.ToolsDenied || c.CLIFormatJSONEnforcement || c.RequestBodySHA256 != sha256Hex(c.RequestBody) || c.InlineConfigSHA256 != sha256Hex(c.InlineConfig) || c.ResolvedConfigSHA256 == "" {
		return nil, "", fmt.Errorf("v6 OpenCode capsule drift")
	}
	if _, err := time.Parse(time.RFC3339Nano, evidence.SealedAt); err != nil {
		return nil, "", fmt.Errorf("v6 OpenCode seal time")
	}
	return roleQualificationOpenCodeProviderRawV6(cell, evidence)
}

func SealRoleQualificationOpenCodeEvidenceV6(root string, cell RoleQualificationExecutionCellV3, evidence RoleQualificationOpenCodeEvidenceV6) error {
	var attempt RoleQualificationAttemptV6
	if err := readStrictJSONFileV3(filepath.Join(root, "attempts", cell.CellID+".json"), &attempt); err != nil {
		return err
	}
	attemptRaw, _ := json.Marshal(attempt)
	if evidence.AttemptSHA256 == "" {
		evidence.AttemptSHA256 = sha256Hex(attemptRaw)
	}
	raw, _, err := validateRoleQualificationOpenCodeEvidenceV6(cell, attempt, evidence)
	if err != nil {
		return err
	}
	if err := writeOnceRoleQualificationJSONV3(filepath.Join(root, "opencode-http-receipts", cell.CellID+".json"), evidence); err != nil {
		return err
	}
	r := RoleQualificationProviderReceiptV3{SchemaVersion: "silver-mediums.role-qualification-provider-receipt.v3", CellID: cell.CellID, FrozenRequestSHA256: cell.RequestSHA256, RawResponse: raw, RawResponseSHA256: sha256Hex(raw), SealedAt: evidence.SealedAt}
	return SealRoleQualificationRawReceiptV3(root, r)
}

// ReconcileRoleQualificationOpenCodeEvidenceV6 completes a crash interrupted
// after the immutable HTTP seal but before the normalized receipt seal. It is
// provider-free and never retries a paid turn.
func ReconcileRoleQualificationOpenCodeEvidenceV6(root string) error {
	plan, err := BuildRoleQualificationExecutionPlanV3()
	if err != nil {
		return err
	}
	for _, cell := range plan.Cells {
		path := filepath.Join(root, "opencode-http-receipts", cell.CellID+".json")
		if !fileExistsV3(path) {
			continue
		}
		var evidence RoleQualificationOpenCodeEvidenceV6
		if err := readStrictJSONFileV3(path, &evidence); err != nil {
			return err
		}
		var attempt RoleQualificationAttemptV6
		if err := readStrictJSONFileV3(filepath.Join(root, "attempts", cell.CellID+".json"), &attempt); err != nil {
			return err
		}
		raw, _, err := validateRoleQualificationOpenCodeEvidenceV6(cell, attempt, evidence)
		if err != nil {
			return err
		}
		rp := filepath.Join(root, "receipts", cell.CellID+".json")
		if !fileExistsV3(rp) {
			r := RoleQualificationProviderReceiptV3{SchemaVersion: "silver-mediums.role-qualification-provider-receipt.v3", CellID: cell.CellID, FrozenRequestSHA256: cell.RequestSHA256, RawResponse: raw, RawResponseSHA256: sha256Hex(raw), SealedAt: evidence.SealedAt}
			if err := SealRoleQualificationRawReceiptV3(root, r); err != nil {
				return err
			}
		}
	}
	return nil
}

func PrepareRoleQualificationGrantV6(root, cellID, now string) (RoleQualificationClaimV3, error) {
	plan, err := BuildRoleQualificationExecutionPlanV3()
	if err != nil {
		return RoleQualificationClaimV3{}, err
	}
	replay, err := RoleQualificationExecutionReplayV6(root)
	if err != nil {
		return RoleQualificationClaimV3{}, err
	}
	if replay.GlobalHalt {
		return RoleQualificationClaimV3{}, fmt.Errorf("global halt %s", replay.GlobalHaltClass)
	}
	cell, ok := findRoleQualificationCellV3(plan, cellID)
	if !ok {
		return RoleQualificationClaimV3{}, fmt.Errorf("unknown cell")
	}
	if containsStringV6(replay.EliminatedBindings, cell.ConfigurationClass.ClassID+":"+cell.CandidateClass) {
		return RoleQualificationClaimV3{}, fmt.Errorf("candidate-local elimination")
	}
	reserved := reservationForRoleQualificationCellV3(cell, plan.PerObservation)
	projected := replay.Totals
	if err := addRoleQualificationUsageV3(&projected, reserved); err != nil {
		return RoleQualificationClaimV3{}, err
	}
	if err := validateRoleQualificationCampaignUsageV3(projected, plan.Campaign); err != nil {
		return RoleQualificationClaimV3{}, fmt.Errorf("global campaign reservation breach: %w", err)
	}
	if now == "" {
		now = time.Now().UTC().Format(time.RFC3339Nano)
	}
	claim := RoleQualificationClaimV3{"silver-mediums.role-qualification-claim.v3", cellID, cell.RequestSHA256, now, reserved, 0, false}
	return claim, writeOnceRoleQualificationJSONV3(filepath.Join(root, "claims", cellID+".json"), claim)
}

func PrepareRoleQualificationAttemptV6(root string, cell RoleQualificationExecutionCellV3, preparedAt string) (RoleQualificationAttemptV6, error) {
	claimRaw, err := os.ReadFile(filepath.Join(root, "claims", cell.CellID+".json"))
	if err != nil {
		return RoleQualificationAttemptV6{}, err
	}
	var claim RoleQualificationClaimV3
	if err := strictJSON(claimRaw, &claim); err != nil {
		return RoleQualificationAttemptV6{}, err
	}
	preflightRaw, err := os.ReadFile(filepath.Join(root, "preflight", cell.ConfigurationClass.ClassID+".json"))
	if err != nil {
		return RoleQualificationAttemptV6{}, err
	}
	if preparedAt == "" {
		preparedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	a := RoleQualificationAttemptV6{attemptSchemaV6, cell.CellID, cell.RequestSHA256, sha256Hex(claimRaw), sha256Hex(preflightRaw), claim.Reserved, 1, 0, false, preparedAt, false}
	if err := validateRoleQualificationAttemptV6(cell, claim, a); err != nil {
		return RoleQualificationAttemptV6{}, err
	}
	return a, writeOnceRoleQualificationJSONV3(filepath.Join(root, "attempts", cell.CellID+".json"), a)
}
func validateRoleQualificationAttemptV6(cell RoleQualificationExecutionCellV3, claim RoleQualificationClaimV3, a RoleQualificationAttemptV6) error {
	if a.SchemaVersion != attemptSchemaV6 || a.CellID != cell.CellID || a.FrozenRequestSHA256 != cell.RequestSHA256 || a.Reserved != claim.Reserved || a.AttemptOrdinal != 1 || a.Retries != 0 || a.Fallbacks || !validSHA256(a.ClaimSHA256) || !validSHA256(a.PreflightSHA256) {
		return fmt.Errorf("attempt drift")
	}
	if _, err := time.Parse(time.RFC3339Nano, a.PreparedAt); err != nil {
		return fmt.Errorf("attempt timestamp")
	}
	return nil
}
func validTransportStageV6(v RoleQualificationTransportStageV6) bool {
	return v == RoleQualificationStagePreProviderV6 || v == RoleQualificationStageProviderV6 || v == RoleQualificationStageResponseV6 || v == RoleQualificationStageParseV6
}
func validTransportCauseV6(v RoleQualificationTransportCauseV6) bool {
	switch v {
	case RoleQualificationCauseTimeoutV6, RoleQualificationCauseConnectionV6, RoleQualificationCauseReadV6, RoleQualificationCauseRouteV6, RoleQualificationCauseModelV6, RoleQualificationCauseProviderV6, RoleQualificationCauseTurnV6, RoleQualificationCauseMalformedV6, RoleQualificationCauseCredentialV6, RoleQualificationCauseAuthV6, RoleQualificationCauseRuntimeV6, RoleQualificationCauseConfigV6, RoleQualificationCauseInterruptedV6:
		return true
	default:
		return false
	}
}

func failureScopeForCauseV6(v RoleQualificationTransportCauseV6) string {
	if v == RoleQualificationCauseTimeoutV6 || v == RoleQualificationCauseConnectionV6 || v == RoleQualificationCauseReadV6 || v == RoleQualificationCauseInterruptedV6 {
		return "observation_local_unscored"
	}
	return "global_halt"
}

func SealRoleQualificationTransportFailureV6(root string, cell RoleQualificationExecutionCellV3, f RoleQualificationTransportFailureV6, sealedAt string) error {
	var claim RoleQualificationClaimV3
	if err := readStrictJSONFileV3(filepath.Join(root, "claims", cell.CellID+".json"), &claim); err != nil {
		return err
	}
	var attempt RoleQualificationAttemptV6
	if err := readStrictJSONFileV3(filepath.Join(root, "attempts", cell.CellID+".json"), &attempt); err != nil {
		return err
	}
	if !validTransportStageV6(f.Stage) || !validTransportCauseV6(f.Cause) || f.ErrorDetail == "" {
		return fmt.Errorf("typed transport failure invalid")
	}
	start, e1 := time.Parse(time.RFC3339Nano, f.StartedAt)
	end, e2 := time.Parse(time.RFC3339Nano, f.CompletedAt)
	if e1 != nil || e2 != nil || end.Before(start) || validateRoleQualificationUsageShapeV3(f.ReportedUsage) != nil {
		return fmt.Errorf("transport timing or usage invalid")
	}
	wall := end.Sub(start).Milliseconds()
	if f.TimingKnown {
		if f.ReportedUsage.WallMillis != wall {
			return fmt.Errorf("transport wall mismatch")
		}
	} else if f.ReportedUsage.WallMillis != 0 {
		return fmt.Errorf("unknown timing invented")
	}
	accounted := f.ReportedUsage
	if !f.ProviderCallStarted {
		if !f.UsageKnown || f.ReportedUsage != (RoleQualificationUsageV3{WallMillis: f.ReportedUsage.WallMillis}) {
			return fmt.Errorf("pre-provider failure must report known zero provider usage")
		}
	} else if !f.UsageKnown {
		if f.ReportedUsage != (RoleQualificationUsageV3{WallMillis: f.ReportedUsage.WallMillis}) {
			return fmt.Errorf("unknown provider usage must not be invented")
		}
		accounted = claim.Reserved
	} else if f.ReportedUsage.Calls != 1 || f.ReportedUsage.WallMillis != wall {
		return fmt.Errorf("known provider usage accounting")
	}
	if len(f.PartialBody) == 0 && f.HTTPStatus == 0 && f.ContentType != "" {
		return fmt.Errorf("content type without HTTP response")
	}
	attemptRaw, _ := json.Marshal(attempt)
	if sealedAt == "" {
		sealedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	r := RoleQualificationTransportReceiptV6{SchemaVersion: transportReceiptSchemaV6, CellID: cell.CellID, FrozenRequestSHA256: cell.RequestSHA256, AttemptSHA256: sha256Hex(attemptRaw), Stage: f.Stage, Cause: f.Cause, ProviderCallStarted: f.ProviderCallStarted, UsageKnown: f.UsageKnown, StartedAt: f.StartedAt, CompletedAt: f.CompletedAt, ErrorDetail: f.ErrorDetail, ErrorDetailSHA256: sha256Hex([]byte(f.ErrorDetail)), ReportedUsage: f.ReportedUsage, AccountedUsage: accounted, SealedAt: sealedAt, FailureScope: failureScopeForCauseV6(f.Cause), TimingKnown: f.TimingKnown, HTTPStatus: f.HTTPStatus, ContentType: f.ContentType, PartialBody: append([]byte(nil), f.PartialBody...), PartialBodySHA256: hashOptionalBytesV6(f.PartialBody), ReadError: f.ReadError, ReadErrorSHA256: hashOptionalStringV6(f.ReadError)}
	if _, err := time.Parse(time.RFC3339Nano, sealedAt); err != nil {
		return err
	}
	return writeOnceRoleQualificationJSONV3(filepath.Join(root, "transport-receipts", cell.CellID+".json"), r)
}

func ExecuteRoleQualificationBatchV6(ctx context.Context, root, classID string, now time.Time, provider RoleQualificationProviderV6) (RoleQualificationReplayV6, error) {
	d, err := CurrentRoleQualificationExecutionDefinitionV6()
	if err != nil {
		return RoleQualificationReplayV6{}, err
	}
	if _, err := CurrentRoleQualificationExecutionFreezeV6(); err != nil {
		return RoleQualificationReplayV6{}, err
	}
	var p RoleQualificationPreflightV6
	if err := readStrictJSONFileV3(filepath.Join(root, "preflight", classID+".json"), &p); err != nil {
		return RoleQualificationReplayV6{}, err
	}
	if err := ValidateRoleQualificationPreflightV6(p, classID, now); err != nil {
		return RoleQualificationReplayV6{}, err
	}
	cells, err := RoleQualificationClassCellsV4(classID)
	if err != nil || len(cells) > d.MaxCellsPerInvocation {
		return RoleQualificationReplayV6{}, fmt.Errorf("v6 batch scope")
	}
	if len(cells) > 0 && cells[0].ConfigurationClass.RuntimeProvider == "claude_code_subscription" {
		return RoleQualificationReplayV6{}, fmt.Errorf("v6 Claude subscription class held incomplete: multi-model modelUsage route evidence unresolved")
	}
	if err := ReconcileRoleQualificationOpenCodeEvidenceV6(root); err != nil {
		return RoleQualificationReplayV6{}, err
	}
	if err := ReconcileRoleQualificationOpenAttemptsV6(root, now.Format(time.RFC3339Nano)); err != nil {
		return RoleQualificationReplayV6{}, err
	}
	for _, cell := range cells {
		replay, err := RoleQualificationExecutionReplayV6(root)
		if err != nil {
			return replay, err
		}
		if replay.GlobalHalt {
			return replay, fmt.Errorf("global halt %s", replay.GlobalHaltClass)
		}
		// A sealed observation is terminal regardless of pass/fail/local-invalid.
		// Replay above has already re-derived it from immutable evidence, so skip
		// before grant and never create a duplicate paid attempt.
		if fileExistsV3(filepath.Join(root, "observations", cell.CellID+".json")) {
			continue
		}
		if containsStringV6(replay.EliminatedBindings, cell.ConfigurationClass.ClassID+":"+cell.CandidateClass) {
			continue
		}
		if _, err := PrepareRoleQualificationGrantV6(root, cell.CellID, now.Format(time.RFC3339Nano)); err != nil {
			return replay, err
		}
		if _, err := PrepareRoleQualificationAttemptV6(root, cell, now.Format(time.RFC3339Nano)); err != nil {
			return replay, err
		}
		result := provider.Call(ctx, cell)
		if (len(result.RawResponse) > 0) == (result.TransportFailure != nil) || (result.TransportFailure != nil && result.OpenCodeEvidence != nil) {
			return RoleQualificationExecutionReplayV6(root)
		}
		if result.TransportFailure != nil {
			if err := SealRoleQualificationTransportFailureV6(root, cell, *result.TransportFailure, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
				return RoleQualificationExecutionReplayV6(root)
			}
		} else {
			if result.OpenCodeEvidence == nil {
				return RoleQualificationExecutionReplayV6(root)
			}
			if err := SealRoleQualificationOpenCodeEvidenceV6(root, cell, *result.OpenCodeEvidence); err != nil {
				return RoleQualificationExecutionReplayV6(root)
			}
		}
		if err := ReconcileRoleQualificationClaimV6(root, cell.CellID); err != nil {
			return RoleQualificationExecutionReplayV6(root)
		}
	}
	return RoleQualificationExecutionReplayV6(root)
}

// ReconcileRoleQualificationOpenAttemptsV6 closes a durable attempt which has
// no terminal receipt as ambiguous local evidence. It never calls a provider.
func ReconcileRoleQualificationOpenAttemptsV6(root, reconciledAt string) error {
	claims, err := readRoleQualificationClaimsV3(root)
	if err != nil {
		return err
	}
	plan, err := BuildRoleQualificationExecutionPlanV3()
	if err != nil {
		return err
	}
	byID := map[string]RoleQualificationExecutionCellV3{}
	for _, cell := range plan.Cells {
		byID[cell.CellID] = cell
	}
	if reconciledAt == "" {
		reconciledAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	for id := range claims {
		attemptPath := filepath.Join(root, "attempts", id+".json")
		if !fileExistsV3(attemptPath) {
			continue
		}
		rp, tp := filepath.Join(root, "receipts", id+".json"), filepath.Join(root, "transport-receipts", id+".json")
		if fileExistsV3(rp) || fileExistsV3(tp) {
			if fileExistsV3(rp) != fileExistsV3(tp) {
				if err := ReconcileRoleQualificationClaimV6(root, id); err != nil {
					return err
				}
			}
			continue
		}
		cell, ok := byID[id]
		if !ok {
			return fmt.Errorf("unknown attempted cell")
		}
		var attempt RoleQualificationAttemptV6
		if err := readStrictJSONFileV3(attemptPath, &attempt); err != nil {
			return err
		}
		failure := RoleQualificationTransportFailureV6{Stage: RoleQualificationStageProviderV6, Cause: RoleQualificationCauseInterruptedV6, ProviderCallStarted: true, UsageKnown: false, StartedAt: attempt.PreparedAt, CompletedAt: attempt.PreparedAt, ErrorDetail: "durable attempt found without authoritative terminal receipt; no retry permitted", TimingKnown: false}
		if err := SealRoleQualificationTransportFailureV6(root, cell, failure, reconciledAt); err != nil {
			return err
		}
		if err := ReconcileRoleQualificationClaimV6(root, id); err != nil {
			return err
		}
	}
	return nil
}

func ReconcileRoleQualificationClaimV6(root, cellID string) error {
	plan, err := BuildRoleQualificationExecutionPlanV3()
	if err != nil {
		return err
	}
	cell, ok := findRoleQualificationCellV3(plan, cellID)
	if !ok {
		return fmt.Errorf("unknown cell")
	}
	op := filepath.Join(root, "observations", cellID+".json")
	if fileExistsV3(op) {
		var existing RoleQualificationObservationV6
		if err := readStrictJSONFileV3(op, &existing); err != nil {
			return err
		}
		return ensureRoleQualificationLedgerEntryV6(root, existing)
	}
	rp, tp := filepath.Join(root, "receipts", cellID+".json"), filepath.Join(root, "transport-receipts", cellID+".json")
	if fileExistsV3(rp) == fileExistsV3(tp) {
		return fmt.Errorf("terminal receipt XOR violation")
	}
	var out RoleQualificationObservationV6
	if fileExistsV3(tp) {
		var r RoleQualificationTransportReceiptV6
		if err := readStrictJSONFileV3(tp, &r); err != nil {
			return err
		}
		var a RoleQualificationAttemptV6
		if err := readStrictJSONFileV3(filepath.Join(root, "attempts", cellID+".json"), &a); err != nil {
			return err
		}
		receiptRaw, _ := json.Marshal(r)
		var claim RoleQualificationClaimV3
		if err := readStrictJSONFileV3(filepath.Join(root, "claims", cellID+".json"), &claim); err != nil {
			return err
		}
		if err := validateRoleQualificationTransportReceiptV6(cell, claim, a, r); err != nil {
			return fmt.Errorf("transport receipt drift")
		}
		status, class := "invalid_unscored", "observation_transport_"+string(r.Cause)
		if r.FailureScope == "global_halt" {
			status = "failed"
		}
		base := RoleQualificationObservationV3{SchemaVersion: observationSchemaV6, CellID: cellID, FrozenRequestSHA256: cell.RequestSHA256, RawReceiptSHA256: sha256Hex(receiptRaw), Status: status, FailureScope: r.FailureScope, FailureClass: class, Usage: r.AccountedUsage}
		out = RoleQualificationObservationV6{base, r.AttemptSHA256, sha256Hex(receiptRaw), r.UsageKnown, r.ProviderCallStarted}
	} else {
		var r RoleQualificationProviderReceiptV3
		if err := readStrictJSONFileV3(rp, &r); err != nil {
			return err
		}
		base, err := deriveRoleQualificationObservationV3(cell, r, plan.PerObservation)
		if err != nil {
			return err
		}
		base.SchemaVersion = observationSchemaV6
		var a RoleQualificationAttemptV6
		if err := readStrictJSONFileV3(filepath.Join(root, "attempts", cellID+".json"), &a); err != nil {
			return err
		}
		attemptRaw, _ := json.Marshal(a)
		receiptRaw, _ := json.Marshal(r)
		out = RoleQualificationObservationV6{base, sha256Hex(attemptRaw), sha256Hex(receiptRaw), true, true}
	}
	if err := writeOnceRoleQualificationJSONV3(op, out); err != nil {
		return err
	}
	raw, _ := json.Marshal(out)
	return appendRoleQualificationLedgerV3(root, raw)
}

func validateRoleQualificationTransportReceiptV6(cell RoleQualificationExecutionCellV3, claim RoleQualificationClaimV3, attempt RoleQualificationAttemptV6, r RoleQualificationTransportReceiptV6) error {
	attemptRaw, _ := json.Marshal(attempt)
	if r.SchemaVersion != transportReceiptSchemaV6 || r.CellID != cell.CellID || r.FrozenRequestSHA256 != cell.RequestSHA256 || r.AttemptSHA256 != sha256Hex(attemptRaw) || !validTransportStageV6(r.Stage) || !validTransportCauseV6(r.Cause) || r.ErrorDetail == "" || r.ErrorDetailSHA256 != sha256Hex([]byte(r.ErrorDetail)) || r.LegacyV4Recovery != attempt.LegacyV4Recovery || r.FailureScope != failureScopeForCauseV6(r.Cause) || r.PartialBodySHA256 != hashOptionalBytesV6(r.PartialBody) || r.ReadErrorSHA256 != hashOptionalStringV6(r.ReadError) {
		return fmt.Errorf("transport receipt drift")
	}
	start, e1 := time.Parse(time.RFC3339Nano, r.StartedAt)
	end, e2 := time.Parse(time.RFC3339Nano, r.CompletedAt)
	if e1 != nil || e2 != nil || end.Before(start) || validateRoleQualificationUsageShapeV3(r.ReportedUsage) != nil || validateRoleQualificationUsageShapeV3(r.AccountedUsage) != nil {
		return fmt.Errorf("transport receipt timing or usage drift")
	}
	wall := end.Sub(start).Milliseconds()
	if r.TimingKnown {
		if r.ReportedUsage.WallMillis != wall {
			return fmt.Errorf("transport wall drift")
		}
	} else if r.ReportedUsage.WallMillis != 0 {
		return fmt.Errorf("unknown timing drift")
	}
	if !r.ProviderCallStarted {
		if !r.UsageKnown || r.ReportedUsage != (RoleQualificationUsageV3{WallMillis: r.ReportedUsage.WallMillis}) || r.AccountedUsage != r.ReportedUsage {
			return fmt.Errorf("pre-provider accounting drift")
		}
	} else if !r.UsageKnown {
		if r.ReportedUsage != (RoleQualificationUsageV3{WallMillis: r.ReportedUsage.WallMillis}) || r.AccountedUsage != claim.Reserved {
			return fmt.Errorf("unknown provider usage reservation drift")
		}
	} else if r.ReportedUsage.Calls != 1 || r.ReportedUsage.WallMillis != wall || r.AccountedUsage != r.ReportedUsage {
		return fmt.Errorf("known provider accounting drift")
	}
	if _, err := time.Parse(time.RFC3339Nano, r.SealedAt); err != nil {
		return fmt.Errorf("transport receipt seal time")
	}
	return nil
}

func RoleQualificationExecutionReplayV6(root string) (RoleQualificationReplayV6, error) {
	plan, err := BuildRoleQualificationExecutionPlanV3()
	if err != nil {
		return RoleQualificationReplayV6{}, err
	}
	byID := map[string]RoleQualificationExecutionCellV3{}
	for _, c := range plan.Cells {
		byID[c.CellID] = c
	}
	claims, err := readRoleQualificationClaimsV3(root)
	if err != nil {
		return RoleQualificationReplayV6{}, err
	}
	out := RoleQualificationReplayV6{Claims: len(claims)}
	eliminated := map[string]bool{}
	for id, claim := range claims {
		cell, ok := byID[id]
		if !ok || claim.FrozenRequestSHA256 != cell.RequestSHA256 || claim.Reserved != reservationForRoleQualificationCellV3(cell, plan.PerObservation) || claim.Retries != 0 || claim.Fallbacks {
			return out, fmt.Errorf("claim drift %s", id)
		}
		var attempt RoleQualificationAttemptV6
		if err := readStrictJSONFileV3(filepath.Join(root, "attempts", id+".json"), &attempt); err != nil {
			out.GlobalHalt = true
			out.GlobalHaltClass = "evidence_transport_uncertainty"
			return out, fmt.Errorf("claim without durable attempt %s", id)
		}
		claimRaw, _ := json.Marshal(claim)
		if attempt.ClaimSHA256 != sha256Hex(claimRaw) || attempt.Reserved != claim.Reserved || attempt.CellID != id || attempt.FrozenRequestSHA256 != cell.RequestSHA256 || attempt.AttemptOrdinal != 1 || attempt.Retries != 0 || attempt.Fallbacks {
			return out, fmt.Errorf("attempt drift %s", id)
		}
		if !attempt.LegacyV4Recovery {
			preflightRaw, err := os.ReadFile(filepath.Join(root, "preflight", cell.ConfigurationClass.ClassID+".json"))
			if err != nil || attempt.PreflightSHA256 != sha256Hex(preflightRaw) {
				return out, fmt.Errorf("attempt preflight drift %s", id)
			}
		}
		out.Attempts++
		rp, tp := filepath.Join(root, "receipts", id+".json"), filepath.Join(root, "transport-receipts", id+".json")
		hasSuccess, hasFailure := fileExistsV3(rp), fileExistsV3(tp)
		if hasSuccess == hasFailure {
			out.GlobalHalt = true
			out.GlobalHaltClass = "evidence_transport_uncertainty"
			return out, fmt.Errorf("terminal receipt XOR violation %s", id)
		}
		op := filepath.Join(root, "observations", id+".json")
		if !fileExistsV3(op) {
			return out, fmt.Errorf("receipt pending reconciliation %s", id)
		}
		var actual RoleQualificationObservationV6
		if err := readStrictJSONFileV3(op, &actual); err != nil {
			return out, err
		}
		var expected RoleQualificationObservationV6
		attemptRaw, _ := json.Marshal(attempt)
		if hasFailure {
			var r RoleQualificationTransportReceiptV6
			if err := readStrictJSONFileV3(tp, &r); err != nil {
				return out, err
			}
			receiptRaw, _ := json.Marshal(r)
			if err := validateRoleQualificationTransportReceiptV6(cell, claim, attempt, r); err != nil {
				return out, fmt.Errorf("%w %s", err, id)
			}
			status, class := "invalid_unscored", "observation_transport_"+string(r.Cause)
			if r.FailureScope == "global_halt" {
				status = "failed"
				out.GlobalHalt = true
				out.GlobalHaltClass = class
			}
			base := RoleQualificationObservationV3{SchemaVersion: observationSchemaV6, CellID: id, FrozenRequestSHA256: cell.RequestSHA256, RawReceiptSHA256: sha256Hex(receiptRaw), Status: status, FailureScope: r.FailureScope, FailureClass: class, Usage: r.AccountedUsage}
			expected = RoleQualificationObservationV6{base, r.AttemptSHA256, sha256Hex(receiptRaw), r.UsageKnown, r.ProviderCallStarted}
			out.TransportReceipts++
			if r.FailureScope == "observation_local_unscored" {
				out.LocalUnscored = append(out.LocalUnscored, id)
			}
		} else {
			var r RoleQualificationProviderReceiptV3
			if err := readStrictJSONFileV3(rp, &r); err != nil {
				return out, err
			}
			base, err := deriveRoleQualificationObservationV3(cell, r, plan.PerObservation)
			if err != nil {
				return out, err
			}
			if cell.ConfigurationClass.RuntimeProvider != "claude_code_subscription" {
				var evidence RoleQualificationOpenCodeEvidenceV6
				if err := readStrictJSONFileV3(filepath.Join(root, "opencode-http-receipts", id+".json"), &evidence); err != nil {
					return out, fmt.Errorf("v6 OpenCode source evidence missing %s: %w", id, err)
				}
				derivedRaw, _, err := validateRoleQualificationOpenCodeEvidenceV6(cell, attempt, evidence)
				if err != nil || !bytes.Equal(derivedRaw, r.RawResponse) || r.RawResponseSHA256 != sha256Hex(derivedRaw) {
					return out, fmt.Errorf("v6 OpenCode replay derivation drift %s", id)
				}
			}
			base.SchemaVersion = observationSchemaV6
			receiptRaw, _ := json.Marshal(r)
			expected = RoleQualificationObservationV6{base, sha256Hex(attemptRaw), sha256Hex(receiptRaw), true, true}
			out.SuccessReceipts++
			if base.FailureScope == "candidate_local" {
				eliminated[cell.ConfigurationClass.ClassID+":"+cell.CandidateClass] = true
			}
			if base.FailureScope == "global_halt" {
				out.GlobalHalt = true
				out.GlobalHaltClass = base.FailureClass
			}
		}
		a, _ := json.Marshal(actual)
		e, _ := json.Marshal(expected)
		if !bytes.Equal(a, e) {
			return out, fmt.Errorf("observation derived evidence drift %s", id)
		}
		if err := addRoleQualificationUsageV3(&out.Totals, actual.Usage); err != nil {
			return out, err
		}
		out.Observations++
	}
	if class := campaignCapBreachV3(out.Totals, plan.Campaign); class != "" {
		out.GlobalHalt = true
		out.GlobalHaltClass = class
	}
	for key := range eliminated {
		out.EliminatedBindings = append(out.EliminatedBindings, key)
	}
	sort.Strings(out.EliminatedBindings)
	sort.Strings(out.LocalUnscored)
	if err := validateRoleQualificationLedgerV6(root, out.Observations); err != nil {
		return out, err
	}
	return out, nil
}

func validateRoleQualificationLedgerV6(root string, want int) error {
	path := filepath.Join(root, "ledger.jsonl")
	if want == 0 && !fileExistsV3(path) {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	count := 0
	seen := map[string]bool{}
	for scanner.Scan() {
		var ledger RoleQualificationObservationV6
		if err := strictJSON(scanner.Bytes(), &ledger); err != nil {
			return err
		}
		if seen[ledger.CellID] {
			return fmt.Errorf("duplicate ledger")
		}
		seen[ledger.CellID] = true
		var sealed RoleQualificationObservationV6
		if err := readStrictJSONFileV3(filepath.Join(root, "observations", ledger.CellID+".json"), &sealed); err != nil {
			return err
		}
		a, _ := json.Marshal(ledger)
		b, _ := json.Marshal(sealed)
		if !bytes.Equal(a, b) {
			return fmt.Errorf("ledger mismatch")
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if count != want {
		return fmt.Errorf("ledger completeness")
	}
	return nil
}

func ensureRoleQualificationLedgerEntryV6(root string, want RoleQualificationObservationV6) error {
	path := filepath.Join(root, "ledger.jsonl")
	if fileExistsV3(path) {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			var got RoleQualificationObservationV6
			if err := strictJSON(scanner.Bytes(), &got); err != nil {
				f.Close()
				return err
			}
			if got.CellID == want.CellID {
				a, _ := json.Marshal(got)
				b, _ := json.Marshal(want)
				f.Close()
				if !bytes.Equal(a, b) {
					return fmt.Errorf("ledger mismatch")
				}
				return nil
			}
		}
		if err := scanner.Err(); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}
	raw, _ := json.Marshal(want)
	return appendRoleQualificationLedgerV3(root, raw)
}

func RecoverLegacyRoleQualificationV4StrandedV6(legacyRoot, v6Root, recoveredAt string) error {
	claimRaw, err := os.ReadFile(filepath.Join(legacyRoot, "claims", LegacyV4StrandedCellIDV6+".json"))
	if err != nil || sha256Hex(claimRaw) != LegacyV4StrandedClaimSHA256V6 {
		return fmt.Errorf("legacy v4 claim evidence mismatch")
	}
	preflightRaw, err := os.ReadFile(filepath.Join(legacyRoot, "preflight", "carryforward.nexn25pro.free.openrouter.floor.v1.json"))
	if err != nil || sha256Hex(preflightRaw) != LegacyV4StrandedPreflightSHA256V6 {
		return fmt.Errorf("legacy v4 preflight evidence mismatch")
	}
	var claim RoleQualificationClaimV3
	if err := strictJSON(claimRaw, &claim); err != nil {
		return err
	}
	plan, err := BuildRoleQualificationExecutionPlanV3()
	if err != nil {
		return err
	}
	cell, ok := findRoleQualificationCellV3(plan, LegacyV4StrandedCellIDV6)
	if !ok || claim.Reserved != reservationForRoleQualificationCellV3(cell, plan.PerObservation) {
		return fmt.Errorf("legacy v4 cell drift")
	}
	recoveryPath := filepath.Join(v6Root, "legacy-recovery", cell.CellID+".json")
	if fileExistsV3(recoveryPath) {
		var existing RoleQualificationLegacyRecoveryV6
		if err := readStrictJSONFileV3(recoveryPath, &existing); err != nil {
			return err
		}
		if existing.SchemaVersion != legacyRecoverySchemaV6 || existing.LegacyClaimSHA256 != LegacyV4StrandedClaimSHA256V6 || existing.LegacyPreflightSHA256 != LegacyV4StrandedPreflightSHA256V6 || existing.CellID != cell.CellID || existing.Disposition != "invalid_unscored_no_retry_conservative_full_reservation" {
			return fmt.Errorf("legacy recovery drift")
		}
		return ReconcileRoleQualificationClaimV6(v6Root, cell.CellID)
	}
	if recoveredAt == "" {
		recoveredAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if _, err := time.Parse(time.RFC3339Nano, recoveredAt); err != nil {
		return err
	}
	if err := writeOnceOrExactJSONV6(filepath.Join(v6Root, "claims", cell.CellID+".json"), claim); err != nil {
		return err
	}
	attempt := RoleQualificationAttemptV6{attemptSchemaV6, cell.CellID, cell.RequestSHA256, LegacyV4StrandedClaimSHA256V6, LegacyV4StrandedPreflightSHA256V6, claim.Reserved, 1, 0, false, claim.ClaimedAt, true}
	if err := writeOnceOrExactJSONV6(filepath.Join(v6Root, "attempts", cell.CellID+".json"), attempt); err != nil {
		return err
	}
	attemptRaw, _ := json.Marshal(attempt)
	detail := "legacy v4 provider activity and settlement are unknown; no retry permitted"
	receipt := RoleQualificationTransportReceiptV6{SchemaVersion: transportReceiptSchemaV6, CellID: cell.CellID, FrozenRequestSHA256: cell.RequestSHA256, AttemptSHA256: sha256Hex(attemptRaw), Stage: RoleQualificationStageProviderV6, Cause: RoleQualificationCauseInterruptedV6, ProviderCallStarted: true, UsageKnown: false, StartedAt: claim.ClaimedAt, CompletedAt: claim.ClaimedAt, ErrorDetail: detail, ErrorDetailSHA256: sha256Hex([]byte(detail)), AccountedUsage: claim.Reserved, SealedAt: recoveredAt, LegacyV4Recovery: true, FailureScope: "observation_local_unscored", TimingKnown: false}
	if err := writeOnceOrExactJSONV6(filepath.Join(v6Root, "transport-receipts", cell.CellID+".json"), receipt); err != nil {
		return err
	}
	recovery := RoleQualificationLegacyRecoveryV6{legacyRecoverySchemaV6, LegacyV4StrandedClaimSHA256V6, LegacyV4StrandedPreflightSHA256V6, cell.CellID, "invalid_unscored_no_retry_conservative_full_reservation", recoveredAt}
	if err := writeOnceOrExactJSONV6(recoveryPath, recovery); err != nil {
		return err
	}
	return ReconcileRoleQualificationClaimV6(v6Root, cell.CellID)
}

func writeOnceOrExactJSONV6(path string, value any) error {
	want, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if raw, err := os.ReadFile(path); err == nil {
		if bytes.Equal(raw, want) {
			return nil
		}
		return fmt.Errorf("immutable evidence drift %s", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	return writeOnceRoleQualificationJSONV3(path, value)
}

func hashOptionalBytesV6(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	return sha256Hex(raw)
}
func hashOptionalStringV6(value string) string {
	if value == "" {
		return ""
	}
	return sha256Hex([]byte(value))
}

func elapsedMillisV6(startText, endText string) int64 {
	start, e1 := time.Parse(time.RFC3339Nano, startText)
	end, e2 := time.Parse(time.RFC3339Nano, endText)
	if e1 != nil || e2 != nil || end.Before(start) {
		return 0
	}
	return end.Sub(start).Milliseconds()
}
func containsStringV6(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func hashV6(v any) string {
	raw, _ := json.Marshal(v)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
