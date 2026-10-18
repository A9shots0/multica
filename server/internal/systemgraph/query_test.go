package systemgraph

import (
	"slices"
	"testing"
	"time"
)

func defaultForTest(t *testing.T) Catalog {
	t.Helper()
	catalog, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func TestDefaultCatalogValidAndCoversActiveProducts(t *testing.T) {
	catalog := defaultForTest(t)
	for _, want := range []string{"crm", "linkhub", "canopy", "viral_queue", "phonefarm_app", "phonefleet", "photobot", "multica_controller"} {
		found := false
		for _, node := range catalog.Nodes {
			if node.ID == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("active product node %q missing", want)
		}
	}
	if len(catalog.Journeys) < 13 || len(catalog.Regressions) < 9 {
		t.Fatalf("graph too shallow: journeys=%d regressions=%d", len(catalog.Journeys), len(catalog.Regressions))
	}
}

func TestGraphCoversCanopyAccountsDesktopTelnyxAndCRMProduct(t *testing.T) {
	catalog := defaultForTest(t)
	for _, want := range []string{"j_crm_product", "j_canopy_accounts", "j_canopy_desktop", "j_telnyx_messaging"} {
		if _, err := catalog.JourneySubgraph(want); err != nil {
			t.Errorf("missing locked journey %s: %v", want, err)
		}
	}
}

func TestMaintainedMediaOutreachAndContentAuthoritiesAreNotReversed(t *testing.T) {
	graph := defaultForTest(t)
	wants := map[string][3]string{
		"e_drive_catalog":     {"drive", "canopy_media_pipeline", "canopy"},
		"e_outreach_provider": {"outreach", "x_provider", "canopy"},
		"e_content_vq":        {"viral_queue", "content_inspo", ""},
	}
	for edgeID, want := range wants {
		found := false
		for _, edge := range graph.Edges {
			if edge.ID != edgeID {
				continue
			}
			found = true
			if edge.From != want[0] || edge.To != want[1] || (want[2] != "" && edge.CredentialOwner != want[2]) {
				t.Errorf("%s authority reversed: %+v", edgeID, edge)
			}
		}
		if !found {
			t.Errorf("missing authority edge %s", edgeID)
		}
	}
	media, _ := graph.JourneySubgraph("j_media_availability")
	if slices.Contains(media.Journey.NodeIDs, "photobot") {
		t.Fatal("legacy Photobot is in active media journey")
	}
}

func TestJourneySubgraphCarriesAcceptanceAndRegressionProof(t *testing.T) {
	graph, err := defaultForTest(t).JourneySubgraph("j_linkhub_customer")
	if err != nil {
		t.Fatal(err)
	}
	if graph.Journey.AccountableCapability != "linkhub_customer_journey" {
		t.Fatalf("accountability = %q", graph.Journey.AccountableCapability)
	}
	if !slices.Contains(graph.Journey.AcceptanceProof, "public_customer_flow") || len(graph.Regressions) == 0 {
		t.Fatalf("journey proof incomplete: %+v", graph)
	}
}

func TestBlastRadiusFollowsDownstreamBoundariesAndExposesUnknownCoverage(t *testing.T) {
	radius, err := defaultForTest(t).BlastRadius([]string{"media_catalog"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"canopy_media_pipeline", "r2", "employee_experience"} {
		if !slices.Contains(radius.ImpactedNodeIDs, want) {
			t.Errorf("blast radius missing %q: %+v", want, radius)
		}
	}
	if !radius.UnknownCoverage {
		t.Fatal("partial graph coverage must fail open for inspection, not claim completeness")
	}
}

func TestBlastRadiusAcceptsEdgeSeedsAndPreservesBoundedJourney(t *testing.T) {
	radius, err := defaultForTest(t).BlastRadiusFrom(nil, []string{"e_identity_linkhub"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"crm_identity", "linkhub", "customer_link_journey"} {
		if !slices.Contains(radius.ImpactedNodeIDs, want) {
			t.Errorf("edge blast radius missing %s: %+v", want, radius)
		}
	}
	if slices.Contains(radius.ImpactedNodeIDs, "phonefleet") {
		t.Fatalf("edge query expanded into unrelated company graph: %+v", radius)
	}
}

func TestAuthorityFreshnessReleaseConflictAndRelatedOutcomeQueries(t *testing.T) {
	graph := defaultForTest(t)
	authority, err := graph.AuthorityFor("e_outreach_provider")
	if err != nil || authority.CredentialOwner != "canopy" {
		t.Fatalf("authority query = %+v err=%v", authority, err)
	}
	phoneAuthority, err := graph.AuthorityFor("phonefarm_app")
	if err != nil {
		t.Fatal(err)
	}
	if phoneAuthority.TechnicalBoundary == "" || phoneAuthority.AllowedExecutor != "dedicated_mac_studio_agent" || !slices.Contains(phoneAuthority.AllowedHosts, "mac_studio_only") || !slices.Contains(phoneAuthority.ProhibitedHosts, "windows") || phoneAuthority.ApprovalRequirement == "" {
		t.Fatalf("Phone Farm authority query lacks executor/approval/host boundary: %+v", phoneAuthority)
	}
	if len(graph.StaleEvidence(time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC))) == 0 {
		t.Fatal("missing evidence timestamps did not fail closed as unknown")
	}
	requirements, err := graph.RequirementsForRelease("j_canopy_desktop")
	if err != nil || len(requirements.AcceptanceProof) == 0 || len(requirements.Regressions) == 0 {
		t.Fatalf("release requirements incomplete: %+v err=%v", requirements, err)
	}
	if graph.ConflictingClaims() == nil {
		t.Fatal("conflict query must return an explicit empty array, not nil")
	}
	related, err := graph.RelatedOutcomes(nil, []string{"e_content_vq"})
	if err != nil || !slices.Contains(related.Outcomes, "content_supply") {
		t.Fatalf("related outcome query = %+v err=%v", related, err)
	}
}

func TestRouteIssueKeepsPhoneFarmOnMacAndRequiresAssurance(t *testing.T) {
	plan, err := defaultForTest(t).RouteIssue(RouteRequest{
		ObservedNodeIDs: []string{"phonefarm_app", "phonefleet"},
		RiskFlags:       []string{"production", "host_boundary"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Mode != "execute" || plan.AccountableCapability != "phonefarm_operation" {
		t.Fatalf("unexpected route: %+v", plan)
	}
	if !slices.Contains(plan.AllowedHosts, "mac_studio_only") || !slices.Contains(plan.ProhibitedHosts, "windows") {
		t.Fatalf("Phone Farm host boundary missing: %+v", plan)
	}
	if slices.Contains(plan.AllowedHosts, "railway") || slices.Contains(plan.AllowedHosts, "alienware_windows") {
		t.Fatalf("Phone Farm route leaked unrelated host authority: %+v", plan)
	}
	for _, want := range []string{"architecture_assurance", "release_runtime_assurance", "independent_outcome_assurance"} {
		if !slices.Contains(plan.RequiredFunctions, want) {
			t.Errorf("required function %q missing: %+v", want, plan.RequiredFunctions)
		}
	}
}

func TestRouteIssueUsesBoundedTriageForAmbiguousCrossSystemSymptom(t *testing.T) {
	plan, err := defaultForTest(t).RouteIssue(RouteRequest{ObservedNodeIDs: []string{"crm_identity"}})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Mode != "triage" || !slices.Contains(plan.RequiredFunctions, "bounded_first_divergence_triage") {
		t.Fatalf("ambiguous symptom was prematurely assigned: %+v", plan)
	}
}

func TestRouteIssueAddsSecurityReviewWithoutGrantingExecutionAuthority(t *testing.T) {
	plan, err := defaultForTest(t).RouteIssue(RouteRequest{
		ObservedNodeIDs: []string{"crm_identity"},
		RiskFlags:       []string{"security", "credential"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(plan.RequiredFunctions, "security_privacy_review") || !slices.Contains(plan.RequiredFunctions, "architecture_assurance") {
		t.Fatalf("security/architecture separation missing: %+v", plan)
	}
}

func TestChangedAuthorityNodeDeterministicallyRequiresAssurance(t *testing.T) {
	plan, err := defaultForTest(t).RouteIssue(RouteRequest{
		ObservedNodeIDs: []string{"crm_product"},
		ChangedNodeIDs:  []string{"crm_db"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.RiskClass != "consequential" || plan.Admission != "denied_until_typed_authority_receipt" || !slices.Contains(plan.RequiredFunctions, "architecture_assurance") {
		t.Fatalf("changed authority node was not fail-closed: %+v", plan)
	}
}

func TestAllNineteenLockedCasesProduceRequiredRouteFunctions(t *testing.T) {
	tests := []struct {
		id                      string
		nodes, flags, functions []string
	}{
		{"T01", []string{"crm_identity", "linkhub"}, nil, []string{"architecture_assurance", "data_sync_integrity", "product_assurance"}},
		{"T02", []string{"linkhub", "customer_link_journey"}, []string{"production"}, []string{"product_assurance", "release_runtime_assurance"}},
		{"T03", []string{"drive", "media_catalog"}, nil, []string{"data_sync_integrity", "product_assurance"}},
		{"T04", []string{"outreach", "x_provider"}, []string{"external_communication"}, []string{"data_sync_integrity", "security_privacy_review", "product_assurance"}},
		{"T05", []string{"phonefarm_app", "phonefleet"}, []string{"host_boundary"}, []string{"phone_fleet_reliability", "architecture_assurance"}},
		{"T06", []string{"railway", "regression_ledger"}, []string{"rollback"}, []string{"release_runtime_assurance"}},
		{"T07", []string{"viral_queue", "content_inspo"}, nil, []string{"data_sync_integrity", "product_assurance"}},
		{"T08", []string{"railway"}, []string{"rollback"}, []string{"release_runtime_assurance"}},
		{"T09", []string{"reporting", "employee_experience"}, nil, []string{"product_assurance"}},
		{"T10", []string{"multica_controller", "sms"}, []string{"external_communication"}, []string{"security_privacy_review", "release_runtime_assurance"}},
		{"T11", []string{"owner", "workflow_ledger"}, nil, []string{"independent_outcome_assurance"}},
		{"T12", []string{"crm_identity", "linkhub", "canopy_accounts"}, []string{"cross_app_edge"}, []string{"architecture_assurance", "bounded_first_divergence_triage"}},
		{"T13", []string{"crm_product", "employee_experience"}, nil, []string{"product_assurance"}},
		{"T14", []string{"multica_controller", "onedrive"}, []string{"destructive_action"}, []string{"security_privacy_review", "release_runtime_assurance"}},
		{"T15", []string{"crm_identity", "canopy_accounts"}, []string{"identity"}, []string{"architecture_assurance", "data_sync_integrity"}},
		{"T16", []string{"crm", "telnyx_messaging"}, []string{"security", "credential"}, []string{"architecture_assurance", "security_privacy_review"}},
		{"T17", []string{"canopy_accounts", "crm_identity"}, nil, []string{"data_sync_integrity", "security_privacy_review"}},
		{"T18", []string{"canopy_desktop", "canopy"}, []string{"deployment"}, []string{"product_assurance", "release_runtime_assurance"}},
		{"T19", []string{"crm", "telnyx_messaging"}, []string{"external_communication"}, []string{"data_sync_integrity", "security_privacy_review"}},
	}
	graph := defaultForTest(t)
	for _, test := range tests {
		plan, err := graph.RouteIssue(RouteRequest{ObservedNodeIDs: test.nodes, RiskFlags: test.flags})
		if err != nil {
			t.Errorf("%s route error: %v", test.id, err)
			continue
		}
		for _, function := range append(test.functions, "independent_outcome_assurance") {
			if !slices.Contains(plan.RequiredFunctions, function) {
				t.Errorf("%s missing %s: %+v", test.id, function, plan)
			}
		}
	}
}
