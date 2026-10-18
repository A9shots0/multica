package agentqualification

import (
	"fmt"
	"strings"
)

const R02RepeatRunRecordSchemaVersion = "silver-mediums.realistic-r02-repeat-run-record.v1"

type R02RepeatRunRecord struct {
	SchemaVersion string               `json:"schema_version"`
	AdmissionID   string               `json:"admission_id"`
	AdmissionHash string               `json:"admission_sha256"`
	Outcome       string               `json:"outcome"`
	FailureClass  string               `json:"failure_class,omitempty"`
	Observation   RealisticObservation `json:"observation"`
}

// ClassifyR02RepeatController maps the older controller vocabulary onto the
// generation-owned stop policy. It intentionally never lets an ordinary model
// failure halt unrelated work and never treats transport as model competence.
func ClassifyR02RepeatController(result RealisticControllerResult, executionErr error) (string, string) {
	if result.TerminalState == "completed" {
		return "completed_pending_grade", ""
	}
	switch result.FailureClass {
	case "budget_breach":
		return "global_halt", "uncontrolled_spending"
	case "evidence_integrity_failure":
		return "global_halt", "evidence_corruption"
	case "unsafe_external_action":
		return "global_halt", "unsafe_external_action"
	case "provider_transport_failure":
		return "invalid", "provider_transport_imperfection"
	case "test_invalidity":
		message := strings.ToLower(errorStringR02Repeat(executionErr))
		if strings.Contains(message, "credential") || strings.Contains(message, "api key") || strings.Contains(message, "unauthorized") || strings.Contains(message, "authentication") {
			return "global_halt", "credential_failure"
		}
		return "invalid", "non_safety_harness_imperfection"
	case "response_or_tool_protocol_failure":
		return "candidate_fail", "response_or_tool_protocol_failure"
	case "competence_failure", "hard_disqualifier":
		return "candidate_fail", result.FailureClass
	default:
		return "invalid", "non_safety_harness_imperfection"
	}
}

func ValidateR02RepeatRunRecord(admission R02RepeatAdmission, record R02RepeatRunRecord, controller RealisticControllerResult) error {
	if err := ValidateR02RepeatAdmissionContract(admission); err != nil {
		return err
	}
	if record.SchemaVersion != R02RepeatRunRecordSchemaVersion || record.AdmissionID != admission.AdmissionID || !validSHA256(record.AdmissionHash) || record.Observation.ConfigurationID != admission.ConfigurationID || record.Observation.CaseID != admission.CaseID {
		return fmt.Errorf("R02 repeat run identity is invalid")
	}
	if controller.ProviderCalls != record.Observation.ProviderCalls || controller.InputTokens+controller.OutputTokens+controller.ReasoningTokens != record.Observation.TotalTokens || controller.CostMicros != record.Observation.CostMicros {
		return fmt.Errorf("R02 repeat run accounting disagrees with controller")
	}
	if record.Observation.TotalTokens > admission.MaxTotalTokens || controller.OutputTokens+controller.ReasoningTokens > admission.MaxGeneratedTokens || record.Observation.CostMicros > admission.MaxSpendMicros || len(controller.ToolEvents) > admission.MaxActions {
		if record.Outcome != "global_halt" || record.FailureClass != "uncontrolled_spending" {
			return fmt.Errorf("R02 repeat budget breach was not globally halted")
		}
	}
	switch record.Outcome {
	case "pass":
		if record.FailureClass != "" || !validSHA256(record.Observation.ExecutionReceiptSHA256) || !validSHA256(record.Observation.GradeSHA256) {
			return fmt.Errorf("R02 repeat pass is not independently graded and sealed")
		}
	case "candidate_fail":
		if !contains(admission.CandidateLocalFailureReasons, record.FailureClass) {
			return fmt.Errorf("R02 repeat candidate failure escaped local policy")
		}
	case "invalid":
		if !contains(admission.ObservationLocalInvalidReasons, record.FailureClass) {
			return fmt.Errorf("R02 repeat invalid observation escaped local policy")
		}
	case "global_halt":
		if !contains(admission.GlobalHaltReasons, record.FailureClass) {
			return fmt.Errorf("R02 repeat global halt reason is not authorized")
		}
	default:
		return fmt.Errorf("R02 repeat outcome is invalid")
	}
	return nil
}

func errorStringR02Repeat(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
