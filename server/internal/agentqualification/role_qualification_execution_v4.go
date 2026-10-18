package agentqualification

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const RoleQualificationExecutionSchemaV4 = "silver-mediums.role-qualification-execution.v4"

type RoleQualificationExecutionDefinitionV4 struct {
	SchemaVersion                      string `json:"schema_version"`
	ExecutionID                        string `json:"execution_id"`
	Status                             string `json:"status"`
	V3FreezeSHA256                     string `json:"v3_freeze_sha256"`
	MaxCellsPerInvocation              int    `json:"max_cells_per_invocation"`
	FreshPreflightSeconds              int64  `json:"fresh_preflight_seconds"`
	ProviderCallsAllowedAfterExactGate bool   `json:"provider_calls_allowed_after_exact_gate"`
	Retries                            int    `json:"retries"`
	Fallbacks                          bool   `json:"fallbacks"`
	ImplicitAllModels                  bool   `json:"implicit_all_models"`
	ExternalSideEffectsAllowed         bool   `json:"external_side_effects_allowed"`
	ProviderGate                       string `json:"provider_gate"`
}
type RoleQualificationPreflightV4 struct {
	SchemaVersion                 string `json:"schema_version"`
	ExecutionFreezeSHA256         string `json:"execution_freeze_sha256"`
	V3FreezeSHA256                string `json:"v3_freeze_sha256"`
	ConfigurationClassID          string `json:"configuration_class_id"`
	RequestedModel                string `json:"requested_model"`
	RequestedProvider             string `json:"requested_provider"`
	RequestedEffort               string `json:"requested_effort"`
	Transport                     string `json:"transport"`
	RuntimeVersion                string `json:"runtime_version"`
	RuntimeExecutablePath         string `json:"runtime_executable_path"`
	RuntimeExecutableSHA256       string `json:"runtime_executable_sha256"`
	ResolvedConfigSHA256          string `json:"resolved_config_sha256"`
	ExactRouteValidated           bool   `json:"exact_route_validated"`
	FallbacksDisabled             bool   `json:"fallbacks_disabled"`
	CredentialState               string `json:"credential_state"`
	OpenRouterSpendBaselineMicros int64  `json:"openrouter_spend_baseline_micros"`
	ObservedAt                    string `json:"observed_at"`
	Ready                         bool   `json:"ready"`
}
type RoleQualificationProviderV4 interface {
	Call(context.Context, RoleQualificationExecutionCellV3) (json.RawMessage, error)
}

//go:embed role_qualification_execution_v4.json
var roleQualificationExecutionV4JSON []byte

func CurrentRoleQualificationExecutionDefinitionV4() (RoleQualificationExecutionDefinitionV4, error) {
	var d RoleQualificationExecutionDefinitionV4
	if err := strictJSON(roleQualificationExecutionV4JSON, &d); err != nil {
		return d, err
	}
	if err := ValidateRoleQualificationExecutionDefinitionV4(d); err != nil {
		return RoleQualificationExecutionDefinitionV4{}, err
	}
	return d, nil
}
func ValidateRoleQualificationExecutionDefinitionV4(d RoleQualificationExecutionDefinitionV4) error {
	if d.SchemaVersion != RoleQualificationExecutionSchemaV4 || d.ExecutionID != "role-qualification-provider-batches-v4" || d.Status != "frozen_preflight_required_not_executed" || d.V3FreezeSHA256 != "cd2f01e9ff7beaf2bbb0f4c4bfa06a4c6ebc11060a8a62fb15ef4c616cb7a0f3" || d.V3FreezeSHA256 != RoleQualificationExecutionFreezeV3SHA256() {
		return fmt.Errorf("v4 identity drift")
	}
	if d.MaxCellsPerInvocation != 12 || d.FreshPreflightSeconds != 600 || !d.ProviderCallsAllowedAfterExactGate || d.Retries != 0 || d.Fallbacks || d.ImplicitAllModels || d.ExternalSideEffectsAllowed || d.ProviderGate != "exact_v3_and_v4_freezes_plus_fresh_class_preflight" {
		return fmt.Errorf("v4 authority drift")
	}
	return nil
}
func RoleQualificationClassCellsV4(classID string) ([]RoleQualificationExecutionCellV3, error) {
	plan, err := BuildRoleQualificationExecutionPlanV3()
	if err != nil {
		return nil, err
	}
	var cells []RoleQualificationExecutionCellV3
	for _, c := range plan.Cells {
		if c.ConfigurationClass.ClassID == classID {
			cells = append(cells, c)
		}
	}
	if len(cells) != 12 {
		return nil, fmt.Errorf("exact configuration class with 12 cells required")
	}
	return cells, nil
}
func ValidateRoleQualificationPreflightV4(p RoleQualificationPreflightV4, classID string, now time.Time) error {
	d, err := CurrentRoleQualificationExecutionDefinitionV4()
	if err != nil {
		return err
	}
	cells, err := RoleQualificationClassCellsV4(classID)
	if err != nil {
		return err
	}
	c := cells[0].ConfigurationClass
	if p.SchemaVersion != "silver-mediums.role-qualification-preflight.v4" || p.ExecutionFreezeSHA256 != RoleQualificationExecutionFreezeV4SHA256() || p.V3FreezeSHA256 != d.V3FreezeSHA256 || p.ConfigurationClassID != classID || p.RequestedModel != c.Model || p.RequestedProvider != c.ServingProvider || p.RequestedEffort != c.Effort || p.Transport != transportForRoleQualificationCellV3(cells[0]) || p.RuntimeVersion == "" || p.RuntimeExecutablePath == "" || !validSHA256(p.RuntimeExecutableSHA256) || !validSHA256(p.ResolvedConfigSHA256) || !p.ExactRouteValidated || !p.FallbacksDisabled || p.CredentialState != "authenticated" || p.OpenRouterSpendBaselineMicros < 0 || !p.Ready {
		return fmt.Errorf("v4 preflight incomplete or route drift")
	}
	observed, err := time.Parse(time.RFC3339Nano, p.ObservedAt)
	if err != nil || now.Before(observed) || now.Sub(observed) > time.Duration(d.FreshPreflightSeconds)*time.Second {
		return fmt.Errorf("v4 preflight stale")
	}
	if c.RuntimeProvider == "claude_code_subscription" && p.OpenRouterSpendBaselineMicros != 0 {
		return fmt.Errorf("subscription preflight conflates OpenRouter spend")
	}
	return nil
}
func WriteRoleQualificationPreflightV4(root string, p RoleQualificationPreflightV4) error {
	if err := ValidateRoleQualificationPreflightV4(p, p.ConfigurationClassID, time.Now().UTC()); err != nil {
		return err
	}
	return writeOnceRoleQualificationJSONV3(filepath.Join(root, "preflight", p.ConfigurationClassID+".json"), p)
}
func ExecuteRoleQualificationBatchV4(ctx context.Context, root, classID string, now time.Time, provider RoleQualificationProviderV4) (RoleQualificationReplayV3, error) {
	d, err := CurrentRoleQualificationExecutionDefinitionV4()
	if err != nil {
		return RoleQualificationReplayV3{}, err
	}
	if _, err := CurrentRoleQualificationExecutionFreezeV4(); err != nil {
		return RoleQualificationReplayV3{}, err
	}
	var preflight RoleQualificationPreflightV4
	if err := readStrictJSONFileV3(filepath.Join(root, "preflight", classID+".json"), &preflight); err != nil {
		return RoleQualificationReplayV3{}, err
	}
	if err := ValidateRoleQualificationPreflightV4(preflight, classID, now); err != nil {
		return RoleQualificationReplayV3{}, err
	}
	cells, err := RoleQualificationClassCellsV4(classID)
	if err != nil || len(cells) > d.MaxCellsPerInvocation {
		return RoleQualificationReplayV3{}, fmt.Errorf("v4 batch scope")
	}
	for _, cell := range cells {
		replay, err := RoleQualificationExecutionReplayV3(root)
		if err != nil {
			return replay, err
		}
		if replay.GlobalHalt {
			return replay, fmt.Errorf("global halt %s", replay.GlobalHaltClass)
		}
		pair := cell.ConfigurationClass.ClassID + ":" + cell.CandidateClass
		skip := false
		for _, e := range replay.EliminatedBindings {
			if e == pair {
				skip = true
			}
		}
		if skip {
			continue
		}
		if _, err := PrepareRoleQualificationGrantV3(root, cell.CellID, now.Format(time.RFC3339Nano)); err != nil {
			return replay, err
		}
		raw, callErr := provider.Call(ctx, cell)
		if len(raw) == 0 {
			replay, replayErr := RoleQualificationExecutionReplayV3(root)
			if replayErr != nil {
				return replay, replayErr
			}
			if callErr != nil {
				return replay, callErr
			}
			return replay, fmt.Errorf("provider returned no authoritative raw receipt")
		}
		receipt := RoleQualificationProviderReceiptV3{"silver-mediums.role-qualification-provider-receipt.v3", cell.CellID, cell.RequestSHA256, append(json.RawMessage(nil), raw...), sha256Hex(raw), time.Now().UTC().Format(time.RFC3339Nano)}
		if err := SealRoleQualificationRawReceiptV3(root, receipt); err != nil {
			replay, replayErr := RoleQualificationExecutionReplayV3(root)
			if replayErr != nil {
				return replay, replayErr
			}
			return replay, err
		}
		if _, err := ReconcileRoleQualificationClaimV3(root, cell.CellID); err != nil {
			replay, replayErr := RoleQualificationExecutionReplayV3(root)
			if replayErr != nil {
				return replay, replayErr
			}
			return replay, err
		}
		if callErr != nil {
			return RoleQualificationExecutionReplayV3(root)
		}
	}
	return RoleQualificationExecutionReplayV3(root)
}
func RoleQualificationPreflightSHA256V4(p RoleQualificationPreflightV4) string {
	raw, _ := json.Marshal(p)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func fileSHA256V4(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
