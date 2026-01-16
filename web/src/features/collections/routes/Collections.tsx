import ErrorFallBack from "@/components/ErrorFallBack";
import PageHeader from "@/components/PageHeader";
import { ErrorBoundary } from "react-error-boundary";
import { Album } from "lucide-react";
import { ImgStackGrid } from "../components/ImgStackGrid";
import { useI18n } from "@/lib/i18n.tsx";
import { useAlbums } from "../hooks/useAlbums";
import { useNavigate } from "react-router-dom";

function Collections() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const { data, isPending, hasNextPage, fetchNextPage, isFetchingNextPage } = useAlbums(t);

  // Flatten all pages into a single array
  const albums = data?.pages.flatMap((page) => page.albums) ?? [];

  const handleAlbumClick = (album: { id: string }) => {
    navigate(`/collections/${album.id}`);
  };

  return (
    <ErrorBoundary
      FallbackComponent={(props) => (
        <ErrorFallBack code={500} title="Something went wrong" {...props} />
      )}
    >
      <PageHeader
        title={t("routes.collections")}
        icon={<Album className="w-6 h-6 text-primary" strokeWidth={1.5} />}
      />

      <ImgStackGrid
        albums={albums}
        onAlbumClick={handleAlbumClick}
        loading={isPending}
        emptyMessage={t("collections.emptyMessage", {
          defaultValue: "Create your first album to get started",
        })}
      />

      {hasNextPage && (
        <div className="flex justify-center p-4">
          <button
            onClick={() => fetchNextPage()}
            disabled={isFetchingNextPage}
            className="px-6 py-2 bg-primary text-white rounded-lg hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            {isFetchingNextPage
              ? t("common.loading", { defaultValue: "Loading..." })
              : t("common.loadMore", { defaultValue: "Load More" })}
          </button>
        </div>
      )}
    </ErrorBoundary>
  );
}

export default Collections;
