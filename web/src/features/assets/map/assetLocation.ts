import { autoConvertCoordinates } from "@/lib/geo/coordinateConversion";
import type { Asset } from "@/lib/http-commons";
import type { PhotoLocation } from "./types";

/**
 * Convert an Asset with GPS coordinates to PhotoLocation format
 * Automatically converts coordinates for Gaode Map if in China region
 */
export const assetToPhotoLocation = (
  asset: Asset,
  useGaodeMap: boolean = false,
): PhotoLocation | null => {
  if (asset.type !== "PHOTO") {
    return null;
  }

  const latitude = asset.gps_latitude;
  const longitude = asset.gps_longitude;

  if (typeof latitude !== "number" || typeof longitude !== "number") {
    return null;
  }

  // Convert coordinates if using Gaode Map and location is in China
  const convertedCoords = autoConvertCoordinates(longitude, latitude, useGaodeMap);

  return {
    id: asset.asset_id || `asset-${Date.now()}`,
    position: [convertedCoords.latitude, convertedCoords.longitude],
    title: asset.original_filename || "Photo",
    description: asset.specific_metadata?.description,
    asset: asset,
  };
};
