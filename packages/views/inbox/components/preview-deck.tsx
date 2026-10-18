"use client";
import { useT } from "../../i18n";

import type { NameableSource } from "@multica/core/inbox/owner-inbox-model";
import { cn } from "@multica/ui/lib/utils";

/**
 * The evidence, at a size worth looking at.
 *
 * This used to be a row of 96×64 thumbnails, cropped to their top edge. On a
 * 1027-pixel card that spent 9% of the width on the only thing the owner was
 * actually being asked to judge and left 600 pixels of the row empty, and the
 * crop meant a phone capture showed its top quarter — the header of a page,
 * never the card being approved. They stayed 96×64 on a phone too, so the
 * narrower the screen got the more of it the previews refused to use.
 *
 * So the first one is as wide as the card and tall enough to read, shown whole
 * rather than cropped, and the rest sit under it as a rail with names. Clicking
 * any of them goes straight to full screen: there is no in-between step,
 * because "look at it properly" is the only reason to touch a preview at all.
 */
export function PreviewDeck({
  sources,
  names,
  onOpen,
}: {
  sources: readonly NameableSource[];
  /** Plain-English name per source id. Filenames and addresses never show. */
  names: ReadonlyMap<string, string>;
  onOpen: (id: string) => void;
}) {
  const { t: tc } = useT("inbox");
  if (sources.length === 0) return null;
  const [lead, ...rest] = sources;
  if (!lead) return null;

  return (
    <div className="space-y-2">
      <button
        type="button"
        onClick={() => onOpen(lead.id)}
        aria-label={tc(($) => $.operational_copy.open_preview_fullscreen, { name: names.get(lead.id) ?? lead.label })}
        className="group relative block w-full overflow-hidden rounded-lg border bg-muted/30 outline-none transition-colors hover:border-primary/60 focus-visible:ring-2 focus-visible:ring-ring"
      >
        {lead.kind === "image" ? (
          // Contained, not cropped, and free to use the whole width of the
          // card. A 1440×900 capture lands about 660 wide here; a portrait
          // phone capture stays portrait instead of being sliced into a strip.
          <img
            src={lead.url}
            alt={names.get(lead.id) ?? lead.label}
            className="mx-auto block max-h-[380px] w-auto max-w-full object-contain"
            draggable={false}
          />
        ) : (
          // No name in here: the caption bar below already carries it, and
          // printing it twice was the first thing that made this box look
          // unfinished rather than simply empty of a picture.
          <LivePageFace />
        )}
        <span className="pointer-events-none absolute inset-x-0 bottom-0 flex items-center justify-between gap-3 bg-gradient-to-t from-background/95 to-transparent px-3 pb-2 pt-6 text-caption">
          <span className="min-w-0 truncate font-medium">{names.get(lead.id) ?? lead.label}</span>
          <span className="shrink-0 rounded-md bg-foreground px-2 py-1 text-[11px] font-semibold text-background">
            {sources.length > 1 ? `Open all ${sources.length}` : "Open full screen"}
          </span>
        </span>
      </button>

      {rest.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {rest.map((source) => (
            <button
              key={source.id}
              type="button"
              onClick={() => onOpen(source.id)}
              aria-label={tc(($) => $.operational_copy.open_preview_fullscreen, { name: names.get(source.id) ?? source.label })}
              className="w-[132px] overflow-hidden rounded-md border bg-muted/30 p-1 text-left outline-none transition-colors hover:border-primary/60 focus-visible:ring-2 focus-visible:ring-ring"
            >
              <span className="block h-[74px] overflow-hidden rounded-sm bg-background/40">
                {source.kind === "image" ? (
                  <img
                    src={source.url}
                    alt=""
                    className="size-full object-contain"
                    draggable={false}
                  />
                ) : (
                  <LivePageFace small />
                )}
              </span>
              <span className="mt-1 block truncate px-1 pb-0.5 text-[11px] text-muted-foreground">
                {names.get(source.id) ?? source.label}
              </span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

/**
 * A running page has no thumbnail to show, so it says what it is instead of
 * rendering as an empty grey box the owner has to click to find out.
 *
 * Says only that it is running — the name sits in the caption bar this shares
 * a card with, and saying it twice is what made a plain "nothing to show
 * here" panel read as unfinished rather than simply empty of a picture.
 */
function LivePageFace({ small }: { small?: boolean }) {
  const { t: tc } = useT("inbox");
  return (
    <span
      className={cn(
        "flex w-full items-center justify-center gap-1.5 bg-muted/40 text-caption font-medium",
        small ? "h-full" : "h-[104px]",
      )}
    >
      <span className="size-1.5 rounded-full bg-emerald-500" />{tc(($) => $.operational_copy.running_now)}</span>
  );
}
