import { UserRound } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { assetUrls } from "@/lib/assets/assetUrls";
import type { PersonSummary } from "@/features/people/people.types";

type PeopleGridProps = {
  people?: PersonSummary[];
  isLoading?: boolean;
  repositoryId?: string;
  onPersonClick?: (person: PersonSummary) => void;
};

const PlaceholderCards = () => (
  <div className="grid grid-cols-2 gap-3 md:grid-cols-4 xl:grid-cols-6">
    {Array.from({ length: 6 }).map((_, index) => (
      <div
        key={index}
        className="overflow-hidden rounded-[1.5rem] border border-base-300/70 bg-base-200/70"
      >
        <div className="aspect-[4/5] animate-pulse bg-base-300/70" />
        <div className="space-y-2 p-4">
          <div className="h-4 w-24 animate-pulse rounded bg-base-300/70" />
          <div className="h-3 w-16 animate-pulse rounded bg-base-300/50" />
        </div>
      </div>
    ))}
  </div>
);

export default function PeopleGrid({
  people = [],
  isLoading = false,
  repositoryId,
  onPersonClick,
}: PeopleGridProps) {
  const { t } = useI18n();

  return (
    <section className="px-4 pb-4">
      <div className="mb-4 flex items-end justify-between gap-4">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.3em] text-primary/70">
            {t("home.people.eyebrow")}
          </p>
          <h2 className="mt-1 text-2xl font-black tracking-tight">
            {t("home.people.title")}
          </h2>
          <p className="mt-1 text-sm text-base-content/60">
            {t("home.people.subtitle")}
          </p>
        </div>
      </div>

      {isLoading ? (
        <PlaceholderCards />
      ) : people.length === 0 ? (
        <div className="rounded-[1.5rem] border border-dashed border-base-300 px-6 py-8 text-sm text-base-content/60">
          {t("home.people.empty")}
        </div>
      ) : (
        <div className="grid grid-cols-2 gap-3 md:grid-cols-4 xl:grid-cols-6">
          {people.map((person) => {
            const coverUrl = assetUrls.getPersonCoverUrl(
              person.person_id ?? 0,
              repositoryId,
            );

            return (
              <button
                key={person.person_id}
                type="button"
                onClick={() => onPersonClick?.(person)}
                className="group overflow-hidden rounded-[1.5rem] border border-base-300/70 bg-base-100 text-left shadow-[0_18px_48px_-32px_rgba(15,23,42,0.35)] transition hover:-translate-y-0.5 hover:border-primary/30 hover:shadow-[0_24px_60px_-34px_rgba(15,23,42,0.45)]"
              >
                <div className="relative aspect-[4/5] overflow-hidden bg-base-200">
                  {person.cover_face_image_path ? (
                    <img
                      src={coverUrl}
                      alt={person.name || t("people.unnamed")}
                      className="h-full w-full object-cover transition duration-500 group-hover:scale-[1.03]"
                      loading="lazy"
                    />
                  ) : (
                    <div className="flex h-full items-center justify-center bg-gradient-to-br from-base-200 via-base-300/70 to-base-200">
                      <UserRound className="size-12 text-base-content/40" />
                    </div>
                  )}
                  {person.is_confirmed ? (
                    <span className="absolute right-3 top-3 rounded-full bg-base-100/90 px-2 py-1 text-[10px] font-bold uppercase tracking-[0.2em] text-primary shadow-sm">
                      {t("people.confirmed")}
                    </span>
                  ) : null}
                </div>
                <div className="space-y-1 p-4">
                  <h3 className="truncate text-sm font-bold">
                    {person.name || t("people.unnamed")}
                  </h3>
                  <p className="text-xs uppercase tracking-[0.2em] text-base-content/50">
                    {t("people.photosCount", { count: person.asset_count ?? 0 })}
                  </p>
                </div>
              </button>
            );
          })}
        </div>
      )}
    </section>
  );
}
