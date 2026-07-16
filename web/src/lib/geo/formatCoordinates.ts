/** Formats decimal GPS coordinates with cardinal directions for display. */
export const formatGPSCoordinates = (
  latitude: number,
  longitude: number,
  precision: number = 6,
): string => {
  const lat = latitude.toFixed(precision);
  const lng = longitude.toFixed(precision);
  const latDir = latitude >= 0 ? "N" : "S";
  const lngDir = longitude >= 0 ? "E" : "W";

  return `${Math.abs(parseFloat(lat))}°${latDir}, ${Math.abs(parseFloat(lng))}°${lngDir}`;
};
