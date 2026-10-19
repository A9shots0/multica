export interface CompanyEvidenceLane {
  lane: string;
  source_system: string;
  event_count: number;
  lifecycle_state: string;
  severity: string;
  latest_summary: string;
  affected_refs: string[];
  evidence_refs: string[];
  latest_observed_at: string;
}

export interface CompanyIncidentProjection {
  id: string;
  correlation_key: string;
  generation: number;
  service_ref: string;
  environment_ref: string;
  title: string;
  lifecycle_state: string;
  severity: string;
  action_required: boolean;
  next_step: string;
  verification_state: string;
  project_id?: string;
  linked_issue_id?: string;
  opened_at: string;
  last_observed_at: string;
  resolved_at?: string;
  revision: number;
  affected_refs: string[];
  evidence_lanes: CompanyEvidenceLane[];
  system_node_ids: string[];
  journey_ids: string[];
}

export interface CompanyServiceProjection {
  service_key: string;
  display_name: string;
  area_key: string;
  environment: string;
  source_system: string;
  source_native_id: string;
  project_id?: string;
  portfolio_binding?: CompanyPortfolioBinding;
  authority: string;
  freshness_state: string;
  privacy_class: string;
  observed_at: string;
  expires_at?: string;
  metadata: Record<string, unknown>;
  system_node_ids: string[];
  journey_ids: string[];
  revision: number;
}

export interface CompanyPortfolioBinding {
  canonical_project_id?: string;
  company_membership: "company" | "personal" | "unknown";
  portfolio_classification:
    | "business_application"
    | "company_infrastructure"
    | "personal"
    | "experimental"
    | "archived"
    | "unknown";
  verification_state: "verified" | "unverified";
  provenance?: CompanyPortfolioProvenance;
}

export interface CompanyPortfolioProvenance {
  source_system: "silver-mediums-knowledge";
  source_repository: string;
  source_reference: string;
  source_revision: string;
  source_sha256?: string;
  source_observed_at: string;
  evidence_stage: string;
  program_id?: string;
  projection_system: "silver-mediums-brain";
  projection_revision: string;
  projected_at: string;
}

export interface CompanyOperatingPicture {
  contract_version: number;
  generated_at: string;
  services: CompanyServiceProjection[];
  incidents: CompanyIncidentProjection[];
  current_effort: CompanyCurrentEffortProjection[];
  work_topology: CompanyWorkTopologyProjection[];
  work_relationships: CompanyWorkRelationshipProjection[];
}

export interface CompanyCurrentEffortProjection {
  id: string;
  source_system: string;
  provider_report_only: boolean;
  work_id: string;
  run_id?: string;
  issue_id?: string;
  project_id?: string;
  title: string;
  actor_id?: string;
  actor_name?: string;
  work_source: string;
  state: "queued" | "active" | "waiting" | "reported_done_unaccepted" | "stopped" | "stale";
  reported_state: string;
  native_outcome_state?: string;
  observed_at: string;
  freshness_state: "current" | "stale";
  system_node_ids: string[];
  journey_ids: string[];
}

export interface CompanyWorkTopologyProjection {
  issue_id: string;
  graph_version: string;
  system_node_ids: string[];
  journey_ids: string[];
  authority: "owner" | "cited";
  evidence_refs: string[];
  observed_at: string;
  expires_at?: string;
  freshness_state: "current" | "stale";
  revision: number;
}

export interface PutCompanyWorkTopologyInput {
  graph_version: string;
  system_node_ids: string[];
  journey_ids: string[];
  authority: "owner" | "cited";
  evidence_refs: string[];
  observed_at: string;
  expires_at?: string;
}

export interface CompanyWorkRelationshipProjection {
  id: string;
  source_issue_id: string;
  affected_issue_id: string;
  relationship_kind: string;
  graph_version: string;
  committed_at: string;
}
