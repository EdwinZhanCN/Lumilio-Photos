import type { ReactNode } from "react";
import { useShareLinks, type CreateShareLinkResponseDTO } from "../../api/useShareLinks";
import ShareLinkCreateModal from "./ShareLinkCreateModal";

export type ShareSourceKind = "asset_snapshot" | "album" | "person" | "utility_query" | "pin";

export interface CreateShareLinkModalProps {
  open: boolean;
  onClose: () => void;
  sourceKind: ShareSourceKind;
  /** Required for sourceKind "asset_snapshot"; ignored otherwise. */
  assetIds?: string[];
  /** Required for album/person/utility_query/pin sources. */
  sourceRef?: string;
  defaultTitle?: string;
  onCreated?: (link: CreateShareLinkResponseDTO) => void;
}

/** Resolves a regular gallery source and delegates presentation to the shared share-link form. */
export function CreateShareLinkModal({
  open,
  onClose,
  sourceKind,
  assetIds,
  sourceRef,
  defaultTitle,
  onCreated,
}: CreateShareLinkModalProps): ReactNode {
  const { createShareLink, isCreating } = useShareLinks();

  return (
    <ShareLinkCreateModal
      open={open}
      onClose={onClose}
      defaultTitle={defaultTitle}
      isCreating={isCreating}
      onCreated={onCreated}
      onCreate={(values) =>
        createShareLink({
          title: values.title,
          description: values.description,
          source_kind: sourceKind,
          source_ref: sourceRef,
          asset_ids: sourceKind === "asset_snapshot" ? assetIds : undefined,
          expires_in_days: values.expiresInDays,
          allow_download: values.allowDownload,
          include_originals: values.includeOriginals,
        })
      }
    />
  );
}

export default CreateShareLinkModal;
