"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { ChevronLeft, ChevronRight, ExternalLink, Trash2, Undo2, X } from "lucide-react";
import { attachmentDownloadPath, type Attachment } from "@multica/core/types";
import type { NameableSource } from "@multica/core/inbox/owner-inbox-model";
import {
  attachmentPreviewId,
  describeSources,
  framingBlocked,
  sandboxFor,
} from "@multica/core/inbox/owner-inbox-model";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../../i18n";
import { HtmlPreviewBody } from "../../editor/html-preview-body";
import { MarkSurface, type Mark } from "./screenshot-annotator";

/** One thing that can be opened full screen. The same shape the naming in core
 *  works on, so a picture is called the same thing everywhere it appears. */
export type PreviewSource = NameableSource;

/** Full-screen media and feedback. Decisions stay in the surrounding review. */
export function PreviewViewer({
  sources,
  startId,
  marks,
  onChangeMarks,
  onClose,
  title,
  question,
  position,
  onStep,
  allowMarking = true,
  primaryAction,
}: {
  sources: PreviewSource[];
  startId?: string;
  marks: Mark[];
  onChangeMarks: (marks: Mark[]) => void;
  onClose: () => void;
  title: string;
  /** Context to show if there is no media. */
  question?: string;
  /** Where this sits in the queue — "3 of 7". */
  position?: { index: number; total: number };
  /** Walk to the previous or next thing without leaving full screen. */
  onStep?: (delta: number) => void;
  /** False for read-only review surfaces that do not yet own a feedback draft. */
  allowMarking?: boolean;
  /** Optional owning-surface action, e.g. place completed annotations in Chat. */
  primaryAction?: {
    label: string;
    disabled?: boolean;
    pending?: boolean;
    onClick: () => void | Promise<void>;
  };
}) {
  const { t } = useT("inbox");
  const [activeId, setActiveId] = useState(startId || sources[0]?.id || "");
  const [marking, setMarking] = useState(false);
  const [notesOpen, setNotesOpen] = useState(false);
  const [editingNoteId, setEditingNoteId] = useState<number | null>(null);
  const [editingOriginalNote, setEditingOriginalNote] = useState<string | null>(null);
  const active = sources.find((s) => s.id === activeId) ?? sources[0];
  const mine = useMemo(
    () => (active ? marks.filter((mark) => mark.shot === active.id) : []),
    [active, marks],
  );
  const changeMarks = (next: Mark[]) => {
    const added = next.find((mark) => !marks.some((existing) => existing.id === mark.id));
    onChangeMarks(next);
    if (added) {
      setEditingNoteId(added.id);
      setEditingOriginalNote(null);
      setNotesOpen(false);
    }
  };
  const editingNote = marks.find((mark) => mark.id === editingNoteId && mark.shot === activeId);
  const closeEditing = useCallback(() => {
    setEditingNoteId(null);
    setEditingOriginalNote(null);
  }, []);
  const saveEditing = useCallback(() => {
    if (!editingNote) return;
    if (!editingNote.note.trim()) {
      onChangeMarks(marks.filter((mark) => mark.id !== editingNote.id));
    }
    closeEditing();
  }, [closeEditing, editingNote, marks, onChangeMarks]);
  const cancelEditing = useCallback(() => {
    if (!editingNote) return;
    if (editingOriginalNote === null) {
      onChangeMarks(marks.filter((mark) => mark.id !== editingNote.id));
    } else if (editingNote.note !== editingOriginalNote) {
      onChangeMarks(marks.map((mark) => mark.id === editingNote.id ? { ...mark, note: editingOriginalNote } : mark));
    }
    closeEditing();
  }, [closeEditing, editingNote, editingOriginalNote, marks, onChangeMarks]);
  const removeEditing = useCallback(() => {
    if (!editingNote) return;
    onChangeMarks(marks.filter((mark) => mark.id !== editingNote.id));
    closeEditing();
  }, [closeEditing, editingNote, marks, onChangeMarks]);
  const selectMark = useCallback((mark: Mark) => {
    setEditingNoteId(mark.id);
    setEditingOriginalNote(mark.note);
    setNotesOpen(false);
  }, []);
  const noteEditor = editingNote ? (
    <div className="rounded-2xl border bg-popover p-2 shadow-xl">
      <label htmlFor="preview-note" className="sr-only">{t(($) => $.preview.note, { id: editingNote.id })}</label>
      <textarea
        id="preview-note"
        autoFocus
        rows={editingNote.note ? 2 : 1}
        value={editingNote.note}
        placeholder={t(($) => $.preview.placeholder)}
        onChange={(event) => onChangeMarks(marks.map((mark) => mark.id === editingNote.id ? { ...mark, note: event.target.value } : mark))}
        onKeyDown={(event) => {
          if (event.key === "Escape") {
            event.stopPropagation();
            cancelEditing();
          } else if (
            event.key === "Enter" &&
            !event.shiftKey &&
            !event.nativeEvent.isComposing
          ) {
            event.preventDefault();
            saveEditing();
          }
        }}
        className="max-h-28 min-h-10 w-full resize-none bg-transparent px-2 py-1.5 text-body outline-none placeholder:text-muted-foreground"
      />
      {editingNote.note.trim() && (
        <div className="flex items-center justify-end gap-1 border-t pt-2">
          <Button size="icon-sm" variant="ghost" aria-label={t(($) => $.preview.remove)} onClick={removeEditing}><Trash2 className="size-4" /></Button>
          <Button size="sm" variant="ghost" onClick={cancelEditing}>{t(($) => $.preview.cancel)}</Button>
          <Button size="sm" onClick={saveEditing}>{t(($) => $.preview.save)}</Button>
        </div>
      )}
    </div>
  ) : null;
  const activeIndex = active ? Math.max(0, sources.findIndex((source) => source.id === active.id)) : 0;
  const canMarkActive = allowMarking && active?.kind === "image";
  const names = useMemo(() => describeSources(sources), [sources]);
  const nameOf = useCallback(
    (source: PreviewSource) => names.get(source.id) ?? source.label,
    [names],
  );

  const shell = useRef<HTMLDivElement | null>(null);
  const area = useRef<HTMLDivElement | null>(null);
  const natural = useRef<{ w: number; h: number } | null>(null);
  const [box, setBox] = useState<{ w: number; h: number } | null>(null);

  // The picture is sized explicitly rather than left to object-fit, because the
  // box the marks are measured against has to stay exactly the box the picture
  // occupies. Enlarging stops at one source pixel per device pixel: a 390-wide
  // capture may reach 585 CSS pixels on a 1.5x display and still be sharp, and
  // must not be dragged out to 1100 just because the space is there.
  const fit = useCallback(() => {
    const size = natural.current;
    const bounds = area.current?.getBoundingClientRect();
    if (!size || !bounds || size.w === 0 || size.h === 0) return;
    const room = Math.min(bounds.width / size.w, bounds.height / size.h);
    const sharp = Math.max(1, typeof window === "undefined" ? 1 : window.devicePixelRatio || 1);
    const scale = Math.min(room, sharp);
    setBox({ w: Math.round(size.w * scale), h: Math.round(size.h * scale) });
  }, []);

  useEffect(() => {
    natural.current = null;
    setBox(null);
    setMarking(false);
    setNotesOpen(false);
    setEditingNoteId(null);
    setEditingOriginalNote(null);
  }, [activeId]);

  useEffect(() => {
    if (sources.length > 0 && !sources.some((source) => source.id === activeId)) {
      setActiveId(sources[0]?.id ?? "");
    }
  }, [activeId, sources]);

  const finishMarking = useCallback(() => {
    const incomplete = mine.find((mark) => !mark.note.trim());
    if (incomplete) {
      onChangeMarks(marks.filter((mark) => mark.id !== incomplete.id));
    }
    setMarking(false);
    setNotesOpen(false);
    setEditingNoteId(null);
    setEditingOriginalNote(null);
  }, [marks, mine, onChangeMarks]);

  const undoLastMark = () => {
    const latest = mine.reduce<Mark | null>((last, mark) => (!last || mark.id > last.id ? mark : last), null);
    if (!latest) return;
    onChangeMarks(marks.filter((mark) => mark.id !== latest.id));
    if (editingNoteId === latest.id) closeEditing();
  };

  useEffect(() => {
    if (!editingNote) return;
    const dismiss = (event: PointerEvent) => {
      const target = event.target;
      if (target instanceof Element && target.closest('[data-annotation-control="true"]')) return;
      saveEditing();
    };
    document.addEventListener("pointerdown", dismiss);
    return () => document.removeEventListener("pointerdown", dismiss);
  }, [editingNote, saveEditing]);

  // Watches the area itself, not just the window: the panel carrying the
  // question takes width or height off this box depending on how wide the
  // screen is, and measuring once left the picture hanging off the edge.
  useEffect(() => {
    window.addEventListener("resize", fit);
    const node = area.current;
    const observer =
      node && typeof ResizeObserver !== "undefined" ? new ResizeObserver(fit) : null;
    if (node && observer) observer.observe(node);
    return () => {
      window.removeEventListener("resize", fit);
      observer?.disconnect();
    };
  }, [fit]);

  // Arrows move within the thing you are looking at — the next picture of the
  // same layout — because that is what a left/right key means when six captures
  // are open. j/k and PageUp/PageDown walk the queue of work. A field with
  // focus keeps its own keys.
  const stepSource = useCallback(
    (delta: number) => {
      if (sources.length < 2) return false;
      const from = sources.findIndex((s) => s.id === activeId);
      const next = sources[(from + delta + sources.length) % sources.length];
      if (!next) return false;
      setActiveId(next.id);
      return true;
    },
    [sources, activeId],
  );

  /**
   * Focus lives on the shell, not in the page being previewed.
   *
   * A live preview is a real page in a frame, and once it has the keyboard it
   * keeps every keystroke: arrows stopped moving between pictures the moment
   * the owner landed on one, with nothing on screen saying why. Taking focus
   * back whenever the picture changes fixes the common case, and the arrows
   * beside the picture mean the keyboard is never the only way through.
   */
  useEffect(() => {
    shell.current?.focus({ preventScroll: true });
  }, [activeId]);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        if (editingNoteId !== null) {
          event.preventDefault();
          cancelEditing();
          return;
        }
        if (notesOpen || marking) {
          event.preventDefault();
          setNotesOpen(false);
          setMarking(false);
          return;
        }
        onClose();
        return;
      }
      const target = event.target as HTMLElement | null;
      if (target && (target.tagName === "INPUT" || target.tagName === "TEXTAREA")) return;
      const delta =
        event.key === "ArrowRight" || event.key === "ArrowDown"
          ? 1
          : event.key === "ArrowLeft" || event.key === "ArrowUp"
            ? -1
            : 0;
      if (delta !== 0) {
        event.preventDefault();
        if (!stepSource(delta)) onStep?.(delta);
        return;
      }
      if (event.key === "j" || event.key === "PageDown") onStep?.(1);
      if (event.key === "k" || event.key === "PageUp") onStep?.(-1);
    };
    document.addEventListener("keydown", onKey);
    const previous = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = previous;
    };
  }, [cancelEditing, editingNoteId, marking, notesOpen, onClose, onStep, stepSource]);

  // Restore focus only when the viewer closes, not after each parent draft update.
  useEffect(() => {
    const returnTo = document.activeElement as HTMLElement | null;
    return () => { returnTo?.focus?.({ preventScroll: true }); };
  }, []);

  if (typeof document === "undefined") return null;

  const origin = typeof window === "undefined" ? undefined : window.location.origin;
  const blocked = active?.kind === "page" && framingBlocked(active.url, origin);
  const attachedPageId = active?.kind === "page"
    ? attachmentPreviewId(active.url, origin)
    : null;

  return createPortal(
    <div
      ref={shell}
      tabIndex={-1}
      role="dialog"
      aria-modal="true"
      aria-label={title}
      className="fixed inset-0 z-50 flex flex-col bg-background outline-none"
      data-testid="preview-viewer"
    >
      <header className="flex shrink-0 items-center gap-2 border-b px-3 py-2.5 sm:gap-3 sm:px-4">
        {onStep && (
          <div className="flex shrink-0 items-center gap-1">
            <Button
              size="icon-sm"
              variant="ghost"
              onClick={() => onStep(-1)}
              aria-label={t(($) => $.preview.previous_work)}
              title={t(($) => $.preview.previous_work_key)}
            >
              <ChevronLeft className="size-4" />
            </Button>
            {position && (
              <span className="text-caption tabular-nums text-muted-foreground">
                {t(($) => $.preview.position, { current: position.index + 1, total: position.total })}
              </span>
            )}
            <Button
              size="icon-sm"
              variant="ghost"
              onClick={() => onStep(1)}
              aria-label={t(($) => $.preview.next_work)}
              title={t(($) => $.preview.next_work_key)}
            >
              <ChevronRight className="size-4" />
            </Button>
          </div>
        )}
        <span className="min-w-0 flex-1 truncate text-body font-semibold">{title}</span>
        {active && canMarkActive && (
          <Button
            size="sm"
            variant={marking ? "brand" : "outline"}
            className="shrink-0"
            onClick={() => {
              if (marking) finishMarking();
              else {
                closeEditing();
                setNotesOpen(false);
                setMarking(true);
              }
            }}
          >
            {marking ? t(($) => $.preview.done_marking) : t(($) => $.preview.mark_it_up)}
          </Button>
        )}
        {marking && mine.length > 0 && (
          <Button size="sm" variant="ghost" className="shrink-0" onClick={undoLastMark}>
            <Undo2 className="mr-1.5 size-4" />
            {t(($) => $.preview.undo_last)}
          </Button>
        )}
        {sources.length > 1 && (
          <span className="shrink-0 text-caption tabular-nums text-muted-foreground">
            {t(($) => $.preview.result_position, { current: activeIndex + 1, total: sources.length })}
          </span>
        )}
        {primaryAction && (
          <Button
            size="sm"
            className="shrink-0"
            disabled={primaryAction.disabled || primaryAction.pending}
            onClick={() => {
              closeEditing();
              setMarking(false);
              void primaryAction.onClick();
            }}
          >
            {primaryAction.pending ? `${primaryAction.label}…` : primaryAction.label}
          </Button>
        )}
        <Button size="sm" variant="ghost" onClick={() => { closeEditing(); setNotesOpen((open) => !open); }}>{t(($) => $.preview.notes, { count: mine.length })}</Button>
        <Button size="sm" variant="ghost" className="shrink-0" onClick={onClose} aria-label={t(($) => $.preview.close)}>
          <X className="size-4" />
        </Button>
      </header>

      {/* Optional notes stay bounded beside or below the media. */}
      <div className="flex min-h-0 flex-1 flex-col xl:flex-row">
        <div className="flex min-h-0 min-w-0 flex-1 flex-col">
          <div
            ref={area}
            className="relative flex min-h-0 flex-1 items-center justify-center overflow-hidden bg-muted/20 p-3"
          >
            {/* Always on screen, whatever has the keyboard. */}
            {sources.length > 1 && (
              <>
                <StageArrow side="left" onClick={() => stepSource(-1)} />
                <StageArrow side="right" onClick={() => stepSource(1)} />
              </>
            )}
            {marking && (
              <div className="pointer-events-none absolute left-1/2 top-3 z-20 -translate-x-1/2 rounded-full border bg-background/95 px-3 py-1.5 text-caption font-medium shadow-sm">
                {t(($) => $.preview.drag_to_annotate)}
              </div>
            )}

            {/* Keep context visible when there is no attachment. */}
            {!active ? (
              <p className="max-w-2xl whitespace-pre-wrap text-center text-title leading-relaxed">
                {question}
              </p>
            ) : active.kind === "image" ? (
              <MarkSurface
                surfaceId={active.id}
                marks={marks}
                onChange={changeMarks}
                enabled={canMarkActive && marking}
                activeMarkId={editingNoteId}
                editor={noteEditor}
                onSelectMark={selectMark}
                onDismissActive={saveEditing}
              >
                <img
                  src={active.url}
                  alt={nameOf(active)}
                  className="block"
                  style={box ? { width: box.w, height: box.h } : undefined}
                  onLoad={(event) => {
                    natural.current = {
                      w: event.currentTarget.naturalWidth,
                      h: event.currentTarget.naturalHeight,
                    };
                    fit();
                  }}
                  draggable={false}
                />
              </MarkSurface>
            ) : attachedPageId ? (
              <div className="h-full w-full">
                <HtmlPreviewBody
                  source={{ kind: "attachment", attachmentId: attachedPageId }}
                  title={nameOf(active)}
                  className="h-full"
                  iframeClassName="h-full w-full rounded-none border-0"
                  placeholderClassName="h-full"
                />
              </div>
            ) : blocked ? (
              <BlockedPage url={active.url} />
            ) : (
              <div className="h-full w-full">
                {/* A preview address comes out of whatever an agent typed into
                    its question, so the page it loads is not trusted with the
                    tab it is loaded into: no allow-top-navigation, and no
                    referrer carrying the inbox address out to it. */}
                <iframe
                  src={active.url}
                  title={nameOf(active)}
                  sandbox={sandboxFor(active.url, origin)}
                  referrerPolicy="no-referrer"
                  className="h-full w-full rounded-md border bg-white"
                />
              </div>
            )}
          </div>

        </div>

        {notesOpen && (
          <aside data-testid="preview-note-inspector" aria-label={t(($) => $.preview.notes_on_image)} className="flex max-h-[38vh] shrink-0 flex-col overflow-hidden border-t bg-background p-3 xl:max-h-none xl:w-80 xl:border-l xl:border-t-0">
            <div className="mb-2 flex items-center justify-between"><span className="text-caption font-medium">{t(($) => $.preview.notes_on_image)}</span><Button size="icon-sm" variant="ghost" aria-label={t(($) => $.preview.hide_notes)} onClick={() => setNotesOpen(false)}><X className="size-4" /></Button></div>
            <div className="min-h-0 overflow-y-auto">
              {mine.length === 0 && <p className="py-3 text-caption text-muted-foreground">{t(($) => $.preview.empty_notes)}</p>}
              {mine.map((mark) => <button key={mark.id} type="button" onClick={() => selectMark(mark)} className="flex w-full gap-2 rounded-md px-2 py-2 text-left text-caption hover:bg-accent"><span className="shrink-0 text-muted-foreground">{mark.id}.</span><span className="line-clamp-2">{mark.note || t(($) => $.preview.add_feedback)}</span></button>)}
            </div>
          </aside>
        )}
      </div>
    </div>,
    document.body,
  );
}

/** A big, always-present way between pictures, because the keyboard stops
 *  reaching this component the moment a live page takes focus. */
function StageArrow({ side, onClick }: { side: "left" | "right"; onClick: () => void }) {
  const { t } = useT("inbox");
  const Icon = side === "left" ? ChevronLeft : ChevronRight;
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={side === "left" ? t(($) => $.preview.previous_picture) : t(($) => $.preview.next_picture)}
      className={`absolute top-1/2 z-10 flex size-11 -translate-y-1/2 items-center justify-center rounded-full border bg-background/85 text-foreground shadow-md outline-none backdrop-blur transition-colors hover:bg-background focus-visible:ring-2 focus-visible:ring-ring ${side === "left" ? "left-3" : "right-3"}`}
    >
      <Icon className="size-5" />
    </button>
  );
}

/**
 * A preview the browser will not show, said out loud.
 *
 * The inbox is reached over https from a phone and over plain http at the desk.
 * A preview served over plain http cannot be framed by an https page — every
 * browser refuses, silently — so the same preview works in one place and is an
 * empty rectangle in the other. Opening it in its own tab still works.
 */
function BlockedPage({ url }: { url: string }) {
  const { t } = useT("inbox");
  return (
    <div className="max-w-md space-y-3 text-center">
      <p className="text-body leading-relaxed">{t(($) => $.preview.blocked_title)}</p>
      <p className="text-caption text-muted-foreground">{t(($) => $.preview.blocked_detail)}</p>
      <Button
        variant="outline"
        size="sm"
        render={<a href={url} target="_blank" rel="noreferrer noopener" />}
      >
        <ExternalLink className="size-4" />
        {t(($) => $.preview.open_new_tab)}
      </Button>
    </div>
  );
}

/**
 * Live addresses the owner is being pointed at.
 *
 * Agents write them into the question itself — "the candidate shown at
 * http://100.64.218.108:3135/viral_queue_accounts.html" — rather than into any
 * structured field, so that is where they are read from. A bare URL in prose is
 * not something a non-developer should have to copy into a browser.
 *
 * The address is kept whole as the label: it is what the naming in core reads
 * to work out what the thing is, and it is never what ends up on screen.
 */
export function extractPageLinks(...texts: Array<string | null | undefined>): PreviewSource[] {
  const found = new Map<string, PreviewSource>();
  for (const text of texts) {
    if (!text) continue;
    for (const match of text.matchAll(/https?:\/\/[^\s<>"')\]]+/g)) {
      // Trailing sentence punctuation is part of the prose, not the address.
      const url = match[0].replace(/[.,;:]+$/, "");
      if (found.has(url)) continue;
      try {
        new URL(url);
      } catch {
        continue;
      }
      found.set(url, { id: `page:${url}`, label: url, kind: "page", url });
    }
  }
  return [...found.values()];
}

/**
 * What is attached to the issue, as things the viewer can show.
 *
 * Pictures open as pictures. An attached HTML page opens as a page, live in a
 * frame — agents publish preview pages as attachments, and treating them as
 * unviewable files meant the owner was asked to approve a layout with the
 * layout one download away.
 *
 * Everything else — specs, bundles, zips — is deliberately absent. They are not
 * things to look at, and a tab that opens a .zip is a dead end.
 */
export function attachmentSources(
  attachments: readonly Attachment[],
  excludedAttachmentIds: ReadonlySet<string> = new Set(),
): PreviewSource[] {
  return attachments.flatMap<PreviewSource>((a) => {
    if (excludedAttachmentIds.has(a.id)) return [];
    const type = a.content_type ?? "";
    // The stable, site-relative path rather than `a.url`. `a.url` is the raw
    // storage address, which on this deployment is absolute and carries the
    // hostname the file happened to be uploaded through: every preview in the
    // inbox was being fetched from a fixed machine name regardless of where
    // the inbox itself was being read, and on storage backends that sign it
    // the signature expires in half an hour while a question sits unanswered
    // for a day. This endpoint re-resolves on every request.
    const url = attachmentDownloadPath(a.id);
    if (type.startsWith("image/")) {
      return [{ id: a.id, label: a.filename, kind: "image" as const, url }];
    }
    if (type.startsWith("text/html")) {
      return [{ id: a.id, label: a.filename, kind: "page" as const, url }];
    }
    return [];
  });
}
