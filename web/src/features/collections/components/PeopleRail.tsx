import { UserRound } from "lucide-react";
import { assetUrls } from "@/lib/assets/assetUrls";
import { useI18n } from "@/lib/i18n.tsx";
import type { PersonSummaryList } from "@/features/people/people.types";

type PeopleRailProps = {
  people: PersonSummaryList;
  loading?: boolean;
  repositoryId?: string;
  onPersonClick?: (person: PersonSummaryList[number]) => void;
};

const PeopleRailSkeleton = () => (
  <div className="flex gap-4 overflow-x-auto pb-2">
    {Array.from({ length: 5 }).map((_, index) => (
      <div key={index} className="w-36 shrink-0">
        <div className="aspect-[4/5] animate-pulse rounded-[1.75rem] bg-base-300/70" />
        <div className="mt-3 space-y-2 px-1">
          <div className="h-4 w-20 animate-pulse rounded bg-base-300/70" />
          <div className="h-3 w-14 animate-pulse rounded bg-base-300/50" />
        </div>
      </div>
    ))}
  </div>
);

export default function PeopleRail({
  people,
  loading = false,
  repositoryId,
  onPersonClick,
}: PeopleRailProps) {
  const { t } = useI18n();

  if (loading) {
    return <PeopleRailSkeleton />;
  }

  if (people.length === 0) {
    return (
      <div className="rounded-[1.75rem] border border-dashed border-base-300 px-6 py-8 text-sm text-base-content/60">
        {t("collections.emptyPeople")}
      </div>
    );
  }

  return (
    <div className="flex gap-4 overflow-x-auto pb-2">
      {people.map((person) => {
        const personId = person.person_id ?? 0;
        const coverUrl = assetUrls.getPersonCoverUrl(personId, repositoryId);

        return (
          <button
            key={personId}
            type="button"
            onClick={() => onPersonClick?.(person)}
            className="group w-36 shrink-0 cursor-pointer text-left"
          >
            <div className="relative aspect-[4/5] overflow-hidden rounded-[1.75rem] bg-base-200 transition duration-300">
              {person.cover_face_image_path ? (
                <img
                  src={coverUrl}
                  alt={person.name || t("people.unnamed")}
                  className="h-full w-full object-cover transition duration-500 group-hover:scale-[1.03]"
                  loading="lazy"
                />
              ) : (
                <div className="flex h-full items-center justify-center bg-gradient-to-br from-base-200 via-base-300/70 to-base-200">
                  <UserRound className="size-11 text-base-content/35" />
                </div>
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
