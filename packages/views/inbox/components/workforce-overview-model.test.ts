import { expect, it } from "vitest";
import type { Issue, IssueCurrentPictureCompact } from "@multica/core/types";
import { workforceItem } from "./workforce-overview-model";

function issue(picture?: Partial<IssueCurrentPictureCompact>): Issue {
  return {
    id: "issue-1",
    workspace_id: "workspace-1",
    number: 98,
    identifier: "SILV-98",
    title: "Build Multica AI Workforce HQ",
    description: null,
    status: "todo",
    priority: "high",
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
    created_at: "2026-10-01T00:00:00Z",
    updated_at: "2026-10-15T00:00:00Z",
    ...(picture ? { current_picture: picture as IssueCurrentPictureCompact } : {}),
  };
}

function picture(overrides: Partial<IssueCurrentPictureCompact> = {}): Partial<IssueCurrentPictureCompact> {
  return {
    contract_version: 2,
    owner_summary: {
      schema_version: 1,
      current_picture_revision: 1,
      stage: "active",
      signals: [],
      next_action: null,
      latest_meaningful_change: null,
    },
    revision: 1,
    scope_revision: 1,
    authority_revision: 1,
    outcome_state: "open",
    outcome_open: true,
    execution_state: "working",
    release_state: "pending",
    health_state: "unknown",
    attention_state: "resolved",
    current_milestone: "working",
    latest_meaningful_change: {},
    next_automatic_action: {},
    active_actor: {},
    wait_reason: {},
    candidate: {},
    target: {},
    evidence_summary: {},
    unresolved_decisions: [],
    unresolved_incidents: [],
    unresolved_dependencies: [],
    terminal_reason: "",
    next_actor: {},
    generated_at: "2026-10-15T21:15:47Z",
    ...overrides,
  };
}

it("does not label active work stopped when a different branch is waiting", () => {
  const result = workforceItem(issue(picture({
    owner_summary: {
      schema_version: 1,
      current_picture_revision: 1,
      stage: "blocked",
      signals: [],
      next_action: null,
      latest_meaningful_change: null,
    },
    execution_state: "working",
    unresolved_dependencies: [{ state: "blocked_external" }],
  })));
  expect(result.state).toBe("working");
  expect(result.context).toContain("Another part");
});

it("translates the known outside-work test wording into owner language", () => {
  const result = workforceItem(issue(picture({
    latest_meaningful_change: {
      external_work: {
        provider: "claude",
        reported_state: "working",
        change_since_last_receipt: "A synthetic Claude-format receipt was emitted for deterministic ingestion testing.",
        next_action: "Independently verify the receipt appears once and does not complete the issue.",
      },
    },
  })));
  expect(result.latest).toBe("Multica received a test update from Claude and recorded it successfully.");
  expect(result.next).toBe("Check that the update appears once and does not mark the work finished.");
});

it("keeps missing evidence visibly unknown", () => {
  const result = workforceItem(issue());
  expect(result.state).toBe("no_update");
  expect(result.latest).toContain("No recent progress update");
});

it("does not accept an outside completion report as company completion", () => {
  const result = workforceItem(issue(picture({
    execution_state: "complete",
    latest_meaningful_change: {
      external_work: {
        provider: "claude",
        reported_state: "complete",
        change_since_last_receipt: "The worker says the change is complete.",
        next_action: "Verify it in production.",
      },
    },
  })));
  expect(result.state).toBe("reported_done");
  expect(result.context).toContain("has not accepted");
});
