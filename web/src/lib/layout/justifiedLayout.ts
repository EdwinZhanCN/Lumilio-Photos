import type { Asset } from "@/lib/http-commons";

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
    spacing: 2,
    heightTolerance: 0.3,
  };
};

export const assetToLayoutBox = (asset: Asset): LayoutBox => {
  // Layout uses the orientation-corrected top-level width/height. Missing
  // dimensions default to 3:2.
  const { width, height } = asset;

  return {
    width: width || 3,
    height: height || 2,
  };
};

export const assetsToLayoutBoxes = (assets: Asset[]): LayoutBox[] =>
  assets.map((asset) => assetToLayoutBox(asset));
