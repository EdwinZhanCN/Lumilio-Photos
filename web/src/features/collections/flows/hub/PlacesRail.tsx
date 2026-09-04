import { MapIcon, MapPin } from "lucide-react";
import { useMemo } from "react";
import RailCard from "@/components/collection/RailCard";
import type { LocationCluster } from "@/features/assets/map";
import { useI18n } from "@/lib/i18n.tsx";
import Rail from "../../components/Rail";

export type PlaceSummary = {
  id: string;
  label: string;
  photoCount: number;
  latitude?: number;
  longitude?: number;
};

type PlaceAccumulator = PlaceSummary & {
  coordinateWeight: number;
  weightedLatitude: number;
  weightedLongitude: number;
};

type PlacesRailProps = {
  clusters: LocationCluster[];
  loading?: boolean;
  onMapClick?: () => void;
  onPlaceClick?: (place: PlaceSummary) => void;
};

function placeIdentity(cluster: LocationCluster, index: number) {
  const parts = [cluster.city, cluster.region, cluster.country]
    .map((part) => part?.trim())
    .filter((part): part is string => Boolean(part));
  if (parts.length > 0) {
    return {
      id: `locality:${parts.join("\u0000")}`,
      label: parts.join(", "),
    };
  }

  const label = cluster.label?.trim();
  if (label) {
    return { id: `label:${label}`, label };
  }

  return {
    id: `cluster:${cluster.cluster_id ?? cluster.geohash ?? index}`,
    label: cluster.geohash?.trim() ?? "",
  };
}

export function summarizePlaces(clusters: LocationCluster[], unknownLabel: string): PlaceSummary[] {
  const places = new Map<string, PlaceAccumulator>();

  clusters.forEach((cluster, index) => {
    const identity = placeIdentity(cluster, index);
    const photoCount = cluster.photo_count ?? 0;
    const coordinate =
      typeof cluster.centroid_latitude === "number" &&
      typeof cluster.centroid_longitude === "number"
        ? ([cluster.centroid_latitude, cluster.centroid_longitude] as const)
        : undefined;
    const coordinateWeight = coordinate ? Math.max(photoCount, 1) : 0;
    const existing = places.get(identity.id);

    if (existing) {
      existing.photoCount += photoCount;
      existing.coordinateWeight += coordinateWeight;
      if (coordinate) {
        existing.weightedLatitude += coordinate[0] * coordinateWeight;
        existing.weightedLongitude += coordinate[1] * coordinateWeight;
      }
      return;
    }

    places.set(identity.id, {
      ...identity,
      label: identity.label || unknownLabel,
      photoCount,
      coordinateWeight,
      weightedLatitude: coordinate ? coordinate[0] * coordinateWeight : 0,
      weightedLongitude: coordinate ? coordinate[1] * coordinateWeight : 0,
    });
  });

  return [...places.values()]
    .map(({ coordinateWeight, weightedLatitude, weightedLongitude, ...place }) => ({
      ...place,
      latitude: coordinateWeight > 0 ? weightedLatitude / coordinateWeight : undefined,
      longitude: coordinateWeight > 0 ? weightedLongitude / coordinateWeight : undefined,
    }))
    .sort(
      (left, right) => right.photoCount - left.photoCount || left.label.localeCompare(right.label),
    );
}

export default function PlacesRail({
  clusters,
  loading = false,
  onMapClick,
  onPlaceClick,
}: PlacesRailProps) {
  const { t } = useI18n();
  const places = useMemo(
    () => summarizePlaces(clusters, t("collections.places.unknown", "Unknown place")),
    [clusters, t],
  );

  return (
    <Rail loading={loading} skeletonCount={5}>
      <RailCard
        media={{ kind: "icon", icon: MapIcon, tone: "primary" }}
        title={t("collections.places.mapView")}
        onClick={onMapClick}
        className="w-48"
      />

      {places.slice(0, 12).map((place) => (
        <RailCard
          key={place.id}
          media={{ kind: "icon", icon: MapPin, tone: "accent" }}
          title={place.label}
          subtitle={t("collections.itemsCount", { count: place.photoCount })}
          onClick={() => onPlaceClick?.(place)}
          className="w-48"
        />
      ))}

      {places.length === 0 && (
        <div className="flex aspect-square w-48 shrink-0 items-center justify-center rounded-[1.75rem] border border-dashed border-base-300 px-4 text-center text-sm text-base-content/60">
          {t("collections.places.empty", "No places with location data yet.")}
        </div>
      )}
    </Rail>
  );
}
