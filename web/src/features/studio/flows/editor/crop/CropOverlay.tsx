import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  displayedFrameSize,
  mapRectDisplayedToSource,
  mapRectSourceToDisplayed,
  type SourceRect,
} from "../../../modules/rendering/coordinateSystem";
import {
  createDefaultCropRect,
  moveCropRect,
  resizeCropRect,
  type CropHandle,
  type CropPoint,
  type CropRect,
} from "../../../modules/crop/cropMath";

type CropOverlayProps = {
  sourceWidth: number;
  sourceHeight: number;
  rotation: number;
  flipHorizontal: boolean;
  flipVertical: boolean;
  /** Resolved numeric aspect (or null for free), in DISPLAYED orientation. */
  aspect: number | null;
  /** Saved crop in source pixels to seed from, or null for the whole frame. */
  initialCrop: SourceRect | null;
  /** Emits the committed crop in source pixels, or null when it covers the frame. */
  onChange: (crop: SourceRect | null) => void;
};

const HANDLES: Array<{ key: CropHandle; x: string; y: string; cursor: string }> = [
  { key: "nw", x: "0%", y: "0%", cursor: "nwse-resize" },
  { key: "n", x: "50%", y: "0%", cursor: "ns-resize" },
  { key: "ne", x: "100%", y: "0%", cursor: "nesw-resize" },
  { key: "e", x: "100%", y: "50%", cursor: "ew-resize" },
  { key: "se", x: "100%", y: "100%", cursor: "nwse-resize" },
  { key: "s", x: "50%", y: "100%", cursor: "ns-resize" },
  { key: "sw", x: "0%", y: "100%", cursor: "nesw-resize" },
  { key: "w", x: "0%", y: "50%", cursor: "ew-resize" },
];

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

/**
 * Crop box editor drawn over the viewport. It works entirely in the displayed
 * (rotated) frame using {@link cropMath}, then maps the committed rectangle back
 * to source pixels via the coordinate system — so the stored crop is
 * resolution- and orientation-independent.
 */
export function CropOverlay({
  sourceWidth,
  sourceHeight,
  rotation,
  flipHorizontal,
  flipVertical,
  aspect,
  initialCrop,
  onChange,
}: CropOverlayProps): React.JSX.Element {
  const rootRef = useRef<HTMLDivElement>(null);
  const bounds = useMemo(
    () => displayedFrameSize(sourceWidth, sourceHeight, rotation),
    [sourceWidth, sourceHeight, rotation],
  );

  const seed = useCallback(
    (): CropRect =>
      initialCrop
        ? mapRectSourceToDisplayed(
            initialCrop,
            sourceWidth,
            sourceHeight,
            rotation,
            flipHorizontal,
            flipVertical,
          )
        : { x: 0, y: 0, width: bounds.width, height: bounds.height },
    // Seed only on mount; later changes are driven by aspect/rotation effects.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [],
  );

  const [draft, setDraft] = useState<CropRect>(seed);
  const draftRef = useRef(draft);
  const applyDraft = useCallback((rect: CropRect) => {
    draftRef.current = rect;
    setDraft(rect);
  }, []);

  const emit = useCallback(
    (rect: CropRect) => {
      const isFull =
        rect.x <= 1 &&
        rect.y <= 1 &&
        rect.width >= bounds.width - 1 &&
        rect.height >= bounds.height - 1;
      onChange(
        isFull
          ? null
          : mapRectDisplayedToSource(
              rect,
              sourceWidth,
              sourceHeight,
              rotation,
              flipHorizontal,
              flipVertical,
            ),
      );
    },
    [bounds, sourceWidth, sourceHeight, rotation, flipHorizontal, flipVertical, onChange],
  );

  // A new aspect re-fits the largest centered crop; a rotation invalidates the
  // frame, so reset to full. Both re-emit so the stored crop stays in sync.
  const aspectRef = useRef(aspect);
  useEffect(() => {
    if (aspectRef.current === aspect) return;
    aspectRef.current = aspect;
    const next = createDefaultCropRect(bounds, aspect);
    applyDraft(next);
    emit(next);
  }, [aspect, bounds, applyDraft, emit]);

  const boundsKey = `${bounds.width}x${bounds.height}`;
  const boundsRef = useRef(boundsKey);
  useEffect(() => {
    if (boundsRef.current === boundsKey) return;
    boundsRef.current = boundsKey;
    const next = { x: 0, y: 0, width: bounds.width, height: bounds.height };
    applyDraft(next);
    emit(next);
  }, [boundsKey, bounds, applyDraft, emit]);

  const dragRef = useRef<{
    handle: CropHandle | "move";
    startPoint: CropPoint;
    startRect: CropRect;
  } | null>(null);

  const pointToBounds = useCallback(
    (event: PointerEvent | React.PointerEvent): CropPoint => {
      const rect = rootRef.current?.getBoundingClientRect();
      if (!rect || rect.width === 0) return { x: 0, y: 0 };
      const scaleX = bounds.width / rect.width;
      const scaleY = bounds.height / rect.height;
      return {
        x: clamp((event.clientX - rect.left) * scaleX, 0, bounds.width),
        y: clamp((event.clientY - rect.top) * scaleY, 0, bounds.height),
      };
    },
    [bounds],
  );

  const onPointerMove = useCallback(
    (event: PointerEvent) => {
      const drag = dragRef.current;
      if (!drag) return;
      const point = pointToBounds(event);
      const next =
        drag.handle === "move"
          ? moveCropRect(
              drag.startRect,
              bounds,
              point.x - drag.startPoint.x,
              point.y - drag.startPoint.y,
            )
          : resizeCropRect(drag.startRect, drag.handle, point, bounds, aspect);
      applyDraft(next);
    },
    [pointToBounds, bounds, aspect, applyDraft],
  );

  const onPointerUp = useCallback(() => {
    window.removeEventListener("pointermove", onPointerMove);
    window.removeEventListener("pointerup", onPointerUp);
    if (dragRef.current) {
      emit(draftRef.current);
      dragRef.current = null;
    }
  }, [onPointerMove, emit]);

  const beginDrag = useCallback(
    (handle: CropHandle | "move", event: React.PointerEvent) => {
      event.preventDefault();
      event.stopPropagation();
      dragRef.current = { handle, startPoint: pointToBounds(event), startRect: draftRef.current };
      window.addEventListener("pointermove", onPointerMove);
      window.addEventListener("pointerup", onPointerUp);
    },
    [pointToBounds, onPointerMove, onPointerUp],
  );

  useEffect(
    () => () => {
      window.removeEventListener("pointermove", onPointerMove);
      window.removeEventListener("pointerup", onPointerUp);
    },
    [onPointerMove, onPointerUp],
  );

  const pct = (value: number, total: number) => `${(value / total) * 100}%`;
  const box = {
    left: pct(draft.x, bounds.width),
    top: pct(draft.y, bounds.height),
    width: pct(draft.width, bounds.width),
    height: pct(draft.height, bounds.height),
  };

  return (
    <div ref={rootRef} className="absolute inset-0 select-none" style={{ touchAction: "none" }}>
      {/* Dim outside the crop with a single box-shadow spill. */}
      <div
        className="pointer-events-none absolute"
        style={{ ...box, boxShadow: "0 0 0 100vmax rgba(0,0,0,0.5)" }}
      />

      {/* Crop frame: move target, thirds grid, and 8 handles. */}
      <div
        className="absolute cursor-move"
        style={{ ...box, outline: "1.5px solid rgba(255,255,255,0.9)" }}
        onPointerDown={(event) => beginDrag("move", event)}
      >
        <div className="pointer-events-none absolute inset-y-0 left-1/3 w-px bg-white/50" />
        <div className="pointer-events-none absolute inset-y-0 left-2/3 w-px bg-white/50" />
        <div className="pointer-events-none absolute inset-x-0 top-1/3 h-px bg-white/50" />
        <div className="pointer-events-none absolute inset-x-0 top-2/3 h-px bg-white/50" />

        {HANDLES.map((handle) => (
          <div
            key={handle.key}
            className="absolute h-4 w-4 rounded-full border-2 border-white bg-white/20"
            style={{
              left: handle.x,
              top: handle.y,
              transform: "translate(-50%, -50%)",
              cursor: handle.cursor,
            }}
            onPointerDown={(event) => beginDrag(handle.key, event)}
          />
        ))}
      </div>
    </div>
  );
}
