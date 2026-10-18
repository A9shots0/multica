import { useState } from "react";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
import enInbox from "../../locales/en/inbox.json";
import { PreviewViewer, attachmentSources } from "./preview-viewer";
import type { Mark } from "./screenshot-annotator";
import type { Attachment } from "@multica/core/types";

vi.mock("../../editor/html-preview-body", () => ({
  HtmlPreviewBody: ({ source }: { source: { attachmentId: string } }) => (
    <div data-testid="secure-attached-page">{source.attachmentId}</div>
  ),
}));

const sources = [{ id: "shot", kind: "image" as const, label: "Desktop", url: "/preview.png" }];
const marks: Mark[] = Array.from({ length: 12 }, (_, index) => ({
  id: index + 1, shot: "shot", x: .1, y: .1, w: .2, h: .2, note: `Feedback ${index + 1}`,
}));

function Review({
  initial = marks,
  primaryAction,
}: {
  initial?: Mark[];
  primaryAction?: { label: string; onClick: () => void };
}) {
  const [draft, setDraft] = useState(initial);
  return <I18nProvider locale="en" resources={{ en: { inbox: enInbox } }}><PreviewViewer title="Review" sources={sources} marks={draft} onChangeMarks={setDraft} onClose={vi.fn()} primaryAction={primaryAction} /></I18nProvider>;
}

describe("media feedback viewer", () => {
  it("renders an attached HTML page from its content instead of framing the app shell", () => {
    const id = "117a9167-c091-49f8-b03f-1637756141bb";
    render(
      <I18nProvider locale="en" resources={{ en: { inbox: enInbox } }}>
        <PreviewViewer
          title="Review"
          sources={[{
            id: "page",
            kind: "page",
            label: "Checkout — full page",
            url: `/local-candidate-samples/attachments/${id}/preview`,
          }]}
          marks={[]}
          onChangeMarks={vi.fn()}
          onClose={vi.fn()}
        />
      </I18nProvider>,
    );

    expect(screen.getByTestId("secure-attached-page").textContent).toBe(id);
    expect(screen.queryByTitle("Checkout — full page")).toBeNull();
  });

  it("uses a compact position control instead of exposing attachment filenames", () => {
    const namedSources = [
      { id: "original", kind: "image" as const, label: "Owner review acceptance sample.svg", url: "/original.svg" },
      { id: "marked-1", kind: "image" as const, label: "Owner review acceptance sample marked 1.png", url: "/marked-1.png" },
      { id: "marked-2", kind: "image" as const, label: "Owner review acceptance sample marked 2.png", url: "/marked-2.png" },
    ];
    render(<I18nProvider locale="en" resources={{ en: { inbox: enInbox } }}><PreviewViewer title="Review" sources={namedSources} marks={[]} onChangeMarks={vi.fn()} onClose={vi.fn()} /></I18nProvider>);
    const viewer = within(screen.getByRole("dialog"));
    expect(viewer.getByText("Result 1 of 3")).toBeTruthy();
    expect(viewer.queryByText("Owner review acceptance sample.svg")).toBeNull();
    expect(viewer.queryByText("Owner review acceptance sample marked 1")).toBeNull();
  });

  it("keeps generated marked evidence out of the reviewable result carousel", () => {
    const attachment = (id: string, filename: string): Attachment => ({
      id, filename, content_type: "image/png", workspace_id: "workspace",
      issue_id: "issue", comment_id: null, chat_session_id: null, chat_message_id: null,
      uploader_type: "agent", uploader_id: "agent", url: "", download_url: "",
      markdown_url: "", size_bytes: 1, created_at: "2026-10-03T00:00:00Z",
    });
    const results = attachmentSources(
      [attachment("source", "source.png"), attachment("marked", "source-marked.png")],
      new Set(["marked"]),
    );
    expect(results.map((result) => result.id)).toEqual(["source"]);
  });

  it("shows saved annotations as pins and expands only the pin being edited", () => {
    render(<Review />);
    const viewer = within(screen.getByRole("dialog"));
    expect(viewer.getAllByRole("button", { name: /^Note \d+:/ })).toHaveLength(12);
    expect(viewer.queryByTestId(/^annotation-region-/)).toBeNull();
    expect(viewer.queryByRole("textbox")).toBeNull();
    expect(viewer.queryByRole("button", { name: /approve|ask for a change/i })).toBeNull();
    fireEvent.click(viewer.getByRole("button", { name: "Note 4: Feedback 4" }));
    expect(viewer.getByTestId("annotation-region-4")).toBeTruthy();
    expect(viewer.queryByTestId("annotation-region-5")).toBeNull();
    expect(viewer.getAllByRole("textbox")).toHaveLength(1);
    fireEvent.change(viewer.getByRole("textbox"), { target: { value: "Make this compact" } });
    expect(document.activeElement).toBe(viewer.getByRole("textbox"));
    fireEvent.keyDown(viewer.getByRole("textbox"), { key: "Enter", shiftKey: true });
    expect(viewer.getByRole("textbox")).toBeTruthy();
    fireEvent.keyDown(viewer.getByRole("textbox"), { key: "Enter" });
    expect(viewer.queryByRole("textbox")).toBeNull();
    expect(viewer.queryByTestId(/^annotation-region-/)).toBeNull();
    expect(viewer.getByRole("button", { name: "Note 4: Make this compact" })).toBeTruthy();
    fireEvent.click(viewer.getByRole("button", { name: "Note 5: Feedback 5" }));
    expect(viewer.getByTestId("annotation-region-5")).toBeTruthy();
    expect(viewer.queryByTestId("annotation-region-4")).toBeNull();
    fireEvent.pointerDown(document.body);
    expect(viewer.queryByTestId(/^annotation-region-/)).toBeNull();
    expect(viewer.getAllByRole("button", { name: /^Note \d+:/ })).toHaveLength(12);
  });

  it("opens an anchored comment after drawing, collapses to a pin on click-out, and reopens from the pin", () => {
    render(<Review initial={[]} />);
    fireEvent.click(screen.getByRole("button", { name: "Annotate" }));
    const surface = screen.getByRole("img").parentElement!;
    vi.spyOn(surface, "getBoundingClientRect").mockReturnValue({ x: 0, y: 0, left: 0, top: 0, right: 500, bottom: 400, width: 500, height: 400, toJSON: () => ({}) });
    const overlay = screen.getByTestId("mark-surface-overlay");
    fireEvent.pointerDown(overlay, { pointerId: 1, pointerType: "mouse", button: 0, clientX: 50, clientY: 50 });
    fireEvent.pointerMove(overlay, { pointerId: 1, pointerType: "mouse", clientX: 180, clientY: 140 });
    fireEvent.pointerUp(overlay, { pointerId: 1, pointerType: "mouse", clientX: 180, clientY: 140 });
    expect(screen.getByRole("textbox", { name: "Note 1" })).toBe(document.activeElement);
    expect(surface.contains(screen.getByRole("textbox", { name: "Note 1" }))).toBe(true);
    expect(screen.getByTestId("annotation-region-1")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Finish" })).toBeTruthy();
    fireEvent.change(screen.getByRole("textbox", { name: "Note 1" }), { target: { value: "Tighten this section" } });
    fireEvent.pointerDown(document.body);
    expect(screen.getByRole("button", { name: "Finish" })).toBeTruthy();
    expect(screen.queryByTestId("annotation-region-1")).toBeNull();
    expect(screen.getByRole("button", { name: "Note 1: Tighten this section" })).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Note 1: Tighten this section" }));
    expect(screen.getByTestId("annotation-region-1")).toBeTruthy();
    expect(screen.getByRole("textbox", { name: "Note 1" })).toBe(document.activeElement);
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    fireEvent.click(screen.getByRole("button", { name: "Undo" }));
    expect(screen.getByRole("button", { name: "Annotations 0" })).toBeTruthy();
  });

  it("discards a blank annotation when the owner clicks away", () => {
    render(<Review initial={[]} />);
    fireEvent.click(screen.getByRole("button", { name: "Annotate" }));
    const surface = screen.getByRole("img").parentElement!;
    vi.spyOn(surface, "getBoundingClientRect").mockReturnValue({ x: 0, y: 0, left: 0, top: 0, right: 500, bottom: 400, width: 500, height: 400, toJSON: () => ({}) });
    const overlay = screen.getByTestId("mark-surface-overlay");
    fireEvent.pointerDown(overlay, { pointerId: 1, pointerType: "mouse", button: 0, clientX: 50, clientY: 50 });
    fireEvent.pointerMove(overlay, { pointerId: 1, pointerType: "mouse", clientX: 180, clientY: 140 });
    fireEvent.pointerUp(overlay, { pointerId: 1, pointerType: "mouse", clientX: 180, clientY: 140 });
    fireEvent.pointerDown(document.body);
    expect(screen.getByRole("button", { name: "Annotations 0" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: /^Note \d+:/ })).toBeNull();
  });

  it("lets an owning surface act on completed annotations", () => {
    const onClick = vi.fn();
    render(<Review initial={[marks[0]!]} primaryAction={{ label: "Add 1 note to chat", onClick }} />);
    fireEvent.click(screen.getByRole("button", { name: "Add 1 note to chat" }));
    expect(onClick).toHaveBeenCalledOnce();
  });
});
