package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
)

const RoleQualificationExecutionFreezeSchemaV2 = "silver-mediums.role-qualification-execution-freeze.v2"

type RoleQualificationExecutionFreezeV2 struct {
	SchemaVersion          string            `json:"schema_version"`
	FreezeID               string            `json:"freeze_id"`
	Status                 string            `json:"status"`
	V1FreezeSHA256         string            `json:"v1_freeze_sha256"`
	ProviderCallsAllowed   bool              `json:"provider_calls_allowed"`
	RuntimeWiring          string            `json:"runtime_wiring"`
	FreshPreflightRequired bool              `json:"fresh_preflight_required"`
	Cells                  int               `json:"cells"`
	Retries                int               `json:"retries"`
	Fallbacks              bool              `json:"fallbacks"`
	Artifacts              map[string]string `json:"artifacts"`
	TransportAntecedents   map[string]string `json:"transport_antecedents"`
}

//go:embed role_qualification_execution_v2_freeze.json
var roleQualificationExecutionFreezeV2JSON []byte

//go:embed role_qualification_execution_v2.json
var roleQualificationExecutionFreezeDefinitionV2 []byte

//go:embed role_qualification_execution_v2.go
var roleQualificationExecutionFreezeCoreV2 []byte

//go:embed role_qualification_execution_v2_lifecycle.go
var roleQualificationExecutionFreezeLifecycleV2 []byte

//go:embed role_qualification_execution_v2_test.go
var roleQualificationExecutionFreezeTestsV2 []byte

//go:embed role_qualification_execution_v2_prevalidation.md
var roleQualificationExecutionFreezePrevalidationV2 []byte

//go:embed role_qualification_execution_v2_freeze.go
var roleQualificationExecutionFreezeSourceV2 []byte

//go:embed role_qualification_execution_v2_freeze_test.go
var roleQualificationExecutionFreezeSelfTestsV2 []byte

func CurrentRoleQualificationExecutionFreezeV2() (RoleQualificationExecutionFreezeV2, error) {
	var f RoleQualificationExecutionFreezeV2
	if err := strictJSON(roleQualificationExecutionFreezeV2JSON, &f); err != nil {
		return f, err
	}
	if err := ValidateRoleQualificationExecutionFreezeV2(f); err != nil {
		return RoleQualificationExecutionFreezeV2{}, err
	}
	return f, nil
}
func ValidateRoleQualificationExecutionFreezeV2(f RoleQualificationExecutionFreezeV2) error {
	if f.SchemaVersion != RoleQualificationExecutionFreezeSchemaV2 || f.FreezeID != "role-qualification-execution-freeze-v2" || f.Status != "frozen_provider_locked" || f.V1FreezeSHA256 != "b047faaec585beee2cf00640f53e35d2b09b54618c6923c6b9458d2851d1b2e4" || f.V1FreezeSHA256 != RoleQualificationFreezeV1SHA256() {
		return fmt.Errorf("role qualification execution v2 freeze identity drifted")
	}
	if f.ProviderCallsAllowed || f.RuntimeWiring != "none" || !f.FreshPreflightRequired || f.Cells != 36 || f.Retries != 0 || f.Fallbacks {
		return fmt.Errorf("role qualification execution v2 freeze opened provider boundary")
	}
	if err := ValidateRoleQualificationExecutionDefinitionV2(mustRoleQualificationExecutionDefinitionV2()); err != nil {
		return err
	}
	embedded := map[string][]byte{"server/internal/agentqualification/role_qualification_execution_v2.json": roleQualificationExecutionFreezeDefinitionV2, "server/internal/agentqualification/role_qualification_execution_v2.go": roleQualificationExecutionFreezeCoreV2, "server/internal/agentqualification/role_qualification_execution_v2_lifecycle.go": roleQualificationExecutionFreezeLifecycleV2, "server/internal/agentqualification/role_qualification_execution_v2_test.go": roleQualificationExecutionFreezeTestsV2, "server/internal/agentqualification/role_qualification_execution_v2_prevalidation.md": roleQualificationExecutionFreezePrevalidationV2, "server/internal/agentqualification/role_qualification_execution_v2_freeze.go": roleQualificationExecutionFreezeSourceV2, "server/internal/agentqualification/role_qualification_execution_v2_freeze_test.go": roleQualificationExecutionFreezeSelfTestsV2}
	if len(f.Artifacts) != 9 || len(embedded) != 7 {
		return fmt.Errorf("role qualification execution v2 freeze artifact coverage drifted")
	}
	for path, raw := range embedded {
		sum := sha256.Sum256(raw)
		if f.Artifacts[path] != hex.EncodeToString(sum[:]) {
			return fmt.Errorf("role qualification execution v2 frozen artifact drifted: %s", path)
		}
	}
	for _, path := range []string{"server/cmd/role-qualification-run/main.go", "server/cmd/role-qualification-run/main_test.go"} {
		if !validSHA256(f.Artifacts[path]) {
			return fmt.Errorf("role qualification command hash missing: %s", path)
		}
	}
	for _, path := range []string{"server/internal/agentqualification/opencode_structured_v33.go", "server/internal/agentqualification/opencode_structured_v33_lifecycle.go", "server/internal/agentqualification/claude_transport.go", "server/internal/agentqualification/claude_receipt.go"} {
		if !validSHA256(f.TransportAntecedents[path]) {
			return fmt.Errorf("role qualification transport antecedent missing: %s", path)
		}
	}
	if len(f.TransportAntecedents) != 4 {
		return fmt.Errorf("role qualification transport antecedent coverage drifted")
	}
	return nil
}
func mustRoleQualificationExecutionDefinitionV2() RoleQualificationExecutionDefinitionV2 {
	var d RoleQualificationExecutionDefinitionV2
	if err := strictJSON(roleQualificationExecutionV2JSON, &d); err != nil {
		panic(err)
	}
	return d
}
func RoleQualificationExecutionFreezeV2SHA256() string {
	sum := sha256.Sum256(roleQualificationExecutionFreezeV2JSON)
	return hex.EncodeToString(sum[:])
}
