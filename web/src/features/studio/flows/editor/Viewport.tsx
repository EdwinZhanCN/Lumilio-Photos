import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Maximize2, Minus, Plus, SlidersHorizontal, TriangleAlert, X } from "lucide-react";
import { useI18n } from "@/lib/i18n";

const VIEWPORT_PAD = 48;
const MIN_ZOOM = 0.25;
const MAX_ZOOM = 5;

type ViewportProps = {
  /**
   * Receives the preview canvas element once, so the editor can hand its
   * control to the render worker via `transferControlToOffscreen`.
   */
  onCanvasReady: (canvas: HTMLCanvasElement) => void;
  /**
   * Identity of the current photo. A change remounts the canvas, because
   * `transferControlToOffscreen` may be called only once per element, so each
   * asset needs a fresh one to hand to its worker.
   */
  canvasKey: string;
  /** Untouched original object URL, shown while "Before" is held. */
  originalUrl: string | null;
  showOriginal: boolean;
  /** Aspect (w/h) of the composed preview the worker draws — sizes the canvas box. */
  outputAspect: number;
  /** Aspect (w/h) of the untouched original — sizes the "Before" image box. */
  originalAspect: number;
  /** True once the worker has drawn at least one frame. */
  ready: boolean;
  loading: boolean;
  error: string | null;
  onDismissError: () => void;
  fileName: string;
  /** Rendered over the canvas box (e.g. the crop overlay), aligned to the image. */
  overlay?: React.ReactNode;
};

export function Viewport({
  onCanvasReady,
  canvasKey,
  originalUrl,
  showOriginal,
  outputAspect,
  originalAspect,
  ready,
  loading,
  error,
  onDismissError,
  fileName,
  overlay,
}: ViewportProps): React.JSX.Element {
  const { t } = useI18n();
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const [box, setBox] = useState({ w: 0, h: 0 });
  const [zoom, setZoom] = useState(1);
  const [fitMode, setFitMode] = useState(true);

  const displayedAr = showOriginal ? originalAspect : outputAspect;

  const canvasCallback = useCallback(
    (node: HTMLCanvasElement | null) => {
      if (node) onCanvasReady(node);
    },
    [onCanvasReady],
  );

  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    setBox({ w: el.clientWidth, h: el.clientHeight });
    const ro = new ResizeObserver((entries) => {
      const cr = entries[0].contentRect;
      setBox({ w: cr.width, h: cr.height });
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  const fit = useMemo(() => {
    const availW = Math.max(0, box.w - VIEWPORT_PAD * 2);
    const availH = Math.max(0, box.h - VIEWPORT_PAD * 2);
    if (availW <= 0 || availH <= 0 || !Number.isFinite(displayedAr) || displayedAr <= 0) {
      return { w: 0, h: 0 };
    }
    let w = availW;
    let h = w / displayedAr;
    if (h > availH) {
      h = availH;
      w = h * displayedAr;
    }
    return { w, h };
  }, [box, displayedAr]);

  const eff = fitMode ? 1 : zoom;
  const boxW = fit.w * eff;
  const boxH = fit.h * eff;

  const clampZoom = (z: number) => Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, z));
  const zoomTo = useCallback((z: number) => {
    setFitMode(false);
    setZoom(clampZoom(z));
  }, []);
  const doFit = useCallback(() => {
    setFitMode(true);
    setZoom(1);
  }, []);
  const toggleZoom = useCallback(() => {
    if (fitMode) zoomTo(2);
    else doFit();
  }, [fitMode, zoomTo, doFit]);

  const onWheel = useCallback(
    (e: React.WheelEvent) => {
      if (!(e.ctrlKey || e.metaKey)) return;
      e.preventDefault();
      const factor = e.deltaY < 0 ? 1.12 : 1 / 1.12;
      zoomTo((fitMode ? 1 : zoom) * factor);
    },
    [fitMode, zoom, zoomTo],
  );

  const showWaiting = !ready && !showOriginal;

  return (
    <div className="relative flex min-w-0 flex-1">
      <div
        ref={scrollRef}
        onWheel={onWheel}
        onDoubleClick={toggleZoom}
        className="absolute inset-0 overflow-auto bg-base-300/60"
        style={{
          backgroundImage:
            "linear-gradient(45deg,oklch(0 0 0/0.04) 25%,transparent 25%,transparent 75%,oklch(0 0 0/0.04) 75%),linear-gradient(45deg,oklch(0 0 0/0.04) 25%,transparent 25%,transparent 75%,oklch(0 0 0/0.04) 75%)",
          backgroundSize: "24px 24px",
          backgroundPosition: "0 0,12px 12px",
        }}
      >
        <div
          className="flex min-h-full min-w-full items-center justify-center"
          style={{ padding: `${VIEWPORT_PAD}px` }}
        >
          <div
            className={`relative shrink-0 ${fitMode ? "" : "cursor-zoom-out"}`}
            style={{ width: `${boxW}px`, height: `${boxH}px` }}
          >
            {/* The worker owns this canvas after transferControlToOffscreen; we
                never touch its backing size, only its CSS display size. */}
            <canvas
              key={canvasKey}
              ref={canvasCallback}
              className="absolute inset-0 h-full w-full rounded-md shadow-2xl ring-1 ring-black/10"
              style={{ display: showOriginal ? "none" : "block" }}
            />
            {showOriginal && originalUrl && (
              <img
                src={originalUrl}
                alt={fileName}
                className="absolute inset-0 h-full w-full rounded-md object-contain shadow-2xl ring-1 ring-black/10"
                draggable={false}
              />
            )}
            {overlay}
          </div>

          {showWaiting && boxW <= 0 && (
            <div className="flex flex-col items-center gap-3 text-base-content/40">
              <SlidersHorizontal className="h-12 w-12" />
              <span className="text-sm">
                {t("studio.editor.waiting", { defaultValue: "Waiting for preview" })}
              </span>
            </div>
          )}
        </div>
      </div>

      {/* Error banner */}
      {error && (
        <div className="pointer-events-none absolute inset-x-4 top-4 z-30 mx-auto max-w-lg">
          <div role="alert" className="alert alert-error pointer-events-auto shadow-lg">
            <TriangleAlert size={18} />
            <div className="min-w-0">
              <div className="text-sm font-semibold">
                {t("studio.editor.renderFailed", { defaultValue: "Render failed" })}
              </div>
              <div className="truncate text-xs opacity-80">{error}</div>
            </div>
            <button
              type="button"
              onClick={onDismissError}
              aria-label={t("common.dismiss", { defaultValue: "Dismiss" })}
              className="btn btn-ghost btn-xs"
            >
              <X size={14} />
            </button>
          </div>
        </div>
      )}

      {/* Loading pill */}
      {loading && (
        <div className="pointer-events-none absolute bottom-4 left-1/2 z-30 -translate-x-1/2">
          <div className="flex items-center gap-2 rounded-full border border-base-300 bg-base-100/90 px-3.5 py-1.5 shadow-lg backdrop-blur">
            <span className="loading loading-spinner loading-xs text-primary" />
            <span className="text-xs font-medium text-base-content/70">
              {t("studio.editor.rendering", { defaultValue: "Rendering preview…" })}
            </span>
          </div>
        </div>
      )}

      {/* Zoom controls */}
      <div className="absolute bottom-4 right-4 z-30 flex items-center gap-0.5 rounded-lg border border-base-300 bg-base-100/90 p-1 shadow-lg backdrop-blur">
        <button
          type="button"
          onClick={() => zoomTo(eff / 1.25)}
          disabled={eff <= MIN_ZOOM + 0.001}
          aria-label={t("studio.editor.zoomOut", { defaultValue: "Zoom out" })}
          className="btn btn-ghost btn-xs btn-square text-base-content/70"
        >
          <Minus size={15} />
        </button>
        <button
          type="button"
          onClick={doFit}
          aria-label={t("studio.editor.fitToScreen", { defaultValue: "Fit to screen" })}
          className="btn btn-ghost btn-xs min-w-[3.25rem] font-mono text-[11px] tabular-nums text-base-content/80"
        >
          {fitMode ? t("studio.editor.fit", { defaultValue: "Fit" }) : `${Math.round(eff * 100)}%`}
        </button>
        <button
          type="button"
          onClick={() => zoomTo(eff * 1.25)}
          disabled={eff >= MAX_ZOOM - 0.001}
          aria-label={t("studio.editor.zoomIn", { defaultValue: "Zoom in" })}
          className="btn btn-ghost btn-xs btn-square text-base-content/70"
        >
          <Plus size={15} />
        </button>
        <div className="mx-0.5 h-4 w-px bg-base-300" />
        <div
          className="tooltip tooltip-left"
          data-tip={t("studio.editor.fit", { defaultValue: "Fit to screen" })}
        >
          <button
            type="button"
            onClick={doFit}
            aria-label={t("studio.editor.resetZoom", { defaultValue: "Reset zoom" })}
            className="btn btn-ghost btn-xs btn-square text-base-content/70"
          >
            <Maximize2 size={14} />
          </button>
        </div>
      </div>
    </div>
  );
}
