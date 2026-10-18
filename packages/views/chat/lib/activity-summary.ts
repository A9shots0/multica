import type { ChatTimelineItem } from "@multica/core/chat";

export type ActivityKind = "commands" | "reading" | "searching" | "editing" | "web" | "other";

const kinds: Record<string, ActivityKind> = {
  bash: "commands", exec: "commands", exec_command: "commands", powershell: "commands",
  read: "reading", read_file: "reading", glob: "reading",
  grep: "searching", search: "searching",
  write: "editing", edit: "editing", multi_edit: "editing", multiedit: "editing", apply_patch: "editing", patch_apply: "editing",
  web_search: "web", websearch: "web", web_fetch: "web",
};

/** Factual categories of tool requests, never inferred completion or success.
 * Do not read command text: quoted scripts or data can describe an operation
 * that was never executed. Unknown tools retain a neutral category. */
export function summarizeActivity(items: ChatTimelineItem[]) {
  const counts = new Map<ActivityKind, number>();
  for (const item of items) {
    if (item.type !== "tool_use") continue;
    const tool = (item.tool ?? "").toLowerCase().split(".").at(-1) ?? "";
    const kind = kinds[tool] ?? "other";
    counts.set(kind, (counts.get(kind) ?? 0) + 1);
  }
  return Array.from(counts, ([kind, count]) => ({ kind, count }));
}
