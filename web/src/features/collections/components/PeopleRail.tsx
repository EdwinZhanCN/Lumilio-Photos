import { UserRound } from "lucide-react";
import { assetUrls } from "@/lib/assets/assetUrls";
import { useI18n } from "@/lib/i18n.tsx";
import Rail from "./Rail";
import RailCard from "./RailCard";
import type { PersonSummaryList } from "@/features/people/people.types";

type PeopleRailProps = {
  people: PersonSummaryList;
  loading?: boolean;
  repositoryId?: string;
  onPersonClick?: (person: PersonSummaryList[number]) => void;
};

export default function PeopleRail({
  people,
  loading = false,
  repositoryId,
  onPersonClick,
}: PeopleRailProps) {
  const { t } = useI18n();

  return (
    <Rail
      loading={loading}
      skeletonCount={5}
      isEmpty={people.length === 0}
      empty={
        <div className="rounded-[1.75rem] border border-dashed border-base-300 px-6 py-8 text-sm text-base-content/60">
          {t("collections.emptyPeople")}
        </div>
      }
    >
      {people.map((person) => {
        const personId = person.person_id ?? 0;
        return (
          <RailCard
            key={personId}
            media={{
              kind: "photo",
              src: person.cover_face_image_path
                ? assetUrls.getPersonCoverUrl(personId, repositoryId)
                : null,
              fallbackIcon: UserRound,
            }}
            title={person.name || t("people.unnamed")}
            subtitle={t("people.photosCount", { count: person.asset_count ?? 0 })}
            onClick={() => onPersonClick?.(person)}
            className="w-48"
          />
        );
      })}
    </Rail>
  );
}
