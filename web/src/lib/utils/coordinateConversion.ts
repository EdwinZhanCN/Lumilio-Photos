/**
 * Coordinate Conversion Utility for Gaode Map (AMap)
 * Handles conversion between WGS-84 and GCJ-02 coordinate systems
 *
 * WGS-84: World Geodetic System 1984 (GPS coordinates)
 * GCJ-02: Chinese encrypted coordinate system used by Gaode Map
 */

export type Coordinate = {
  latitude: number;
  longitude: number;
};

export type CoordinateConversionResult = {
  original: Coordinate;
  converted: Coordinate;
  offsetDistance: number; // Distance in meters
  offsetDirection: number; // Direction in degrees (0-360)
};

/**
 * Gaode Map Coordinate Transform Algorithm
 * Based on official Gaode Map conversion algorithm
 */
export class CoordinateConverter {
  private static readonly PI = Math.PI;
  private static readonly A = 6378245.0; // Semi-major axis of WGS-84
  private static readonly EE = 0.0066934216229659; // Eccentricity squared

  /**
   * Check if coordinates are outside China
   * If outside China, no conversion is needed
   */
  private static isOutOfChina(longitude: number, latitude: number): boolean {
    return (
      longitude < 72.004 ||
      longitude > 137.8347 ||
      latitude < 0.8293 ||
      latitude > 55.8271
    );
  }

  /**
   * Transform latitude component
   */
  private static transformLatitude(
    longitude: number,
    latitude: number,
  ): number {
    let ret =
      -100.0 +
      2.0 * longitude +
      3.0 * latitude +
      0.2 * latitude * latitude +
      0.1 * longitude * latitude +
      0.2 * Math.sqrt(Math.abs(longitude));

    ret +=
      ((20.0 * Math.sin(6.0 * longitude * this.PI) +
        20.0 * Math.sin(2.0 * longitude * this.PI)) *
        2.0) /
      3.0;

    ret +=
      ((20.0 * Math.sin(latitude * this.PI) +
        40.0 * Math.sin((latitude / 3.0) * this.PI)) *
        2.0) /
      3.0;

    ret +=
      ((160.0 * Math.sin((latitude / 12.0) * this.PI) +
        320 * Math.sin((latitude * this.PI) / 30.0)) *
        2.0) /
      3.0;

    return ret;
  }

  /**
   * Transform longitude component
   */
  private static transformLongitude(
    longitude: number,
    latitude: number,
  ): number {
    let ret =
      300.0 +
      longitude +
      2.0 * latitude +
      0.1 * longitude * longitude +
      0.1 * longitude * latitude +
      0.1 * Math.sqrt(Math.abs(longitude));

    ret +=
      ((20.0 * Math.sin(6.0 * longitude * this.PI) +
        20.0 * Math.sin(2.0 * longitude * this.PI)) *
        2.0) /
      3.0;

    ret +=
      ((20.0 * Math.sin(longitude * this.PI) +
        40.0 * Math.sin((longitude / 3.0) * this.PI)) *
        2.0) /
      3.0;

    ret +=
      ((150.0 * Math.sin((longitude / 12.0) * this.PI) +
        300.0 * Math.sin((longitude / 30.0) * this.PI)) *
        2.0) /
      3.0;

    return ret;
  }

  /**
   * Convert WGS-84 coordinates to GCJ-02 (Gaode Map coordinates)
   * @param longitude WGS-84 longitude
   * @param latitude WGS-84 latitude
   * @returns Converted GCJ-02 coordinates
   */
  static wgs84ToGcj02(longitude: number, latitude: number): Coordinate {
    // If outside China, no conversion needed
    if (this.isOutOfChina(longitude, latitude)) {
      return { longitude, latitude };
    }

    let dlat = this.transformLatitude(longitude - 105.0, latitude - 35.0);
    let dlng = this.transformLongitude(longitude - 105.0, latitude - 35.0);

    const radlat = (latitude / 180.0) * this.PI;
    let magic = Math.sin(radlat);
    magic = 1 - this.EE * magic * magic;
    const sqrtmagic = Math.sqrt(magic);

    dlat =
      (dlat * 180.0) /
      (((this.A * (1 - this.EE)) / (magic * sqrtmagic)) * this.PI);
    dlng = (dlng * 180.0) / ((this.A / sqrtmagic) * Math.cos(radlat) * this.PI);

    const convertedLatitude = latitude + dlat;
    const convertedLongitude = longitude + dlng;

    return {
      longitude: convertedLongitude,
      latitude: convertedLatitude,
    };
  }

  /**
   * Convert GCJ-02 coordinates back to WGS-84
   * This is an approximation as the conversion is not perfectly reversible
   */
  static gcj02ToWgs84(longitude: number, latitude: number): Coordinate {
    if (this.isOutOfChina(longitude, latitude)) {
      return { longitude, latitude };
    }

    // Use iteration to approximate the reverse conversion
    const converted = this.wgs84ToGcj02(longitude, latitude);
    const deltaLng = converted.longitude - longitude;
    const deltaLat = converted.latitude - latitude;

    return {
      longitude: longitude - deltaLng,
      latitude: latitude - deltaLat,
    };
  }

  /**
   * Calculate distance between two coordinates in meters
   */
  static calculateDistance(coord1: Coordinate, coord2: Coordinate): number {
    const R = 6371000; // Earth's radius in meters
    const lat1Rad = (coord1.latitude * Math.PI) / 180;
    const lat2Rad = (coord2.latitude * Math.PI) / 180;
    const deltaLatRad = ((coord2.latitude - coord1.latitude) * Math.PI) / 180;
    const deltaLngRad = ((coord2.longitude - coord1.longitude) * Math.PI) / 180;

    const a =
      Math.sin(deltaLatRad / 2) * Math.sin(deltaLatRad / 2) +
      Math.cos(lat1Rad) *
        Math.cos(lat2Rad) *
        Math.sin(deltaLngRad / 2) *
        Math.sin(deltaLngRad / 2);

    const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));

    return R * c;
  }

  /**
   * Calculate bearing/direction between two coordinates in degrees
   */
  static calculateBearing(from: Coordinate, to: Coordinate): number {
    const lat1Rad = (from.latitude * Math.PI) / 180;
    const lat2Rad = (to.latitude * Math.PI) / 180;
    const deltaLngRad = ((to.longitude - from.longitude) * Math.PI) / 180;

    const y = Math.sin(deltaLngRad) * Math.cos(lat2Rad);
    const x =
      Math.cos(lat1Rad) * Math.sin(lat2Rad) -
      Math.sin(lat1Rad) * Math.cos(lat2Rad) * Math.cos(deltaLngRad);

    const bearing = (Math.atan2(y, x) * 180) / Math.PI;
    return (bearing + 360) % 360;
  }

  /**
   * Perform coordinate conversion with detailed results
   */
  static convertWithDetails(
    longitude: number,
    latitude: number,
    fromSystem: "WGS84" | "GCJ02" = "WGS84",
  ): CoordinateConversionResult {
    const original: Coordinate = { longitude, latitude };

    let converted: Coordinate;
    if (fromSystem === "WGS84") {
      converted = this.wgs84ToGcj02(longitude, latitude);
    } else {
      converted = this.gcj02ToWgs84(longitude, latitude);
    }

    const offsetDistance = this.calculateDistance(original, converted);
    const offsetDirection = this.calculateBearing(original, converted);

    return {
      original,
      converted,
      offsetDistance,
      offsetDirection,
    };
  }
}

/**
 * Helper functions for easier usage
 */

/**
 * Convert WGS-84 coordinates to GCJ-02 for Gaode Map display
 * @param longitude WGS-84 longitude (GPS longitude)
 * @param latitude WGS-84 latitude (GPS latitude)
 * @returns GCJ-02 coordinates for Gaode Map
 */
export function convertToGaodeCoordinates(
  longitude: number,
  latitude: number,
): Coordinate {
  return CoordinateConverter.wgs84ToGcj02(longitude, latitude);
}

/**
 * Convert GCJ-02 coordinates back to WGS-84
 * @param longitude GCJ-02 longitude (Gaode Map longitude)
 * @param latitude GCJ-02 latitude (Gaode Map latitude)
 * @returns Approximate WGS-84 coordinates
 */
export function convertFromGaodeCoordinates(
  longitude: number,
  latitude: number,
): Coordinate {
  return CoordinateConverter.gcj02ToWgs84(longitude, latitude);
}

/**
 * Check if coordinates need conversion (if in China)
 */
export function needsCoordinateConversion(
  longitude: number,
  latitude: number,
): boolean {
  return !CoordinateConverter["isOutOfChina"](longitude, latitude);
}

/**
 * Auto-convert coordinates based on region setting
 * @param longitude Original longitude
 * @param latitude Original latitude
 * @param useGaodeMap Whether to use Gaode Map (China region)
 * @returns Converted coordinates if needed, original otherwise
 */
export function autoConvertCoordinates(
  longitude: number,
  latitude: number,
  useGaodeMap: boolean,
): Coordinate {
  if (useGaodeMap && needsCoordinateConversion(longitude, latitude)) {
    return convertToGaodeCoordinates(longitude, latitude);
  }
  return { longitude, latitude };
}

/**
 * Batch convert multiple coordinates
 */
export function batchConvertCoordinates(
  coordinates: Coordinate[],
  useGaodeMap: boolean,
): Coordinate[] {
  return coordinates.map((coord) =>
    autoConvertCoordinates(coord.longitude, coord.latitude, useGaodeMap),
  );
}

/**
 * Convert coordinate array format [lat, lng] to Coordinate object
 */
export function arrayToCoordinate(coords: [number, number]): Coordinate {
  return { latitude: coords[0], longitude: coords[1] };
}

/**
 * Convert Coordinate object to array format [lat, lng]
 */
export function coordinateToArray(coord: Coordinate): [number, number] {
  return [coord.latitude, coord.longitude];
}

/**
 * Format coordinate conversion result for display
 */
export function formatConversionResult(
  result: CoordinateConversionResult,
): string {
  return `
Original (WGS-84): ${result.original.latitude.toFixed(6)}, ${result.original.longitude.toFixed(6)}
Converted (GCJ-02): ${result.converted.latitude.toFixed(6)}, ${result.converted.longitude.toFixed(6)}
Offset: ${result.offsetDistance.toFixed(2)}m at ${result.offsetDirection.toFixed(1)}°
  `.trim();
}

export default CoordinateConverter;
