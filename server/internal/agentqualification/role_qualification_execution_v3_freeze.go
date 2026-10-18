package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const RoleQualificationExecutionFreezeSchemaV3 = "silver-mediums.role-qualification-execution-freeze.v3"

type RoleQualificationExecutionFreezeV3 struct {
	SchemaVersion          string            `json:"schema_version"`
	FreezeID               string            `json:"freeze_id"`
	Status                 string            `json:"status"`
	V1FreezeSHA256         string            `json:"v1_freeze_sha256"`
	RejectedV2FreezeSHA256 string            `json:"rejected_v2_freeze_sha256"`
	Cells                  int               `json:"cells"`
	ProviderCallsAllowed   bool              `json:"provider_calls_allowed"`
	RuntimeWiring          string            `json:"runtime_wiring"`
	Retries                int               `json:"retries"`
	Fallbacks              bool              `json:"fallbacks"`
	Artifacts              map[string]string `json:"artifacts"`
	TransportAntecedents   map[string]string `json:"transport_antecedents"`
}

//go:embed role_qualification_execution_v3_freeze.json
var roleQualificationExecutionFreezeV3JSON []byte

func CurrentRoleQualificationExecutionFreezeV3() (RoleQualificationExecutionFreezeV3, error) {
	var f RoleQualificationExecutionFreezeV3
	if err := strictJSON(roleQualificationExecutionFreezeV3JSON, &f); err != nil {
		return f, err
	}
	if err := ValidateRoleQualificationExecutionFreezeV3(f); err != nil {
		return RoleQualificationExecutionFreezeV3{}, err
	}
	return f, nil
}
func ValidateRoleQualificationExecutionFreezeV3(f RoleQualificationExecutionFreezeV3) error {
	if f.SchemaVersion != RoleQualificationExecutionFreezeSchemaV3 || f.FreezeID != "role-qualification-execution-freeze-v3" || f.Status != "corrected_frozen_provider_locked" || f.V1FreezeSHA256 != RoleQualificationFreezeV1SHA256() || f.RejectedV2FreezeSHA256 != "39f31fd1725d38cf00640f53e35d2b09b54618c6923c6b9458d2851d1b2e4" {
		return fmt.Errorf("execution v3 freeze identity drift")
	}
	if f.Cells != 36 || f.ProviderCallsAllowed || f.RuntimeWiring != "none" || f.Retries != 0 || f.Fallbacks {
		return fmt.Errorf("execution v3 freeze authority drift")
	}
	if len(f.Artifacts) != 10 || len(f.TransportAntecedents) != 4 {
		return fmt.Errorf("execution v3 freeze coverage drift")
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
	for path, want := range f.TransportAntecedents {
		if err := validateExactFileHashV3(root, path, want); err != nil {
			return err
		}
	}
	return nil
}
func validateExactFileHashV3(root, path, want string) error {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		return fmt.Errorf("freeze read %s: %w", path, err)
	}
	sum := sha256.Sum256(raw)
	got := hex.EncodeToString(sum[:])
	if got != want {
		return fmt.Errorf("frozen byte drift %s got=%s want=%s", path, got, want)
	}
	return nil
}
func roleQualificationRepoRootV3() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("cannot resolve freeze source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..")), nil
}
func RoleQualificationExecutionFreezeV3SHA256() string {
	sum := sha256.Sum256(roleQualificationExecutionFreezeV3JSON)
	return hex.EncodeToString(sum[:])
}
