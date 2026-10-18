"use client";

import { useState } from "react";
import { ChevronRight } from "lucide-react";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@multica/ui/components/ui/collapsible";
import { RichContent } from "../../rich-content";
import type { HubDocSection } from "../hub-doc-sections";

/** Splits a changelog entry's "2026-09-22 - Title" (or em-dash) label into
 * its date and plain-English title, for a scannable headline list — the
 * dense technical paragraph stays one click away instead of being the
 * default view. */
function splitEntryLabel(label: string): { date: string; title: string } {
  const match = label.match(/^(\S+)\s*[-—]\s*(.+)$/);
  if (!match) return { date: "", title: label };
  return { date: match[1]!, title: match[2]!.trim() };
}

function ChangelogEntryRow({ entry }: { entry: HubDocSection }) {
  const [open, setOpen] = useState(false);
  const { date, title } = splitEntryLabel(entry.label);

  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <CollapsibleTrigger className="flex w-full items-start gap-2 py-2.5 text-left hover:bg-muted/50 rounded-md px-2 -mx-2">
        <ChevronRight
          className={`mt-0.5 size-3.5 shrink-0 text-muted-foreground transition-transform ${open ? "rotate-90" : ""}`}
        />
        <div className="min-w-0 flex-1">
          <p className="text-body">{title}</p>
          {date && <p className="text-micro text-muted-foreground">{date}</p>}
        </div>
      </CollapsibleTrigger>
      <CollapsibleContent>
        <div className="pb-3 pl-[22px] pr-2">
          <RichContent content={entry.content} density="compact" phase="settled" />
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
}

/**
 * Headline-first changelog: each entry collapses to its date + plain-English
 * title, expanding only on click to show the full technical write-up. The
 * previous version of this page rendered every entry's full body at once —
 * rejected as "an endless wall of .md" by the owner, who wants a scannable
 * timeline, not a document to read front to back.
 */
export function HubChangelogTimeline({
  entries,
  limit,
}: {
  entries: HubDocSection[];
  limit?: number;
}) {
  const visible = limit ? entries.slice(0, limit) : entries;
  return (
    <div className="divide-y">
      {visible.map((entry) => (
        <ChangelogEntryRow key={entry.id} entry={entry} />
      ))}
    </div>
  );
}
