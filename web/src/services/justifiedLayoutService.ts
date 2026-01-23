import { JustifiedLayout } from "@immich/justified-layout-wasm";
import type { Asset } from "@/lib/http-commons";
import { isPhotoMetadata } from "@/lib/http-commons";

export interface LayoutPosition {
  top: number;
  left: number;
  width: number;
  height: number;
}

export interface LayoutConfig {
  rowHeight: number;
  rowWidth: number;
  spacing: number;
  heightTolerance: number;
}

export interface LayoutResult {
  positions: LayoutPosition[];
  containerWidth: number;
  containerHeight: number;
}

export interface LayoutBox {
  width: number;
  height: number;
}

class JustifiedLayoutService {
  /**
   * Calculate layout.
   */
  calculateLayout(
    boxes: LayoutBox[],
    config: LayoutConfig,
  ): LayoutResult {
    if (boxes.length === 0) {
      return { positions: [], containerWidth: 0, containerHeight: 0 };
    }

    const aspectRatios = new Float32Array(
      boxes.map((b) => Math.max(0.1, Math.min(10, (b.width || 1) / (b.height || 1))))
    );

    // The WASM module might be using initSync internally or via the constructor.
    // We ensure the config is passed correctly as per the latest API.
    const layout = new JustifiedLayout(aspectRatios, {
      rowHeight: config.rowHeight,
      rowWidth: config.rowWidth,
      spacing: config.spacing,
      heightTolerance: config.heightTolerance,
    });

    return {
      positions: boxes.map((_, i) => {
        const p = layout.getPosition(i);
        return {
          top: Math.round(p.top),
          left: Math.round(p.left),
          width: Math.round(p.width),
          height: Math.round(p.height),
        };
      }),
      containerWidth: Math.round(layout.containerWidth),
      containerHeight: Math.round(layout.containerHeight),
    };
  }

  /**
   * Batch calculate layouts for multiple groups.
   */
  async calculateMultipleLayouts(
    groups: Record<string, LayoutBox[]>,
    config: LayoutConfig,
  ): Promise<Record<string, LayoutResult>> {
    const results: Record<string, LayoutResult> = {};

    for (const [groupKey, boxes] of Object.entries(groups)) {
      if (boxes.length > 0) {
        results[groupKey] = this.calculateLayout(boxes, config);
      }
    }

    return results;
  }

  /**
   * Create responsive configuration based on container width.
   */
  createResponsiveConfig(containerWidth: number): LayoutConfig {
    const width = Math.max(containerWidth, 300);
    
    let rowHeight = 220;
    if (width < 640) rowHeight = 140;
    else if (width < 1024) rowHeight = 180;

    return {
      rowHeight,
      rowWidth: width,
      spacing: 4,
      heightTolerance: 0.3,
    };
  }

  /**
   * Convert Asset objects to LayoutBox format.
   */
  assetToLayoutBox(asset: Asset): LayoutBox {
    let { width, height } = asset;

    if (!width || !height) {
      const metadata = asset.specific_metadata;
      if (isPhotoMetadata(asset.type, metadata) && metadata.dimensions) {
        const match = metadata.dimensions.match(/(\d+)(?:×|x)(\d+)/i);
        if (match) {
          width = parseInt(match[1], 10);
          height = parseInt(match[2], 10);
        }
      }
    }

    return {
      width: width || 3,
      height: height || 2,
    };
  }

  assetsToLayoutBoxes(assets: Asset[]): LayoutBox[] {
    return assets.map((asset) => this.assetToLayoutBox(asset));
  }

  // Compatibility methods for existing hooks
  async initialize(): Promise<void> { return Promise.resolve(); }
  isReady(): boolean { return true; }
}

export const justifiedLayoutService = new JustifiedLayoutService();
export default justifiedLayoutService;
