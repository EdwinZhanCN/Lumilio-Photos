---
name: lumilio-e2e-spec
description: Use when writing or debugging a Playwright E2E spec in Lumilio
  Photos — locator order, i18n-safe names, forbidden aria-label hooks, and
  assertions on data and API facts rather than copy.
---

# Write A Playwright E2E Spec

Specs live in `web/e2e/specs/*.spec.ts` and run against the isolated Compose
stack ([lumilio-e2e-environment](../lumilio-e2e-environment/SKILL.md)). They
run under `locale: "en-US"` (set in `playwright.config.ts`), which is what
i18next detects from `navigator`. Layer assignment:
[lumilio-write-a-test](../lumilio-write-a-test/SKILL.md).

## Locator order

1. `getByRole(role, { name })` with the name resolved through
   `e2e/support/i18n.ts`, which reads the same `en` bundle the app renders.
   Roles are semantic and are never translated; rewording a string keeps
   specs green, renaming a key fails them — which is the structural change
   that should fail.
2. Data anchors from `.cache/e2e/seed.json` via the `seed` export in
   `e2e/fixtures/test.ts`. Filenames and ids are data, not copy.
3. API and URL facts. `waitForResponse` on a real response beats waiting for
   UI wording, and it is the only reliable signal for work that continues
   after the request is accepted.
4. `getByTestId`, only for elements with no stable accessible semantics.
   Scope dynamic rows through a container test id plus a data attribute
   instead of interpolating a runtime id into the test id.

## Forbidden

- **UI copy literals in specs.** They couple tests to translations, so every
  wording change drags a spec change behind it.
- **`aria-label` as a test hook.** It is user-facing text that screen readers
  announce. Translated, it couples to copy exactly like visible text and
  buys nothing; frozen in English to stabilise tests, it breaks non-English
  assistive technology. On an element that already has visible text it also
  overrides the accessible name, violating WCAG 2.5.3 (Label in Name) and
  breaking voice control. Reserve `aria-label` for elements with no visible
  text, such as icon-only buttons.

Form fields get their accessible name from `<label htmlFor>` paired with
`<input id>`. `Field` in `features/auth/components/ui/Fields.tsx` generates
the id with `useId` and passes it down through context, so `TextInput` and
`PasswordField` pair automatically and no call site can forget. Follow that
shape when adding field components rather than re-exposing an optional
`htmlFor`, which is how the pairing was missed before.

Assert on data and API facts, not on wording; copy correctness belongs to
i18n, not to E2E. `getByLabel` matches substrings — pass `{ exact: true }`
where a shorter label would otherwise also match a longer one, as "Password"
does against the "Show password" toggle.

Type API bodies from generated OpenAPI types
(`components["schemas"]["dto.X"]`) rather than hand-written shapes, so a
browse-contract change breaks the build instead of failing at runtime.

## Verify

Run only the slice the spec belongs to:

```sh
task web:e2e:up
task web:test:browser          # or auth-hardening / auth-totp / agent-trust / video-semantic / backup-recovery
task web:e2e:down
```

Do not add a scheduled full-library matrix or fold a heavy slice into every
Web run. CI selects slices through path filters.
