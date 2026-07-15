# People

Recognized people — face clusters surfaced as named, browsable identities —
and the correction tools that keep them honest. The `collections` feature
owns the people *rail/grid* entry point; this feature owns person **detail**
and every identity correction. AI grouping is assistive; the user is the
authority for who is who.

## Ownership

A person is owner-scoped but **not** repository-scoped: clusters span
repositories, so only the list read follows the browse scope.
[usePeople](./hooks/usePeople.ts) lists people under the current browse scope (with an
`includeHidden` toggle that switches the grid from visible-only to all
people); [usePersonDetails](./hooks/usePeople.ts) loads one person plus its rename, with no
repository filter — as do the face list and every mutation below.
[PersonDetails](./routes/PersonDetails.tsx) is the detail route: a [CollectionHero](@/components/collection) with an
edit action that opens [PersonRenameModal](./components/PersonRenameModal.tsx), and the person's asset
gallery rendered through [AssetsGalleryPage](@/features/assets/components/page/AssetsGalleryPage.tsx).

## Corrections

Two distinct surfaces sit on the same person:

- **Asset gallery** — the person's *photos*, scoped by `{ person_id }`. This
  is the browsing surface and does not render face crops as gallery content.
- **Edit modal** — [PersonRenameModal](./components/PersonRenameModal.tsx) owns tabbed identity management:
  info/name/hidden state, face corrections through [PersonFacesPanel](./components/PersonFacesPanel.tsx)
  over [usePersonFaces](./hooks/usePeople.ts), and merge via [PersonPicker](./components/PersonPicker.tsx).

The face-level operations:

- [useSetPersonCover](./hooks/usePeople.ts) — promote one face to the representative cover
  from that face's non-selection-mode menu.
- [useMoveFace](./hooks/usePeople.ts) — reassign selected faces to another person
  ([PersonPicker](./components/PersonPicker.tsx) picks the target). Each reassignment becomes a manual
  correction. Photos and other faces in the same asset are unchanged.
- [useRemoveFaceFromPerson](./hooks/usePeople.ts) — detach selected faces, leaving them
  unclustered for a later rebuild. The original assets are never modified.
- [useMergePeople](./hooks/usePeople.ts) — fold source people into the target from the merge
  tab. Source clusters are emptied and removed; the target
  name/confirmation/hidden state survive.
- [useSetPersonHidden](./hooks/usePeople.ts) — hide a person from the default grid via a
  normal switch in the info tab. Faces, assets and names are preserved.

## Decisions

Corrections are durable: moves and merges mark membership `is_manual`, and a
full cluster rebuild replays manual assignments so user corrections are not
discarded. Editing is modal-only, matching the collections mental model and
keeping the gallery focused on photos. Face crops are served per-face (not
full thumbnails) so the correction grid shows exactly what the model grouped.
