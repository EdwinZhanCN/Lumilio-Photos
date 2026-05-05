import { useNavigate } from "react-router-dom";
import { ErrorBoundary } from "react-error-boundary";
import { Album, AlertTriangle, ArrowRight, Users } from "lucide-react";
import ErrorFallBack from "@/components/ErrorFallBack";
import PageHeader from "@/components/PageHeader";
import { useI18n } from "@/lib/i18n.tsx";
import { useWorkingRepository } from "@/features/settings";
import { usePeople } from "@/features/people/hooks/usePeople";
import AlbumRail from "../components/AlbumRail";
import PeopleRail from "../components/PeopleRail";
import { useAlbums } from "../hooks/useAlbums";

function CollectionsContent() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const { scopedRepositoryId } = useWorkingRepository();
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

  return (
    <div className="flex h-full flex-col">
      <PageHeader
        title={t("routes.collections")}
        icon={<Album className="h-6 w-6 text-primary" strokeWidth={1.5} />}
      />

      <div className="flex-1 min-h-0 overflow-y-auto px-4 pb-8 pt-4">
        <div className="space-y-6">
          {isAlbumsError && (
            <div className="alert alert-warning">
              <AlertTriangle className="size-5" />
              <span>
                {t("collections.messages.loadAlbumsError", {
                  message:
                    albumsError instanceof Error
                      ? albumsError.message
                      : t("home.errors.unknown"),
                })}
              </span>
            </div>
          )}

          {isPeopleError && (
            <div className="alert alert-warning">
              <AlertTriangle className="size-5" />
              <span>
                {t("collections.messages.loadPeopleError", {
                  message:
                    peopleError instanceof Error
                      ? peopleError.message
                      : t("home.errors.unknown"),
                })}
              </span>
            </div>
          )}

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
                navigate(`/people/${person.person_id}?groupBy=date`);
              }}
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
