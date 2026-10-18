// @vitest-environment node
import { describe, expect, it } from "vitest";
import type { Issue, IssueCurrentPictureCompact } from "@multica/core/types";
import { buildOperationalIssueTree, issueHasNotStarted, operationalLaneForIssue } from "./operational-workflow";

// Canonical matrix for the unstarted rule. The component suite keeps the
// rendered copy regression; this file owns the boundary cases.

function issue(overrides: Partial<Issue> = {}): Issue {
  return {
    id: "issue-1",
    workspace_id: "ws-1",
    number: 41,
    identifier: "SILV-41",
    title: "Phone Farm: implement Schedule Cards build spec",
    description: null,
    status: "backlog",
    status_category: "backlog",
    priority: "none",
    assignee_type: null,
    assignee_id: null,
    creator_type: "member",
    creator_id: "owner-1",
    parent_issue_id: null,
    project_id: null,
    position: 0,
    stage: null,
    start_date: null,
    due_date: null,
    metadata: {},
    properties: {},
    created_at: "2026-09-17T23:00:00Z",
    updated_at: "2026-09-17T23:00:00Z",
    current_picture: undefined,
    ...overrides,
  } as Issue;
}

function pictureWithoutSummary(revision: number): IssueCurrentPictureCompact {
  return { revision, generated_at: "2026-09-17T23:00:00Z" } as unknown as IssueCurrentPictureCompact;
}

describe("issueHasNotStarted", () => {
  it("is true for a freshly filed backlog issue nobody has picked up", () => {
    expect(issueHasNotStarted(issue())).toBe(true);
  });

  it("is true in todo as well as backlog", () => {
    expect(issueHasNotStarted(issue({ status: "todo", status_category: "todo" }))).toBe(true);
  });

  it("is still true when assigned but nothing has run: ownership is not progress", () => {
    expect(issueHasNotStarted(issue({ assignee_type: "agent", assignee_id: "agent-1" }))).toBe(true);
  });

  it("still reads a backlog issue whose response omitted status_category", () => {
    const partial = issue();
    delete (partial as { status_category?: unknown }).status_category;
    expect(issueHasNotStarted(partial)).toBe(true);
    expect(operationalLaneForIssue(partial)).toBe("not_started");
  });

  it("does not claim active execution from raw in-progress status", () => {
    const routed = issue({ status: "in_progress", status_category: "in_progress",
                           assignee_type: "agent", assignee_id: "agent-1" });
    expect(issueHasNotStarted(routed)).toBe(false);
    expect(operationalLaneForIssue(routed)).toBe("needs_next_step");
  });

  it("uses durable review status while the controller picture is missing", () => {
    const review = issue({ status: "in_review", status_category: "in_review" });
    expect(issueHasNotStarted(review)).toBe(false);
    expect(operationalLaneForIssue(review)).toBe("in_review");
  });

  it("uses durable blocked status while the controller picture is missing", () => {
    const blocked = issue({ status: "blocked", status_category: "blocked" });
    expect(operationalLaneForIssue(blocked)).toBe("technical_stop");
  });

  it("is false for finished work, which is not waiting to start", () => {
    expect(issueHasNotStarted(issue({ status: "done", status_category: "done" }))).toBe(false);
    expect(issueHasNotStarted(issue({ status: "cancelled", status_category: "cancelled" }))).toBe(false);
  });

  it("is false when a current picture already carries an owner summary", () => {
    const picture = {
      revision: 3,
      owner_summary: { schema_version: 1, current_picture_revision: 3, stage: "active", signals: [] },
    } as unknown as IssueCurrentPictureCompact;
    expect(issueHasNotStarted(issue({ current_picture: picture }))).toBe(false);
  });
});

describe("operationalLaneForIssue", () => {
  it("puts an unstarted issue in its own lane instead of reporting a missing status", () => {
    expect(operationalLaneForIssue(issue())).toBe("not_started");
  });

  it("requires inspection when a picture exists but its summary is stale", () => {
    // A present but revision-misaligned picture is a real inability to state
    // controller status, so the durable fallback must not mask it.
    const stale = issue({
      assignee_type: "agent",
      assignee_id: "agent-1",
      status: "in_progress",
      status_category: "in_progress",
      current_picture: pictureWithoutSummary(9),
    });
    expect(operationalLaneForIssue(stale)).toBe("needs_next_step");
  });

  it("does not claim a lane for work that has genuinely started", () => {
    const picture = {
      revision: 4,
      owner_summary: {
        schema_version: 1,
        current_picture_revision: 4,
        stage: "in_review",
        signals: [],
        next_action: null,
        latest_meaningful_change: null,
      },
    } as unknown as IssueCurrentPictureCompact;
    expect(operationalLaneForIssue(issue({ current_picture: picture }))).toBe("in_review");
  });
});

describe("buildOperationalIssueTree", () => {
  it("nests loaded children under their parent in stage order", () => {
    const parent = issue({ id: "parent", number: 1, identifier: "SILV-1", position: 1 });
    const later = issue({ id: "later", number: 3, identifier: "SILV-3", parent_issue_id: "parent", stage: 2, position: 3 });
    const earlier = issue({ id: "earlier", number: 2, identifier: "SILV-2", parent_issue_id: "parent", stage: 1, position: 2 });
    const [tree] = buildOperationalIssueTree([parent, later, earlier]);
    expect(tree?.issue.id).toBe("parent");
    expect(tree?.children.map((child) => child.issue.id)).toEqual(["earlier", "later"]);
  });

  it("keeps a child visible when its parent is filtered out", () => {
    const child = issue({ id: "child", parent_issue_id: "missing" });
    const [tree] = buildOperationalIssueTree([child]);
    expect(tree?.issue.id).toBe("child");
    expect(tree?.children).toEqual([]);
  });
});
