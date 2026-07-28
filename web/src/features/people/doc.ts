/**
 * # People
 *
 * People owns recognized identities, person detail, and the correction tools
 * for face clusters. Collections owns the people rail/grid entry; Assets owns
 * photo presentation. AI clustering is assistive, while user corrections are
 * durable authority.
 *
 * ## State
 *
 * People lists, details, faces, and mutations remain TanStack Query server
 * state. The list read follows {@link usePeople}'s browse scope, but person
 * detail and mutations are owner-scoped rather than repository-scoped because
 * one identity can span repositories.
 *
 * Rename-modal tabs, face selection, and merge target are flow-local
 * interaction. No person or face collection is mirrored into Context,
 * Zustand, URL state, or browser persistence.
 *
 * ## Flows
 *
 * ```mermaid
 * flowchart TD
 *     COLLECTIONS["Collections people entry"] --> DETAIL["PersonDetails"]
 *     DETAIL --> HERO["identity hero"]
 *     DETAIL --> PHOTOS["AssetBrowser"]
 *     HERO --> MODAL["PersonRenameModal"]
 *     MODAL --> FACES["PersonFacesPanel"]
 *     MODAL --> PICKER["PersonPicker"]
 *     FACES --> CORRECT["cover / move / remove"]
 *     PICKER --> MERGE["merge people"]
 * ```
 *
 * {@link PersonDetails} renders a collection hero and a person-constrained
 * {@link AssetBrowser}. The gallery contains photos, not face crops.
 * {@link PersonRenameModal} owns identity info, hidden state, face correction,
 * and merge tabs. {@link PersonFacesPanel} works on face crops and
 * {@link PersonPicker} selects move/merge targets.
 *
 * ## Data
 *
 * {@link usePeople} lists summaries and exposes the optional hidden-person
 * view. {@link usePersonDetails} and {@link usePersonFaces} load one identity
 * and its face memberships. {@link useSetPersonCover},
 * {@link useMoveFace}, {@link useRemoveFaceFromPerson},
 * {@link useMergePeople}, and {@link useSetPersonHidden} own correction
 * mutations and invalidation.
 *
 * Moves and merges preserve manual assignments so a later clustering rebuild
 * cannot discard user corrections. Removing a face detaches its membership;
 * original media is never modified. The root `index.ts` exports only list,
 * rebuild, and summary contracts needed by Collections and Manage.
 *
 * @module
 */
import type {
  useMergePeople,
  useMoveFace,
  usePeople,
  usePersonDetails,
  usePersonFaces,
  useRemoveFaceFromPerson,
  useSetPersonCover,
  useSetPersonHidden,
} from "./api/usePeople.ts";
import type PersonDetails from "./flows/detail/PersonDetailsFlow.tsx";
import type PersonFacesPanel from "./flows/detail/PersonFacesPanel.tsx";
import type PersonPicker from "./flows/detail/PersonPicker.tsx";
import type PersonRenameModal from "./flows/detail/PersonRenameModal.tsx";
import type { AssetBrowser } from "../assets/index.ts";

export {};
