"use client";

import {
  IssueAgentActivityIndicator,
} from "../../issues/components/issue-agent-activity-indicator";
import { Archive, ArchiveRestore } from "lucide-react";
import type { InboxItem } from "@multica/core/types";
import type { InboxView } from "./inbox-view";
import { InboxDetailLabel, useTypeLabels } from "./inbox-detail-label";
import {
  getInboxDisplayTitle,
  isAutopilotQuotaNotice,
} from "./inbox-display";
import { useInboxContextMenu } from "./inbox-context-menu";
import { InboxRowMenu } from "./inbox-row-menu";
import { handleRowActivationKey } from "../../common/row-actions-menu";
import { useT } from "../../i18n";
import { paths, useWorkspaceSlug } from "@multica/core/paths";
import { resolveClickIntent, useIntentNavigate } from "../../navigation";
import { ownerInboxCategory } from "@multica/core/inbox/owner-control";

// Hook returning a localized relative-time formatter — the i18n equivalent
// of the previous static `timeAgo` function. Returning a function (rather
// than a string) keeps call-site usage identical: `timeAgo(dateStr)`.
export function useTimeAgo() {
  const { t } = useT("inbox");
  return (dateStr: string): string => {
    const diff = Date.now() - new Date(dateStr).getTime();
    const minutes = Math.floor(diff / 60000);
    if (minutes < 1) return t(($) => $.list.time.just_now);
    if (minutes < 60) return t(($) => $.list.time.minutes, { count: minutes });
    const hours = Math.floor(minutes / 60);
    if (hours < 24) return t(($) => $.list.time.hours, { count: hours });
    const days = Math.floor(hours / 24);
    return t(($) => $.list.time.days, { count: days });
  };
}

export function InboxListItem({
  item,
  view,
  isSelected,
  onClick,
  onAction,
}: {
  item: InboxItem;
  view: InboxView;
  isSelected: boolean;
  onClick: () => void;
  // Archive in the main list, unarchive in the archived one — the row action is
  // always the reversal of the current view, so the two lists share this row.
  onAction: () => void;
}) {
  const { t } = useT("inbox");
  const timeAgo = useTimeAgo();
  const typeLabels = useTypeLabels();
  const openContextMenu = useInboxContextMenu();
  // Null-safe slug (not useWorkspacePaths, which throws): the row renders in
  // tests and could render outside a workspace route; without a slug the
  // modifier-click affordance simply stays off.
  const slug = useWorkspaceSlug();
  const issueHref =
    slug && item.issue_id
      ? paths.workspace(slug).issueDetail(item.issue_id)
      : null;
  const intentNavigate = useIntentNavigate();
  const displayTitle = isAutopilotQuotaNotice(item.type)
    ? typeLabels[item.type]
    : getInboxDisplayTitle(item);
  const isArchivedView = view === "archived";
  // Archiving deliberately leaves `read` untouched so unarchiving restores the
  // real unread state, so archived rows would otherwise keep an unread marker
  // the user cannot clear from this view. Suppress the affordance here only.
  const showUnread = item.read !== true && !isArchivedView;
  const ActionIcon = isArchivedView ? ArchiveRestore : Archive;
  const actionLabel = isArchivedView
    ? t(($) => $.list.unarchive_tooltip)
    : t(($) => $.list.archive_tooltip);
  const category = ownerInboxCategory(item);
  const categoryLabel = category === "needs_you" ? "Needs you" : category === "incident" ? "Incident" : category === "completion" ? "Verified" : "Update";

  return (
    // A div, not a <button>: the row carries its own controls (the action
    // button, the compact menu), and interactive descendants of a <button>
    // are invalid HTML that screen readers cannot reach. `role="button"` plus
    // the Enter/Space handler keeps the row's own keyboard behavior.
    <div
      data-testid="inbox-row"
      role="button"
      tabIndex={0}
      onClick={(e) => {
        // Plain click keeps the master-detail selection; a modifier click on
        // a row that references an issue opens that issue as its own tab.
        if (issueHref) {
          const intent = resolveClickIntent(e);
          if (intent !== "push") {
            intentNavigate(issueHref, intent);
            return;
          }
        }
        onClick();
      }}
      onKeyDown={(e) => handleRowActivationKey(e, onClick)}
      onAuxClick={(e) => {
        if (e.defaultPrevented || e.button !== 1 || !issueHref) return;
        e.preventDefault();
        intentNavigate(issueHref, "background-tab");
      }}
      // Right-click opens the list's shared menu (mark read/unread, archive).
      // `select-none` mirrors what Base UI's own trigger used to merge in, so
      // right-clicking a row never starts a text selection.
      onContextMenu={
        openContextMenu ? (e) => openContextMenu(item, e) : undefined
      }
      className={`group w-full cursor-default select-none rounded-md px-3 py-3 text-left outline-none transition-colors focus-visible:ring-1 focus-visible:ring-ring ${
        isSelected
          ? "bg-accent"
          : "hover:bg-accent/50 data-[popup-open]:bg-accent/50"
      }`}
    >
      <div className="min-w-0">
        <div className="flex items-center justify-between gap-2">
          <div className="flex min-w-0 items-center gap-1.5">
            {showUnread && (
              <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-brand" />
            )}
            <span
              data-testid="inbox-row-title"
              className={`line-clamp-2 text-body leading-snug ${showUnread ? "font-medium" : "text-muted-foreground"}`}
            >
              {displayTitle}
            </span>
          </div>
          <div className="flex shrink-0 items-center gap-1">
            {/* Pointer-only affordance: revealed on hover, and on keyboard
                focus anywhere in the row so it is reachable by Tab. Touch has
                neither, so it stays hidden on a pointer that cannot hover and
                the compact menu below carries the same action. */}
            <button
              type="button"
              title={actionLabel}
              aria-label={actionLabel}
              onClick={(e) => {
                e.stopPropagation();
                onAction();
              }}
              className="hidden rounded p-0.5 text-muted-foreground outline-none hover:bg-accent hover:text-foreground focus-visible:ring-1 focus-visible:ring-ring [@media(hover:hover)]:group-hover:inline-flex [@media(hover:hover)]:group-focus-within:inline-flex"
            >
              <ActionIcon className="h-3.5 w-3.5" />
            </button>
            <InboxRowMenu item={item} view={view} />
          </div>
        </div>
        <div className="mt-1.5">
          <p data-testid="inbox-row-description" className="line-clamp-2 text-caption leading-relaxed text-muted-foreground">
            <InboxDetailLabel item={item} />
          </p>
          <div className="mt-2 flex items-center justify-between gap-2">
            <span className="rounded bg-muted px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">{categoryLabel}</span>
            <div className="flex shrink-0 items-center gap-1.5">
            {item.issue_id && (
              <IssueAgentActivityIndicator
                issueId={item.issue_id}
                hoverCard={false}
              />
            )}
            <span className="text-caption text-muted-foreground">{timeAgo(item.created_at)}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
