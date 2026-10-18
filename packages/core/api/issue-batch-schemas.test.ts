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

describe("batchUpdateIssues", () => {
  it("returns parsed value for a well-formed response", async () => {
    vi.stubGlobal("fetch", mockFetch({ updated: 3 }));
    const client = new ApiClient("https://api.example.test");
    const result = await client.batchUpdateIssues(["id-1", "id-2", "id-3"], { status: "done" });
    expect(result).toEqual({ updated: 3 });
  });

  it("returns fallback for a malformed response", async () => {
    vi.stubGlobal("fetch", mockFetch({ updated: "not-a-number" }));
    const client = new ApiClient("https://api.example.test");
    const result = await client.batchUpdateIssues(["id-1"], { status: "done" });
    expect(result).toEqual({ updated: 0 });
  });

  it("does not pass a malformed response through to the caller", async () => {
    vi.stubGlobal("fetch", mockFetch({ completely: "wrong" }));
    const client = new ApiClient("https://api.example.test");
    const result = await client.batchUpdateIssues(["id-1"], { status: "done" });
    expect(typeof result.updated).toBe("number");
  });
});

describe("batchDeleteIssues", () => {
  it("returns parsed value for a well-formed response", async () => {
    vi.stubGlobal("fetch", mockFetch({ deleted: 5 }));
    const client = new ApiClient("https://api.example.test");
    const result = await client.batchDeleteIssues(["id-1", "id-2", "id-3", "id-4", "id-5"]);
    expect(result).toEqual({ deleted: 5 });
  });

  it("returns fallback for a malformed response", async () => {
    vi.stubGlobal("fetch", mockFetch({ deleted: "five" }));
    const client = new ApiClient("https://api.example.test");
    const result = await client.batchDeleteIssues(["id-1"]);
    expect(result).toEqual({ deleted: 0 });
  });

  it("does not pass a malformed response through to the caller", async () => {
    vi.stubGlobal("fetch", mockFetch(null));
    const client = new ApiClient("https://api.example.test");
    const result = await client.batchDeleteIssues(["id-1"]);
    expect(typeof result.deleted).toBe("number");
  });
});

describe("getAssigneeFrequency", () => {
  it("returns parsed value for a well-formed response", async () => {
    const entries = [
      { assignee_type: "agent", assignee_id: "agent-1", frequency: 10 },
      { assignee_type: "member", assignee_id: "member-2", frequency: 3 },
    ];
    vi.stubGlobal("fetch", mockFetch(entries));
    const client = new ApiClient("https://api.example.test");
    const result = await client.getAssigneeFrequency();
    expect(result).toEqual(entries);
  });

  it("returns fallback for a malformed response (not an array)", async () => {
    vi.stubGlobal("fetch", mockFetch({ not: "an-array" }));
    const client = new ApiClient("https://api.example.test");
    const result = await client.getAssigneeFrequency();
    expect(result).toEqual([]);
  });

  it("does not pass a malformed response through to the caller", async () => {
    vi.stubGlobal("fetch", mockFetch("bad"));
    const client = new ApiClient("https://api.example.test");
    const result = await client.getAssigneeFrequency();
    expect(Array.isArray(result)).toBe(true);
  });

  it("passes unknown extra fields through (lenient schema)", async () => {
    const entries = [
      { assignee_type: "agent", assignee_id: "agent-1", frequency: 7, future_field: "ok" },
    ];
    vi.stubGlobal("fetch", mockFetch(entries));
    const client = new ApiClient("https://api.example.test");
    const result = await client.getAssigneeFrequency();
    expect(result[0]?.assignee_id).toBe("agent-1");
    expect(result[0]?.frequency).toBe(7);
  });
});
