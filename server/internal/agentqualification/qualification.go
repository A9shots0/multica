package agentqualification

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

type Case struct {
	ID                     string   `json:"id"`
	Name                   string   `json:"name"`
	TargetFunctions        []string `json:"target_functions"`
	RequiredCapabilities   []string `json:"required_capabilities"`
	RequiredEvidence       []string `json:"required_evidence"`
	AutomaticDisqualifiers []string `json:"automatic_disqualifiers"`
	Mandatory              bool     `json:"mandatory"`
}

type Configuration struct {
	ID                 string            `json:"id"`
	RoleID             string            `json:"role_id"`
	Model              string            `json:"model"`
	ReasoningEffort    string            `json:"reasoning_effort"`
	InstructionVersion string            `json:"instruction_version"`
	InstructionSHA256  string            `json:"instruction_sha256,omitempty"`
	SkillVersions      map[string]string `json:"skill_versions"`
	SkillContentSHA256 map[string]string `json:"skill_content_sha256,omitempty"`
	PermissionVersion  string            `json:"permission_version"`
	RuntimeProvider    string            `json:"runtime_provider"`
	ServingProvider    string            `json:"serving_provider,omitempty"`
	ProviderFallbacks  *bool             `json:"provider_fallbacks,omitempty"`
}

type Trial struct {
	ConfigurationID  string            `json:"configuration_id"`
	CaseID           string            `json:"case_id"`
	Attempt          int               `json:"attempt"`
	OutcomePass      bool              `json:"outcome_pass"`
	SafetyPass       bool              `json:"safety_pass"`
	EvidencePass     bool              `json:"evidence_pass"`
	IndependencePass bool              `json:"independence_pass"`
	ObservedEvidence []string          `json:"observed_evidence"`
	Violations       []string          `json:"violations,omitempty"`
	Observation      ObservationRecord `json:"observation"`
}

type TrialVerdict struct {
	Qualified   bool     `json:"qualified"`
	Reasons     []string `json:"reasons"`
	TotalTokens int64    `json:"total_tokens"`
}

type CandidateResult struct {
	Configuration      Configuration `json:"configuration"`
	Qualified          bool          `json:"qualified"`
	Reasons            []string      `json:"reasons,omitempty"`
	CasesPassed        int           `json:"cases_passed"`
	CasesRequired      int           `json:"cases_required"`
	TotalTokens        int64         `json:"total_tokens"`
	FreshInputTokens   int64         `json:"fresh_input_tokens"`
	CachedInputTokens  int64         `json:"cached_input_tokens"`
	OutputTokens       int64         `json:"output_tokens"`
	RunComputeMillis   int64         `json:"run_compute_millis"`
	EndToEndMillis     int64         `json:"end_to_end_millis"`
	Runs               int           `json:"runs"`
	OwnerInterruptions int           `json:"owner_interruptions"`
	Retries            int           `json:"retries"`
}

//go:embed cases.json
var casesJSON []byte

var (
	casesOnce sync.Once
	cases     []Case
	casesErr  error
)

func Cases() ([]Case, error) {
	casesOnce.Do(func() {
		if err := json.Unmarshal(casesJSON, &cases); err != nil {
			casesErr = fmt.Errorf("decode qualification cases: %w", err)
			return
		}
		seen := map[string]struct{}{}
		for _, c := range cases {
			if c.ID == "" || c.Name == "" || len(c.RequiredEvidence) == 0 || len(c.AutomaticDisqualifiers) == 0 {
				casesErr = fmt.Errorf("incomplete qualification case %+v", c)
				return
			}
			if _, exists := seen[c.ID]; exists {
				casesErr = fmt.Errorf("duplicate qualification case %s", c.ID)
				return
			}
			seen[c.ID] = struct{}{}
		}
	})
	return append([]Case(nil), cases...), casesErr
}

func ValidateConfiguration(c Configuration) error {
	if c.ID == "" || c.RoleID == "" || c.Model == "" || c.InstructionVersion == "" || c.PermissionVersion == "" || c.RuntimeProvider == "" {
		return fmt.Errorf("configuration must name role, model, instructions, permissions, and runtime")
	}
	if len(c.SkillVersions) == 0 {
		return fmt.Errorf("configuration must pin at least one skill version")
	}
	return nil
}

func EvaluateTrial(c Case, trial Trial) TrialVerdict {
	validity := ValidateObservation(trial.Observation)
	verdict := TrialVerdict{Qualified: true}
	if validity.Valid {
		verdict.TotalTokens = validity.Totals.FreshInputTokens + validity.Totals.CachedInputTokens + validity.Totals.OutputTokens
	} else {
		for _, reason := range validity.Reasons {
			verdict.Reasons = append(verdict.Reasons, "invalid_observation:"+reason)
		}
	}
	if trial.Observation.CaseID != c.ID || trial.CaseID != c.ID {
		verdict.Reasons = append(verdict.Reasons, "observation_case_mismatch")
	}
	if trial.Observation.CandidateID == "" || trial.ConfigurationID == "" {
		verdict.Reasons = append(verdict.Reasons, "observation_candidate_missing")
	}
	if trial.Attempt < 1 {
		verdict.Reasons = append(verdict.Reasons, "attempt_missing")
	}
	if trial.Observation.TrialStatus != "passed" {
		verdict.Reasons = append(verdict.Reasons, "blind_grade_"+trial.Observation.TrialStatus)
	}
	for _, failedCheck := range trial.Observation.BlindGradeFailedChecks {
		verdict.Reasons = append(verdict.Reasons, "blind_grade_failed_check:"+failedCheck)
	}
	for _, disqualifier := range trial.Observation.Disqualifiers {
		verdict.Reasons = append(verdict.Reasons, "automatic_disqualifier:"+disqualifier)
	}
	if !trial.OutcomePass {
		verdict.Reasons = append(verdict.Reasons, "outcome_failed")
	}
	if !trial.SafetyPass {
		verdict.Reasons = append(verdict.Reasons, "safety_failed")
	}
	if !trial.EvidencePass {
		verdict.Reasons = append(verdict.Reasons, "evidence_failed")
	}
	if !trial.IndependencePass {
		verdict.Reasons = append(verdict.Reasons, "independence_failed")
	}
	observed := make(map[string]struct{}, len(trial.ObservedEvidence))
	for _, item := range trial.ObservedEvidence {
		observed[item] = struct{}{}
	}
	for _, required := range c.RequiredEvidence {
		if _, ok := observed[required]; !ok {
			verdict.Reasons = append(verdict.Reasons, "missing_evidence:"+required)
		}
	}
	for _, violation := range trial.Violations {
		for _, disqualifier := range c.AutomaticDisqualifiers {
			if violation == disqualifier {
				verdict.Reasons = append(verdict.Reasons, "automatic_disqualifier:"+violation)
			}
		}
	}
	verdict.Reasons = unique(verdict.Reasons)
	verdict.Qualified = len(verdict.Reasons) == 0
	return verdict
}

func EvaluateConfiguration(configuration Configuration, trials []Trial, requiredRepeats int) CandidateResult {
	result := CandidateResult{Configuration: configuration}
	if err := ValidateConfiguration(configuration); err != nil {
		result.Reasons = []string{err.Error()}
		return result
	}
	if requiredRepeats < 1 {
		requiredRepeats = 1
	}
	allCases, err := Cases()
	if err != nil {
		result.Reasons = []string{err.Error()}
		return result
	}
	applicable := applicableCases(configuration.RoleID, allCases)
	trialByCase := make(map[string][]Trial)
	seenObservations := make(map[string]struct{})
	seenAttempts := make(map[string]struct{})
	for _, trial := range trials {
		if trial.ConfigurationID != configuration.ID {
			continue
		}
		observationID := trial.Observation.ObservationID
		if observationID != "" {
			if _, duplicate := seenObservations[observationID]; duplicate {
				result.Reasons = append(result.Reasons, trial.CaseID+":duplicate_observation:"+observationID)
				continue
			}
			seenObservations[observationID] = struct{}{}
		}
		// Cost accounting is independent of outcome validity. Once a paid run has
		// complete usage, failed and invalid work still counts against the candidate.
		measured, complete := measuredObservationTotals(trial.Observation)
		if complete {
			result.FreshInputTokens += measured.FreshInputTokens
			result.CachedInputTokens += measured.CachedInputTokens
			result.OutputTokens += measured.OutputTokens
			result.RunComputeMillis += measured.RunComputeMillis
			result.EndToEndMillis += measured.EndToEndMillis
			result.Runs += measured.Runs
			result.OwnerInterruptions += measured.OwnerInterventions
			result.Retries += measured.Retries
		} else {
			result.Reasons = append(result.Reasons, trial.CaseID+":usage_incomplete")
		}

		attemptKey := fmt.Sprintf("%s:%s:%d", trial.ConfigurationID, trial.CaseID, trial.Attempt)
		if _, duplicate := seenAttempts[attemptKey]; duplicate {
			result.Reasons = append(result.Reasons, trial.CaseID+":duplicate_attempt:"+attemptKey)
			continue
		}
		seenAttempts[attemptKey] = struct{}{}
		trialByCase[trial.CaseID] = append(trialByCase[trial.CaseID], trial)

		validity := ValidateObservation(trial.Observation)
		if validity.Valid && !observationContainsConfiguration(trial.Observation, configuration) {
			result.Reasons = append(result.Reasons, trial.CaseID+":configuration_snapshot_mismatch")
		}
	}
	result.TotalTokens = result.FreshInputTokens + result.CachedInputTokens + result.OutputTokens
	for _, c := range applicable {
		if !c.Mandatory {
			continue
		}
		result.CasesRequired++
		caseTrials := trialByCase[c.ID]
		passes := 0
		for _, trial := range caseTrials {
			verdict := EvaluateTrial(c, trial)
			if verdict.Qualified {
				passes++
			} else {
				for _, reason := range verdict.Reasons {
					if len(reason) > len("automatic_disqualifier:") && reason[:len("automatic_disqualifier:")] == "automatic_disqualifier:" {
						result.Reasons = append(result.Reasons, c.ID+":"+reason)
					}
				}
			}
		}
		if passes >= requiredRepeats {
			result.CasesPassed++
		} else {
			result.Reasons = append(result.Reasons, fmt.Sprintf("%s:passes_%d_of_%d", c.ID, passes, requiredRepeats))
		}
	}
	result.Reasons = unique(result.Reasons)
	result.Qualified = result.CasesRequired > 0 && result.CasesPassed == result.CasesRequired && len(result.Reasons) == 0
	return result
}

func measuredObservationTotals(observation ObservationRecord) (ObservationTotals, bool) {
	totals := ObservationTotals{}
	started, startErr := time.Parse(time.RFC3339Nano, observation.StartedAt)
	completed, completeErr := time.Parse(time.RFC3339Nano, observation.CompletedAt)
	if startErr != nil || completeErr != nil || completed.Before(started) {
		return totals, false
	}
	totals.EndToEndMillis = completed.Sub(started).Milliseconds()
	seenRuns := make(map[string]struct{}, len(observation.RunReceipts))
	for _, receipt := range observation.RunReceipts {
		if receipt.RunID == "" {
			return ObservationTotals{}, false
		}
		if _, duplicate := seenRuns[receipt.RunID]; duplicate {
			return ObservationTotals{}, false
		}
		seenRuns[receipt.RunID] = struct{}{}
		values := []*int64{receipt.Usage.FreshInputTokens, receipt.Usage.CachedInputTokens, receipt.Usage.OutputTokens, receipt.Usage.WallTimeMillis}
		for _, value := range values {
			if value == nil || *value < 0 {
				return ObservationTotals{}, false
			}
		}
		totals.FreshInputTokens += *receipt.Usage.FreshInputTokens
		totals.CachedInputTokens += *receipt.Usage.CachedInputTokens
		totals.OutputTokens += *receipt.Usage.OutputTokens
		totals.RunComputeMillis += *receipt.Usage.WallTimeMillis
		totals.OwnerInterventions += receipt.OwnerInterventions
		totals.Runs++
		if receipt.Retry {
			totals.Retries++
		}
	}
	return totals, true
}

func observationContainsConfiguration(observation ObservationRecord, expected Configuration) bool {
	wantHash, err := ConfigurationSHA256(expected)
	if err != nil {
		return false
	}
	for _, configuration := range observation.Configurations {
		if configuration.ID != expected.ID {
			continue
		}
		gotHash, err := ConfigurationSHA256(configuration)
		return err == nil && gotHash == wantHash
	}
	return false
}

func applicableCases(roleID string, all []Case) []Case {
	roleFunctions := map[string][]string{
		"mika_owner_interface":             {"owner_interface"},
		"shared_windows_executor":          {"primary_executor"},
		"phonefarm_application_specialist": {"phonefarm_application_specialist"},
		"phone_fleet_reliability":          {"phone_fleet_reliability"},
		"data_sync_integrity":              {"data_sync_integrity"},
		"product_ui":                       {"product_ui"},
		"release_runtime_assurance":        {"release_runtime_assurance"},
		"quality_checker":                  {"quality_checker"},
		"architecture_assurance":           {"architecture_assurance"},
		"security_privacy_review":          {"security_privacy_review"},
	}
	roleCapabilities := map[string][]string{
		"crm_access_outcome":          {"identity_access_authority"},
		"linkhub_domains_outcome":     {"linkhub_customer_journey"},
		"media_pipeline_outcome":      {"media_availability"},
		"content_inspiration_outcome": {"content_supply"},
		"telnyx_messaging_outcome":    {"telnyx_messaging"},
		"canopy_publishing_outcome":   {"outreach_conversation"},
		"reporting_outcome":           {"reporting_client_communication"},
		"crm_product_outcome":         {"crm_product_execution"},
		"canopy_accounts_outcome":     {"canopy_account_connection"},
		"canopy_desktop_outcome":      {"canopy_desktop_experience"},
	}
	functions := roleFunctions[roleID]
	capabilities := roleCapabilities[roleID]
	var result []Case
	for _, c := range all {
		matched := false
		for _, function := range functions {
			if contains(c.TargetFunctions, function) {
				matched = true
				break
			}
		}
		if !matched {
			for _, capability := range capabilities {
				if contains(c.RequiredCapabilities, capability) {
					matched = true
					break
				}
			}
		}
		if matched {
			result = append(result, c)
		}
	}
	return result
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func RankQualified(results []CandidateResult) []CandidateResult {
	var qualified []CandidateResult
	for _, result := range results {
		if result.Qualified {
			qualified = append(qualified, result)
		}
	}
	sort.SliceStable(qualified, func(i, j int) bool {
		if qualified[i].OwnerInterruptions != qualified[j].OwnerInterruptions {
			return qualified[i].OwnerInterruptions < qualified[j].OwnerInterruptions
		}
		if qualified[i].Retries != qualified[j].Retries {
			return qualified[i].Retries < qualified[j].Retries
		}
		return qualified[i].TotalTokens < qualified[j].TotalTokens
	})
	return qualified
}

func unique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		seen[value] = struct{}{}
	}
	result := make([]string, 0, len(seen))
	for value := range seen {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
