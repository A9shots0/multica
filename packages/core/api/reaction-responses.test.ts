import { describe, expect, it, vi, afterEach } from "vitest";
import { ApiClient } from "./client";

afterEach(() => {
  vi.unstubAllGlobals();
});

function mockFetch(body: unknown, status = 201) {
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

// Matches Go's ReactionResponse (handler/reaction.go addedReactionToResponse).
const wellFormedReaction = {
  id: "reaction-uuid-1",
  comment_id: "comment-uuid-1",
  actor_type: "member",
  actor_id: "user-uuid-1",
  emoji: "👍",
  created_at: "2024-01-01T00:00:00Z",
};

// Matches Go's IssueReactionResponse (handler/issue_reaction.go addedIssueReactionToResponse).
const wellFormedIssueReaction = {
  id: "reaction-uuid-2",
  issue_id: "issue-uuid-1",
  actor_type: "member",
  actor_id: "user-uuid-1",
  emoji: "🎉",
  created_at: "2024-01-01T00:00:00Z",
};

describe("ApiClient.addReaction", () => {
  it("returns the validated reaction on a well-formed response", async () => {
    mockFetch(wellFormedReaction);
    const client = new ApiClient("https://api.example.test");
    const result = await client.addReaction("comment-uuid-1", "👍");
    expect(result.id).toBe("reaction-uuid-1");
    expect(result.comment_id).toBe("comment-uuid-1");
    expect(result.emoji).toBe("👍");
  });

  it("includes comment_revision when the server sends it (omitempty field)", async () => {
    mockFetch({ ...wellFormedReaction, comment_revision: 42 });
    const client = new ApiClient("https://api.example.test");
    const result = await client.addReaction("comment-uuid-1", "👍");
    expect(result.comment_revision).toBe(42);
  });

  it("throws on a malformed response missing required fields", async () => {
    mockFetch({ emoji: "👍" });
    const client = new ApiClient("https://api.example.test");
    await expect(
      client.addReaction("comment-uuid-1", "👍"),
    ).rejects.toThrow(/malformed reaction/i);
  });
});

describe("ApiClient.addIssueReaction", () => {
  it("returns the validated issue reaction on a well-formed response", async () => {
    mockFetch(wellFormedIssueReaction);
    const client = new ApiClient("https://api.example.test");
    const result = await client.addIssueReaction("issue-uuid-1", "🎉");
    expect(result.id).toBe("reaction-uuid-2");
    expect(result.issue_id).toBe("issue-uuid-1");
    expect(result.emoji).toBe("🎉");
  });

  it("includes issue_revision when the server sends it (omitempty field)", async () => {
    mockFetch({ ...wellFormedIssueReaction, issue_revision: 7 });
    const client = new ApiClient("https://api.example.test");
    const result = await client.addIssueReaction("issue-uuid-1", "🎉");
    expect(result.issue_revision).toBe(7);
  });

  it("throws on a malformed response missing required fields", async () => {
    mockFetch({ emoji: "🎉" });
    const client = new ApiClient("https://api.example.test");
    await expect(
      client.addIssueReaction("issue-uuid-1", "🎉"),
    ).rejects.toThrow(/malformed reaction/i);
  });
});
