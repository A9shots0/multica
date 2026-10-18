import type { ReviewMark } from "@multica/core/types";

export interface PixelMark {
  id: number;
  x: number;
  y: number;
  w: number;
  h: number;
  note: string;
}

export function marksToPixels(marks: ReviewMark[], width: number, height: number): PixelMark[] {
  return marks.map((mark) => ({
    id: mark.id,
    x: Math.round(mark.x * width),
    y: Math.round(mark.y * height),
    w: Math.round(mark.w * width),
    h: Math.round(mark.h * height),
    note: mark.note,
  }));
}

function safeFilename(name: string): string {
  const stem = name.replace(/\.[^.]+$/, "").replace(/[^a-zA-Z0-9_-]+/g, "-").replace(/^-+|-+$/g, "") || "review";
  return `${stem}-marked.png`;
}

/**
 * Produces the evidence image that travels with an owner's structured marks.
 * The original attachment remains untouched; this is a new PNG rendition.
 */
export async function renderMarkedScreenshot(
  url: string,
  filename: string,
  marks: ReviewMark[],
): Promise<File> {
  const response = await fetch(url, { credentials: "same-origin" });
  if (!response.ok) throw new Error("The screenshot could not be loaded for marking.");
  const blob = await response.blob();
  let source: CanvasImageSource;
  let width: number;
  let height: number;
  let close = () => {};
  if (typeof createImageBitmap === "function") {
    try {
      const bitmap = await createImageBitmap(blob);
      source = bitmap;
      width = bitmap.width;
      height = bitmap.height;
      close = () => bitmap.close();
    } catch {
      const image = await decodeWithImageElement(blob);
      source = image;
      width = image.naturalWidth;
      height = image.naturalHeight;
    }
  } else {
    const image = await decodeWithImageElement(blob);
    source = image;
    width = image.naturalWidth;
    height = image.naturalHeight;
  }
  const canvas = document.createElement("canvas");
  canvas.width = width;
  canvas.height = height;
  const context = canvas.getContext("2d");
  if (!context) {
    close();
    throw new Error("This browser cannot create a marked screenshot.");
  }
  context.drawImage(source, 0, 0);
  close();

  const scale = Math.max(1, Math.min(canvas.width, canvas.height) / 900);
  context.lineWidth = Math.max(3, Math.round(4 * scale));
  context.strokeStyle = "#2563eb";
  context.fillStyle = "#2563eb";
  context.font = `700 ${Math.max(15, Math.round(18 * scale))}px ui-sans-serif, system-ui, sans-serif`;
  context.textAlign = "center";
  context.textBaseline = "middle";
  for (const mark of marksToPixels(marks, canvas.width, canvas.height)) {
    context.strokeRect(mark.x, mark.y, mark.w, mark.h);
    const radius = Math.max(13, Math.round(15 * scale));
    const cx = Math.min(canvas.width - radius, Math.max(radius, mark.x + mark.w));
    const cy = Math.min(canvas.height - radius, Math.max(radius, mark.y + mark.h));
    context.beginPath();
    context.arc(cx, cy, radius, 0, Math.PI * 2);
    context.fill();
    context.fillStyle = "#ffffff";
    context.fillText(String(mark.id), cx, cy + scale);
    context.fillStyle = "#2563eb";
  }

  const markedBlob = await new Promise<Blob>((resolve, reject) => {
    canvas.toBlob((value) => value ? resolve(value) : reject(new Error("The marked screenshot could not be encoded.")), "image/png");
  });
  return new File([markedBlob], safeFilename(filename), { type: "image/png" });
}

async function decodeWithImageElement(blob: Blob): Promise<HTMLImageElement> {
  const objectURL = URL.createObjectURL(blob);
  const image = new Image();
  image.src = objectURL;
  try {
    await image.decode();
    return image;
  } catch (error) {
    throw new Error("The source image could not be prepared for annotation.", { cause: error });
  } finally {
    URL.revokeObjectURL(objectURL);
  }
}
