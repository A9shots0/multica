/**
 * Inbox title display helpers.
 *
 * Mirrors packages/views/inbox/components/inbox-display.ts. Keeping behavior
 * identical is required by apps/mobile/CLAUDE.md "Behavioral parity":
 * the title a user sees in the mobile inbox MUST match what they see on
 * web for the same item. When the web version changes, sync this file.
 */
import type { InboxItem } from "@multica/core/types";
import {
  isOwnerRelevantInboxItem,
  ownerInboxCategory,
} from "@multica/core/inbox/owner-control";

export { isOwnerRelevantInboxItem, ownerInboxCategory };

function formatResetAt(value: string | undefined): string {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date);
}

export function getAutopilotQuotaBody(item: InboxItem): string | null {
  if (item.type !== "autopilot_quota_exceeded") return item.body;
  const details = item.details ?? {};
  const resetAt = formatResetAt(details.reset_at);
  if (!details.limit || !resetAt) return item.body;
  if (details.autopilot_title) {
    return `Autopilot “${details.autopilot_title}” was not started because this workspace has reached its limit of ${details.limit} runs for the current period. The allowance resets ${resetAt}.`;
  }
  return `This workspace has reached its limit of ${details.limit} autopilot runs for the current period. This execution was not started. The allowance resets ${resetAt}.`;
}

function singleLine(value: string | null | undefined): string {
  return (value ?? "").replace(/\s+/g, " ").trim();
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

export function stripQuickCreatePrefix(
  title: string,
  identifier?: string,
): string {
  const normalized = singleLine(title);
  if (!normalized) return "";
  if (identifier) {
    const exactPrefix = new RegExp(
      `^Created\\s+${escapeRegExp(identifier)}:\\s*`,
      "i",
    );
    const withoutExactPrefix = normalized.replace(exactPrefix, "");
    if (withoutExactPrefix !== normalized) return withoutExactPrefix.trim();
  }
  return normalized.replace(/^Created\s+[A-Z][A-Z0-9]*-\d+:\s*/i, "").trim();
}

export function getInboxDisplayTitle(item: InboxItem): string {
  const details = item.details ?? {};
  // Mobile is English-only today. Mirror web's localized system-notice titles
  // rather than exposing backend fallback copy that can include raw counts.
  switch (item.type) {
    case "autopilot_quota_exceeded":
      return "Autopilot run limit reached";
  }
  if (item.type === "quick_create_done") {
    const cleanedTitle = stripQuickCreatePrefix(item.title, details.identifier);
    if (cleanedTitle) return cleanedTitle;
    const prompt = singleLine(details.original_prompt);
    if (prompt) return prompt;
  }
  // Both non-success quick-create outcomes surface the user's original input
  // as the row title. Mirrors isQuickCreateOutcome in
  // packages/views/inbox/components/inbox-display.ts.
  if (item.type === "quick_create_failed" || item.type === "quick_create_unconfirmed") {
    const prompt = singleLine(details.original_prompt);
    if (prompt) return prompt;
  }
  return item.title;
}

export function getInboxNavigationTarget(
  item: InboxItem,
  workspace: string | null,
  historyToken: string,
) {
  if (!workspace) return null;
  if (isOwnerRelevantInboxItem(item) || item.type === "autopilot_paused") {
    return {
      pathname: "/[workspace]/inbox/[id]" as const,
      params: { workspace, id: item.id },
    };
  }
  if (item.issue_id) {
    return {
      pathname: "/[workspace]/issue/[id]" as const,
      params: {
        workspace,
        id: item.issue_id,
        highlight: item.details?.comment_id,
        h: historyToken,
      },
    };
  }
  return null;
}

/**
 * Build the owner-control Inbox projection.
 *
 * Raw run/status chatter is excluded. Every material owner record keeps its
 * own inbox-item identity, including multiple decisions or incidents on the
 * same issue, so each can be opened and acknowledged independently.
 */
export function deduplicateInboxItems(items: InboxItem[]): InboxItem[] {
  const active = items.filter((i) => !i.archived && isOwnerRelevantInboxItem(i));
  const groups = new Map<string, InboxItem[]>();
  for (const item of active) {
    const key = item.id;
    const group = groups.get(key) ?? [];
    group.push(item);
    groups.set(key, group);
  }
  const merged: InboxItem[] = [];
  for (const group of groups.values()) {
    group.sort(
      (a, b) =>
        new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
    );
    if (group[0]) merged.push(group[0]);
  }
  return merged.sort(
    (a, b) =>
      new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
  );
}
