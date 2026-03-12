import { useParams } from "react-router-dom";
import { useState, useEffect, useMemo, useCallback, useRef } from "react";
import { Users, UserRound, Check } from "lucide-react";
import { AssetsProvider } from "@/features/assets/AssetsProvider";
import AssetsPageHeader from "@/features/assets/components/shared/AssetsPageHeader";
import {
  useGroupBy,
  useIsCarouselOpen,
  useUIActions,
} from "@/features/assets/selectors";
import { useAssetsNavigation } from "@/features/assets/hooks/useAssetsNavigation";
import FullScreenCarousel from "@/features/assets/components/page/FullScreen/FullScreenCarousel/FullScreenCarousel";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import PhotosLoadingSkeleton from "@/features/assets/components/page/LoadingSkeleton";
import { JustifiedGallery } from "@/features/assets";
import { useWorkingRepository } from "@/features/settings";
import {
  findAssetIndex,
  flattenAssetGroups,
} from "@/features/assets/utils/assetGroups";
import { useI18n } from "@/lib/i18n.tsx";
import { usePersonAssetsView } from "../hooks/usePersonAssetsView";
import { usePersonDetails } from "../hooks/usePeople";
import { assetUrls } from "@/lib/assets/assetUrls";

const PersonAssetsContent = () => {
  const { t, i18n } = useI18n();
  const { personId, assetId } = useParams<{
    personId: string;
    assetId: string;
  }>();
  const { scopedRepositoryId } = useWorkingRepository();
  const groupBy = useGroupBy();
  const isCarouselOpen = useIsCarouselOpen();
  const { setGroupBy } = useUIActions();
  const { openCarousel, closeCarousel } = useAssetsNavigation();
  const [isScrolled, setIsScrolled] = useState(false);
  const [draftName, setDraftName] = useState("");
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const personIdNumber = personId ? Number(personId) : 0;

  const {
    person,
    isLoading: isPersonLoading,
    error: personError,
    renamePerson,
    isRenaming,
  } = usePersonDetails(personIdNumber || undefined, scopedRepositoryId);

  useEffect(() => {
    setDraftName(person?.name ?? "");
  }, [person?.name, person?.person_id]);

  const {
    assets,
    groups,
    isLoading,
    isLoadingMore,
    fetchMore,
    hasMore,
    error,
  } = usePersonAssetsView(personIdNumber, { withGroups: true });

  const groupedPhotos = groups ?? [];
  const flatAssets = useMemo(
    () => flattenAssetGroups(groupedPhotos),
    [groupedPhotos],
  );

  const slideIndex = useMemo(() => {
    if (assetId && flatAssets.length > 0) {
      return findAssetIndex(flatAssets, assetId);
    }
    return -1;
  }, [assetId, flatAssets]);

  const [isLocatingAsset, setIsLocatingAsset] = useState(false);

  useEffect(() => {
    if (isCarouselOpen && assetId && flatAssets.length > 0) {
      const index = findAssetIndex(flatAssets, assetId);
      if (index < 0) {
        if (hasMore && !isLoading && !isLoadingMore) {
          setIsLocatingAsset(true);
          fetchMore();
        }
      } else {
        setIsLocatingAsset(false);
      }
    }
  }, [
    assetId,
    flatAssets,
    isCarouselOpen,
    hasMore,
    isLoading,
    isLoadingMore,
    fetchMore,
  ]);

  const handleLoadMore = useCallback(() => {
    if (hasMore && !isLoadingMore) {
      fetchMore();
    }
  }, [hasMore, isLoadingMore, fetchMore]);

  const handleScroll = useCallback((e: React.UIEvent<HTMLDivElement>) => {
    setIsScrolled(e.currentTarget.scrollTop > 60);
  }, []);

  const handleRename = useCallback(async () => {
    const nextName = draftName.trim();
    if (!nextName || nextName === person?.name) {
      return;
    }

    await renamePerson(nextName);
  }, [draftName, person?.name, renamePerson]);

  if (error || personError) {
    return (
      <div className="p-8 text-error">
        {t("people.details.loadError", {
          error: String(error ?? personError),
        })}
      </div>
    );
  }

  const isInitialLoading = isLoading && assets.length === 0;
  const displayName = person?.name || t("people.unnamed");
  const coverUrl =
    person?.person_id && person.cover_face_image_path
      ? assetUrls.getPersonCoverUrl(person.person_id, scopedRepositoryId)
      : null;

  return (
    <div className="flex h-full flex-col relative">
      <div className="sticky top-0 z-30 border-b border-base-200/30 bg-base-100/80 backdrop-blur-md">
        <AssetsPageHeader
          groupBy={groupBy}
          onGroupByChange={setGroupBy}
          title={displayName}
          icon={<Users className="w-6 h-6 text-primary" />}
        />

        <div
          className={`overflow-hidden px-4 transition-all duration-500 ease-in-out ${isScrolled ? "py-2" : "py-4"}`}
        >
          <div
            className={`transition-all duration-500 ease-in-out ${isScrolled ? "max-h-0 opacity-0 -translate-y-2" : "max-h-[20rem] opacity-100 translate-y-0"}`}
          >
            <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
              <div className="flex items-start gap-4">
                <div className="h-20 w-20 overflow-hidden rounded-[1.5rem] border border-base-300/70 bg-base-200">
                  {coverUrl ? (
                    <img
                      src={coverUrl}
                      alt={displayName}
                      className="h-full w-full object-cover"
                    />
                  ) : (
                    <div className="flex h-full w-full items-center justify-center">
                      <UserRound className="size-8 text-base-content/40" />
                    </div>
                  )}
                </div>

                <div className="min-w-0">
                  {isPersonLoading && !person ? (
                    <div className="h-10 w-56 animate-pulse rounded-lg bg-base-300" />
                  ) : (
                    <>
                      <div className="flex items-baseline gap-4">
                        <h1 className="truncate text-4xl font-black tracking-tight text-primary">
                          {displayName}
                        </h1>
                        <span className="badge badge-ghost font-mono text-xs opacity-50">
                          {t("people.details.personCode", { id: personId })}
                        </span>
                      </div>
                      <p className="mt-2 max-w-2xl text-sm text-base-content/60">
                        {person?.is_confirmed
                          ? t("people.details.confirmedHint")
                          : t("people.details.unconfirmedHint")}
                      </p>
                    </>
                  )}
                </div>
              </div>

              <div className="flex w-full max-w-md items-center gap-2">
                <input
                  value={draftName}
                  onChange={(event) => setDraftName(event.target.value)}
                  placeholder={t("people.details.renamePlaceholder")}
                  className="input input-bordered w-full"
                />
                <button
                  type="button"
                  className="btn btn-primary"
                  onClick={handleRename}
                  disabled={isRenaming || !draftName.trim()}
                >
                  {isRenaming ? (
                    <span className="loading loading-spinner loading-sm" />
                  ) : (
                    <Check className="size-4" />
                  )}
                  {t("people.details.save")}
                </button>
              </div>
            </div>
          </div>

          <div
            className={`flex items-center gap-6 transition-all duration-500 ease-in-out ${isScrolled ? "mt-0 text-[10px] opacity-60" : "mt-6 text-xs opacity-40"}`}
          >
            <div className="flex items-center gap-2 font-bold uppercase tracking-widest">
              <span className="text-primary text-[8px]">●</span>
              <span>
                {t("people.membersCount", {
                  count: person?.member_count || 0,
                })}
              </span>
            </div>
            <div className="flex items-center gap-2 font-bold uppercase tracking-widest">
              <span className="text-primary text-[8px]">●</span>
              <span>
                {t("people.photosCount", {
                  count: person?.asset_count || 0,
                })}
              </span>
            </div>
            <div className="flex items-center gap-2 font-bold uppercase tracking-widest">
              <span className="text-primary text-[8px]">●</span>
              <span>
                {t("people.details.updatedLabel")}{" "}
                {person?.updated_at
                  ? new Date(person.updated_at).toLocaleDateString(
                      i18n.resolvedLanguage || i18n.language,
                    )
                  : ""}
              </span>
            </div>
          </div>
        </div>
      </div>

      <div
        ref={scrollContainerRef}
        className="flex-1 overflow-y-auto"
        onScroll={handleScroll}
      >
        <div>
          {isInitialLoading ? (
            <PhotosLoadingSkeleton />
          ) : (
            <JustifiedGallery
              groups={groupedPhotos}
              openCarousel={openCarousel}
              onLoadMore={handleLoadMore}
              hasMore={hasMore}
              isLoadingMore={isLoadingMore}
            />
          )}
        </div>
      </div>

      {isCarouselOpen && flatAssets.length > 0 && (
        <>
          <FullScreenCarousel
            photos={flatAssets}
            initialSlide={slideIndex >= 0 ? slideIndex : 0}
            slideIndex={slideIndex >= 0 ? slideIndex : undefined}
            onClose={closeCarousel}
            onNavigate={openCarousel}
          />
          {isLocatingAsset && (
            <div className="fixed inset-0 z-[60] flex items-center justify-center bg-black/70">
              <div className="max-w-md rounded-2xl bg-black/50 p-8 text-center text-white backdrop-blur-sm">
                <div className="loading loading-spinner loading-lg mb-4"></div>
                <p className="mb-2 text-lg font-medium">
                  {t("assets.photos.locating_asset")}
                </p>
                <p className="text-sm text-gray-300">
                  {t("assets.photos.loading_more_data")}
                </p>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
};

const PersonDetails = () => {
  const { personId } = useParams<{ personId: string }>();

  return (
    <WorkerProvider preload={["exif", "export"]}>
      <AssetsProvider persist={false} basePath={`/people/${personId}`}>
        <PersonAssetsContent />
      </AssetsProvider>
    </WorkerProvider>
  );
};

export default PersonDetails;
