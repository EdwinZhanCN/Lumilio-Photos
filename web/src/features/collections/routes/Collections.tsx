import { useNavigate } from "react-router-dom";
import { ErrorBoundary } from "react-error-boundary";
import { Album, ArrowRight, FolderTree, LibraryBig, MapPin, Users, Wrench } from "lucide-react";
import ErrorFallBack from "@/components/ErrorFallBack";
import PageHeader from "@/components/PageHeader";
import BrowseScopeSelect from "@/components/BrowseScopeSelect";
import { useBreadcrumbs } from "@/components/breadcrumbs";
import { CollectionErrorAlert } from "@/components/collection";
import { useI18n } from "@/lib/i18n.tsx";
import { useBrowseScope } from "@/features/settings";
import { usePeople } from "@/features/people/hooks/usePeople";
import AlbumRail from "../components/AlbumRail";
import FoldersRail from "../components/FoldersRail";
import MapRail from "../components/MapRail";
import PeopleRail from "../components/PeopleRail";
import UtilitiesRail from "../components/UtilitiesRail";
import { useAlbums } from "../hooks/useAlbums";
import { useCityTrips } from "../hooks/useCityTrips";
import { useFolders } from "../hooks/useFolders";
import { encodeFolderKey } from "../utils/folderKey";

function CollectionsContent() {
  const { t } = useI18n();
  const navigate = useNavigate();
  useBreadcrumbs([
    { label: t("sidebar.home", "Home"), to: "/" },
    { label: t("sidebar.collections", "Collections") },
  ]);
  const { scopedRepositoryId } = useBrowseScope();
  const {
    data,
    isPending: isAlbumsLoading,
    isError: isAlbumsError,
    error: albumsError,
  } = useAlbums(t, scopedRepositoryId);
  const {
    people,
    isLoading: isPeopleLoading,
    isError: isPeopleError,
    error: peopleError,
  } = usePeople({
    limit: 18,
    repositoryId: scopedRepositoryId,
  });

  const albums = data?.pages.flatMap((page) => page.albums) ?? [];
  const { trips, isLoading: isTripsLoading } = useCityTrips({ repositoryId: scopedRepositoryId });
  const { data: foldersData, isPending: isFoldersLoading } = useFolders(scopedRepositoryId, "");
  const folders = foldersData?.folders ?? [];

  return (
    <div className="flex h-full flex-col">
      <PageHeader
        title={t("routes.collections")}
        icon={<LibraryBig className="h-6 w-6 text-primary" strokeWidth={1.5} />}
      >
        <BrowseScopeSelect />
      </PageHeader>

      <div className="flex-1 min-h-0 overflow-y-auto px-4 pb-8 pt-4">
        <div className="space-y-6">
          {isAlbumsError && (
            <CollectionErrorAlert
              message={t("collections.messages.loadAlbumsError", {
                message:
                  albumsError instanceof Error ? albumsError.message : t("home.errors.unknown"),
              })}
            />
          )}

          {isPeopleError && (
            <CollectionErrorAlert
              message={t("collections.messages.loadPeopleError", {
                message:
                  peopleError instanceof Error ? peopleError.message : t("home.errors.unknown"),
              })}
            />
          )}

          <section className="space-y-4">
            <div className="flex items-center justify-between gap-4">
              <div className="flex items-center gap-3">
                <div className="rounded-2xl bg-base-200 p-3 text-primary">
                  <Wrench className="size-5" strokeWidth={1.75} />
                </div>
                <h2 className="text-2xl font-black tracking-tight">
                  {t("collections.sections.utilities")}
                </h2>
              </div>
              <button
                type="button"
                className="btn btn-ghost btn-sm rounded-full"
                onClick={() => navigate("/collections/utilities")}
              >
                {t("common.viewAll")}
                <ArrowRight className="size-4" />
              </button>
            </div>

            <UtilitiesRail />
          </section>

          <section className="space-y-4">
            <div className="flex items-center justify-between gap-4">
              <div className="flex items-center gap-3">
                <div className="rounded-2xl bg-base-200 p-3 text-primary">
                  <MapPin className="size-5" strokeWidth={1.75} />
                </div>
                <h2 className="text-2xl font-black tracking-tight">
                  {t("collections.sections.places")}
                </h2>
              </div>
              <button
                type="button"
                className="btn btn-ghost btn-sm rounded-full"
                onClick={() => navigate("/collections/map")}
              >
                {t("common.viewAll")}
                <ArrowRight className="size-4" />
              </button>
            </div>

            <MapRail
              trips={trips.slice(0, 12)}
              loading={isTripsLoading}
              onMapClick={() => navigate("/collections/map")}
              onTripClick={(trip) =>
                navigate(`/collections/places/${trip.id}`, {
                  state: { trip },
                })
              }
            />
          </section>

          <section className="space-y-4">
            <div className="flex items-center justify-between gap-4">
              <div className="flex items-center gap-3">
                <div className="rounded-2xl bg-base-200 p-3 text-primary">
                  <Album className="size-5" strokeWidth={1.75} />
                </div>
                <h2 className="text-2xl font-black tracking-tight">
                  {t("collections.sections.albums")}
                </h2>
              </div>
              <button
                type="button"
                className="btn btn-ghost btn-sm rounded-full"
                onClick={() => navigate("/collections/albums")}
              >
                {t("common.viewAll")}
                <ArrowRight className="size-4" />
              </button>
            </div>

            <AlbumRail
              albums={albums.slice(0, 12)}
              loading={isAlbumsLoading}
              onAlbumClick={(album) => navigate(`/collections/${album.id}`)}
            />
          </section>

          <section className="space-y-4">
            <div className="flex items-center justify-between gap-4">
              <div className="flex items-center gap-3">
                <div className="rounded-2xl bg-base-200 p-3 text-primary">
                  <Users className="size-5" strokeWidth={1.75} />
                </div>
                <h2 className="text-2xl font-black tracking-tight">
                  {t("collections.sections.people")}
                </h2>
              </div>
              <button
                type="button"
                className="btn btn-ghost btn-sm rounded-full"
                onClick={() => navigate("/collections/people")}
              >
                {t("common.viewAll")}
                <ArrowRight className="size-4" />
              </button>
            </div>

            <PeopleRail
              people={people}
              loading={isPeopleLoading}
              repositoryId={scopedRepositoryId}
              onPersonClick={(person) => {
                if (!person?.person_id) return;
                void navigate(`/people/${person.person_id}`);
              }}
            />
          </section>

          <section className="space-y-4">
            <div className="flex items-center justify-between gap-4">
              <div className="flex items-center gap-3">
                <div className="rounded-2xl bg-base-200 p-3 text-primary">
                  <FolderTree className="size-5" strokeWidth={1.75} />
                </div>
                <h2 className="text-2xl font-black tracking-tight">
                  {t("collections.sections.folders")}
                </h2>
              </div>
              <button
                type="button"
                className="btn btn-ghost btn-sm rounded-full"
                onClick={() => navigate("/collections/folders")}
              >
                {t("common.viewAll")}
                <ArrowRight className="size-4" />
              </button>
            </div>

            <FoldersRail
              folders={folders.slice(0, 12)}
              loading={isFoldersLoading}
              onFolderClick={(folder) =>
                navigate(
                  `/collections/folders/${encodeFolderKey({
                    repositoryId: folder.repository_id ?? "",
                    folderPath: folder.folder_path ?? "",
                  })}`,
                )
              }
            />
          </section>
        </div>
      </div>
    </div>
  );
}

export default function Collections() {
  const { t } = useI18n();

  return (
    <ErrorBoundary
      FallbackComponent={(props) => (
        <ErrorFallBack
          code={500}
          title={t("assets.errorFallback.something_went_wrong")}
          {...props}
        />
      )}
    >
      <CollectionsContent />
    </ErrorBoundary>
  );
}
