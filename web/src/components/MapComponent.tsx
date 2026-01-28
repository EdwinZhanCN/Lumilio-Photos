import { MapContainer, TileLayer, Marker, Popup } from "react-leaflet";
import "leaflet/dist/leaflet.css";
import L, { LatLngExpression, LatLngTuple, DivIcon } from "leaflet";
import { useSettingsContext } from "@/features/settings";
import { useI18n } from "@/lib/i18n.tsx";
import { useEffect, useState } from "react";
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
              background: linear-gradient(45deg, #3498db, #2980b9);
              display: flex;
              align-items: center;
              justify-content: center;
              color: white;
              font-size: ${size * 0.4}px;
            ">📷</div>`
        }
      </div>
    `,
    className: "photo-marker-container",
    iconSize: [size, size],
    iconAnchor: [size / 2, size],
    popupAnchor: [0, -size],
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

  // Determine which map provider to use based on region setting
  const region = state.ui.region || "other";
  const mapConfig = mapConfigs[region];
  const isChina = region === "china";

  // Convert photo locations coordinates for the appropriate map system
  const convertedPhotoLocations = photoLocations.map((location) => {
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
  });

  // Default center based on region with coordinate conversion
  const getDefaultCenter = (): LatLngTuple => {
    if (center) {
      const converted = convertCoordinatesForMap(center[1], center[0], isChina);
      return [converted.latitude, converted.longitude];
    }
    return isChina ? [39.9042, 116.4074] : [51.505, -0.09]; // Beijing or London
  };

  // Force map re-render when region changes
  useEffect(() => {
    setMapKey((prev) => prev + 1);
  }, [region]);

  // Calculate map bounds if multiple photos
  const getMapBounds = () => {
    if (convertedPhotoLocations.length === 0) return undefined;
    if (convertedPhotoLocations.length === 1) return undefined;

    const bounds = L.latLngBounds(
      convertedPhotoLocations.map(
        (location) => location.position as LatLngTuple,
      ),
    );
    return bounds;
  };

  // Get appropriate zoom level
  const getZoomLevel = () => {
    if (showSinglePhoto) return 15;
    if (convertedPhotoLocations.length <= 1) return zoom;
    return undefined; // Let fitBounds determine zoom
  };

  // Get map center
  const getMapCenter = (): LatLngTuple => {
    if (convertedPhotoLocations.length === 1) {
      return convertedPhotoLocations[0].position as LatLngTuple;
    }
    return getDefaultCenter();
  };

  return (
    <div className={`map-container ${className}`} style={{ height }}>
      <style>{`
        .photo-marker-container .photo-marker:hover {
          transform: scale(1.1);
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
        center={getMapCenter()}
        zoom={getZoomLevel()}
        bounds={getMapBounds()}
        boundsOptions={{ padding: [20, 20] }}
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

        {convertedPhotoLocations.map((location) => {
          const thumbnailUrl = location.asset?.asset_id
            ? assetUrls.getThumbnailUrl(location.asset.asset_id, "small")
            : undefined;
          const markerSize = showSinglePhoto ? 50 : 40;

          return (
            <Marker
              key={location.id}
              position={location.position}
              icon={createPhotoMarkerIcon(
                location.thumbnailUrl || thumbnailUrl,
                markerSize,
              )}
            >
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
                          :{" "}
                          {new Date(
                            location.asset.upload_time,
                          ).toLocaleDateString()}
                        </div>
                      )}
                    </div>
                  )}
                </div>
              </Popup>
            </Marker>
          );
        })}
      </MapContainer>
    </div>
  );
}

export default MapComponent;
