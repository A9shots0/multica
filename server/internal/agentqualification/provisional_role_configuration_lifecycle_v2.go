package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"time"
)

const ProvisionalRoleConfigurationLifecycleV2Schema = "silver-mediums.provisional-role-configuration-lifecycle.v2"

type ProvisionalRoleConfigurationLifecycleEntryV2 struct {
	ConfigurationID     string `json:"configuration_id"`
	ConfigurationSHA256 string `json:"configuration_sha256"`
	Status              string `json:"status"`
	ValidFrom           string `json:"valid_from"`
	ValidUntil          string `json:"valid_until"`
}

type ProvisionalRoleConfigurationTombstoneV2 struct {
	ConfigurationID     string `json:"configuration_id"`
	ConfigurationSHA256 string `json:"configuration_sha256"`
	RetiredAt           string `json:"retired_at"`
	Reason              string `json:"reason"`
}

type ProvisionalRoleConfigurationRevocationV2 struct {
	ConfigurationID     string `json:"configuration_id"`
	ConfigurationSHA256 string `json:"configuration_sha256"`
	RevokedAt           string `json:"revoked_at"`
	Reason              string `json:"reason"`
}

type ProvisionalRoleConfigurationLifecycleV2 struct {
	SchemaVersion string                                       `json:"schema_version"`
	RegistryID    string                                       `json:"registry_id"`
	ControllerID  string                                       `json:"controller_id"`
	Generation    int                                          `json:"generation"`
	Active        ProvisionalRoleConfigurationLifecycleEntryV2 `json:"active"`
	Retired       []ProvisionalRoleConfigurationTombstoneV2    `json:"retired"`
	Revoked       []ProvisionalRoleConfigurationRevocationV2   `json:"revoked"`
}

//go:embed provisional_role_configuration_lifecycle_v2.json
var provisionalRoleConfigurationLifecycleV2JSON []byte

func CurrentProvisionalRoleConfigurationLifecycleV2() (ProvisionalRoleConfigurationLifecycleV2, error) {
	var lifecycle ProvisionalRoleConfigurationLifecycleV2
	if err := strictJSON(provisionalRoleConfigurationLifecycleV2JSON, &lifecycle); err != nil {
		return lifecycle, fmt.Errorf("decode provisional role configuration lifecycle v2: %w", err)
	}
	if err := ValidateProvisionalRoleConfigurationLifecycleV2(lifecycle); err != nil {
		return ProvisionalRoleConfigurationLifecycleV2{}, err
	}
	if lifecycle.Active.Status != "active" || len(lifecycle.Retired) != 1 || len(lifecycle.Revoked) != 0 {
		return ProvisionalRoleConfigurationLifecycleV2{}, fmt.Errorf("current provisional role configuration lifecycle v2 state drifted")
	}
	return lifecycle, nil
}

func ProvisionalRoleConfigurationLifecycleV2SHA256() string {
	sum := sha256.Sum256(provisionalRoleConfigurationLifecycleV2JSON)
	return hex.EncodeToString(sum[:])
}

func ValidateProvisionalRoleConfigurationLifecycleV2(lifecycle ProvisionalRoleConfigurationLifecycleV2) error {
	if lifecycle.SchemaVersion != ProvisionalRoleConfigurationLifecycleV2Schema || lifecycle.RegistryID != "item-14-controller-owned-provisional-configuration-registry-20260915" || lifecycle.ControllerID != "00000000-0000-4000-8000-000000001402" || lifecycle.Generation != 2 {
		return fmt.Errorf("provisional role configuration lifecycle v2 identity drifted")
	}
	active := lifecycle.Active
	if active.ConfigurationID != "item-14-local-r01-guarded-claude-composite-20260915" || active.ConfigurationSHA256 != ProvisionalRoleConfigurationV2SHA256() || (active.Status != "active" && active.Status != "retired" && active.Status != "revoked") || active.ValidFrom != "2026-09-15T13:15:00Z" || active.ValidUntil != "2026-09-22T13:15:00Z" {
		return fmt.Errorf("provisional role configuration lifecycle v2 active entry drifted")
	}
	if len(lifecycle.Retired) != 1 {
		return fmt.Errorf("provisional role configuration lifecycle v2 tombstone coverage drifted")
	}
	retired := lifecycle.Retired[0]
	if retired.ConfigurationID != "item-14-local-r01-inline-architecture-20260914" || retired.ConfigurationSHA256 != ProvisionalRoleConfigurationV1SHA256() || retired.RetiredAt != active.ValidFrom || retired.Reason != "superseded_by_item_14_provisional_configuration_v2" {
		return fmt.Errorf("provisional role configuration lifecycle v2 predecessor tombstone drifted")
	}
	if len(lifecycle.Revoked) > 1 {
		return fmt.Errorf("provisional role configuration lifecycle v2 revocation coverage drifted")
	}
	if len(lifecycle.Revoked) == 1 {
		revoked := lifecycle.Revoked[0]
		if revoked.ConfigurationID != active.ConfigurationID || revoked.ConfigurationSHA256 != active.ConfigurationSHA256 || revoked.RevokedAt == "" || revoked.Reason == "" {
			return fmt.Errorf("provisional role configuration lifecycle v2 revocation drifted")
		}
	}
	return nil
}

func ResolveProvisionalRoleConfigurationLifecycleV2(lifecycle ProvisionalRoleConfigurationLifecycleV2, configurationID, configurationSHA256 string, now time.Time) (bool, string) {
	if err := ValidateProvisionalRoleConfigurationLifecycleV2(lifecycle); err != nil {
		return false, "lifecycle_invalid"
	}
	for _, retired := range lifecycle.Retired {
		if retired.ConfigurationID == configurationID && retired.ConfigurationSHA256 == configurationSHA256 {
			return false, "superseded_configuration"
		}
	}
	active := lifecycle.Active
	if active.ConfigurationID != configurationID || active.ConfigurationSHA256 != configurationSHA256 {
		return false, "configuration_not_registered"
	}
	if active.Status == "retired" {
		return false, "configuration_retired"
	}
	if active.Status == "revoked" {
		return false, "configuration_revoked"
	}
	for _, revoked := range lifecycle.Revoked {
		if revoked.ConfigurationID == configurationID && revoked.ConfigurationSHA256 == configurationSHA256 {
			return false, "configuration_revoked"
		}
	}
	validFrom, _ := time.Parse(time.RFC3339, active.ValidFrom)
	validUntil, _ := time.Parse(time.RFC3339, active.ValidUntil)
	now = now.UTC()
	if now.Before(validFrom) {
		return false, "configuration_not_yet_valid"
	}
	if !now.Before(validUntil) {
		return false, "configuration_expired"
	}
	return true, "configuration_active"
}
