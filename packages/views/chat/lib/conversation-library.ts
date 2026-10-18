import type { ChatSession } from "@multica/core/types";

/** Filter real session metadata; never infer a project or work state from prose. */
export function filterConversations(
  sessions: ChatSession[],
  search: string,
  filter: "all" | "unread" | "working" | "needs_you",
  workingIds: ReadonlySet<string>,
  projectNames: ReadonlyMap<string, string>,
  needsYouIds: ReadonlySet<string> = new Set(),
): ChatSession[] {
  const query = search.trim().toLocaleLowerCase();
  return sessions.filter((session) => {
    if (filter === "unread" && !session.has_unread && !(session.unread_count ?? 0)) return false;
    if (filter === "needs_you" && !needsYouIds.has(session.id)) return false;
    if (filter === "working" && !workingIds.has(session.id)) return false;
    const text = [session.title, session.project_id ? projectNames.get(session.project_id) : "", session.last_message?.content].filter(Boolean).join(" ");
    return !query || text.toLocaleLowerCase().includes(query);
  });
}

export function groupConversationsByProject(sessions: ChatSession[]) {
  const groups = new Map<string | null, ChatSession[]>();
  for (const session of sessions) {
    const key = session.project_id || null;
    const group = groups.get(key) ?? [];
    group.push(session);
    groups.set(key, group);
  }
  return Array.from(groups, ([projectId, items]) => ({ projectId, sessions: items }));
}
