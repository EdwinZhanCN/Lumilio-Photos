import { useMemo } from "react";
import { useLocationClusters, useMapPhotoAssets } from "@/features/assets/map";
import { encodeGeohash } from "@/lib/geo/geohash";
export type CityTripGroup = {
  id: string;
  city: string;
  region?: string;
  country?: string;
  displayTitle: string;
  startTime: Date;
  endTime: Date;
  photoCount: number;
  coverAssetId: string;
  memberClusterGeohashes: string[];
  bbox: { north: number; south: number; east: number; west: number };
};

type ClusterCityInfo = {
  city?: string;
  region?: string;
  country?: string;
  geohash: string;
};

const TIME_GAP_MS = 3 * 24 * 60 * 60 * 1000; // 3 days
const MIN_TRIP_PHOTOS = 5;
const BBOX_PADDING = 0.01; // ~1km padding

type PointWithCity = {
  assetId: string;
  lat: number;
  lng: number;
  takenTime: Date;
  city: string;
  region?: string;
  country?: string;
  geohash: string;
};

function buildCityKey(city: string, region?: string, country?: string): string {
  return [city, region, country].filter(Boolean).join("|");
}

function buildDisplayTitle(city: string, region?: string, country?: string): string {
  if (region && country && region !== city) {
    return `${city}, ${country}`;
  }
  if (country) {
    return `${city}, ${country}`;
  }
  return city;
}

function formatTripId(city: string, startTime: Date): string {
  const datePart = startTime.toISOString().slice(0, 10);
  const slug = city
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/(^-|-$)/g, "");
  return `${slug}-${datePart}`;
}

function computeBBox(points: PointWithCity[]) {
  let north = -90,
    south = 90,
    east = -180,
    west = 180;
  for (const p of points) {
    if (p.lat > north) north = p.lat;
    if (p.lat < south) south = p.lat;
    if (p.lng > east) east = p.lng;
    if (p.lng < west) west = p.lng;
  }
  return {
    north: north + BBOX_PADDING,
    south: south - BBOX_PADDING,
    east: east + BBOX_PADDING,
    west: west - BBOX_PADDING,
  };
}

function segmentByTimeGap(points: PointWithCity[]): PointWithCity[][] {
  if (points.length === 0) return [];

  const sorted = [...points].sort((a, b) => a.takenTime.getTime() - b.takenTime.getTime());

  const segments: PointWithCity[][] = [[sorted[0]]];
  for (let i = 1; i < sorted.length; i++) {
    const gap = sorted[i].takenTime.getTime() - sorted[i - 1].takenTime.getTime();
    if (gap > TIME_GAP_MS) {
      segments.push([sorted[i]]);
    } else {
      segments[segments.length - 1].push(sorted[i]);
    }
  }

  return segments;
}

export type UseCityTripsOptions = {
  repositoryId?: string;
  minPhotos?: number;
};

export function useCityTrips(options: UseCityTripsOptions = {}) {
  const { repositoryId, minPhotos = MIN_TRIP_PHOTOS } = options;

  const {
    points: mapPoints,
    isLoading: isMapLoading,
    hasNextPage: mapHasNextPage,
  } = useMapPhotoAssets({ repositoryId, autoFetchAll: true });

  const {
    clusters,
    isLoading: isClustersLoading,
    hasNextPage: clustersHaveNextPage,
  } = useLocationClusters({ repositoryId, autoFetchAll: true });

  const isLoading = isMapLoading || isClustersLoading;
  const isIncomplete = mapHasNextPage || clustersHaveNextPage;

  const trips = useMemo(() => {
    if (clusters.length === 0 || mapPoints.length === 0) {
      return [];
    }

    // Build geohash -> city lookup from clusters
    const geohashToCityMap = new Map<string, ClusterCityInfo>();
    for (const cluster of clusters) {
      if (!cluster.geohash) continue;
      geohashToCityMap.set(cluster.geohash, {
        city: cluster.city ?? undefined,
        region: cluster.region ?? undefined,
        country: cluster.country ?? undefined,
        geohash: cluster.geohash,
      });
    }

    // Map each point to a city
    const pointsWithCity: PointWithCity[] = [];
    for (const point of mapPoints) {
      if (
        !point.asset_id ||
        typeof point.gps_latitude !== "number" ||
        typeof point.gps_longitude !== "number"
      ) {
        continue;
      }

      const timeStr = point.taken_time ?? point.upload_time;
      if (!timeStr) continue;

      const geohash = encodeGeohash(point.gps_latitude, point.gps_longitude, 7);
      if (!geohash) continue;

      const cityInfo = geohashToCityMap.get(geohash);
      if (!cityInfo?.city) continue;

      pointsWithCity.push({
        assetId: point.asset_id,
        lat: point.gps_latitude,
        lng: point.gps_longitude,
        takenTime: new Date(timeStr),
        city: cityInfo.city,
        region: cityInfo.region,
        country: cityInfo.country,
        geohash,
      });
    }

    // Group by city key
    const cityGroups = new Map<string, PointWithCity[]>();
    for (const point of pointsWithCity) {
      const key = buildCityKey(point.city, point.region, point.country);
      const group = cityGroups.get(key);
      if (group) {
        group.push(point);
      } else {
        cityGroups.set(key, [point]);
      }
    }

    // Segment each city group by time gaps, produce trips
    const result: CityTripGroup[] = [];
    for (const [, points] of cityGroups) {
      const segments = segmentByTimeGap(points);
      for (const segment of segments) {
        if (segment.length < minPhotos) continue;

        const first = segment[0];
        const last = segment[segment.length - 1];
        const geohashes = [...new Set(segment.map((p) => p.geohash))];

        result.push({
          id: formatTripId(first.city, first.takenTime),
          city: first.city,
          region: first.region,
          country: first.country,
          displayTitle: buildDisplayTitle(first.city, first.region, first.country),
          startTime: first.takenTime,
          endTime: last.takenTime,
          photoCount: segment.length,
          coverAssetId: first.assetId,
          memberClusterGeohashes: geohashes,
          bbox: computeBBox(segment),
        });
      }
    }

    result.sort((a, b) => b.photoCount - a.photoCount);
    return result;
  }, [clusters, mapPoints, minPhotos]);

  return {
    trips,
    isLoading,
    isIncomplete,
  };
}
