import { useParams, useLocation } from "react-router-dom";
import { useMemo } from "react";
import { MapPin } from "lucide-react";
import { AssetsProvider } from "@/features/assets/AssetsProvider";
import { AssetsGalleryPage } from "@/features/assets/components/page/AssetsGalleryPage";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import type { CityTripGroup } from "../hooks/useCityTrips";
import { useCityTrips } from "../hooks/useCityTrips";
import { useWorkingRepository } from "@/features/settings";

const TripDetails = () => {
  const { tripId } = useParams<{
    tripId: string;
    assetId: string;
  }>();
  const location = useLocation();
  const { scopedRepositoryId } = useWorkingRepository();
  const { trips } = useCityTrips({ repositoryId: scopedRepositoryId });

  // Try to get trip from route state first, fall back to lookup by id
  const routeTrip = (location.state as { trip?: CityTripGroup } | null)?.trip;
  const trip = useMemo(() => {
    if (routeTrip) return routeTrip;
    return trips.find((t) => t.id === tripId);
  }, [routeTrip, trips, tripId]);

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
        <p>Trip not found.</p>
      </div>
    );
  }

  return (
    <WorkerProvider>
      <AssetsProvider
        key={`trip:${trip.id}`}
        scopeId={`trip:${trip.id}`}
        persist={false}
        basePath={`/collections/places/${trip.id}`}
      >
        <AssetsGalleryPage
          title={trip.displayTitle}
          icon={<MapPin className="w-6 h-6 text-primary" />}
          baseFilter={{
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
      </AssetsProvider>
    </WorkerProvider>
  );
};

export default TripDetails;
