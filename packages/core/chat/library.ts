import { z } from "zod";

export const ConversationLibrarySchema = z.object({
  grouping: z.enum(["recent", "project", "custom"]).default("recent"),
  filter: z.enum(["all", "unread", "working", "needs_you"]).default("all"),
  groups: z.array(z.object({ id: z.string(), name: z.string().min(1).max(60), sessionIds: z.array(z.string()) })).default([]),
});
export type ConversationLibrary = z.infer<typeof ConversationLibrarySchema>;
export const SavedConversationLibrarySchema = ConversationLibrarySchema.extend({ revision: z.number().int().nonnegative() });
export type SavedConversationLibrary = z.infer<typeof SavedConversationLibrarySchema>;
export type ConversationLibraryChange = { action: "set_grouping" | "set_filter" | "create_group" | "rename_group" | "delete_group" | "move_chat" | "import"; id?: string; name?: string; session_id?: string; value?: string; library?: ConversationLibrary };
export const emptyLibrary = (): ConversationLibrary => ({ grouping: "recent", filter: "all", groups: [] });
export function readLibrary(raw: string | null): ConversationLibrary {
  try { const result = ConversationLibrarySchema.safeParse(JSON.parse(raw ?? "{}")); return result.success ? result.data : emptyLibrary(); }
  catch { return emptyLibrary(); }
}
export function moveToGroup(library: ConversationLibrary, sessionId: string, groupId: string | null): ConversationLibrary {
  return { ...library, groups: library.groups.map(group => ({ ...group, sessionIds: [...group.sessionIds.filter(id => id !== sessionId), ...(group.id === groupId ? [sessionId] : [])] })) };
}
