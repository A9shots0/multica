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
	"time"
)

const (
	RoleQualificationExecutionSchemaV5 = "silver-mediums.role-qualification-execution.v5"
	preflightSchemaV5                  = "silver-mediums.role-qualification-preflight.v5"
	attemptSchemaV5                    = "silver-mediums.role-qualification-attempt.v5"
	transportReceiptSchemaV5           = "silver-mediums.role-qualification-transport-receipt.v5"
	observationSchemaV5                = "silver-mediums.role-qualification-observation.v5"
	legacyRecoverySchemaV5             = "silver-mediums.role-qualification-legacy-recovery.v5"
	LegacyV4StrandedCellID             = "rqv3-25-nex-product_ui-rq-pui-03"
	LegacyV4StrandedClaimSHA256        = "e5395343d22c0dd67e255d0296a16308f5341205d52ef68382082761469c418b"
	LegacyV4StrandedPreflightSHA256    = "eade462c805eddac64db83e09906326a715ebb6b2578d13ebc0e961e9ce1a73d"
)

type RoleQualificationExecutionDefinitionV5 struct {
	SchemaVersion         string `json:"schema_version"`
	ExecutionID           string `json:"execution_id"`
	Status                string `json:"status"`
	V4FreezeSHA256        string `json:"v4_freeze_sha256"`
	MaxCellsPerInvocation int    `json:"max_cells_per_invocation"`
	FreshPreflightSeconds int64  `json:"fresh_preflight_seconds"`
	Retries               int    `json:"retries"`
	Fallbacks             bool   `json:"fallbacks"`
	TransportErrorScope   string `json:"transport_error_scope"`
	ContinueUnaffected    bool   `json:"continue_unaffected"`
	ProviderGate          string `json:"provider_gate"`
}
type RoleQualificationPreflightV5 RoleQualificationPreflightV4

type RoleQualificationAttemptV5 struct {
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

type RoleQualificationTransportStageV5 string
type RoleQualificationTransportCauseV5 string

const (
	RoleQualificationStagePreProvider RoleQualificationTransportStageV5 = "pre_provider"
	RoleQualificationStageProvider    RoleQualificationTransportStageV5 = "provider_request"
	RoleQualificationStageResponse    RoleQualificationTransportStageV5 = "response_read"
	RoleQualificationStageParse       RoleQualificationTransportStageV5 = "response_parse"
	RoleQualificationCauseTimeout     RoleQualificationTransportCauseV5 = "timeout"
	RoleQualificationCauseConnection  RoleQualificationTransportCauseV5 = "connection_error"
	RoleQualificationCauseRead        RoleQualificationTransportCauseV5 = "response_read_error"
	RoleQualificationCauseRoute       RoleQualificationTransportCauseV5 = "route_drift"
	RoleQualificationCauseModel       RoleQualificationTransportCauseV5 = "model_drift"
	RoleQualificationCauseProvider    RoleQualificationTransportCauseV5 = "provider_drift"
	RoleQualificationCauseTurn        RoleQualificationTransportCauseV5 = "turn_drift"
	RoleQualificationCauseMalformed   RoleQualificationTransportCauseV5 = "malformed_evidence"
	RoleQualificationCauseCredential  RoleQualificationTransportCauseV5 = "credential_failure"
	RoleQualificationCauseAuth        RoleQualificationTransportCauseV5 = "auth_failure"
	RoleQualificationCauseRuntime     RoleQualificationTransportCauseV5 = "runtime_failure"
	RoleQualificationCauseConfig      RoleQualificationTransportCauseV5 = "config_failure"
	RoleQualificationCauseInterrupted RoleQualificationTransportCauseV5 = "interrupted"
)

type RoleQualificationTransportFailureV5 struct {
	Stage               RoleQualificationTransportStageV5 `json:"stage"`
	Cause               RoleQualificationTransportCauseV5 `json:"cause"`
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
type RoleQualificationProviderResultV5 struct {
	RawResponse      json.RawMessage                      `json:"raw_response,omitempty"`
	TransportFailure *RoleQualificationTransportFailureV5 `json:"transport_failure,omitempty"`
}
type RoleQualificationProviderV5 interface {
	Call(context.Context, RoleQualificationExecutionCellV3) RoleQualificationProviderResultV5
}

type RoleQualificationTransportReceiptV5 struct {
	SchemaVersion       string                            `json:"schema_version"`
	CellID              string                            `json:"cell_id"`
	FrozenRequestSHA256 string                            `json:"frozen_request_sha256"`
	AttemptSHA256       string                            `json:"attempt_sha256"`
	Stage               RoleQualificationTransportStageV5 `json:"stage"`
	Cause               RoleQualificationTransportCauseV5 `json:"cause"`
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
type RoleQualificationObservationV5 struct {
	RoleQualificationObservationV3
	AttemptSHA256         string `json:"attempt_sha256"`
	TerminalReceiptSHA256 string `json:"terminal_receipt_sha256"`
	UsageKnown            bool   `json:"usage_known"`
	ProviderCallStarted   bool   `json:"provider_call_started"`
}
type RoleQualificationReplayV5 struct {
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
type RoleQualificationLegacyRecoveryV5 struct {
	SchemaVersion         string `json:"schema_version"`
	LegacyClaimSHA256     string `json:"legacy_claim_sha256"`
	LegacyPreflightSHA256 string `json:"legacy_preflight_sha256"`
	CellID                string `json:"cell_id"`
	Disposition           string `json:"disposition"`
	RecoveredAt           string `json:"recovered_at"`
}

//go:embed role_qualification_execution_v5.json
var roleQualificationExecutionV5JSON []byte

func CurrentRoleQualificationExecutionDefinitionV5() (RoleQualificationExecutionDefinitionV5, error) {
	var d RoleQualificationExecutionDefinitionV5
	if err := strictJSON(roleQualificationExecutionV5JSON, &d); err != nil {
		return d, err
	}
	if d.SchemaVersion != RoleQualificationExecutionSchemaV5 || d.ExecutionID != "role-qualification-transport-local-v5" || d.Status != "frozen_preflight_required_not_executed" || d.V4FreezeSHA256 != "2c747be170367ea9eafc36551ffe344e18d7d5ff0c97b60e098ac6226368b2b0" || d.V4FreezeSHA256 != RoleQualificationExecutionFreezeV4SHA256() || d.MaxCellsPerInvocation != 12 || d.FreshPreflightSeconds != 600 || d.Retries != 0 || d.Fallbacks || d.TransportErrorScope != "observation_local_unscored" || !d.ContinueUnaffected || d.ProviderGate != "exact_v5_freeze_plus_fresh_v5_preflight" {
		return d, fmt.Errorf("v5 definition drift")
	}
	return d, nil
}

func ValidateRoleQualificationPreflightV5(p RoleQualificationPreflightV5, classID string, now time.Time) error {
	x := RoleQualificationPreflightV4(p)
	x.SchemaVersion = "silver-mediums.role-qualification-preflight.v4"
	x.ExecutionFreezeSHA256 = RoleQualificationExecutionFreezeV4SHA256()
	if err := ValidateRoleQualificationPreflightV4(x, classID, now); err != nil {
		return err
	}
	if p.SchemaVersion != preflightSchemaV5 || p.ExecutionFreezeSHA256 != RoleQualificationExecutionFreezeV5SHA256() {
		return fmt.Errorf("v5 preflight identity")
	}
	return nil
}
func WriteRoleQualificationPreflightV5(root string, p RoleQualificationPreflightV5) error {
	if err := ValidateRoleQualificationPreflightV5(p, p.ConfigurationClassID, time.Now().UTC()); err != nil {
		return err
	}
	return writeOnceRoleQualificationJSONV3(filepath.Join(root, "preflight", p.ConfigurationClassID+".json"), p)
}

func PrepareRoleQualificationGrantV5(root, cellID, now string) (RoleQualificationClaimV3, error) {
	plan, err := BuildRoleQualificationExecutionPlanV3()
	if err != nil {
		return RoleQualificationClaimV3{}, err
	}
	replay, err := RoleQualificationExecutionReplayV5(root)
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
	if containsStringV5(replay.EliminatedBindings, cell.ConfigurationClass.ClassID+":"+cell.CandidateClass) {
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

func PrepareRoleQualificationAttemptV5(root string, cell RoleQualificationExecutionCellV3, preparedAt string) (RoleQualificationAttemptV5, error) {
	claimRaw, err := os.ReadFile(filepath.Join(root, "claims", cell.CellID+".json"))
	if err != nil {
		return RoleQualificationAttemptV5{}, err
	}
	var claim RoleQualificationClaimV3
	if err := strictJSON(claimRaw, &claim); err != nil {
		return RoleQualificationAttemptV5{}, err
	}
	preflightRaw, err := os.ReadFile(filepath.Join(root, "preflight", cell.ConfigurationClass.ClassID+".json"))
	if err != nil {
		return RoleQualificationAttemptV5{}, err
	}
	if preparedAt == "" {
		preparedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	a := RoleQualificationAttemptV5{attemptSchemaV5, cell.CellID, cell.RequestSHA256, sha256Hex(claimRaw), sha256Hex(preflightRaw), claim.Reserved, 1, 0, false, preparedAt, false}
	if err := validateRoleQualificationAttemptV5(cell, claim, a); err != nil {
		return RoleQualificationAttemptV5{}, err
	}
	return a, writeOnceRoleQualificationJSONV3(filepath.Join(root, "attempts", cell.CellID+".json"), a)
}
func validateRoleQualificationAttemptV5(cell RoleQualificationExecutionCellV3, claim RoleQualificationClaimV3, a RoleQualificationAttemptV5) error {
	if a.SchemaVersion != attemptSchemaV5 || a.CellID != cell.CellID || a.FrozenRequestSHA256 != cell.RequestSHA256 || a.Reserved != claim.Reserved || a.AttemptOrdinal != 1 || a.Retries != 0 || a.Fallbacks || !validSHA256(a.ClaimSHA256) || !validSHA256(a.PreflightSHA256) {
		return fmt.Errorf("attempt drift")
	}
	if _, err := time.Parse(time.RFC3339Nano, a.PreparedAt); err != nil {
		return fmt.Errorf("attempt timestamp")
	}
	return nil
}
func validTransportStageV5(v RoleQualificationTransportStageV5) bool {
	return v == RoleQualificationStagePreProvider || v == RoleQualificationStageProvider || v == RoleQualificationStageResponse || v == RoleQualificationStageParse
}
func validTransportCauseV5(v RoleQualificationTransportCauseV5) bool {
	switch v {
	case RoleQualificationCauseTimeout, RoleQualificationCauseConnection, RoleQualificationCauseRead, RoleQualificationCauseRoute, RoleQualificationCauseModel, RoleQualificationCauseProvider, RoleQualificationCauseTurn, RoleQualificationCauseMalformed, RoleQualificationCauseCredential, RoleQualificationCauseAuth, RoleQualificationCauseRuntime, RoleQualificationCauseConfig, RoleQualificationCauseInterrupted:
		return true
	default:
		return false
	}
}

func failureScopeForCauseV5(v RoleQualificationTransportCauseV5) string {
	if v == RoleQualificationCauseTimeout || v == RoleQualificationCauseConnection || v == RoleQualificationCauseRead || v == RoleQualificationCauseInterrupted {
		return "observation_local_unscored"
	}
	return "global_halt"
}

func SealRoleQualificationTransportFailureV5(root string, cell RoleQualificationExecutionCellV3, f RoleQualificationTransportFailureV5, sealedAt string) error {
	var claim RoleQualificationClaimV3
	if err := readStrictJSONFileV3(filepath.Join(root, "claims", cell.CellID+".json"), &claim); err != nil {
		return err
	}
	var attempt RoleQualificationAttemptV5
	if err := readStrictJSONFileV3(filepath.Join(root, "attempts", cell.CellID+".json"), &attempt); err != nil {
		return err
	}
	if !validTransportStageV5(f.Stage) || !validTransportCauseV5(f.Cause) || f.ErrorDetail == "" {
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
	r := RoleQualificationTransportReceiptV5{SchemaVersion: transportReceiptSchemaV5, CellID: cell.CellID, FrozenRequestSHA256: cell.RequestSHA256, AttemptSHA256: sha256Hex(attemptRaw), Stage: f.Stage, Cause: f.Cause, ProviderCallStarted: f.ProviderCallStarted, UsageKnown: f.UsageKnown, StartedAt: f.StartedAt, CompletedAt: f.CompletedAt, ErrorDetail: f.ErrorDetail, ErrorDetailSHA256: sha256Hex([]byte(f.ErrorDetail)), ReportedUsage: f.ReportedUsage, AccountedUsage: accounted, SealedAt: sealedAt, FailureScope: failureScopeForCauseV5(f.Cause), TimingKnown: f.TimingKnown, HTTPStatus: f.HTTPStatus, ContentType: f.ContentType, PartialBody: append([]byte(nil), f.PartialBody...), PartialBodySHA256: hashOptionalBytesV5(f.PartialBody), ReadError: f.ReadError, ReadErrorSHA256: hashOptionalStringV5(f.ReadError)}
	if _, err := time.Parse(time.RFC3339Nano, sealedAt); err != nil {
		return err
	}
	return writeOnceRoleQualificationJSONV3(filepath.Join(root, "transport-receipts", cell.CellID+".json"), r)
}

func ExecuteRoleQualificationBatchV5(ctx context.Context, root, classID string, now time.Time, provider RoleQualificationProviderV5) (RoleQualificationReplayV5, error) {
	d, err := CurrentRoleQualificationExecutionDefinitionV5()
	if err != nil {
		return RoleQualificationReplayV5{}, err
	}
	if _, err := CurrentRoleQualificationExecutionFreezeV5(); err != nil {
		return RoleQualificationReplayV5{}, err
	}
	var p RoleQualificationPreflightV5
	if err := readStrictJSONFileV3(filepath.Join(root, "preflight", classID+".json"), &p); err != nil {
		return RoleQualificationReplayV5{}, err
	}
	if err := ValidateRoleQualificationPreflightV5(p, classID, now); err != nil {
		return RoleQualificationReplayV5{}, err
	}
	if err := ReconcileRoleQualificationOpenAttemptsV5(root, now.Format(time.RFC3339Nano)); err != nil {
		return RoleQualificationReplayV5{}, err
	}
	cells, err := RoleQualificationClassCellsV4(classID)
	if err != nil || len(cells) > d.MaxCellsPerInvocation {
		return RoleQualificationReplayV5{}, fmt.Errorf("v5 batch scope")
	}
	for _, cell := range cells {
		replay, err := RoleQualificationExecutionReplayV5(root)
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
		if containsStringV5(replay.EliminatedBindings, cell.ConfigurationClass.ClassID+":"+cell.CandidateClass) {
			continue
		}
		if _, err := PrepareRoleQualificationGrantV5(root, cell.CellID, now.Format(time.RFC3339Nano)); err != nil {
			return replay, err
		}
		if _, err := PrepareRoleQualificationAttemptV5(root, cell, now.Format(time.RFC3339Nano)); err != nil {
			return replay, err
		}
		result := provider.Call(ctx, cell)
		if (len(result.RawResponse) > 0) == (result.TransportFailure != nil) {
			return RoleQualificationExecutionReplayV5(root)
		}
		if result.TransportFailure != nil {
			if err := SealRoleQualificationTransportFailureV5(root, cell, *result.TransportFailure, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
				return RoleQualificationExecutionReplayV5(root)
			}
		} else {
			r := RoleQualificationProviderReceiptV3{"silver-mediums.role-qualification-provider-receipt.v3", cell.CellID, cell.RequestSHA256, append(json.RawMessage(nil), result.RawResponse...), sha256Hex(result.RawResponse), time.Now().UTC().Format(time.RFC3339Nano)}
			if err := SealRoleQualificationRawReceiptV3(root, r); err != nil {
				return RoleQualificationExecutionReplayV5(root)
			}
		}
		if err := ReconcileRoleQualificationClaimV5(root, cell.CellID); err != nil {
			return RoleQualificationExecutionReplayV5(root)
		}
	}
	return RoleQualificationExecutionReplayV5(root)
}

// ReconcileRoleQualificationOpenAttemptsV5 closes a durable attempt which has
// no terminal receipt as ambiguous local evidence. It never calls a provider.
func ReconcileRoleQualificationOpenAttemptsV5(root, reconciledAt string) error {
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
				if err := ReconcileRoleQualificationClaimV5(root, id); err != nil {
					return err
				}
			}
			continue
		}
		cell, ok := byID[id]
		if !ok {
			return fmt.Errorf("unknown attempted cell")
		}
		var attempt RoleQualificationAttemptV5
		if err := readStrictJSONFileV3(attemptPath, &attempt); err != nil {
			return err
		}
		failure := RoleQualificationTransportFailureV5{Stage: RoleQualificationStageProvider, Cause: RoleQualificationCauseInterrupted, ProviderCallStarted: true, UsageKnown: false, StartedAt: attempt.PreparedAt, CompletedAt: attempt.PreparedAt, ErrorDetail: "durable attempt found without authoritative terminal receipt; no retry permitted", TimingKnown: false}
		if err := SealRoleQualificationTransportFailureV5(root, cell, failure, reconciledAt); err != nil {
			return err
		}
		if err := ReconcileRoleQualificationClaimV5(root, id); err != nil {
			return err
		}
	}
	return nil
}

func ReconcileRoleQualificationClaimV5(root, cellID string) error {
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
		var existing RoleQualificationObservationV5
		if err := readStrictJSONFileV3(op, &existing); err != nil {
			return err
		}
		return ensureRoleQualificationLedgerEntryV5(root, existing)
	}
	rp, tp := filepath.Join(root, "receipts", cellID+".json"), filepath.Join(root, "transport-receipts", cellID+".json")
	if fileExistsV3(rp) == fileExistsV3(tp) {
		return fmt.Errorf("terminal receipt XOR violation")
	}
	var out RoleQualificationObservationV5
	if fileExistsV3(tp) {
		var r RoleQualificationTransportReceiptV5
		if err := readStrictJSONFileV3(tp, &r); err != nil {
			return err
		}
		var a RoleQualificationAttemptV5
		if err := readStrictJSONFileV3(filepath.Join(root, "attempts", cellID+".json"), &a); err != nil {
			return err
		}
		receiptRaw, _ := json.Marshal(r)
		var claim RoleQualificationClaimV3
		if err := readStrictJSONFileV3(filepath.Join(root, "claims", cellID+".json"), &claim); err != nil {
			return err
		}
		if err := validateRoleQualificationTransportReceiptV5(cell, claim, a, r); err != nil {
			return fmt.Errorf("transport receipt drift")
		}
		status, class := "invalid_unscored", "observation_transport_"+string(r.Cause)
		if r.FailureScope == "global_halt" {
			status = "failed"
		}
		base := RoleQualificationObservationV3{SchemaVersion: observationSchemaV5, CellID: cellID, FrozenRequestSHA256: cell.RequestSHA256, RawReceiptSHA256: sha256Hex(receiptRaw), Status: status, FailureScope: r.FailureScope, FailureClass: class, Usage: r.AccountedUsage}
		out = RoleQualificationObservationV5{base, r.AttemptSHA256, sha256Hex(receiptRaw), r.UsageKnown, r.ProviderCallStarted}
	} else {
		var r RoleQualificationProviderReceiptV3
		if err := readStrictJSONFileV3(rp, &r); err != nil {
			return err
		}
		base, err := deriveRoleQualificationObservationV3(cell, r, plan.PerObservation)
		if err != nil {
			return err
		}
		base.SchemaVersion = observationSchemaV5
		var a RoleQualificationAttemptV5
		if err := readStrictJSONFileV3(filepath.Join(root, "attempts", cellID+".json"), &a); err != nil {
			return err
		}
		attemptRaw, _ := json.Marshal(a)
		receiptRaw, _ := json.Marshal(r)
		out = RoleQualificationObservationV5{base, sha256Hex(attemptRaw), sha256Hex(receiptRaw), true, true}
	}
	if err := writeOnceRoleQualificationJSONV3(op, out); err != nil {
		return err
	}
	raw, _ := json.Marshal(out)
	return appendRoleQualificationLedgerV3(root, raw)
}

func validateRoleQualificationTransportReceiptV5(cell RoleQualificationExecutionCellV3, claim RoleQualificationClaimV3, attempt RoleQualificationAttemptV5, r RoleQualificationTransportReceiptV5) error {
	attemptRaw, _ := json.Marshal(attempt)
	if r.SchemaVersion != transportReceiptSchemaV5 || r.CellID != cell.CellID || r.FrozenRequestSHA256 != cell.RequestSHA256 || r.AttemptSHA256 != sha256Hex(attemptRaw) || !validTransportStageV5(r.Stage) || !validTransportCauseV5(r.Cause) || r.ErrorDetail == "" || r.ErrorDetailSHA256 != sha256Hex([]byte(r.ErrorDetail)) || r.LegacyV4Recovery != attempt.LegacyV4Recovery || r.FailureScope != failureScopeForCauseV5(r.Cause) || r.PartialBodySHA256 != hashOptionalBytesV5(r.PartialBody) || r.ReadErrorSHA256 != hashOptionalStringV5(r.ReadError) {
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

func RoleQualificationExecutionReplayV5(root string) (RoleQualificationReplayV5, error) {
	plan, err := BuildRoleQualificationExecutionPlanV3()
	if err != nil {
		return RoleQualificationReplayV5{}, err
	}
	byID := map[string]RoleQualificationExecutionCellV3{}
	for _, c := range plan.Cells {
		byID[c.CellID] = c
	}
	claims, err := readRoleQualificationClaimsV3(root)
	if err != nil {
		return RoleQualificationReplayV5{}, err
	}
	out := RoleQualificationReplayV5{Claims: len(claims)}
	eliminated := map[string]bool{}
	for id, claim := range claims {
		cell, ok := byID[id]
		if !ok || claim.FrozenRequestSHA256 != cell.RequestSHA256 || claim.Reserved != reservationForRoleQualificationCellV3(cell, plan.PerObservation) || claim.Retries != 0 || claim.Fallbacks {
			return out, fmt.Errorf("claim drift %s", id)
		}
		var attempt RoleQualificationAttemptV5
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
		var actual RoleQualificationObservationV5
		if err := readStrictJSONFileV3(op, &actual); err != nil {
			return out, err
		}
		var expected RoleQualificationObservationV5
		attemptRaw, _ := json.Marshal(attempt)
		if hasFailure {
			var r RoleQualificationTransportReceiptV5
			if err := readStrictJSONFileV3(tp, &r); err != nil {
				return out, err
			}
			receiptRaw, _ := json.Marshal(r)
			if err := validateRoleQualificationTransportReceiptV5(cell, claim, attempt, r); err != nil {
				return out, fmt.Errorf("%w %s", err, id)
			}
			status, class := "invalid_unscored", "observation_transport_"+string(r.Cause)
			if r.FailureScope == "global_halt" {
				status = "failed"
				out.GlobalHalt = true
				out.GlobalHaltClass = class
			}
			base := RoleQualificationObservationV3{SchemaVersion: observationSchemaV5, CellID: id, FrozenRequestSHA256: cell.RequestSHA256, RawReceiptSHA256: sha256Hex(receiptRaw), Status: status, FailureScope: r.FailureScope, FailureClass: class, Usage: r.AccountedUsage}
			expected = RoleQualificationObservationV5{base, r.AttemptSHA256, sha256Hex(receiptRaw), r.UsageKnown, r.ProviderCallStarted}
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
			base.SchemaVersion = observationSchemaV5
			receiptRaw, _ := json.Marshal(r)
			expected = RoleQualificationObservationV5{base, sha256Hex(attemptRaw), sha256Hex(receiptRaw), true, true}
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
	if err := validateRoleQualificationLedgerV5(root, out.Observations); err != nil {
		return out, err
	}
	return out, nil
}

func validateRoleQualificationLedgerV5(root string, want int) error {
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
		var ledger RoleQualificationObservationV5
		if err := strictJSON(scanner.Bytes(), &ledger); err != nil {
			return err
		}
		if seen[ledger.CellID] {
			return fmt.Errorf("duplicate ledger")
		}
		seen[ledger.CellID] = true
		var sealed RoleQualificationObservationV5
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

func ensureRoleQualificationLedgerEntryV5(root string, want RoleQualificationObservationV5) error {
	path := filepath.Join(root, "ledger.jsonl")
	if fileExistsV3(path) {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			var got RoleQualificationObservationV5
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

func RecoverLegacyRoleQualificationV4StrandedV5(legacyRoot, v5Root, recoveredAt string) error {
	claimRaw, err := os.ReadFile(filepath.Join(legacyRoot, "claims", LegacyV4StrandedCellID+".json"))
	if err != nil || sha256Hex(claimRaw) != LegacyV4StrandedClaimSHA256 {
		return fmt.Errorf("legacy v4 claim evidence mismatch")
	}
	preflightRaw, err := os.ReadFile(filepath.Join(legacyRoot, "preflight", "carryforward.nexn25pro.free.openrouter.floor.v1.json"))
	if err != nil || sha256Hex(preflightRaw) != LegacyV4StrandedPreflightSHA256 {
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
	cell, ok := findRoleQualificationCellV3(plan, LegacyV4StrandedCellID)
	if !ok || claim.Reserved != reservationForRoleQualificationCellV3(cell, plan.PerObservation) {
		return fmt.Errorf("legacy v4 cell drift")
	}
	recoveryPath := filepath.Join(v5Root, "legacy-recovery", cell.CellID+".json")
	if fileExistsV3(recoveryPath) {
		var existing RoleQualificationLegacyRecoveryV5
		if err := readStrictJSONFileV3(recoveryPath, &existing); err != nil {
			return err
		}
		if existing.SchemaVersion != legacyRecoverySchemaV5 || existing.LegacyClaimSHA256 != LegacyV4StrandedClaimSHA256 || existing.LegacyPreflightSHA256 != LegacyV4StrandedPreflightSHA256 || existing.CellID != cell.CellID || existing.Disposition != "invalid_unscored_no_retry_conservative_full_reservation" {
			return fmt.Errorf("legacy recovery drift")
		}
		return ReconcileRoleQualificationClaimV5(v5Root, cell.CellID)
	}
	if recoveredAt == "" {
		recoveredAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if _, err := time.Parse(time.RFC3339Nano, recoveredAt); err != nil {
		return err
	}
	if err := writeOnceOrExactJSONV5(filepath.Join(v5Root, "claims", cell.CellID+".json"), claim); err != nil {
		return err
	}
	attempt := RoleQualificationAttemptV5{attemptSchemaV5, cell.CellID, cell.RequestSHA256, LegacyV4StrandedClaimSHA256, LegacyV4StrandedPreflightSHA256, claim.Reserved, 1, 0, false, claim.ClaimedAt, true}
	if err := writeOnceOrExactJSONV5(filepath.Join(v5Root, "attempts", cell.CellID+".json"), attempt); err != nil {
		return err
	}
	attemptRaw, _ := json.Marshal(attempt)
	detail := "legacy v4 provider activity and settlement are unknown; no retry permitted"
	receipt := RoleQualificationTransportReceiptV5{SchemaVersion: transportReceiptSchemaV5, CellID: cell.CellID, FrozenRequestSHA256: cell.RequestSHA256, AttemptSHA256: sha256Hex(attemptRaw), Stage: RoleQualificationStageProvider, Cause: RoleQualificationCauseInterrupted, ProviderCallStarted: true, UsageKnown: false, StartedAt: claim.ClaimedAt, CompletedAt: claim.ClaimedAt, ErrorDetail: detail, ErrorDetailSHA256: sha256Hex([]byte(detail)), AccountedUsage: claim.Reserved, SealedAt: recoveredAt, LegacyV4Recovery: true, FailureScope: "observation_local_unscored", TimingKnown: false}
	if err := writeOnceOrExactJSONV5(filepath.Join(v5Root, "transport-receipts", cell.CellID+".json"), receipt); err != nil {
		return err
	}
	recovery := RoleQualificationLegacyRecoveryV5{legacyRecoverySchemaV5, LegacyV4StrandedClaimSHA256, LegacyV4StrandedPreflightSHA256, cell.CellID, "invalid_unscored_no_retry_conservative_full_reservation", recoveredAt}
	if err := writeOnceOrExactJSONV5(recoveryPath, recovery); err != nil {
		return err
	}
	return ReconcileRoleQualificationClaimV5(v5Root, cell.CellID)
}

func writeOnceOrExactJSONV5(path string, value any) error {
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

func hashOptionalBytesV5(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	return sha256Hex(raw)
}
func hashOptionalStringV5(value string) string {
	if value == "" {
		return ""
	}
	return sha256Hex([]byte(value))
}

func elapsedMillisV5(startText, endText string) int64 {
	start, e1 := time.Parse(time.RFC3339Nano, startText)
	end, e2 := time.Parse(time.RFC3339Nano, endText)
	if e1 != nil || e2 != nil || end.Before(start) {
		return 0
	}
	return end.Sub(start).Milliseconds()
}
func containsStringV5(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func hashV5(v any) string {
	raw, _ := json.Marshal(v)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
