import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";

/**
 * Workspace-independent on purpose: the knowledge hub and the Intelligence
 * ledger are both company-wide, not scoped to one Multica workspace — unlike
 * chatKeys (see chat/queries.ts), no wsId belongs in these keys.
 */
export const knowledgeKeys = {
  hubProjects: () => ["knowledge", "hub-projects"] as const,
  hubTree: () => ["knowledge", "hub-tree"] as const,
  hubFile: (path: string) => ["knowledge", "hub-file", path] as const,
  intelligenceSummary: () => ["knowledge", "intelligence-summary"] as const,
  intelligenceBusinessSummary: () => ["knowledge", "intelligence-business-summary"] as const,
};

export function intelligenceBusinessSummaryOptions() {
  return queryOptions({
    queryKey: knowledgeKeys.intelligenceBusinessSummary(),
    queryFn: () => api.getIntelligenceBusinessSummary(),
    staleTime: 30_000,
  });
}

export function knowledgeHubProjectsOptions() {
  return queryOptions({
    queryKey: knowledgeKeys.hubProjects(),
    queryFn: () => api.getKnowledgeHubProjects(),
    staleTime: 60_000,
  });
}

export function knowledgeHubTreeOptions() {
  return queryOptions({
    queryKey: knowledgeKeys.hubTree(),
    queryFn: () => api.getKnowledgeHubTree(),
    // The hub changes by other agents/sessions editing files on disk, not by
    // anything this page does — a short stale time keeps a left-open tab from
    // hammering the endpoint while still catching same-session edits on
    // refocus.
    staleTime: 60_000,
  });
}

export function knowledgeHubFileOptions(path: string) {
  return queryOptions({
    queryKey: knowledgeKeys.hubFile(path),
    queryFn: () => api.getKnowledgeHubFile(path),
    enabled: path !== "",
    staleTime: 60_000,
  });
}

export function intelligenceSummaryOptions() {
  return queryOptions({
    queryKey: knowledgeKeys.intelligenceSummary(),
    queryFn: () => api.getIntelligenceSummary(),
    // The ledger is actively growing (hourly collectors) — refetch on focus
    // is worth it here in a way it usually isn't, since "how fresh is this"
    // is the whole point of the freshness column.
    staleTime: 30_000,
  });
}
