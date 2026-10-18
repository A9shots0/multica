package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// v2 adds immutable assignment binding and automatic-disqualifier receipts.
// No v1 observation is silently upgraded because it cannot prove either fact.
const ObservationSchemaVersion = "silver-mediums.agent-observation.v2"

const BatchUsageSchemaVersion = "silver-mediums.agent-batch-usage.v1"

// ObservationRecord is the immutable evidence envelope for one case attempt.
// It measures a complete run family, not merely the task that produced the
// final answer. A single-agent attempt has one expected run; controller-led
// attempts include intake, workers, reviewers, retries, and recovery runs.
type ObservationRecord struct {
	SchemaVersion              string            `json:"schema_version"`
	ObservationID              string            `json:"observation_id"`
	EvaluationBinding          string            `json:"evaluation_binding"`
	CandidateID                string            `json:"candidate_id"`
	CandidateVersion           string            `json:"candidate_version"`
	CaseID                     string            `json:"case_id"`
	CaseVersion                string            `json:"case_version"`
	HarnessVersion             string            `json:"harness_version"`
	EnvironmentVersion         string            `json:"environment_version"`
	GraderVersion              string            `json:"grader_version"`
	HumanCalibrationStatus     string            `json:"human_calibration_status"`
	TrialStatus                string            `json:"trial_status"`
	FailureClass               string            `json:"failure_class"`
	Severity                   string            `json:"severity"`
	Confidence                 string            `json:"confidence"`
	UnaffectedControls         []string          `json:"unaffected_controls"`
	DeterministicChecks        []EvidenceRef     `json:"deterministic_checks"`
	StartedAt                  string            `json:"started_at"`
	CompletedAt                string            `json:"completed_at"`
	Configurations             []Configuration   `json:"configurations"`
	ExpectedRuns               []ExpectedRun     `json:"expected_runs"`
	PreExecutionManifestSHA256 string            `json:"pre_execution_manifest_sha256"`
	FinalManifestSHA256        string            `json:"final_manifest_sha256"`
	AuthoritativeLedger        RunLedgerSnapshot `json:"authoritative_ledger"`
	RunReceipts                []RunReceipt      `json:"run_receipts"`
	RawTraceSHA256             string            `json:"raw_trace_sha256"`
	OutcomeEvidenceSHA256      string            `json:"outcome_evidence_sha256"`
	Limitations                []string          `json:"limitations,omitempty"`
	Disqualifiers              []string          `json:"disqualifiers,omitempty"`
	BlindGradeFailedChecks     []string          `json:"blind_grade_failed_checks,omitempty"`
}

// ExpectedRun is the controller's closed manifest for the complete attempt.
// The manifest is finalized before scoring. Dynamic retries may be added while
// executing, but a scorer cannot omit them afterward to improve cost numbers.
type ExpectedRun struct {
	RunID           string `json:"run_id"`
	Stage           string `json:"stage"`
	ConfigurationID string `json:"configuration_id"`
	Dynamic         bool   `json:"dynamic"`
}

type EvidenceRef struct {
	Kind   string `json:"kind"`
	SHA256 string `json:"sha256"`
}

// RunLedgerSnapshot is returned by the controller/platform after a run family
// is closed. It is independent of the candidate's narrative and includes
// dynamically appended retries. Its digest binds the authoritative source,
// snapshot identity, family, closure time, and exact run IDs.
type RunLedgerSnapshot struct {
	Source       string   `json:"source"`
	SnapshotID   string   `json:"snapshot_id"`
	FamilyRootID string   `json:"family_root_id"`
	ClosedAt     string   `json:"closed_at"`
	RunIDs       []string `json:"run_ids"`
	SHA256       string   `json:"sha256"`
}

// Int64Measurement uses a pointer so zero is distinguishable from unavailable.
// A cost baseline requires every field; providers that do not expose one must
// mark the whole observation ineligible rather than silently treating it as 0.
type Int64Measurement struct {
	FreshInputTokens  *int64 `json:"fresh_input_tokens"`
	CachedInputTokens *int64 `json:"cached_input_tokens"`
	OutputTokens      *int64 `json:"output_tokens"`
	WallTimeMillis    *int64 `json:"wall_time_millis"`
}

type RunReceipt struct {
	RunID                string           `json:"run_id"`
	FamilyRootID         string           `json:"family_root_id"`
	ParentRunID          string           `json:"parent_run_id,omitempty"`
	AgentID              string           `json:"agent_id"`
	ConfigurationID      string           `json:"configuration_id"`
	ConfigurationSHA256  string           `json:"configuration_sha256"`
	Provider             string           `json:"provider"`
	ReportedModelVersion string           `json:"reported_model_version"`
	EvaluationBinding    string           `json:"evaluation_binding"`
	UsageSource          string           `json:"usage_source"`
	UsageBoundary        string           `json:"usage_boundary"`
	Usage                Int64Measurement `json:"usage"`
	OwnerInterventions   int              `json:"owner_interventions"`
	Retry                bool             `json:"retry"`
}

type ObservationTotals struct {
	FreshInputTokens   int64 `json:"fresh_input_tokens"`
	CachedInputTokens  int64 `json:"cached_input_tokens"`
	OutputTokens       int64 `json:"output_tokens"`
	RunComputeMillis   int64 `json:"run_compute_millis"`
	EndToEndMillis     int64 `json:"end_to_end_millis"`
	OwnerInterventions int   `json:"owner_interventions"`
	Runs               int   `json:"runs"`
	Retries            int   `json:"retries"`
}

type BatchUsageLedger struct {
	SchemaVersion       string              `json:"schema_version"`
	BatchID             string              `json:"batch_id"`
	TokenCeiling        int64               `json:"token_ceiling"`
	ObservationReceipts []ObservationRecord `json:"observation_receipts"`
}

type BatchUsageVerdict struct {
	CanLaunchNext bool              `json:"can_launch_next"`
	Reasons       []string          `json:"reasons,omitempty"`
	Totals        ObservationTotals `json:"totals"`
}

type ObservationValidity struct {
	Valid                bool              `json:"valid"`
	CostBaselineEligible bool              `json:"cost_baseline_eligible"`
	Reasons              []string          `json:"reasons,omitempty"`
	Totals               ObservationTotals `json:"totals"`
}

func ConfigurationSHA256(configuration Configuration) (string, error) {
	if err := ValidateConfiguration(configuration); err != nil {
		return "", err
	}
	raw, err := json.Marshal(configuration)
	if err != nil {
		return "", fmt.Errorf("marshal configuration: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func ExpectedRunsSHA256(runs []ExpectedRun, includeDynamic bool) (string, error) {
	selected := make([]ExpectedRun, 0, len(runs))
	for _, run := range runs {
		if includeDynamic || !run.Dynamic {
			selected = append(selected, run)
		}
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].RunID < selected[j].RunID })
	raw, err := json.Marshal(selected)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func RunLedgerSHA256(snapshot RunLedgerSnapshot) (string, error) {
	copySnapshot := snapshot
	copySnapshot.SHA256 = ""
	copySnapshot.RunIDs = append([]string(nil), snapshot.RunIDs...)
	sort.Strings(copySnapshot.RunIDs)
	raw, err := json.Marshal(copySnapshot)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// ValidateObservation proves that the scorer received the exact complete run
// family and complete usage fields. It deliberately does not score competence;
// invalid experimental data must be rejected before any outcome rubric runs.
func ValidateObservation(record ObservationRecord) ObservationValidity {
	result := ObservationValidity{}
	add := func(reason string) { result.Reasons = append(result.Reasons, reason) }

	if record.SchemaVersion != ObservationSchemaVersion {
		add("schema_version_invalid")
	}
	for name, value := range map[string]string{
		"observation_id":           record.ObservationID,
		"evaluation_binding":       record.EvaluationBinding,
		"candidate_id":             record.CandidateID,
		"candidate_version":        record.CandidateVersion,
		"case_id":                  record.CaseID,
		"case_version":             record.CaseVersion,
		"harness_version":          record.HarnessVersion,
		"environment_version":      record.EnvironmentVersion,
		"grader_version":           record.GraderVersion,
		"human_calibration_status": record.HumanCalibrationStatus,
	} {
		if strings.TrimSpace(value) == "" {
			add(name + "_missing")
		}
	}
	if !contains([]string{"calibrated", "calibration_pending", "not_applicable"}, record.HumanCalibrationStatus) {
		add("human_calibration_status_invalid")
	}
	if !contains([]string{"passed", "failed", "invalid", "inconclusive"}, record.TrialStatus) {
		add("trial_status_invalid")
	}
	for _, disqualifier := range record.Disqualifiers {
		if strings.TrimSpace(disqualifier) == "" {
			add("disqualifier_empty")
		}
	}
	for _, check := range record.BlindGradeFailedChecks {
		if strings.TrimSpace(check) == "" {
			add("blind_grade_failed_check_empty")
		}
	}
	if !contains([]string{"none", "model", "infrastructure", "grader", "environment", "authority", "unknown"}, record.FailureClass) {
		add("failure_class_invalid")
	}
	if !contains([]string{"none", "p0", "p1", "p2"}, record.Severity) {
		add("severity_invalid")
	}
	if !contains([]string{"low", "medium", "high"}, record.Confidence) {
		add("confidence_invalid")
	}
	if len(record.UnaffectedControls) == 0 {
		add("unaffected_controls_missing")
	}
	if len(record.DeterministicChecks) == 0 {
		add("deterministic_checks_missing")
	}
	for _, check := range record.DeterministicChecks {
		if strings.TrimSpace(check.Kind) == "" || !validSHA256(check.SHA256) {
			add("deterministic_check_invalid")
		}
	}
	started, startErr := time.Parse(time.RFC3339Nano, record.StartedAt)
	completed, completeErr := time.Parse(time.RFC3339Nano, record.CompletedAt)
	if startErr != nil {
		add("started_at_invalid")
	}
	if completeErr != nil {
		add("completed_at_invalid")
	}
	if startErr == nil && completeErr == nil && completed.Before(started) {
		add("completion_precedes_start")
	}
	if startErr == nil && completeErr == nil && !completed.Before(started) {
		result.Totals.EndToEndMillis = completed.Sub(started).Milliseconds()
	}
	if !validSHA256(record.RawTraceSHA256) {
		add("raw_trace_sha256_invalid")
	}
	if !validSHA256(record.OutcomeEvidenceSHA256) {
		add("outcome_evidence_sha256_invalid")
	}

	configurationHashes := make(map[string]string, len(record.Configurations))
	for _, configuration := range record.Configurations {
		if _, duplicate := configurationHashes[configuration.ID]; duplicate {
			add("configuration_duplicate:" + configuration.ID)
			continue
		}
		hash, err := ConfigurationSHA256(configuration)
		if err != nil {
			add("configuration_invalid:" + configuration.ID)
			continue
		}
		configurationHashes[configuration.ID] = hash
	}
	if len(record.Configurations) == 0 {
		add("configurations_missing")
	}

	expected := make(map[string]ExpectedRun, len(record.ExpectedRuns))
	for _, run := range record.ExpectedRuns {
		if strings.TrimSpace(run.RunID) == "" || strings.TrimSpace(run.Stage) == "" || strings.TrimSpace(run.ConfigurationID) == "" {
			add("expected_run_incomplete")
			continue
		}
		if _, duplicate := expected[run.RunID]; duplicate {
			add("expected_run_duplicate:" + run.RunID)
			continue
		}
		if _, ok := configurationHashes[run.ConfigurationID]; !ok {
			add("expected_run_configuration_missing:" + run.RunID)
		}
		expected[run.RunID] = run
	}
	if len(record.ExpectedRuns) == 0 {
		add("expected_runs_missing")
	}
	preHash, _ := ExpectedRunsSHA256(record.ExpectedRuns, false)
	if record.PreExecutionManifestSHA256 != preHash {
		add("pre_execution_manifest_hash_invalid")
	}
	finalHash, _ := ExpectedRunsSHA256(record.ExpectedRuns, true)
	if record.FinalManifestSHA256 != finalHash {
		add("final_manifest_hash_invalid")
	}
	ledgerHash, _ := RunLedgerSHA256(record.AuthoritativeLedger)
	if record.AuthoritativeLedger.Source != "controller_run_ledger" ||
		record.AuthoritativeLedger.SnapshotID == "" ||
		record.AuthoritativeLedger.FamilyRootID != record.ObservationID ||
		record.AuthoritativeLedger.SHA256 != ledgerHash {
		add("authoritative_ledger_invalid")
	}
	ledgerClosedAt, ledgerTimeErr := time.Parse(time.RFC3339Nano, record.AuthoritativeLedger.ClosedAt)
	if ledgerTimeErr != nil || (completeErr == nil && ledgerClosedAt.Before(completed)) {
		add("authoritative_ledger_not_closed_after_attempt")
	}
	ledgerRuns := make(map[string]struct{}, len(record.AuthoritativeLedger.RunIDs))
	for _, runID := range record.AuthoritativeLedger.RunIDs {
		if runID == "" {
			add("authoritative_ledger_run_missing_id")
			continue
		}
		if _, duplicate := ledgerRuns[runID]; duplicate {
			add("authoritative_ledger_run_duplicate:" + runID)
		}
		ledgerRuns[runID] = struct{}{}
	}
	for runID := range expected {
		if _, ok := ledgerRuns[runID]; !ok {
			add("authoritative_ledger_missing_expected_run:" + runID)
		}
	}
	for runID := range ledgerRuns {
		if _, ok := expected[runID]; !ok {
			add("authoritative_ledger_unmanifested_run:" + runID)
		}
	}

	receipts := make(map[string]struct{}, len(record.RunReceipts))
	for _, receipt := range record.RunReceipts {
		if strings.TrimSpace(receipt.RunID) == "" {
			add("run_id_missing")
			continue
		}
		if _, duplicate := receipts[receipt.RunID]; duplicate {
			add("run_receipt_duplicate:" + receipt.RunID)
			continue
		}
		receipts[receipt.RunID] = struct{}{}
		expectedRun, ok := expected[receipt.RunID]
		if !ok {
			add("unexpected_run_receipt:" + receipt.RunID)
		} else if expectedRun.ConfigurationID != receipt.ConfigurationID {
			add("run_configuration_mismatch:" + receipt.RunID)
		}
		if receipt.FamilyRootID != record.ObservationID {
			add("run_family_unbound:" + receipt.RunID)
		}
		if receipt.EvaluationBinding != record.EvaluationBinding {
			add("run_evaluation_unbound:" + receipt.RunID)
		}
		for name, value := range map[string]string{
			"agent":                  receipt.AgentID,
			"provider":               receipt.Provider,
			"reported_model_version": receipt.ReportedModelVersion,
			"usage_source":           receipt.UsageSource,
			"usage_boundary":         receipt.UsageBoundary,
		} {
			if strings.TrimSpace(value) == "" {
				add("run_" + name + "_missing:" + receipt.RunID)
			}
		}
		if !contains([]string{"provider_receipt", "platform_receipt", "controller_aggregate"}, receipt.UsageSource) {
			add("run_usage_source_invalid:" + receipt.RunID)
		}
		if !contains([]string{"api_tokens", "subscription_reported_tokens", "provider_reported_tokens"}, receipt.UsageBoundary) {
			add("run_usage_boundary_invalid:" + receipt.RunID)
		}
		wantHash, known := configurationHashes[receipt.ConfigurationID]
		if !known || receipt.ConfigurationSHA256 != wantHash {
			add("run_configuration_hash_invalid:" + receipt.RunID)
		}
		measurements := map[string]*int64{
			"fresh_input_tokens":  receipt.Usage.FreshInputTokens,
			"cached_input_tokens": receipt.Usage.CachedInputTokens,
			"output_tokens":       receipt.Usage.OutputTokens,
			"wall_time_millis":    receipt.Usage.WallTimeMillis,
		}
		completeUsage := true
		for name, value := range measurements {
			if value == nil {
				add("run_" + name + "_unavailable:" + receipt.RunID)
				completeUsage = false
			} else if *value < 0 {
				add("run_" + name + "_negative:" + receipt.RunID)
				completeUsage = false
			}
		}
		if receipt.OwnerInterventions < 0 {
			add("run_owner_interventions_negative:" + receipt.RunID)
		}
		if completeUsage {
			result.Totals.FreshInputTokens += *receipt.Usage.FreshInputTokens
			result.Totals.CachedInputTokens += *receipt.Usage.CachedInputTokens
			result.Totals.OutputTokens += *receipt.Usage.OutputTokens
			result.Totals.RunComputeMillis += *receipt.Usage.WallTimeMillis
		}
		result.Totals.OwnerInterventions += receipt.OwnerInterventions
		result.Totals.Runs++
		if receipt.Retry {
			result.Totals.Retries++
		}
	}
	for runID := range expected {
		if _, ok := receipts[runID]; !ok {
			add("expected_run_receipt_missing:" + runID)
		}
	}

	result.Reasons = unique(result.Reasons)
	result.Valid = len(result.Reasons) == 0
	result.CostBaselineEligible = result.Valid
	return result
}

// EvaluateBatchUsage counts every uniquely identified paid observation,
// including invalid, failed, and inconclusive attempts. Missing usage fails
// closed: the next launch cannot start because the remaining budget is unknown.
func EvaluateBatchUsage(ledger BatchUsageLedger, nextLaunchReservation int64) BatchUsageVerdict {
	result := BatchUsageVerdict{}
	add := func(reason string) { result.Reasons = append(result.Reasons, reason) }
	if ledger.SchemaVersion != BatchUsageSchemaVersion || ledger.BatchID == "" || ledger.TokenCeiling <= 0 {
		add("batch_contract_invalid")
	}
	if nextLaunchReservation < 0 {
		add("next_launch_reservation_invalid")
	}
	seenObservations := map[string]struct{}{}
	seenRuns := map[string]struct{}{}
	for _, observation := range ledger.ObservationReceipts {
		if observation.ObservationID == "" {
			add("batch_observation_id_missing")
			continue
		}
		if _, duplicate := seenObservations[observation.ObservationID]; duplicate {
			add("batch_observation_duplicate:" + observation.ObservationID)
			continue
		}
		seenObservations[observation.ObservationID] = struct{}{}
		ledgerRuns := make(map[string]struct{}, len(observation.AuthoritativeLedger.RunIDs))
		for _, runID := range observation.AuthoritativeLedger.RunIDs {
			if runID == "" {
				add("batch_authoritative_run_id_missing:" + observation.ObservationID)
				continue
			}
			ledgerRuns[runID] = struct{}{}
		}
		observationReceipts := make(map[string]struct{}, len(observation.RunReceipts))
		for _, receipt := range observation.RunReceipts {
			if receipt.RunID == "" {
				add("batch_run_id_missing")
				continue
			}
			if _, duplicate := seenRuns[receipt.RunID]; duplicate {
				add("batch_run_duplicate:" + receipt.RunID)
				continue
			}
			seenRuns[receipt.RunID] = struct{}{}
			observationReceipts[receipt.RunID] = struct{}{}
			values := []*int64{receipt.Usage.FreshInputTokens, receipt.Usage.CachedInputTokens, receipt.Usage.OutputTokens, receipt.Usage.WallTimeMillis}
			complete := true
			for _, value := range values {
				if value == nil || *value < 0 {
					complete = false
				}
			}
			if !complete {
				add("batch_run_usage_unknown:" + receipt.RunID)
				continue
			}
			result.Totals.FreshInputTokens += *receipt.Usage.FreshInputTokens
			result.Totals.CachedInputTokens += *receipt.Usage.CachedInputTokens
			result.Totals.OutputTokens += *receipt.Usage.OutputTokens
			result.Totals.RunComputeMillis += *receipt.Usage.WallTimeMillis
			result.Totals.OwnerInterventions += receipt.OwnerInterventions
			result.Totals.Runs++
			if receipt.Retry {
				result.Totals.Retries++
			}
		}
		for runID := range ledgerRuns {
			if _, ok := observationReceipts[runID]; !ok {
				add("batch_authoritative_run_usage_missing:" + runID)
			}
		}
	}
	spent := result.Totals.FreshInputTokens + result.Totals.CachedInputTokens + result.Totals.OutputTokens
	if spent+nextLaunchReservation > ledger.TokenCeiling {
		add("batch_token_ceiling_reached")
	}
	result.Reasons = unique(result.Reasons)
	result.CanLaunchNext = len(result.Reasons) == 0
	return result
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
