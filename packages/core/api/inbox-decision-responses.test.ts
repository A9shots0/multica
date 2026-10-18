// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "./client";

afterEach(() => vi.restoreAllMocks());

function mockFetch(body: unknown, status = 200) {
  vi.spyOn(global, "fetch").mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    statusText: status === 200 ? "OK" : status === 201 ? "Created" : "Error",
    json: () => Promise.resolve(body),
    headers: new Headers(),
  } as Response);
}

const client = new ApiClient("https://api.example.test");

// ---------------------------------------------------------------------------
// archiveCompletedInbox — POST /api/inbox/archive-completed
// Server always returns { count: <number> }; InboxCountResponseSchema applies.
// ---------------------------------------------------------------------------
describe("archiveCompletedInbox", () => {
  it("returns the count from a well-formed response", async () => {
    mockFetch({ count: 3 });
    const result = await client.archiveCompletedInbox();
    expect(result).toEqual({ count: 3 });
  });

  it("falls back to count=0 when count field is missing", async () => {
    mockFetch({});
    const result = await client.archiveCompletedInbox();
    expect(result).toEqual({ count: 0 });
  });

  it("falls back to count=0 when count is a string", async () => {
    mockFetch({ count: "many" });
    const result = await client.archiveCompletedInbox();
    expect(result).toEqual({ count: 0 });
  });

  it("falls back to count=0 when response is not an object", async () => {
    mockFetch("malformed");
    const result = await client.archiveCompletedInbox();
    expect(result).toEqual({ count: 0 });
  });

  it("falls back to count=0 when response is null", async () => {
    mockFetch(null);
    const result = await client.archiveCompletedInbox();
    expect(result).toEqual({ count: 0 });
  });

  it("passes through extra server fields alongside count", async () => {
    mockFetch({ count: 2, extra_field: "ignored" });
    const result = await client.archiveCompletedInbox();
    expect(result.count).toBe(2);
  });
});

// ---------------------------------------------------------------------------
// answerWorkflowOwnerDecision — POST /api/issues/{id}/workflow-owner-decision
//
// Server returns issueWorkflowEventResponse (201):
//   event_id: string (required UUID)
//   dispatch: string (omitempty — "queued" or "pending_retry")
//   plan, decision, replay: also present but not consumed by the caller.
//
// The caller in mutations.ts discards the return value and only cares about
// success vs failure.  We still validate so a 2xx with a missing event_id
// (server contract violation) surfaces as a hard error rather than silent
// success, consistent with resolveComment / updateComment / archiveInbox.
// ---------------------------------------------------------------------------
describe("answerWorkflowOwnerDecision", () => {
  it("returns a typed receipt for a well-formed response with dispatch", async () => {
    mockFetch({ event_id: "evt-uuid-1", dispatch: "queued" }, 201);
    const result = await client.answerWorkflowOwnerDecision("issue-1", "req-evt-1", "Approved");
    expect(result).toMatchObject({ event_id: "evt-uuid-1", dispatch: "queued" });
  });

  it("returns a typed receipt when dispatch is omitted (omitempty)", async () => {
    mockFetch({ event_id: "evt-uuid-2" }, 201);
    const result = await client.answerWorkflowOwnerDecision("issue-1", "req-evt-1", "Approved");
    expect(result.event_id).toBe("evt-uuid-2");
    expect(result.dispatch).toBeUndefined();
  });

  it("passes through extra fields from the server response", async () => {
    mockFetch({ event_id: "evt-uuid-3", dispatch: "queued", plan: { revision: 7 } }, 201);
    const result = await client.answerWorkflowOwnerDecision("issue-1", "req-evt-1", "Approved") as Record<string, unknown>;
    expect(result.event_id).toBe("evt-uuid-3");
  });

  it("throws when event_id is missing (malformed — contract violation)", async () => {
    mockFetch({ dispatch: "queued" }, 201);
    await expect(
      client.answerWorkflowOwnerDecision("issue-1", "req-evt-1", "Approved"),
    ).rejects.toThrow("malformed");
  });

  it("throws when event_id is an empty string", async () => {
    mockFetch({ event_id: "", dispatch: "queued" }, 201);
    await expect(
      client.answerWorkflowOwnerDecision("issue-1", "req-evt-1", "Approved"),
    ).rejects.toThrow("malformed");
  });

  it("throws when the response is null", async () => {
    mockFetch(null, 201);
    await expect(
      client.answerWorkflowOwnerDecision("issue-1", "req-evt-1", "Approved"),
    ).rejects.toThrow("malformed");
  });

  it("throws when the response is an array instead of an object", async () => {
    mockFetch([{ event_id: "evt-uuid-1" }], 201);
    await expect(
      client.answerWorkflowOwnerDecision("issue-1", "req-evt-1", "Approved"),
    ).rejects.toThrow("malformed");
  });
});
