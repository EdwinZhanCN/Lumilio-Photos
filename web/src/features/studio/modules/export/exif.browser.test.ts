import { describe, it, expect } from "vite-plus/test";
import zeroperlWasmUrl from "@colorhythm/exiftool-wasm/dist/esm/zeroperl-mqcadjqm.wasm?url";
import { preserveExif } from "./exif";

// Real Chromium + the ExifTool wasm (no GPU needed, so this runs headless in
// CI). A full round-trip: stamp EXIF onto a fixture with the wasm, run it
// through preserveExif, then read the result back and assert the descriptive
// tags carried over while Orientation was reset to upright.

function wasmFetch(...args: unknown[]): Promise<Response> {
  const first = args[0];
  let url = "";
  if (typeof first === "string") url = first;
  else if (first instanceof URL) url = first.href;
  else if (first instanceof Request) url = first.url;
  if (url.includes("zeroperl") && url.endsWith(".wasm")) return fetch(zeroperlWasmUrl);
  return fetch(first as RequestInfo, args[1] as RequestInit | undefined);
}

async function jpeg(color: string): Promise<Blob> {
  const canvas = new OffscreenCanvas(80, 60);
  const ctx = canvas.getContext("2d")!;
  ctx.fillStyle = color;
  ctx.fillRect(0, 0, 80, 60);
  return canvas.convertToBlob({ type: "image/jpeg", quality: 0.9 });
}

describe("preserveExif", () => {
  it("copies descriptive tags onto the export and resets Orientation", async () => {
    const { writeMetadata, parseMetadata } = await import("@colorhythm/exiftool-wasm");

    // A source that carries EXIF, including a non-upright Orientation.
    const stamped = await writeMetadata(
      { name: "original.jpg", data: await jpeg("rgb(120,120,120)") },
      { "EXIF:Make": "TESTCAM", "EXIF:Model": "M1", "EXIF:Orientation": 6 },
      { args: ["-n", "-m"], fetch: wasmFetch },
    );
    expect(stamped.success).toBe(true);
    if (!stamped.success) return;
    const original = new Blob([stamped.data], { type: "image/jpeg" });

    // A freshly re-encoded export (canvas strips all metadata).
    const exported = await jpeg("rgb(200,50,50)");
    const withExif = await preserveExif(exported, original, {
      format: "image/jpeg",
      width: 80,
      height: 60,
    });

    const read = await parseMetadata(
      { name: "out.jpg", data: withExif },
      { args: ["-json", "-n"], fetch: wasmFetch },
    );
    expect(read.success).toBe(true);
    if (!read.success) return;
    const tags = (JSON.parse(read.data) as Array<Record<string, unknown>>)[0];

    expect(tags.Make).toBe("TESTCAM");
    expect(tags.Model).toBe("M1");
    expect(tags.Orientation).toBe(1); // reset — rotation is baked into pixels
  });

  it("returns PNG exports untouched (PNG carries no EXIF)", async () => {
    const canvas = new OffscreenCanvas(8, 8);
    canvas.getContext("2d")!.fillRect(0, 0, 8, 8);
    const png = await canvas.convertToBlob({ type: "image/png" });
    const out = await preserveExif(png, png, { format: "image/png", width: 8, height: 8 });
    expect(out).toBe(png);
  });
});
