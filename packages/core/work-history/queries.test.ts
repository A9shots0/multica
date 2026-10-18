import { describe, expect, it, vi } from "vitest";
import { workHistorySearchOptions } from "./queries";

const api = vi.hoisted(() => ({ searchWorkHistory: vi.fn() }));
vi.mock("../api", () => ({ api }));

describe("workHistorySearchOptions", () => {
  it("does not search while there is no submitted query", () => {
    const options = workHistorySearchOptions("workspace-1", "   ");
    expect(options.enabled).toBe(false);
  });

  it("normalizes the submitted query and keeps it in the workspace cache key", async () => {
    api.searchWorkHistory.mockResolvedValue({ history: { groups: [] } });
    const options = workHistorySearchOptions("workspace-1", "  LinkHub 403  ");
    expect(options.enabled).toBe(true);
    expect(options.queryKey).toEqual(["work-history", "workspace-1", "search", "LinkHub 403"]);
    await options.queryFn?.({} as never);
    expect(api.searchWorkHistory).toHaveBeenCalledWith({ query: "LinkHub 403", limit: 30 });
  });
});
