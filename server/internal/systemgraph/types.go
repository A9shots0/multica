package systemgraph

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

type CoverageState string

const (
	CoverageComplete     CoverageState = "complete"
	CoveragePartial      CoverageState = "partial"
	CoverageUnknown      CoverageState = "unknown"
	CoverageContradicted CoverageState = "contradicted"
)

type EvidenceRef struct {
	Kind       string `json:"kind"`
	Locator    string `json:"locator"`
	VerifiedAt string `json:"verified_at,omitempty"`
}

type Node struct {
	ID                string        `json:"id"`
	Name              string        `json:"name"`
	Kind              string        `json:"kind"`
	Authority         string        `json:"authority,omitempty"`
	Host              string        `json:"host,omitempty"`
	CredentialOwner   string        `json:"credential_owner,omitempty"`
	ActiveState       string        `json:"active_state,omitempty"`
	TechnicalBoundary string        `json:"technical_boundary,omitempty"`
	EvidenceStage     string        `json:"evidence_stage,omitempty"`
	ObservedAt        string        `json:"observed_at,omitempty"`
	ReviewAfter       string        `json:"review_after,omitempty"`
	KnownUnknowns     []string      `json:"known_unknowns,omitempty"`
	AllowedState      string        `json:"allowed_state,omitempty"`
	RegressionRefs    []string      `json:"regression_refs,omitempty"`
	Coverage          CoverageState `json:"coverage"`
	Evidence          []EvidenceRef `json:"evidence,omitempty"`
}

type Edge struct {
	ID                 string        `json:"id"`
	From               string        `json:"from"`
	To                 string        `json:"to"`
	Kind               string        `json:"kind"`
	Boundary           string        `json:"boundary,omitempty"`
	Contract           string        `json:"contract,omitempty"`
	ExecutionMode      string        `json:"execution_mode,omitempty"`
	Host               string        `json:"host,omitempty"`
	Authority          string        `json:"authority,omitempty"`
	CredentialOwner    string        `json:"credential_owner,omitempty"`
	ActiveState        string        `json:"active_state,omitempty"`
	TechnicalBoundary  string        `json:"technical_boundary,omitempty"`
	EvidenceStage      string        `json:"evidence_stage,omitempty"`
	ObservedAt         string        `json:"observed_at,omitempty"`
	ReviewAfter        string        `json:"review_after,omitempty"`
	KnownUnknowns      []string      `json:"known_unknowns,omitempty"`
	AllowedState       string        `json:"allowed_state,omitempty"`
	FallbackRecovery   string        `json:"fallback_recovery,omitempty"`
	RegressionRefs     []string      `json:"regression_refs,omitempty"`
	FailureConsequence string        `json:"failure_consequence,omitempty"`
	Coverage           CoverageState `json:"coverage"`
	Evidence           []EvidenceRef `json:"evidence,omitempty"`
}

type Journey struct {
	ID                    string        `json:"id"`
	Name                  string        `json:"name"`
	Outcome               string        `json:"outcome"`
	NodeIDs               []string      `json:"node_ids"`
	EdgeIDs               []string      `json:"edge_ids"`
	AccountableCapability string        `json:"accountable_capability"`
	AcceptanceProof       []string      `json:"acceptance_proof"`
	AllowedExecutionHosts []string      `json:"allowed_execution_hosts"`
	RequiredFunctions     []string      `json:"required_functions,omitempty"`
	Coverage              CoverageState `json:"coverage"`
	HighRisk              bool          `json:"high_risk,omitempty"`
}

type Regression struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	JourneyID   string   `json:"journey_id"`
	RequiredFor []string `json:"required_for"`
	Proof       string   `json:"proof"`
}

type Catalog struct {
	Version     string       `json:"version"`
	GeneratedAt string       `json:"generated_at"`
	Nodes       []Node       `json:"nodes"`
	Edges       []Edge       `json:"edges"`
	Journeys    []Journey    `json:"journeys"`
	Regressions []Regression `json:"regressions"`
}

func validCoverage(value CoverageState) bool {
	switch value {
	case CoverageComplete, CoveragePartial, CoverageUnknown, CoverageContradicted:
		return true
	default:
		return false
	}
}

func (c Catalog) Validate() error {
	if strings.TrimSpace(c.Version) == "" {
		return errors.New("system graph version is required")
	}
	nodes := make(map[string]Node, len(c.Nodes))
	for _, node := range c.Nodes {
		if node.ID == "" || node.Name == "" || node.Kind == "" {
			return fmt.Errorf("node id, name, and kind are required: %+v", node)
		}
		if !validCoverage(node.Coverage) {
			return fmt.Errorf("node %s has invalid coverage %q", node.ID, node.Coverage)
		}
		if _, exists := nodes[node.ID]; exists {
			return fmt.Errorf("duplicate node %s", node.ID)
		}
		nodes[node.ID] = node
	}
	edges := make(map[string]Edge, len(c.Edges))
	for _, edge := range c.Edges {
		if edge.ID == "" || edge.Kind == "" {
			return fmt.Errorf("edge id and kind are required: %+v", edge)
		}
		if _, ok := nodes[edge.From]; !ok {
			return fmt.Errorf("edge %s references missing from node %s", edge.ID, edge.From)
		}
		if _, ok := nodes[edge.To]; !ok {
			return fmt.Errorf("edge %s references missing to node %s", edge.ID, edge.To)
		}
		if !validCoverage(edge.Coverage) {
			return fmt.Errorf("edge %s has invalid coverage %q", edge.ID, edge.Coverage)
		}
		if _, exists := edges[edge.ID]; exists {
			return fmt.Errorf("duplicate edge %s", edge.ID)
		}
		edges[edge.ID] = edge
	}
	journeys := make(map[string]struct{}, len(c.Journeys))
	for _, journey := range c.Journeys {
		if journey.ID == "" || journey.Outcome == "" || journey.AccountableCapability == "" || len(journey.AllowedExecutionHosts) == 0 {
			return fmt.Errorf("journey id, outcome, and accountable capability are required: %+v", journey)
		}
		for _, host := range journey.AllowedExecutionHosts {
			if host != "alienware_windows" && host != "mac_studio_only" {
				return fmt.Errorf("journey %s has invalid execution host %s", journey.ID, host)
			}
		}
		if !validCoverage(journey.Coverage) {
			return fmt.Errorf("journey %s has invalid coverage %q", journey.ID, journey.Coverage)
		}
		if _, exists := journeys[journey.ID]; exists {
			return fmt.Errorf("duplicate journey %s", journey.ID)
		}
		journeys[journey.ID] = struct{}{}
		for _, id := range journey.NodeIDs {
			if _, ok := nodes[id]; !ok {
				return fmt.Errorf("journey %s references missing node %s", journey.ID, id)
			}
		}
		for _, id := range journey.EdgeIDs {
			if _, ok := edges[id]; !ok {
				return fmt.Errorf("journey %s references missing edge %s", journey.ID, id)
			}
		}
	}
	for _, regression := range c.Regressions {
		if regression.ID == "" || regression.JourneyID == "" || regression.Proof == "" {
			return fmt.Errorf("regression id, journey, and proof are required: %+v", regression)
		}
		if _, ok := journeys[regression.JourneyID]; !ok {
			return fmt.Errorf("regression %s references missing journey %s", regression.ID, regression.JourneyID)
		}
	}
	return nil
}

func uniqueSorted(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value != "" {
			seen[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for value := range seen {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
