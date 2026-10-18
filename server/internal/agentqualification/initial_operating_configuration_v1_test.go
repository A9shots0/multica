package agentqualification

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func insideWindow() time.Time {
	return time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
}

func eligibleFacts() InitialOperatingAdmissionFactsV1 {
	return InitialOperatingAdmissionFactsV1{
		Repository:                         "SMK-multica-selfhost-pilot",
		Host:                               "alienware_windows",
		WorkspaceInternal:                  true,
		IssueState:                         "received",
		OwnerStarted:                       true,
		ExecutionPlanNamesDistinctReviewer: true,
		DeclaredReleaseGate:                "none",
		ExistingProviderTasks:              0,
		ConcurrentEligibleIssues:           0,
	}
}

// Golden hash. Any edit to the shipped record changes it, so this fails
// independently of the validator rather than restating it.
func TestInitialOperatingConfigurationV1IsTheReviewedRecord(t *testing.T) {
	const reviewed = "dadf6a215f9ed8fb6d4ed5b957a6be143e3226dab080452c4c9d63454aaffa6d"
	if got := InitialOperatingConfigurationV1SHA256(); got != reviewed {
		t.Fatalf("shipped configuration is not the reviewed one: got %s, want %s (update the doc and the lifecycle registry together with this constant)", got, reviewed)
	}
	lifecycle, err := CurrentInitialOperatingConfigurationLifecycleV1()
	if err != nil {
		t.Fatalf("current initial operating configuration lifecycle v1: %v", err)
	}
	if lifecycle.Active.ConfigurationSHA256 != reviewed {
		t.Fatalf("the lifecycle registry points at a different configuration: %s", lifecycle.Active.ConfigurationSHA256)
	}
}

// The record must not describe an interlock the product does not have, an
// effort the accepted run never observed, or a failed review with nowhere to go.
func TestInitialOperatingConfigurationV1DoesNotOverstateItself(t *testing.T) {
	configuration, err := CurrentInitialOperatingConfigurationV1()
	if err != nil {
		t.Fatalf("current initial operating configuration v1: %v", err)
	}
	if configuration.Enforcement != "declarative_contract_checked_out_of_band" {
		t.Fatalf("the record claims enforcement it does not have: %q", configuration.Enforcement)
	}
	if !strings.Contains(configuration.EnforcementNote, "no server, controller or daemon code path consults this record") {
		t.Fatalf("the record does not say that nothing consults it at runtime: %q", configuration.EnforcementNote)
	}
	if strings.Contains(configuration.RollbackAction, "immediately") {
		t.Fatalf("rollback_action still promises an immediate runtime effect: %q", configuration.RollbackAction)
	}
	if configuration.ReasoningEffortEvidence != "requested_only_not_observed_in_the_accepted_run" {
		t.Fatalf("the record overstates its effort evidence: %q", configuration.ReasoningEffortEvidence)
	}
	if configuration.ReviewFailedState != "blocked" || configuration.ReviewFailedEscalation != "owner_decision_required" {
		t.Fatalf("a failed review has no declared destination: %+v", configuration)
	}
	acceptance, err := CurrentNativePublicationAcceptanceV1()
	if err != nil {
		t.Fatalf("current native publication acceptance v1: %v", err)
	}
	if configuration.MaximumProviderTasksPerIssue > acceptance.PublisherTurns+acceptance.ReviewerTurns {
		t.Fatalf("the configuration allows more provider work than the accepted run")
	}
}

func TestInitialOperatingConfigurationV1RejectsUnknownFields(t *testing.T) {
	var configuration InitialOperatingConfigurationV1
	if err := strictJSON([]byte(`{"schema_version":"silver-mediums.initial-operating-configuration.v1","unexpected":true}`), &configuration); err == nil {
		t.Fatalf("an unknown configuration field was accepted")
	}
}

func TestInitialOperatingConfigurationV1FailsClosedOnEveryMutation(t *testing.T) {
	base, err := CurrentInitialOperatingConfigurationV1()
	if err != nil {
		t.Fatalf("current initial operating configuration v1: %v", err)
	}
	mutations := map[string]func(*InitialOperatingConfigurationV1){
		"identity": func(c *InitialOperatingConfigurationV1) { c.ConfigurationID = "other" },
		"status":   func(c *InitialOperatingConfigurationV1) { c.Status = "permanent" },
		"window":   func(c *InitialOperatingConfigurationV1) { c.ValidUntil = "2027-09-30T00:00:00Z" },
		"source_hash": func(c *InitialOperatingConfigurationV1) {
			c.SourceHashes.NativePublicationAcceptance = strings.Repeat("0", 64)
		},
		"work_class":     func(c *InitialOperatingConfigurationV1) { c.EligibleWorkClass = "anything" },
		"admission_mode": func(c *InitialOperatingConfigurationV1) { c.AdmissionMode = "automatic" },
		"repository":     func(c *InitialOperatingConfigurationV1) { c.EligibilityRequirements.Repository = "other-repo" },
		"host":           func(c *InitialOperatingConfigurationV1) { c.EligibilityRequirements.Host = "mac_studio" },
		"issue_states": func(c *InitialOperatingConfigurationV1) {
			c.EligibilityRequirements.AdmissibleIssueStates = []string{"live"}
		},
		"owner_gate_dropped":      func(c *InitialOperatingConfigurationV1) { c.OwnerGatedConditions = c.OwnerGatedConditions[1:] },
		"owner_gate_state":        func(c *InitialOperatingConfigurationV1) { c.OwnerGateState = "queued" },
		"route":                   func(c *InitialOperatingConfigurationV1) { c.RouteStages = []string{"controller_intake"} },
		"reviewer_role":           func(c *InitialOperatingConfigurationV1) { c.ReviewerRole = "shared_windows_executor" },
		"shared_session":          func(c *InitialOperatingConfigurationV1) { c.SeparateExecutionAndReviewSessions = false },
		"review_dropped":          func(c *InitialOperatingConfigurationV1) { c.IndependentReviewRequired = false },
		"independence_overstated": func(c *InitialOperatingConfigurationV1) { c.ReviewIndependenceClass = "distinct_provider" },
		"permanent_seat":          func(c *InitialOperatingConfigurationV1) { c.PermanentAgentSeats = 1 },
		"permanent_selection":     func(c *InitialOperatingConfigurationV1) { c.PermanentRoleOrModelSelectionAllowed = true },
		"more_provider_tasks":     func(c *InitialOperatingConfigurationV1) { c.MaximumProviderTasksPerIssue = 5 },
		"corrections_enabled":     func(c *InitialOperatingConfigurationV1) { c.MaximumCorrections = 1 },
		"retries_enabled":         func(c *InitialOperatingConfigurationV1) { c.Retries = 1 },
		"fallbacks_enabled":       func(c *InitialOperatingConfigurationV1) { c.Fallbacks = true },
		"automatic_dispatch":      func(c *InitialOperatingConfigurationV1) { c.AutomaticDispatchAllowed = true },
		"public_access":           func(c *InitialOperatingConfigurationV1) { c.PublicAccessAllowed = true },
		"production_access":       func(c *InitialOperatingConfigurationV1) { c.ProductionAccessAllowed = true },
		"other_production":        func(c *InitialOperatingConfigurationV1) { c.OtherProductionSystemAccessAllowed = true },
		"other_repository":        func(c *InitialOperatingConfigurationV1) { c.OtherRepositoryAccessAllowed = true },
		"outbound":                func(c *InitialOperatingConfigurationV1) { c.OutboundActionsAllowed = true },
		"phone_farm":              func(c *InitialOperatingConfigurationV1) { c.PhoneFarmWindowsAccessAllowed = true },
		"provider_tools":          func(c *InitialOperatingConfigurationV1) { c.ProviderChildToolsAllowed = true },
		"credential_read":         func(c *InitialOperatingConfigurationV1) { c.ConfigurationAuthorizesCredentialRead = true },
		"recovery_authority":      func(c *InitialOperatingConfigurationV1) { c.RecoverySensitiveAuthority = "granted" },
		"release_authority":       func(c *InitialOperatingConfigurationV1) { c.ReleaseAndRuntimeVerificationAuthority = "agent_operated" },
		"execution_model":         func(c *InitialOperatingConfigurationV1) { c.ExecutionLane.RequestedPrimaryModel = "claude-opus-5" },
		"exact_model_claim":       func(c *InitialOperatingConfigurationV1) { c.ReviewLane.ExactSingleModelClaimAllowed = true },
		"provisional_erased":      func(c *InitialOperatingConfigurationV1) { c.ProvisionalItems = nil },
		"triggers_erased":         func(c *InitialOperatingConfigurationV1) { c.ReevaluationTriggers = nil },
		"not_reversible":          func(c *InitialOperatingConfigurationV1) { c.Reversible = false },
		"silent_supersession": func(c *InitialOperatingConfigurationV1) {
			c.SupersedesConfigurationID = "item-14-local-r01-guarded-claude-composite-20260915"
		},
	}
	for name, mutate := range mutations {
		mutated := base
		mutated.OwnerGatedConditions = slices.Clone(base.OwnerGatedConditions)
		mutated.ProvisionalItems = slices.Clone(base.ProvisionalItems)
		mutated.ReevaluationTriggers = slices.Clone(base.ReevaluationTriggers)
		mutated.RouteStages = slices.Clone(base.RouteStages)
		mutate(&mutated)
		if err := ValidateInitialOperatingConfigurationV1(mutated); err == nil {
			t.Fatalf("mutation %q was accepted", name)
		}
	}
}

func TestInitialOperatingAdmissionAdmitsOneEligibleIssue(t *testing.T) {
	configuration, err := CurrentInitialOperatingConfigurationV1()
	if err != nil {
		t.Fatalf("current initial operating configuration v1: %v", err)
	}
	for _, state := range configuration.EligibilityRequirements.AdmissibleIssueStates {
		facts := eligibleFacts()
		facts.IssueState = state
		admitted, reason := EvaluateInitialOperatingAdmissionV1(configuration, facts, insideWindow())
		if !admitted || reason != "admitted" {
			t.Fatalf("eligible issue in state %q was denied: %s", state, reason)
		}
	}
}

func TestInitialOperatingAdmissionDeniesEveryOutOfScopeFact(t *testing.T) {
	configuration, err := CurrentInitialOperatingConfigurationV1()
	if err != nil {
		t.Fatalf("current initial operating configuration v1: %v", err)
	}
	denials := map[string]struct {
		mutate func(*InitialOperatingAdmissionFactsV1)
		reason string
	}{
		"other repository":     {func(f *InitialOperatingAdmissionFactsV1) { f.Repository = "SMC" }, "repository_out_of_scope"},
		"other host":           {func(f *InitialOperatingAdmissionFactsV1) { f.Host = "mac_studio" }, "host_out_of_scope"},
		"external workspace":   {func(f *InitialOperatingAdmissionFactsV1) { f.WorkspaceInternal = false }, "workspace_not_internal"},
		"wrong state":          {func(f *InitialOperatingAdmissionFactsV1) { f.IssueState = "working" }, "issue_state_not_admissible"},
		"not owner started":    {func(f *InitialOperatingAdmissionFactsV1) { f.OwnerStarted = false }, "owner_did_not_start_this_issue"},
		"no distinct reviewer": {func(f *InitialOperatingAdmissionFactsV1) { f.ExecutionPlanNamesDistinctReviewer = false }, "execution_plan_does_not_name_a_distinct_reviewer"},
		"release gate":         {func(f *InitialOperatingAdmissionFactsV1) { f.DeclaredReleaseGate = "deployment_required" }, "declared_release_gate_out_of_scope"},
		"existing tasks":       {func(f *InitialOperatingAdmissionFactsV1) { f.ExistingProviderTasks = 1 }, "issue_already_has_provider_tasks"},
		"already busy":         {func(f *InitialOperatingAdmissionFactsV1) { f.ConcurrentEligibleIssues = 1 }, "concurrent_eligible_issue_limit_reached"},
	}
	for name, denial := range denials {
		facts := eligibleFacts()
		denial.mutate(&facts)
		admitted, reason := EvaluateInitialOperatingAdmissionV1(configuration, facts, insideWindow())
		if admitted || reason != denial.reason {
			t.Fatalf("%s: admitted=%v reason=%q, want denial %q", name, admitted, reason, denial.reason)
		}
	}
}

func TestInitialOperatingAdmissionSendsEveryOwnerDecisionToTheOwner(t *testing.T) {
	configuration, err := CurrentInitialOperatingConfigurationV1()
	if err != nil {
		t.Fatalf("current initial operating configuration v1: %v", err)
	}
	for _, condition := range InitialOperatingOwnerGatedConditionsV1() {
		facts := eligibleFacts()
		facts.OwnerGatedConditions = []string{condition}
		admitted, reason := EvaluateInitialOperatingAdmissionV1(configuration, facts, insideWindow())
		if admitted || reason != condition {
			t.Fatalf("owner-gated condition %q: admitted=%v reason=%q", condition, admitted, reason)
		}
	}
	facts := eligibleFacts()
	facts.OwnerGatedConditions = []string{"something_the_configuration_has_never_heard_of"}
	admitted, reason := EvaluateInitialOperatingAdmissionV1(configuration, facts, insideWindow())
	if admitted || reason != "unrecognised_condition_requires_owner" {
		t.Fatalf("unrecognised condition: admitted=%v reason=%q", admitted, reason)
	}
	// An unrecognised condition anywhere in the list still denies.
	facts.OwnerGatedConditions = []string{"requires_outbound_communication", "brand_new_condition"}
	admitted, reason = EvaluateInitialOperatingAdmissionV1(configuration, facts, insideWindow())
	if admitted || reason != "unrecognised_condition_requires_owner" {
		t.Fatalf("mixed conditions: admitted=%v reason=%q", admitted, reason)
	}
}

func TestInitialOperatingLifecycleControlsTheWindowAndRevocation(t *testing.T) {
	lifecycle, err := CurrentInitialOperatingConfigurationLifecycleV1()
	if err != nil {
		t.Fatalf("current initial operating configuration lifecycle v1: %v", err)
	}
	id := initialOperatingConfigurationV1ID
	sum := InitialOperatingConfigurationV1SHA256()

	if admitted, reason := ResolveInitialOperatingConfigurationLifecycleV1(lifecycle, id, sum, insideWindow()); !admitted || reason != "configuration_active" {
		t.Fatalf("active window denied: %v %s", admitted, reason)
	}
	if _, reason := ResolveInitialOperatingConfigurationLifecycleV1(lifecycle, id, sum, time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)); reason != "configuration_not_yet_valid" {
		t.Fatalf("early clock reason %q", reason)
	}
	if _, reason := ResolveInitialOperatingConfigurationLifecycleV1(lifecycle, id, sum, time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)); reason != "configuration_expired" {
		t.Fatalf("expiry is not exclusive at valid_until: %q", reason)
	}
	if _, reason := ResolveInitialOperatingConfigurationLifecycleV1(lifecycle, id, strings.Repeat("0", 64), insideWindow()); reason != "configuration_not_registered" {
		t.Fatalf("altered configuration hash reason %q", reason)
	}
	if _, reason := ResolveInitialOperatingConfigurationLifecycleV1(lifecycle, "other-configuration", sum, insideWindow()); reason != "configuration_not_registered" {
		t.Fatalf("unregistered configuration reason %q", reason)
	}

	revoked := lifecycle
	revoked.Active.Status = "revoked"
	revoked.Revoked = []InitialOperatingLifecycleRevocationV1{{ConfigurationID: id, ConfigurationSHA256: sum, RevokedAt: "2026-09-18T00:00:00Z", Reason: "owner_paused_adoption"}}
	if admitted, reason := ResolveInitialOperatingConfigurationLifecycleV1(revoked, id, sum, insideWindow()); admitted || reason != "configuration_revoked" {
		t.Fatalf("revocation did not stop admission: %v %s", admitted, reason)
	}
	// Half-undoing a revocation must fail closed rather than silently restoring
	// the configuration: the status word and the record have to agree.
	statusOnly := lifecycle
	statusOnly.Active.Status = "revoked"
	if _, reason := ResolveInitialOperatingConfigurationLifecycleV1(statusOnly, id, sum, insideWindow()); reason != "lifecycle_invalid" {
		t.Fatalf("revoked status without a record was accepted: %q", reason)
	}
	recordOnly := lifecycle
	recordOnly.Revoked = []InitialOperatingLifecycleRevocationV1{{ConfigurationID: id, ConfigurationSHA256: sum, RevokedAt: "2026-09-18T00:00:00Z", Reason: "owner_paused_adoption"}}
	if _, reason := ResolveInitialOperatingConfigurationLifecycleV1(recordOnly, id, sum, insideWindow()); reason != "lifecycle_invalid" {
		t.Fatalf("a revocation record with an active status was accepted: %q", reason)
	}
	retired := lifecycle
	retired.Active.Status = "retired"
	retired.Retired = []InitialOperatingLifecycleTombstoneV1{{ConfigurationID: id, ConfigurationSHA256: sum, RetiredAt: "2026-09-18T00:00:00Z", Reason: "superseded"}}
	if admitted, reason := ResolveInitialOperatingConfigurationLifecycleV1(retired, id, sum, insideWindow()); admitted || reason != "superseded_configuration" {
		t.Fatalf("tombstone did not stop admission: %v %s", admitted, reason)
	}
	// The registry has to say who turned this on.
	unauthorized := lifecycle
	unauthorized.Activation = InitialOperatingActivationV1{}
	if _, reason := ResolveInitialOperatingConfigurationLifecycleV1(unauthorized, id, sum, insideWindow()); reason != "lifecycle_invalid" {
		t.Fatalf("an unrecorded activation was accepted: %q", reason)
	}
	noReference := lifecycle
	noReference.Activation.AuthorizationReference = ""
	if _, reason := ResolveInitialOperatingConfigurationLifecycleV1(noReference, id, sum, insideWindow()); reason != "lifecycle_invalid" {
		t.Fatalf("an activation without an authorization reference was accepted: %q", reason)
	}
	incomplete := lifecycle
	incomplete.Revoked = []InitialOperatingLifecycleRevocationV1{{ConfigurationID: id, ConfigurationSHA256: sum}}
	if _, reason := ResolveInitialOperatingConfigurationLifecycleV1(incomplete, id, sum, insideWindow()); reason != "lifecycle_invalid" {
		t.Fatalf("incomplete revocation reason %q", reason)
	}
}

func TestInitialOperatingDecisionIsRefusedOutsideTheWindow(t *testing.T) {
	decision, err := EvaluateInitialOperatingConfigurationAt(eligibleFacts(), insideWindow())
	if err != nil {
		t.Fatalf("evaluate inside window: %v", err)
	}
	if !decision.LifecycleAdmitted || !decision.IssueAdmitted || decision.IssueReason != "admitted" {
		t.Fatalf("eligible issue denied inside the window: %+v", decision)
	}
	if decision.ConfigurationSHA256 != InitialOperatingConfigurationV1SHA256() {
		t.Fatalf("decision did not report the exact configuration hash")
	}
	expired, err := EvaluateInitialOperatingConfigurationAt(eligibleFacts(), time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("evaluate after window: %v", err)
	}
	if expired.LifecycleAdmitted || expired.IssueAdmitted || expired.LifecycleReason != "configuration_expired" || expired.IssueReason != "configuration_not_live" {
		t.Fatalf("expired configuration still admitted work: %+v", expired)
	}
}

func TestNativePublicationAcceptanceV1RecordsItsOwnLimits(t *testing.T) {
	acceptance, err := CurrentNativePublicationAcceptanceV1()
	if err != nil {
		t.Fatalf("current native publication acceptance v1: %v", err)
	}
	if acceptance.AuthorizesPermanentRoleOrModelSelection || acceptance.AuthorizesUnrestrictedAutomaticDispatch || acceptance.AuthorizesPublicExposure || acceptance.AuthorizesFurtherProviderRun {
		t.Fatalf("the accepted run claimed authority it does not carry")
	}
	if acceptance.ReviewerWasDistinctProvider || acceptance.RecoverySensitiveAuthorityDemonstrated || acceptance.ExactSingleModelProven {
		t.Fatalf("the accepted run overstated what it proved")
	}
	overstated := acceptance
	overstated.ReviewerWasDistinctProvider = true
	if err := ValidateNativePublicationAcceptanceV1(overstated); err == nil {
		t.Fatalf("an overstated independence claim was accepted")
	}
}
