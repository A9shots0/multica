import type { ChatMessage } from "@multica/core/types";
import type { ChatTimelineItem } from "@multica/core/chat";

/** The persisted answer identifies a real message boundary that older streams
 * lost when commentary and final text were coalesced. Never guess boundaries
 * between ordinary token fragments or alter an unmatched/partial answer. */
export function separateFinalAnswer(items: ChatTimelineItem[], answer?: string): ChatTimelineItem[] {
  if (!answer) return items;
  const last = items.at(-1);
  const content = last?.content;
  if (last?.type !== "text" || !content?.endsWith(answer) || content === answer) return items;
  const prefix = content.slice(0, -answer.length);
  if (!prefix.trim() || /\n\s*\n$/.test(prefix)) return items;
  return [...items.slice(0, -1), { ...last, content: `${prefix.trimEnd()}\n\n${answer}` }];
}

/**
 * Split an assistant timeline into three regions for the conductor-style fold:
 *   preface — text items before the first thinking/tool/error item
 *   middle  — everything from the first to the last non-text item (inclusive),
 *             including any text items sandwiched between them
 *   final   — text items after the last non-text item
 *
 * UI renders preface above the outer fold, middle inside the fold (with each
 * row keeping its existing inner Collapsible), and final below the fold.
 * Copy concatenates preface + final — the fold's contents are intentionally
 * omitted, mirroring what's visible when the fold is closed.
 */
export function splitTimeline(items: ChatTimelineItem[]): {
  preface: ChatTimelineItem[];
  middle: ChatTimelineItem[];
  final: ChatTimelineItem[];
} {
  const firstNonTextIdx = items.findIndex((i) => i.type !== "text");
  if (firstNonTextIdx === -1) {
    return { preface: [], middle: [], final: items };
  }
  let lastNonTextIdx = items.length - 1;
  while (lastNonTextIdx >= 0 && items[lastNonTextIdx]!.type === "text") {
    lastNonTextIdx--;
  }
  return {
    preface: items.slice(0, firstNonTextIdx),
    middle: items.slice(firstNonTextIdx, lastNonTextIdx + 1),
    final: items.slice(lastNonTextIdx + 1),
  };
}

/**
 * Markdown source the Copy action puts on the clipboard. By design this is
 * the user-visible answer only — anything inside the outer fold (thinking,
 * tool calls, sandwiched intermediate text) is dropped. Falls back to
 * `message.content` for legacy messages without a timeline and for the
 * pathological all-non-text shape so Copy never produces an empty string.
 */
export function extractCopyText(
  message: ChatMessage,
  timeline: ChatTimelineItem[],
): string {
  if (timeline.length === 0) return message.content ?? "";
  const { preface, final } = splitTimeline(timeline);
  const pieces = [...preface, ...final]
    .map((i) => i.content ?? "")
    .filter((s) => s.length > 0);
  if (pieces.length === 0) return message.content ?? "";
  return pieces.join("\n\n");
}
