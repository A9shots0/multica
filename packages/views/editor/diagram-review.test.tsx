import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { DiagramReview } from "./diagram-review";
import type { DiagramReviewDraft } from "@multica/core/chat/diagram-review";
import type { ZoomCanvasApi } from "./hooks/use-zoom-canvas";

vi.mock("../i18n", async () => {
  const en = (await import("../locales/en/editor.json")).default;
  return { useT: () => ({ t: (pick: (e: typeof en) => string, args?: { number: number }) => pick(en).replace("{{number}}", String(args?.number)) }) };
});
vi.mock("./zoom-canvas", () => ({ ZoomCanvas: ({ children }: { children: React.ReactNode }) => <div>{children}</div> }));
const rasterize = vi.hoisted(() => vi.fn(async (svg: string) => new Blob([svg], { type: "image/png" })));
vi.mock("./utils/mermaid-export", () => ({ renderSvgToPngBlob: rasterize }));

function setup(saved?: DiagramReviewDraft, accepts = true) {
  const actions = { readDraft: vi.fn(() => saved), saveDraft: vi.fn(), addToDraft: vi.fn(() => accepts) };
  const onAdded = vi.fn();
  render(<DiagramReview chart="flowchart TD; A-->B" svg='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 400 800" />'
    layout={{ width: 400, height: 800 }} canvas={{ fit: vi.fn() } as unknown as ZoomCanvasApi} canvasRef={{ current: null }} actions={actions} onAdded={onAdded}><span>diagram</span></DiagramReview>);
  return { actions, onAdded };
}
beforeEach(() => rasterize.mockClear());

describe("diagram annotation review", () => {
  it("converts a zoomed screen tap into natural coordinates and carries its comment into the draft", async () => {
    const { actions, onAdded } = setup();
    fireEvent.click(screen.getByRole("button", { name: "Comment" }));
    const overlay = screen.getByRole("img");
    vi.spyOn(overlay, "getBoundingClientRect").mockReturnValue({ left: 100, top: 50, width: 200, height: 400 } as DOMRect);
    const down = new Event("pointerdown", { bubbles: true });
    Object.assign(down, { button: 0, isPrimary: true, clientX: 150, clientY: 150, pointerId: 1 });
    fireEvent(overlay, down);
    expect(actions.saveDraft).toHaveBeenLastCalledWith(expect.any(String), expect.objectContaining({ marks: [expect.objectContaining({ point: { x: 100, y: 200 } })] }));
    expect(screen.queryByRole("button", { name: "Add to chat" })).not.toBeInTheDocument();
    fireEvent.change(screen.getByRole("textbox", { name: "Comment 1" }), { target: { value: "Why this branch?" } });
    fireEvent.click(screen.getByRole("button", { name: "Done editing" }));
    fireEvent.click(screen.getByRole("button", { name: "Add to chat" }));
    await waitFor(() => expect(actions.addToDraft).toHaveBeenCalledWith(expect.objectContaining({ file: expect.any(File), notes: "Diagram feedback\n\n1. Why this branch?" })));
    expect(rasterize.mock.calls[0]?.[0]).toContain('cx="100"');
    expect(onAdded).toHaveBeenCalledTimes(1);
  });
  it("retains saved marks on failure and supports undo", async () => {
    const { actions, onAdded } = setup({ note: "Question", marks: [{ id: "a", kind: "stroke", points: [{ x: 1, y: 1 }, { x: 2, y: 2 }] }] }, false);
    fireEvent.click(screen.getByRole("button", { name: "Add to chat" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Your marks are saved");
    expect(onAdded).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Comments and questions" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Add a question" }));
    expect(screen.getByLabelText("Ask about your markings…")).toHaveValue("Question");
    fireEvent.click(screen.getByRole("button", { name: "Undo last change" }));
    expect(actions.saveDraft).toHaveBeenLastCalledWith(expect.any(String), { note: "Question", marks: [] });
  });
  it("erases one stroke or pin and can undo erasing the last mark", () => {
    const stroke = { id: "stroke", kind: "stroke" as const, points: [{ x: 1, y: 1 }, { x: 2, y: 2 }] };
    const pin = { id: "pin", kind: "pin" as const, point: { x: 30, y: 40 }, text: "Question" };
    const { actions } = setup({ note: "Keep this", marks: [stroke, pin] });
    fireEvent.click(screen.getByRole("button", { name: "Erase" }));
    const eraseTarget = (target: Element) => {
      const down = new Event("pointerdown", { bubbles: true });
      Object.assign(down, { button: 0, isPrimary: true });
      fireEvent(target, down);
    };
    eraseTarget(screen.getByRole("img").querySelector('path[stroke="transparent"]')!);
    expect(actions.saveDraft).toHaveBeenLastCalledWith(expect.any(String), { note: "Keep this", marks: [pin] });
    eraseTarget(screen.getByRole("img").querySelector("circle")!);
    expect(screen.queryByRole("button", { name: "Comment 1" })).not.toBeInTheDocument();
    expect(actions.saveDraft).toHaveBeenLastCalledWith(expect.any(String), { note: "Keep this", marks: [] });
    fireEvent.click(screen.getByRole("button", { name: "Undo last change" }));
    fireEvent.keyDown(screen.getByRole("button", { name: "Comment 1" }), { key: "Enter" });
    expect(screen.getByRole("textbox", { name: "Comment 1" })).toHaveValue("Question");
    fireEvent.click(screen.getByRole("button", { name: "Undo last change" }));
    expect(actions.saveDraft).toHaveBeenLastCalledWith(expect.any(String), { note: "Keep this", marks: [stroke, pin] });
  });
});
