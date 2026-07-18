import { MapIcon } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { assetUrls } from "@/lib/assets/assetUrls";
import Rail from "../../components/Rail";
import RailCard from "../../components/RailCard";
import type { CityTripGroup } from "../places/useCityTrips";

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

export default function MapRail({ trips, loading = false, onMapClick, onTripClick }: MapRailProps) {
  const { t, i18n } = useI18n();
  const locale = i18n.resolvedLanguage || i18n.language;

  return (
    <Rail loading={loading} skeletonCount={5}>
      {/* Fixed "open the map" entry. */}
      <RailCard
        media={{ kind: "icon", icon: MapIcon, tone: "primary" }}
        title={t("collections.places.mapView")}
        onClick={onMapClick}
        className="w-48"
      />

      {trips.map((trip) => (
        <RailCard
          key={trip.id}
          media={{
            kind: "photo",
            src: assetUrls.getThumbnailUrl(trip.coverAssetId, "medium"),
          }}
          title={trip.displayTitle}
          subtitle={`${formatTripStart(trip.startTime, locale)} · ${t("collections.itemsCount", {
            count: trip.photoCount,
          })}`}
          onClick={() => onTripClick?.(trip)}
          className="w-48"
        />
      ))}

      {trips.length === 0 && (
        <div className="flex aspect-square w-48 shrink-0 items-center justify-center rounded-[1.75rem] border border-dashed border-base-300 text-sm text-base-content/60">
          {t("collections.places.empty")}
        </div>
      )}
    </Rail>
  );
}
