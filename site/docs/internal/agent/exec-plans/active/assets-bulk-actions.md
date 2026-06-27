# Assets Bulk Actions Context Refactor

## Context

`AssetsPageHeader` is shared by the main assets page, album details, people
details, trip/place details, utility classifier collections, and `PhotoPicker`.
The component currently owns a fixed selection-mode bulk menu:

- rating update
- liked/unliked update
- add to album
- download selected
- delete selected assets

That default works for the main `/assets` view, but scoped views need different
semantics. Album details is the immediate example: selected assets should be
removable from the current album without deleting the original media. Other
scoped views may later need their own curation actions, such as "not this
person" for people pages or "exclude from this smart collection" for classifier
views.

Keep the shared header, but move domain-specific bulk behavior out of the
header's hard-coded menu. The header should render default actions plus
page-provided action definitions, and pages should decide which defaults are
available in their context.

Current implementation scope: checklist sections 1-7 plus 3a are complete.
Section 8 is a follow-up only; all stack selection behavior remains
collapsed-mode and whole-stack for now.

## Current Ownership

- `AssetsProvider` creates one scoped Zustand store per asset surface. Selection
  state is isolated per provider scope, not global.
- `AssetsPageHeader` reads the current provider's selection/filter state and
  receives `browseItems` so it can resolve selected browse item ids to real asset
  ids.
- `useBulkAssetOperations` owns default action execution for selected asset ids.
- Album details uses `AssetsGalleryPage` with `baseFilter={{ album_id }}`, so the
  header only sees a filtered asset view and does not know it is operating inside
  album membership.

## Goals

1. Support context-specific bulk action combinations without route-name checks
   inside `AssetsPageHeader`.
2. Add "Remove from current album" on album details using the existing
   `DELETE /api/v1/albums/{id}/assets/{assetId}` endpoint.
3. Let scoped pages hide or reorder default actions where the default semantics
   are dangerous or noisy.
4. Preserve current collapsed-stack selection behavior: selected stack browse
   rows still resolve to affected asset ids with `stackMode: "whole-stack"`.
5. Define a future expanded-stack mode where every visible row is an individual
   asset and selection no longer implies the whole stack.
6. Keep API contracts generated and typed; no hand-written endpoint types or
   response casts.
7. Prepare the same contextual action surface for
   [assets-feature-review.md](assets-feature-review.md) F4: ordinary library
   delete will become "Move to Trash", while the future Trash view will expose
   "Restore" through page-provided bulk actions instead of header route checks.

## Proposed API

Add a small bulk action contract in the assets feature, close to
`AssetsPageHeader` or a new `bulkActions.ts` helper:

```ts
export type AssetsBulkActionId =
  | "set-rating"
  | "set-liked"
  | "add-to-album"
  | "download"
  | "delete-assets"
  | string;

export type AssetsBulkActionTone = "default" | "info" | "danger";

export interface AssetsBulkActionContext {
  selectedItemCount: number;
  affectedAssetCount: number;
  selectedAssetIds: string[];
  selectedAssets: Asset[];
  clearSelection: () => void;
}

export interface AssetsBulkActionItem {
  id: AssetsBulkActionId;
  label: string;
  icon?: ReactNode;
  tone?: AssetsBulkActionTone;
  disabled?: boolean;
  requiresConfirmation?: boolean;
  confirmationTitle?: string;
  confirmationMessage?: string;
  onRun: (context: AssetsBulkActionContext) => Promise<void> | void;
}
```

Expose it through the shared components:

```ts
interface AssetsPageHeaderProps {
  // existing props...
  bulkActions?: AssetsBulkActionItem[] | ((context: AssetsBulkActionContext) => AssetsBulkActionItem[]);
  hiddenBulkActions?: readonly AssetsBulkActionId[];
}

interface AssetsGalleryPageProps {
  // existing props...
  bulkActions?: AssetsPageHeaderProps["bulkActions"];
  hiddenBulkActions?: readonly AssetsBulkActionId[];
}
```

The exact type names can change during implementation, but keep the shape:
header computes selection context once, default actions are explicit ids, and
pages pass additive/custom actions plus hidden default ids.

## Action Composition Rules

Default actions remain available unless hidden:

- `set-rating`
- `set-liked`
- `add-to-album`
- `download`
- `delete-assets`

Page-provided actions are appended after metadata actions and before destructive
asset deletion unless the implementation chooses a more explicit slot API.
Danger actions should use `tone: "danger"` and a confirmation modal.

`delete-assets` should remain an asset delete operation. Do not overload it for
"remove from album". In album contexts, prefer hiding `delete-assets` or visually
separating it below "Remove from current album" so users do not confuse media
deletion with membership removal.

F4 in [assets-feature-review.md](assets-feature-review.md) now treats
`delete-assets` as the default "Move to Trash" action. The Trash page hides
`delete-assets` and provides a page-specific `restore-assets` action. Permanent
delete is intentionally out of scope for this plan and this milestone.

## Stack Display And Selection Rules

Current frontend behavior is collapsed-stack browsing. `useAssetsView` and asset
search both request `stack_mode: "collapsed"`, so gallery rows are either
standalone assets or stack browse items. Regular stack expansion currently exists
as `StackCarouselOverlay`, which opens a stack-specific full-screen carousel for
viewing members; it does not change gallery selection state. Live Photo stacks
are handled inside the main `MediaViewer` by loading stack members only for
motion playback.

Keep the immediate bulk-action refactor compatible with this current model:

```text
Collapsed mode = stack as one browse item
Selecting a stack row = whole stack
Bulk actions on a selected stack row = affect all stack members
StackCarouselOverlay = inspect members only, not gallery/member selection
```

Expanded mode should be introduced as a separate view mode, not by mixing group
selection and member selection inside one grid. Target rule:

```text
Expanded mode = every visible row is an asset
Selecting a stack member = selected asset only
Bulk actions in expanded mode = affect selected assets only
Mode switch between collapsed/expanded clears selection
```

Avoid a first version where a grid contains both selectable stack group wrappers
and selectable stack members. That creates two selection units in the same
surface and makes bulk action impact hard to explain.

Cross-plan link: [assets-feature-review.md](assets-feature-review.md) F3 tracks
the missing stack create/unstack capability. This bulk-action plan owns the
multi-selection entry point for "Stack selected" and the selection impact rules;
F3 owns the typed stack mutations plus the "Remove from stack" affordance in
stack detail/overlay surfaces. Implementing either plan should update the other
if the stack UX contract changes.

## Immediate Implementation Checklist

### 1. Extract Default Bulk Action Model — ✅ DONE

- [x] Move fixed menu action definitions out of inline JSX where practical.
- [x] Keep the existing rating/liked nested menus, but represent their root ids
      as `set-rating` and `set-liked` for hiding/composition.
- [x] Build `AssetsBulkActionContext` from the values already computed in
      `AssetsPageHeader`:
      - `selectedItemCount`
      - `affectedAssetCount`
      - `resolvedSelectedAssetIds`
      - `selectedAssets`
      - `selection.clear`
- [x] Preserve existing confirmation flows for rating/liked/delete.
- [x] Preserve existing add-to-album modal and download behavior.

### 2. Add Header And Gallery Extension Props — ✅ DONE

- [x] Add `bulkActions` and `hiddenBulkActions` to `AssetsPageHeaderProps`.
- [x] Add matching props to `AssetsGalleryPageProps`.
- [x] Pass the props from `AssetsGalleryPage` into `AssetsPageHeader`.
- [x] Ensure direct users of `AssetsPageHeader` can also pass the same props:
      people details, trip details, and `PhotoPicker`.

### 3. Album Details: Remove From Current Album — ✅ DONE

- [x] In `AlbumDetails`, create a typed `$api.useMutation("delete",
      "/api/v1/albums/{id}/assets/{assetId}")`.
- [x] Pass a custom bulk action to `AssetsGalleryPage`:
      - id: `remove-from-current-album`
      - label: "Remove from this album"
      - icon: `FolderMinus`
      - tone: `danger`
      - confirmation message uses selected item / affected asset counts
      - `onRun` calls the delete mutation for every selected asset id
- [x] On success:
      - clear selection
      - invalidate/refetch the current album asset query
      - invalidate/refetch album metadata so `asset_count` updates
      - show success feedback through existing message/toast infrastructure
- [x] On error:
      - show an error message
      - keep selection intact so the user can retry
- [x] Hide or demote `delete-assets` on album pages. Default starting point:
      `hiddenBulkActions={["delete-assets"]}`.

### 3a. Stack Selected Integration With F3 — ✅ DONE

This item belongs to the bulk-action refactor but completes the selection-toolbar
half of [assets-feature-review.md](assets-feature-review.md) F3.

- [x] Add a custom or default-able `stack-selected` bulk action that is enabled
      only when at least two affected asset ids are selected.
- [x] Run it through the F3 stack mutation hook (`createStack(assetIds)`) rather
      than calling raw client code from `AssetsPageHeader`.
- [x] In collapsed mode, preserve the current whole-stack selection behavior:
      selecting a stack browse row passes all stack member ids into
      `createStack`, so the action text/confirmation must make the affected asset
      count visible.
- [x] Invalidate/refetch the same asset-list and stack-detail queries described
      by F3 after a stack is created.
- [x] Keep "Remove from stack" in the stack detail/overlay flow, not as a generic
      bulk action, unless expanded stack mode later exposes individual member
      rows in the gallery.

### 4. PhotoPicker Bulk Menu Policy — ✅ DONE

- [x] Hide bulk actions that mutate library state:
      `set-rating`, `set-liked`, `add-to-album`, `delete-assets`.
- [x] Consider hiding `download` too unless there is a concrete picker use case.
- [x] Keep `defaultSelectionMode="single"` and the existing auto-select behavior.
- [x] Verify the header still supports filter/sort controls needed by the picker.

### 5. Other Scoped Surfaces — ✅ DONE

- [x] People details: leave defaults for now, but the new API should make it easy
      to add a future `remove-from-person` / `not-this-person` action.
- [x] Trip/place details: leave defaults for now; this is a derived location/date
      view, not explicit membership.
- [x] Utility classifier collections: leave defaults for now; future tag
      mutation can add `remove-classifier-tag` or `exclude-from-smart-collection`.
- [x] Main assets page: keep the full default action set unchanged, plus the new
      collapsed-mode `stack-selected` action from F3.

### 6. i18n — ✅ DONE

- [x] Add extractable strings for new action labels, confirmation title/message,
      success, and error states.
- [x] Run i18n extraction/status if strings are added:
      `cd web && vp exec i18next-cli extract && vp exec i18next-cli status`.
- [x] Do not leave new user-facing English literals outside the i18n layer.

### 7. Tests — ✅ DONE

- [x] Add or update focused tests for `AssetsPageHeader` action composition:
      hidden defaults, custom action rendering, custom action invocation, and
      confirmation behavior.
- [x] Add an album-specific test that verifies "Remove from this album" invokes
      the album remove mutation and hides asset delete.
- [x] Update `PhotoPicker` tests if hidden bulk action props affect mocked
      `AssetsPageHeader` expectations.
- [x] Keep browse item / stack resolution tests unchanged unless the context
      builder changes behavior; whole-stack selection should remain covered.

### 8. Stack Expanded Mode Follow-Up

This can ship after the contextual bulk-action refactor. Do not block album
removal or Trash/Restore on full expanded-mode work. The current milestone
explicitly keeps stacks collapsed; section 8 is not part of the 1-7 completion
scope.

- [ ] Add a view-level `stackMode: "collapsed" | "expanded"` state in the assets
      UI slice or route-level gallery state.
- [ ] Pass `stackMode` into `useCurrentAssetsView` and
      `useCurrentAssetsSearchView` instead of hard-coding `stack_mode:
      "collapsed"`.
- [ ] Include `stackMode` in `generateViewKey` inputs so collapsed and expanded
      query caches do not overlap.
- [ ] When switching stack mode, clear selection to avoid mixing `stack:*` and
      `asset:*` browse ids.
- [ ] In expanded mode, render stack members as ordinary `asset:*` browse rows
      with a visual stack/member badge. Do not render a selectable stack group
      wrapper in the same grid.
- [ ] In expanded mode, make `AssetsBulkActionContext.selectedAssetIds` come
      directly from selected asset rows; do not apply whole-stack expansion.
- [ ] Add an explicit member-row affordance for stack context actions, such as
      "View stack", "Select whole stack", or "Collapse stack", after the base
      mode is stable.
- [ ] Keep `StackCarouselOverlay` as an inspect-only path for collapsed mode. If
      actions are later added inside the overlay, default them to the current
      member unless the action text explicitly says "Apply to stack".
- [ ] Add tests for mode switching, selection clearing, and expanded-mode bulk
      context resolving only selected member assets.

## Affected Files

Expected frontend files:

- `web/src/features/assets/components/shared/AssetsPageHeader.tsx`
- `web/src/features/assets/components/page/AssetsGalleryPage.tsx`
- `web/src/features/assets/hooks/useSelection.tsx`
- `web/src/features/assets/hooks/useAssetsView.tsx`
- `web/src/features/assets/types/assets.type.ts` or a new
  `web/src/features/assets/components/shared/bulkActions.ts`
- `web/src/features/assets/slices/ui.slice.ts` if expanded mode is persisted in
  assets UI state
- `web/src/features/collections/routes/AlbumDetails.tsx`
- `web/src/components/PhotoPicker.tsx`
- tests near the touched components
- locale resources generated/updated through the i18n workflow

No backend implementation should be required for the first album action because
the album remove endpoint already exists. If generated frontend types are stale,
fix OpenAPI annotations/DTOs and run `make dto`; do not cast around `$api`.

## Validation

Run the frontend gate:

```bash
make web-test
```

If intentionally scoped to `web/`, the equivalent is:

```bash
cd web && vp check --no-fmt --no-lint && vp lint && vp test
```

Manual checks:

- Main `/assets`: selection mode shows the same default actions as before.
- Album details: selection mode shows "Remove from this album"; running it
  removes membership, updates the album count, and does not delete original
  assets.
- Album details: asset delete is hidden or clearly separated from album removal.
- PhotoPicker: selecting a photo still calls `onSelect`; mutating bulk actions
  are not exposed.
- Stack selection: selecting a stack still affects all member assets where the
  action is defined as whole-stack.
- Expanded mode follow-up: switching between collapsed and expanded clears
  selection; expanded-mode selection affects selected asset rows only.
- Mobile compact menu and desktop action menu expose the same contextual action
  set.

## Open Questions

- Should album pages keep a secondary "Delete original assets" escape hatch, or
  should asset deletion be reachable only from the main assets page and asset
  detail actions?
- Should action ordering be a simple append model or a slot model
  (`metadata`, `organize`, `export`, `danger`) before implementation?
- Should custom actions be allowed to declare `selectionResolveMode` in the
  future (`visible` vs `whole-stack`), or is the current whole-stack behavior the
  right default for all bulk actions?
- Should expanded stack mode be a global asset-page preference, a per-route
  session state, or a one-off toolbar toggle that resets on navigation?
