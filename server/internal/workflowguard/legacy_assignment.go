// Package workflowguard defines the bounded compatibility contract used when
// the legacy workflow controller creates a provider-backed task.
package workflowguard

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/google/uuid"
)

const (
	LegacyAssignmentKey       = "workflow_assignment_v1"
	LegacyAssignmentVersion   = 1
	MaxLegacyAssignmentBytes  = 256 << 10
	MaxLegacyAssignmentSkills = 16
	LegacyEnvironmentPolicy   = "exact_allowlist_v1"
	LegacyToolPolicy          = "tool_free_v1"
	// LegacyBrokerToolPolicy is retained only so old serialized assignments fail
	// closed with an explicit unsupported policy. Broker authority now exists
	// exclusively in controller-owned qualified-runtime-binding.v2 receipts and
	// per-task grants; editable issue acceptance data grants nothing.
	LegacyBrokerToolPolicy       = "scoped_parent_broker_tool_free_child_v1"
	LegacyBrokerAcceptancePrefix = "bounded-workflow-broker-v1 "
	StaticPreviewKind            = "static"
	StaticPreviewArtifactRoot    = ".multica/preview"
	StaticPreviewPolicyID        = "synthetic-or-redacted-read-only-v1"
)

var bundleHashPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

type ContextPolicy struct {
	LoadIssue          bool `json:"load_issue"`
	LoadComments       bool `json:"load_comments"`
	LoadRepositoryDocs bool `json:"load_repository_docs"`
}

type ExecutionPolicy struct {
	EnvironmentPolicy      string `json:"environment_policy"`
	ToolPolicy             string `json:"tool_policy"`
	FreshTaskHome          bool   `json:"fresh_task_home"`
	NetworkAccess          bool   `json:"network_access"`
	BrokerExecutableSHA256 string `json:"broker_executable_sha256,omitempty"`
}

type RequiredSkill struct {
	ID        string `json:"id"`
	Source    string `json:"source"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"size_bytes"`
	FileCount int    `json:"file_count"`
}

// StaticPreviewDeclaration is controller-owned authority for the daemon's
// bounded, in-process preview producer. It intentionally contains no command,
// URL, or worker-selected executable field.
type StaticPreviewDeclaration struct {
	Kind              string `json:"kind"`
	ArtifactRoot      string `json:"artifact_root"`
	CandidateID       string `json:"candidate_id"`
	CandidateRevision int64  `json:"candidate_revision"`
	SourceRevision    string `json:"source_revision"`
	PolicyID          string `json:"policy_id"`
}

type LegacyAssignmentV1 struct {
	ContractVersion      int                       `json:"contract_version"`
	TaskKind             string                    `json:"task_kind"`
	IssueID              string                    `json:"issue_id"`
	IssueTitle           string                    `json:"issue_title"`
	IssueDescription     string                    `json:"issue_description"`
	AcceptanceCriteria   json.RawMessage           `json:"acceptance_criteria"`
	SourceTaskID         string                    `json:"source_task_id"`
	WorkflowEventID      string                    `json:"workflow_event_id"`
	WorkflowEventType    string                    `json:"workflow_event_type"`
	WorkflowEventReason  string                    `json:"workflow_event_reason"`
	WorkflowEventPayload json.RawMessage           `json:"workflow_event_payload"`
	TargetAgentID        string                    `json:"target_agent_id"`
	SquadID              string                    `json:"squad_id"`
	RuntimeID            string                    `json:"runtime_id"`
	RuntimeProvider      string                    `json:"runtime_provider"`
	Model                string                    `json:"model"`
	ReasoningEffort      string                    `json:"reasoning_effort"`
	ContextPolicy        ContextPolicy             `json:"context_policy"`
	ExecutionPolicy      ExecutionPolicy           `json:"execution_policy"`
	RequiredSkills       []RequiredSkill           `json:"required_skills"`
	Preview              *StaticPreviewDeclaration `json:"preview,omitempty"`
}

func (assignment LegacyAssignmentV1) Validate() error {
	if assignment.ContractVersion != LegacyAssignmentVersion {
		return errors.New("unsupported workflow assignment contract version")
	}
	switch assignment.TaskKind {
	case "implementation", "review", "correction", "release", "verification", "owner_resume":
	default:
		return fmt.Errorf("unsupported workflow task kind %q", assignment.TaskKind)
	}
	for name, value := range map[string]string{
		"issue_id": assignment.IssueID, "issue_title": assignment.IssueTitle,
		"source_task_id": assignment.SourceTaskID, "workflow_event_id": assignment.WorkflowEventID,
		"workflow_event_type": assignment.WorkflowEventType, "workflow_event_reason": assignment.WorkflowEventReason,
		"target_agent_id": assignment.TargetAgentID, "squad_id": assignment.SquadID,
		"runtime_id": assignment.RuntimeID, "runtime_provider": assignment.RuntimeProvider,
		"model": assignment.Model, "reasoning_effort": assignment.ReasoningEffort,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("workflow assignment %s is required", name)
		}
	}
	if assignment.ContextPolicy.LoadIssue || assignment.ContextPolicy.LoadComments || assignment.ContextPolicy.LoadRepositoryDocs {
		return errors.New("workflow assignment context policy must deny ambient corpus loading")
	}
	if assignment.ExecutionPolicy.EnvironmentPolicy != LegacyEnvironmentPolicy ||
		!assignment.ExecutionPolicy.FreshTaskHome || assignment.ExecutionPolicy.NetworkAccess {
		return errors.New("workflow assignment must require the isolated execution policy")
	}
	switch assignment.ExecutionPolicy.ToolPolicy {
	case LegacyToolPolicy:
		if assignment.ExecutionPolicy.BrokerExecutableSHA256 != "" {
			return errors.New("tool-free workflow assignment cannot bind a broker executable")
		}
	case LegacyBrokerToolPolicy:
		return errors.New("legacy workflow broker authority is retired")
	default:
		return errors.New("workflow assignment has an unsupported tool policy")
	}
	if !validObjectOrArray(assignment.AcceptanceCriteria) || !validObject(assignment.WorkflowEventPayload) {
		return errors.New("workflow assignment contains invalid structured evidence")
	}
	if assignment.Preview != nil {
		if assignment.TaskKind != "correction" ||
			assignment.Preview.Kind != StaticPreviewKind ||
			assignment.Preview.ArtifactRoot != StaticPreviewArtifactRoot ||
			assignment.Preview.PolicyID != StaticPreviewPolicyID ||
			assignment.Preview.CandidateRevision != 1 {
			return errors.New("workflow assignment contains an invalid static preview declaration")
		}
		parsedCandidateID, err := uuid.Parse(assignment.Preview.CandidateID)
		if err != nil || parsedCandidateID.String() != assignment.Preview.CandidateID {
			return errors.New("workflow assignment preview candidate id is invalid")
		}
		if strings.TrimSpace(assignment.Preview.SourceRevision) == "" ||
			len(assignment.Preview.SourceRevision) > 256 ||
			strings.ContainsAny(assignment.Preview.SourceRevision, "\r\n\x00") {
			return errors.New("workflow assignment preview source revision is invalid")
		}
	}
	if len(assignment.RequiredSkills) == 0 || len(assignment.RequiredSkills) > MaxLegacyAssignmentSkills {
		return fmt.Errorf("workflow assignment must bind between 1 and %d skills", MaxLegacyAssignmentSkills)
	}
	seen := make(map[string]struct{}, len(assignment.RequiredSkills))
	for _, skill := range assignment.RequiredSkills {
		key := skill.Source + "\x00" + skill.ID
		if strings.TrimSpace(skill.ID) == "" || strings.TrimSpace(skill.Source) == "" || !bundleHashPattern.MatchString(skill.SHA256) || skill.SizeBytes < 0 || skill.FileCount < 0 {
			return errors.New("workflow assignment contains an invalid skill binding")
		}
		if _, duplicate := seen[key]; duplicate {
			return errors.New("workflow assignment contains a duplicate skill binding")
		}
		seen[key] = struct{}{}
	}
	return nil
}

// LegacyExecutionPolicyForAcceptance always returns tool-free policy. Issue
// acceptance criteria are user-editable requirements, never process-launch
// authority. The parameter remains for wire/backward compatibility.
func LegacyExecutionPolicyForAcceptance(acceptanceCriteria json.RawMessage) ExecutionPolicy {
	_ = acceptanceCriteria
	return ExecutionPolicy{
		EnvironmentPolicy: LegacyEnvironmentPolicy,
		ToolPolicy:        LegacyToolPolicy,
		FreshTaskHome:     true,
	}
}

// MatchesAuthoritativeSnapshot compares every controller-owned assignment
// field other than RequiredSkills. Skills are validated against their current
// byte snapshots at claim; this comparison binds all remaining prompt content
// to authoritative issue, event, plan, and task rows.
func MatchesAuthoritativeSnapshot(actual, expected LegacyAssignmentV1) bool {
	return actual.ContractVersion == expected.ContractVersion &&
		actual.TaskKind == expected.TaskKind &&
		actual.IssueID == expected.IssueID &&
		actual.IssueTitle == expected.IssueTitle &&
		actual.IssueDescription == expected.IssueDescription &&
		jsonSemanticallyEqual(actual.AcceptanceCriteria, expected.AcceptanceCriteria) &&
		actual.SourceTaskID == expected.SourceTaskID &&
		actual.WorkflowEventID == expected.WorkflowEventID &&
		actual.WorkflowEventType == expected.WorkflowEventType &&
		reflect.DeepEqual(actual.Preview, expected.Preview) &&
		actual.WorkflowEventReason == expected.WorkflowEventReason &&
		jsonSemanticallyEqual(actual.WorkflowEventPayload, expected.WorkflowEventPayload) &&
		actual.TargetAgentID == expected.TargetAgentID &&
		actual.SquadID == expected.SquadID &&
		actual.RuntimeID == expected.RuntimeID &&
		actual.RuntimeProvider == expected.RuntimeProvider &&
		actual.Model == expected.Model &&
		actual.ReasoningEffort == expected.ReasoningEffort &&
		actual.ContextPolicy == expected.ContextPolicy &&
		actual.ExecutionPolicy == expected.ExecutionPolicy
}

func WrapLegacyAssignment(assignment LegacyAssignmentV1) ([]byte, error) {
	if err := assignment.Validate(); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(map[string]any{LegacyAssignmentKey: assignment})
	if err != nil {
		return nil, err
	}
	if len(raw) > MaxLegacyAssignmentBytes {
		return nil, fmt.Errorf("workflow assignment exceeds %d bytes", MaxLegacyAssignmentBytes)
	}
	return raw, nil
}

func ExtractLegacyAssignment(contextJSON []byte) (json.RawMessage, LegacyAssignmentV1, bool, error) {
	if len(bytes.TrimSpace(contextJSON)) == 0 {
		return nil, LegacyAssignmentV1{}, false, nil
	}
	if len(contextJSON) > MaxLegacyAssignmentBytes {
		return nil, LegacyAssignmentV1{}, true, fmt.Errorf("workflow assignment context exceeds %d bytes", MaxLegacyAssignmentBytes)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(contextJSON, &envelope); err != nil {
		return nil, LegacyAssignmentV1{}, false, nil
	}
	raw, present := envelope[LegacyAssignmentKey]
	if !present {
		return nil, LegacyAssignmentV1{}, false, nil
	}
	var assignment LegacyAssignmentV1
	if err := json.Unmarshal(raw, &assignment); err != nil {
		return nil, LegacyAssignmentV1{}, true, fmt.Errorf("decode workflow assignment: %w", err)
	}
	if err := assignment.Validate(); err != nil {
		return nil, LegacyAssignmentV1{}, true, err
	}
	return append(json.RawMessage(nil), raw...), assignment, true, nil
}

func MatchRequiredSkills(required []RequiredSkill, actual []RequiredSkill) bool {
	if len(required) != len(actual) {
		return false
	}
	canonical := func(values []RequiredSkill) []RequiredSkill {
		result := append([]RequiredSkill(nil), values...)
		sort.Slice(result, func(i, j int) bool {
			if result[i].Source != result[j].Source {
				return result[i].Source < result[j].Source
			}
			return result[i].ID < result[j].ID
		})
		return result
	}
	want, got := canonical(required), canonical(actual)
	for i := range want {
		if want[i] != got[i] {
			return false
		}
	}
	return true
}

func validObject(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) >= 2 && trimmed[0] == '{' && json.Valid(trimmed)
}

func validObjectOrArray(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) >= 2 && (trimmed[0] == '{' || trimmed[0] == '[') && json.Valid(trimmed)
}

func jsonSemanticallyEqual(left, right json.RawMessage) bool {
	var leftValue, rightValue any
	if json.Unmarshal(left, &leftValue) != nil || json.Unmarshal(right, &rightValue) != nil {
		return false
	}
	leftCanonical, leftErr := json.Marshal(leftValue)
	rightCanonical, rightErr := json.Marshal(rightValue)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftCanonical, rightCanonical)
}
