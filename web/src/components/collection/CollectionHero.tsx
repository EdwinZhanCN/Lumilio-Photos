import type { ReactNode } from "react";
import { Pencil } from "lucide-react";
import { CollectionTitle } from "./CollectionTitle";
import { MetaStatRow } from "./MetaStatRow";

/**
 * The hero's edit affordance. Bundles the button trigger and the page-owned
 * modal so they always travel together — the page keeps open/close state, the
 * hero just renders the button and the modal node.
 */
export interface CollectionHeroEdit {
  /** Opens the edit modal (the page owns the open state). */
  onOpen: () => void;
  /** The page-owned edit modal, rendered in the tree below the hero. */
  modal: ReactNode;
  /** Label for the edit button (defaults to a pencil only). */
  label?: ReactNode;
}

export interface CollectionHeroProps {
  /** The big primary title (album name, person name, trip title). */
  title: ReactNode;
  /** Optional mono code badge, e.g. "ALBUM #12". */
  code?: ReactNode;
  /** Skeleton the title while the collection metadata is still loading. */
  loading?: boolean;
  /** Optional thumbnail/cover block rendered to the left of the title. */
  cover?: ReactNode;
  /** Extra badges or inline content rendered after the code badge. */
  badges?: ReactNode;
  /** Secondary line under the title (album description, person hint). */
  description?: ReactNode;
  /** `MetaStat` children for the dotted statistics strip. */
  stats?: ReactNode;
  /** Edit affordance: top-right edit button + the modal it toggles. */
  edit?: CollectionHeroEdit;
  /** Extra top-right actions rendered before the edit button (e.g. a Share button). */
  actions?: ReactNode;
  /** Extra content below the stat row (feedback alerts, etc.). */
  footer?: ReactNode;
  className?: string;
}

/**
 * The shared hero block at the top of a scoped collection view (album, person,
 * trip). Composes `CollectionTitle` + `MetaStatRow` and an optional edit button
 * that toggles a page-supplied `editModal`, so detail routes stop hand-wiring
 * the same title/stat/edit assembly. Rendered by `AssetsGalleryPage` via its
 * `hero` slot.
 */
export function CollectionHero({
  title,
  code,
  loading = false,
  cover,
  badges,
  description,
  stats,
  edit,
  actions,
  footer,
  className = "",
}: CollectionHeroProps): ReactNode {
  return (
    <div className={`px-4 py-4 ${className}`}>
      <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
        <div className="flex items-start gap-4">
          {cover}
          <div className="min-w-0">
            <CollectionTitle title={title} code={code} loading={loading}>
              {badges}
            </CollectionTitle>
            {description && (
              <p className="mt-2 max-w-3xl text-sm leading-relaxed text-base-content/70 line-clamp-2">
                {description}
              </p>
            )}
          </div>
        </div>

        {(actions || edit) && (
          <div className="flex items-center gap-2">
            {actions}
            {edit && (
              <button
                type="button"
                className="btn btn-ghost btn-sm gap-1.5 rounded-full"
                onClick={edit.onOpen}
              >
                <Pencil className="size-3.5" />
                {edit.label}
              </button>
            )}
          </div>
        )}
      </div>

      {stats && <MetaStatRow className="mt-6">{stats}</MetaStatRow>}

      {footer}

      {edit?.modal}
    </div>
  );
}

export default CollectionHero;
