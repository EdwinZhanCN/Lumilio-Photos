import { useParams, useLocation } from "react-router-dom";
import { useMemo } from "react";
import { MapPin } from "lucide-react";
import { AssetBrowser, AssetBrowserScope } from "@/features/assets";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import { useBreadcrumbs } from "@/components/breadcrumbs";
import { useI18n } from "@/lib/i18n.tsx";
import type { CityTripGroup } from "./useCityTrips";
import { useCityTrips } from "./useCityTrips";
import { useBrowseScope } from "@/features/repositories";

const TripDetails = () => {
  const { t } = useI18n();
  const { tripId } = useParams<{
    tripId: string;
    assetId: string;
  }>();
  const location = useLocation();
  const { scopedRepositoryId } = useBrowseScope();
  const { trips } = useCityTrips({ repositoryId: scopedRepositoryId });

  // Try to get trip from route state first, fall back to lookup by id
  const routeTrip = (location.state as { trip?: CityTripGroup } | null)?.trip;
  const trip = useMemo(() => {
    if (routeTrip) return routeTrip;
    return trips.find((t) => t.id === tripId);
  }, [routeTrip, trips, tripId]);

  useBreadcrumbs([
    { label: t("sidebar.home", "Home"), to: "/" },
    { label: t("sidebar.collections", "Collections"), to: "/collections" },
    { label: t("collections.sections.places", "Places"), to: "/collections/map" },
    { label: trip?.displayTitle || t("collections.places.tripFallback", "Trip") },
  ]);

  if (!trip) {
    if (trips.length === 0) {
      return (
        <div className="flex h-full items-center justify-center">
          <div className="loading loading-spinner loading-lg"></div>
        </div>
      );
    }
    return (
      <div className="flex h-full items-center justify-center text-base-content/60">
        <p>{t("collections.places.tripNotFound", "Trip not found.")}</p>
      </div>
    );
  }

  return (
    <WorkerProvider>
      <AssetBrowserScope
        key={`trip:${trip.id}`}
        scopeId={`trip:${trip.id}`}
        basePath={`/collections/places/${trip.id}`}
      >
        <AssetBrowser
          title={trip.displayTitle}
          icon={<MapPin className="w-6 h-6 text-primary" />}
          constraint={{
            location: {
              north: trip.bbox.north,
              south: trip.bbox.south,
              east: trip.bbox.east,
              west: trip.bbox.west,
            },
            date: {
              from: trip.startTime.toISOString(),
              to: trip.endTime.toISOString(),
            },
          }}
          viewKey={`trip:${trip.id}`}
        />
      </AssetBrowserScope>
    </WorkerProvider>
  );
};

export default TripDetails;
