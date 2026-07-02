import { useParams } from "react-router-dom";
import { useState, useCallback } from "react";
import { EyeOff, Users, UserRound } from "lucide-react";
import { AssetsProvider } from "@/features/assets/AssetsProvider";
import { AssetsGalleryPage } from "@/features/assets/components/page/AssetsGalleryPage";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import { useBrowseScope } from "@/features/settings";
import { useBreadcrumbs } from "@/components/breadcrumbs";
import { useI18n } from "@/lib/i18n.tsx";
import { usePersonDetails } from "../hooks/usePeople";
import { assetUrls } from "@/lib/assets/assetUrls";
import { CollectionHero, MetaStat } from "@/components/collection";
import PersonRenameModal from "../components/PersonRenameModal";

const PersonAssetsContent = () => {
  const { t, i18n } = useI18n();
  const { personId } = useParams<{
    personId: string;
    assetId: string;
  }>();
  const { scopedRepositoryId } = useBrowseScope();
  const [isRenameOpen, setIsRenameOpen] = useState(false);
  const personIdNumber = personId ? Number(personId) : 0;

  const {
    person,
    isLoading: isPersonLoading,
    renamePerson,
    isRenaming,
  } = usePersonDetails(personIdNumber || undefined, scopedRepositoryId);

  const handleRename = useCallback(
    async (nextName: string) => {
      if (!nextName || nextName === person?.name) return;
      await renamePerson(nextName);
    },
    [person?.name, renamePerson],
  );

  const displayName = person?.name || t("people.unnamed");
  useBreadcrumbs([
    { label: t("sidebar.home", "Home"), to: "/" },
    { label: t("sidebar.collections", "Collections"), to: "/collections" },
    { label: t("collections.sections.people", "People"), to: "/collections/people" },
    { label: displayName },
  ]);
  const coverUrl =
    person?.person_id && person.cover_face_image_path
      ? assetUrls.getPersonCoverUrl(person.person_id, scopedRepositoryId)
      : null;

  const cover = (
    <div className="h-20 w-20 overflow-hidden rounded-[1.5rem] border border-base-300/70 bg-base-200">
      {coverUrl ? (
        <img src={coverUrl} alt={displayName} className="h-full w-full object-cover" />
      ) : (
        <div className="flex h-full w-full items-center justify-center">
          <UserRound className="size-8 text-base-content/40" />
        </div>
      )}
    </div>
  );

  const hero = (
    <CollectionHero
      loading={isPersonLoading && !person}
      cover={cover}
      title={displayName}
      code={t("people.details.personCode", { id: personId })}
      badges={
        person?.is_hidden ? (
          <span className="badge badge-neutral badge-sm gap-1">
            <EyeOff className="size-3" />
            {t("people.hidden.badge", "Hidden")}
          </span>
        ) : null
      }
      description={
        person?.is_confirmed
          ? t("people.details.confirmedHint")
          : t("people.details.unconfirmedHint")
      }
      edit={{
        onOpen: () => setIsRenameOpen(true),
        label: t("people.details.editAction", "Edit"),
        modal: (
          <PersonRenameModal
            open={isRenameOpen}
            person={person}
            currentName={person?.name ?? ""}
            repositoryId={scopedRepositoryId}
            isSaving={isRenaming}
            onClose={() => setIsRenameOpen(false)}
            onSubmit={handleRename}
          />
        ),
      }}
      stats={
        <>
          <MetaStat>{t("people.membersCount", { count: person?.member_count || 0 })}</MetaStat>
          <MetaStat>{t("people.photosCount", { count: person?.asset_count || 0 })}</MetaStat>
          <MetaStat>
            {t("people.details.updatedLabel")}{" "}
            {person?.updated_at
              ? new Date(person.updated_at).toLocaleDateString(
                  i18n.resolvedLanguage || i18n.language,
                )
              : ""}
          </MetaStat>
        </>
      }
    />
  );

  return (
    <AssetsGalleryPage
      title={displayName}
      icon={<Users className="w-6 h-6 text-primary" />}
      viewKey={`person:${personId}`}
      baseFilter={{ person_id: personIdNumber }}
      hero={hero}
    />
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
