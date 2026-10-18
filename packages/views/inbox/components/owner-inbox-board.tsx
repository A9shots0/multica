"use client";
import { useT } from "../../i18n";

import { useCallback, useMemo, useState } from "react";
import { Archive, CheckCircle2, ChevronRight } from "lucide-react";
import type { OwnerInboxGroup } from "@multica/core/inbox/owner-inbox-model";
import { batchOwnerInbox, wantsAnswer } from "@multica/core/inbox/owner-inbox-model";
import { Button } from "@multica/ui/components/ui/button";
import type { InboxItem } from "@multica/core/types";
import {
  OwnerInboxBatchCard,
  OwnerInboxCard,
  OwnerInboxNote,
  OwnerInboxNoticeCard,
} from "./owner-inbox-card";

/**
 * The owner's inbox: one column of things, each answered where it is read.
 *
 * It replaces a list beside a detail pane. That layout cost a click before
 * anything could be dealt with, gave a one-sentence question half a wide
 * screen to be empty in, and made every row wear a badge repeating the heading
 * of the section it was already under. Ten questions could not be seen at
 * once, which is the only thing the owner actually wanted from an inbox.
 *
 * Here everything is visible and everything is actionable in place. Work stuck
 * for the identical reason is one card with one button, not six cards saying
 * the same sentence. Below the line sit the things with nothing to answer, one
 * line each.
 */
export function OwnerInboxBoard({
  groups,
  highlightKey,
  onClearGroup,
  onClearMany,
  onLaterGroup,
  noticeActions,
  archivedCount,
  onOpenArchived,
  emptyLabel,
  emptyAction,
}: {
  groups: OwnerInboxGroup[];
  /** The item a notification link opened, ringed so it can be found. */
  highlightKey: string;
  onClearGroup: (group: OwnerInboxGroup) => void;
  onClearMany: (groups: OwnerInboxGroup[]) => void;
  /** Mark as seen without answering, so it sinks below what is still new. */
  onLaterGroup: (group: OwnerInboxGroup) => void;
  /** Repair controls for a notice with no piece of work behind it — retrying a
   *  quick-create, opening quota recovery. The page owns that wiring. */
  noticeActions?: (item: InboxItem) => React.ReactNode;
  archivedCount: number;
  onOpenArchived: () => void;
  emptyLabel?: string;
  emptyAction?: React.ReactNode;
}) {
  const { t: tc } = useT("inbox");
  // What has not been looked at yet comes first. "Not now" marks a card read,
  // which is what sinks it — the owner gets a way past something without
  // answering it or filing it away, and it is still there tomorrow.
  const waiting = useMemo(
    () =>
      groups
        .filter((g) => wantsAnswer(g.ask.kind))
        .slice()
        .sort(
          (a, b) =>
            // A question only the owner can answer outranks work that merely
            // stalled, whatever their read state: sorting on unread alone put
            // a batch of stopped runs above four real decisions.
            (a.ask.kind === "decision" ? 0 : 1) - (b.ask.kind === "decision" ? 0 : 1) ||
            Number(b.unread) - Number(a.unread),
        ),
    [groups],
  );
  const rest = useMemo(() => groups.filter((g) => !wantsAnswer(g.ask.kind)), [groups]);

  // The full-screen view belongs to the board, not to one card, so that
  // answering something can move straight on to the next thing instead of
  // dropping the owner back into the list to find it.
  const [viewerKey, setViewerKey] = useState<string | null>(null);
  const [viewerSource, setViewerSource] = useState<string | undefined>(undefined);
  const walk = useMemo(() => [...waiting, ...rest].filter((g) => g.issueId), [waiting, rest]);
  const viewerIndex = walk.findIndex((g) => g.key === viewerKey);

  const step = useCallback(
    (delta: number) => {
      if (walk.length === 0) return;
      const from = walk.findIndex((g) => g.key === viewerKey);
      const next = walk[(from + delta + walk.length) % walk.length];
      if (!next) return;
      setViewerSource(undefined);
      setViewerKey(next.key);
    },
    [walk, viewerKey],
  );

  const viewerFor = useCallback(
    (group: OwnerInboxGroup) =>
      group.key === viewerKey && viewerIndex >= 0
        ? { sourceId: viewerSource, position: { index: viewerIndex, total: walk.length } }
        : undefined,
    [viewerKey, viewerSource, viewerIndex, walk.length],
  );

  const viewerProps = useCallback(
    (group: OwnerInboxGroup) => ({
      viewer: viewerFor(group),
      onOpenViewer: (sourceId?: string) => {
        setViewerSource(sourceId);
        setViewerKey(group.key);
      },
      onCloseViewer: () => setViewerKey(null),
      onStepViewer: step,
    }),
    [viewerFor, step],
  );
  const batches = useMemo(() => batchOwnerInbox(waiting), [waiting]);
  const brokeBatches = useMemo(
    () => batchOwnerInbox(rest.filter((g) => g.ask.kind === "broke")),
    [rest],
  );
  const notes = useMemo(() => rest.filter((g) => g.ask.kind !== "broke"), [rest]);

  if (groups.length === 0) {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center overflow-y-auto px-6 py-20">
        <div className="flex flex-col items-center gap-2 text-center">
          {!emptyLabel && <CheckCircle2 className="size-8 text-emerald-500/70" />}
          <p className="text-body font-medium">{emptyLabel ?? "Nothing needs you"}</p>
          {!emptyLabel && (
            <p className="max-w-sm text-caption text-muted-foreground">{tc(($) => $.operational_copy.autonomous_empty_description)}</p>
          )}
          {emptyAction && <div className="mt-2">{emptyAction}</div>}
          {archivedCount > 0 && (
            <ClearedLink count={archivedCount} onOpen={onOpenArchived} />
          )}
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-0 flex-1 overflow-y-auto">
      {/* Wide enough that a card can put its answers beside its question, and
          still capped so a line of text never runs the width of a 4K screen. */}
      <div className="mx-auto w-full max-w-[1100px] space-y-3 px-5 py-5">
        {batches.map((batch) => (
          <Entry
            key={batch.key}
            batch={batch}
            highlightKey={highlightKey}
            onClearGroup={onClearGroup}
            onLaterGroup={onLaterGroup}
            noticeActions={noticeActions}
            viewerProps={viewerProps}
          />
        ))}

        {(brokeBatches.length > 0 || notes.length > 0) && (
          <div className="flex items-center gap-3 pt-4">
            <h2 className="text-micro font-semibold uppercase tracking-wider text-muted-foreground">{tc(($) => $.operational_copy.worth_knowing)}</h2>
            <div className="h-px flex-1 bg-border" />
            <button
              type="button"
              onClick={() => onClearMany(rest)}
              className="text-caption text-muted-foreground underline-offset-2 hover:text-foreground hover:underline"
            >{tc(($) => $.operational_copy.clear_all)}</button>
          </div>
        )}

        {brokeBatches.map((batch) => (
          <Entry
            key={batch.key}
            batch={batch}
            highlightKey={highlightKey}
            onClearGroup={onClearGroup}
            onLaterGroup={onLaterGroup}
            noticeActions={noticeActions}
            viewerProps={viewerProps}
          />
        ))}

        {notes.length > 0 && (
          <div className="divide-y rounded-xl border bg-card">
            {notes.map((group) => (
              <OwnerInboxNote key={group.key} group={group} onClear={() => onClearGroup(group)} />
            ))}
          </div>
        )}

        {archivedCount > 0 && (
          <div className="pt-2">
            <ClearedLink count={archivedCount} onOpen={onOpenArchived} />
          </div>
        )}
      </div>
    </div>
  );
}

/** One entry in the column: a merged batch, a notice with its own repair
 *  controls, or a single piece of work. */
function Entry({
  batch,
  highlightKey,
  onClearGroup,
  onLaterGroup,
  noticeActions,
  viewerProps,
}: {
  batch: ReturnType<typeof batchOwnerInbox>[number];
  highlightKey: string;
  onClearGroup: (group: OwnerInboxGroup) => void;
  onLaterGroup: (group: OwnerInboxGroup) => void;
  noticeActions?: (item: InboxItem) => React.ReactNode;
  viewerProps: (group: OwnerInboxGroup) => {
    viewer?: { sourceId?: string; position: { index: number; total: number } };
    onOpenViewer: (sourceId?: string) => void;
    onCloseViewer: () => void;
    onStepViewer: (delta: number) => void;
  };
}) {
  if (batch.shared) {
    return (
      <OwnerInboxBatchCard
        batch={batch}
        onClear={onClearGroup}
        onLater={onLaterGroup}
        viewerProps={viewerProps}
      />
    );
  }
  const group = batch.groups[0]!;
  if (!group.issueId) {
    return (
      <OwnerInboxNoticeCard
        group={group}
        onClear={() => onClearGroup(group)}
        extras={noticeActions?.(group.lead)}
      />
    );
  }
  return (
    <OwnerInboxCard
      group={group}
      highlighted={group.items.some((i) => i.id === highlightKey)}
      onClear={() => onClearGroup(group)}
      onLater={() => onLaterGroup(group)}
      {...viewerProps(group)}
    />
  );
}

function ClearedLink({ count, onOpen }: { count: number; onOpen: () => void }) {
  const { t: tc } = useT("inbox");
  return (
    <Button variant="ghost" size="sm" onClick={onOpen} className="gap-2 text-muted-foreground">
      <Archive className="size-4" />{tc(($) => $.operational_copy.cleared)}<span className="tabular-nums">{count}</span>
      <ChevronRight className="size-4" />
    </Button>
  );
}
