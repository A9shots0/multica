package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
)

const RoleQualificationExecutionFreezeSchemaV6 = "silver-mediums.role-qualification-execution-freeze.v6"

type RoleQualificationExecutionFreezeV6 struct {
	SchemaVersion     string            `json:"schema_version"`
	FreezeID          string            `json:"freeze_id"`
	Status            string            `json:"status"`
	V5FreezeSHA256    string            `json:"v5_freeze_sha256"`
	CellsPerBatch     int               `json:"cells_per_batch"`
	ProviderCallsMade int               `json:"provider_calls_made"`
	Retries           int               `json:"retries"`
	Fallbacks         bool              `json:"fallbacks"`
	Artifacts         map[string]string `json:"artifacts"`
}

//go:embed role_qualification_execution_v6_freeze.json
var roleQualificationExecutionFreezeV6JSON []byte

func CurrentRoleQualificationExecutionFreezeV6() (RoleQualificationExecutionFreezeV6, error) {
	var f RoleQualificationExecutionFreezeV6
	if err := strictJSON(roleQualificationExecutionFreezeV6JSON, &f); err != nil {
		return f, err
	}
	if f.SchemaVersion != RoleQualificationExecutionFreezeSchemaV6 || f.FreezeID != "role-qualification-execution-freeze-v6" || f.Status != "frozen_preflight_required_not_executed" || f.V5FreezeSHA256 != RoleQualificationExecutionFreezeV5SHA256() || f.CellsPerBatch != 12 || f.ProviderCallsMade != 0 || f.Retries != 0 || f.Fallbacks || len(f.Artifacts) == 0 {
		return f, fmt.Errorf("v6 freeze drift")
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

func RoleQualificationExecutionFreezeV6SHA256() string {
	sum := sha256.Sum256(roleQualificationExecutionFreezeV6JSON)
	return hex.EncodeToString(sum[:])
}
