// @vitest-environment node
import { QueryClient, InfiniteQueryObserver } from "@tanstack/react-query";
import { describe, it, expect, vi } from "vitest";
import { api } from "../api";
import { chatWorkHistoryOptions } from "./queries";

vi.mock("../api", () => ({ api: { getChatWork: vi.fn() } }));
describe("work history cursor", () => {
  it("loads older pages with the exact time and ID, in a separate scoped cache", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const item = { id: "event-a", issue_id: "one", event_type: "review", reason: "Reviewed", created_at: "2026-09-24T12:00:00.123456Z" };
    vi.mocked(api.getChatWork).mockResolvedValueOnce({ issues: [], observed_at: "now", activity: [item], has_older: true }).mockResolvedValueOnce({ issues: [], observed_at: "now", activity: [{ ...item, id: "event-0" }], has_older: false });
    const observer = new InfiniteQueryObserver(client, chatWorkHistoryOptions("workspace", "session"));
    await observer.refetch();
    await observer.fetchNextPage();
    expect(api.getChatWork).toHaveBeenLastCalledWith("session", { time: item.created_at, id: item.id });
    expect(observer.getCurrentResult().data?.pages).toHaveLength(2);
    expect(observer.getCurrentResult().hasNextPage).toBe(false);
    expect(client.getQueryData(["chat-work", "workspace", "session"])).toBeUndefined();
    client.clear();
  });
  it("treats an invalid packet as failure rather than the end of history", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    vi.mocked(api.getChatWork).mockResolvedValue(null);
    await expect(client.fetchInfiniteQuery(chatWorkHistoryOptions("workspace", "session"))).rejects.toThrow("unavailable");
    client.clear();
  });
});
