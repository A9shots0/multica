// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "./client";

afterEach(() => vi.unstubAllGlobals());

function mockFetch(body: unknown, status = 200) {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue(new Response(JSON.stringify(body), { status })),
  );
}

const client = new ApiClient("https://api.example.test");

const wellFormedTask = {
  id: "task-1",
  agent_id: "agent-1",
  runtime_id: "runtime-1",
  issue_id: "issue-1",
  status: "cancelled",
  priority: 0,
  dispatched_at: null,
  started_at: null,
  completed_at: null,
  result: null,
  error: null,
  created_at: "2026-09-16T10:00:00Z",
};

describe("cancelTask", () => {
  it("returns the validated task on a well-formed response", async () => {
    mockFetch(wellFormedTask);
    const result = await client.cancelTask("issue-1", "task-1");
    expect(result).toMatchObject({ id: "task-1", status: "cancelled" });
  });

  it("passes through extra server fields (agent_id, issue_id)", async () => {
    mockFetch(wellFormedTask);
    const result = await client.cancelTask("issue-1", "task-1");
    expect(result.agent_id).toBe("agent-1");
    expect(result.issue_id).toBe("issue-1");
  });

  it("throws on a malformed response (missing required id)", async () => {
    const { id, ...noId } = wellFormedTask;
    void id;
    mockFetch(noId);
    await expect(client.cancelTask("issue-1", "task-1")).rejects.toThrow("malformed task");
  });

  it("throws when the server returns null", async () => {
    mockFetch(null);
    await expect(client.cancelTask("issue-1", "task-1")).rejects.toThrow("malformed task");
  });
});

describe("rerunIssue", () => {
  it("returns the validated task on a well-formed response", async () => {
    mockFetch({ ...wellFormedTask, status: "queued" });
    const result = await client.rerunIssue("issue-1");
    expect(result).toMatchObject({ id: "task-1", status: "queued" });
  });

  it("returns the validated task when taskId is provided", async () => {
    mockFetch(wellFormedTask);
    const result = await client.rerunIssue("issue-1", "task-1");
    expect(result.id).toBe("task-1");
  });

  it("throws on a malformed response (missing required id)", async () => {
    const { id, ...noId } = wellFormedTask;
    void id;
    mockFetch(noId);
    await expect(client.rerunIssue("issue-1")).rejects.toThrow("malformed task");
  });

  it("throws when the server returns an array instead of an object", async () => {
    mockFetch([wellFormedTask]);
    await expect(client.rerunIssue("issue-1")).rejects.toThrow("malformed task");
  });
});

describe("cancelAgentTasks", () => {
  it("returns the validated count on a well-formed response", async () => {
    mockFetch({ cancelled: 3 });
    const result = await client.cancelAgentTasks("agent-1");
    expect(result).toEqual({ cancelled: 3 });
  });

  it("returns zero count on a malformed response (missing cancelled field)", async () => {
    mockFetch({});
    const result = await client.cancelAgentTasks("agent-1");
    expect(result).toEqual({ cancelled: 0 });
  });

  it("returns zero count when the server returns null", async () => {
    mockFetch(null);
    const result = await client.cancelAgentTasks("agent-1");
    expect(result).toEqual({ cancelled: 0 });
  });

  it("returns zero count when cancelled is not a number", async () => {
    mockFetch({ cancelled: "many" });
    const result = await client.cancelAgentTasks("agent-1");
    expect(result).toEqual({ cancelled: 0 });
  });
});
