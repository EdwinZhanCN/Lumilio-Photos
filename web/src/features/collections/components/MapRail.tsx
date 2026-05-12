import { MapIcon } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { assetUrls } from "@/lib/assets/assetUrls";
import type { CityTripGroup } from "../hooks/useCityTrips";

type MapRailProps = {
  trips: CityTripGroup[];
  loading?: boolean;
  onMapClick?: () => void;
  onTripClick?: (trip: CityTripGroup) => void;
};

function formatTripStart(date: Date, locale?: string): string {
  return date.toLocaleDateString(locale || "en", {
    month: "short",
    year: "numeric",
  });
}

const MapRailSkeleton = () => (
  <div className="flex gap-4 overflow-x-auto pb-2">
    {Array.from({ length: 5 }).map((_, index) => (
      <div
        key={index}
        className="aspect-square w-48 shrink-0 animate-pulse rounded-[1.75rem] bg-base-300/70"
      />
    ))}
  </div>
);

export default function MapRail({
  trips,
  loading = false,
  onMapClick,
  onTripClick,
}: MapRailProps) {
  const { t, i18n } = useI18n();
  const locale = i18n.resolvedLanguage || i18n.language;

  if (loading) {
    return <MapRailSkeleton />;
  }

  return (
    <div className="flex gap-4 overflow-x-auto pb-2">
      {/* Fixed Map View entry */}
      <button
        type="button"
        onClick={onMapClick}
        className="group w-48 shrink-0 cursor-pointer"
      >
        <div className="relative aspect-square overflow-hidden rounded-[1.75rem] bg-gradient-to-br from-primary/20 via-primary/10 to-base-200 transition duration-300">
          <div className="flex h-full flex-col items-center justify-center gap-2">
            <MapIcon className="size-10 text-primary" strokeWidth={1.5} />
            <span className="text-sm font-semibold text-primary">
              {t("collections.places.mapView")}
            </span>
          </div>
          <div className="absolute inset-x-0 bottom-0 p-3">
            <p className="text-xs text-base-content/50">
              {t("collections.places.exploreAll")}
            </p>
          </div>
        </div>
      </button>

      {/* Trip cards */}
      {trips.map((trip) => (
        <button
          key={trip.id}
          type="button"
          onClick={() => onTripClick?.(trip)}
          className="group w-48 shrink-0 cursor-pointer"
        >
          <div className="relative aspect-square overflow-hidden rounded-[1.75rem] bg-base-200 transition duration-300">
            <img
              src={assetUrls.getThumbnailUrl(trip.coverAssetId, "medium")}
              alt={trip.displayTitle}
              className="h-full w-full object-cover transition duration-500 group-hover:scale-[1.03]"
              loading="lazy"
            />
            <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/70 via-black/40 to-transparent p-3 pt-10">
              <p className="text-sm font-bold text-white drop-shadow-sm">
                {trip.displayTitle}
              </p>
              <p className="mt-0.5 text-xs text-white/70">
                {formatTripStart(trip.startTime, locale)}
                {" · "}
                {t("collections.itemsCount", { count: trip.photoCount })}
              </p>
            </div>
          </div>
        </button>
      ))}

      {trips.length === 0 && (
        <div className="flex aspect-square w-48 shrink-0 items-center justify-center rounded-[1.75rem] border border-dashed border-base-300 text-sm text-base-content/60">
          {t("collections.places.empty")}
        </div>
      )}
    </div>
  );
}
