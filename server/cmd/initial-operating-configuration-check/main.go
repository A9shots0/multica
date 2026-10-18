// Command initial-operating-configuration-check verifies the initial operating
// configuration without contacting any provider. It prints the controller's
// live decision and an admission matrix over representative issue facts, so the
// configuration can be checked before anything is activated.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

type admissionSample struct {
	Name     string `json:"name"`
	Admitted bool   `json:"admitted"`
	Reason   string `json:"reason"`
}

func main() {
	at := flag.String("at", "", "RFC3339 instant to evaluate (default: now)")
	flag.Parse()

	now := time.Now().UTC()
	if *at != "" {
		parsed, err := time.Parse(time.RFC3339, *at)
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse -at: %v\n", err)
			os.Exit(2)
		}
		now = parsed.UTC()
	}

	configuration, err := agentqualification.CurrentInitialOperatingConfigurationV1()
	if err != nil {
		fmt.Fprintf(os.Stderr, "initial operating configuration: %v\n", err)
		os.Exit(1)
	}
	lifecycle, err := agentqualification.CurrentInitialOperatingConfigurationLifecycleV1()
	if err != nil {
		fmt.Fprintf(os.Stderr, "initial operating configuration lifecycle: %v\n", err)
		os.Exit(1)
	}
	acceptance, err := agentqualification.CurrentNativePublicationAcceptanceV1()
	if err != nil {
		fmt.Fprintf(os.Stderr, "native publication acceptance: %v\n", err)
		os.Exit(1)
	}

	eligible := agentqualification.InitialOperatingAdmissionFactsV1{
		Repository:                         configuration.EligibilityRequirements.Repository,
		Host:                               configuration.EligibilityRequirements.Host,
		WorkspaceInternal:                  true,
		IssueState:                         configuration.EligibilityRequirements.AdmissibleIssueStates[0],
		OwnerStarted:                       true,
		ExecutionPlanNamesDistinctReviewer: true,
		DeclaredReleaseGate:                configuration.EligibilityRequirements.RequiredDeclaredReleaseGate,
	}

	samples := []admissionSample{}
	record := func(name string, facts agentqualification.InitialOperatingAdmissionFactsV1) {
		admitted, reason := agentqualification.EvaluateInitialOperatingAdmissionV1(configuration, facts, now)
		samples = append(samples, admissionSample{Name: name, Admitted: admitted, Reason: reason})
	}
	record("eligible_internal_issue", eligible)

	otherRepository := eligible
	otherRepository.Repository = "another-repository"
	record("other_repository", otherRepository)

	otherHost := eligible
	otherHost.Host = "mac_studio"
	record("other_host", otherHost)

	releaseWork := eligible
	releaseWork.DeclaredReleaseGate = "deployment_required"
	record("release_gated_issue", releaseWork)

	notOwnerStarted := eligible
	notOwnerStarted.OwnerStarted = false
	record("not_owner_started", notOwnerStarted)

	for _, condition := range agentqualification.InitialOperatingOwnerGatedConditionsV1() {
		gated := eligible
		gated.OwnerGatedConditions = []string{condition}
		record("owner_gated:"+condition, gated)
	}

	unknown := eligible
	unknown.OwnerGatedConditions = []string{"condition_with_no_rule"}
	record("unrecognised_condition", unknown)

	decision, err := agentqualification.EvaluateInitialOperatingConfigurationAt(eligible, now)
	if err != nil {
		fmt.Fprintf(os.Stderr, "evaluate initial operating configuration: %v\n", err)
		os.Exit(1)
	}

	output := map[string]any{
		"evaluated_at":                    now.Format(time.RFC3339),
		"configuration_id":                configuration.ConfigurationID,
		"configuration_sha256":            agentqualification.InitialOperatingConfigurationV1SHA256(),
		"lifecycle_sha256":                agentqualification.InitialOperatingConfigurationLifecycleV1SHA256(),
		"lifecycle_registry_id":           lifecycle.RegistryID,
		"lifecycle_status":                lifecycle.Active.Status,
		"valid_from":                      configuration.ValidFrom,
		"valid_until":                     configuration.ValidUntil,
		"acceptance_id":                   acceptance.AcceptanceID,
		"acceptance_sha256":               agentqualification.NativePublicationAcceptanceV1SHA256(),
		"provider_calls_made":             0,
		"credentials_read":                0,
		"decision":                        decision,
		"admission_samples":               samples,
		"permanent_agent_seats":           configuration.PermanentAgentSeats,
		"automatic_dispatch_allowed":      configuration.AutomaticDispatchAllowed,
		"public_access_allowed":           configuration.PublicAccessAllowed,
		"production_access_allowed":       configuration.ProductionAccessAllowed,
		"other_repository_access_allowed": configuration.OtherRepositoryAccessAllowed,
		"phone_farm_windows_access":       configuration.PhoneFarmWindowsAccessAllowed,
		"recovery_sensitive_authority":    configuration.RecoverySensitiveAuthority,
		"review_independence_class":       configuration.ReviewIndependenceClass,
		"provisional_items":               configuration.ProvisionalItems,
		"reevaluation_triggers":           configuration.ReevaluationTriggers,
		"change_path":                     configuration.ChangePath,
		"rollback_action":                 configuration.RollbackAction,
	}
	encoded, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "encode result: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(encoded))
	if !decision.LifecycleAdmitted || !decision.IssueAdmitted {
		os.Exit(3)
	}
}
