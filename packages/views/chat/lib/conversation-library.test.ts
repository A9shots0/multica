import { describe, expect, it } from "vitest";
import type { ChatSession } from "@multica/core/types";
import { filterConversations, groupConversationsByProject } from "./conversation-library";

const session = (id: string, values: Partial<ChatSession> = {}): ChatSession => ({
  id, workspace_id: "workspace", agent_id: "agent", creator_id: "owner",
  title: id, status: "active", has_unread: false,
  created_at: "2026-09-24T00:00:00Z", updated_at: "2026-09-24T00:00:00Z", ...values,
});

describe("conversation library", () => {
  it("groups only by durable project identity and preserves conversation order", () => {
    const sessions = [session("Viral Queue ideas"), session("A", { project_id: "project" }), session("B", { project_id: "project" })];
    expect(groupConversationsByProject(sessions).map((g) => [g.projectId, g.sessions.map((s) => s.id)]))
      .toEqual([[null, ["Viral Queue ideas"]], ["project", ["A", "B"]]]);
    expect(sessions.map((s) => s.id)).toEqual(["Viral Queue ideas", "A", "B"]);
  });
  it("combines search with unread state, including older unread-count payloads", () => {
    const sessions = [session("one", { project_id: "p", unread_count: 2 }), session("two", { project_id: "p" }), session("three", { has_unread: true })];
    expect(filterConversations(sessions, "  VIRAL ", "unread", new Set(), new Map([["p", "Viral Queue"]])).map((s) => s.id)).toEqual(["one"]);
  });
  it("does not claim work is running from a title or last-message text", () => {
    const sessions = [session("Running audit"), session("Actually pending")];
    expect(filterConversations(sessions, "", "working", new Set(["Actually pending"]), new Map()).map((s) => s.id)).toEqual(["Actually pending"]);
  });
});
