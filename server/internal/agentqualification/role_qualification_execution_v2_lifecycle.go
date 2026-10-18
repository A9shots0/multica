package agentqualification

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	RoleQualificationOpenCodeTransportV2 = "opencode_structured_v33_http_loopback"
	RoleQualificationClaudeTransportV2   = "claude_code_subscription_tool_free"
)

type RoleQualificationUsageV2 struct {
	Calls                                  int64 `json:"calls"`
	InputTokens                            int64 `json:"input_tokens"`
	OutputTokens                           int64 `json:"output_tokens"`
	ReasoningTokens                        int64 `json:"reasoning_tokens"`
	TotalTokens                            int64 `json:"total_tokens"`
	OpenRouterCashMicros                   int64 `json:"openrouter_cash_micros"`
	ClaudeSubscriptionCostEquivalentMicros int64 `json:"claude_subscription_cost_equivalent_micros"`
	WallMillis                             int64 `json:"wall_millis"`
}
type RoleQualificationClaimV2 struct {
	SchemaVersion     string `json:"schema_version"`
	CellID            string `json:"cell_id"`
	PacketSHA256      string `json:"packet_sha256"`
	ClaimedAt         string `json:"claimed_at"`
	ProviderCallLimit int    `json:"provider_call_limit"`
	Retries           int    `json:"retries"`
}
type RoleQualificationProviderReceiptV2 struct {
	SchemaVersion        string                   `json:"schema_version"`
	CellID               string                   `json:"cell_id"`
	ProviderCallID       string                   `json:"provider_call_id"`
	RequestedModel       string                   `json:"requested_model"`
	ActualModel          string                   `json:"actual_model"`
	RequestedProvider    string                   `json:"requested_provider"`
	ActualProvider       string                   `json:"actual_provider"`
	RequestedEffort      string                   `json:"requested_effort"`
	ActualEffort         string                   `json:"actual_effort"`
	RequestedTransport   string                   `json:"requested_transport"`
	ActualTransport      string                   `json:"actual_transport"`
	ProviderFallbackUsed bool                     `json:"provider_fallback_used"`
	RetryCount           int                      `json:"retry_count"`
	StartedAt            string                   `json:"started_at"`
	CompletedAt          string                   `json:"completed_at"`
	RawResponse          json.RawMessage          `json:"raw_response"`
	RawResponseSHA256    string                   `json:"raw_response_sha256"`
	StructuredOutput     json.RawMessage          `json:"structured_output,omitempty"`
	FailureClass         string                   `json:"failure_class,omitempty"`
	Usage                RoleQualificationUsageV2 `json:"usage"`
}
type RoleQualificationObservationV2 struct {
	SchemaVersion         string                   `json:"schema_version"`
	CellID                string                   `json:"cell_id"`
	Status                string                   `json:"status"`
	FailureScope          string                   `json:"failure_scope,omitempty"`
	FailureClass          string                   `json:"failure_class,omitempty"`
	ReceiptSHA256         string                   `json:"receipt_sha256"`
	CanonicalOutputSHA256 string                   `json:"canonical_output_sha256,omitempty"`
	Grade                 RoleQualificationGradeV1 `json:"grade"`
	Usage                 RoleQualificationUsageV2 `json:"usage"`
}
type RoleQualificationReplayV2 struct {
	Claims             int                      `json:"claims"`
	Receipts           int                      `json:"receipts"`
	Observations       int                      `json:"observations"`
	StrandedNoReceipt  []string                 `json:"stranded_no_receipt_no_retry"`
	Totals             RoleQualificationUsageV2 `json:"totals"`
	GlobalHalt         bool                     `json:"global_halt"`
	EliminatedBindings []string                 `json:"eliminated_configuration_role_bindings,omitempty"`
}

func RoleQualificationExecutionReplayV2(root string) (RoleQualificationReplayV2, error) {
	plan, err := BuildRoleQualificationExecutionPlanV2()
	if err != nil {
		return RoleQualificationReplayV2{}, err
	}
	byID := map[string]RoleQualificationExecutionCellV2{}
	for _, c := range plan.Cells {
		byID[c.CellID] = c
	}
	replay := RoleQualificationReplayV2{}
	claims, err := readRoleQualificationClaimsV2(root)
	if err != nil {
		return replay, err
	}
	replay.Claims = len(claims)
	eliminated := map[string]bool{}
	for cellID, claim := range claims {
		cell, ok := byID[cellID]
		if !ok || claim.PacketSHA256 != cell.PacketSHA256 || claim.ProviderCallLimit != 1 || claim.Retries != 0 {
			return replay, fmt.Errorf("claim evidence drift for %s", cellID)
		}
		receiptPath := filepath.Join(root, "receipts", cellID+".json")
		observationPath := filepath.Join(root, "observations", cellID+".json")
		receiptExists := fileExistsV2(receiptPath)
		observationExists := fileExistsV2(observationPath)
		if receiptExists {
			replay.Receipts++
		}
		if !receiptExists && !observationExists {
			replay.StrandedNoReceipt = append(replay.StrandedNoReceipt, cellID)
			continue
		}
		if observationExists && !receiptExists {
			return replay, fmt.Errorf("observation without raw receipt %s", cellID)
		}
		if receiptExists && !observationExists {
			return replay, fmt.Errorf("receipt pending provider-free reconciliation %s", cellID)
		}
		var receipt RoleQualificationProviderReceiptV2
		if err := readStrictJSONFileV2(receiptPath, &receipt); err != nil {
			return replay, err
		}
		if err := validateRoleQualificationReceiptV2(cell, receipt, plan.PerObservation); err != nil {
			return replay, err
		}
		var observation RoleQualificationObservationV2
		if err := readStrictJSONFileV2(observationPath, &observation); err != nil {
			return replay, err
		}
		if err := validateRoleQualificationObservationV2(receipt, observation); err != nil {
			return replay, err
		}
		if err := addRoleQualificationUsageV2(&replay.Totals, observation.Usage); err != nil {
			return replay, err
		}
		replay.Observations++
		if observation.FailureScope == "global_halt" {
			replay.GlobalHalt = true
		}
		if observation.FailureScope == "candidate_local" {
			key := cell.ConfigurationClass.ClassID + ":" + cell.CandidateClass
			if !eliminated[key] {
				eliminated[key] = true
				replay.EliminatedBindings = append(replay.EliminatedBindings, key)
			}
		}
	}
	if err := validateRoleQualificationCampaignUsageV2(replay.Totals, plan.Campaign); err != nil {
		return replay, err
	}
	if err := validateRoleQualificationLedgerV2(root, replay.Observations); err != nil {
		return replay, err
	}
	sort.Strings(replay.StrandedNoReceipt)
	sort.Strings(replay.EliminatedBindings)
	return replay, nil
}

func WriteRoleQualificationClaimV2(root, cellID, now string) error {
	plan, err := BuildRoleQualificationExecutionPlanV2()
	if err != nil {
		return err
	}
	replay, err := RoleQualificationExecutionReplayV2(root)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	cell, ok := findRoleQualificationCellV2(plan, cellID)
	if !ok {
		return fmt.Errorf("unknown cell %s", cellID)
	}
	if replay.GlobalHalt {
		return fmt.Errorf("global halt forbids another grant")
	}
	pair := cell.ConfigurationClass.ClassID + ":" + cell.CandidateClass
	for _, eliminated := range replay.EliminatedBindings {
		if eliminated == pair {
			return fmt.Errorf("candidate-local failure eliminated %s", pair)
		}
	}
	if now == "" {
		now = time.Now().UTC().Format(time.RFC3339Nano)
	}
	claim := RoleQualificationClaimV2{"silver-mediums.role-qualification-claim.v2", cellID, cell.PacketSHA256, now, 1, 0}
	return writeOnceRoleQualificationJSONV2(filepath.Join(root, "claims", cellID+".json"), claim)
}

func WriteRoleQualificationReceiptV2(root string, receipt RoleQualificationProviderReceiptV2) error {
	plan, err := BuildRoleQualificationExecutionPlanV2()
	if err != nil {
		return err
	}
	cell, ok := findRoleQualificationCellV2(plan, receipt.CellID)
	if !ok {
		return fmt.Errorf("unknown cell %s", receipt.CellID)
	}
	var claim RoleQualificationClaimV2
	if err := readStrictJSONFileV2(filepath.Join(root, "claims", receipt.CellID+".json"), &claim); err != nil {
		return fmt.Errorf("write-ahead claim required: %w", err)
	}
	if claim.PacketSHA256 != cell.PacketSHA256 {
		return fmt.Errorf("claim packet drift")
	}
	if err := validateRoleQualificationReceiptV2(cell, receipt, plan.PerObservation); err != nil {
		return err
	}
	return writeOnceRoleQualificationJSONV2(filepath.Join(root, "receipts", receipt.CellID+".json"), receipt)
}

func ReconcileRoleQualificationClaimV2(root, cellID string) (string, error) {
	plan, err := BuildRoleQualificationExecutionPlanV2()
	if err != nil {
		return "", err
	}
	cell, ok := findRoleQualificationCellV2(plan, cellID)
	if !ok {
		return "", fmt.Errorf("unknown cell %s", cellID)
	}
	claimPath := filepath.Join(root, "claims", cellID+".json")
	if !fileExistsV2(claimPath) {
		return "", fmt.Errorf("claim missing")
	}
	observationPath := filepath.Join(root, "observations", cellID+".json")
	if fileExistsV2(observationPath) {
		var observation RoleQualificationObservationV2
		if err := readStrictJSONFileV2(observationPath, &observation); err != nil {
			return "", err
		}
		if err := ensureRoleQualificationLedgerEntryV2(root, observation); err != nil {
			return "", err
		}
		return "already_closed_no_retry", nil
	}
	receiptPath := filepath.Join(root, "receipts", cellID+".json")
	if !fileExistsV2(receiptPath) {
		return "stranded_no_receipt_no_retry", nil
	}
	var receipt RoleQualificationProviderReceiptV2
	if err := readStrictJSONFileV2(receiptPath, &receipt); err != nil {
		return "", err
	}
	if err := validateRoleQualificationReceiptV2(cell, receipt, plan.PerObservation); err != nil {
		return "", err
	}
	observation, err := deriveRoleQualificationObservationV2(cell, receipt)
	if err != nil {
		return "", err
	}
	if err := writeOnceRoleQualificationJSONV2(observationPath, observation); err != nil {
		return "", err
	}
	raw, _ := json.Marshal(observation)
	if err := appendRoleQualificationLedgerV2(root, raw); err != nil {
		return "", err
	}
	return observation.Status, nil
}

func deriveRoleQualificationObservationV2(cell RoleQualificationExecutionCellV2, r RoleQualificationProviderReceiptV2) (RoleQualificationObservationV2, error) {
	receiptRaw, _ := json.Marshal(r)
	observation := RoleQualificationObservationV2{SchemaVersion: "silver-mediums.role-qualification-observation.v2", CellID: r.CellID, ReceiptSHA256: sha256Hex(receiptRaw), Usage: r.Usage}
	if r.FailureClass != "" {
		scope, ok := classifyRoleQualificationFailureV2(r.FailureClass)
		if !ok {
			return observation, fmt.Errorf("unrecognized failure class %s", r.FailureClass)
		}
		observation.Status = "failed"
		observation.FailureScope = scope
		observation.FailureClass = r.FailureClass
		return observation, nil
	}
	canonical, grade, err := ValidateAndGradeRoleQualificationExecutionOutputV2(cell.CaseID, r.StructuredOutput)
	if err != nil {
		observation.Status = "failed"
		observation.FailureScope = "candidate_local"
		observation.FailureClass = "response_or_tool_protocol_failure"
		return observation, nil
	}
	observation.CanonicalOutputSHA256 = sha256Hex(canonical)
	observation.Grade = grade
	if grade.Pass {
		observation.Status = "passed"
	} else {
		observation.Status = "failed"
		observation.FailureScope = "candidate_local"
		observation.FailureClass = "competence_failure"
	}
	return observation, nil
}

func validateRoleQualificationReceiptV2(cell RoleQualificationExecutionCellV2, r RoleQualificationProviderReceiptV2, caps RoleQualificationObservationCapsV2) error {
	if r.SchemaVersion != "silver-mediums.role-qualification-provider-receipt.v2" || r.CellID != cell.CellID || r.ProviderCallID == "" || r.ProviderFallbackUsed || r.RetryCount != 0 || r.Usage.Calls != 1 {
		return fmt.Errorf("receipt identity, call, retry, or fallback drifted")
	}
	c := cell.ConfigurationClass
	if r.RequestedModel != c.Model || r.ActualModel != c.Model || r.RequestedProvider != c.ServingProvider || r.ActualProvider != c.ServingProvider || r.RequestedEffort != c.Effort || r.ActualEffort != c.Effort {
		return fmt.Errorf("receipt requested/actual route drifted")
	}
	wantTransport := RoleQualificationOpenCodeTransportV2
	if c.RuntimeProvider == "claude_code_subscription" {
		wantTransport = RoleQualificationClaudeTransportV2
	}
	if r.RequestedTransport != wantTransport || r.ActualTransport != wantTransport {
		return fmt.Errorf("receipt transport drifted")
	}
	if len(r.RawResponse) == 0 || !json.Valid(r.RawResponse) || sha256Hex(r.RawResponse) != r.RawResponseSHA256 {
		return fmt.Errorf("raw receipt evidence invalid")
	}
	if _, err := time.Parse(time.RFC3339Nano, r.StartedAt); err != nil {
		return fmt.Errorf("receipt start invalid")
	}
	if _, err := time.Parse(time.RFC3339Nano, r.CompletedAt); err != nil {
		return fmt.Errorf("receipt completion invalid")
	}
	u := r.Usage
	if u.InputTokens < 0 || u.OutputTokens < 0 || u.ReasoningTokens < 0 || u.TotalTokens < 0 || u.OpenRouterCashMicros < 0 || u.ClaudeSubscriptionCostEquivalentMicros < 0 || u.WallMillis < 0 || u.TotalTokens != u.InputTokens+u.OutputTokens+u.ReasoningTokens {
		return fmt.Errorf("receipt accounting invalid")
	}
	if u.InputTokens > caps.MaxInputTokens || u.OutputTokens > caps.MaxOutputTokens || u.TotalTokens > caps.MaxTotalTokens || u.WallMillis > caps.MaxWallSeconds*1000 {
		return fmt.Errorf("observation_resource_exhaustion")
	}
	if c.RuntimeProvider == "claude_code_subscription" {
		if u.OpenRouterCashMicros != 0 {
			return fmt.Errorf("Claude subscription conflated with marginal cash")
		}
	} else if u.ClaudeSubscriptionCostEquivalentMicros != 0 {
		return fmt.Errorf("OpenRouter receipt conflated with subscription equivalent")
	}
	return nil
}

func validateRoleQualificationObservationV2(r RoleQualificationProviderReceiptV2, o RoleQualificationObservationV2) error {
	if o.SchemaVersion != "silver-mediums.role-qualification-observation.v2" || o.CellID != r.CellID || o.Usage != r.Usage {
		return fmt.Errorf("observation semantic drift")
	}
	raw, _ := json.Marshal(r)
	if o.ReceiptSHA256 != sha256Hex(raw) {
		return fmt.Errorf("observation receipt hash drift")
	}
	return nil
}
func classifyRoleQualificationFailureV2(class string) (string, bool) {
	candidate := map[string]bool{"competence_failure": true, "response_or_tool_protocol_failure": true, "observation_resource_exhaustion": true}
	isolated := map[string]bool{"isolated_fixture_imperfection": true, "isolated_grader_imperfection": true}
	global := map[string]bool{"shared_test_invalidity": true, "evidence_integrity_failure": true, "credential_failure": true, "unsafe_external_action": true, "runtime_configuration_failure": true, "credits_failure": true, "transport_failure": true, "campaign_token_cap_breach": true, "campaign_spend_cap_breach": true, "campaign_wall_cap_breach": true}
	if candidate[class] {
		return "candidate_local", true
	}
	if isolated[class] {
		return "observation_local_unscored", true
	}
	if global[class] {
		return "global_halt", true
	}
	return "", false
}

func addRoleQualificationUsageV2(total *RoleQualificationUsageV2, next RoleQualificationUsageV2) error {
	fields := []struct {
		dst   *int64
		value int64
	}{{&total.Calls, next.Calls}, {&total.InputTokens, next.InputTokens}, {&total.OutputTokens, next.OutputTokens}, {&total.ReasoningTokens, next.ReasoningTokens}, {&total.TotalTokens, next.TotalTokens}, {&total.OpenRouterCashMicros, next.OpenRouterCashMicros}, {&total.ClaudeSubscriptionCostEquivalentMicros, next.ClaudeSubscriptionCostEquivalentMicros}, {&total.WallMillis, next.WallMillis}}
	for _, f := range fields {
		if f.value < 0 || *f.dst > int64(^uint64(0)>>1)-f.value {
			return fmt.Errorf("accounting overflow")
		}
		*f.dst += f.value
	}
	return nil
}
func validateRoleQualificationCampaignUsageV2(u RoleQualificationUsageV2, c RoleQualificationCampaignCapsV2) error {
	if u.Calls > int64(c.MaxCalls) || u.InputTokens > c.MaxInputTokens || u.OutputTokens > c.MaxOutputTokens || u.TotalTokens > c.MaxTotalTokens || u.WallMillis > c.MaxWallSeconds*1000 {
		return fmt.Errorf("campaign token/call/wall cap breach")
	}
	if u.OpenRouterCashMicros > c.MaxOpenRouterCashMicros {
		return fmt.Errorf("campaign spend cap breach")
	}
	return nil
}
func findRoleQualificationCellV2(plan RoleQualificationExecutionPlanV2, id string) (RoleQualificationExecutionCellV2, bool) {
	for _, c := range plan.Cells {
		if c.CellID == id {
			return c, true
		}
	}
	return RoleQualificationExecutionCellV2{}, false
}
func writeOnceRoleQualificationJSONV2(path string, value any) error {
	raw, err := json.Marshal(value)
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
	if _, err := f.Write(raw); err != nil {
		return err
	}
	return f.Sync()
}
func appendRoleQualificationLedgerV2(root string, raw []byte) error {
	path := filepath.Join(root, "ledger.jsonl")
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(raw, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

func ensureRoleQualificationLedgerEntryV2(root string, observation RoleQualificationObservationV2) error {
	path := filepath.Join(root, "ledger.jsonl")
	if fileExistsV2(path) {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			var existing RoleQualificationObservationV2
			if err := strictJSON(scanner.Bytes(), &existing); err != nil {
				return err
			}
			if existing.CellID == observation.CellID {
				existingHash, _ := roleQualificationCanonicalHashV2(existing)
				observationHash, _ := roleQualificationCanonicalHashV2(observation)
				if existingHash != observationHash {
					return fmt.Errorf("ledger observation drift for %s", observation.CellID)
				}
				return nil
			}
		}
		if err := scanner.Err(); err != nil {
			return err
		}
	}
	raw, _ := json.Marshal(observation)
	return appendRoleQualificationLedgerV2(root, raw)
}
func readRoleQualificationClaimsV2(root string) (map[string]RoleQualificationClaimV2, error) {
	out := map[string]RoleQualificationClaimV2{}
	entries, err := os.ReadDir(filepath.Join(root, "claims"))
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		var claim RoleQualificationClaimV2
		if err := readStrictJSONFileV2(filepath.Join(root, "claims", entry.Name()), &claim); err != nil {
			return nil, err
		}
		if claim.SchemaVersion != "silver-mediums.role-qualification-claim.v2" || claim.CellID == "" {
			return nil, fmt.Errorf("invalid claim %s", entry.Name())
		}
		if _, exists := out[claim.CellID]; exists {
			return nil, fmt.Errorf("duplicate claim %s", claim.CellID)
		}
		out[claim.CellID] = claim
	}
	return out, nil
}
func readStrictJSONFileV2(path string, dst any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return strictJSON(raw, dst)
}
func fileExistsV2(path string) bool { _, err := os.Stat(path); return err == nil }
func validateRoleQualificationLedgerV2(root string, want int) error {
	path := filepath.Join(root, "ledger.jsonl")
	if want == 0 && !fileExistsV2(path) {
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
		var observation RoleQualificationObservationV2
		if err := strictJSON(scanner.Bytes(), &observation); err != nil {
			return err
		}
		if seen[observation.CellID] {
			return fmt.Errorf("duplicate ledger observation %s", observation.CellID)
		}
		seen[observation.CellID] = true
		var sealed RoleQualificationObservationV2
		if err := readStrictJSONFileV2(filepath.Join(root, "observations", observation.CellID+".json"), &sealed); err != nil {
			return err
		}
		sealedHash, _ := roleQualificationCanonicalHashV2(sealed)
		observationHash, _ := roleQualificationCanonicalHashV2(observation)
		if sealedHash != observationHash {
			return fmt.Errorf("ledger and observation disagree for %s", observation.CellID)
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if count != want {
		return fmt.Errorf("ledger count=%d want %d", count, want)
	}
	return nil
}
