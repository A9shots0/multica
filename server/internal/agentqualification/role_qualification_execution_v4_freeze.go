package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
)

const RoleQualificationExecutionFreezeSchemaV4 = "silver-mediums.role-qualification-execution-freeze.v4"

type RoleQualificationExecutionFreezeV4 struct {
	SchemaVersion     string            `json:"schema_version"`
	FreezeID          string            `json:"freeze_id"`
	Status            string            `json:"status"`
	V3FreezeSHA256    string            `json:"v3_freeze_sha256"`
	CellsPerBatch     int               `json:"cells_per_batch"`
	ProviderCallsMade int               `json:"provider_calls_made"`
	Retries           int               `json:"retries"`
	Fallbacks         bool              `json:"fallbacks"`
	ImplicitAllModels bool              `json:"implicit_all_models"`
	Artifacts         map[string]string `json:"artifacts"`
}

//go:embed role_qualification_execution_v4_freeze.json
var roleQualificationExecutionFreezeV4JSON []byte

func CurrentRoleQualificationExecutionFreezeV4() (RoleQualificationExecutionFreezeV4, error) {
	var f RoleQualificationExecutionFreezeV4
	if err := strictJSON(roleQualificationExecutionFreezeV4JSON, &f); err != nil {
		return f, err
	}
	if err := ValidateRoleQualificationExecutionFreezeV4(f); err != nil {
		return RoleQualificationExecutionFreezeV4{}, err
	}
	return f, nil
}
func ValidateRoleQualificationExecutionFreezeV4(f RoleQualificationExecutionFreezeV4) error {
	if f.SchemaVersion != RoleQualificationExecutionFreezeSchemaV4 || f.FreezeID != "role-qualification-execution-freeze-v4" || f.Status != "frozen_preflight_required_not_executed" || f.V3FreezeSHA256 != RoleQualificationExecutionFreezeV3SHA256() {
		return fmt.Errorf("v4 freeze identity drift")
	}
	if f.CellsPerBatch != 12 || f.ProviderCallsMade != 0 || f.Retries != 0 || f.Fallbacks || f.ImplicitAllModels || len(f.Artifacts) != 10 {
		return fmt.Errorf("v4 freeze authority/coverage drift")
	}
	if _, err := CurrentRoleQualificationExecutionFreezeV3(); err != nil {
		return err
	}
	root, err := roleQualificationRepoRootV3()
	if err != nil {
		return err
	}
	for path, want := range f.Artifacts {
		if err := validateExactFileHashV3(root, path, want); err != nil {
			return err
		}
	}
	return nil
}
func RoleQualificationExecutionFreezeV4SHA256() string {
	s := sha256.Sum256(roleQualificationExecutionFreezeV4JSON)
	return hex.EncodeToString(s[:])
}
