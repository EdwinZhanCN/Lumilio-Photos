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

export const createResponsiveConfig = (containerWidth: number): LayoutConfig => {
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
};

export const assetToLayoutBox = (asset: Asset): LayoutBox => {
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
};

export const assetsToLayoutBoxes = (assets: Asset[]): LayoutBox[] =>
  assets.map((asset) => assetToLayoutBox(asset));
