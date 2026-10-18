/** @vitest-environment jsdom */
import { afterEach, describe, expect, it, vi } from "vitest";
import { marksToPixels, renderMarkedScreenshot } from "./marked-screenshot";

describe("marksToPixels", () => {
  it("keeps feedback aligned with its source at natural resolution", () => {
    expect(marksToPixels([{
      id: 3, shot: "source", x: .125, y: .25, w: .5, h: .125, note: "Too dense",
    }], 1600, 800)).toEqual([{
      id: 3, x: 200, y: 200, w: 800, h: 100, note: "Too dense",
    }]);
  });

  it("falls back to the image element when createImageBitmap cannot decode an SVG", async () => {
    const decode = vi.fn(async () => undefined);
    const drawImage = vi.fn();
    const context = {
      drawImage,
      strokeRect: vi.fn(),
      beginPath: vi.fn(),
      arc: vi.fn(),
      fill: vi.fn(),
      fillText: vi.fn(),
      lineWidth: 0,
      strokeStyle: "",
      fillStyle: "",
      font: "",
      textAlign: "start",
      textBaseline: "alphabetic",
    };
    class TestImage {
      src = "";
      naturalWidth = 1440;
      naturalHeight = 900;
      decode = decode;
    }
    vi.stubGlobal("fetch", vi.fn(async () => new Response("<svg/>", { status: 200, headers: { "content-type": "image/svg+xml" } })));
    vi.stubGlobal("createImageBitmap", vi.fn(async () => { throw new Error("The source image could not be decoded."); }));
    vi.stubGlobal("Image", TestImage);
    vi.stubGlobal("URL", { createObjectURL: vi.fn(() => "blob:test-svg"), revokeObjectURL: vi.fn() });
    vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue(context as unknown as CanvasRenderingContext2D);
    vi.spyOn(HTMLCanvasElement.prototype, "toBlob").mockImplementation((callback) => callback(new Blob(["png"], { type: "image/png" })));

    const file = await renderMarkedScreenshot("/source.svg", "source.svg", [{
      id: 1, shot: "source", x: .1, y: .1, w: .2, h: .2, note: "Change this",
    }]);

    expect(decode).toHaveBeenCalledTimes(1);
    expect(drawImage).toHaveBeenCalledTimes(1);
    expect(file.name).toBe("source-marked.png");
  });
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});
