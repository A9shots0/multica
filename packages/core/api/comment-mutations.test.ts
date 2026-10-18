import { describe, expect, it, vi, afterEach } from "vitest";
import { ApiClient } from "./client";

afterEach(() => {
  vi.unstubAllGlobals();
});

// A well-formed comment body matching what the Go handler's CommentResponse struct
// serialises — all required CommentSchema fields present.
const wellFormedComment = {
  id: "comment-uuid-1",
  issue_id: "issue-uuid-1",
  author_type: "member",
  author_id: "user-uuid-1",
  content: "hello world",
  type: "comment",
  parent_id: null,
  created_at: "2024-01-01T00:00:00Z",
  updated_at: "2024-01-01T00:00:00Z",
  revision: 1,
  reactions: [],
  attachments: [],
  resolved_at: null,
  resolved_by_type: null,
  resolved_by_id: null,
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

describe("ApiClient.updateComment", () => {
  it("returns the validated comment on a well-formed response", async () => {
    mockFetch(wellFormedComment);
    const client = new ApiClient("https://api.example.test");
    const result = await client.updateComment("comment-uuid-1", "hello world");
    expect(result.id).toBe("comment-uuid-1");
    expect(result.issue_id).toBe("issue-uuid-1");
  });

  it("throws and does not return a fabricated comment on a malformed response", async () => {
    // Missing required fields: id, issue_id, content, created_at, updated_at
    mockFetch({ type: "comment" });
    const client = new ApiClient("https://api.example.test");
    await expect(
      client.updateComment("comment-uuid-1", "hello world"),
    ).rejects.toThrow(/malformed comment/i);
  });
});

describe("ApiClient.resolveComment", () => {
  it("returns the validated comment on a well-formed response", async () => {
    mockFetch({ ...wellFormedComment, resolved_at: "2024-01-02T00:00:00Z" });
    const client = new ApiClient("https://api.example.test");
    const result = await client.resolveComment("comment-uuid-1");
    expect(result.id).toBe("comment-uuid-1");
  });

  it("throws and does not return a fabricated comment on a malformed response", async () => {
    mockFetch({ resolved_at: "2024-01-02T00:00:00Z" });
    const client = new ApiClient("https://api.example.test");
    await expect(
      client.resolveComment("comment-uuid-1"),
    ).rejects.toThrow(/malformed comment/i);
  });
});

describe("ApiClient.unresolveComment", () => {
  it("returns the validated comment on a well-formed response", async () => {
    mockFetch(wellFormedComment);
    const client = new ApiClient("https://api.example.test");
    const result = await client.unresolveComment("comment-uuid-1");
    expect(result.id).toBe("comment-uuid-1");
  });

  it("throws and does not return a fabricated comment on a malformed response", async () => {
    mockFetch({ resolved_at: null });
    const client = new ApiClient("https://api.example.test");
    await expect(
      client.unresolveComment("comment-uuid-1"),
    ).rejects.toThrow(/malformed comment/i);
  });
});
