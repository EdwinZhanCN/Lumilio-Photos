import { MapContainer, Marker, Popup, TileLayer, useMap } from "react-leaflet";
import "leaflet/dist/leaflet.css";
import L, { DivIcon, LatLngExpression, LatLngTuple } from "leaflet";
import Supercluster, {
  type BBox,
  type ClusterFeature,
  type PointFeature,
} from "supercluster";
import { useSettingsContext } from "@/features/settings";
import { useI18n } from "@/lib/i18n.tsx";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { assetUrls } from "@/lib/assets/assetUrls";
import { convertCoordinatesForMap } from "@/lib/utils/mapUtils";
import { Asset } from "@/lib/assets/types";

// Fix Leaflet default icon issue
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-expect-error
delete L.Icon.Default.prototype._getIconUrl;
L.Icon.Default.mergeOptions({
  iconRetinaUrl:
    "https://unpkg.com/leaflet@1.7.1/dist/images/marker-icon-2x.png",
  iconUrl: "https://unpkg.com/leaflet@1.7.1/dist/images/marker-icon.png",
  shadowUrl: "https://unpkg.com/leaflet@1.7.1/dist/images/marker-shadow.png",
});

// Photo location data type
export type PhotoLocation = {
  id: string;
  position: LatLngExpression;
  title: string;
  description?: string;
  thumbnailUrl?: string;
  asset?: Asset;
};

// Map component props
interface MapComponentProps {
  photoLocations?: PhotoLocation[];
  center?: LatLngTuple;
  zoom?: number;
  height?: string | number;
  className?: string;
  showSinglePhoto?: boolean; // For single photo display (basic info view)
}

type ClusterPointProperties = {
  locationId: string;
};

type ClusterProperties = {
  cluster: true;
  cluster_id: number;
  point_count: number;
  point_count_abbreviated: number | string;
};

type ClusterPoint = PointFeature<ClusterPointProperties>;
type MapCluster = ClusterFeature<Record<string, never>>;
type ClusterResult = ClusterPoint | MapCluster;

type MapViewport = {
  bbox: BBox;
  zoom: number;
};

const isClusterResult = (feature: ClusterResult): feature is MapCluster => {
  const properties = feature.properties as Partial<ClusterProperties>;
  return properties.cluster === true && typeof properties.cluster_id === "number";
};

const isSameBBox = (left: BBox, right: BBox): boolean =>
  left.every((value, index) => Math.abs(value - right[index]) < 1e-6);

// Create custom photo marker icon
const createPhotoMarkerIcon = (
  thumbnailUrl?: string,
  size: number = 40,
): DivIcon => {
  return L.divIcon({
    html: `
      <div class="photo-marker" style="
        width: ${size}px;
        height: ${size}px;
        border-radius: 8px;
        border: 3px solid #ffffff;
        box-shadow: 0 2px 8px rgba(0,0,0,0.3);
        overflow: hidden;
        background: #f0f0f0;
        display: flex;
        align-items: center;
        justify-content: center;
        cursor: pointer;
        transition: transform 0.2s ease;
      ">
        ${
          thumbnailUrl
            ? `<img src="${thumbnailUrl}" alt="Photo" style="
              width: 100%;
              height: 100%;
              object-fit: cover;
            "/>`
            : `<div style="
              width: 100%;
              height: 100%;
              background: linear-gradient(145deg, #2563eb, #0ea5e9);
            "></div>`
        }
      </div>
    `,
    className: "photo-marker-container",
    iconSize: [size, size],
    iconAnchor: [size / 2, size],
    popupAnchor: [0, -size],
  });
};

const createClusterMarkerIcon = (count: number): DivIcon => {
  const size = count < 10 ? 36 : count < 100 ? 44 : 52;
  return L.divIcon({
    html: `
      <div class="photo-cluster-marker" style="
        width: ${size}px;
        height: ${size}px;
      ">
        <span>${count}</span>
      </div>
    `,
    className: "photo-cluster-marker-container",
    iconSize: [size, size],
    iconAnchor: [size / 2, size / 2],
  });
};

// Map tile layer configurations
const mapConfigs = {
  china: {
    url: "https://webrd0{s}.is.autonavi.com/appmaptile?lang=zh_cn&size=1&scale=1&style=8&x={x}&y={y}&z={z}",
    attribution: '&copy; <a href="https://ditu.amap.com/">高德地图</a>',
    subdomains: ["1", "2", "3", "4"],
    maxZoom: 18,
  },
  other: {
    url: "https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png",
    attribution:
      '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
    subdomains: ["a", "b", "c"],
    maxZoom: 19,
  },
};

interface MapViewportWatcherProps {
  onViewportChange: (viewport: MapViewport) => void;
}

function MapViewportWatcher({ onViewportChange }: MapViewportWatcherProps) {
  const map = useMap();

  const emitViewport = useCallback(() => {
    const bounds = map.getBounds();
    onViewportChange({
      bbox: [
        bounds.getWest(),
        bounds.getSouth(),
        bounds.getEast(),
        bounds.getNorth(),
      ],
      zoom: map.getZoom(),
    });
  }, [map, onViewportChange]);

  useEffect(() => {
    emitViewport();

    map.on("moveend", emitViewport);
    map.on("zoomend", emitViewport);
    map.on("resize", emitViewport);

    return () => {
      map.off("moveend", emitViewport);
      map.off("zoomend", emitViewport);
      map.off("resize", emitViewport);
    };
  }, [emitViewport, map]);

  return null;
}

interface MapAutoFitBoundsProps {
  bounds: L.LatLngBounds | null;
  enabled: boolean;
  fitKey: number;
}

function MapAutoFitBounds({ bounds, enabled, fitKey }: MapAutoFitBoundsProps) {
  const map = useMap();
  const hasFittedRef = useRef(false);

  useEffect(() => {
    hasFittedRef.current = false;
  }, [fitKey]);

  useEffect(() => {
    if (!enabled || !bounds || hasFittedRef.current) {
      return;
    }

    map.fitBounds(bounds, { padding: [20, 20] });
    hasFittedRef.current = true;
  }, [bounds, enabled, map]);

  return null;
}

interface ClusterMarkerProps {
  cluster: MapCluster;
  clusterIndex: Supercluster<ClusterPointProperties, Record<string, never>>;
  maxZoom: number;
}

function ClusterMarker({ cluster, clusterIndex, maxZoom }: ClusterMarkerProps) {
  const map = useMap();
  const [lng, lat] = cluster.geometry.coordinates;
  const count = cluster.properties.point_count;
  const clusterId = cluster.properties.cluster_id;

  return (
    <Marker
      position={[lat, lng]}
      icon={createClusterMarkerIcon(count)}
      eventHandlers={{
        click: () => {
          const expansionZoom = Math.min(
            clusterIndex.getClusterExpansionZoom(clusterId),
            maxZoom,
          );
          map.flyTo([lat, lng], expansionZoom, { duration: 0.35 });
        },
      }}
    />
  );
}

function MapComponent({
  photoLocations = [],
  center,
  zoom = 10,
  height = "400px",
  className = "",
  showSinglePhoto = false,
}: MapComponentProps) {
  const { state } = useSettingsContext();
  const { t } = useI18n();
  const [mapKey, setMapKey] = useState(0);
  const [viewport, setViewport] = useState<MapViewport | null>(null);

  // Determine which map provider to use based on region setting
  const region = state.ui.region || "other";
  const mapConfig = mapConfigs[region];
  const isChina = region === "china";

  // Convert photo locations coordinates for the appropriate map system
  const convertedPhotoLocations = useMemo(
    () =>
      photoLocations.map((location) => {
        const position = location.position as [number, number];
        const converted = convertCoordinatesForMap(
          position[1], // longitude
          position[0], // latitude
          isChina,
        );
        return {
          ...location,
          position: [converted.latitude, converted.longitude] as LatLngTuple,
        };
      }),
    [isChina, photoLocations],
  );

  // Default center based on region with coordinate conversion
  const getDefaultCenter = useCallback((): LatLngTuple => {
    if (center) {
      const converted = convertCoordinatesForMap(center[1], center[0], isChina);
      return [converted.latitude, converted.longitude];
    }
    return isChina ? [39.9042, 116.4074] : [51.505, -0.09]; // Beijing or London
  }, [center, isChina]);

  // Force map re-render when region changes
  useEffect(() => {
    setMapKey((prev) => prev + 1);
    setViewport(null);
  }, [region]);

  const initialCenter = useMemo((): LatLngTuple => {
    if (convertedPhotoLocations.length === 1) {
      return convertedPhotoLocations[0].position as LatLngTuple;
    }
    return getDefaultCenter();
  }, [convertedPhotoLocations, getDefaultCenter]);

  const initialZoom = showSinglePhoto ? 15 : zoom;

  const fitBounds = useMemo(() => {
    if (showSinglePhoto || convertedPhotoLocations.length <= 1) {
      return null;
    }

    return L.latLngBounds(
      convertedPhotoLocations.map(
        (location) => location.position as LatLngTuple,
      ),
    );
  }, [convertedPhotoLocations, showSinglePhoto]);

  const locationById = useMemo(() => {
    const lookup = new Map<string, PhotoLocation>();
    convertedPhotoLocations.forEach((location) => {
      lookup.set(location.id, location);
    });
    return lookup;
  }, [convertedPhotoLocations]);

  const points = useMemo<ClusterPoint[]>(
    () =>
      convertedPhotoLocations.map((location) => {
        const [lat, lng] = location.position as LatLngTuple;
        return {
          type: "Feature",
          geometry: {
            type: "Point",
            coordinates: [lng, lat],
          },
          properties: {
            locationId: location.id,
          },
        };
      }),
    [convertedPhotoLocations],
  );

  const clusterIndex = useMemo(() => {
    const index = new Supercluster<ClusterPointProperties, Record<string, never>>(
      {
        radius: 64,
        minPoints: 2,
        minZoom: 0,
        maxZoom: Math.max(0, mapConfig.maxZoom - 1),
      },
    );
    index.load(points);
    return index;
  }, [mapConfig.maxZoom, points]);

  const defaultBbox = useMemo<BBox>(() => {
    if (fitBounds) {
      return [
        fitBounds.getWest(),
        fitBounds.getSouth(),
        fitBounds.getEast(),
        fitBounds.getNorth(),
      ];
    }

    const [lat, lng] = initialCenter;
    return [lng - 0.2, lat - 0.2, lng + 0.2, lat + 0.2];
  }, [fitBounds, initialCenter]);

  const handleViewportChange = useCallback((nextViewport: MapViewport) => {
    setViewport((currentViewport) => {
      if (
        currentViewport &&
        currentViewport.zoom === nextViewport.zoom &&
        isSameBBox(currentViewport.bbox, nextViewport.bbox)
      ) {
        return currentViewport;
      }
      return nextViewport;
    });
  }, []);

  const visibleFeatures = useMemo(() => {
    if (showSinglePhoto || points.length === 0) {
      return [] as ClusterResult[];
    }

    const bbox = viewport?.bbox ?? defaultBbox;
    const zoomLevel = Math.floor(viewport?.zoom ?? initialZoom);
    return clusterIndex.getClusters(bbox, zoomLevel);
  }, [
    clusterIndex,
    defaultBbox,
    initialZoom,
    points.length,
    showSinglePhoto,
    viewport?.bbox,
    viewport?.zoom,
  ]);

  const renderPopup = (location: PhotoLocation) => {
    const thumbnailUrl = location.asset?.asset_id
      ? assetUrls.getThumbnailUrl(location.asset.asset_id, "small")
      : undefined;

    return (
      <Popup>
        <div className="photo-popup">
          {(location.thumbnailUrl || thumbnailUrl) && (
            <img
              src={location.thumbnailUrl || thumbnailUrl}
              alt={location.title}
              className="photo-popup-image"
              onError={(e) => {
                const target = e.target as HTMLImageElement;
                target.style.display = "none";
              }}
            />
          )}
          <h3>{location.title}</h3>
          {location.description && <p>{location.description}</p>}
          {location.asset && (
            <div className="text-xs text-gray-500 mt-2">
              {location.asset.original_filename}
              {location.asset.upload_time && (
                <div>
                  {t("common.uploaded", {
                    defaultValue: "Uploaded",
                  })}
                  : {new Date(location.asset.upload_time).toLocaleDateString()}
                </div>
              )}
            </div>
          )}
        </div>
      </Popup>
    );
  };

  return (
    <div className={`map-container ${className}`} style={{ height }}>
      <style>{`
        .photo-marker-container .photo-marker:hover {
          transform: scale(1.1);
        }
        .photo-cluster-marker {
          border-radius: 999px;
          border: 3px solid #ffffff;
          background: radial-gradient(circle at 30% 30%, #38bdf8, #2563eb);
          box-shadow: 0 6px 14px rgba(0, 0, 0, 0.26);
          display: flex;
          align-items: center;
          justify-content: center;
          transition: transform 0.2s ease;
          color: #ffffff;
          font-weight: 700;
          font-size: 13px;
          line-height: 1;
          cursor: pointer;
        }
        .photo-cluster-marker:hover {
          transform: scale(1.08);
        }
        .leaflet-container {
          border-radius: 8px;
        }
        .leaflet-popup-content {
          margin: 8px 12px;
          line-height: 1.4;
        }
        .leaflet-popup-content h3 {
          margin: 0 0 8px 0;
          font-size: 16px;
          font-weight: 600;
        }
        .leaflet-popup-content p {
          margin: 0;
          color: #666;
          font-size: 14px;
        }
        .photo-popup-image {
          width: 120px;
          height: 80px;
          object-fit: cover;
          border-radius: 4px;
          margin-bottom: 8px;
        }
      `}</style>

      <MapContainer
        key={mapKey}
        center={initialCenter}
        zoom={initialZoom}
        style={{ height: "100%", width: "100%" }}
        zoomControl={true}
        scrollWheelZoom={true}
      >
        <TileLayer
          attribution={mapConfig.attribution}
          url={mapConfig.url}
          subdomains={mapConfig.subdomains}
          maxZoom={mapConfig.maxZoom}
        />

        <MapViewportWatcher onViewportChange={handleViewportChange} />
        <MapAutoFitBounds
          bounds={fitBounds}
          enabled={!showSinglePhoto}
          fitKey={mapKey}
        />

        {showSinglePhoto
          ? convertedPhotoLocations.map((location) => {
              const thumbnailUrl = location.asset?.asset_id
                ? assetUrls.getThumbnailUrl(location.asset.asset_id, "small")
                : undefined;

              return (
                <Marker
                  key={location.id}
                  position={location.position}
                  icon={createPhotoMarkerIcon(
                    location.thumbnailUrl || thumbnailUrl,
                    50,
                  )}
                >
                  {renderPopup(location)}
                </Marker>
              );
            })
          : visibleFeatures.map((feature) => {
              if (isClusterResult(feature)) {
                return (
                  <ClusterMarker
                    key={`cluster-${feature.properties.cluster_id}`}
                    cluster={feature}
                    clusterIndex={clusterIndex}
                    maxZoom={mapConfig.maxZoom}
                  />
                );
              }

              const location = locationById.get(feature.properties.locationId);
              if (!location) {
                return null;
              }

              const thumbnailUrl = location.asset?.asset_id
                ? assetUrls.getThumbnailUrl(location.asset.asset_id, "small")
                : undefined;

              return (
                <Marker
                  key={location.id}
                  position={location.position}
                  icon={createPhotoMarkerIcon(
                    location.thumbnailUrl || thumbnailUrl,
                    40,
                  )}
                >
                  {renderPopup(location)}
                </Marker>
              );
            })}
      </MapContainer>
    </div>
  );
}

export default MapComponent;
