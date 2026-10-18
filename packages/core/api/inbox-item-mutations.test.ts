// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "./client";

afterEach(() => vi.unstubAllGlobals());

const wellFormed = {
  id: "inbox-1",
  workspace_id: "ws-1",
  recipient_type: "member",
  recipient_id: "member-1",
  type: "new_comment",
  severity: "info",
  issue_id: "issue-1",
  title: "Issue title",
  body: null,
  issue_status: "in_progress",
  issue_priority: "high",
  read: true,
  archived: false,
  created_at: "2026-09-16T10:00:00Z",
  actor_type: "member",
  actor_id: "member-2",
  details: { comment_id: "c-1" },
};

function mockFetch(body: unknown, status = 200) {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue(new Response(JSON.stringify(body), { status })),
  );
}

const client = new ApiClient("https://api.example.test");

describe("markInboxRead", () => {
  it("returns the validated inbox item on a well-formed response", async () => {
    mockFetch(wellFormed);
    const result = await client.markInboxRead("inbox-1");
    expect(result).toMatchObject({ id: "inbox-1", read: true, archived: false });
  });

  it("passes through extra server fields (actor_type, actor_id, details)", async () => {
    mockFetch(wellFormed);
    const result = (await client.markInboxRead("inbox-1")) as unknown as Record<string, unknown>;
    expect(result.actor_type).toBe("member");
    expect(result.actor_id).toBe("member-2");
  });

  it("throws on a malformed response (missing required id)", async () => {
    const { id, ...noId } = wellFormed;
    void id;
    mockFetch(noId);
    await expect(client.markInboxRead("inbox-1")).rejects.toThrow("malformed inbox item");
  });

  it("throws on a malformed response (read is not a boolean)", async () => {
    mockFetch({ ...wellFormed, read: "yes" });
    await expect(client.markInboxRead("inbox-1")).rejects.toThrow("malformed inbox item");
  });

  it("throws when the server returns null", async () => {
    mockFetch(null);
    await expect(client.markInboxRead("inbox-1")).rejects.toThrow("malformed inbox item");
  });
});

describe("markInboxUnread", () => {
  it("returns the validated inbox item on a well-formed response", async () => {
    mockFetch({ ...wellFormed, read: false });
    const result = await client.markInboxUnread("inbox-1");
    expect(result).toMatchObject({ id: "inbox-1", read: false });
  });

  it("throws on a malformed response (missing required workspace_id)", async () => {
    const { workspace_id, ...noWs } = wellFormed;
    void workspace_id;
    mockFetch(noWs);
    await expect(client.markInboxUnread("inbox-1")).rejects.toThrow("malformed inbox item");
  });

  it("throws when the server returns an array instead of an object", async () => {
    mockFetch([wellFormed]);
    await expect(client.markInboxUnread("inbox-1")).rejects.toThrow("malformed inbox item");
  });
});

describe("archiveInbox", () => {
  it("returns the validated inbox item on a well-formed response", async () => {
    mockFetch({ ...wellFormed, archived: true });
    const result = await client.archiveInbox("inbox-1");
    expect(result).toMatchObject({ id: "inbox-1", archived: true });
  });

  it("throws on a malformed response (archived is not a boolean)", async () => {
    mockFetch({ ...wellFormed, archived: 1 });
    await expect(client.archiveInbox("inbox-1")).rejects.toThrow("malformed inbox item");
  });

  it("throws when the server returns null", async () => {
    mockFetch(null);
    await expect(client.archiveInbox("inbox-1")).rejects.toThrow("malformed inbox item");
  });
});

describe("unarchiveInbox", () => {
  it("returns the validated inbox item on a well-formed response", async () => {
    mockFetch({ ...wellFormed, archived: false });
    const result = await client.unarchiveInbox("inbox-1");
    expect(result).toMatchObject({ id: "inbox-1", archived: false });
  });

  it("throws on a malformed response (missing required title)", async () => {
    const { title, ...noTitle } = wellFormed;
    void title;
    mockFetch(noTitle);
    await expect(client.unarchiveInbox("inbox-1")).rejects.toThrow("malformed inbox item");
  });

  it("throws when the server returns a non-object", async () => {
    mockFetch("not-an-object");
    await expect(client.unarchiveInbox("inbox-1")).rejects.toThrow("malformed inbox item");
  });
});
