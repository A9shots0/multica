import { externalWorkSummaryFromPicture } from "@multica/core/issues";
import type { Issue } from "@multica/core/types";

export type WorkforceState =
  | "needs_you"
  | "stopped"
  | "working"
  | "waiting"
  | "reported_done"
  | "no_update";

export interface WorkforceItem {
  issue: Issue;
  state: WorkforceState;
  label: string;
  latest: string;
  next: string;
  context: string | null;
  source: string | null;
  updatedAt: string;
}

const STATE_LABEL: Record<WorkforceState, string> = {
  needs_you: "Needs you",
  stopped: "Stopped",
  working: "Working",
  waiting: "Waiting",
  reported_done: "Reported done",
  no_update: "No recent update",
};

const STATE_ORDER: Record<WorkforceState, number> = {
  needs_you: 0,
  stopped: 1,
  working: 2,
  waiting: 3,
  reported_done: 4,
  no_update: 5,
};

const PRIORITY_ORDER: Record<Issue["priority"], number> = {
  urgent: 0,
  high: 1,
  medium: 2,
  low: 3,
  none: 4,
};

function hasKeys(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value) && Object.keys(value).length > 0;
}

function automaticAction(value: unknown): string | null {
  if (!hasKeys(value)) return null;
  const expected = hasKeys(value.expected_event) ? value.expected_event : null;
  const kind = typeof expected?.type === "string" ? expected.type : "";
  if (kind === "provider_readback") return "Wait for the outside worker's next update, then verify it.";
  if (kind === "deployment") return "Verify the release in the live product.";
  if (kind === "review") return "Complete the independent review.";
  return "Continue with the next recorded step.";
}

function ownerFriendlyUpdate(value: string): string {
  const text = value.trim();
  const testReceipt = text.match(/^A synthetic ([A-Za-z]+)-format receipt was emitted for deterministic ingestion testing\.?$/i);
  if (testReceipt) {
    return `Multica received a test update from ${testReceipt[1]} and recorded it successfully.`;
  }
  if (/^Independently verify the receipt appears once and does not complete the issue\.?$/i.test(text)) {
    return "Check that the update appears once and does not mark the work finished.";
  }
  return text;
}

/**
 * Converts workflow evidence into the small set of states the owner needs.
 * A missing Current Picture stays unknown. Outside-provider claims remain
 * reports and never become native completion.
 */
export function workforceItem(issue: Issue): WorkforceItem {
  const picture = issue.current_picture;
  const outside = externalWorkSummaryFromPicture(picture);
  const stage = picture?.owner_summary.stage;
  const execution = picture?.execution_state;
  const dependencies = picture?.unresolved_dependencies ?? [];

  let state: WorkforceState;
  if (
    issue.owner_question ||
    stage === "needs_owner" ||
    outside?.reportedState === "waiting_owner"
  ) {
    state = "needs_you";
  } else if (outside?.reportedState === "complete" && picture?.outcome_open !== false) {
    state = "reported_done";
  } else if (
    stage === "technical_stop" ||
    execution === "stopped" ||
    execution === "failed" ||
    outside?.reportedState === "stopped"
  ) {
    state = "stopped";
  } else if (execution === "working" || outside?.reportedState === "working") {
    state = "working";
  } else if (
    stage === "blocked" ||
    execution === "blocked" ||
    execution === "waiting_external" ||
    outside?.reportedState === "waiting_external" ||
    outside?.reportedState === "blocked" ||
    dependencies.length > 0
  ) {
    state = "waiting";
  } else if (picture) {
    state = "waiting";
  } else {
    state = "no_update";
  }

  const latest = (outside?.summary ? ownerFriendlyUpdate(outside.summary) : "") ||
    (picture?.current_milestone && picture.current_milestone !== "working"
      ? picture.current_milestone
      : state === "working"
        ? "Work is active, but no clearer progress note has been recorded yet."
        : "No recent progress update has been recorded.");

  const next = issue.owner_question?.question || (outside?.nextAction ? ownerFriendlyUpdate(outside.nextAction) : "") ||
    automaticAction(picture?.next_automatic_action) ||
    (state === "no_update"
      ? "Multica needs a fresh update before it can say what happens next."
      : state === "stopped"
        ? "The worker needs to recover or explain what is blocking it."
        : "Wait for the next meaningful update.");

  const context = state === "working" && dependencies.length > 0
    ? "Another part of this work is waiting for an outside update."
    : state === "reported_done"
      ? "The outside worker says it finished. Multica has not accepted the final result yet."
      : null;

  return {
    issue,
    state,
    label: STATE_LABEL[state],
    latest,
    next,
    context,
    source: outside ? `Reported by ${outside.provider === "chatgpt" ? "ChatGPT" : outside.provider[0]!.toUpperCase() + outside.provider.slice(1)}` : null,
    updatedAt: picture?.generated_at || issue.last_activity_at || issue.updated_at,
  };
}

export function workforceItems(issues: readonly Issue[]): WorkforceItem[] {
  return issues
    .map(workforceItem)
    .sort((a, b) =>
      STATE_ORDER[a.state] - STATE_ORDER[b.state] ||
      PRIORITY_ORDER[a.issue.priority] - PRIORITY_ORDER[b.issue.priority] ||
      Date.parse(b.updatedAt) - Date.parse(a.updatedAt),
    );
}
