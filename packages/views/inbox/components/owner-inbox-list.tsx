"use client";

import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { issueDetailOptions } from "@multica/core/issues/queries";
import type { OwnerInboxBatch, OwnerInboxGroup } from "@multica/core/inbox/owner-inbox-model";
import { waitingFor } from "@multica/core/inbox/owner-inbox-model";
import { cn } from "@multica/ui/lib/utils";

/**
 * The left panel: one row per piece of work, click to read it on the right.
 *
 * This is the same list-beside-detail shape the rest of Multica already uses
 * — the archived inbox view, Issues — not a new pattern. Row chrome is taken
 * directly from `InboxListItem`: the same padding, the same `bg-accent`
 * selected state, the same category pill.
 *
 * A row is one entry from `batchOwnerInbox`: usually one piece of work, but
 * several stalled for the identical reason collapse into one row — six
 * issues that all read "Nobody is working on this right now" were six
 * near-identical rows before this, which reads as the list repeating itself
 * because it was.
 */
export function OwnerInboxList({
  batches,
  selectedKey,
  onSelect,
  emptyLabel,
  emptyAction,
}: {
  batches: readonly OwnerInboxBatch[];
  selectedKey: string;
  onSelect: (group: OwnerInboxGroup) => void;
  /** Shown instead of the default empty message — the filters hid every
   *  row, not "there is nothing here." Same convention `InboxList` already
   *  uses for the archive. */
  emptyLabel?: string;
  emptyAction?: React.ReactNode;
}) {
  if (batches.length === 0) {
    return (
      <div className="flex flex-1 flex-col items-center justify-center gap-2 px-6 py-16 text-center">
        <p className="text-body text-muted-foreground">{emptyLabel ?? "Nothing needs you."}</p>
        {emptyAction}
      </div>
    );
  }

  return (
    <div className="flex-1 min-h-0 overflow-y-auto p-2">
      {batches.map((batch) => {
        const isSelected = batch.groups.some((group) =>
          group.items.some((item) => item.id === selectedKey),
        );
        return (
          <OwnerInboxListRow
            key={batch.key}
            batch={batch}
            isSelected={isSelected}
            // Selecting a batch row opens its first member; the detail pane
            // recognises which batch that member belongs to and shows the
            // shared view, not a single-issue one.
            onClick={() => onSelect(batch.groups[0]!)}
          />
        );
      })}
    </div>
  );
}

const KIND_LABEL: Record<OwnerInboxGroup["ask"]["kind"], string> = {
  decision: "Needs you",
  stopped: "Needs you",
  broke: "Stopped",
  news: "Update",
};

function OwnerInboxListRow({
  batch,
  isSelected,
  onClick,
}: {
  batch: OwnerInboxBatch;
  isSelected: boolean;
  onClick: () => void;
}) {
  const count = batch.groups.length;
  const lead = batch.groups[0]!;
  const newest = batch.groups.reduce(
    (latest, group) => (group.updatedAt > latest ? group.updatedAt : latest),
    lead.updatedAt,
  );
  const unread = batch.groups.some((group) => group.unread);

  const title = batch.shared
    ? `${count} pieces of work ${batch.kind === "broke" ? "stopped on their own" : "are not moving"}`
    : lead.title;

  return (
    <button
      type="button"
      onClick={onClick}
      aria-current={isSelected}
      className={cn(
        "group relative flex w-full flex-col gap-1 rounded-md px-3.5 py-2.5 text-left outline-none transition-colors focus-visible:ring-1 focus-visible:ring-ring",
        isSelected ? "bg-accent" : "hover:bg-accent/50",
      )}
    >
      <div className="flex items-start justify-between gap-2">
        <span
          className={cn(
            "line-clamp-2 text-body leading-snug",
            unread ? "font-medium" : "text-muted-foreground",
          )}
        >
          {title}
        </span>
        <span className="shrink-0 pt-0.5 text-[10px] text-muted-foreground tabular-nums">
          {waitingFor(newest)}
        </span>
      </div>
      <div className="flex items-center justify-between gap-2">
        {batch.shared ? (
          <span className="line-clamp-1 text-caption text-muted-foreground">{batch.shared}</span>
        ) : (
          <RowContext group={lead} />
        )}
        <span className="shrink-0 rounded bg-muted px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">
          {lead.lead.type === "external_work_update" && lead.lead.details?.reported_state === "complete"
            ? "Reported done"
            : lead.lead.type === "external_work_update" && lead.lead.details?.reported_state === "waiting_owner"
              ? "Needs you"
              : KIND_LABEL[lead.ask.kind]}
        </span>
      </div>
    </button>
  );
}

/** The issue id + question, for a row that stands for exactly one piece of
 *  work. Its own component only so the per-row identifier lookup below is
 *  not paid for the (common) batch rows that don't show it. */
function RowContext({ group }: { group: OwnerInboxGroup }) {
  // The same lightweight per-row lookup pattern the current inbox row
  // already uses for its agent-activity indicator — a small, cached query
  // per visible row, not a fetch of the whole list. Real identifier
  // ("SILV-55"), never a raw id or a guess: while it loads, the row falls
  // back to the question itself rather than showing nothing.
  const wsId = useWorkspaceId();
  const { data: issue } = useQuery({
    ...issueDetailOptions(wsId, group.issueId ?? ""),
    enabled: !!group.issueId,
  });
  const context = issue?.identifier ? `${issue.identifier} · ${group.ask.detail}` : group.ask.detail;
  return <span className="line-clamp-1 text-caption text-muted-foreground">{context}</span>;
}
