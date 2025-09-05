import React from "react";
import { SwatchIcon } from "@heroicons/react/24/outline";

export type FilterItem = {
  id?: string;
  name: string;
  imageUrl?: string;
  subtitle?: string;
  /**
   * Custom content rendered inside the tile (beneath overlays).
   * If provided, it replaces the default image/placeholder.
   */
  renderContent?: (filter: FilterItem, index: number) => React.ReactNode;
};

export type FiltersCarouselProps = {
  /**
   * Filters to display. If omitted, a default list is shown.
   */
  filters?: FilterItem[];
  /**
   * Called when a filter is selected (click on the card).
   */
  onSelect?: (filter: FilterItem, index: number) => void;
  /**
   * Called when user clicks the preview button overlay.
   */
  onPreview?: (filter: FilterItem, index: number) => void;
  /**
   * Whether to show the preview button overlay.
   * Default: true
   */
  showPreviewButton?: boolean;
  /**
   * Text shown on the preview button.
   * Default: "预览效果"
   */
  previewLabel?: string;
  /**
   * Class applied to the outer carousel section.
   */
  className?: string;
  /**
   * Class applied to the carousel item container.
   */
  itemClassName?: string;
  /**
   * Class applied to the inner card tile.
   */
  tileClassName?: string;
};

const DEFAULT_FILTERS: FilterItem[] = [
  { name: "胶片模拟" },
  { name: "赛博朋克" },
  { name: "复古褪色" },
  { name: "黑金夜景" },
];

const FiltersCarousel: React.FC<FiltersCarouselProps> = ({
  filters = DEFAULT_FILTERS,
  onSelect,
  onPreview,
  showPreviewButton = true,
  previewLabel = "预览效果",
  className = "",
  itemClassName = "",
  tileClassName = "",
}) => {
  return (
    <section
      className={`carousel carousel-center gap-4 p-4 bg-base-200 rounded-3xl ${className}`}
      aria-label="创意滤镜预览墙"
    >
      {filters.map((filter, i) => {
        const key = filter.id ?? filter.name ?? `filter-${i}`;

        const handleSelect = () => onSelect?.(filter, i);
        const handlePreview = (e?: React.MouseEvent) => {
          e?.stopPropagation();
          onPreview?.(filter, i);
        };

        const handleKeyDown: React.KeyboardEventHandler<HTMLDivElement> = (
          e,
        ) => {
          if (e.key === "Enter") {
            e.preventDefault();
            handleSelect();
          } else if (e.key === " ") {
            // Space triggers preview if available, otherwise select.
            e.preventDefault();
            if (showPreviewButton && onPreview) {
              handlePreview();
            } else {
              handleSelect();
            }
          }
        };

        return (
          <div key={key} className={`carousel-item ${itemClassName}`}>
            <div
              className={`group relative w-48 h-64 bg-base-100 rounded-xl shadow-lg hover:shadow-2xl transition-all cursor-pointer focus:outline-none focus:ring-2 focus:ring-primary ${tileClassName}`}
              role="button"
              tabIndex={0}
              aria-label={`滤镜：${filter.name}`}
              onClick={handleSelect}
              onKeyDown={handleKeyDown}
            >
              {/* Base visual (image or custom content or placeholder) */}
              {filter.renderContent ? (
                <div className="absolute inset-0 overflow-hidden rounded-xl">
                  {filter.renderContent(filter, i)}
                </div>
              ) : filter.imageUrl ? (
                <img
                  src={filter.imageUrl}
                  alt={`${filter.name} 预览`}
                  className="absolute inset-0 h-full w-full object-cover rounded-xl"
                  loading="lazy"
                />
              ) : (
                <div className="absolute inset-0 bg-base-200 rounded-xl" />
              )}

              {/* Top gradient overlay */}
              <div className="absolute inset-0 bg-gradient-to-t from-black/40 to-transparent rounded-xl pointer-events-none" />

              {/* Bottom-left label */}
              <div className="absolute bottom-3 left-3 text-white text-sm">
                <div className="font-semibold leading-5">{filter.name}</div>
                {filter.subtitle && (
                  <div className="text-xs text-white/80">{filter.subtitle}</div>
                )}
              </div>

              {/* Top-right icon */}
              <div className="absolute top-3 right-3">
                <SwatchIcon className="size-5 text-primary/80" />
              </div>

              {/* Hover preview button overlay */}
              {showPreviewButton && (
                <button
                  type="button"
                  onClick={handlePreview}
                  className="absolute inset-0 opacity-0 group-hover:opacity-100 transition-opacity bg-black/50 backdrop-blur-sm flex items-center justify-center rounded-xl"
                  aria-label={`${filter.name} ${previewLabel}`}
                >
                  <span className="btn btn-xs btn-primary">{previewLabel}</span>
                </button>
              )}
            </div>
          </div>
        );
      })}
    </section>
  );
};

export default FiltersCarousel;
