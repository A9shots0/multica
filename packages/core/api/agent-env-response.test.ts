import { describe, it, expect, vi } from "vitest";
import { AgentEnvResponseSchema } from "./schemas";
import { parseWithFallback, setSchemaLogger } from "./schema";
import type { AgentEnvResponse } from "../types";

// ---------------------------------------------------------------------------
// Schema tests
// ---------------------------------------------------------------------------

describe("AgentEnvResponseSchema", () => {
  it("parses a well-formed response with env values", () => {
    const raw = {
      agent_id: "abc-123",
      custom_env: { SECRET_KEY: "super-secret", API_URL: "https://example.com" },
    };
    const result = AgentEnvResponseSchema.safeParse(raw);
    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.data.agent_id).toBe("abc-123");
      expect(result.data.custom_env).toEqual({
        SECRET_KEY: "super-secret",
        API_URL: "https://example.com",
      });
    }
  });

  it("parses a well-formed response with an empty custom_env map", () => {
    const raw = { agent_id: "abc-123", custom_env: {} };
    const result = AgentEnvResponseSchema.safeParse(raw);
    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.data.custom_env).toEqual({});
    }
  });

  it("rejects a response missing agent_id", () => {
    const raw = { custom_env: { KEY: "val" } };
    const result = AgentEnvResponseSchema.safeParse(raw);
    expect(result.success).toBe(false);
  });

  it("rejects a response missing custom_env", () => {
    const raw = { agent_id: "abc-123" };
    const result = AgentEnvResponseSchema.safeParse(raw);
    expect(result.success).toBe(false);
  });

  it("rejects a response where custom_env has a non-string value", () => {
    const raw = { agent_id: "abc-123", custom_env: { KEY: 42 } };
    const result = AgentEnvResponseSchema.safeParse(raw);
    expect(result.success).toBe(false);
  });

  it("rejects null", () => {
    const result = AgentEnvResponseSchema.safeParse(null);
    expect(result.success).toBe(false);
  });

  it("rejects an array", () => {
    const result = AgentEnvResponseSchema.safeParse([]);
    expect(result.success).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// parseWithFallback + redactReceived tests
// ---------------------------------------------------------------------------

describe("parseWithFallback with AgentEnvResponseSchema", () => {
  const FALLBACK = null as AgentEnvResponse | null;

  it("returns parsed value for a well-formed response", () => {
    const raw = { agent_id: "x", custom_env: { A: "1" } };
    const result = parseWithFallback<AgentEnvResponse | null>(
      raw,
      AgentEnvResponseSchema,
      FALLBACK,
      { endpoint: "GET /api/agents/:id/env", redactReceived: true },
    );
    expect(result).not.toBeNull();
    expect(result?.agent_id).toBe("x");
    expect(result?.custom_env).toEqual({ A: "1" });
  });

  it("returns fallback for a malformed response", () => {
    const raw = { not_what_we_expected: true };
    const result = parseWithFallback<AgentEnvResponse | null>(
      raw,
      AgentEnvResponseSchema,
      FALLBACK,
      { endpoint: "GET /api/agents/:id/env", redactReceived: true },
    );
    expect(result).toBeNull();
  });

  it("does NOT include environment values in the warning log when redactReceived is true", () => {
    const warnSpy = vi.fn();
    setSchemaLogger({ warn: warnSpy, info: vi.fn(), error: vi.fn(), debug: vi.fn() });

    const secretValue = "super-secret-token-abc123";
    // A malformed response that still contains secret-looking values in
    // the shape we'd receive from the server (missing agent_id).
    const raw = { custom_env: { SECRET: secretValue } };

    parseWithFallback<AgentEnvResponse | null>(
      raw,
      AgentEnvResponseSchema,
      FALLBACK,
      { endpoint: "GET /api/agents/:id/env", redactReceived: true },
    );

    expect(warnSpy).toHaveBeenCalledOnce();
    const [_message, context] = warnSpy.mock.calls[0] as [string, Record<string, unknown>];
    // The received payload (containing the secret) must NOT appear in the log.
    expect(context).not.toHaveProperty("received");
    // Serialised form must not contain the secret value either.
    const serialised = JSON.stringify(context);
    expect(serialised).not.toContain(secretValue);

    // Restore no-op logger so other tests are not affected.
    setSchemaLogger({ warn: vi.fn(), info: vi.fn(), error: vi.fn(), debug: vi.fn() });
  });

  it("DOES include received in the log when redactReceived is not set", () => {
    const warnSpy = vi.fn();
    setSchemaLogger({ warn: warnSpy, info: vi.fn(), error: vi.fn(), debug: vi.fn() });

    const raw = { wrong: true };
    parseWithFallback<AgentEnvResponse | null>(
      raw,
      AgentEnvResponseSchema,
      FALLBACK,
      { endpoint: "some/other/endpoint" },
    );

    expect(warnSpy).toHaveBeenCalledOnce();
    const [_message, context] = warnSpy.mock.calls[0] as [string, Record<string, unknown>];
    expect(context).toHaveProperty("received");

    setSchemaLogger({ warn: vi.fn(), info: vi.fn(), error: vi.fn(), debug: vi.fn() });
  });
});
