"use client";

import { useRef } from "react";
import { Info } from "lucide-react";
import { MulticaIcon } from "@multica/ui/components/common/multica-icon";
import {
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverHeader,
  PopoverTitle,
  PopoverTrigger,
} from "@multica/ui/components/ui/popover";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../i18n";

// Upstream project the control plane is built on. The Multica License keeps the
// logo, product name, copyright and attribution unmodified, so this surface adds
// the Silver Mediums layer beside them rather than in place of them.
const MULTICA_PROJECT_URL = "https://github.com/multica-ai/multica";

// Ordered so the first three answer "can I trust what I am looking at", the
// next three "who is allowed to do what", and the last three "what happens when
// something goes wrong or costs money".
const ABOUT_POINT_KEYS = [
  "truthful_states",
  "routing",
  "shared_knowledge",
  "bounded_permissions",
  "independent_review",
  "release_verification",
  "recovery",
  "owner_inbox",
  "usage",
] as const;

export function ControlPlaneBadge({ className }: { className?: string }) {
  const { t } = useT("layout");
  const headingRef = useRef<HTMLDivElement>(null);
  const productName = t(($) => $.control_plane.name);
  const builtOn = t(($) => $.control_plane.built_on);

  return (
    <div
      className={cn(
        // The row must survive a 200px sidebar, so the label column is allowed
        // to shrink below its content width and truncate rather than push the
        // info trigger out of the panel.
        "flex min-w-0 items-center gap-2 rounded-md px-2 py-1.5",
        className,
      )}
    >
      {/* The Multica mark sits on the "Built on Multica" line, not beside the
          layer name, so the logo is never read as branding this layer. Both
          lines truncate at narrow widths, so each carries its full text as a
          title for hover and assistive technology. */}
      <div className="flex min-w-0 flex-1 flex-col leading-tight">
        <span
          className="truncate text-caption font-medium text-sidebar-foreground"
          title={productName}
        >
          {productName}
        </span>
        <span className="flex min-w-0 items-center gap-1.5 text-caption text-muted-foreground">
          <MulticaIcon noSpin size="sm" />
          <span className="truncate" title={builtOn}>
            {builtOn}
          </span>
        </span>
      </div>
      <Popover>
        <PopoverTrigger
          aria-label={t(($) => $.control_plane.about_trigger)}
          title={t(($) => $.control_plane.about_trigger)}
          className="inline-flex size-7 shrink-0 items-center justify-center rounded-full text-muted-foreground transition-colors cursor-pointer hover:bg-accent hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50 focus-visible:outline-hidden data-popup-open:bg-accent data-popup-open:text-foreground"
        >
          <Info className="size-4" />
        </PopoverTrigger>
        <PopoverContent
          align="end"
          side="top"
          sideOffset={8}
          // Wide enough to read, never wider than a phone-width viewport.
          className="max-h-[min(28rem,80vh)] w-[min(20rem,calc(100vw-2rem))] overflow-y-auto"
          // The panel is taller than its own scroll box, and the default
          // initial focus lands on the project link at the very bottom, which
          // scrolls the explanation out of view before it has been read. Focus
          // the heading instead so it opens at the top and still keeps focus
          // inside the panel for Tab and Escape.
          initialFocus={headingRef}
        >
          <PopoverHeader ref={headingRef} tabIndex={-1} className="outline-hidden">
            <PopoverTitle>{productName}</PopoverTitle>
            <PopoverDescription>
              {t(($) => $.control_plane.about_intro)}
            </PopoverDescription>
          </PopoverHeader>
          <dl className="flex flex-col gap-2">
            {ABOUT_POINT_KEYS.map((key) => (
              <div key={key} className="flex flex-col gap-0.5">
                <dt className="text-caption font-medium text-foreground">
                  {t(($) => $.control_plane.points[key].title)}
                </dt>
                <dd className="text-caption text-muted-foreground">
                  {t(($) => $.control_plane.points[key].body)}
                </dd>
              </div>
            ))}
          </dl>
          <div className="flex flex-col gap-1 border-t border-surface-border pt-2">
            <span className="text-caption text-muted-foreground">
              {t(($) => $.control_plane.attribution)}
            </span>
            <span className="text-caption text-muted-foreground">
              {t(($) => $.control_plane.copyright)}
            </span>
            <a
              href={MULTICA_PROJECT_URL}
              target="_blank"
              rel="noreferrer"
              className="text-caption text-muted-foreground underline underline-offset-2 hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50 focus-visible:outline-hidden"
            >
              {MULTICA_PROJECT_URL}
            </a>
          </div>
        </PopoverContent>
      </Popover>
    </div>
  );
}
