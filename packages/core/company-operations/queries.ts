import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";

export const companyOperationsKeys = {
  all: (wsId: string) => ["company-operations", wsId] as const,
  picture: (wsId: string) => [...companyOperationsKeys.all(wsId), "picture"] as const,
  graph: (wsId: string) => [...companyOperationsKeys.all(wsId), "system-graph"] as const,
  journey: (wsId: string, journeyId: string) => [
    ...companyOperationsKeys.graph(wsId), "journey", journeyId,
  ] as const,
  blastRadius: (wsId: string, nodeIds: string[], edgeIds: string[]) => [
    ...companyOperationsKeys.graph(wsId), "blast-radius",
    [...nodeIds].sort(), [...edgeIds].sort(),
  ] as const,
};

export function companyOperatingPictureOptions(wsId: string) {
  return queryOptions({
    queryKey: companyOperationsKeys.picture(wsId),
    queryFn: () => api.getCompanyOperatingPicture(wsId),
    enabled: !!wsId,
    staleTime: 15_000,
    refetchInterval: 30_000,
    refetchOnWindowFocus: true,
  });
}

export function companySystemGraphOptions(wsId: string) {
  return queryOptions({
    queryKey: companyOperationsKeys.graph(wsId),
    queryFn: () => api.getSystemGraph(wsId),
    enabled: !!wsId,
    staleTime: 5 * 60_000,
  });
}

export function companySystemGraphJourneyOptions(wsId: string, journeyId: string) {
  return queryOptions({
    queryKey: companyOperationsKeys.journey(wsId, journeyId),
    queryFn: () => api.querySystemGraphJourney(wsId, {
      query: "journey_subgraph",
      journey_id: journeyId,
    }),
    enabled: !!wsId && !!journeyId,
    staleTime: 5 * 60_000,
  });
}

export function companySystemGraphBlastRadiusOptions(
  wsId: string,
  seedNodeIds: string[] = [],
  seedEdgeIds: string[] = [],
) {
  return queryOptions({
    queryKey: companyOperationsKeys.blastRadius(wsId, seedNodeIds, seedEdgeIds),
    queryFn: () => api.querySystemGraphBlastRadius(wsId, {
      query: "blast_radius",
      seed_node_ids: seedNodeIds,
      seed_edge_ids: seedEdgeIds,
    }),
    enabled: !!wsId && (seedNodeIds.length > 0 || seedEdgeIds.length > 0),
    staleTime: 60_000,
  });
}
