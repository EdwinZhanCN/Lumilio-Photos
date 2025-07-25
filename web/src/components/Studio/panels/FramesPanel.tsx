import { useState } from "react";
import { RectangleGroupIcon } from "@heroicons/react/24/outline";
import {
  BorderOptions,
  BorderParams,
} from "@/hooks/util-hooks/useGenerateBorder";

type FramesPanelProps = {
  isGenerating: boolean;
  onGenerate: (
    option: BorderOptions,
    param: BorderParams[BorderOptions],
  ) => void;
};

export function FramesPanel({ isGenerating, onGenerate }: FramesPanelProps) {
  const [activeFrame, setActiveFrame] = useState<BorderOptions>("COLORED");

  const [coloredParams, setColoredParams] = useState<BorderParams["COLORED"]>({
    border_width: 20,
    r: 255,
    g: 255,
    b: 255,
    jpeg_quality: 90,
  });

  const [frostedParams, setFrostedParams] = useState<BorderParams["FROSTED"]>({
    blur_sigma: 15.0,
    brightness_adjustment: -40,
    corner_radius: 30,
    jpeg_quality: 90,
  });

  const [vignetteParams, setVignetteParams] = useState<
    BorderParams["VIGNETTE"]
  >({
    strength: 0.7,
    jpeg_quality: 90,
  });

  const handleGenerateClick = () => {
    switch (activeFrame) {
      case "COLORED":
        onGenerate("COLORED", coloredParams);
        break;
      case "FROSTED":
        onGenerate("FROSTED", frostedParams);
        break;
      case "VIGNETTE":
        onGenerate("VIGNETTE", vignetteParams);
        break;
    }
  };

  return (
    <div>
      <div className="flex items-center mb-4">
        <RectangleGroupIcon className="w-5 h-5 mr-2" />
        <h2 className="text-lg font-semibold">Photo Frames</h2>
      </div>

      <div className="tabs tabs-boxed mb-4">
        <a
          className={`tab ${activeFrame === "COLORED" ? "tab-active" : ""}`}
          onClick={() => setActiveFrame("COLORED")}
        >
          Color
        </a>
        <a
          className={`tab ${activeFrame === "FROSTED" ? "tab-active" : ""}`}
          onClick={() => setActiveFrame("FROSTED")}
        >
          Frosted
        </a>
        <a
          className={`tab ${activeFrame === "VIGNETTE" ? "tab-active" : ""}`}
          onClick={() => setActiveFrame("VIGNETTE")}
        >
          Vignette
        </a>
      </div>

      <div className="space-y-4 p-4 rounded-lg bg-base-100">
        {activeFrame === "COLORED" && (
          <div>
            <label className="label">
              Border Width: {coloredParams.border_width}
            </label>
            <input
              type="range"
              min={5}
              max={100}
              value={coloredParams.border_width}
              className="range range-primary"
              onChange={(e) =>
                setColoredParams((p) => ({
                  ...p,
                  border_width: Number(e.target.value),
                }))
              }
            />
            <label className="label mt-2">Border Color</label>
            <input
              type="color"
              value={`#${coloredParams.r.toString(16).padStart(2, "0")}${coloredParams.g.toString(16).padStart(2, "0")}${coloredParams.b.toString(16).padStart(2, "0")}`}
              className="w-full h-10 p-1"
              onChange={(e) => {
                const hex = e.target.value;
                setColoredParams((p) => ({
                  ...p,
                  r: parseInt(hex.slice(1, 3), 16),
                  g: parseInt(hex.slice(3, 5), 16),
                  b: parseInt(hex.slice(5, 7), 16),
                }));
              }}
            />
          </div>
        )}
        {activeFrame === "FROSTED" && (
          <div>
            <label className="label">
              Blur: {frostedParams.blur_sigma.toFixed(1)}
            </label>
            <input
              type="range"
              min={1}
              max={50}
              step={0.5}
              value={frostedParams.blur_sigma}
              className="range range-primary"
              onChange={(e) =>
                setFrostedParams((p) => ({
                  ...p,
                  blur_sigma: Number(e.target.value),
                }))
              }
            />
            <label className="label">
              Brightness: {frostedParams.brightness_adjustment}
            </label>
            <input
              type="range"
              min={-100}
              max={0}
              value={frostedParams.brightness_adjustment}
              className="range range-primary"
              onChange={(e) =>
                setFrostedParams((p) => ({
                  ...p,
                  brightness_adjustment: Number(e.target.value),
                }))
              }
            />
            <label className="label">
              Corner Radius: {frostedParams.corner_radius}
            </label>
            <input
              type="range"
              min={0}
              max={100}
              value={frostedParams.corner_radius}
              className="range range-primary"
              onChange={(e) =>
                setFrostedParams((p) => ({
                  ...p,
                  corner_radius: Number(e.target.value),
                }))
              }
            />
          </div>
        )}
        {activeFrame === "VIGNETTE" && (
          <div>
            <label className="label">
              Strength: {vignetteParams.strength.toFixed(2)}
            </label>
            <input
              type="range"
              min={0.1}
              max={1.5}
              step={0.05}
              value={vignetteParams.strength}
              className="range range-primary"
              onChange={(e) =>
                setVignetteParams((p) => ({
                  ...p,
                  strength: Number(e.target.value),
                }))
              }
            />
          </div>
        )}
      </div>

      <div className="mt-6">
        <button
          className="btn btn-primary w-full"
          onClick={handleGenerateClick}
          disabled={isGenerating}
        >
          {isGenerating ? (
            <span className="loading loading-spinner"></span>
          ) : (
            "Apply Border"
          )}
        </button>
      </div>
    </div>
  );
}
