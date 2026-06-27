import { useParams } from "react-router-dom";
import { useState, useEffect, useMemo, useCallback, useRef } from "react";
import { Users, UserRound, Pencil } from "lucide-react";
import { AssetsProvider } from "@/features/assets/AssetsProvider";
import AssetsPageHeader from "@/features/assets/components/shared/AssetsPageHeader";
import {
  useSortBy,
  useIsCarouselOpen,
  useUIActions,
} from "@/features/assets/selectors";
import { useAssetsNavigation } from "@/features/assets/hooks/useAssetsNavigation";
import FullScreenCarousel from "@/features/assets/components/page/FullScreen/FullScreenCarousel/FullScreenCarousel";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import PhotosLoadingSkeleton from "@/features/assets/components/page/LoadingSkeleton";
import { JustifiedGallery } from "@/features/assets";
import { useWorkingRepository } from "@/features/settings";
import { findBrowseItemIndexByAssetId } from "@/features/assets/utils/browseItems";
import { useI18n } from "@/lib/i18n.tsx";
import { usePersonAssetsView } from "../hooks/usePersonAssetsView";
import { usePersonDetails } from "../hooks/usePeople";
import { assetUrls } from "@/lib/assets/assetUrls";
import { CollectionTitle, MetaStat, MetaStatRow } from "@/components/collection";
import PersonRenameModal from "../components/PersonRenameModal";

const PersonAssetsContent = () => {
  const { t, i18n } = useI18n();
  const { personId, assetId } = useParams<{
    personId: string;
    assetId: string;
  }>();
  const { scopedRepositoryId } = useWorkingRepository();
  const sortBy = useSortBy();
  const isCarouselOpen = useIsCarouselOpen();
  const { setSortBy } = useUIActions();
  const { openCarousel, closeCarousel } = useAssetsNavigation();
  const [isScrolled, setIsScrolled] = useState(false);
  const [isRenameOpen, setIsRenameOpen] = useState(false);
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const personIdNumber = personId ? Number(personId) : 0;

  const {
    person,
    isLoading: isPersonLoading,
    error: personError,
    renamePerson,
    isRenaming,
  } = usePersonDetails(personIdNumber || undefined, scopedRepositoryId);

  const {
    assets,
    browseGroups,
    browseItems,
    browseAssets: flatAssets,
    isLoading,
    isLoadingMore,
    fetchMore,
    hasMore,
    error,
  } = usePersonAssetsView(personIdNumber, { withGroups: true });

  const slideIndex = useMemo(() => {
    if (assetId && flatAssets.length > 0) {
      return findBrowseItemIndexByAssetId(browseItems, assetId);
    }
    return -1;
  }, [assetId, browseItems, flatAssets.length]);

  const [isLocatingAsset, setIsLocatingAsset] = useState(false);

  useEffect(() => {
    if (isCarouselOpen && assetId && flatAssets.length > 0) {
      const index = findBrowseItemIndexByAssetId(browseItems, assetId);
      if (index < 0) {
        if (hasMore && !isLoading && !isLoadingMore) {
          setIsLocatingAsset(true);
          void fetchMore();
        }
      } else {
        setIsLocatingAsset(false);
      }
    }
  }, [
    assetId,
    flatAssets,
    browseItems,
    isCarouselOpen,
    hasMore,
    isLoading,
    isLoadingMore,
    fetchMore,
  ]);

  const handleLoadMore = useCallback(() => {
    if (hasMore && !isLoadingMore) {
      void fetchMore();
    }
  }, [hasMore, isLoadingMore, fetchMore]);

  const handleScroll = useCallback((e: React.UIEvent<HTMLDivElement>) => {
    setIsScrolled(e.currentTarget.scrollTop > 60);
  }, []);

  const handleRename = useCallback(
    async (nextName: string) => {
      if (!nextName || nextName === person?.name) return;
      await renamePerson(nextName);
    },
    [person?.name, renamePerson],
  );

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
          sortBy={sortBy}
          onSortByChange={setSortBy}
          title={displayName}
          icon={<Users className="w-6 h-6 text-primary" />}
          browseItems={browseItems}
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
                      <CollectionTitle
                        title={displayName}
                        code={t("people.details.personCode", { id: personId })}
                      />
                      <p className="mt-2 max-w-2xl text-sm text-base-content/60">
                        {person?.is_confirmed
                          ? t("people.details.confirmedHint")
                          : t("people.details.unconfirmedHint")}
                      </p>
                    </>
                  )}
                </div>
              </div>

              <button
                type="button"
                className="btn btn-outline btn-sm gap-1.5 rounded-full"
                onClick={() => setIsRenameOpen(true)}
              >
                <Pencil className="size-3.5" />
                {t("people.details.renameAction", "Rename")}
              </button>
            </div>
          </div>

          <MetaStatRow dense={isScrolled} className={isScrolled ? "mt-0" : "mt-6"}>
            <MetaStat>
              {t("people.membersCount", { count: person?.member_count || 0 })}
            </MetaStat>
            <MetaStat>
              {t("people.photosCount", { count: person?.asset_count || 0 })}
            </MetaStat>
            <MetaStat>
              {t("people.details.updatedLabel")}{" "}
              {person?.updated_at
                ? new Date(person.updated_at).toLocaleDateString(
                    i18n.resolvedLanguage || i18n.language,
                  )
                : ""}
            </MetaStat>
          </MetaStatRow>
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
              browseGroups={browseGroups}
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
                  {t("assets.all.locating_asset")}
                </p>
                <p className="text-sm text-gray-300">
                  {t("assets.all.loading_more_data")}
                </p>
              </div>
            </div>
          )}
        </>
      )}

      <PersonRenameModal
        open={isRenameOpen}
        currentName={person?.name ?? ""}
        isSaving={isRenaming}
        onClose={() => setIsRenameOpen(false)}
        onSubmit={handleRename}
      />
    </div>
  );
};

const PersonDetails = () => {
  const { personId } = useParams<{ personId: string }>();

  return (
    <WorkerProvider>
      <AssetsProvider
        key={`person:${personId}`}
        scopeId={`person:${personId}`}
        persist={false}
        basePath={`/people/${personId}`}
      >
        <PersonAssetsContent />
      </AssetsProvider>
    </WorkerProvider>
  );
};

export default PersonDetails;
