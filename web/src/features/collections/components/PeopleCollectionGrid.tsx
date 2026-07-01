import { EyeOff, UserRound } from "lucide-react";
import { assetUrls } from "@/lib/assets/assetUrls";
import { useI18n } from "@/lib/i18n.tsx";
import type { PersonSummaryList } from "@/features/people/people.types";

type PeopleCollectionGridProps = {
  people: PersonSummaryList;
  loading?: boolean;
  repositoryId?: string;
  onPersonClick?: (person: PersonSummaryList[number]) => void;
};

const PeopleGridSkeleton = () => (
  <div className="grid grid-cols-2 gap-4 md:grid-cols-4 xl:grid-cols-6">
    {Array.from({ length: 12 }).map((_, index) => (
      <div key={index}>
        <div className="aspect-[4/5] animate-pulse rounded-[1.75rem] bg-base-300/70" />
        <div className="mt-3 space-y-2 px-1">
          <div className="h-4 w-24 animate-pulse rounded bg-base-300/70" />
          <div className="h-3 w-16 animate-pulse rounded bg-base-300/50" />
        </div>
      </div>
    ))}
  </div>
);

export default function PeopleCollectionGrid({
  people,
  loading = false,
  repositoryId,
  onPersonClick,
}: PeopleCollectionGridProps) {
  const { t } = useI18n();

  if (loading) {
    return <PeopleGridSkeleton />;
  }

  if (people.length === 0) {
    return (
      <div className="rounded-[1.75rem] border border-dashed border-base-300 px-6 py-8 text-sm text-base-content/60">
        {t("collections.emptyPeople")}
      </div>
    );
  }

  return (
    <div className="grid grid-cols-2 gap-4 md:grid-cols-4 xl:grid-cols-6">
      {people.map((person) => {
        const personId = person.person_id ?? 0;
        const coverUrl = assetUrls.getPersonCoverUrl(personId, repositoryId);

        return (
          <button
            key={personId}
            type="button"
            onClick={() => onPersonClick?.(person)}
            className="group text-left"
          >
            <div className="relative aspect-[4/5] overflow-hidden rounded-[1.75rem] bg-base-200 shadow-[0_18px_48px_-32px_rgba(15,23,42,0.45)] transition duration-300 group-hover:-translate-y-0.5 group-hover:shadow-[0_24px_56px_-32px_rgba(15,23,42,0.55)]">
              {person.cover_face_image_path ? (
                <img
                  src={coverUrl}
                  alt={person.name || t("people.unnamed")}
                  className="h-full w-full object-cover transition duration-500 group-hover:scale-[1.03]"
                  loading="lazy"
                />
              ) : (
                <div className="flex h-full items-center justify-center bg-gradient-to-br from-base-200 via-base-300/70 to-base-200">
                  <UserRound className="size-12 text-base-content/35" />
                </div>
              )}
              {person.is_hidden && (
                <span className="badge badge-neutral badge-sm absolute left-2 top-2 gap-1 border-none bg-base-content/70 text-base-100">
                  <EyeOff className="size-3" />
                  {t("people.hidden.badge", "Hidden")}
                </span>
              )}
            </div>
            <div className="mt-3 space-y-1 px-1">
              <p className="truncate text-base font-semibold">
                {person.name || t("people.unnamed")}
              </p>
              <p className="text-sm text-base-content/55">
                {t("people.photosCount", { count: person.asset_count ?? 0 })}
              </p>
            </div>
          </button>
        );
      })}
    </div>
  );
}
