import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";

export const workHistoryKeys = {
  all: (wsId: string) => ["work-history", wsId] as const,
  search: (wsId: string, query: string) =>
    [...workHistoryKeys.all(wsId), "search", query] as const,
};

export function workHistorySearchOptions(wsId: string, query: string) {
  const normalized = query.trim();
  return queryOptions({
    queryKey: workHistoryKeys.search(wsId, normalized),
    queryFn: () => api.searchWorkHistory({ query: normalized, limit: 30 }),
    enabled: !!wsId && normalized.length > 0,
    staleTime: 60_000,
    retry: 1,
  });
}
