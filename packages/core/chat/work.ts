import { z } from "zod";
import { InboxItemSchema } from "../api/schemas";

export const ChatWorkTargetSchema = z.object({
  issue_id: z.string(), issue_revision: z.number(), owner_id: z.string(),
  decision_id: z.string().optional(), decision_revision: z.number().optional(),
  request_event_id: z.string().optional(), intake_id: z.string().optional(),
  request_id: z.string().optional(), needs_request_selection: z.boolean().optional(),
  result_task_id: z.string().optional(), result_digest: z.string().optional(),
});
export type ChatWorkTarget = z.infer<typeof ChatWorkTargetSchema>;

/** One independently guarded answer collected into a single Chat turn. */
export const ChatWorkResponseDraftSchema = z.object({
  issue: z.lazy(() => ChatWorkSelectionSchema),
  answer: z.string(),
});
export type ChatWorkResponseDraft = z.infer<typeof ChatWorkResponseDraftSchema>;

/** Wire shape. The server freezes and assigns a durable response id. */
export interface ChatWorkResponseInput {
  target: ChatWorkTarget;
  content: string;
}

export function chatWorkResponseKey(target: ChatWorkTarget): string {
  return target.request_id || target.decision_id || target.intake_id || `${target.issue_id}:${target.issue_revision}`;
}

/** One visible composer, with an independent draft for every explicit reply. */
export function chatWorkDraftKey(sessionId: string, target: ChatWorkTarget): string {
  return `${sessionId}::work::${target.request_id || target.issue_id}`;
}

/** Suggested replies seed an empty request draft, but never overwrite or
 * duplicate words the owner has already started for that exact request. */
export function seedChatWorkDraft(existing: string | undefined, suggestion: string): string {
  return existing?.trim() ? existing : suggestion.trim();
}

/** Replace only the authored choice at the start of a request draft. */
export function selectChatWorkOption(
  existing: string | undefined,
  options: string[],
  nextOption: string,
): string {
  const next = nextOption.trim();
  const current = existing ?? "";
  const trimmed = current.trim();
  if (!trimmed) return `${next}\n\n`;

  const previous = options.find((option) => {
    const value = option.trim();
    return trimmed === value || trimmed.startsWith(`${value}\n`) || trimmed.startsWith(`${value}\r\n`);
  });
  if (!previous) return current;

  const suffix = trimmed.slice(previous.trim().length).trim();
  return suffix ? `${next}\n\n${suffix}` : `${next}\n\n`;
}
export const ChatWorkIssueSchema = z.object({
  id: z.string(), number: z.number(), title: z.string(), revision: z.number(),
  project_id: z.string().nullable(), state: z.string(), source: z.string(),
  task_id: z.string().nullable(), task_status: z.string(), wait_reason: z.string(),
  observed_at: z.string(),
  summary: z.object({ stage: z.string(), current_picture_revision: z.number() }).passthrough(),
  target: ChatWorkTargetSchema,
  pending_requests: z.array(z.object({ id: z.string(), question: z.string(), target: ChatWorkTargetSchema })).optional(),
  picture: z.record(z.string(), z.unknown()),
});
export const ChatWorkSchema = z.object({
  // Delivery uses deterministic PostgreSQL UUID identities (not necessarily a
  // versioned RFC UUID). Match the server parser without rejecting saved rows.
  catch_up: z.array(z.object({ message_id: z.string().regex(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i), issue_id: z.string(), title: z.string(), kind: z.string(), body: z.string(), occurred_at: z.string() })).optional(),
  has_more_catch_up: z.boolean().optional(),
  observed_at: z.string(), issues: z.array(ChatWorkIssueSchema), has_older: z.boolean(),
  // The server includes the canonical Inbox rows for pending owner requests.
  // Preserve authored options so Chat can offer them as editable suggested
  // replies while the normal message path keeps exact target/revision guards.
  requests: z.array(InboxItemSchema).optional(),
  activity: z.array(z.object({ id: z.string(), issue_id: z.string(), event_type: z.string(), reason: z.string(), created_at: z.string() })),
});
export type ChatWork = z.infer<typeof ChatWorkSchema>;
export type ChatWorkIssue = z.infer<typeof ChatWorkIssueSchema>;

// A user's captured composer choice, not a persisted copy of live work state.
export const ChatWorkSelectionSchema = ChatWorkIssueSchema.pick({ id: true, title: true, target: true, pending_requests: true });
export type ChatWorkSelection = z.infer<typeof ChatWorkSelectionSchema>;
export function captureChatWorkSelection(issue: ChatWorkSelection): ChatWorkSelection {
  return { id: issue.id, title: issue.title, target: { ...issue.target },
    pending_requests: issue.pending_requests?.filter(request => request.id === issue.target.request_id) };
}
export function readChatWorkSelections(raw: string | null): Record<string, ChatWorkSelection> {
  try {
    const value: unknown = JSON.parse(raw ?? "{}");
    if (!value || typeof value !== "object" || Array.isArray(value)) return {};
    const entries: Record<string, ChatWorkSelection> = {};
    for (const [key, selection] of Object.entries(value)) {
      const parsed = ChatWorkSelectionSchema.safeParse(selection);
      if (parsed.success && parsed.data.id === parsed.data.target.issue_id) entries[key] = captureChatWorkSelection(parsed.data);
    }
    return entries;
  } catch { return {}; }
}

export function readChatWorkResponseDrafts(raw: string | null): Record<string, Record<string, ChatWorkResponseDraft>> {
  try {
    const value: unknown = JSON.parse(raw ?? "{}");
    if (!value || typeof value !== "object" || Array.isArray(value)) return {};
    const sessions: Record<string, Record<string, ChatWorkResponseDraft>> = {};
    for (const [sessionId, draftValue] of Object.entries(value)) {
      if (!draftValue || typeof draftValue !== "object" || Array.isArray(draftValue)) continue;
      const drafts: Record<string, ChatWorkResponseDraft> = {};
      for (const [key, candidate] of Object.entries(draftValue)) {
        const parsed = ChatWorkResponseDraftSchema.safeParse(candidate);
        if (parsed.success && parsed.data.issue.id === parsed.data.issue.target.issue_id && key === chatWorkResponseKey(parsed.data.issue.target)) {
          drafts[key] = { issue: captureChatWorkSelection(parsed.data.issue), answer: parsed.data.answer };
        }
      }
      if (Object.keys(drafts).length) sessions[sessionId] = drafts;
    }
    return sessions;
  } catch { return {}; }
}

/** Only an authoritative disappearance of this exact question clears its
 * discussion target. A revision change or unavailable snapshot does not. */
export function chatWorkRequestResolved(selected: ChatWorkSelection | undefined, work: ChatWork | null | undefined) {
  const requestId = selected?.target.request_id;
  if (!requestId) return false;
  const current = work?.issues.find(issue => issue.id === selected.id);
  return !!current?.pending_requests && !current.pending_requests.some(request => request.id === requestId);
}

export function workCounts(work: ChatWork | null | undefined) {
  const counts = { total: 0, working: 0, needsYou: 0, waiting: 0, done: 0, stopped: 0, unavailable: 0 };
  for (const issue of work?.issues ?? []) {
    counts.total++;
    switch (issue.state) {
      case "working": case "checking": counts.working++; break;
      case "needs_you": counts.needsYou++; break;
      case "queued": case "waiting": counts.waiting++; break;
      case "done": counts.done++; break;
      case "stopped": counts.stopped++; break;
      default: counts.unavailable++;
    }
  }
  return counts;
}

export function sortWork(issues: ChatWorkIssue[]) {
  const rank: Record<string, number> = { needs_you: 0, stopped: 1, unavailable: 2, working: 3, checking: 3, queued: 4, waiting: 4, done: 5 };
  return [...issues].sort((a, b) => (rank[a.state] ?? 2) - (rank[b.state] ?? 2) || a.number - b.number);
}

export const ChatWorkIndexSchema = z.record(z.string(), z.object({working:z.boolean(),needs_you:z.boolean()}));

export type ChatWorkIndex = z.infer<typeof ChatWorkIndexSchema>;
