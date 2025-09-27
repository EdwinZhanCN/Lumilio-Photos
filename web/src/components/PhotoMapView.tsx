import { useState, useEffect } from "react";
import { MapIcon, ListIcon } from "lucide-react";
import MapComponent, { PhotoLocation } from "./MapComponent";
import {
  assetsToPhotoLocations,
  formatGPSCoordinates,
} from "@/lib/utils/mapUtils";
import { useI18n } from "@/lib/i18n.tsx";

interface PhotoMapViewProps {
  assets: Asset[];
  onAssetClick?: (asset: Asset) => void;
  className?: string;
  showViewToggle?: boolean;
  defaultView?: "map" | "list";
  height?: string | number;
}

type ViewMode = "map" | "list";

export default function PhotoMapView({
  assets,
  onAssetClick,
  className = "",
  showViewToggle = true,
  defaultView = "map",
  height = "600px",
}: PhotoMapViewProps) {
  const { t } = useI18n();
  const [viewMode, setViewMode] = useState<ViewMode>(defaultView);
  const [photoLocations, setPhotoLocations] = useState<PhotoLocation[]>([]);
  const [, setSelectedLocation] = useState<PhotoLocation | null>(null);

  // Convert assets to photo locations
  useEffect(() => {
    const locations = assetsToPhotoLocations(assets);
    setPhotoLocations(locations);
  }, [assets]);

  // Handle photo location click
  const handleLocationClick = (location: PhotoLocation) => {
    setSelectedLocation(location);
    if (location.asset && onAssetClick) {
      onAssetClick(location.asset);
    }
  };

  // Group photos by location (approximately)
  const groupPhotosByLocation = (locations: PhotoLocation[]) => {
    const groups: { [key: string]: PhotoLocation[] } = {};
    const precision = 0.001; // Approximately 100m

    locations.forEach((location) => {
      const pos = location.position as [number, number];
      const lat = Math.round(pos[0] / precision) * precision;
      const lng = Math.round(pos[1] / precision) * precision;
      const key = `${lat},${lng}`;

      if (!groups[key]) {
        groups[key] = [];
      }
      groups[key].push(location);
    });

    return Object.values(groups);
  };

  const locationGroups = groupPhotosByLocation(photoLocations);

  const renderMapView = () => (
    <div className="relative h-full">
      <MapComponent
        photoLocations={photoLocations}
        height="100%"
        className="rounded-lg"
      />

      {/* Photo count indicator */}
      {photoLocations.length > 0 && (
        <div className="absolute top-4 left-4 bg-base-100 px-3 py-1 rounded-full shadow-md text-sm font-medium">
          {photoLocations.length}{" "}
          {photoLocations.length === 1
            ? t("assets.photos.title", { count: 1 })
            : t("assets.photos.title", { count: 2 })}
        </div>
      )}
    </div>
  );

  const renderListView = () => (
    <div className="h-full overflow-y-auto bg-base-100 rounded-lg">
      {locationGroups.length === 0 ? (
        <div className="flex flex-col items-center justify-center h-full text-base-content/60">
          <MapIcon className="w-16 h-16 mb-4" />
          <p className="text-lg font-medium mb-2">
            {t("map.noLocationData", {
              defaultValue: "No location data available",
            })}
          </p>
          <p className="text-sm text-center max-w-sm">
            Photos with GPS coordinates will appear here
          </p>
        </div>
      ) : (
        <div className="p-4 space-y-4">
          {locationGroups.map((group, groupIndex) => {
            const firstLocation = group[0];
            const pos = firstLocation.position as [number, number];
            const formattedCoords = formatGPSCoordinates(pos[0], pos[1]);

            return (
              <div key={groupIndex} className="card bg-base-200 shadow-sm">
                <div className="card-body p-4">
                  <div className="flex items-start justify-between mb-3">
                    <div>
                      <h3 className="font-semibold text-base">
                        {group.length === 1
                          ? firstLocation.title
                          : `${group.length} Photos`}
                      </h3>
                      <p className="text-sm text-base-content/70 font-mono">
                        {formattedCoords}
                      </p>
                    </div>
                    <div className="badge badge-primary badge-sm">
                      {group.length}
                    </div>
                  </div>

                  {/* Photo thumbnails grid */}
                  <div className="grid grid-cols-4 sm:grid-cols-6 md:grid-cols-8 gap-2">
                    {group.slice(0, 8).map((location) => (
                      <div
                        key={location.id}
                        className="aspect-square bg-base-300 rounded cursor-pointer hover:opacity-80 transition-opacity overflow-hidden"
                        onClick={() => handleLocationClick(location)}
                      >
                        {location.asset?.asset_id ? (
                          <img
                            src={`/api/assets/${location.asset.asset_id}/thumbnail?size=small`}
                            alt={location.title}
                            className="w-full h-full object-cover"
                            onError={(e) => {
                              const target = e.target as HTMLImageElement;
                              target.src =
                                "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='100' height='100' viewBox='0 0 100 100'%3E%3Crect width='100' height='100' fill='%23f0f0f0'/%3E%3Ctext x='50' y='50' text-anchor='middle' dy='.3em' font-size='30'%3E📷%3C/text%3E%3C/svg%3E";
                            }}
                          />
                        ) : (
                          <div className="w-full h-full flex items-center justify-center text-base-content/50">
                            📷
                          </div>
                        )}
                      </div>
                    ))}
                    {group.length > 8 && (
                      <div className="aspect-square bg-base-300 rounded flex items-center justify-center text-xs font-medium text-base-content/70">
                        +{group.length - 8}
                      </div>
                    )}
                  </div>

                  {/* Location description if available */}
                  {firstLocation.description && (
                    <p className="text-sm text-base-content/70 mt-2">
                      {firstLocation.description}
                    </p>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );

  return (
    <div className={`photo-map-view ${className}`} style={{ height }}>
      {/* View toggle */}
      {showViewToggle && (
        <div className="flex justify-end mb-4">
          <div className="join">
            <button
              className={`btn btn-sm join-item ${viewMode === "map" ? "btn-primary" : "btn-ghost"}`}
              onClick={() => setViewMode("map")}
            >
              <MapIcon className="w-4 h-4" />
              <span className="hidden sm:inline ml-1">Map</span>
            </button>
            <button
              className={`btn btn-sm join-item ${viewMode === "list" ? "btn-primary" : "btn-ghost"}`}
              onClick={() => setViewMode("list")}
            >
              <ListIcon className="w-4 h-4" />
              <span className="hidden sm:inline ml-1">List</span>
            </button>
          </div>
        </div>
      )}

      {/* Content */}
      <div className="h-full">
        {viewMode === "map" ? renderMapView() : renderListView()}
      </div>
    </div>
  );
}

// Export types for external use
export type { PhotoLocation, ViewMode };
