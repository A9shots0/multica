package agentqualification

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	RoleQualificationOpenCodeTransportV3 = "opencode_structured_v33_http_loopback"
	RoleQualificationClaudeTransportV3   = "claude_code_subscription_tool_free"
)

type RoleQualificationUsageV3 struct {
	Calls                                  int64 `json:"calls"`
	InputTokens                            int64 `json:"input_tokens"`
	OutputTokens                           int64 `json:"output_tokens"`
	ReasoningTokens                        int64 `json:"reasoning_tokens"`
	TotalTokens                            int64 `json:"total_tokens"`
	OpenRouterCashMicros                   int64 `json:"openrouter_cash_micros"`
	ClaudeSubscriptionCostEquivalentMicros int64 `json:"claude_subscription_cost_equivalent_micros"`
	WallMillis                             int64 `json:"wall_millis"`
}

type RoleQualificationClaimV3 struct {
	SchemaVersion       string                   `json:"schema_version"`
	CellID              string                   `json:"cell_id"`
	FrozenRequestSHA256 string                   `json:"frozen_request_sha256"`
	ClaimedAt           string                   `json:"claimed_at"`
	Reserved            RoleQualificationUsageV3 `json:"reserved"`
	Retries             int                      `json:"retries"`
	Fallbacks           bool                     `json:"fallbacks"`
}

// RoleQualificationRawProviderResponseV3 is the immutable provider envelope.
// Every receipt and observation is derived from these bytes, never caller-supplied duplicates.
type RoleQualificationRawProviderResponseV3 struct {
	SchemaVersion    string                   `json:"schema_version"`
	ProviderCallID   string                   `json:"provider_call_id"`
	ActualModel      string                   `json:"actual_model"`
	ActualProvider   string                   `json:"actual_provider"`
	ActualEffort     string                   `json:"actual_effort"`
	ActualTransport  string                   `json:"actual_transport"`
	FallbackUsed     bool                     `json:"fallback_used"`
	RetryCount       int                      `json:"retry_count"`
	StartedAt        string                   `json:"started_at"`
	CompletedAt      string                   `json:"completed_at"`
	StructuredOutput json.RawMessage          `json:"structured_output,omitempty"`
	FailureClass     string                   `json:"failure_class,omitempty"`
	Usage            RoleQualificationUsageV3 `json:"usage"`
}

type RoleQualificationProviderReceiptV3 struct {
	SchemaVersion       string          `json:"schema_version"`
	CellID              string          `json:"cell_id"`
	FrozenRequestSHA256 string          `json:"frozen_request_sha256"`
	RawResponse         json.RawMessage `json:"raw_response"`
	RawResponseSHA256   string          `json:"raw_response_sha256"`
	SealedAt            string          `json:"sealed_at"`
}

type RoleQualificationObservationV3 struct {
	SchemaVersion              string                   `json:"schema_version"`
	CellID                     string                   `json:"cell_id"`
	FrozenRequestSHA256        string                   `json:"frozen_request_sha256"`
	RawReceiptSHA256           string                   `json:"raw_receipt_sha256"`
	RawResponseSHA256          string                   `json:"raw_response_sha256"`
	CanonicalTypedOutputSHA256 string                   `json:"canonical_typed_output_sha256,omitempty"`
	Status                     string                   `json:"status"`
	FailureScope               string                   `json:"failure_scope,omitempty"`
	FailureClass               string                   `json:"failure_class,omitempty"`
	Grade                      RoleQualificationGradeV1 `json:"grade"`
	ActualModel                string                   `json:"actual_model"`
	ActualProvider             string                   `json:"actual_provider"`
	ActualEffort               string                   `json:"actual_effort"`
	ActualTransport            string                   `json:"actual_transport"`
	ProviderCallID             string                   `json:"provider_call_id"`
	Usage                      RoleQualificationUsageV3 `json:"usage"`
}

type RoleQualificationReplayV3 struct {
	Claims                         int                      `json:"claims"`
	Receipts                       int                      `json:"receipts"`
	Observations                   int                      `json:"observations"`
	StrandedNoAuthoritativeReceipt []string                 `json:"stranded_no_authoritative_receipt"`
	Totals                         RoleQualificationUsageV3 `json:"totals"`
	ReservedOpenClaims             RoleQualificationUsageV3 `json:"reserved_open_claims"`
	GlobalHalt                     bool                     `json:"global_halt"`
	GlobalHaltClass                string                   `json:"global_halt_class,omitempty"`
	EliminatedBindings             []string                 `json:"eliminated_configuration_role_bindings,omitempty"`
}

func reservationForRoleQualificationCellV3(cell RoleQualificationExecutionCellV3, caps RoleQualificationObservationCapsV2) RoleQualificationUsageV3 {
	u := RoleQualificationUsageV3{Calls: 1, InputTokens: caps.MaxInputTokens, OutputTokens: caps.MaxOutputTokens, TotalTokens: caps.MaxTotalTokens, WallMillis: caps.MaxWallSeconds * 1000}
	// Full-token price bound: $3/M input plus $15/M generated output. The
	// subscription lane records the same conservative value as cost-equivalent,
	// never as marginal cash. The named Nex route is frozen free.
	cost := int64(120000)
	if cell.ConfigurationClass.RuntimeProvider == "claude_code_subscription" {
		u.ClaudeSubscriptionCostEquivalentMicros = cost
	} else if !strings.HasSuffix(cell.ConfigurationClass.Model, ":free") {
		u.OpenRouterCashMicros = cost
	}
	return u
}

func PrepareRoleQualificationGrantV3(root, cellID, now string) (RoleQualificationClaimV3, error) {
	plan, err := BuildRoleQualificationExecutionPlanV3()
	if err != nil {
		return RoleQualificationClaimV3{}, err
	}
	replay, err := RoleQualificationExecutionReplayV3(root)
	if err != nil {
		return RoleQualificationClaimV3{}, err
	}
	if replay.GlobalHalt {
		return RoleQualificationClaimV3{}, fmt.Errorf("global halt %s forbids grant", replay.GlobalHaltClass)
	}
	if len(replay.StrandedNoAuthoritativeReceipt) > 0 {
		return RoleQualificationClaimV3{}, fmt.Errorf("global evidence_transport_uncertainty: stranded claim")
	}
	cell, ok := findRoleQualificationCellV3(plan, cellID)
	if !ok {
		return RoleQualificationClaimV3{}, fmt.Errorf("unknown cell %s", cellID)
	}
	for _, pair := range replay.EliminatedBindings {
		if pair == cell.ConfigurationClass.ClassID+":"+cell.CandidateClass {
			return RoleQualificationClaimV3{}, fmt.Errorf("candidate-local elimination")
		}
	}
	reservation := reservationForRoleQualificationCellV3(cell, plan.PerObservation)
	projected := replay.Totals
	if err := addRoleQualificationUsageV3(&projected, replay.ReservedOpenClaims); err != nil {
		return RoleQualificationClaimV3{}, err
	}
	if err := addRoleQualificationUsageV3(&projected, reservation); err != nil {
		return RoleQualificationClaimV3{}, err
	}
	if err := validateRoleQualificationCampaignUsageV3(projected, plan.Campaign); err != nil {
		return RoleQualificationClaimV3{}, fmt.Errorf("global campaign reservation breach: %w", err)
	}
	if now == "" {
		now = time.Now().UTC().Format(time.RFC3339Nano)
	}
	claim := RoleQualificationClaimV3{"silver-mediums.role-qualification-claim.v3", cell.CellID, cell.RequestSHA256, now, reservation, 0, false}
	if err := writeOnceRoleQualificationJSONV3(filepath.Join(root, "claims", cellID+".json"), claim); err != nil {
		return RoleQualificationClaimV3{}, err
	}
	return claim, nil
}

func SealRoleQualificationRawReceiptV3(root string, receipt RoleQualificationProviderReceiptV3) error {
	plan, err := BuildRoleQualificationExecutionPlanV3()
	if err != nil {
		return err
	}
	cell, ok := findRoleQualificationCellV3(plan, receipt.CellID)
	if !ok {
		return fmt.Errorf("unknown cell")
	}
	var claim RoleQualificationClaimV3
	if err := readStrictJSONFileV3(filepath.Join(root, "claims", cell.CellID+".json"), &claim); err != nil {
		return fmt.Errorf("write-ahead claim required: %w", err)
	}
	if claim.FrozenRequestSHA256 != cell.RequestSHA256 || receipt.FrozenRequestSHA256 != cell.RequestSHA256 {
		return fmt.Errorf("frozen request substitution")
	}
	if _, err := validateRoleQualificationRawReceiptV3(cell, receipt); err != nil {
		return err
	}
	if _, err := time.Parse(time.RFC3339Nano, receipt.SealedAt); err != nil {
		return fmt.Errorf("sealed_at invalid")
	}
	// Cap evaluation deliberately happens after this immutable raw seal.
	return writeOnceRoleQualificationJSONV3(filepath.Join(root, "receipts", cell.CellID+".json"), receipt)
}

func validateRoleQualificationRawReceiptV3(cell RoleQualificationExecutionCellV3, receipt RoleQualificationProviderReceiptV3) (RoleQualificationRawProviderResponseV3, error) {
	if receipt.SchemaVersion != "silver-mediums.role-qualification-provider-receipt.v3" || receipt.CellID != cell.CellID || receipt.FrozenRequestSHA256 != cell.RequestSHA256 || len(receipt.RawResponse) == 0 || !json.Valid(receipt.RawResponse) || sha256Hex(receipt.RawResponse) != receipt.RawResponseSHA256 {
		return RoleQualificationRawProviderResponseV3{}, fmt.Errorf("raw response or frozen request evidence invalid")
	}
	return parseAndValidateRawProviderResponseV3(cell, receipt.RawResponse)
}

func parseAndValidateRawProviderResponseV3(cell RoleQualificationExecutionCellV3, raw []byte) (RoleQualificationRawProviderResponseV3, error) {
	var p RoleQualificationRawProviderResponseV3
	if err := strictJSON(raw, &p); err != nil {
		return p, err
	}
	if p.SchemaVersion != "silver-mediums.role-qualification-raw-provider-response.v3" || p.ProviderCallID == "" || p.FallbackUsed || p.RetryCount != 0 {
		return p, fmt.Errorf("raw provider identity/retry/fallback drift")
	}
	c := cell.ConfigurationClass
	if p.ActualModel != c.Model || p.ActualProvider != c.ServingProvider || p.ActualEffort != c.Effort || p.ActualTransport != transportForRoleQualificationCellV3(cell) {
		return p, fmt.Errorf("actual provider route drift")
	}
	start, e1 := time.Parse(time.RFC3339Nano, p.StartedAt)
	end, e2 := time.Parse(time.RFC3339Nano, p.CompletedAt)
	if e1 != nil || e2 != nil || end.Before(start) {
		return p, fmt.Errorf("provider timing invalid")
	}
	if err := validateRoleQualificationUsageShapeV3(p.Usage); err != nil {
		return p, err
	}
	if p.Usage.Calls != 1 {
		return p, fmt.Errorf("provider call accounting")
	}
	if end.Sub(start).Milliseconds() != p.Usage.WallMillis {
		return p, fmt.Errorf("provider wall accounting mismatch")
	}
	if c.RuntimeProvider == "claude_code_subscription" {
		if p.Usage.OpenRouterCashMicros != 0 {
			return p, fmt.Errorf("subscription cost conflated with marginal cash")
		}
	} else if p.Usage.ClaudeSubscriptionCostEquivalentMicros != 0 {
		return p, fmt.Errorf("OpenRouter cost conflated with subscription equivalent")
	}
	if p.FailureClass == "" {
		if len(p.StructuredOutput) == 0 || !json.Valid(p.StructuredOutput) {
			return p, fmt.Errorf("structured output absent")
		}
	} else if len(p.StructuredOutput) > 0 {
		return p, fmt.Errorf("failure cannot carry output")
	}
	return p, nil
}

func ReconcileRoleQualificationClaimV3(root, cellID string) (string, error) {
	plan, err := BuildRoleQualificationExecutionPlanV3()
	if err != nil {
		return "", err
	}
	cell, ok := findRoleQualificationCellV3(plan, cellID)
	if !ok {
		return "", fmt.Errorf("unknown cell")
	}
	if !fileExistsV3(filepath.Join(root, "claims", cellID+".json")) {
		return "", fmt.Errorf("claim missing")
	}
	observationPath := filepath.Join(root, "observations", cellID+".json")
	if fileExistsV3(observationPath) {
		var o RoleQualificationObservationV3
		if err := readStrictJSONFileV3(observationPath, &o); err != nil {
			return "", err
		}
		if err := ensureRoleQualificationLedgerEntryV3(root, o); err != nil {
			return "", err
		}
		return "already_closed_no_retry", nil
	}
	receiptPath := filepath.Join(root, "receipts", cellID+".json")
	if !fileExistsV3(receiptPath) {
		return "stranded_no_authoritative_receipt_global_halt", nil
	}
	var r RoleQualificationProviderReceiptV3
	if err := readStrictJSONFileV3(receiptPath, &r); err != nil {
		return "", err
	}
	o, err := deriveRoleQualificationObservationV3(cell, r, plan.PerObservation)
	if err != nil {
		return "", err
	}
	if err := writeOnceRoleQualificationJSONV3(observationPath, o); err != nil {
		return "", err
	}
	raw, _ := json.Marshal(o)
	if err := appendRoleQualificationLedgerV3(root, raw); err != nil {
		return "", err
	}
	return o.Status, nil
}

func deriveRoleQualificationObservationV3(cell RoleQualificationExecutionCellV3, r RoleQualificationProviderReceiptV3, caps RoleQualificationObservationCapsV2) (RoleQualificationObservationV3, error) {
	p, err := validateRoleQualificationRawReceiptV3(cell, r)
	if err != nil {
		return RoleQualificationObservationV3{}, err
	}
	rawReceipt, _ := json.Marshal(r)
	o := RoleQualificationObservationV3{SchemaVersion: "silver-mediums.role-qualification-observation.v3", CellID: cell.CellID, FrozenRequestSHA256: cell.RequestSHA256, RawReceiptSHA256: sha256Hex(rawReceipt), RawResponseSHA256: r.RawResponseSHA256, ActualModel: p.ActualModel, ActualProvider: p.ActualProvider, ActualEffort: p.ActualEffort, ActualTransport: p.ActualTransport, ProviderCallID: p.ProviderCallID, Usage: p.Usage}
	if class := observationCapBreachV3(p.Usage, caps); class != "" {
		o.Status = "failed"
		o.FailureScope = "global_halt"
		o.FailureClass = class
		return o, nil
	}
	reserved := reservationForRoleQualificationCellV3(cell, caps)
	if p.Usage.OpenRouterCashMicros > reserved.OpenRouterCashMicros || p.Usage.ClaudeSubscriptionCostEquivalentMicros > reserved.ClaudeSubscriptionCostEquivalentMicros {
		o.Status = "failed"
		o.FailureScope = "global_halt"
		o.FailureClass = "observation_cost_reservation_breach"
		return o, nil
	}
	if p.FailureClass != "" {
		scope, ok := classifyRoleQualificationFailureV3(p.FailureClass)
		if !ok {
			return o, fmt.Errorf("unknown failure class")
		}
		o.Status = "failed"
		o.FailureScope = scope
		o.FailureClass = p.FailureClass
		return o, nil
	}
	values, canonical, err := ParseRoleQualificationTypedOutputV3(cell.Request.CandidatePacket.ResponseSchema, p.StructuredOutput)
	if err != nil {
		o.Status = "failed"
		o.FailureScope = "candidate_local"
		o.FailureClass = "response_or_tool_protocol_failure"
		return o, nil
	}
	o.CanonicalTypedOutputSHA256 = sha256Hex(canonical)
	o.Grade, err = GradeRoleQualificationTypedOutputV3(cell.CaseID, values)
	if err != nil {
		return o, err
	}
	if o.Grade.Pass {
		o.Status = "passed"
	} else {
		o.Status = "failed"
		o.FailureScope = "candidate_local"
		o.FailureClass = "competence_failure"
	}
	return o, nil
}

func RoleQualificationExecutionReplayV3(root string) (RoleQualificationReplayV3, error) {
	plan, err := BuildRoleQualificationExecutionPlanV3()
	if err != nil {
		return RoleQualificationReplayV3{}, err
	}
	byID := map[string]RoleQualificationExecutionCellV3{}
	for _, c := range plan.Cells {
		byID[c.CellID] = c
	}
	claims, err := readRoleQualificationClaimsV3(root)
	if err != nil {
		return RoleQualificationReplayV3{}, err
	}
	out := RoleQualificationReplayV3{Claims: len(claims)}
	eliminated := map[string]bool{}
	for id, claim := range claims {
		cell, ok := byID[id]
		if !ok || claim.FrozenRequestSHA256 != cell.RequestSHA256 || claim.Reserved != reservationForRoleQualificationCellV3(cell, plan.PerObservation) || claim.Retries != 0 || claim.Fallbacks {
			return out, fmt.Errorf("claim drift %s", id)
		}
		rp := filepath.Join(root, "receipts", id+".json")
		op := filepath.Join(root, "observations", id+".json")
		hasR, hasO := fileExistsV3(rp), fileExistsV3(op)
		if !hasR {
			if hasO {
				return out, fmt.Errorf("observation without receipt")
			}
			out.StrandedNoAuthoritativeReceipt = append(out.StrandedNoAuthoritativeReceipt, id)
			if err := addRoleQualificationUsageV3(&out.ReservedOpenClaims, claim.Reserved); err != nil {
				return out, err
			}
			continue
		}
		out.Receipts++
		var r RoleQualificationProviderReceiptV3
		if err := readStrictJSONFileV3(rp, &r); err != nil {
			return out, err
		}
		expected, err := deriveRoleQualificationObservationV3(cell, r, plan.PerObservation)
		if err != nil {
			return out, err
		}
		if !hasO {
			return out, fmt.Errorf("receipt pending reconciliation %s", id)
		}
		var actual RoleQualificationObservationV3
		if err := readStrictJSONFileV3(op, &actual); err != nil {
			return out, err
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
		if actual.FailureScope == "global_halt" {
			out.GlobalHalt = true
			out.GlobalHaltClass = actual.FailureClass
		}
		if actual.FailureScope == "candidate_local" {
			key := cell.ConfigurationClass.ClassID + ":" + cell.CandidateClass
			eliminated[key] = true
		}
	}
	if len(out.StrandedNoAuthoritativeReceipt) > 0 {
		out.GlobalHalt = true
		out.GlobalHaltClass = "evidence_transport_uncertainty"
	}
	if class := campaignCapBreachV3(out.Totals, plan.Campaign); class != "" {
		out.GlobalHalt = true
		out.GlobalHaltClass = class
	}
	for k := range eliminated {
		out.EliminatedBindings = append(out.EliminatedBindings, k)
	}
	sort.Strings(out.EliminatedBindings)
	sort.Strings(out.StrandedNoAuthoritativeReceipt)
	if err := validateRoleQualificationLedgerV3(root, out.Observations); err != nil {
		return out, err
	}
	return out, nil
}

func observationCapBreachV3(u RoleQualificationUsageV3, c RoleQualificationObservationCapsV2) string {
	if u.InputTokens > c.MaxInputTokens || u.OutputTokens > c.MaxOutputTokens || u.TotalTokens > c.MaxTotalTokens {
		return "observation_token_cap_breach"
	}
	if u.WallMillis > c.MaxWallSeconds*1000 {
		return "observation_wall_cap_breach"
	}
	return ""
}
func campaignCapBreachV3(u RoleQualificationUsageV3, c RoleQualificationCampaignCapsV3) string {
	if u.Calls > int64(c.MaxCalls) || u.InputTokens > c.MaxInputTokens || u.OutputTokens > c.MaxOutputTokens || u.TotalTokens > c.MaxTotalTokens {
		return "campaign_token_or_call_cap_breach"
	}
	if u.WallMillis > c.MaxWallSeconds*1000 {
		return "campaign_wall_cap_breach"
	}
	if u.OpenRouterCashMicros > c.MaxOpenRouterCashMicros || u.ClaudeSubscriptionCostEquivalentMicros > c.MaxClaudeCostEquivalentMicros {
		return "campaign_spend_cap_breach"
	}
	return ""
}
func validateRoleQualificationCampaignUsageV3(u RoleQualificationUsageV3, c RoleQualificationCampaignCapsV3) error {
	if x := campaignCapBreachV3(u, c); x != "" {
		return errors.New(x)
	}
	return nil
}
func classifyRoleQualificationFailureV3(class string) (string, bool) {
	candidate := map[string]bool{"competence_failure": true, "response_or_tool_protocol_failure": true, "observation_resource_exhaustion": true}
	local := map[string]bool{"isolated_fixture_imperfection": true, "isolated_grader_imperfection": true}
	global := map[string]bool{"shared_test_invalidity": true, "evidence_integrity_failure": true, "credential_failure": true, "unsafe_external_action": true, "runtime_configuration_failure": true, "credits_failure": true, "transport_failure": true, "campaign_token_or_call_cap_breach": true, "campaign_spend_cap_breach": true, "campaign_wall_cap_breach": true, "observation_token_cap_breach": true, "observation_wall_cap_breach": true, "observation_cost_reservation_breach": true, "evidence_transport_uncertainty": true}
	if candidate[class] {
		return "candidate_local", true
	}
	if local[class] {
		return "observation_local_unscored", true
	}
	if global[class] {
		return "global_halt", true
	}
	return "", false
}
func validateRoleQualificationUsageShapeV3(u RoleQualificationUsageV3) error {
	values := []int64{u.Calls, u.InputTokens, u.OutputTokens, u.ReasoningTokens, u.TotalTokens, u.OpenRouterCashMicros, u.ClaudeSubscriptionCostEquivalentMicros, u.WallMillis}
	for _, v := range values {
		if v < 0 {
			return fmt.Errorf("negative usage")
		}
	}
	if u.TotalTokens != u.InputTokens+u.OutputTokens+u.ReasoningTokens {
		return fmt.Errorf("token reconciliation")
	}
	if u.OpenRouterCashMicros > 0 && u.ClaudeSubscriptionCostEquivalentMicros > 0 {
		return fmt.Errorf("cost lane conflation")
	}
	return nil
}
func addRoleQualificationUsageV3(dst *RoleQualificationUsageV3, n RoleQualificationUsageV3) error {
	d := []*int64{&dst.Calls, &dst.InputTokens, &dst.OutputTokens, &dst.ReasoningTokens, &dst.TotalTokens, &dst.OpenRouterCashMicros, &dst.ClaudeSubscriptionCostEquivalentMicros, &dst.WallMillis}
	v := []int64{n.Calls, n.InputTokens, n.OutputTokens, n.ReasoningTokens, n.TotalTokens, n.OpenRouterCashMicros, n.ClaudeSubscriptionCostEquivalentMicros, n.WallMillis}
	for i := range d {
		if v[i] < 0 || *d[i] > math.MaxInt64-v[i] {
			return fmt.Errorf("accounting overflow")
		}
		*d[i] += v[i]
	}
	return nil
}
func transportForRoleQualificationCellV3(c RoleQualificationExecutionCellV3) string {
	if c.ConfigurationClass.RuntimeProvider == "claude_code_subscription" {
		return RoleQualificationClaudeTransportV3
	}
	return RoleQualificationOpenCodeTransportV3
}
func findRoleQualificationCellV3(p RoleQualificationExecutionPlanV3, id string) (RoleQualificationExecutionCellV3, bool) {
	for _, c := range p.Cells {
		if c.CellID == id {
			return c, true
		}
	}
	return RoleQualificationExecutionCellV3{}, false
}
func writeOnceRoleQualificationJSONV3(path string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = f.Write(raw); err != nil {
		return err
	}
	return f.Sync()
}
func readStrictJSONFileV3(path string, v any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return strictJSON(raw, v)
}
func fileExistsV3(path string) bool { _, err := os.Stat(path); return err == nil }
func readRoleQualificationClaimsV3(root string) (map[string]RoleQualificationClaimV3, error) {
	out := map[string]RoleQualificationClaimV3{}
	entries, err := os.ReadDir(filepath.Join(root, "claims"))
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		var c RoleQualificationClaimV3
		if err := readStrictJSONFileV3(filepath.Join(root, "claims", e.Name()), &c); err != nil {
			return nil, err
		}
		if c.SchemaVersion != "silver-mediums.role-qualification-claim.v3" || c.CellID == "" {
			return nil, fmt.Errorf("invalid claim")
		}
		if _, ok := out[c.CellID]; ok {
			return nil, fmt.Errorf("duplicate claim")
		}
		out[c.CellID] = c
	}
	return out, nil
}
func appendRoleQualificationLedgerV3(root string, raw []byte) error {
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(root, "ledger.jsonl"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = f.Write(append(raw, '\n')); err != nil {
		return err
	}
	return f.Sync()
}
func ensureRoleQualificationLedgerEntryV3(root string, o RoleQualificationObservationV3) error {
	path := filepath.Join(root, "ledger.jsonl")
	if fileExistsV3(path) {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		s := bufio.NewScanner(f)
		for s.Scan() {
			var old RoleQualificationObservationV3
			if err := strictJSON(s.Bytes(), &old); err != nil {
				return err
			}
			if old.CellID == o.CellID {
				a, _ := json.Marshal(old)
				b, _ := json.Marshal(o)
				if !bytes.Equal(a, b) {
					return fmt.Errorf("ledger drift")
				}
				return nil
			}
		}
		if err := s.Err(); err != nil {
			return err
		}
	}
	raw, _ := json.Marshal(o)
	return appendRoleQualificationLedgerV3(root, raw)
}
func validateRoleQualificationLedgerV3(root string, want int) error {
	path := filepath.Join(root, "ledger.jsonl")
	if want == 0 && !fileExistsV3(path) {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	count := 0
	seen := map[string]bool{}
	for s.Scan() {
		var o RoleQualificationObservationV3
		if err := strictJSON(s.Bytes(), &o); err != nil {
			return err
		}
		if seen[o.CellID] {
			return fmt.Errorf("duplicate ledger")
		}
		seen[o.CellID] = true
		var sealed RoleQualificationObservationV3
		if err := readStrictJSONFileV3(filepath.Join(root, "observations", o.CellID+".json"), &sealed); err != nil {
			return err
		}
		a, _ := json.Marshal(o)
		b, _ := json.Marshal(sealed)
		if !bytes.Equal(a, b) {
			return fmt.Errorf("ledger mismatch")
		}
		count++
	}
	if err := s.Err(); err != nil {
		return err
	}
	if count != want {
		return fmt.Errorf("ledger completeness")
	}
	return nil
}
