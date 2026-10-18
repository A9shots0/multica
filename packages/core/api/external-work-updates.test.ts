// @vitest-environment node

import { describe, expect, it } from "vitest";
import { ExternalWorkUpdatesResponseSchema } from "./schemas";

const update = {
  id: "receipt-1",
  provider: "codex",
  reported_state: "complete",
  observed_at: "2026-10-14T21:00:00Z",
  recorded_at: "2026-10-14T21:00:01Z",
  receipt_revision: 1,
  current_milestone: "Importer merged",
  summary: "The importer is merged.",
  evidence: [{ type: "pr", reference: "https://github.com/example/repo/pull/36" }],
  evidence_total: 1,
  blockers: [],
  blocker_total: 0,
  next_action: "Verify the release.",
  acceptance_stage: "merged",
  acceptance_status: "accepted",
  resource_usage: {
    source: "measured",
    attempts: 2,
    wall_clock_seconds: 180,
    active_compute_seconds: 120,
    input_tokens: 1000,
    output_tokens: 200,
    cost_usd: 0.42,
    capacity_window_ref: null,
  },
  provider_report_only: true,
};

describe("ExternalWorkUpdatesResponseSchema", () => {
  it("accepts a bounded provider report", () => {
    expect(ExternalWorkUpdatesResponseSchema.safeParse({ schema_version: 1, issue_id: "issue-1", items: [update] }).success).toBe(true);
  });

  it.each([
    ["unknown provider", { ...update, provider: "unknown" }],
    ["unbounded evidence", { ...update, evidence: Array.from({ length: 9 }, () => update.evidence[0]), evidence_total: 9 }],
    ["false native authority", { ...update, provider_report_only: false }],
    ["inconsistent total", { ...update, evidence_total: 0 }],
    ["impossible usage duration", { ...update, resource_usage: { ...update.resource_usage, active_compute_seconds: 181 } }],
  ])("rejects %s", (_name, item) => {
    expect(ExternalWorkUpdatesResponseSchema.safeParse({ schema_version: 1, issue_id: "issue-1", items: [item] }).success).toBe(false);
  });
});
