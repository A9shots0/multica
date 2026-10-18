package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"slices"
	"time"
)

// NativePublicationAcceptanceV1Schema names the immutable transcription of the
// one accepted native publisher -> independent reviewer run. It records what the
// run proved and, just as importantly, what it did not.
const NativePublicationAcceptanceV1Schema = "silver-mediums.native-publication-acceptance.v1"

type NativePublicationAcceptanceV1 struct {
	SchemaVersion                           string   `json:"schema_version"`
	AcceptanceID                            string   `json:"acceptance_id"`
	Status                                  string   `json:"status"`
	AcceptedAt                              string   `json:"accepted_at"`
	ReleaseSourceID                         string   `json:"release_source_id"`
	BackendImage                            string   `json:"backend_image"`
	IssueID                                 string   `json:"issue_id"`
	IssueNumber                             int      `json:"issue_number"`
	IssueStatus                             string   `json:"issue_status"`
	IssueRevision                           int      `json:"issue_revision"`
	PublisherTurns                          int      `json:"publisher_turns"`
	ReviewerTurns                           int      `json:"reviewer_turns"`
	Retries                                 int      `json:"retries"`
	Corrections                             int      `json:"corrections"`
	FailedTasksOnIssue                      int      `json:"failed_tasks_on_issue"`
	OperatorAssistanceRequired              bool     `json:"operator_assistance_required"`
	ReviewerRoutedByWorkflow                bool     `json:"reviewer_routed_by_workflow"`
	ProviderChildToolsUsed                  bool     `json:"provider_child_tools_used"`
	WorkflowEvents                          []string `json:"workflow_events"`
	ConsistentSurfaces                      []string `json:"consistent_surfaces"`
	EvidenceKind                            string   `json:"evidence_kind"`
	TransportLane                           string   `json:"transport_lane"`
	RuntimeProvider                         string   `json:"runtime_provider"`
	RequestedPrimaryModel                   string   `json:"requested_primary_model"`
	ObservedModels                          []string `json:"observed_models"`
	ExactSingleModelProven                  bool     `json:"exact_single_model_proven"`
	ReviewerWasDistinctSession              bool     `json:"reviewer_was_distinct_session"`
	ReviewerWasDistinctProvider             bool     `json:"reviewer_was_distinct_provider"`
	RecoverySensitiveAuthorityDemonstrated  bool     `json:"recovery_sensitive_authority_demonstrated"`
	TemporaryBindingsRevoked                bool     `json:"temporary_bindings_revoked"`
	AuthorizesPermanentRoleOrModelSelection bool     `json:"authorizes_permanent_role_or_model_selection"`
	AuthorizesUnrestrictedAutomaticDispatch bool     `json:"authorizes_unrestricted_automatic_dispatch"`
	AuthorizesPublicExposure                bool     `json:"authorizes_public_exposure"`
	AuthorizesFurtherProviderRun            bool     `json:"authorizes_further_provider_run"`
	EvidencePath                            string   `json:"evidence_path"`
	ReportPath                              string   `json:"report_path"`
}

//go:embed native_publication_acceptance_v1.json
var nativePublicationAcceptanceV1JSON []byte

func CurrentNativePublicationAcceptanceV1() (NativePublicationAcceptanceV1, error) {
	var acceptance NativePublicationAcceptanceV1
	if err := strictJSON(nativePublicationAcceptanceV1JSON, &acceptance); err != nil {
		return acceptance, fmt.Errorf("decode native publication acceptance v1: %w", err)
	}
	return acceptance, ValidateNativePublicationAcceptanceV1(acceptance)
}

func NativePublicationAcceptanceV1SHA256() string {
	sum := sha256.Sum256(nativePublicationAcceptanceV1JSON)
	return hex.EncodeToString(sum[:])
}

func ValidateNativePublicationAcceptanceV1(acceptance NativePublicationAcceptanceV1) error {
	if acceptance.SchemaVersion != NativePublicationAcceptanceV1Schema || acceptance.AcceptanceID != "native-publication-acceptance-20260916" || acceptance.Status != "accepted" {
		return fmt.Errorf("native publication acceptance v1 identity drifted")
	}
	if _, err := time.Parse(time.RFC3339, acceptance.AcceptedAt); err != nil {
		return fmt.Errorf("parse native publication acceptance v1 accepted_at: %w", err)
	}
	if acceptance.ReleaseSourceID != "60fea34b9770-local-9eaa711d97b1" || acceptance.BackendImage != "sha256:e710767a6769ae581c9ff94978246a03f815f6afead02cfb9d5a7b560a878801" {
		return fmt.Errorf("native publication acceptance v1 release binding drifted")
	}
	if acceptance.IssueID != "01a0a7a4-2415-7ada-9a47-667a5bc6a094" || acceptance.IssueNumber != 40 || acceptance.IssueStatus != "done" || acceptance.IssueRevision != 7 {
		return fmt.Errorf("native publication acceptance v1 issue binding drifted")
	}
	if acceptance.PublisherTurns != 1 || acceptance.ReviewerTurns != 1 || acceptance.Retries != 0 || acceptance.Corrections != 0 || acceptance.FailedTasksOnIssue != 0 {
		return fmt.Errorf("native publication acceptance v1 run shape drifted")
	}
	if acceptance.OperatorAssistanceRequired || !acceptance.ReviewerRoutedByWorkflow || acceptance.ProviderChildToolsUsed {
		return fmt.Errorf("native publication acceptance v1 automation claim drifted")
	}
	if !slices.Equal(acceptance.WorkflowEvents, []string{"implementation_ready", "review_passed"}) {
		return fmt.Errorf("native publication acceptance v1 workflow event chain drifted")
	}
	if !slices.Equal(acceptance.ConsistentSurfaces, []string{"issue", "current_picture", "now", "inbox", "board"}) {
		return fmt.Errorf("native publication acceptance v1 surface coverage drifted")
	}
	if acceptance.EvidenceKind != "api_and_projection_evidence_not_rendered_screenshot" {
		return fmt.Errorf("native publication acceptance v1 evidence boundary drifted")
	}
	if acceptance.TransportLane != "claude_code_subscription_composite" || acceptance.RuntimeProvider != "claude" || acceptance.RequestedPrimaryModel != "claude-sonnet-4-6" || !slices.Equal(acceptance.ObservedModels, []string{"claude-haiku-4-5-20251001", "claude-sonnet-4-6"}) || acceptance.ExactSingleModelProven {
		return fmt.Errorf("native publication acceptance v1 transport binding drifted")
	}
	// The run proved a distinct reviewer session, not a distinct provider, and it
	// never exercised recovery-sensitive authority. Those limits are part of the
	// record, so a later configuration cannot quietly read more into it.
	if !acceptance.ReviewerWasDistinctSession || acceptance.ReviewerWasDistinctProvider || acceptance.RecoverySensitiveAuthorityDemonstrated {
		return fmt.Errorf("native publication acceptance v1 independence claim drifted")
	}
	if !acceptance.TemporaryBindingsRevoked {
		return fmt.Errorf("native publication acceptance v1 cleanup claim drifted")
	}
	if acceptance.AuthorizesPermanentRoleOrModelSelection || acceptance.AuthorizesUnrestrictedAutomaticDispatch || acceptance.AuthorizesPublicExposure || acceptance.AuthorizesFurtherProviderRun {
		return fmt.Errorf("native publication acceptance v1 expanded authority")
	}
	if acceptance.EvidencePath != ".qualification-evidence/native-acceptance-v3-20260915/" || acceptance.ReportPath != "docs/silver-mediums/NATIVE-ACCEPTANCE-RESULT-20260916.md" {
		return fmt.Errorf("native publication acceptance v1 evidence path drifted")
	}
	return nil
}
