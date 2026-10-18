package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"time"
)

const InitialOperatingConfigurationLifecycleV1Schema = "silver-mediums.initial-operating-configuration-lifecycle.v1"

const initialOperatingConfigurationRegistryIDV1 = "silver-mediums-controller-owned-initial-operating-registry-20260916"

const initialOperatingConfigurationControllerIDV1 = "00000000-0000-4000-8000-000000001501"

type InitialOperatingLifecycleEntryV1 struct {
	ConfigurationID     string `json:"configuration_id"`
	ConfigurationSHA256 string `json:"configuration_sha256"`
	Status              string `json:"status"`
	ValidFrom           string `json:"valid_from"`
	ValidUntil          string `json:"valid_until"`
}

type InitialOperatingLifecycleTombstoneV1 struct {
	ConfigurationID     string `json:"configuration_id"`
	ConfigurationSHA256 string `json:"configuration_sha256"`
	RetiredAt           string `json:"retired_at"`
	Reason              string `json:"reason"`
}

type InitialOperatingLifecycleRevocationV1 struct {
	ConfigurationID     string `json:"configuration_id"`
	ConfigurationSHA256 string `json:"configuration_sha256"`
	RevokedAt           string `json:"revoked_at"`
	Reason              string `json:"reason"`
}

// InitialOperatingActivationV1 records who authorized this configuration to go
// live and under what scope. Without it the registry could show an active
// configuration that nobody is recorded as having turned on.
type InitialOperatingActivationV1 struct {
	AuthorizedBy           string `json:"authorized_by"`
	AuthorizationReference string `json:"authorization_reference"`
	ActivatedAt            string `json:"activated_at"`
	Scope                  string `json:"scope"`
}

type InitialOperatingConfigurationLifecycleV1 struct {
	SchemaVersion string                                  `json:"schema_version"`
	RegistryID    string                                  `json:"registry_id"`
	ControllerID  string                                  `json:"controller_id"`
	Generation    int                                     `json:"generation"`
	Activation    InitialOperatingActivationV1            `json:"activation"`
	Active        InitialOperatingLifecycleEntryV1        `json:"active"`
	Retired       []InitialOperatingLifecycleTombstoneV1  `json:"retired"`
	Revoked       []InitialOperatingLifecycleRevocationV1 `json:"revoked"`
}

//go:embed initial_operating_configuration_lifecycle_v1.json
var initialOperatingConfigurationLifecycleV1JSON []byte

func CurrentInitialOperatingConfigurationLifecycleV1() (InitialOperatingConfigurationLifecycleV1, error) {
	var lifecycle InitialOperatingConfigurationLifecycleV1
	if err := strictJSON(initialOperatingConfigurationLifecycleV1JSON, &lifecycle); err != nil {
		return lifecycle, fmt.Errorf("decode initial operating configuration lifecycle v1: %w", err)
	}
	return lifecycle, ValidateInitialOperatingConfigurationLifecycleV1(lifecycle)
}

func InitialOperatingConfigurationLifecycleV1SHA256() string {
	sum := sha256.Sum256(initialOperatingConfigurationLifecycleV1JSON)
	return hex.EncodeToString(sum[:])
}

func ValidateInitialOperatingConfigurationLifecycleV1(lifecycle InitialOperatingConfigurationLifecycleV1) error {
	if lifecycle.SchemaVersion != InitialOperatingConfigurationLifecycleV1Schema || lifecycle.RegistryID != initialOperatingConfigurationRegistryIDV1 || lifecycle.ControllerID != initialOperatingConfigurationControllerIDV1 || lifecycle.Generation != 1 {
		return fmt.Errorf("initial operating configuration lifecycle v1 identity drifted")
	}
	activation := lifecycle.Activation
	if activation.AuthorizedBy != "owner" || activation.AuthorizationReference == "" || activation.Scope == "" {
		return fmt.Errorf("initial operating configuration lifecycle v1 has no recorded owner activation")
	}
	if _, err := time.Parse(time.RFC3339, activation.ActivatedAt); err != nil {
		return fmt.Errorf("parse initial operating configuration lifecycle v1 activated_at: %w", err)
	}
	active := lifecycle.Active
	if active.ConfigurationID != initialOperatingConfigurationV1ID || active.ConfigurationSHA256 != InitialOperatingConfigurationV1SHA256() {
		return fmt.Errorf("initial operating configuration lifecycle v1 active binding drifted")
	}
	if active.Status != "active" && active.Status != "retired" && active.Status != "revoked" {
		return fmt.Errorf("initial operating configuration lifecycle v1 active status is not a known state")
	}
	if active.ValidFrom != "2026-09-16T00:00:00Z" || active.ValidUntil != "2026-09-30T00:00:00Z" {
		return fmt.Errorf("initial operating configuration lifecycle v1 active window drifted")
	}
	for _, retired := range lifecycle.Retired {
		if retired.ConfigurationID == "" || retired.ConfigurationSHA256 == "" || retired.RetiredAt == "" || retired.Reason == "" {
			return fmt.Errorf("initial operating configuration lifecycle v1 tombstone is incomplete")
		}
	}
	for _, revoked := range lifecycle.Revoked {
		if revoked.ConfigurationID == "" || revoked.ConfigurationSHA256 == "" || revoked.RevokedAt == "" || revoked.Reason == "" {
			return fmt.Errorf("initial operating configuration lifecycle v1 revocation is incomplete")
		}
	}
	// The status word and the lists have to agree. Half an edit — clearing the
	// revocation entry but leaving the status, or the reverse — fails closed
	// rather than quietly restoring a configuration somebody switched off.
	listedRevoked := false
	for _, revoked := range lifecycle.Revoked {
		if revoked.ConfigurationID == active.ConfigurationID && revoked.ConfigurationSHA256 == active.ConfigurationSHA256 {
			listedRevoked = true
		}
	}
	if (active.Status == "revoked") != listedRevoked {
		return fmt.Errorf("initial operating configuration lifecycle v1 revocation status and record disagree")
	}
	listedRetired := false
	for _, retired := range lifecycle.Retired {
		if retired.ConfigurationID == active.ConfigurationID && retired.ConfigurationSHA256 == active.ConfigurationSHA256 {
			listedRetired = true
		}
	}
	if (active.Status == "retired") != listedRetired {
		return fmt.Errorf("initial operating configuration lifecycle v1 retirement status and record disagree")
	}
	return nil
}

// ResolveInitialOperatingConfigurationLifecycleV1 answers only the controller's
// question: is this exact configuration allowed to admit work right now? The
// requester cannot supply the clock, the status or the window.
func ResolveInitialOperatingConfigurationLifecycleV1(lifecycle InitialOperatingConfigurationLifecycleV1, configurationID, configurationSHA256 string, now time.Time) (bool, string) {
	if err := ValidateInitialOperatingConfigurationLifecycleV1(lifecycle); err != nil {
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
	validFrom, err := time.Parse(time.RFC3339, active.ValidFrom)
	if err != nil {
		return false, "lifecycle_invalid"
	}
	validUntil, err := time.Parse(time.RFC3339, active.ValidUntil)
	if err != nil {
		return false, "lifecycle_invalid"
	}
	now = now.UTC()
	if now.Before(validFrom) {
		return false, "configuration_not_yet_valid"
	}
	if !now.Before(validUntil) {
		return false, "configuration_expired"
	}
	return true, "configuration_active"
}

// InitialOperatingDecisionV1 is the whole answer for one issue: whether the
// configuration is live, and whether this issue may run under it.
type InitialOperatingDecisionV1 struct {
	ConfigurationID     string `json:"configuration_id"`
	ConfigurationSHA256 string `json:"configuration_sha256"`
	LifecycleAdmitted   bool   `json:"lifecycle_admitted"`
	LifecycleReason     string `json:"lifecycle_reason"`
	IssueAdmitted       bool   `json:"issue_admitted"`
	IssueReason         string `json:"issue_reason"`
	EvaluatedAt         string `json:"evaluated_at"`
}

func EvaluateInitialOperatingConfigurationAt(facts InitialOperatingAdmissionFactsV1, now time.Time) (InitialOperatingDecisionV1, error) {
	decision := InitialOperatingDecisionV1{
		ConfigurationID:     initialOperatingConfigurationV1ID,
		ConfigurationSHA256: InitialOperatingConfigurationV1SHA256(),
		EvaluatedAt:         now.UTC().Format(time.RFC3339),
	}
	configuration, err := CurrentInitialOperatingConfigurationV1()
	if err != nil {
		return decision, err
	}
	lifecycle, err := CurrentInitialOperatingConfigurationLifecycleV1()
	if err != nil {
		return decision, err
	}
	decision.LifecycleAdmitted, decision.LifecycleReason = ResolveInitialOperatingConfigurationLifecycleV1(lifecycle, configuration.ConfigurationID, InitialOperatingConfigurationV1SHA256(), now)
	if !decision.LifecycleAdmitted {
		decision.IssueReason = "configuration_not_live"
		return decision, nil
	}
	decision.IssueAdmitted, decision.IssueReason = EvaluateInitialOperatingAdmissionV1(configuration, facts, now)
	return decision, nil
}
