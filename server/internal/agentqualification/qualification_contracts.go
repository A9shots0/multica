package agentqualification

import (
	"encoding/json"
	"fmt"
	"github.com/multica-ai/multica/server/internal/qualificationarchive"
	"github.com/multica-ai/multica/server/internal/systemgraph"
)

// QualificationContracts supplies explicit immutable inputs to pure protocol
// validation. Its zero value is the current-source contract used by every
// existing public entrypoint. Historical contracts never grant execution.
type QualificationContracts struct {
	historicalSuite        []byte
	historicalRoles        *systemgraph.RoleCatalog
	historicalSources      map[string]LocalShadowSourceArtifactV1
	historicalGuardedChild []byte
}

// HistoricalQualificationContracts loads original, hash-verified experiment
// inputs for offline replay and fake transports. Live runners reject this input.
func HistoricalQualificationContracts() (QualificationContracts, error) {
	var c QualificationContracts
	raw, err := qualificationarchive.Read("suite-v1", "server/internal/agentqualification/local_shadow_workflow_suite_v1.json", "9c8803d41b12b99fa7286ecd379d82ca3f2497bae8f181f1d4c524e8912e32be")
	if err != nil {
		return c, err
	}
	var suite LocalShadowWorkflowSuiteV1
	if err := strictJSON(raw, &suite); err != nil {
		return c, err
	}
	c.historicalSources = make(map[string]LocalShadowSourceArtifactV1, len(suite.SourceArtifacts))
	for _, source := range suite.SourceArtifacts {
		bytes, err := qualificationarchive.Read("suite-v1", source.Path, source.SHA256)
		if err != nil {
			return QualificationContracts{}, err
		}
		if source.ID == "role_catalog" {
			var roles systemgraph.RoleCatalog
			if err := strictJSON(bytes, &roles); err != nil {
				return QualificationContracts{}, err
			}
			c.historicalRoles = &roles
		}
		c.historicalSources[source.ID] = source
	}
	if c.historicalRoles == nil {
		return QualificationContracts{}, fmt.Errorf("historical role catalog missing")
	}
	c.historicalSuite = raw
	c.historicalGuardedChild, err = qualificationarchive.Read("primary-v1", "server/internal/agentqualification/guarded_child_execution_v1.go", "36aa20239d05801a062adeab29b455ef806704eea86c328b3375afe0f8a31f56")
	if err != nil {
		return QualificationContracts{}, err
	}
	return c, nil
}

func (c QualificationContracts) Historical() bool { return c.historicalSuite != nil }
func (c QualificationContracts) suiteJSON() []byte {
	if c.historicalSuite != nil {
		return c.historicalSuite
	}
	return localShadowWorkflowSuiteV1JSON
}
func (c QualificationContracts) roles() (systemgraph.RoleCatalog, error) {
	if c.historicalRoles != nil {
		return *c.historicalRoles, nil
	}
	return systemgraph.DefaultRoles()
}
func (c QualificationContracts) guardedChild() []byte {
	if c.historicalGuardedChild != nil {
		return c.historicalGuardedChild
	}
	return guardedChildExecutionV1Source
}

func CurrentLocalShadowWorkflowSuiteV1() (LocalShadowWorkflowSuiteV1, error) {
	return (QualificationContracts{}).CurrentLocalShadowWorkflowSuiteV1()
}

func LocalShadowWorkflowSuiteV1SHA256() string {
	return (QualificationContracts{}).LocalShadowWorkflowSuiteV1SHA256()
}

func ValidateLocalShadowWorkflowSuiteV1(suite LocalShadowWorkflowSuiteV1) error {
	return (QualificationContracts{}).ValidateLocalShadowWorkflowSuiteV1(suite)
}

func validateLocalShadowSourcesV1(values []LocalShadowSourceArtifactV1) (map[string]LocalShadowSourceArtifactV1, error) {
	return (QualificationContracts{}).validateLocalShadowSourcesV1(values)
}

func RunLocalShadowWorkflowSuiteFakeV1(suite LocalShadowWorkflowSuiteV1) (LocalShadowFakeRunV1, error) {
	return (QualificationContracts{}).RunLocalShadowWorkflowSuiteFakeV1(suite)
}

func ValidateLocalShadowFakeRunV1(suite LocalShadowWorkflowSuiteV1, run LocalShadowFakeRunV1) error {
	return (QualificationContracts{}).ValidateLocalShadowFakeRunV1(suite, run)
}

func CurrentOpenRouterLocalShadowDefinitionV1() (OpenRouterLocalShadowDefinitionV1, error) {
	return (QualificationContracts{}).CurrentOpenRouterLocalShadowDefinitionV1()
}

func ValidateOpenRouterLocalShadowDefinitionV1(definition OpenRouterLocalShadowDefinitionV1) error {
	return (QualificationContracts{}).ValidateOpenRouterLocalShadowDefinitionV1(definition)
}

func BuildOpenRouterLocalShadowPlansV1(definition OpenRouterLocalShadowDefinitionV1, suite LocalShadowWorkflowSuiteV1) ([]OpenCodeScreeningPlan, error) {
	return (QualificationContracts{}).BuildOpenRouterLocalShadowPlansV1(definition, suite)
}

func BuildOpenRouterLocalShadowReceiptV1(definition OpenRouterLocalShadowDefinitionV1, suite LocalShadowWorkflowSuiteV1, plan OpenCodeScreeningPlan, result ProviderExecutionResult, responseRaw, rawExport []byte) (OpenRouterLocalShadowReceiptV1, error) {
	return (QualificationContracts{}).BuildOpenRouterLocalShadowReceiptV1(definition, suite, plan, result, responseRaw, rawExport)
}

func RunOpenRouterLocalShadowFakeV1(definition OpenRouterLocalShadowDefinitionV1, suite LocalShadowWorkflowSuiteV1) (OpenRouterLocalShadowRunV1, error) {
	return (QualificationContracts{}).RunOpenRouterLocalShadowFakeV1(definition, suite)
}

func BuildOpenRouterLocalShadowRunV1(definition OpenRouterLocalShadowDefinitionV1, suite LocalShadowWorkflowSuiteV1, receipts []OpenRouterLocalShadowReceiptV1) (OpenRouterLocalShadowRunV1, error) {
	return (QualificationContracts{}).BuildOpenRouterLocalShadowRunV1(definition, suite, receipts)
}

func ValidateOpenRouterLocalShadowRunV1(definition OpenRouterLocalShadowDefinitionV1, suite LocalShadowWorkflowSuiteV1, run OpenRouterLocalShadowRunV1) error {
	return (QualificationContracts{}).ValidateOpenRouterLocalShadowRunV1(definition, suite, run)
}

func CurrentOpenRouterDirectShadowDefinitionV2() (OpenRouterDirectShadowDefinitionV2, error) {
	return (QualificationContracts{}).CurrentOpenRouterDirectShadowDefinitionV2()
}

func ValidateOpenRouterDirectShadowDefinitionV2(definition OpenRouterDirectShadowDefinitionV2) error {
	return (QualificationContracts{}).ValidateOpenRouterDirectShadowDefinitionV2(definition)
}

func BuildOpenRouterDirectShadowPlansV2(definition OpenRouterDirectShadowDefinitionV2, suite LocalShadowWorkflowSuiteV1) ([]OpenRouterDirectShadowPlanV2, error) {
	return (QualificationContracts{}).BuildOpenRouterDirectShadowPlansV2(definition, suite)
}

func buildOpenRouterDirectShadowPlanV2(definition OpenRouterDirectShadowDefinitionV2, workflow LocalShadowWorkflowV1) (OpenRouterDirectShadowPlanV2, error) {
	return (QualificationContracts{}).buildOpenRouterDirectShadowPlanV2(definition, workflow)
}

func ValidateOpenRouterDirectShadowPlanV2(definition OpenRouterDirectShadowDefinitionV2, plan OpenRouterDirectShadowPlanV2) error {
	return (QualificationContracts{}).ValidateOpenRouterDirectShadowPlanV2(definition, plan)
}

func BuildOpenRouterDirectShadowObservationV2(definition OpenRouterDirectShadowDefinitionV2, suite LocalShadowWorkflowSuiteV1, plan OpenRouterDirectShadowPlanV2, raw []byte, providerRequests int, startedAt, completedAt string, durationMillis int64) (OpenRouterDirectShadowObservationV2Record, error) {
	return (QualificationContracts{}).BuildOpenRouterDirectShadowObservationV2(definition, suite, plan, raw, providerRequests, startedAt, completedAt, durationMillis)
}

func BuildOpenRouterDirectShadowRunV2(definition OpenRouterDirectShadowDefinitionV2, suite LocalShadowWorkflowSuiteV1, observations []OpenRouterDirectShadowObservationV2Record, fake bool) (OpenRouterDirectShadowRunV2, error) {
	return (QualificationContracts{}).BuildOpenRouterDirectShadowRunV2(definition, suite, observations, fake)
}

func ValidateOpenRouterDirectShadowRunV2(definition OpenRouterDirectShadowDefinitionV2, suite LocalShadowWorkflowSuiteV1, run OpenRouterDirectShadowRunV2) error {
	return (QualificationContracts{}).ValidateOpenRouterDirectShadowRunV2(definition, suite, run)
}

func CurrentOpenRouterDirectShadowDefinitionV3() (OpenRouterDirectShadowDefinitionV3, error) {
	return (QualificationContracts{}).CurrentOpenRouterDirectShadowDefinitionV3()
}

func ValidateOpenRouterDirectShadowDefinitionV3(definition OpenRouterDirectShadowDefinitionV3) error {
	return (QualificationContracts{}).ValidateOpenRouterDirectShadowDefinitionV3(definition)
}

func BuildOpenRouterDirectShadowPlansV3(definition OpenRouterDirectShadowDefinitionV3, suite LocalShadowWorkflowSuiteV1) ([]OpenRouterDirectShadowPlanV2, error) {
	return (QualificationContracts{}).BuildOpenRouterDirectShadowPlansV3(definition, suite)
}

func ValidateOpenRouterDirectShadowPlanV3(definition OpenRouterDirectShadowDefinitionV3, plan OpenRouterDirectShadowPlanV2) error {
	return (QualificationContracts{}).ValidateOpenRouterDirectShadowPlanV3(definition, plan)
}

func BuildOpenRouterDirectShadowObservationV3(definition OpenRouterDirectShadowDefinitionV3, suite LocalShadowWorkflowSuiteV1, plan OpenRouterDirectShadowPlanV2, raw []byte, providerRequests int, startedAt, completedAt string, durationMillis int64) (OpenRouterDirectShadowObservationV2Record, error) {
	return (QualificationContracts{}).BuildOpenRouterDirectShadowObservationV3(definition, suite, plan, raw, providerRequests, startedAt, completedAt, durationMillis)
}

func BuildOpenRouterDirectShadowRunV3(definition OpenRouterDirectShadowDefinitionV3, suite LocalShadowWorkflowSuiteV1, observations []OpenRouterDirectShadowObservationV2Record, fake bool) (OpenRouterDirectShadowRunV3, error) {
	return (QualificationContracts{}).BuildOpenRouterDirectShadowRunV3(definition, suite, observations, fake)
}

func ValidateOpenRouterDirectShadowRunV3(definition OpenRouterDirectShadowDefinitionV3, suite LocalShadowWorkflowSuiteV1, run OpenRouterDirectShadowRunV3) error {
	return (QualificationContracts{}).ValidateOpenRouterDirectShadowRunV3(definition, suite, run)
}

func CurrentOpenRouterDirectShadowDefinitionV4() (OpenRouterDirectShadowDefinitionV4, error) {
	return (QualificationContracts{}).CurrentOpenRouterDirectShadowDefinitionV4()
}

func ValidateOpenRouterDirectShadowDefinitionV4(definition OpenRouterDirectShadowDefinitionV4) error {
	return (QualificationContracts{}).ValidateOpenRouterDirectShadowDefinitionV4(definition)
}

func BuildOpenRouterDirectShadowPlansV4(definition OpenRouterDirectShadowDefinitionV4, suite LocalShadowWorkflowSuiteV1) ([]OpenRouterDirectShadowPlanV2, error) {
	return (QualificationContracts{}).BuildOpenRouterDirectShadowPlansV4(definition, suite)
}

func ValidateOpenRouterDirectShadowPlanV4(definition OpenRouterDirectShadowDefinitionV4, plan OpenRouterDirectShadowPlanV2) error {
	return (QualificationContracts{}).ValidateOpenRouterDirectShadowPlanV4(definition, plan)
}

func BuildOpenRouterDirectShadowObservationV4(definition OpenRouterDirectShadowDefinitionV4, suite LocalShadowWorkflowSuiteV1, plan OpenRouterDirectShadowPlanV2, raw []byte, providerRequests int, startedAt, completedAt string, durationMillis int64) (OpenRouterDirectShadowObservationV2Record, error) {
	return (QualificationContracts{}).BuildOpenRouterDirectShadowObservationV4(definition, suite, plan, raw, providerRequests, startedAt, completedAt, durationMillis)
}

func BuildOpenRouterDirectShadowRunV4(definition OpenRouterDirectShadowDefinitionV4, suite LocalShadowWorkflowSuiteV1, observations []OpenRouterDirectShadowObservationV2Record, fake bool) (OpenRouterDirectShadowRunV4, error) {
	return (QualificationContracts{}).BuildOpenRouterDirectShadowRunV4(definition, suite, observations, fake)
}

func ValidateOpenRouterDirectShadowRunV4(definition OpenRouterDirectShadowDefinitionV4, suite LocalShadowWorkflowSuiteV1, run OpenRouterDirectShadowRunV4) error {
	return (QualificationContracts{}).ValidateOpenRouterDirectShadowRunV4(definition, suite, run)
}

func CurrentPrimarySubscriptionRoleTrialGateV1() (PrimarySubscriptionRoleTrialGateV1, error) {
	return (QualificationContracts{}).CurrentPrimarySubscriptionRoleTrialGateV1()
}

func ValidatePrimarySubscriptionRoleTrialGateV1(gate PrimarySubscriptionRoleTrialGateV1) error {
	return (QualificationContracts{}).ValidatePrimarySubscriptionRoleTrialGateV1(gate)
}

func validatePrimarySubscriptionRoleTrialSourcesV1(h PrimarySubscriptionRoleTrialSourceHashesV1) error {
	return (QualificationContracts{}).validatePrimarySubscriptionRoleTrialSourcesV1(h)
}

func CurrentPrimarySubscriptionRoleTrialV1() (PrimarySubscriptionRoleTrialV1, error) {
	return (QualificationContracts{}).CurrentPrimarySubscriptionRoleTrialV1()
}

func ValidatePrimarySubscriptionRoleTrialV1(definition PrimarySubscriptionRoleTrialV1) error {
	return (QualificationContracts{}).ValidatePrimarySubscriptionRoleTrialV1(definition)
}

func BuildPrimarySubscriptionTrialPromptV1(invocation PrimarySubscriptionTrialInvocationV1, workspace RealisticCaseWorkspace, priorSpecialist json.RawMessage) (string, []byte, error) {
	return (QualificationContracts{}).BuildPrimarySubscriptionTrialPromptV1(invocation, workspace, priorSpecialist)
}

func PrimarySubscriptionTrialResponseSchemaV1(invocation PrimarySubscriptionTrialInvocationV1) ([]byte, error) {
	return (QualificationContracts{}).PrimarySubscriptionTrialResponseSchemaV1(invocation)
}

func ParsePrimarySubscriptionTrialParticipantOutputV1(raw []byte, invocation PrimarySubscriptionTrialInvocationV1, workspace RealisticCaseWorkspace) (PrimarySubscriptionTrialParticipantOutputV1, error) {
	return (QualificationContracts{}).ParsePrimarySubscriptionTrialParticipantOutputV1(raw, invocation, workspace)
}

func CurrentRoleSquadOperatingContractV1() (RoleSquadOperatingContractV1, error) {
	return (QualificationContracts{}).CurrentRoleSquadOperatingContractV1()
}

func CurrentRoleSquadOperatingValidationV1() (RoleSquadOperatingValidationV1, error) {
	return (QualificationContracts{}).CurrentRoleSquadOperatingValidationV1()
}

func ValidateRoleSquadOperatingContractV1(contract RoleSquadOperatingContractV1) error {
	return (QualificationContracts{}).ValidateRoleSquadOperatingContractV1(contract)
}

func FakeRoleSquadOperatingValidationV1() (RoleSquadOperatingValidationV1, error) {
	return (QualificationContracts{}).FakeRoleSquadOperatingValidationV1()
}

func CurrentRoleSquadRehearsalResultV1() (RoleSquadRehearsalResultV1, error) {
	return (QualificationContracts{}).CurrentRoleSquadRehearsalResultV1()
}

func RunRoleSquadRehearsalV1() (RoleSquadRehearsalResultV1, []RoleSquadRehearsalReceiptV1, error) {
	return (QualificationContracts{}).RunRoleSquadRehearsalV1()
}

func CurrentRoleStageEvidenceContractV1() (RoleStageEvidenceContractV1, error) {
	return (QualificationContracts{}).CurrentRoleStageEvidenceContractV1()
}

func ValidateRoleStageEvidenceContractV1(contract RoleStageEvidenceContractV1) error {
	return (QualificationContracts{}).ValidateRoleStageEvidenceContractV1(contract)
}

func (c QualificationContracts) rolesSHA256() string {
	if c.historicalSources != nil {
		return c.historicalSources["role_catalog"].SHA256
	}
	return systemgraph.RolesArtifactSHA256()
}

func CurrentRoleSquadReadinessMatrixV2() (RoleSquadReadinessMatrixV2, error) {
	return (QualificationContracts{}).CurrentRoleSquadReadinessMatrixV2()
}

func ValidateRoleSquadReadinessMatrixV2(matrix RoleSquadReadinessMatrixV2) error {
	return (QualificationContracts{}).ValidateRoleSquadReadinessMatrixV2(matrix)
}

func CurrentRoleSquadReadinessMatrixV3() (RoleSquadReadinessMatrixV3, error) {
	return (QualificationContracts{}).CurrentRoleSquadReadinessMatrixV3()
}

func ValidateRoleSquadReadinessMatrixV3(matrix RoleSquadReadinessMatrixV3) error {
	return (QualificationContracts{}).ValidateRoleSquadReadinessMatrixV3(matrix)
}

func CurrentRoleStageEvidenceRehearsalV1() (RoleStageEvidenceRehearsalResultV1, error) {
	return (QualificationContracts{}).CurrentRoleStageEvidenceRehearsalV1()
}

func RunRoleStageEvidenceRehearsalV1() (RoleStageEvidenceRehearsalResultV1, []RoleStageEvidenceRehearsalReceiptV1, error) {
	return (QualificationContracts{}).RunRoleStageEvidenceRehearsalV1()
}

func ValidateRoleStageEvidenceRehearsalV1(result RoleStageEvidenceRehearsalResultV1, receipts []RoleStageEvidenceRehearsalReceiptV1) error {
	return (QualificationContracts{}).ValidateRoleStageEvidenceRehearsalV1(result, receipts)
}

func CurrentClaudeSubscriptionStaticV34Freeze() (ClaudeSubscriptionStaticV34Freeze, error) {
	return (QualificationContracts{}).CurrentClaudeSubscriptionStaticV34Freeze()
}

func ValidateClaudeSubscriptionStaticV34Freeze(f ClaudeSubscriptionStaticV34Freeze) error {
	return (QualificationContracts{}).ValidateClaudeSubscriptionStaticV34Freeze(f)
}

func CurrentClaudeSubscriptionStaticV35Freeze() (ClaudeSubscriptionStaticV35Freeze, error) {
	return (QualificationContracts{}).CurrentClaudeSubscriptionStaticV35Freeze()
}

func ValidateClaudeSubscriptionStaticV35Freeze(freeze ClaudeSubscriptionStaticV35Freeze) error {
	return (QualificationContracts{}).ValidateClaudeSubscriptionStaticV35Freeze(freeze)
}
