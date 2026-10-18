import { describe, expect, it } from "vitest";
import { parseWithFallback } from "./schema";
import {
  CompanyOperatingPictureSchema,
  EMPTY_COMPANY_OPERATING_PICTURE,
  EMPTY_SYSTEM_GRAPH_CATALOG,
  SystemGraphBlastRadiusSchema,
  SystemGraphCatalogSchema,
  SystemGraphSubgraphSchema,
} from "./schemas";

describe("CompanyOperatingPictureSchema", () => {
  it("keeps typed service and incident evidence", () => {
    const parsed = CompanyOperatingPictureSchema.parse({
      contract_version: 1,
      generated_at: "2026-10-17T20:00:00Z",
      services: [{
        service_key: "phonefarm",
        display_name: "PhoneFarm",
        area_key: "operations",
        environment: "production",
        source_system: "watcher",
        source_native_id: "phonefarm-prod",
        authority: "observed",
        freshness_state: "fresh",
        privacy_class: "internal",
        observed_at: "2026-10-17T20:00:00Z",
        metadata: {},
        revision: 2,
      }],
      incidents: [],
      current_effort: [{
        id: "receipt-1",
        source_system: "codex",
        provider_report_only: true,
        work_id: "hq-foundation",
        run_id: "run-1",
        issue_id: "issue-1",
        project_id: "project-1",
        title: "Build company operating picture",
        work_source: "external_receipt",
        state: "reported_done_unaccepted",
        reported_state: "complete",
        native_outcome_state: "open",
        observed_at: "2026-10-17T19:58:00Z",
        freshness_state: "current",
      }],
      work_topology: [{
        issue_id: "issue-1",
        graph_version: "silver-mediums.system-graph.v1",
        system_node_ids: ["phonefarm_app"],
        journey_ids: ["j_phonefarm_operation"],
        authority: "cited",
        evidence_refs: ["receipt:phonefarm-1"],
        observed_at: "2026-10-17T19:57:00Z",
        freshness_state: "current",
        revision: 1,
      }],
    });
    expect(parsed.services[0]?.display_name).toBe("PhoneFarm");
    expect(parsed.services[0]?.system_node_ids).toEqual([]);
    expect(parsed.services[0]?.journey_ids).toEqual([]);
    expect(parsed.current_effort[0]?.state).toBe("reported_done_unaccepted");
    expect(parsed.work_topology[0]?.system_node_ids).toEqual(["phonefarm_app"]);
    expect(parsed.work_relationships).toEqual([]);
  });

  it("degrades a malformed response to honest empty evidence", () => {
    expect(parseWithFallback(
      { services: "healthy", incidents: [] },
      CompanyOperatingPictureSchema,
      EMPTY_COMPANY_OPERATING_PICTURE,
      { endpoint: "GET /api/workspaces/:id/company-operating-picture", redactReceived: true },
    )).toEqual(EMPTY_COMPANY_OPERATING_PICTURE);
  });
});

describe("SystemGraphCatalogSchema", () => {
  it("keeps the safe graph contract and defaults optional collections", () => {
    const parsed = SystemGraphCatalogSchema.parse({
      version: "v1",
      generated_at: "2026-10-17T12:00:00Z",
      nodes: [{ id: "phonefarm_app", name: "Phone Farm", kind: "application", coverage: "partial" }],
      edges: [],
      journeys: [],
      regressions: [],
    });
    expect(parsed.nodes[0]?.known_unknowns).toEqual([]);
    expect(parsed.nodes[0]?.evidence).toEqual([]);
  });

  it("provides a fail-closed empty fallback shape", () => {
    expect(EMPTY_SYSTEM_GRAPH_CATALOG).toEqual({
      version: "", generated_at: "", nodes: [], edges: [], journeys: [], regressions: [],
    });
  });
});

describe("bounded system graph query schemas", () => {
  it("keeps a journey subgraph as typed operational context", () => {
    const parsed = SystemGraphSubgraphSchema.parse({
      journey: {
        id: "j_phonefarm",
        name: "Phone Farm availability",
        outcome: "Employees can use Phone Farm",
        node_ids: ["phonefarm_app"],
        edge_ids: [],
        accountable_capability: "platform_reliability",
        acceptance_proof: ["employee_workflow"],
        allowed_execution_hosts: ["mac_studio_only"],
        required_functions: ["release_runtime_assurance"],
        coverage: "partial",
      },
      nodes: [{
        id: "phonefarm_app",
        name: "Phone Farm",
        kind: "application",
        coverage: "partial",
      }],
      edges: [],
    });

    expect(parsed.journey.id).toBe("j_phonefarm");
    expect(parsed.regressions).toEqual([]);
    expect(parsed.nodes[0]?.known_unknowns).toEqual([]);
  });

  it("preserves unknown coverage in a bounded blast radius", () => {
    const parsed = SystemGraphBlastRadiusSchema.parse({
      seed_node_ids: ["phonefarm_app"],
      impacted_node_ids: ["phonefarm_app", "crm"],
      journey_ids: ["j_phonefarm"],
      outcomes: ["Employees can use Phone Farm"],
      unknown_coverage: true,
    });

    expect(parsed.seed_edge_ids).toEqual([]);
    expect(parsed.unknown_coverage).toBe(true);
  });
});
