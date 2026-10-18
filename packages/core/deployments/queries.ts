import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";

export const deploymentKeys = {
  history: () => ["deployments", "history"] as const,
};

export function deploymentHistoryOptions() {
  return queryOptions({
    queryKey: deploymentKeys.history(),
    queryFn: () => api.getDeploymentHistory(),
    staleTime: 30_000,
  });
}
