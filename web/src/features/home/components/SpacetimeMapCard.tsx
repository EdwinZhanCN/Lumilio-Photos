import React from "react";
import { MapIcon } from "@heroicons/react/24/outline";
import PhotoMapView from "@/components/PhotoMapView";
import type { components } from "@/lib/http-commons/schema.d.ts";
import { useI18n } from "@/lib/i18n.tsx";

type Schemas = components["schemas"];
type MapPhotoPoint = Schemas["dto.AssetMapPointDTO"];

export interface SpacetimeMapCardProps {
  /**
   * Title displayed in the header.
   * Default: "时空轨迹"
   */
  title?: string;
  /**
   * Optional subtitle below the title.
   */
  subtitle?: string;
  /**
   * Override the header icon (defaults to MapIcon).
   */
  icon?: React.ReactNode;
  /**
   * Right-aligned content in the header (e.g., actions).
   */
  headerRight?: React.ReactNode;
  /**
   * Lightweight photo map points with GPS coordinates.
   */
  points?: MapPhotoPoint[];
  /**
   * Navigate to the corresponding asset when a single point is clicked.
   */
  onPointClick?: (assetId: string) => void;
  /**
   * Outer card className.
   */
  className?: string;
  /**
   * Aspect ratio class for the map container.
   * Default: "aspect-[16/9]"
   */
  aspect?: string;
  /**
   * Optional inline styles.
   */
  style?: React.CSSProperties;
}

/**
 * SpacetimeMapCard
 * Extracted from Home page to a reusable component.
 * Renders a card with a header and a Leaflet map for displaying photo locations.
 */
const SpacetimeMapCard: React.FC<SpacetimeMapCardProps> = ({
  title,
  subtitle,
  icon,
  headerRight,
  points = [],
  onPointClick,
  className = "",
  aspect = "aspect-[16/9]",
  style,
}) => {
  const { t } = useI18n();
  const resolvedTitle = title ?? t("home.map.title");

  return (
    <section
      className={`card bg-base-100 shadow-xl overflow-hidden ${className}`}
      style={style}
    >
      <div>
        <div className="flex items-center justify-between bg-base-200 p-4">
          <div className="flex items-center gap-3">
            <div className="text-primary">
              {icon ?? <MapIcon className="size-6" />}
            </div>
            <div>
              <h2 className="text-xl font-bold">{resolvedTitle}</h2>
              {subtitle && (
                <p className="text-sm text-base-content/70">{subtitle}</p>
              )}
            </div>
          </div>
          {headerRight && (
            <div className="flex items-center gap-2">{headerRight}</div>
          )}
        </div>

        <div className={aspect}>
          <PhotoMapView
            points={points}
            onPointClick={onPointClick}
            showViewToggle={false}
            height="100%"
            mapRounded={false}
          />
        </div>
      </div>
    </section>
  );
};

export default SpacetimeMapCard;
