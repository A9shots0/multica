import type { DiagramMark, DiagramReviewDraft } from "@multica/core/chat/diagram-review";
import type { Size } from "./zoom-transform";

export const DIAGRAM_INK = "#f97316";
export const diagramStrokePath = (points: { x: number; y: number }[]) =>
  points.map((p, i) => `${i ? "L" : "M"}${p.x.toFixed(2)},${p.y.toFixed(2)}`).join(" ");

/** Marks live in natural iframe pixels, independent of the viewer's pan/zoom.
 * Map them to the SVG viewBox on export (which may have a non-zero origin). */
export function buildAnnotatedDiagramSvg(svg: string, marks: DiagramMark[], layout: Size): string {
  const doc = new DOMParser().parseFromString(svg, "image/svg+xml");
  if (doc.querySelector("parsererror")) throw new Error("Invalid diagram SVG");
  const root = doc.documentElement;
  const box = (root.getAttribute("viewBox") ?? `0 0 ${layout.width} ${layout.height}`).split(/[\s,]+/).map(Number);
  const [x = 0, y = 0, w = layout.width, h = layout.height] = box;
  const ns = "http://www.w3.org/2000/svg";
  const layer = doc.createElementNS(ns, "g");
  layer.setAttribute("transform", `translate(${x} ${y}) scale(${w / layout.width} ${h / layout.height})`);
  layer.setAttribute("data-diagram-annotations", "");
  let pin = 0;
  for (const mark of marks) {
    if (mark.kind === "stroke") {
      const path = doc.createElementNS(ns, "path");
      path.setAttribute("d", diagramStrokePath(mark.points));
      path.setAttribute("fill", "none");
      path.setAttribute("stroke", DIAGRAM_INK);
      path.setAttribute("stroke-width", "4");
      path.setAttribute("stroke-linecap", "round");
      path.setAttribute("stroke-linejoin", "round");
      layer.appendChild(path);
    } else {
      pin++;
      const circle = doc.createElementNS(ns, "circle");
      circle.setAttribute("cx", String(mark.point.x));
      circle.setAttribute("cy", String(mark.point.y));
      circle.setAttribute("r", "12");
      circle.setAttribute("fill", DIAGRAM_INK);
      const text = doc.createElementNS(ns, "text");
      text.setAttribute("x", String(mark.point.x));
      text.setAttribute("y", String(mark.point.y + 4));
      // Inline style outranks Mermaid's generated text selectors.
      text.setAttribute("style", "fill: #111; font: bold 13px sans-serif; text-anchor: middle;");
      text.textContent = String(pin);
      layer.append(circle, text);
    }
  }
  root.appendChild(layer);
  return new XMLSerializer().serializeToString(root);
}

export function diagramReviewNotes(draft: DiagramReviewDraft, heading: string): string {
  const pins = draft.marks.filter((m): m is Extract<DiagramMark, { kind: "pin" }> => m.kind === "pin");
  return [heading, draft.note.trim(), ...pins.map((pin, i) => `${i + 1}. ${pin.text.trim()}`)]
    .filter(Boolean).join("\n\n");
}
