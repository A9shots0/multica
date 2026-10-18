export interface DiagramPoint { x: number; y: number }
export type DiagramMark =
  | { id: string; kind: "stroke"; points: DiagramPoint[] }
  | { id: string; kind: "pin"; point: DiagramPoint; text: string };
export interface DiagramReviewDraft { marks: DiagramMark[]; note: string }

export function readDiagramReviewDrafts(raw: string | null): Record<string, DiagramReviewDraft> {
  if (!raw) return {};
  try {
    const data: unknown = JSON.parse(raw);
    if (!data || typeof data !== "object" || Array.isArray(data)) return {};
    const point = (p: unknown): p is DiagramPoint => !!p && typeof p === "object"
      && Number.isFinite((p as DiagramPoint).x) && Number.isFinite((p as DiagramPoint).y);
    return Object.fromEntries(Object.entries(data).filter(([, draft]) =>
      draft && typeof draft.note === "string" && Array.isArray(draft.marks)
      && draft.marks.every((mark: DiagramMark) => typeof mark?.id === "string"
        && (mark.kind === "stroke" ? Array.isArray(mark.points) && mark.points.every(point)
          : mark.kind === "pin" && point(mark.point) && typeof mark.text === "string")),
    ));
  } catch { return {}; }
}
