import { describe, expect, it, vi, afterEach } from "vitest";
import { ApiClient } from "./client";

afterEach(() => {
  vi.unstubAllGlobals();
});

// A well-formed agent body matching what the Go handler's AgentResponse struct
// serialises — all required AgentSchema fields present, pointer fields as
// nullable JSON values, omitempty fields omitted.
const wellFormedAgent = {
  id: "agent-uuid-1",
  workspace_id: "workspace-uuid-1",
  runtime_id: "runtime-uuid-1",
  runtime_bound: true,
  name: "Test Agent",
  description: "A test agent",
  instructions: "Do things",
  conversation_starters: [],
  avatar_url: null,
  runtime_mode: "local",
  runtime_config: {},
  custom_args: [],
  mcp_config: null,
  has_custom_env: false,
  custom_env_key_count: 0,
  mcp_config_redacted: false,
  visibility: "private",
  permission_mode: "private",
  invocation_targets: [],
  status: "idle",
  max_concurrent_tasks: 1,
  model: "",
  thinking_level: "",
  service_tier: "",
  owner_id: "owner-uuid-1",
  skills: [],
  disabled_runtime_skills: [],
  created_at: "2024-01-01T00:00:00Z",
  updated_at: "2024-01-01T00:00:00Z",
  archived_at: null,
  archived_by: null,
};

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

describe("ApiClient.getAgent", () => {
  it("returns the validated agent on a well-formed response", async () => {
    mockFetch(wellFormedAgent);
    const client = new ApiClient("https://api.example.test");
    const result = await client.getAgent("agent-uuid-1");
    expect(result.id).toBe("agent-uuid-1");
    expect(result.workspace_id).toBe("workspace-uuid-1");
  });

  it("throws and does not return a fabricated agent on a malformed response", async () => {
    // Missing required fields: id, workspace_id, created_at, updated_at
    mockFetch({ name: "ghost", status: "idle" });
    const client = new ApiClient("https://api.example.test");
    await expect(client.getAgent("agent-uuid-1")).rejects.toThrow(/malformed agent/i);
  });
});

describe("ApiClient.archiveAgent", () => {
  it("returns the validated agent on a well-formed response", async () => {
    const archived = { ...wellFormedAgent, archived_at: "2024-06-01T00:00:00Z", archived_by: "owner-uuid-1" };
    mockFetch(archived);
    const client = new ApiClient("https://api.example.test");
    const result = await client.archiveAgent("agent-uuid-1");
    expect(result.id).toBe("agent-uuid-1");
    expect(result.archived_at).toBe("2024-06-01T00:00:00Z");
  });

  it("throws and does not return a fabricated agent on a malformed response", async () => {
    mockFetch({ archived_at: "2024-06-01T00:00:00Z" });
    const client = new ApiClient("https://api.example.test");
    await expect(client.archiveAgent("agent-uuid-1")).rejects.toThrow(/malformed agent/i);
  });
});

describe("ApiClient.restoreAgent", () => {
  it("returns the validated agent on a well-formed response", async () => {
    const restored = { ...wellFormedAgent, archived_at: null, archived_by: null };
    mockFetch(restored);
    const client = new ApiClient("https://api.example.test");
    const result = await client.restoreAgent("agent-uuid-1");
    expect(result.id).toBe("agent-uuid-1");
    expect(result.archived_at).toBeNull();
  });

  it("throws and does not return a fabricated agent on a malformed response", async () => {
    mockFetch({ archived_at: null });
    const client = new ApiClient("https://api.example.test");
    await expect(client.restoreAgent("agent-uuid-1")).rejects.toThrow(/malformed agent/i);
  });
});
