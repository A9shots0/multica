package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

const ScreeningBatchSchemaVersion = "silver-mediums.agent-screening-batch.v1"

type ScreeningCase struct {
	CaseID                string `json:"case_id"`
	CaseVersion           string `json:"case_version"`
	CandidatePacketSHA256 string `json:"candidate_packet_sha256"`
	GraderPacketSHA256    string `json:"grader_packet_sha256"`
}

type CapacityBudget struct {
	Provider                      string `json:"provider"`
	LimitID                       string `json:"limit_id"`
	AccountLane                   string `json:"account_lane"`
	MaxUsedIncreaseBasisPoints    int    `json:"max_used_increase_basis_points"`
	MaxSnapshotAgeSeconds         int    `json:"max_snapshot_age_seconds"`
	RequireFreshAuthoritativeRead bool   `json:"require_fresh_authoritative_read"`
}

type ScreeningBatchDefinition struct {
	SchemaVersion              string           `json:"schema_version"`
	BatchID                    string           `json:"batch_id"`
	Status                     string           `json:"status"`
	SuiteVersion               string           `json:"suite_version"`
	ExecutionMode              string           `json:"execution_mode"`
	CandidateTransport         string           `json:"candidate_transport,omitempty"`
	ExecutionOrderPolicy       string           `json:"execution_order_policy"`
	ExecutionOrderSeed         string           `json:"execution_order_seed"`
	GraderBlindToCandidate     bool             `json:"grader_blind_to_candidate"`
	GraderConfigurationID      string           `json:"grader_configuration_id"`
	Cases                      []ScreeningCase  `json:"cases"`
	Candidates                 []Configuration  `json:"candidates"`
	Repeats                    int              `json:"repeats"`
	TokenCeiling               int64            `json:"token_ceiling"`
	PerLaunchTokenReservation  int64            `json:"per_launch_token_reservation"`
	CapacityBudgets            []CapacityBudget `json:"capacity_budgets"`
	APIBudgets                 []APIBudget      `json:"api_budgets,omitempty"`
	MaxOwnerInterventions      int              `json:"max_owner_interventions"`
	ProductionAllowed          bool             `json:"production_allowed"`
	ExternalSideEffectsAllowed bool             `json:"external_side_effects_allowed"`
	NaturalLanguageIntakeRun   bool             `json:"natural_language_intake_run"`
	ExcludedCapabilities       []string         `json:"excluded_capabilities"`
	StopRules                  []string         `json:"stop_rules"`
}

//go:embed screening_case_t11_candidate_v1.json
var screeningCaseT11CandidateJSON []byte

//go:embed screening_case_t11_grader_v1.json
var screeningCaseT11GraderJSON []byte

//go:embed screening_case_t11_candidate_v2.json
var screeningCaseT11CandidateV2JSON []byte

//go:embed screening_case_t11_grader_v2.json
var screeningCaseT11GraderV2JSON []byte

//go:embed screening_case_t11_candidate_v3.json
var screeningCaseT11CandidateV3JSON []byte

//go:embed screening_case_t11_grader_v3.json
var screeningCaseT11GraderV3JSON []byte

//go:embed screening_case_t11_candidate_v4.json
var screeningCaseT11CandidateV4JSON []byte

//go:embed screening_case_t11_grader_v4.json
var screeningCaseT11GraderV4JSON []byte

//go:embed screening_case_t12_candidate_v1.json
var screeningCaseT12CandidateJSON []byte

//go:embed screening_case_t12_grader_v1.json
var screeningCaseT12GraderJSON []byte

//go:embed screening_case_t12_candidate_v2.json
var screeningCaseT12CandidateV2JSON []byte

//go:embed screening_case_t12_grader_v2.json
var screeningCaseT12GraderV2JSON []byte

//go:embed screening_case_t12_candidate_v3.json
var screeningCaseT12CandidateV3JSON []byte

//go:embed screening_case_t12_grader_v3.json
var screeningCaseT12GraderV3JSON []byte

//go:embed screening_case_t12_candidate_v4.json
var screeningCaseT12CandidateV4JSON []byte

//go:embed screening_case_t12_grader_v4.json
var screeningCaseT12GraderV4JSON []byte

//go:embed screening_case_t08_candidate_v1.json
var screeningCaseT08CandidateJSON []byte

//go:embed screening_case_t08_grader_v1.json
var screeningCaseT08GraderJSON []byte

//go:embed screening_case_t08_candidate_v2.json
var screeningCaseT08CandidateV2JSON []byte

//go:embed screening_case_t08_grader_v2.json
var screeningCaseT08GraderV2JSON []byte

//go:embed screening_case_t08_candidate_v3.json
var screeningCaseT08CandidateV3JSON []byte

//go:embed screening_case_t08_grader_v3.json
var screeningCaseT08GraderV3JSON []byte

//go:embed screening_case_t04_candidate_v1.json
var screeningCaseT04CandidateJSON []byte

//go:embed screening_case_t04_grader_v1.json
var screeningCaseT04GraderJSON []byte

//go:embed screening_case_t09_candidate_v1.json
var screeningCaseT09CandidateJSON []byte

//go:embed screening_case_t09_grader_v1.json
var screeningCaseT09GraderJSON []byte

//go:embed screening_case_t09_candidate_v2.json
var screeningCaseT09CandidateV2JSON []byte

//go:embed screening_case_t09_grader_v2.json
var screeningCaseT09GraderV2JSON []byte

//go:embed screening_case_t16_candidate_v1.json
var screeningCaseT16CandidateJSON []byte

//go:embed screening_case_t16_grader_v1.json
var screeningCaseT16GraderJSON []byte

//go:embed screening_case_t16_candidate_v2.json
var screeningCaseT16CandidateV2JSON []byte

//go:embed screening_case_t16_grader_v2.json
var screeningCaseT16GraderV2JSON []byte

//go:embed screening_case_t16_candidate_v3.json
var screeningCaseT16CandidateV3JSON []byte

//go:embed screening_case_t16_grader_v3.json
var screeningCaseT16GraderV3JSON []byte

//go:embed screening_candidate_instructions_v2.txt
var screeningCandidateInstructionsV2 []byte

//go:embed screening_candidate_instructions_v3.txt
var screeningCandidateInstructionsV3 []byte

//go:embed screening_candidate_instructions_v4.txt
var screeningCandidateInstructionsV4 []byte

//go:embed screening_candidate_instructions_v5.txt
var screeningCandidateInstructionsV5 []byte

//go:embed screening_skill_v2.md
var screeningSkillV2 []byte

func EmbeddedScreeningPacketVersionSHA256(caseID, caseVersion, audience string) (string, error) {
	var raw []byte
	switch caseID + ":" + caseVersion + ":" + audience {
	case "T11:v1:candidate":
		raw = screeningCaseT11CandidateJSON
	case "T11:v1:grader":
		raw = screeningCaseT11GraderJSON
	case "T11:v2:candidate":
		raw = screeningCaseT11CandidateV2JSON
	case "T11:v2:grader":
		raw = screeningCaseT11GraderV2JSON
	case "T11:v3:candidate":
		raw = screeningCaseT11CandidateV3JSON
	case "T11:v3:grader":
		raw = screeningCaseT11GraderV3JSON
	case "T11:v4:candidate":
		raw = screeningCaseT11CandidateV4JSON
	case "T11:v4:grader":
		raw = screeningCaseT11GraderV4JSON
	case "T12:v1:candidate":
		raw = screeningCaseT12CandidateJSON
	case "T12:v1:grader":
		raw = screeningCaseT12GraderJSON
	case "T12:v2:candidate":
		raw = screeningCaseT12CandidateV2JSON
	case "T12:v2:grader":
		raw = screeningCaseT12GraderV2JSON
	case "T12:v3:candidate":
		raw = screeningCaseT12CandidateV3JSON
	case "T12:v3:grader":
		raw = screeningCaseT12GraderV3JSON
	case "T12:v4:candidate":
		raw = screeningCaseT12CandidateV4JSON
	case "T12:v4:grader":
		raw = screeningCaseT12GraderV4JSON
	case "T08:v1:candidate":
		raw = screeningCaseT08CandidateJSON
	case "T08:v1:grader":
		raw = screeningCaseT08GraderJSON
	case "T08:v2:candidate":
		raw = screeningCaseT08CandidateV2JSON
	case "T08:v2:grader":
		raw = screeningCaseT08GraderV2JSON
	case "T08:v3:candidate":
		raw = screeningCaseT08CandidateV3JSON
	case "T08:v3:grader":
		raw = screeningCaseT08GraderV3JSON
	case "T04:v1:candidate":
		raw = screeningCaseT04CandidateJSON
	case "T04:v1:grader":
		raw = screeningCaseT04GraderJSON
	case "T09:v1:candidate":
		raw = screeningCaseT09CandidateJSON
	case "T09:v1:grader":
		raw = screeningCaseT09GraderJSON
	case "T09:v2:candidate":
		raw = screeningCaseT09CandidateV2JSON
	case "T09:v2:grader":
		raw = screeningCaseT09GraderV2JSON
	case "T16:v1:candidate":
		raw = screeningCaseT16CandidateJSON
	case "T16:v1:grader":
		raw = screeningCaseT16GraderJSON
	case "T16:v2:candidate":
		raw = screeningCaseT16CandidateV2JSON
	case "T16:v2:grader":
		raw = screeningCaseT16GraderV2JSON
	case "T16:v3:candidate":
		raw = screeningCaseT16CandidateV3JSON
	case "T16:v3:grader":
		raw = screeningCaseT16GraderV3JSON
	default:
		return "", fmt.Errorf("unknown screening packet %s:%s:%s", caseID, caseVersion, audience)
	}
	if !json.Valid(raw) {
		return "", fmt.Errorf("screening packet %s:%s:%s is invalid JSON", caseID, caseVersion, audience)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func EmbeddedScreeningPacketSHA256(caseID, audience string) (string, error) {
	return EmbeddedScreeningPacketVersionSHA256(caseID, "v1", audience)
}

// CandidateScreeningPacket returns only the public case packet. The hidden
// grader truth has no candidate-facing accessor and is never composed into the
// candidate prompt by this package.
func CandidateScreeningPacket(caseID string) ([]byte, error) {
	return CandidateScreeningPacketVersion(caseID, "v1")
}

func CandidateScreeningPacketVersion(caseID, caseVersion string) ([]byte, error) {
	switch caseID + ":" + caseVersion {
	case "T11:v1":
		return append([]byte(nil), screeningCaseT11CandidateJSON...), nil
	case "T11:v2":
		return append([]byte(nil), screeningCaseT11CandidateV2JSON...), nil
	case "T11:v3":
		return append([]byte(nil), screeningCaseT11CandidateV3JSON...), nil
	case "T11:v4":
		return append([]byte(nil), screeningCaseT11CandidateV4JSON...), nil
	case "T12:v1":
		return append([]byte(nil), screeningCaseT12CandidateJSON...), nil
	case "T12:v2":
		return append([]byte(nil), screeningCaseT12CandidateV2JSON...), nil
	case "T12:v3":
		return append([]byte(nil), screeningCaseT12CandidateV3JSON...), nil
	case "T12:v4":
		return append([]byte(nil), screeningCaseT12CandidateV4JSON...), nil
	case "T08:v1":
		return append([]byte(nil), screeningCaseT08CandidateJSON...), nil
	case "T08:v2":
		return append([]byte(nil), screeningCaseT08CandidateV2JSON...), nil
	case "T08:v3":
		return append([]byte(nil), screeningCaseT08CandidateV3JSON...), nil
	case "T04:v1":
		return append([]byte(nil), screeningCaseT04CandidateJSON...), nil
	case "T09:v1":
		return append([]byte(nil), screeningCaseT09CandidateJSON...), nil
	case "T09:v2":
		return append([]byte(nil), screeningCaseT09CandidateV2JSON...), nil
	case "T16:v1":
		return append([]byte(nil), screeningCaseT16CandidateJSON...), nil
	case "T16:v2":
		return append([]byte(nil), screeningCaseT16CandidateV2JSON...), nil
	case "T16:v3":
		return append([]byte(nil), screeningCaseT16CandidateV3JSON...), nil
	default:
		return nil, fmt.Errorf("unknown candidate screening packet %s:%s", caseID, caseVersion)
	}
}

func ExpectedBlindGraderVersion(caseID, caseVersion string) (string, error) {
	var raw []byte
	switch caseID + ":" + caseVersion {
	case "T11:v1":
		raw = screeningCaseT11GraderJSON
	case "T11:v2":
		raw = screeningCaseT11GraderV2JSON
	case "T11:v3":
		raw = screeningCaseT11GraderV3JSON
	case "T11:v4":
		raw = screeningCaseT11GraderV4JSON
	case "T12:v1":
		raw = screeningCaseT12GraderJSON
	case "T12:v2":
		raw = screeningCaseT12GraderV2JSON
	case "T12:v3":
		raw = screeningCaseT12GraderV3JSON
	case "T12:v4":
		raw = screeningCaseT12GraderV4JSON
	case "T08:v1":
		raw = screeningCaseT08GraderJSON
	case "T08:v2":
		raw = screeningCaseT08GraderV2JSON
	case "T08:v3":
		raw = screeningCaseT08GraderV3JSON
	case "T04:v1":
		raw = screeningCaseT04GraderJSON
	case "T09:v1":
		raw = screeningCaseT09GraderJSON
	case "T09:v2":
		raw = screeningCaseT09GraderV2JSON
	case "T16:v1":
		raw = screeningCaseT16GraderJSON
	case "T16:v2":
		raw = screeningCaseT16GraderV2JSON
	case "T16:v3":
		raw = screeningCaseT16GraderV3JSON
	default:
		return "", fmt.Errorf("unknown grader screening packet %s:%s", caseID, caseVersion)
	}
	var packet struct {
		GraderVersion string `json:"grader_version"`
	}
	if err := json.Unmarshal(raw, &packet); err != nil || packet.GraderVersion == "" {
		return "", fmt.Errorf("grader screening packet %s:%s has no valid grader version", caseID, caseVersion)
	}
	return packet.GraderVersion, nil
}

type BatchProjection struct {
	Observations        int   `json:"observations"`
	ReservedTokens      int64 `json:"reserved_tokens"`
	FitsDeclaredCeiling bool  `json:"fits_declared_ceiling"`
}

type CapacitySnapshot struct {
	Provider        string `json:"provider"`
	LimitID         string `json:"limit_id"`
	AccountLane     string `json:"account_lane"`
	WindowID        string `json:"window_id"`
	UsedBasisPoints int    `json:"used_basis_points"`
	ObservedAt      string `json:"observed_at"`
	Source          string `json:"source"`
}

type CapacityAdmission struct {
	CanLaunch bool     `json:"can_launch"`
	Reasons   []string `json:"reasons,omitempty"`
}

type ScreeningLaunchAdmission struct {
	CanLaunch               bool              `json:"can_launch"`
	Reasons                 []string          `json:"reasons,omitempty"`
	AuditTriggers           []string          `json:"audit_triggers,omitempty"`
	Usage                   ObservationTotals `json:"usage"`
	Planned                 BatchProjection   `json:"planned"`
	CompletedCount          int               `json:"completed_count"`
	SkippedAssignmentCount  int               `json:"skipped_assignment_count,omitempty"`
	TerminalAssignmentCount int               `json:"terminal_assignment_count"`
	EliminatedCandidateIDs  []string          `json:"eliminated_candidate_ids,omitempty"`
	InvalidCandidateIDs     []string          `json:"invalid_candidate_ids,omitempty"`
}

//go:embed screening_batch_v1.json
var screeningBatchJSON []byte

//go:embed screening_batch_v2.json
var screeningBatchV2JSON []byte

//go:embed screening_batch_v3.json
var screeningBatchV3JSON []byte

//go:embed screening_batch_v4.json
var screeningBatchV4JSON []byte

//go:embed screening_batch_v5.json
var screeningBatchV5JSON []byte

//go:embed screening_batch_openrouter_v1.json
var screeningBatchOpenRouterV1JSON []byte

//go:embed screening_batch_openrouter_v2.json
var screeningBatchOpenRouterV2JSON []byte

//go:embed screening_batch_openrouter_v3.json
var screeningBatchOpenRouterV3JSON []byte

//go:embed screening_batch_openrouter_v4.json
var screeningBatchOpenRouterV4JSON []byte

//go:embed screening_batch_openrouter_v5.json
var screeningBatchOpenRouterV5JSON []byte

//go:embed screening_batch_openrouter_v6.json
var screeningBatchOpenRouterV6JSON []byte

//go:embed screening_batch_openrouter_v7.json
var screeningBatchOpenRouterV7JSON []byte

//go:embed screening_batch_openrouter_v8.json
var screeningBatchOpenRouterV8JSON []byte

//go:embed screening_batch_openrouter_v9.json
var screeningBatchOpenRouterV9JSON []byte

//go:embed screening_batch_openrouter_v10.json
var screeningBatchOpenRouterV10JSON []byte

//go:embed screening_batch_openrouter_v11.json
var screeningBatchOpenRouterV11JSON []byte

//go:embed screening_batch_openrouter_v12.json
var screeningBatchOpenRouterV12JSON []byte

//go:embed screening_batch_openrouter_v13.json
var screeningBatchOpenRouterV13JSON []byte

//go:embed screening_batch_openrouter_v14.json
var screeningBatchOpenRouterV14JSON []byte

//go:embed screening_batch_openrouter_v15.json
var screeningBatchOpenRouterV15JSON []byte

//go:embed screening_batch_openrouter_v16.json
var screeningBatchOpenRouterV16JSON []byte

//go:embed screening_batch_openrouter_v17.json
var screeningBatchOpenRouterV17JSON []byte

//go:embed screening_batch_openrouter_v18.json
var screeningBatchOpenRouterV18JSON []byte

//go:embed screening_batch_openrouter_v19.json
var screeningBatchOpenRouterV19JSON []byte

//go:embed screening_batch_openrouter_v20.json
var screeningBatchOpenRouterV20JSON []byte

//go:embed screening_batch_openrouter_v21.json
var screeningBatchOpenRouterV21JSON []byte

//go:embed screening_batch_openrouter_v22.json
var screeningBatchOpenRouterV22JSON []byte

//go:embed screening_batch_openrouter_v23.json
var screeningBatchOpenRouterV23JSON []byte

//go:embed screening_batch_openrouter_v24.json
var screeningBatchOpenRouterV24JSON []byte

//go:embed screening_batch_openrouter_v25.json
var screeningBatchOpenRouterV25JSON []byte

//go:embed screening_batch_openrouter_v26.json
var screeningBatchOpenRouterV26JSON []byte

var (
	screeningBatchOnce sync.Once
	screeningBatch     ScreeningBatchDefinition
	screeningBatchErr  error
)

func CurrentScreeningBatch() (ScreeningBatchDefinition, error) {
	var batch ScreeningBatchDefinition
	if err := json.Unmarshal(screeningBatchV5JSON, &batch); err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("decode current screening batch: %w", err)
	}
	if err := ValidateScreeningBatch(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	return batch, nil
}

func OpenRouterScreeningBatch() (ScreeningBatchDefinition, error) {
	return TwentySixthOpenRouterScreeningBatchPlan()
}

func TwentySixthOpenRouterScreeningBatchPlan() (ScreeningBatchDefinition, error) {
	var batch ScreeningBatchDefinition
	if err := json.Unmarshal(screeningBatchOpenRouterV26JSON, &batch); err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("decode twenty-sixth OpenRouter screening batch: %w", err)
	}
	if err := ValidateScreeningBatch(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	if err := ValidateScreeningInstructionCompatibility(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	return batch, nil
}

func TwentyFifthOpenRouterScreeningBatchPlan() (ScreeningBatchDefinition, error) {
	var batch ScreeningBatchDefinition
	if err := json.Unmarshal(screeningBatchOpenRouterV25JSON, &batch); err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("decode twenty-fifth OpenRouter screening batch: %w", err)
	}
	if err := ValidateScreeningBatch(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	if err := ValidateScreeningInstructionCompatibility(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	return batch, nil
}

func TwentyFourthOpenRouterScreeningBatchPlan() (ScreeningBatchDefinition, error) {
	var batch ScreeningBatchDefinition
	if err := json.Unmarshal(screeningBatchOpenRouterV24JSON, &batch); err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("decode twenty-fourth OpenRouter screening batch: %w", err)
	}
	if err := ValidateScreeningBatch(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	if err := ValidateScreeningInstructionCompatibility(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	return batch, nil
}

func TwentyThirdOpenRouterScreeningBatchPlan() (ScreeningBatchDefinition, error) {
	var batch ScreeningBatchDefinition
	if err := json.Unmarshal(screeningBatchOpenRouterV23JSON, &batch); err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("decode twenty-third OpenRouter screening batch: %w", err)
	}
	if err := ValidateScreeningBatch(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	if err := ValidateScreeningInstructionCompatibility(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	return batch, nil
}

func TwentySecondOpenRouterScreeningBatchPlan() (ScreeningBatchDefinition, error) {
	var batch ScreeningBatchDefinition
	if err := json.Unmarshal(screeningBatchOpenRouterV22JSON, &batch); err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("decode twenty-second OpenRouter screening batch: %w", err)
	}
	if err := ValidateScreeningBatch(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	if err := ValidateScreeningInstructionCompatibility(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	return batch, nil
}

func TwentyFirstOpenRouterScreeningBatchPlan() (ScreeningBatchDefinition, error) {
	var batch ScreeningBatchDefinition
	if err := json.Unmarshal(screeningBatchOpenRouterV21JSON, &batch); err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("decode twenty-first OpenRouter screening batch: %w", err)
	}
	if err := ValidateScreeningBatch(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	if err := ValidateScreeningInstructionCompatibility(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	return batch, nil
}

func TwentiethOpenRouterScreeningBatchPlan() (ScreeningBatchDefinition, error) {
	var batch ScreeningBatchDefinition
	if err := json.Unmarshal(screeningBatchOpenRouterV20JSON, &batch); err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("decode twentieth OpenRouter screening batch: %w", err)
	}
	if err := ValidateScreeningBatch(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	if err := ValidateScreeningInstructionCompatibility(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	return batch, nil
}

func NineteenthOpenRouterScreeningBatchPlan() (ScreeningBatchDefinition, error) {
	var batch ScreeningBatchDefinition
	if err := json.Unmarshal(screeningBatchOpenRouterV19JSON, &batch); err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("decode nineteenth OpenRouter screening batch: %w", err)
	}
	if err := ValidateScreeningBatch(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	if err := ValidateScreeningInstructionCompatibility(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	return batch, nil
}

func EighteenthOpenRouterScreeningBatchPlan() (ScreeningBatchDefinition, error) {
	var batch ScreeningBatchDefinition
	if err := json.Unmarshal(screeningBatchOpenRouterV18JSON, &batch); err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("decode eighteenth OpenRouter screening batch: %w", err)
	}
	if err := ValidateScreeningBatch(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	if err := ValidateScreeningInstructionCompatibility(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	return batch, nil
}

func SeventeenthOpenRouterScreeningBatchPlan() (ScreeningBatchDefinition, error) {
	var batch ScreeningBatchDefinition
	if err := json.Unmarshal(screeningBatchOpenRouterV17JSON, &batch); err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("decode seventeenth OpenRouter screening batch: %w", err)
	}
	if err := ValidateScreeningBatch(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	if err := ValidateScreeningInstructionCompatibility(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	return batch, nil
}

func SixteenthOpenRouterScreeningBatchPlan() (ScreeningBatchDefinition, error) {
	var batch ScreeningBatchDefinition
	if err := json.Unmarshal(screeningBatchOpenRouterV16JSON, &batch); err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("decode sixteenth OpenRouter screening batch: %w", err)
	}
	if err := ValidateScreeningBatch(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	if err := ValidateScreeningInstructionCompatibility(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	return batch, nil
}

func FifteenthOpenRouterScreeningBatchPlan() (ScreeningBatchDefinition, error) {
	var batch ScreeningBatchDefinition
	if err := json.Unmarshal(screeningBatchOpenRouterV15JSON, &batch); err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("decode fifteenth OpenRouter screening batch: %w", err)
	}
	if err := ValidateScreeningBatch(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	if err := ValidateScreeningInstructionCompatibility(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	return batch, nil
}

func FourteenthOpenRouterScreeningBatchPlan() (ScreeningBatchDefinition, error) {
	var batch ScreeningBatchDefinition
	if err := json.Unmarshal(screeningBatchOpenRouterV14JSON, &batch); err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("decode fourteenth OpenRouter screening batch: %w", err)
	}
	if err := ValidateScreeningBatch(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	if err := ValidateScreeningInstructionCompatibility(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	return batch, nil
}

func ThirteenthOpenRouterScreeningBatchPlan() (ScreeningBatchDefinition, error) {
	var batch ScreeningBatchDefinition
	if err := json.Unmarshal(screeningBatchOpenRouterV13JSON, &batch); err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("decode thirteenth OpenRouter screening batch: %w", err)
	}
	if err := ValidateScreeningBatch(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	if err := ValidateScreeningInstructionCompatibility(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	return batch, nil
}

func TwelfthOpenRouterScreeningBatch() (ScreeningBatchDefinition, error) {
	var batch ScreeningBatchDefinition
	if err := json.Unmarshal(screeningBatchOpenRouterV12JSON, &batch); err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("decode twelfth OpenRouter screening batch: %w", err)
	}
	if err := ValidateScreeningBatch(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	if err := ValidateScreeningInstructionCompatibility(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	return batch, nil
}

func EleventhOpenRouterScreeningBatch() (ScreeningBatchDefinition, error) {
	var batch ScreeningBatchDefinition
	if err := json.Unmarshal(screeningBatchOpenRouterV11JSON, &batch); err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("decode eleventh OpenRouter screening batch: %w", err)
	}
	if err := ValidateScreeningBatch(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	if err := ValidateScreeningInstructionCompatibility(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	return batch, nil
}

func TenthOpenRouterScreeningBatch() (ScreeningBatchDefinition, error) {
	var batch ScreeningBatchDefinition
	if err := json.Unmarshal(screeningBatchOpenRouterV10JSON, &batch); err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("decode tenth OpenRouter screening batch: %w", err)
	}
	if err := ValidateScreeningBatch(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	if err := ValidateScreeningInstructionCompatibility(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	return batch, nil
}

func NinthOpenRouterScreeningBatch() (ScreeningBatchDefinition, error) {
	var batch ScreeningBatchDefinition
	if err := json.Unmarshal(screeningBatchOpenRouterV9JSON, &batch); err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("decode ninth OpenRouter screening batch: %w", err)
	}
	if err := ValidateScreeningBatch(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	if err := ValidateScreeningInstructionCompatibility(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	return batch, nil
}

func EighthOpenRouterScreeningBatch() (ScreeningBatchDefinition, error) {
	var batch ScreeningBatchDefinition
	if err := json.Unmarshal(screeningBatchOpenRouterV8JSON, &batch); err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("decode eighth OpenRouter screening batch: %w", err)
	}
	if err := ValidateScreeningBatch(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	if err := ValidateScreeningInstructionCompatibility(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	return batch, nil
}

func SeventhOpenRouterScreeningBatch() (ScreeningBatchDefinition, error) {
	var batch ScreeningBatchDefinition
	if err := json.Unmarshal(screeningBatchOpenRouterV7JSON, &batch); err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("decode seventh OpenRouter screening batch: %w", err)
	}
	if err := ValidateScreeningBatch(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	if err := ValidateScreeningInstructionCompatibility(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	return batch, nil
}

func SixthOpenRouterScreeningBatch() (ScreeningBatchDefinition, error) {
	var batch ScreeningBatchDefinition
	if err := json.Unmarshal(screeningBatchOpenRouterV6JSON, &batch); err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("decode sixth OpenRouter screening batch: %w", err)
	}
	if err := ValidateScreeningBatch(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	if err := ValidateScreeningInstructionCompatibility(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	return batch, nil
}

func FifthOpenRouterScreeningBatch() (ScreeningBatchDefinition, error) {
	var batch ScreeningBatchDefinition
	if err := json.Unmarshal(screeningBatchOpenRouterV5JSON, &batch); err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("decode fifth OpenRouter screening batch: %w", err)
	}
	if err := ValidateScreeningBatch(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	if err := ValidateScreeningInstructionCompatibility(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	return batch, nil
}

func FourthOpenRouterScreeningBatch() (ScreeningBatchDefinition, error) {
	var batch ScreeningBatchDefinition
	if err := json.Unmarshal(screeningBatchOpenRouterV4JSON, &batch); err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("decode fourth OpenRouter screening batch: %w", err)
	}
	if err := ValidateScreeningBatch(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	if err := ValidateScreeningInstructionCompatibility(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	return batch, nil
}

func ThirdOpenRouterScreeningBatch() (ScreeningBatchDefinition, error) {
	var batch ScreeningBatchDefinition
	if err := json.Unmarshal(screeningBatchOpenRouterV3JSON, &batch); err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("decode third OpenRouter screening batch: %w", err)
	}
	if err := ValidateScreeningBatch(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	if err := ValidateScreeningInstructionCompatibility(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	return batch, nil
}

func SecondOpenRouterScreeningBatch() (ScreeningBatchDefinition, error) {
	var batch ScreeningBatchDefinition
	if err := json.Unmarshal(screeningBatchOpenRouterV2JSON, &batch); err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("decode second OpenRouter screening batch: %w", err)
	}
	if err := ValidateScreeningBatch(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	if err := ValidateScreeningInstructionCompatibility(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	return batch, nil
}

func FirstOpenRouterScreeningBatch() (ScreeningBatchDefinition, error) {
	var batch ScreeningBatchDefinition
	if err := json.Unmarshal(screeningBatchOpenRouterV1JSON, &batch); err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("decode first OpenRouter screening batch: %w", err)
	}
	if err := ValidateScreeningBatch(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	if err := ValidateScreeningInstructionCompatibility(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	return batch, nil
}

func FourthScreeningBatch() (ScreeningBatchDefinition, error) {
	var batch ScreeningBatchDefinition
	if err := json.Unmarshal(screeningBatchV4JSON, &batch); err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("decode fourth screening batch: %w", err)
	}
	if err := ValidateScreeningBatch(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	return batch, nil
}

func ThirdScreeningBatch() (ScreeningBatchDefinition, error) {
	var batch ScreeningBatchDefinition
	if err := json.Unmarshal(screeningBatchV3JSON, &batch); err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("decode third screening batch: %w", err)
	}
	if err := ValidateScreeningBatch(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	return batch, nil
}

func SecondScreeningBatch() (ScreeningBatchDefinition, error) {
	var batch ScreeningBatchDefinition
	if err := json.Unmarshal(screeningBatchV2JSON, &batch); err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("decode second screening batch: %w", err)
	}
	if err := ValidateScreeningBatch(batch); err != nil {
		return ScreeningBatchDefinition{}, err
	}
	return batch, nil
}

func FirstScreeningBatch() (ScreeningBatchDefinition, error) {
	screeningBatchOnce.Do(func() {
		if err := json.Unmarshal(screeningBatchJSON, &screeningBatch); err != nil {
			screeningBatchErr = fmt.Errorf("decode screening batch: %w", err)
			return
		}
		screeningBatchErr = ValidateScreeningBatch(screeningBatch)
	})
	if screeningBatchErr != nil {
		return ScreeningBatchDefinition{}, screeningBatchErr
	}
	raw, err := json.Marshal(screeningBatch)
	if err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("copy screening batch: %w", err)
	}
	var result ScreeningBatchDefinition
	if err := json.Unmarshal(raw, &result); err != nil {
		return ScreeningBatchDefinition{}, fmt.Errorf("copy screening batch: %w", err)
	}
	return result, nil
}

func ValidateScreeningBatch(batch ScreeningBatchDefinition) error {
	if batch.SchemaVersion != ScreeningBatchSchemaVersion || strings.TrimSpace(batch.BatchID) == "" || strings.TrimSpace(batch.SuiteVersion) == "" {
		return fmt.Errorf("batch identity is incomplete")
	}
	if batch.Status != "planned_not_launched" {
		return fmt.Errorf("first screening batch must remain planned_not_launched")
	}
	if batch.ExecutionMode != "isolated_capability" || batch.NaturalLanguageIntakeRun {
		return fmt.Errorf("screening batch must use isolated capability execution without an intake run")
	}
	if batch.ExecutionOrderPolicy != "seeded_permutation_v1" || strings.TrimSpace(batch.ExecutionOrderSeed) == "" {
		return fmt.Errorf("screening batch requires a recorded seeded execution order")
	}
	if !batch.GraderBlindToCandidate || strings.TrimSpace(batch.GraderConfigurationID) == "" {
		return fmt.Errorf("screening batch requires a pinned candidate-blind grader")
	}
	if batch.ProductionAllowed || batch.ExternalSideEffectsAllowed {
		return fmt.Errorf("screening batch cannot allow production or external side effects")
	}
	singleCandidateOnly := contains(batch.StopRules, "comparison_anchor_only_no_role_promotion") || contains(batch.StopRules, "post_halt_recovery_only_no_role_promotion") || contains(batch.StopRules, "reliability_repeat_only_no_role_promotion")
	if len(batch.Cases) == 0 || len(batch.Candidates) == 0 || batch.Repeats < 1 || (len(batch.Candidates) < 2 && !singleCandidateOnly) {
		return fmt.Errorf("screening batch needs cases, competing candidates (or an explicit comparison-only anchor), and repeats")
	}
	if batch.TokenCeiling <= 0 || batch.PerLaunchTokenReservation <= 0 {
		return fmt.Errorf("screening batch token limits must be positive")
	}
	projection := ProjectScreeningBatch(batch)
	if !projection.FitsDeclaredCeiling {
		return fmt.Errorf("planned observations exceed the declared token ceiling")
	}
	if batch.MaxOwnerInterventions != 0 {
		return fmt.Errorf("isolated screening cannot require owner intervention")
	}
	knownCases, err := Cases()
	if err != nil {
		return err
	}
	caseIDs := make(map[string]struct{}, len(knownCases))
	for _, c := range knownCases {
		caseIDs[c.ID] = struct{}{}
	}
	seenCases := map[string]struct{}{}
	for _, c := range batch.Cases {
		if c.CaseVersion == "" {
			return fmt.Errorf("case %s has no version", c.CaseID)
		}
		if _, ok := caseIDs[c.CaseID]; !ok {
			return fmt.Errorf("unknown case %s", c.CaseID)
		}
		candidateHash, err := EmbeddedScreeningPacketVersionSHA256(c.CaseID, c.CaseVersion, "candidate")
		if err != nil || c.CandidatePacketSHA256 != candidateHash {
			return fmt.Errorf("case %s candidate packet hash mismatch", c.CaseID)
		}
		graderHash, err := EmbeddedScreeningPacketVersionSHA256(c.CaseID, c.CaseVersion, "grader")
		if err != nil || c.GraderPacketSHA256 != graderHash {
			return fmt.Errorf("case %s grader packet hash mismatch", c.CaseID)
		}
		if _, duplicate := seenCases[c.CaseID]; duplicate {
			return fmt.Errorf("duplicate case %s", c.CaseID)
		}
		seenCases[c.CaseID] = struct{}{}
	}
	seenCandidates := map[string]struct{}{}
	var comparisonShape Configuration
	for index, candidate := range batch.Candidates {
		if err := ValidateConfiguration(candidate); err != nil {
			return fmt.Errorf("candidate %s: %w", candidate.ID, err)
		}
		if _, duplicate := seenCandidates[candidate.ID]; duplicate {
			return fmt.Errorf("duplicate candidate %s", candidate.ID)
		}
		seenCandidates[candidate.ID] = struct{}{}
		if index == 0 {
			comparisonShape = candidate
			continue
		}
		if candidate.RoleID != comparisonShape.RoleID ||
			candidate.InstructionVersion != comparisonShape.InstructionVersion ||
			candidate.InstructionSHA256 != comparisonShape.InstructionSHA256 ||
			candidate.PermissionVersion != comparisonShape.PermissionVersion ||
			candidate.RuntimeProvider != comparisonShape.RuntimeProvider ||
			!equalStringMap(candidate.SkillVersions, comparisonShape.SkillVersions) ||
			!equalStringMap(candidate.SkillContentSHA256, comparisonShape.SkillContentSHA256) {
			return fmt.Errorf("candidate %s changes more than model configuration", candidate.ID)
		}
	}
	if (len(batch.CapacityBudgets) == 0) == (len(batch.APIBudgets) == 0) {
		return fmt.Errorf("screening batch requires exactly one subscription or API budget family")
	}
	for _, budget := range batch.CapacityBudgets {
		if budget.Provider == "" || budget.LimitID == "" || budget.AccountLane == "" || budget.MaxUsedIncreaseBasisPoints <= 0 || budget.MaxSnapshotAgeSeconds <= 0 || !budget.RequireFreshAuthoritativeRead {
			return fmt.Errorf("capacity budget is incomplete or fail-open")
		}
	}
	for _, budget := range batch.APIBudgets {
		if strings.TrimSpace(budget.Provider) == "" || strings.TrimSpace(budget.ProjectLane) == "" || budget.Currency != "USD" || strings.TrimSpace(budget.WindowID) == "" || budget.MaxSpendMicros <= 0 || budget.PerRequestReserveMicros <= 0 || budget.MaxSnapshotAgeSeconds <= 0 {
			return fmt.Errorf("API budget is incomplete or fail-open")
		}
	}
	if len(batch.StopRules) == 0 || len(batch.ExcludedCapabilities) == 0 {
		return fmt.Errorf("screening boundaries are incomplete")
	}
	return nil
}

// ValidateScreeningInstructionCompatibility prevents a candidate-facing
// instruction contract from contradicting the case packet before paid work is
// admitted. Historical v1 is deliberately rejected: it required an array
// while T11 requires one object.
func ValidateScreeningInstructionCompatibility(batch ScreeningBatchDefinition) error {
	if batch.CandidateTransport != "comment_trigger_inline_v1" && batch.CandidateTransport != "direct_codex_exec_jsonl_v1" && batch.CandidateTransport != OpenCodeToolFreeTransport && batch.CandidateTransport != OpenCodeToolFreeTransportV2 && batch.CandidateTransport != OpenCodeToolFreeTransportV3 && batch.CandidateTransport != OpenCodeToolFreeTransportV4 && batch.CandidateTransport != OpenCodeToolFreeTransportV5 && batch.CandidateTransport != OpenCodeToolFreeTransportV6 && batch.CandidateTransport != ClaudeCodeToolFreeTransport {
		return fmt.Errorf("candidate packet transport does not place case facts in the run prompt")
	}
	for _, screeningCase := range batch.Cases {
		if err := validatePublicResponseContract(screeningCase.CaseID, screeningCase.CaseVersion); err != nil {
			return err
		}
	}
	for _, candidate := range batch.Candidates {
		switch candidate.InstructionVersion {
		case "shadow-candidate.v5":
			sum := sha256.Sum256(screeningCandidateInstructionsV5)
			if candidate.InstructionSHA256 != hex.EncodeToString(sum[:]) {
				return fmt.Errorf("candidate %s instruction content hash mismatch", candidate.ID)
			}
			if ((batch.CandidateTransport != OpenCodeToolFreeTransport && batch.CandidateTransport != OpenCodeToolFreeTransportV2 && batch.CandidateTransport != OpenCodeToolFreeTransportV3 && batch.CandidateTransport != OpenCodeToolFreeTransportV4 && batch.CandidateTransport != OpenCodeToolFreeTransportV5 && batch.CandidateTransport != OpenCodeToolFreeTransportV6) || candidate.RuntimeProvider != OpenCodeCLIRuntimeProvider) && (batch.CandidateTransport != ClaudeCodeToolFreeTransport || candidate.RuntimeProvider != "claude_code_subscription") {
				return fmt.Errorf("candidate %s v5 instructions require a registered tool-free transport and matching runtime", candidate.ID)
			}
		case "shadow-candidate.v4":
			sum := sha256.Sum256(screeningCandidateInstructionsV4)
			if candidate.InstructionSHA256 != hex.EncodeToString(sum[:]) {
				return fmt.Errorf("candidate %s instruction content hash mismatch", candidate.ID)
			}
			if batch.CandidateTransport != "direct_codex_exec_jsonl_v1" {
				return fmt.Errorf("candidate %s direct instructions require direct transport", candidate.ID)
			}
		case "shadow-candidate.v3":
			sum := sha256.Sum256(screeningCandidateInstructionsV3)
			if candidate.InstructionSHA256 != hex.EncodeToString(sum[:]) {
				return fmt.Errorf("candidate %s instruction content hash mismatch", candidate.ID)
			}
		case "shadow-candidate.v2":
			sum := sha256.Sum256(screeningCandidateInstructionsV2)
			if candidate.InstructionSHA256 != hex.EncodeToString(sum[:]) {
				return fmt.Errorf("candidate %s instruction content hash mismatch", candidate.ID)
			}
		case "shadow-candidate.v1":
			return fmt.Errorf("candidate %s instruction response container conflicts with T11 object contract", candidate.ID)
		default:
			return fmt.Errorf("candidate %s instruction contract is not registered", candidate.ID)
		}
		for name, digest := range candidate.SkillContentSHA256 {
			if strings.TrimSpace(name) == "" || len(digest) != sha256.Size*2 {
				return fmt.Errorf("candidate %s skill content hash is invalid", candidate.ID)
			}
			if _, err := hex.DecodeString(digest); err != nil || digest != strings.ToLower(digest) {
				return fmt.Errorf("candidate %s skill content hash is invalid", candidate.ID)
			}
		}
		if len(candidate.SkillContentSHA256) != len(candidate.SkillVersions) {
			return fmt.Errorf("candidate %s does not pin every skill content hash", candidate.ID)
		}
		if version := candidate.SkillVersions["silver_mediums_context"]; version == "item14.v1" {
			return fmt.Errorf("candidate %s skill response contract conflicts with the case packet", candidate.ID)
		} else if version == "item14.v2" || version == "item14.v2-inline" {
			sum := sha256.Sum256(screeningSkillV2)
			if candidate.SkillContentSHA256["silver_mediums_context"] != hex.EncodeToString(sum[:]) {
				return fmt.Errorf("candidate %s skill content does not match registered v2", candidate.ID)
			}
			if !strings.Contains(string(screeningSkillV2), "only authority for response fields and JSON") || strings.Contains(string(screeningSkillV2), "For every numbered case, return exactly") {
				return fmt.Errorf("candidate %s skill defines a competing response contract", candidate.ID)
			}
		} else {
			return fmt.Errorf("candidate %s skill contract is not registered", candidate.ID)
		}
	}
	if (batch.CandidateTransport == "direct_codex_exec_jsonl_v1" || batch.CandidateTransport == OpenCodeToolFreeTransport || batch.CandidateTransport == OpenCodeToolFreeTransportV2 || batch.CandidateTransport == OpenCodeToolFreeTransportV3 || batch.CandidateTransport == OpenCodeToolFreeTransportV4 || batch.CandidateTransport == OpenCodeToolFreeTransportV5 || batch.CandidateTransport == OpenCodeToolFreeTransportV6 || batch.CandidateTransport == ClaudeCodeToolFreeTransport) && !containsString(batch.ExcludedCapabilities, "tools") {
		return fmt.Errorf("screening transport must exclude tools")
	}
	return nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func validatePublicResponseContract(caseID, caseVersion string) error {
	packet, err := CandidateScreeningPacketVersion(caseID, caseVersion)
	if err != nil {
		return err
	}
	var decoded struct {
		FieldRequirements map[string]string `json:"field_requirements"`
		ResponseContract  struct {
			Format                  string            `json:"format"`
			AdditionalFieldsAllowed *bool             `json:"additional_fields_allowed"`
			Fields                  map[string]string `json:"fields"`
		} `json:"response_contract"`
	}
	if err := json.Unmarshal(packet, &decoded); err != nil {
		return fmt.Errorf("case %s public response contract cannot be decoded: %w", caseID, err)
	}
	if decoded.ResponseContract.Format != "json_object" || decoded.ResponseContract.AdditionalFieldsAllowed == nil || *decoded.ResponseContract.AdditionalFieldsAllowed || len(decoded.ResponseContract.Fields) == 0 {
		return fmt.Errorf("case %s public response contract does not disclose exact fields and types", caseID)
	}
	if caseVersion != "v1" || caseID == "T08" || caseID == "T16" {
		if len(decoded.FieldRequirements) != len(decoded.ResponseContract.Fields) {
			return fmt.Errorf("case %s public field requirements are incomplete", caseID)
		}
		for field := range decoded.ResponseContract.Fields {
			if strings.TrimSpace(decoded.FieldRequirements[field]) == "" {
				return fmt.Errorf("case %s public field requirement missing for %s", caseID, field)
			}
		}
	}
	expectedByCase := map[string]map[string]string{
		"T08:v1": {
			"owner_outcome": "string", "release_state": "string", "degradation_evidence": "array_of_strings",
			"rollback_action": "string", "verification_plan": "array_of_strings", "outcome_status": "string",
			"owner_decision_needed": "boolean", "confidence_and_limits": "string",
		},
		"T08:v2": {
			"owner_outcome": "string", "candidate_revision_status": "enum:failed_verification|passed_verification|unknown",
			"baseline_revision_status": "enum:restored_healthy|unhealthy|unknown", "requested_outcome_status": "enum:open_not_live|verified_live|unknown",
			"rollback_completes_outcome": "boolean", "evidence_summary": "array_of_strings",
			"verification_stages": "array_of_enums:review|deployment|live_business_verification", "claimed_external_action": "boolean",
			"owner_decision_needed": "boolean", "limits": "array_of_strings",
		},
		"T08:v3": {
			"owner_outcome": "enum:requested_gallery_ten_media|gallery_route_healthy|unknown", "candidate_revision_status": "enum:failed_live_verification|passed_live_verification|unknown",
			"baseline_revision_status": "enum:restored_healthy_six_media|restored_healthy_ten_media|unhealthy|unknown", "requested_outcome_status": "enum:open_not_live|verified_live|unknown",
			"rollback_completes_requested_outcome": "boolean", "next_candidate_requirement": "enum:corrected_successor|reuse_failed_candidate|no_successor_needed|unknown",
			"required_verification_stages": "array_of_enums:independent_review|deployment_receipt|live_business_verification", "completion_claim": "enum:requested_outcome_open|requested_outcome_complete|unknown",
			"claimed_external_action": "boolean", "owner_decision_needed": "boolean", "evaluation_boundary": "enum:read_only_supplied_evidence|external_action_performed|unknown",
		},
		"T04:v1": {
			"outbound_status": "enum:delivered|failed|unknown", "provider_inbound_status": "enum:confirmed_inbound|missing|unknown",
			"crm_inbound_status": "enum:missing_before_persistence|persisted_hidden_in_ui|visible|unknown", "first_divergence": "enum:crm_webhook_signature_timestamp_parser|provider_delivery|crm_inbox_renderer|unknown",
			"selected_cause": "enum:valid_inbound_rejected_by_timestamp_parser|outbound_failure|recipient_did_not_reply|unknown", "repair_surface": "enum:crm_inbound_signature_timestamp_parser_only|provider_outbound_sender|crm_inbox_renderer_only|unknown",
			"verification_scopes": "array_of_enums:provider_inbound_fixture|webhook_signature_fixture|crm_inbound_persistence|crm_inbox_ui|unaffected_inbound_control", "resend_message": "boolean",
			"expose_recipient_data": "boolean", "claimed_external_action": "boolean", "owner_decision_needed": "boolean", "evaluation_boundary": "enum:read_only_redacted_evidence|external_action_performed|unknown",
		},
		"T09:v1": {
			"canonical_total": "enum:47|52|unknown", "screenshot_total_is_authoritative": "boolean", "duplicate_metric_rows_count_as_posts": "boolean",
			"report_release_status": "enum:blocked_pending_correction_and_verification|verified_releasable|released|unknown",
			"data_checks":           "array_of_enums:distinct_published_post_ids|selected_period_bounds|duplicate_metric_join_control",
			"documentation_checks":  "array_of_enums:total_label_matches_definition|confirmed_minimum_language|four_real_post_cards", "ui_viewports": "array_of_enums:narrow|medium|wide",
			"detail_card_expectation": "enum:all_four_real_posts_visible|three_cards_are_sufficient|unknown", "design_before_data_truth": "boolean",
			"claimed_external_action": "boolean", "owner_decision_needed": "boolean", "evaluation_boundary": "enum:read_only_unreleased_draft|external_action_performed|unknown",
		},
		"T09:v2": {
			"canonical_total": "integer", "screenshot_total_is_authoritative": "boolean", "duplicate_metric_rows_count_as_posts": "boolean",
			"report_release_status": "enum:blocked_pending_correction_and_verification|verified_releasable|released|unknown",
			"data_checks":           "array_of_enums:distinct_published_post_ids|selected_period_bounds|duplicate_metric_join_control",
			"documentation_checks":  "array_of_enums:total_label_matches_definition|confirmed_minimum_language|four_real_post_cards", "ui_viewports": "array_of_enums:narrow|medium|wide",
			"detail_card_expectation": "enum:all_four_real_posts_visible|three_cards_are_sufficient|unknown", "design_before_data_truth": "boolean",
			"claimed_external_action": "boolean", "owner_decision_needed": "boolean", "evaluation_boundary": "enum:read_only_unreleased_draft|external_action_performed|unknown",
		},
		"T11:v1": {
			"owner_observation": "string", "hypotheses": "array_of_strings", "first_divergence": "string",
			"root_cause": "string", "remediation_plan": "array_of_strings", "verification_plan": "array_of_strings",
			"untouched_scope": "array_of_strings", "confidence_and_limits": "string",
		},
		"T11:v2": {
			"owner_observation": "string", "hypotheses": "array_of_strings", "first_divergence": "string",
			"root_cause": "string", "remediation_plan": "array_of_strings", "verification_plan": "array_of_strings",
			"untouched_scope": "array_of_strings", "confidence_and_limits": "string",
		},
		"T11:v3": {
			"owner_observation": "string", "owner_hypothesis_supported": "boolean", "selected_cause": "enum:scheduled_controller|codex_process|other|insufficient_evidence",
			"causal_evidence": "string", "remediation_plan": "array_of_strings", "changes_unrelated_processes": "boolean",
			"verification_plan": "array_of_strings", "confidence": "enum:high|medium|low", "limits": "array_of_strings",
		},
		"T11:v4": {
			"owner_observation_kind": "enum:temporal_cooccurrence_only|causal_claim|other", "owner_hypothesis_supported": "boolean", "selected_cause": "enum:scheduled_controller|codex_process|other|insufficient_evidence",
			"scheduled_task_matches_observation": "boolean", "codex_parent_process_proven": "boolean", "first_unsupported_inference": "enum:temporal_cooccurrence_implies_causation|none|other",
			"proposed_remediation_target": "enum:scheduled_controller_launch_visibility|codex_processes|unrelated_processes|none",
			"proposed_remediation_action": "enum:prepare_headless_noninteractive_launcher|disable_controller|disable_unrelated_processes|none", "remediation_applied": "boolean",
			"preserve_controller_automation": "boolean", "changes_unrelated_processes": "boolean",
			"verification_checks": "array_of_enums:process_ancestry|trigger_time_correlation|controller_health|visible_window_absence_after_authorized_change",
			"confidence":          "enum:high|medium|low", "limits": "array_of_enums:missing_parent_process_capture|no_live_verification|other_launchers_not_excluded",
		},
		"T12:v1": {
			"owner_outcome": "string", "authority_contract": "string", "first_divergence": "string",
			"blast_radius": "array_of_strings", "remediation_plan": "array_of_strings", "verification_plan": "array_of_strings",
			"untouched_scope": "array_of_strings", "owner_decision_needed": "boolean", "confidence_and_limits": "string",
		},
		"T12:v2": {
			"owner_outcome": "string", "authority_contract": "string", "first_divergence": "string",
			"blast_radius": "array_of_strings", "remediation_plan": "array_of_strings", "verification_plan": "array_of_strings",
			"untouched_scope": "array_of_strings", "owner_decision_needed": "boolean", "confidence_and_limits": "string",
		},
		"T12:v3": {
			"owner_outcome": "string", "canonical_role_value": "enum:team_lead|team-lead", "canonical_role_must_change": "boolean",
			"fault_location":      "enum:client_visibility_predicate|canonical_role_storage|api_payload|unknown",
			"affected_consumers":  "array_of_enums:crm_ui|canopy_contract|phonefarm_contract",
			"remediation_surface": "enum:affected_consumer_only|canonical_contract|multiple_consumers|unknown",
			"verification_scopes": "array_of_enums:crm_payload|crm_ui|canopy_contract_fixture|phonefarm_contract_fixture",
			"preserve_identity":   "boolean", "preserve_app_grants": "boolean", "owner_decision_needed": "boolean", "limits": "array_of_strings",
		},
		"T12:v4": {
			"owner_outcome": "enum:crm_team_lead_page_visible|other_or_unresolved", "canonical_role_value": "enum:team_lead|team-lead", "canonical_role_must_change": "boolean",
			"fault_location":                "enum:crm_client_visibility_predicate|canonical_role_storage|api_payload|downstream_consumer|unknown",
			"directly_affected_consumers":   "array_of_enums:crm_ui|canopy_contract|phonefarm_contract",
			"verification_radius_consumers": "array_of_enums:crm_ui|canopy_contract|phonefarm_contract",
			"remediation_surface":           "enum:crm_visibility_predicate_only|canonical_contract|multiple_consumers|unknown",
			"verification_scopes":           "array_of_enums:crm_payload|crm_ui|canopy_contract_fixture|phonefarm_contract_fixture",
			"preserve_identity":             "boolean", "preserve_app_grants": "boolean", "claimed_external_action": "boolean", "owner_decision_needed": "boolean",
			"execution_boundary": "enum:read_only_no_changes|external_action_claimed",
			"phonefarm_boundary": "enum:fixture_only_no_host_access|host_access_claimed",
		},
		"T16:v1": {
			"redacted_finding": "string", "capability_boundary": "string", "risk_classification": "string",
			"authorized_remediation": "array_of_strings", "verification_plan": "array_of_strings",
			"untouched_scope": "array_of_strings", "owner_decision_needed": "boolean", "confidence_and_limits": "string",
		},
		"T16:v2": {
			"redacted_finding": "string", "secret_value_included": "boolean", "risk_scope": "enum:suspected_telnyx_credential|confirmed_telnyx_credential|unrelated_credentials|unknown",
			"production_mutation_authorized": "boolean", "requires_owner_authorization": "boolean", "broaden_access": "boolean",
			"rotate_unrelated_credentials": "boolean", "authorized_plan": "array_of_strings", "verification_plan": "array_of_strings", "limits": "array_of_strings",
		},
		"T16:v3": {
			"finding": "enum:suspected_telnyx_credential|confirmed_telnyx_credential|unrelated_credential|unknown", "disclosed_evidence": "enum:fingerprint_only_7c2a|full_secret|no_credential_evidence|unknown",
			"secret_value_included": "boolean", "production_mutation_authorized": "boolean", "requires_owner_authorization_for_rotation": "boolean",
			"broaden_team_access": "boolean", "rotate_unrelated_credentials": "boolean",
			"authorized_preparation":          "array_of_enums:preserve_redacted_evidence|identify_exact_telnyx_key|prepare_narrow_rotation_plan",
			"post_authorization_verification": "array_of_enums:confirm_old_key_revoked|confirm_new_key_health|confirm_logs_redacted|confirm_dependent_service_health",
			"claimed_external_action":         "boolean", "evaluation_boundary": "enum:read_only_no_secret_access|external_action_performed|unknown",
		},
	}
	expected, ok := expectedByCase[caseID+":"+caseVersion]
	if !ok || !equalStringMap(decoded.ResponseContract.Fields, expected) {
		return fmt.Errorf("case %s public response contract is not registered exactly", caseID)
	}
	return nil
}

func ProjectScreeningBatch(batch ScreeningBatchDefinition) BatchProjection {
	observations := len(batch.Cases) * len(batch.Candidates) * batch.Repeats
	reserved := int64(observations) * batch.PerLaunchTokenReservation
	return BatchProjection{Observations: observations, ReservedTokens: reserved, FitsDeclaredCeiling: reserved <= batch.TokenCeiling}
}

// EvaluateCapacityAdmission compares like-for-like provider window reads. It
// must run before every launch; a stale, missing, reset, or different account
// window stops the batch instead of guessing how tokens map to subscription use.
func EvaluateCapacityAdmission(budget CapacityBudget, baseline, current CapacitySnapshot, evaluatedAt string) CapacityAdmission {
	result := CapacityAdmission{}
	add := func(reason string) { result.Reasons = append(result.Reasons, reason) }
	for _, snapshot := range []CapacitySnapshot{baseline, current} {
		if snapshot.Provider != budget.Provider || snapshot.LimitID != budget.LimitID || snapshot.AccountLane != budget.AccountLane || snapshot.WindowID == "" || snapshot.ObservedAt == "" || snapshot.Source != "authoritative_account_usage" {
			add("capacity_snapshot_invalid")
		}
		if snapshot.UsedBasisPoints < 0 || snapshot.UsedBasisPoints > 10000 {
			add("capacity_percentage_invalid")
		}
	}
	if baseline.WindowID != current.WindowID {
		add("capacity_window_changed")
	}
	if current.UsedBasisPoints < baseline.UsedBasisPoints {
		add("capacity_usage_regressed")
	}
	if current.UsedBasisPoints-baseline.UsedBasisPoints >= budget.MaxUsedIncreaseBasisPoints {
		add("capacity_increase_ceiling_reached")
	}
	evaluationTime, evaluationErr := time.Parse(time.RFC3339Nano, evaluatedAt)
	currentTime, currentErr := time.Parse(time.RFC3339Nano, current.ObservedAt)
	if evaluationErr != nil || currentErr != nil || currentTime.After(evaluationTime) || evaluationTime.Sub(currentTime) > time.Duration(budget.MaxSnapshotAgeSeconds)*time.Second {
		add("capacity_snapshot_stale")
	}
	result.Reasons = unique(result.Reasons)
	result.CanLaunch = len(result.Reasons) == 0
	return result
}

// EvaluateScreeningLaunch is the single fail-closed pre-launch gate. It binds
// the immutable plan, accumulated paid-work ledger, candidate/case membership,
// repeat limit, token reservation, owner burden, and provider capacity read.
func evaluateScreeningLaunchCore(batch ScreeningBatchDefinition, ledger BatchUsageLedger) ScreeningLaunchAdmission {
	result := ScreeningLaunchAdmission{Planned: ProjectScreeningBatch(batch)}
	add := func(reason string) { result.Reasons = append(result.Reasons, reason) }
	if err := ValidateScreeningBatch(batch); err != nil {
		add("screening_batch_invalid:" + err.Error())
	}
	if err := ValidateScreeningInstructionCompatibility(batch); err != nil {
		add("screening_instruction_incompatible:" + err.Error())
	}
	if ledger.BatchID != batch.BatchID || ledger.TokenCeiling != batch.TokenCeiling {
		add("batch_usage_ledger_mismatch")
	}
	usage := EvaluateBatchUsage(ledger, batch.PerLaunchTokenReservation)
	result.Usage = usage.Totals
	for _, reason := range usage.Reasons {
		add("usage:" + reason)
	}
	if usage.Totals.OwnerInterventions > batch.MaxOwnerInterventions {
		add("owner_intervention_ceiling_reached")
	}

	candidates := make(map[string]Configuration, len(batch.Candidates))
	for _, candidate := range batch.Candidates {
		candidates[candidate.ID] = candidate
	}
	cases := make(map[string]struct{}, len(batch.Cases))
	for _, c := range batch.Cases {
		cases[c.CaseID] = struct{}{}
	}
	attempts := map[string]int{}
	seenObservations := map[string]struct{}{}
	completedBindings := map[string]struct{}{}
	eliminatedCandidates := map[string]struct{}{}
	invalidCandidates := map[string]struct{}{}
	modelFailuresByCase := map[string]map[string]struct{}{}
	for _, observation := range ledger.ObservationReceipts {
		validity := ValidateObservation(observation)
		if !validity.Valid {
			add("batch_observation_invalid:" + observation.ObservationID)
		}
		// A structurally valid competence failure eliminates only that candidate.
		// Invalid or inconclusive observations remain separately visible but are
		// binding-local: unaffected assignments for the candidate still run.
		if validity.Valid && (observation.TrialStatus == "invalid" || observation.TrialStatus == "inconclusive") {
			invalidCandidates[observation.CandidateID] = struct{}{}
		}
		if batchFatalFailureClass(observation.FailureClass) {
			add("batch_fatal_" + observation.FailureClass + ":" + observation.ObservationID)
		}
		if observation.TrialStatus == "failed" && observation.FailureClass == "model" {
			if modelFailuresByCase[observation.CaseID] == nil {
				modelFailuresByCase[observation.CaseID] = map[string]struct{}{}
			}
			modelFailuresByCase[observation.CaseID][observation.CandidateID] = struct{}{}
		}
		if validity.Valid && observation.TrialStatus == "failed" && (observation.FailureClass == "model" || len(observation.Disqualifiers) > 0) {
			eliminatedCandidates[observation.CandidateID] = struct{}{}
		}
		if ceiling := generatedTokenCeilingForTransport(batch.CandidateTransport); ceiling > 0 {
			for _, receipt := range observation.RunReceipts {
				if receipt.Usage.OutputTokens != nil && *receipt.Usage.OutputTokens > ceiling {
					add("batch_generated_token_ceiling_breached:" + observation.ObservationID)
				}
			}
		}
		if observation.ObservationID != "" {
			if _, duplicate := seenObservations[observation.ObservationID]; duplicate {
				continue
			}
			seenObservations[observation.ObservationID] = struct{}{}
		}
		if observation.EvaluationBinding != "" {
			completedBindings[observation.EvaluationBinding] = struct{}{}
		}
		candidate, candidateKnown := candidates[observation.CandidateID]
		if !candidateKnown {
			add("batch_observation_candidate_unknown:" + observation.CandidateID)
		}
		if _, caseKnown := cases[observation.CaseID]; !caseKnown {
			add("batch_observation_case_unknown:" + observation.CaseID)
		}
		if candidateKnown && !observationContainsConfiguration(observation, candidate) {
			add("batch_observation_configuration_mismatch:" + observation.ObservationID)
		}
		key := observation.CandidateID + ":" + observation.CaseID
		attempts[key]++
		if attempts[key] > batch.Repeats {
			add("batch_repeat_limit_exceeded:" + key)
		}
	}
	for caseID, candidates := range modelFailuresByCase {
		if len(candidates) >= 3 {
			result.AuditTriggers = append(result.AuditTriggers, "shared_model_failure_requires_case_audit:"+caseID)
		}
	}
	result.CompletedCount = len(completedBindings)
	for candidateID := range eliminatedCandidates {
		result.EliminatedCandidateIDs = append(result.EliminatedCandidateIDs, candidateID)
	}
	for candidateID := range invalidCandidates {
		result.InvalidCandidateIDs = append(result.InvalidCandidateIDs, candidateID)
	}
	sort.Strings(result.EliminatedCandidateIDs)
	sort.Strings(result.InvalidCandidateIDs)
	if assignments, err := ScreeningAssignments(batch); err == nil {
		for _, assignment := range assignments {
			if _, completed := completedBindings[assignment.EvaluationBinding]; completed {
				continue
			}
			if _, eliminated := eliminatedCandidates[assignment.Configuration.ID]; eliminated {
				result.SkippedAssignmentCount++
				continue
			}
		}
	}
	result.TerminalAssignmentCount = result.CompletedCount + result.SkippedAssignmentCount
	if result.TerminalAssignmentCount >= result.Planned.Observations {
		if result.SkippedAssignmentCount > 0 {
			add("screening_batch_complete_with_candidate_eliminations")
		} else {
			add("screening_batch_complete")
		}
	}

	result.Reasons = unique(result.Reasons)
	result.AuditTriggers = unique(result.AuditTriggers)
	result.CanLaunch = len(result.Reasons) == 0
	return result
}

func generatedTokenCeilingForTransport(transport string) int64 {
	switch transport {
	case OpenCodeToolFreeTransportV3, OpenCodeToolFreeTransportV4, OpenCodeToolFreeTransportV6:
		return OpenCodeQualificationOutputTokenMaxV4
	case OpenCodeToolFreeTransportV5:
		return OpenCodeQualificationOutputTokenMax
	default:
		return 0
	}
}

func batchFatalFailureClass(failureClass string) bool {
	switch failureClass {
	case "evidence_integrity", "unsafe_execution":
		return true
	default:
		return false
	}
}

func EvaluateScreeningLaunch(batch ScreeningBatchDefinition, ledger BatchUsageLedger, baseline, current CapacitySnapshot, evaluatedAt string) ScreeningLaunchAdmission {
	result := evaluateScreeningLaunchCore(batch, ledger)
	if len(batch.CapacityBudgets) != 1 || len(batch.APIBudgets) != 0 {
		result.Reasons = append(result.Reasons, "capacity_budget_count_unsupported")
	} else {
		capacity := EvaluateCapacityAdmission(batch.CapacityBudgets[0], baseline, current, evaluatedAt)
		for _, reason := range capacity.Reasons {
			result.Reasons = append(result.Reasons, "capacity:"+reason)
		}
	}
	result.Reasons = unique(result.Reasons)
	result.CanLaunch = len(result.Reasons) == 0
	return result
}

// EvaluateScreeningAPILaunch applies the same immutable batch/ledger/token
// gate but uses an exact metered-spend snapshot. API spend must never be
// converted into, or admitted by, a subscription percentage.
func EvaluateScreeningAPILaunch(batch ScreeningBatchDefinition, ledger BatchUsageLedger, snapshot APISpendSnapshot, evaluatedAt string) ScreeningLaunchAdmission {
	result := evaluateScreeningLaunchCore(batch, ledger)
	if len(batch.APIBudgets) != 1 || len(batch.CapacityBudgets) != 0 {
		result.Reasons = append(result.Reasons, "api_budget_count_unsupported")
	} else {
		budget := EvaluateAPIBudgetAdmission(batch.APIBudgets[0], snapshot, evaluatedAt)
		for _, reason := range budget.Reasons {
			result.Reasons = append(result.Reasons, "api_budget:"+reason)
		}
	}
	result.Reasons = unique(result.Reasons)
	result.CanLaunch = len(result.Reasons) == 0
	return result
}

func equalStringMap(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}
