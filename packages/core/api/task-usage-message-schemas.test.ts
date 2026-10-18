// @vitest-environment node
import { describe, expect, it, vi, afterEach } from "vitest";
import { ApiClient } from "./client";

afterEach(() => {
  vi.unstubAllGlobals();
});

function mockFetch(body: unknown, status = 200) {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue(
      new Response(JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
    ),
  );
}

// Matches Go's protocol.TaskMessagePayload struct (pkg/protocol/messages.go).
const wellFormedMessage = {
  task_id: "task-uuid-1",
  issue_id: "issue-uuid-1",
  seq: 1,
  type: "text",
  content: "hello world",
  created_at: "2024-01-01T00:00:00Z",
};

// Matches Go's GetIssueUsage handler response (handler/daemon.go).
const wellFormedUsage = {
  total_input_tokens: 100,
  total_output_tokens: 200,
  total_cache_read_tokens: 50,
  total_cache_write_tokens: 10,
  cost_usd_ticks: 999,
  uncosted_input_tokens: 0,
  uncosted_output_tokens: 0,
  uncosted_cache_read_tokens: 0,
  uncosted_cache_write_tokens: 0,
  task_count: 3,
};

describe("ApiClient.listTaskMessages", () => {
  it("returns the parsed list for a well-formed response", async () => {
    mockFetch([wellFormedMessage]);
    const client = new ApiClient("https://api.example.test");
    const result = await client.listTaskMessages("task-uuid-1");
    expect(result).toHaveLength(1);
    const first = result[0];
    expect(first).toBeDefined();
    expect(first!.task_id).toBe("task-uuid-1");
    expect(first!.seq).toBe(1);
    expect(first!.type).toBe("text");
    expect(first!.content).toBe("hello world");
  });

  it("parses all known message types", async () => {
    const types = ["text", "thinking", "tool_use", "tool_result", "error"];
    const messages = types.map((type, i) => ({ ...wellFormedMessage, seq: i, type }));
    mockFetch(messages);
    const client = new ApiClient("https://api.example.test");
    const result = await client.listTaskMessages("task-uuid-1");
    expect(result).toHaveLength(types.length);
    for (const [i, type] of types.entries()) {
      expect(result[i]!.type).toBe(type);
    }
  });

  it("does not drop a message with an unknown type", async () => {
    const unknownTypeMessage = { ...wellFormedMessage, type: "future_unknown_type" };
    mockFetch([wellFormedMessage, unknownTypeMessage]);
    const client = new ApiClient("https://api.example.test");
    const result = await client.listTaskMessages("task-uuid-1");
    expect(result).toHaveLength(2);
    expect(result[1]!.type).toBe("future_unknown_type");
  });

  it("includes optional fields when present (tool_use message)", async () => {
    const toolMsg = {
      task_id: "task-uuid-1",
      issue_id: "issue-uuid-1",
      seq: 5,
      type: "tool_use",
      tool: "bash",
      input: { command: "ls -la" },
      created_at: "2024-01-01T00:00:05Z",
    };
    mockFetch([toolMsg]);
    const client = new ApiClient("https://api.example.test");
    const result = await client.listTaskMessages("task-uuid-1");
    expect(result).toHaveLength(1);
    const toolResult = result[0];
    expect(toolResult).toBeDefined();
    expect(toolResult!.tool).toBe("bash");
    expect(toolResult!.input).toEqual({ command: "ls -la" });
  });

  it("returns empty array for a malformed response (not an array)", async () => {
    mockFetch({ not: "an array" });
    const client = new ApiClient("https://api.example.test");
    const result = await client.listTaskMessages("task-uuid-1");
    expect(result).toEqual([]);
  });

  it("returns empty array for a null response", async () => {
    mockFetch(null);
    const client = new ApiClient("https://api.example.test");
    const result = await client.listTaskMessages("task-uuid-1");
    expect(result).toEqual([]);
  });

  it("does not pass a malformed individual message through — drops it from the list", async () => {
    // A message missing required `seq` and `task_id` is malformed and must not appear.
    const malformed = { type: "text", content: "orphan" };
    mockFetch([wellFormedMessage, malformed]);
    const client = new ApiClient("https://api.example.test");
    const result = await client.listTaskMessages("task-uuid-1");
    // The whole list falls back to [] because the array schema rejects items
    // missing required fields. A well-formed message is still included if it
    // comes from a server that sends a homogeneous valid list.
    // Verify at minimum that the malformed item did not survive unvalidated.
    for (const msg of result) {
      expect(msg).toHaveProperty("task_id");
      expect(msg).toHaveProperty("seq");
    }
  });
});

describe("ApiClient.getIssueUsage", () => {
  it("returns the parsed summary for a well-formed response", async () => {
    mockFetch(wellFormedUsage);
    const client = new ApiClient("https://api.example.test");
    const result = await client.getIssueUsage("issue-uuid-1");
    expect(result.total_input_tokens).toBe(100);
    expect(result.total_output_tokens).toBe(200);
    expect(result.total_cache_read_tokens).toBe(50);
    expect(result.total_cache_write_tokens).toBe(10);
    expect(result.task_count).toBe(3);
    expect(result.cost_usd_ticks).toBe(999);
  });

  it("parses a response from an older server that omits cost-split fields", async () => {
    const olderServerResponse = {
      total_input_tokens: 50,
      total_output_tokens: 80,
      total_cache_read_tokens: 0,
      total_cache_write_tokens: 0,
      task_count: 1,
    };
    mockFetch(olderServerResponse);
    const client = new ApiClient("https://api.example.test");
    const result = await client.getIssueUsage("issue-uuid-1");
    expect(result.total_input_tokens).toBe(50);
    expect(result.task_count).toBe(1);
    expect(result.cost_usd_ticks).toBeUndefined();
    expect(result.uncosted_input_tokens).toBeUndefined();
  });

  it("returns the empty fallback for a malformed response (not an object)", async () => {
    mockFetch("not an object");
    const client = new ApiClient("https://api.example.test");
    const result = await client.getIssueUsage("issue-uuid-1");
    expect(result.total_input_tokens).toBe(0);
    expect(result.total_output_tokens).toBe(0);
    expect(result.task_count).toBe(0);
  });

  it("returns the empty fallback for a null response", async () => {
    mockFetch(null);
    const client = new ApiClient("https://api.example.test");
    const result = await client.getIssueUsage("issue-uuid-1");
    expect(result.total_input_tokens).toBe(0);
    expect(result.task_count).toBe(0);
  });

  it("does not pass through a response that is an array instead of an object", async () => {
    mockFetch([{ total_input_tokens: 999 }]);
    const client = new ApiClient("https://api.example.test");
    const result = await client.getIssueUsage("issue-uuid-1");
    // Must have returned the fallback, not the malformed array content.
    expect(result.total_input_tokens).toBe(0);
  });
});
