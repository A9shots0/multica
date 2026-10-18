package systemgraph

import (
	"fmt"
	"slices"
	"sort"
	"time"
)

type Subgraph struct {
	Journey     Journey      `json:"journey"`
	Nodes       []Node       `json:"nodes"`
	Edges       []Edge       `json:"edges"`
	Regressions []Regression `json:"regressions,omitempty"`
}

type BlastRadius struct {
	SeedNodeIDs     []string `json:"seed_node_ids"`
	SeedEdgeIDs     []string `json:"seed_edge_ids,omitempty"`
	ImpactedNodeIDs []string `json:"impacted_node_ids"`
	JourneyIDs      []string `json:"journey_ids"`
	Outcomes        []string `json:"outcomes"`
	UnknownCoverage bool     `json:"unknown_coverage"`
}

type FreshnessFinding struct {
	Kind        string `json:"kind"`
	ID          string `json:"id"`
	State       string `json:"state"`
	ObservedAt  string `json:"observed_at,omitempty"`
	ReviewAfter string `json:"review_after,omitempty"`
}

type AuthorityResult struct {
	Kind                string   `json:"kind"`
	ID                  string   `json:"id"`
	Authority           string   `json:"authority,omitempty"`
	CredentialOwner     string   `json:"credential_owner,omitempty"`
	AllowedState        string   `json:"allowed_state,omitempty"`
	KnownUnknowns       []string `json:"known_unknowns,omitempty"`
	TechnicalBoundary   string   `json:"technical_boundary,omitempty"`
	AllowedExecutor     string   `json:"allowed_executor"`
	ApprovalRequirement string   `json:"approval_requirement"`
	AllowedHosts        []string `json:"allowed_hosts"`
	ProhibitedHosts     []string `json:"prohibited_hosts,omitempty"`
}

type ReleaseRequirements struct {
	JourneyID       string        `json:"journey_id"`
	AcceptanceProof []string      `json:"acceptance_proof"`
	Regressions     []Regression  `json:"regressions"`
	Coverage        CoverageState `json:"coverage"`
}

type RouteRequest struct {
	ObservedNodeIDs []string `json:"observed_node_ids"`
	ChangedNodeIDs  []string `json:"changed_node_ids,omitempty"`
	RiskFlags       []string `json:"risk_flags,omitempty"`
	RequestedAction string   `json:"requested_action,omitempty"`
}

type AccountableActor struct {
	RoleID              string `json:"role_id"`
	ConfigurationID     string `json:"configuration_id"`
	AcceptanceAuthority string `json:"acceptance_authority"`
	Precedence          string `json:"precedence"`
	Fallback            string `json:"fallback"`
	RebindRule          string `json:"rebind_rule"`
}

type RoutePlan struct {
	Mode                  string   `json:"mode"`
	AccountableCapability string   `json:"accountable_capability,omitempty"`
	CandidateJourneyIDs   []string `json:"candidate_journey_ids"`
	RequiredFunctions     []string `json:"required_functions"`
	AllowedHosts          []string `json:"allowed_hosts"`
	ProhibitedHosts       []string `json:"prohibited_hosts,omitempty"`
	RequiredEvidence      []string `json:"required_evidence"`
	ScopedNodeIDs         []string `json:"scoped_node_ids"`
	CoverageGate          string   `json:"coverage_gate"`
	Reason                string   `json:"reason"`
	RiskClass             string   `json:"risk_class"`
	Admission             string   `json:"admission"`
	RequestedAction       string   `json:"requested_action,omitempty"`
}

func (c Catalog) JourneySubgraph(journeyID string) (Subgraph, error) {
	var journey *Journey
	for i := range c.Journeys {
		if c.Journeys[i].ID == journeyID {
			journey = &c.Journeys[i]
			break
		}
	}
	if journey == nil {
		return Subgraph{}, fmt.Errorf("unknown journey %q", journeyID)
	}
	nodeSet := make(map[string]struct{}, len(journey.NodeIDs))
	for _, id := range journey.NodeIDs {
		nodeSet[id] = struct{}{}
	}
	edgeSet := make(map[string]struct{}, len(journey.EdgeIDs))
	for _, id := range journey.EdgeIDs {
		edgeSet[id] = struct{}{}
	}
	result := Subgraph{Journey: *journey}
	for _, node := range c.Nodes {
		if _, ok := nodeSet[node.ID]; ok {
			result.Nodes = append(result.Nodes, node)
		}
	}
	for _, edge := range c.Edges {
		if _, ok := edgeSet[edge.ID]; ok {
			result.Edges = append(result.Edges, edge)
		}
	}
	for _, regression := range c.Regressions {
		if regression.JourneyID == journeyID {
			result.Regressions = append(result.Regressions, regression)
		}
	}
	return result, nil
}

func (c Catalog) BlastRadius(seedNodeIDs []string) (BlastRadius, error) {
	return c.BlastRadiusFrom(seedNodeIDs, nil)
}

func (c Catalog) BlastRadiusFrom(seedNodeIDs, seedEdgeIDs []string) (BlastRadius, error) {
	known := make(map[string]Node, len(c.Nodes))
	for _, node := range c.Nodes {
		known[node.ID] = node
	}
	seeds := uniqueSorted(seedNodeIDs)
	edgeSeeds := uniqueSorted(seedEdgeIDs)
	if len(seeds) == 0 && len(edgeSeeds) == 0 {
		return BlastRadius{}, fmt.Errorf("at least one seed node or edge is required")
	}
	visited := make(map[string]struct{}, len(seeds))
	seedSet := make(map[string]struct{}, len(seeds))
	for _, id := range seeds {
		if _, ok := known[id]; !ok {
			return BlastRadius{}, fmt.Errorf("unknown seed node %q", id)
		}
		visited[id] = struct{}{}
		seedSet[id] = struct{}{}
	}
	knownEdges := make(map[string]Edge, len(c.Edges))
	for _, edge := range c.Edges {
		knownEdges[edge.ID] = edge
	}
	edgeSeedSet := make(map[string]struct{}, len(edgeSeeds))
	for _, id := range edgeSeeds {
		edge, ok := knownEdges[id]
		if !ok {
			return BlastRadius{}, fmt.Errorf("unknown seed edge %q", id)
		}
		visited[edge.From] = struct{}{}
		visited[edge.To] = struct{}{}
		edgeSeedSet[id] = struct{}{}
	}
	// Expand through every business journey directly containing a changed node.
	// This includes its upstream authorities and downstream consumers without
	// recursively walking shared hubs into an unrelated company-wide graph.
	result := BlastRadius{SeedNodeIDs: seeds, SeedEdgeIDs: edgeSeeds}
	for _, journey := range c.Journeys {
		matched := false
		for _, id := range journey.NodeIDs {
			if _, ok := seedSet[id]; ok {
				matched = true
				break
			}
		}
		if !matched {
			for _, id := range journey.EdgeIDs {
				if _, ok := edgeSeedSet[id]; ok {
					matched = true
					break
				}
			}
		}
		if !matched {
			continue
		}
		result.JourneyIDs = append(result.JourneyIDs, journey.ID)
		result.Outcomes = append(result.Outcomes, journey.Outcome)
		if journey.Coverage != CoverageComplete {
			result.UnknownCoverage = true
		}
		for _, id := range journey.NodeIDs {
			visited[id] = struct{}{}
		}
		for _, edgeID := range journey.EdgeIDs {
			for _, edge := range c.Edges {
				if edge.ID == edgeID && edge.Coverage != CoverageComplete {
					result.UnknownCoverage = true
				}
			}
		}
	}
	for id := range visited {
		result.ImpactedNodeIDs = append(result.ImpactedNodeIDs, id)
		if known[id].Coverage != CoverageComplete {
			result.UnknownCoverage = true
		}
	}
	sort.Strings(result.ImpactedNodeIDs)
	result.JourneyIDs = uniqueSorted(result.JourneyIDs)
	result.Outcomes = uniqueSorted(result.Outcomes)
	return result, nil
}

func (c Catalog) StaleEvidence(now time.Time) []FreshnessFinding {
	var result []FreshnessFinding
	appendFinding := func(kind, id, observed, review string, coverage CoverageState) {
		state := "fresh"
		if observed == "" || review == "" {
			state = "unknown"
		} else if deadline, err := time.Parse(time.RFC3339, review); err != nil || !now.Before(deadline) {
			state = "stale"
		}
		if coverage == CoverageContradicted {
			state = "contradicted"
		}
		if state != "fresh" {
			result = append(result, FreshnessFinding{Kind: kind, ID: id, State: state, ObservedAt: observed, ReviewAfter: review})
		}
	}
	for _, node := range c.Nodes {
		appendFinding("node", node.ID, node.ObservedAt, node.ReviewAfter, node.Coverage)
	}
	for _, edge := range c.Edges {
		appendFinding("edge", edge.ID, edge.ObservedAt, edge.ReviewAfter, edge.Coverage)
	}
	return result
}

func (c Catalog) AuthorityFor(id string) (AuthorityResult, error) {
	for _, node := range c.Nodes {
		if node.ID == id {
			allowedHosts, prohibitedHosts := c.executionHostsForNode(id, node.Host)
			return AuthorityResult{Kind: "node", ID: id, Authority: node.Authority, CredentialOwner: node.CredentialOwner, AllowedState: node.AllowedState, KnownUnknowns: node.KnownUnknowns, TechnicalBoundary: node.TechnicalBoundary, AllowedExecutor: executorForHosts(allowedHosts), ApprovalRequirement: "controller_issued_typed_capability_receipt; durable_owner_authorization_for_consequential_action", AllowedHosts: allowedHosts, ProhibitedHosts: prohibitedHosts}, nil
		}
	}
	for _, edge := range c.Edges {
		if edge.ID == id {
			allowedHosts, prohibitedHosts := c.executionHostsForEdge(id, edge.Host)
			return AuthorityResult{Kind: "edge", ID: id, Authority: edge.Authority, CredentialOwner: edge.CredentialOwner, AllowedState: edge.AllowedState, KnownUnknowns: edge.KnownUnknowns, TechnicalBoundary: edge.TechnicalBoundary, AllowedExecutor: executorForHosts(allowedHosts), ApprovalRequirement: "controller_issued_typed_capability_receipt; durable_owner_authorization_for_consequential_action", AllowedHosts: allowedHosts, ProhibitedHosts: prohibitedHosts}, nil
		}
	}
	return AuthorityResult{}, fmt.Errorf("unknown node or edge %q", id)
}

func (c Catalog) executionHostsForNode(id, declaredHost string) ([]string, []string) {
	var hosts []string
	for _, journey := range c.Journeys {
		if slices.Contains(journey.NodeIDs, id) {
			hosts = append(hosts, journey.AllowedExecutionHosts...)
		}
	}
	return normalizeAuthorityHosts(hosts, declaredHost)
}

func (c Catalog) executionHostsForEdge(id, declaredHost string) ([]string, []string) {
	var hosts []string
	for _, journey := range c.Journeys {
		if slices.Contains(journey.EdgeIDs, id) {
			hosts = append(hosts, journey.AllowedExecutionHosts...)
		}
	}
	return normalizeAuthorityHosts(hosts, declaredHost)
}

func normalizeAuthorityHosts(hosts []string, declaredHost string) ([]string, []string) {
	// A host-locked technical node cannot inherit another journey's executor
	// merely because it consumes a shared contract in that journey.
	if declaredHost == "mac_studio_only" {
		return []string{"mac_studio_only"}, []string{"windows"}
	}
	if len(hosts) == 0 {
		hosts = []string{"alienware_windows"}
	}
	hosts = uniqueSorted(hosts)
	if len(hosts) == 1 && hosts[0] == "mac_studio_only" {
		return hosts, []string{"windows"}
	}
	return hosts, nil
}

func executorForHosts(hosts []string) string {
	if len(hosts) == 1 && hosts[0] == "mac_studio_only" {
		return "dedicated_mac_studio_agent"
	}
	return "controller_selected_qualified_configuration"
}

func (c Catalog) RequirementsForRelease(journeyID string) (ReleaseRequirements, error) {
	subgraph, err := c.JourneySubgraph(journeyID)
	if err != nil {
		return ReleaseRequirements{}, err
	}
	return ReleaseRequirements{JourneyID: journeyID, AcceptanceProof: subgraph.Journey.AcceptanceProof, Regressions: subgraph.Regressions, Coverage: subgraph.Journey.Coverage}, nil
}

func (c Catalog) ConflictingClaims() []AuthorityResult {
	result := make([]AuthorityResult, 0)
	for _, node := range c.Nodes {
		if node.Coverage == CoverageContradicted {
			item, _ := c.AuthorityFor(node.ID)
			result = append(result, item)
		}
	}
	for _, edge := range c.Edges {
		if edge.Coverage == CoverageContradicted {
			item, _ := c.AuthorityFor(edge.ID)
			result = append(result, item)
		}
	}
	return result
}

func (c Catalog) RelatedOutcomes(nodeIDs, edgeIDs []string) (BlastRadius, error) {
	return c.BlastRadiusFrom(nodeIDs, edgeIDs)
}

func (c Catalog) RouteIssue(request RouteRequest) (RoutePlan, error) {
	allIDs := uniqueSorted(append(append([]string{}, request.ObservedNodeIDs...), request.ChangedNodeIDs...))
	if len(allIDs) == 0 {
		return RoutePlan{}, fmt.Errorf("observed_node_ids is required")
	}
	radius, err := c.BlastRadius(allIDs)
	if err != nil {
		return RoutePlan{}, err
	}
	candidateSet := make(map[string]struct{})
	journeyScores := make(map[string]int)
	maxScore := 0
	for _, journey := range c.Journeys {
		score := 0
		for _, nodeID := range journey.NodeIDs {
			for _, observedID := range allIDs {
				if nodeID == observedID {
					score++
				}
			}
		}
		if score > 0 {
			journeyScores[journey.ID] = score
			if score > maxScore {
				maxScore = score
			}
		}
	}
	// Multiple journeys can share a boundary node (for example Phone Farm also
	// participates in identity/access). Prefer the journey explaining the most
	// observed nodes; preserve ties as genuine ambiguity for bounded triage.
	for journeyID, score := range journeyScores {
		if score == maxScore {
			candidateSet[journeyID] = struct{}{}
		}
	}
	plan := RoutePlan{
		CandidateJourneyIDs: uniqueSorted(mapKeys(candidateSet)),
		RequiredFunctions:   []string{"primary_executor", "independent_outcome_assurance"},
		RequiredEvidence:    []string{"first_divergence", "acceptance_result"},
		ScopedNodeIDs:       radius.ImpactedNodeIDs,
		CoverageGate:        "open",
		RiskClass:           "routine",
		Admission:           "route_only",
		RequestedAction:     request.RequestedAction,
	}
	capabilities := make(map[string]struct{})
	for _, journey := range c.Journeys {
		if _, ok := candidateSet[journey.ID]; ok {
			capabilities[journey.AccountableCapability] = struct{}{}
			plan.RequiredEvidence = append(plan.RequiredEvidence, journey.AcceptanceProof...)
			if journey.HighRisk {
				plan.RequiredFunctions = append(plan.RequiredFunctions, "release_runtime_assurance")
			}
			plan.RequiredFunctions = append(plan.RequiredFunctions, journey.RequiredFunctions...)
		}
	}
	for _, flag := range request.RiskFlags {
		switch flag {
		case "cross_app_edge", "truth_authority", "identity", "schema_contract", "credential", "data_lifecycle", "host_boundary":
			plan.RiskClass = "consequential"
			plan.RequiredFunctions = append(plan.RequiredFunctions, "architecture_assurance")
		case "security", "privacy", "destructive_action", "external_communication":
			plan.RiskClass = "consequential"
			plan.RequiredFunctions = append(plan.RequiredFunctions, "security_privacy_review")
		case "production", "deployment", "rollback", "migration", "restart":
			plan.RiskClass = "consequential"
			plan.RequiredFunctions = append(plan.RequiredFunctions, "release_runtime_assurance")
		}
	}
	for _, id := range request.ChangedNodeIDs {
		for _, node := range c.Nodes {
			if node.ID != id {
				continue
			}
			switch node.Kind {
			case "provider", "datastore", "deployment_platform", "host", "device_fleet", "contract":
				plan.RiskClass = "consequential"
				plan.RequiredFunctions = append(plan.RequiredFunctions, "architecture_assurance")
			}
			if node.Authority != "" || node.CredentialOwner != "" || node.Host == "mac_studio_only" {
				plan.RiskClass = "consequential"
			}
		}
	}
	for _, journey := range c.Journeys {
		if _, ok := candidateSet[journey.ID]; !ok {
			continue
		}
		for _, host := range journey.AllowedExecutionHosts {
			plan.AllowedHosts = append(plan.AllowedHosts, host)
			if host == "mac_studio_only" {
				plan.ProhibitedHosts = append(plan.ProhibitedHosts, "windows")
			}
		}
	}
	if len(capabilities) == 1 && len(candidateSet) == 1 {
		plan.Mode = "execute"
		plan.AccountableCapability = mapKeys(capabilities)[0]
		plan.Reason = "one journey and one accountable capability match current graph evidence"
	} else {
		plan.Mode = "triage"
		plan.RequiredFunctions = append(plan.RequiredFunctions, "bounded_first_divergence_triage")
		plan.Reason = "the observation crosses or ambiguously matches multiple business journeys"
	}
	if radius.UnknownCoverage {
		plan.CoverageGate = "expand_before_consequential_action"
		plan.RequiredEvidence = append(plan.RequiredEvidence, "fresh_graph_expansion")
	}
	if plan.RiskClass == "consequential" || plan.CoverageGate != "open" {
		plan.Admission = "denied_until_typed_authority_receipt"
	} else {
		plan.Admission = "eligible_for_typed_authority_receipt"
	}
	plan.RequiredFunctions = uniqueSorted(plan.RequiredFunctions)
	plan.RequiredEvidence = uniqueSorted(plan.RequiredEvidence)
	plan.AllowedHosts = uniqueSorted(plan.AllowedHosts)
	plan.ProhibitedHosts = uniqueSorted(plan.ProhibitedHosts)
	return plan, nil
}

func mapKeys[V any](values map[string]V) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}
