# Decision: Maintain one canonical product terminology registry

Status: implemented — `lumilio-frontend-i18n` owns the bilingual registry and
the procedure for changing and enforcing it.

## Problem

Product terminology was split between a Lumen-only table, Repository synonym
rules, user documentation, and architecture-check regular expressions. Storage
Location and Repository had consistent implementation semantics but no single
agent-loaded registry that named both entities, their reserved qualified forms,
their boundary, and their forbidden synonyms. A future term could therefore be
introduced correctly in one surface and drift elsewhere.

## Decision

The canonical terminology registry is a compact Markdown table in
[`lumilio-frontend-i18n`](../skills/lumilio-frontend-i18n/SKILL.md). Each row has
a stable non-user-facing key, exact English and Simplified Chinese labels, one
boundary definition, and forbidden synonyms. Storage terms and Lumen capability
labels use the same table and change protocol.

Every human-facing surface consults the registry. A new or changed convention
updates the registry first, then its translations and deterministic terminology
gates in the same change. Existing protocol, model, database, and API identifiers
remain stable; the registry explicitly maps technical identifiers when their
names differ from the product concept.

The table stays inline while it is small and core to every terminology task. If
it approaches the skill's context budget, move it without duplication to one
direct `references/` file and make reading that file a mandatory skill step.

## Alternatives considered

**Use JSON or YAML as the source of truth** — rejected for now. Machine-readable
data would help generate gates, but nuanced boundary definitions and technical
exceptions are harder to review there, and the current registry does not justify
a parser or generation workflow.

**Keep separate storage and Lumen tables** — rejected because contributors
would need to discover which naming rules belong to which table, recreating the
same drift this decision removes.

**Put the registry only in the user glossary** — rejected because the i18n
workflow must load the rules before editing code, Desktop strings, API prose, or
release material. The public glossary remains useful reader documentation but
does not own the agent procedure or enforcement workflow.

**Move the small table into a separate skill reference immediately** — rejected
because it is core context rather than optional detail. An inline table is
loaded reliably and remains well below the skill size limit.

**Rename every existing technical identifier to match product terminology** —
rejected because it would couple a wording cleanup to database migrations,
SQLC regeneration, the HTTP v1 wire contract, generated clients and docs,
durable lifecycle payloads, and historical migration vocabulary. That risk has
no matching product benefit. Preserve the identifiers and maintain one explicit
mapping from each technical layer to the canonical product concept instead.
