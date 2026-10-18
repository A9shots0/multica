package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
)

const RoleQualificationExecutionFreezeSchemaV5 = "silver-mediums.role-qualification-execution-freeze.v5"

type RoleQualificationExecutionFreezeV5 struct {
	SchemaVersion     string            `json:"schema_version"`
	FreezeID          string            `json:"freeze_id"`
	Status            string            `json:"status"`
	V4FreezeSHA256    string            `json:"v4_freeze_sha256"`
	CellsPerBatch     int               `json:"cells_per_batch"`
	ProviderCallsMade int               `json:"provider_calls_made"`
	Retries           int               `json:"retries"`
	Fallbacks         bool              `json:"fallbacks"`
	Artifacts         map[string]string `json:"artifacts"`
}

//go:embed role_qualification_execution_v5_freeze.json
var roleQualificationExecutionFreezeV5JSON []byte

func CurrentRoleQualificationExecutionFreezeV5() (RoleQualificationExecutionFreezeV5, error) {
	var f RoleQualificationExecutionFreezeV5
	if err := strictJSON(roleQualificationExecutionFreezeV5JSON, &f); err != nil {
		return f, err
	}
	if f.SchemaVersion != RoleQualificationExecutionFreezeSchemaV5 || f.FreezeID != "role-qualification-execution-freeze-v5" || f.Status != "frozen_preflight_required_not_executed" || f.V4FreezeSHA256 != RoleQualificationExecutionFreezeV4SHA256() || f.CellsPerBatch != 12 || f.ProviderCallsMade != 0 || f.Retries != 0 || f.Fallbacks || len(f.Artifacts) == 0 {
		return f, fmt.Errorf("v5 freeze drift")
	}
	root, err := roleQualificationRepoRootV3()
	if err != nil {
		return f, err
	}
	for path, want := range f.Artifacts {
		if err := validateExactFileHashV3(root, path, want); err != nil {
			return f, err
		}
	}
	return f, nil
}

func RoleQualificationExecutionFreezeV5SHA256() string {
	sum := sha256.Sum256(roleQualificationExecutionFreezeV5JSON)
	return hex.EncodeToString(sum[:])
}
