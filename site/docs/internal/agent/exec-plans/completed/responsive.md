# Responsive Adaptation

Status: completed. All planned code-review batches and `make web-test` passed;
the final authenticated multi-viewport browser walk remains release validation.

## Result

- App shell navigation collapses for narrow viewports and overlay/dropdown
  surfaces are viewport-clamped.
- Auth and standalone share routes use dynamic viewport height.
- Asset, collection, settings, upload, share, monitor, and people surfaces wrap
  or reflow without changing their desktop information architecture.
- Studio moves its develop panel to a mobile bottom sheet while retaining the
  desktop sidebar.
- Hover-only actions needed on touch devices are visible on small screens.

## Validation still required

Walk authenticated routes at 375, 768, 1024, and 1440 px in one light and one
dark theme. Pay particular attention to the mobile drawer, fullscreen asset
info, collection stack grids, Studio develop sheet, virtualized gallery resize,
maps, and canvas sizing.

## Known exception

`web/src/features/lumilio/components/Board/AgentBoard.tsx` still uses one
persisted 12-column layout at every width. Mobile reflow needs an explicit
layout-migration design and is tracked in `tech-debt-tracker.md`.
