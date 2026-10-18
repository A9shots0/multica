import { describe, expect, it } from "vitest";
import { buildAnnotatedDiagramSvg, diagramReviewNotes } from "./diagram-review-export";

describe("diagram review export", () => {
  it("keeps marks aligned with a nonzero SVG origin and preserves diagram content", () => {
    const result = buildAnnotatedDiagramSvg('<svg xmlns="http://www.w3.org/2000/svg" viewBox="-20 10 400 800"><text>Review goal</text></svg>', [
      { id: "s", kind: "stroke", points: [{ x: 5, y: 10 }, { x: 40, y: 60 }] },
      { id: "p", kind: "pin", point: { x: 20, y: 30 }, text: "Why this step?" },
    ], { width: 400, height: 800 });
    const doc = new DOMParser().parseFromString(result, "image/svg+xml");
    expect(doc.querySelector("parsererror")).toBeNull();
    expect(doc.querySelector("[data-diagram-annotations]")?.getAttribute("transform")).toBe("translate(-20 10) scale(1 1)");
    expect(doc.querySelector("path")?.getAttribute("d")).toBe("M5.00,10.00 L40.00,60.00");
    expect(doc.querySelector("circle")?.getAttribute("cx")).toBe("20");
    expect(doc.documentElement.textContent).toBe("Review goal1");
  });

  it("numbers comments in the same order as pins and carries the owner's question", () => {
    expect(diagramReviewNotes({ note: " Could we simplify? ", marks: [
      { id: "s", kind: "stroke", points: [{ x: 0, y: 0 }] },
      { id: "a", kind: "pin", point: { x: 1, y: 2 }, text: "Why return here?" },
      { id: "b", kind: "pin", point: { x: 2, y: 3 }, text: "Who accepts?" },
    ] }, "Diagram feedback")).toBe("Diagram feedback\n\nCould we simplify?\n\n1. Why return here?\n\n2. Who accepts?");
  });
});
