# Desktop Onboarding And Boot Experience

Status: completed 2026-07-06. Real-display UX review and native Windows smoke
remain release validation, not implementation work.

## Result

- Desktop startup reports named stages, applies per-stage timeouts, and includes
  PostgreSQL log tails in failures.
- Windows PostgreSQL paths are normalized, subprocess output uses a stable
  locale, and console tools do not open terminal windows.
- Native onboarding is intentionally thin: license acceptance, storage-root
  selection, writability/free-space checks, and an optional-AI hint.
- Account creation, language, MFA, repository setup, and duplicate policy stay
  in the browser `BootstrapWizard`.
- ML remains external, optional, and outside the boot-critical path.

## Contract to preserve

```text
desktop storage choice
  -> DesktopParams.StoragePath
  -> repository_defaults.default_root
  -> browser wizard displays the root read-only
```

Do not duplicate browser onboarding in the native shell or make Lumen startup a
prerequisite for photo management.

## Validation record

`make desktop-test` and an arm64 DMG build passed. A real Windows boot smoke and
visual pass remain part of release artifact validation.
