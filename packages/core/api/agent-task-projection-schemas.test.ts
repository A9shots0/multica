import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "./client";

function mockFetch(body: unknown, status = 200) {
  return vi.fn().mockResolvedValue(
    new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
});

const VALID_TASK = {
  id: "task-1",
  agent_id: "agent-1",
  runtime_id: "rt-1",
  issue_id: "issue-1",
  status: "running",
  priority: 0,
  dispatched_at: null,
  started_at: null,
  completed_at: null,
  result: null,
  error: null,
  created_at: "2024-01-01T00:00:00Z",
};

describe("getAgentTaskSnapshot", () => {
  it("returns parsed tasks for a well-formed response", async () => {
    vi.stubGlobal("fetch", mockFetch([VALID_TASK]));
    const client = new ApiClient("https://api.example.test");
    const result = await client.getAgentTaskSnapshot();
    expect(result).toHaveLength(1);
    expect(result[0]?.id).toBe("task-1");
  });

  it("returns the fallback empty array for a malformed response", async () => {
    vi.stubGlobal("fetch", mockFetch({ not: "an array" }));
    const client = new ApiClient("https://api.example.test");
    const result = await client.getAgentTaskSnapshot();
    expect(result).toEqual([]);
  });

  it("does not pass a malformed response through unchanged", async () => {
    vi.stubGlobal("fetch", mockFetch("totally wrong"));
    const client = new ApiClient("https://api.example.test");
    const result = await client.getAgentTaskSnapshot();
    expect(Array.isArray(result)).toBe(true);
    expect(result).toEqual([]);
  });
});

describe("getWorkspaceAgentRunCounts", () => {
  const VALID_RUN_COUNT = { agent_id: "agent-1", run_count: 42 };

  it("returns parsed run counts for a well-formed response", async () => {
    vi.stubGlobal("fetch", mockFetch([VALID_RUN_COUNT]));
    const client = new ApiClient("https://api.example.test");
    const result = await client.getWorkspaceAgentRunCounts();
    expect(result).toHaveLength(1);
    expect(result[0]?.agent_id).toBe("agent-1");
    expect(result[0]?.run_count).toBe(42);
  });

  it("returns the fallback empty array for a malformed response", async () => {
    vi.stubGlobal("fetch", mockFetch({ unexpected: "object" }));
    const client = new ApiClient("https://api.example.test");
    const result = await client.getWorkspaceAgentRunCounts();
    expect(result).toEqual([]);
  });

  it("does not pass a malformed response through unchanged", async () => {
    vi.stubGlobal("fetch", mockFetch(null));
    const client = new ApiClient("https://api.example.test");
    const result = await client.getWorkspaceAgentRunCounts();
    expect(Array.isArray(result)).toBe(true);
    expect(result).toEqual([]);
  });
});

describe("getActiveTasksForIssue", () => {
  it("returns parsed tasks for a well-formed response", async () => {
    vi.stubGlobal("fetch", mockFetch({ tasks: [VALID_TASK] }));
    const client = new ApiClient("https://api.example.test");
    const result = await client.getActiveTasksForIssue("issue-1");
    expect(result.tasks).toHaveLength(1);
    expect(result.tasks[0]?.id).toBe("task-1");
  });

  it("returns the fallback for a malformed response (not an object)", async () => {
    vi.stubGlobal("fetch", mockFetch([VALID_TASK]));
    const client = new ApiClient("https://api.example.test");
    const result = await client.getActiveTasksForIssue("issue-1");
    expect(result).toEqual({ tasks: [] });
  });

  it("does not pass a malformed response through unchanged", async () => {
    vi.stubGlobal("fetch", mockFetch("not valid"));
    const client = new ApiClient("https://api.example.test");
    const result = await client.getActiveTasksForIssue("issue-1");
    expect(result).toEqual({ tasks: [] });
    expect(Array.isArray((result as unknown as unknown[]))).toBe(false);
  });
});
