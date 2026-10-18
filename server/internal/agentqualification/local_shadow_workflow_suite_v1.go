package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/multica-ai/multica/server/internal/systemgraph"
)

const (
	LocalShadowWorkflowSuiteSchemaV1 = "silver-mediums.local-shadow-workflow-suite.v1"
	LocalShadowFakeRunSchemaV1       = "silver-mediums.local-shadow-fake-run.v1"
	localShadowSuiteIDV1             = "item-14-complete-local-shadow-suite-v1"
	localShadowSuiteStatusV1         = "prepared_frozen_fake_only"
	localShadowExecutionModeV1       = "deterministic_fake_provider_free"
	localShadowHostV1                = "alienware_windows"
)

type LocalShadowWorkflowSuiteV1 struct {
	SchemaVersion           string                          `json:"schema_version"`
	SuiteID                 string                          `json:"suite_id"`
	Status                  string                          `json:"status"`
	ExecutionMode           string                          `json:"execution_mode"`
	ProviderCallsAllowed    bool                            `json:"provider_calls_allowed"`
	CredentialsAllowed      bool                            `json:"credentials_allowed"`
	ToolsAllowed            bool                            `json:"tools_allowed"`
	RepositoryAccessAllowed bool                            `json:"repository_access_allowed"`
	ProductionAccessAllowed bool                            `json:"production_access_allowed"`
	OutboundActionsAllowed  bool                            `json:"outbound_actions_allowed"`
	PhoneFarmAccessAllowed  bool                            `json:"phone_farm_access_allowed"`
	RoleModelSelection      string                          `json:"role_model_selection"`
	MaxAutomaticCorrections int                             `json:"max_automatic_corrections"`
	MaxReviewAttempts       int                             `json:"max_review_attempts"`
	IndependentReviews      int                             `json:"independent_reviews"`
	ConsolidatedCorrections int                             `json:"consolidated_corrections"`
	SecondReview            bool                            `json:"second_review"`
	RequiredSurfaces        []string                        `json:"required_surfaces"`
	SourceArtifacts         []LocalShadowSourceArtifactV1   `json:"source_artifacts"`
	EvidenceContracts       []LocalShadowEvidenceContractV1 `json:"evidence_contracts"`
	Workflows               []LocalShadowWorkflowV1         `json:"workflows"`
}

type LocalShadowSourceArtifactV1 struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type LocalShadowEvidenceContractV1 struct {
	ID                string   `json:"id"`
	ProofLane         string   `json:"proof_lane"`
	SourceArtifactIDs []string `json:"source_artifact_ids"`
	RequiredFields    []string `json:"required_fields"`
}

type LocalShadowWorkflowV1 struct {
	ID                   string                         `json:"id"`
	Surfaces             []string                       `json:"surfaces"`
	HarnessArtifactIDs   []string                       `json:"harness_artifact_ids"`
	SquadID              string                         `json:"squad_id"`
	Capability           string                         `json:"capability,omitempty"`
	AccountableRoleID    string                         `json:"accountable_role_id,omitempty"`
	AccountableConfigID  string                         `json:"accountable_configuration_id,omitempty"`
	Host                 string                         `json:"host"`
	MaxConcurrentWorkers int                            `json:"max_concurrent_workers"`
	MaxCorrectionCycles  int                            `json:"max_correction_cycles"`
	ControllerOwnsMoves  bool                           `json:"controller_owns_moves"`
	RequiresRelease      bool                           `json:"requires_release"`
	Intake               LocalShadowStageAssignmentV1   `json:"intake"`
	Stages               []LocalShadowStageAssignmentV1 `json:"stages"`
}

type LocalShadowStageAssignmentV1 struct {
	Stage               string   `json:"stage"`
	RoleID              string   `json:"role_id"`
	ArtifactBindingID   string   `json:"artifact_binding_id"`
	EvidenceContractIDs []string `json:"evidence_contract_ids"`
}

// ShadowRoleArtifactRefV1 is a model-neutral bridge for local orchestration
// fixtures. It resolves accepted v1 and v2 role artifacts but deliberately
// cannot carry runtime, provider, model, effort, credential, or tool fields,
// and is not accepted by the guarded runtime-dispatch envelope.
type ShadowRoleArtifactRefV1 struct {
	ArtifactVersion      string `json:"artifact_version"`
	RoleID               string `json:"role_id"`
	BindingID            string `json:"binding_id"`
	InstructionPath      string `json:"instruction_path"`
	InstructionSHA256    string `json:"instruction_sha256"`
	SkillPath            string `json:"skill_path"`
	SkillSHA256          string `json:"skill_sha256"`
	SourceManifestSHA256 string `json:"source_manifest_sha256"`
}

type LocalShadowFakeRunV1 struct {
	SchemaVersion         string                     `json:"schema_version"`
	RunID                 string                     `json:"run_id"`
	SuiteSHA256           string                     `json:"suite_sha256"`
	ExecutionMode         string                     `json:"execution_mode"`
	ProviderRequestCount  int                        `json:"provider_request_count"`
	CredentialReadCount   int                        `json:"credential_read_count"`
	ToolCallCount         int                        `json:"tool_call_count"`
	RepositoryAccessCount int                        `json:"repository_access_count"`
	ProductionAccessCount int                        `json:"production_access_count"`
	OutboundActionCount   int                        `json:"outbound_action_count"`
	PhoneFarmAccessCount  int                        `json:"phone_farm_access_count"`
	Receipts              []LocalShadowFakeReceiptV1 `json:"receipts"`
}

type LocalShadowFakeReceiptV1 struct {
	ReceiptID          string                      `json:"receipt_id"`
	ReplayKey          string                      `json:"replay_key"`
	Sequence           int                         `json:"sequence"`
	WorkflowID         string                      `json:"workflow_id"`
	Surfaces           []string                    `json:"surfaces"`
	HarnessArtifactIDs []string                    `json:"harness_artifact_ids"`
	SquadID            string                      `json:"squad_id"`
	Stage              string                      `json:"stage"`
	RoleID             string                      `json:"role_id"`
	ArtifactBindingID  string                      `json:"artifact_binding_id"`
	Host               string                      `json:"host"`
	Evidence           []LocalShadowFakeEvidenceV1 `json:"evidence"`
	SideEffects        bool                        `json:"side_effects"`
}

type LocalShadowFakeEvidenceV1 struct {
	ContractID string            `json:"contract_id"`
	Fields     map[string]string `json:"fields"`
}

//go:embed local_shadow_workflow_suite_v1.json
var localShadowWorkflowSuiteV1JSON []byte

func (ctxContracts QualificationContracts) CurrentLocalShadowWorkflowSuiteV1() (LocalShadowWorkflowSuiteV1, error) {
	var suite LocalShadowWorkflowSuiteV1
	if err := strictJSON(ctxContracts.suiteJSON(), &suite); err != nil {
		return suite, fmt.Errorf("decode local shadow workflow suite v1: %w", err)
	}
	if err := ctxContracts.ValidateLocalShadowWorkflowSuiteV1(suite); err != nil {
		return LocalShadowWorkflowSuiteV1{}, err
	}
	return suite, nil
}

func (ctxContracts QualificationContracts) LocalShadowWorkflowSuiteV1SHA256() string {
	return localShadowSHA256V1(ctxContracts.suiteJSON())
}

func (ctxContracts QualificationContracts) ValidateLocalShadowWorkflowSuiteV1(suite LocalShadowWorkflowSuiteV1) error {
	if suite.SchemaVersion != LocalShadowWorkflowSuiteSchemaV1 || suite.SuiteID != localShadowSuiteIDV1 || suite.Status != localShadowSuiteStatusV1 || suite.ExecutionMode != localShadowExecutionModeV1 {
		return fmt.Errorf("local shadow suite identity or status drifted")
	}
	if suite.ProviderCallsAllowed || suite.CredentialsAllowed || suite.ToolsAllowed || suite.RepositoryAccessAllowed || suite.ProductionAccessAllowed || suite.OutboundActionsAllowed || suite.PhoneFarmAccessAllowed || suite.RoleModelSelection != "forbidden" {
		return fmt.Errorf("local shadow suite opened a forbidden authority lane")
	}
	if suite.MaxAutomaticCorrections != 1 || suite.MaxReviewAttempts != 2 || suite.IndependentReviews != 1 || suite.ConsolidatedCorrections != 1 || suite.SecondReview {
		return fmt.Errorf("local shadow correction or review policy drifted")
	}
	wantSurfaces := []string{"intake", "implementation", "review", "release_truth", "recovery", "inbox", "chat", "mobile", "notifications"}
	if !slices.Equal(suite.RequiredSurfaces, wantSurfaces) || len(suite.Workflows) != 4 {
		return fmt.Errorf("local shadow suite surface coverage drifted")
	}

	sources, err := ctxContracts.validateLocalShadowSourcesV1(suite.SourceArtifacts)
	if err != nil {
		return err
	}
	if err := validateLocalShadowRoleArtifactsV1(); err != nil {
		return err
	}
	contracts, err := validateLocalShadowEvidenceContractsV1(suite.EvidenceContracts, sources)
	if err != nil {
		return err
	}
	roles, err := ctxContracts.roles()
	if err != nil {
		return fmt.Errorf("load role catalog: %w", err)
	}
	squads := make(map[string]systemgraph.SquadTemplate, len(roles.Squads))
	for _, squad := range roles.Squads {
		squads[squad.ID] = squad
	}
	bindings := make(map[string]systemgraph.AccountableActorBinding, len(roles.AccountableActorBindings))
	for _, binding := range roles.AccountableActorBindings {
		bindings[binding.Capability] = binding
	}

	seenWorkflows := make(map[string]struct{}, len(suite.Workflows))
	seenSurfaces := make(map[string]struct{}, len(wantSurfaces))
	usedContracts := make(map[string]struct{}, len(contracts))
	wantJourneys := []struct {
		id        string
		squadID   string
		surfaces  []string
		harnesses []string
	}{
		{"local-intake-decision-coordination", "standard_outcome_delivery", []string{"intake", "chat", "inbox", "mobile", "notifications"}, []string{"handler_workflow_control_tests", "handler_chat_operational_guard", "handler_notification_dry_run_e2e", "core_owner_control_tests", "views_owner_inbox_tests", "mobile_inbox_schema_tests", "mobile_inbox_display_tests"}},
		{"local-implementation-review-mobile", "standard_outcome_delivery", []string{"implementation", "review", "inbox", "mobile", "notifications"}, []string{"handler_workflow_control_tests", "workflowstate_v2_scenarios", "core_owner_control_tests", "views_owner_inbox_tests", "mobile_inbox_schema_tests", "mobile_inbox_display_tests"}},
		{"local-release-regression-rollback", "cross_boundary_change", []string{"release_truth", "inbox", "mobile", "notifications"}, []string{"workflowstate_v2_scenarios", "workflowcommand_state_loader_tests", "workflowprojection_projector_tests", "ownernotification_policy_tests", "handler_notification_dry_run_e2e", "views_owner_inbox_tests", "mobile_inbox_schema_tests", "mobile_inbox_display_tests"}},
		{"local-interruption-recovery", "cross_boundary_change", []string{"recovery", "inbox", "notifications"}, []string{"service_delegated_recovery_tests", "service_delegated_recovery_settled_tests", "ownernotification_policy_tests"}},
	}
	for index, workflow := range suite.Workflows {
		wantJourney := wantJourneys[index]
		if workflow.ID != wantJourney.id || workflow.SquadID != wantJourney.squadID || !slices.Equal(workflow.Surfaces, wantJourney.surfaces) || !slices.Equal(workflow.HarnessArtifactIDs, wantJourney.harnesses) || workflow.Host != localShadowHostV1 {
			return fmt.Errorf("local shadow workflow %d identity, surfaces, squad, or host drifted", index)
		}
		seenHarnesses := map[string]struct{}{}
		for _, artifactID := range workflow.HarnessArtifactIDs {
			if _, ok := sources[artifactID]; !ok {
				return fmt.Errorf("workflow %s references unknown harness artifact %s", workflow.ID, artifactID)
			}
			if _, duplicate := seenHarnesses[artifactID]; duplicate {
				return fmt.Errorf("workflow %s repeats harness artifact %s", workflow.ID, artifactID)
			}
			seenHarnesses[artifactID] = struct{}{}
		}
		if _, duplicate := seenWorkflows[workflow.ID]; duplicate {
			return fmt.Errorf("duplicate local shadow workflow %s", workflow.ID)
		}
		seenWorkflows[workflow.ID] = struct{}{}
		for _, surface := range workflow.Surfaces {
			if !slices.Contains(wantSurfaces, surface) {
				return fmt.Errorf("workflow %s references unknown surface %s", workflow.ID, surface)
			}
			seenSurfaces[surface] = struct{}{}
		}
		squad, ok := squads[workflow.SquadID]
		if !ok || !squad.ControllerOwnsMoves || !workflow.ControllerOwnsMoves || workflow.MaxConcurrentWorkers != squad.MaxConcurrentWorkers || workflow.MaxCorrectionCycles != squad.MaxCorrectionCycles {
			return fmt.Errorf("workflow %s squad controller contract drifted", workflow.ID)
		}
		binding, ok := bindings[workflow.Capability]
		if !ok || binding.RoleID != workflow.AccountableRoleID || binding.ConfigurationID != workflow.AccountableConfigID {
			return fmt.Errorf("workflow %s accountable actor binding drifted", workflow.ID)
		}
		if err := validateLocalShadowAssignmentV1(workflow.Intake, "intake", []string{"mika_owner_interface"}, roles, contracts, usedContracts); err != nil {
			return fmt.Errorf("workflow %s intake: %w", workflow.ID, err)
		}
		if err := validateLocalShadowWorkflowStagesV1(workflow, squad, roles, contracts, usedContracts); err != nil {
			return err
		}
	}
	if len(seenSurfaces) != len(wantSurfaces) {
		return fmt.Errorf("local shadow required surface coverage is incomplete")
	}
	if len(usedContracts) != len(contracts) {
		return fmt.Errorf("local shadow evidence contract coverage is incomplete")
	}
	return nil
}

func validateLocalShadowRoleArtifactsV1() error {
	group, err := CurrentRoleQualificationCaseGroupV1()
	if err != nil {
		return fmt.Errorf("load v1 candidate role group: %w", err)
	}
	var manifest roleQualificationArtifactManifestFileV1
	if err := strictJSON(roleQualificationFreezeArtifactManifestV1, &manifest); err != nil {
		return fmt.Errorf("decode v1 role artifact manifest: %w", err)
	}
	if err := validateArtifactManifestV1(group, manifest); err != nil {
		return fmt.Errorf("validate v1 role artifacts: %w", err)
	}
	bindings, err := EmbeddedRoleArtifactBindingsV2()
	if err != nil {
		return fmt.Errorf("load v2 role artifacts: %w", err)
	}
	if err := ValidateRoleArtifactBindingsV2(bindings); err != nil {
		return fmt.Errorf("validate v2 role artifacts: %w", err)
	}
	return nil
}

func (ctxContracts QualificationContracts) validateLocalShadowSourcesV1(values []LocalShadowSourceArtifactV1) (map[string]LocalShadowSourceArtifactV1, error) {
	want := map[string]struct {
		path string
		hash string
	}{
		"role_catalog":                             {"server/internal/systemgraph/roles.json", systemgraph.RolesArtifactSHA256()},
		"role_artifacts_v1":                        {"server/internal/agentqualification/role_qualification_artifact_manifest_v1.json", localShadowSHA256V1(roleQualificationFreezeArtifactManifestV1)},
		"role_artifacts_v2":                        {"server/internal/agentqualification/role_artifact_bindings_v2.json", localShadowRoleArtifactBindingsV2SHA256()},
		"hosted_workflow_audit":                    {"docs/silver-mediums/HOSTED-WORKFLOW-INTERACTION-AUDIT-20260910.md", "afa2965d29644fa93b938ecf698840749e81958c889c327f24796ddf4337d375"},
		"accepted_items_09_12":                     {"docs/silver-mediums/ACCEPTANCE.md", "f229251e7812f16cb1d663c528334780b404f274d192390ffc7338864ff09bff"},
		"inbox_chat_acceptance":                    {"docs/silver-mediums/ITEM-10-INBOX-CHAT-ACCEPTANCE.md", "333845344ef7b27c329569c2748790fdc8feddb3d5452b1e81a33df31e9d5643"},
		"mobile_parity_acceptance":                 {"docs/silver-mediums/ITEM-11-MOBILE-PARITY.md", "c0ab1dd5db9622417367b3f4fe425e2dc4348f2c70c556df31243150f7e5425b"},
		"notification_acceptance":                  {"docs/silver-mediums/ITEM-12-SMS-NOTIFICATIONS.md", "0b9afc179a56a71cd2789304d3d16cac08466a31de560009d23fc5fe65b648fb"},
		"recovery_contract":                        {"docs/silver-mediums/ITEM-13-RECOVERY-CONTRACT.md", "2deabf6bb551f3c106d49a96b7c75a7b1602ad2c7aedeb0a1bb3a7373241e1ce"},
		"role_instruction_contract":                {"docs/silver-mediums/ITEM-14-ROLE-INSTRUCTION-CONTRACT.md", "749f346b700041c1bf7b3f3928135f76a0ca5872394f647a3b36c03a22a3fd85"},
		"handler_workflow_control_tests":           {"server/internal/handler/workflow_control_test.go", "134d0ac875a208a233c588b1240f0899bcd8a21e1949cba86bd0d9ccc12339f3"},
		"handler_chat_operational_guard":           {"server/internal/handler/workflow_chat_operational_guard_test.go", "ce781eab3ab503c252824472233694bfc77fdc6a8dcba5a0edd35291fcd2baab"},
		"handler_notification_dry_run_e2e":         {"server/internal/handler/owner_notification_dry_run_e2e_test.go", "65a3f029b0b93e5b248bcc2abc6494f95452bf77c02d5c460f04f4169ca30da6"},
		"workflowstate_v2_scenarios":               {"server/internal/workflowstate/v2/scenario_test.go", "081ee47d4b9f8a62a402179cc3708a7f69aec7adb904631fc78a16582d5959d9"},
		"workflowcommand_state_loader_tests":       {"server/internal/workflowcommand/state_loader_test.go", "47cfb6d9ec887a01df6e03251780f7340f0f191cc16e4e142d876e25816ba625"},
		"workflowprojection_projector_tests":       {"server/internal/workflowprojection/projector_test.go", "e9ba6d4acb86a1fc41916b9bcdcb6f08e886dec2cf3636bec677a6ab7ca5974c"},
		"ownernotification_policy_tests":           {"server/internal/ownernotification/policy_test.go", "54a4b5c7a6e9ea97d60eb1ecd99779d62350e1e10246051c4aa3bae5e864a401"},
		"service_delegated_recovery_tests":         {"server/internal/service/delegated_failure_recovery_test.go", "7b39b05af84e7c02b8a2f04d61ee736846f162d1b2e19ae7e3faed6ec44568b4"},
		"service_delegated_recovery_settled_tests": {"server/internal/service/delegated_failure_recovery_settled_test.go", "613025ac3f75e4058d78af8f79e0414f5ceed260fca5272c47588e65fc195c08"},
		"core_owner_control_tests":                 {"packages/core/inbox/owner-control.test.ts", "7c957d9d8d0628830d01ebf73a401e16a6ac1f7a51f8f40476bd2b9e6bdc0923"},
		"views_owner_inbox_tests":                  {"packages/views/inbox/components/owner-inbox-panel.test.tsx", "6c8cecb6570969826c79c93a613241fab00d16d70333478f343b744c47c85ec0"},
		"mobile_inbox_schema_tests":                {"apps/mobile/data/inbox-schema.test.ts", "392d88ec4bd4e548cfd287457eb2282bf50618f334536b6dcd350360409ec74f"},
		"mobile_inbox_display_tests":               {"apps/mobile/lib/inbox-display.test.ts", "4c767d147a3ffbdc6c7d59f165824b7855f36850cadff2559c9995c23685d140"},
	}
	if ctxContracts.historicalSources != nil {
		want = make(map[string]struct {
			path string
			hash string
		}, len(ctxContracts.historicalSources))
		for id, source := range ctxContracts.historicalSources {
			want[id] = struct {
				path string
				hash string
			}{source.Path, source.SHA256}
		}
	}

	if len(values) != len(want) {
		return nil, fmt.Errorf("local shadow source artifact coverage drifted")
	}
	seen := make(map[string]LocalShadowSourceArtifactV1, len(values))
	for _, value := range values {
		expected, ok := want[value.ID]
		cleaned := path.Clean(value.Path)
		if !ok || value.Path == "" || cleaned != value.Path || cleaned == "." || strings.HasPrefix(cleaned, "../") || path.IsAbs(cleaned) || value.Path != expected.path || value.SHA256 != expected.hash || !validSHA256(value.SHA256) {
			return nil, fmt.Errorf("local shadow source artifact %s drifted", value.ID)
		}
		if _, duplicate := seen[value.ID]; duplicate {
			return nil, fmt.Errorf("duplicate local shadow source artifact %s", value.ID)
		}
		seen[value.ID] = value
	}
	return seen, nil
}

func validateLocalShadowEvidenceContractsV1(values []LocalShadowEvidenceContractV1, sources map[string]LocalShadowSourceArtifactV1) (map[string]LocalShadowEvidenceContractV1, error) {
	if len(values) != 12 {
		return nil, fmt.Errorf("local shadow evidence contract count drifted")
	}
	allowedLanes := []string{"controller_state", "local_code", "remote_git", "deployed_revision", "observed_runtime", "business_outcome"}
	seen := make(map[string]LocalShadowEvidenceContractV1, len(values))
	for _, value := range values {
		if value.ID == "" || !slices.Contains(allowedLanes, value.ProofLane) || len(value.SourceArtifactIDs) == 0 || len(value.RequiredFields) == 0 {
			return nil, fmt.Errorf("local shadow evidence contract %s is incomplete", value.ID)
		}
		if _, duplicate := seen[value.ID]; duplicate {
			return nil, fmt.Errorf("duplicate local shadow evidence contract %s", value.ID)
		}
		seen[value.ID] = value
		seenSources := map[string]struct{}{}
		for _, sourceID := range value.SourceArtifactIDs {
			if _, ok := sources[sourceID]; !ok {
				return nil, fmt.Errorf("evidence contract %s references unknown source %s", value.ID, sourceID)
			}
			if _, duplicate := seenSources[sourceID]; duplicate {
				return nil, fmt.Errorf("evidence contract %s repeats source %s", value.ID, sourceID)
			}
			seenSources[sourceID] = struct{}{}
		}
		seenFields := map[string]struct{}{}
		for _, field := range value.RequiredFields {
			if strings.TrimSpace(field) == "" {
				return nil, fmt.Errorf("evidence contract %s has an empty field", value.ID)
			}
			if _, duplicate := seenFields[field]; duplicate {
				return nil, fmt.Errorf("evidence contract %s repeats field %s", value.ID, field)
			}
			seenFields[field] = struct{}{}
		}
	}
	return seen, nil
}

func validateLocalShadowWorkflowStagesV1(workflow LocalShadowWorkflowV1, squad systemgraph.SquadTemplate, roles systemgraph.RoleCatalog, contracts map[string]LocalShadowEvidenceContractV1, used map[string]struct{}) error {
	stageDefinitions := make(map[string]systemgraph.SquadStage, len(squad.Stages))
	stageOrder := make(map[string]int, len(squad.Stages))
	for index, stage := range squad.Stages {
		stageDefinitions[stage.Name] = stage
		stageOrder[stage.Name] = index
	}
	seen := map[string]struct{}{}
	lastOrder := -1
	for _, assignment := range workflow.Stages {
		definition, ok := stageDefinitions[assignment.Stage]
		if !ok || stageOrder[assignment.Stage] <= lastOrder {
			return fmt.Errorf("workflow %s stage order drifted", workflow.ID)
		}
		lastOrder = stageOrder[assignment.Stage]
		if _, duplicate := seen[assignment.Stage]; duplicate {
			return fmt.Errorf("workflow %s repeats stage %s", workflow.ID, assignment.Stage)
		}
		seen[assignment.Stage] = struct{}{}
		if err := validateLocalShadowAssignmentV1(assignment, assignment.Stage, definition.EligibleRoles, roles, contracts, used); err != nil {
			return fmt.Errorf("workflow %s stage %s: %w", workflow.ID, assignment.Stage, err)
		}
	}
	for _, definition := range squad.Stages {
		_, present := seen[definition.Name]
		if definition.Required && !present {
			return fmt.Errorf("workflow %s omits required squad stage %s", workflow.ID, definition.Name)
		}
	}
	_, hasRelease := seen["release"]
	if hasRelease != workflow.RequiresRelease {
		return fmt.Errorf("workflow %s release declaration drifted", workflow.ID)
	}
	if slices.Contains(workflow.Surfaces, "release_truth") && !hasRelease {
		return fmt.Errorf("release truth workflow lacks a release stage")
	}
	if verification, ok := localShadowAssignmentForStageV1(workflow.Stages, "verification"); ok {
		if verification.RoleID != "quality_checker" {
			return fmt.Errorf("workflow %s verification is not independently assigned", workflow.ID)
		}
		if implementation, ok := localShadowAssignmentForStageV1(workflow.Stages, "implementation"); ok && implementation.RoleID == verification.RoleID {
			return fmt.Errorf("workflow %s reviewer shares implementation role", workflow.ID)
		}
	}
	return nil
}

func validateLocalShadowAssignmentV1(assignment LocalShadowStageAssignmentV1, wantStage string, eligible []string, roles systemgraph.RoleCatalog, contracts map[string]LocalShadowEvidenceContractV1, used map[string]struct{}) error {
	if assignment.Stage != wantStage || !slices.Contains(eligible, assignment.RoleID) || len(assignment.EvidenceContractIDs) == 0 {
		return fmt.Errorf("stage identity, role, or evidence coverage drifted")
	}
	if _, ok := roles.Role(assignment.RoleID); !ok {
		return fmt.Errorf("unknown role %s", assignment.RoleID)
	}
	if strings.HasPrefix(assignment.ArtifactBindingID, "silver-mediums.role-catalog.v1.") {
		if assignment.Stage != "outcome_binding" || assignment.ArtifactBindingID != "silver-mediums.role-catalog.v1."+assignment.RoleID || !strings.HasSuffix(assignment.RoleID, "_outcome") {
			return fmt.Errorf("role %s catalog binding drifted", assignment.RoleID)
		}
	} else {
		artifact, err := ValidatedShadowRoleArtifactRefV1(assignment.ArtifactBindingID)
		if err != nil || artifact.RoleID != assignment.RoleID {
			return fmt.Errorf("role %s artifact binding drifted", assignment.RoleID)
		}
	}
	seen := map[string]struct{}{}
	for _, contractID := range assignment.EvidenceContractIDs {
		if _, ok := contracts[contractID]; !ok {
			return fmt.Errorf("unknown evidence contract %s", contractID)
		}
		if _, duplicate := seen[contractID]; duplicate {
			return fmt.Errorf("duplicate evidence contract %s", contractID)
		}
		seen[contractID] = struct{}{}
		used[contractID] = struct{}{}
	}
	return nil
}

func ValidatedShadowRoleArtifactRefV1(bindingID string) (ShadowRoleArtifactRefV1, error) {
	const v1Prefix = "silver-mediums.role-artifact.v1."
	const v2Prefix = "silver-mediums.role-artifact.v2."
	if strings.HasPrefix(bindingID, v1Prefix) {
		roleID := strings.TrimPrefix(bindingID, v1Prefix)
		allowed := []string{"mika_owner_interface", "shared_windows_executor", "product_ui", "quality_checker"}
		if !slices.Contains(allowed, roleID) {
			return ShadowRoleArtifactRefV1{}, fmt.Errorf("v1 shadow role %s is not allowlisted", roleID)
		}
		freeze, err := CurrentRoleQualificationFreezeV1()
		if err != nil {
			return ShadowRoleArtifactRefV1{}, fmt.Errorf("validate v1 role freeze: %w", err)
		}
		var manifest roleQualificationArtifactManifestFileV1
		if err := strictJSON(roleQualificationFreezeArtifactManifestV1, &manifest); err != nil {
			return ShadowRoleArtifactRefV1{}, fmt.Errorf("decode v1 role manifest: %w", err)
		}
		artifacts := make(map[string]roleQualificationManifestArtifactV1, len(manifest.Artifacts))
		for _, artifact := range manifest.Artifacts {
			artifacts[artifact.ID] = artifact
		}
		for _, binding := range manifest.CandidateBindings {
			if binding.CandidateClass != roleID {
				continue
			}
			instruction, instructionOK := artifacts[binding.InstructionArtifactID]
			skill, skillOK := artifacts[binding.SkillArtifactID]
			if !instructionOK || !skillOK || freeze.Artifacts[instruction.Path] != instruction.SHA256 || freeze.Artifacts[skill.Path] != skill.SHA256 {
				return ShadowRoleArtifactRefV1{}, fmt.Errorf("v1 shadow role %s artifact freeze drifted", roleID)
			}
			return ShadowRoleArtifactRefV1{
				ArtifactVersion:      "v1",
				RoleID:               roleID,
				BindingID:            bindingID,
				InstructionPath:      instruction.Path,
				InstructionSHA256:    instruction.SHA256,
				SkillPath:            skill.Path,
				SkillSHA256:          skill.SHA256,
				SourceManifestSHA256: localShadowSHA256V1(roleQualificationFreezeArtifactManifestV1),
			}, nil
		}
		return ShadowRoleArtifactRefV1{}, fmt.Errorf("v1 shadow role %s has no artifact binding", roleID)
	}
	if strings.HasPrefix(bindingID, v2Prefix) {
		binding, err := ValidatedRoleArtifactBindingV2(bindingID)
		if err != nil {
			return ShadowRoleArtifactRefV1{}, err
		}
		return ShadowRoleArtifactRefV1{
			ArtifactVersion:      "v2",
			RoleID:               binding.RoleID,
			BindingID:            binding.BindingID,
			InstructionPath:      binding.Instruction.Path,
			InstructionSHA256:    binding.Instruction.SHA256,
			SkillPath:            binding.Skill.Path,
			SkillSHA256:          binding.Skill.SHA256,
			SourceManifestSHA256: localShadowRoleArtifactBindingsV2SHA256(),
		}, nil
	}
	return ShadowRoleArtifactRefV1{}, fmt.Errorf("shadow artifact binding %s is not accepted", bindingID)
}

func localShadowAssignmentForStageV1(values []LocalShadowStageAssignmentV1, stage string) (LocalShadowStageAssignmentV1, bool) {
	for _, value := range values {
		if value.Stage == stage {
			return value, true
		}
	}
	return LocalShadowStageAssignmentV1{}, false
}

func (ctxContracts QualificationContracts) RunLocalShadowWorkflowSuiteFakeV1(suite LocalShadowWorkflowSuiteV1) (LocalShadowFakeRunV1, error) {
	if err := ctxContracts.ValidateLocalShadowWorkflowSuiteV1(suite); err != nil {
		return LocalShadowFakeRunV1{}, err
	}
	run := LocalShadowFakeRunV1{
		SchemaVersion: LocalShadowFakeRunSchemaV1,
		RunID:         "item-14-local-shadow-fake-run-v1",
		SuiteSHA256:   ctxContracts.LocalShadowWorkflowSuiteV1SHA256(),
		ExecutionMode: localShadowExecutionModeV1,
	}
	contracts := make(map[string]LocalShadowEvidenceContractV1, len(suite.EvidenceContracts))
	for _, contract := range suite.EvidenceContracts {
		contracts[contract.ID] = contract
	}
	sequence := 0
	for _, workflow := range suite.Workflows {
		assignments := append([]LocalShadowStageAssignmentV1{workflow.Intake}, workflow.Stages...)
		for _, assignment := range assignments {
			receipt := LocalShadowFakeReceiptV1{
				ReceiptID:          fmt.Sprintf("fake/%s/%02d/%s", workflow.ID, sequence, assignment.Stage),
				ReplayKey:          fmt.Sprintf("%s:%s", workflow.ID, assignment.Stage),
				Sequence:           sequence,
				WorkflowID:         workflow.ID,
				Surfaces:           append([]string(nil), workflow.Surfaces...),
				HarnessArtifactIDs: append([]string(nil), workflow.HarnessArtifactIDs...),
				SquadID:            workflow.SquadID,
				Stage:              assignment.Stage,
				RoleID:             assignment.RoleID,
				ArtifactBindingID:  assignment.ArtifactBindingID,
				Host:               workflow.Host,
			}
			for _, contractID := range assignment.EvidenceContractIDs {
				contract := contracts[contractID]
				fields := make(map[string]string, len(contract.RequiredFields))
				for _, field := range contract.RequiredFields {
					fields[field] = "fake:" + workflow.ID + ":" + contractID + ":" + field
				}
				receipt.Evidence = append(receipt.Evidence, LocalShadowFakeEvidenceV1{ContractID: contractID, Fields: fields})
			}
			run.Receipts = append(run.Receipts, receipt)
			sequence++
		}
	}
	if err := ctxContracts.ValidateLocalShadowFakeRunV1(suite, run); err != nil {
		return LocalShadowFakeRunV1{}, err
	}
	return run, nil
}

func (ctxContracts QualificationContracts) ValidateLocalShadowFakeRunV1(suite LocalShadowWorkflowSuiteV1, run LocalShadowFakeRunV1) error {
	if err := ctxContracts.ValidateLocalShadowWorkflowSuiteV1(suite); err != nil {
		return err
	}
	if run.SchemaVersion != LocalShadowFakeRunSchemaV1 || run.RunID != "item-14-local-shadow-fake-run-v1" || run.SuiteSHA256 != ctxContracts.LocalShadowWorkflowSuiteV1SHA256() || run.ExecutionMode != localShadowExecutionModeV1 {
		return fmt.Errorf("local shadow fake run identity drifted")
	}
	if run.ProviderRequestCount != 0 || run.CredentialReadCount != 0 || run.ToolCallCount != 0 || run.RepositoryAccessCount != 0 || run.ProductionAccessCount != 0 || run.OutboundActionCount != 0 || run.PhoneFarmAccessCount != 0 {
		return fmt.Errorf("local shadow fake run crossed a forbidden boundary")
	}
	contracts := make(map[string]LocalShadowEvidenceContractV1, len(suite.EvidenceContracts))
	for _, contract := range suite.EvidenceContracts {
		contracts[contract.ID] = contract
	}
	expectedCount := 0
	for _, workflow := range suite.Workflows {
		expectedCount += 1 + len(workflow.Stages)
	}
	if len(run.Receipts) != expectedCount {
		return fmt.Errorf("local shadow fake receipt coverage drifted")
	}
	seenReceipts := map[string]struct{}{}
	seenReplayKeys := map[string]struct{}{}
	sequence := 0
	for _, workflow := range suite.Workflows {
		assignments := append([]LocalShadowStageAssignmentV1{workflow.Intake}, workflow.Stages...)
		for _, assignment := range assignments {
			receipt := run.Receipts[sequence]
			wantID := fmt.Sprintf("fake/%s/%02d/%s", workflow.ID, sequence, assignment.Stage)
			wantReplay := fmt.Sprintf("%s:%s", workflow.ID, assignment.Stage)
			if receipt.ReceiptID != wantID || receipt.ReplayKey != wantReplay || receipt.Sequence != sequence || receipt.WorkflowID != workflow.ID || !slices.Equal(receipt.Surfaces, workflow.Surfaces) || !slices.Equal(receipt.HarnessArtifactIDs, workflow.HarnessArtifactIDs) || receipt.SquadID != workflow.SquadID || receipt.Stage != assignment.Stage || receipt.RoleID != assignment.RoleID || receipt.ArtifactBindingID != assignment.ArtifactBindingID || receipt.Host != workflow.Host || receipt.SideEffects {
				return fmt.Errorf("local shadow fake receipt %d drifted", sequence)
			}
			if _, duplicate := seenReceipts[receipt.ReceiptID]; duplicate {
				return fmt.Errorf("duplicate local shadow fake receipt %s", receipt.ReceiptID)
			}
			seenReceipts[receipt.ReceiptID] = struct{}{}
			if _, duplicate := seenReplayKeys[receipt.ReplayKey]; duplicate {
				return fmt.Errorf("replayed local shadow fake receipt %s", receipt.ReplayKey)
			}
			seenReplayKeys[receipt.ReplayKey] = struct{}{}
			if len(receipt.Evidence) != len(assignment.EvidenceContractIDs) {
				return fmt.Errorf("local shadow fake receipt %d evidence coverage drifted", sequence)
			}
			for evidenceIndex, evidence := range receipt.Evidence {
				contractID := assignment.EvidenceContractIDs[evidenceIndex]
				contract := contracts[contractID]
				if evidence.ContractID != contractID || len(evidence.Fields) != len(contract.RequiredFields) {
					return fmt.Errorf("local shadow fake evidence %s drifted", contractID)
				}
				for _, field := range contract.RequiredFields {
					want := "fake:" + workflow.ID + ":" + contractID + ":" + field
					if evidence.Fields[field] != want {
						return fmt.Errorf("local shadow fake evidence %s field %s drifted", contractID, field)
					}
				}
			}
			sequence++
		}
	}
	return nil
}

func localShadowRoleArtifactBindingsV2SHA256() string {
	raw, err := roleArtifactBindingsV2FS.ReadFile("role_artifact_bindings_v2.json")
	if err != nil {
		return ""
	}
	return localShadowSHA256V1(raw)
}

func localShadowSHA256V1(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
