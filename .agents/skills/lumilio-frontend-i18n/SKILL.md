---
name: lumilio-frontend-i18n
description: Use when adding or changing any user-facing string in web/, or
  when naming a Lumen capability (siglip, face, ocr, bioclip) in UI, docs,
  READMEs, or release notes — extract-then-fill i18n, never-hand-edit
  translation JSON, and the canonical product terminology table.
---

# Frontend i18n And Product Terminology

Every user-facing string literal in `web/` goes through the i18n layer. The
extractor owns the key structure of `web/src/locales/{en,zh}/translation.json`;
hand edits to structure are overwritten or drift. Never manually add keys,
restructure nesting, or delete keys.

Canonical product labels (Lumen capabilities, Repository, …) apply everywhere
a human reads them: Web UI, Desktop UI, CLI TUI, `site/docs`, READMEs, release
notes, and deployment tools. Protocol task names, model names, database
fields, and API identifiers stay unchanged.

## Procedure

Run from `web/` (or `task web:i18n:extract` from the root for step 2):

1. Write code first: `t("dotted.key", "English default")`. The inline default
   doubles as the `en` value and tells the extractor the key exists. When the
   string names a Lumen capability, copy the English label from the table
   below into the default — do not re-translate it.
2. Extract: `vp exec i18next-cli extract` — scans `src/**/*.{ts,tsx}` and
   creates/updates keys in both locale files.
3. Fill Chinese: open `src/locales/zh/translation.json` and translate the
   new or empty keys. Only fill values for keys the extractor created.
   Capability labels use the Chinese column of the table, never a paraphrase.
4. Verify: `vp exec i18next-cli status` must report 100% zh coverage.

`task web:test` includes `web/src/lib/i18n/productTerminology.test.ts`, which
asserts the four capability labels exist and the forbidden paraphrases do
not. A new capability updates this table and that test in the same PR.

E2E and integration specs must not embed UI copy; they resolve names through
the `en` bundle
([lumilio-e2e-spec](../lumilio-e2e-spec/SKILL.md),
[lumilio-integration-spec](../lumilio-integration-spec/SKILL.md)).

## Lumen capability labels

Exact product terms. Descriptions may explain that a capability enables
natural-language search, face processing, text extraction, or species
classification; the label itself never paraphrases.

| Internal service | Simplified Chinese | English |
| --- | --- | --- |
| `siglip` | `图像语义分析` | `Image Semantic Analysis` |
| `face` | `人物识别` | `Person Recognition` |
| `ocr` | `OCR文字识别` | `OCR Text Recognition` |
| `bioclip` | `BioCLIP物种识别` | `BioCLIP Species Recognition` |

Forbidden labels (and close variants):

| Do not use | Use instead |
| --- | --- |
| `语义搜索` / `Semantic Search` | `图像语义分析` / `Image Semantic Analysis` |
| `人脸识别` / `Face Recognition` | `人物识别` / `Person Recognition` |
| bare `OCR` | `OCR文字识别` / `OCR Text Recognition` |
| bare `物种识别` / `Species Recognition` | `BioCLIP物种识别` / `BioCLIP Species Recognition` |

Video semantics is not a fifth service. Its UI label is
`图像语义分析（视频）` / `Image Semantic Analysis (video)`, and enabling it
requires the parent `siglip` capability.

## Other product terms

Gated by the same `productTerminology.test.ts`:

- English never uses `library` / `libraries` as a synonym for a Repository.
- Chinese never uses `仓库` or `图库` as a synonym for `资源库` (the one
  allowed exception is the existing string `打开项目仓库`).

## Report

Report the keys added or changed, any terminology table/test update, and the
final `status` coverage figure.
