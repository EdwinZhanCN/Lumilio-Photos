import { useParams } from "react-router-dom";
import { useState, useCallback } from "react";
import { EyeOff, Share2, Users, UserRound } from "lucide-react";
import { AssetBrowser, AssetBrowserScope } from "@/features/assets";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import { useBreadcrumbs } from "@/components/breadcrumbs";
import { useI18n } from "@/lib/i18n.tsx";
import { usePersonDetails } from "../../api/usePeople";
import { assetUrls } from "@/lib/assets/assetUrls";
import { CollectionHero, MetaStat } from "@/components/collection";
import PersonRenameModal from "./PersonRenameModal";
import {
  CreateShareLinkModal,
  createShareSelectedBulkAction,
  type ShareSourceKind,
} from "@/features/share";
import type { AssetsBulkActionContext, AssetsBulkActionItem } from "@/lib/assets/bulkActions";

const PersonAssetsContent = () => {
  const { t, i18n } = useI18n();
  const { personId } = useParams<{
    personId: string;
    assetId: string;
  }>();
  const [isRenameOpen, setIsRenameOpen] = useState(false);
  const [shareRequest, setShareRequest] = useState<{
    sourceKind: ShareSourceKind;
    assetIds?: string[];
    sourceRef?: string;
  } | null>(null);
  const personIdNumber = personId ? Number(personId) : 0;

  const {
    person,
    isLoading: isPersonLoading,
    renamePerson,
    isRenaming,
  } = usePersonDetails(personIdNumber || undefined);

  const handleRename = useCallback(
    async (nextName: string) => {
      if (!nextName || nextName === person?.name) return;
      await renamePerson(nextName);
    },
    [person?.name, renamePerson],
  );

  const bulkActions = useCallback(
    (_context: AssetsBulkActionContext): AssetsBulkActionItem[] => [
      createShareSelectedBulkAction(
        t("assets.assetsPageHeader.bulkActions.share.label", "Share"),
        (assetIds) => setShareRequest({ sourceKind: "asset_snapshot", assetIds }),
      ),
    ],
    [t],
  );

  const openSharePerson = useCallback(() => {
    if (!personIdNumber) return;
    setShareRequest({ sourceKind: "person", sourceRef: String(personIdNumber) });
  }, [personIdNumber]);

  const displayName = person?.name || t("people.unnamed");
  useBreadcrumbs([
    { label: t("sidebar.home", "Home"), to: "/" },
    { label: t("sidebar.collections", "Collections"), to: "/collections" },
    { label: t("collections.sections.people", "People"), to: "/collections/people" },
    { label: displayName },
  ]);
  const coverUrl =
    person?.person_id && person.cover_face_image_path
      ? assetUrls.getPersonCoverUrl(person.person_id)
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
      actions={
        <button
          type="button"
          className="btn btn-ghost btn-sm gap-1.5 rounded-full"
          onClick={openSharePerson}
        >
          <Share2 className="size-3.5" />
          {t("people.details.shareAction", "Share")}
        </button>
      }
      edit={{
        onOpen: () => setIsRenameOpen(true),
        label: t("people.details.editAction", "Edit"),
        modal: (
          <PersonRenameModal
            open={isRenameOpen}
            person={person}
            currentName={person?.name ?? ""}
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
    <>
      <AssetBrowser
        title={displayName}
        icon={<Users className="w-6 h-6 text-primary" />}
        viewKey={`person:${personId}`}
        constraint={{ person_id: personIdNumber }}
        hero={hero}
        bulkActions={bulkActions}
      />
      <CreateShareLinkModal
        open={shareRequest !== null}
        onClose={() => setShareRequest(null)}
        sourceKind={shareRequest?.sourceKind ?? "asset_snapshot"}
        assetIds={shareRequest?.assetIds}
        sourceRef={shareRequest?.sourceRef}
        defaultTitle={shareRequest?.sourceKind === "person" ? displayName : undefined}
      />
    </>
  );
};

const PersonDetails = () => {
  const { personId } = useParams<{ personId: string }>();

  return (
    <WorkerProvider>
      <AssetBrowserScope
        key={`person:${personId}`}
        scopeId={`person:${personId}`}
        basePath={`/people/${personId}`}
      >
        <PersonAssetsContent />
      </AssetBrowserScope>
    </WorkerProvider>
  );
};

export default PersonDetails;
