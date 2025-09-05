import React from "react";
import { MapIcon } from "@heroicons/react/24/outline";
import MapComponent from "@/components/MapComponent";
import type { LatLngExpression } from "leaflet";

type PhotoLocationInput = {
  id: number;
  position: LatLngExpression;
  title: string;
  description: string;
};

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
   * Photo locations to plot on the map.
   * If not provided, MapComponent will use its internal sample data.
   */
  photoLocations?: PhotoLocationInput[];
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
  title = "时空轨迹",
  subtitle,
  icon,
  headerRight,
  photoLocations,
  className = "",
  aspect = "aspect-[16/9]",
  style,
}) => {
  return (
    <section className={`card bg-base-100 shadow-xl overflow-hidden ${className}`} style={style}>
      <div className="card-body p-0">
        <div className="flex items-center justify-between bg-base-200 p-4">
          <div className="flex items-center gap-3">
            <div className="text-primary">
              {icon ?? <MapIcon className="size-6" />}
            </div>
            <div>
              <h2 className="text-xl font-bold">{title}</h2>
              {subtitle && <p className="text-sm text-base-content/70">{subtitle}</p>}
            </div>
          </div>
          {headerRight && <div className="flex items-center gap-2">{headerRight}</div>}
        </div>

        <div className={aspect}>
          <MapComponent photoLocations={photoLocations} />
        </div>
      </div>
    </section>
  );
};

export default SpacetimeMapCard;
