export type SystemGraphCoverage = "complete" | "partial" | "unknown" | "contradicted";

export interface SystemGraphEvidenceRef {
  kind: string;
  locator: string;
  verified_at?: string;
}

export interface SystemGraphNode {
  id: string;
  name: string;
  kind: string;
  authority?: string;
  host?: string;
  credential_owner?: string;
  active_state?: string;
  technical_boundary?: string;
  evidence_stage?: string;
  observed_at?: string;
  review_after?: string;
  known_unknowns: string[];
  allowed_state?: string;
  regression_refs: string[];
  coverage: SystemGraphCoverage;
  evidence: SystemGraphEvidenceRef[];
}

export interface SystemGraphEdge {
  id: string;
  from: string;
  to: string;
  kind: string;
  boundary?: string;
  contract?: string;
  execution_mode?: string;
  host?: string;
  authority?: string;
  credential_owner?: string;
  active_state?: string;
  technical_boundary?: string;
  evidence_stage?: string;
  observed_at?: string;
  review_after?: string;
  known_unknowns: string[];
  allowed_state?: string;
  fallback_recovery?: string;
  regression_refs: string[];
  failure_consequence?: string;
  coverage: SystemGraphCoverage;
  evidence: SystemGraphEvidenceRef[];
}

export interface SystemGraphJourney {
  id: string;
  name: string;
  outcome: string;
  node_ids: string[];
  edge_ids: string[];
  accountable_capability: string;
  acceptance_proof: string[];
  allowed_execution_hosts: string[];
  required_functions: string[];
  coverage: SystemGraphCoverage;
  high_risk?: boolean;
}

export interface SystemGraphRegression {
  id: string;
  name: string;
  journey_id: string;
  required_for: string[];
  proof: string;
}

export interface SystemGraphCatalog {
  version: string;
  generated_at: string;
  nodes: SystemGraphNode[];
  edges: SystemGraphEdge[];
  journeys: SystemGraphJourney[];
  regressions: SystemGraphRegression[];
}

export interface SystemGraphSubgraph {
  journey: SystemGraphJourney;
  nodes: SystemGraphNode[];
  edges: SystemGraphEdge[];
  regressions: SystemGraphRegression[];
}

export interface SystemGraphBlastRadius {
  seed_node_ids: string[];
  seed_edge_ids: string[];
  impacted_node_ids: string[];
  journey_ids: string[];
  outcomes: string[];
  unknown_coverage: boolean;
}

export type SystemGraphJourneyQuery = {
  query: "journey_subgraph";
  journey_id: string;
};

export type SystemGraphBlastRadiusQuery = {
  query: "blast_radius";
  seed_node_ids?: string[];
  seed_edge_ids?: string[];
};
