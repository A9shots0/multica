package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrGuardedClaimExposureDenied    = errors.New("guarded_claim_exposure_denied")
	ErrGuardedEnvironmentNotIsolated = errors.New("guarded_environment_not_isolated")
)

// GuardedClaimExposureFactsV1 contains claim data independently loaded from
// the runtime and agent rows. It is intentionally value-only: task context and
// the guarded package cannot assert these facts.
type GuardedClaimExposureFactsV1 struct {
	RuntimeProvider string
	AgentName       string
	Model           string
	ReasoningEffort string
	ServiceTier     string
	Instructions    string
	SkillSHA256     []string
	CustomEnvCount  int
	CustomArgsCount int
	LazySkills      bool
	DisabledSkills  bool
	DirectMCP       bool
	RuntimeConfig   bool
	ConnectedApps   bool
	RemoteMCP       bool
	PluginHooks     bool
}

// ValidateGuardedClaimExposureV1 permits the server to return only the exact
// accepted instruction/skill pair and no unbrokered configuration or
// credential-bearing surface. External brokers remain denied in this slice;
// their last-mile guards exist, but a positive claim must not expose them
// until durable scope is wired through every resolver.
func ValidateGuardedClaimExposureV1(pkg *GuardedExecutionPackageV1, actual GuardedClaimExposureFactsV1) error {
	if pkg == nil {
		return nil
	}
	if pkg.Envelope.RuntimeProvider != actual.RuntimeProvider || pkg.Envelope.Model != actual.Model || pkg.Envelope.ReasoningEffort != actual.ReasoningEffort {
		return fmt.Errorf("%w: runtime configuration drifted", ErrGuardedClaimExposureDenied)
	}
	if strings.TrimSpace(actual.ServiceTier) != "" {
		return fmt.Errorf("%w: unbound service tier", ErrGuardedClaimExposureDenied)
	}
	if pkg.CapabilityScope.WorkflowBroker != nil && actual.AgentName != pkg.CapabilityScope.WorkflowBroker.AgentName {
		return fmt.Errorf("%w: workflow broker agent identity drifted", ErrGuardedClaimExposureDenied)
	}
	instructionSum := sha256.Sum256([]byte(actual.Instructions))
	if hex.EncodeToString(instructionSum[:]) != pkg.Envelope.ArtifactBinding.Instruction.SHA256 {
		return fmt.Errorf("%w: instruction artifact drifted", ErrGuardedClaimExposureDenied)
	}
	if len(actual.SkillSHA256) != 1 || !strings.EqualFold(actual.SkillSHA256[0], pkg.Envelope.ArtifactBinding.Skill.SHA256) {
		return fmt.Errorf("%w: skill artifact set drifted", ErrGuardedClaimExposureDenied)
	}
	if actual.CustomEnvCount != 0 || actual.CustomArgsCount != 0 || actual.LazySkills || actual.DisabledSkills || actual.DirectMCP || actual.RuntimeConfig || actual.ConnectedApps || actual.RemoteMCP || actual.PluginHooks {
		return fmt.Errorf("%w: unapproved environment or credential surface", ErrGuardedClaimExposureDenied)
	}
	return nil
}

// ValidateGuardedHostExecutionV1 independently binds the daemon's real
// runtime configuration, host, executable bytes, exact child environment, and
// tool-free adapter to the authenticated package. Any drift remains a hard
// pre-launch denial rather than an operator-overridable warning.
func ValidateGuardedHostExecutionV1(pkg *GuardedExecutionPackageV1, expected GuardedExecutionExpectedV1, actual GuardedHostExecutionFactsV1, now time.Time) error {
	if pkg == nil {
		return nil
	}
	if _, err := ValidateGuardedExecutionPackageV1(pkg, expected, now); err != nil {
		return err
	}
	return validateGuardedHostExecutionFactsV1(pkg.RuntimeBinding.ChildExecution, actual)
}
