const BASE32 = "0123456789bcdefghjkmnpqrstuvwxyz";

/** Encodes a latitude/longitude pair as a base32 geohash. */
export function encodeGeohash(latitude: number, longitude: number, precision = 7): string | null {
  if (
    !Number.isFinite(latitude) ||
    !Number.isFinite(longitude) ||
    latitude < -90 ||
    latitude > 90 ||
    longitude < -180 ||
    longitude > 180 ||
    precision <= 0
  ) {
    return null;
  }

  let latRange: [number, number] = [-90, 90];
  let lonRange: [number, number] = [-180, 180];
  let hash = "";
  let bit = 0;
  let value = 0;
  let evenBit = true;

  while (hash.length < precision) {
    if (evenBit) {
      const mid = (lonRange[0] + lonRange[1]) / 2;
      if (longitude >= mid) {
        value = value * 2 + 1;
        lonRange[0] = mid;
      } else {
        value *= 2;
        lonRange[1] = mid;
      }
    } else {
      const mid = (latRange[0] + latRange[1]) / 2;
      if (latitude >= mid) {
        value = value * 2 + 1;
        latRange[0] = mid;
      } else {
        value *= 2;
        latRange[1] = mid;
      }
    }

    evenBit = !evenBit;
    bit += 1;

    if (bit === 5) {
      hash += BASE32[value];
      bit = 0;
      value = 0;
    }
  }

  return hash;
}
