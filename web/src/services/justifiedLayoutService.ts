import { init, JustifiedLayout } from "@immich/justified-layout-wasm";
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
  private initialized = false;
  private initPromise: Promise<void> | null = null;
  private initializationError: Error | null = null;

  /**
   * Initialize the WASM module. This should be called once at app start.
   */
  async initialize(): Promise<void> {
    if (this.initialized) {
      return Promise.resolve();
    }

    if (this.initializationError) {
      throw this.initializationError;
    }

    if (this.initPromise) {
      return this.initPromise;
    }

    this.initPromise = this.performInitialization();
    return this.initPromise;
  }

  private async performInitialization(): Promise<void> {
    try {
      await init();
      this.initialized = true;
      console.log("Justified Layout WASM initialized successfully");
    } catch (error) {
      this.initializationError = error as Error;
      console.error("Failed to initialize Justified Layout WASM:", error);
      throw error;
    }
  }

  /**
   * Check if the service is ready to use
   */
  isReady(): boolean {
    return this.initialized;
  }

  /**
   * Get initialization error if any
   */
  getInitializationError(): Error | null {
    return this.initializationError;
  }

  /**
   * Calculate aspect ratio from box dimensions with fallbacks
   */
  private calculateAspectRatio(box: LayoutBox): number {
    const width = box.width || 1;
    const height = box.height || 1;

    // Ensure reasonable aspect ratio bounds
    const ratio = width / height;
    return Math.max(0.2, Math.min(5.0, ratio)); // clamp between 0.2 and 5.0
  }

  /**
   * Create justified layout for a set of boxes
   */
  async createLayout(
    boxes: LayoutBox[],
    config: LayoutConfig,
  ): Promise<LayoutResult> {
    if (!this.initialized) {
      throw new Error(
        "Justified Layout service not initialized. Call initialize() first.",
      );
    }

    if (boxes.length === 0) {
      return {
        positions: [],
        containerWidth: 0,
        containerHeight: 0,
      };
    }

    try {
      // Calculate aspect ratios
      const aspectRatios = new Float32Array(
        boxes.map((box) => this.calculateAspectRatio(box)),
      );

      // Validate aspect ratios
      if (aspectRatios.some((ratio) => !isFinite(ratio) || ratio <= 0)) {
        throw new Error("Invalid aspect ratios calculated");
      }

      // Create justified layout
      const layout = new JustifiedLayout(aspectRatios, {
        rowHeight: config.rowHeight,
        rowWidth: config.rowWidth,
        spacing: config.spacing,
        heightTolerance: config.heightTolerance,
      });

      // Extract positions
      const positions: LayoutPosition[] = [];
      for (let i = 0; i < boxes.length; i++) {
        const position = layout.getPosition(i);
        positions.push({
          top: Math.round(position.top),
          left: Math.round(position.left),
          width: Math.round(position.width),
          height: Math.round(position.height),
        });
      }

      return {
        positions,
        containerWidth: Math.round(layout.containerWidth),
        containerHeight: Math.round(layout.containerHeight),
      };
    } catch (error) {
      console.error("Failed to create justified layout:", error);
      throw new Error(`Layout calculation failed: ${(error as Error).message}`);
    }
  }

  /**
   * Create a fallback grid layout when justified layout fails
   */
  createFallbackLayout(boxes: LayoutBox[], config: LayoutConfig): LayoutResult {
    if (boxes.length === 0) {
      return {
        positions: [],
        containerWidth: 0,
        containerHeight: 0,
      };
    }

    // Calculate responsive columns based on row width
    const minItemWidth = 150;
    const columns = Math.max(
      1,
      Math.floor(
        (config.rowWidth + config.spacing) / (minItemWidth + config.spacing),
      ),
    );
    const itemWidth =
      (config.rowWidth - (columns - 1) * config.spacing) / columns;

    const positions: LayoutPosition[] = [];
    let maxHeight = 0;

    boxes.forEach((_, index) => {
      const row = Math.floor(index / columns);
      const col = index % columns;

      const left = col * (itemWidth + config.spacing);
      const top = row * (config.rowHeight + config.spacing);

      positions.push({
        top: Math.round(top),
        left: Math.round(left),
        width: Math.round(itemWidth),
        height: Math.round(config.rowHeight),
      });

      maxHeight = Math.max(maxHeight, top + config.rowHeight);
    });

    return {
      positions,
      containerWidth: Math.round(config.rowWidth),
      containerHeight: Math.round(maxHeight),
    };
  }

  /**
   * Calculate layout with automatic fallback on error
   */
  async calculateLayout(
    boxes: LayoutBox[],
    config: LayoutConfig,
  ): Promise<LayoutResult> {
    try {
      if (!this.initialized) {
        await this.initialize();
      }
      return await this.createLayout(boxes, config);
    } catch (error) {
      console.warn("Justified layout failed, using fallback grid:", error);
      return this.createFallbackLayout(boxes, config);
    }
  }

  /**
   * Batch calculate layouts for multiple groups
   */
  async calculateMultipleLayouts(
    groups: Record<string, LayoutBox[]>,
    config: LayoutConfig,
  ): Promise<Record<string, LayoutResult>> {
    const results: Record<string, LayoutResult> = {};

    // Process groups sequentially to avoid overwhelming the WASM module
    for (const [groupKey, boxes] of Object.entries(groups)) {
      if (boxes.length > 0) {
        results[groupKey] = await this.calculateLayout(boxes, config);
      }
    }

    return results;
  }

  /**
   * Create responsive configuration based on container width with better column handling
   * Note: containerWidth should be the actual available width for content (after padding)
   */
  createResponsiveConfig(containerWidth: number): LayoutConfig {
    // Use the provided width directly (component should pass width after padding)
    const availableWidth = Math.max(containerWidth, 300); // Minimum 300px

    // Define responsive breakpoints for columns based on available space
    let targetColumns: number;
    let rowHeight: number;

    if (availableWidth < 480) {
      // Very small containers: 2 columns
      targetColumns = 2;
      rowHeight = 140;
    } else if (availableWidth < 640) {
      // Small containers: 3 columns
      targetColumns = 3;
      rowHeight = 160;
    } else if (availableWidth < 768) {
      // Small-medium containers: 3-4 columns
      targetColumns = 3;
      rowHeight = 180;
    } else if (availableWidth < 1024) {
      // Medium containers: 4 columns
      targetColumns = 4;
      rowHeight = 200;
    } else if (availableWidth < 1280) {
      // Large containers: 5 columns
      targetColumns = 5;
      rowHeight = 220;
    } else if (availableWidth < 1536) {
      // Extra large containers: 6 columns
      targetColumns = 6;
      rowHeight = 240;
    } else {
      // Ultra large containers: 7+ columns
      targetColumns = Math.min(Math.floor(availableWidth / 220), 8);
      rowHeight = 240;
    }

    // Calculate optimal row width for justified layout
    const minItemWidth = 140; // Minimum width per item
    const spacing = 4;
    const minRowWidth =
      targetColumns * minItemWidth + (targetColumns - 1) * spacing;

    // Use available width, but ensure minimum requirements
    const rowWidth = Math.max(availableWidth, minRowWidth);

    return {
      rowHeight,
      rowWidth,
      spacing,
      heightTolerance:
        availableWidth < 480 ? 0.4 : availableWidth < 768 ? 0.3 : 0.25,
    };
  }

  /**
   * Convert Asset objects to LayoutBox format
   */
  assetToLayoutBox(asset: Asset): LayoutBox {
    let width = asset.width;
    let height = asset.height;

    // Try to extract dimensions from metadata if missing
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

    // Use fallback dimensions if still missing
    return {
      width: width || 4, // Default 4:3 aspect ratio
      height: height || 3,
    };
  }

  /**
   * Convert multiple assets to layout boxes
   */
  assetsToLayoutBoxes(assets: Asset[]): LayoutBox[] {
    return assets.map((asset) => this.assetToLayoutBox(asset));
  }
}

// Create and export singleton instance
export const justifiedLayoutService = new JustifiedLayoutService();

// Export default for convenience
export default justifiedLayoutService;
