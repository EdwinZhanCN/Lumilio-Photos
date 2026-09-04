---
name: lumilio-frontend-i18n
description: Use when adding or changing any user-facing string in web/, or
  when defining, changing, reviewing, or applying canonical product terminology
  in UI, Desktop, CLI, docs, READMEs, release notes, or deployment tools —
  consult or update the terminology registry first, then extract/fill i18n and
  update the terminology gates.
---

# Frontend i18n And Product Terminology

Every user-facing string literal in `web/` goes through the i18n layer. The
extractor owns the key structure of `web/src/locales/{en,zh}/translation.json`;
hand edits to structure are overwritten or drift. Never manually add keys,
restructure nesting, or delete keys.

Canonical product terms apply everywhere a human reads them: Web UI, Desktop
UI, CLI TUI, `site/docs`, READMEs, release notes, and deployment tools.

## Canonical terminology registry

This table is the single registry for product terminology. Consult it before
writing human-facing text. Add or change a row here before introducing a new
product-term convention anywhere else. Keep each row's stable key, exact
bilingual labels, boundary definition, and forbidden synonyms together.

| Key | English | Simplified Chinese | Boundary definition | Never use as a synonym |
| --- | --- | --- | --- | --- |
| `storage-location` | `Storage Location` | `存储位置` | A host-authorized parent storage identity marked by `.lumilioroot`. It may contain zero or more direct-child Repositories; its path and display name are not its identity. | `Storage Root`, `Repository Root`; `存储根`, `存储根目录`, `资源库根` |
| `default-storage-location` | `Default Storage Location` | `默认存储位置` | The instance's single non-removable Storage Location registered from `storage.path`. Startup creates or validates its marker but does not create a Repository. | `Default Storage Root`; `默认存储根`, `默认资源库位置` |
| `repository` | `Repository` | `资源库` | A concrete media storage unit under a registered Storage Location, marked by `.lumiliorepo`, with a user-visible media tree and a private `.lumilio/` workspace. | `Library`, `Libraries`; `图库`, `媒体库`, `仓库` |
| `primary-repository` | `Primary Repository` | `主资源库` | The instance's unique active primary-role Repository at `<storage.path>/primary` in the Default Storage Location. Authenticated first-run setup creates it; storage startup does not. | `Primary Library`; `主图库`, `主媒体库`, `主仓库` |
| `siglip` | `Image Semantic Analysis` | `图像语义分析` | The Lumen `siglip` capability for semantic image analysis and the features it enables. | `Semantic Search`; `语义搜索` |
| `siglip-video` | `Image Semantic Analysis (video)` | `图像语义分析（视频）` | The video-qualified presentation of `siglip`, not a fifth Lumen service; it requires the parent `siglip` capability. | A standalone Video Semantic Analysis capability; 独立的“视频语义分析”能力 |
| `face` | `Person Recognition` | `人物识别` | The Lumen `face` capability for person-related face processing. | `Face Recognition`; `人脸识别` |
| `ocr` | `OCR Text Recognition` | `OCR文字识别` | The Lumen `ocr` capability for extracting and recognizing text. | bare `OCR`; 单独的 `OCR` |
| `bioclip` | `BioCLIP Species Recognition` | `BioCLIP物种识别` | The Lumen `bioclip` capability for species classification. | bare `Species Recognition`; 单独的 `物种识别` |

Use the exact labels when naming these entities. English plurals are `Storage
Locations` and `Repositories`; qualifiers do not create aliases—use `Default
Storage Location` and `Primary Repository`. Do not collapse a Storage Location
and a Repository into one concept.

## Technical identifier mapping

Keep stable technical identifiers unchanged and interpret them through this
map. They are implementation vocabulary, not additional product concepts.

| Product concept | Retained technical identifiers | Interpretation |
| --- | --- | --- |
| Storage Location | `.lumilioroot`, root UUID | Portable on-disk Storage Location identity; the marker name is a format, not a user-facing label. |
| Storage Location | `repository_roots`, `repositories.root_id`, `root_id` | Catalog row and foreign-key association for a Storage Location. |
| Storage Location | `repo.RepositoryRoot`, `RepositoryRoot*`, `RootID`, `ErrRepositoryRoot*` | Existing SQLC and Go symbols for Storage Location state, identity, lifecycle, and errors. |
| Storage Location | `/api/v1/repository-roots`, `root_id`, `roots`, `RepositoryRootDTO` | Existing HTTP/OpenAPI wire contract for Storage Location resources. |
| Storage Location | `useRepositoryRoots`, `repositoryRoot`, Repository Root query keys | Frontend adapters over that retained HTTP contract; UI text still says Storage Location / 存储位置. |
| Default Storage Location | `storage.path`, `repository_roots.kind = 'default'`, `GetDefaultRepositoryRoot` | The single non-removable Default Storage Location. |
| Repository | `.lumiliorepo`, `repositories`, `repo_id`, `repository_id`, `Repository*` | Repository identity, catalog rows, and domain symbols. |
| Primary Repository | `repositories.role = 'primary'`, `<storage.path>/primary` | The unique Primary Repository role and fixed directory. |

Do not rename an on-disk, database, durable-payload, or HTTP identifier solely
to match product copy. Do not add cosmetic aliases such as a second Go type for
`repo.RepositoryRoot`; translate terminology at human-facing boundaries. When
extending an existing technical contract, follow its mapped identifiers rather
than creating parallel spellings. Comments, API descriptions, logs intended for
operators, and UI text name the mapped product concept.

The Chinese exception `打开项目仓库` refers to the source-code repository and
is not a Repository synonym.

## Procedure

Run from `web/` (or use the stated root Task target):

1. Consult the registry. When adding or changing a product term, update its row
   first; do not establish a one-off label or definition in another file.
2. Write code: `t("dotted.key", "English default")`. The inline default records
   source intent, provides the runtime fallback, and tells the extractor the key
   exists. When the string names a registered product term, copy the English
   label from the table into the default; do not paraphrase it.
3. Extract with `task web:i18n:extract` from the root, or
   `vp exec i18next-cli extract` from `web/`. The extractor creates and removes
   keys, but preserves values for existing keys; it does not synchronize a
   changed inline default into the existing English translation.
4. Update the English value for each changed existing key, then fill or update
   the corresponding Chinese value. Modify values only for keys present after
   extraction. Use the table's labels for registered terms; never manually add,
   move, rename, or delete keys.
5. When a terminology row or forbidden synonym changes, update
   `web/src/lib/i18n/productTerminology.test.ts` and
   `server/tools/architecturecheck` in the same change wherever deterministic
   enforcement is possible. Keep exceptions narrow and technical.
6. Verify `vp exec i18next-cli status` from `web/` reports 100% zh coverage.
   Run `task web:test` for Web strings and `task architecture:check` whenever
   terminology or its cross-surface gate changes.

`task web:test` includes `web/src/lib/i18n/productTerminology.test.ts`.
`task architecture:check` scans human-facing Web/Desktop text, docs, READMEs,
API annotations, and generated API docs for retired product terms.

E2E and integration specs must not embed UI copy; they resolve names through
the `en` bundle
([lumilio-e2e-spec](../lumilio-e2e-spec/SKILL.md),
[lumilio-integration-spec](../lumilio-integration-spec/SKILL.md)).

## Report

Report the translation keys and terminology rows added or changed, the gates
updated, and the final `status` coverage figure.
