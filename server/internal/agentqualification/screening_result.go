package agentqualification

import (
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

const ScreeningResultSchemaVersion = "silver-mediums.agent-screening-result.v1"

type ScreeningCapacityResult struct {
	Provider            string `json:"provider"`
	LimitID             string `json:"limit_id"`
	AccountLane         string `json:"account_lane"`
	BaselineUsedPercent int    `json:"baseline_used_percent"`
	FinalUsedPercent    int    `json:"final_used_percent"`
}

type ScreeningObservationResult struct {
	CandidateID        string   `json:"candidate_id"`
	IssueIdentifier    string   `json:"issue_identifier"`
	IssueID            string   `json:"issue_id"`
	TriggerCommentID   string   `json:"trigger_comment_id"`
	RunID              string   `json:"run_id"`
	SessionID          string   `json:"session_id"`
	StartedAt          string   `json:"started_at"`
	CompletedAt        string   `json:"completed_at"`
	OutputSHA256       string   `json:"output_sha256"`
	Verdict            string   `json:"verdict"`
	FailedChecks       []string `json:"failed_checks"`
	InputTokens        int64    `json:"input_tokens"`
	CachedInputTokens  int64    `json:"cached_input_tokens"`
	FreshInputTokens   int64    `json:"fresh_input_tokens"`
	OutputTokens       int64    `json:"output_tokens"`
	TotalTokens        int64    `json:"total_tokens"`
	Retries            int      `json:"retries"`
	OwnerInterventions int      `json:"owner_interventions"`
	ToolCalls          int      `json:"tool_calls,omitempty"`
	ExecutedTools      []string `json:"executed_tools,omitempty"`
	BlindGradeVerdict  string   `json:"blind_grade_verdict,omitempty"`
	BlindGradeFailures []string `json:"blind_grade_failures,omitempty"`
}

type ScreeningResultTotals struct {
	FreshInputTokens   int64 `json:"fresh_input_tokens"`
	CachedInputTokens  int64 `json:"cached_input_tokens"`
	OutputTokens       int64 `json:"output_tokens"`
	TotalTokens        int64 `json:"total_tokens"`
	Runs               int   `json:"runs"`
	Retries            int   `json:"retries"`
	OwnerInterventions int   `json:"owner_interventions"`
}

type ScreeningBatchResult struct {
	SchemaVersion         string                       `json:"schema_version"`
	BatchID               string                       `json:"batch_id"`
	Status                string                       `json:"status"`
	GraderConfigurationID string                       `json:"grader_configuration_id"`
	Capacity              ScreeningCapacityResult      `json:"capacity"`
	Observations          []ScreeningObservationResult `json:"observations"`
	Totals                ScreeningResultTotals        `json:"totals"`
	StopReason            string                       `json:"stop_reason,omitempty"`
	PlannedObservations   int                          `json:"planned_observations,omitempty"`
}

//go:embed screening_batch_v3_result.json
var screeningBatchV3ResultJSON []byte

//go:embed screening_batch_v4_result.json
var screeningBatchV4ResultJSON []byte

func CurrentScreeningResult() (ScreeningBatchResult, error) {
	var result ScreeningBatchResult
	if err := json.Unmarshal(screeningBatchV3ResultJSON, &result); err != nil {
		return result, fmt.Errorf("decode screening result: %w", err)
	}
	batch, err := ThirdScreeningBatch()
	if err != nil {
		return result, err
	}
	if err := ValidateScreeningResult(batch, result); err != nil {
		return result, err
	}
	return result, nil
}

func FourthScreeningResult() (ScreeningBatchResult, error) {
	var result ScreeningBatchResult
	if err := json.Unmarshal(screeningBatchV4ResultJSON, &result); err != nil {
		return result, fmt.Errorf("decode fourth screening result: %w", err)
	}
	batch, err := FourthScreeningBatch()
	if err != nil {
		return result, err
	}
	if err := ValidateScreeningResult(batch, result); err != nil {
		return result, err
	}
	return result, nil
}

func ValidateScreeningResult(batch ScreeningBatchDefinition, result ScreeningBatchResult) error {
	if result.SchemaVersion != ScreeningResultSchemaVersion || result.BatchID != batch.BatchID {
		return fmt.Errorf("screening result identity or status mismatch")
	}
	complete := result.Status == "complete_invalid_harness"
	stopped := result.Status == "stopped_invalid_harness"
	if !complete && !stopped {
		return fmt.Errorf("screening result identity or status mismatch")
	}
	if complete && (len(result.Observations) != len(batch.Candidates) || result.StopReason != "") {
		return fmt.Errorf("completed result observation count mismatch")
	}
	if stopped && (len(result.Observations) == 0 || len(result.Observations) >= len(batch.Candidates) || result.StopReason == "") {
		return fmt.Errorf("stopped result boundary mismatch")
	}
	if result.PlannedObservations != 0 && result.PlannedObservations != len(batch.Candidates) {
		return fmt.Errorf("screening result planned observation mismatch")
	}
	if result.GraderConfigurationID != batch.GraderConfigurationID {
		return fmt.Errorf("screening result candidate or grader mismatch")
	}
	if result.Capacity.Provider != batch.CapacityBudgets[0].Provider || result.Capacity.LimitID != batch.CapacityBudgets[0].LimitID || result.Capacity.AccountLane != batch.CapacityBudgets[0].AccountLane {
		return fmt.Errorf("screening result capacity lane mismatch")
	}
	if result.Capacity.FinalUsedPercent < result.Capacity.BaselineUsedPercent || (result.Capacity.FinalUsedPercent-result.Capacity.BaselineUsedPercent)*100 >= batch.CapacityBudgets[0].MaxUsedIncreaseBasisPoints {
		return fmt.Errorf("screening result reached capacity brake")
	}

	totals := ScreeningResultTotals{}
	for i, observation := range result.Observations {
		if observation.CandidateID != batch.Candidates[i].ID {
			return fmt.Errorf("observation %d candidate order mismatch", i)
		}
		if observation.IssueIdentifier == "" || observation.IssueID == "" || observation.TriggerCommentID == "" || observation.RunID == "" || observation.SessionID == "" {
			return fmt.Errorf("observation %d is missing durable receipt identity", i)
		}
		started, startErr := time.Parse(time.RFC3339Nano, observation.StartedAt)
		completed, completeErr := time.Parse(time.RFC3339Nano, observation.CompletedAt)
		if startErr != nil || completeErr != nil || completed.Before(started) {
			return fmt.Errorf("observation %d has invalid timing", i)
		}
		if hash, err := hex.DecodeString(observation.OutputSHA256); err != nil || len(hash) != 32 {
			return fmt.Errorf("observation %d has invalid output hash", i)
		}
		if observation.Verdict != "PASS" && observation.Verdict != "FAIL" && observation.Verdict != "INVALID" {
			return fmt.Errorf("observation %d has invalid verdict", i)
		}
		if observation.Verdict == "PASS" && len(observation.FailedChecks) != 0 || observation.Verdict != "PASS" && len(observation.FailedChecks) == 0 {
			return fmt.Errorf("observation %d verdict evidence mismatch", i)
		}
		if observation.InputTokens-observation.CachedInputTokens != observation.FreshInputTokens || observation.FreshInputTokens+observation.CachedInputTokens+observation.OutputTokens != observation.TotalTokens {
			return fmt.Errorf("observation %d token accounting mismatch", i)
		}
		if observation.Retries != 0 || observation.OwnerInterventions != 0 {
			return fmt.Errorf("observation %d exceeded bounded execution", i)
		}
		totals.FreshInputTokens += observation.FreshInputTokens
		totals.CachedInputTokens += observation.CachedInputTokens
		totals.OutputTokens += observation.OutputTokens
		totals.TotalTokens += observation.TotalTokens
		totals.Runs++
		totals.Retries += observation.Retries
		totals.OwnerInterventions += observation.OwnerInterventions
	}
	if totals != result.Totals || totals.OwnerInterventions > batch.MaxOwnerInterventions {
		return fmt.Errorf("screening result totals or ceilings mismatch")
	}
	if totals.TotalTokens > batch.TokenCeiling && result.StopReason != "token_ceiling_exceeded_and_execution_contract_violated" {
		return fmt.Errorf("screening result exceeded token ceiling without matching stop reason")
	}
	return nil
}
