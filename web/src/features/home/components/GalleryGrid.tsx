import React, { useState } from "react";
import { ExifInfo } from "@/lib/utils/exif-info.ts";
import {
  Camera,
  ScanSearch,
  Aperture,
  Clock,
  Ruler,
  Gauge,
} from "lucide-react";

export type GalleryGridProps = {
  /**
   * Number of placeholder items in the grid.
   * Default: 8
   */
  items?: number;
  /**
   * Optional className applied to the root section.
   */
  className?: string;
  /**
   * Called when a grid item is clicked.
   */
  onItemClick?: (index: number) => void;
  /**
   * Override how EXIF info is fetched per index.
   * By default, uses ExifInfo.getSample().
   */
  getExif?: (index: number) => {
    camera: string;
    lens: string;
    aperture: string | number;
    shutter: string;
    focalLength: string;
    iso: string | number;
  };
  /**
   * Optional: Replace how each tile renders, given index and hover state.
   * If provided, you are responsible for the inner content.
   */
  renderItem?: (index: number, hovered: boolean) => React.ReactNode;
  /**
   * The label shown at the bottom-left of the image tile.
   * Example: "示例照片" (Sample Photo)
   * Default: "示例照片"
   */
  titlePrefix?: string;
};

/**
 * GalleryGrid
 * Extracted from Home page to a reusable component.
 * - Displays a grid of square tiles with hover overlay showing EXIF info.
 * - Matches the original styles used on the Home page.
 */
const GalleryGrid: React.FC<GalleryGridProps> = ({
  items = 8,
  className = "",
  onItemClick,
  getExif,
  renderItem,
  titlePrefix = "示例照片",
}) => {
  const [hoverIndex, setHoverIndex] = useState<number | null>(null);

  const resolveExif = (index: number) => {
    if (getExif) return getExif(index);
    const sample = ExifInfo.getSample();
    return {
      camera: sample.camera,
      lens: sample.lens,
      aperture: sample.aperture,
      shutter: sample.shutter,
      focalLength: sample.focalLength,
      iso: sample.iso,
    };
  };

  const renderExifOverlay = (index: number) => {
    const exif = resolveExif(index);
    const rows = [
      { icon: Camera, label: "Camera", value: exif.camera },
      { icon: ScanSearch, label: "Lens", value: exif.lens },
      { icon: Aperture, label: "Aperture", value: `f/${exif.aperture}` },
      { icon: Clock, label: "Shutter", value: exif.shutter },
      { icon: Ruler, label: "Focal", value: exif.focalLength },
      { icon: Gauge, label: "ISO", value: exif.iso },
    ];

    return (
      <div className="absolute inset-0 p-3 bg-black/80 backdrop-blur-sm flex flex-col justify-center">
        <div className="text-[10px] uppercase tracking-[0.2em] text-white/70 mb-3">
          Exif
        </div>
        <div className="space-y-1.5 text-white/90">
          {rows.map((row) => {
            const Icon = row.icon;
            return (
              <div
                key={row.label}
                className="flex items-center justify-between rounded-md bg-white/5 px-2 py-1"
              >
                <div className="flex items-center gap-1.5 text-white/70 min-w-0">
                  <Icon className="size-3.5 flex-shrink-0" />
                  <span className="text-[10px] tracking-wide uppercase truncate">
                    {row.label}
                  </span>
                </div>
                <span className="text-[11px] font-mono font-medium text-white/95 ml-2 truncate">
                  {row.value}
                </span>
              </div>
            );
          })}
        </div>
      </div>
    );
  };

  return (
    <section className={`min-h-[60vh] relative group ${className}`}>
      {/* Accent gradient background */}
      <div className="absolute inset-0 bg-gradient-to-b from-primary/10 to-transparent rounded-3xl pointer-events-none" />
      <div className="m-3 grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4 transform group-hover:scale-[0.98] transition-transform">
        {Array.from({ length: items }).map((_, i) => {
          const isHovered = hoverIndex === i;

          return (
            <div
              key={i}
              className="aspect-square bg-base-200 rounded-2xl shadow-lg hover:shadow-2xl transition-all hover:-translate-y-2 cursor-zoom-in relative overflow-hidden"
              onMouseEnter={() => setHoverIndex(i)}
              onMouseLeave={() => setHoverIndex(null)}
              onClick={() => onItemClick?.(i)}
              role="button"
              tabIndex={0}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") onItemClick?.(i);
              }}
            >
              {/* Custom item renderer */}
              {renderItem ? (
                <>
                  <div className="absolute inset-0">{renderItem(i, isHovered)}</div>
                  <div className="absolute inset-0 bg-gradient-to-t from-black/30 to-transparent pointer-events-none" />
                  {isHovered && renderExifOverlay(i)}
                  <div className="absolute bottom-2 left-2 text-white text-sm">
                    {titlePrefix} {i + 1}
                  </div>
                </>
              ) : (
                <>
                  {/* Placeholder for image area */}
                  <div className="absolute inset-0 bg-gradient-to-t from-black/30 to-transparent" />

                  {/* Hover overlay with EXIF details */}
                  {isHovered && renderExifOverlay(i)}

                  {/* Bottom-left label */}
                  <div className="absolute bottom-2 left-2 text-white text-sm">
                    {titlePrefix} {i + 1}
                  </div>
                </>
              )}
            </div>
          );
        })}
      </div>
    </section>
  );
};

export default GalleryGrid;
