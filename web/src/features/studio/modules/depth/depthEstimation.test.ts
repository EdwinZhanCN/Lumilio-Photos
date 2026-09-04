import { describe, expect, it } from "vite-plus/test";
import { getDepthCapability } from "./depthEstimation";

describe("getDepthCapability", () => {
  it("rejects WebGPU on an insecure LAN HTTP origin", () => {
    expect(getDepthCapability({ secureContext: false, webGPUAvailable: true })).toEqual({
      supported: false,
      reason: "secure-context-required",
    });
  });

  it("reports browsers without WebGPU", () => {
    expect(getDepthCapability({ secureContext: true, webGPUAvailable: false })).toEqual({
      supported: false,
      reason: "webgpu-unavailable",
    });
  });

  it("accepts WebGPU in a secure context", () => {
    expect(getDepthCapability({ secureContext: true, webGPUAvailable: true })).toEqual({
      supported: true,
    });
  });
});
