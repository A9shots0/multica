"use client";

import { useEffect, useRef, useState, type PointerEvent, type ReactNode } from "react";
import { Eraser, Hand, MessageSquarePlus, Pencil, Undo2, Check, MessageSquare } from "lucide-react";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@multica/ui/components/ui/dropdown-menu";
import { Button } from "@multica/ui/components/ui/button";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { cn } from "@multica/ui/lib/utils";
import type { DiagramMark, DiagramPoint, DiagramReviewDraft } from "@multica/core/chat/diagram-review";
import type { DiagramReviewActions } from "./diagram-review-context";
import type { ZoomCanvasApi } from "./hooks/use-zoom-canvas";
import type { Size } from "./utils/zoom-transform";
import { ZoomCanvas } from "./zoom-canvas";
import { useT } from "../i18n";
import { buildAnnotatedDiagramSvg, diagramReviewNotes, diagramStrokePath, DIAGRAM_INK } from "./utils/diagram-review-export";
import { renderSvgToPngBlob } from "./utils/mermaid-export";
import "./styles/diagram-review.css";

type Tool = "pan" | "draw" | "comment" | "erase";
export function DiagramReview({ chart, svg, layout, canvas, canvasRef, actions, children, onAdded, toolbarEnd }: {
  chart: string; svg: string; layout: Size; canvas: ZoomCanvasApi;
  canvasRef: React.RefObject<HTMLDivElement | null>;
  actions: DiagramReviewActions; children: ReactNode; onAdded: () => void;
  toolbarEnd?: ReactNode;
}) {
  const { t } = useT("editor");
  const [draft, setDraft] = useState<DiagramReviewDraft>(() => actions.readDraft(chart) ?? { marks: [], note: "" });
  const [tool, setTool] = useState<Tool>("pan");
  const [stroke, setStroke] = useState<DiagramPoint[]>([]);
  const drawing = useRef<{ pointer: number; points: DiagramPoint[] } | null>(null);
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);
  const [focusPin, setFocusPin] = useState<string | null>(null);
  const [editingNote, setEditingNote] = useState(false);
  const [history, setHistory] = useState<DiagramMark[][]>([]);
  const { fit } = canvas;
  // Annotation controls reduce the available canvas. Fit after its first
  // measurement; later zoom/pan and edits preserve the user's chosen position.
  useEffect(() => {
    const frame = requestAnimationFrame(() => fit());
    return () => cancelAnimationFrame(frame);
  }, [fit]);
  const pins = draft.marks.filter((m): m is Extract<DiagramMark, { kind: "pin" }> => m.kind === "pin");
  const selectedPin = pins.find(pin => pin.id === focusPin);
  const selectPin = (id: string) => { setFocusPin(id); setEditingNote(false); };
  const toolItems = [
    ["pan", Hand, t(($) => $.mermaid.pan)],
    ["draw", Pencil, t(($) => $.mermaid.draw)],
    ["comment", MessageSquarePlus, t(($) => $.mermaid.comment)],
    ["erase", Eraser, t(($) => $.mermaid.erase)],
  ] as const;
  const selectTool = (value: Tool) => { setTool(value); setFocusPin(null); setEditingNote(false); };
  const update = (next: DiagramReviewDraft) => { setDraft(next); actions.saveDraft(chart, next); setFailed(false); };
  const changeMarks = (marks: DiagramMark[]) => {
    setHistory(previous => [...previous, draft.marks]);
    update({ ...draft, marks });
  };
  const erase = (id: string) => changeMarks(draft.marks.filter(mark => mark.id !== id));
  const pointAt = (event: PointerEvent<SVGSVGElement>) => {
    const rect = event.currentTarget.getBoundingClientRect();
    return {
      x: Math.max(0, Math.min(layout.width, (event.clientX - rect.left) * layout.width / rect.width)),
      y: Math.max(0, Math.min(layout.height, (event.clientY - rect.top) * layout.height / rect.height)),
    };
  };
  const finishStroke = (event: PointerEvent<SVGSVGElement>) => {
    event.stopPropagation();
    if (drawing.current?.pointer !== event.pointerId) return;
    const points = drawing.current.points;
    drawing.current = null;
    setStroke([]);
    if (points.length > 1) changeMarks([...draft.marks, { id: crypto.randomUUID(), kind: "stroke", points }]);
  };
  const addToChat = async () => {
    const unfinished = pins.find(pin => !pin.text.trim());
    if (unfinished) { selectPin(unfinished.id); return; }
    setBusy(true); setFailed(false);
    try {
      const annotated = buildAnnotatedDiagramSvg(svg, draft.marks, layout);
      const blob = await renderSvgToPngBlob(annotated, layout);
      const accepted = actions.addToDraft({
        file: new File([blob], "diagram-review.png", { type: "image/png" }),
        notes: diagramReviewNotes(draft, t(($) => $.mermaid.review_heading)),
      });
      if (!accepted) throw new Error("Composer unavailable");
      onAdded();
    } catch { setFailed(true); }
    finally { setBusy(false); }
  };

  return <>
    <div className="diagram-review-toolbar shrink-0 border-b border-border">
    <div className="flex items-center gap-1 px-2 py-2">
      <div className="diagram-review-tools flex items-center gap-1">
      {toolItems.map(([value, Icon, label]) => <Button key={value} size="icon" variant={tool === value ? "secondary" : "ghost"}
        aria-label={label} title={label} aria-pressed={tool === value} disabled={busy} onClick={() => selectTool(value)} className="size-9 shrink-0">
        <Icon className="size-4" />
      </Button>)}
      </div>
      <div className="diagram-review-tool-picker">
        <DropdownMenu>
          <DropdownMenuTrigger render={<Button size="sm" variant="secondary" className="max-w-24 truncate" aria-label={t(($) => $.mermaid.choose_tool)} disabled={busy} />}>
            {toolItems.find(([value]) => value === tool)?.[2]}
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start">
            {toolItems.map(([value, Icon, label]) => <DropdownMenuItem key={value} onClick={() => selectTool(value)}><Icon className="size-4" />{label}{tool === value && <Check className="ml-auto size-4" />}</DropdownMenuItem>)}
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      <Button size="icon" className="size-9 shrink-0" variant="ghost" aria-label={t(($) => $.mermaid.undo)} title={t(($) => $.mermaid.undo)}
        disabled={busy || (!draft.marks.length && !history.length)} onClick={() => {
          update({ ...draft, marks: history.at(-1) ?? draft.marks.slice(0, -1) });
          setHistory(previous => previous.slice(0, -1));
        }}>
        <Undo2 className="size-4" />
      </Button>
      <div className="ml-auto flex items-center gap-1">{toolbarEnd}</div>
    </div>
    </div>
    <ZoomCanvas canvas={canvas} content={layout} label={t(($) => $.mermaid.review_canvas_label)} className="bg-background" canvasRef={canvasRef}>
      {children}
      <svg className={cn("absolute inset-0", tool === "pan" || busy ? "pointer-events-none" : "cursor-crosshair")}
        style={{ width: layout.width, height: layout.height, touchAction: "none" }}
        viewBox={`0 0 ${layout.width} ${layout.height}`} role="img" aria-label={t(($) => $.mermaid.annotation_canvas)}
        onDoubleClick={e => e.stopPropagation()}
        onPointerDown={event => {
          if (event.button !== 0 || !event.isPrimary || busy) return;
          event.preventDefault(); event.stopPropagation();
          const point = pointAt(event);
          if (tool === "comment") {
            const id = crypto.randomUUID();
            changeMarks([...draft.marks, { id, kind: "pin", point, text: "" }]);
            selectPin(id);
          } else if (tool === "draw") {
            drawing.current = { pointer: event.pointerId, points: [point] };
            setStroke([point]);
            event.currentTarget.setPointerCapture(event.pointerId);
          }
        }}
        onPointerMove={event => {
          event.stopPropagation();
          if (drawing.current?.pointer !== event.pointerId) return;
          drawing.current.points.push(pointAt(event));
          setStroke([...drawing.current.points]);
        }}
        onPointerUp={finishStroke}
        onPointerCancel={event => { event.stopPropagation(); drawing.current = null; setStroke([]); }}>
        {draft.marks.filter((m): m is Extract<DiagramMark, { kind: "stroke" }> => m.kind === "stroke").map(mark =>
          <g key={mark.id} onPointerDown={event => {
            if (tool !== "erase" || busy || event.button !== 0) return;
            event.preventDefault(); event.stopPropagation(); erase(mark.id);
          }}>
            <path d={diagramStrokePath(mark.points)} fill="none" stroke={DIAGRAM_INK} strokeWidth={4} strokeLinecap="round" strokeLinejoin="round" />
            {tool === "erase" && <path d={diagramStrokePath(mark.points)} fill="none" stroke="transparent" strokeWidth={18} vectorEffect="non-scaling-stroke" pointerEvents="stroke" />}
          </g>)}
        <path d={diagramStrokePath(stroke)} fill="none" stroke={DIAGRAM_INK} strokeWidth={4} strokeLinecap="round" strokeLinejoin="round" />
        {pins.map((pin, i) => <g key={pin.id} pointerEvents="auto" role="button" tabIndex={0}
          aria-label={t(($) => $.mermaid.pin_comment, { number: i + 1 })}
          onKeyDown={event => { if (!busy && (event.key === "Enter" || event.key === " ")) { event.preventDefault(); event.stopPropagation(); selectPin(pin.id); } }}
          onPointerDown={event => {
            if (busy || event.button !== 0) return;
            event.preventDefault(); event.stopPropagation();
            if (tool === "erase") erase(pin.id); else selectPin(pin.id);
          }}>
          <circle cx={pin.point.x} cy={pin.point.y} r={12} fill={DIAGRAM_INK} stroke={focusPin === pin.id ? "currentColor" : "none"} strokeWidth={2} />
          <text x={pin.point.x} y={pin.point.y + 4} textAnchor="middle" style={{ fill: "#111", font: "bold 13px sans-serif" }}>{i + 1}</text>
        </g>)}
      </svg>
    </ZoomCanvas>
    <div className="shrink-0 space-y-2 border-t border-border bg-background p-2">
      {(selectedPin || editingNote) ? <div className="min-w-0 space-y-2 p-1">
        <div className="flex items-center justify-between gap-2">
          <span className="text-caption text-muted-foreground">{selectedPin ? t(($) => $.mermaid.pin_comment, { number: pins.indexOf(selectedPin) + 1 }) : t(($) => $.mermaid.add_question)}</span>
          <Button size="sm" variant="ghost" aria-label={t(($) => $.mermaid.done_comment)} onClick={() => { setFocusPin(null); setEditingNote(false); }}>{t(($) => $.mermaid.done)}</Button>
        </div>
        <Textarea key={selectedPin?.id ?? "note"} autoFocus rows={2} className="min-h-16 max-h-24 resize-none text-body"
          aria-label={selectedPin ? t(($) => $.mermaid.pin_comment, { number: pins.indexOf(selectedPin) + 1 }) : t(($) => $.mermaid.annotation_note)}
          placeholder={t(($) => $.mermaid.annotation_note)} value={selectedPin?.text ?? draft.note} disabled={busy}
          onChange={e => update(selectedPin ? { ...draft, marks: draft.marks.map(mark => mark.id === selectedPin.id ? { ...selectedPin, text: e.target.value } : mark) } : { ...draft, note: e.target.value })} />
      </div> : <div className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-2">
        <DropdownMenu>
          <DropdownMenuTrigger render={<Button size="sm" variant="ghost" className="min-w-0 justify-start overflow-hidden" aria-label={t(($) => $.mermaid.comments_menu)} />}>
            <MessageSquare className="size-4 shrink-0" /><span className="truncate">{pins.length ? t(($) => $.mermaid.comments_count, { count: pins.length }) : t(($) => $.mermaid.add_question)}</span>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" className="max-w-[min(20rem,calc(100vw-2rem))]">
            <DropdownMenuItem onClick={() => { setFocusPin(null); setEditingNote(true); }}>{t(($) => $.mermaid.add_question)}</DropdownMenuItem>
            {pins.map((pin, i) => <DropdownMenuItem key={pin.id} onClick={() => selectPin(pin.id)}><span className="shrink-0 text-muted-foreground">{i + 1}</span><span className="truncate">{pin.text || t(($) => $.mermaid.add_question)}</span></DropdownMenuItem>)}
          </DropdownMenuContent>
        </DropdownMenu>
        <Button size="sm" disabled={busy || (!draft.marks.length && !draft.note.trim())} onClick={() => void addToChat()}>
          {busy ? t(($) => $.mermaid.adding_to_chat) : t(($) => $.mermaid.add_to_chat)}
        </Button>
      </div>}
      {failed && <p role="alert" className="text-caption text-destructive">{t(($) => $.mermaid.annotation_error)}</p>}
    </div>
  </>;
}
