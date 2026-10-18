"use client";

import { useEffect, useMemo, useState } from "react";
import "./chat-library.css";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Archive,
  ArchiveRestore,
  ChevronLeft,
  ChevronRight,
  Clock,
  Loader2,
  Pin,
  PinOff,
  Pencil,
  Square,
  Trash2,
  Search,
  Folder,
  FolderCog,
} from "lucide-react";
import type { ConversationLibraryChange } from "@multica/core/chat/library";
import { useConversationLibrary } from "@multica/core/chat/library-queries";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "@multica/ui/components/ui/dialog";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { cn } from "@multica/ui/lib/utils";
import { useWorkspaceId } from "@multica/core/hooks";
import { paths, useWorkspaceSlug } from "@multica/core/paths";
import { useWorkspacePresenceMap } from "@multica/core/agents";
import { api } from "@multica/core/api";
import {
  pendingChatTasksOptions,
  chatWorkIndexOptions,
  chatKeys,
  sessionActivityTime,
  sortChatSessions,
} from "@multica/core/chat/queries";
import {
  useDeleteChatSession,
  useSetChatSessionArchived,
  useSetChatSessionPinned,
  useUpdateChatSession,
} from "@multica/core/chat/mutations";
import { useChatStore } from "@multica/core/chat";
import type { Agent, ChatSession, PendingChatTasksResponse } from "@multica/core/types";
import { projectListOptions } from "@multica/core/projects/queries";
import {
  RowActionsMenu,
  handleRowActivationKey,
  type RowActionItem,
} from "../../common/row-actions-menu";
import { resolveClickIntent, useOptionalNavigation } from "../../navigation";
import { createLogger } from "@multica/core/logger";
import { removeChatMessageFromCaches } from "@multica/core/realtime";
import { useLocale, useT } from "../../i18n";
import { chatTaskIsRunning } from "../lib/task-activity";

import { filterConversations, groupConversationsByProject } from "../lib/conversation-library";

const apiLogger = createLogger("chat.api");

/** Alternate Recent grouping uses the session's last activity. */
const DATE_BUCKETS = ["today", "yesterday", "week", "older"] as const;
type DateBucket = (typeof DATE_BUCKETS)[number];

function dateBucketOf(epochMs: number, now: Date): DateBucket {
  const d = new Date(epochMs);
  const startOfDay = (date: Date) => new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime();
  const todayStart = startOfDay(now);
  const dayStart = startOfDay(d);
  const daysAgo = Math.round((todayStart - dayStart) / 86_400_000);
  if (daysAgo <= 0) return "today";
  if (daysAgo === 1) return "yesterday";
  if (daysAgo <= 7) return "week";
  return "older";
}

// IM-style timestamp: today → clock, this year → M/D, else full date.
function formatChatTime(dateStr: string, locale: string): string {
  const d = new Date(dateStr);
  const now = new Date();
  if (d.toDateString() === now.toDateString()) {
    return d.toLocaleTimeString(locale, { hour: "2-digit", minute: "2-digit" });
  }
  if (d.getFullYear() === now.getFullYear()) {
    return d.toLocaleDateString(locale, { month: "numeric", day: "numeric" });
  }
  return d.toLocaleDateString(locale);
}

/**
 * IM-style conversation list: each row is agent avatar + session title + agent
 * name + last-message preview + time, with a red unread *count* badge. An
 * in-flight agent shows a "typing…" indicator; a failed last reply shows a
 * destructive hint. Rows are rendered in the server's order (most-recent
 * activity first). Renaming lives in the conversation header's ⋯ menu, not
 * here.
 *
 * Two views, toggled locally: the default "history" view lists active chats and
 * hovering a row reveals pin + archive (or stop, while running) — archiving is
 * the reversible, one-click default so nothing is destroyed by accident. A
 * footer entry ("Archived · N") switches to the "archived" view, which lists
 * archived chats and is the ONLY place a chat can be hard-deleted (hover →
 * unarchive + delete). Both views read the same flat sessions cache and split
 * on `status` locally.
 */
export function ChatThreadList({
  sessions,
  agents,
  activeSessionId,
  onSelectSession,
  onArchive,
}: {
  sessions: ChatSession[];
  agents: Agent[];
  activeSessionId: string | null;
  onSelectSession: (session: ChatSession) => void;
  // Archiving is owned by the parent so the selection advance stays layout-
  // aware (desktop advances to the next chat; mobile drops back to the list)
  // and routes through the shared controller — see ChatPage.handleArchive.
  onArchive: (session: ChatSession) => void;
}) {
  const { t } = useT("chat");
  const locale = useLocale();
  const wsId = useWorkspaceId();
  // Null-safe slug (not useWorkspacePaths, which throws): the list renders in
  // tests outside a workspace route; without a slug the web modifier-click
  // affordance simply stays off.
  const slug = useWorkspaceSlug();
  const sessionHref = (sessionId: string) =>
    slug ? `${paths.workspace(slug).chat()}?session=${sessionId}` : null;
  // Optional: the list renders bare in tests; without an adapter the web
  // modifier-click affordance stays off (desktop keeps selection anyway).
  const navigation = useOptionalNavigation();
  const openInNewTab = navigation?.openInNewTab;
  const getShareableUrl = navigation?.getShareableUrl;
  const agentById = useMemo(() => new Map(agents.map((a) => [a.id, a])), [agents]);

  const [search, setSearch] = useState("");
  const legacyLibrary = useChatStore(s => s.library);
  const organization = useConversationLibrary(wsId, legacyLibrary);
  const { library } = organization;
  const changeLibrary = (change: ConversationLibraryChange, done?: () => void) => { void organization.change(change).then(done).catch(() => {}); };
  const { filter, grouping } = library;
  const setFilter = (filter: typeof library.filter) => changeLibrary({ action: "set_filter", value: filter });
  const setGrouping = (grouping: typeof library.grouping) => changeLibrary({ action: "set_grouping", value: grouping });
  const [manageGroups, setManageGroups] = useState(false);
  const [movingSession, setMovingSession] = useState<ChatSession | null>(null);
  const [newGroup, setNewGroup] = useState("");
  const { data: projects = [] } = useQuery(projectListOptions(wsId));
  const projectById = useMemo(() => new Map(projects.map((p) => [p.id, p])), [projects]);
  const projectNames = useMemo(() => new Map(projects.map((p) => [p.id, p.title])), [projects]);
  const { data: pending } = useQuery(pendingChatTasksOptions(wsId));
  const workIndex = useQuery(chatWorkIndexOptions(wsId));
  const index = workIndex.isError ? {} : workIndex.data ?? {};
  const workingIds = new Set(Object.entries(index).filter(([,flags]) => flags.working).map(([id]) => id));
  const needsYouIds = new Set(Object.entries(index).filter(([,flags]) => flags.needs_you).map(([id]) => id));
  const filteredSessions = filterConversations(sessions, search, filter, workingIds, projectNames, needsYouIds);

  // Split the flat cache locally: active chats fill the default history view,
  // archived chats fill the "Archived" view. Both sorted pinned-first (then by
  // activity) so the list stays ordered even after an optimistic pin/archive or
  // a WS patch mutates the flat cache in place.
  const historySessions = useMemo(
    () => sortChatSessions(filteredSessions.filter((s) => s.status !== "archived")),
    [filteredSessions],
  );
  // Pinned sessions already sorted first within `historySessions` — but
  // sitting first in one long list isn't the same as being visibly set apart,
  // which was the actual complaint: a pin only ever showed as a small icon
  // next to a title that otherwise looked like every unpinned row around it.
  const pinnedSessions = useMemo(
    () => historySessions.filter((s) => s.pinned),
    [historySessions],
  );
  const unpinnedSessions = useMemo(
    () => historySessions.filter((s) => !s.pinned),
    [historySessions],
  );
  // Grouped once per render of the list, not per row — bucketing against a
  // single `now` so a session can't read as "Today" in one row's clock and
  // "Yesterday" in the group it actually landed in.
  const dateGroups = useMemo(() => {
    const now = new Date();
    const buckets = new Map<DateBucket, ChatSession[]>();
    for (const session of unpinnedSessions) {
      const bucket = dateBucketOf(sessionActivityTime(session), now);
      const list = buckets.get(bucket);
      if (list) list.push(session);
      else buckets.set(bucket, [session]);
    }
    return DATE_BUCKETS.map((key) => ({ key, sessions: buckets.get(key) ?? [] })).filter(
      (group) => group.sessions.length > 0,
    );
  }, [unpinnedSessions]);
  const DATE_BUCKET_LABEL: Record<DateBucket, string> = {
    today: t(($) => $.list.date_today),
    yesterday: t(($) => $.list.date_yesterday),
    week: t(($) => $.list.date_previous_week),
    older: t(($) => $.list.date_older),
  };
  const archivedSessions = useMemo(
    () => sortChatSessions(sessions.filter((s) => s.status === "archived")),
    [sessions],
  );

  // Which view is showing. Falls back to history when the archived list drains
  // (last chat unarchived / deleted) so we never strand the user on an empty
  // archive.
  const [view, setView] = useState<"history" | "archived">("history");
  useEffect(() => {
    if (view === "archived" && archivedSessions.length === 0) setView("history");
  }, [view, archivedSessions.length]);

  const [confirmingDeleteId, setConfirmingDeleteId] = useState<string | null>(null);
  const [confirmingStopId, setConfirmingStopId] = useState<string | null>(null);
  const [stoppingTaskId, setStoppingTaskId] = useState<string | null>(null);
  const [editingSessionId, setEditingSessionId] = useState<string | null>(null);
  const [editingTitle, setEditingTitle] = useState("");
  const deleteSession = useDeleteChatSession();
  const updateSession = useUpdateChatSession();
  const setPinned = useSetChatSessionPinned();
  const setArchived = useSetChatSessionArchived();
  const setActiveSession = useChatStore((s) => s.setActiveSession);
  const queryClient = useQueryClient();

  const pendingTaskBySessionId = useMemo(
    () => new Map((pending?.tasks ?? []).map((task) => [task.chat_session_id, task])),
    [pending],
  );

  // Per-agent presence, so a pending task on an OFFLINE agent shows "waiting"
  // rather than a misleading "typing…" (the task is queued until the agent is
  // back). Same availability source the conversation pane's status pill uses,
  // keeping the two surfaces consistent.
  const presence = useWorkspacePresenceMap(wsId);

  useEffect(() => {
    if (!confirmingStopId || pendingTaskBySessionId.has(confirmingStopId)) return;
    setConfirmingStopId(null);
  }, [confirmingStopId, pendingTaskBySessionId]);

  const handleConfirmDelete = (session: ChatSession) => {
    const sessionId = session.id;
    if (activeSessionId === sessionId) setActiveSession(null);
    deleteSession.mutate(sessionId, {
      onSettled: () => setConfirmingDeleteId(null),
    });
  };

  const handleStartRename = (session: ChatSession, e?: React.MouseEvent) => {
    e?.stopPropagation();
    setEditingSessionId(session.id);
    setEditingTitle(session.title ?? "");
  };

  const handleSaveRename = (sessionId: string) => {
    const trimmed = editingTitle.trim();
    if (trimmed) {
      updateSession.mutate({ sessionId, title: trimmed });
    }
    setEditingSessionId(null);
  };

  const handleConfirmStop = (
    session: ChatSession,
    task: PendingChatTasksResponse["tasks"][number],
  ) => {
    setStoppingTaskId(task.task_id);
    queryClient.setQueryData<PendingChatTasksResponse>(chatKeys.pendingTasks(wsId), (current) => {
      if (!current) return current;
      return {
        ...current,
        tasks: current.tasks.filter((item) => item.task_id !== task.task_id),
      };
    });
    queryClient.setQueryData(chatKeys.pendingTask(session.id), {});
    queryClient.invalidateQueries({ queryKey: chatKeys.messages(session.id) });
    queryClient.invalidateQueries({ queryKey: chatKeys.messagesPage(session.id) });

    api.cancelTaskById(task.task_id).then(
      (result) => {
        const restored = result.cancelled_chat_message;
        if (restored?.restore_to_input) {
          removeChatMessageFromCaches(queryClient, restored.chat_session_id, restored.message_id);
        }
        apiLogger.info("cancelTask.success (list row)", { taskId: task.task_id, sessionId: session.id });
      },
      (err) =>
        apiLogger.warn("cancelTask.error (list row; task may have already finished)", {
          taskId: task.task_id,
          sessionId: session.id,
          err,
        }),
    ).finally(() => {
      queryClient.invalidateQueries({ queryKey: chatKeys.pendingTasks(wsId) });
      queryClient.invalidateQueries({ queryKey: chatKeys.pendingTask(session.id) });
      setStoppingTaskId(null);
      setConfirmingStopId(null);
    });
  };

  const renderRow = (session: ChatSession) => {
    const isCurrent = session.id === activeSessionId;
    const agent = agentById.get(session.agent_id) ?? null;
    const pendingTask = pendingTaskBySessionId.get(session.id);
    const isRunning = !!pendingTask;
    // Pending controls cancellation; only running status claims execution.
    const agentOffline = agent
      ? presence.byAgent.get(agent.id)?.availability === "offline"
      : false;
    const unread = isCurrent ? 0 : (session.unread_count ?? 0);
    const isConfirmingDelete = confirmingDeleteId === session.id;
    const isConfirmingStop = confirmingStopId === session.id && !!pendingTask;
    const isConfirmingAction = isConfirmingDelete || isConfirmingStop;
    const titleText = session.title?.trim() || t(($) => $.window.untitled);
    const last = session.last_message ?? null;
    const timeText = last
      ? formatChatTime(last.created_at, locale)
      : formatChatTime(session.updated_at, locale);

    // The real relationship, not a guess: `project_id` is a stored field a
    // chat is actually bound to (the same one the composer's project picker
    // sets), not a regex scanning the title or last message for something
    // that looks like an issue code.
    const project = session.project_id ? projectById.get(session.project_id) : null;

    // The second line carries state worth a glance — typing, waiting on an
    // offline agent, failed — never a preview of what was actually said. A
    // sentence of the last message here read as noise on every row in a list
    // meant to be scanned, not read; the real content is one click away in
    // the conversation itself.
    let previewNode: React.ReactNode = null;
    if (isRunning && (agentOffline || !chatTaskIsRunning(pendingTask.status))) {
      // Task is queued but the agent is offline — it will run once the agent
      // is back. Show a static "waiting", not an animated "typing".
      previewNode = (
        <span className="flex min-w-0 items-center gap-1.5 text-muted-foreground">
          <Clock className="size-3 shrink-0" />
          <span className="truncate">{!agentOffline && pendingTask.status === "queued"
            ? t(($) => $.status_pill.stages.queued)
            : t(($) => $.list.waiting)}</span>
        </span>
      );
    } else if (isRunning) {
      previewNode = (
        <span className="flex min-w-0 items-center gap-1.5 text-emerald-500">
          <Loader2 className="size-3 shrink-0 animate-spin" />
          <span className="truncate">{t(($) => $.list.typing)}</span>
        </span>
      );
    } else if (last?.failure_reason) {
      previewNode = <span className="block truncate text-destructive">{t(($) => $.list.failed)}</span>;
    } else if (last?.message_kind === "no_response") {
      previewNode = <span className="block truncate text-muted-foreground">{t(($) => $.list.no_response_preview)}</span>;
    }

    // One list drives both action surfaces — the compact menu without hover
    // and the hover strip with it — so they cannot drift. The archived view
    // is the only place hard-delete lives; the history view offers the
    // reversible archive instead.
    const rowActions: RowActionItem[] =
      view === "archived"
        ? [
            {
              key: "unarchive",
              icon: <ArchiveRestore className="size-3.5" />,
              label: t(($) => $.list.unarchive),
              onSelect: () =>
                setArchived.mutate({ sessionId: session.id, archived: false }),
            },
            {
              key: "delete",
              icon: <Trash2 className="size-3.5" />,
              label: t(($) => $.session_history.row_delete_aria),
              danger: true,
              onSelect: () => setConfirmingDeleteId(session.id),
            },
          ]
        : [
            {
              key: "pin",
              icon: session.pinned ? (
                <PinOff className="size-3.5" />
              ) : (
                <Pin className="size-3.5 -rotate-45" />
              ),
              label: session.pinned
                ? t(($) => $.list.unpin)
                : t(($) => $.list.pin),
              onSelect: () =>
                setPinned.mutate({ sessionId: session.id, pinned: !session.pinned }),
            },
            {
              key: "rename",
              icon: <Pencil className="size-3.5" />,
              label: t(($) => $.header.rename),
              onSelect: () => {
                setEditingSessionId(session.id);
                setEditingTitle(session.title ?? "");
              },
            },
            { key: "group", icon: <Folder className="size-3.5" />, label: t(($) => $.library.move_group), onSelect: () => setMovingSession(session) },
            isRunning
              ? {
                  key: "stop",
                  icon: <Square className="size-3 fill-current" />,
                  label: t(($) => $.session_history.row_stop_aria),
                  danger: true,
                  onSelect: () => setConfirmingStopId(session.id),
                }
              : {
                  key: "archive",
                  icon: <Archive className="size-3.5" />,
                  label: t(($) => $.list.archive),
                  onSelect: () => onArchive(session),
                },
          ];

    return (
      <div
        key={session.id}
        aria-current={isCurrent ? "true" : undefined}
        tabIndex={0}
        onClick={(e) => {
          if (isConfirmingAction || e.defaultPrevented) return;
          // Plain click keeps the master-detail selection. On web, a modifier
          // click opens the session as its own browser tab. Desktop tabs
          // dedupe chat by pathname (a session is view state, not a subject —
          // see tab-store resourceKey), so a second chat tab cannot exist
          // there; modifier clicks keep the selection behavior instead.
          const href = sessionHref(session.id);
          if (
            href &&
            getShareableUrl &&
            !openInNewTab &&
            resolveClickIntent(e) !== "push"
          ) {
            window.open(
              getShareableUrl(href),
              "_blank",
              "noopener,noreferrer",
            );
            return;
          }
          onSelectSession(session);
        }}
        onAuxClick={(e) => {
          if (isConfirmingAction || e.defaultPrevented || e.button !== 1) return;
          if (openInNewTab) return; // desktop: no second chat tab exists
          const href = sessionHref(session.id);
          if (!href || !getShareableUrl) return;
          e.preventDefault();
          window.open(getShareableUrl(href), "_blank", "noopener,noreferrer");
        }}
        onKeyDown={(e) => {
          if (isConfirmingAction) return;
          handleRowActivationKey(e, () => onSelectSession(session));
        }}
        onDoubleClick={(e) => {
          handleStartRename(session, e);
        }}
        className={cn(
          "group/row relative flex min-h-11 min-w-0 cursor-pointer items-center gap-2 rounded-md px-2 py-2 outline-none transition-colors focus-visible:ring-1 focus-visible:ring-ring",
          isCurrent ? "bg-accent" : "hover:bg-accent/40",
          isConfirmingAction && "bg-destructive/5 hover:bg-destructive/5",
        )}
      >
        <div className="min-w-0 flex-1">
          {/* Line 1: title + time. The Pinned section header above is what
              sets these rows apart — no glyph repeated on every row. */}
          <div className="flex items-center gap-1.5">
            {editingSessionId === session.id ? (
              <input
                autoFocus
                value={editingTitle}
                onChange={(e) => setEditingTitle(e.target.value)}
                onClick={(e) => e.stopPropagation()}
                onKeyDown={(e) => {
                  e.stopPropagation();
                  if (e.key === "Enter") {
                    handleSaveRename(session.id);
                  } else if (e.key === "Escape") {
                    setEditingSessionId(null);
                  }
                }}
                onBlur={() => handleSaveRename(session.id)}
                className="min-w-0 flex-1 rounded bg-background px-1.5 py-0.5 text-sm font-medium text-foreground outline-none ring-1 ring-primary shadow-xs"
              />
            ) : (
              <span className={cn("min-w-0 flex-1 truncate text-body", unread > 0 ? "font-semibold text-foreground" : "font-medium text-foreground")}>
                {titleText}
              </span>
            )}
            <span className="ml-auto shrink-0 text-micro text-muted-foreground">{timeText}</span>
          </div>

          {project && (
            // One small pill, the same chip shape used for a project/label tag
            // everywhere else in the app (board-card, the issue list row) —
            // not a second full line of text under every single row.
            <div className="mt-1">
              <span className="inline-flex max-w-full items-center gap-1 truncate rounded-full bg-muted/60 px-1.5 py-0.5 text-micro text-muted-foreground">
                {project.icon && <span aria-hidden>{project.icon}</span>}
                <span className="truncate">{project.title}</span>
              </span>
            </div>
          )}

          {/* Line 2, only when there is real state to show: typing/waiting/
              failed, an unread count, or an inline confirm prompt. Most rows
              have none of those and end after the title now — a preview
              sentence on every row was the busiest thing in the list and the
              least anyone said they wanted to read. */}
          {(isConfirmingDelete || isConfirmingStop || previewNode || unread > 0) && (
          <div className="mt-1 flex items-center gap-2">
            {isConfirmingDelete ? (
                <ConfirmRow
                  label={t(($) => $.session_history.delete_dialog.title)}
                  cancelText={t(($) => $.session_history.delete_dialog.cancel)}
                  confirmText={
                    deleteSession.isPending
                      ? t(($) => $.session_history.delete_dialog.confirming)
                      : t(($) => $.session_history.delete_dialog.confirm)
                  }
                  pending={deleteSession.isPending}
                  onCancel={() => setConfirmingDeleteId(null)}
                  onConfirm={() => handleConfirmDelete(session)}
                />
              ) : isConfirmingStop && pendingTask ? (
                <ConfirmRow
                  label={t(($) => $.session_history.stop_dialog.title)}
                  cancelText={t(($) => $.session_history.stop_dialog.cancel)}
                  confirmText={
                    stoppingTaskId === pendingTask.task_id
                      ? t(($) => $.session_history.stop_dialog.confirming)
                      : t(($) => $.session_history.stop_dialog.confirm)
                  }
                  pending={stoppingTaskId === pendingTask.task_id}
                  onCancel={() => setConfirmingStopId(null)}
                  onConfirm={() => handleConfirmStop(session, pendingTask)}
                />
              ) : (
                <>
                  <div className="flex min-w-0 flex-1 items-center overflow-hidden text-xs text-muted-foreground">
                    <div className="min-w-0 flex-1 overflow-hidden">{previewNode}</div>
                  </div>
                  {unread > 0 && (
                    <span
                      aria-label={t(($) => $.session_history.row_subtitle.new_reply)}
                      // Softer, warmer red than the vivid `destructive` token —
                      // an IM unread badge, not an error.
                      className="inline-flex h-[18px] min-w-[18px] shrink-0 items-center justify-center rounded-full bg-[oklch(0.62_0.14_18)] px-1 text-micro font-semibold text-white"
                    >
                      {unread > 99 ? "99+" : unread}
                    </span>
                  )}
                </>
              )}
          </div>
          )}
        </div>

        {/* Compact action menu — the touch equivalent of the hover strip
            below, which a pointer without hover can never reach. It takes real
            layout space (rather than overlaying the preview) and gives way to
            the hover strip on a hover-capable pointer. */}
        {!isConfirmingAction && (
          <RowActionsMenu
            label={t(($) => $.list.row_actions_aria)}
            groups={[rowActions]}
          />
        )}

        {/* Hover actions — absolutely positioned so showing/hiding them never
            changes the row height (which was making the list jump). Keyboard
            focus reveals them too, so they are reachable without a mouse. */}
        {!isConfirmingAction && (
          <div className="absolute inset-y-0 right-1 hidden items-center gap-0.5 rounded-md bg-gradient-to-l from-accent from-40% to-transparent pl-10 pr-1 [@media(hover:hover)]:group-hover/row:flex [@media(hover:hover)]:group-focus-within/row:flex">
            {rowActions.map((action) => (
              <RowAction
                key={action.key}
                icon={action.icon}
                label={action.label}
                danger={action.danger}
                onClick={action.onSelect}
              />
            ))}
          </div>
        )}
      </div>
    );
  };

  // Archived view: a back header, then the archived rows. Delete lives only
  // here (via each row's hover actions).
  if (view === "archived") {
    return (
      <>
        <button
          type="button"
          onClick={() => setView("history")}
          className="flex w-full items-center gap-1.5 rounded-md px-2 py-2 text-left text-caption font-medium text-muted-foreground outline-none transition-colors hover:bg-accent/50 hover:text-foreground focus-visible:ring-1 focus-visible:ring-ring"
        >
          <ChevronLeft className="size-4 shrink-0" />
          <span className="truncate">{t(($) => $.list.archived_title)}</span>
          <span className="ml-auto shrink-0 tabular-nums text-muted-foreground">
            {archivedSessions.length}
          </span>
        </button>
        {archivedSessions.map(renderRow)}
      </>
    );
  }

  // History (default) view: active rows + a footer entry into the archive.
  const archivedEntry = archivedSessions.length > 0 && (
    <button
      type="button"
      onClick={() => setView("archived")}
      className="mt-1 flex h-10 w-full items-center gap-2 rounded-md px-2 text-left text-caption text-muted-foreground outline-none transition-colors hover:bg-accent/50 hover:text-foreground focus-visible:ring-1 focus-visible:ring-ring"
    >
      <span className="flex size-9 shrink-0 items-center justify-center">
        <Archive className="size-4" />
      </span>
      <span className="min-w-0 flex-1 truncate font-medium">{t(($) => $.list.archived_title)}</span>
      <span className="shrink-0 tabular-nums text-muted-foreground">{archivedSessions.length}</span>
      <ChevronRight className="size-4 shrink-0 text-faint-foreground" />
    </button>
  );

  const libraryControls = (
    <div className="chat-library-controls sticky top-0 z-10 min-w-0 space-y-2 bg-background px-1 pb-2 pt-1">
      <label className="flex h-8 items-center gap-2 rounded-md border bg-background px-2 text-muted-foreground">
        <Search className="size-3.5 shrink-0" />
        <input value={search} onChange={(event) => setSearch(event.target.value)} aria-label={t(($) => $.library.search)} placeholder={t(($) => $.library.search)} className="min-w-0 flex-1 bg-transparent text-caption text-foreground outline-none placeholder:text-muted-foreground" />
      </label>
      <div className="chat-library-toolbar">
        <select aria-label={t(($) => $.library.filter_by)} value={filter} onChange={event => setFilter(event.target.value as typeof filter)} className="chat-library-filter-select rounded-md border bg-background px-2 text-caption text-muted-foreground">
          <option value="all">{t(($) => $.library.all)}</option>
          <option value="unread">{t(($) => $.library.unread)}</option>
          <option value="working">{t(($) => $.library.working)}</option>
          <option value="needs_you">{t(($) => $.work.states.needs_you)}</option>
        </select>
        <div className="chat-library-filter-buttons">
        {(["all", "unread", "working", "needs_you"] as const).map((value) => (
          <button key={value} type="button" aria-pressed={filter === value} onClick={() => setFilter(value)} className={cn("h-8 shrink-0 whitespace-nowrap rounded-md px-2 text-caption", filter === value ? "bg-accent text-foreground" : "text-muted-foreground hover:bg-accent/50")}>
            {value === "all" ? t(($) => $.library.all) : value === "unread" ? t(($) => $.library.unread) : value === "working" ? t(($) => $.library.working) : t(($) => $.work.states.needs_you)}
          </button>
        ))}
        </div>
        <select aria-label={t(($) => $.library.group_by)} value={grouping} onChange={(event) => setGrouping(event.target.value as typeof grouping)} className="chat-library-group-select rounded-md border bg-background px-2 text-caption text-muted-foreground">
          <option value="project">{t(($) => $.library.projects)}</option>
          <option value="recent">{t(($) => $.library.recent)}</option>
          <option value="custom">{t(($) => $.library.groups)}</option>
        </select>
        <Button size="icon-sm" variant="ghost" onClick={() => setManageGroups(true)} aria-label={t(($) => $.library.manage_groups)}><FolderCog className="size-3.5" /></Button>
      </div>
      {organization.failed && <p role="alert" className="px-2 text-caption text-destructive">{t(($) => $.library.save_failed)} <button type="button" className="underline" onClick={organization.retry}>{t(($) => $.library.retry)}</button></p>}
      <Dialog open={manageGroups || !!movingSession} onOpenChange={open => { if (!open) { setManageGroups(false); setMovingSession(null); } }}>
        <DialogContent className="max-h-[80dvh] overflow-y-auto">
          <DialogHeader><DialogTitle>{movingSession ? t(($) => $.library.move_group) : t(($) => $.library.manage_groups)}</DialogTitle><DialogDescription>{t(($) => $.library.group_description)}</DialogDescription></DialogHeader>
          <fieldset disabled={organization.pending || !organization.ready} className="space-y-3">
          {movingSession ? <div className="space-y-1">
            {[{ id: "", name: t(($) => $.library.ungrouped) }, ...library.groups].map(group => <Button key={group.id} variant="ghost" className="w-full justify-start" onClick={() => { changeLibrary({ action: "move_chat", session_id: movingSession.id, id: group.id }, () => setMovingSession(null)); }}>{group.name}</Button>)}
          </div> : library.groups.map(group => <div key={group.id} className="flex gap-2">
            <Input aria-label={t(($) => $.library.group_name)} key={group.name} defaultValue={group.name} maxLength={60} onBlur={event => { if (event.target.value.trim() !== group.name) changeLibrary({ action: "rename_group", id: group.id, name: event.target.value }); }} />
            <Button variant="ghost" size="icon-sm" aria-label={t(($) => $.library.remove_group)} onClick={() => changeLibrary({ action: "delete_group", id: group.id })}><Trash2 className="size-4" /></Button>
          </div>)}
          <form className="flex gap-2" onSubmit={event => { event.preventDefault(); const name = newGroup.trim(); if (!name) return; const id = crypto.randomUUID(); changeLibrary({ action: "create_group", id, name, session_id: movingSession?.id }, () => { setNewGroup(""); setMovingSession(null); }); }}>
            <Input aria-label={t(($) => $.library.new_group)} placeholder={t(($) => $.library.new_group)} value={newGroup} maxLength={60} onChange={event => setNewGroup(event.target.value)} />
            <Button type="submit" variant="outline" disabled={!newGroup.trim()}>{t(($) => $.library.create_group)}</Button>
          </form>
          </fieldset>
          {organization.failed && <p role="alert" className="text-caption text-destructive">{t(($) => $.library.save_failed)} <button type="button" className="underline" onClick={organization.retry}>{t(($) => $.library.retry)}</button></p>}
        </DialogContent>
      </Dialog>
    </div>
  );

  if (historySessions.length === 0) {
    return (
      <>
        {libraryControls}
        <div className="px-2 py-1.5 text-caption text-muted-foreground">
          {sessions.length ? t(($) => $.library.no_matches) : t(($) => $.window.no_previous)}
        </div>
        {archivedEntry}
      </>
    );
  }

  return (
    <>
      {libraryControls}
      {pinnedSessions.length > 0 && (
        <>
          {/* Same section-header shape used elsewhere in the app: a small
              caps label and a rule, not another row that could be mistaken
              for a chat. This — not the small pin glyph on the row itself —
              is what actually sets pinned chats apart from the rest. */}
          <div className="flex items-center gap-2 px-2 pb-1 pt-2">
            <span className="text-micro font-semibold uppercase tracking-wider text-muted-foreground">
              {t(($) => $.list.pinned)}
            </span>
            <div className="h-px flex-1 bg-border" />
          </div>
          {pinnedSessions.map(renderRow)}
          <div className="h-px" />
        </>
      )}
      {grouping === "project" && groupConversationsByProject(unpinnedSessions).map(({ projectId, sessions: groupSessions }) => (
        <details key={projectId ?? "general"} open className="group/project pb-2">
          <summary className="flex min-h-9 cursor-pointer list-none items-center gap-2 px-2 text-caption font-medium text-muted-foreground">
            <ChevronRight className="size-3 shrink-0 group-open/project:rotate-90" />
            <Folder className="size-3.5 shrink-0" />
            <span className="min-w-0 flex-1 truncate">{projectId ? projectById.get(projectId)?.title ?? t(($) => $.library.project_unavailable) : t(($) => $.library.general)}</span>
            <span className="ml-auto shrink-0 text-micro tabular-nums">{groupSessions.length}</span>
          </summary>
          {groupSessions.map(renderRow)}
        </details>
      ))}
      {grouping === "custom" && [...library.groups.map(group => ({ ...group, sessions: unpinnedSessions.filter(session => group.sessionIds.includes(session.id)) })), { id: "ungrouped", name: t(($) => $.library.ungrouped), sessions: unpinnedSessions.filter(session => !library.groups.some(group => group.sessionIds.includes(session.id))) }].map(group => (
        <details key={group.id} open className="group/project pb-2">
          <summary className="flex min-h-9 cursor-pointer list-none items-center gap-2 px-2 text-caption font-medium text-muted-foreground"><ChevronRight className="size-3 shrink-0 group-open/project:rotate-90" /><Folder className="size-3.5 shrink-0" /><span className="min-w-0 flex-1 truncate">{group.name}</span><span className="ml-auto shrink-0 text-micro tabular-nums">{group.sessions.length}</span></summary>
          {group.sessions.map(renderRow)}
        </details>
      ))}
      {grouping === "recent" && dateGroups.map(({ key, sessions: groupSessions }) => (
        <div key={key}>
          <div className="flex items-center gap-2 px-2 pb-1 pt-2">
            <span className="text-micro font-semibold uppercase tracking-wider text-muted-foreground">
              {DATE_BUCKET_LABEL[key]}
            </span>
            <div className="h-px flex-1 bg-border" />
          </div>
          {groupSessions.map(renderRow)}
        </div>
      ))}
      {archivedEntry}
    </>
  );
}

function RowAction({
  icon,
  label,
  onClick,
  danger,
}: {
  icon: React.ReactNode;
  label: string;
  onClick: () => void;
  danger?: boolean;
}) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      onPointerDown={(e) => {
        e.preventDefault();
        e.stopPropagation();
      }}
      onClick={(e) => {
        e.stopPropagation();
        e.preventDefault();
        onClick();
      }}
      className={cn(
        "inline-flex size-7 items-center justify-center rounded text-muted-foreground transition-colors focus-visible:outline-none",
        danger
          ? "hover:bg-destructive/10 hover:text-destructive focus-visible:bg-destructive/10 focus-visible:text-destructive"
          : "hover:bg-accent hover:text-foreground focus-visible:bg-accent focus-visible:text-foreground",
      )}
    >
      {icon}
    </button>
  );
}

function ConfirmRow({
  label,
  cancelText,
  confirmText,
  pending,
  onCancel,
  onConfirm,
}: {
  label: string;
  cancelText: string;
  confirmText: string;
  pending: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  return (
    <div className="flex min-w-0 flex-1 items-center gap-2">
      <span className="min-w-0 flex-1 truncate text-caption font-medium text-destructive">{label}</span>
      <div className="flex shrink-0 items-center gap-1">
        <button
          type="button"
          onPointerDown={(e) => {
            e.preventDefault();
            e.stopPropagation();
          }}
          onClick={(e) => {
            e.stopPropagation();
            e.preventDefault();
            onCancel();
          }}
          disabled={pending}
          className="inline-flex h-6 items-center rounded px-2 text-micro font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-foreground disabled:opacity-50"
        >
          {cancelText}
        </button>
        <button
          type="button"
          onPointerDown={(e) => {
            e.preventDefault();
            e.stopPropagation();
          }}
          onClick={(e) => {
            e.stopPropagation();
            e.preventDefault();
            onConfirm();
          }}
          disabled={pending}
          className="inline-flex h-6 items-center rounded px-2 text-micro font-medium text-destructive transition-colors hover:bg-destructive/10 disabled:opacity-50"
        >
          {confirmText}
        </button>
      </div>
    </div>
  );
}
