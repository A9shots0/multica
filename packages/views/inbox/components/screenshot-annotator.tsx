"use client";

import { useRef, useState, type PointerEvent as ReactPointerEvent, type ReactNode } from "react";
import { useT } from "../../i18n";
import { cn } from "@multica/ui/lib/utils";

/** One rectangle the owner drew, in fractions of the thing it was drawn on so
 *  it survives resizing and means the same at any scale. */
export interface Mark {
  id: number;
  /** The picture or page it belongs to. */
  shot: string;
  x: number;
  y: number;
  w: number;
  h: number;
  note: string;
}

type Corner = "nw" | "ne" | "sw" | "se";
type Point = { x: number; y: number };
type Box = { x: number; y: number; w: number; h: number };

/** Smallest a mark may shrink to while resizing — big enough to still grab. */
const MIN_SIZE = 0.02;

function clampBox(b: Box): Box {
  const w = Math.min(Math.max(b.w, MIN_SIZE), 1);
  const h = Math.min(Math.max(b.h, MIN_SIZE), 1);
  return {
    w,
    h,
    x: Math.min(Math.max(b.x, 0), 1 - w),
    y: Math.min(Math.max(b.y, 0), 1 - h),
  };
}

/** A box dragged from `origin` by a corner: the OPPOSITE corner stays put,
 *  the dragged one follows the pointer. */
function resizeBox(origin: Box, corner: Corner, p: Point): Box {
  const anchorX = corner === "ne" || corner === "se" ? origin.x : origin.x + origin.w;
  const anchorY = corner === "sw" || corner === "se" ? origin.y : origin.y + origin.h;
  return clampBox({
    x: Math.min(anchorX, p.x),
    y: Math.min(anchorY, p.y),
    w: Math.abs(p.x - anchorX),
    h: Math.abs(p.y - anchorY),
  });
}

function moveBox(origin: Box, dx: number, dy: number): Box {
  const w = origin.w;
  const h = origin.h;
  return {
    w,
    h,
    x: Math.min(Math.max(origin.x + dx, 0), 1 - w),
    y: Math.min(Math.max(origin.y + dy, 0), 1 - h),
  };
}

/**
 * Drag-to-circle over anything: a screenshot, or a live page in an iframe.
 * Once drawn, a mark is a real object on the surface — click it to select,
 * drag its body to move it, drag a corner to resize it — not a one-shot
 * rectangle the owner has to delete and redraw to fix.
 *
 * The owner previously had one free-text box and had to describe a visual
 * problem in words — "the right-hand panel is too narrow at the width I use" —
 * slow to write and ambiguous to read. A box drawn over the thing plus a short
 * note is faster for them and unambiguous for whoever picks it up.
 *
 * Marks are fractions of the surface rather than pixels, so the same mark means
 * the same region on a phone and on a 1440px screen, and stays correct if the
 * picture is re-rendered at another size.
 *
 * `enabled` exists because the same surface hosts a LIVE page: while it is off,
 * clicks and scrolls reach the page so the owner can use it, and only when it
 * is on does this layer take the pointer to draw.
 */
export function MarkSurface({
  surfaceId,
  marks,
  onChange,
  enabled,
  activeMarkId,
  children,
  editor,
  className,
  onSelectMark,
  onDismissActive,
}: {
  surfaceId: string;
  marks: readonly Mark[];
  onChange: (marks: Mark[]) => void;
  enabled: boolean;
  /** The one pin expanded back into its rectangle. Every other mark is a pin. */
  activeMarkId: number | null;
  children: ReactNode;
  /** Compact comment controls anchored beside the active rectangle. */
  editor?: ReactNode;
  className?: string;
  onSelectMark?: (mark: Mark) => void;
  onDismissActive?: () => void;
}) {
  const { t } = useT("inbox");
  const [drawing, setDrawing] = useState<Box | null>(null);
  const frame = useRef<HTMLDivElement | null>(null);
  const start = useRef<Point | null>(null);
  const activePointer = useRef<number | null>(null);
  // What a drag on an EXISTING mark is doing, captured at the moment it
  // starts: which mark, whether it's a move or a resize (and which corner),
  // its own box before this drag touched it, and where the pointer began —
  // every subsequent move is computed from this fixed origin, never from the
  // mark's own live (already-moving) box, so small, fast pointer jitters
  // don't compound into a mark that drifts faster than the cursor.
  const interaction = useRef<{
    id: number;
    kind: "move" | "resize";
    corner?: Corner;
    origin: Box;
    start: Point;
  } | null>(null);
  const mine = marks.filter((m) => m.shot === surfaceId);
  const activeMark = mine.find((mark) => mark.id === activeMarkId);

  // Pointer Events keep mouse, pen, and touch on one path. Pointer capture is
  // essential here: without it, a quick drag that leaves the image silently
  // ends or loses the final position, which made marking feel unreliable.
  const point = (source: { clientX: number; clientY: number }): Point | null => {
    const box = frame.current?.getBoundingClientRect();
    if (!box || box.width === 0 || box.height === 0) return null;
    return {
      x: Math.min(Math.max((source.clientX - box.left) / box.width, 0), 1),
      y: Math.min(Math.max((source.clientY - box.top) / box.height, 0), 1),
    };
  };

  const replaceMark = (id: number, box: Box) => {
    onChange(marks.map((m) => (m.id === id ? { ...m, ...box } : m)));
  };

  const capture = (event: ReactPointerEvent<HTMLElement>) => {
    activePointer.current = event.pointerId;
    event.currentTarget.setPointerCapture?.(event.pointerId);
  };

  const beginCreate = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (!enabled || (event.pointerType === "mouse" && event.button !== 0)) return;
    if (activeMarkId !== null) {
      onDismissActive?.();
      return;
    }
    const p = point(event);
    if (!p) return;
    capture(event);
    start.current = p;
    setDrawing({ ...p, w: 0, h: 0 });
  };

  const beginMove = (id: number, box: Box, event: ReactPointerEvent<HTMLDivElement>) => {
    const p = point(event);
    if (!p) return;
    capture(event);
    interaction.current = { id, kind: "move", origin: box, start: p };
  };

  const beginResize = (
    id: number,
    corner: Corner,
    box: Box,
    event: ReactPointerEvent<HTMLDivElement>,
  ) => {
    const p = point(event);
    if (!p) return;
    capture(event);
    interaction.current = { id, kind: "resize", corner, origin: box, start: p };
  };

  const extend = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (activePointer.current !== event.pointerId) return;
    const p = point(event);
    if (!p) return;
    if (interaction.current) {
      const { id, kind, corner, origin } = interaction.current;
      const next =
        kind === "resize" && corner
          ? resizeBox(origin, corner, p)
          : moveBox(origin, p.x - interaction.current.start.x, p.y - interaction.current.start.y);
      replaceMark(id, next);
      return;
    }
    if (!enabled || !start.current) return;
    setDrawing({
      x: Math.min(p.x, start.current.x),
      y: Math.min(p.y, start.current.y),
      w: Math.abs(p.x - start.current.x),
      h: Math.abs(p.y - start.current.y),
    });
  };

  const finish = (event: ReactPointerEvent<HTMLElement>) => {
    if (activePointer.current !== event.pointerId) return;
    if (interaction.current) {
      interaction.current = null;
    } else if (drawing && drawing.w > 0.015 && drawing.h > 0.015) {
      // A click with no drag is not an annotation; without this every stray
      // click would leave a zero-size mark the owner then has to delete.
      const id = marks.reduce((max, m) => Math.max(max, m.id), 0) + 1;
      onChange([...marks, { id, shot: surfaceId, ...drawing, note: "" }]);
    }
    setDrawing(null);
    start.current = null;
    activePointer.current = null;
    if (event.currentTarget.hasPointerCapture?.(event.pointerId)) {
      event.currentTarget.releasePointerCapture?.(event.pointerId);
    }
  };

  return (
    <div
      ref={frame}
      // `cn()`, not string concatenation: a caller passing `w-full` (the live
      // page in an iframe) was landing in the class list ALONGSIDE this div's
      // own default `w-fit`, and which one actually won depended on Tailwind's
      // generated stylesheet order, not on the order the classes were written
      // in — an invisible, arbitrary coin flip that made the marking overlay's
      // own box not reliably match the thing underneath it.
      className={cn("relative w-fit select-none", className)}
    >
      {children}

      {/* Saved annotations stay discoverable as small pins. Only the active
          pin expands into a rectangle, matching the review interaction the
          owner already knows instead of choosing between every box and none. */}
      <div
        data-testid="mark-surface-overlay"
        className={cn(
          "absolute inset-0",
          enabled ? "cursor-crosshair touch-none" : "pointer-events-none",
        )}
        onPointerDown={beginCreate}
        onPointerMove={extend}
        onPointerUp={finish}
        onPointerCancel={finish}
      >
        {activeMark && (
            <div
              data-testid={`annotation-region-${activeMark.id}`}
              data-annotation-control="true"
              onPointerDown={
                enabled
                  ? (event) => {
                      // Stops here: without this, grabbing an existing mark
                      // to move it ALSO reached the overlay behind it and
                      // started drawing a brand new one on top — every
                      // attempt to adjust a mark instead created another.
                      event.stopPropagation();
                      beginMove(activeMark.id, activeMark, event);
                    }
                  : undefined
              }
              className={cn(
                "absolute rounded border-2 border-brand bg-brand/10",
                enabled && "cursor-move touch-none",
              )}
              style={{
                left: `${activeMark.x * 100}%`,
                top: `${activeMark.y * 100}%`,
                width: `${activeMark.w * 100}%`,
                height: `${activeMark.h * 100}%`,
              }}
            >
              {enabled &&
                (["nw", "ne", "sw", "se"] as const).map((corner) => (
                  <div
                    key={corner}
                    onPointerDown={(event) => {
                      event.stopPropagation();
                      beginResize(activeMark.id, corner, activeMark, event);
                    }}
                    className={cn(
                      "absolute size-5 touch-none rounded-full border-2 border-background bg-brand",
                      corner === "nw" && "-left-3 -top-3 cursor-nwse-resize",
                      corner === "ne" && "-right-3 -top-3 cursor-nesw-resize",
                      corner === "sw" && "-bottom-3 -left-3 cursor-nesw-resize",
                      corner === "se" && "-bottom-3 -right-3 cursor-nwse-resize",
                    )}
                  />
                ))}
            </div>
        )}

        {mine.map((mark) => (
          <button
            key={mark.id}
            type="button"
            data-testid={`annotation-pin-${mark.id}`}
            data-annotation-control="true"
            aria-label={t(($) => $.preview.pin, { id: mark.id, note: mark.note || t(($) => $.preview.add_feedback) })}
            onPointerDown={(event) => event.stopPropagation()}
            onClick={() => onSelectMark?.(mark)}
            className={cn(
              "pointer-events-auto absolute z-20 flex size-7 -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-full border-2 border-background bg-brand text-[11px] font-semibold text-brand-foreground shadow-md outline-none transition-transform hover:scale-105 focus-visible:ring-2 focus-visible:ring-ring",
              mark.id === activeMarkId && "ring-2 ring-ring",
            )}
            style={{ left: `${mark.x * 100}%`, top: `${(mark.y + mark.h) * 100}%` }}
          >
            {mark.id}
          </button>
        ))}

        {activeMark && editor && (
          <div
            data-annotation-control="true"
            className={cn(
              "pointer-events-auto absolute left-2 right-2 z-30 flex",
              activeMark.x + activeMark.w / 2 < 0.34
                ? "justify-start"
                : activeMark.x + activeMark.w / 2 > 0.66
                  ? "justify-end"
                  : "justify-center",
            )}
            style={
              activeMark.y + activeMark.h > 0.72
                ? {
                    bottom: `${(1 - activeMark.y) * 100}%`,
                    marginBottom: 12,
                  }
                : {
                    top: `${(activeMark.y + activeMark.h) * 100}%`,
                    marginTop: 12,
                  }
            }
          >
            <div className="w-full max-w-[22rem]">{editor}</div>
          </div>
        )}

        {enabled && drawing && (
          <div
            className="pointer-events-none absolute rounded border-2 border-dashed border-amber-300"
            style={{
              left: `${drawing.x * 100}%`,
              top: `${drawing.y * 100}%`,
              width: `${drawing.w * 100}%`,
              height: `${drawing.h * 100}%`,
            }}
          />
        )}
      </div>
    </div>
  );
}

/** Renders marks as the text an agent reads. Position is given in plain words
 *  because "top-left" is actionable to a reader and "x: 0.12" is not. */
export function describeMarks(
  marks: readonly Mark[],
  names: ReadonlyMap<string, string>,
): string {
  if (marks.length === 0) return "";
  const where = (m: Mark) => {
    const vertical = m.y + m.h / 2 < 0.34 ? "top" : m.y + m.h / 2 > 0.66 ? "bottom" : "middle";
    const horizontal = m.x + m.w / 2 < 0.34 ? "left" : m.x + m.w / 2 > 0.66 ? "right" : "centre";
    return vertical === "middle" && horizontal === "centre" ? "centre" : `${vertical} ${horizontal}`;
  };
  return marks
    .map(
      (m) =>
        `${m.id}. ${names.get(m.shot) ?? "screenshot"}, ${where(m)}: ${m.note.trim() || "this needs changing"}`,
    )
    .join("\n");
}
