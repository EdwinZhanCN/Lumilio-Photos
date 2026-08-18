# Decision: Present storage entities from semantic identity

Status: implemented

## Problem

The Server seeds English names for the Default Storage Location and Primary
Repository, and several Web surfaces rendered those API `name` fields directly.
The same system-owned entities therefore appeared untranslated in Manage and
Server Monitor even though their product terms were defined canonically. Page-
specific fixes would leave every future storage consumer exposed to the same
mistake, while matching the seeded English strings would make localization
depend on mutable data.

Storage diagnostics also omitted the Storage Location `kind` and Repository
`role`, so the Web could not identify these reserved entities without guessing.

## Decision

Storage APIs carry locale-neutral semantic identity. Repository-root responses
already expose `kind`; repository options already expose `role`; storage
diagnostics now expose the corresponding `kind` or `role` for every item.
Backend names remain raw data and are never localized by the Server.

The Repositories feature owns one discriminated `StorageEntity` presentation
model and one `getStorageEntityDisplayName` resolver. Its API adapters rename
transport `name` to `rawName`, normalize unknown identity values explicitly,
and map only `kind = default` and `role = primary` to the canonical
`productTerms` keys. Ordinary user-named Storage Locations and Repositories keep
their raw names. Technical confirmations and mutation payloads use `rawName`
when the exact stored value matters.

All cross-feature consumers use the Repositories public entry. The source-
boundary gate rejects direct storage presentation queries and use of storage
transport DTOs outside the owned adapters, and normalized types do not expose
a generic `name` field. The Primary Repository cannot be renamed from the Web
because its public product name is a fixed role label; showing a rename whose
result is intentionally hidden would be misleading.

## Alternatives considered

**Localize names in the Server** — rejected because locale is a presentation
concern and Server data, diagnostics, support bundles, and mutations need a
stable raw value independent of the current client language.

**Match `Default storage` or `Primary Storage` strings in the Web** — rejected
because names are mutable historical data, not identity. String matching would
break after renames, seed changes, imports, or another language.

**Add page-specific display helpers** — rejected because Manage, Monitor,
upload, browse, settings, and future consumers would drift again. One owned
model, resolver, and boundary gate makes the rule reusable and enforceable.

**Rename every technical identifier** — rejected because `RepositoryRootDTO`,
database columns, URLs, and existing domain symbols are stable protocol names.
The canonical terminology registry already records their product mapping.
