import { expect, test } from "vite-plus/test";
import { Rive } from "@rive-app/canvas";
import { PROCESSING_ARTBOARD, PROCESSING_STATE_MACHINE, processingAssetUrl } from "./runtime";

test("the bundled asset exposes the authored state machine and driven progress property", async () => {
  const url = processingAssetUrl();
  expect(url).toBeTruthy();
  const response = await fetch(url!);
  expect(response.ok).toBe(true);
  const buffer = await response.arrayBuffer();
  const canvas = document.createElement("canvas");
  canvas.width = canvas.height = 512;
  document.body.append(canvas);
  let instance: Rive | undefined;
  try {
    await new Promise<void>((resolve, reject) => {
      instance = new Rive({
        canvas,
        buffer,
        artboard: PROCESSING_ARTBOARD,
        stateMachine: PROCESSING_STATE_MACHINE,
        autoBind: true,
        autoplay: false,
        onLoad: () => resolve(),
        onLoadError: () => reject(new Error("Authored asset failed to load")),
      });
    });
    expect(instance!.stateMachineNames).toContain(PROCESSING_STATE_MACHINE);
    const artboard = instance!.contents.artboards?.find(
      (artboard) => artboard.name === PROCESSING_ARTBOARD,
    );
    expect(artboard?.animations).toEqual(
      expect.arrayContaining(["Full", "Two Thirds", "One Third", "Empty"]),
    );
    const progress = instance!.viewModelInstance?.number("progress");
    expect(progress).toBeTruthy();
    progress!.value = 100;
    expect(progress!.value).toBe(100);
  } finally {
    instance?.cleanup();
    canvas.remove();
  }
});
