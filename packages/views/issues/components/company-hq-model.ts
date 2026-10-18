import type {
  CompanyIncidentProjection,
  CompanyServiceProjection,
  Agent,
  AgentTask,
  Issue,
  MemberWithUser,
  Project,
  SystemGraphCatalog,
  SystemGraphEdge,
  SystemGraphNode,
  WorkspaceWorkingAgent,
} from "@multica/core/types";
import { workforceItem, type WorkforceItem, type WorkforceState } from "../../inbox/components/workforce-overview-model";

export type HqLane = "needs_you" | "moving" | "waiting";
export type HqUrgency = "critical" | "high" | "normal" | "background";

export interface HqSystem {
  id: string;
  name: string;
  areaKey: string;
  kind: string;
  coverage: "complete" | "partial" | "unknown" | "contradicted";
  freshness: string;
  node: SystemGraphNode | null;
}

export interface HqWork {
  issue: Issue;
  item: WorkforceItem;
  workers: WorkspaceWorkingAgent[];
  assigneeName: string | null;
  latestTask: AgentTask | null;
}

export interface HqOutcome {
  id: string;
  project: Project | null;
  title: string;
  description: string | null;
  work: HqWork[];
  workers: WorkspaceWorkingAgent[];
  assignees: string[];
  services: CompanyServiceProjection[];
  systems: HqSystem[];
  incidents: CompanyIncidentProjection[];
  systemEdges: SystemGraphEdge[];
  lane: HqLane;
  urgency: HqUrgency;
  lastUpdatedAt: string;
  isStale: boolean;
}

export interface HqArea {
  id: string;
  name: string;
  outcomeIds: string[];
}

export interface HqSharedIncident {
  incident: CompanyIncidentProjection;
  outcomeIds: string[];
  systemIds: string[];
}

export interface HqMap {
  outcomes: HqOutcome[];
  areas: HqArea[];
  sharedIncidents: HqSharedIncident[];
}

const terminalIssue = new Set(["done", "cancelled"]);
const terminalProject = new Set(["completed", "cancelled"]);
const COMPANY_WORK_ID = "company-work";

export function laneForState(state: WorkforceState): HqLane {
  if (state === "needs_you" || state === "stopped") return "needs_you";
  if (state === "working") return "moving";
  return "waiting";
}

const URGENCY_ORDER: Record<HqUrgency, number> = {
  critical: 0,
  high: 1,
  normal: 2,
  background: 3,
};

const LANE_ORDER: Record<HqLane, number> = {
  needs_you: 0,
  moving: 1,
  waiting: 2,
};

function timestampFor(issue: Issue): string {
  return issue.current_picture?.generated_at || issue.last_activity_at || issue.updated_at;
}

function validTime(value: string): number {
  const parsed = Date.parse(value);
  return Number.isFinite(parsed) ? parsed : 0;
}

function newestTimestamp(values: string[], fallback: string): string {
  return values.reduce((latest, value) => validTime(value) > validTime(latest) ? value : latest, fallback);
}

function laneForWork(work: HqWork[]): HqLane {
  if (work.some((entry) => entry.item.state === "needs_you")) return "needs_you";
  // "Working now" needs live execution evidence. A days-old provider report
  // that once said "working" is history, not proof that an agent is active.
  if (work.some((entry) =>
    entry.workers.length > 0 ||
    entry.latestTask?.status === "queued" ||
    entry.latestTask?.status === "dispatched" ||
    entry.latestTask?.status === "waiting_local_directory" ||
    entry.latestTask?.status === "running"
  )) return "moving";
  return "waiting";
}

function latestTaskForIssue(tasks: AgentTask[], issueId: string): AgentTask | null {
  return tasks
    .filter((task) => task.issue_id === issueId)
    .sort((left, right) => validTime(right.created_at) - validTime(left.created_at))[0] ?? null;
}

function assigneeNameForIssue(
  issue: Issue,
  agents: Map<string, Agent>,
  members: Map<string, MemberWithUser>,
): string | null {
  if (!issue.assignee_id) return null;
  if (issue.assignee_type === "agent") return agents.get(issue.assignee_id)?.name ?? "Assigned agent";
  if (issue.assignee_type === "member") return members.get(issue.assignee_id)?.name ?? "Assigned teammate";
  if (issue.assignee_type === "squad") return "Assigned squad";
  return null;
}

function urgencyFor(
  project: Project | null,
  work: HqWork[],
  incidents: CompanyIncidentProjection[],
): HqUrgency {
  const severities = incidents
    .filter((incident) => incident.lifecycle_state !== "resolved")
    .map((incident) => incident.severity.toLowerCase());
  if (
    project?.priority === "urgent" ||
    work.some((entry) => entry.issue.priority === "urgent") ||
    severities.includes("critical")
  ) return "critical";
  if (
    project?.priority === "high" ||
    work.some((entry) => entry.issue.priority === "high") ||
    severities.includes("high")
  ) return "high";
  if (
    project?.priority === "low" ||
    project?.priority === "none" ||
    (work.length > 0 && work.every((entry) => entry.issue.priority === "low" || entry.issue.priority === "none"))
  ) return "background";
  return "normal";
}

function referenceValue(ref: string, kinds: string[]): string | null {
  const separator = ref.indexOf(":");
  if (separator < 1 || !kinds.includes(ref.slice(0, separator))) return null;
  return ref.slice(separator + 1) || null;
}

function explicitGraphNodeForService(
  service: CompanyServiceProjection,
  nodes: Map<string, SystemGraphNode>,
): SystemGraphNode | null {
  const metadataNode = service.metadata.system_graph_node_id;
  if (typeof metadataNode === "string" && nodes.has(metadataNode)) return nodes.get(metadataNode) ?? null;
  if (nodes.has(service.service_key)) return nodes.get(service.service_key) ?? null;
  if (nodes.has(service.area_key)) return nodes.get(service.area_key) ?? null;
  return null;
}

function systemFromService(
  service: CompanyServiceProjection,
  nodes: Map<string, SystemGraphNode>,
): HqSystem {
  const node = explicitGraphNodeForService(service, nodes);
  return {
    id: node?.id ?? `service:${service.service_key}`,
    name: node?.name ?? service.display_name,
    areaKey: service.area_key,
    kind: node?.kind ?? "service",
    coverage: node?.coverage ?? "unknown",
    freshness: service.freshness_state,
    node,
  };
}

function systemFromNode(node: SystemGraphNode): HqSystem {
  return {
    id: node.id,
    name: node.name,
    areaKey: node.id,
    kind: node.kind,
    coverage: node.coverage,
    freshness: node.evidence_stage || "unknown",
    node,
  };
}

function incidentOutcomeIds(input: {
  incident: CompanyIncidentProjection;
  issueOutcomeIds: Map<string, string>;
  serviceOutcomeIds: Map<string, string>;
  knownOutcomeIds: Set<string>;
}): string[] {
  const result = new Set<string>();
  const { incident } = input;
  if (incident.project_id && input.knownOutcomeIds.has(incident.project_id)) result.add(incident.project_id);
  if (incident.linked_issue_id) {
    const outcomeId = input.issueOutcomeIds.get(incident.linked_issue_id);
    if (outcomeId) result.add(outcomeId);
  }
  const serviceOutcome = input.serviceOutcomeIds.get(incident.service_ref);
  if (serviceOutcome) result.add(serviceOutcome);
  for (const ref of incident.affected_refs) {
    const projectId = referenceValue(ref, ["project", "outcome"]);
    if (projectId && input.knownOutcomeIds.has(projectId)) result.add(projectId);
    const issueId = referenceValue(ref, ["issue", "work"]);
    const issueOutcome = issueId ? input.issueOutcomeIds.get(issueId) : null;
    if (issueOutcome) result.add(issueOutcome);
    const serviceKey = referenceValue(ref, ["service"]);
    const affectedOutcome = serviceKey ? input.serviceOutcomeIds.get(serviceKey) : null;
    if (affectedOutcome) result.add(affectedOutcome);
  }
  return [...result];
}

function outcomeSystems(input: {
  services: CompanyServiceProjection[];
  incidents: CompanyIncidentProjection[];
  nodes: Map<string, SystemGraphNode>;
  servicesByKey: Map<string, CompanyServiceProjection>;
}): HqSystem[] {
  const systems = new Map<string, HqSystem>();
  for (const service of input.services) {
    const system = systemFromService(service, input.nodes);
    systems.set(system.id, system);
  }
  for (const incident of input.incidents) {
    const directService = input.servicesByKey.get(incident.service_ref);
    if (directService) {
      const system = systemFromService(directService, input.nodes);
      systems.set(system.id, system);
    }
    for (const ref of incident.affected_refs) {
      const nodeId = referenceValue(ref, ["system", "node"]);
      const node = nodeId ? input.nodes.get(nodeId) : null;
      if (node) systems.set(node.id, systemFromNode(node));
      const serviceKey = referenceValue(ref, ["service"]);
      const service = serviceKey ? input.servicesByKey.get(serviceKey) : null;
      if (service) {
        const system = systemFromService(service, input.nodes);
        systems.set(system.id, system);
      }
    }
  }
  return [...systems.values()].sort((left, right) => left.name.localeCompare(right.name));
}

export function buildHqMap(input: {
  issues: Issue[];
  projects: Project[];
  agents?: Agent[];
  members?: MemberWithUser[];
  tasks?: AgentTask[];
  workingAgents: WorkspaceWorkingAgent[];
  services: CompanyServiceProjection[];
  incidents: CompanyIncidentProjection[];
  systemGraph: SystemGraphCatalog;
}): HqMap {
  const openIssues = input.issues.filter((issue) => !terminalIssue.has(issue.status_category ?? issue.status));
  const activeProjects = input.projects.filter((project) => !terminalProject.has(project.status));
  const projectIds = new Set(activeProjects.map((project) => project.id));
  const nodes = new Map(input.systemGraph.nodes.map((node) => [node.id, node]));
  const servicesByKey = new Map(input.services.map((service) => [service.service_key, service]));
  const agentsById = new Map((input.agents ?? []).map((agent) => [agent.id, agent]));
  const membersById = new Map((input.members ?? []).map((member) => [member.id, member]));
  const tasks = input.tasks ?? [];

  const issueOutcomeIds = new Map<string, string>();
  for (const issue of openIssues) {
    issueOutcomeIds.set(issue.id, issue.project_id && projectIds.has(issue.project_id) ? issue.project_id : COMPANY_WORK_ID);
  }
  const serviceOutcomeIds = new Map<string, string>();
  for (const service of input.services) {
    if (service.project_id && projectIds.has(service.project_id)) serviceOutcomeIds.set(service.service_key, service.project_id);
  }
  const knownOutcomeIds = new Set([...projectIds, COMPANY_WORK_ID]);
  const affectedByIncident = new Map<string, CompanyIncidentProjection[]>();
  const sharedIncidents: HqSharedIncident[] = [];
  for (const incident of input.incidents) {
    const outcomeIds = incidentOutcomeIds({ incident, issueOutcomeIds, serviceOutcomeIds, knownOutcomeIds });
    for (const outcomeId of outcomeIds) {
      affectedByIncident.set(outcomeId, [...(affectedByIncident.get(outcomeId) ?? []), incident]);
    }
    const systemIds = incident.affected_refs
      .map((ref) => referenceValue(ref, ["system", "node", "service"]))
      .filter((value): value is string => !!value);
    if (outcomeIds.length > 1 || systemIds.length > 1) {
      sharedIncidents.push({ incident, outcomeIds, systemIds: [...new Set(systemIds)] });
    }
  }

  const buildOutcome = (project: Project | null): HqOutcome => {
    const id = project?.id ?? COMPANY_WORK_ID;
    const issues = openIssues.filter((issue) => issueOutcomeIds.get(issue.id) === id);
    const work = issues.map((issue) => ({
      issue,
      item: workforceItem(issue),
      workers: input.workingAgents.filter((agent) => agent.issue_ids.includes(issue.id)),
      assigneeName: assigneeNameForIssue(issue, agentsById, membersById),
      latestTask: latestTaskForIssue(tasks, issue.id),
    })).sort((left, right) => validTime(timestampFor(right.issue)) - validTime(timestampFor(left.issue)));
    const relatedIssueIds = new Set(issues.map((issue) => issue.id));
    const workers = input.workingAgents.filter((agent) => agent.issue_ids.some((issueId) => relatedIssueIds.has(issueId)));
    const assignees = [...new Set(work.map((entry) => entry.assigneeName).filter((name): name is string => !!name))];
    const services = input.services.filter((service) => service.project_id === project?.id);
    const incidents = affectedByIncident.get(id) ?? [];
    const systems = outcomeSystems({ services, incidents, nodes, servicesByKey });
    const systemIds = new Set(systems.map((system) => system.node?.id ?? system.id));
    const systemEdges = input.systemGraph.edges.filter((edge) => systemIds.has(edge.from) && systemIds.has(edge.to));
    const fallbackTime = project?.updated_at || work[0]?.issue.updated_at || new Date(0).toISOString();
    const lastUpdatedAt = newestTimestamp(work.map((entry) => timestampFor(entry.issue)), fallbackTime);
    return {
      id,
      project,
      title: project?.title ?? "Company-wide work",
      description: project?.description ?? null,
      work,
      workers,
      assignees,
      services,
      systems,
      incidents,
      systemEdges,
      // A project marked in-progress is intent, not proof that work is moving.
      // Only current work evidence can earn the moving state.
      lane: laneForWork(work),
      urgency: urgencyFor(project, work, incidents),
      lastUpdatedAt,
      isStale: validTime(lastUpdatedAt) > 0 && Date.now() - validTime(lastUpdatedAt) > 24 * 60 * 60 * 1000,
    };
  };

  const outcomes = activeProjects.map(buildOutcome);
  if (openIssues.some((issue) => issueOutcomeIds.get(issue.id) === COMPANY_WORK_ID)) outcomes.push(buildOutcome(null));
  outcomes.sort((left, right) =>
    LANE_ORDER[left.lane] - LANE_ORDER[right.lane] ||
    URGENCY_ORDER[left.urgency] - URGENCY_ORDER[right.urgency] ||
    validTime(right.lastUpdatedAt) - validTime(left.lastUpdatedAt),
  );

  const areaMap = new Map<string, HqArea>();
  for (const outcome of outcomes) {
    const service = outcome.services[0];
    const areaId = service?.area_key ?? (outcome.project ? "company-outcomes" : "shared-work");
    const existing = areaMap.get(areaId);
    if (existing) existing.outcomeIds.push(outcome.id);
    else areaMap.set(areaId, {
      id: areaId,
      name: service?.display_name ?? (areaId === "company-outcomes" ? "Company outcomes" : "Shared work"),
      outcomeIds: [outcome.id],
    });
  }

  return {
    outcomes,
    areas: [...areaMap.values()].sort((left, right) => left.name.localeCompare(right.name)),
    sharedIncidents: sharedIncidents.sort((left, right) => validTime(right.incident.last_observed_at) - validTime(left.incident.last_observed_at)),
  };
}

export function visibleHqOutcomes(outcomes: HqOutcome[], limit = 8): { visible: HqOutcome[]; overflow: number } {
  return { visible: outcomes.slice(0, limit), overflow: Math.max(0, outcomes.length - limit) };
}

export function outcomeMatches(outcome: HqOutcome, query: string): boolean {
  const needle = query.trim().toLowerCase();
  if (!needle) return true;
  return [
    outcome.title,
    outcome.description,
    ...outcome.work.flatMap((entry) => [entry.issue.title, entry.issue.identifier, entry.item.latest]),
    ...outcome.systems.map((system) => system.name),
    ...outcome.incidents.map((incident) => incident.title),
  ].filter((value): value is string => !!value).some((value) => value.toLowerCase().includes(needle));
}
