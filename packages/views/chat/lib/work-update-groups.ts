import type { ChatMessage } from "@multica/core/types";

export type ChatHistoryRow =
  | { key: string; kind: "message"; message: ChatMessage; taskId: string | null }
  | { key: string; kind: "work_updates"; messages: ChatMessage[]; taskId: null };

// A session may contain ordinary conversation between two deliveries for the
// same issue. Group-local deduplication cannot see across that interruption,
// so decide which visual result is current from the complete message history.
export function latestVisualReviewMessageIds(messages: ChatMessage[]): Set<string> {
  const latestByIssue = new Map<string, string>();
  for (const message of messages) {
    const update = message.work_update;
    if (
      message.message_kind === "work_update" &&
      (update?.kind === "result" || update?.kind === "stopped") &&
      (update.review?.artifacts.length ?? 0) > 0
    ) {
      latestByIssue.set(update.issue_id, message.id);
    }
  }
  return new Set(latestByIssue.values());
}

export function latestQuestionMessageIds(messages: ChatMessage[]): Set<string> {
  const latestByRequest = new Map<string, string>();
  for (const message of messages) {
    const update = message.work_update;
    if (message.message_kind === "work_update" && update?.kind === "question") {
      latestByRequest.set(update.request_id ?? update.issue_id, message.id);
    }
  }
  return new Set(latestByRequest.values());
}

// Keep every event as evidence, but render a burst of background events once.
// Ordinary messages always end a group, preserving conversational chronology.
export function groupChatHistory(messages: ChatMessage[]): ChatHistoryRow[] {
  const rows: ChatHistoryRow[] = [];
  for (const message of messages) {
    if (message.message_kind === "onboarding_kickoff") continue;
    if (message.message_kind === "work_update" && message.work_update) {
      const previous = rows.at(-1);
      if (previous?.kind === "work_updates") previous.messages.push(message);
      else rows.push({ key: message.id, kind: "work_updates", messages: [message], taskId: null });
    } else {
      rows.push({ key: message.role === "assistant" && message.task_id ? `task:${message.task_id}` : message.id, kind: "message", message, taskId: message.task_id });
    }
  }
  return rows;
}

// Virtual indexes count rendered groups, not raw messages. Anchor a surviving
// row when a fetched history page merges with the first existing event group.
export function anchoredFirstIndex(previous: { keys: string[]; first: number }, keys: string[], fallback: number): number {
  const positions = new Map(keys.map((key, index) => [key, index]));
  for (let index = 0; index < previous.keys.length; index++) {
    const next = positions.get(previous.keys[index]!);
    if (next !== undefined) return Math.max(0, previous.first + index - next);
  }
  return fallback;
}
