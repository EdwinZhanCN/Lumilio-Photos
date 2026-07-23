import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { RotateCw } from "lucide-react";
import { displayText, isTextLayer, type Layer, type TextLayer } from "../../../model/layers";
import { measureTextLayer } from "../../../modules/rendering/renderLayers";
import { ensureStudioFontsLoaded } from "../../../modules/rendering/fonts/loadStudioFonts";
import { withOpacity } from "../../../modules/rendering/canvasUtils";

type TextOverlayProps = {
  layers: readonly Layer[];
  selectedLayerId: string | null;
  onSelectLayer: (id: string | null) => void;
  onLayersChange: (next: Layer[]) => void;
  /** A layer is being dragged: the worker should stop rendering it so the live
   * DOM preview is the only copy (no lagging duplicate). Null clears it. */
  onInteractLayer: (id: string | null) => void;
};

/** Live geometry of the layer under the pointer, kept local so dragging is instant. */
type Draft = { id: string; x: number; y: number; size: number; rotation: number };

type DragState =
  | { mode: "move"; startX: number; startY: number; from: Draft }
  | { mode: "scale"; cx: number; cy: number; startDist: number; from: Draft }
  | { mode: "rotate"; cx: number; cy: number };

const CORNERS = [
  { x: 0, y: 0, cursor: "nwse-resize" },
  { x: 1, y: 0, cursor: "nesw-resize" },
  { x: 1, y: 1, cursor: "nwse-resize" },
  { x: 0, y: 1, cursor: "nesw-resize" },
];

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

function draftOf(layer: TextLayer): Draft {
  return { id: layer.id, x: layer.x, y: layer.y, size: layer.font.size, rotation: layer.rotation };
}

/**
 * On-canvas text editing, drawn over the viewport. Select, drag to move, corner
 * handles to scale the font, the top handle to rotate, double-click to edit.
 *
 * Text is rasterized by the worker, which is too slow to re-render every
 * pointer move — so a drag runs entirely on a local {@link Draft}: the selection
 * box and a DOM text preview follow the pointer instantly, the worker is told to
 * hide that layer (no stale duplicate underneath), and only on release does the
 * new geometry commit and the worker re-rasterize once. Same idea as AfterFrame,
 * which keeps text as live DOM and rasterizes on save.
 */
export function TextOverlay({
  layers,
  selectedLayerId,
  onSelectLayer,
  onLayersChange,
  onInteractLayer,
}: TextOverlayProps): React.JSX.Element {
  const rootRef = useRef<HTMLDivElement>(null);
  const [box, setBox] = useState({ w: 0, h: 0 });
  const [fontsReady, setFontsReady] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState<Draft | null>(null);
  const draftRef = useRef<Draft | null>(null);
  const dragRef = useRef<DragState | null>(null);

  const layersRef = useRef(layers);
  layersRef.current = layers;
  const onLayersChangeRef = useRef(onLayersChange);
  onLayersChangeRef.current = onLayersChange;
  const onInteractRef = useRef(onInteractLayer);
  onInteractRef.current = onInteractLayer;

  const measureCtx = useMemo(() => {
    try {
      return new OffscreenCanvas(1, 1).getContext("2d");
    } catch {
      return null;
    }
  }, []);

  useEffect(() => {
    void ensureStudioFontsLoaded().then(() => setFontsReady(true));
  }, []);

  useEffect(() => {
    const el = rootRef.current;
    if (!el) return;
    const update = () => setBox({ w: el.clientWidth, h: el.clientHeight });
    update();
    const ro = new ResizeObserver(update);
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  const setActiveDraft = useCallback((next: Draft | null) => {
    draftRef.current = next;
    setDraft(next);
  }, []);

  // Geometry to render a layer with — the live draft while dragging, else saved.
  const geom = useCallback(
    (layer: TextLayer): Draft => (draft && draft.id === layer.id ? draft : draftOf(layer)),
    [draft],
  );

  const boxOf = useCallback(
    (layer: TextLayer, g: Draft): { cx: number; cy: number; width: number; height: number } => {
      const fontPx = g.size * box.w;
      let width = fontPx * 2;
      let height = fontPx * 1.4;
      if (measureCtx && displayText(layer)) {
        const probe: TextLayer = { ...layer, font: { ...layer.font, size: g.size } };
        const metrics = measureTextLayer(measureCtx, probe, box.w);
        if (metrics.width > 0) {
          width = metrics.width;
          height = metrics.height;
        }
      }
      return {
        cx: g.x * box.w,
        cy: g.y * box.h,
        width: Math.max(width, 12),
        height: Math.max(height, 12),
      };
    },
    // fontsReady forces a re-measure once faces load.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [box.w, box.h, measureCtx, fontsReady],
  );

  const pointerXY = useCallback((event: PointerEvent | React.PointerEvent) => {
    const rect = rootRef.current?.getBoundingClientRect();
    if (!rect) return { x: 0, y: 0 };
    return { x: event.clientX - rect.left, y: event.clientY - rect.top };
  }, []);

  const commit = useCallback((next: Draft) => {
    onLayersChangeRef.current(
      layersRef.current.map((layer) =>
        layer.id === next.id && isTextLayer(layer)
          ? {
              ...layer,
              x: next.x,
              y: next.y,
              rotation: next.rotation,
              font: { ...layer.font, size: next.size },
            }
          : layer,
      ),
    );
  }, []);

  const onMove = useCallback(
    (event: PointerEvent) => {
      const drag = dragRef.current;
      const current = draftRef.current;
      if (!drag || !current) return;
      const point = pointerXY(event);
      if (drag.mode === "move") {
        setActiveDraft({
          ...current,
          x: clamp(drag.from.x + (point.x - drag.startX) / box.w, -0.2, 1.2),
          y: clamp(drag.from.y + (point.y - drag.startY) / box.h, -0.2, 1.2),
        });
      } else if (drag.mode === "scale") {
        const dist = Math.hypot(point.x - drag.cx, point.y - drag.cy);
        const ratio = drag.startDist > 0 ? dist / drag.startDist : 1;
        setActiveDraft({ ...current, size: clamp(drag.from.size * ratio, 0.005, 0.6) });
      } else {
        const angle = (Math.atan2(point.y - drag.cy, point.x - drag.cx) * 180) / Math.PI + 90;
        setActiveDraft({ ...current, rotation: Math.round(angle) });
      }
    },
    [pointerXY, box.w, box.h, setActiveDraft],
  );

  const onUp = useCallback(() => {
    window.removeEventListener("pointermove", onMove);
    window.removeEventListener("pointerup", onUp);
    dragRef.current = null;
    const finished = draftRef.current;
    if (finished) commit(finished);
    setActiveDraft(null);
    onInteractRef.current(null);
  }, [onMove, commit, setActiveDraft]);

  const beginDrag = useCallback(
    (layer: TextLayer, drag: (from: Draft) => DragState, event: React.PointerEvent) => {
      event.preventDefault();
      event.stopPropagation();
      const from = draftOf(layer);
      setActiveDraft(from);
      dragRef.current = drag(from);
      onInteractRef.current(layer.id);
      window.addEventListener("pointermove", onMove);
      window.addEventListener("pointerup", onUp);
    },
    [onMove, onUp, setActiveDraft],
  );

  useEffect(
    () => () => {
      window.removeEventListener("pointermove", onMove);
      window.removeEventListener("pointerup", onUp);
    },
    [onMove, onUp],
  );

  const patchText = useCallback((id: string, text: string) => {
    onLayersChangeRef.current(
      layersRef.current.map((layer) =>
        layer.id === id && isTextLayer(layer) ? { ...layer, text } : layer,
      ),
    );
  }, []);

  const textLayers = layers.filter(isTextLayer);

  return (
    <div
      ref={rootRef}
      className="absolute inset-0"
      style={{ touchAction: "none" }}
      onPointerDown={(event) => {
        if (event.target === rootRef.current) onSelectLayer(null);
      }}
    >
      {textLayers.map((layer) => {
        const g = geom(layer);
        const rect = boxOf(layer, g);
        const selected = layer.id === selectedLayerId;
        const editing = editingId === layer.id;
        const dragging = draft?.id === layer.id;
        return (
          <div
            key={layer.id}
            className="absolute"
            style={{
              left: rect.cx,
              top: rect.cy,
              width: rect.width,
              height: rect.height,
              transform: `translate(-50%, -50%) rotate(${g.rotation}deg)`,
            }}
          >
            {/* Live DOM text while dragging: instant, replaces the hidden raster. */}
            {dragging && (
              <div
                className="pointer-events-none absolute inset-0 flex items-center justify-center overflow-visible"
                style={textPreviewStyle(layer, g.size * box.w)}
              >
                {displayText(layer)}
              </div>
            )}

            <div
              className={`absolute inset-0 cursor-move rounded-sm ${
                selected
                  ? "outline outline-1 outline-primary"
                  : "hover:outline hover:outline-1 hover:outline-white/60"
              }`}
              onPointerDown={(event) => {
                onSelectLayer(layer.id);
                beginDrag(
                  layer,
                  (from) => ({
                    mode: "move",
                    startX: pointerXY(event).x,
                    startY: pointerXY(event).y,
                    from,
                  }),
                  event,
                );
              }}
              onDoubleClick={(event) => {
                event.stopPropagation();
                setEditingId(layer.id);
              }}
            />

            {selected && !editing && (
              <>
                {CORNERS.map((corner) => (
                  <div
                    key={`${corner.x}-${corner.y}`}
                    className="absolute h-3 w-3 rounded-full border border-primary bg-base-100"
                    style={{
                      left: `${corner.x * 100}%`,
                      top: `${corner.y * 100}%`,
                      transform: "translate(-50%, -50%)",
                      cursor: corner.cursor,
                    }}
                    onPointerDown={(event) =>
                      beginDrag(
                        layer,
                        (from) => ({
                          mode: "scale",
                          cx: rect.cx,
                          cy: rect.cy,
                          startDist: Math.hypot(
                            pointerXY(event).x - rect.cx,
                            pointerXY(event).y - rect.cy,
                          ),
                          from,
                        }),
                        event,
                      )
                    }
                  />
                ))}
                <div
                  className="absolute left-1/2 flex h-5 w-5 -translate-x-1/2 items-center justify-center rounded-full border border-primary bg-base-100 text-primary"
                  style={{ top: -26, cursor: "grab" }}
                  onPointerDown={(event) =>
                    beginDrag(layer, () => ({ mode: "rotate", cx: rect.cx, cy: rect.cy }), event)
                  }
                >
                  <RotateCw size={11} />
                </div>
              </>
            )}

            {editing && (
              <textarea
                autoFocus
                value={layer.text}
                onChange={(event) => patchText(layer.id, event.target.value)}
                onBlur={() => setEditingId(null)}
                onKeyDown={(event) => {
                  if (event.key === "Escape") setEditingId(null);
                }}
                className="absolute inset-0 resize-none rounded-sm border border-primary bg-base-100/90 p-0.5 text-center text-xs text-base-content outline-none"
                style={{ transform: `rotate(${-layer.rotation}deg)` }}
              />
            )}
          </div>
        );
      })}
    </div>
  );
}

/** Approximate CSS for the transient drag preview — close enough to place by. */
function textPreviewStyle(layer: TextLayer, fontPx: number): React.CSSProperties {
  const base: React.CSSProperties = {
    fontFamily: layer.font.family,
    fontWeight: layer.font.weight,
    fontStyle: layer.font.italic ? "italic" : "normal",
    fontSize: `${fontPx}px`,
    lineHeight: layer.font.lineHeight,
    letterSpacing: `${layer.font.tracking * fontPx}px`,
    textAlign: layer.align,
    whiteSpace: "pre",
    opacity: layer.opacity,
  };
  if (layer.fill.kind === "gradient") {
    return {
      ...base,
      backgroundImage: `linear-gradient(${layer.fill.angle}deg, ${withOpacity(layer.fill.from, layer.fill.fromOpacity)}, ${withOpacity(layer.fill.to, layer.fill.toOpacity)})`,
      WebkitBackgroundClip: "text",
      backgroundClip: "text",
      color: "transparent",
    };
  }
  return { ...base, color: withOpacity(layer.fill.color, layer.fill.opacity) };
}
