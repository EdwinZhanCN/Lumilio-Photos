# Responsive Adaptation (All TSX)

## Goal

Make every page and component in `web/src` adapt cleanly across screen sizes —
phone (~375px), tablet (~768px), laptop (~1024px), and wide desktop (≥1440px) —
with no horizontal page scroll, no clipped controls, and touch-friendly targets
on small screens.

Coverage target: **all 192 non-test `.tsx` files** under `web/src`. Every file
is either adapted or explicitly audited as "no visual surface / already fine"
and checked off in the tracker below.

## Ground Rules

- **Load skills first.** Each implementation session MUST invoke the
  **frontend skill** and the **daisyUI skill** (via the Skill tool) before
  writing any UI code, and follow their guidance for layout and component
  choices. If a skill is unavailable in the environment, note that in the
  session summary and proceed with the principles below.
- Stack: Tailwind CSS 4 + daisyUI 5, React 19. Use **pure daisyUI semantic
  tokens** (`bg-base-100`, `text-base-content`, `primary/secondary/accent`) —
  no custom color tokens.
- **Mobile-first**: base styles target the smallest screen; add `sm:`/`md:`/
  `lg:`/`xl:` upward. Prefer CSS (responsive utilities, `grid`
  `auto-fill/minmax`, flex-wrap, container queries `@container` for
  self-contained components) over JS breakpoint logic.
- Prefer daisyUI responsive primitives: `drawer` (sidebar collapse), `dock`
  (mobile bottom nav if needed), `modal-bottom sm:modal-middle` (modals become
  bottom sheets on phones), `menu` horizontal/vertical switches, responsive
  `join`/`btn` sizing.
- When JS is unavoidable (virtualized galleries, canvas, maps), add ONE shared
  hook `web/src/hooks/util-hooks/useBreakpoint.ts` (matchMedia-based, aligned
  to Tailwind breakpoints) and reuse it everywhere. Do not scatter ad-hoc
  `window.innerWidth` reads.
- Touch targets ≥ 44px on `<lg`; hover-only affordances need a
  touch-visible equivalent (e.g. overlay actions on thumbnails).
- Never run bare `vp fmt`. Never hand-edit `translation.json` (if copy is
  added: `t()` → `vp exec i18next-cli extract` → fill zh). Never hand-edit
  generated files (`doc.md`, `schema.d.ts`, `src/wasm/**`).

## Non-Goals

- No visual redesign — same look, adapted layout only.
- No new mobile-only routes or a separate mobile app shell rewrite.
- No dependency/toolchain changes (Vite+ stays pinned).
- No backend changes.

## Current State

- 46 of 192 files use responsive prefixes; the rest are fixed-desktop layouts.
- Batch 0 done: App shell now uses a daisyUI `drawer lg:drawer-open` — sidebar
  is `drawer-side` (mobile overlay, permanent rail on `lg`+), NavBar has the
  `lg:hidden` hamburger trigger (`label[for=app-drawer]`).
- `hooks/util-hooks/useBreakpoint.ts` exists (`useBreakpoint(bp)` /
  `useIsMobile()`, matchMedia-based, Tailwind default breakpoints) — reuse
  this in later batches instead of ad-hoc `window.innerWidth`.
- Galleries (`JustifiedGallery`, `SquareGallery`) compute layout from container
  width via `lib/utils/smartBatchSizing.ts` — verify rather than rewrite.

## Workflow (per batch)

1. Load frontend + daisyUI skills.
2. Read each file in the batch; classify: **adapt** / **already responsive** /
   **no visual surface** (providers, hooks without JSX layout, contexts).
3. Apply fixes with Tailwind responsive utilities + daisyUI primitives.
4. Verify visually in the browser (Chrome tools: `resize_window` or devtools
   device emulation) at **375 / 768 / 1024 / 1440** widths against `make web-dev`.
   Checks: no horizontal body scroll, no overlapping/clipped controls, modals
   and menus reachable, galleries reflow.
5. Run `make web-test` (or `cd web && vp check --no-fmt --no-lint && vp lint && vp test`).
6. Tick the tracker checkboxes in this file, commit per batch:
   `refactor(web): responsive — <batch name>`.

Batches are ordered so shared shells/components land first; feature batches can
then rely on them. One batch ≈ one session/PR.

## Batch 0 — Shell & Foundation

The drawer/nav shell decision here shapes everything else. Sidebar should
collapse into a daisyUI `drawer` below `lg`; NavBar gets a hamburger trigger.
Add `useBreakpoint` hook here.

- [x] App.tsx — shell rebuilt on daisyUI `drawer lg:drawer-open`; sidebar is
      `drawer-side` (overlay + auto-close on nav below `lg`), navbar stays in
      `drawer-content`.
- [x] components/NavBar.tsx — hamburger `label[for=app-drawer]` shown `lg:hidden`;
      logo wordmark hides below `sm`; gaps tighten on small screens.
- [x] components/SideBar.tsx — menu content reused for both the `lg:drawer-open`
      rail and the mobile drawer; links close the drawer on navigate.
- [x] components/PageHeader.tsx — header wraps (`flex-wrap`), title/subtitle
      shrink on `<sm`, action children wrap and right-align.
- [x] components/Modal.tsx — `modal-bottom sm:modal-middle` (bottom sheet on
      phones), header/footer padding tightens below `sm`.
- [x] routes/routes.tsx — no visual surface (route/element data only).
- [x] main.tsx — no visual surface (bootstrap only).
- [x] lib/i18n.tsx — no visual surface (provider only).
- [x] lib/theme/ThemeEffects.tsx — no visual surface (effect only, no JSX).
- [x] contexts/GlobalContext.tsx — no visual surface (provider only).
- [x] contexts/WorkerProvider.tsx — no visual surface (provider only).
- [x] hooks/util-hooks/useExportImage.tsx — no visual surface (hook only).
- [x] hooks/util-hooks/useGenerateHashcode.tsx — no visual surface (hook only).
- [x] hooks/util-hooks/useMessage.tsx — no visual surface (hook only).

## Batch 1 — Shared Components

- [x] components/breadcrumbs/BreadcrumbContext.tsx — no visual surface (context only).
- [x] components/breadcrumbs/Breadcrumbs.tsx — trail scrolls horizontally
      (`overflow-x-auto`) instead of wrapping/clipping on narrow screens.
- [x] components/BrowseScopeSelect.tsx — already responsive (bounded width select).
- [x] components/collection/CollectionErrorAlert.tsx — already responsive.
- [x] components/collection/CollectionHero.tsx — already responsive
      (`flex-col lg:flex-row`); depends on CollectionTitle/MetaStatRow fixes below.
- [x] components/collection/CollectionTitle.tsx — title scales
      `text-2xl sm:text-3xl lg:text-4xl` instead of fixed `text-4xl`.
- [x] components/collection/LoadMoreButton.tsx — already responsive.
- [x] components/collection/MetaStatRow.tsx — stat strip wraps (`flex-wrap`)
      instead of overflowing when there are many stats on a narrow screen.
- [x] components/EmptyState.tsx — padding tightens below `sm`.
- [x] components/ErrorFallBack.tsx — already responsive (`text-3xl sm:text-4xl`,
      `flex-wrap` actions).
- [x] components/ExifDataDisplay.tsx — already responsive.
- [x] components/ExportModal.tsx — action icon row wraps (`flex-wrap`) instead
      of overflowing on narrow modal widths.
- [x] components/Heatmap/CalendarHeatmap.tsx — wrapper scrolls horizontally
      (`overflow-x-auto`) instead of causing page overflow; grid itself stays
      fixed-cell by design (calendar heatmap convention).
- [x] components/Heatmap/GitHubStyleHeatmap.tsx — same horizontal-scroll fix.
- [x] components/Heatmap/Histogram.tsx — already responsive (SVG `viewBox` +
      100% width).
- [x] components/icons/LensIcon.tsx — no visual surface (icon primitive).
- [x] components/icons/LivePhotos.tsx — no visual surface (icon primitive).
- [x] components/MapComponent.tsx — already responsive (sizes from container
      height/width props; Leaflet handles its own resize).
- [x] components/MessageCenter.tsx — already responsive (dropdown clamps to
      `max-w-[calc(100vw-2rem)]`).
- [x] components/Notifications.tsx — no visual surface (renders `<Toaster/>`).
- [x] components/PhotoMapView.tsx — already responsive
      (`grid-cols-4 sm:grid-cols-6 md:grid-cols-8`, toggle labels hide `<sm`).
- [x] components/PhotoPicker.tsx — already responsive (flex column, scrollable body).
- [x] components/ui/InlineTextEditor.tsx — already responsive (`min-w-0`, wraps).
- [x] components/ui/RatingComponent.tsx — already responsive (compact, no fixed widths).
- [x] components/ui/Sonner.tsx — already responsive (Sonner handles its own
      mobile layout).
- [x] components/UserAvatar.tsx — already responsive (size fully caller-controlled).

## Batch 2 — Assets (gallery core)

Highest-traffic surface. Watch: gallery virtualization width math,
full-screen carousel + info panel (info should become a bottom
sheet/drawer on phones), selection toolbar overflow on narrow widths.

- [x] features/assets/AssetsProvider.tsx — no visual surface (provider only).
- [x] features/assets/components/page/AssetsGalleryPage.tsx — already responsive
      (delegates layout entirely to header/gallery/carousel sub-components).
- [x] features/assets/components/page/FilterTool/FilterTool.tsx — dropdown
      panel clamps (`max-w-[calc(100vw-2rem)]`) instead of overflowing off the
      right edge on narrow screens.
- [x] features/assets/components/page/FullScreen/FullScreenCarousel/FullScreenCarousel.tsx —
      already responsive (field-guide aside sizes off viewport, FAB flower is
      daisyUI-driven).
- [x] features/assets/components/page/FullScreen/FullScreenInfo/AudioInfoView.tsx —
      info panel becomes a bottom sheet (`fixed inset-x-0 bottom-0`) below
      `sm`, floating top-right card at `sm:`+ (was a fixed `w-[380px]`
      absolute panel that overflowed phone widths).
- [x] features/assets/components/page/FullScreen/FullScreenInfo/FullScreenBasicInfo.tsx —
      fallback generic-type card now clamps to viewport width.
- [x] features/assets/components/page/FullScreen/FullScreenInfo/PhotoInfoView.tsx —
      same bottom-sheet-on-mobile fix as AudioInfoView.
- [x] features/assets/components/page/FullScreen/FullScreenInfo/TagList.tsx —
      already responsive (horizontal-scroll tag row, viewport-clamped portal popover).
- [x] features/assets/components/page/FullScreen/FullScreenInfo/VideoInfoView.tsx —
      same bottom-sheet-on-mobile fix as AudioInfoView.
- [x] features/assets/components/page/JustifiedGallery/JustifiedGallery.tsx —
      verified: layout is fully container-width-driven via ResizeObserver, no
      rewrite needed.
- [x] features/assets/components/page/LoadingSkeleton.tsx — already responsive.
- [x] features/assets/components/page/SearchFAB.tsx — search input clamps
      (`max-w-[calc(100vw-6rem)]`) instead of nearly touching the viewport
      edge on the smallest phones.
- [x] features/assets/components/page/SquareGallery/SquareGallery.tsx —
      verified: already `grid-cols-2` below `md`, custom column count at `md:`+.
- [x] features/assets/components/shared/AssetsPageHeader.tsx — already
      responsive (full `lg:` toolbar vs. compact `Ellipsis` dropdown menu below `lg`).
- [x] features/assets/components/shared/MediaThumbnail.tsx — already responsive
      (fills parent grid cell).
- [x] features/assets/components/shared/MediaViewer.tsx — already responsive
      (`calc(100vw/100vh)`-bounded media).
- [x] features/assets/components/shared/StackCarouselOverlay.tsx — already responsive.
- [x] features/assets/components/shared/StackDetailModal.tsx — already
      responsive (`sm:` padding, `md:`/`xl:` grid columns).
- [x] features/assets/components/shared/StackedThumbnail.tsx — already responsive.
- [x] features/assets/components/shared/TagPickerMenu.tsx — already responsive.
- [x] features/assets/hooks/useAssetActions.tsx — no visual surface.
- [x] features/assets/hooks/useAssetsView.tsx — no visual surface.
- [x] features/assets/hooks/useAssetTags.tsx — no visual surface.
- [x] features/assets/hooks/usePinAssetsView.tsx — no visual surface.
- [x] features/assets/hooks/useSelection.tsx — no visual surface.
- [x] features/assets/routes/Assets.tsx — no visual surface (delegates to
      AssetsGalleryPage).
- [x] features/assets/routes/AssetsTrash.tsx — no visual surface (delegates to
      AssetsGalleryPage).

## Batch 3 — Collections

Rails (`Rail`/`RailCard`) are shared — fix once, all rails inherit. Grids
should use `auto-fill/minmax` instead of fixed column counts.

- [x] features/collections/CollectionsProvider.tsx — no visual surface (reducer provider).
- [x] features/collections/components/AlbumFormModal.tsx — already responsive
      (Modal's `modal-bottom sm:modal-middle`, single-col below `md`).
- [x] features/collections/components/AlbumRail.tsx — already responsive (Rail/RailCard).
- [x] features/collections/components/FoldersRail.tsx — already responsive.
- [x] features/collections/components/ImgStackGrid/ImgStackGrid.tsx — grid
      already responsive; wraps the fixed-size stack view in a `w-full flex
      justify-center` cell so ImgStackView can size to the column.
- [x] features/collections/components/ImgStackView/ImgStackView.tsx — was a
      hardcoded `size-50` (200px) that overflowed the `grid-cols-2` mobile
      column; now `w-full max-w-50 aspect-square`.
- [x] features/collections/components/MapRail.tsx — already responsive.
- [x] features/collections/components/PeopleCollectionGrid.tsx — already responsive.
- [x] features/collections/components/PeopleRail.tsx — already responsive.
- [x] features/collections/components/Rail.tsx — already responsive (horizontal scroll row).
- [x] features/collections/components/RailCard.tsx — already responsive.
- [x] features/collections/components/UtilitiesRail.tsx — already responsive.
- [x] features/collections/routes/AlbumDetails.tsx — already responsive (delegates to CollectionHero/AssetsGalleryPage).
- [x] features/collections/routes/Albums.tsx — already responsive.
- [x] features/collections/routes/Collections.tsx — section headers wrap
      (`flex-wrap`) and title scales `text-xl sm:text-2xl` instead of a fixed
      size that could crowd the "View all" button on narrow screens.
- [x] features/collections/routes/Duplicates.tsx — already responsive
      (`grid-cols-2 sm:grid-cols-4` summary, `grid-cols-2 ... lg:grid-cols-5` thumbnails).
- [x] features/collections/routes/FolderDetails.tsx — already responsive.
- [x] features/collections/routes/Folders.tsx — already responsive.
- [x] features/collections/routes/Liked.tsx — already responsive (delegates to AssetsGalleryPage).
- [x] features/collections/routes/MapView.tsx — already responsive (map fills container).
- [x] features/collections/routes/People.tsx — already responsive.
- [x] features/collections/routes/TagDetails.tsx — already responsive.
- [x] features/collections/routes/Tags.tsx — already responsive.
- [x] features/collections/routes/TripDetails.tsx — already responsive.
- [x] features/collections/routes/Utilities.tsx — already responsive.
- [x] features/collections/routes/UtilityClassifierAlbum.tsx — already responsive.

## Batch 4 — Auth & Onboarding

Centered-card pages; verify small-phone heights (keyboard overlap) and wide
screens. Includes MFA/passkey flows.

- [x] features/auth/AuthProvider.tsx — no visual surface (reducer provider).
- [x] features/auth/components/BootstrapGate.tsx — `min-h-screen` → `min-h-dvh`
      (correct height on mobile browsers with dynamic chrome/keyboard).
- [x] features/auth/components/PrimaryRepositoryGate.tsx — same `min-h-dvh` fix;
      form grid already `sm:grid-cols-2`.
- [x] features/auth/components/ProtectedRoute.tsx — same `min-h-dvh` fix.
- [x] features/auth/components/RegistrationForm.tsx — same `min-h-dvh` fix;
      already responsive otherwise (AuthShell caps width, wraps).
- [x] features/auth/components/SetupGate.tsx — same `min-h-dvh` fix.
- [x] features/auth/components/ui.tsx — already responsive (`AuthShell` uses
      `w-full` + `maxWidth`, `p-7 sm:p-8`); the whole auth kit was already
      built mobile-first.
- [x] features/auth/routes/BootstrapWizard.tsx — same `min-h-dvh` fix;
      already responsive (sidebar `hidden md:flex`, dedicated mobile stepper).
- [x] features/auth/routes/ChangePasswordPage.tsx — same `min-h-dvh` fix;
      already responsive (`p-8 sm:p-10`, `text-3xl sm:text-4xl`).
- [x] features/auth/routes/LoginPage.tsx — same `min-h-dvh` fix.
- [x] features/auth/routes/MFAPage.tsx — same `min-h-dvh` fix.
- [x] features/auth/routes/RegisterPage.tsx — no visual surface (thin wrapper
      around RegistrationForm).

## Batch 5 — Home, Settings, People, Manage, Updates, Portfolio

Settings uses the renew (macOS inset-grouped) shell — keep its design
language; the shell likely needs a single-column collapse below `md`.

- [x] features/home/components/GalleryGrid.tsx — already responsive (delegates to SquareGallery).
- [x] features/home/components/SpacetimeMapCard.tsx — already responsive.
- [x] features/home/components/StatsCards.tsx — already responsive
      (`grid-cols-1 md:grid-cols-2`, horizontal-scroll heatmap).
- [x] features/home/routes/Home.tsx — already responsive (tabs wrap via daisyUI `tabs-box`).
- [x] features/settings/components/renew/SettingsDropdown.tsx — already responsive.
- [x] features/settings/components/renew/SettingsGroup.tsx — already responsive
      (mobile stacks, `lg:grid-cols-[...]` two-column on desktop).
- [x] features/settings/components/renew/SettingsPage.tsx — already responsive.
- [x] features/settings/components/renew/SettingsSaveBar.tsx — already
      responsive (`sm:flex-row`).
- [x] features/settings/components/renew/SettingsShell.tsx — already
      responsive (tab strip `flex-wrap`).
- [x] features/settings/components/renew/tabs/AccountTab.tsx — already
      responsive (`modal-bottom sm:modal-middle`, `flex-wrap`).
- [x] features/settings/components/renew/tabs/AiTab.tsx — already responsive.
- [x] features/settings/components/renew/tabs/AppearanceTab.tsx — already responsive.
- [x] features/settings/components/renew/tabs/CloudTab.tsx — already responsive.
- [x] features/settings/components/renew/tabs/ServerTab.tsx — already responsive.
- [x] features/settings/components/renew/tabs/UsersTab.tsx — already responsive.
- [x] features/settings/components/renew/ThemePicker.tsx — already responsive
      (`grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5`).
- [x] features/settings/preferencesEffects.tsx — no visual surface (effect only).
- [x] features/settings/routes/Settings.tsx — no visual surface (thin wrapper).
- [x] features/people/components/PersonFacesPanel.tsx — already responsive
      (`grid-cols-4 sm:grid-cols-5 md:grid-cols-7 lg:grid-cols-8`).
- [x] features/people/components/PersonPicker.tsx — already responsive.
- [x] features/people/components/PersonRenameModal.tsx — already responsive
      (Modal's bottom-sheet-on-mobile, `flex-col sm:flex-row`).
- [x] features/people/routes/PersonDetails.tsx — already responsive.
- [x] features/manage/components/RepositoryGrid.tsx — header wraps
      (`flex-wrap`, `min-w-0`) instead of squeezing title against action buttons.
- [x] features/manage/routes/Manage.tsx — dropped a stray `min-h-screen` on a
      route already nested in the app shell's own scroll container (was
      inflating the scrollable area beyond the content's real height).
- [x] features/updates/routes/Updates.tsx — no visual surface (PageHeader only, placeholder route).
- [x] features/portfolio/routes/Portfolio.tsx — no visual surface (PageHeader only, placeholder route).

## Batch 6 — Upload, Share, Monitor

PublicShare is unauthenticated and mobile-heavy (shared links get opened on
phones) — treat it as a first-class mobile surface.

- [ ] features/upload/components/FileDropZone.tsx
- [ ] features/upload/components/NavbarUploadQueue.tsx
- [ ] features/upload/components/ProgressIndicator.tsx
- [ ] features/upload/components/SupportedFormatsModal.tsx
- [ ] features/upload/components/UnifiedUploadSection.tsx
- [ ] features/upload/hooks/useUpload.tsx
- [ ] features/upload/hooks/useUploadProcess.tsx
- [ ] features/upload/UploadProvider.tsx
- [ ] features/share/components/CreateShareLinkModal.tsx
- [ ] features/share/components/PublicShareGrid.tsx
- [ ] features/share/components/PublicShareHeader.tsx
- [ ] features/share/components/PublicShareLightbox.tsx
- [ ] features/share/routes/PublicShare.tsx
- [ ] features/share/routes/SharedLinks.tsx
- [ ] features/share/utils/shareBulkAction.tsx
- [ ] features/monitor/components/CapabilitiesMonitor.tsx
- [ ] features/monitor/components/MLMonitor.tsx
- [ ] features/monitor/components/QueueSummaryList.tsx
- [ ] features/monitor/components/StatMonitor.tsx
- [ ] features/monitor/routes/Monitor.tsx

## Batch 7 — Studio

Canvas/editor layout: side panels should collapse to bottom drawers/tabs on
narrow widths; viewport must keep correct canvas sizing on resize. Worker
graph must stay DOM-free — breakpoint logic lives in React, never in workers.

- [ ] features/studio/develop/BorderToolSection.tsx
- [ ] features/studio/develop/DevelopPanel.tsx
- [ ] features/studio/develop/SectionHeader.tsx
- [ ] features/studio/develop/SliderRow.tsx
- [ ] features/studio/editor/AssetPanel.tsx
- [ ] features/studio/editor/StudioEditor.tsx
- [ ] features/studio/editor/TopBar.tsx
- [ ] features/studio/editor/Viewport.tsx
- [ ] features/studio/home/RecentEditItem.tsx
- [ ] features/studio/home/StudioHome.tsx
- [ ] features/studio/routes/StudioEditMvp.tsx
- [ ] features/studio/shared/PhotoThumb.tsx
- [ ] features/studio/tools/border/BorderPanel.tsx

## Batch 8 — Lumilio (agent chat + widgets)

ChatDock and AgentBoard: on phones the dock should become full-width; widget
tiles reflow to a single column. Keep pure daisyUI tokens.

- [ ] features/lumilio/components/blocks/ConfirmBlock.tsx
- [ ] features/lumilio/components/blocks/ReasoningBlock.tsx
- [ ] features/lumilio/components/blocks/ToolCallBlock.tsx
- [ ] features/lumilio/components/Board/AgentBoard.tsx
- [ ] features/lumilio/components/Chat/ChatDock.tsx
- [ ] features/lumilio/components/Chat/ChatMessages.tsx
- [ ] features/lumilio/components/Chat/ContextChips.tsx
- [ ] features/lumilio/components/Chat/MentionInput.tsx
- [ ] features/lumilio/components/LumilioAvatar/LumilioAvatar.tsx
- [ ] features/lumilio/components/LumilioMarkdown/Markdown.tsx
- [ ] features/lumilio/components/LumilioMarkdown/MarkdownBlocks/ImgBlock.tsx
- [ ] features/lumilio/components/LumilioMarkdown/MarkdownBlocks/index.tsx
- [ ] features/lumilio/components/LumilioMarkdown/MarkdownBlocks/LinkBlock.tsx
- [ ] features/lumilio/routes/LumilioChat.tsx
- [ ] features/lumilio/widgets/chrome/BoardTile.tsx
- [ ] features/lumilio/widgets/chrome/InlineWidgetCard.tsx
- [ ] features/lumilio/widgets/chrome/LiveBadge.tsx
- [ ] features/lumilio/widgets/chrome/MoreMenu.tsx
- [ ] features/lumilio/widgets/chrome/TileBody.tsx
- [ ] features/lumilio/widgets/chrome/TileHeader.tsx
- [ ] features/lumilio/widgets/chrome/ViewSwitcher.tsx
- [ ] features/lumilio/widgets/PinButton.tsx
- [ ] features/lumilio/widgets/views/CoverView.tsx
- [ ] features/lumilio/widgets/views/MosaicView.tsx
- [ ] features/lumilio/widgets/views/states.tsx
- [ ] features/lumilio/widgets/views/StatView.tsx
- [ ] features/lumilio/widgets/views/TimelineView.tsx
- [ ] features/lumilio/widgets/WidgetAssetThumbnail.tsx

## Validation

Per batch: `make web-test` + browser smoke at 375/768/1024/1440.

Final pass (after Batch 8):

- Walk every route in `routes/routes.tsx` at 375px and 1440px.
- Confirm desktop (Wails at localhost:6680) is visually unchanged at
  common window sizes — desktop is the primary product; no regressions there.
- Check both a light and a dark daisyUI theme once.

## Risks & Decisions

- **Desktop regressions** are the main risk: mobile-first rewrites of
  existing desktop-only classes must reproduce the current desktop look
  exactly. When rewriting a class list, diff the ≥`lg` result mentally
  against the original.
- **Virtualized galleries**: JustifiedGallery/SquareGallery derive layout from
  measured container width; the drawer collapse changes available width —
  verify remeasure-on-resize works instead of adding breakpoint CSS there.
- **Maps and canvas** (MapComponent, PhotoMapView, Studio Viewport) size via
  JS; ensure resize observers fire after layout changes (drawer open/close).
- **Hooks/providers with `.tsx`** often render no layout — mark them checked
  with a "no visual surface" note rather than forcing changes.
- **Scope creep**: this plan changes layout only. If a component's design
  itself is inadequate on mobile (needs a genuinely new UI), file it in the
  tech-debt tracker or ask for a Claude Design handoff instead of improvising
  a redesign mid-batch.

## Critical Files for Implementation

- web/src/App.tsx
- web/src/components/SideBar.tsx
- web/src/components/NavBar.tsx
- web/src/features/assets/components/page/AssetsGalleryPage.tsx
- web/src/hooks/util-hooks/useBreakpoint.ts (new)
