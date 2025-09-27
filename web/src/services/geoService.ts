/**
 * Geo Service for reverse geocoding
 * Supports China mirror service and OpenStreetMap based on region settings
 */

export interface GeoResponse {
  display_name: string;
}

class GeoService {
  /**
   * Get location name from GPS coordinates
   */
  async reverseGeocode(
    latitude: number,
    longitude: number,
    region: string = "other",
    language: string = "en"
  ): Promise<string> {
    try {
      const isChina = region === "china";
      const baseUrl = isChina
        ? "https://api.mirror-earth.com/nominatim/reverse"
        : "https://nominatim.openstreetmap.org/reverse";

      const acceptLanguage = language === "zh" ? "zh-CN" : "en-US,en";

      const params = new URLSearchParams({
        lat: latitude.toString(),
        lon: longitude.toString(),
        format: "jsonv2",
        addressdetails: "1",
        "accept-language": acceptLanguage,
      });

      // Add namedetails for OpenStreetMap only
      if (!isChina) {
        params.set("namedetails", "1");
      }

      const response = await fetch(`${baseUrl}?${params.toString()}`);

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
      }

      const data: GeoResponse = await response.json();
      return data.display_name || `${latitude.toFixed(4)}, ${longitude.toFixed(4)}`;
    } catch (error) {
      console.error("Reverse geocoding failed:", error);
      // Fallback to coordinates
      return `${latitude.toFixed(4)}, ${longitude.toFixed(4)}`;
    }
  }
}

export const geoService = new GeoService();
