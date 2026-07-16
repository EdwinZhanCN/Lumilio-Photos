import { useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { ErrorBoundary } from "react-error-boundary";
import { MapIcon } from "lucide-react";
import ErrorFallback from "@/components/ui/ErrorFallback";
import PageHeader from "@/components/ui/PageHeader";
import {
  MapComponent,
  type MapViewport,
  type PhotoLocation,
  useLocationClusters,
  useMapPhotoAssets,
} from "@/features/assets/map";
import { BrowseScopeSelect, useBrowseScope } from "@/features/repositories";
import { useBreadcrumbs } from "@/components/breadcrumbs";
import { useI18n } from "@/lib/i18n.tsx";
import { assetUrls } from "@/lib/assets/assetUrls";

function MapViewContent() {
  const { t } = useI18n();
  const navigate = useNavigate();
  useBreadcrumbs([
    { label: t("sidebar.home", "Home"), to: "/" },
    { label: t("sidebar.collections", "Collections"), to: "/collections" },
    { label: t("collections.sections.places", "Places") },
  ]);
  const { scopedRepositoryId } = useBrowseScope();
  const [viewport, setViewport] = useState<MapViewport | null>(null);
  const handleViewportChange = useCallback((nextViewport: MapViewport) => {
    setViewport((current) => {
      if (
        current?.zoom === nextViewport.zoom &&
        current.bbox.every((value, index) => Math.abs(value - nextViewport.bbox[index]) < 1e-5)
      ) {
        return current;
      }
      return nextViewport;
    });
  }, []);

  const {
    points: mapPoints,
    loadedPhotos: mapLoadedPhotos,
    totalPhotos: mapTotalPhotos,
    isLoading: isMapLoading,
    isFetchingNextPage: isMapFetchingNextPage,
    hasNextPage: mapHasNextPage,
  } = useMapPhotoAssets({
    repositoryId: scopedRepositoryId,
    viewport,
    enabled: viewport !== null,
  });

  const { clusters, loadedClusters, totalClusters } = useLocationClusters({
    repositoryId: scopedRepositoryId,
  });
  const initialCenter = useMemo<[number, number] | undefined>(() => {
    const cluster = clusters[0];
    return typeof cluster?.centroid_latitude === "number" &&
      typeof cluster.centroid_longitude === "number"
      ? [cluster.centroid_latitude, cluster.centroid_longitude]
      : undefined;
  }, [clusters]);

  useEffect(() => {
    setViewport(null);
  }, [scopedRepositoryId]);

  const photoLocations = useMemo(
    () =>
      mapPoints
        .map((point, index): PhotoLocation | null => {
          if (
            !point.asset_id ||
            typeof point.gps_latitude !== "number" ||
            typeof point.gps_longitude !== "number"
          ) {
            return null;
          }
          const title = point.original_filename || `Photo ${index + 1}`;
          const subtitle = point.taken_time ?? point.upload_time;
          return {
            id: point.asset_id,
            position: [point.gps_latitude, point.gps_longitude],
            title,
            description: subtitle ? new Date(subtitle).toLocaleString() : undefined,
            thumbnailUrl: assetUrls.getThumbnailUrl(point.asset_id, "small"),
          };
        })
        .filter((loc): loc is PhotoLocation => loc !== null),
    [mapPoints],
  );

  const statusText = (() => {
    if (isMapLoading && mapLoadedPhotos === 0) return t("home.map.loading");
    if (mapPoints.length === 0) return t("home.map.empty");
    let base = t("home.map.loadedStatus", {
      pointsCount: mapPoints.length,
      loadedCount: mapLoadedPhotos,
    });
    if (mapTotalPhotos) {
      base += t("home.map.loadedStatusTotal", { totalCount: mapTotalPhotos });
    }
    if (isMapFetchingNextPage || mapHasNextPage) {
      base += t("home.map.loadedStatusMore");
    }
    return base;
  })();

  const placesText =
    loadedClusters > 0
      ? t("home.map.placesCount", { count: totalClusters ?? loadedClusters })
      : undefined;

  return (
    <div className="flex h-full flex-col">
      <PageHeader
        title={t("collections.places.mapViewTitle")}
        icon={<MapIcon className="h-6 w-6 text-primary" strokeWidth={1.5} />}
      >
        <BrowseScopeSelect />
      </PageHeader>

      <div className="flex-1 min-h-0 relative">
        <MapComponent
          key={`${scopedRepositoryId ?? "all"}:${initialCenter?.join(",") ?? "default"}`}
          photoLocations={photoLocations}
          onPointClick={(assetId) => navigate(`/assets/${assetId}`)}
          height="100%"
          rounded={false}
          center={initialCenter}
          onViewportChange={handleViewportChange}
        />

        {/* Status overlay */}
        <div className="absolute bottom-4 left-4 flex items-center gap-2">
          <div className="rounded-full bg-base-100/90 px-3 py-1.5 text-xs font-medium shadow-md backdrop-blur-sm">
            {statusText}
          </div>
          {placesText && (
            <div className="rounded-full bg-base-100/90 px-3 py-1.5 text-xs font-medium shadow-md backdrop-blur-sm">
              {placesText}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

export default function MapView() {
  const { t } = useI18n();

  return (
    <ErrorBoundary
      FallbackComponent={(props) => (
        <ErrorFallback
          code={500}
          title={t("assets.errorFallback.something_went_wrong")}
          {...props}
        />
      )}
    >
      <MapViewContent />
    </ErrorBoundary>
  );
}
