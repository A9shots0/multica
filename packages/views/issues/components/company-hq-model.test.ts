// @vitest-environment node
import { describe, expect, it } from "vitest";
import type {
  Agent,
  AgentTask,
  CompanyIncidentProjection,
  CompanyServiceProjection,
  Issue,
  Project,
  SystemGraphCatalog,
  WorkspaceWorkingAgent,
} from "@multica/core/types";
import { buildHqMap, outcomeMatches, visibleHqOutcomes } from "./company-hq-model";

function issue(overrides: Partial<Issue>): Issue {
  return {
    id: "root", workspace_id: "ws", number: 1, identifier: "MUL-1", title: "Modernize PhoneFarm",
    description: null, status: "in_progress", priority: "high", assignee_type: null, assignee_id: null,
    creator_type: "member", creator_id: "owner", parent_issue_id: null, project_id: "phonefarm", position: 0,
    stage: null, start_date: null, due_date: null, metadata: {}, properties: {}, created_at: "2026-10-17T12:00:00Z",
    updated_at: "2026-10-17T12:00:00Z", ...overrides,
  };
}

function project(overrides: Partial<Project>): Project {
  return {
    id: "phonefarm", workspace_id: "ws", title: "PhoneFarm", description: "Reliable phone operation", icon: null,
    status: "in_progress", priority: "high", lead_type: null, lead_id: null, start_date: null, due_date: null,
    created_at: "2026-10-01T00:00:00Z", updated_at: "2026-10-17T12:00:00Z", issue_count: 2, done_count: 0,
    resource_count: 0, ...overrides,
  };
}

const graph: SystemGraphCatalog = {
  version: "v1", generated_at: "2026-10-17T12:00:00Z",
  nodes: [
    { id: "phonefarm_app", name: "Phone Farm application", kind: "application", coverage: "partial", known_unknowns: [], regression_refs: [], evidence: [] },
    { id: "phonefleet", name: "Phone fleet", kind: "device_fleet", coverage: "partial", known_unknowns: [], regression_refs: [], evidence: [] },
  ],
  edges: [
    { id: "commands", from: "phonefarm_app", to: "phonefleet", kind: "commands", coverage: "partial", known_unknowns: [], regression_refs: [], evidence: [] },
  ],
  journeys: [], regressions: [],
};

function service(overrides: Partial<CompanyServiceProjection>): CompanyServiceProjection {
  return {
    service_key: "phonefarm_app", display_name: "Phone Farm", area_key: "phonefarm_app", environment: "production",
    source_system: "watcher", source_native_id: "farm", project_id: "phonefarm", authority: "source",
    freshness_state: "current", privacy_class: "internal", observed_at: "2026-10-17T12:00:00Z", metadata: {}, revision: 1,
    ...overrides,
  };
}

function incident(overrides: Partial<CompanyIncidentProjection>): CompanyIncidentProjection {
  return {
    id: "incident", correlation_key: "farm-down", generation: 1, service_ref: "phonefarm_app",
    environment_ref: "production", title: "Posting is down", lifecycle_state: "open", severity: "high",
    action_required: true, next_step: "Restore posting", verification_state: "unverified", project_id: "phonefarm",
    opened_at: "2026-10-17T12:00:00Z", last_observed_at: "2026-10-17T12:00:00Z", revision: 1,
    affected_refs: ["system:phonefarm_app", "system:phonefleet"], evidence_lanes: [], ...overrides,
  };
}

describe("buildHqMap", () => {
  it("uses projects as outcomes and keeps their work, workers, systems, and typed graph links together", () => {
    const root = issue({});
    const child = issue({ id: "child", number: 2, identifier: "MUL-2", title: "Repair releases", parent_issue_id: root.id });
    const workers = [{ id: "agent", name: "Mika worker", avatar_url: null, running_task_count: 1, issue_ids: [child.id] }] as WorkspaceWorkingAgent[];
    const map = buildHqMap({
      issues: [root, child], projects: [project({})], workingAgents: workers,
      services: [service({})], incidents: [incident({})], systemGraph: graph,
    });

    expect(map.outcomes).toHaveLength(1);
    expect(map.outcomes[0]?.id).toBe("phonefarm");
    expect(map.outcomes[0]?.work.map((entry) => entry.issue.id)).toEqual(["root", "child"]);
    expect(map.outcomes[0]?.workers.map((worker) => worker.id)).toEqual(["agent"]);
    expect(map.outcomes[0]?.systems.map((system) => system.id)).toEqual(["phonefarm_app", "phonefleet"]);
    expect(map.outcomes[0]?.systemEdges.map((edge) => edge.id)).toEqual(["commands"]);
  });

  it("groups unprojected work once instead of promoting every root issue to a company outcome", () => {
    const map = buildHqMap({
      issues: [
        issue({ id: "one", identifier: "MUL-3", project_id: null, title: "First task" }),
        issue({ id: "two", identifier: "MUL-4", project_id: null, title: "Second task" }),
      ],
      projects: [], workingAgents: [], services: [], incidents: [], systemGraph: { ...graph, nodes: [], edges: [] },
    });

    expect(map.outcomes).toHaveLength(1);
    expect(map.outcomes[0]?.id).toBe("company-work");
    expect(map.outcomes[0]?.work).toHaveLength(2);
  });

  it("does not call a stale project moving without current work evidence", () => {
    const map = buildHqMap({
      issues: [issue({ status: "todo" })],
      projects: [project({ status: "in_progress" })],
      workingAgents: [], services: [], incidents: [], systemGraph: { ...graph, nodes: [], edges: [] },
    });

    expect(map.outcomes[0]?.lane).toBe("waiting");
  });

  it("uses live task evidence for movement and preserves assignment and failure evidence", () => {
    const assignedIssue = issue({ assignee_type: "agent", assignee_id: "specialist" });
    const specialist = { id: "specialist", name: "Phone Farm Application Specialist" } as Agent;
    const failedTask = {
      id: "failed-task",
      agent_id: "specialist",
      runtime_id: "runtime",
      issue_id: assignedIssue.id,
      status: "failed",
      priority: 0,
      dispatched_at: "2026-10-17T12:00:00Z",
      started_at: "2026-10-17T12:00:01Z",
      completed_at: "2026-10-17T12:00:02Z",
      result: null,
      error: "Expired in queue",
      created_at: "2026-10-17T12:00:00Z",
    } as AgentTask;

    const failedMap = buildHqMap({
      issues: [assignedIssue], projects: [project({})], agents: [specialist], tasks: [failedTask],
      workingAgents: [], services: [], incidents: [], systemGraph: { ...graph, nodes: [], edges: [] },
    });
    expect(failedMap.outcomes[0]?.lane).toBe("waiting");
    expect(failedMap.outcomes[0]?.assignees).toEqual(["Phone Farm Application Specialist"]);
    expect(failedMap.outcomes[0]?.work[0]?.latestTask?.error).toBe("Expired in queue");

    const movingMap = buildHqMap({
      issues: [assignedIssue], projects: [project({})], agents: [specialist], tasks: [{ ...failedTask, status: "running", error: null }],
      workingAgents: [], services: [], incidents: [], systemGraph: { ...graph, nodes: [], edges: [] },
    });
    expect(movingMap.outcomes[0]?.lane).toBe("moving");
  });

  it("represents one incident once while connecting every explicitly affected outcome", () => {
    const crm = project({ id: "crm", title: "CRM", priority: "medium" });
    const shared = incident({
      project_id: undefined,
      affected_refs: ["project:phonefarm", "project:crm", "system:phonefarm_app"],
    });
    const map = buildHqMap({
      issues: [issue({}), issue({ id: "crm-work", project_id: "crm", title: "Repair CRM" })],
      projects: [project({}), crm], workingAgents: [], services: [service({})], incidents: [shared], systemGraph: graph,
    });

    expect(map.sharedIncidents).toHaveLength(1);
    expect(new Set(map.sharedIncidents[0]?.outcomeIds)).toEqual(new Set(["phonefarm", "crm"]));
    expect(map.outcomes.filter((outcome) => outcome.incidents.some((value) => value.id === shared.id))).toHaveLength(2);
  });
});

describe("HQ scale and search", () => {
  it("caps the company view and keeps search as emphasis metadata", () => {
    const projects = Array.from({ length: 12 }, (_, index) => project({ id: `p${index}`, title: `Outcome ${index}` }));
    const map = buildHqMap({ issues: [], projects, workingAgents: [], services: [], incidents: [], systemGraph: graph });
    expect(visibleHqOutcomes(map.outcomes, 8)).toMatchObject({ overflow: 4 });
    expect(outcomeMatches(map.outcomes[0]!, "Outcome")).toBe(true);
    expect(outcomeMatches(map.outcomes[0]!, "missing phrase")).toBe(false);
  });

  it("keeps project outcomes and unprojected shared work in distinct regions", () => {
    const map = buildHqMap({
      issues: [issue({}), issue({ id: "shared", identifier: "MUL-9", project_id: null })],
      projects: [project({})], workingAgents: [], services: [], incidents: [], systemGraph: { ...graph, nodes: [], edges: [] },
    });

    expect(map.areas.map((area) => [area.id, area.name])).toEqual([
      ["company-outcomes", "Company outcomes"],
      ["shared-work", "Shared work"],
    ]);
  });
});
