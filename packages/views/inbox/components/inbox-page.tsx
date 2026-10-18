"use client";

import {
  useState,
  useEffect,
  useCallback,
  useDeferredValue,
  useMemo,
  useRef,
} from "react";
import { useDefaultLayout } from "react-resizable-panels";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { ApiError, errorCode } from "@multica/core/api";
import { useWorkspacePaths } from "@multica/core/paths";
import { useModalStore } from "@multica/core/modals";
import {
  getShortcut,
  isEditableShortcutTarget,
  isPortalLayerShortcutTarget,
  shortcutMatchesEvent,
} from "@multica/core/shortcuts";
import { isImeComposing } from "@multica/core/utils";
import { useIssueDraftStore } from "@multica/core/issues/stores/draft-store";
import {
  developmentMemoryAttentionOptions,
  workforceOverviewIssuesOptions,
} from "@multica/core/issues/queries";
import {
  inboxListOptions,
  archivedInboxListOptions,
  deduplicateInboxItems,
  deduplicateArchivedInboxItems,
  useInboxUnreadCount,
} from "@multica/core/inbox/queries";
import { isOwnerRelevantInboxItem } from "@multica/core/inbox/owner-control";
import { batchOwnerInbox, groupOwnerInbox, needsOwnerAction } from "@multica/core/inbox/owner-inbox-model";
import type { OwnerInboxGroup } from "@multica/core/inbox/owner-inbox-model";
import {
  useMarkInboxRead,
  useMarkInboxUnread,
  useArchiveInbox,
  useUnarchiveInbox,
  useMarkAllInboxRead,
  useArchiveAllInbox,
  useArchiveAllReadInbox,
  useArchiveCompletedInbox,
  useRetrySourceContextQuickCreate,
} from "@multica/core/inbox/mutations";
import {
  filterInboxItems,
  inboxFiltersForPrioritySupport,
  inboxFilterCount,
  inboxPriorityFilterSupport,
  useInboxFilters,
  useInboxFilterStore,
} from "@multica/core/inbox/filter-store";

import { issueHighlightMementoKey } from "../../issues/components";
import { useViewStateWriter } from "../../platform";
import { useNavigation, useReportNavigating } from "../../navigation";
import { toast } from "sonner";
import {
  MoreHorizontal,
  Inbox,
  CheckCheck,
  Archive,
  ArchiveRestore,
  BookCheck,
  ChevronLeft,
  ListChecks,
  ArrowLeft,
} from "lucide-react";
import type { InboxItem } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import {
  ResizablePanelGroup,
  ResizablePanel,
  ResizableHandle,
} from "@multica/ui/components/ui/resizable";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { NumberFlow } from "@multica/ui/components/ui/number-flow";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
} from "@multica/ui/components/ui/dropdown-menu";
import { useIsCompact } from "@multica/ui/hooks/use-mobile";
import { cn } from "@multica/ui/lib/utils";
import { PAGE_GUTTER, PageHeader } from "../../layout/page-header";
import { useTimeAgo } from "./inbox-list-item";
import { InboxList } from "./inbox-list";
import { InboxFilterMenu } from "./inbox-filter-menu";
import { InboxContextMenuProvider } from "./inbox-context-menu";
import { ARCHIVED_VIEW_PARAM, type InboxView } from "./inbox-view";
import { useTypeLabels } from "./inbox-detail-label";
import {
  getInboxDisplayTitle,
  isAutopilotQuotaNotice,
  isQuickCreateOutcome,
  resolveDetailItem,
} from "./inbox-display";
import { AutopilotQuotaNotice } from "./autopilot-quota-notice";
import { useT } from "../../i18n";
import { useIssueLimitUpgradePrompt } from "../../modals/use-issue-limit-upgrade-prompt";
import { OwnerInboxList } from "./owner-inbox-list";
import { OwnerInboxDetail, OwnerInboxBatchDetail } from "./owner-inbox-detail";
import { DevelopmentMemoryReviewList } from "../../issues/components/development-memory-section";
import { InboxModeTabs, WorkforceOverview } from "./workforce-overview";

export function InboxPage() {
  const { t } = useT("inbox");
  const showIssueLimitUpgradePrompt = useIssueLimitUpgradePrompt();
  const showAutopilotQuotaRecoveryPrompt = useIssueLimitUpgradePrompt(
    "autopilot_quota",
  );
  const { searchParams, replace } = useNavigation();
  const urlItem = searchParams.get("item") ?? "";
  const urlIssue = searchParams.get("issue") ?? "";
  const isWorkMode = searchParams.get("mode") === "work";
  const workSelectedId = searchParams.get("work") ?? "";
  const urlView: InboxView =
    searchParams.get("view") === ARCHIVED_VIEW_PARAM ? "archived" : "inbox";
  const wsPaths = useWorkspacePaths();

  const [selectedKey, setSelectedKeyState] = useState(() => urlItem);
  const [view, setViewState] = useState<InboxView>(() => urlView);

  // Sync from URL when searchParams change (e.g. navigation)
  useEffect(() => {
    setSelectedKeyState(urlItem);
  }, [urlItem]);
  useEffect(() => {
    setViewState(urlView);
  }, [urlView]);

  const wsId = useWorkspaceId();
  const { data: memoryAttention = [] } = useQuery(developmentMemoryAttentionOptions(wsId));
  const { data: workforceIssues = [], isLoading: workforceLoading } = useQuery(
    workforceOverviewIssuesOptions(wsId),
  );
  const { data: rawItems = [], isLoading: loading } = useQuery(inboxListOptions(wsId));
  const items = useMemo(
    () => deduplicateInboxItems(rawItems.filter(isOwnerRelevantInboxItem)),
    [rawItems],
  );

  // Fetched in both views, not just the archived one: the main list's entry
  // into the archive is labelled with this count, so it has to be known before
  // the user goes there.
  const {
    data: rawArchivedItems = [],
    isLoading: archivedLoading,
    isError: archivedError,
  } = useQuery(archivedInboxListOptions(wsId));
  const archivedItems = useMemo(
    () => deduplicateArchivedInboxItems(rawArchivedItems),
    [rawArchivedItems],
  );

  const isArchivedView = view === "archived";
  const viewItems = isArchivedView ? archivedItems : items;
  const filters = useInboxFilters(wsId);
  const clearFilters = useInboxFilterStore((state) => state.clearFilters);
  // Active and archived endpoints return the same row contract and are both
  // already loaded on this page. Requiring the combined response to expose the
  // projection prevents one pod in a rolling deploy from advertising a
  // capability the other pod does not have yet.
  const priorityFilterSupport = useMemo(
    () => inboxPriorityFilterSupport([...rawItems, ...rawArchivedItems]),
    [rawItems, rawArchivedItems],
  );
  const effectiveFilters = useMemo(
    () => inboxFiltersForPrioritySupport(filters, priorityFilterSupport),
    [filters, priorityFilterSupport],
  );
  const generallyVisibleItems = useMemo(
    () => filterInboxItems(viewItems, effectiveFilters),
    [viewItems, effectiveFilters],
  );
  const hasActiveFilters = inboxFilterCount(effectiveFilters) > 0;
  const visibleItems = generallyVisibleItems;
  // One entry per piece of work rather than one per event. The main list reads
  // from this; the archive keeps the plain event list, because an archive is a
  // record of what happened and grouping it would hide entries.
  const ownerGroups = useMemo(
    () => (isArchivedView ? [] : groupOwnerInbox(visibleItems).filter(needsOwnerAction)),
    [isArchivedView, visibleItems],
  );
  // Several pieces of work stalled for the identical reason collapse into one
  // row — six issues that all read "Nobody is working on this right now" were
  // six near-identical rows before this, which reads as the list repeating
  // itself because it was. A decision never merges (batchOwnerInbox's own
  // rule): two issues can share the exact same question and still want
  // separate answers.
  const ownerBatches = useMemo(() => batchOwnerInbox(ownerGroups), [ownerGroups]);
  // What the number beside the title counts. It used to be the server's unread
  // event count, which said 7 while the page showed five questions and a group
  // of six — nothing on screen added up to it. It counts the things waiting on
  // an answer, which is the number the page is showing.
  const waitingCount = useMemo(
    () => ownerGroups.length + memoryAttention.length,
    [ownerGroups, memoryAttention.length],
  );
  // The batch the selected row belongs to, for the list-beside-detail main
  // view. `selectedKey` is always a raw item id (URL-shareable, matches the
  // archive's own selection semantics), so this finds whichever batch folded
  // that item in — a batch of one is just that piece of work.
  const detailBatch = useMemo(
    () =>
      ownerBatches.find((batch) =>
        batch.groups.some((group) => group.items.some((entry) => entry.id === selectedKey)),
      ) ?? null,
    [ownerBatches, selectedKey],
  );
  const detailGroup = detailBatch && !detailBatch.shared ? detailBatch.groups[0]! : null;
  const selected =
    visibleItems.find((i) => i.id === selectedKey) ?? null;
  const selectedInView =
    viewItems.find((i) => i.id === selectedKey) ?? null;
  const selectionFilteredOut =
    selectedKey.length > 0 && selectedInView !== null && selected === null;

  // What the DETAIL pane shows, one React transition behind the click.
  //
  // Mounting `IssueDetail` is the expensive half of switching rows, and driven
  // straight off `selectedKey` it ran as an urgent update: the main thread
  // blocked from the click until the new detail was ready, which is the
  // "click, freeze, jump" the desktop shell showed (MUL-6404). Deferred, the
  // click commits the row highlight and the URL right away, React renders the
  // new detail at transition priority — interruptible, so the shell keeps
  // painting — and the previous issue stays on screen until it is ready.
  const detailKey = useDeferredValue(selectedKey);
  const detailSwapping = detailKey !== selectedKey;
  const detailItem = resolveDetailItem(visibleItems, selectedKey, detailKey);

  // The gap above is invisible to the navigation adapter — the inbox stays on
  // the same route and only rewrites `?issue=` — so report it explicitly and
  // the shell's progress bar covers the swap on both platforms.
  useReportNavigating(detailSwapping);

  // Track the last key we actually resolved against the inbox list. Lets the
  // fallback effect distinguish "shared-link to a notification not in our
  // inbox" (never resolved → redirect to the issue page) from "item was in
  // our inbox and just got removed" (was resolved → stay on /inbox).
  const lastResolvedKeyRef = useRef<string>("");
  useEffect(() => {
    if (selected) lastResolvedKeyRef.current = selectedKey;
  }, [selected, selectedKey]);

  // Both the view and the selection live in the URL, so every write has to
  // carry the other one — a bare `?issue=` would silently drop the user out of
  // the archived view on the next selection.
  const buildInboxUrl = useCallback(
    (nextView: InboxView, key: string) => {
      const params = new URLSearchParams();
      if (nextView === "archived") params.set("view", ARCHIVED_VIEW_PARAM);
      if (key) params.set("item", key);
      const query = params.toString();
      const inboxPath = wsPaths.inbox();
      return query ? `${inboxPath}?${query}` : inboxPath;
    },
    [wsPaths],
  );

  const setSelectedKey = useCallback((key: string) => {
    setSelectedKeyState(key);
    replace(buildInboxUrl(view, key));
  }, [replace, buildInboxUrl, view]);

  // Switching views always clears the selection: the two lists are mutually
  // exclusive, so a key carried across would never resolve, and the fallback
  // effect below would bounce the user to the issue page.
  const setView = useCallback((nextView: InboxView) => {
    setViewState(nextView);
    setSelectedKeyState("");
    replace(buildInboxUrl(nextView, ""));
  }, [replace, buildInboxUrl]);

  // Stable identity: InboxList memoizes the archive entry on this callback, so
  // an inline arrow here would rebuild (and remount) the entry every render.
  const openArchived = useCallback(() => setView("archived"), [setView]);

  // Applying a filter can remove the open row from the list. Clear that local
  // selection instead of treating it as a broken deep link and redirecting to
  // the issue page; the notification still exists, it is simply filtered out.
  useEffect(() => {
    if (selectionFilteredOut) setSelectedKey("");
  }, [selectionFilteredOut, setSelectedKey]);

  // Whether the list currently on screen has finished its first load. The
  // fallback and drain effects below both key on this, and getting it wrong in
  // the archived view means acting on an empty list that simply hasn't arrived.
  const viewLoading = isArchivedView ? archivedLoading : loading;

  // Compatibility for existing notification links that name an issue. Pick
  // its newest visible attention, then canonicalize to the item identity so
  // two decisions on the same issue remain separately selectable.
  useEffect(() => {
    if (!urlIssue || urlItem || viewLoading) return;
    const linked = visibleItems.find((item) => item.issue_id === urlIssue);
    if (linked) setSelectedKey(linked.id);
    else replace(wsPaths.issueDetail(urlIssue));
  }, [urlIssue, urlItem, viewLoading, visibleItems, setSelectedKey, replace, wsPaths]);

  // Shared inbox links (?issue=<id>) may point to notifications not in this
  // user's inbox (archived, or never received). Fall back to the issue page
  // so the URL still resolves to something meaningful. But if the key was
  // previously resolvable (e.g. the issue was just deleted in another tab
  // and `onInboxIssueDeleted` pruned the cache), the issue detail would 404
  // too — clear the selection and stay on /inbox instead.
  useEffect(() => {
    if (viewLoading) return;
    if (!selectedKey) return;
    if (selected) return;
    if (selectionFilteredOut) return;
    if (lastResolvedKeyRef.current === selectedKey) {
      setSelectedKey("");
      return;
    }
    setSelectedKey("");
  }, [
    viewLoading,
    selectedKey,
    selected,
    selectionFilteredOut,
    replace,
    wsPaths,
    setSelectedKey,
  ]);

  // Never strand the user on an empty archive: when the last archived issue is
  // restored (or a new notification revives it into the main inbox), fall back
  // to the main list. Same fallback chat's archived view has. Gated on the load
  // so a cold `?view=archived` open doesn't bounce before the data lands.
  useEffect(() => {
    if (!isArchivedView) return;
    if (archivedLoading) return;
    // A failed fetch is also "no items" — bouncing here would swap the error
    // message for the main list and leave the user with no idea it failed.
    if (archivedError) return;
    // Let the fallback effect above settle an unresolved selection first. On a
    // deep link like `?view=archived&issue=X` into an empty archive both would
    // otherwise fire in the same commit, and this one's replace() would land
    // last and swallow the redirect to X.
    if (selectedKey) return;
    if (archivedItems.length > 0) return;
    setView("inbox");
  }, [
    isArchivedView,
    archivedLoading,
    archivedError,
    archivedItems.length,
    selectedKey,
    setView,
  ]);

  const { defaultLayout, onLayoutChanged } = useDefaultLayout({
    id: "multica_inbox_layout",
  });

  const isCompact = useIsCompact();
  const unreadCount = useInboxUnreadCount(wsId);

  const markReadMutation = useMarkInboxRead();
  const markUnreadMutation = useMarkInboxUnread();
  const archiveMutation = useArchiveInbox();
  const unarchiveMutation = useUnarchiveInbox();
  const markAllReadMutation = useMarkAllInboxRead();
  const archiveAllMutation = useArchiveAllInbox();
  const archiveAllReadMutation = useArchiveAllReadInbox();
  const archiveCompletedMutation = useArchiveCompletedInbox();
  const retrySourceContextMutation = useRetrySourceContextQuickCreate();
  const timeAgo = useTimeAgo();
  const typeLabels = useTypeLabels();


  // An explicit "mark as unread" on the row that is currently open has to
  // survive the auto-read effect below, which would otherwise fire on the very
  // next commit and silently undo it. Holds that one item's id, and only while
  // it stays selected: moving the selection elsewhere releases the guard, so
  // re-opening the row later marks it read again like any other open.
  const manualUnreadIdRef = useRef<string | null>(null);

  // Auto-mark-read whenever a selected item is unread — covers both click-
  // to-select and URL-param-select (e.g. OS notification click on desktop).
  // The mutation flips `read: true` optimistically, so this effect settles
  // in one pass and can't loop. Kept in a `useEffect` rather than inlined
  // in handleSelect so URL-driven selection triggers it too.
  const markReadMutate = markReadMutation.mutate;
  const selectedId = selected?.id;
  const selectedRead = selected?.read;
  useEffect(() => {
    if (!selectedId || selectedRead) return;
    if (manualUnreadIdRef.current === selectedId) return;
    markReadMutate(selectedId, {
      onError: (err) =>
        toast.error(
          err instanceof Error && err.message
            ? err.message
            : t(($) => $.errors.mark_read_failed),
        ),
    });
  }, [selectedId, selectedRead, markReadMutate, t]);

  // Release the guard as soon as the selection moves off the parked row.
  useEffect(() => {
    if (manualUnreadIdRef.current && manualUnreadIdRef.current !== selectedId) {
      manualUnreadIdRef.current = null;
    }
  }, [selectedId]);

  const writeViewState = useViewStateWriter();
  // Bumped when the already-open notification row is re-clicked; threaded to
  // IssueDetail so it replays the comment-highlight landing without a
  // remount. Selection changes don't need it — they remount the detail (key
  // by issue) and a fresh mount with a cleared memento entry lands by itself.
  const handleSelect = (item: InboxItem) => {
    const nextKey = item.id;
    // Every click on a notification row is a fresh deep-link intent: clear
    // the "highlight already landed" memento entry so the comment jump runs
    // again even if this issue's detail was opened (and its landing
    // consumed) before. Selection restored from the URL on a remount goes
    // through the effects above, not through here, so a tab switch back
    // keeps the entry and does NOT re-jump.
    if (item.issue_id) {
      writeViewState(issueHighlightMementoKey(item.issue_id), undefined);
    }
    setSelectedKey(nextKey);
  };

  // Same shape as handleSelect, for a row that stands for a whole group
  // instead of one raw event — select the lead event, which is the one every
  // other piece of this page already keys highlighting and read-state off of.
  const handleSelectGroup = (group: OwnerInboxGroup) => {
    if (group.issueId) writeViewState(issueHighlightMementoKey(group.issueId), undefined);
    setSelectedKey(group.lead.id);
  };

  const handleMarkRead = (id: string) => {
    // Reading it back explicitly cancels an earlier park on the same row.
    if (manualUnreadIdRef.current === id) manualUnreadIdRef.current = null;
    markReadMutation.mutate(id, {
      onError: (err) =>
        toast.error(
          err instanceof Error && err.message
            ? err.message
            : t(($) => $.errors.mark_read_failed),
        ),
    });
  };

  const handleMarkUnread = (id: string) => {
    // Only the open row needs the guard — the auto-read effect never touches
    // the others, and arming it for a background row would suppress the very
    // first open of that row later.
    if (selected?.id === id) manualUnreadIdRef.current = id;
    markUnreadMutation.mutate(id, {
      onError: (err) =>
        toast.error(
          err instanceof Error && err.message
            ? err.message
            : t(($) => $.errors.mark_unread_failed),
        ),
    });
  };

  // Both archive and unarchive remove the row from the list it was actioned
  // from, so both have to move the selection off it first. `list` is whichever
  // list the row came from — the main one for archive, the archived one for
  // unarchive.
  const advanceSelectionPast = (id: string, list: InboxItem[]) => {
    const idx = list.findIndex((i) => i.id === id);
    const target = idx >= 0 ? list[idx] : null;
    const wasSelected = !!target && target.id === selectedKey;
    if (!wasSelected) return;
    // List is sorted newest-first; prefer the next (older) item, fall back
    // to the previous (newer) one when actioning at the bottom, and only
    // clear the selection when nothing else is left.
    const next = list[idx + 1] ?? list[idx - 1] ?? null;
    setSelectedKey(next ? next.id : "");
  };

  // Toasts live in these shared handlers so every archive surface confirms alike.
  const handleArchive = (id: string) => {
    advanceSelectionPast(id, visibleItems);
    archiveMutation.mutate(id, {
      onSuccess: () => toast.success(t(($) => $.toasts.archived)),
      onError: (err) =>
        toast.error(
          err instanceof Error && err.message
            ? err.message
            : t(($) => $.errors.archive_failed),
        ),
    });
  };

  // Clearing a piece of work clears every event that made it up. Archiving only
  // the row the owner clicked would leave its two crashed runs behind, and the
  // work would reappear at the top of the list looking untouched.
  const handleArchiveGroup = (target: { items: InboxItem[] }) => {
    if (target.items.some((entry) => entry.id === selectedKey)) {
      advanceSelectionPast(selectedKey, visibleItems);
    }
    for (const entry of target.items) {
      archiveMutation.mutate(entry.id, {
        onError: (err) =>
          toast.error(
            err instanceof Error && err.message
              ? err.message
              : t(($) => $.errors.archive_failed),
          ),
      });
    }
  };

  /**
   * The repair controls that belong to a notice rather than to a piece of
   * work: retrying a quick-create with the context it lost, recovering an
   * autopilot that ran out of its allowance. They used to live in the detail
   * pane, which the owner's inbox no longer has, so the card carries them.
   */
  const noticeActions = (notice: InboxItem) => (
    <>
      {isAutopilotQuotaNotice(notice.type) && (
        <AutopilotQuotaNotice item={notice} onOpenRecovery={showAutopilotQuotaRecoveryPrompt} />
      )}
      {notice.type === "quick_create_failed" &&
        notice.details?.source_context_id &&
        notice.details?.task_id && (
          <Button
            size="sm"
            data-testid="retry-source-context"
            disabled={retrySourceContextMutation.isPending}
            onClick={async () => {
              try {
                await retrySourceContextMutation.mutateAsync(notice.details!.task_id!);
                toast.success(t(($) => $.toasts.source_context_retry_started));
              } catch (error) {
                if (error instanceof ApiError && errorCode(error) === "issue_limit_reached") {
                  showIssueLimitUpgradePrompt();
                  return;
                }
                toast.error(
                  error instanceof ApiError &&
                    errorCode(error) === "source_context_retry_unavailable"
                    ? t(($) => $.errors.source_context_retry_unavailable)
                    : t(($) => $.errors.source_context_retry_failed),
                );
              }
            }}
          >
            {t(($) => $.detail.retry_with_context)}
          </Button>
        )}
      {isQuickCreateOutcome(notice.type) && (
        <Button
          size="sm"
          variant="outline"
          onClick={() => {
            // Seed the advanced form with the original prompt so the input can
            // be recovered in the full editor instead of retyped.
            useIssueDraftStore.getState().setManual({
              description: notice.details?.original_prompt ?? "",
              ...(notice.details?.agent_id
                ? { assigneeType: "agent" as const, assigneeId: notice.details.agent_id }
                : {}),
            });
            useModalStore.getState().open("create-issue");
          }}
        >
          {t(($) => $.detail.edit_advanced)}
        </Button>
      )}
    </>
  );

  const handleUnarchive = (id: string) => {
    advanceSelectionPast(id, visibleItems);
    unarchiveMutation.mutate(id, {
      onSuccess: () => toast.success(t(($) => $.toasts.unarchived)),
      onError: (err) =>
        toast.error(
          err instanceof Error && err.message
            ? err.message
            : t(($) => $.errors.unarchive_failed),
        ),
    });
  };

  // Keep the listener stable while using the latest selected-item action.
  const actionOnSelectedRef = useRef<(() => void) | null>(null);
  useEffect(() => {
    actionOnSelectedRef.current = selected
      ? () => (isArchivedView ? handleUnarchive(selected.id) : handleArchive(selected.id))
      : null;
  });

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.defaultPrevented || event.repeat || isImeComposing(event)) return;
      if (isEditableShortcutTarget(event.target)) return;
      if (isPortalLayerShortcutTarget(event.target)) return;
      if (useModalStore.getState().modal) return;
      if (!shortcutMatchesEvent(getShortcut("archiveInboxItem"), event)) return;
      const run = actionOnSelectedRef.current;
      if (!run) return;
      event.preventDefault();
      run();
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, []);

  // Batch operations
  const handleMarkAllRead = () => {
    markAllReadMutation.mutate(undefined, {
      onError: (err) =>
        toast.error(
          err instanceof Error && err.message
            ? err.message
            : t(($) => $.errors.mark_all_read_failed),
        ),
    });
  };

  const handleArchiveAll = () => {
    setSelectedKey("");
    archiveAllMutation.mutate(undefined, {
      onError: (err) =>
        toast.error(
          err instanceof Error && err.message
            ? err.message
            : t(($) => $.errors.archive_all_failed),
        ),
    });
  };

  const handleArchiveAllRead = () => {
    const readKeys = items.filter((i) => i.read).map((i) => i.id);
    if (readKeys.includes(selectedKey)) setSelectedKey("");
    archiveAllReadMutation.mutate(undefined, {
      onError: (err) =>
        toast.error(
          err instanceof Error && err.message
            ? err.message
            : t(($) => $.errors.archive_all_read_failed),
        ),
    });
  };

  const handleArchiveCompleted = () => {
    setSelectedKey("");
    archiveCompletedMutation.mutate(undefined, {
      onError: (err) =>
        toast.error(
          err instanceof Error && err.message
            ? err.message
            : t(($) => $.errors.archive_completed_failed),
        ),
    });
  };

  // -- Shared sub-components --------------------------------------------------

  const listHeader = (
    <PageHeader>
      <div className="flex flex-1 items-center gap-2">
        <h1 className="text-body font-semibold">{t(($) => $.page.title)}</h1>
        {(isArchivedView ? unreadCount : waitingCount) > 0 && (
          <NumberFlow
            value={isArchivedView ? unreadCount : waitingCount}
            animated={false}
            format={{ maximumFractionDigits: 0 }}
            aria-label={String(isArchivedView ? unreadCount : waitingCount)}
            className="text-caption text-muted-foreground"
          />
        )}
      </div>
      <InboxFilterMenu
        wsId={wsId}
        items={viewItems}
        priorityFilterSupport={priorityFilterSupport}
      />
      {/* Batch actions are main-view only. Every entry archives from the MAIN
          inbox, so offering them while the archived list is on screen reads as
          "archive all of these" and does the opposite of what it looks like. */}
      {!isArchivedView && (
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button
              variant="ghost"
              size="icon-sm"
              className="text-muted-foreground"
            />
          }
        >
          <MoreHorizontal className="h-4 w-4" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-auto">
          <DropdownMenuItem onClick={handleMarkAllRead}>
            <CheckCheck className="h-4 w-4" />
            {t(($) => $.menu.mark_all_read)}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem onClick={handleArchiveAll}>
            <Archive className="h-4 w-4" />
            {t(($) => $.menu.archive_all)}
          </DropdownMenuItem>
          <DropdownMenuItem onClick={handleArchiveAllRead}>
            <BookCheck className="h-4 w-4" />
            {t(($) => $.menu.archive_all_read)}
          </DropdownMenuItem>
          <DropdownMenuItem onClick={handleArchiveCompleted}>
            <ListChecks className="h-4 w-4" />
            {t(($) => $.menu.archive_completed)}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      )}
    </PageHeader>
  );

  // Back out of the archive. Sits inside the list panel rather than replacing
  // the PageHeader: the user is still in the Inbox, so the page title stays put
  // and this reads as a sub-view — the same shape chat's archived view uses.
  const archivedBackRow = (
    <button
      type="button"
      onClick={() => setView("inbox")}
      className="flex w-full shrink-0 items-center gap-1.5 border-b px-3 py-2 text-left text-caption font-medium text-muted-foreground outline-none transition-colors hover:bg-accent/50 hover:text-foreground focus-visible:ring-1 focus-visible:ring-ring"
    >
      <ChevronLeft className="size-4 shrink-0" />
      <span className="truncate">{t(($) => $.list.archived_title)}</span>
      <span className="ml-auto shrink-0 tabular-nums text-muted-foreground">
        {archivedItems.length}
      </span>
    </button>
  );

  const list = archivedError && isArchivedView ? (
    <div className="flex-1 min-h-0 overflow-y-auto">
      <div className="flex flex-col items-center justify-center py-16 text-muted-foreground">
        <Archive className="mb-3 h-8 w-8 text-faint-foreground" />
        <p className="text-body">{t(($) => $.errors.archived_load_failed)}</p>
      </div>
    </div>
  ) : (
    <InboxContextMenuProvider
      view={view}
      actions={{
        onMarkRead: handleMarkRead,
        onMarkUnread: handleMarkUnread,
        onAction: isArchivedView ? handleUnarchive : handleArchive,
      }}
    >
      <InboxList
        items={visibleItems}
        view={view}
        selectedKey={selectedKey}
        archivedCount={archivedItems.length}
        onSelect={handleSelect}
        onAction={isArchivedView ? handleUnarchive : handleArchive}
        onOpenArchived={openArchived}
        emptyLabel={
          hasActiveFilters && viewItems.length > 0 && visibleItems.length === 0
            ? t(($) => $.filters.empty)
            : undefined
        }
        emptyAction={
          hasActiveFilters && viewItems.length > 0 && visibleItems.length === 0 ? (
            <Button
              variant="outline"
              size="sm"
              onClick={() => clearFilters(wsId)}
            >
              {t(($) => $.filters.clear)}
            </Button>
          ) : undefined
        }
      />
    </InboxContextMenuProvider>
  );

  const listPanel = (
    <>
      {listHeader}
      {isArchivedView && archivedBackRow}
      {list}
    </>
  );

  const filtersHidEverything =
    hasActiveFilters && viewItems.length > 0 && visibleItems.length === 0;

  // The main inbox: the same list-beside-detail shape the archive already
  // uses, one row per piece of work instead of one row per raw event. This
  // replaced a single-column feed of self-contained cards — that shape read
  // as disconnected from the rest of Multica, where a list beside a detail
  // pane is the established surface (Issues, the archive below).
  const ownerListPanel = (
    <>
      {listHeader}
      <InboxModeTabs
        mode="needs"
        needsCount={waitingCount}
        workCount={workforceIssues.length}
        onNeedsYou={() => undefined}
        onAllWork={() => replace(`${wsPaths.inbox()}?mode=work`)}
      />
      {memoryAttention.length > 0 && (
        <div className={cn("max-h-[45%] shrink-0 overflow-y-auto border-b py-3", PAGE_GUTTER)}>
          <DevelopmentMemoryReviewList wsId={wsId} proposals={memoryAttention} />
        </div>
      )}
      <OwnerInboxList
        batches={ownerBatches}
        selectedKey={selectedKey}
        onSelect={handleSelectGroup}
        emptyLabel={filtersHidEverything ? t(($) => $.filters.empty) : undefined}
        emptyAction={
          filtersHidEverything ? (
            <Button variant="outline" size="sm" onClick={() => clearFilters(wsId)}>
              {t(($) => $.filters.clear)}
            </Button>
          ) : undefined
        }
      />
    </>
  );

  const ownerDetailContent = detailBatch?.shared ? (
    <OwnerInboxBatchDetail key={detailBatch.key} batch={detailBatch} />
  ) : detailGroup ? (
    <OwnerInboxDetail
      key={detailGroup.key}
      group={detailGroup}
      onClear={() => handleArchiveGroup(detailGroup)}
      onLater={() => {
        for (const entry of detailGroup.items) if (!entry.read) handleMarkRead(entry.id);
      }}
      extras={noticeActions(detailGroup.lead)}
    />
  ) : (
    <div className="flex h-full flex-col items-center justify-center text-muted-foreground">
      <Inbox className="mb-3 h-10 w-10 text-faint-foreground" />
      <p className="text-body">
        {ownerGroups.length === 0 ? "Nothing needs you." : "Select something on the left to read it."}
      </p>
    </div>
  );

  // Compact widths only: the detail fills the screen there, so the trip back to
  // the list has to live inside it. On desktop the list is still on screen in
  // the left panel and needs no back control at all.
  //
  // The detail owns the whole compact screen in four states — loaded, loading,
  // not-found and crashed — so this control is threaded into all four below.
  // Miss one and that state strands the reader with no way out.
  const compactBackAction = isCompact ? (
    <Button
      variant="ghost"
      size="sm"
      onClick={() => setSelectedKey("")}
      className="-ml-2 shrink-0 gap-1.5 text-muted-foreground"
    >
      <ArrowLeft className="h-4 w-4" />
      {/* Back goes to the list the user came FROM, so the label has to
          name it — "Inbox" here would be a lie about the destination. */}
      {isArchivedView ? t(($) => $.list.archived_title) : t(($) => $.page.back)}
    </Button>
  ) : undefined;

  const compactBackBar = compactBackAction ? (
    <div className={cn("flex h-12 shrink-0 items-center gap-2 border-b", PAGE_GUTTER)}>
      {compactBackAction}
    </div>
  ) : null;

  // Only the archive has a detail pane now — it is a record to read, not work
  // to direct. Everything actionable is answered on its own card in the board.
  const detailContent = detailItem ? (
    <div className="p-6">
      <h2 className="text-title font-semibold">
        {isAutopilotQuotaNotice(detailItem.type)
          ? typeLabels[detailItem.type]
          : getInboxDisplayTitle(detailItem)}
      </h2>
      <p className="mt-1 text-body text-muted-foreground">
        {typeLabels[detailItem.type]} · {timeAgo(detailItem.created_at)}
      </p>
      {isAutopilotQuotaNotice(detailItem.type) ? (
        <AutopilotQuotaNotice
          item={detailItem}
          onOpenRecovery={showAutopilotQuotaRecoveryPrompt}
        />
      ) : detailItem.body ? (
        <div className="mt-4 whitespace-pre-wrap text-body leading-relaxed text-foreground">
          {detailItem.body}
        </div>
      ) : null}
      {isQuickCreateOutcome(detailItem.type) && detailItem.details?.original_prompt && (
        <div className="mt-4 rounded-md border bg-muted/40 p-3">
          <p className="text-caption font-medium text-muted-foreground">
            {t(($) => $.detail.original_input)}
          </p>
          <p className="mt-1 whitespace-pre-wrap text-body">{detailItem.details.original_prompt}</p>
        </div>
      )}
      <div className="mt-4 flex gap-2">
        {detailItem.type === "quick_create_failed" &&
          detailItem.details?.source_context_id &&
          detailItem.details?.task_id && (
            <Button
              size="sm"
              data-testid="retry-source-context"
              disabled={retrySourceContextMutation.isPending}
              onClick={async () => {
                try {
                  await retrySourceContextMutation.mutateAsync(detailItem.details!.task_id!);
                  toast.success(t(($) => $.toasts.source_context_retry_started));
                } catch (error) {
                  if (
                    error instanceof ApiError &&
                    errorCode(error) === "issue_limit_reached"
                  ) {
                    showIssueLimitUpgradePrompt();
                    return;
                  }
                  toast.error(
                    error instanceof ApiError &&
                      errorCode(error) === "source_context_retry_unavailable"
                      ? t(($) => $.errors.source_context_retry_unavailable)
                      : t(($) => $.errors.source_context_retry_failed),
                  );
                }
              }}
            >
              {t(($) => $.detail.retry_with_context)}
            </Button>
          )}
        {isQuickCreateOutcome(detailItem.type) && (
          <Button
            size="sm"
            onClick={() => {
              // Seed the legacy advanced form with the original prompt so the
              // user can recover their input in the full editor instead of
              // retyping. The agent picker hint becomes the assignee
              // candidate (still editable).
              const prompt = detailItem.details?.original_prompt ?? "";
              const agentId = detailItem.details?.agent_id;
              useIssueDraftStore.getState().setManual({
                description: prompt,
                ...(agentId
                  ? { assigneeType: "agent" as const, assigneeId: agentId }
                  : {}),
              });
              useModalStore.getState().open("create-issue");
            }}
          >
            {t(($) => $.detail.edit_advanced)}
          </Button>
        )}
        {/* Mirrors the row action: the button always reverses the view the
            item is being read in. */}
        {isArchivedView ? (
          <Button
            variant="outline"
            size="sm"
            onClick={() => handleUnarchive(detailItem.id)}
          >
            <ArchiveRestore className="mr-1.5 h-3.5 w-3.5" />
            {t(($) => $.detail.unarchive)}
          </Button>
        ) : (
          <Button
            variant="outline"
            size="sm"
            onClick={() => handleArchive(detailItem.id)}
          >
            <Archive className="mr-1.5 h-3.5 w-3.5" />
            {t(($) => $.detail.archive)}
          </Button>
        )}
      </div>
    </div>
  ) : null;

  // -- Compact layout: list / detail toggle -----------------------------------

  if (isWorkMode) {
    const workUrl = (issueId = "") => {
      const params = new URLSearchParams({ mode: "work" });
      if (issueId) params.set("work", issueId);
      return `${wsPaths.inbox()}?${params.toString()}`;
    };
    return (
      <WorkforceOverview
        issues={workforceIssues}
        loading={workforceLoading}
        selectedId={workSelectedId}
        needsCount={waitingCount}
        onNeedsYou={() => replace(wsPaths.inbox())}
        onSelect={(issueId) => replace(workUrl(issueId))}
        onClearSelection={() => replace(workUrl())}
      />
    );
  }

  if (isCompact) {
    if (viewLoading) {
      return (
        <div className="flex flex-1 flex-col min-h-0">
          <div className={cn("flex h-12 shrink-0 items-center border-b", PAGE_GUTTER)}>
            <Skeleton className="h-5 w-16" />
          </div>
          <div className="flex-1 min-h-0 overflow-y-auto space-y-1 p-2">
            {Array.from({ length: 5 }).map((_, i) => (
              <div key={i} className="flex items-center gap-3 px-2 py-2.5">
                <div className="flex-1 space-y-2">
                  <Skeleton className="h-4 w-3/4" />
                  <Skeleton className="h-3 w-1/2" />
                </div>
              </div>
            ))}
          </div>
        </div>
      );
    }

    // Compact: a selected record fills the screen, in either view.
    if (isArchivedView ? detailItem : detailBatch) {
      // A notification body is a plain block with no header to host a leading
      // slot and no scroller of its own, so this branch keeps supplying both.
      return (
        <div className="flex flex-1 flex-col min-h-0">
          {compactBackBar}
          <div className="flex-1 min-h-0 overflow-y-auto">
            {isArchivedView ? detailContent : ownerDetailContent}
          </div>
        </div>
      );
    }

    // Compact: full-screen list
    return (
      <div className="flex flex-1 flex-col min-h-0">
        {isArchivedView ? listPanel : ownerListPanel}
      </div>
    );
  }

  // -- Desktop --------------------------------------------------------------
  //
  // List beside detail in both views now — the same ResizablePanelGroup, the
  // same panel sizes, whichever view is active.

  if (viewLoading) {
    return (
      <ResizablePanelGroup orientation="horizontal" className="flex-1 min-h-0" defaultLayout={defaultLayout} onLayoutChanged={onLayoutChanged}>
        <ResizablePanel id="list" defaultSize={360} minSize={320} maxSize={560} groupResizeBehavior="preserve-pixel-size">
          <div className="flex flex-col border-r h-full">
            <div className={cn("flex h-12 shrink-0 items-center border-b", PAGE_GUTTER)}>
              <Skeleton className="h-5 w-16" />
            </div>
            <div className="flex-1 min-h-0 overflow-y-auto space-y-1 p-2">
              {Array.from({ length: 5 }).map((_, i) => (
                <div key={i} className="flex items-center gap-3 px-2 py-2.5">
                  <Skeleton className="h-7 w-7 shrink-0 rounded-full" />
                  <div className="flex-1 space-y-2">
                    <Skeleton className="h-4 w-3/4" />
                    <Skeleton className="h-3 w-1/2" />
                  </div>
                </div>
              ))}
            </div>
          </div>
        </ResizablePanel>
        <ResizableHandle />
        <ResizablePanel id="detail" minSize="40%">
          <div className="p-6">
            <Skeleton className="h-6 w-48" />
            <Skeleton className="mt-4 h-4 w-32" />
          </div>
        </ResizablePanel>
      </ResizablePanelGroup>
    );
  }

  return (
    <ResizablePanelGroup orientation="horizontal" className="flex-1 min-h-0" defaultLayout={defaultLayout} onLayoutChanged={onLayoutChanged}>
      <ResizablePanel id="list" defaultSize={360} minSize={320} maxSize={560} groupResizeBehavior="preserve-pixel-size">
      <div className="flex flex-col border-r h-full">
        {isArchivedView ? listPanel : ownerListPanel}
      </div>
      </ResizablePanel>
      <ResizableHandle />
      <ResizablePanel id="detail" minSize="40%">
      <div className="flex flex-col min-h-0 h-full">
        {isArchivedView
          ? (detailContent ?? (
              <div className="flex h-full flex-col items-center justify-center text-muted-foreground">
                <Inbox className="mb-3 h-10 w-10 text-faint-foreground" />
                <p className="text-body">
                  {visibleItems.length === 0
                    ? t(($) => $.detail.empty)
                    : t(($) => $.detail.select_prompt)}
                </p>
              </div>
            ))
          : ownerDetailContent}
      </div>
      </ResizablePanel>
    </ResizablePanelGroup>
  );
}
