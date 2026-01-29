/// <reference lib="webworker" />

import { JustifiedLayout } from "@immich/justified-layout-wasm";
import type {
  LayoutBox,
  LayoutConfig,
  LayoutResult,
} from "@/lib/layout/justifiedLayout";

interface LayoutRequestPayload {
  requestId: number;
  boxes: LayoutBox[];
  config: LayoutConfig;
}

interface LayoutsRequestPayload {
  requestId: number;
  groups: Record<string, LayoutBox[]>;
  config: LayoutConfig;
}

type WorkerMessage =
  | { type: "INIT" }
  | { type: "CALCULATE_LAYOUT"; payload: LayoutRequestPayload }
  | { type: "CALCULATE_MULTIPLE_LAYOUTS"; payload: LayoutsRequestPayload };

const clampAspectRatio = (width: number, height: number) => {
  const safeWidth = width || 1;
  const safeHeight = height || 1;
  const ratio = safeWidth / safeHeight;
  return Math.max(0.1, Math.min(10, ratio));
};

const buildLayout = (boxes: LayoutBox[], config: LayoutConfig): LayoutResult => {
  if (boxes.length === 0) {
    return { positions: [], containerWidth: 0, containerHeight: 0 };
  }

  const aspectRatios = new Float32Array(
    boxes.map((box) => clampAspectRatio(box.width, box.height)),
  );

  const layout = new JustifiedLayout(aspectRatios, {
    rowHeight: config.rowHeight,
    rowWidth: config.rowWidth,
    spacing: config.spacing,
    heightTolerance: config.heightTolerance,
  });

  return {
    positions: boxes.map((_, i) => {
      const position = layout.getPosition(i);
      return {
        top: Math.round(position.top),
        left: Math.round(position.left),
        width: Math.round(position.width),
        height: Math.round(position.height),
      };
    }),
    containerWidth: Math.round(layout.containerWidth),
    containerHeight: Math.round(layout.containerHeight),
  };
};

let isInitialized = false;

const ensureInitialized = (config?: LayoutConfig) => {
  if (isInitialized) return;
  const seedConfig = config ?? {
    rowHeight: 1,
    rowWidth: 1,
    spacing: 0,
    heightTolerance: 0,
  };
  new JustifiedLayout(new Float32Array(0), seedConfig);
  isInitialized = true;
};

self.onmessage = (event: MessageEvent<WorkerMessage>) => {
  const { type } = event.data;

  switch (type) {
    case "INIT": {
      try {
        ensureInitialized();
        self.postMessage({ type: "JUSTIFIED_READY" });
      } catch (error) {
        self.postMessage({
          type: "ERROR",
          payload: {
            error: (error as Error).message || "Failed to initialize layout worker",
          },
        });
      }
      break;
    }

    case "CALCULATE_LAYOUT": {
      const { payload } = event.data;
      try {
        ensureInitialized(payload.config);
        const result = buildLayout(payload.boxes, payload.config);
        self.postMessage({
          type: "JUSTIFIED_LAYOUT_COMPLETE",
          payload: { requestId: payload.requestId, result },
        });
      } catch (error) {
        self.postMessage({
          type: "ERROR",
          payload: {
            requestId: payload.requestId,
            error: (error as Error).message || "Failed to calculate layout",
          },
        });
      }
      break;
    }

    case "CALCULATE_MULTIPLE_LAYOUTS": {
      const { payload } = event.data;
      try {
        ensureInitialized(payload.config);
        const results: Record<string, LayoutResult> = {};

        for (const [groupKey, boxes] of Object.entries(payload.groups)) {
          if (boxes.length > 0) {
            results[groupKey] = buildLayout(boxes, payload.config);
          }
        }

        self.postMessage({
          type: "JUSTIFIED_LAYOUTS_COMPLETE",
          payload: { requestId: payload.requestId, results },
        });
      } catch (error) {
        self.postMessage({
          type: "ERROR",
          payload: {
            requestId: payload.requestId,
            error: (error as Error).message || "Failed to calculate layouts",
          },
        });
      }
      break;
    }

    default:
      self.postMessage({
        type: "ERROR",
        payload: { error: "Unknown message type" },
      });
  }
};

export {};
