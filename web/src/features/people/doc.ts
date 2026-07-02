/**
 * # People
 *
 * Recognized people — face clusters surfaced as named, browsable identities —
 * and the correction tools that keep them honest. The `collections` feature
 * owns the people *rail/grid* entry point; this feature owns person **detail**
 * and every identity correction. AI grouping is assistive; the user is the
 * authority for who is who.
 *
 * ## Ownership
 *
 * A person is owner-scoped but **not** repository-scoped: clusters span
 * repositories, so only the list read follows the browse scope.
 * {@link usePeople} lists people under the current browse scope (with an
 * `includeHidden` toggle that switches the grid from visible-only to all
 * people); {@link usePersonDetails} loads one person plus its rename, with no
 * repository filter — as do the face list and every mutation below.
 * {@link PersonDetails} is the detail route: a {@link CollectionHero} with an
 * edit action that opens {@link PersonRenameModal}, and the person's asset
 * gallery rendered through {@link AssetsGalleryPage}. See
 * `site/docs/internal/agent/scoping.md` for the scoping model.
 *
 * ## Corrections
 *
 * Two distinct surfaces sit on the same person:
 *
 * - **Asset gallery** — the person's *photos*, scoped by `{ person_id }`. This
 *   is the browsing surface and does not render face crops as gallery content.
 * - **Edit modal** — {@link PersonRenameModal} owns tabbed identity management:
 *   info/name/hidden state, face corrections through {@link PersonFacesPanel}
 *   over {@link usePersonFaces}, and merge via {@link PersonPicker}.
 *
 * The face-level operations:
 *
 * - {@link useSetPersonCover} — promote one face to the representative cover
 *   from that face's non-selection-mode menu.
 * - {@link useMoveFace} — reassign selected faces to another person
 *   ({@link PersonPicker} picks the target). Each reassignment becomes a manual
 *   correction. Photos and other faces in the same asset are unchanged.
 * - {@link useRemoveFaceFromPerson} — detach selected faces, leaving them
 *   unclustered for a later rebuild. The original assets are never modified.
 * - {@link useMergePeople} — fold source people into the target from the merge
 *   tab. Source clusters are emptied and removed; the target
 *   name/confirmation/hidden state survive.
 * - {@link useSetPersonHidden} — hide a person from the default grid via a
 *   normal switch in the info tab. Faces, assets and names are preserved.
 *
 * ## Decisions
 *
 * Corrections are durable: moves and merges mark membership `is_manual`, and a
 * full cluster rebuild replays manual assignments so user corrections are not
 * discarded. Editing is modal-only, matching the collections mental model and
 * keeping the gallery focused on photos. Face crops are served per-face (not
 * full thumbnails) so the correction grid shows exactly what the model grouped.
 *
 * @module
 */
import type { CollectionHero } from "@/components/collection";
import type { AssetsGalleryPage } from "@/features/assets/components/page/AssetsGalleryPage.tsx";
import type {
  usePeople,
  usePersonDetails,
  usePersonFaces,
  useMergePeople,
  useMoveFace,
  useRemoveFaceFromPerson,
  useSetPersonCover,
  useSetPersonHidden,
} from "./hooks/usePeople.ts";
import type PersonDetails from "./routes/PersonDetails.tsx";
import type PersonRenameModal from "./components/PersonRenameModal.tsx";
import type PersonFacesPanel from "./components/PersonFacesPanel.tsx";
import type PersonPicker from "./components/PersonPicker.tsx";
export {};
