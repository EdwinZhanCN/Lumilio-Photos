
import { useMemo, useCallback, useState, useEffect } from "react";
import { useParams } from "react-router-dom";
import AssetsPageHeader from "@/features/assets/components/shared/AssetsPageHeader";
import FullScreenCarousel from "@/features/assets/components/page/FullScreen/FullScreenCarousel/FullScreenCarousel";
import JustifiedGallery from "@/features/assets/components/page/JustifiedGallery/JustifiedGallery";
import PhotosLoadingSkeleton from "@/features/assets/components/page/LoadingSkeleton";
import { useAssetsNavigation } from "@/features/assets/hooks/useAssetsNavigation";
import { useCurrentTabAssets } from "@/features/assets/hooks/useAssetsView";
import {
    useGroupBy,
    useIsCarouselOpen,
    useUIActions
} from "@/features/assets/selectors";
import {
    groupAssets,
    getFlatAssetsFromGrouped,
    findAssetIndex,
} from "@/lib/utils/assetGrouping.ts";
import { useI18n } from "@/lib/i18n";



export type AssetCategory = 'photos' | 'videos' | 'audios';

interface AssetsGalleryPageProps {
    category: AssetCategory;
}

export function AssetsGalleryPage({ category }: AssetsGalleryPageProps) {
    const { assetId } = useParams<{ assetId: string }>();
    const { openCarousel, closeCarousel } = useAssetsNavigation();
    const { t } = useI18n();

    // Selectors
    const groupBy = useGroupBy();
    const isCarouselOpen = useIsCarouselOpen();
    const { setGroupBy } = useUIActions();

    // Get assets for current view using the new hook
    const {
        assets: allAssets,
        groups: groupedAssets,
        isLoading: isFetching,
        isLoadingMore: isFetchingNextPage,
        fetchMore: fetchNextPage,
        hasMore: hasNextPage,
        isFetched,
        error,

    } = useCurrentTabAssets({
        withGroups: true,
        groupBy,
    });

    const hasFetchedOnce = isFetched;

    const handleLoadMore = useCallback(() => {
        if (hasNextPage && !isFetchingNextPage) {
            fetchNextPage();
        }
    }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

    // Use grouped assets if available, otherwise group manually
    const finalGroupedAssets = useMemo(() => {
        if (groupedAssets && Object.keys(groupedAssets).length > 0) {
            return groupedAssets;
        }
        return groupAssets(allAssets, groupBy);
    }, [groupedAssets, allAssets, groupBy]);

    const flatAssets = useMemo(() => {
        return getFlatAssetsFromGrouped(finalGroupedAssets);
    }, [finalGroupedAssets]);



    // Carousel positioning logic
    const [slideIndex, setSlideIndex] = useState<number>(-1);
    const [isLocatingAsset, setIsLocatingAsset] = useState(false);



    useEffect(() => {
        if (!isCarouselOpen) return;
        if (assetId && hasFetchedOnce) {
            const index = findAssetIndex(flatAssets, assetId);
            if (index >= 0) {
                setSlideIndex(index);
                setIsLocatingAsset(false);
                return;
            }
            setIsLocatingAsset(true);
            if (hasNextPage && !isFetching && !isFetchingNextPage) {
                fetchNextPage();
                return;
            }
            if (!hasNextPage && !isFetching && !isFetchingNextPage) {
                const timer = setTimeout(() => closeCarousel(), 500);
                return () => clearTimeout(timer);
            }
        }
    }, [assetId, flatAssets, isCarouselOpen, hasFetchedOnce, hasNextPage, isFetching, isFetchingNextPage, fetchNextPage, closeCarousel]);

    useEffect(() => {
        if (assetId && flatAssets.length > 0) {
            const index = findAssetIndex(flatAssets, assetId);
            if (index >= 0) {
                setSlideIndex(index);
                setIsLocatingAsset(false);
            }
        }
    }, [assetId, flatAssets]);

    if (error) throw new Error(error);

    return (
        <div>
            <AssetsPageHeader groupBy={groupBy} onGroupByChange={setGroupBy} onFiltersChange={() => { }} />

            {isFetching && allAssets.length === 0 ? (
                <PhotosLoadingSkeleton />
            ) : (
                <JustifiedGallery
                    groupedPhotos={finalGroupedAssets}
                    openCarousel={openCarousel}
                    onLoadMore={handleLoadMore}
                    hasMore={hasNextPage}
                    isLoadingMore={isFetchingNextPage}
                />
            )}

            {!hasNextPage && allAssets.length > 0 && (
                <div className="text-center p-4 text-gray-500">{t(`assets.${category}.end_of_results`)}</div>
            )}

            {isCarouselOpen && (
                flatAssets.length > 0 ? (
                    <>
                        <FullScreenCarousel
                            photos={flatAssets}
                            initialSlide={slideIndex >= 0 ? slideIndex : 0}
                            slideIndex={slideIndex >= 0 ? slideIndex : undefined}
                            onClose={closeCarousel}
                            onNavigate={openCarousel}
                        />
                        {isLocatingAsset && (
                            <div className="fixed inset-0 bg-black/70 z-[60] flex items-center justify-center">
                                <div className="text-white text-center bg-black/50 backdrop-blur-sm rounded-2xl p-8 max-w-md">
                                    <div className="loading loading-spinner loading-lg mb-4"></div>
                                    <p className="text-lg font-medium mb-2">{t(`assets.${category}.locating_asset`)}</p>
                                    {hasNextPage && !isFetching && !isFetchingNextPage ? (
                                        <p className="text-sm text-gray-300">
                                            {t(`assets.${category}.loading_more_data`)}
                                        </p>
                                    ) : (
                                        <p className="text-sm text-gray-300">
                                            {t(`assets.${category}.asset_not_available`)}
                                        </p>
                                    )}
                                </div>
                            </div>
                        )}
                    </>
                ) : (
                    <div className="fixed inset-0 bg-black/90 z-50 flex items-center justify-center">
                        <div className="text-white text-center">
                            <div className="loading loading-spinner loading-lg mb-4"></div>
                            <p>{t(`assets.${category}.loading_assets`)}</p>
                        </div>
                    </div>
                )
            )}
        </div>
    );
}
