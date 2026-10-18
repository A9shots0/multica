import { describe, expect, it } from "vitest";
import type { InboxItem } from "@multica/core/types";
import {
  deduplicateInboxItems,
  getAutopilotQuotaBody,
  getInboxDisplayTitle,
  getInboxNavigationTarget,
} from "./inbox-display";

function item(overrides: Partial<InboxItem>): InboxItem {
  return {
    id: "inbox-1",
    workspace_id: "workspace-1",
    recipient_type: "member",
    recipient_id: "member-1",
    actor_type: "agent",
    actor_id: "agent-1",
    type: "new_comment",
    severity: "info",
    issue_id: "issue-1",
    title: "Issue title",
    body: null,
    issue_status: null,
    read: false,
    archived: false,
    created_at: "2026-06-15T08:00:00Z",
    details: null,
    ...overrides,
  };
}

describe("deduplicateInboxItems", () => {
  it("filters raw issue noise out of the owner control center", () => {
    const merged = deduplicateInboxItems([
      item({
        id: "comment-notification",
        created_at: "2026-06-15T08:00:00Z",
        details: { comment_id: "comment-1" },
      }),
      item({
        id: "status-notification",
        type: "status_changed",
        created_at: "2026-06-15T08:01:00Z",
        details: { from: "in_progress", to: "in_review" },
      }),
    ]);

    expect(merged).toEqual([]);
  });

  it("keeps independently actionable owner records for the same issue", () => {
    const merged = deduplicateInboxItems([
      item({
        id: "owner-decision",
        type: "workflow_owner_decision",
        created_at: "2026-06-15T08:00:00Z",
        details: { workflow_event_id: "event-1" },
      }),
      item({
        id: "newer-progress",
        type: "status_changed",
        created_at: "2026-06-15T08:02:00Z",
        details: { owner_relevant: "true", owner_category: "update" },
      }),
    ]);

    expect(merged.map((entry) => entry.id)).toEqual([
      "newer-progress",
      "owner-decision",
    ]);
  });
});

describe("getInboxDisplayTitle", () => {
  it("uses the same stable quota title as web instead of backend fallback copy", () => {
    expect(
      getInboxDisplayTitle(
        item({
          issue_id: null,
          type: "autopilot_quota_exceeded",
          title: "Autopilot quota exceeded (100/100)",
        }),
      ),
    ).toBe("Autopilot run limit reached");
  });

  it("keeps paused autopilot copy on the server fallback path", () => {
    expect(
      getInboxDisplayTitle(
        item({
          issue_id: null,
          type: "autopilot_paused",
          title: "Paused after repeated failures",
        }),
      ),
    ).toBe("Paused after repeated failures");
  });
});

describe("getInboxNavigationTarget", () => {
  it("opens issue-less quota notices in their workspace sheet", () => {
    expect(
      getInboxNavigationTarget(
        item({ issue_id: null, type: "autopilot_quota_exceeded" }),
        "acme",
        "history-1",
      ),
    ).toEqual({
      pathname: "/[workspace]/inbox/[id]",
      params: { workspace: "acme", id: "inbox-1" },
    });
  });

  it("opens owner work in the bounded inbox screen and legacy noise in its issue", () => {
    expect(
      getInboxNavigationTarget(item({ type: "workflow_owner_decision" }), "acme", "history-1"),
    ).toMatchObject({
      pathname: "/[workspace]/inbox/[id]",
      params: { workspace: "acme", id: "inbox-1" },
    });
    expect(getInboxNavigationTarget(item({}), "acme", "history-1")).toMatchObject({
      pathname: "/[workspace]/issue/[id]",
    });
    expect(
      getInboxNavigationTarget(
        item({ issue_id: null, type: "autopilot_paused" }),
        "acme",
        "history-1",
      ),
    ).toEqual({
      pathname: "/[workspace]/inbox/[id]",
      params: { workspace: "acme", id: "inbox-1" },
    });
  });
});

describe("getAutopilotQuotaBody", () => {
  it("formats the machine reset timestamp for the device locale", () => {
    const body = getAutopilotQuotaBody(
      item({
        issue_id: null,
        type: "autopilot_quota_exceeded",
        body: "Raw fallback 2026-09-01T00:00:00Z",
        details: {
          limit: "100",
          reset_at: "2026-09-01T00:00:00Z",
          autopilot_title: "Daily triage",
        },
      }),
    );

    expect(body).toContain("Daily triage");
    expect(body).toContain("limit of 100 runs");
    expect(body).not.toContain("2026-09-01T00:00:00Z");
  });

  it("keeps the server fallback when structured facts are incomplete", () => {
    expect(
      getAutopilotQuotaBody(
        item({
          issue_id: null,
          type: "autopilot_quota_exceeded",
          body: "Readable server fallback",
          details: { limit: "100" },
        }),
      ),
    ).toBe("Readable server fallback");
  });
});
