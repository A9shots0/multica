import { describe, expect, it } from "vitest";
import type { InboxItem } from "../types";
import { isOwnerRelevantInboxItem, ownerInboxCategory } from "./owner-control";

const item = (overrides: Partial<InboxItem>): InboxItem => ({
  id: "i", workspace_id: "w", recipient_type: "member", recipient_id: "m",
  actor_type: "system", actor_id: null, type: "task_failed", severity: "info",
  issue_id: "issue", title: "title", body: null, issue_status: null,
  read: false, archived: false, created_at: "2026-09-11T00:00:00Z", details: null,
  ...overrides,
});

describe("owner inbox control projection", () => {
  it("excludes status noise with no owner decision", () => {
    expect(isOwnerRelevantInboxItem(item({ type: "status_changed", issue_status: "working" }))).toBe(false);
    expect(isOwnerRelevantInboxItem(item({ type: "review_requested" }))).toBe(false);
  });

  it("surfaces failed runs as owner-visible incidents", () => {
    expect(isOwnerRelevantInboxItem(item({ type: "task_failed" }))).toBe(true);
    expect(ownerInboxCategory(item({ type: "task_failed" }))).toBe("incident");
  });

  it("keeps decisions, explicit incidents, meaningful updates, and verified completions", () => {
    expect(ownerInboxCategory(item({ type: "workflow_owner_decision" }))).toBe("needs_you");
    expect(ownerInboxCategory(item({ type: "agent_blocked", severity: "attention", details: { owner_relevant: "true" } }))).toBe("incident");
    expect(ownerInboxCategory(item({ type: "status_changed", issue_status: "working", details: { owner_relevant: "true", owner_category: "update" } }))).toBe("update");
    expect(ownerInboxCategory(item({ type: "task_completed", details: { independently_verified: "true" } }))).toBe("completion");
  });
});
